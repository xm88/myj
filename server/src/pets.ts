/** 旧版 TypeScript 迁移参考：灵伴成长、主战、派遣和新闻。默认服务启动执行 server/*.go。 */

/**
 * 灵伴：契约、主战、派遣。
 * 派遣的结束时间存在数据库里。人下线、服务器重启，回来仍然可以领。
 * 奖励在领取时才结算，所以改数值不会追溯已经生成的旧结果——结果也是领取时才写下来。
 */

import type { App } from './app.ts'
import { many, one, run, transaction } from './db.ts'
import { emit } from './events.ts'
import { addItem } from './inventory.ts'
import { sendMail } from './mail.ts'
import { broadcast, notify, toast } from './net.ts'
import { addGold, addTrackExp, getPlayer, setFlag } from './player.ts'
import { refreshStats } from './stats.ts'
import { GameError, now, rollInt, uid } from './util.ts'

/** 数据库灵伴实例记录，包含拥有者、模板、成长与主战状态。 */
export interface PetRow {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 记录所属角色标识，业务查询同时校验归属。 */
  player_id: string
  /** 关联配置模板的标识，实例数值和模板规则分开保存。 */
  template_id: string
  /** 当前成长等级；具体上限由角色、熟练或灵伴规则决定。 */
  level: number
  /** 当前等级累计经验，升级后扣除所需经验。 */
  exp: number
  /** 灵伴是否设为主战，派遣中的灵伴不可同时出战。 */
  active: number
  /** 记录创建时间，使用 Unix 毫秒时间戳。 */
  created_at: number
}

/** 数据库派遣实例记录，保存起止时间、领取状态与结算结果。 */
export interface DispatchRow {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 记录所属角色标识，业务查询同时校验归属。 */
  player_id: string
  /** 本次派遣使用的角色灵伴实例标识。 */
  player_pet_id: string
  /** 本次派遣选择的路线模板标识。 */
  node_id: string
  /** 本次派遣选择的时长模板标识。 */
  duration_id: string
  /** 派遣出发时间，按此时间判断适用的探索增益。 */
  started_at: number
  /** 派遣预计完成时间，重启后仍按此毫秒时间判断。 */
  ends_at: number
  /** 当前业务状态，用于校验操作条件和防止重复结算。 */
  status: string
  /** 领取时生成并保存的派遣结算结果 JSON。 */
  result: string | null
  /** 记录创建时间，使用 Unix 毫秒时间戳。 */
  created_at: number
}

/** 读取角色拥有的全部灵伴实例。 */
export function listPets(app: App, playerId: string): PetRow[] {
  return many<PetRow>(app.db, 'SELECT * FROM player_pets WHERE player_id = ?', playerId)
}

/** 读取仍在进行或等待领取的派遣存档。 */
export function runningDispatches(app: App, playerId: string): DispatchRow[] {
  return many<DispatchRow>(app.db, `SELECT * FROM pet_dispatch WHERE player_id = ? AND status = 'running'`, playerId)
}

/** 查询灵伴是否占用未领取派遣，避免同时出战或再次派遣。 */
export function petBusy(app: App, petId: string): boolean {
  return Boolean(one(app.db, `SELECT id FROM pet_dispatch WHERE player_pet_id = ? AND status = 'running'`, petId))
}

/** 发放灵伴并处理重复契约；没有主战灵伴时自动出战，稀有契约产生新闻。 */
export function givePet(app: App, playerId: string, templateId: string, fromCapture: boolean) {
  const def = app.pets.get(templateId)
  if (!def) throw new GameError('没有这种灵伴')
  const owned = one<PetRow>(app.db, 'SELECT * FROM player_pets WHERE player_id = ? AND template_id = ?', playerId, templateId)
  if (owned) {
    addGold(app, playerId, 30)
    sendMail(app, playerId, '已经契约过了', `${def.name} 把多余的契约化成了 30 金币。`)
    if (fromCapture) emit(app, playerId, { type: 'capture', petId: templateId })
    return { duplicate: true, id: owned.id }
  }
  const id = uid()
  const hasActive = one(app.db, 'SELECT id FROM player_pets WHERE player_id = ? AND active = 1', playerId)
  run(
    app.db,
    'INSERT INTO player_pets(id, player_id, template_id, level, exp, active, created_at) VALUES(?, ?, ?, 1, 0, ?, ?)',
    id,
    playerId,
    templateId,
    hasActive ? 0 : 1,
    now(),
  )
  if (!hasActive) refreshStats(app, playerId)
  if (fromCapture) emit(app, playerId, { type: 'capture', petId: templateId })
  if (def.rarity === 'epic') {
    const player = getPlayer(app, playerId)
    const text = `旅人「${player.name}」与${def.name}建立了契约。`
    pushNews(app, text)
  }
  toast(app, playerId, fromCapture ? `契约成功：${def.name}` : `${def.name} 加入了旅团`)
  notify(app, playerId)
  return { duplicate: false, id }
}

