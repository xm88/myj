/** 迁移前配置参考：所有配置模板的数据结构。当前 Go 服务读取根目录 config/*.json。 */

/**
 * 配置表的形状。
 * 策划向的数据全部放在 content/ 下，服务端逻辑只按 id 查表，不写死怪物血量或任务文案。
 */

import type {
  BuffStat,
  DayPeriod,
  Element,
  EquipSlot,
  ItemKind,
  NodeKind,
  Quality,
  StatBlock,
  WeaponType,
  WeatherId,
} from '../types.ts'

/** 技能模板，定义武器归属、消耗、冷却、攻击倍率和特殊效果。 */
export interface SkillDef {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 客户端展示的内容说明。 */
  desc: string
  /** 技能所归属的武器类型，灵伴技能可使用独立标识。 */
  weapon?: WeaponType
  /** 只有该灵伴在主战位时才能放到灵伴技上 */
  pet?: string
  /** 相对于基础攻击的技能伤害倍率。 */
  coefficient: number
  /** 技能或怪物攻击所属元素。 */
  element: Element
  /** 技能冷却时长，单位为毫秒。 */
  cooldown: number
  /** 当前法力值或模板法力属性。 */
  mp: number
  /** 攻击或技能有效距离，结算时可能叠加目标碰撞半径。 */
  range: number
  /** 是否对范围内多个目标结算技能效果。 */
  aoe?: boolean
  /** 生命恢复或治疗增益比例，具体由所属模板定义。 */
  healPct?: number
  /** 技能或消耗品提供的临时增益配置。 */
  buff?: { stat: BuffStat; value: number; ms: number }
  /** 嘲讽持续毫秒数。 */
  tauntMs?: number
  /** 减速持续毫秒数。 */
  slowMs?: number
  /** 破甲持续毫秒数。 */
  shredMs?: number
  /** 易伤持续毫秒数。 */
  vulnerableMs?: number
  /** 技能给予的固定护盾吸收量。 */
  shieldFlat?: number
  /** 是否强制本次攻击暴击。 */
  forceCrit?: boolean
}

/** 物品模板，定义堆叠、交易、装备部位、基础属性和消耗品效果。 */
export interface ItemTemplate {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 物品、地图、技能事件或话题的类别标识。 */
  kind: ItemKind
  /** 模板声明的堆叠数量上限；当前库存实现合并普通材料，尚未按此字段拆堆。 */
  stack: number
  /** 卖给商人的单价 */
  sell: number
  /** 客户端展示的内容说明。 */
  desc: string
  /** 绑定物品限制交易或仓库存放。 */
  bound?: boolean
  /** 使用消耗品时应用的恢复、增益或灵伴奖励。 */
  effect?: {
    healHp?: number
    healMp?: number
    buff?: { stat: BuffStat; value: number; ms: number }
    /** 使用后获得一只灵伴（遗迹蛋这类一次性契约） */
    grantPet?: string
  }
  /** 装备所占部位。 */
  slot?: EquipSlot
  /** 当前武器或装备对应的武器协议标识。 */
  weaponType?: WeaponType
  /** 物品装备或产出所属的配置等级档位。 */
  tier?: number
  /** 模板提供的基础面板属性，装备品质在结算时乘以倍率。 */
  base?: Partial<StatBlock>
}

/** 灵伴模板，定义主战光环、技能、派遣专长及天气加成。 */
export interface PetDef {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 灵伴稀有度，特殊稀有契约可能产生世界新闻。 */
  rarity: 'common' | 'uncommon' | 'rare' | 'epic'
  /** 灵伴战斗特性的中文描述。 */
  battle: string
  /** 灵伴派遣特性的中文描述。 */
  work: string
  /** 客户端展示的内容说明。 */
  desc: string
  /** 灵伴主动技能对应的技能模板 ID。 */
  skillId: string
  /** 设为主战时应用的面板光环。 */
  aura: {
    critRate?: number
    atkPct?: number
    def?: number
    hp?: number
    healPct?: number
    elem?: Partial<Record<Element, number>>
  }
  /** 派遣效率加成。0.3 = +30%。versatile 会加到所有地点上。 */
  dispatch: Partial<Record<NodeKind | 'versatile', number>>
  /** 天气限制或天气专属效率加成。 */
  weather?: Partial<Record<WeatherId, number>>
}

/** 单项掉落规则，包含概率、数量范围与可选装备品质权重。 */
export interface DropDef {
  /** 奖励或材料对应的物品模板 ID。 */
  itemId: string
  /** 掉落或刷新概率，使用小数比例表示。 */
  chance: number
  /** 随机数量或词条数值的下限。 */
  min: number
  /** 随机数量、词条数值或场内数量的上限。 */
  max: number
  /** 装备品质，决定基础属性倍率和随机词条数量。 */
  quality?: Quality
  /** 装备品质与抽取权重的列表。 */
  qualities?: { quality: Quality; weight: number }[]
  /** 指定天气提高此项掉落概率，服务器仍按概率上限处理。 */
  weatherBoost?: WeatherId
}

