<script setup lang="ts">
/** 世界地图：固定岛屿布局，显示等级解锁，并按当前位置决定航标传送是否可用。 */

import { computed } from 'vue'
import { mapThumb } from '../game/art'
import { useGame } from '../stores/game'

/** 共享游戏状态仓库，组件通过它读取服务端数据和提交操作。 */
const game = useGame()

/** 和设计图一致的十字群岛：高地在北，森林和矿坑夹着星落镇，湖南边是遗迹。 */
const nodes = [
  { id: 'wild_windhigh', x: 50, y: 10 },
  { id: 'wild_thunder', x: 50, y: 28 },
  { id: 'wild_greenleaf', x: 16, y: 50 },
  { id: 'town_starfall', x: 50, y: 50 },
  { id: 'wild_redrock', x: 84, y: 50 },
  { id: 'wild_moonlake', x: 50, y: 72 },
  { id: 'dungeon_ruins', x: 50, y: 90 },
]

/** 世界地图岛屿之间的显示连线，表达地图布局关系。 */
const links = [
  ['wild_windhigh', 'wild_thunder'],
  ['wild_thunder', 'town_starfall'],
  ['wild_greenleaf', 'town_starfall'],
  ['town_starfall', 'wild_redrock'],
  ['town_starfall', 'wild_moonlake'],
  ['wild_moonlake', 'dungeon_ruins'],
]

/** 以地图 ID 建立客户端查找索引。 */
const byId = computed(() => {
  const maps = game.config?.maps ?? []
  return Object.fromEntries(maps.map((map: any) => [map.id, map]))
})

/** 当前地图或最近互动位置的标识，用于标出角色所在位置。 */
const hereId = computed(() => game.snap?.mapId || game.profile?.mapId || 'town_starfall')
/** 角色当前所在地图的模板信息。 */
const here = computed(() => byId.value[hereId.value])
/** 当前角色等级，用于显示地图解锁状态。 */
const level = computed(() => game.profile?.level ?? 1)

/** 读取世界地图固定布局中的岛屿坐标。 */
function place(id: string) {
  return nodes.find((node) => node.id === id)!
}

/** 根据服务器地图配置的等级门槛显示岛屿是否已解锁。 */
function unlocked(id: string) {
  return level.value >= (byId.value[id]?.unlockLevel ?? 99)
}

/** 结合当前地图类型与等级，判断航标界面是否可以申请传送。 */
function canGo(id: string) {
  if (id === hereId.value || !unlocked(id)) return false
  if (here.value?.kind === 'town') return true
  if (here.value?.kind === 'dungeon' && id === 'town_starfall') return true
  return false
}

/** 向服务器申请航标传送，成功后关闭世界地图面板。 */
async function go(id: string) {
  if (!canGo(id)) return
  await game.act('/api/travel', { mapId: id })
  game.panel = ''
}
</script>

<template>
  <!-- 世界地图：固定岛屿布局，显示等级解锁，并按当前位置决定航标传送是否可用。 -->
  <p class="lede">
    <template v-if="here?.kind === 'town'">在星落镇可以使用航标。点亮的岛可以直接过去。</template>
    <template v-else-if="here?.kind === 'dungeon'">先离开遗迹，回到星落镇。</template>
    <template v-else>你在野外。世界地图可以查看，传送要回星落镇，或走上岛边的门。</template>
  </p>
  <div class="world">
    <img class="world-bg" src="/art/fi-world-bg.jpg" alt="" />
    <svg class="world-links" viewBox="0 0 100 100" preserveAspectRatio="none">
      <line
        v-for="[a, b] in links"
        :key="a + b"
        :x1="place(a).x"
        :y1="place(a).y"
        :x2="place(b).x"
        :y2="place(b).y"
      />
    </svg>
    <button
      v-for="node in nodes"
      :key="node.id"
      class="isle"
      :class="{ here: node.id === hereId, locked: !unlocked(node.id), ready: canGo(node.id) }"
      :style="{ left: node.x + '%', top: node.y + '%' }"
      :disabled="!canGo(node.id)"
      @click="go(node.id)"
    >
      <img :src="mapThumb(node.id)" alt="" />
      <b>{{ byId[node.id]?.name ?? node.id }}</b>
      <small v-if="node.id === hereId">你在这里</small>
      <small v-else-if="!unlocked(node.id)">{{ byId[node.id]?.unlockLevel }} 级解锁</small>
      <small v-else>Lv.{{ byId[node.id]?.levelMin }}–{{ byId[node.id]?.levelMax }}</small>
    </button>
  </div>
</template>
