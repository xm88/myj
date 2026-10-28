/** 旧版 TypeScript 迁移参考：装备、灵伴、增益和种族属性叠加。默认服务启动执行 server/*.go。 */

/**
 * 角色属性。
 * 等级、武器、装备词条、武器熟练、主战灵伴光环叠在一起。
 * Buff 不写进这个基础值，命中时再乘，这样到期会立刻失效。
 */

import type { App } from './app.ts'
import { many, one } from './db.ts'
import { getPlayer, getTrack } from './player.ts'
import type { ComputedStats, LiveBuff, LivePlayer } from './runtime.ts'
import type { Element, ItemExtra, StatBlock, WeaponType } from './types.ts'
import { applyAttributeBonuses, parseAttributes, parseRace, QUALITY_TABLE, totalAttributes } from './formula.ts'

/** 面板初始化时需要覆盖的所有伤害元素。 */
const ELEMENTS: Element[] = ['physical', 'fire', 'water', 'thunder', 'wind', 'earth']

/** 不同武器的普攻间隔、射程和面板风格加成。 */
const WEAPON_STYLE: Record<WeaponType, { interval: number; range: number; atkPct?: number; def?: number; hp?: number; mp?: number; crit?: number }> = {
  sword_shield: { interval: 1100, range: 64, def: 4, hp: 12 },
  greatsword: { interval: 1450, range: 72, atkPct: 0.08 },
  staff: { interval: 1200, range: 220, mp: 22, atkPct: -0.04 },
  bow: { interval: 900, range: 250, crit: 0.04 },
}

/** 创建默认战斗面板，初始化所有元素加成和抗性。 */
export function emptyStats(): ComputedStats {
  const elem = Object.fromEntries(ELEMENTS.map((e) => [e, 0])) as Record<Element, number>
  return {
    maxHp: 100,
    maxMp: 40,
    atk: 8,
    spell: 0,
    def: 4,
    critRate: 0.05,
    critDmg: 1.5,
    speed: 170,
    attackInterval: 1200,
    range: 64,
    healPct: 1,
    elem,
    elemRes: { ...elem },
  }
}

/** 数据库物品实例记录，保存归属、模板、数量、位置和随机词条。 */
interface ItemRow {
  /** 关联配置模板的标识，实例数值和模板规则分开保存。 */
  template_id: string
  /** 物品当前位于背包、仓库、装备或其他业务位置。 */
  location: string
  /** 装备实例的品质与词条 JSON，普通堆叠材料为空。 */
  extra: string | null
}

/** 叠加等级、武器、装备词条、熟练、主战灵伴、增益和种族属性。 */
export function computeStats(app: App, playerId: string, level: number, weaponType: WeaponType, buffs: LiveBuff[] = []): ComputedStats {
  const stats = emptyStats()
  const style = WEAPON_STYLE[weaponType] ?? WEAPON_STYLE.sword_shield
  stats.maxHp = 90 + level * 14 + (style.hp ?? 0)
  stats.maxMp = 40 + level * 4 + (style.mp ?? 0)
  stats.atk = 8 + level * 2
  stats.def = 4 + level * 1.2 + (style.def ?? 0)
  stats.critRate = 0.05 + (style.crit ?? 0)
  stats.attackInterval = style.interval
  stats.range = style.range
  if (style.atkPct) stats.atk *= 1 + style.atkPct

  const mastery = getTrack(app, playerId, `weapon:${weaponType}`)
  stats.atk *= 1 + (mastery.level - 1) * 0.01

  const rows = many<ItemRow>(app.db, 'SELECT template_id, location, extra FROM player_items WHERE player_id = ? AND location = ?', playerId, 'equipped')
  let armed = false
  for (const row of rows) {
    const tpl = app.items.get(row.template_id)
    if (!tpl?.base || !tpl.slot) continue
    if (tpl.slot === 'weapon') armed = true
    const extra = parseExtra(row.extra)
    const mul = QUALITY_TABLE[extra.quality]?.mul ?? 1
    addBase(stats, tpl.base, mul)
    for (const affix of extra.affixes) applyAffix(stats, affix.stat, affix.value)
  }
  if (!armed) {
    stats.atk *= 0.55
    stats.range = 48
    stats.attackInterval = 1300
  }

  const pet = one<{ template_id: string; level: number }>(
    app.db,
    'SELECT template_id, level FROM player_pets WHERE player_id = ? AND active = 1',
    playerId,
  )
  if (pet) {
    const def = app.pets.get(pet.template_id)
    if (def) {
      const scale = 1 + (pet.level - 1) * 0.04
      stats.critRate += (def.aura.critRate ?? 0) * scale
      stats.atk *= 1 + (def.aura.atkPct ?? 0) * scale
      stats.def += (def.aura.def ?? 0) * scale
      stats.maxHp += (def.aura.hp ?? 0) * scale
      stats.healPct += (def.aura.healPct ?? 0) * scale
      for (const [el, value] of Object.entries(def.aura.elem ?? {})) {
        stats.elem[el as Element] += (value ?? 0) * scale
      }
    }
  }

  // 食物之类的常驻 buff 参与面板显示。战斗里还会再读一次，避免到期后面板撒谎。
  const t = Date.now()
  for (const buff of buffs) {
    if (buff.until < t) continue
    if (buff.stat === 'atkPct') stats.atk *= 1 + buff.value
    if (buff.stat === 'defPct') stats.def *= 1 + buff.value
    if (buff.stat === 'critRate') stats.critRate += buff.value
    if (buff.stat === 'speedPct') stats.speed *= 1 + buff.value
  }

  const player = getPlayer(app, playerId)
  const race = parseRace(player.race)
  applyAttributeBonuses(stats, totalAttributes(level, parseAttributes(player.attributes), race), race)

  stats.maxHp = Math.round(stats.maxHp)
  stats.maxMp = Math.round(stats.maxMp)
  stats.atk = Math.round(stats.atk * 10) / 10
  stats.def = Math.round(stats.def * 10) / 10
  stats.critRate = Math.round(stats.critRate * 1000) / 1000
  stats.speed = Math.round(stats.speed)
  return stats
}

