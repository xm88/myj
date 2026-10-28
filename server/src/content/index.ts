/** 迁移前配置参考：模板汇总与引用校验。当前 Go 服务读取根目录 config/*.json。 */

/**
 * 把配置收成一份只读数据，并在启动时检查 id 没有断。
 * 断掉的任务奖励或掉落会直接让进程起不来，而不是等玩家做到那一步才发现。
 */

import { affixes, BAG_CAP, BANK_CAP, dispatchNodes, durations, GUILD_CREATE_COST, LINE_CAP, MARKET_TAX, STARTER_GOLD, STARTER_ORBS, WEATHER_MS, weathers } from './world.ts'
import { items } from './items.ts'
import { maps } from './maps.ts'
import { monsters } from './monsters.ts'
import { npcs } from './npcs.ts'
import { pets } from './pets.ts'
import { quests } from './quests.ts'
import { recipes } from './recipes.ts'
import { skills } from './skills.ts'
import type { AffixDef, DispatchNode, DurationDef, ItemTemplate, MapDef, MonsterDef, NpcDef, PetDef, QuestDef, RecipeDef, SkillDef, WeatherDef } from './schema.ts'

/** 旧版完整游戏配置集合，作为导表与迁移数值对照的来源。 */
export interface GameData {
  /** 物品配置模板列表，按所属结构决定存储形式。 */
  items: ItemTemplate[]
  /** 武器与灵伴技能模板列表。 */
  skills: SkillDef[]
  /** 灵伴模板列表，包含光环、技能和派遣规则。 */
  pets: PetDef[]
  /** 当前分线内怪物实例 ID 到状态的映射。 */
  monsters: MonsterDef[]
  /** 地图模板列表，包含入口、阻挡、传送门和刷新规则。 */
  maps: MapDef[]
  /** NPC 模板列表，包含位置、对话和服务配置。 */
  npcs: NpcDef[]
  /** 任务模板列表，包含前置、目标和奖励。 */
  quests: QuestDef[]
  /** 制造和料理配方模板列表。 */
  recipes: RecipeDef[]
  /** 全服天气模板列表，切换时沿配置顺序推进。 */
  weathers: WeatherDef[]
  /** 该实例已经生成的装备词条集合。 */
  affixes: AffixDef[]
  /** 派遣时长、奖励倍率和事件概率的模板列表。 */
  durations: DurationDef[]
  /** 可选择的派遣路线及其基础产出模板列表。 */
  dispatchNodes: DispatchNode[]
  /** 容量、税率、初始奖励和天气周期等全局规则。 */
  constants: {
    weatherMs: number
    lineCap: number
    marketTax: number
    bagCap: number
    bankCap: number
    guildCost: number
    starterGold: number
    starterOrbs: number
  }
}

/** 旧版完整配置集合，包含各模板表和全局常数。 */
export const gameData: GameData = {
  items,
  skills,
  pets,
  monsters,
  maps,
  npcs,
  quests,
  recipes,
  weathers,
  affixes,
  durations,
  dispatchNodes,
  constants: {
    weatherMs: WEATHER_MS,
    lineCap: LINE_CAP,
    marketTax: MARKET_TAX,
    bagCap: BAG_CAP,
    bankCap: BANK_CAP,
    guildCost: GUILD_CREATE_COST,
    starterGold: STARTER_GOLD,
    starterOrbs: STARTER_ORBS,
  },
}

/** 检查配置数组的 ID 唯一性，避免索引建立时覆盖同名模板。 */
function mustUnique(label: string, ids: string[]) {
  const seen = new Set<string>()
  for (const id of ids) {
    if (seen.has(id)) throw new Error(`配置 id 重复：${label} / ${id}`)
    seen.add(id)
  }
}

