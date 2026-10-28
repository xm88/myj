/** 旧版 TypeScript 迁移参考：制造、交易、公会和社交业务。默认服务启动执行 server/*.go。 */

/**
 * 城镇系统：制造、商店、拍卖、公会、邮件、好友、队伍、聊天。
 * 拍卖成交放在一个事务里：扣钱、给货、给卖家税后金币，要么一起成功。
 */

import type { App } from './app.ts'
import { many, one, run, transaction } from './db.ts'
import { emit } from './events.ts'
import { addItem, consumeTemplate, makeEquipmentExtra, takeItem } from './inventory.ts'
import { sendMail, type MailAttachment } from './mail.ts'
import { notify, toast } from './net.ts'
import { addGold, addTrackExp, getPlayer, type PlayerRow } from './player.ts'
import { GameError, now, uid } from './util.ts'
import type { ItemExtra, Quality } from './types.ts'

/** 按配方扣金币与材料并发放产物，随后推进生活熟练和制造任务。 */
export function craft(app: App, playerId: string, recipeId: string) {
  const recipe = app.recipes.get(recipeId)
  if (!recipe) throw new GameError('没有这个配方')
  const track = getTrackLevel(app, playerId, `life:${recipe.life}`)
  if (track < recipe.skill) throw new GameError(`需要${recipe.life === 'cook' ? '料理' : '制造'} ${recipe.skill} 级`)
  const tpl = app.items.get(recipe.output.itemId)
  if (!tpl) throw new GameError('产物不存在')
  // 扣钱、扣材料和发产物放在同一事务里，材料不够时金币不会少。
  transaction(app.db, () => {
    addGold(app, playerId, -recipe.gold)
    for (const input of recipe.inputs) consumeTemplate(app, playerId, input.itemId, input.count)
    if (tpl.kind === 'equipment') {
      addItem(app, playerId, tpl.id, recipe.output.count, makeEquipmentExtra(app, (recipe.output.quality ?? 'green') as Quality))
    } else {
      addItem(app, playerId, tpl.id, recipe.output.count)
    }
  })
  addTrackExp(app, playerId, `life:${recipe.life}`, 12 * recipe.skill, 'life')
  emit(app, playerId, { type: 'craft', recipeId })
  toast(app, playerId, `做完了：${recipe.name}`)
  notify(app, playerId)
}

/** 读取生活技能等级，缺失熟练记录时按一级处理。 */
function getTrackLevel(app: App, playerId: string, track: string): number {
  const row = one<{ level: number }>(app.db, 'SELECT level FROM player_tracks WHERE player_id = ? AND track = ?', playerId, track)
  return row?.level ?? 1
}

/** 校验商店货物、购买数量和金币，再发放对应模板物品。 */
export function buyFromShop(app: App, playerId: string, npcId: string, itemId: string, count: number) {
  const npc = app.npcs.get(npcId)
  const good = npc?.shop?.find((entry) => entry.itemId === itemId)
  if (!good) throw new GameError('这里不卖这个')
  if (count < 1 || count > 99) throw new GameError('数量不对')
  const cost = good.price * count
  addGold(app, playerId, -cost)
  addItem(app, playerId, itemId, count)
  emit(app, playerId, { type: 'buy_shop' })
  notify(app, playerId)
}

/** 扣除可售背包物品，并按配置价格结算金币。 */
export function sellToShop(app: App, playerId: string, itemId: string, count: number) {
  const row = takeItem(app, playerId, itemId)
  if (row.location !== 'bag') throw new GameError('只能卖背包里的东西')
  const tpl = app.items.get(row.template_id)
  if (!tpl || tpl.bound || tpl.sell <= 0) throw new GameError('这件不能卖给商人')
  const amount = tpl.kind === 'equipment' ? 1 : Math.min(count, row.count)
  if (amount <= 0) throw new GameError('数量不对')
  if (amount === row.count) run(app.db, 'DELETE FROM player_items WHERE id = ?', row.id)
  else run(app.db, 'UPDATE player_items SET count = ? WHERE id = ?', row.count - amount, row.id)
  addGold(app, playerId, tpl.sell * amount)
  notify(app, playerId)
}

