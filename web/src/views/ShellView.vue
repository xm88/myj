<script setup lang="ts">
/** 冒险主界面：状态栏、附近名册、场景与战斗切换、对话、快捷键、任务引导和聊天。 */

import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api } from '../api'
import ActorFigure from '../components/ActorFigure.vue'
import { lookForMonster, lookForNpc, lookForTraveler } from '../game/look'
import { nearestPacks } from '../game/crowd'
import { syncAmbience } from '../game/ambience'
import { describeQuest, questTracks } from '../game/guide'
import { useDropFlash } from '../game/motion'
import { closestSpot, focusReach, nodeFocus, npcAction, npcFocus, npcHint, npcMark, portalFocus } from '../game/npc'
import AssistantView from '../components/AssistantView.vue'
import BattleBoard from '../components/BattleBoard.vue'
import Panels from '../components/Panels.vue'
import PrimerCard from '../components/PrimerCard.vue'
import SceneBoard from '../components/SceneBoard.vue'
import { useGame } from '../stores/game'

/** 共享游戏状态仓库，组件通过它读取服务端数据和提交操作。 */
const game = useGame()
/** 主界面快捷菜单的展开状态。 */
const menu = ref(false)
/** 操作帮助的显示状态。 */
const help = ref(false)
/** 当前待发送的聊天文本。 */
const draft = ref('')
/** 附近名册当前显示 NPC、怪物或其他玩家。 */
const rosterTab = ref<'people' | 'mobs' | 'players'>('people')

/** 服务器推送的自身战斗快照，缺失时部分视图使用档案坐标。 */
const self = computed(() => game.snap?.self)
/** 优先使用实时快照中的地图，再回退到持久化角色档案。 */
const mapId = computed(() => game.snap?.mapId || game.profile?.mapId || '')
/** 当前地图模板，提供尺寸、背景、资源点与传送门配置。 */
const map = computed(() => game.config?.maps?.find((item: { id: string }) => item.id === mapId.value))
/** 角色当前主战灵伴，用于主界面头像与状态显示。 */
const deployedBeast = computed(() => game.profile?.pets?.find((item: { deployed?: boolean }) => item.deployed) ?? null)
const auraPet = computed(() => game.profile?.pets?.find((item: { active: boolean }) => item.active) ?? null)
/** 将自身当前生命换算成血条显示百分比。 */
const hpPct = computed(() => (self.value ? Math.max(0, (self.value.hp / self.value.maxHp) * 100) : 0))
/** 将自身当前法力换算成法力条显示百分比。 */
const mpPct = computed(() => (self.value ? Math.max(0, (self.value.mp / self.value.maxMp) * 100) : 0))
/** 角色经验相对于升级需求的进度条百分比。 */
const expPct = computed(() => (game.profile ? Math.max(0, (game.profile.exp / game.profile.expMax) * 100) : 0))
/** 自己掉血时，顶栏血条亮一下。 */
const hpHurt = useDropFlash(() => Number(self.value?.hp))
/** 技能栏刚按下去的槽位，只负责出手那一下脉冲。 */
const castKey = ref('')
let castTimer = 0

function whole(value: unknown) {
  const n = Number(value)
  return Number.isFinite(n) ? Math.round(n) : 0
}

function castSkill(slot: number | 'pet') {
  castKey.value = String(slot)
  window.clearTimeout(castTimer)
  castTimer = window.setTimeout(() => {
    castKey.value = ''
  }, 420)
  const cooling = slot !== 'pet' && (skills.value[slot - 1]?.left ?? 0) > 0
  if (!cooling) game.noteCast()
  game.send({ type: 'skill', slot })
}

