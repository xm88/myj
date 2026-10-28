/** 旧版 TypeScript 迁移参考：完整档案和公开配置视图。默认服务启动执行 server/*.go。 */

/**
 * 给网页和助手用的角色档案。
 * 战斗快照只带血量和坐标；背包、任务、派遣都从这里读，保证和数据库一致。
 */

import type { Objective } from './content/schema.ts'
import type { App } from './app.ts'
import { many, one } from './db.ts'
import { attributeBudget, attributeRows, expToNext, lifeExpToNext, masteryExpToNext, parseAttributes, parseRace, PERIOD_LABEL, RACES, spentAttributes, totalAttributes, WEAPON_LABEL } from './formula.ts'
import { listItems } from './inventory.ts'
import { parseExtra } from './stats.ts'
import { getPlayer, getTrack, readFlags, type PlayerRow } from './player.ts'
import { canAccept } from './quest.ts'
import { listPets, runningDispatches } from './pets.ts'
import { computeStats, parseBuffs } from './stats.ts'
import type { ItemExtra, WeaponType } from './types.ts'

/** 读取任务目标所需次数，无数量字段的目标按一次处理。 */
function goalNeed(goal: Objective): number {
  if ('count' in goal && typeof goal.count === 'number') return goal.count
  return 1
}

/** 提取任务目标的怪物、地图、NPC、灵伴、资源种类或物品标识。 */
function goalTarget(goal: Objective): string {
  if ('monsterId' in goal) return goal.monsterId
  if ('mapId' in goal) return goal.mapId
  if ('npcId' in goal) return goal.npcId
  if ('petId' in goal) return goal.petId
  if ('nodeKind' in goal) return goal.nodeKind
  if ('itemId' in goal) return goal.itemId
  return ''
}

/** 把数据库物品与模板、品质、词条合并成网页可用的物品视图。 */
function viewItem(app: App, row: { id: string; template_id: string; count: number; location: string; extra: string | null }) {
  const tpl = app.items.get(row.template_id)
  const extra: ItemExtra | null = row.extra ? parseExtra(row.extra) : null
  return {
    id: row.id,
    templateId: row.template_id,
    name: tpl?.name ?? row.template_id,
    desc: tpl?.desc ?? '',
    kind: tpl?.kind ?? 'material',
    count: row.count,
    location: row.location,
    slot: tpl?.slot ?? null,
    weaponType: tpl?.weaponType ?? null,
    bound: Boolean(tpl?.bound),
    sell: tpl?.sell ?? 0,
    quality: extra?.quality ?? null,
    affixes: extra?.affixes ?? [],
  }
}