/** 数据库拍卖订单记录，包含所有权、价格、状态与物品快照。 */
interface OrderRow {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 出售此订单的角色 ID，税后收入写入该角色。 */
  seller_id: string
  /** 买下订单的角色 ID，未成交时为空。 */
  buyer_id: string | null
  /** 上架时的物品模板、数量与词条 JSON，成交按快照发货。 */
  snapshot: string
  /** 当前订单整单的金币价格。 */
  price: number
  /** 当前业务状态，用于校验操作条件和防止重复结算。 */
  status: string
  /** 记录创建时间，使用 Unix 毫秒时间戳。 */
  created_at: number
  /** 记录最近写入时间，使用 Unix 毫秒时间戳。 */
  updated_at: number
}

/** 上架时冻结的物品模板、数量与词条，成交或撤回都按此快照发货。 */
export interface OrderSnapshot {
  /** 实例关联的配置模板标识。 */
  templateId: string
  /** 堆叠数量或任务目标所需次数。 */
  count: number
  /** 装备实例的品质与词条 JSON，普通堆叠材料为空。 */
  extra: ItemExtra | null
}

/** 查询开放拍卖订单，将快照与物品模板合并为客户端列表。 */
export function listMarket(app: App) {
  const rows = many<OrderRow>(app.db, `SELECT * FROM market_orders WHERE status = 'open' ORDER BY created_at DESC LIMIT 80`)
  return rows.map((row) => {
    const seller = one<PlayerRow>(app.db, 'SELECT * FROM players WHERE id = ?', row.seller_id)
    const snap = JSON.parse(row.snapshot) as OrderSnapshot
    const tpl = app.items.get(snap.templateId)
    return {
      id: row.id,
      sellerId: row.seller_id,
      sellerName: seller?.name ?? '旅人',
      price: row.price,
      templateId: snap.templateId,
      name: tpl?.name ?? snap.templateId,
      count: snap.count,
      extra: snap.extra,
      kind: tpl?.kind ?? 'material',
      createdAt: row.created_at,
    }
  })
}

/** 将可交易的背包物品转为订单快照，上架数量从原背包扣除。 */
export function listOrder(app: App, playerId: string, itemId: string, count: number, price: number) {
  if (!Number.isInteger(price) || price < 1 || price > 1_000_000) throw new GameError('标价不合法')
  const row = takeItem(app, playerId, itemId)
  if (row.location !== 'bag') throw new GameError('只能上架背包里的东西')
  const tpl = app.items.get(row.template_id)
  if (!tpl || tpl.bound || tpl.kind === 'quest') throw new GameError('这件不能交易')
  const amount = tpl.kind === 'equipment' || row.extra ? 1 : Math.min(count, row.count)
  if (amount < 1) throw new GameError('数量不对')
  const snapshot: OrderSnapshot = {
    templateId: row.template_id,
    count: amount,
    extra: row.extra ? (JSON.parse(row.extra) as ItemExtra) : null,
  }
  if (amount === row.count) run(app.db, 'DELETE FROM player_items WHERE id = ?', row.id)
  else run(app.db, 'UPDATE player_items SET count = ? WHERE id = ?', row.count - amount, row.id)
  const id = uid()
  const t = now()
  run(
    app.db,
    `INSERT INTO market_orders(id, seller_id, buyer_id, snapshot, price, status, created_at, updated_at)
     VALUES(?, ?, NULL, ?, ?, 'open', ?, ?)`,
    id,
    playerId,
    JSON.stringify(snapshot),
    price,
    t,
    t,
  )
  emit(app, playerId, { type: 'market_list' })
  notify(app, playerId)
  return id
}

/** 独占变更开放订单，扣买家金币、发货并结算卖家税后收入。 */
export function buyOrder(app: App, playerId: string, orderId: string) {
  transaction(app.db, () => {
    const order = one<OrderRow>(app.db, 'SELECT * FROM market_orders WHERE id = ?', orderId)
    if (!order || order.status !== 'open') throw new GameError('已经被买走了')
    if (order.seller_id === playerId) throw new GameError('不能买自己的东西')
    const changed = run(
      app.db,
      `UPDATE market_orders SET status = 'sold', buyer_id = ?, updated_at = ? WHERE id = ? AND status = 'open'`,
      playerId,
      now(),
      order.id,
    )
    if (changed !== 1) throw new GameError('已经被买走了')
    addGold(app, playerId, -order.price)
    const tax = Math.floor(order.price * app.data.constants.marketTax)
    addGold(app, order.seller_id, order.price - tax)
    const snap = JSON.parse(order.snapshot) as OrderSnapshot
    addItem(app, playerId, snap.templateId, snap.count, snap.extra)
    const buyer = getPlayer(app, playerId)
    const tpl = app.items.get(snap.templateId)
    sendMail(
      app,
      order.seller_id,
      '拍卖成交',
      `${buyer.name} 买下了 ${tpl?.name ?? '物品'} x${snap.count}。你得到 ${order.price - tax} 金币（税 ${tax}）。`,
    )
  })
  emit(app, playerId, { type: 'market_buy' })
  notify(app, playerId)
}

