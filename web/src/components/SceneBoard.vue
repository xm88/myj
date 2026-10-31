<script setup lang="ts">
/** 自由镜头：当前地块和相邻地块按世界原点铺开，镜头跟着人走。副本仍只有自己这一张。 */

import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import ActorFigure from './ActorFigure.vue'
import { nearestPacks } from '../game/crowd'
import { questGuide } from '../game/guide'
import { mapThumb } from '../game/art'
import { lookForMonster, lookForNpc, lookForPet, lookForSelf, lookForTraveler, type Face } from '../game/look'
import { useDropFlash, useGait } from '../game/motion'
import { chunkAt, frameOf, screenToWorld, worldToScreen, type Chunk } from '../game/camera'
import { nodeFocus, npcFocus, npcMark, portalFocus } from '../game/npc'
import { useGame } from '../stores/game'

const game = useGame()
const board = ref<HTMLElement | null>(null)
const view = ref({ w: 960, h: 540 })
/** 点地后的一圈反馈，只记屏幕位置，镜头仍跟着人。 */
const clickFx = ref<{ x: number; y: number; id: number } | null>(null)
/** 技能出手时，自己脚下闪一圈。光效关掉时不会触发。 */
const casting = ref(false)
let observer: ResizeObserver | null = null
let clickTimer = 0
let castFxTimer = 0

onMounted(() => {
  const el = board.value
  if (!el || typeof ResizeObserver === 'undefined') return
  const apply = () => {
    view.value = { w: el.clientWidth || 960, h: el.clientHeight || 540 }
  }
  apply()
  observer = new ResizeObserver(apply)
  observer.observe(el)
})
onUnmounted(() => {
  observer?.disconnect()
  window.clearTimeout(clickTimer)
  window.clearTimeout(veilTimer)
  window.clearTimeout(castFxTimer)
})

const mapId = computed(() => game.snap?.mapId || game.profile?.mapId || 'town_starfall')
const map = computed(() => game.config?.maps?.find((item: { id: string }) => item.id === mapId.value))
const self = computed(() => game.snap?.self ?? { x: game.profile?.x ?? 0, y: game.profile?.y ?? 0, hp: 0, maxHp: 0, mp: 0, maxMp: 0, worldX: undefined as number | undefined, worldY: undefined as number | undefined, originX: 0, originY: 0 })
const pet = computed(() => {
  const pets = game.profile?.pets ?? []
  const beast = pets.find((item: { deployed?: boolean }) => item.deployed)
  if (beast) return { ...beast, tag: '出战' }
  const carried = pets.find((item: { active?: boolean }) => item.active)
  if (carried) return { ...carried, tag: carried.role === 'pet' ? '携带' : '光环' }
  return null
})
const hpPct = computed(() => (self.value.maxHp ? Math.max(0, (self.value.hp / self.value.maxHp) * 100) : 0))
const hpHurt = useDropFlash(() => Number(self.value?.hp))
const mpPct = computed(() => (self.value.maxMp ? Math.max(0, (self.value.mp / self.value.maxMp) * 100) : 0))
const expPct = computed(() => (game.profile?.expMax ? Math.max(0, (Number(game.profile.exp) / Number(game.profile.expMax)) * 100) : 0))
const periodId = computed(() => String(game.profile?.period || 'day'))
const weatherId = computed(() => String(game.profile?.weather?.id || 'clear'))
watch(() => game.castSeq, (n) => {
  if (!n) return
  casting.value = true
  window.clearTimeout(castFxTimer)
  castFxTimer = window.setTimeout(() => {
    casting.value = false
  }, 520)
})

