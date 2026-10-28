/** 旧版 TypeScript 迁移参考：HTTP、WebSocket 和模拟循环的启动关闭。默认服务启动执行 server/*.go。 */

/**
 * 浮岛旅团 V0.1 入口。
 * 一个进程里同时跑 HTTP 助手接口、WebSocket 世界，以及 10Hz 的战斗循环。
 */

import path from 'node:path'
import { createServer } from 'node:http'
import { WebSocketServer } from 'ws'
import { createApp } from './app.ts'
import { openDatabase } from './db.ts'
import { bindQuest } from './events.ts'
import { handleHttp } from './http.ts'
import { onGameEvent } from './quest.ts'
import { TICK_MS, tick } from './sim.ts'
import { handleSocket } from './ws.ts'
import { saveLive } from './player.ts'

/** 项目根目录，用于定位配置、存档和本地工具链。 */
const root = path.resolve(import.meta.dirname, '../..')
/** 游戏存档路径，可通过 FLOAT_DB 指定独立数据库。 */
const dbFile = process.env.FLOAT_DB || path.join(root, 'data', 'float-island.sqlite')
/** 服务监听端口，默认值可被 PORT 环境变量覆盖。 */
const port = Number(process.env.PORT || 8787)

/** 当前旧版服务使用的 SQLite 连接。 */
const db = openDatabase(dbFile)
/** 旧版游戏应用，集中维护配置索引与在线世界状态。 */
const app = createApp(db)
bindQuest(onGameEvent)

/** 承载 REST 请求的旧版 HTTP 服务。 */
const server = createServer((req, res) => {
  void handleHttp(app, req, res)
})
/** 挂载在 HTTP 服务上的旧版 WebSocket 世界通道。 */
const wss = new WebSocketServer({ server, path: '/ws' })
wss.on('connection', (socket, req) => handleSocket(app, socket, req))

/** 循环或倒计时的定时器句柄，组件卸载或进程关闭时需要清理。 */
const timer = setInterval(() => tick(app), TICK_MS)
/** 定期保存所有在线角色的计时器。 */
const saveTimer = setInterval(() => {
  for (const live of app.live.values()) saveLive(app, live)
}, 5000)

server.listen(port, '0.0.0.0', () => {
  console.log(`浮岛旅团服务已启动 http://127.0.0.1:${port}`)
  console.log(`数据库 ${dbFile}`)
  console.log(app.devTools ? '封闭测试开关已打开：助手里可以把派遣立刻完成。' : '生产模式')
})

/** 保存在线角色后退出旧版进程，用于迁移参考服务的信号处理。 */
function shutdown() {
  clearInterval(timer)
  clearInterval(saveTimer)
  for (const live of app.live.values()) saveLive(app, live)
  server.close()
  process.exit(0)
}

process.on('SIGINT', shutdown)
process.on('SIGTERM', shutdown)
