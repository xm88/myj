/** 互动提示公共规则：统一 NPC 任务标记、精确 focus 标识和有效互动距离。 */

/** 地图、名册和按键共用的 NPC 状态。任务标记只看档案，不自己猜进度。 */

export type NpcMark = 'turnin' | 'talk' | 'offer' | 'progress' | ''

/** 网页任务目标视图，包含服务器进度、需求和可定位目标。 */
interface QuestGoal {
  /** 服务器确认的当前任务目标进度。 */
  current: number
  /** 完成当前任务目标要求的进度。 */
  need: number
  /** 服务器任务目标的事件类型，用于匹配引导与 NPC 提示。 */
  type?: string
  /** 任务引导可定位的模板、地图或资源种类标识。 */
  target?: string
}

/** NPC 标记判断需要的任务交付目标与当前进度。 */
interface QuestView {
  /** 接收交付任务的 NPC ID 或其显示名称。 */
  turnin?: string
  /** 需要交付任务的 NPC 模板 ID，优先于显示名称匹配。 */
  turninId?: string
  /** 任务需要完成的全部目标列表。 */
  objectives?: QuestGoal[]
}

/** 可接取任务的发放 NPC 信息。 */
interface OfferView {
  /** 发放任务的 NPC 模板 ID。 */
  giver?: string
  /** 发放可接取任务的 NPC 模板 ID。 */
  giverId?: string
}

/** NPC 标记判断使用的角色任务与可接取任务列表。 */
interface ProfileView {
  /** 当前已接取任务视图，NPC 任务标记只根据服务器进度判断。 */
  quests?: QuestView[]
  /** 服务器确认当前可接取的任务视图。 */
  available?: OfferView[]
}

/** NPC 标记与提示使用的最小身份信息。 */
interface NpcView {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** NPC 职能或对话窗口的显示标题。 */
  title?: string
}

/** 生成服务器能识别的精确 NPC 互动标识。 */
export function npcFocus(id: string) {
  return `npc:${id}`
}

/** 生成采集点互动标识，避免在附近触发其他对象。 */
export function nodeFocus(id: string) {
  return `node:${id}`
}

/** 将传送门坐标编码为互动标识，与服务端匹配规则保持一致。 */
export function portalFocus(x: number, y: number) {
  return `portal:${Math.round(x)},${Math.round(y)}`
}

/** 和服务端 interact 的距离一致。再远就只会走过去，不会直接触发。 */
export function focusReach(focus?: string) {
  if (focus?.startsWith('npc:')) return 72
  if (focus?.startsWith('node:')) return 48
  if (focus?.startsWith('portal:')) return 58
  return 28
}

/** 走到这个距离就停下来互动，比判定距离稍近，避免卡在边界上。 */
export function arriveDistance(focus?: string) {
  if (!focus) return 28
  return Math.max(24, focusReach(focus) - 8)
}

/** 依次判断可交付、待交谈、可接取和进行中任务，生成 NPC 标记。 */
export function npcMark(npc: NpcView, profile: ProfileView | null): NpcMark {
  const quests = profile?.quests ?? []
  if (quests.some((quest) => sameTurnin(quest, npc) && isDone(quest))) return 'turnin'
  if (quests.some((quest) => (quest.objectives ?? []).some((goal) => goal.type === 'talk' && goal.target === npc.id && goal.current < goal.need))) return 'talk'
  if ((profile?.available ?? []).some((quest) => quest.giverId === npc.id || (!quest.giverId && quest.giver === npc.name))) return 'offer'
  if (quests.some((quest) => sameTurnin(quest, npc) && !isDone(quest))) return 'progress'
  return ''
}

/** 将 NPC 任务标记转为提示文字，没有任务时展示职能名称。 */
export function npcHint(npc: NpcView, profile: ProfileView | null) {
  const mark = npcMark(npc, profile)
  if (mark === 'turnin') return '可以交差'
  if (mark === 'talk') return '找你说话'
  if (mark === 'offer') return '有任务'
  if (mark === 'progress') return '任务还没做完'
  return npc.title || '对话'
}

/** 将 NPC 任务状态转换为按钮上的动作文字。 */
export function npcAction(mark: NpcMark) {
  if (mark === 'turnin') return '交差'
  if (mark === 'offer') return '接取'
  if (mark === 'talk') return '交谈'
  if (mark === 'progress') return '问问'
  return '对话'
}

/** 在各对象提示范围内选出离角色最近的互动对象。 */
export function closestSpot<T extends { x: number; y: number; notice: number }>(spots: T[], x: number, y: number): T | null {
  let best: T | null = null
  let bestDistance = Infinity
  for (const spot of spots) {
    const distance = Math.hypot(spot.x - x, spot.y - y)
    if (distance <= spot.notice && distance < bestDistance) {
      best = spot
      bestDistance = distance
    }
  }
  return best
}

/** 优先按 NPC ID 匹配交付目标，兼容仅提供名字的旧任务视图。 */
function sameTurnin(quest: QuestView, npc: NpcView) {
  if (quest.turninId) return quest.turninId === npc.id
  return quest.turnin === npc.name
}

/** 仅当任务存在目标且所有目标完成时显示可交付状态。 */
function isDone(quest: QuestView) {
  const goals = quest.objectives ?? []
  return goals.length > 0 && goals.every((goal) => goal.current >= goal.need)
}
