/** 任务导航：根据服务器任务进度生成可高亮、可执行的地图目标和下一步操作。 */

/** 当前任务该去哪。地图高亮和「点任务就走」共用这一份。 */

import { nodeFocus, npcFocus, portalFocus } from './npc'

/** 网页任务目标视图，包含服务器进度、需求和可定位目标。 */
export interface QuestGoal {
  /** 展示给玩家的提示、聊天或任务目标文字。 */
  text: string
  /** 服务器确认的当前任务目标进度。 */
  current: number
  /** 完成当前任务目标要求的进度。 */
  need: number
  /** 服务器任务目标的事件类型，用于匹配引导与 NPC 提示。 */
  type?: string
  /** 任务引导可定位的模板、地图或资源种类标识。 */
  target?: string
}

/** 角色任务记录或视图，包含任务标识与当前进度信息。 */
export interface QuestRow {
  /** 任务标识。点追踪时用它定位这一条，而不是总拿列表里的第一条。 */
  id?: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 接收交付任务的 NPC ID 或其显示名称。 */
  turnin?: string
  /** 需要交付任务的 NPC 模板 ID，优先于显示名称匹配。 */
  turninId?: string
  /** 任务需要完成的全部目标列表。 */
  objectives?: QuestGoal[]
}

/** 任务引导生成的场景标记及下一步移动、互动或战斗操作。 */
export interface Guide {
  /** 用于场景标记、实例索引或配置查找的键。 */
  key: string
  /** 任务引导选中的怪物实例 ID。 */
  fightId?: string
  /** 世界横坐标，单位与地图尺寸保持一致。 */
  x?: number
  /** 世界纵坐标，单位与地图尺寸保持一致。 */
  y?: number
  /** 抵达目的地后执行的动作，当前使用互动动作。 */
  act?: 'interact'
  /** 精确互动标识，避免就近触发其他 NPC、资源点或门。 */
  focus?: string
  /** 引导是否以契约而非攻击作为最终操作。 */
  capture?: boolean
}

/** 优先选择仍有未完成目标的任务，否则将首个任务标记为可交付。 */
export function currentQuest(quests: QuestRow[]) {
  for (const quest of quests) {
    const goal = quest.objectives?.find((item) => item.current < item.need)
    if (goal) return { quest, goal, ready: false as const }
  }
  const first = quests[0]
  if (first) return { quest: first, goal: null, ready: true as const }
  return null
}

/** 将当前任务步骤转换为状态栏里的简短进度文字。 */
export function describeQuest(quests: QuestRow[]) {
  const step = currentQuest(quests)
  if (!step) return ''
  if (!step.ready && step.goal) return `${step.quest.name} ${step.goal.current}/${step.goal.need} ${step.goal.text}`
  return `${step.quest.name} · 交给${step.quest.turnin || '委托人'}`
}

/** 任务追踪行：右侧列表一次显示手上的几条，点哪条就去哪条。 */
export interface QuestTrack {
  /** 任务标识，没有标识时退回名称。 */
  id: string
  /** 任务名称。 */
  name: string
  /** 主线或支线。 */
  kind?: string
  /** 目标已经做完，等着交付。 */
  ready: boolean
  /** 一行进度。 */
  line: string
}

/** 把进行中的任务收成追踪列表。主线排在前面，钉上的那条再提前，最多四条。 */
export function questTracks(quests: Array<QuestRow & { kind?: string }>, pin?: string) {
  const ordered = [...quests].sort((a, b) => Number(b.kind === 'main') - Number(a.kind === 'main'))
  if (pin) {
    const index = ordered.findIndex((quest) => (quest.id || quest.name) === pin)
    if (index > 0) {
      const [row] = ordered.splice(index, 1)
      if (row) ordered.unshift(row)
    }
  }
  return ordered.slice(0, 4).map((quest): QuestTrack => {
    const goal = quest.objectives?.find((item) => item.current < item.need)
    return {
      id: quest.id || quest.name,
      name: quest.name,
      kind: quest.kind,
      ready: !goal,
      line: goal ? `${goal.current}/${goal.need} ${goal.text}` : `交给${quest.turnin || '委托人'}`,
    }
  })
}

/** 任务引导需要的最小地图信息，包含门和怪物刷新来源。 */
interface MapLike {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 通往其他地图的传送门配置。 */
  portals?: { x: number; y: number; to: string; label: string }[]
  /** 地图初始普通怪物的模板与位置列表。 */
  spawns?: { monsterId: string }[]
  /** 天气和时段限定的稀有刷新规则。 */
  rares?: { monsterId: string }[]
}

