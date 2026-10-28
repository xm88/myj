/** 旧版 TypeScript 迁移参考：WebSocket 提示与广播。默认服务启动执行 server/*.go。 */

/** 给在线玩家推一条消息。人没连着 websocket 时静默丢掉，数据已经在数据库里。 */

import type { WebSocket } from 'ws'
import type { App } from './app.ts'
import type { LivePlayer } from './runtime.ts'

/** 当前 WebSocket 可写时向指定在线角色发送 JSON 消息。 */
export function send(live: LivePlayer, msg: object) {
  const socket = live.socket as WebSocket | null
  if (socket && socket.readyState === 1) socket.send(JSON.stringify(msg))
}

/** 向在线角色发送中文提示消息。 */
export function toast(app: App, playerId: string, text: string) {
  const live = app.live.get(playerId)
  if (live) send(live, { type: 'toast', text })
}

/** 告诉客户端重新拉档案。背包、任务、金币明细都走这一下，避免快照越来越大。 */
export function notify(app: App, playerId: string) {
  const live = app.live.get(playerId)
  if (live) send(live, { type: 'dirty' })
}

/** 向所有在线连接广播同一条世界消息。 */
export function broadcast(app: App, msg: object) {
  const raw = JSON.stringify(msg)
  for (const live of app.live.values()) {
    if (live.socket && live.socket.readyState === 1) live.socket.send(raw)
  }
}
