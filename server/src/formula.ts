/** 旧版 TypeScript 迁移参考：伤害、成长、种族与品质规则。默认服务启动执行 server/*.go。 */

/**
 * 战斗公式。
 * 设计约束（V0.1 战斗）：伤害只在服务器算，客户端只收到结果。
 *
 *   damage = attack × skillCoefficient × defenseReduction × elementalCoefficient × criticalCoefficient
 *   defenseReduction = 1 - Defense / (Defense + 500)
 *
 * 防御常数 500 让前期防御有意义，又不会把伤害减到 0。
 * 元素系数有 0.2 的下限，避免堆抗性把输出抹平。
 */

import type { Element } from './types.ts'

/** 伤害计算所需的攻击方、技能、防御方和暴击参数。 */
export interface DamageInput {
  /** 伤害公式的攻击输入值。 */
  attack: number
  /** 技能伤害相对于攻击的倍率。 */
  skillCoefficient: number
  /** 防守方防御数值。 */
  defense: number
  /** 攻击方该元素的额外伤害，0.12 表示 +12% */
  elementBonus: number
  /** 防守方该元素抗性 */
  elementResist: number
  /** 暴击概率，使用小数比例表示。 */
  critRate: number
  /** 暴击伤害倍率，例如1.5表示基础伤害的一点五倍。 */
  critDmg: number
  /** 是否强制本次攻击暴击。 */
  forceCrit?: boolean
  /** 易伤等额外承伤倍率，1 表示没有 */
  takenMul?: number
  /** 可注入的随机数函数，测试时用固定结果验证公式。 */
  rng?: () => number
}

/** 服务器计算后的最终伤害、暴击标记与防御减伤比例。 */
export interface DamageResult {
  /** 最终伤害或治疗量，由服务器计算。 */
  amount: number
  /** 该事件是否为暴击。 */
  crit: boolean
  /** 本次伤害计算采用的防御减伤比例。 */
  reducedByDefense: number
}

/** 将非负防御换算为减伤比例，防御五百时减伤为百分之五十。 */
export function defenseReduction(defense: number): number {
  const def = Math.max(0, defense)
  return def / (def + 500)
}

/** 按技能、防御、元素和暴击计算伤害；允许注入随机函数用于确定性测试。 */
export function computeDamage(input: DamageInput): DamageResult {
  const rng = input.rng ?? Math.random
  const reduction = defenseReduction(input.defense)
  const elemental = Math.max(0.2, 1 + input.elementBonus - input.elementResist)
  const crit = input.forceCrit || rng() < input.critRate
  const critCoeff = crit ? input.critDmg : 1
  const taken = input.takenMul ?? 1
  const raw =
    input.attack * input.skillCoefficient * (1 - reduction) * elemental * critCoeff * taken
  return {
    amount: Math.max(1, Math.round(raw)),
    crit,
    reducedByDefense: reduction,
  }
}

/** 升级所需经验。指数略大于 1，让 1–10 级一个晚上能到，30 级仍要持续玩。 */
export function expToNext(level: number): number {
  return Math.round(100 * Math.pow(Math.max(1, level), 1.35))
}

/** 计算武器熟练升到下一级所需经验。 */
export function masteryExpToNext(level: number): number {
  return 40 + level * 30
}

/** 计算生活技能升到下一级所需经验。 */
export function lifeExpToNext(level: number): number {
  return 30 + level * 25
}

/** 计算灵伴升到下一级所需经验。 */
export function petExpToNext(level: number): number {
  return 24 + level * 18
}

/** 品质决定基础属性倍率，以及随机词条数量。金装词条不多于紫装，强度差在倍率上。 */
export const QUALITY_TABLE: Record<
  string,
  { name: string; mul: number; affixes: number }
> = {
  white: { name: '普通', mul: 1, affixes: 0 },
  green: { name: '优秀', mul: 1.12, affixes: 1 },
  blue: { name: '精良', mul: 1.28, affixes: 2 },
  purple: { name: '史诗', mul: 1.5, affixes: 3 },
  gold: { name: '传说', mul: 1.75, affixes: 3 },
}

/** 武器类型到中文名称的映射。 */
export const WEAPON_LABEL: Record<string, string> = {
  sword_shield: '剑盾',
  greatsword: '巨剑',
  staff: '法杖',
  bow: '弓',
}