/** 校验订单归属并关闭订单，将快照中的物品归还卖家。 */
export function cancelOrder(app: App, playerId: string, orderId: string) {
  transaction(app.db, () => {
    const order = one<OrderRow>(app.db, 'SELECT * FROM market_orders WHERE id = ?', orderId)
    if (!order || order.status !== 'open') throw new GameError('订单不在')
    if (order.seller_id !== playerId) throw new GameError('不是你的订单')
    const changed = run(app.db, `UPDATE market_orders SET status = 'cancelled', updated_at = ? WHERE id = ? AND status = 'open'`, now(), order.id)
    if (changed !== 1) throw new GameError('订单不在')
    const snap = JSON.parse(order.snapshot) as OrderSnapshot
    addItem(app, playerId, snap.templateId, snap.count, snap.extra)
  })
  notify(app, playerId)
}

/** 校验名称与成员关系，收取创建费用并建立会长记录。 */
export function createGuild(app: App, playerId: string, name: string) {
  if (!/^[\u4e00-\u9fa5A-Za-z0-9_]{2,8}$/.test(name)) throw new GameError('公会名需要 2 到 8 个字')
  if (one(app.db, 'SELECT guild_id FROM guild_members WHERE player_id = ?', playerId)) throw new GameError('你已经在公会里')
  if (one(app.db, 'SELECT id FROM guilds WHERE name = ?', name)) throw new GameError('已经有这个公会了')
  addGold(app, playerId, -app.data.constants.guildCost)
  const id = uid()
  const t = now()
  run(app.db, 'INSERT INTO guilds(id, name, leader_id, level, exp, notice, created_at) VALUES(?, ?, ?, 1, 0, ?, ?)', id, name, playerId, '欢迎加入。', t)
  run(app.db, 'INSERT INTO guild_members(guild_id, player_id, role, joined_at) VALUES(?, ?, ?, ?)', id, playerId, 'leader', t)
  emit(app, playerId, { type: 'join_guild' })
  notify(app, playerId)
  return id
}

/** 检查公会存在、人数上限和已有成员关系，再写入成员记录。 */
export function joinGuild(app: App, playerId: string, guildId: string) {
  if (one(app.db, 'SELECT guild_id FROM guild_members WHERE player_id = ?', playerId)) throw new GameError('你已经在公会里')
  const guild = one(app.db, 'SELECT id FROM guilds WHERE id = ?', guildId)
  if (!guild) throw new GameError('没有这个公会')
  const count = one<{ n: number }>(app.db, 'SELECT COUNT(*) AS n FROM guild_members WHERE guild_id = ?', guildId)
  if (Number(count?.n ?? 0) >= 30) throw new GameError('公会满了')
  run(app.db, 'INSERT INTO guild_members(guild_id, player_id, role, joined_at) VALUES(?, ?, ?, ?)', guildId, playerId, 'member', now())
  emit(app, playerId, { type: 'join_guild' })
  notify(app, playerId)
}

/** 移除成员；有其他成员时会长不能离开，空公会会被清理。 */
export function leaveGuild(app: App, playerId: string) {
  const member = one<{ guild_id: string; role: string }>(app.db, 'SELECT guild_id, role FROM guild_members WHERE player_id = ?', playerId)
  if (!member) throw new GameError('你不在公会里')
  const others = one<{ n: number }>(app.db, 'SELECT COUNT(*) AS n FROM guild_members WHERE guild_id = ? AND player_id <> ?', member.guild_id, playerId)
  if (member.role === 'leader' && Number(others?.n ?? 0) > 0) throw new GameError('还有成员时会长不能离开')
  run(app.db, 'DELETE FROM guild_members WHERE player_id = ?', playerId)
  if (Number(others?.n ?? 0) === 0) run(app.db, 'DELETE FROM guilds WHERE id = ?', member.guild_id)
  notify(app, playerId)
}

