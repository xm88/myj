/** 旧版 TypeScript 迁移参考：账号、角色、加点与存档。默认服务启动执行 server/*.go。 */

/**
 * 账号与角色。
 * 客户端只提交“我是谁、我选了哪把武器”，等级、金币和初始物品都由这里发放。
 */

import bcrypt from 'bcryptjs'
import jwt from 'jsonwebtoken'
import type { App } from './app.ts'
import { one, run } from './db.ts'
import {
  attributeBudget,
  ATTRIBUTE_KEYS,
  emptyAttributes,
  expToNext,
  lifeExpToNext,
  masteryExpToNext,
  parseAttributes,
  parseRace,
  POINTS_PER_LEVEL,
  spentAttributes,
  WEAPON_LABEL,
  type AttributeKey,
  type Attributes,
} from './formula.ts'
import { notify, toast } from './net.ts'
import type { LivePlayer } from './runtime.ts'
import type { WeaponType } from './types.ts'
import { GameError, now, uid } from './util.ts'
import { emit } from './events.ts'
import { addItem, makeEquipmentExtra } from './inventory.ts'
import { acceptQuest } from './quest.ts'
import { refreshStats } from './stats.ts'

/** 数据库角色存档，保存成长、位置、资源、技能和解锁状态。 */
export interface PlayerRow {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 所属账号在 users 表中的标识。 */
  user_id: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 当前成长等级；具体上限由角色、熟练或灵伴规则决定。 */
  level: number
  /** 当前等级累计经验，升级后扣除所需经验。 */
  exp: number
  /** 角色断线后恢复使用的地图标识。 */
  map_id: string
  /** 世界横坐标，单位与地图尺寸保持一致。 */
  x: number
  /** 世界纵坐标，单位与地图尺寸保持一致。 */
  y: number
  /** 当前生命值或模板生命属性。 */
  hp: number
  /** 当前法力值或模板法力属性。 */
  mp: number
  /** 金币数量，所有变更由服务器校验余额并持久化。 */
  gold: number
  /** 角色当前武器类型，与默认技能栏关联。 */
  weapon_type: string
  /** 四个已选择技能 ID 的 JSON 存档。 */
  skill_loadout: string
  /** 五项潜力点分配的 JSON 存档。 */
  attributes: string
  /** 角色种族标识，决定自动成长与面板换算。 */
  race: string
  /** 已解锁内容或隐藏线索的持久化标记。 */
  flags: string
  /** 临时增益列表，在线计算时只应用尚未过期的增益。 */
  buffs: string
  /** 记录创建时间，使用 Unix 毫秒时间戳。 */
  created_at: number
  /** 记录最近写入时间，使用 Unix 毫秒时间戳。 */
  updated_at: number
}

/** 数据库账号记录，保存邮箱、密码摘要与游客标记。 */
export interface UserRow {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 规范化后的账号邮箱，数据库要求唯一。 */
  email: string
  /** bcrypt 密码摘要，数据库不保存明文密码。 */
  password_hash: string
  /** 是否为游客账号，以整数开关持久化。 */
  is_guest: number
  /** 记录创建时间，使用 Unix 毫秒时间戳。 */
  created_at: number
}

/** 创建角色时允许选择的四种武器类型。 */
const WEAPONS: WeaponType[] = ['sword_shield', 'greatsword', 'staff', 'bow']

/** 签发十四天有效的角色账号令牌，供 REST 与 WebSocket 共用。 */
export function signToken(app: App, userId: string): string {
  return jwt.sign({ uid: userId }, app.jwtSecret, { expiresIn: '14d' })
}

/** 校验 Bearer JWT，成功后返回账号 ID，过期或错误令牌返回鉴权失败。 */
export function readUserId(app: App, header: string | undefined): string {
  if (!header?.startsWith('Bearer ')) throw new GameError('需要登录', 401)
  try {
    const payload = jwt.verify(header.slice(7), app.jwtSecret) as { uid: string }
    return payload.uid
  } catch {
    throw new GameError('登录已过期', 401)
  }
}