/** 游戏时段到中文名称的映射。 */
export const PERIOD_LABEL: Record<string, string> = {
  morning: '早晨',
  day: '白天',
  dusk: '黄昏',
  night: '夜晚',
  late: '深夜',
}

/**
 * 现实时间映射到游戏时段，使用东八区。
 * 服务器和所有客户端看到同一时段，NPC、稀有灵伴才有“世界在运转”的感觉。
 */
export function dayPeriod(now = new Date()): 'morning' | 'day' | 'dusk' | 'night' | 'late' {
  const hour = (now.getUTCHours() + 8) % 24
  if (hour >= 5 && hour < 8) return 'morning'
  if (hour >= 8 && hour < 17) return 'day'
  if (hour >= 17 && hour < 19) return 'dusk'
  if (hour >= 19 && hour < 23) return 'night'
  return 'late'
}

/** 每级 5 点潜力，和梦幻西游一样，1 级就能分配。 */
export const POINTS_PER_LEVEL = 5

/** 五项潜力的固定协议字段集合。 */
export const ATTRIBUTE_KEYS = ['vit', 'mag', 'str', 'agi', 'end'] as const
/** 可自行分配的五项属性标识。 */
export type AttributeKey = (typeof ATTRIBUTE_KEYS)[number]

/** 角色自行分配或成长合成的五项属性数值。 */
export interface Attributes {
  /** 体力属性值，主要影响生命，同时贡献少量灵力与速度。 */
  vit: number
  /** 魔力属性值，影响法力与灵力。 */
  mag: number
  /** 力量属性值，影响物理伤害，同时贡献灵力与速度。 */
  str: number
  /** 敏捷属性值，影响速度以及自动攻击间隔。 */
  agi: number
  /** 耐力属性值，影响防御，同时贡献少量灵力与速度。 */
  end: number
}

/**
 * 梦幻西游三族。
 * 每级另外自动长 5 点属性，族与族的分配不同；潜力仍是每级 5 点，自己加。
 * 一点属性换成面板的比例也按族来：魔族血和伤害更高，仙族魔法和防御更高，人族居中。
 * 速度落到出手和走路，因为这版没有回合条。
 */
export type RaceId = 'human' | 'demon' | 'immortal'

/** 各属性换算成战斗面板的比例，以及速度和攻击间隔的规则。 */
interface RaceRate {
  /** 每点体力增加的生命上限。 */
  vitHp: number
  /** 每点体力增加的灵力。 */
  vitSpell: number
  /** 每点体力增加的属性速度值。 */
  vitSpeed: number
  /** 每点魔力增加的法力上限。 */
  magMp: number
  /** 每点魔力增加的灵力。 */
  magSpell: number
  /** 每点力量增加的物理攻击。 */
  strAtk: number
  /** 每点力量增加的灵力。 */
  strSpell: number
  /** 每点力量增加的属性速度值。 */
  strSpeed: number
  /** 每点耐力增加的防御。 */
  endDef: number
  /** 每点耐力增加的灵力。 */
  endSpell: number
  /** 每点耐力增加的属性速度值。 */
  endSpeed: number
  /** 每点敏捷增加的属性速度值。 */
  agiSpeed: number
  /** 每点属性速度减少的攻击间隔毫秒数。 */
  speedToInterval: number
  /** 每点属性速度增加的世界移动速度。 */
  speedToMove: number
  /** 自动攻击间隔的毫秒下限，避免敏捷无限提高出手频率。 */
  minInterval: number
}

/** 三族共用的灵力和速度换算参数。 */
const sharedRate = {
  vitSpell: 0.3,
  vitSpeed: 0.1,
  magSpell: 0.7,
  strSpell: 0.4,
  strSpeed: 0.1,
  endSpell: 0.2,
  endSpeed: 0.1,
  agiSpeed: 0.7,
  speedToInterval: 4,
  speedToMove: 0.8,
  minInterval: 650,
}

/** 种族名称、简介、每级自动成长与属性换算比例。 */
export interface RaceDef {
  /** 当前记录或模板的唯一标识，用于协议引用与数据库关联。 */
  id: RaceId
  /** 用于客户端显示的中文名称。 */
  name: string
  /** 界面显示的简短玩法或成长说明。 */
  blurb: string
  /** 该种族每级自动增加的五项属性。 */
  growth: Attributes
  /** 该种族每点属性转换为面板的比例。 */
  rate: RaceRate
}