/** 检查配置数量、重复 ID 与跨表引用，启动前阻止无效配置。 */
export function validate(data: GameData) {
  mustUnique('item', data.items.map((x) => x.id))
  mustUnique('skill', data.skills.map((x) => x.id))
  mustUnique('pet', data.pets.map((x) => x.id))
  mustUnique('monster', data.monsters.map((x) => x.id))
  mustUnique('map', data.maps.map((x) => x.id))
  mustUnique('npc', data.npcs.map((x) => x.id))
  mustUnique('quest', data.quests.map((x) => x.id))
  mustUnique('recipe', data.recipes.map((x) => x.id))
  mustUnique('node', data.dispatchNodes.map((x) => x.id))

  const itemIds = new Set(data.items.map((x) => x.id))
  const skillIds = new Set(data.skills.map((x) => x.id))
  const petIds = new Set(data.pets.map((x) => x.id))
  const monsterIds = new Set(data.monsters.map((x) => x.id))
  const mapIds = new Set(data.maps.map((x) => x.id))
  const npcIds = new Set(data.npcs.map((x) => x.id))
  const questIds = new Set(data.quests.map((x) => x.id))
  const recipeIds = new Set(data.recipes.map((x) => x.id))

  for (const skill of data.skills) {
    if (skill.pet && !petIds.has(skill.pet)) throw new Error(`技能 ${skill.id} 的灵伴不存在`)
  }
  for (const pet of data.pets) {
    if (!skillIds.has(pet.skillId)) throw new Error(`灵伴 ${pet.id} 缺少技能 ${pet.skillId}`)
  }
  for (const monster of data.monsters) {
    if (monster.petId && !petIds.has(monster.petId)) throw new Error(`怪物 ${monster.id} 的契约目标不存在`)
    for (const drop of monster.drops) {
      if (!itemIds.has(drop.itemId)) throw new Error(`怪物 ${monster.id} 掉落了不存在的 ${drop.itemId}`)
    }
  }
  for (const map of data.maps) {
    for (const spawn of [...map.spawns, ...map.rares]) {
      if (!monsterIds.has(spawn.monsterId)) throw new Error(`地图 ${map.id} 刷新了不存在的 ${spawn.monsterId}`)
    }
    for (const portal of map.portals) {
      if (!mapIds.has(portal.to)) throw new Error(`地图 ${map.id} 的门通向不存在的 ${portal.to}`)
    }
    for (const node of map.nodes) {
      if (!itemIds.has(node.itemId)) throw new Error(`资源点 ${node.id} 物品不存在`)
    }
  }
  for (const npc of data.npcs) {
    if (!mapIds.has(npc.mapId)) throw new Error(`NPC ${npc.id} 的地图不存在`)
    for (const good of npc.shop ?? []) {
      if (!itemIds.has(good.itemId)) throw new Error(`商店 ${npc.id} 出售不存在的 ${good.itemId}`)
    }
  }
  for (const quest of data.quests) {
    if (!npcIds.has(quest.giver) || !npcIds.has(quest.turnin)) throw new Error(`任务 ${quest.id} 的 NPC 不存在`)
    for (const id of quest.prereq) if (!questIds.has(id)) throw new Error(`任务 ${quest.id} 前置不存在`)
    if (quest.next && !questIds.has(quest.next)) throw new Error(`任务 ${quest.id} 的后续不存在`)
    for (const item of quest.rewards.items ?? []) {
      if (!itemIds.has(item.itemId)) throw new Error(`任务 ${quest.id} 奖励物品不存在`)
    }
    if (quest.rewards.petId && !petIds.has(quest.rewards.petId)) throw new Error(`任务 ${quest.id} 奖励灵伴不存在`)
    for (const goal of quest.objectives) {
      if (goal.type === 'kill' && !monsterIds.has(goal.monsterId)) throw new Error(`任务 ${quest.id} 击杀目标不存在`)
      if (goal.type === 'collect' && !itemIds.has(goal.itemId)) throw new Error(`任务 ${quest.id} 收集目标不存在`)
      if (goal.type === 'capture' && !petIds.has(goal.petId)) throw new Error(`任务 ${quest.id} 契约目标不存在`)
      if (goal.type === 'craft' && !recipeIds.has(goal.recipeId)) throw new Error(`任务 ${quest.id} 配方不存在`)
      if (goal.type === 'enter' && !mapIds.has(goal.mapId)) throw new Error(`任务 ${quest.id} 地图不存在`)
      if ((goal.type === 'talk') && !npcIds.has(goal.npcId)) throw new Error(`任务 ${quest.id} 对话 NPC 不存在`)
    }
  }
  for (const recipe of data.recipes) {
    for (const input of recipe.inputs) if (!itemIds.has(input.itemId)) throw new Error(`配方 ${recipe.id} 材料不存在`)
    if (!itemIds.has(recipe.output.itemId)) throw new Error(`配方 ${recipe.id} 产物不存在`)
  }
  for (const node of data.dispatchNodes) {
    if (!mapIds.has(node.mapId)) throw new Error(`派遣点 ${node.id} 地图不存在`)
    for (const base of node.basePerHour) if (!itemIds.has(base.itemId)) throw new Error(`派遣点 ${node.id} 产物不存在`)
  }

  const normals = data.monsters.filter((m) => m.role === 'normal').length
  const elites = data.monsters.filter((m) => m.role === 'elite').length
  const bosses = data.monsters.filter((m) => m.role === 'boss').length
  const weaponSkills = data.skills.filter((s) => s.weapon).length
  if (weaponSkills !== 24) throw new Error(`武器技能应为 24，实际 ${weaponSkills}`)
  if (data.pets.length !== 20) throw new Error(`灵伴应为 20，实际 ${data.pets.length}`)
  if (data.maps.length !== 7) throw new Error(`地图应为 1 镇 + 5 野外 + 1 副本`)
  return { normals, elites, bosses, quests: data.quests.length, items: data.items.length, recipes: data.recipes.length }
}
