<script setup lang="ts">
/** 地图场景摘要：聚合怪物并避让对象标签，点击后提交世界坐标和精确互动目标。 */

import { computed } from 'vue'
import { nearestPacks } from '../game/crowd'
import { questGuide } from '../game/guide'
import { mapThumb } from '../game/art'
import { nodeFocus, npcFocus, npcMark, portalFocus } from '../game/npc'
import { useGame } from '../stores/game'

/** 共享游戏状态仓库，组件通过它读取服务端数据和提交操作。 */
const game = useGame()

/** 优先使用实时快照中的地图，再回退到持久化角色档案。 */
const mapId = computed(() => game.snap?.mapId || game.profile?.mapId || 'town_starfall')
/** 当前地图模板，提供尺寸、背景、资源点与传送门配置。 */
const map = computed(() => game.config?.maps?.find((item: { id: string; width?: number; height?: number; portals?: { x: number; y: number; to: string; label: string }[] }) => item.id === mapId.value))
/** 服务器推送的自身战斗快照，缺失时部分视图使用档案坐标。 */
const self = computed(() => game.snap?.self ?? { x: game.profile?.x ?? 0, y: game.profile?.y ?? 0 })

/** NPC 名称、位置、对话和服务入口的配置清单。 */
const npcs = computed(() => (game.config?.npcs ?? []).filter((npc: { mapId: string }) => npc.mapId === mapId.value))
/** 地图资源点或世界地图固定岛屿坐标，具体形状由所在组件定义。 */
const nodes = computed(() => map.value?.nodes ?? [])
/** 当前地图可通往其他地图的传送门列表。 */
const portals = computed(() => map.value?.portals ?? [])
/** 当前分线快照中仍存活的怪物实例，供聚合、选择和任务引导使用。 */
const monsters = computed(() => (game.snap?.entities ?? []).filter((entity: { kind: string; alive: boolean }) => entity.kind === 'monster' && entity.alive))
/** 按模板聚合的附近怪物，使用最近实例作为操作目标。 */
const packs = computed(() => nearestPacks(monsters.value, self.value))

/** 当前任务推荐对象的场景标记，用于高亮下一步位置。 */
const guideKey = computed(() => questGuide(game.profile?.quests ?? [], game.config, mapId.value, packs.value)?.key ?? '')

/** 在九十坐标单位以内选择最近的门、资源点或 NPC，用于场景位置高亮。 */
const hereId = computed(() => {
  const who = self.value
  let best = ''
  let bestDistance = 90
  const spots = [
    ...portals.value.map((portal: { x: number; y: number; label: string }) => ({ id: `p:${portal.label}`, x: portal.x, y: portal.y })),
    ...nodes.value.map((node: { id: string; x: number; y: number }) => ({ id: node.id, x: node.x, y: node.y })),
    ...npcs.value.map((npc: { id: string; x: number; y: number }) => ({ id: npc.id, x: npc.x, y: npc.y })),
  ]
  for (const spot of spots) {
    const distance = Math.hypot(spot.x - who.x, spot.y - who.y)
    if (distance < bestDistance) {
      bestDistance = distance
      best = spot.id
    }
  }
  return best
})