/** 服务器怪物模板，包含面板、行为范围、重生、掉落与契约规则。 */
export interface MonsterDef {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 当前成长等级；具体上限由角色、熟练或灵伴规则决定。 */
  level: number
  /** 怪物类别或公会职位，具体含义由所属结构决定。 */
  role: 'normal' | 'elite' | 'boss' | 'world'
  /** 当前生命值或模板生命属性。 */
  hp: number
  /** 攻击属性，进入服务器伤害公式。 */
  atk: number
  /** 防御属性，通过防御常数换算为减伤。 */
  def: number
  /** 当前等级累计经验，升级后扣除所需经验。 */
  exp: number
  /** 击杀后随机金币奖励的下限。 */
  goldMin: number
  /** 击杀后随机金币奖励的上限。 */
  goldMax: number
  /** 攻击或技能有效距离，结算时可能叠加目标碰撞半径。 */
  range: number
  /** 怪物自动攻击的毫秒间隔。 */
  interval: number
  /** 主动寻找玩家的仇恨距离。 */
  aggro: number
  /** 怪物追击与回位的距离限制。 */
  leash: number
  /** 技能或怪物攻击所属元素。 */
  element: Element
  /** 按元素分类的抗性，进入服务器伤害公式。 */
  elemRes?: Partial<Record<Element, number>>
  /** 夜晚与深夜是否提高该怪物攻击。 */
  nightPower?: boolean
  /** 可获得或可契约的灵伴模板标识。 */
  petId?: string
  /** 目标残血后契约成功的基础概率。 */
  captureBase?: number
  /** 对应天气对契约成功率的加成。 */
  captureWeather?: Partial<Record<WeatherId, number>>
  /** 允许契约的游戏时段列表。 */
  captureTime?: DayPeriod[]
  /** 怪物标记的显示色。 */
  tint: string
  /** 实体碰撞与范围判定使用的半径。 */
  radius: number
  /** 死亡或采集后到再次可用的毫秒间隔。 */
  respawnMs: number
  /** 怪物被击败后逐项随机判断的掉落配置。 */
  drops: DropDef[]
  /** 每 N 次攻击改为重击 */
  heavyEvery?: number
}

/** 地图稀有刷新规则，受天气、时段、概率和数量上限共同约束。 */
export interface RareSpawn {
  /** 当前规则或任务目标引用的怪物模板 ID。 */
  monsterId: string
  /** 世界横坐标，单位与地图尺寸保持一致。 */
  x: number
  /** 世界纵坐标，单位与地图尺寸保持一致。 */
  y: number
  /** 每次检查的出现概率 */
  chance: number
  /** 随机数量、词条数值或场内数量的上限。 */
  max: number
  /** 天气限制或天气专属效率加成。 */
  weather?: WeatherId[]
  /** 允许稀有刷新的游戏时段列表，未配置时不限制时段。 */
  time?: DayPeriod[]
}

/** 地图模板，包含尺寸、入口、阻挡、刷新、资源点和传送门。 */
export interface MapDef {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 客户端展示的内容说明。 */
  desc: string
  /** 物品、地图、技能事件或话题的类别标识。 */
  kind: 'town' | 'wild' | 'dungeon'
  /** 地图建议的最低等级，用于内容分区与显示。 */
  levelMin: number
  /** 地图建议的最高等级，用于内容分区与显示。 */
  levelMax: number
  /** 地图或派遣路线所需的最低角色等级。 */
  unlockLevel: number
  /** 地图世界坐标宽度。 */
  width: number
  /** 地图世界坐标高度。 */
  height: number
  /** 进入地图或倒地后使用的入口坐标。 */
  spawn: { x: number; y: number }
  /** 地图缺失素材时使用的背景颜色与显示风格。 */
  theme: { ground: string; ground2: string; accent: string; path: string }
  /** 影响服务器碰撞的矩形阻挡区域。 */
  blocks: { x: number; y: number; w: number; h: number; kind: string }[]
  /** 仅用于客户端绘制的场景装饰。 */
  scenery: { x: number; y: number; kind: string }[]
  /** 地图初始普通怪物的模板与位置列表。 */
  spawns: { monsterId: string; x: number; y: number }[]
  /** 天气和时段限定的稀有刷新规则。 */
  rares: RareSpawn[]
  /** 通往其他地图的传送门配置。 */
  portals: { x: number; y: number; to: string; label: string }[]
  /** 地图采集点配置或实例采集点的冷却状态。 */
  nodes: { id: string; kind: NodeKind; itemId: string; x: number; y: number; respawnMs: number; name: string }[]
}