/** NPC 名称、位置、对话和服务入口的配置清单。 */
const npcs = computed(() => (game.config?.npcs ?? []).filter((npc: { mapId: string }) => npc.mapId === mapId.value))
/** 按任务状态排序后的当前地图 NPC 名册。 */
const people = computed(() => {
  const rank: Record<string, number> = { turnin: 0, talk: 1, offer: 2, progress: 3, '': 4 }
  return [...npcs.value].sort((a: { id: string; name: string }, b: { id: string; name: string }) => (rank[npcMark(a, game.profile)] ?? 4) - (rank[npcMark(b, game.profile)] ?? 4))
})
/** 地图资源点或世界地图固定岛屿坐标，具体形状由所在组件定义。 */
const nodes = computed(() => map.value?.nodes ?? [])
/** 当前地图可通往其他地图的传送门列表。 */
const portals = computed(() => map.value?.portals ?? [])
/** 当前角色档案或快照视图，用于派生面板与位置状态。 */
const who = computed(() => game.snap?.self ?? { x: game.profile?.x ?? 0, y: game.profile?.y ?? 0 })
/** 当前分线快照中仍存活的怪物实例，供聚合、选择和任务引导使用。 */
const monsters = computed(() => (game.snap?.entities ?? []).filter((entity: { kind: string; alive: boolean; seam?: boolean; mapId?: string }) => entity.kind === 'monster' && entity.alive && !entity.seam && (!entity.mapId || entity.mapId === mapId.value)))
/** 按模板聚合的附近怪物，使用最近实例作为操作目标。 */
const packs = computed(() => nearestPacks(monsters.value, who.value))
/** 当前分线快照中其他玩家的显示列表。 */
const players = computed(() => (game.snap?.entities ?? []).filter((entity: { kind: string; seam?: boolean; mapId?: string }) => entity.kind === 'player' && !entity.seam && (!entity.mapId || entity.mapId === mapId.value)))
/** 主界面显示的当前任务进度摘要。 */
const questLine = computed(() => describeQuest(game.profile?.quests ?? []))
/** 右侧追踪的几条任务。点一条就按那一条走。 */
const tracked = computed(() => questTracks(game.profile?.quests ?? [], game.pinnedQuest))
/** 可快捷使用的背包恢复道具，具体效果仍由服务器结算。 */
const quickItems = computed(() => {
  const bag = (game.profile?.bag ?? []).filter((item: { kind: string }) => item.kind === 'consumable')
  const order = ['potion_hp', 'potion_hp2', 'bread', 'potion_mp', 'food_atk', 'food_def']
  const picked = []
  for (const id of order) {
    const item = bag.find((row: { templateId: string }) => row.templateId === id)
    if (item) picked.push(item)
  }
  return picked.slice(0, 2)
})
/** 最近聊天记录倒序展示，保持输入列表的原始顺序。 */
const recentChat = computed(() => [...game.chat].slice(-16).reverse())
/** 当前在提示范围内最近的 NPC、资源点或传送门。 */
const near = computed(() => {
  const spots = [
    ...npcs.value.map((npc: { id: string; name: string; x: number; y: number }) => ({
      focus: npcFocus(npc.id),
      x: npc.x,
      y: npc.y,
      notice: focusReach(npcFocus(npc.id)) + 48,
      prompt: nearPrompt(npc),
    })),
    ...nodes.value.map((node: { id: string; name: string; x: number; y: number }) => ({
      focus: nodeFocus(node.id),
      x: node.x,
      y: node.y,
      notice: focusReach(nodeFocus(node.id)) + 48,
      prompt: `采集${node.name}`,
    })),
    ...portals.value.map((portal: { x: number; y: number; label: string }) => ({
      focus: portalFocus(portal.x, portal.y),
      x: portal.x,
      y: portal.y,
      notice: focusReach(portalFocus(portal.x, portal.y)) + 48,
      prompt: `前往${portal.label}`,
    })),
  ]
  return closestSpot(spots, who.value.x, who.value.y)
})
/** NPC 对话中结构化的任务话题，兼容没有 topics 的旧响应。 */
const dialogTopics = computed(() => (Array.isArray(game.dialog?.topics) ? game.dialog.topics : []))
/** NPC 对话中的商店、制造等功能入口。 */
const dialogServices = computed(() => (game.dialog?.actions ?? []).filter((action: { id: string }) => String(action.id).startsWith('open:')))
/** 当前可通过数字键选择的任务和服务操作。 */
const dialogChoices = computed(() => {
  const quests = dialogTopics.value.filter((topic: { kind: string }) => topic.kind === 'turnin' || topic.kind === 'accept')
  return [...quests, ...dialogServices.value]
})
/** 判断对话是否包含可接取或可交付任务，决定默认按钮提示。 */
const hasQuestChoice = computed(() => dialogTopics.value.some((topic: { kind: string }) => topic.kind === 'turnin' || topic.kind === 'accept'))
/** 当前对话的造型，和地图上是同一套颜色和部件。 */
const bust = computed(() => {
  const id = String(game.dialog?.npcId || '')
  const row = (game.config?.npcs ?? []).find((npc: { id: string }) => npc.id === id)
  return lookForNpc(id, row?.title || game.dialog?.title || '', row?.panels ?? [])
})
/** 天气还要多久换，以及下一候的名字。时间用服务器给的截止点。 */
const weatherSoon = computed(() => {
  const until = Number(game.profile?.weatherUntil)
  if (!Number.isFinite(until)) return ''
  const left = until - Date.now()
  if (left <= 0) return '即将转换'
  const min = Math.max(1, Math.ceil(left / 60000))
  const next = game.profile?.weatherNext ? `转${game.profile.weatherNext}` : '转换'
  return `约 ${min} 分钟后${next}`
})

