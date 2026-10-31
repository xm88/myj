<script setup lang="ts">
/** 可跳过的三步：进镇、接第一只召唤兽、打第一场。进度记在本机。 */

import { computed, watch } from 'vue'
import { primerStep } from '../game/primer'
import { npcFocus, portalFocus } from '../game/npc'
import { nearestPacks } from '../game/crowd'
import { useGame } from '../stores/game'

const game = useGame()
const skipKey = 'float_primer'
const foughtKey = 'float_primer_fought'

const skipped = computed(() => localStorage.getItem(skipKey) === 'skip')
const fought = computed(() => localStorage.getItem(foughtKey) === '1' || Boolean(game.snap?.bout))

const step = computed(() => primerStep({
  ready: Boolean(game.profile?.quests),
  skipped: skipped.value,
  fought: fought.value,
  quests: game.profile?.quests ?? [],
  pets: game.profile?.pets ?? [],
}))

watch(() => game.snap?.bout, (bout) => {
  if (bout) localStorage.setItem(foughtKey, '1')
})

const copy: Record<string, { title: string; text: string; cta: string }> = {
  town: {
    title: '进镇',
    text: '你到了星落镇。先去和镇长艾兰说一句话，主线会从这里往下走。',
    cta: '去找镇长',
  },
  beast: {
    title: '接兽',
    text: '第一只召唤兽是翠叶森林的森林猫。打到三成血再契约；七成血也可以用捕兽索试一次，套住的猫还不熟。',
    cta: '前往翠叶',
  },
  fight: {
    title: '首战',
    text: '点一只怪物开战。轮到你和召唤兽时，中间会亮出动作，出手会有飘字和一条短读条。',
    cta: '去打一场',
  },
}

const card = computed(() => (step.value === 'done' ? null : copy[step.value]))

function skip() {
  localStorage.setItem(skipKey, 'skip')
  game.toast('引导已跳过，可以随时从任务追踪继续')
}

function go() {
  const mapId = game.snap?.mapId || game.profile?.mapId || ''
  if (step.value === 'town') {
    const elder = (game.config?.npcs ?? []).find((npc: { id: string; mapId?: string; x?: number; y?: number }) => npc.id === 'npc_elder')
    if (elder?.mapId === mapId && typeof elder.x === 'number' && typeof elder.y === 'number') game.go(elder.x, elder.y, 'interact', npcFocus(elder.id))
    else game.pursue('q_main_01')
    return
  }
  if (step.value === 'beast') {
    const here = (game.config?.maps ?? []).find((map: { id: string; portals?: { to: string; x: number; y: number }[] }) => map.id === mapId)
    const door = here?.portals?.find((portal: { to: string }) => portal.to === 'wild_greenleaf')
    if (door) {
      game.go(door.x, door.y, 'interact', portalFocus(door.x, door.y))
      return
    }
    const packs = nearestPacks(
      (game.snap?.entities ?? []).filter((entity: { kind: string; alive: boolean }) => entity.kind === 'monster' && entity.alive),
      game.snap?.self ?? { x: 0, y: 0 },
    )
    const cat = packs.find((pack) => pack.templateId === 'forest_cat')
    if (cat) game.fight(cat.id)
    else game.toast('森林猫在翠叶的空地上。打残后再契约。')
    return
  }
  const packs = nearestPacks(
    (game.snap?.entities ?? []).filter((entity: { kind: string; alive: boolean }) => entity.kind === 'monster' && entity.alive),
    game.snap?.self ?? { x: 0, y: 0 },
  )
  const slime = packs.find((pack) => pack.templateId === 'slime') ?? packs[0]
  if (slime) {
    game.fight(slime.id)
    return
  }
  const here = (game.config?.maps ?? []).find((map: { id: string; portals?: { to: string; x: number; y: number }[] }) => map.id === mapId)
  const door = here?.portals?.find((portal: { to: string }) => portal.to === 'wild_greenleaf')
  if (door) game.go(door.x, door.y, 'interact', portalFocus(door.x, door.y))
  else game.toast('这片地方现在没有怪物。往翠叶森林走。')
}
</script>

<template>
  <aside v-if="card" class="primer">
    <header>
      <b>新手 · {{ card.title }}</b>
      <button type="button" @click="skip">跳过</button>
    </header>
    <ol>
      <li :class="{ on: step === 'town', done: step !== 'town' }">进镇</li>
      <li :class="{ on: step === 'beast', done: step === 'fight' }">接兽</li>
      <li :class="{ on: step === 'fight' }">首战</li>
    </ol>
    <p>{{ card.text }}</p>
    <button class="btn lantern" type="button" @click="go">{{ card.cta }}</button>
  </aside>
</template>
