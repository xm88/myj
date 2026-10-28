/** 旧版 TypeScript 迁移参考：客户端世界意图消息处理。默认服务启动执行 server/*.go。 */

/**
 * 战斗通道。一条 WebSocket 只做“我想做什么”，不做“我造成了多少伤害”。
 */

import type { IncomingMessage } from 'node:http'
import type { WebSocket } from 'ws'
import type { App } from './app.ts'
import { postChat } from './economy.ts'
import { readUserId, requirePlayer } from './player.ts'
import { attachPlayer, castSkill, detachPlayer, interact, setInput, setMode, setTarget, tryCapture } from './sim.ts'
import { GameError } from './util.ts'
import { toast } from './net.ts'

/** 验证令牌、接管角色连接并读取意图；旧连接不能再操控已被新连接接管的角色。 */
export function handleSocket(app: App, socket: WebSocket, req: IncomingMessage) {
  const url = new URL(req.url ?? '/', 'http://127.0.0.1')
  let playerId = ''
  try {
    const userId = readUserId(app, `Bearer ${url.searchParams.get('token') ?? ''}`)
    playerId = requirePlayer(app, userId).id
  } catch (err) {
    socket.send(JSON.stringify({ type: 'toast', text: err instanceof GameError ? err.message : '登录失败' }))
    socket.close()
    return
  }
  attachPlayer(app, playerId, socket)

  socket.on('message', (raw) => {
    let message: { type?: string; [key: string]: unknown }
    try {
      message = JSON.parse(String(raw)) as { type?: string }
    } catch {
      return
    }
    try {
      switch (message.type) {
        case 'input':
          setInput(app, playerId, Number(message.x ?? 0), Number(message.y ?? 0), optionalNumber(message.destX), optionalNumber(message.destY))
          break
        case 'target':
          setTarget(app, playerId, typeof message.id === 'string' ? message.id : null)
          break
        case 'skill':
          if (message.slot === 'dodge' || message.slot === 'pet') castSkill(app, playerId, message.slot)
          else castSkill(app, playerId, Number(message.slot ?? 0))
          break
        case 'capture':
          tryCapture(app, playerId)
          break
        case 'interact':
          interact(app, playerId, typeof message.focus === 'string' ? message.focus : undefined)
          break
        case 'mode':
          setMode(app, playerId, String(message.mode ?? 'play'))
          break
        case 'chat':
          postChat(app, playerId, String(message.channel ?? 'world'), String(message.text ?? ''))
          break
        default:
          break
      }
    } catch (err) {
      toast(app, playerId, err instanceof GameError ? err.message : '做不到')
    }
  })

  socket.on('close', () => detachPlayer(app, playerId, socket))
}

/** 只接受有限数字作为可选坐标，拒绝异常输入值。 */
function optionalNumber(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined
}