/** 扣除捐赠金币并增加公会经验，按累计经验更新公会等级。 */
export function donateGuild(app: App, playerId: string, gold: number) {
  if (!Number.isInteger(gold) || gold < 50) throw new GameError('至少捐 50 金币')
  const member = one<{ guild_id: string }>(app.db, 'SELECT guild_id FROM guild_members WHERE player_id = ?', playerId)
  if (!member) throw new GameError('你不在公会里')
  addGold(app, playerId, -gold)
  const guild = one<{ exp: number; level: number }>(app.db, 'SELECT exp, level FROM guilds WHERE id = ?', member.guild_id)!
  const exp = guild.exp + gold
  const level = Math.min(10, 1 + Math.floor(exp / 500))
  run(app.db, 'UPDATE guilds SET exp = ?, level = ? WHERE id = ?', exp, level, member.guild_id)
  emit(app, playerId, { type: 'donate' })
  notify(app, playerId)
}

/** 读取公会列表，并补充成员人数和会长名称。 */
export function listGuilds(app: App) {
  const guilds = many<{ id: string; name: string; level: number; exp: number; notice: string; leader_id: string }>(
    app.db,
    'SELECT id, name, level, exp, notice, leader_id FROM guilds ORDER BY level DESC, exp DESC LIMIT 40',
  )
  return guilds.map((guild) => {
    const count = one<{ n: number }>(app.db, 'SELECT COUNT(*) AS n FROM guild_members WHERE guild_id = ?', guild.id)
    const leader = one<PlayerRow>(app.db, 'SELECT * FROM players WHERE id = ?', guild.leader_id)
    return { ...guild, members: Number(count?.n ?? 0), leaderName: leader?.name ?? '' }
  })
}

/** 数据库邮件记录，保存附件以及已读、已领取状态。 */
interface MailRow {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 记录所属角色标识，业务查询同时校验归属。 */
  player_id: string
  /** NPC 职能或对话窗口的显示标题。 */
  title: string
  /** HTTP 参数对象或邮件正文，含义由所属结构决定。 */
  body: string
  /** 可领取附件的 JSON 存档，无奖励邮件可为空。 */
  attachments: string | null
  /** 邮件已读状态，以整数开关存储。 */
  is_read: number
  /** 奖励已领取状态，领取后不得再次结算。 */
  claimed: number
  /** 记录创建时间，使用 Unix 毫秒时间戳。 */
  created_at: number
}

/** 将邮件存档转换为客户端列表，包含附件、已读和领取状态。 */
export function listMail(app: App, playerId: string) {
  return many<MailRow>(app.db, 'SELECT * FROM mail WHERE player_id = ? ORDER BY created_at DESC LIMIT 40', playerId).map((row) => ({
    id: row.id,
    title: row.title,
    body: row.body,
    attachments: row.attachments ? (JSON.parse(row.attachments) as MailAttachment) : null,
    read: Boolean(row.is_read),
    claimed: Boolean(row.claimed),
    createdAt: row.created_at,
  }))
}

/** 只允许领取所属邮件一次；标记领取后发放金币和物品附件。 */
export function claimMail(app: App, playerId: string, mailId: string) {
  transaction(app.db, () => {
    const row = one<MailRow>(app.db, 'SELECT * FROM mail WHERE id = ? AND player_id = ?', mailId, playerId)
    if (!row) throw new GameError('没有这封信')
    if (row.claimed) throw new GameError('已经领过了')
    const changed = run(app.db, 'UPDATE mail SET claimed = 1, is_read = 1 WHERE id = ? AND claimed = 0', mailId)
    if (changed !== 1) throw new GameError('已经领过了')
    if (row.attachments) {
      const att = JSON.parse(row.attachments) as MailAttachment
      if (att.gold) addGold(app, playerId, att.gold)
      for (const item of att.items ?? []) addItem(app, playerId, item.templateId, item.count, item.extra)
    }
  })
  notify(app, playerId)
}

