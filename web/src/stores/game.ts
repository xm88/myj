/** 网页共享状态：协调登录、档案、实时快照、自动追击意图、提示与功能面板。 */

/**
 * 客户端状态。
 * profile 来自 REST，snap 来自 WebSocket。两边不要互相覆盖背包：
 * 快照只更新血、蓝、坐标和金币，物品变化靠服务器的 dirty 消息再拉一次档案。
 */

import { defineStore } from 'pinia'
import { api, ApiError } from '../api'
import { arriveDistance, focusReach } from '../game/npc'

/** 界面短提示，使用独立 ID 管理自动消失与展示顺序。 */
export interface Toast {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: number
  /** 展示给玩家的提示、聊天或任务目标文字。 */
  text: string
}

/** 猫游记式操作：点地名赶路，点「战斗 / 对话」先走过去再交给服务器。 */
export type Intent =
  | { kind: 'move'; x: number; y: number; act?: 'interact'; focus?: string }
  | { kind: 'fight'; id: string }
  | { kind: 'capture'; id: string }

/** 全网页共用的 Pinia 状态仓库，协调登录、REST 档案与 WebSocket 快照。 */
export const useGame = defineStore('game', {
  state: () => ({
    // 令牌供 REST 和世界连接共用，刷新页面时从本地存储恢复。
    token: localStorage.getItem('float_token') || '',
    // 公共配置启动时读取一次，为显示、技能和任务导航提供模板。
    config: null as any,
    // 完整业务档案来自 REST，包含背包、任务、灵伴和成长信息。
    profile: null as any,
    // 世界高频快照来自 WebSocket，只包含当前分线与战斗状态。
    snap: null as any,
    // 冒险或助手模式同步给服务器，助手模式停止战斗并保护角色。
    mode: 'play' as 'play' | 'assistant',
    // 当前功能面板的 ID，空字符串表示没有打开弹窗。
    panel: '',
    // 服务器生成的 NPC 话题与可执行选项。
    dialog: null as any,
    // 短提示按独立编号定时移除，避免一个提示误删其他提示。
    toasts: [] as Toast[],
    // 最近的玩法文字记录，与战斗日志和聊天分开保存。
    log: [] as string[],
    // 按时间记录世界、地图和公会频道消息。
    chat: [] as { name: string; text: string; t: number; channel: string }[],
    // 当前发送频道，实际地图或公会归属由服务器确定。
    chatChannel: 'map' as 'map' | 'world' | 'guild',
    // 已选中的聊天呼叫对象，发送时加入文本前缀。
    callName: '',
    // 主界面当前显示地图场景或战斗视图。
    view: 'scene' as 'scene' | 'battle',
    // 正在执行的移动、追击或契约意图，由新快照推动下一步。
    intent: null as Intent | null,
    // 最近战斗事件与提示，用于战斗视图的记录列表。
    battleLog: [] as string[],
    // 上次发出追击请求的 performance.now() 时间，用于节流。
    lastChase: 0,
    // 近期世界新闻，启动读取历史，后续通过 WebSocket 接收。
    news: [] as { t: number; text: string }[],
    // 打开拍卖面板时读取的开放订单列表。
    market: [] as any[],
    // 打开社交面板时读取的邮件及领取状态。
    mail: [] as any[],
    // 好友关系和待接受申请的列表。
    friends: [] as any[],
    // 公会浏览列表，角色所属公会详情另在 profile 中。
    guilds: [] as any[],
    // 当前队伍详情，未加入队伍时为空。
    party: null as any,
    // 世界 WebSocket 当前是否已建立连接。
    connected: false,
    // 启动加载尚未结束时显示进入游戏提示。
    booting: true,
    // 已登录账号尚无角色时进入创建角色流程。
    needCharacter: false,
    // 当前世界连接，重连时替换并关闭旧连接。
    ws: null as WebSocket | null,
    // 本地短提示的递增编号，和数据库标识无关。
    toastSeq: 1,
    // 档案请求序号。后返回的旧请求不能覆盖更新的档案。
    profileEpoch: 0,
    // 断线后的重连等待，成功连上后回到 800 毫秒。
    retryDelay: 800,
    // 角色已在另一个窗口打开时，这个窗口停止抢连接。
    displaced: false,
  }),
  actions: {
    /** 为提示分配递增编号，并在3.2秒后移除对应提示。 */
    toast(text: string) {
      const id = this.toastSeq++
      this.toasts.push({ id, text })
      setTimeout(() => {
        this.toasts = this.toasts.filter((item) => item.id !== id)
      }, 3200)
    },
    /** 启动时加载公共配置，恢复已保存的登录状态并进入角色流程。 */
    async boot() {
      this.booting = true
      try {
        this.config = await api('/api/config')
        if (!this.token) return
        await this.refresh()
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) this.logout()
        else if (!(err instanceof ApiError && err.status === 404)) this.toast(err instanceof Error ? err.message : '加载失败')
      } finally {
        this.booting = false
      }
    },
    /** 保存登录令牌到状态仓库与本地存储，供刷新页面后继续登录。 */
    saveToken(token: string) {
      this.token = token
      localStorage.setItem('float_token', token)
    },
    /** 清理令牌与角色档案并关闭世界连接，返回登录界面。 */
    logout() {
      this.token = ''
      this.profile = null
      this.needCharacter = false
      this.displaced = false
      this.retryDelay = 800
      localStorage.removeItem('float_token')
      this.ws?.close()
      this.ws = null
      this.connected = false
    },
    /** 读取角色档案；区分未创建角色、鉴权失效与其他错误。 */
    async refresh() {
      const epoch = ++this.profileEpoch
      try {
        const profile = await api('/api/profile')
        if (epoch !== this.profileEpoch) return
        this.profile = profile
        this.needCharacter = false
        if (!this.ws || this.ws.readyState === WebSocket.CLOSED) this.connect()
      } catch (err) {
        if (epoch !== this.profileEpoch) return
        if (err instanceof ApiError && err.status === 404) {
          this.needCharacter = true
          this.profile = null
          return
        }
        throw err
      }
    },
    /** 建立世界 WebSocket 并绑定消息处理，断线后按当前登录状态尝试重连。 */
    connect() {
      if (!this.token || this.displaced) return
      const state = this.ws?.readyState
      if (state === WebSocket.CONNECTING || state === WebSocket.OPEN || state === WebSocket.CLOSING) return
      const proto = location.protocol === 'https:' ? 'wss' : 'ws'
      const socket = new WebSocket(`${proto}://${location.host}/ws?token=${encodeURIComponent(this.token)}`)
      this.ws = socket
      let replaced = false
      const giveUp = window.setTimeout(() => {
        if (socket.readyState === WebSocket.CONNECTING) socket.close()
      }, 4000)
      socket.onopen = () => {
        window.clearTimeout(giveUp)
        this.connected = true
        this.displaced = false
        this.retryDelay = 800
        socket.send(JSON.stringify({ type: 'mode', mode: this.mode }))
      }
      socket.onerror = () => {
        // 关闭事件会接着到来，这里不重复提示。
      }
      socket.onclose = (ev) => {
        window.clearTimeout(giveUp)
        if (this.ws === socket) {
          this.connected = false
          this.ws = null
        }
        if (replaced || ev.code === 4001) {
          this.displaced = true
          this.toast('这个角色已经在另一个窗口里打开')
          return
        }
        if (ev.code === 4003) {
          this.toast('登录已过期')
          this.logout()
          return
        }
        if (this.token && !this.displaced) {
          const wait = this.retryDelay
          this.retryDelay = Math.min(8000, this.retryDelay * 2)
          window.setTimeout(() => {
            if (this.token && !this.displaced) this.connect()
          }, wait)
        }
      }
      socket.onmessage = (ev) => {
        let msg: { type?: string } | null = null
        try {
          msg = JSON.parse(String(ev.data))
        } catch {
          return
        }
        if (msg?.type === 'replaced') {
          replaced = true
          this.displaced = true
          socket.close()
          return
        }
        this.onMessage(String(ev.data))
      }
    },
    /** 分发服务器快照、档案刷新、对话、聊天、新闻和天气消息。 */
    onMessage(raw: string) {
      let msg: any
      try {
        msg = JSON.parse(raw)
      } catch {
        return
      }
      if (!msg || typeof msg !== 'object') return
      if (msg.type === 'snapshot') {
        // 金币和等级以快照为准，避免打怪时档案还没刷新。
        if (this.profile && msg.self) {
          this.profile.hp = msg.self.hp
          this.profile.maxHp = msg.self.maxHp
          this.profile.mp = msg.self.mp
          this.profile.maxMp = msg.self.maxMp
          this.profile.level = msg.self.level
          this.profile.exp = msg.self.exp
          this.profile.expMax = msg.self.expMax
          this.profile.gold = msg.self.gold
          this.profile.mapId = msg.mapId
        }
        for (const event of msg.events ?? []) {
          if (event.kind === 'dmg' && event.amount) {
            const mine = event.entityId === this.profile?.id
            this.noteBattle(`${mine ? '受到' : '造成'} ${event.crit ? '暴击 ' : ''}${event.amount}`)
          } else if (event.kind === 'heal' && event.amount) {
            this.noteBattle(`回复 ${event.amount}`)
          } else if (event.text) {
            this.remember(event.text)
            this.noteBattle(event.text)
          }
        }
        this.log = this.log.slice(0, 24)
        this.snap = msg
        this.stepIntent()
      } else if (msg.type === 'dirty') {
        void this.refresh()
      } else if (msg.type === 'toast') {
        this.toast(msg.text)
        this.remember(msg.text)
        this.noteBattle(msg.text)
        if (String(msg.text).includes('击倒')) {
          if (this.intent?.kind !== 'move') this.intent = null
          this.view = 'scene'
        }
      } else if (msg.type === 'dialog') {
        this.dialog = msg
        this.view = 'scene'
      } else if (msg.type === 'chat') {
        this.chat.push({ name: msg.name, text: msg.text, t: msg.t, channel: msg.channel ?? 'world' })
        this.chat = this.chat.slice(-80)
      } else if (msg.type === 'news') {
        this.news.unshift({ t: Date.now(), text: msg.text })
        this.toast(msg.text)
      } else if (msg.type === 'world' && this.profile) {
        this.profile.weather = msg.weather
        this.profile.period = msg.period
        this.profile.periodName = msg.periodName
      }
    },
    /** 只在 WebSocket 已连接时提交 JSON 意图消息。 */
    send(message: object) {
      if (this.ws?.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify(message))
    },
    /** 记录最近战斗提示，并抑制连续重复的闪避文本。 */
    noteBattle(text: string) {
      if (!text || (text === '闪避' && this.battleLog[0] === '闪避')) return
      this.battleLog.unshift(text)
      this.battleLog = this.battleLog.slice(0, 24)
    },
    /** 保存最近玩法提示，避免同一条文本连续刷屏。 */
    remember(text: string) {
      if (!text || this.log[0] === text) return
      this.log.unshift(text)
      this.log = this.log.slice(0, 24)
    },
    /** 把「点一个名字」变成坐标移动。focus 指定要找的人、采集点或门，避免走到跟前却触发旁边的东西。 */
    go(x: number, y: number, act?: 'interact', focus?: string) {
      this.dialog = null
      this.view = 'scene'
      this.send({ type: 'target', id: null })
      const self = this.snap?.self
      if (self && act === 'interact' && focus && Math.hypot(self.x - x, self.y - y) <= focusReach(focus)) {
        this.intent = null
        this.send({ type: 'interact', focus })
        return
      }
      this.intent = { kind: 'move', x, y, act, focus }
      this.send({ type: 'input', x: 0, y: 0, destX: x, destY: y })
    },
    /** 选中怪物并进入战斗视图，追击和普攻由客户端意图与服务器配合完成。 */
    fight(id: string) {
      this.dialog = null
      this.view = 'battle'
      this.intent = { kind: 'fight', id }
      this.battleLog = ['进入战斗。双方会自动攻击，技能在下面的快捷栏。']
      this.send({ type: 'target', id })
    },
    /** 选中契约目标并设置追击意图，靠近后再请求服务器尝试契约。 */
    capture(id: string) {
      this.dialog = null
      this.view = 'battle'
      this.intent = { kind: 'capture', id }
      this.noteBattle('靠近后尝试契约')
      this.send({ type: 'target', id })
    },
    /** 取消攻击目标，优先移动到回镇门或地图入口并更新战斗提示。 */
    flee() {
      const map = this.config?.maps?.find((item: { id: string; portals?: { x: number; y: number; to: string }[]; spawn?: { x: number; y: number } }) => item.id === (this.snap?.mapId ?? this.profile?.mapId))
      this.view = 'scene'
      this.send({ type: 'target', id: null })
      const door = map?.portals?.find((portal) => portal.to === 'town_starfall') ?? map?.portals?.[0]
      const point = door ?? map?.spawn
      if (point) {
        this.intent = { kind: 'move', x: point.x, y: point.y, act: door ? 'interact' : undefined, focus: door ? `portal:${door.x},${door.y}` : undefined }
        this.send({ type: 'input', x: 0, y: 0, destX: point.x, destY: point.y })
      } else {
        this.intent = null
      }
      this.noteBattle('你选择了逃跑')
      this.remember('你选择了逃跑')
    },
    /** 根据每次服务器快照继续移动或追击，到达有效距离后提交互动或契约。 */
    stepIntent() {
      const self = this.snap?.self
      const intent = this.intent
      if (!self || !intent) return
      if (intent.kind === 'move') {
        const distance = Math.hypot(self.x - intent.x, self.y - intent.y)
        const reach = intent.act === 'interact' ? arriveDistance(intent.focus) : 28
        if (distance > reach) {
          const stamp = performance.now()
          if (stamp - this.lastChase > 400) {
            this.lastChase = stamp
            this.send({ type: 'input', x: 0, y: 0, destX: intent.x, destY: intent.y })
          }
          return
        }
        const act = intent.act
        const focus = intent.focus
        this.intent = null
        if (act === 'interact') this.send(focus ? { type: 'interact', focus } : { type: 'interact' })
        return
      }
      const mob = (this.snap?.entities ?? []).find((entity: { id: string; alive: boolean; x: number; y: number; name?: string }) => entity.id === intent.id)
      if (!mob?.alive) {
        this.intent = null
        this.noteBattle(mob?.name ? `${mob.name} 被击败了` : '目标已经不在了')
        return
      }
      const distance = Math.hypot(self.x - mob.x, self.y - mob.y)
      if (distance > 64) {
        const stamp = performance.now()
        if (stamp - this.lastChase > 180) {
          this.lastChase = stamp
          this.send({ type: 'input', x: 0, y: 0, destX: mob.x, destY: mob.y })
        }
        return
      }
      if (intent.kind === 'capture') {
        this.intent = null
        this.send({ type: 'capture' })
      }
    },
    /** 同步冒险或助手模式，清空功能面板并通知服务器切换安全状态。 */
    async setMode(mode: 'play' | 'assistant') {
      this.mode = mode
      this.panel = ''
      this.send({ type: 'mode', mode })
    },
    /** 提交 REST 业务操作，更新返回档案并统一显示失败原因。 */
    async act(path: string, body?: unknown) {
      try {
        const result = await api<any>(path, body ?? {})
        if (result?.profile) {
          this.profileEpoch++
          this.profile = result.profile
        } else if (result?.bag || result?.quests) {
          this.profileEpoch++
          this.profile = result
        } else await this.refresh()
        return result
      } catch (err) {
        this.toast(err instanceof Error ? err.message : '没做成')
        throw err
      }
    },
  },
})
