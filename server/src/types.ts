/** 旧版 TypeScript 迁移参考：配置与协议共享的字符串类型。默认服务启动执行 server/*.go。 */

/**
 * 全项目共用的枚举式字符串。
 * 用字符串而不是 TypeScript enum，是为了能直接写进 SQLite / JSON，客户端也能原样比较。
 */

export type WeaponType = 'sword_shield' | 'greatsword' | 'staff' | 'bow'
/** 物理与五种元素的协议标识，用于伤害加成和抗性索引。 */
export type Element = 'physical' | 'fire' | 'water' | 'thunder' | 'wind' | 'earth'
/** 六个装备部位的协议标识。 */
export type EquipSlot = 'weapon' | 'head' | 'chest' | 'gloves' | 'boots' | 'ring'
/** 装备从普通到传说的品质标识，决定面板倍率和词条数量。 */
export type Quality = 'white' | 'green' | 'blue' | 'purple' | 'gold'
/** 材料、消耗品、装备和任务物品的分类标识。 */
export type ItemKind = 'material' | 'consumable' | 'equipment' | 'quest'
/** 采集、制造和料理三条生活熟练的标识。 */
export type LifeSkill = 'gather' | 'craft' | 'cook'
/** 采集和派遣资源种类的标识。 */
export type NodeKind = 'herb' | 'wood' | 'fish' | 'ore' | 'thunder_ore' | 'explore' | 'escort'
/** 九种全服天气的协议标识。 */
export type WeatherId =
  | 'clear'
  | 'rain'
  | 'storm'
  | 'wind'
  | 'snow'
  | 'fog'
  | 'aurora'
  | 'blood_moon'
  | 'star_tide'
/** 按东八区真实时间划分的五种游戏时段。 */
export type DayPeriod = 'morning' | 'day' | 'dusk' | 'night' | 'late'
/** 临时增益能够影响的面板属性标识。 */
export type BuffStat = 'atkPct' | 'defPct' | 'critRate' | 'speedPct'

/** 已经掷出数值的装备词条，与物品实例一起持久化。 */
export interface AffixRoll {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: string
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 词条或增益作用的属性字段。 */
  stat: string
  /** 已确定的属性或增益数值。 */
  value: number
}

/** 装备实例额外数据，保存品质与已经生成的词条。 */
export interface ItemExtra {
  /** 装备品质，决定基础属性倍率和随机词条数量。 */
  quality: Quality
  /** 该实例已经生成的装备词条集合。 */
  affixes: AffixRoll[]
}

/** 装备基础属性块，作为面板叠加时的输入。 */
export interface StatBlock {
  /** 当前生命值或模板生命属性。 */
  hp: number
  /** 当前法力值或模板法力属性。 */
  mp: number
  /** 攻击属性，进入服务器伤害公式。 */
  atk: number
  /** 防御属性，通过防御常数换算为减伤。 */
  def: number
  /** 暴击概率，使用小数比例表示。 */
  critRate: number
  /** 暴击伤害倍率，例如1.5表示基础伤害的一点五倍。 */
  critDmg: number
  /** 世界移动速度或属性贡献的速度值。 */
  speed: number
}

/** 所有基础装备属性为零的公共初始值。 */
export const EMPTY_STATS: StatBlock = {
  hp: 0,
  mp: 0,
  atk: 0,
  def: 0,
  critRate: 0,
  critDmg: 0,
  speed: 0,
}