/** 规范邮箱并校验密码，使用 bcrypt 保存密码摘要后返回登录令牌。 */
export function register(app: App, email: string, password: string): { token: string } {
  const normalized = email.trim().toLowerCase()
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(normalized)) throw new GameError('邮箱格式不对')
  if (password.length < 6 || password.length > 64) throw new GameError('密码需要 6 到 64 位')
  if (one(app.db, 'SELECT id FROM users WHERE email = ?', normalized)) throw new GameError('这个邮箱已经注册过了')
  const id = uid()
  run(
    app.db,
    'INSERT INTO users(id, email, password_hash, is_guest, created_at) VALUES(?, ?, ?, 0, ?)',
    id,
    normalized,
    bcrypt.hashSync(password, 10),
    now(),
  )
  return { token: signToken(app, id) }
}

/** 验证邮箱与 bcrypt 密码摘要，校验失败统一返回登录错误。 */
export function login(app: App, email: string, password: string): { token: string } {
  const user = one<UserRow>(app.db, 'SELECT * FROM users WHERE email = ?', email.trim().toLowerCase())
  if (!user || !bcrypt.compareSync(password, user.password_hash)) throw new GameError('邮箱或密码不对', 401)
  return { token: signToken(app, user.id) }
}

/** 创建游客账号及初始角色，返回登录令牌。 */
export function loginGuest(app: App): { token: string } {
  const id = uid()
  const email = `guest_${id.slice(0, 8)}@guest.local`
  run(
    app.db,
    'INSERT INTO users(id, email, password_hash, is_guest, created_at) VALUES(?, ?, ?, 1, ?)',
    id,
    email,
    bcrypt.hashSync(uid(), 4),
    now(),
  )
  const name = uniqueName(app, `旅人${Math.floor(1000 + Math.random() * 9000)}`)
  createCharacter(app, id, name, 'sword_shield')
  return { token: signToken(app, id) }
}

/** 读取账号唯一的角色记录，未创建时返回空值。 */
export function getPlayerByUser(app: App, userId: string): PlayerRow | undefined {
  return one<PlayerRow>(app.db, 'SELECT * FROM players WHERE user_id = ?', userId)
}

/** 按角色 ID 读取存档，角色缺失时返回业务错误。 */
export function getPlayer(app: App, playerId: string): PlayerRow {
  const row = one<PlayerRow>(app.db, 'SELECT * FROM players WHERE id = ?', playerId)
  if (!row) throw new GameError('角色不存在', 404)
  return row
}

/** 读取账号唯一的角色，未创建角色时返回明确的缺失错误。 */
export function requirePlayer(app: App, userId: string): PlayerRow {
  const row = getPlayerByUser(app, userId)
  if (!row) throw new GameError('还没有角色', 404)
  return row
}

/** 为游客寻找未占用的名称，多次重名时追加随机标识。 */
function uniqueName(app: App, base: string): string {
  let name = base
  for (let i = 0; i < 20; i++) {
    if (!one(app.db, 'SELECT id FROM players WHERE name = ?', name)) return name
    name = `${base}${i + 1}`
  }
  return `${base}${uid().slice(0, 4)}`
}

