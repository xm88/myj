/** 旧版 TypeScript 迁移参考：邮件存档和附件。默认服务启动执行 server/*.go。 */

/** 系统邮件。背包满了、拍卖成交、派遣发现事件，都从这里进信箱。 */

import type { App } from './app.ts'
import { run } from './db.ts'
import { notify } from './net.ts'
import type { ItemExtra } from './types.ts'
import { now, uid } from './util.ts'

/** 邮件可领取的金币和物品列表，物品可保留装备词条。 */
export interface MailAttachment {
  /** 金币数量，所有变更由服务器校验余额并持久化。 */
  gold?: number
  /** 奖励物品列表，每项包含模板、数量及可选装备词条。 */
  items?: { templateId: string; count: number; extra?: ItemExtra | null }[]
}

/** 写入角色邮件及可选附件，再通知客户端刷新档案。 */
export function sendMail(app: App, playerId: string, title: string, body: string, attachments?: MailAttachment) {
  const has = Boolean(attachments && ((attachments.gold ?? 0) > 0 || (attachments.items?.length ?? 0) > 0))
  run(
    app.db,
    'INSERT INTO mail(id, player_id, title, body, attachments, is_read, claimed, created_at) VALUES(?, ?, ?, ?, ?, 0, ?, ?)',
    uid(),
    playerId,
    title,
    body,
    attachments ? JSON.stringify(attachments) : null,
    has ? 0 : 1,
    now(),
  )
  notify(app, playerId)
}
