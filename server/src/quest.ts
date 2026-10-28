/** 旧版 TypeScript 迁移参考：任务进度、接取交付和 NPC 话题。默认服务启动执行 server/*.go。 */

/**
 * 任务进度。
 * 目标是否完成只看服务器记下的 progress，不看客户端自己报的击杀数。
 */

import type { Objective, QuestDef } from './content/schema.ts'
import type { App } from './app.ts'
import { one, many, run } from './db.ts'
import { emit, type GameEvent } from './events.ts'
import { countItem, addItem, makeEquipmentExtra } from './inventory.ts'
import { notify, toast } from './net.ts'
import { addExp, addGold, getPlayer, readFlags, type PlayerRow } from './player.ts'
import { givePet } from './pets.ts'
import { dist, GameError, now, QUALITY_RANK } from './util.ts'

/** 角色任务记录或视图，包含任务标识与当前进度信息。 */
interface QuestRow {
  /** 记录所属角色标识，业务查询同时校验归属。 */
  player_id: string
  /** 当前记录对应的任务模板 ID。 */
  quest_id: string
  /** 当前业务状态，用于校验操作条件和防止重复结算。 */
  status: string
  /** 服务器记录的任务目标进度 JSON。 */
  progress: string
  /** 记录最近写入时间，使用 Unix 毫秒时间戳。 */
  updated_at: number
}

/** 返回任务目标完成门槛，事件型目标默认需要一次。 */
function needOf(goal: Objective): number {
  if ('count' in goal && typeof goal.count === 'number') return goal.count
  return 1
}

/** 用现有等级、背包物品和公会关系初始化新接任务的进度。 */
function seed(app: App, player: PlayerRow, quest: QuestDef): number[] {
  return quest.objectives.map((goal) => {
    if (goal.type === 'reach_level') return player.level >= goal.level ? 1 : 0
    if (goal.type === 'collect') return Math.min(goal.count, countItem(app, player.id, goal.itemId))
    if (goal.type === 'join_guild') {
      const member = one(app.db, 'SELECT guild_id FROM guild_members WHERE player_id = ?', player.id)
      return member ? 1 : 0
    }
    return 0
  })
}

/** 读取角色的指定任务状态与服务器记录的进度。 */
export function questState(app: App, playerId: string, questId: string): QuestRow | undefined {
  return one<QuestRow>(app.db, 'SELECT * FROM player_quests WHERE player_id = ? AND quest_id = ?', playerId, questId)
}

/** 检查等级、解锁标记、前置任务和历史领取状态，决定任务是否可接。 */
export function canAccept(app: App, player: PlayerRow, quest: QuestDef): boolean {
  const existing = questState(app, player.id, quest.id)
  if (existing) return false
  if (player.level < quest.minLevel) return false
  if (quest.flag && !readFlags(player)[quest.flag]) return false
  for (const pre of quest.prereq) {
    const row = questState(app, player.id, pre)
    if (!row || row.status !== 'turned_in') return false
  }
  return true
}

/** 创建活动任务，并用当前等级、背包材料和公会状态初始化适用目标。 */
export function acceptQuest(app: App, playerId: string, questId: string, force = false) {
  const quest = app.quests.get(questId)
  if (!quest) throw new GameError('没有这个任务')
  const player = getPlayer(app, playerId)
  if (!force && !canAccept(app, player, quest)) throw new GameError('现在还不能接')
  if (questState(app, playerId, questId)) throw new GameError('已经接过了')
  const progress = seed(app, player, quest)
  run(
    app.db,
    'INSERT INTO player_quests(player_id, quest_id, status, progress, updated_at) VALUES(?, ?, ?, ?, ?)',
    playerId,
    questId,
    'active',
    JSON.stringify(progress),
    now(),
  )
  notify(app, playerId)
}

/** 仅在所有任务目标都达到服务器记录的需求时返回完成。 */
function done(quest: QuestDef, progress: number[]): boolean {
  return quest.objectives.every((goal, index) => (progress[index] ?? 0) >= needOf(goal))
}

/** 将玩法事件匹配到活动任务，写入进度并通知可交付状态。 */
export function onGameEvent(app: App, playerId: string, ev: GameEvent) {
  const rows = many<QuestRow>(app.db, `SELECT * FROM player_quests WHERE player_id = ? AND status = 'active'`, playerId)
  let changed = false
  let ready = false
  for (const row of rows) {
    const quest = app.quests.get(row.quest_id)
    if (!quest) continue
    const progress = JSON.parse(row.progress) as number[]
    let local = false
    quest.objectives.forEach((goal, index) => {
      const need = needOf(goal)
      if ((progress[index] ?? 0) >= need) return
      const step = applyGoal(goal, ev)
      if (step === null) return
      progress[index] = Math.min(need, (progress[index] ?? 0) + step)
      local = true
    })
    if (!local) continue
    run(
      app.db,
      'UPDATE player_quests SET progress = ?, updated_at = ? WHERE player_id = ? AND quest_id = ?',
      JSON.stringify(progress),
      now(),
      playerId,
      row.quest_id,
    )
    changed = true
    if (done(quest, progress)) ready = true
  }
  if (changed) notify(app, playerId)
  if (ready) toast(app, playerId, '有任务可以交付了')
}