/** 保存最多二十条新闻，并向在线客户端广播新消息。 */
export function pushNews(app: App, text: string) {
  const row = one<{ value: string }>(app.db, 'SELECT value FROM world_state WHERE key = ?', 'news')
  const list = row ? (JSON.parse(row.value) as { t: number; text: string }[]) : []
  list.unshift({ t: now(), text })
  const value = JSON.stringify(list.slice(0, 20))
  if (row) run(app.db, 'UPDATE world_state SET value = ? WHERE key = ?', value, 'news')
  else run(app.db, 'INSERT INTO world_state(key, value) VALUES(?, ?)', 'news', value)
  broadcast(app, { type: 'news', text })
}

/** 读取最近的世界新闻，缺失或损坏的数据按空列表处理。 */
export function readNews(app: App): { t: number; text: string }[] {
  const row = one<{ value: string }>(app.db, 'SELECT value FROM world_state WHERE key = ?', 'news')
  if (!row) return []
  try {
    return JSON.parse(row.value) as { t: number; text: string }[]
  } catch {
    return []
  }
}

/** 校验灵伴归属和忙碌状态，再切换主战灵伴与面板光环。 */
export function setActivePet(app: App, playerId: string, petId: string | null) {
  if (petId) {
    const pet = one<PetRow>(app.db, 'SELECT * FROM player_pets WHERE id = ? AND player_id = ?', petId, playerId)
    if (!pet) throw new GameError('没有这只灵伴')
    if (petBusy(app, pet.id)) throw new GameError('它还在派遣途中')
  }
  run(app.db, 'UPDATE player_pets SET active = 0 WHERE player_id = ?', playerId)
  if (petId) run(app.db, 'UPDATE player_pets SET active = 1 WHERE id = ?', petId)
  refreshStats(app, playerId)
  notify(app, playerId)
}

/** 增加灵伴经验，并逐级升级到二十级上限。 */
export function addPetExp(app: App, petId: string, amount: number) {
  const pet = one<PetRow>(app.db, 'SELECT * FROM player_pets WHERE id = ?', petId)
  if (!pet || amount <= 0 || pet.level >= 20) return
  let level = pet.level
  let exp = pet.exp + amount
  while (level < 20 && exp >= 24 + level * 18) {
    exp -= 24 + level * 18
    level += 1
  }
  run(app.db, 'UPDATE player_pets SET level = ?, exp = ? WHERE id = ?', level, exp, pet.id)
}

/** 校验灵伴归属、忙碌状态和路线解锁，取消出战后持久化结束时间。 */
export function startDispatch(app: App, playerId: string, petId: string, nodeId: string, durationId: string) {
  const player = getPlayer(app, playerId)
  const pet = one<PetRow>(app.db, 'SELECT * FROM player_pets WHERE id = ? AND player_id = ?', petId, playerId)
  if (!pet) throw new GameError('没有这只灵伴')
  if (petBusy(app, pet.id)) throw new GameError('它已经出门了')
  const running = runningDispatches(app, playerId)
  if (running.length >= 3) throw new GameError('同时最多派遣 3 只')
  const node = app.nodes.get(nodeId)
  const duration = app.data.durations.find((item) => item.id === durationId)
  if (!node || !duration) throw new GameError('没有这条派遣')
  if (player.level < node.unlockLevel) throw new GameError(`需要 ${node.unlockLevel} 级`)
  if (pet.active) {
    run(app.db, 'UPDATE player_pets SET active = 0 WHERE id = ?', pet.id)
    refreshStats(app, playerId)
  }
  const start = now()
  run(
    app.db,
    `INSERT INTO pet_dispatch(id, player_id, player_pet_id, node_id, duration_id, started_at, ends_at, status, result, created_at)
     VALUES(?, ?, ?, ?, ?, ?, ?, 'running', NULL, ?)`,
    uid(),
    playerId,
    pet.id,
    node.id,
    duration.id,
    start,
    start + duration.ms,
    start,
  )
  emit(app, playerId, { type: 'dispatch_start' })
  toast(app, playerId, `${app.pets.get(pet.template_id)?.name ?? '灵伴'}出发了：${node.name}`)
  notify(app, playerId)
}