watch(
  () => [game.ambience, game.profile?.weather?.id, game.profile?.period] as const,
  () => {
    syncAmbience(Boolean(game.ambience), String(game.profile?.weather?.id || 'clear'), String(game.profile?.period || 'day'))
  },
  { immediate: true },
)

/** 时段和天气只染界面，不改地图或战斗。 */
const atmoClass = computed(() => {
  const period = String(game.profile?.period || 'day')
  const sky = String(game.profile?.weather?.id || 'clear')
  return ['atmo-' + period, sky === 'clear' ? '' : 'sky-' + sky]
})

/** 武器技能与灵伴技能模板，客户端只选择技能 ID。 */
const skills = computed(() => {
  const loadout: string[] = game.profile?.loadout ?? []
  const all = game.config?.skills ?? []
  return loadout.map((id: string, index: number) => {
    const skill = all.find((item: { id: string; cooldown?: number }) => item.id === id)
    const left = game.snap?.cooldowns?.[id] ?? 0
    const max = Number(skill?.cooldown) || 0
    const ratio = max > 0 ? Math.max(0, Math.min(1, left / max)) : (left > 0 ? 1 : 0)
    return { index: index + 1, name: skill?.name ?? '技能', left, ratio, mp: skill?.mp ?? 0, desc: skill?.desc ?? '' }
  })
})

/** 主界面功能菜单的 ID、图标和显示名称清单。 */
const panels = [
  ['quest', '任务'],
  ['album', '图鉴'],
  ['bag', '道具'],
  ['pets', '灵兽'],
  ['dispatch', '派遣'],
  ['craft', '制造'],
  ['market', '跳市'],
  ['shop', '商店'],
  ['guild', '工会'],
  ['party', '队伍'],
  ['social', '好友'],
  ['info', '角色'],
  ['travel', '地图'],
  ['stall', '摆摊'],
  ['trade', '交易'],
] as const

/** 主界面优先显示的交互或状态提示。 */
const notice = computed(() => {
  const lines = (game.news ?? []).map((item: { text: string }) => item.text)
  if (!lines.length) return '浮岛旅团：点地图上的地名走路，点右边的「对话」或「战斗」。红字是通往别的地图的路。'
  return lines.join('　　')
})

/** 判断当前是否正在执行移动意图，用于状态与按钮提示。 */
const moving = computed(() => {
  if (game.intent?.kind === 'fight') return '正在靠近目标'
  if (game.intent?.kind === 'capture') return '正在靠近，准备契约'
  if (game.intent?.kind === 'snare') return '正在靠近，准备野捕'
  if (game.intent?.kind === 'move' && game.intent.act === 'interact') return '正在走过去'
  if (game.intent?.kind === 'move') return '赶路中'
  return ''
})

/** 切换指定功能面板，并关闭快捷菜单。 */
function toggle(id: string) {
  game.panel = game.panel === id ? '' : id
  menu.value = false
}

/** 把怪物角色分类转成列表里的首领、精英或稀有标记。 */
function roleName(role: string) {
  if (role === 'boss') return '首领'
  if (role === 'elite') return '精英'
  if (role === 'world') return '稀有'
  return ''
}

/** 从怪物模板查询等级，供附近名册显示。 */
function mobLevel(templateId: string) {
  return game.config?.monsters?.find((item: { id: string }) => item.id === templateId)?.level
}

/** 名册上的怪物和地图用同一种主色。 */
function mobTint(templateId: string) {
  return game.config?.monsters?.find((item: { id: string; tint?: string }) => item.id === templateId)?.tint as string | undefined
}

/** 选中聊天呼叫对象并切换到当前地图频道。 */
function call(name: string) {
  game.callName = name
  game.chatChannel = 'map'
}

/** 把世界、地图或公会频道转换为界面频道标签。 */
function channelTag(channel: string) {
  if (channel === 'world') return '世界'
  if (channel?.startsWith('guild')) return '工会'
  if (channel === 'party' || channel?.startsWith('party')) return '队伍'
  return '区域'
}

/** 把金币收成读得清的数字。 */
function goldText(value: number) {
  return Math.floor(Number(value) || 0).toLocaleString('zh-CN')
}

/** 读取频道历史并合并到本地消息列表，按时间、名字与内容去重。 */
async function loadChannel(channel: string) {
  try {
    const data = await api<{ messages: { player_name: string; text: string; created_at: number }[] }>(`/api/chat?channel=${channel}`)
    const tag = channel === 'world' ? 'world' : channel === 'guild' ? 'guild:' : 'map:'
    for (const item of data.messages) {
      const exists = game.chat.some((line) => line.t === item.created_at && line.text === item.text && line.name === item.player_name)
      if (!exists) game.chat.push({ name: item.player_name, text: item.text, t: item.created_at, channel: tag })
    }
    game.chat = game.chat.slice(-80)
  } catch {
    /* 没进世界时频道会失败，输入时再提示。 */
  }
}

