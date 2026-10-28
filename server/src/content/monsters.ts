/** 迁移前配置参考：怪物数值、掉落与契约。当前 Go 服务读取根目录 config/*.json。 */

/**
 * 怪物表。
 * 普通怪、精英、首领、世界首领的血量和经验由 pack() 按等级拉开，
 * 具体掉落和能否契约写在每只怪自己身上。
 */

import type { Element } from '../types.ts'
import type { DropDef, MonsterDef } from './schema.ts'

/** 按等级和普通、精英、首领分类生成基础生命、攻击、防御、经验与金币范围。 */
function pack(level: number, role: MonsterDef['role']) {
  const hpN = 42 + level * 16
  const mul = role === 'elite' ? 2.6 : role === 'boss' ? 7.2 : role === 'world' ? 16 : 1
  const atkMul = role === 'elite' ? 1.22 : role === 'boss' ? 1.5 : role === 'world' ? 1.65 : 1
  return {
    level,
    role,
    hp: Math.round(hpN * mul),
    atk: Math.round((5 + level * 1.55) * atkMul),
    def: Math.round((1 + level * 1.25) * (role === 'normal' ? 1 : 1.15)),
    exp: Math.round((14 + level * 7) * (role === 'elite' ? 3 : role === 'boss' ? 8 : role === 'world' ? 10 : 1)),
    goldMin: Math.round((2 + level) * (role === 'boss' || role === 'world' ? 4 : 1)),
    goldMax: Math.round((5 + level * 2) * (role === 'boss' || role === 'world' ? 4 : 1)),
  }
}

/** 按通用基础参数生成怪物模板，再应用该怪物的额外配置。 */
function mob(
  id: string,
  name: string,
  level: number,
  role: MonsterDef['role'],
  tint: string,
  extra: Partial<MonsterDef> & { drops?: DropDef[] },
): MonsterDef {
  const base = pack(level, role)
  return {
    id,
    name,
    ...base,
    range: 46,
    interval: role === 'boss' || role === 'world' ? 1600 : 1400,
    aggro: 150,
    leash: 380,
    element: 'physical' as Element,
    tint,
    radius: role === 'world' ? 34 : role === 'boss' ? 26 : 16,
    respawnMs: role === 'world' ? 300000 : role === 'boss' ? 180000 : role === 'elite' ? 90000 : 28000,
    drops: [],
    heavyEvery: role === 'boss' || role === 'world' ? 4 : undefined,
    ...extra,
  }
}

/** 怪物装备掉落的公共配置，用于复用品质与概率规则。 */
const gear = (itemId: string, chance: number): DropDef => ({
  itemId,
  chance,
  min: 1,
  max: 1,
  qualities: [
    { quality: 'white', weight: 50 },
    { quality: 'green', weight: 32 },
    { quality: 'blue', weight: 14 },
    { quality: 'purple', weight: 4 },
  ],
})

