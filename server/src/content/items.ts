/** 迁移前配置参考：材料、消耗品与装备。当前 Go 服务读取根目录 config/*.json。 */

/**
 * 物品模板。
 * 装备的随机词条不写在模板上，掉落时掷到玩家物品实例里。
 * 模板只描述“这是什么”，实例描述“这一件长什么样”。
 */

import type { EquipSlot, StatBlock, WeaponType } from '../types.ts'
import type { ItemTemplate } from './schema.ts'

/** 生成材料模板的公共工厂，复用堆叠与商店出售规则。 */
const mat = (id: string, name: string, sell: number, desc: string, bound = false): ItemTemplate => ({
  id, name, kind: 'material', stack: 999, sell, desc, bound,
})

/** 构造消耗品模板，合并默认类别、售价与具体物品效果。 */
const use = (
  id: string,
  name: string,
  sell: number,
  desc: string,
  effect: ItemTemplate['effect'],
): ItemTemplate => ({
  id, name, kind: 'consumable', stack: 99, sell, desc, effect,
})

/** 创建武器模板，共用部位、基础属性与武器类型的生成规则。 */
function weapon(
  id: string,
  name: string,
  weaponType: WeaponType,
  tier: number,
  sell: number,
  desc: string,
  base: Partial<StatBlock>,
): ItemTemplate {
  return { id, name, kind: 'equipment', stack: 1, sell, desc, slot: 'weapon', weaponType, tier, base }
}

/** 物品模板清单，包含材料、消耗品、武器和防具。 */
export const items: ItemTemplate[] = [
  mat('slime_gel', '史莱姆凝胶', 2, '软，黏，意外地能入药。'),
  mat('rat_tail', '鼠尾', 2, '森林鼠留下的尾巴。'),
  mat('mushroom_cap', '蘑菇盖', 3, '还在轻轻发光。'),
  mat('boar_hide', '野猪皮', 4, '厚，适合做甲。'),
  mat('wolf_fang', '狼牙', 5, '森林狼的犬齿。'),
  mat('herb_green', '翠叶草', 3, '星落镇外最常见的药草。'),
  mat('wood', '浮木', 2, '从岛缘漂来的木头。'),
  mat('fish_silver', '银鳞鱼', 4, '月光湖的常见渔获。'),
  mat('moon_dew', '月光露', 8, '夜里凝在湖草上的露水。'),
  mat('iron_ore', '铁矿', 6, '赤岩矿坑的基础矿。'),
  mat('copper_ore', '铜矿', 5, '比铁软，做饰品刚好。'),
  mat('silver_ore', '银矿', 14, '矿脉深处才有。'),
  mat('thunder_ore', '雷鸣矿', 18, '雷暴时更容易从岩层里露出来。'),
  mat('wind_feather', '风羽', 12, '风鸣高地的鸟羽，轻得不像话。'),
  mat('relic_fragment', '遗迹碎片', 16, '刻着断掉的契约文。'),
  mat('spore_king', '蘑菇王孢子', 20, '巨型蘑菇王留下的孢子。'),
  mat('serpent_scale', '湖蛇鳞', 20, '又冷又硬。'),
  mat('tyrant_horn', '暴君之角', 24, '矿坑暴君的角。'),
  mat('storm_core', '风暴核心', 30, '还在轻轻放电。'),
  mat('relic_heart', '遗迹之心', 40, '古代遗迹最后的搏动。'),
  mat('map_fragment', '神秘地图碎片', 0, '派遣时偶尔带回的残图。', true),

  use('capture_orb', '灵契球', 8, '把残血的灵伴从战斗里带回来。空的契约，不是抽卡。', {}),
  use('bread', '干粮', 2, '回复少量生命。', { healHp: 28 }),
  use('potion_hp', '史莱姆药剂', 6, '用凝胶熬成，回复生命。', { healHp: 70 }),
  use('potion_hp2', '星露药剂', 16, '回复大量生命。', { healHp: 160 }),
  use('potion_mp', '回蓝茶', 6, '回复法力。', { healMp: 40 }),
  use('food_atk', '炙狼排', 10, '短时间提高攻击。', { buff: { stat: 'atkPct', value: 0.12, ms: 300000 } }),
  use('food_def', '岩汤', 10, '短时间提高防御。', { buff: { stat: 'defPct', value: 0.12, ms: 300000 } }),
  use('food_explore', '夜行汤', 12, '喝下去再派遣，探索效率会好一些。', { buff: { stat: 'speedPct', value: 0.05, ms: 300000 } }),
  use('relic_egg', '遗迹蛋', 0, '使用后与古代精灵建立契约。', { grantPet: 'ancient_sprite' }),

  weapon('wpn_sword_1', '旅团练习剑盾', 'sword_shield', 1, 10, '星落镇发给新旅人的第一套兵器。', { atk: 6, def: 3, hp: 12 }),
  weapon('wpn_sword_2', '铁卫剑盾', 'sword_shield', 2, 40, '铁矿打造的剑盾，比练习用的沉得多。', { atk: 12, def: 6, hp: 24 }),
  weapon('wpn_sword_3', '翠叶守护', 'sword_shield', 3, 120, '盾面上嵌着翠叶森林的木纹。', { atk: 20, def: 11, hp: 42 }),
  weapon('wpn_sword_4', '雷鸣壁垒', 'sword_shield', 4, 280, '雷暴里锻过的盾，敲起来会响。', { atk: 30, def: 16, hp: 64 }),

  weapon('wpn_great_1', '学徒巨剑', 'greatsword', 1, 10, '挥起来会把自己带倒，但确实痛。', { atk: 10 }),
  weapon('wpn_great_2', '铁矿巨剑', 'greatsword', 2, 40, '霍特说这才叫剑。', { atk: 18 }),
  weapon('wpn_great_3', '狼牙巨剑', 'greatsword', 3, 120, '剑脊上嵌了一排狼牙。', { atk: 28, critRate: 0.02 }),
  weapon('wpn_great_4', '裂风巨剑', 'greatsword', 4, 280, '砍下去时风会迟到一步。', { atk: 40, critRate: 0.03 }),

  weapon('wpn_staff_1', '学徒法杖', 'staff', 1, 10, '杖头的水晶只是玻璃。', { atk: 9, mp: 16 }),
  weapon('wpn_staff_2', '菌光法杖', 'staff', 2, 40, '蘑菇盖晒干后居然能导魔。', { atk: 16, mp: 26 }),
  weapon('wpn_staff_3', '湖光法杖', 'staff', 3, 120, '夜里会自己亮一下。', { atk: 26, mp: 38 }),
  weapon('wpn_staff_4', '遗迹法杖', 'staff', 4, 280, '古代文明显然也怕黑。', { atk: 38, mp: 52 }),

  weapon('wpn_bow_1', '猎手短弓', 'bow', 1, 10, '能把史莱姆钉在树上。', { atk: 8, critRate: 0.03 }),
  weapon('wpn_bow_2', '藤木弓', 'bow', 2, 40, '翠叶森林的藤，晒干后很有弹性。', { atk: 15, critRate: 0.04 }),
  weapon('wpn_bow_3', '风羽弓', 'bow', 3, 120, '弓梢插着风羽。', { atk: 24, critRate: 0.05 }),
  weapon('wpn_bow_4', '雷鸣长弓', 'bow', 4, 280, '拉满时弦上有静电。', { atk: 36, critRate: 0.06 }),
]