/** 合并存档、在线状态与模板，输出背包、装备、任务、派遣等完整档案。 */
export function buildProfile(app: App, playerId: string) {
  const row = getPlayer(app, playerId)
  const live = app.live.get(playerId)
  const level = live?.level ?? row.level
  const exp = live?.exp ?? row.exp
  const weapon = (live?.weaponType ?? row.weapon_type) as WeaponType
  const buffs = (live?.buffs ?? parseBuffs(row.buffs)).filter((buff) => buff.until > Date.now())
  const stats = live?.stats ?? computeStats(app, playerId, level, weapon, buffs)
  const items = listItems(app, playerId)
  const equipment: Record<string, ReturnType<typeof viewItem> | null> = {
    weapon: null, head: null, chest: null, gloves: null, boots: null, ring: null,
  }
  const bag = []
  const bank = []
  for (const item of items) {
    const view = viewItem(app, item)
    if (item.location === 'equipped' && view.slot) equipment[view.slot] = view
    else if (item.location === 'bank') bank.push(view)
    else if (item.location === 'bag') bag.push(view)
  }

  const pets = listPets(app, playerId).map((pet) => {
    const def = app.pets.get(pet.template_id)
    return {
      id: pet.id,
      templateId: pet.template_id,
      name: def?.name ?? pet.template_id,
      rarity: def?.rarity ?? 'common',
      battle: def?.battle ?? '',
      work: def?.work ?? '',
      desc: def?.desc ?? '',
      level: pet.level,
      exp: pet.exp,
      expMax: 24 + pet.level * 18,
      active: Boolean(pet.active),
      busy: runningDispatches(app, playerId).some((job) => job.player_pet_id === pet.id),
    }
  })

  const dispatches = runningDispatches(app, playerId).map((job) => {
    const pet = pets.find((item) => item.id === job.player_pet_id)
    const node = app.nodes.get(job.node_id)
    return {
      id: job.id,
      petName: pet?.name ?? '灵伴',
      nodeId: job.node_id,
      nodeName: node?.name ?? job.node_id,
      durationId: job.duration_id,
      endsAt: job.ends_at,
      ready: Date.now() >= job.ends_at,
    }
  })

  const questRows = many<{ quest_id: string; status: string; progress: string }>(
    app.db,
    'SELECT quest_id, status, progress FROM player_quests WHERE player_id = ?',
    playerId,
  )
  const quests = questRows
    .filter((item) => item.status === 'active')
    .map((item) => {
      const def = app.quests.get(item.quest_id)
      const progress = JSON.parse(item.progress) as number[]
      return {
        id: item.quest_id,
        name: def?.name ?? item.quest_id,
        desc: def?.desc ?? '',
        kind: def?.kind ?? 'side',
        turnin: def ? app.npcs.get(def.turnin)?.name ?? '' : '',
        turninId: def?.turnin ?? '',
        objectives: (def?.objectives ?? []).map((goal, index) => ({
          text: goal.text,
          current: progress[index] ?? 0,
          need: goalNeed(goal),
          type: goal.type,
          target: goalTarget(goal),
        })),
      }
    })

  const available = app.data.quests.filter((quest) => canAccept(app, row, quest)).map((quest) => ({
    id: quest.id,
    name: quest.name,
    desc: quest.desc,
    kind: quest.kind,
    giver: app.npcs.get(quest.giver)?.name ?? '',
    giverId: quest.giver,
    minLevel: quest.minLevel,
  }))

  const member = one<{ guild_id: string; role: string }>(app.db, 'SELECT guild_id, role FROM guild_members WHERE player_id = ?', playerId)
  let guild = null
  if (member) {
    const info = one<{ id: string; name: string; level: number; exp: number; notice: string; leader_id: string }>(
      app.db,
      'SELECT * FROM guilds WHERE id = ?',
      member.guild_id,
    )
    const members = many<{ player_id: string; role: string }>(app.db, 'SELECT player_id, role FROM guild_members WHERE guild_id = ?', member.guild_id)
    guild = {
      id: info?.id,
      name: info?.name,
      level: info?.level ?? 1,
      exp: info?.exp ?? 0,
      notice: info?.notice ?? '',
      role: member.role,
      members: members.map((item) => ({
        id: item.player_id,
        role: item.role,
        name: one<PlayerRow>(app.db, 'SELECT * FROM players WHERE id = ?', item.player_id)?.name ?? '',
      })),
    }
  }

  const unread = one<{ n: number }>(app.db, 'SELECT COUNT(*) AS n FROM mail WHERE player_id = ? AND is_read = 0', playerId)
  const weaponTrack = getTrack(app, playerId, `weapon:${weapon}`)
  const flags = readFlags(row)

  return {
    id: row.id,
    name: row.name,
    level,
    exp,
    expMax: expToNext(level),
    gold: live?.gold ?? row.gold,
    mapId: live?.mapId ?? row.map_id,
    x: live?.x ?? row.x,
    y: live?.y ?? row.y,
    hp: live?.hp ?? row.hp,
    maxHp: stats.maxHp,
    mp: live?.mp ?? row.mp,
    maxMp: stats.maxMp,
    weaponType: weapon,
    weaponName: WEAPON_LABEL[weapon] ?? weapon,
    loadout: live?.loadout ?? (JSON.parse(row.skill_loadout) as string[]),
    ...attributeProfile(level, row.attributes, row.race),
    stats,
    buffs,
    equipment,
    bag,
    bank,
    pets,
    dispatches,
    quests,
    available,
    guild,
    unreadMail: Number(unread?.n ?? 0),
    tracks: {
      weapon: { ...weaponTrack, expMax: masteryExpToNext(weaponTrack.level) },
      gather: withCurve(getTrack(app, playerId, 'life:gather')),
      craft: withCurve(getTrack(app, playerId, 'life:craft')),
      cook: withCurve(getTrack(app, playerId, 'life:cook')),
    },
    weather: app.data.weathers.find((item) => item.id === app.weather.id),
    period: app.period,
    periodName: PERIOD_LABEL[app.period],
    flags,
    online: Boolean(live?.socket),
    safe: Boolean(live?.safe),
    devTools: app.devTools,
  }
}

/** 生成种族说明、潜力余额和五项属性展示行。 */
function attributeProfile(level: number, raw: string | null | undefined, raceRaw: string | null | undefined) {
  const race = parseRace(raceRaw)
  const spent = parseAttributes(raw)
  const info = RACES[race]
  return {
    race,
    raceName: info.name,
    raceBlurb: info.blurb,
    raceGrowth: (['vit', 'mag', 'str', 'end', 'agi'] as const)
      .filter((key) => info.growth[key] > 0)
      .map((key) => {
        const name = { vit: '体力', mag: '魔力', str: '力量', end: '耐力', agi: '敏捷' }[key]
        return `${name} +${info.growth[key]}`
      })
      .join('，'),
    attributes: spent,
    unspent: Math.max(0, attributeBudget(level) - spentAttributes(spent)),
    attributeRows: attributeRows(totalAttributes(level, spent, race), race),
  }
}

/** 给生活熟练视图补充下一等级所需经验。 */
function withCurve(track: { level: number; exp: number }) {
  return { ...track, expMax: lifeExpToNext(track.level) }
}

/** 输出客户端显示与操作需要的配置，怪物列表省略服务器战斗结算细节。 */
export function publicConfig(app: App) {
  return {
    version: '0.1.0',
    maps: app.data.maps,
    npcs: app.data.npcs,
    monsters: app.data.monsters.map((monster) => ({
      id: monster.id,
      name: monster.name,
      level: monster.level,
      role: monster.role,
      tint: monster.tint,
      petId: monster.petId ?? null,
      element: monster.element,
    })),
    items: app.data.items,
    skills: app.data.skills,
    pets: app.data.pets,
    quests: app.data.quests.map((quest) => ({
      id: quest.id,
      name: quest.name,
      desc: quest.desc,
      kind: quest.kind,
      minLevel: quest.minLevel,
    })),
    recipes: app.data.recipes,
    durations: app.data.durations,
    dispatchNodes: app.data.dispatchNodes,
    weathers: app.data.weathers,
    constants: app.data.constants,
  }
}
