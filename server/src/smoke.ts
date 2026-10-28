/** 客户端协议冒烟测试：连接目标服务，验证初始档案、地图切换、移动与服务器伤害快照。 */

/**
 * 对着已经启动的服务走一遍核心循环。
 * 用法：先 npm run dev，再 npx tsx server/src/smoke.ts
 */
import WebSocket from 'ws'

/** 客户端回归服务地址，允许通过 FLOAT_URL 指向独立测试实例。 */
const base = process.env.FLOAT_URL || 'http://127.0.0.1:8787'

/** 向目标测试实例发送带令牌的 REST 请求，非成功响应立即让回归失败。 */
async function api(path: string, token: string | null, body?: unknown) {
  const response = await fetch(base + path, {
    method: body === undefined ? 'GET' : 'POST',
    headers: {
      'content-type': 'application/json',
      ...(token ? { authorization: `Bearer ${token}` } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const data = await response.json()
  if (!response.ok) throw new Error(`${path} ${response.status} ${data.error}`)
  return data
}

/** 建立回归测试 WebSocket，连接成功后等待地图与伤害快照。 */
function connect(token: string): Promise<WebSocket> {
  return new Promise((resolve, reject) => {
    const ws = new WebSocket(base.replace('http', 'ws') + `/ws?token=${encodeURIComponent(token)}`)
    ws.on('error', reject)
    ws.on('open', () => resolve(ws))
  })
}

/** 监听直到出现满足条件的服务器快照，超时则让回归测试失败。 */
function onceSnapshot(ws: WebSocket, pred: (msg: any) => boolean, ms = 8000): Promise<any> {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('等快照超时')), ms)
    const onMessage = (raw: Buffer) => {
      const msg = JSON.parse(String(raw))
      if (msg.type === 'snapshot' && pred(msg)) {
        clearTimeout(timer)
        ws.off('message', onMessage)
        resolve(msg)
      }
    }
    ws.on('message', onMessage)
  })
}

const health = await api('/api/health', null)
if (!health.ok) throw new Error('health')

const guest = await api('/api/auth/guest', null, {})
const token = guest.token as string
const profile = await api('/api/profile', token)
if (profile.level !== 1 || !profile.quests.length) throw new Error('初始档案不对')

const ws = await connect(token)
const town = await onceSnapshot(ws, (msg) => msg.mapId === 'town_starfall')
if (!town.self) throw new Error('没有自己的坐标')

await api('/api/travel', token, { mapId: 'wild_greenleaf' })
const forest = await onceSnapshot(ws, (msg) => msg.mapId === 'wild_greenleaf' && msg.entities.some((e: any) => e.templateId === 'slime' && e.alive))
const slime = forest.entities.find((e: any) => e.templateId === 'slime' && e.alive)
ws.send(JSON.stringify({ type: 'target', id: slime.id }))
ws.send(JSON.stringify({ type: 'input', x: 0, y: 0, destX: slime.x, destY: slime.y }))

const hurt = await onceSnapshot(ws, (msg) => msg.events?.some((e: any) => e.kind === 'dmg' && e.entityId === slime.id), 15000)
if (!hurt) throw new Error('没有打出伤害')

const after = await api('/api/profile', token)
if (after.mapId !== 'wild_greenleaf') throw new Error('地图没有切过去')

ws.close()
console.log('smoke ok', { name: profile.name, slime: slime.name, gold: after.gold, quests: after.quests.map((q: any) => q.name) })
