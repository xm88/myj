/** 旧版 TypeScript 迁移参考：在线角色、怪物、分线和队伍状态。默认服务启动执行 server/*.go。 */

/**
 * 在线世界里的运行时状态。
 * 数据库是掉线之后还在的东西；这里是“人正站在地图上”的那一份。
 * 金币、物品、任务以数据库为准，血量和坐标在线时以这里为准，定时写回。
 */

import type { WebSocket } from 'ws'
import type { BuffStat, Element, WeaponType } from './types.ts'

/** 在线临时增益，保存来源、受影响属性、数值和失效时间。 */
export interface LiveBuff {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 词条或增益作用的属性字段。 */
  stat: BuffStat
  /** 已确定的属性或增益数值。 */
  value: number
  /** 增益、天气或邀请的截止时间，使用 Unix 毫秒时间戳。 */
  until: number
}

/** 服务器叠加装备、熟练、灵伴和种族属性后的最终战斗面板。 */
export interface ComputedStats {
  /** 最终面板的生命上限。 */
  maxHp: number
  /** 最终面板的法力上限。 */
  maxMp: number
  /** 攻击属性，进入服务器伤害公式。 */
  atk: number
  /** 法术附加伤害。物理普攻不加，元素技能会加上。 */
  spell: number
  /** 防御属性，通过防御常数换算为减伤。 */
  def: number
  /** 暴击概率，使用小数比例表示。 */
  critRate: number
  /** 暴击伤害倍率，例如1.5表示基础伤害的一点五倍。 */
  critDmg: number
  /** 世界移动速度或属性贡献的速度值。 */
  speed: number
  /** 两次自动攻击之间的毫秒间隔。 */
  attackInterval: number
  /** 攻击或技能有效距离，结算时可能叠加目标碰撞半径。 */
  range: number
  /** 生命恢复或治疗增益比例，具体由所属模板定义。 */
  healPct: number
  /** 按元素分类的攻击加成，使用小数比例表示。 */
  elem: Record<Element, number>
  /** 按元素分类的抗性，进入服务器伤害公式。 */
  elemRes: Record<Element, number>
}

/** 发给客户端的伤害、恢复、掉落、闪避或文字事件。 */
export interface CombatEvent {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: number
  /** 物品、地图、技能事件或话题的类别标识。 */
  kind: 'dmg' | 'heal' | 'text' | 'loot' | 'miss'
  /** 展示给玩家的提示、聊天或任务目标文字。 */
  text?: string
  /** 最终伤害或治疗量，由服务器计算。 */
  amount?: number
  /** 该事件是否为暴击。 */
  crit?: boolean
  /** 世界横坐标，单位与地图尺寸保持一致。 */
  x?: number
  /** 世界纵坐标，单位与地图尺寸保持一致。 */
  y?: number
  /** 战斗事件对应的玩家或怪物实例 ID。 */
  entityId?: string
}