/** 种族的成长分配和属性换算表，创建与面板计算共用。 */
export const RACES: Record<RaceId, RaceDef> = {
  human: {
    id: 'human',
    name: '人族',
    blurb: '五项一起长，没有短板。',
    growth: { vit: 1, mag: 1, str: 1, end: 1, agi: 1 },
    rate: { ...sharedRate, vitHp: 5, magMp: 3, strAtk: 0.66, endDef: 1.5 },
  },
  demon: {
    id: 'demon',
    name: '魔族',
    blurb: '体力和力量长得快，耐力几乎不长。',
    growth: { vit: 2, mag: 0, str: 2, end: 0, agi: 1 },
    rate: { ...sharedRate, vitHp: 6, magMp: 2.5, strAtk: 0.76, endDef: 1.4 },
  },
  immortal: {
    id: 'immortal',
    name: '仙族',
    blurb: '魔力和耐力长得快，体力几乎不长。',
    growth: { vit: 0, mag: 2, str: 0, end: 2, agi: 1 },
    rate: { ...sharedRate, vitHp: 4.5, magMp: 3.5, strAtk: 0.57, endDef: 1.6 },
  },
}

/** 识别三种开放种族，无效或缺失值回退到人族。 */
export function parseRace(raw: string | null | undefined): RaceId {
  if (raw === 'demon' || raw === 'immortal' || raw === 'human') return raw
  return 'human'
}

/** 输出角色创建所需的种族简介和成长文字。 */
export function listRaces() {
  return (Object.values(RACES) as RaceDef[]).map((race) => ({
    id: race.id,
    name: race.name,
    blurb: race.blurb,
    growth: growthLine(race),
  }))
}

/** 等级带来的种族成长，加上玩家自己点的潜力。 */
export function totalAttributes(level: number, spent: Attributes, raceId: RaceId): Attributes {
  const growth = RACES[parseRace(raceId)].growth
  const steps = Math.max(0, Math.floor(level))
  return {
    vit: spent.vit + growth.vit * steps,
    mag: spent.mag + growth.mag * steps,
    str: spent.str + growth.str * steps,
    end: spent.end + growth.end * steps,
    agi: spent.agi + growth.agi * steps,
  }
}

/** 把种族自动成长转换为中文展示文本，只显示有增长的属性。 */
function growthLine(race: RaceDef) {
  const name: Record<AttributeKey, string> = { vit: '体力', mag: '魔力', str: '力量', end: '耐力', agi: '敏捷' }
  return ATTRIBUTE_KEYS.filter((key) => race.growth[key] > 0)
    .map((key) => `${name[key]} +${race.growth[key]}`)
    .join('，')
}

/** 创建五项均为零的潜力分配记录。 */
export function emptyAttributes(): Attributes {
  return { vit: 0, mag: 0, str: 0, agi: 0, end: 0 }
}

/** 读取潜力点存档，异常或负值按未分配处理。 */
export function parseAttributes(raw: string | null | undefined): Attributes {
  const attrs = emptyAttributes()
  try {
    const parsed = JSON.parse(raw || '{}') as Partial<Record<AttributeKey, unknown>>
    for (const key of ATTRIBUTE_KEYS) {
      const n = Math.floor(Number(parsed[key] ?? 0))
      attrs[key] = Number.isFinite(n) && n > 0 ? n : 0
    }
  } catch {
    /* 坏掉的存档当成没加过点 */
  }
  return attrs
}

/** 每级提供五点可分配潜力，一级即可分配。 */
export function attributeBudget(level: number): number {
  return Math.max(0, Math.floor(level)) * POINTS_PER_LEVEL
}

/** 计算已分配潜力总数，用于剩余点数与超额校验。 */
export function spentAttributes(attrs: Attributes): number {
  return ATTRIBUTE_KEYS.reduce((sum, key) => sum + attrs[key], 0)
}

