<script setup lang="ts">
/** 战斗界面：展示服务器目标、生命法力、主战灵伴与契约入口，客户端仅提交动作。 */

import { computed, ref, watch } from 'vue'
import IconMedal from './IconMedal.vue'
import { mapThumb, portrait } from '../game/art'
import { useGame } from '../stores/game'

/** 共享游戏状态仓库，组件通过它读取服务端数据和提交操作。 */
const game = useGame()

/** 当前选中或正在追击的怪物快照。 */
const foe = computed(() => {
  const intent = game.intent
  const id = intent && intent.kind !== 'move' ? intent.id : game.snap?.self?.targetId
  return (game.snap?.entities ?? []).find((entity: { id: string }) => entity.id === id) ?? null
})
/** 角色当前设为主战的灵伴，用于战斗搭档展示。 */
const pet = computed(() => game.profile?.pets?.find((item: { active: boolean }) => item.active) ?? null)
/** 服务器推送的自身战斗快照，缺失时部分视图使用档案坐标。 */
const self = computed(() => game.snap?.self)
/** 将自身当前生命换算成血条显示百分比。 */
const hpPct = computed(() => (self.value ? Math.max(0, (self.value.hp / self.value.maxHp) * 100) : 0))
/** 将自身当前法力换算成法力条显示百分比。 */
const mpPct = computed(() => (self.value ? Math.max(0, (self.value.mp / self.value.maxMp) * 100) : 0))
/** 将目标当前生命换算成目标血条百分比。 */
const foePct = computed(() => (foe.value?.maxHp ? Math.max(0, (foe.value.hp / foe.value.maxHp) * 100) : 0))
/** 显示目标是否可尝试契约；最终条件与成功率由服务器校验。 */
const canCapture = computed(() => Boolean(foe.value?.alive && foe.value.capturable && foe.value.hp / foe.value.maxHp <= 0.3))
/** 从公共模板读取目标等级，供战斗界面展示。 */
const foeLevel = computed(() => game.config?.monsters?.find((item: { id: string }) => item.id === foe.value?.templateId)?.level)
/** 追击已结束且当前目标死亡时显示战斗结束状态。 */
const won = computed(() => !chasing.value && !foe.value?.alive)
/** 出手条：间隔走完就满，表示下一拍普通攻击准备好了。 */
const readyPct = computed(() => {
  const interval = Number(self.value?.interval) || 0
  const swing = Number(self.value?.swing) || 0
  if (!interval) return self.value?.targetId ? 100 : 0
  return Math.min(100, Math.max(0, (1 - swing / interval) * 100))
})
/** 守势剩余时，出手条改成守势提示。 */
const guarding = computed(() => Number(self.value?.guard) > 0)
/** 这一场已经打出的普通攻击拍数。 */
const beats = computed(() => Number(self.value?.beats) || 0)
/** 战斗画面上跟着自己或目标的飘字。 */
function hits(id: string | undefined) {
  if (!id) return []
  return game.floats.filter((item) => item.entityId === id)
}
/** 优先使用实时快照中的地图，再回退到持久化角色档案。 */
const mapId = computed(() => game.snap?.mapId || game.profile?.mapId || 'town_starfall')
/** 判断当前意图是否为追击攻击或契约目标。 */
const chasing = computed(() => game.intent?.kind === 'fight' || game.intent?.kind === 'capture')
/** 缓存最近目标名称，目标消失后仍能显示战斗结束提示。 */
const lastName = ref('')
watch(foe, (next) => {
  if (next?.name) lastName.value = next.name
})
</script>

<template>
  <!-- 战斗界面：展示服务器目标、生命法力、主战灵伴与契约入口，客户端仅提交动作。 -->
  <div class="battle">
    <img class="scene-bg dim" :src="mapThumb(mapId)" alt="" />
    <div class="fighter">
      <IconMedal class="portrait" :src="portrait('hero')" />
      <b>{{ game.profile?.name }}</b>
      <small>{{ game.profile?.weaponName }} · Lv.{{ game.profile?.level }}</small>
      <div class="bar"><i class="hp" :style="{ width: hpPct + '%' }" /></div>
      <small>{{ self?.hp ?? 0 }} / {{ self?.maxHp ?? 0 }}</small>
      <span v-for="hit in hits(game.profile?.id)" :key="hit.id" class="float" :class="{ crit: hit.crit, heal: hit.kind === 'heal', miss: hit.kind === 'miss', guarded: hit.guarded }">{{ hit.text }}</span>
      <div class="bar"><i class="mp" :style="{ width: mpPct + '%' }" /></div>
      <div v-if="pet" class="pet-row">
        <IconMedal :src="portrait(pet.templateId)" />
        <span>{{ pet.name }}<small>主战</small></span>
      </div>
      <p v-else class="hint">还没有主战灵伴</p>
    </div>
    <div class="battle-mid">
      <div class="cadence">
        <div class="swing" :title="guarding ? '守势中，这一拍少挨打' : '普通攻击的出手节奏'">
          <i :class="{ guard: guarding }" :style="{ width: readyPct + '%' }" />
        </div>
        <small>{{ guarding ? '守势' : (beats ? '第 ' + beats + ' 拍' : '等待出手') }}{{ game.autoFight ? ' · 连招' : '' }}</small>
      </div>
      <div class="battle-log">
        <p v-if="chasing">正在靠近{{ foe?.name ? ' ' + foe.name : '' }}。走进攻击距离后会自动出手。</p>
        <p v-else-if="won">{{ lastName ? lastName + ' 已被击败。' : '战斗结束。' }}</p>
        <p v-for="(line, index) in game.battleLog" :key="index" :class="{ crit: line.includes('暴击'), guarded: line.includes('守势') }">{{ line }}</p>
      </div>
      <div class="row">
        <button class="btn" :class="{ lantern: guarding }" @click="game.guard()">守势</button>
        <button class="btn" :class="{ lantern: game.autoFight }" @click="game.setAuto(!game.autoFight)">{{ game.autoFight ? '连招中' : '连招' }}</button>
        <button class="btn danger" @click="game.flee()">逃跑</button>
        <button class="btn" :class="{ lantern: won }" @click="game.view = 'scene'">{{ won ? '胜利，返回地图' : '返回地图' }}</button>
        <button class="btn lantern" :disabled="!canCapture || !foe" @click="foe && game.capture(foe.id)">契约</button>
      </div>
    </div>
    <div class="fighter">
      <template v-if="foe">
        <IconMedal class="portrait" :src="portrait(foe.templateId)" />
        <b>{{ foe.name }}</b>
        <small>{{ foeLevel ? foeLevel + '级 · ' : '' }}{{ foe.alive ? (foe.capturable ? '可以契约' : '战斗中') : '已被击败' }}</small>
        <div class="bar"><i class="hp foe" :style="{ width: foePct + '%' }" /></div>
        <small>{{ foe.hp }} / {{ foe.maxHp }}</small>
        <span v-for="hit in hits(foe.id)" :key="hit.id" class="float foe" :class="{ crit: hit.crit, heal: hit.kind === 'heal' }">{{ hit.text }}</span>
      </template>
      <p v-else class="hint">{{ lastName ? lastName + ' 已被击败' : '这场已经结束' }}</p>
    </div>
  </div>
</template>