/** 旧版在线角色状态，血量和坐标在线时以此记录为准。 */
export interface LivePlayer {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 所属账号标识，区别于角色实例标识。 */
  userId: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 当前成长等级；具体上限由角色、熟练或灵伴规则决定。 */
  level: number
  /** 当前等级累计经验，升级后扣除所需经验。 */
  exp: number
  /** 所属或当前所在的地图模板标识。 */
  mapId: string
  /** 世界横坐标，单位与地图尺寸保持一致。 */
  x: number
  /** 世界纵坐标，单位与地图尺寸保持一致。 */
  y: number
  /** 当前生命值或模板生命属性。 */
  hp: number
  /** 当前法力值或模板法力属性。 */
  mp: number
  /** 金币数量，所有变更由服务器校验余额并持久化。 */
  gold: number
  /** 当前武器或装备对应的武器协议标识。 */
  weaponType: WeaponType
  /** 手上有没有武器。没武器时不能放武器技能。 */
  armed: boolean
  /** 四个武器技能栏 ID，顺序对应数字快捷键。 */
  loadout: string[]
  /** 角色当前公开分线或副本的索引键。 */
  instanceKey: string
  /** 客户端意图的横向方向，服务器限制在负一到一之间。 */
  inputX: number
  /** 客户端意图的纵向方向，服务器限制在负一到一之间。 */
  inputY: number
  /** 点击移动的目标横坐标，方向输入会取消目的地。 */
  destX: number | null
  /** 点击移动的目标纵坐标，方向输入会取消目的地。 */
  destY: number | null
  /** 当前攻击或仇恨目标的实例 ID，无目标时为空。 */
  targetId: string | null
  /** 助手模式下不挨打，也不自动攻击。 */
  safe: boolean
  /** 技能 ID 到下次可用毫秒时间的映射。 */
  cd: Record<string, number>
  /** 闪避或复活后无敌状态的截止毫秒时间。 */
  invulnUntil: number
  /** 尚可吸收的护盾伤害量。 */
  shield: number
  /** 临时增益列表，在线计算时只应用尚未过期的增益。 */
  buffs: LiveBuff[]
  /** 最近参与战斗的时间，用于脱战自动恢复判断。 */
  lastCombatAt: number
  /** 下一次允许自动攻击的服务器毫秒时间。 */
  attackAt: number
  /** 在线角色当前最终战斗面板，结算时读取此结果。 */
  stats: ComputedStats
  /** 当前角色下一条战斗事件使用的递增编号。 */
  eventSeq: number
  /** 本帧待发的战斗事件，快照发送后清空。 */
  events: CombatEvent[]
  /** 当前世界连接，断线时为空且角色进入安全状态。 */
  socket: WebSocket | null
  /** 最近发言的服务器时间，用于聊天频率限制。 */
  chatAt: number
  /** 最近断线的服务器时间，用于六十秒在线状态保留。 */
  disconnectAt: number
}

/** 分线中的怪物实例状态，包含出生点、仇恨、控制和重生时间。 */
export interface MonsterState {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 实例关联的配置模板标识。 */
  templateId: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 世界横坐标，单位与地图尺寸保持一致。 */
  x: number
  /** 世界纵坐标，单位与地图尺寸保持一致。 */
  y: number
  /** 怪物出生点横坐标，脱战时返回这里。 */
  homeX: number
  /** 怪物出生点纵坐标，脱战时返回这里。 */
  homeY: number
  /** 当前生命值或模板生命属性。 */
  hp: number
  /** 最终面板的生命上限。 */
  maxHp: number
  /** 当前实例是否存活，死亡实例不能继续被选作目标。 */
  alive: boolean
  /** 怪物预定重生时间，零表示没有定时重生。 */
  respawnAt: number
  /** 怪物死亡的毫秒时间，用于短暂显示死亡实体。 */
  deadAt: number
  /** 下一次允许自动攻击的服务器毫秒时间。 */
  attackAt: number
  /** 当前攻击或仇恨目标的实例 ID，无目标时为空。 */
  targetId: string | null
  /** 强制维持嘲讽目标的截止时间。 */
  tauntUntil: number
  /** 减速状态的截止时间。 */
  slowUntil: number
  /** 破甲状态的截止时间。 */
  shredUntil: number
  /** 易伤状态的截止时间。 */
  vulnerableUntil: number
  /** 已经出手的次数，用于触发周期性重击。 */
  heavyCount: number
}

/** 地图公开分线或队伍副本，独立维护怪物、角色、采集点与稀有刷新计时。 */
export interface Instance {
  /** 用于场景标记、实例索引或配置查找的键。 */
  key: string
  /** 所属或当前所在的地图模板标识。 */
  mapId: string
  /** 公开地图分线编号，独立副本使用零。 */
  line: number
  /** 该实例是否为队伍隔离的副本。 */
  dungeon: boolean
  /** 当前分线内怪物实例 ID 到状态的映射。 */
  monsters: Map<string, MonsterState>
  /** 当前分线内角色 ID 的集合。 */
  players: Set<string>
  /** 资源点下一次可采集的时间 */
  nodes: Map<string, number>
  /** 下一次检查稀有刷新的服务器毫秒时间。 */
  rareAt: number
}

/** 队伍临时状态，包含队长与成员 ID，决定副本的共享归属。 */
export interface Party {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 当前队伍队长标识，同时决定副本共享归属。 */
  leaderId: string
  /** 队伍成员角色 ID 列表。 */
  memberIds: string[]
}
