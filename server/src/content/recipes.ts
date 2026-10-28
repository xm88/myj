/** 迁移前配置参考：制造与料理配方。当前 Go 服务读取根目录 config/*.json。 */

/**
 * 配方。第一版只开三条生活技能：采集、制造、料理。
 * 采集没有配方，站在资源点上互动即可；制造和料理走这里。
 */

import type { RecipeDef } from './schema.ts'

/** 制造和料理配方，材料、费用与产物均由服务器读取。 */
export const recipes: RecipeDef[] = [
  { id: 'recipe_potion_slime', name: '史莱姆药剂', life: 'cook', skill: 1, gold: 5, inputs: [{ itemId: 'slime_gel', count: 3 }], output: { itemId: 'potion_hp', count: 1 } },
  { id: 'recipe_potion_star', name: '星露药剂', life: 'cook', skill: 3, gold: 15, inputs: [{ itemId: 'herb_green', count: 4 }, { itemId: 'moon_dew', count: 1 }], output: { itemId: 'potion_hp2', count: 1 } },
  { id: 'recipe_tea', name: '回蓝茶', life: 'cook', skill: 1, gold: 4, inputs: [{ itemId: 'herb_green', count: 2 }], output: { itemId: 'potion_mp', count: 1 } },
  { id: 'recipe_food_atk', name: '炙狼排', life: 'cook', skill: 2, gold: 8, inputs: [{ itemId: 'wolf_fang', count: 1 }, { itemId: 'herb_green', count: 2 }], output: { itemId: 'food_atk', count: 1 } },
  { id: 'recipe_food_def', name: '岩汤', life: 'cook', skill: 2, gold: 8, inputs: [{ itemId: 'iron_ore', count: 1 }, { itemId: 'herb_green', count: 2 }], output: { itemId: 'food_def', count: 1 } },
  { id: 'recipe_food_explore', name: '夜行汤', life: 'cook', skill: 2, gold: 8, inputs: [{ itemId: 'moon_dew', count: 1 }, { itemId: 'fish_silver', count: 1 }], output: { itemId: 'food_explore', count: 1 } },
  { id: 'recipe_bread', name: '干粮', life: 'cook', skill: 1, gold: 2, inputs: [{ itemId: 'wood', count: 1 }, { itemId: 'herb_green', count: 1 }], output: { itemId: 'bread', count: 2 } },

  { id: 'recipe_iron_sword', name: '铁矿巨剑', life: 'craft', skill: 1, gold: 20, inputs: [{ itemId: 'iron_ore', count: 8 }, { itemId: 'wood', count: 3 }], output: { itemId: 'wpn_great_2', count: 1, quality: 'green' } },
  { id: 'recipe_iron_shield', name: '铁卫剑盾', life: 'craft', skill: 1, gold: 20, inputs: [{ itemId: 'iron_ore', count: 6 }, { itemId: 'boar_hide', count: 2 }], output: { itemId: 'wpn_sword_2', count: 1, quality: 'green' } },
  { id: 'recipe_vine_bow', name: '藤木弓', life: 'craft', skill: 1, gold: 18, inputs: [{ itemId: 'wood', count: 8 }, { itemId: 'wolf_fang', count: 1 }], output: { itemId: 'wpn_bow_2', count: 1, quality: 'green' } },
  { id: 'recipe_shroom_staff', name: '菌光法杖', life: 'craft', skill: 1, gold: 18, inputs: [{ itemId: 'wood', count: 4 }, { itemId: 'mushroom_cap', count: 6 }], output: { itemId: 'wpn_staff_2', count: 1, quality: 'green' } },
  { id: 'recipe_chest', name: '锁子甲', life: 'craft', skill: 2, gold: 24, inputs: [{ itemId: 'iron_ore', count: 6 }, { itemId: 'boar_hide', count: 4 }], output: { itemId: 'arm_chest_2', count: 1, quality: 'green' } },
  { id: 'recipe_wolf_blade', name: '狼牙巨剑', life: 'craft', skill: 4, gold: 40, inputs: [{ itemId: 'iron_ore', count: 10 }, { itemId: 'wolf_fang', count: 6 }, { itemId: 'silver_ore', count: 1 }], output: { itemId: 'wpn_great_3', count: 1, quality: 'blue' } },
  { id: 'recipe_lake_staff', name: '湖光法杖', life: 'craft', skill: 4, gold: 40, inputs: [{ itemId: 'wood', count: 8 }, { itemId: 'moon_dew', count: 4 }, { itemId: 'serpent_scale', count: 1 }], output: { itemId: 'wpn_staff_3', count: 1, quality: 'blue' } },
  { id: 'recipe_thunder_bow', name: '雷鸣长弓', life: 'craft', skill: 6, gold: 80, inputs: [{ itemId: 'wind_feather', count: 6 }, { itemId: 'thunder_ore', count: 8 }], output: { itemId: 'wpn_bow_4', count: 1, quality: 'blue' } },
  { id: 'recipe_relic_staff', name: '遗迹法杖', life: 'craft', skill: 7, gold: 100, inputs: [{ itemId: 'relic_fragment', count: 8 }, { itemId: 'storm_core', count: 1 }, { itemId: 'wood', count: 4 }], output: { itemId: 'wpn_staff_4', count: 1, quality: 'purple' } },
]
