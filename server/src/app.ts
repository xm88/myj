/** 旧版 TypeScript 迁移参考：配置索引与共享游戏应用。默认服务启动执行 server/*.go。 */

/**
 * 进程里的那一个游戏服。
 * 设计文档里的 Gateway / Player / World / Combat / Pet / Market / Guild
 * 在 V0.1 是同一个进程里的模块，而不是一组微服务。
 */

import type { DB } from './db.ts'
import { one, run } from './db.ts'
import { gameData, validate, type GameData } from './content/index.ts'
import type { DispatchNode, ItemTemplate, MapDef, MonsterDef, NpcDef, PetDef, QuestDef, RecipeDef, SkillDef } from './content/schema.ts'
import { dayPeriod } from './formula.ts'
import type { DayPeriod, WeatherId } from './types.ts'
import type { Instance, LivePlayer, Party } from './runtime.ts'

/** 旧版单进程应用状态，维护配置索引、在线世界、队伍、天气与鉴权配置。 */
export interface App {
  /** 旧版 SQLite 数据库连接。 */
  db: DB
  /** 完整游戏配置集合，配置索引由此构建。 */
  data: GameData
  /** 按模板 ID 查找物品的配置索引。 */
  items: Map<string, ItemTemplate>
  /** 按模板 ID 查找技能的配置索引。 */
  skills: Map<string, SkillDef>
  /** 按模板 ID 查找灵伴的配置索引。 */
  pets: Map<string, PetDef>
  /** 当前分线内怪物实例 ID 到状态的映射。 */
  monsters: Map<string, MonsterDef>
  /** 按模板 ID 查找地图的配置索引。 */
  maps: Map<string, MapDef>
  /** 按模板 ID 查找NPC的配置索引。 */
  npcs: Map<string, NpcDef>
  /** 按模板 ID 查找任务的配置索引。 */
  quests: Map<string, QuestDef>
  /** 按模板 ID 查找配方的配置索引。 */
  recipes: Map<string, RecipeDef>
  /** 地图采集点配置或实例采集点的冷却状态。 */
  nodes: Map<string, DispatchNode>
  /** 按角色 ID 查找旧版在线角色状态。 */
  live: Map<string, LivePlayer>
  /** 按分线或副本键查找地图实例。 */
  instances: Map<string, Instance>
  /** 按队伍 ID 查找临时队伍状态。 */
  parties: Map<string, Party>
  /** 角色 ID 到当前队伍 ID 的反向索引。 */
  playerParty: Map<string, string>
  /** 角色待接受的组队邀请及其截止时间。 */
  invites: Map<string, { fromId: string; fromName: string; until: number }>
  /** 天气限制或天气专属效率加成。 */
  weather: { id: WeatherId; until: number }
  /** 全服当前东八区游戏时段。 */
  period: DayPeriod
  /** 是否开放立即完成派遣等开发测试操作。 */
  devTools: boolean
  /** JWT 签名密钥，验证登录令牌时使用同一配置。 */
  jwtSecret: string
}

/** 初始化游戏应用、配置索引和内存世界状态，并恢复持久化天气。 */
export function createApp(db: DB): App {
  const data = gameData
  const summary = validate(data)
  console.log(
    `配置已加载：普通怪 ${summary.normals}，精英 ${summary.elites}，首领 ${summary.bosses}，灵伴 ${data.pets.length}，物品 ${summary.items}，任务 ${summary.quests}，配方 ${summary.recipes}`,
  )

  const app: App = {
    db,
    data,
    items: new Map(data.items.map((x) => [x.id, x])),
    skills: new Map(data.skills.map((x) => [x.id, x])),
    pets: new Map(data.pets.map((x) => [x.id, x])),
    monsters: new Map(data.monsters.map((x) => [x.id, x])),
    maps: new Map(data.maps.map((x) => [x.id, x])),
    npcs: new Map(data.npcs.map((x) => [x.id, x])),
    quests: new Map(data.quests.map((x) => [x.id, x])),
    recipes: new Map(data.recipes.map((x) => [x.id, x])),
    nodes: new Map(data.dispatchNodes.map((x) => [x.id, x])),
    live: new Map(),
    instances: new Map(),
    parties: new Map(),
    playerParty: new Map(),
    invites: new Map(),
    weather: { id: 'clear', until: Date.now() + data.constants.weatherMs },
    period: dayPeriod(),
    devTools: process.env.NODE_ENV !== 'production',
    jwtSecret: process.env.JWT_SECRET || 'dev-float-island-change-me',
  }

  const saved = one<{ value: string }>(db, 'SELECT value FROM world_state WHERE key = ?', 'weather')
  if (saved) {
    const parsed = JSON.parse(saved.value) as { id: WeatherId; until: number }
    app.weather = parsed
  } else {
    run(db, 'INSERT INTO world_state(key, value) VALUES(?, ?)', 'weather', JSON.stringify(app.weather))
  }
  return app
}