const chunks = computed<Chunk[]>(() => {
  const raw = game.snap?.chunks
  if (Array.isArray(raw) && raw.length) {
    return raw.map((chunk: Chunk) => ({
      id: chunk.id,
      name: chunk.name,
      kind: chunk.kind,
      width: Number(chunk.width) || 1,
      height: Number(chunk.height) || 1,
      originX: Number(chunk.originX) || 0,
      originY: Number(chunk.originY) || 0,
    }))
  }
  if (!map.value) return []
  return [{
    id: map.value.id,
    name: map.value.name,
    kind: map.value.kind,
    width: map.value.width || 1,
    height: map.value.height || 1,
    originX: Number(self.value.originX) || 0,
    originY: Number(self.value.originY) || 0,
  }]
})
const hereChunk = computed(() => chunks.value.find((chunk) => chunk.id === mapId.value) ?? chunks.value[0])
const selfWorld = computed(() => {
  if (self.value.worldX != null && self.value.worldY != null) return { x: Number(self.value.worldX), y: Number(self.value.worldY) }
  const origin = hereChunk.value
  return { x: (origin?.originX ?? 0) + Number(self.value.x), y: (origin?.originY ?? 0) + Number(self.value.y) }
})
/** 走路时造型跟着位移转，镜头计算不动。 */
const facing = ref<Face>('down')
const walking = computed(() => {
  const kind = game.intent?.kind
  return kind === 'move' || kind === 'fight' || kind === 'capture'
})
const veilKey = ref(0)
let veilTimer = 0
watch(() => `${Math.round(selfWorld.value.x * 10)}:${Math.round(selfWorld.value.y * 10)}`, (next, prev) => {
  if (!prev) return
  const [x, y] = next.split(':').map(Number)
  const [px, py] = prev.split(':').map(Number)
  const dx = x - px
  const dy = y - py
  if (Math.abs(dx) + Math.abs(dy) < 3) return
  facing.value = Math.abs(dx) > Math.abs(dy) ? (dx > 0 ? 'right' : 'left') : (dy > 0 ? 'down' : 'up')
})
watch(mapId, (next, prev) => {
  if (!prev || next === prev) return
  veilKey.value = Date.now()
  window.clearTimeout(veilTimer)
  veilTimer = window.setTimeout(() => {
    veilKey.value = 0
  }, 700)
})
const frame = computed(() => frameOf(selfWorld.value.x, selfWorld.value.y, view.value.w, view.value.h, Number(game.snap?.camera) || 1100))
const drawChunks = computed(() => [...chunks.value].sort((a, b) => Number(a.id === mapId.value) - Number(b.id === mapId.value)))

function chunkBox(chunk: Chunk) {
  const point = worldToScreen(frame.value, chunk.originX, chunk.originY)
  return {
    left: `${point.x}px`,
    top: `${point.y}px`,
    width: `${chunk.width * frame.value.scale}px`,
    height: `${chunk.height * frame.value.scale}px`,
  }
}
function worldOf(id: string, x: number, y: number) {
  const chunk = chunks.value.find((item) => item.id === id)
  if (!chunk) return { x, y }
  return { x: chunk.originX + x, y: chunk.originY + y }
}
function onScreen(wx: number, wy: number) {
  const point = worldToScreen(frame.value, wx, wy)
  return point.x >= -80 && point.y >= -80 && point.x <= frame.value.viewW + 80 && point.y <= frame.value.viewH + 80
}

const monsters = computed(() => (game.snap?.entities ?? []).filter((entity: { kind: string; alive: boolean; seam?: boolean; afar?: boolean; mapId?: string }) => entity.kind === 'monster' && entity.alive && !entity.seam && !entity.afar && (!entity.mapId || entity.mapId === mapId.value)))
const afar = computed(() => (game.snap?.entities ?? []).filter((entity: { afar?: boolean; seam?: boolean }) => entity.afar && !entity.seam))
const people = computed(() => (game.snap?.entities ?? []).filter((entity: { kind: string; seam?: boolean; afar?: boolean; mapId?: string }) => entity.kind === 'player' && !entity.seam && !entity.afar && (!entity.mapId || entity.mapId === mapId.value)))
const stalls = computed(() => (game.snap?.entities ?? []).filter((entity: { kind: string; seam?: boolean; afar?: boolean; mapId?: string }) => entity.kind === 'stall' && !entity.seam && !entity.afar && (!entity.mapId || entity.mapId === mapId.value)))
const packs = computed(() => nearestPacks(monsters.value, self.value))
const guideKey = computed(() => questGuide(game.profile?.quests ?? [], game.config, mapId.value, packs.value)?.key ?? '')
const selfLook = computed(() => lookForSelf(game.profile?.weaponType))
/** 怪物和旁边的旅人走路时换朝向。自己仍看点击后的位移。 */
const gait = useGait(() => {
  const list: { id: string; x: number; y: number }[] = []
  for (const pack of packs.value) list.push({ id: pack.templateId, x: pack.x, y: pack.y })
  for (const person of people.value) list.push({ id: String(person.id), x: Number(person.x), y: Number(person.y) })
  return list
})
function tintOf(id?: string) {
  if (!id) return undefined
  return game.config?.monsters?.find((item: { id: string; tint?: string }) => item.id === id)?.tint as string | undefined
}
function actorLook(spot: Spot) {
  if (spot.kind === 'npc' && spot.npc) return lookForNpc(spot.npc.id, spot.npc.title, spot.npc.panels)
  if (spot.kind === 'player') return lookForTraveler(spot.id || spot.name, spot.weapon)
  return lookForMonster(spot.templateId || '', spot.role, tintOf(spot.templateId))
}
function actorFace(spot: Spot): Face {
  const key = spot.kind === 'player' ? spot.id || '' : spot.templateId || spot.key
  return gait.value[key]?.face || (spot.kind === 'npc' ? 'down' : 'right')
}
function actorStep(spot: Spot) {
  const key = spot.kind === 'player' ? spot.id || '' : spot.templateId || spot.key
  return Boolean(gait.value[key]?.step)
}
function figured(kind: string) {
  return kind === 'npc' || kind === 'pack' || kind === 'monster' || kind === 'player'
}

