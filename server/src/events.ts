/** 旧版 TypeScript 迁移参考：服务器玩法事件到任务模块的分发。默认服务启动执行 server/*.go。 */

/**
 * 任务事件总线。
 * 战斗、采集、拍卖各自只喊一声“发生了什么”，任务模块决定要不要记进度。
 * 这样战斗代码不用知道有哪些任务。
 */

import type { App } from './app.ts'

/** 由服务器玩法产生的任务事件，客户端不能直接提交进度。 */
export type GameEvent =
  | { type: 'kill'; monsterId: string }
  | { type: 'collect'; itemId: string; count: number }
  | { type: 'capture'; petId: string }
  | { type: 'craft'; recipeId: string }
  | { type: 'dispatch_start' }
  | { type: 'dispatch_claim' }
  | { type: 'market_list' }
  | { type: 'market_buy' }
  | { type: 'enter'; mapId: string }
  | { type: 'gather'; nodeKind: string }
  | { type: 'join_guild' }
  | { type: 'donate' }
  | { type: 'level'; level: number }
  | { type: 'world_boss' }
  | { type: 'buy_shop' }
  | { type: 'bank' }
  | { type: 'equip'; quality: string }
  | { type: 'talk'; npcId: string }

/** 任务事件接收函数的签名，用于解耦战斗与任务模块。 */
type Sink = (app: App, playerId: string, ev: GameEvent) => void
/** 已注册的任务事件接收器，玩法事件通过它转交任务模块。 */
let sink: Sink | null = null

/** 注册任务事件接收器，让各玩法通过同一事件入口推进任务。 */
export function bindQuest(fn: Sink) {
  sink = fn
}

/** 把玩法事件分发给活动任务，保存变化并提示可交付任务。 */
export function emit(app: App, playerId: string, ev: GameEvent) {
  sink?.(app, playerId, ev)
}