/** 按角色名查找目标，排除自己与重复关系后发送好友申请。 */
export function requestFriend(app: App, playerId: string, name: string) {
  const target = one<PlayerRow>(app.db, 'SELECT * FROM players WHERE name = ?', name)
  if (!target) throw new GameError('找不到这个旅人')
  if (target.id === playerId) throw new GameError('不能加自己')
  const existing = one(
    app.db,
    'SELECT id, status FROM friends WHERE (from_id = ? AND to_id = ?) OR (from_id = ? AND to_id = ?)',
    playerId,
    target.id,
    target.id,
    playerId,
  ) as { id: string; status: string } | undefined
  if (existing?.status === 'accepted') throw new GameError('已经是好友')
  if (existing) throw new GameError('申请已经在了')
  run(app.db, 'INSERT INTO friends(id, from_id, to_id, status, created_at) VALUES(?, ?, ?, ?, ?)', uid(), playerId, target.id, 'pending', now())
  toast(app, target.id, `${getPlayer(app, playerId).name} 想加你为好友`)
  notify(app, target.id)
}

/** 只允许收件人接受待处理的好友申请。 */
export function acceptFriend(app: App, playerId: string, requestId: string) {
  const row = one<{ id: string; to_id: string; status: string }>(app.db, 'SELECT * FROM friends WHERE id = ?', requestId)
  if (!row || row.to_id !== playerId || row.status !== 'pending') throw new GameError('没有这条申请')
  run(app.db, `UPDATE friends SET status = 'accepted' WHERE id = ?`, requestId)
  notify(app, playerId)
}

/** 将双向好友关系转成对方名称、在线状态和待处理申请。 */
export function listFriends(app: App, playerId: string) {
  const rows = many<{ id: string; from_id: string; to_id: string; status: string }>(
    app.db,
    'SELECT * FROM friends WHERE from_id = ? OR to_id = ?',
    playerId,
    playerId,
  )
  return rows.map((row) => {
    const otherId = row.from_id === playerId ? row.to_id : row.from_id
    const other = one<PlayerRow>(app.db, 'SELECT * FROM players WHERE id = ?', otherId)
    return {
      id: row.id,
      status: row.status,
      incoming: row.to_id === playerId && row.status === 'pending',
      name: other?.name ?? '旅人',
      online: app.live.has(otherId),
    }
  })
}

/** 整理文本、校验长度与发送频率，持久化后发给频道内的在线角色。 */
export function postChat(app: App, playerId: string, channel: string, text: string) {
  const player = getPlayer(app, playerId)
  const clean = text.replace(/\s+/g, ' ').trim()
  if (!clean || clean.length > 80) throw new GameError('消息太长或是空的')
  const live = app.live.get(playerId)
  const t = now()
  if (live && t - live.chatAt < 700) throw new GameError('说得太快了')
  if (live) live.chatAt = t
  let stored = channel
  if (channel === 'map') {
    if (!live) throw new GameError('先进入世界才能用地图频道')
    stored = `map:${live.mapId}`
  } else if (channel === 'guild') {
    const member = one<{ guild_id: string }>(app.db, 'SELECT guild_id FROM guild_members WHERE player_id = ?', playerId)
    if (!member) throw new GameError('你不在公会里')
    stored = `guild:${member.guild_id}`
  } else if (channel !== 'world') {
    throw new GameError('没有这个频道')
  }
  const id = uid()
  run(
    app.db,
    'INSERT INTO chat_messages(id, channel, player_id, player_name, text, created_at) VALUES(?, ?, ?, ?, ?, ?)',
    id,
    stored,
    playerId,
    player.name,
    clean,
    t,
  )
  const message = { type: 'chat', id, channel: stored, name: player.name, text: clean, t }
  for (const other of app.live.values()) {
    if (!other.socket) continue
    if (stored === 'world') other.socket.send(JSON.stringify(message))
    else if (stored.startsWith('map:') && other.mapId === stored.slice(4)) other.socket.send(JSON.stringify(message))
    else if (stored.startsWith('guild:')) {
      const member = one(app.db, 'SELECT guild_id FROM guild_members WHERE player_id = ? AND guild_id = ?', other.id, stored.slice(6))
      if (member) other.socket.send(JSON.stringify(message))
    }
  }
  return message
}

/** 按频道读取最近聊天记录，并按时间从早到晚返回。 */
export function chatHistory(app: App, playerId: string, channel: string) {
  let stored = 'world'
  if (channel === 'map') {
    const live = app.live.get(playerId)
    const row = getPlayer(app, playerId)
    stored = `map:${live?.mapId ?? row.map_id}`
  } else if (channel === 'guild') {
    const member = one<{ guild_id: string }>(app.db, 'SELECT guild_id FROM guild_members WHERE player_id = ?', playerId)
    if (!member) return []
    stored = `guild:${member.guild_id}`
  }
  return many<{ id: string; player_name: string; text: string; created_at: number }>(
    app.db,
    'SELECT id, player_name, text, created_at FROM chat_messages WHERE channel = ? ORDER BY created_at DESC LIMIT 30',
    stored,
  ).reverse()
}