const configOf = (id: string) => game.config?.maps?.find((item: { id: string }) => item.id === id)

interface Spot {
  key: string
  kind: string
  name: string
  wx: number
  wy: number
  x: number
  y: number
  mapId: string
  badge: string
  id?: string
  focus?: string
  stallId?: string
  count?: number
  npc?: { id: string; name: string; title?: string; panels?: string[] }
  templateId?: string
  role?: string
  weapon?: string
}

/** 名字牌左侧的小标：等级、职责或类型，不另占一行。 */
function roleBadge(role: string | undefined, templateId: string) {
  const row = game.config?.monsters?.find((item: { id: string; level?: number }) => item.id === templateId)
  const level = row?.level ? String(row.level) : ''
  if (role === 'boss') return level ? `首${level}` : '首'
  if (role === 'elite') return level ? `精${level}` : '精'
  if (role === 'world') return level ? `稀${level}` : '稀'
  return level || '怪'
}
function shortTitle(title?: string) {
  const text = String(title || '').trim()
  if (!text) return '人'
  return [...text].slice(0, 2).join('')
}
function plateBadge(kind: string, extra: { role?: string; templateId?: string; level?: unknown; title?: string }) {
  if (kind === 'portal') return '门'
  if (kind === 'node') return '采'
  if (kind === 'stall') return '摊'
  if (kind === 'npc') return shortTitle(extra.title)
  if (kind === 'player') return extra.level ? String(extra.level) : '旅'
  if (kind === 'pack' || kind === 'monster') return roleBadge(extra.role, extra.templateId || '')
  return ''
}

