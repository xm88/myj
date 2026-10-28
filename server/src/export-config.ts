/** 旧版导表工具：从 TypeScript 配置生成 JSON，仅在需要重建迁移参考时运行；默认 npm 配置命令执行 Go 校验。 */

/**
 * 把内存里的配置表导出成 JSON，方便对照设计文档里的「配置驱动」。
 * 运行：npm run export-config
 */
import fs from 'node:fs'
import path from 'node:path'
import { gameData, validate } from './content/index.ts'

/** 配置校验后的内容统计，用于导出日志和检查模板数量。 */
const summary = validate(gameData)
/** 当前脚本所在目录，用于解析项目内的固定路径。 */
const dir = path.resolve(import.meta.dirname, '../../config')
fs.mkdirSync(dir, { recursive: true })

/** 待导出的配置表或统一版本化素材路径清单。 */
const files: Record<string, unknown> = {
  'items.json': gameData.items,
  'skills.json': gameData.skills,
  'pets.json': gameData.pets,
  'monsters.json': gameData.monsters,
  'maps.json': gameData.maps,
  'npcs.json': gameData.npcs,
  'quests.json': gameData.quests,
  'recipes.json': gameData.recipes,
  'world.json': {
    weathers: gameData.weathers,
    durations: gameData.durations,
    dispatchNodes: gameData.dispatchNodes,
    affixes: gameData.affixes,
    constants: gameData.constants,
  },
}

for (const [name, value] of Object.entries(files)) {
  fs.writeFileSync(path.join(dir, name), JSON.stringify(value, null, 2), 'utf8')
}

console.log('已导出 config/*.json', summary)