/** 校验角色名与武器种族，创建初始存档、熟练、装备、道具和主线任务。 */
export function createCharacter(app: App, userId: string, name: string, weaponType: string, raceId = 'human'): PlayerRow {
  if (getPlayerByUser(app, userId)) throw new GameError('已经有角色了')
  if (!WEAPONS.includes(weaponType as WeaponType)) throw new GameError('这把武器还没开放')
  if (!/^[\u4e00-\u9fa5A-Za-z0-9_]{2,12}$/.test(name)) throw new GameError('名字需要 2 到 12 个字，可用中文、字母和数字')
  if (one(app.db, 'SELECT id FROM players WHERE name = ?', name)) throw new GameError('已经有人用这个名字了')

  const weapon = weaponType as WeaponType
  const race = parseRace(raceId)
  const town = app.maps.get('town_starfall')!
  const id = uid()
  const t = now()
  const loadout = defaultLoadout(app, weapon)
  run(
    app.db,
    `INSERT INTO players(id, user_id, name, level, exp, map_id, x, y, hp, mp, gold, weapon_type, skill_loadout, attributes, race, flags, buffs, created_at, updated_at)
     VALUES(?, ?, ?, 1, 0, ?, ?, ?, 120, 40, ?, ?, ?, '{}', ?, '{}', '[]', ?, ?)`,
    id,
    userId,
    name,
    town.id,
    town.spawn.x,
    town.spawn.y,
    app.data.constants.starterGold,
    weapon,
    JSON.stringify(loadout),
    race,
    t,
    t,
  )
  for (const track of [`weapon:${weapon}`, 'life:gather', 'life:craft', 'life:cook']) {
    run(app.db, 'INSERT INTO player_tracks(player_id, track, level, exp) VALUES(?, ?, 1, 0)', id, track)
  }
  const weaponItem = app.items.get(starterWeaponId(weapon))!
  addItem(app, id, weaponItem.id, 1, makeEquipmentExtra(app, 'white'), 'equipped')
  addItem(app, id, 'capture_orb', app.data.constants.starterOrbs)
  addItem(app, id, 'bread', 2)
  acceptQuest(app, id, 'q_main_01', true)
  return getPlayer(app, id)
}

/** 将初始武器类型映射到对应的新手装备模板。 */
export function starterWeaponId(weapon: WeaponType): string {
  const map: Record<WeaponType, string> = {
    sword_shield: 'wpn_sword_1',
    greatsword: 'wpn_great_1',
    staff: 'wpn_staff_1',
    bow: 'wpn_bow_1',
  }
  return map[weapon]
}

/** 选择当前武器前四个技能，作为创建角色或换武器后的默认技能栏。 */
export function defaultLoadout(app: App, weapon: WeaponType): string[] {
  return app.data.skills.filter((s) => s.weapon === weapon).slice(0, 4).map((s) => s.id)
}

/** 将武器类型转换为中文名称，未知类型保留原标识。 */
export function weaponLabel(weapon: string): string {
  return WEAPON_LABEL[weapon] ?? weapon
}

/** 优先读取在线金币，余额不足时拒绝扣款；成功后同步数据库与在线状态。 */
export function addGold(app: App, playerId: string, delta: number): number {
  const row = getPlayer(app, playerId)
  const live = app.live.get(playerId)
  const current = live?.gold ?? row.gold
  const next = current + delta
  if (next < 0) throw new GameError('金币不够')
  run(app.db, 'UPDATE players SET gold = ?, updated_at = ? WHERE id = ?', next, now(), playerId)
  if (live) live.gold = next
  return next
}

/** 累计经验并逐级升级到等级上限，升级后刷新面板、恢复状态并推进等级任务。 */
export function addExp(app: App, playerId: string, amount: number) {
  if (amount <= 0) return
  const row = getPlayer(app, playerId)
  const live = app.live.get(playerId)
  let level = live?.level ?? row.level
  let exp = (live?.exp ?? row.exp) + amount
  let ups = 0
  while (level < 30 && exp >= expToNext(level)) {
    exp -= expToNext(level)
    level += 1
    ups += 1
  }
  run(app.db, 'UPDATE players SET level = ?, exp = ?, updated_at = ? WHERE id = ?', level, exp, now(), playerId)
  if (live) {
    live.level = level
    live.exp = exp
  }
  if (ups > 0) {
    refreshStats(app, playerId)
    if (live) {
      live.hp = live.stats.maxHp
      live.mp = live.stats.maxMp
    }
    emit(app, playerId, { type: 'level', level })
    toast(app, playerId, `升级了，现在是 ${level} 级，获得 ${ups * POINTS_PER_LEVEL} 点属性`)
    notify(app, playerId)
  }
}

/** 校验新增潜力点为非负整数且不超过等级预算，再保存与更新面板。 */
export function allocateAttributes(app: App, playerId: string, add: Partial<Record<AttributeKey, unknown>>) {
  const row = getPlayer(app, playerId)
  const live = app.live.get(playerId)
  const level = live?.level ?? row.level
  const current = parseAttributes(row.attributes)
  const next: Attributes = { ...current }
  let added = 0
  for (const key of ATTRIBUTE_KEYS) {
    const n = Math.floor(Number(add[key] ?? 0))
    if (!Number.isFinite(n) || n < 0) throw new GameError('点数不对')
    next[key] += n
    added += n
  }
  if (added <= 0) throw new GameError('没有要加的点')
  if (spentAttributes(next) > attributeBudget(level)) throw new GameError('点数不够')
  run(app.db, 'UPDATE players SET attributes = ?, updated_at = ? WHERE id = ?', JSON.stringify(next), now(), playerId)
  refreshStats(app, playerId)
  notify(app, playerId)
}