/** 提交当前聊天文本和频道，可加入已选对象的呼叫前缀。 */
function sendChat() {
  const text = draft.value.trim()
  if (!text) return
  const body = game.callName ? `对${game.callName}：${text}` : text
  game.send({ type: 'chat', channel: game.chatChannel, text: body })
  draft.value = ''
}

/** 复用 NPC 任务状态规则，生成主界面名册标记。 */
function mark(npc: { id: string; name: string }) {
  return npcMark(npc, game.profile)
}

/** 按附近 NPC 的任务状态生成交谈、接取或交差提示。 */
function nearPrompt(npc: { id: string; name: string }) {
  const state = mark(npc)
  if (state === 'turnin') return `和${npc.name}交差`
  if (state === 'offer') return `向${npc.name}接任务`
  if (state === 'progress') return `问问${npc.name}`
  return `和${npc.name}说话`
}

/** 为最多九个对话选项生成数字快捷键编号。 */
function choiceNo(id: string) {
  const index = dialogChoices.value.findIndex((item: { id: string }) => item.id === id)
  return index >= 0 && index < 9 ? index + 1 : 0
}

/** 将任务对话话题分类转换成中文状态标签。 */
function topicKind(kind: string) {
  if (kind === 'turnin') return '可以交差'
  if (kind === 'accept') return '可以接受'
  return '进行中'
}

/** 提交接取、交付或服务选项，按服务器响应更新对话和功能面板。 */
async function chooseDialog(actionId: string) {
  const npcId = game.dialog?.npcId
  if (!npcId || !actionId) return
  try {
    const result = await game.act('/api/dialog', { npcId, action: actionId })
    game.dialog = result.dialog ?? null
    if (result.open) game.panel = result.open
  } catch {
    /* 失败原因已经在 toast 里。 */
  }
}

/** 把最近对象转换成移动加精确互动意图，没有目标时显示提示。 */
function interactNear() {
  const spot = near.value
  if (!spot) {
    game.toast('附近没有可以互动的东西')
    return
  }
  game.go(spot.x, spot.y, 'interact', spot.focus)
}

/** 处理快捷键并跳过输入框，避免打字时误触技能和互动。 */
function onKey(ev: KeyboardEvent) {
  const tag = (ev.target as HTMLElement | null)?.tagName?.toLowerCase() ?? ''
  if (tag === 'input' || tag === 'textarea' || tag === 'select') {
    if (ev.key === 'Escape') (ev.target as HTMLElement).blur()
    return
  }
  if (ev.key === 'Escape') {
    game.panel = ''
    game.dialog = null
    help.value = false
    menu.value = false
    return
  }
  if (game.dialog) {
    if (ev.key === 'Enter') {
      const first = dialogChoices.value.find((item: { id: string; kind?: string }) => item.kind === 'turnin' || item.kind === 'accept')
      if (first) {
        ev.preventDefault()
        void chooseDialog(first.id)
      }
      return
    }
    if (ev.key >= '1' && ev.key <= '9') {
      const choice = dialogChoices.value[Number(ev.key) - 1]
      if (choice) {
        ev.preventDefault()
        void chooseDialog(choice.id)
      }
      return
    }
    return
  }
  if (ev.key.toLowerCase() === 'e') {
    ev.preventDefault()
    interactNear()
    return
  }
  if (ev.key.toLowerCase() === 'g') {
    ev.preventDefault()
    game.guard()
    return
  }
  if (ev.key >= '1' && ev.key <= '4') {
    ev.preventDefault()
    castSkill(Number(ev.key))
  }
  if (ev.key.toLowerCase() === 'f') castSkill('pet')
}

/** 已有攻击目标时从地图标题切换回战斗视图。 */
function onMapName() {
  if (self.value?.targetId) game.view = 'battle'
}

/** 按一条任务走。不传标识时跟当前主步骤。 */
function followQuest(questId?: string) {
  game.pursue(questId)
}

/** 优先走回星落镇的传送门，没有可用门时打开世界地图。 */
function goHome() {
  const doors = map.value?.portals ?? []
  const door = doors.find((portal: { to: string }) => portal.to === 'town_starfall') ?? doors[0]
  if (door) game.go(door.x, door.y, 'interact', portalFocus(door.x, door.y))
  else game.panel = 'travel'
}

watch(mapId, (id) => {
  const kind = game.config?.maps?.find((item: { id: string; kind?: string }) => item.id === id)?.kind
  rosterTab.value = kind === 'town' ? 'people' : 'mobs'
})

