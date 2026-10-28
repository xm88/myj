/** 迁移前配置参考：天气、词条、派遣与全局常数。当前 Go 服务读取根目录 config/*.json。 */

/**
 * 天气、词条、派遣时长和派遣点。
 * 天气是全服一份，由世界时钟推进，不只是滤镜。
 */

import type { AffixDef, DispatchNode, DurationDef, WeatherDef } from './schema.ts'

/** 全服天气的显示信息和协议标识。 */
export const weathers: WeatherDef[] = [
  { id: 'clear', name: '晴', desc: '火狐更常出现。大多数野外维持原样。' },
  { id: 'rain', name: '雨', desc: '翠叶森林里的森林猫更容易现身，也更好契约。' },
  { id: 'storm', name: '雷暴', desc: '雷属性怪物更强，雷鸣矿更容易掉，雷猫出现率上升。' },
  { id: 'wind', name: '大风', desc: '风鸣高地的风灵更活跃。' },
  { id: 'snow', name: '雪', desc: '视野安静。第一版没有雪地专属怪，但世界时钟仍会走到这里。' },
  { id: 'fog', name: '迷雾', desc: '雾狐会出现在翠叶森林。' },
  { id: 'aurora', name: '极光', desc: '很罕见的天象。第一版主要作为世界新闻出现。' },
  { id: 'blood_moon', name: '血月', desc: '怪物稍微更凶。幼年天空鲸在这种夜里显得更大。' },
  { id: 'star_tide', name: '星潮', desc: '据说和金色史莱姆有关。出现率会略微提高。' },
]

/** 装备词条的属性种类与随机数值范围。 */
export const affixes: AffixDef[] = [
  { id: 'atk', name: '攻击', stat: 'atk', min: 2, max: 9 },
  { id: 'hp', name: '生命', stat: 'hp', min: 8, max: 40 },
  { id: 'def', name: '防御', stat: 'def', min: 2, max: 8 },
  { id: 'mp', name: '法力', stat: 'mp', min: 4, max: 16 },
  { id: 'crit', name: '暴击', stat: 'critRate', min: 0.01, max: 0.04 },
  { id: 'critd', name: '暴击伤害', stat: 'critDmg', min: 0.05, max: 0.15 },
  { id: 'fire', name: '火属性伤害', stat: 'elem_fire', min: 0.04, max: 0.12 },
  { id: 'water', name: '水属性伤害', stat: 'elem_water', min: 0.04, max: 0.12 },
  { id: 'thunder', name: '雷属性伤害', stat: 'elem_thunder', min: 0.04, max: 0.12 },
  { id: 'wind', name: '风属性伤害', stat: 'elem_wind', min: 0.04, max: 0.12 },
  { id: 'earth', name: '土属性伤害', stat: 'elem_earth', min: 0.04, max: 0.12 },
]

/** 各派遣时长的持续时间、产量倍率和事件概率。 */
export const durations: DurationDef[] = [
  // 短途侦察是封闭测试用的 1 分钟档，让“派遣 → 领取 → 发现事件”不用干等半小时。
  // 正式节奏仍是 30 分钟、2 小时、4 小时、8 小时。短途的产量系数只有 0.08。
  { id: 'scout', name: '短途侦察', ms: 60_000, mul: 0.08, eventChance: 0.55 },
  { id: 'half', name: '半小时', ms: 30 * 60_000, mul: 0.5, eventChance: 0.12 },
  { id: 'two', name: '两小时', ms: 2 * 3600_000, mul: 1, eventChance: 0.18 },
  { id: 'four', name: '四小时', ms: 4 * 3600_000, mul: 2.2, eventChance: 0.28 },
  { id: 'eight', name: '八小时', ms: 8 * 3600_000, mul: 4.8, eventChance: 0.4 },
]

/** 可选择的派遣路线及其解锁门槛与资源产出。 */
export const dispatchNodes: DispatchNode[] = [
  { id: 'node_green_herb', name: '翠叶药圃', mapId: 'wild_greenleaf', kind: 'herb', unlockLevel: 1, blurb: '采药。森林猫和花灵会快不少。', basePerHour: [{ itemId: 'herb_green', count: 28 }] },
  { id: 'node_green_wood', name: '林缘浮木', mapId: 'wild_greenleaf', kind: 'wood', unlockLevel: 1, blurb: '捡木头。', basePerHour: [{ itemId: 'wood', count: 24 }] },
  { id: 'node_lake_fish', name: '月光渔场', mapId: 'wild_moonlake', kind: 'fish', unlockLevel: 10, blurb: '钓鱼。水獭和锦鲤是这里的专家。', basePerHour: [{ itemId: 'fish_silver', count: 18 }, { itemId: 'moon_dew', count: 4 }] },
  { id: 'node_mine', name: '赤岩矿道', mapId: 'wild_redrock', kind: 'ore', unlockLevel: 15, blurb: '挖矿。矿洞鼠的主场。', basePerHour: [{ itemId: 'iron_ore', count: 22 }, { itemId: 'copper_ore', count: 10 }, { itemId: 'silver_ore', count: 2 }] },
  { id: 'node_thunder', name: '雷鸣矿脉', mapId: 'wild_thunder', kind: 'thunder_ore', unlockLevel: 20, blurb: '雷暴天派遣雷猫更合适。', basePerHour: [{ itemId: 'thunder_ore', count: 12 }] },
  { id: 'node_wind', name: '风鸣侦察', mapId: 'wild_windhigh', kind: 'explore', unlockLevel: 25, blurb: '探索高地，偶尔带回风羽。', basePerHour: [{ itemId: 'wind_feather', count: 10 }] },
  { id: 'node_escort', name: '星落商路', mapId: 'town_starfall', kind: 'escort', unlockLevel: 5, blurb: '护送一小段商路，狗骑士和狼崽更稳。', basePerHour: [{ itemId: 'bread', count: 6 }, { itemId: 'wood', count: 8 }] },
]

/** 全服天气每隔这么久换一次。测试服偏短，让人能看到天气玩法。 */
export const WEATHER_MS = 8 * 60_000

/** 一张公开地图分线最多容纳的人数。超过就开下一线。 */
export const LINE_CAP = 20

/** 拍卖税。税直接销毁，用来压通胀。 */
export const MARKET_TAX = 0.05

/** 背包最多容纳的物品记录数，堆叠材料共占一格。 */
export const BAG_CAP = 60
/** 仓库最多容纳的物品记录数。 */
export const BANK_CAP = 80
/** 创建公会需要支付的金币数量。 */
export const GUILD_CREATE_COST = 200
/** 新建角色统一获得的初始金币。 */
export const STARTER_GOLD = 80
/** 新建角色统一获得的灵契球数量。 */
export const STARTER_ORBS = 3
