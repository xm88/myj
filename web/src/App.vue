<script setup lang="ts">
/** 根视图：启动时恢复游戏状态，根据加载、登录和角色创建状态切换界面。 */

import { onMounted } from 'vue'
import GateView from './views/GateView.vue'
import ShellView from './views/ShellView.vue'
import { useGame } from './stores/game'

/** 共享游戏状态仓库，组件通过它读取服务端数据和提交操作。 */
const game = useGame()
onMounted(() => game.boot())
</script>

<template>
  <!-- 根视图：启动时恢复游戏状态，根据加载、登录和角色创建状态切换界面。 -->
  <p v-if="game.booting" class="gate boot-line">正在进入游戏…</p>
  <GateView v-else-if="!game.token || game.needCharacter" />
  <ShellView v-else />
</template>