/** 防具部位、等级与基础属性的生成计划。 */
const armorPlan: { slot: EquipSlot; names: [string, string, string, string]; base: Partial<StatBlock> }[] = [
  { slot: 'head', names: ['布帽', '铁盔', '星落兜', '浮岛冠'], base: { hp: 10, def: 2 } },
  { slot: 'chest', names: ['旅袍', '锁子甲', '星落衣', '浮岛袍'], base: { hp: 18, def: 3 } },
  { slot: 'gloves', names: ['布手套', '皮手套', '星落手套', '浮岛手套'], base: { atk: 2, critRate: 0.01 } },
  { slot: 'boots', names: ['草鞋', '皮靴', '星落靴', '浮岛靴'], base: { hp: 8, def: 2, speed: 4 } },
  { slot: 'ring', names: ['铜戒', '铁戒', '星落戒', '浮岛戒'], base: { atk: 3, critRate: 0.015, mp: 6 } },
]

for (const plan of armorPlan) {
  for (let tier = 1; tier <= 4; tier++) {
    const scale = tier
    const base: Partial<StatBlock> = {}
    for (const [key, value] of Object.entries(plan.base) as [keyof StatBlock, number][]) {
      base[key] = key === 'critRate' ? Math.round(value * (0.6 + tier * 0.35) * 1000) / 1000 : Math.round(value * scale)
    }
    items.push({
      id: `arm_${plan.slot}_${tier}`,
      name: plan.names[tier - 1]!,
      kind: 'equipment',
      stack: 1,
      sell: 8 * tier * tier,
      desc: `${plan.names[tier - 1]}。阶位 ${tier}。`,
      slot: plan.slot,
      tier,
      base,
    })
  }
}