onMounted(() => {
  window.addEventListener('keydown', onKey)
  void api<{ news: { t: number; text: string }[] }>('/api/news').then((data) => {
    game.news = data.news
  }).catch(() => {})
  void loadChannel('world')
})
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKey)
  window.clearTimeout(castTimer)
})
</script>

<template>
  <!-- 冒险主界面：状态栏、附近名册、场景与战斗切换、对话、快捷键、任务引导和聊天。 -->
  <Transition name="assist">
    <AssistantView v-if="game.mode === 'assistant'" />
  </Transition>
  <div class="desk" :class="[atmoClass, { tucked: game.mode === 'assistant' }]" :inert="game.mode === 'assistant'">
    <header class="mast">
      <img class="mark" src="/art/fi-ui-emblem.jpg?v=q2" alt="" />
      <b class="brand">浮岛旅团</b>
      <div class="menu">
        <button class="btn slim" @click="menu = !menu">功能</button>
        <div v-if="menu" class="menu-pop">
          <button v-for="[id, label] in panels" :key="id" @click="toggle(id)">{{ label }}</button>
        </div>
      </div>
      <button class="btn slim" @click="help = !help">帮助</button>
      <div class="ticker"><span>{{ notice }}</span></div>
      <div class="mast-user" v-if="game.profile">
        <button class="name-link" title="角色和技能" @click="game.panel = 'info'">{{ game.profile.name }}</button>
        <span>Lv.{{ game.profile.level }}</span>
        <span>{{ goldText(game.profile.gold) }} 金币</span>
        <button class="btn slim" @click="game.setMode('assistant')">助手</button>
        <button class="btn slim" @click="game.logout()">离开</button>
      </div>
    </header>

    <div class="mapbar">
      <div class="place">
        <button class="map-name" :title="self?.targetId ? '回到战斗画面' : map?.desc" @click="onMapName">
          {{ map?.name ?? '…' }}
          <small v-if="game.snap?.line">{{ game.snap.line }} 线</small>
        </button>
        <button v-if="map && map.kind !== 'town'" class="linkish home" @click="goHome">回城</button>
        <button v-if="self?.targetId && game.view === 'scene'" class="linkish" type="button" @click="game.view = 'battle'">返回战斗</button>
      </div>
      <div class="map-acts">
        <button v-if="questLine" class="quest-chip" title="点这里去交任务，或走到下一步" @click="followQuest()">{{ questLine }}</button>
        <button v-if="near && !game.dialog" class="near-act" title="按 E" @click="interactNear">{{ near.prompt }}</button>
        <span v-if="moving" class="moving">{{ moving }}</span>
      </div>
      <div class="vitals" :title="'生命 ' + whole(self?.hp) + '/' + whole(self?.maxHp) + '  法力 ' + whole(self?.mp) + '/' + whole(self?.maxMp) + '  经验 ' + whole(game.profile?.exp) + '/' + whole(game.profile?.expMax)">
        <span class="meter hp" :class="{ hurt: hpHurt }"><em>血</em><i><b :style="{ width: hpPct + '%' }" /></i><small>{{ whole(self?.hp) }}/{{ whole(self?.maxHp) }}</small></span>
        <span class="meter mp"><em>法</em><i><b :style="{ width: mpPct + '%' }" /></i><small>{{ whole(self?.mp) }}/{{ whole(self?.maxMp) }}</small></span>
        <span class="meter xp"><em>经</em><i><b :style="{ width: expPct + '%' }" /></i><small>{{ whole(game.profile?.exp) }}/{{ whole(game.profile?.expMax) }}</small></span>
      </div>
      <div class="map-meta">
        <span class="status">出战 {{ deployedBeast ? deployedBeast.name : '无' }}</span>
        <span class="status">光环 {{ auraPet ? auraPet.name : '无' }}</span>
        <span class="weather" :title="(game.profile?.weather?.desc || '') + (weatherSoon ? ' · ' + weatherSoon : '')">
          <i class="sky-orb" :class="game.profile?.period || 'day'" />
          {{ game.profile?.weather?.name }} · {{ game.profile?.periodName }}
          <small v-if="weatherSoon">{{ weatherSoon }}</small>
        </span>
        <span class="conn" :class="{ warn: !game.connected }"><i />{{ game.displaced ? '已在其他窗口打开' : game.connected ? '在线' : '重连中' }}</span>
      </div>
    </div>

    <div class="play">
      <BattleBoard v-if="game.view === 'battle'" />
      <SceneBoard v-else />
      <aside class="roster">
        <div v-if="tracked.length" class="quest-track">
          <b class="track-title">任务</b>
          <button v-for="row in tracked" :key="row.id" :class="[row.kind === 'main' ? 'main' : 'side', { ready: row.ready }]" @click="followQuest(row.id)">
            <b><em class="lane">{{ row.kind === 'main' ? '主线' : '支线' }}</em>{{ row.name }}</b>
            <small>{{ row.line }}</small>
            <i class="qtrack"><b :style="{ width: Math.round(row.ratio * 100) + '%' }" /></i>
          </button>
        </div>
        <div class="roster-tabs">
          <button :class="{ on: rosterTab === 'people' }" @click="rosterTab = 'people'">人物 {{ npcs.length + nodes.length }}</button>
          <button :class="{ on: rosterTab === 'mobs' }" @click="rosterTab = 'mobs'">怪物 {{ monsters.length }}</button>
          <button :class="{ on: rosterTab === 'players' }" @click="rosterTab = 'players'">旅人 {{ players.length }}</button>
        </div>
        <template v-if="rosterTab === 'people'">
          <p v-if="!npcs.length && !nodes.length" class="hint">这里没有可以对话的人。</p>
          <div v-for="npc in people" :key="npc.id" class="who">
            <ActorFigure class="roster-fig" :pip="false" :look="lookForNpc(npc.id, npc.title, npc.panels)" />
            <span>
              <b><i v-if="mark(npc) === 'turnin'" class="qmark">?</i><i v-else-if="mark(npc) === 'offer' || mark(npc) === 'talk'" class="qmark">!</i>{{ npc.name }}</b>
              <small>{{ npcHint(npc, game.profile) }}</small>
            </span>
            <button @click="game.go(npc.x, npc.y, 'interact', npcFocus(npc.id))">{{ npcAction(mark(npc)) }}</button>
          </div>
          <div v-for="node in nodes" :key="node.id" class="who">
            <span><b>{{ node.name }}</b><small>采集</small></span>
            <button @click="game.go(node.x, node.y, 'interact', nodeFocus(node.id))">采集</button>
          </div>
        </template>
        <template v-else-if="rosterTab === 'mobs'">
          <p v-if="!packs.length" class="hint">这片地方现在没有怪物。</p>
          <div v-for="pack in packs" :key="pack.templateId" class="who">
            <ActorFigure class="roster-fig" :pip="false" :look="lookForMonster(pack.templateId, pack.role, mobTint(pack.templateId))" />
            <span>
              <b>{{ pack.name }}<template v-if="pack.count > 1"> ×{{ pack.count }}</template></b>
              <small>{{ [roleName(pack.role || ''), (mobLevel(pack.templateId) ?? '?') + '级', pack.capturable ? '可契约' : ''].filter(Boolean).join(' · ') }}</small>
              <i v-if="pack.maxHp" class="hpbar"><b :style="{ width: Math.max(0, ((pack.hp ?? 0) / pack.maxHp) * 100) + '%' }" /></i>
            </span>
            <span class="acts">
              <button class="fight" @click="game.fight(pack.id)">战斗</button>
              <button v-if="pack.capturable" @click="game.capture(pack.id)">契约</button>
              <button v-if="pack.capturable" @click="game.snare(pack.id)">野捕</button>
            </span>
          </div>
        </template>
        <template v-else>
          <p v-if="!players.length" class="hint">旁边没有别的旅人。</p>
          <div v-for="player in players" :key="player.id" class="who">
            <ActorFigure class="roster-fig" :pip="false" :look="lookForTraveler(player.id, player.templateId)" />
            <span><b>{{ player.name }}</b></span>
            <button @click="call(player.name)">交谈</button>
            <button @click="game.act('/api/party/invite', { name: player.name })">组队</button>
          </div>
        </template>
      </aside>

      <Transition name="fade">
        <button v-if="game.panel" class="scrim" aria-label="关闭窗口" @click="game.panel = ''" />
      </Transition>
      <Transition name="sheet">
      <div v-if="game.dialog" class="dialog stage" role="dialog" :aria-label="game.dialog.name">
        <aside class="bust">
          <div class="bust-frame" :style="{ '--wash': bust.coat }">
            <ActorFigure :look="bust" />
          </div>
          <b class="bust-name">{{ game.dialog.name }}</b>
          <small>{{ game.dialog.title }}</small>
        </aside>
        <div class="dialog-col">
          <header>
            <b>对话</b>
            <button class="btn slim" type="button" @click="game.dialog = null">离开</button>
          </header>
          <div class="dialog-body">
            <p class="say">{{ game.dialog.text }}</p>
            <template v-if="Array.isArray(game.dialog.topics)">
              <article v-for="topic in dialogTopics" :key="topic.id" class="topic" :class="topic.kind === 'progress' ? 'pending' : topic.kind">
                <div>
                  <b>
                    {{ topic.name }}
                    <small>{{ topicKind(topic.kind) }}</small>
                  </b>
                  <p>{{ topic.desc }}</p>
                  <p v-if="topic.detail" class="hint">{{ topic.detail }}</p>
                  <p v-if="topic.reward" class="reward">{{ topic.kind === 'progress' ? '做完可得 ' : '奖励 ' }}{{ topic.reward }}</p>
                </div>
              </article>
              <div class="talk-choices">
                <button v-for="topic in dialogTopics.filter((item) => item.kind !== 'progress')" :key="'c' + topic.id" class="talk-choice" :class="{ lantern: topic.kind === 'turnin' }" type="button" @click="chooseDialog(topic.id)">
                  <em v-if="choiceNo(topic.id)" class="key">{{ choiceNo(topic.id) }}</em>
                  <span>{{ topic.kind === 'turnin' ? '交付' : '接受' }} · {{ topic.name }}</span>
                </button>
                <button v-for="action in dialogServices" :key="action.id" class="talk-choice" type="button" @click="chooseDialog(action.id)">
                  <em v-if="choiceNo(action.id)" class="key">{{ choiceNo(action.id) }}</em>
                  <span>{{ action.label }}</span>
                </button>
                <button class="talk-choice leave" type="button" @click="game.dialog = null"><span>离开</span></button>
              </div>
              <p v-if="hasQuestChoice" class="hint">数字键选上面的事，Enter 交付或接受，Esc 离开。</p>
              <p v-else-if="dialogServices.length" class="hint">数字键打开功能，Esc 离开。</p>
            </template>
            <div v-else class="talk-choices">
              <button v-for="action in game.dialog.actions" :key="action.id" class="talk-choice" type="button" @click="chooseDialog(action.id)"><span>{{ action.label }}</span></button>
              <button class="talk-choice leave" type="button" @click="game.dialog = null"><span>离开</span></button>
            </div>
          </div>
        </div>
      </div>
      </Transition>
      <Transition name="sheet">
      <section v-if="help" class="note">
        <header><b>怎么玩</b><button @click="help = false">关闭</button></header>
        <p>画面铺满这一带。左上是你的头像、生命和法力，右上是小地图。点地面会走过去，点名字也一样。地图名和任务在上头，附近的人收在右边的名册里。技能在底下中间，聊天在左下。</p>
        <p>和人说话时，左边是立绘框，台词在羊皮纸上。数字键选事项，Enter 交付或接受，Esc 离开。走近按 E，或点名字后面的按钮。问号是可以交差，叹号是有任务。右边的任务追踪分主线和支线，点一下就会过去。</p>
        <p>点怪物会走近并开战。开战之后按速度轮流出手。旅人、召唤兽和对手分成三列，出手时有飘字和一条短读条。轮到你时中间会亮出「请出手」。可以攻击、放技能、吃药、防御或逃跑。打到七成血可以野捕，三成血再用契约。召唤兽能刻兽纹帖，两只可以合灵。左下角的三步引导可以跳过。</p>
        <p>战斗时如果想先看地图，点「返回地图」。镜头跟着你走过相邻的地块，走到门外就进下一片，不会被拉回入口。副本仍是单独一张。摆摊和当面交易都要站在旁边，成交扣和拍卖一样的旅税。</p>
        <p>点自己的名字，或下面的「角色」，可以看属性和换技能。技能栏一次放 4 个：先点栏位，再点要换上的技能。</p>
        <p>下面一排还有任务、图鉴、道具、宠物、跳市和工会。图鉴里是旅迹、遇见过的异兽、契约过的灵兽，以及踏过的地方。聊天选区域、世界或工会。点别的旅人「交谈」，会在区域频道叫他。</p>
      </section>
      </Transition>
      <PrimerCard />
      <Panels />
      <TransitionGroup name="toast" tag="div" class="toasts">
        <div v-for="item in game.toasts" :key="item.id" class="toast">{{ item.text }}</div>
      </TransitionGroup>
    </div>

    <div class="dock">
    <div class="exp" :title="'经验 ' + (game.profile?.exp ?? 0) + ' / ' + (game.profile?.expMax ?? 0)">
      <em>经验</em>
      <i :style="{ width: expPct + '%' }" />
      <small>{{ whole(game.profile?.exp) }}/{{ whole(game.profile?.expMax) }}</small>
    </div>

    <footer class="hotbar">
      <div class="hot-skills">
        <button
          v-for="skill in skills"
          :key="skill.index"
          class="skill"
          :class="{ cooling: skill.left > 0, cast: castKey === String(skill.index) }"
          :style="skill.left > 0 ? { '--cd': skill.ratio } : undefined"
          :title="skill.index + ' · ' + skill.desc + (skill.mp ? ' · 法力 ' + skill.mp : '')"
          :disabled="skill.left > 0"
          @click="castSkill(skill.index)"
        >
          <small>{{ skill.index }}<template v-if="skill.mp"> · {{ skill.mp }}</template></small>
          <span>{{ skill.name }}</span>
          <b v-if="skill.left > 0" class="cd">{{ Math.ceil(skill.left / 1000) }}</b>
        </button>
        <button class="skill" :class="{ cast: castKey === 'pet' }" title="F 携带者的灵技" @click="castSkill('pet')"><small>F</small><span>灵技</span></button>
        <button class="skill guard" :class="{ on: (self?.guard ?? 0) > 0 }" title="G 这一拍少挨打，也暂不出手" @click="game.guard()"><small>G</small><span>守势</span></button>
        <button class="skill" :class="{ on: game.autoFight }" title="轮到你时自动攻击" @click="game.setAuto(!game.autoFight)"><small>连</small><span>{{ game.autoFight ? '连招中' : '连招' }}</span></button>
        <button v-for="item in quickItems" :key="item.id" class="skill drink" :title="'使用 ' + item.name" @click="game.act('/api/use', { itemId: item.id })">
          <small>×{{ item.count }}</small>
          <span>{{ item.name }}</span>
        </button>
      </div>
      <p class="hint hot-hint">1–4 技能 · F 灵技 · G 守势</p>
      <nav class="hot-quick" aria-label="常用">
        <button class="skill quick" :class="{ on: game.panel === 'bag' }" title="背包" @click="toggle('bag')"><small>包</small><span>背包</span></button>
        <button class="skill quick" :class="{ on: game.panel === 'quest' }" title="任务" @click="toggle('quest')"><small>任</small><span>任务</span></button>
        <button class="skill quick" :class="{ on: game.panel === 'travel' }" title="地图" @click="toggle('travel')"><small>图</small><span>地图</span></button>
        <button class="skill quick" :class="{ on: game.panel === 'album' }" title="旅迹和图鉴" @click="toggle('album')"><small>册</small><span>图鉴</span></button>
        <button class="skill quick" title="助手" @click="game.setMode('assistant')"><small>助</small><span>助手</span></button>
        <button class="skill quick" :class="{ on: game.showFx }" title="施法圈，可关" @click="game.setFx(!game.showFx)"><small>光</small><span>{{ game.showFx ? '光效' : '光关' }}</span></button>
        <button class="skill quick" :class="{ on: game.ambience }" title="场景氛围底噪，可关" @click="game.setAmbience(!game.ambience)"><small>氛</small><span>{{ game.ambience ? '氛围' : '静音' }}</span></button>
      </nav>
    </footer>

    <form class="talk" @submit.prevent="sendChat">
      <div class="logbox">
        <p v-if="!recentChat.length" class="hint">公聊会显示在这里。</p>
        <p v-for="(line, index) in recentChat" :key="line.t + '-' + index">
          <button type="button" class="who-name" @click="call(line.name)">[{{ channelTag(line.channel) }}] {{ line.name }}</button>
          {{ line.text }}
        </p>
      </div>
      <div class="logbox info">
        <p v-if="!game.log.length" class="hint">掉落、任务和系统消息在这里。</p>
        <p v-for="(line, index) in game.log.slice(0, 8)" :key="index">{{ line }}</p>
      </div>
      <div class="composer">
        <select v-model="game.chatChannel" @change="loadChannel(game.chatChannel)">
          <option value="map">区域</option>
          <option value="world">世界</option>
          <option value="party">队伍</option>
          <option value="guild">工会</option>
        </select>
        <button type="button" class="phrase" @click="draft = '求组队'">求组</button>
        <button type="button" class="phrase" @click="draft = '收材料，跳市见'">收货</button>
        <button type="button" class="phrase" @click="draft = '这张图怎么走？'">问路</button>
        <span v-if="game.callName" class="call">对 {{ game.callName }} <button type="button" @click="game.callName = ''">×</button></span>
        <input v-model="draft" maxlength="80" placeholder="说点什么" />
        <button class="btn" type="submit">发送</button>
      </div>
    </form>

    <nav class="funcs">
      <button v-for="[id, label] in panels" :key="id" :class="{ on: game.panel === id }" @click="toggle(id)">
        {{ label }}<em v-if="id === 'social' && game.profile?.unreadMail">{{ game.profile.unreadMail }}</em><em v-if="id === 'info' && game.profile?.unspent">{{ game.profile.unspent }}</em>
      </button>
    </nav>
    </div>
  </div>
</template>