/** 普通、精英、首领和可契约生物的服务器模板清单。 */
export const monsters: MonsterDef[] = [
  mob('slime', '史莱姆', 1, 'normal', '#6cbf73', {
    petId: 'slime_pet', captureBase: 0.55, aggro: 120,
    drops: [{ itemId: 'slime_gel', chance: 0.9, min: 1, max: 2 }, gear('wpn_sword_1', 0.03)],
  }),
  mob('forest_rat', '森林鼠', 2, 'normal', '#8a6a45', {
    drops: [{ itemId: 'rat_tail', chance: 0.7, min: 1, max: 1 }],
  }),
  mob('shroom', '蘑菇怪', 4, 'normal', '#c45b8a', {
    petId: 'shroom_spirit', captureBase: 0.42, element: 'earth',
    drops: [{ itemId: 'mushroom_cap', chance: 0.75, min: 1, max: 2 }, { itemId: 'herb_green', chance: 0.3, min: 1, max: 1 }],
  }),
  mob('boar', '野猪', 6, 'normal', '#a56b45', {
    petId: 'boar_cub', captureBase: 0.4, hp: 150,
    drops: [{ itemId: 'boar_hide', chance: 0.7, min: 1, max: 1 }, gear('arm_chest_1', 0.05)],
  }),
  mob('wolf', '森林狼', 8, 'normal', '#6d7380', {
    petId: 'wolf_pup', captureBase: 0.38,
    drops: [{ itemId: 'wolf_fang', chance: 0.65, min: 1, max: 1 }, gear('wpn_bow_1', 0.04)],
  }),
  mob('forest_cat', '森林猫', 3, 'normal', '#d7a15a', {
    petId: 'forest_cat', captureBase: 0.72, captureWeather: { rain: 0.2 }, aggro: 90, respawnMs: 40000,
    drops: [{ itemId: 'herb_green', chance: 0.4, min: 1, max: 1 }],
  }),
  mob('elite_boar', '狂暴野猪', 9, 'elite', '#c45a3a', {
    drops: [{ itemId: 'boar_hide', chance: 1, min: 2, max: 3 }, gear('arm_chest_2', 0.35), gear('wpn_great_2', 0.2)],
  }),
  mob('elite_wolf', '狼王', 10, 'elite', '#4d5566', {
    drops: [{ itemId: 'wolf_fang', chance: 1, min: 2, max: 3 }, gear('wpn_bow_2', 0.28)],
  }),
  mob('boss_mushroom', '巨型蘑菇王', 10, 'boss', '#9b3d68', {
    element: 'earth',
    drops: [{ itemId: 'spore_king', chance: 1, min: 1, max: 1 }, { itemId: 'mushroom_cap', chance: 1, min: 3, max: 5 }, gear('wpn_staff_2', 0.4), gear('arm_head_2', 0.35)],
  }),
  mob('gold_slime', '金色史莱姆', 6, 'normal', '#e2c14d', {
    petId: 'gold_slime', captureBase: 0.2, radius: 18, respawnMs: 0,
    drops: [{ itemId: 'slime_gel', chance: 1, min: 3, max: 5 }, gear('arm_ring_2', 0.5)],
    goldMin: 40, goldMax: 80, exp: 80,
  }),
  mob('flower_bud', '花苞灵', 5, 'normal', '#e7a0c4', {
    petId: 'flower_spirit', captureBase: 0.34, element: 'earth', respawnMs: 0, aggro: 80,
    drops: [{ itemId: 'herb_green', chance: 1, min: 2, max: 3 }],
  }),
  mob('fire_fox', '火狐', 7, 'normal', '#e07030', {
    petId: 'fire_fox', captureBase: 0.3, captureWeather: { clear: 0.1 }, element: 'fire', respawnMs: 0,
    drops: [{ itemId: 'herb_green', chance: 0.5, min: 1, max: 1 }],
  }),
  mob('mist_fox', '雾狐', 8, 'normal', '#b7c4ce', {
    petId: 'mist_fox', captureBase: 0.28, captureWeather: { fog: 0.22 }, respawnMs: 0, aggro: 80,
    drops: [{ itemId: 'moon_dew', chance: 0.5, min: 1, max: 1 }],
  }),

  mob('pond_slime', '湖史莱姆', 10, 'normal', '#4aa3a8', {
    element: 'water',
    drops: [{ itemId: 'slime_gel', chance: 0.8, min: 1, max: 2 }, { itemId: 'moon_dew', chance: 0.25, min: 1, max: 1 }],
  }),
  mob('crab', '钳蟹', 12, 'normal', '#c4553c', {
    drops: [{ itemId: 'fish_silver', chance: 0.35, min: 1, max: 1 }],
  }),
  mob('heron', '夜鹭', 13, 'normal', '#d9e2ea', {
    nightPower: true, range: 150, element: 'wind',
    drops: [{ itemId: 'wind_feather', chance: 0.25, min: 1, max: 1 }],
  }),
  mob('fishman', '鱼人斥候', 14, 'normal', '#3d7ea6', {
    element: 'water',
    drops: [{ itemId: 'fish_silver', chance: 0.55, min: 1, max: 1 }, { itemId: 'moon_dew', chance: 0.3, min: 1, max: 1 }],
  }),
  mob('otter', '水獭', 11, 'normal', '#8a5a3a', {
    petId: 'otter', captureBase: 0.5, aggro: 100,
    drops: [{ itemId: 'fish_silver', chance: 0.4, min: 1, max: 1 }],
  }),
  mob('elite_crab', '巨钳蟹', 14, 'elite', '#a33b32', {
    drops: [{ itemId: 'fish_silver', chance: 1, min: 2, max: 4 }, gear('arm_gloves_2', 0.3)],
  }),
  mob('elite_koi', '巨锦鲤', 15, 'elite', '#e07a4a', {
    petId: 'koi', captureBase: 0.36, element: 'water', range: 40,
    drops: [{ itemId: 'fish_silver', chance: 1, min: 2, max: 3 }, { itemId: 'moon_dew', chance: 0.8, min: 1, max: 2 }, gear('arm_ring_3', 0.25)],
  }),
  mob('moon_cat', '月光猫', 12, 'normal', '#f2e7b0', {
    petId: 'moon_cat', captureBase: 0.22, captureTime: ['night', 'late'], captureWeather: { clear: 0.08, fog: 0.08 },
    respawnMs: 0, aggro: 70,
    drops: [{ itemId: 'moon_dew', chance: 1, min: 1, max: 2 }],
  }),
  mob('boss_serpent', '湖蛇', 15, 'boss', '#2f6d78', {
    element: 'water', range: 70,
    drops: [{ itemId: 'serpent_scale', chance: 1, min: 1, max: 1 }, gear('wpn_staff_3', 0.35), gear('arm_boots_2', 0.4)],
  }),

  mob('bat', '矿蝠', 16, 'normal', '#5c4d73', {
    petId: 'bat', captureBase: 0.48, range: 130, element: 'wind',
    drops: [{ itemId: 'copper_ore', chance: 0.45, min: 1, max: 1 }],
  }),
  mob('lizard', '岩蜥', 17, 'normal', '#c46a32', {
    petId: 'ember_lizard', captureBase: 0.4, element: 'fire',
    drops: [{ itemId: 'iron_ore', chance: 0.55, min: 1, max: 2 }],
  }),
  mob('ore_golem', '矿石傀儡', 18, 'normal', '#7d848c', {
    element: 'earth',
    drops: [{ itemId: 'iron_ore', chance: 0.8, min: 1, max: 2 }, { itemId: 'copper_ore', chance: 0.45, min: 1, max: 1 }],
  }),
  mob('mine_rat', '矿洞鼠', 16, 'normal', '#6b5344', {
    petId: 'mine_rat', captureBase: 0.58, aggro: 100,
    drops: [{ itemId: 'iron_ore', chance: 0.7, min: 1, max: 2 }, { itemId: 'copper_ore', chance: 0.4, min: 1, max: 1 }],
  }),
  mob('elite_golem', '精英岩精', 19, 'elite', '#596068', {
    petId: 'rock_golem', captureBase: 0.26, element: 'earth',
    drops: [{ itemId: 'iron_ore', chance: 1, min: 3, max: 5 }, { itemId: 'silver_ore', chance: 0.45, min: 1, max: 1 }, gear('arm_chest_3', 0.3)],
  }),
  mob('boss_tyrant', '矿坑暴君', 20, 'boss', '#8a3e2f', {
    element: 'earth',
    drops: [{ itemId: 'tyrant_horn', chance: 1, min: 1, max: 1 }, { itemId: 'silver_ore', chance: 1, min: 1, max: 2 }, gear('wpn_great_3', 0.32), gear('wpn_sword_3', 0.32)],
  }),

  mob('spark', '电浆史莱姆', 21, 'normal', '#7ec8f0', {
    element: 'thunder',
    drops: [{ itemId: 'slime_gel', chance: 0.7, min: 1, max: 2 }, { itemId: 'thunder_ore', chance: 0.35, min: 1, max: 1, weatherBoost: 'storm' }],
  }),
  mob('thunder_bird', '雷鸟', 22, 'normal', '#f0d36a', {
    element: 'thunder', range: 160,
    drops: [{ itemId: 'wind_feather', chance: 0.45, min: 1, max: 1 }, { itemId: 'thunder_ore', chance: 0.3, min: 1, max: 1, weatherBoost: 'storm' }],
  }),
  mob('storm_wolf', '风暴狼', 23, 'normal', '#6aa0c8', {
    element: 'thunder',
    drops: [{ itemId: 'wolf_fang', chance: 0.6, min: 1, max: 1 }, { itemId: 'thunder_ore', chance: 0.25, min: 1, max: 1, weatherBoost: 'storm' }],
  }),
  mob('thunder_cat', '雷猫', 22, 'normal', '#f2e27a', {
    petId: 'thunder_cat', captureBase: 0.26, captureWeather: { storm: 0.25 }, element: 'thunder', respawnMs: 0, aggro: 90,
    drops: [{ itemId: 'thunder_ore', chance: 0.8, min: 1, max: 1 }],
  }),
  mob('elite_thunder', '雷鸣兽', 24, 'elite', '#4f8fd6', {
    element: 'thunder',
    drops: [{ itemId: 'thunder_ore', chance: 1, min: 2, max: 4, weatherBoost: 'storm' }, gear('wpn_bow_4', 0.12), gear('wpn_staff_4', 0.12)],
  }),
  mob('boss_storm', '风暴核心', 25, 'boss', '#8fd0ff', {
    element: 'thunder', range: 180,
    drops: [{ itemId: 'storm_core', chance: 1, min: 1, max: 1 }, { itemId: 'thunder_ore', chance: 1, min: 2, max: 4 }, gear('wpn_staff_4', 0.28), gear('arm_ring_4', 0.22)],
  }),

  mob('hare', '风原兔', 26, 'normal', '#d9cbb8', {
    drops: [{ itemId: 'wind_feather', chance: 0.2, min: 1, max: 1 }],
  }),
  mob('raptor', '崖鹫', 27, 'normal', '#8a5a44', {
    element: 'wind', range: 150,
    drops: [{ itemId: 'wind_feather', chance: 0.65, min: 1, max: 2 }],
  }),
  mob('gale_sprite', '风灵', 28, 'normal', '#b7e0c8', {
    petId: 'storm_bird', captureBase: 0.3, captureWeather: { wind: 0.15 }, element: 'wind', respawnMs: 0,
    drops: [{ itemId: 'wind_feather', chance: 1, min: 1, max: 2 }],
  }),
  mob('elite_raptor', '风鸣巨鹫', 29, 'elite', '#6d4030', {
    element: 'wind', range: 170,
    drops: [{ itemId: 'wind_feather', chance: 1, min: 3, max: 5 }, gear('wpn_bow_4', 0.22), gear('arm_boots_4', 0.2)],
  }),
  mob('boss_wind', '风原霸主', 30, 'boss', '#d7e7c8', {
    element: 'wind', range: 160,
    drops: [{ itemId: 'wind_feather', chance: 1, min: 4, max: 6 }, gear('wpn_bow_4', 0.3), gear('arm_head_4', 0.25)],
  }),
  mob('world_whale', '幼年天空鲸', 30, 'world', '#8eb4d4', {
    element: 'wind', range: 200, leash: 900, aggro: 280, interval: 2000,
    drops: [{ itemId: 'wind_feather', chance: 1, min: 6, max: 10 }, { itemId: 'storm_core', chance: 0.5, min: 1, max: 1 }, gear('arm_chest_4', 0.45)],
    goldMin: 200, goldMax: 360, exp: 1200,
  }),

  mob('relic_guard', '遗迹守卫', 26, 'normal', '#c8b48a', {
    element: 'earth',
    drops: [{ itemId: 'relic_fragment', chance: 0.7, min: 1, max: 1 }],
  }),
  mob('relic_mage', '遗迹法师', 28, 'normal', '#7a6bb5', {
    element: 'thunder', range: 180,
    drops: [{ itemId: 'relic_fragment', chance: 0.75, min: 1, max: 2 }],
  }),
  mob('elite_relic', '遗迹守卫长', 29, 'elite', '#e6d7a8', {
    element: 'earth',
    drops: [{ itemId: 'relic_fragment', chance: 1, min: 2, max: 3 }, gear('wpn_sword_4', 0.2)],
  }),
  mob('boss_relic', '遗迹之心', 30, 'boss', '#e6c56a', {
    element: 'thunder', range: 190,
    drops: [
      { itemId: 'relic_heart', chance: 1, min: 1, max: 1 },
      { itemId: 'relic_fragment', chance: 1, min: 3, max: 5 },
      { itemId: 'relic_egg', chance: 0.35, min: 1, max: 1 },
      gear('wpn_staff_4', 0.3),
    ],
  }),
]
