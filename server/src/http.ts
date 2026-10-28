/** 旧版 TypeScript 迁移参考：REST 请求路由和鉴权。默认服务启动执行 server/*.go。 */

/**
 * REST。手机和网页助手走这里，不走战斗用的 WebSocket。
 * 能改金币、物品、拍卖和派遣的入口都在服务器里，浏览器只提交想做什么。
 */

import type { IncomingMessage, ServerResponse } from 'node:http'
import type { App } from './app.ts'
import {
  acceptFriend,
  buyFromShop,
  buyOrder,
  cancelOrder,
  chatHistory,
  claimMail,
  craft,
  createGuild,
  donateGuild,
  inviteParty,
  joinGuild,
  joinParty,
  leaveGuild,
  leaveParty,
  listFriends,
  listGuilds,
  listMail,
  listMarket,
  listOrder,
  partyView,
  postChat,
  requestFriend,
  sellToShop,
  setLoadout,
  leaderboard,
} from './economy.ts'
import { GameError } from './util.ts'
import { equipItem, moveStorage, unequipItem, useItem } from './inventory.ts'
import { listRaces } from './formula.ts'
import { allocateAttributes, listWeapons, login, loginGuest, readUserId, register, requirePlayer, createCharacter, resetAttributes } from './player.ts'
import { buildProfile, publicConfig } from './profile.ts'
import { acceptQuest, openDialog, turnIn } from './quest.ts'
import { claimDispatch, finishDispatchNow, readNews, setActivePet, startDispatch } from './pets.ts'
import { givePet } from './pets.ts'
import { travel } from './sim.ts'

/** REST 路由上下文，集中保存请求、JSON 参数和已验证的账号身份。 */
interface Ctx {
  /** 当前旧版游戏应用，提供数据库与在线世界状态。 */
  app: App
  /** 原始 HTTP 请求，保留方法、请求头与请求体。 */
  req: IncomingMessage
  /** 解析后的请求地址，路由读取路径和查询参数。 */
  url: URL
  /** HTTP 参数对象或邮件正文，含义由所属结构决定。 */
  body: Record<string, unknown>
  /** 所属账号标识，区别于角色实例标识。 */
  userId: string | null
}

/** 读取 REST 请求、验证登录并统一处理跨域与业务错误响应。 */
export async function handleHttp(app: App, req: IncomingMessage, res: ServerResponse) {
  res.setHeader('access-control-allow-origin', '*')
  res.setHeader('access-control-allow-headers', 'authorization, content-type')
  res.setHeader('access-control-allow-methods', 'GET, POST, OPTIONS')
  if (req.method === 'OPTIONS') {
    res.writeHead(204)
    res.end()
    return
  }
  try {
    const url = new URL(req.url ?? '/', 'http://127.0.0.1')
    const body = await readBody(req)
    const ctx: Ctx = { app, req, url, body, userId: null }
    const auth = req.headers.authorization
    if (auth) {
      try {
        ctx.userId = readUserId(app, auth)
      } catch (err) {
        if (!url.pathname.startsWith('/api/auth') && url.pathname !== '/api/health' && url.pathname !== '/api/config' && url.pathname !== '/api/weapons') {
          throw err
        }
      }
    }
    const payload = await route(ctx)
    send(res, 200, payload ?? { ok: true })
  } catch (err) {
    const known = err instanceof GameError
    if (!known) console.error(err)
    send(res, known ? err.status : 500, { error: known ? err.message : '服务器出错了' })
  }
}

