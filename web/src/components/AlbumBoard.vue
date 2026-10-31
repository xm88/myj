<script setup lang="ts">
/** 旅册：旅迹、异兽、灵兽和踏过的地方。见闻由服务器记，这里只翻页。 */

import { computed, onMounted, ref, watch } from 'vue'
import { api } from '../api'
import ActorFigure from './ActorFigure.vue'
import { lookForMonster, lookForPet } from '../game/look'
import { useGame } from '../stores/game'

type Deed = { id: string; name: string; blurb: string; current: number; need: number; done: boolean }
type Beast = {
  id: string
  name: string
  level: number
  role: string
  roleName: string
  element: string
  tint: string
  state: 'unknown' | 'met' | 'felled'
  stateName: string
  kills: number
  places: string[]
}
type Companion = {
  id: string
  name: string
  known: boolean
  role: string
  roleName: string
  rarityName: string
  battle: string
  work: string
  desc: string
}
type Land = { id: string; name: string; kind: string; kindName: string; unlockLevel: number; visited: boolean; blurb: string }
type Book = {
  summary: { deedsDone: number; deeds: number; seen: number; beasts: number; pets: number; petTotal: number; maps: number; mapTotal: number }
  deeds: Deed[]
  beasts: Beast[]
  companions: Companion[]
  lands: Land[]
}

const game = useGame()
const book = ref<Book | null>(null)
const tab = ref<'deed' | 'beast' | 'pet' | 'land'>('deed')
const beastFilter = ref<'all' | 'known' | 'unknown'>('all')
const petFilter = ref<'all' | 'known' | 'unknown'>('all')
const picked = ref('')
let ticket = 0

async function load() {
  const mine = ++ticket
  try {
    const next = await api<Book>('/api/album')
    if (mine !== ticket) return
    book.value = next
  } catch (err) {
    if (mine === ticket) game.toast(err instanceof Error ? err.message : '旅册没有翻开')
  }
}

onMounted(load)
watch(() => game.profile, () => {
  if (game.panel === 'album') void load()
})

const summary = computed(() => book.value?.summary)
const deeds = computed(() => book.value?.deeds ?? [])
const beasts = computed(() => {
  const rows = book.value?.beasts ?? []
  if (beastFilter.value === 'known') return rows.filter((row) => row.state !== 'unknown')
  if (beastFilter.value === 'unknown') return rows.filter((row) => row.state === 'unknown')
  return rows
})
const pets = computed(() => {
  const rows = book.value?.companions ?? []
  if (petFilter.value === 'known') return rows.filter((row) => row.known)
  if (petFilter.value === 'unknown') return rows.filter((row) => !row.known)
  return rows
})
const lands = computed(() => book.value?.lands ?? [])
const selectedDeed = computed(() => deeds.value.find((row) => row.id === picked.value) ?? deeds.value.find((row) => !row.done) ?? deeds.value[0] ?? null)
const selectedBeast = computed(() => beasts.value.find((row) => row.id === picked.value) ?? beasts.value[0] ?? null)
const selectedPet = computed(() => pets.value.find((row) => row.id === picked.value) ?? pets.value[0] ?? null)
const selectedLand = computed(() => lands.value.find((row) => row.id === picked.value) ?? lands.value[0] ?? null)

function width(current: number, need: number) {
  if (!need) return '0%'
  return `${Math.min(100, Math.max(0, (current / need) * 100))}%`
}

function beastLook(beast: Beast) {
  if (beast.state === 'unknown') return lookForMonster(beast.id, 'normal', '#8a7a66')
  return lookForMonster(beast.id, beast.role, beast.tint)
}
</script>

