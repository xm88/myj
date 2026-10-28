/** 迁移前配置参考：灵伴光环、技能与派遣专长。当前 Go 服务读取根目录 config/*.json。 */

/**
 * 20 只灵伴。
 * 每只都同时有战斗光环和派遣专长。战力最高的不一定是你想带去下线的那只。
 * 获取方式写在 desc 里，逻辑由怪物的 petId、任务奖励或物品 grantPet 触发。
 */

import type { PetDef } from './schema.ts'

/** 灵伴模板清单，主战光环与派遣专长分别配置。 */
export const pets: PetDef[] = [
  { id: 'forest_cat', name: '森林猫', rarity: 'uncommon', battle: '暴击', work: '采药', skillId: 'pet_cat', desc: '翠叶森林的常客。下雨时更愿意现身，也更好契约。', aura: { critRate: 0.08 }, dispatch: { herb: 0.28, wood: 0.1 }, weather: { rain: 0.2 } },
  { id: 'mine_rat', name: '矿洞鼠', rarity: 'common', battle: '普通', work: '挖矿', skillId: 'pet_rat', desc: '打架一般，下矿是好手。赤岩矿坑里就能遇到。', aura: { def: 3 }, dispatch: { ore: 0.35, thunder_ore: 0.15 } },
  { id: 'fire_fox', name: '火狐', rarity: 'rare', battle: '火系输出', work: '探索', skillId: 'pet_fox', desc: '白天偶尔出现在翠叶森林东侧。探索时更容易带回意外。', aura: { elem: { fire: 0.18 }, atkPct: 0.04 }, dispatch: { explore: 0.22 }, weather: { clear: 0.1 } },
  { id: 'otter', name: '水獭', rarity: 'uncommon', battle: '治疗', work: '钓鱼', skillId: 'pet_otter', desc: '月光湖边晒太阳的治疗手。', aura: { healPct: 0.12, hp: 10 }, dispatch: { fish: 0.32 } },
  { id: 'thunder_cat', name: '雷猫', rarity: 'rare', battle: '雷系攻击', work: '雷暴探索', skillId: 'pet_tcat', desc: '只在雷鸣峡谷、而且最好是雷暴里出现。', aura: { elem: { thunder: 0.2 } }, dispatch: { thunder_ore: 0.2, explore: 0.16 }, weather: { storm: 0.25 } },
  { id: 'dog_knight', name: '狗骑士', rarity: 'uncommon', battle: '防御', work: '护送', skillId: 'pet_dog', desc: '星落镇的护卫犬，不会在野外刷新，要靠任务结识。', aura: { def: 8, hp: 36 }, dispatch: { escort: 0.34 } },
  { id: 'slime_pet', name: '史莱姆', rarity: 'common', battle: '回复', work: '收集', skillId: 'pet_slime', desc: '什么活都能帮一点，但哪样都不顶尖。', aura: { hp: 24 }, dispatch: { versatile: 0.12 } },
  { id: 'shroom_spirit', name: '蘑菇精灵', rarity: 'uncommon', battle: '破甲', work: '采药', skillId: 'pet_shroom', desc: '从发光蘑菇里钻出来的小东西。', aura: { elem: { earth: 0.1 } }, dispatch: { herb: 0.22 } },
  { id: 'boar_cub', name: '野猪幼崽', rarity: 'common', battle: '生命', work: '搬运', skillId: 'pet_boar', desc: '撞人很疼，运矿也卖力。', aura: { hp: 30, atkPct: 0.03 }, dispatch: { ore: 0.12, wood: 0.18 } },
  { id: 'wolf_pup', name: '森林狼崽', rarity: 'uncommon', battle: '速度', work: '狩猎', skillId: 'pet_wolf', desc: '把速度带进队伍。探索和护卫都沾一点。', aura: { critRate: 0.04, atkPct: 0.04 }, dispatch: { explore: 0.12, escort: 0.12 } },
  { id: 'moon_cat', name: '月光猫', rarity: 'epic', battle: '夜间爆发', work: '夜间探索', skillId: 'pet_moon', desc: '只在月光湖的夜晚或深夜出现。社区里会互相提醒。', aura: { critRate: 0.1, elem: { water: 0.12 } }, dispatch: { explore: 0.2, fish: 0.1 }, weather: { clear: 0.08, fog: 0.08 } },
  { id: 'koi', name: '锦鲤', rarity: 'rare', battle: '幸运', work: '钓鱼', skillId: 'pet_koi', desc: '巨锦鲤被契约后会缩小成能跟在身边的样子。', aura: { critRate: 0.05, hp: 16 }, dispatch: { fish: 0.28, explore: 0.08 } },
  { id: 'rock_golem', name: '岩精', rarity: 'rare', battle: '坦克', work: '挖矿', skillId: 'pet_golem', desc: '从精英岩精身上分出来的一小块。', aura: { def: 12, hp: 40 }, dispatch: { ore: 0.26 } },
  { id: 'storm_bird', name: '风鸣鸟', rarity: 'rare', battle: '风系范围', work: '高地探索', skillId: 'pet_bird', desc: '风鸣高地上空偶尔落下的幼鸟。', aura: { elem: { wind: 0.16 } }, dispatch: { explore: 0.24 }, weather: { wind: 0.15 } },
  { id: 'bat', name: '矿蝠', rarity: 'common', battle: '减速', work: '挖矿', skillId: 'pet_bat', desc: '习惯黑暗，矿坑和夜间探索都合适。', aura: { critRate: 0.03 }, dispatch: { ore: 0.16, explore: 0.08 } },
  { id: 'flower_spirit', name: '花灵', rarity: 'rare', battle: '治疗', work: '采药', skillId: 'pet_flower', desc: '翠叶森林白天偶尔绽开的花苞里。', aura: { healPct: 0.16 }, dispatch: { herb: 0.3 } },
  { id: 'ember_lizard', name: '余烬蜥', rarity: 'uncommon', battle: '火系', work: '熔岩矿', skillId: 'pet_lizard', desc: '赤岩矿坑的岩蜥，体温很高。', aura: { elem: { fire: 0.12 }, atkPct: 0.03 }, dispatch: { ore: 0.14 } },
  { id: 'mist_fox', name: '雾狐', rarity: 'rare', battle: '生存', work: '迷雾探索', skillId: 'pet_mist', desc: '大雾时才会从林子里走出来。', aura: { def: 6, critRate: 0.04 }, dispatch: { explore: 0.2 }, weather: { fog: 0.22 } },
  { id: 'ancient_sprite', name: '遗迹精灵', rarity: 'epic', battle: '奥术', work: '遗迹探索', skillId: 'pet_sprite', desc: '不在野外跑。古代遗迹的首领会留下一枚蛋。', aura: { elem: { thunder: 0.12, fire: 0.08 }, atkPct: 0.05 }, dispatch: { explore: 0.18 } },
  { id: 'gold_slime', name: '金色史莱姆', rarity: 'epic', battle: '全面', work: '收集', skillId: 'pet_gold', desc: '极稀有。抓住它的旅人会被写进镇里的传闻。', aura: { critRate: 0.06, hp: 20, atkPct: 0.05 }, dispatch: { versatile: 0.18 } },
]
