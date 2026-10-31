/** 网页共享状态：协调登录、档案、实时快照、自动追击意图、提示与功能面板。 */

/**
 * 客户端状态。
 * profile 来自 REST，snap 来自 WebSocket。两边不要互相覆盖背包：
 * 快照只更新血、蓝、坐标和金币，物品变化靠服务器的 dirty 消息再拉一次档案。
 */

import { defineStore } from 'pinia'
import { api, ApiError } from '../api'
import { gateEdge, pastGate } from '../game/camera'
import { nearestPacks } from '../game/crowd'
import { guideQuest, questGuide } from '../game/guide'
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
  | { kind: 'move'; x: number; y: number; act?: 'interact'; focus?: string; worldX?: number; worldY?: number; mapId?: string }
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
    // 连招打开后，轮到自己时自动普攻。记在本机，不进存档。
    autoFight: localStorage.getItem('float_auto') === '1',
    // 施法圈开关。只影响画面上那一下闪光，不进存档。
    showFx: localStorage.getItem('float_fx') !== '0',
    // 每次放技能加一，战斗和场景用来闪一下施法圈。
    castSeq: 0,
    // 飘字。只活大约一秒，用来在场景和战斗画面上标伤害。
    floats: [] as { id: number; text: string; kind: string; crit: boolean; guarded: boolean; x: number; y: number; entityId: string; at: number }[],
    // 飘字编号。
    floatSeq: 1,
    // 当面交易的双栏。没有交易时是空的。
    trade: null as any,
    // 场景里点中的摊位，摆摊面板会先打开这一家。
    stallFocus: '',
    // 已经对这个目标发过开战，避免每个快照都重复请求。
    engaging: '',
    // 连招在同一回合只自动攻击一次。
    lastBoutKey: '',
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
          this.pushFloat(event)
          if (event.kind === 'dmg' && event.amount) {
            const mine = event.entityId === this.profile?.id
            const guard = event.guarded ? '守势 ' : ''
            const hit = `${guard}${event.crit ? '暴击 ' : ''}${event.amount}`
            this.noteBattle(event.beast ? `召唤兽受到 ${hit}` : `${mine ? '受到' : '造成'} ${hit}`)
          } else if (event.kind === 'heal' && event.amount) {
            this.noteBattle(`回复 ${event.amount}`)
          } else if (event.text) {
            this.remember(event.text)
            this.noteBattle(event.text)
          }
        }
        this.log = this.log.slice(0, 24)
        this.snap = msg
        if (msg.bout?.phase === 'fled' || msg.bout?.phase === 'defeat') this.view = 'scene'
        if ((msg.bout?.yours || msg.bout?.yoursBeast) && this.autoFight && this.view === 'battle') {
          const key = `${msg.bout.round}:${msg.bout.await}:${msg.bout.yoursBeast ? 'beast' : 'you'}`
          if (this.lastBoutKey !== key) {
            this.lastBoutKey = key
            this.send({ type: 'bout', action: 'attack' })
          }
        }
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
      } else if (msg.type === 'trade') {
        this.trade = msg.trade ?? null
        if (this.trade) this.panel = 'trade'
        else if (this.panel === 'trade') this.panel = ''
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
    /** 记下一条会自己消失的飘字。 */
    pushFloat(event: { kind?: string; amount?: number; text?: string; crit?: boolean; guarded?: boolean; x?: number; y?: number; entityId?: string }) {
      const kind = String(event.kind || '')
      let text = ''
      if (kind === 'dmg' && event.amount) text = `${event.crit ? '暴' : ''}${event.amount}`
      else if (kind === 'heal' && event.amount) text = `+${event.amount}`
      else if (kind === 'miss') text = '闪'
      else if (event.text) text = String(event.text)
      else return
      const at = performance.now()
      const id = this.floatSeq++
      this.floats.push({
        id,
        text,
        kind,
        crit: Boolean(event.crit),
        guarded: Boolean(event.guarded),
        x: Number(event.x) || 0,
        y: Number(event.y) || 0,
        entityId: event.entityId ? String(event.entityId) : '',
        at,
      })
      this.floats = this.floats.filter((item) => at - item.at < 1100).slice(-12)
    },
    /** 开关连招。服务器仍逐次校验蓝耗、冷却和距离。 */
    setAuto(on: boolean) {
      this.autoFight = on
      localStorage.setItem('float_auto', on ? '1' : '0')
      this.toast(on ? '连招开了。轮到你时会自动攻击' : '连招关了')
    },
    /** 开关施法圈。关掉之后技能仍照常发出。 */
    setFx(on: boolean) {
      this.showFx = on
      localStorage.setItem('float_fx', on ? '1' : '0')
    },
    /** 记一笔技能出手，给施法圈用。关闭光效时什么都不做。 */
    noteCast() {
      if (!this.showFx) return
      this.castSeq += 1
    },
    /** 摆出守势。回合战斗里这是防御；平时仍是少挨打的那一拍。 */
    guard() {
      if (this.snap?.bout?.yours) this.send({ type: 'bout', action: 'defend' })
      else this.send({ type: 'guard' })
    },
    /** 提交当前回合的动作。 */
    bout(action: string, extra: Record<string, unknown> = {}) {
      if (action === 'skill') this.noteCast()
      this.send({ type: 'bout', action, ...extra })
    },
    /** 按一条任务或当前主步骤走过去。找不到落点就打开任务窗口。 */
    pursue(questId?: string) {
      const quests = this.profile?.quests ?? []
      const mapId = this.snap?.mapId || this.profile?.mapId || ''
      const who = this.snap?.self ?? { x: this.profile?.x ?? 0, y: this.profile?.y ?? 0 }
      const monsters = (this.snap?.entities ?? []).filter((entity: { kind: string; alive: boolean }) => entity.kind === 'monster' && entity.alive)
      const packs = nearestPacks(monsters, who)
      const quest = questId ? quests.find((item: { id: string }) => item.id === questId) : null
      const guide = quest ? guideQuest(quest, this.config, mapId, packs) : questGuide(quests, this.config, mapId, packs)
      if (!guide) {
        this.panel = 'quest'
        return
      }
      if (guide.fightId) {
        if (guide.capture) this.capture(guide.fightId)
        else this.fight(guide.fightId)
        return
      }
      if (typeof guide.x === 'number' && typeof guide.y === 'number') this.go(guide.x, guide.y, guide.act, guide.focus)
      else this.panel = 'quest'
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
    /** 当前地图通向目标地块的门外一步。没有直连的门就返回空。 */
    gateExit(mapId: string) {
      const here = this.snap?.mapId || this.profile?.mapId
      const map = this.config?.maps?.find((item: { id: string; width?: number; height?: number; portals?: { x: number; y: number; to: string }[] }) => item.id === here)
      const portal = map?.portals?.find((item) => item.to === mapId)
      if (!map || !portal) return null
      return pastGate(gateEdge(portal.x, portal.y, map.width || 1, map.height || 1), portal.x, portal.y)
    },
    /** 走到另一块地上的世界坐标。先穿过相连的门，落地后再改成那一块上的本地坐标。 */
    goAcross(worldX: number, worldY: number, mapId: string, act?: 'interact', focus?: string) {
      const here = this.snap?.mapId || this.profile?.mapId || ''
      if (!mapId || mapId === here) {
        const chunk = (this.snap?.chunks ?? []).find((item: { id: string }) => item.id === here)
        const x = chunk ? worldX - chunk.originX : worldX
        const y = chunk ? worldY - chunk.originY : worldY
        this.go(x, y, act, focus)
        return
      }
      const gate = this.gateExit(mapId)
      if (!gate) return
      this.dialog = null
      this.view = 'scene'
      this.send({ type: 'target', id: null })
      this.intent = { kind: 'move', x: gate.x, y: gate.y, act, focus, worldX, worldY, mapId }
      this.send({ type: 'input', x: 0, y: 0, destX: gate.x, destY: gate.y })
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
      this.engaging = ''
      this.battleLog = ['进入战斗。靠近后按速度排回合，轮到你再选攻击、技能、物品、防御或逃跑。']
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
      if (this.snap?.bout?.phase === 'choose') {
        this.send({ type: 'bout', action: 'flee' })
        this.noteBattle('你尝试脱身')
        return
      }
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
      if (this.snap?.bout) {
        this.intent = null
        return
      }
      if (intent.kind === 'move') {
        if (intent.mapId && intent.mapId !== this.snap.mapId) {
          const stamp = performance.now()
          if (stamp - this.lastChase > 400) {
            this.lastChase = stamp
            this.send({ type: 'input', x: 0, y: 0, destX: intent.x, destY: intent.y })
          }
          return
        }
        if (intent.worldX != null && intent.worldY != null && intent.mapId && intent.mapId === this.snap.mapId) {
          const chunk = (this.snap.chunks ?? []).find((item: { id: string; originX: number; originY: number }) => item.id === intent.mapId)
          if (chunk) {
            const x = intent.worldX - chunk.originX
            const y = intent.worldY - chunk.originY
            this.intent = { kind: 'move', x, y, act: intent.act, focus: intent.focus }
            this.send({ type: 'input', x: 0, y: 0, destX: x, destY: y })
            return
          }
        }
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
      const mob = (this.snap?.entities ?? []).find((entity: { id: string; alive: boolean; x: number; y: number; name?: string; mapId?: string; seam?: boolean; worldX?: number; worldY?: number }) => entity.id === intent.id)
      if (!mob?.alive) {
        this.intent = null
        this.noteBattle(mob?.name ? `${mob.name} 被击败了` : '目标已经不在了')
        return
      }
      const sameMap = !mob.mapId || mob.seam || mob.mapId === this.snap.mapId
      if (!sameMap) {
        const wd = Math.hypot((self.worldX ?? self.x) - (mob.worldX ?? mob.x), (self.worldY ?? self.y) - (mob.worldY ?? mob.y))
        if (wd <= 80) {
          if (intent.kind === 'capture') {
            this.intent = null
            this.send({ type: 'capture' })
            return
          }
          if (this.engaging !== intent.id) {
            this.engaging = intent.id
            this.send({ type: 'engage' })
          }
          return
        }
        const gate = this.gateExit(mob.mapId)
        if (gate) {
          const stamp = performance.now()
          if (stamp - this.lastChase > 180) {
            this.lastChase = stamp
            this.send({ type: 'input', x: 0, y: 0, destX: gate.x, destY: gate.y })
          }
        }
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
        return
      }
      if (this.engaging !== intent.id) {
        this.engaging = intent.id
        this.send({ type: 'engage' })
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
        if (result && Object.prototype.hasOwnProperty.call(result, 'trade')) {
          this.trade = result.trade
          if (result.trade) this.panel = 'trade'
          else if (this.panel === 'trade') this.panel = ''
        }
        return result
      } catch (err) {
        this.toast(err instanceof Error ? err.message : '没做成')
        throw err
      }
    },
  },
})
