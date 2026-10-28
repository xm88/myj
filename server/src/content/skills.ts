/** 迁移前配置参考：武器和灵伴技能。当前 Go 服务读取根目录 config/*.json。 */

/**
 * 主动技能。
 * 四种武器各 6 个，合计 24，正好是 V0.1 的主动技能规模。
 * 角色同时只能把其中 4 个放进技能栏。灵伴技另算，不占这 24 个名额。
 */

import type { SkillDef } from './schema.ts'

/** 生成灵伴技能的公共工厂，复用技能模板的默认字段。 */
const pet = (
  id: string,
  petId: string,
  name: string,
  desc: string,
  extra: Partial<SkillDef>,
): SkillDef => ({
  id,
  pet: petId,
  name,
  desc,
  coefficient: extra.coefficient ?? 0,
  element: extra.element ?? 'physical',
  cooldown: extra.cooldown ?? 8000,
  mp: extra.mp ?? 8,
  range: extra.range ?? 180,
  ...extra,
})

/** 武器技能与灵伴技能模板，客户端只选择技能 ID。 */
export const skills: SkillDef[] = [
  // —— 剑盾：保命和拉怪。系数偏低，但附带嘲讽、护盾、治疗。
  { id: 'ss_bash', name: '盾击', desc: '用盾牌砸击，并嘲讽目标。', weapon: 'sword_shield', coefficient: 0.95, element: 'physical', cooldown: 4000, mp: 6, range: 64, tauntMs: 4000 },
  { id: 'ss_guard', name: '坚守', desc: '举起盾墙，短时间提高防御。', weapon: 'sword_shield', coefficient: 0, element: 'physical', cooldown: 10000, mp: 8, range: 0, buff: { stat: 'defPct', value: 0.3, ms: 6000 } },
  { id: 'ss_slam', name: '盾牌猛击', desc: '把力量压进一次重击。', weapon: 'sword_shield', coefficient: 1.65, element: 'physical', cooldown: 6000, mp: 10, range: 64 },
  { id: 'ss_will', name: '钢铁意志', desc: '稳住呼吸，回复一部分生命。', weapon: 'sword_shield', coefficient: 0, element: 'physical', cooldown: 12000, mp: 12, range: 0, healPct: 0.1 },
  { id: 'ss_charge', name: '壁垒冲锋', desc: '撞向敌人，并获得一层护盾。', weapon: 'sword_shield', coefficient: 1.25, element: 'physical', cooldown: 8000, mp: 10, range: 70, shieldFlat: 28 },
  { id: 'ss_oath', name: '守誓', desc: '短时间提高暴击，适合和灵伴一起压血线。', weapon: 'sword_shield', coefficient: 1.1, element: 'physical', cooldown: 9000, mp: 8, range: 64, buff: { stat: 'critRate', value: 0.12, ms: 6000 } },

  // —— 巨剑：慢，但每一刀都很重。
  { id: 'gs_slash', name: '重斩', desc: '把巨剑砍下去。', weapon: 'greatsword', coefficient: 1.85, element: 'physical', cooldown: 3500, mp: 8, range: 70 },
  { id: 'gs_spin', name: '旋风', desc: '转一圈，打中身边所有敌人。', weapon: 'greatsword', coefficient: 1.15, element: 'physical', cooldown: 8000, mp: 14, range: 90, aoe: true },
  { id: 'gs_break', name: '破甲', desc: '砍在护甲上，降低目标防御。', weapon: 'greatsword', coefficient: 1.3, element: 'physical', cooldown: 7000, mp: 10, range: 70, shredMs: 5000 },
  { id: 'gs_rage', name: '怒击', desc: '蓄力后的一击。', weapon: 'greatsword', coefficient: 2.45, element: 'physical', cooldown: 10000, mp: 16, range: 70 },
  { id: 'gs_shout', name: '战吼', desc: '提高自己的攻击。', weapon: 'greatsword', coefficient: 0, element: 'physical', cooldown: 14000, mp: 10, range: 0, buff: { stat: 'atkPct', value: 0.15, ms: 8000 } },
  { id: 'gs_cleave', name: '裂地', desc: '砸地，对周围造成伤害。', weapon: 'greatsword', coefficient: 1.7, element: 'earth', cooldown: 9000, mp: 14, range: 100, aoe: true },

  // —— 法杖：元素和治疗。
  { id: 'st_fire', name: '火球', desc: '一颗直线飞出的火球。', weapon: 'staff', coefficient: 1.7, element: 'fire', cooldown: 2500, mp: 8, range: 220 },
  { id: 'st_ice', name: '冰环', desc: '冻住身边的敌人，让他们变慢。', weapon: 'staff', coefficient: 1.05, element: 'water', cooldown: 8000, mp: 14, range: 140, aoe: true, slowMs: 3000 },
  { id: 'st_bolt', name: '雷击', desc: '把雷引到目标身上。', weapon: 'staff', coefficient: 2.15, element: 'thunder', cooldown: 7000, mp: 16, range: 230 },
  { id: 'st_arcane', name: '奥术冲击', desc: '稳定的奥术弹。', weapon: 'staff', coefficient: 1.45, element: 'physical', cooldown: 3000, mp: 7, range: 220 },
  { id: 'st_heal', name: '治疗术', desc: '回复生命。', weapon: 'staff', coefficient: 0, element: 'water', cooldown: 8000, mp: 14, range: 0, healPct: 0.18 },
  { id: 'st_surge', name: '元素涌动', desc: '下一段时间攻击提高。', weapon: 'staff', coefficient: 0.8, element: 'fire', cooldown: 12000, mp: 12, range: 220, buff: { stat: 'atkPct', value: 0.18, ms: 6000 } },

  // —— 弓：攻速和暴击。
  { id: 'bw_quick', name: '速射', desc: '几乎不耽误走位的一箭。', weapon: 'bow', coefficient: 1.05, element: 'physical', cooldown: 1600, mp: 4, range: 250 },
  { id: 'bw_aim', name: '瞄准射击', desc: '这一箭更容易打出暴击。', weapon: 'bow', coefficient: 1.9, element: 'physical', cooldown: 5000, mp: 8, range: 260, forceCrit: false, buff: undefined },
  { id: 'bw_rain', name: '箭雨', desc: '向周围撒箭。', weapon: 'bow', coefficient: 1.05, element: 'physical', cooldown: 8000, mp: 12, range: 200, aoe: true },
  { id: 'bw_pierce', name: '穿透', desc: '一箭破开防御，使目标易伤。', weapon: 'bow', coefficient: 1.55, element: 'physical', cooldown: 6000, mp: 8, range: 250, vulnerableMs: 5000 },
  { id: 'bw_mark', name: '标记', desc: '标记目标并提高自己的暴击。', weapon: 'bow', coefficient: 1.15, element: 'wind', cooldown: 7000, mp: 8, range: 250, buff: { stat: 'critRate', value: 0.1, ms: 6000 } },
  { id: 'bw_fatal', name: '致命射击', desc: '长时间瞄准后的重箭，必定暴击。', weapon: 'bow', coefficient: 2.2, element: 'physical', cooldown: 12000, mp: 14, range: 270, forceCrit: true },

  // 瞄准射击本身不 forceCrit，而是临时暴击率。上面 bw_aim 没加 buff 到自己之前的那一下。
  // 下面把 bw_aim 的暴击放到技能结算时：forceCrit 设为 false，额外在描述里说明暴击率上升由 buff 承担。
  // 为了让“瞄准”这一下就有感觉，直接强制提高本次暴击率的做法写在结算里不通用。
  // 这里改为本次 forceCrit 概率由技能 id 特殊处理？ 保持数据驱动：给 bw_aim 一个高 forceCrit 不符合“更容易”。
  // 用 buff 先加上再伤害的顺序在 sim 里实现：先 buff 再伤害。bw_aim 同时有 coefficient 和 crit buff。

  pet('pet_cat', 'forest_cat', '夜袭', '森林猫扑击要害，提高暴击。', { coefficient: 1.35, element: 'physical', buff: { stat: 'critRate', value: 0.1, ms: 5000 } }),
  pet('pet_rat', 'mine_rat', '钻地', '矿洞鼠掀起碎石，给你一层护盾。', { coefficient: 0.6, element: 'earth', shieldFlat: 36 }),
  pet('pet_fox', 'fire_fox', '火息', '火狐喷出一团火。', { coefficient: 1.7, element: 'fire', aoe: true, range: 140 }),
  pet('pet_otter', 'otter', '水疗', '水獭把水膜覆在你身上。', { coefficient: 0, element: 'water', healPct: 0.12 }),
  pet('pet_tcat', 'thunder_cat', '落雷', '雷猫引下一道雷。', { coefficient: 1.9, element: 'thunder' }),
  pet('pet_dog', 'dog_knight', '护主', '狗骑士挡到前面并嘲讽。', { coefficient: 0.8, element: 'physical', tauntMs: 5000, buff: { stat: 'defPct', value: 0.2, ms: 6000 } }),
  pet('pet_slime', 'slime_pet', '凝胶', '史莱姆糊住伤口。', { coefficient: 0, element: 'water', healPct: 0.08, shieldFlat: 20 }),
  pet('pet_shroom', 'shroom_spirit', '孢子', '孢子降低目标防御。', { coefficient: 1.1, element: 'earth', shredMs: 5000 }),
  pet('pet_boar', 'boar_cub', '冲撞', '野猪幼崽一头撞上去。', { coefficient: 1.5, element: 'physical' }),
  pet('pet_wolf', 'wolf_pup', '撕咬', '狼崽加速你的行动。', { coefficient: 1.2, element: 'physical', buff: { stat: 'speedPct', value: 0.15, ms: 5000 } }),
  pet('pet_moon', 'moon_cat', '月斩', '月光变成一刀。', { coefficient: 1.65, element: 'water', forceCrit: true }),
  pet('pet_koi', 'koi', '鳞光', '锦鲤的鳞光回复生命。', { coefficient: 0, element: 'water', healPct: 0.1, buff: { stat: 'critRate', value: 0.06, ms: 6000 } }),
  pet('pet_golem', 'rock_golem', '岩壁', '岩精立起一面墙。', { coefficient: 0.7, element: 'earth', shieldFlat: 60, buff: { stat: 'defPct', value: 0.15, ms: 6000 } }),
  pet('pet_bird', 'storm_bird', '俯冲', '风鸣鸟从高处冲下。', { coefficient: 1.6, element: 'wind', aoe: true, range: 160 }),
  pet('pet_bat', 'bat', '超声', '超声波让周围敌人变慢。', { coefficient: 0.9, element: 'wind', aoe: true, slowMs: 3000, range: 140 }),
  pet('pet_flower', 'flower_spirit', '花露', '花灵洒下治疗的露水。', { coefficient: 0, element: 'earth', healPct: 0.16 }),
  pet('pet_lizard', 'ember_lizard', '余烬', '余烬蜥留下灼烧般的一击。', { coefficient: 1.55, element: 'fire', vulnerableMs: 4000 }),
  pet('pet_mist', 'mist_fox', '迷雾', '雾狐隐藏你的身形，提高防御。', { coefficient: 0.4, element: 'water', buff: { stat: 'defPct', value: 0.25, ms: 5000 } }),
  pet('pet_sprite', 'ancient_sprite', '遗言', '遗迹精灵放出一道奥术。', { coefficient: 1.8, element: 'thunder' }),
  pet('pet_gold', 'gold_slime', '金雾', '金色史莱姆爆开，伤害周围并回复你。', { coefficient: 1.2, element: 'earth', aoe: true, healPct: 0.06, range: 130 }),
]

// 瞄准射击：这一箭自带更高暴击。数据上用 forceCrit 不准确，改为在 id 旁增加 critRate 增量字段会扩大技能结构。
// 实践上把 bw_aim 的 forceCrit 打开会太强。保留暴击 buff，并让系数已经高于普攻。
/** 查找瞄准射击模板，将短时暴击增益写入该技能配置。 */
const aim = skills.find((s) => s.id === 'bw_aim')
if (aim) aim.buff = { stat: 'critRate', value: 0.35, ms: 2000 }
