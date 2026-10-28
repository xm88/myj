/** 客户端完整流程回归：依次验证 NPC 交任务、战斗掉落、契约、派遣领取与上架拍卖。 */

/**
 * 把新手循环在本地服上走一遍：对话、进森林、打史莱姆、抓猫、派遣、上架。
 * 需要服务已经在 8787 端口运行。
 */
import WebSocket from 'ws'

/** 客户端回归服务地址，允许通过 FLOAT_URL 指向独立测试实例。 */
const base = process.env.FLOAT_URL || 'http://127.0.0.1:8787'

/** 向目标测试实例发送 REST 请求，接口失败时报告路径、状态与错误。 */
async function api(path: string, token: string, body?: unknown) {
  const response = await fetch(base + path, {
    method: body === undefined ? 'GET' : 'POST',
    headers: { 'content-type': 'application/json', authorization: `Bearer ${token}` },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const data = await response.json()
  if (!response.ok) throw new Error(`${path} ${response.status} ${data.error}`)
  return data
}

/** 建立客户端回归连接，并缓存消息供后续步骤等待条件。 */
function openSocket(token: string) {
  const ws = new WebSocket(base.replace('http', 'ws') + `/ws?token=${encodeURIComponent(token)}`)
  const queue: any[] = []
  let waiter: ((msg: any) => void) | null = null
  ws.on('message', (raw) => {
    const msg = JSON.parse(String(raw))
    if (waiter) {
      const next = waiter
      waiter = null
      next(msg)
    } else queue.push(msg)
  })
  return {
    ws,
    /** 从回归消息队列读取下一条消息，没有消息时等待并设置超时。 */
    next(ms = 8000): Promise<any> {
      const queued = queue.shift()
      if (queued) return Promise.resolve(queued)
      return new Promise((resolve, reject) => {
        const timer = setTimeout(() => reject(new Error('消息超时')), ms)
        waiter = (msg) => {
          clearTimeout(timer)
          resolve(msg)
        }
      })
    },
    /** 循环读取回归消息，直到条件满足或总等待时间耗尽。 */
    async until(pred: (msg: any) => boolean, ms = 20000) {
      const started = Date.now()
      while (Date.now() - started < ms) {
        const msg = await this.next(Math.max(1000, ms - (Date.now() - started)))
        if (pred(msg)) return msg
      }
      throw new Error('条件超时')
    },
  }
}

const guest = await api('/api/auth/guest', '', {})
const token = guest.token as string
const sock = openSocket(token)
await new Promise((resolve) => sock.ws.once('open', resolve))
await sock.until((msg) => msg.type === 'snapshot')

/** 持续发送点击目的地并检查服务器坐标，走近目标后继续下一步回归。 */
async function walk(x: number, y: number) {
  for (let i = 0; i < 40; i++) {
    sock.ws.send(JSON.stringify({ type: 'input', x: 0, y: 0, destX: x, destY: y }))
    const msg = await sock.until((item) => item.type === 'snapshot', 3000)
    if (Math.hypot(msg.self.x - x, msg.self.y - y) < 40) return
  }
  throw new Error(`走不到 ${x},${y}`)
}

await walk(480, 340)
sock.ws.send(JSON.stringify({ type: 'interact' }))
const dialog = await sock.until((msg) => msg.type === 'dialog')
const turnin = dialog.actions.find((action: any) => String(action.id).startsWith('turnin:'))
if (!turnin) throw new Error('镇长没有可交付的任务 ' + JSON.stringify(dialog.actions))
await api('/api/dialog', token, { npcId: 'npc_elder', action: turnin.id })

await api('/api/travel', token, { mapId: 'wild_greenleaf' })
await sock.until((msg) => msg.type === 'snapshot' && msg.mapId === 'wild_greenleaf')
await walk(260, 560)
sock.ws.send(JSON.stringify({ type: 'interact' }))
const ranger = await sock.until((msg) => msg.type === 'dialog')
const turninForest = ranger.actions.find((action: any) => String(action.id).startsWith('turnin:'))
if (!turninForest) throw new Error('林守不能交付 ' + JSON.stringify(ranger.actions))
await api('/api/dialog', token, { npcId: 'npc_ranger', action: turninForest.id })

const started = Date.now()
while (Date.now() - started < 40000) {
  const current = await api('/api/profile', token)
  const quest = current.quests.find((item: any) => item.id === 'q_main_03')
  if (quest && quest.objectives[0].current >= quest.objectives[0].need) break
  if (current.quests.some((item: any) => item.id === 'q_main_04')) break
  const snap = await sock.until((msg) => msg.type === 'snapshot', 3000)
  const mob = snap.entities.find((entity: any) => entity.templateId === 'slime' && entity.alive)
  if (!mob) continue
  sock.ws.send(JSON.stringify({ type: 'target', id: mob.id }))
  sock.ws.send(JSON.stringify({ type: 'input', x: 0, y: 0, destX: mob.x, destY: mob.y }))
}
let profile = await api('/api/profile', token)
const kill = profile.quests.find((item: any) => item.id === 'q_main_03')
if (!kill || kill.objectives[0].current < 5) {
  throw new Error('史莱姆没打够 ' + JSON.stringify(profile.quests.map((item: any) => [item.id, item.objectives])))
}
if (!profile.bag.some((item: any) => item.templateId === 'slime_gel')) throw new Error('没有掉落凝胶')

let captured = false
const captureStarted = Date.now()
while (!captured && Date.now() - captureStarted < 25000) {
  const msg = await sock.until((item) => item.type === 'snapshot', 3000)
  const cat = msg.entities.find((entity: any) => entity.templateId === 'forest_cat' && entity.alive)
  if (!cat) continue
  sock.ws.send(JSON.stringify({ type: 'target', id: cat.id }))
  sock.ws.send(JSON.stringify({ type: 'input', x: 0, y: 0, destX: cat.x, destY: cat.y }))
  if (cat.hp / cat.maxHp > 0.3) continue
  sock.ws.send(JSON.stringify({ type: 'input', x: 0, y: 0 }))
  sock.ws.send(JSON.stringify({ type: 'capture' }))
  const toast = await sock.until((item) => item.type === 'toast', 4000)
  if (String(toast.text).includes('成功') || String(toast.text).includes('加入')) captured = true
}
if (!captured) throw new Error('没有抓到森林猫')
profile = await api('/api/profile', token)

/** 生成灵伴技能的公共工厂，复用技能模板的默认字段。 */
const pet = profile.pets.find((item: any) => item.templateId === 'forest_cat')
await api('/api/dispatches', token, { petId: pet.id, nodeId: 'node_green_herb', durationId: 'scout' })
const running = (await api('/api/profile', token)).dispatches[0]
await api('/api/dispatches/finish', token, { id: running.id })
await api('/api/dispatches/claim', token, { id: running.id })
profile = await api('/api/profile', token)
if (!profile.bag.some((item: any) => item.templateId === 'herb_green')) throw new Error('派遣没有带回药草')
const stack = profile.bag.find((item: any) => item.templateId === 'herb_green')
await api('/api/market/list', token, { itemId: stack.id, count: 1, price: 12 })
const orders = await api('/api/market', token)
if (!orders.orders.some((order: any) => order.sellerId === profile.id)) throw new Error('拍卖没有挂上')

console.log('loop ok', { name: profile.name, pets: profile.pets.map((item: any) => item.name), gold: profile.gold })
sock.ws.close()