/** 按种族属性换算比例计算生命、法力、伤害、防御、灵力与速度贡献。 */
export function dreamContribution(attrs: Attributes, raceId: RaceId = 'human') {
  const rate = RACES[parseRace(raceId)].rate
  const spell = attrs.vit * rate.vitSpell + attrs.mag * rate.magSpell + attrs.str * rate.strSpell + attrs.end * rate.endSpell
  const speed = attrs.vit * rate.vitSpeed + attrs.str * rate.strSpeed + attrs.end * rate.endSpeed + attrs.agi * rate.agiSpeed
  return {
    hp: attrs.vit * rate.vitHp,
    mp: attrs.mag * rate.magMp,
    atk: attrs.str * rate.strAtk,
    def: attrs.end * rate.endDef,
    spell,
    speed,
  }
}

/** 把属性贡献应用到角色面板，并限制最短攻击间隔。 */
export function applyAttributeBonuses(
  stats: { maxHp: number; maxMp: number; atk: number; def: number; spell: number; attackInterval: number; speed: number },
  attrs: Attributes,
  raceId: RaceId = 'human',
) {
  const got = dreamContribution(attrs, raceId)
  const rate = RACES[parseRace(raceId)].rate
  stats.maxHp += got.hp
  stats.maxMp += got.mp
  stats.atk += got.atk
  stats.def += got.def
  stats.spell += got.spell
  stats.attackInterval = Math.max(rate.minInterval, stats.attackInterval - got.speed * rate.speedToInterval)
  stats.speed += got.speed * rate.speedToMove
}

/** 将面板贡献按一位小数舍入，整数不显示多余的小数位。 */
function shown(n: number) {
  const rounded = Math.round(n * 10) / 10
  return Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(1)
}

/** 生成五项属性的数值、每点效果和累计贡献说明。 */
export function attributeRows(attrs: Attributes, raceId: RaceId = 'human') {
  const rate = RACES[parseRace(raceId)].rate
  return [
    {
      key: 'vit' as const,
      name: '体力',
      hint: `每点生命 +${shown(rate.vitHp)}，灵力 +${shown(rate.vitSpell)}，速度 +${shown(rate.vitSpeed)}`,
      value: attrs.vit,
      gain: attrs.vit ? `生命 +${shown(attrs.vit * rate.vitHp)}，灵力 +${shown(attrs.vit * rate.vitSpell)}，速度 +${shown(attrs.vit * rate.vitSpeed)}` : '',
    },
    {
      key: 'mag' as const,
      name: '魔力',
      hint: `每点法力 +${shown(rate.magMp)}，灵力 +${shown(rate.magSpell)}，不加速度`,
      value: attrs.mag,
      gain: attrs.mag ? `法力 +${shown(attrs.mag * rate.magMp)}，灵力 +${shown(attrs.mag * rate.magSpell)}` : '',
    },
    {
      key: 'str' as const,
      name: '力量',
      hint: `每点物理伤害 +${shown(rate.strAtk)}，灵力 +${shown(rate.strSpell)}，速度 +${shown(rate.strSpeed)}`,
      value: attrs.str,
      gain: attrs.str ? `伤害 +${shown(attrs.str * rate.strAtk)}，灵力 +${shown(attrs.str * rate.strSpell)}，速度 +${shown(attrs.str * rate.strSpeed)}` : '',
    },
    {
      key: 'agi' as const,
      name: '敏捷',
      hint: `每点速度 +${shown(rate.agiSpeed)}，出手和走路更快，不加灵力`,
      value: attrs.agi,
      gain: attrs.agi ? `速度 +${shown(attrs.agi * rate.agiSpeed)}` : '',
    },
    {
      key: 'end' as const,
      name: '耐力',
      hint: `每点防御 +${shown(rate.endDef)}，灵力 +${shown(rate.endSpell)}，速度 +${shown(rate.endSpeed)}`,
      value: attrs.end,
      gain: attrs.end ? `防御 +${shown(attrs.end * rate.endDef)}，灵力 +${shown(attrs.end * rate.endSpell)}，速度 +${shown(attrs.end * rate.endSpeed)}` : '',
    },
  ]
}

/** 将元素协议标识转换为中文名称。 */
export function elementLabel(el: Element): string {
  const map: Record<Element, string> = {
    physical: '物理',
    fire: '火',
    water: '水',
    thunder: '雷',
    wind: '风',
    earth: '土',
  }
  return map[el]
}
