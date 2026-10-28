/** REST 客户端：复用登录令牌和中文错误响应，只有提供请求体时才发送 POST。 */

/** 和游戏服说话。错误信息用服务器返回的中文，不在浏览器里另造一套。 */

export class ApiError extends Error {
  status: number
  /** 保存错误文本及其业务状态，供调用方统一处理。 */
  constructor(message: string, status: number) {
    super(message)
    this.status = status
  }
}

/** 从本地存储读取令牌，生成 REST 请求共用的鉴权头。 */
export function authHeader(): HeadersInit {
  const token = localStorage.getItem('float_token')
  return token ? { authorization: `Bearer ${token}` } : {}
}

/** 通过同源地址请求 REST；提供 body 时使用 POST，服务器错误转换成 ApiError。 */
export async function api<T>(path: string, body?: unknown): Promise<T> {
  const response = await fetch(path, {
    method: body === undefined ? 'GET' : 'POST',
    headers: { 'content-type': 'application/json', ...authHeader() },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const data = (await response.json().catch(() => ({}))) as { error?: string }
  if (!response.ok) throw new ApiError(data.error || '请求失败', response.status)
  return data as T
}