/** 任务引导需要的最小 NPC 信息，包含地图与世界坐标。 */
interface NpcLike {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 所属或当前所在的地图模板标识。 */
  mapId: string
  /** 世界横坐标，单位与地图尺寸保持一致。 */
  x: number
  /** 世界纵坐标，单位与地图尺寸保持一致。 */
  y: number
}

/** 任务引导用的怪物与灵伴模板对应关系。 */
interface MonsterLike {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 可获得或可契约的灵伴模板标识。 */
  petId?: string | null
}

/** 任务引导当前可以选择的怪物实例及其位置。 */
interface PackLike {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 实例关联的配置模板标识。 */
  templateId: string
  /** 世界横坐标，单位与地图尺寸保持一致。 */
  x: number
  /** 世界纵坐标，单位与地图尺寸保持一致。 */
  y: number
}

/** 查找通往目标地图的门，没有直达门时优先提示回星落镇。 */
function doorTo(map: MapLike | undefined, mapId: string) {
  const doors = map?.portals ?? []
  return doors.find((portal) => portal.to === mapId) ?? doors.find((portal) => portal.to === 'town_starfall')
}

/** 从普通与稀有刷新配置中查找目标怪物所在地图。 */
function mapOfMonster(maps: MapLike[], monsterId: string) {
  return maps.find((map) => map.spawns?.some((spawn) => spawn.monsterId === monsterId) || map.rares?.some((spawn) => spawn.monsterId === monsterId))
}

/** 把传送门转换为可高亮、可点击前往的任务引导目标。 */
function atDoor(door: { x: number; y: number; label: string }): Guide {
  return { key: `p:${door.label}`, x: door.x, y: door.y, act: 'interact', focus: portalFocus(door.x, door.y) }
}

/** 同地图时引导到 NPC，跨地图时先引导到可用传送门。 */
function atNpc(npc: NpcLike, here: MapLike | undefined, mapId: string): Guide | null {
  if (npc.mapId === mapId) return { key: npc.id, x: npc.x, y: npc.y, act: 'interact', focus: npcFocus(npc.id) }
  const door = doorTo(here, npc.mapId)
  return door ? atDoor(door) : null
}

/** 返回地图上要高亮的标记，以及点任务时该做的那一步。 */
export function questGuide(
  quests: QuestRow[],
  config: { maps?: MapLike[]; npcs?: NpcLike[]; monsters?: MonsterLike[] } | null,
  mapId: string,
  packs: PackLike[],
): Guide | null {
  const step = currentQuest(quests)
  if (!step) return null
  return guideQuest(step.quest, config, mapId, packs)
}

/** 为指定的一条任务生成下一步。追踪列表和状态栏共用这套寻路。 */
export function guideQuest(
  quest: QuestRow,
  config: { maps?: MapLike[]; npcs?: NpcLike[]; monsters?: MonsterLike[] } | null,
  mapId: string,
  packs: PackLike[],
): Guide | null {
  const open = quest.objectives?.find((item) => item.current < item.need)
  const step = open ? { quest, goal: open, ready: false as const } : { quest, goal: null, ready: true as const }
  if (!config) return null
  const here = config.maps?.find((map) => map.id === mapId)
  const npcs = config.npcs ?? []

  if (step.ready) {
    const npc = npcs.find((item) => item.id === step.quest.turninId) ?? npcs.find((item) => item.name === step.quest.turnin)
    if (!npc) return null
    return atNpc(npc, here, mapId)
  }

  const goal = step.goal
  if (!goal?.type) return null

  if (goal.type === 'kill' || goal.type === 'capture') {
    const monsterId = goal.type === 'kill'
      ? goal.target
      : config.monsters?.find((monster) => monster.petId === goal.target)?.id
    if (!monsterId) return null
    const pack = packs.find((item) => item.templateId === monsterId)
    if (pack) return { key: pack.templateId, fightId: pack.id, capture: goal.type === 'capture' }
    const home = mapOfMonster(config.maps ?? [], monsterId)
    const door = home ? doorTo(here, home.id) : undefined
    return door ? atDoor(door) : null
  }

  if (goal.type === 'enter' && goal.target) {
    if (goal.target === mapId) return null
    const door = doorTo(here, goal.target)
    return door ? atDoor(door) : null
  }

  if (goal.type === 'talk' && goal.target) {
    const npc = npcs.find((item) => item.id === goal.target)
    if (!npc) return null
    return atNpc(npc, here, mapId)
  }

  if (goal.type === 'gather' && goal.target) {
    const node = (here as { nodes?: { id: string; kind: string; x: number; y: number }[] } | undefined)?.nodes?.find((item) => item.kind === goal.target)
    if (node) return { key: node.id, x: node.x, y: node.y, act: 'interact', focus: nodeFocus(node.id) }
  }

  return null
}