/** 按装备品质倍率增加基础面板属性。 */
function addBase(stats: ComputedStats, base: Partial<StatBlock>, mul: number) {
  stats.maxHp += Math.round((base.hp ?? 0) * mul)
  stats.maxMp += Math.round((base.mp ?? 0) * mul)
  stats.atk += (base.atk ?? 0) * mul
  stats.def += (base.def ?? 0) * mul
  stats.critRate += (base.critRate ?? 0) * mul
  stats.critDmg += (base.critDmg ?? 0) * mul
  stats.speed += Math.round((base.speed ?? 0) * mul)
}

/** 把装备词条应用到普通属性或对应元素加成。 */
function applyAffix(stats: ComputedStats, stat: string, value: number) {
  if (stat === 'atk') stats.atk += value
  else if (stat === 'hp') stats.maxHp += value
  else if (stat === 'def') stats.def += value
  else if (stat === 'mp') stats.maxMp += value
  else if (stat === 'critRate') stats.critRate += value
  else if (stat === 'critDmg') stats.critDmg += value
  else if (stat.startsWith('elem_')) {
    const el = stat.slice(5) as Element
    if (stats.elem[el] !== undefined) stats.elem[el] += value
  }
}

/** 解析装备品质与随机词条，缺失或损坏数据回退到普通品质。 */
export function parseExtra(raw: string | null): ItemExtra {
  if (!raw) return { quality: 'white', affixes: [] }
  try {
    const parsed = JSON.parse(raw) as ItemExtra
    return { quality: parsed.quality ?? 'white', affixes: parsed.affixes ?? [] }
  } catch {
    return { quality: 'white', affixes: [] }
  }
}

/** 装备、升级、换灵伴之后调用。血量按比例保留，避免换装直接回满或倒地。 */
export function refreshStats(app: App, playerId: string) {
  const live = app.live.get(playerId)
  const row = getPlayer(app, playerId)
  const level = live?.level ?? row.level
  const weapon = (live?.weaponType ?? row.weapon_type) as WeaponType
  const buffs = live?.buffs ?? parseBuffs(row.buffs)
  const stats = computeStats(app, playerId, level, weapon, buffs)
  if (!live) return stats
  const ratio = live.stats.maxHp > 0 ? live.hp / live.stats.maxHp : 1
  live.stats = stats
  if (live.hp > stats.maxHp || live.hp === 999999) live.hp = stats.maxHp
  else if (ratio >= 0.99) live.hp = stats.maxHp
  else live.hp = Math.max(1, Math.min(stats.maxHp, live.hp))
  live.mp = Math.min(stats.maxMp, Math.max(0, live.mp))
  live.armed = hasWeapon(app, playerId)
  return stats
}

/** 检查已装备物品中是否存在武器部位，用于技能与徒手面板判定。 */
export function hasWeapon(app: App, playerId: string): boolean {
  const rows = many<ItemRow>(app.db, 'SELECT template_id, location, extra FROM player_items WHERE player_id = ? AND location = ?', playerId, 'equipped')
  return rows.some((row) => app.items.get(row.template_id)?.slot === 'weapon')
}

/** 从角色存档读取临时增益列表，无效存档返回空列表。 */
export function parseBuffs(raw: string | null): LiveBuff[] {
  try {
    return JSON.parse(raw || '[]') as LiveBuff[]
  } catch {
    return []
  }
}

/** 同步在线角色是否装备武器，供技能和徒手攻击规则使用。 */
export function syncArmed(app: App, live: LivePlayer) {
  live.armed = hasWeapon(app, live.id)
}