/** 返回本次增加的进度；不相关则返回 null。 */
function applyGoal(goal: Objective, ev: GameEvent): number | null {
  switch (goal.type) {
    case 'kill':
      return ev.type === 'kill' && ev.monsterId === goal.monsterId ? 1 : null
    case 'kill_any':
      return ev.type === 'kill' ? 1 : null
    case 'collect':
      return ev.type === 'collect' && ev.itemId === goal.itemId ? ev.count : null
    case 'capture':
      return ev.type === 'capture' && ev.petId === goal.petId ? 1 : null
    case 'craft':
      return ev.type === 'craft' && ev.recipeId === goal.recipeId ? 1 : null
    case 'dispatch_start':
      return ev.type === 'dispatch_start' ? 1 : null
    case 'dispatch_claim':
      return ev.type === 'dispatch_claim' ? 1 : null
    case 'market_list':
      return ev.type === 'market_list' ? 1 : null
    case 'market_buy':
      return ev.type === 'market_buy' ? 1 : null
    case 'enter':
      return ev.type === 'enter' && ev.mapId === goal.mapId ? 1 : null
    case 'gather':
      return ev.type === 'gather' && ev.nodeKind === goal.nodeKind ? 1 : null
    case 'join_guild':
      return ev.type === 'join_guild' ? 1 : null
    case 'donate':
      return ev.type === 'donate' ? 1 : null
    case 'reach_level':
      return ev.type === 'level' && ev.level >= goal.level ? 1 : null
    case 'world_boss':
      return ev.type === 'world_boss' ? 1 : null
    case 'buy_shop':
      return ev.type === 'buy_shop' ? 1 : null
    case 'bank':
      return ev.type === 'bank' ? 1 : null
    case 'equip_quality':
      return ev.type === 'equip' && (QUALITY_RANK[ev.quality] ?? 0) >= (QUALITY_RANK[goal.quality] ?? 99) ? 1 : null
    case 'talk':
      return ev.type === 'talk' && ev.npcId === goal.npcId ? 1 : null
    default:
      return null
  }
}

/** 校验任务完成后标记交付、发放奖励，并按前置条件接续后续主线。 */
export function turnIn(app: App, playerId: string, questId: string) {
  const quest = app.quests.get(questId)
  if (!quest) throw new GameError('没有这个任务')
  const row = questState(app, playerId, questId)
  if (!row || row.status !== 'active') throw new GameError('任务还没接')
  const progress = JSON.parse(row.progress) as number[]
  if (!done(quest, progress)) throw new GameError('还没完成')
  run(
    app.db,
    `UPDATE player_quests SET status = 'turned_in', updated_at = ? WHERE player_id = ? AND quest_id = ?`,
    now(),
    playerId,
    questId,
  )
  if (quest.rewards.exp) addExp(app, playerId, quest.rewards.exp)
  if (quest.rewards.gold) addGold(app, playerId, quest.rewards.gold)
  for (const item of quest.rewards.items ?? []) {
    const tpl = app.items.get(item.itemId)
    if (tpl?.kind === 'equipment') addItem(app, playerId, item.itemId, item.count, makeEquipmentExtra(app, item.quality ?? 'white'))
    else addItem(app, playerId, item.itemId, item.count)
  }
  if (quest.rewards.petId) givePet(app, playerId, quest.rewards.petId, false)
  toast(app, playerId, `完成了「${quest.name}」`)
  if (quest.next) {
    const player = getPlayer(app, playerId)
    const next = app.quests.get(quest.next)
    if (next && canAccept(app, player, next)) acceptQuest(app, playerId, next.id)
  }
  notify(app, playerId)
}

/** 要求角色进入世界、处于冒险模式并靠近同地图 NPC。 */
export function assertNearNpc(app: App, playerId: string, npcId: string) {
  const npc = app.npcs.get(npcId)
  if (!npc) throw new GameError('没有这个人')
  const live = app.live.get(playerId)
  if (!live) throw new GameError('先进入世界')
  if (live.safe) throw new GameError('助手模式下不能交谈，切回冒险')
  if (live.mapId !== npc.mapId || dist(live.x, live.y, npc.x, npc.y) > 90) throw new GameError('再走近一点')
  return npc
}

/** NPC 对话中的接取、交付或进度话题，包含目标说明与奖励文字。 */
interface DialogTopic {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 物品、地图、技能事件或话题的类别标识。 */
  kind: 'accept' | 'turnin' | 'progress'
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 客户端展示的内容说明。 */
  desc: string
  /** 对话中需要展示的目标、进度或下一步说明。 */
  detail: string
  /** 已经转换为中文的任务奖励说明。 */
  reward: string
}