const spots = computed<Spot[]>(() => {
  const list: Spot[] = []
  const add = (spot: Spot) => {
    if (onScreen(spot.wx, spot.wy)) list.push(spot)
  }
  for (const chunk of chunks.value) {
    const cfg = configOf(chunk.id)
    if (!cfg) continue
    for (const portal of cfg.portals ?? []) {
      const world = worldOf(chunk.id, portal.x, portal.y)
      add({ key: chunk.id === mapId.value ? `p:${portal.label}` : `p:${chunk.id}:${portal.label}`, kind: 'portal', name: portal.label, badge: '门', wx: world.x, wy: world.y, x: portal.x, y: portal.y, mapId: chunk.id, focus: portalFocus(portal.x, portal.y) })
    }
    for (const node of cfg.nodes ?? []) {
      const world = worldOf(chunk.id, node.x, node.y)
      add({ key: node.id, kind: 'node', name: node.name, badge: '采', wx: world.x, wy: world.y, x: node.x, y: node.y, mapId: chunk.id, focus: nodeFocus(node.id) })
    }
  }
  for (const npc of game.config?.npcs ?? []) {
    if (!chunks.value.some((chunk) => chunk.id === npc.mapId)) continue
    const world = worldOf(npc.mapId, npc.x, npc.y)
    add({ key: npc.id, kind: 'npc', name: npc.name, badge: shortTitle(npc.title), wx: world.x, wy: world.y, x: npc.x, y: npc.y, mapId: npc.mapId, focus: npcFocus(npc.id), npc })
  }
  for (const pack of packs.value) {
    const world = worldOf(mapId.value, pack.x, pack.y)
    add({ key: pack.templateId, kind: 'pack', name: pack.name, badge: roleBadge(pack.role, pack.templateId), wx: world.x, wy: world.y, x: pack.x, y: pack.y, mapId: mapId.value, id: pack.id, count: pack.count, templateId: pack.templateId, role: pack.role })
  }
  for (const person of people.value) {
    const world = person.worldX != null ? { x: Number(person.worldX), y: Number(person.worldY) } : worldOf(mapId.value, person.x, person.y)
    add({ key: 'who:' + person.id, kind: 'player', name: person.name, badge: person.level ? String(person.level) : '旅', wx: world.x, wy: world.y, x: person.x, y: person.y, mapId: mapId.value, id: person.id, weapon: person.templateId })
  }
  for (const stall of stalls.value) {
    const world = stall.worldX != null ? { x: Number(stall.worldX), y: Number(stall.worldY) } : worldOf(mapId.value, stall.x, stall.y)
    add({ key: stall.id, kind: 'stall', name: stall.name, badge: '摊', wx: world.x, wy: world.y, x: stall.x, y: stall.y, mapId: mapId.value, stallId: stall.stallId })
  }
  for (const mark of afar.value) {
    const world = mark.worldX != null ? { x: Number(mark.worldX), y: Number(mark.worldY) } : worldOf(mark.mapId, mark.x, mark.y)
    add({ key: 'afar:' + mark.id, kind: mark.kind, name: mark.name, badge: plateBadge(mark.kind, mark), wx: world.x, wy: world.y, x: mark.x, y: mark.y, mapId: mark.mapId, id: mark.id, stallId: mark.stallId, templateId: mark.templateId, role: mark.role, weapon: mark.kind === 'player' ? mark.templateId : undefined })
  }
  return list
})

const laid = computed(() => {
  const marks = spots.value.map((spot) => ({ ...spot, ...worldToScreen(frame.value, spot.wx, spot.wy) }))
  for (let pass = 0; pass < 8; pass++) {
    for (let i = 0; i < marks.length; i++) {
      for (let j = i + 1; j < marks.length; j++) {
        const a = marks[i]!
        const b = marks[j]!
        let dx = b.x - a.x
        let dy = b.y - a.y
        const dist = Math.hypot(dx, dy) || 0.01
        const gap = figured(a.kind) && figured(b.kind) ? 50 : 36
        if (dist >= gap) continue
        const push = (gap - dist) / 2
        dx /= dist
        dy /= dist
        a.x -= dx * push
        a.y -= dy * push
        b.x += dx * push
        b.y += dy * push
      }
    }
  }
  const at: Record<string, { left: string; top: string }> = {}
  for (const mark of marks) {
    at[mark.key] = {
      left: `${Math.min(frame.value.viewW - 8, Math.max(8, mark.x))}px`,
      top: `${Math.min(frame.value.viewH - 8, Math.max(8, mark.y))}px`,
    }
  }
  return at
})

function at(key: string) {
  return laid.value[key] ?? { left: '50%', top: '50%' }
}
function markOf(npc: { id: string; name: string }) {
  return npcMark(npc, game.profile)
}
const hereId = computed(() => {
  let best = ''
  let bestDistance = 90
  for (const spot of spots.value) {
    if (spot.kind === 'pack' || spot.kind === 'player' || spot.kind === 'stall' || spot.kind === 'monster') continue
    const distance = Math.hypot(spot.wx - selfWorld.value.x, spot.wy - selfWorld.value.y)
    if (distance < bestDistance) {
      bestDistance = distance
      best = spot.key
    }
  }
  return best
})
const dest = computed(() => {
  const intent = game.intent
  if (!intent || intent.kind !== 'move') return null
  if (intent.worldX != null && intent.worldY != null && intent.mapId && intent.mapId !== mapId.value) return { x: intent.worldX, y: intent.worldY }
  const origin = hereChunk.value
  if (!origin) return null
  return { x: origin.originX + intent.x, y: origin.originY + intent.y }
})