/** 将已分配潜力清零，等级预算保留，客户端可重新分配。 */
export function resetAttributes(app: App, playerId: string) {
  const row = getPlayer(app, playerId)
  if (spentAttributes(parseAttributes(row.attributes)) <= 0) throw new GameError('还没有加过点')
  run(app.db, 'UPDATE players SET attributes = ?, updated_at = ? WHERE id = ?', JSON.stringify(emptyAttributes()), now(), playerId)
  refreshStats(app, playerId)
  notify(app, playerId)
}

/** 读取熟练等级与经验，尚未保存的熟练按一级零经验处理。 */
export function getTrack(app: App, playerId: string, track: string): { level: number; exp: number } {
  const row = one<{ level: number; exp: number }>(
    app.db,
    'SELECT level, exp FROM player_tracks WHERE player_id = ? AND track = ?',
    playerId,
    track,
  )
  return row ?? { level: 1, exp: 0 }
}

/** 累计武器或生活熟练经验，按各自等级上限升级并写回存档。 */
export function addTrackExp(app: App, playerId: string, track: string, amount: number, curve: 'weapon' | 'life') {
  const current = getTrack(app, playerId, track)
  let level = current.level
  let exp = current.exp + amount
  const cap = curve === 'weapon' ? 20 : 10
  const next = curve === 'weapon' ? masteryExpToNext : lifeExpToNext
  while (level < cap && exp >= next(level)) {
    exp -= next(level)
    level += 1
  }
  run(
    app.db,
    `INSERT INTO player_tracks(player_id, track, level, exp) VALUES(?, ?, ?, ?)
     ON CONFLICT(player_id, track) DO UPDATE SET level = excluded.level, exp = excluded.exp`,
    playerId,
    track,
    level,
    exp,
  )
}

/** 解析角色解锁标记，损坏存档回退为空对象。 */
export function readFlags(row: PlayerRow): Record<string, boolean> {
  try {
    return JSON.parse(row.flags) as Record<string, boolean>
  } catch {
    return {}
  }
}

/** 持久化角色解锁标记，用于隐藏任务和探索内容的前置条件。 */
export function setFlag(app: App, playerId: string, key: string) {
  const row = getPlayer(app, playerId)
  const flags = readFlags(row)
  if (flags[key]) return
  flags[key] = true
  run(app.db, 'UPDATE players SET flags = ?, updated_at = ? WHERE id = ?', JSON.stringify(flags), now(), playerId)
}

/** 定期或断线时保存在线角色的坐标、战斗状态和成长信息。 */
export function saveLive(app: App, live: LivePlayer) {
  run(
    app.db,
    `UPDATE players SET level = ?, exp = ?, map_id = ?, x = ?, y = ?, hp = ?, mp = ?, gold = ?, weapon_type = ?, skill_loadout = ?, buffs = ?, updated_at = ?
     WHERE id = ?`,
    live.level,
    live.exp,
    live.mapId,
    live.x,
    live.y,
    live.hp,
    live.mp,
    live.gold,
    live.weaponType,
    JSON.stringify(live.loadout),
    JSON.stringify(live.buffs),
    now(),
    live.id,
  )
}

/** 输出可选择的初始武器及中文玩法说明。 */
export function listWeapons() {
  return WEAPONS.map((id) => ({
    id,
    name: WEAPON_LABEL[id],
    blurb:
      id === 'sword_shield'
        ? '扛得住，适合和灵伴慢慢磨。'
        : id === 'greatsword'
          ? '慢，但每一刀都很重。'
          : id === 'staff'
            ? '元素和治疗都在这根杖上。'
            : '站得远，暴击更高。',
  }))
}
