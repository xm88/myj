# 配置表

这些 JSON 是 Go 服务端运行时读取的配置源。

改数值请直接修改这里的 JSON，运行 `npm run export-config`（兼容原命令名，现执行 Go 配置校验）或在 `server/` 运行 `go run . -validate-config`。启动时会检查重复 ID 和配置引用。修改配置后重启服务端。

| 文件 | 内容 |
| --- | --- |
| maps.json | 7 张地图、阻挡、传送门、资源点、刷新点 |
| monsters.json | 怪物属性和掉落 |
| pets.json | 20 只灵伴的光环和派遣专长 |
| skills.json | 24 个武器技能和灵伴技 |
| items.json | 材料、消耗品、装备模板 |
| quests.json | 主线和支线 |
| recipes.json | 制造和料理 |
| world.json | 天气、派遣点、时长、词条、常数 |
