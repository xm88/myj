/** 旧版 TypeScript 迁移参考：错误、标识、时间和数值工具。默认服务启动执行 server/*.go。 */

/** 业务错误。HTTP 层把它变成 4xx，WebSocket 层把它变成一条 toast。 */
export class GameError extends Error {
  status: number
  /** 保存错误文本及其业务状态，供调用方统一处理。 */
  constructor(message: string, status = 400) {
    super(message)
    this.status = status
  }
}

/** 生成随机标识，用于账号、角色、物品和其他实例。 */
export function uid(): string {
  return crypto.randomUUID()
}

/** 返回 Unix 毫秒时间戳；数据库、冷却和派遣共用这一单位。 */
export function now(): number {
  return Date.now()
}

/** 将数值限制在给定区间，防止坐标、血量或输入越界。 */
export function clamp(n: number, min: number, max: number): number {
  return Math.max(min, Math.min(max, n))
}

/** 计算两个世界坐标之间的欧氏距离，用于攻击、互动和追击范围判断。 */
export function dist(ax: number, ay: number, bx: number, by: number): number {
  return Math.hypot(ax - bx, ay - by)
}

/** 在包含两个端点的整数区间内随机取值。 */
export function rollInt(min: number, max: number): number {
  const a = Math.ceil(min)
  const b = Math.floor(max)
  return a + Math.floor(Math.random() * (b - a + 1))
}

/** 品质从低到高的排序值，用于任务门槛比较。 */
export const QUALITY_RANK: Record<string, number> = {
  white: 0,
  green: 1,
  blue: 2,
  purple: 3,
  gold: 4,
}