/** 确认派遣已结束且未领取，按专长、天气和等级计算奖励并只发放一次。 */
export function claimDispatch(app: App, playerId: string, dispatchId: string) {
  const row = one<DispatchRow>(app.db, 'SELECT * FROM pet_dispatch WHERE id = ? AND player_id = ?', dispatchId, playerId)
  if (!row || row.status !== 'running') throw new GameError('没有这次派遣')
  if (now() < row.ends_at) throw new GameError('还没回来')
  const pet = one<PetRow>(app.db, 'SELECT * FROM player_pets WHERE id = ?', row.player_pet_id)
  const node = app.nodes.get(row.node_id)
  const duration = app.data.durations.find((item) => item.id === row.duration_id)
  if (!pet || !node || !duration) throw new GameError('派遣数据坏了')
  const def = app.pets.get(pet.template_id)!
  let efficiency = 1 + (def.dispatch[node.kind] ?? 0) + (def.dispatch.versatile ?? 0)
  efficiency += (def.dispatch.explore ?? 0) * 0.5
  const weatherBonus = def.weather?.[app.weather.id] ?? 0
  efficiency += weatherBonus
  const live = app.live.get(playerId)
  if (live?.buffs.some((buff) => buff.id === 'food_explore' && buff.until > row.started_at)) efficiency += 0.15
  efficiency *= 1 + (pet.level - 1) * 0.03
  const random = 0.85 + Math.random() * 0.3
  const items: { itemId: string; count: number }[] = []
  for (const base of node.basePerHour) {
    const count = Math.max(1, Math.round(base.count * duration.mul * efficiency * random))
    items.push({ itemId: base.itemId, count })
  }
  const explore = def.dispatch.explore ?? 0
  const event = Math.random() < duration.eventChance + explore * 0.2
  const result = { items, event, efficiency: Math.round(efficiency * 100) / 100 }

  transaction(app.db, () => {
    const changed = run(app.db, `UPDATE pet_dispatch SET status = 'claimed', result = ? WHERE id = ? AND status = 'running'`, JSON.stringify(result), row.id)
    if (changed !== 1) throw new GameError('已经领过了')
    for (const item of items) addItem(app, playerId, item.itemId, item.count)
    if (event) {
      setFlag(app, playerId, 'hidden_cave')
      addItem(app, playerId, 'map_fragment', 1)
      sendMail(app, playerId, '灵伴有发现', `${def.name} 在${node.name}里找到了不在航标上的入口。去告诉艾兰。`)
    }
  })
  addPetExp(app, pet.id, 16)
  addTrackExp(app, playerId, 'life:gather', 10, 'life')
  emit(app, playerId, { type: 'dispatch_claim' })
  toast(app, playerId, event ? '派遣回来了，而且还发现了新线索' : '派遣回来了')
  notify(app, playerId)
  return result
}

/** 仅在开发模式下允许立即完成指定角色的派遣。 */
export function finishDispatchNow(app: App, playerId: string, dispatchId: string) {
  if (!app.devTools) throw new GameError('测试开关没开', 403)
  const changed = run(
    app.db,
    `UPDATE pet_dispatch SET ends_at = ? WHERE id = ? AND player_id = ? AND status = 'running'`,
    now() - 1,
    dispatchId,
    playerId,
  )
  if (changed !== 1) throw new GameError('没有进行中的派遣')
  notify(app, playerId)
}

/** 击杀掉落用的金币浮动。放在这里是因为世界新闻也归灵伴模块管。 */
export function goldRange(min: number, max: number): number {
  return rollInt(min, max)
}