const mini = computed(() => {
  const aspect = 148 / 98
  const width = frame.value.span * 1.15
  const height = width / aspect
  return { minX: frame.value.camX - width / 2, minY: frame.value.camY - height / 2, width, height }
})
function mm(wx: number, wy: number) {
  const box = mini.value
  return { left: `${((wx - box.minX) / box.width) * 100}%`, top: `${((wy - box.minY) / box.height) * 100}%` }
}
function land(chunk: Chunk) {
  const box = mini.value
  return {
    left: `${((chunk.originX - box.minX) / box.width) * 100}%`,
    top: `${((chunk.originY - box.minY) / box.height) * 100}%`,
    width: `${(chunk.width / box.width) * 100}%`,
    height: `${(chunk.height / box.height) * 100}%`,
  }
}
function pinWorld(wx: number, wy: number) {
  const point = worldToScreen(frame.value, wx, wy)
  return { left: `${point.x}px`, top: `${point.y}px` }
}

function openSpot(spot: Spot) {
  const here = spot.mapId === mapId.value
  if (spot.kind === 'portal' || spot.kind === 'node' || spot.kind === 'npc') {
    if (here) game.go(spot.x, spot.y, 'interact', spot.focus)
    else game.goAcross(spot.wx, spot.wy, spot.mapId, 'interact', spot.focus)
    return
  }
  if (spot.kind === 'pack' || spot.kind === 'monster') {
    if (spot.id) game.fight(spot.id)
    return
  }
  if (spot.kind === 'player') {
    game.callName = spot.name
    void game.act('/api/trade/invite', { name: spot.name })
    return
  }
  if (spot.kind === 'stall') {
    game.stallFocus = spot.stallId || ''
    game.panel = 'stall'
  }
}

/** 靠近本图的门时提示下一片。再近一点就改成按键提示。 */
const gateHint = computed(() => {
  let best: Spot | null = null
  let bestDistance = 220
  for (const spot of spots.value) {
    if (spot.kind !== 'portal' || spot.mapId !== mapId.value) continue
    const distance = Math.hypot(spot.wx - selfWorld.value.x, spot.wy - selfWorld.value.y)
    if (distance < bestDistance) {
      bestDistance = distance
      best = spot
    }
  }
  if (!best) return null
  return { name: best.name, close: bestDistance < 96 }
})

function walk(ev: MouseEvent) {
  const target = ev.target as HTMLElement | null
  if (target?.closest('button')) return
  const el = board.value
  if (!el || !chunks.value.length) return
  const box = el.getBoundingClientRect()
  const localX = ev.clientX - box.left
  const localY = ev.clientY - box.top
  const world = screenToWorld(frame.value, localX, localY)
  const chunk = chunkAt(chunks.value, world.x, world.y, mapId.value)
  if (!chunk) return
  clickFx.value = { x: localX, y: localY, id: Date.now() }
  window.clearTimeout(clickTimer)
  clickTimer = window.setTimeout(() => {
    clickFx.value = null
  }, 460)
  if (chunk.id === mapId.value) game.go(world.x - chunk.originX, world.y - chunk.originY)
  else game.goAcross(world.x, world.y, chunk.id)
}

const floats = computed(() => {
  const origin = hereChunk.value
  return game.floats.map((hit) => {
    const world = origin ? worldOf(mapId.value, hit.x, hit.y) : { x: hit.x, y: hit.y }
    return { ...hit, wx: world.x, wy: world.y }
  })
})
</script>