/** 必要时创建队伍，再向在线目标发送限时组队邀请。 */
export function inviteParty(app: App, playerId: string, name: string) {
  const target = one<PlayerRow>(app.db, 'SELECT * FROM players WHERE name = ?', name)
  if (!target) throw new GameError('找不到这个旅人')
  if (!app.live.get(target.id)) throw new GameError('对方不在线')
  let partyId = app.playerParty.get(playerId)
  if (!partyId) {
    partyId = uid()
    app.parties.set(partyId, { id: partyId, leaderId: playerId, memberIds: [playerId] })
    app.playerParty.set(playerId, partyId)
  }
  const party = app.parties.get(partyId)!
  if (party.memberIds.length >= 4) throw new GameError('队伍满了')
  app.invites.set(target.id, { fromId: playerId, fromName: getPlayer(app, playerId).name, until: now() + 60_000 })
  toast(app, target.id, `${getPlayer(app, playerId).name} 邀请你组队`)
}

/** 验证邀请期限与队伍容量，加入目标队伍并清理邀请。 */
export function joinParty(app: App, playerId: string) {
  const invite = app.invites.get(playerId)
  if (!invite || invite.until < now()) throw new GameError('邀请过期了')
  const partyId = app.playerParty.get(invite.fromId)
  const party = partyId ? app.parties.get(partyId) : undefined
  if (!party) throw new GameError('队伍已经散了')
  if (party.memberIds.length >= 4) throw new GameError('队伍满了')
  const old = app.playerParty.get(playerId)
  if (old && old !== party.id) leaveParty(app, playerId)
  if (!party.memberIds.includes(playerId)) party.memberIds.push(playerId)
  app.playerParty.set(playerId, party.id)
  app.invites.delete(playerId)
}

/** 从队伍移除角色；队长离开时转交队长，最后一人离开时解散。 */
export function leaveParty(app: App, playerId: string) {
  const partyId = app.playerParty.get(playerId)
  if (!partyId) return
  const party = app.parties.get(partyId)
  app.playerParty.delete(playerId)
  if (!party) return
  party.memberIds = party.memberIds.filter((id) => id !== playerId)
  if (party.memberIds.length === 0) {
    app.parties.delete(partyId)
    return
  }
  if (party.leaderId === playerId) party.leaderId = party.memberIds[0]!
}

/** 输出当前队伍的队长和成员名称，未组队时返回空值。 */
export function partyView(app: App, playerId: string) {
  const partyId = app.playerParty.get(playerId)
  if (!partyId) return null
  const party = app.parties.get(partyId)
  if (!party) return null
  return {
    id: party.id,
    leaderId: party.leaderId,
    members: party.memberIds.map((id) => ({ id, name: app.live.get(id)?.name ?? getPlayer(app, id).name })),
  }
}

/** 以队长或单人角色 ID 标识副本归属，让同队成员进入同一副本。 */
export function partyKey(app: App, playerId: string): string {
  const partyId = app.playerParty.get(playerId)
  const party = partyId ? app.parties.get(partyId) : undefined
  return party?.leaderId ?? playerId
}

/** 校验四个互不重复的当前武器技能，并同步存档与在线技能栏。 */
export function setLoadout(app: App, playerId: string, ids: string[]) {
  const player = getPlayer(app, playerId)
  if (!Array.isArray(ids) || ids.length !== 4) throw new GameError('技能栏是 4 个')
  const unique = new Set(ids)
  if (unique.size !== 4) throw new GameError('技能不能重复')
  for (const id of ids) {
    const skill = app.skills.get(id)
    if (!skill || skill.weapon !== player.weapon_type) throw new GameError('这不是当前武器的技能')
  }
  run(app.db, 'UPDATE players SET skill_loadout = ?, updated_at = ? WHERE id = ?', JSON.stringify(ids), now(), playerId)
  const live = app.live.get(playerId)
  if (live) live.loadout = ids
  notify(app, playerId)
}

/** 按等级和经验降序输出前十名角色。 */
export function leaderboard(app: App) {
  return many<{ name: string; level: number; exp: number }>(
    app.db,
    'SELECT name, level, exp FROM players ORDER BY level DESC, exp DESC LIMIT 10',
  )
}