/** 按方法与路径分发 REST 请求，公开接口之后要求登录与角色归属校验。 */
async function route(ctx: Ctx): Promise<unknown> {
  const { app, url, body } = ctx
  const path = url.pathname
  const method = ctx.req.method ?? 'GET'

  if (method === 'GET' && path === '/api/health') {
    return { ok: true, version: '0.1.0', devTools: app.devTools, weather: app.weather.id, period: app.period }
  }
  if (method === 'GET' && path === '/api/config') return publicConfig(app)
  if (method === 'GET' && path === '/api/weapons') return { weapons: listWeapons(), races: listRaces() }
  if (method === 'POST' && path === '/api/auth/register') return register(app, str(body.email), str(body.password))
  if (method === 'POST' && path === '/api/auth/login') return login(app, str(body.email), str(body.password))
  if (method === 'POST' && path === '/api/auth/guest') return loginGuest(app)

  const userId = ctx.userId
  if (!userId) throw new GameError('需要登录', 401)

  if (method === 'POST' && path === '/api/characters') {
    createCharacter(app, userId, str(body.name), str(body.weaponType), str(body.race) || 'human')
    return buildProfile(app, requirePlayer(app, userId).id)
  }

  if (method === 'GET' && path === '/api/profile') return buildProfile(app, requirePlayer(app, userId).id)
  const player = requirePlayer(app, userId)
  const id = player.id

  if (method === 'POST' && path === '/api/equip') {
    equipItem(app, id, str(body.itemId))
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/unequip') {
    unequipItem(app, id, str(body.slot))
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/use') {
    const used = useItem(app, id, str(body.itemId))
    if (used.grantPet) givePet(app, id, used.grantPet, false)
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/storage') {
    moveStorage(app, id, str(body.itemId), body.direction === 'withdraw' ? 'withdraw' : 'deposit', num(body.count, 1))
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/skills') {
    setLoadout(app, id, (body.ids as string[]) ?? [])
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/attributes') {
    allocateAttributes(app, id, body)
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/attributes/reset') {
    resetAttributes(app, id)
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/pets/active') {
    setActivePet(app, id, body.petId ? str(body.petId) : null)
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/dispatches') {
    startDispatch(app, id, str(body.petId), str(body.nodeId), str(body.durationId))
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/dispatches/claim') {
    claimDispatch(app, id, str(body.id))
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/dispatches/finish') {
    finishDispatchNow(app, id, str(body.id))
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/craft') {
    craft(app, id, str(body.recipeId))
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/shop/buy') {
    buyFromShop(app, id, str(body.npcId), str(body.itemId), num(body.count, 1))
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/shop/sell') {
    sellToShop(app, id, str(body.itemId), num(body.count, 1))
    return buildProfile(app, id)
  }
  if (method === 'GET' && path === '/api/market') return { orders: listMarket(app) }
  if (method === 'POST' && path === '/api/market/list') {
    listOrder(app, id, str(body.itemId), num(body.count, 1), num(body.price, 0))
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/market/buy') {
    buyOrder(app, id, str(body.id))
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/market/cancel') {
    cancelOrder(app, id, str(body.id))
    return buildProfile(app, id)
  }
  if (method === 'GET' && path === '/api/guilds') return { guilds: listGuilds(app) }
  if (method === 'POST' && path === '/api/guilds') {
    createGuild(app, id, str(body.name))
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/guilds/join') {
    joinGuild(app, id, str(body.guildId))
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/guilds/leave') {
    leaveGuild(app, id)
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/guilds/donate') {
    donateGuild(app, id, num(body.gold, 0))
    return buildProfile(app, id)
  }
  if (method === 'GET' && path === '/api/mail') return { mail: listMail(app, id) }
  if (method === 'POST' && path === '/api/mail/claim') {
    claimMail(app, id, str(body.id))
    return buildProfile(app, id)
  }
  if (method === 'GET' && path === '/api/friends') return { friends: listFriends(app, id) }
  if (method === 'POST' && path === '/api/friends/request') {
    requestFriend(app, id, str(body.name))
    return { ok: true }
  }
  if (method === 'POST' && path === '/api/friends/accept') {
    acceptFriend(app, id, str(body.id))
    return { friends: listFriends(app, id) }
  }
  if (method === 'GET' && path === '/api/chat') return { messages: chatHistory(app, id, url.searchParams.get('channel') ?? 'world') }
  if (method === 'POST' && path === '/api/chat') return postChat(app, id, str(body.channel), str(body.text))
  if (method === 'GET' && path === '/api/party') return { party: partyView(app, id) }
  if (method === 'POST' && path === '/api/party/invite') {
    inviteParty(app, id, str(body.name))
    return { party: partyView(app, id) }
  }
  if (method === 'POST' && path === '/api/party/join') {
    joinParty(app, id)
    return { party: partyView(app, id) }
  }
  if (method === 'POST' && path === '/api/party/leave') {
    leaveParty(app, id)
    return { party: null }
  }
  if (method === 'POST' && path === '/api/travel') {
    travel(app, id, str(body.mapId))
    return buildProfile(app, id)
  }
  if (method === 'POST' && path === '/api/dialog') {
    const action = str(body.action)
    const npcId = str(body.npcId)
    if (action.startsWith('accept:')) acceptQuest(app, id, action.slice(7))
    else if (action.startsWith('turnin:')) turnIn(app, id, action.slice(7))
    else if (action.startsWith('open:')) return { open: action.slice(5), profile: buildProfile(app, id) }
    else openDialog(app, id, npcId)
    return { profile: buildProfile(app, id), dialog: openDialog(app, id, npcId) }
  }
  if (method === 'GET' && path === '/api/news') return { news: readNews(app) }
  if (method === 'GET' && path === '/api/leaderboard') return { rows: leaderboard(app) }
  throw new GameError('没有这个接口', 404)
}

/** 只读取字符串请求参数并去掉首尾空白。 */
function str(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

/** 把请求参数转换为有限整数，异常值使用调用方指定的默认值。 */
function num(value: unknown, fallback: number): number {
  const n = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(n) ? Math.floor(n) : fallback
}

/** 读取并解析 JSON 请求体，GET 与 HEAD 不读取业务参数。 */
async function readBody(req: IncomingMessage): Promise<Record<string, unknown>> {
  if (req.method === 'GET' || req.method === 'HEAD') return {}
  const chunks: Buffer[] = []
  for await (const chunk of req) chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk))
  if (chunks.length === 0) return {}
  const raw = Buffer.concat(chunks).toString('utf8')
  if (!raw) return {}
  const parsed = JSON.parse(raw) as unknown
  if (!parsed || typeof parsed !== 'object') throw new GameError('请求格式不对')
  return parsed as Record<string, unknown>
}

/** 设置 JSON 响应头并写入 HTTP 状态码和响应正文。 */
function send(res: ServerResponse, status: number, payload: unknown) {
  const body = JSON.stringify(payload)
  res.writeHead(status, { 'content-type': 'application/json; charset=utf-8' })
  res.end(body)
}