/** NPC 模板，包含位置、对话、商店与服务面板入口。 */
export interface NpcDef {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** NPC 职能或对话窗口的显示标题。 */
  title: string
  /** 所属或当前所在的地图模板标识。 */
  mapId: string
  /** 世界横坐标，单位与地图尺寸保持一致。 */
  x: number
  /** 世界纵坐标，单位与地图尺寸保持一致。 */
  y: number
  /** NPC 无特定任务话题时可使用的对话文本。 */
  lines: string[]
  /** shop / craft / market / dispatch / guild / bank / travel / skills */
  panels: string[]
  /** NPC 售卖的物品模板与单价列表。 */
  shop?: { itemId: string; price: number }[]
}

/** 任务目标的联合类型，不同事件携带各自需要的模板标识和数量。 */
export type Objective =
  | { type: 'talk'; npcId: string; text: string }
  | { type: 'kill'; monsterId: string; count: number; text: string }
  | { type: 'kill_any'; count: number; text: string }
  | { type: 'collect'; itemId: string; count: number; text: string }
  | { type: 'capture'; petId: string; count: number; text: string }
  | { type: 'craft'; recipeId: string; count: number; text: string }
  | { type: 'dispatch_start'; count: number; text: string }
  | { type: 'dispatch_claim'; count: number; text: string }
  | { type: 'market_list'; count: number; text: string }
  | { type: 'market_buy'; count: number; text: string }
  | { type: 'enter'; mapId: string; text: string }
  | { type: 'gather'; nodeKind: string; count: number; text: string }
  | { type: 'join_guild'; text: string }
  | { type: 'donate'; text: string }
  | { type: 'reach_level'; level: number; text: string }
  | { type: 'world_boss'; text: string }
  | { type: 'buy_shop'; text: string }
  | { type: 'bank'; text: string }
  | { type: 'equip_quality'; quality: Quality; text: string }

/** 任务模板，定义接取条件、目标、奖励、交付 NPC 与后续任务。 */
export interface QuestDef {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 客户端展示的内容说明。 */
  desc: string
  /** 物品、地图、技能事件或话题的类别标识。 */
  kind: 'main' | 'side'
  /** 发放任务的 NPC 模板 ID。 */
  giver: string
  /** 接收交付任务的 NPC ID 或其显示名称。 */
  turnin: string
  /** 接取当前任务前必须交付的任务 ID 列表。 */
  prereq: string[]
  /** 允许接取任务的最低等级。 */
  minLevel: number
  /** 接取任务要求已持久化解锁的角色标记。 */
  flag?: string
  /** 任务需要完成的全部目标列表。 */
  objectives: Objective[]
  /** 任务交付时由服务器发放的经验、金币、物品和灵伴。 */
  rewards: {
    exp: number
    gold: number
    items?: { itemId: string; count: number; quality?: Quality }[]
    petId?: string
  }
  /** 交付后可自动接续的任务 ID，消息队列结构中则表示下一条消息读取方法。 */
  next?: string
}

/** 制造或料理配方，定义熟练门槛、金币材料消耗和产物。 */
export interface RecipeDef {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 配方所属制造或料理熟练。 */
  life: 'craft' | 'cook'
  /** 执行配方要求的生活技能等级。 */
  skill: number
  /** 金币数量，所有变更由服务器校验余额并持久化。 */
  gold: number
  /** 配方消耗的材料模板与数量。 */
  inputs: { itemId: string; count: number }[]
  /** 制造完成后获得的产物模板、数量与品质。 */
  output: { itemId: string; count: number; quality?: Quality }
}

/** 灵伴派遣路线，定义解锁等级、资源种类与每小时基准产出。 */
export interface DispatchNode {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 所属或当前所在的地图模板标识。 */
  mapId: string
  /** 物品、地图、技能事件或话题的类别标识。 */
  kind: NodeKind
  /** 地图或派遣路线所需的最低角色等级。 */
  unlockLevel: number
  /** 派遣路线每小时的基准资源产量。 */
  basePerHour: { itemId: string; count: number }[]
  /** 界面显示的简短玩法或成长说明。 */
  blurb: string
}

/** 派遣时长规则，定义持续毫秒数、产量倍率和探索事件概率。 */
export interface DurationDef {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 持续时长，单位为毫秒。 */
  ms: number
  /** 用于品质属性或派遣产量的倍率。 */
  mul: number
  /** 派遣发生隐藏探索事件的基础概率。 */
  eventChance: number
}

/** 全服天气模板，包含协议标识、中文名称和效果说明。 */
export interface WeatherDef {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: WeatherId
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 客户端展示的内容说明。 */
  desc: string
}

/** 装备随机词条模板，包含目标属性与可随机生成的数值范围。 */
export interface AffixDef {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 词条或增益作用的属性字段。 */
  stat: string
  /** 随机数量或词条数值的下限。 */
  min: number
  /** 随机数量、词条数值或场内数量的上限。 */
  max: number
}
