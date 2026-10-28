/** npm 与 Go 的桥接脚本：选择工具链、设置项目路径并转发命令输出、信号和退出码。 */

import { spawn } from 'node:child_process'
import { existsSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import path from 'node:path'

/** 当前脚本所在目录，用于解析项目内的固定路径。 */
const dir = path.dirname(fileURLToPath(import.meta.url))
/** 项目根目录，用于定位配置、存档和本地工具链。 */
const root = path.dirname(dir)
/** 项目内便携 Go 可执行文件路径，存在时无需修改系统 PATH。 */
const local = path.join(root, '.tools', 'go', 'bin', process.platform === 'win32' ? 'go.exe' : 'go')
/** Go 启动命令，优先采用 GO_BINARY，其次便携工具链，最后系统 go。 */
const command = process.env.GO_BINARY || (existsSync(local) ? local : 'go')
/** 转发给 Go 的参数。Windows 构建补上 .exe，其他系统保持无扩展名。 */
const args = process.argv.slice(2)
if (process.platform === 'win32') {
  const out = args.indexOf('-o')
  if (out >= 0 && args[out + 1] && !args[out + 1].toLowerCase().endsWith('.exe')) args[out + 1] += '.exe'
}
/** 实际执行 Go 命令的子进程，沿用终端输入输出与退出状态。 */
const child = spawn(command, args, {
  cwd: dir,
  stdio: 'inherit',
  env: {
    ...process.env,
    FLOAT_ROOT: root,
    ...(command === local ? {
      GOPATH: process.env.GOPATH || path.join(root, '.tools', 'gopath'),
      GOCACHE: process.env.GOCACHE || path.join(root, '.tools', 'gocache'),
    } : {}),
  },
})
child.on('error', (error) => {
  console.error(`无法启动 Go：${error.message}。请安装 Go 1.25+，或设置 GO_BINARY。`)
  process.exitCode = 1
})
child.on('exit', (code, signal) => { process.exitCode = code ?? (signal ? 1 : 0) })
for (const signal of ['SIGINT', 'SIGTERM']) process.on(signal, () => child.kill(signal))