/** 把场景对象转换成百分比位置并做多轮避让，减少标签重叠。 */
const laid = computed(() => {
  const width = map.value?.width || 1
  const height = map.value?.height || 1
  const marks = [
    ...portals.value.map((portal: { label: string; x: number; y: number }) => ({ key: `p:${portal.label}`, x: portal.x, y: portal.y })),
    ...nodes.value.map((node: { id: string; x: number; y: number }) => ({ key: node.id, x: node.x, y: node.y })),
    ...npcs.value.map((npc: { id: string; x: number; y: number }) => ({ key: npc.id, x: npc.x, y: npc.y })),
    ...packs.value.map((pack) => ({ key: pack.templateId, x: pack.x, y: pack.y })),
  ].map((mark) => ({ ...mark, left: (mark.x / width) * 100, top: (mark.y / height) * 100 }))
  for (let pass = 0; pass < 10; pass++) {
    for (let i = 0; i < marks.length; i++) {
      for (let j = i + 1; j < marks.length; j++) {
        const a = marks[i]!
        const b = marks[j]!
        let dx = b.left - a.left
        let dy = b.top - a.top
        const dist = Math.hypot(dx, dy) || 0.01
        if (dist >= 8) continue
        const push = (8 - dist) / 2
        dx /= dist
        dy /= dist
        a.left -= dx * push
        a.top -= dy * push
        b.left += dx * push
        b.top += dy * push
      }
    }
  }
  const at: Record<string, { left: string; top: string }> = {}
  for (const mark of marks) {
    at[mark.key] = {
      left: `${Math.min(94, Math.max(6, mark.left))}%`,
      top: `${Math.min(92, Math.max(8, mark.top))}%`,
    }
  }
  return at
})

/** 读取场景标记的避让布局坐标，缺失标记暂置于场景中央。 */
function at(key: string) {
  return laid.value[key] ?? { left: '50%', top: '50%' }
}

/** 复用任务状态规则，生成场景 NPC 的任务高亮标记。 */
function markOf(npc: { id: string; name: string }) {
  return npcMark(npc, game.profile)
}

/** 当前移动意图的目的地，供场景显示目标位置。 */
const dest = computed(() => (game.intent?.kind === 'move' ? game.intent : null))

/** 将服务器世界坐标按地图尺寸换成百分比界面位置。 */
function pin(x: number, y: number) {
  const width = map.value?.width || 1
  const height = map.value?.height || 1
  return { left: `${(x / width) * 100}%`, top: `${(y / height) * 100}%` }
}
</script>

<template>
  <!-- 地图场景摘要：聚合怪物并避让对象标签，点击后提交世界坐标和精确互动目标。 -->
  <div class="scene" v-if="map">
    <img class="scene-bg" :src="mapThumb(map.id)" alt="" />
    <div class="pin you" :style="pin(self.x, self.y)">你</div>
    <div v-if="dest" class="pin dest" :style="pin(dest.x, dest.y)">去</div>
    <button
      v-for="portal in portals"
      :key="portal.label + portal.x"
      class="tag exit"
      :class="{ here: hereId === `p:${portal.label}`, goal: guideKey === `p:${portal.label}` }"
      :style="at(`p:${portal.label}`)"
      @click="game.go(portal.x, portal.y, 'interact', portalFocus(portal.x, portal.y))"
    >{{ portal.label }}</button>
    <button
      v-for="node in nodes"
      :key="node.id"
      class="tag"
      :class="{ here: hereId === node.id }"
      :style="at(node.id)"
      @click="game.go(node.x, node.y, 'interact', nodeFocus(node.id))"
    >{{ node.name }}</button>
    <button
      v-for="npc in npcs"
      :key="npc.id"
      class="tag"
      :class="{ here: hereId === npc.id, goal: guideKey === npc.id, turnin: markOf(npc) === 'turnin', offer: markOf(npc) === 'offer' || markOf(npc) === 'talk' }"
      :style="at(npc.id)"
      @click="game.go(npc.x, npc.y, 'interact', npcFocus(npc.id))"
    ><i v-if="markOf(npc) === 'turnin'" class="qmark">?</i><i v-else-if="markOf(npc) === 'offer' || markOf(npc) === 'talk'" class="qmark">!</i>{{ npc.name }}</button>
    <button
      v-for="pack in packs"
      :key="pack.templateId"
      class="tag foe"
      :style="at(pack.templateId)"
      @click="game.fight(pack.id)"
    >{{ pack.name }}<template v-if="pack.count > 1"> ×{{ pack.count }}</template></button>
  </div>
  <div v-else class="scene empty-scene">地图还在加载</div>
</template>