/** 将任务经验、金币、物品和灵伴奖励拼成对话展示文本。 */
function rewardText(app: App, quest: QuestDef): string {
  const parts: string[] = []
  if (quest.rewards.exp) parts.push(`${quest.rewards.exp} 经验`)
  if (quest.rewards.gold) parts.push(`${quest.rewards.gold} 金币`)
  for (const item of quest.rewards.items ?? []) {
    const name = app.items.get(item.itemId)?.name ?? item.itemId
    parts.push(item.count > 1 ? `${name} ×${item.count}` : name)
  }
  if (quest.rewards.petId) parts.push(app.pets.get(quest.rewards.petId)?.name ?? '一只灵伴')
  return parts.join('，')
}

/** 把每个目标的当前进度与需求拼成 NPC 对话说明。 */
function progressText(quest: QuestDef, progress: number[]): string {
  return quest.objectives
    .map((goal, index) => `${goal.text} ${Math.min(progress[index] ?? 0, needOf(goal))}/${needOf(goal)}`)
    .join('，')
}

/** 按可交付、可接取和进行中的任务选择 NPC 开场文本。 */
function openingLine(npcLines: string[], turnins: DialogTopic[], accepts: DialogTopic[], pending: DialogTopic[]) {
  if (turnins.length === 1) return `「${turnins[0]!.name}」可以交差了。`
  if (turnins.length > 1) return `${turnins.map((topic) => `「${topic.name}」`).join('、')}可以交差了。`
  if (accepts.length === 0 && pending.length === 1) {
    const topic = pending[0]!
    if (topic.detail.startsWith('做完了')) return `「${topic.name}」${topic.detail}。`
    return `「${topic.name}」还没做完：${topic.detail}。`
  }
  if (accepts.length === 0 && pending.length > 1) return '这几件事还没做完。'
  if (accepts.length === 1 && pending.length === 0) return accepts[0]!.desc
  return npcLines[Math.floor(Math.random() * npcLines.length)] ?? '……'
}

/** 根据角色任务状态生成对话话题、接交任务操作和 NPC 服务入口。 */
export function openDialog(app: App, playerId: string, npcId: string) {
  const npc = assertNearNpc(app, playerId, npcId)
  emit(app, playerId, { type: 'talk', npcId })
  const player = getPlayer(app, playerId)
  const actions: { id: string; label: string }[] = []
  const turnins: DialogTopic[] = []
  const accepts: DialogTopic[] = []
  const pending: DialogTopic[] = []
  for (const quest of app.data.quests) {
    if (quest.giver === npcId && canAccept(app, player, quest)) {
      actions.push({ id: `accept:${quest.id}`, label: `接受：${quest.name}` })
      accepts.push({
        id: `accept:${quest.id}`,
        kind: 'accept',
        name: quest.name,
        desc: quest.desc,
        detail: quest.objectives.map((goal) => goal.text).join('，'),
        reward: rewardText(app, quest),
      })
    }
    const row = questState(app, playerId, quest.id)
    if (!row || row.status !== 'active') continue
    if (quest.giver !== npcId && quest.turnin !== npcId) continue
    const progress = JSON.parse(row.progress) as number[]
    const finished = done(quest, progress)
    if (quest.turnin === npcId && finished) {
      actions.push({ id: `turnin:${quest.id}`, label: `交付：${quest.name}` })
      turnins.push({
        id: `turnin:${quest.id}`,
        kind: 'turnin',
        name: quest.name,
        desc: quest.desc,
        detail: '',
        reward: rewardText(app, quest),
      })
      continue
    }
    if (!finished) {
      pending.push({
        id: `progress:${quest.id}`,
        kind: 'progress',
        name: quest.name,
        desc: quest.desc,
        detail: progressText(quest, progress),
        reward: rewardText(app, quest),
      })
      continue
    }
    if (quest.giver === npcId && quest.turnin !== npcId) {
      const who = app.npcs.get(quest.turnin)?.name ?? '委托人'
      pending.push({
        id: `progress:${quest.id}`,
        kind: 'progress',
        name: quest.name,
        desc: quest.desc,
        detail: `做完了，交给${who}`,
        reward: rewardText(app, quest),
      })
    }
  }
  for (const panel of npc.panels) {
    const label: Record<string, string> = {
      shop: '买东西',
      craft: '打造 / 料理',
      market: '拍卖行',
      dispatch: '派遣灵伴',
      guild: '公会',
      bank: '仓库',
      travel: '航标',
      skills: '技能',
    }
    actions.push({ id: `open:${panel}`, label: label[panel] ?? panel })
  }
  const topics = [...turnins, ...accepts, ...pending]
  return {
    npcId,
    name: npc.name,
    title: npc.title,
    text: openingLine(npc.lines, turnins, accepts, pending),
    actions,
    topics,
  }
}