<template>
  <div class="scene" v-if="map" ref="board" @click="walk">
    <img
      v-for="chunk in drawChunks"
      :key="chunk.id"
      class="chunk-bg"
      :class="{ here: chunk.id === mapId }"
      :src="mapThumb(chunk.id)"
      alt=""
      :style="chunkBox(chunk)"
    />
    <div class="atmo" :class="['atmo-' + periodId, weatherId === 'clear' ? '' : 'sky-' + weatherId]" />
    <div class="hud-plate">
      <ActorFigure class="hud-fig" :look="selfLook" />
      <div>
        <b>{{ game.profile?.name }} <small>Lv.{{ game.profile?.level }}</small></b>
        <small class="hud-gold">{{ game.profile?.gold }} 金</small>
        <div class="hud-meters">
          <span title="生命"><em>血</em><span class="bar" :class="{ hurt: hpHurt }"><i class="hp" :style="{ width: hpPct + '%' }" /></span></span>
          <span title="法力"><em>法</em><span class="bar"><i class="mp" :style="{ width: mpPct + '%' }" /></span></span>
          <span title="经验"><em>经</em><span class="bar"><i class="xp" :style="{ width: expPct + '%' }" /></span></span>
        </div>
      </div>
      <div v-if="pet" class="hud-pet" :title="pet.name">
        <ActorFigure class="hud-fig" :look="lookForPet(pet.templateId)" />
        <small>{{ pet.tag }} {{ pet.name }}</small>
      </div>
    </div>
    <div class="minimap" :title="map.name">
      <i v-for="chunk in chunks" :key="'land' + chunk.id" class="land" :class="{ here: chunk.id === mapId }" :style="land(chunk)" />
      <i class="dot you" :style="mm(selfWorld.x, selfWorld.y)" />
      <i v-if="dest" class="dot dest" :style="mm(dest.x, dest.y)" />
      <i v-for="spot in spots.filter((item) => item.kind === 'portal')" :key="'m' + spot.key" class="dot exit" :style="mm(spot.wx, spot.wy)" />
      <b>{{ map.name }}</b>
    </div>
    <div class="coords">{{ Math.round(self.x) }} , {{ Math.round(self.y) }}<small>世界 {{ Math.round(selfWorld.x) }} , {{ Math.round(selfWorld.y) }}</small></div>
    <div v-if="gateHint" class="gate-banner" :class="{ close: gateHint.close }">
      <em>门</em>
      <span>{{ gateHint.close ? '按 E 进入' : '再走近' }}「{{ gateHint.name }}」</span>
    </div>
    <span v-if="clickFx" :key="clickFx.id" class="click-fx" :style="{ left: clickFx.x + 'px', top: clickFx.y + 'px' }" />
    <div v-if="veilKey" :key="veilKey" class="map-veil" />
    <div class="pin you" :class="{ step: walking, cast: casting }" :style="pinWorld(selfWorld.x, selfWorld.y)">
      <span class="plate-line"><em class="badge">{{ game.profile?.level }}</em><span>你</span></span>
      <ActorFigure :look="selfLook" :facing="facing" :step="walking" />
    </div>
    <div v-if="dest" class="pin dest" :style="pinWorld(dest.x, dest.y)">去</div>
    <span
      v-for="hit in floats"
      :key="hit.id"
      class="float"
      :class="{ crit: hit.crit, heal: hit.kind === 'heal', miss: hit.kind === 'miss' || hit.kind === 'text', guarded: hit.guarded }"
      :style="pinWorld(hit.wx, hit.wy)"
    >{{ hit.text }}</span>
    <button
      v-for="spot in spots"
      :key="spot.key"
      class="tag plate"
      :class="{
        exit: spot.kind === 'portal',
        foe: spot.kind === 'pack' || spot.kind === 'monster',
        mate: spot.kind === 'player',
        figure: figured(spot.kind),
        ['rank-' + (spot.role || '')]: spot.role === 'elite' || spot.role === 'boss' || spot.role === 'world',
        here: hereId === spot.key,
        goal: guideKey === spot.key,
        turnin: spot.npc && markOf(spot.npc) === 'turnin',
        offer: spot.npc && (markOf(spot.npc) === 'offer' || markOf(spot.npc) === 'talk'),
        seam: spot.mapId !== mapId,
      }"
      :style="at(spot.key)"
      @click="openSpot(spot)"
    >
      <span class="plate-line">
        <i v-if="spot.npc && markOf(spot.npc) === 'turnin'" class="qmark">?</i>
        <i v-else-if="spot.npc && (markOf(spot.npc) === 'offer' || markOf(spot.npc) === 'talk')" class="qmark">!</i>
        <em v-if="spot.badge" class="badge">{{ spot.badge }}</em>
        <span class="plate-name">{{ spot.name }}<template v-if="spot.count && spot.count > 1"> ×{{ spot.count }}</template></span>
        <small v-if="spot.kind === 'player'">交易</small><small v-else-if="spot.kind === 'stall'">摊</small><small v-else-if="spot.kind === 'portal'">{{ spot.mapId === mapId ? '进入' : '邻门' }}</small><small v-else-if="spot.mapId !== mapId">界外</small>
      </span>
      <ActorFigure v-if="figured(spot.kind)" :look="actorLook(spot)" :facing="actorFace(spot)" :step="actorStep(spot)" :pip="spot.role === 'elite' || spot.role === 'boss' || spot.role === 'world'" />
    </button>
  </div>
  <div v-else class="scene empty-scene">地图还在加载</div>
</template>