<template>
  <p v-if="!book" class="empty">正在翻开旅册…</p>
  <template v-else>
    <p class="album-sum">
      旅迹 {{ summary?.deedsDone }}/{{ summary?.deeds }}
      · 异兽 {{ summary?.seen }}/{{ summary?.beasts }}
      · 灵兽 {{ summary?.pets }}/{{ summary?.petTotal }}
      · 踏足 {{ summary?.maps }}/{{ summary?.mapTotal }}
    </p>
    <div class="seg album-tabs">
      <button type="button" :class="{ on: tab === 'deed' }" @click="tab = 'deed'">旅迹</button>
      <button type="button" :class="{ on: tab === 'beast' }" @click="tab = 'beast'">异兽</button>
      <button type="button" :class="{ on: tab === 'pet' }" @click="tab = 'pet'">灵兽</button>
      <button type="button" :class="{ on: tab === 'land' }" @click="tab = 'land'">山川</button>
    </div>

    <div v-if="tab === 'deed'" class="album-layout">
      <div class="deed-list">
        <button
          v-for="deed in deeds"
          :key="deed.id"
          type="button"
          class="deed"
          :class="{ done: deed.done, on: selectedDeed?.id === deed.id }"
          @click="picked = deed.id"
        >
          <i class="seal">{{ deed.done ? '成' : '迹' }}</i>
          <span>
            <b>{{ deed.name }}</b>
            <small>{{ deed.blurb }}</small>
            <em class="deed-bar"><i :style="{ width: width(deed.current, deed.need) }" /></em>
          </span>
          <em class="count">{{ Math.min(deed.current, deed.need) }}/{{ deed.need }}</em>
        </button>
      </div>
      <aside v-if="selectedDeed" class="album-side" :class="{ done: selectedDeed.done }">
        <i class="seal big">{{ selectedDeed.done ? '成' : '迹' }}</i>
        <h3>{{ selectedDeed.name }}</h3>
        <p>{{ selectedDeed.blurb }}</p>
        <p class="hint">{{ selectedDeed.done ? '已经记下。' : '还差一截。' }} {{ Math.min(selectedDeed.current, selectedDeed.need) }}/{{ selectedDeed.need }}</p>
      </aside>
    </div>

    <template v-else-if="tab === 'beast'">
      <div class="seg album-tabs thin">
        <button type="button" :class="{ on: beastFilter === 'all' }" @click="beastFilter = 'all'">全部</button>
        <button type="button" :class="{ on: beastFilter === 'known' }" @click="beastFilter = 'known'">已见</button>
        <button type="button" :class="{ on: beastFilter === 'unknown' }" @click="beastFilter = 'unknown'">未遇</button>
      </div>
      <p v-if="!beasts.length" class="empty">这一栏是空的。</p>
      <div v-else class="album-layout">
        <div class="album-grid">
          <button
            v-for="beast in beasts"
            :key="beast.id"
            type="button"
            class="album-card"
            :class="[beast.state, { on: selectedBeast?.id === beast.id }]"
            :aria-label="beast.state === 'unknown' ? '未曾遇见的异兽' : beast.name"
            @click="picked = beast.id"
          >
            <ActorFigure class="album-fig" :look="beastLook(beast)" />
            <b class="nm">{{ beast.state === 'unknown' ? '？？？' : beast.name }}</b>
            <small>{{ beast.state === 'unknown' ? '未遇' : beast.stateName }}</small>
          </button>
        </div>
        <aside v-if="selectedBeast" class="album-side">
          <ActorFigure class="album-fig side" :look="beastLook(selectedBeast)" />
          <h3>{{ selectedBeast.state === 'unknown' ? '未曾遇见' : selectedBeast.name }}</h3>
          <p v-if="selectedBeast.state === 'unknown'">还没遇见。走进它活动的地方，打一场就会记上。</p>
          <template v-else>
            <p>{{ selectedBeast.roleName }} · {{ selectedBeast.level }} 级 · {{ selectedBeast.element }}</p>
            <p>{{ selectedBeast.places.length ? selectedBeast.places.join('、') : '没有固定的出没地' }}</p>
            <p>{{ selectedBeast.state === 'met' ? '遇见过，还没击败。' : `击败 ${selectedBeast.kills} 次。` }}</p>
          </template>
        </aside>
      </div>
    </template>

    <template v-else-if="tab === 'pet'">
      <div class="seg album-tabs thin">
        <button type="button" :class="{ on: petFilter === 'all' }" @click="petFilter = 'all'">全部</button>
        <button type="button" :class="{ on: petFilter === 'known' }" @click="petFilter = 'known'">已契约</button>
        <button type="button" :class="{ on: petFilter === 'unknown' }" @click="petFilter = 'unknown'">未契约</button>
      </div>
      <p v-if="!pets.length" class="empty">这一栏是空的。</p>
      <div v-else class="album-layout">
        <div class="album-grid">
          <button
            v-for="pet in pets"
            :key="pet.id"
            type="button"
            class="album-card"
            :class="{ unknown: !pet.known, felled: pet.known, on: selectedPet?.id === pet.id }"
            :aria-label="pet.known ? pet.name : '尚未契约的灵兽'"
            @click="picked = pet.id"
          >
            <ActorFigure class="album-fig" :look="lookForPet(pet.id)" />
            <b class="nm">{{ pet.known ? pet.name : '？？？' }}</b>
            <small>{{ pet.known ? pet.roleName : '未契约' }}</small>
          </button>
        </div>
        <aside v-if="selectedPet" class="album-side">
          <ActorFigure class="album-fig side" :look="lookForPet(selectedPet.id)" />
          <h3>{{ selectedPet.known ? selectedPet.name : '尚未契约' }}</h3>
          <p v-if="!selectedPet.known">还没契约。契约或获赠之后，会留在这本册子里。</p>
          <template v-else>
            <p>{{ selectedPet.roleName }} · {{ selectedPet.rarityName }}</p>
            <p>{{ selectedPet.battle }} · {{ selectedPet.work }}</p>
            <p>{{ selectedPet.desc }}</p>
          </template>
        </aside>
      </div>
    </template>

    <div v-else class="album-layout">
      <div class="album-grid lands">
        <button
          v-for="land in lands"
          :key="land.id"
          type="button"
          class="album-card land"
          :class="{ unknown: !land.visited, felled: land.visited, on: selectedLand?.id === land.id }"
          @click="picked = land.id"
        >
          <i class="seal">{{ land.kind === 'town' ? '镇' : land.kind === 'dungeon' ? '副本' : '野' }}</i>
          <b class="nm">{{ land.name }}</b>
          <small>{{ land.visited ? '已踏足' : '尚未踏足' }}</small>
        </button>
      </div>
      <aside v-if="selectedLand" class="album-side">
        <h3>{{ selectedLand.name }}</h3>
        <p>{{ selectedLand.kindName }} · {{ selectedLand.unlockLevel }} 级可入</p>
        <p>{{ selectedLand.visited ? '已经踏过。' : '还没去过。' }}</p>
        <p>{{ selectedLand.blurb }}</p>
      </aside>
    </div>
  </template>
</template>
