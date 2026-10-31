<script setup lang="ts">
/** 功能弹窗：统一角色、装备、派遣、制造、拍卖、公会、社交、任务和队伍操作。 */

import { computed, ref, watch } from 'vue'
import { api } from '../api'
import ActorFigure from './ActorFigure.vue'
import IconMedal from './IconMedal.vue'
import WorldMap from './WorldMap.vue'
import StallBoard from './StallBoard.vue'
import TradeBoard from './TradeBoard.vue'
import { itemIcon, portrait, weaponIcon } from '../game/art'
import { lookForPet } from '../game/look'
import { npcFocus, portalFocus } from '../game/npc'
import { useGame } from '../stores/game'

/**
 * 弹窗都是「先点选，再操作」。
 * 格子和列表在 panel-body 里滚动，装备、出战、出发、制作、上架固定在底部，不再盖住图标。
 * pickedId 在不同弹窗里分别指向物品、灵伴或配方，切换弹窗时清空。
 */
const game = useGame()
/** 上架拍卖时提交的整单价格，合法范围由服务器校验。 */
const price = ref(20)
/** 买卖或转移时提交的数量，服务器按物品规则再次校验。 */
const count = ref(1)
/** 创建公会的名称输入。 */
const guildName = ref('')
/** 申请添加好友的目标角色名输入。 */
const friendName = ref('')
/** 向公会捐赠的金币输入，最低值由服务器校验。 */
const donate = ref(50)
/** 当前选择的灵伴派遣时长模板 ID。 */
const durationId = ref('scout')
/** 当前选择的灵伴派遣路线模板 ID。 */
const nodeId = ref('node_green_herb')
/** 背包面板当前显示装备、背包或仓库页签。 */
const bagTab = ref<'gear' | 'bag' | 'bank'>('bag')
/** 背包里再按种类筛一层，方便在材料堆里找装备。 */
const bagFilter = ref<'all' | 'equipment' | 'consumable' | 'material' | 'quest'>('all')
/** 当前选中的物品、灵伴或配方 ID；切换面板时清空。 */
const pickedId = ref<string | null>(null)
/** 制造列表当前选择全部、制造或料理配方。 */
const craftTab = ref<'all' | 'craft' | 'cook'>('all')
/** 社交面板当前选择好友或邮件列表。 */
const socialTab = ref<'friend' | 'mail'>('friend')
/** 组队邀请的目标角色名输入。 */
const partyName = ref('')
/** 拍卖面板当前显示浏览订单或上架物品页签。 */
const marketTab = ref<'browse' | 'sell'>('browse')
/** 角色面板当前显示属性、技能或玩法记录。 */
const infoTab = ref<'sheet' | 'skill' | 'log'>('sheet')
/** 当前待替换的技能栏索引，成功选择后自动移到下一栏。 */
const skillSlot = ref(0)
/** 灵兽列表按召唤兽、宠物或全部来看。 */
const petLane = ref<'all' | 'beast' | 'pet'>('all')
/** 合灵时献上的那只。 */
const fuseOffer = ref('')
/** 兽纹帖栏满时要覆掉的旧招。 */
const coverSkill = ref('')

/** 装备部位的固定展示顺序。 */
const slotOrder = ['weapon', 'head', 'chest', 'gloves', 'boots', 'ring'] as const
/** 装备部位对应的中文显示名称。 */
const slotNames: Record<string, string> = { weapon: '武器', head: '头', chest: '衣', gloves: '手', boots: '鞋', ring: '戒' }

/** 功能面板 ID 到窗口标题的映射。 */
const titles: Record<string, string> = {
  bag: '道具',
  pets: '召唤兽与宠物',
  dispatch: '派遣',
  craft: '制造',
  market: '跳市',
  guild: '工会',
  travel: '世界地图',
  social: '好友',
  quest: '任务',
  party: '队伍',
  shop: '商店',
  info: '角色',
  stall: '摆摊',
  trade: '当面交易',
}
/** 根据当前打开的面板选择窗口标题。 */
const title = computed(() => titles[game.panel] ?? '窗口')
/** 窗口标题下的一行说明，只写和这一扇窗有关的事。 */
const kicker = computed(() => {
  const lines: Record<string, string> = {
    bag: '装备、背包和仓库',
    pets: '召唤兽出战进回合，宠物携带给光环',
    quest: '点「前往」会带你走过去',
    stall: '买家支付标价，旅税从卖家手里扣',
    trade: '两边锁定之后才能确认',
  }
  return lines[game.panel] ?? ''
})

/** 从配置中选择提供商店货物的 NPC。 */
const shopNpc = computed(() => (game.config?.npcs ?? []).find((npc: any) => npc.shop?.length))
/** 把商店价格与物品模板的名称、说明合成商品列表。 */
const shopGoods = computed(() => {
  const items = game.config?.items ?? []
  return (shopNpc.value?.shop ?? []).map((good: { itemId: string; price: number }) => {
    const item = items.find((row: { id: string; sell?: number }) => row.id === good.itemId)
    return { ...good, name: item?.name ?? good.itemId, desc: item?.desc ?? '', sell: item?.sell ?? 0 }
  })
})
/** 背包里某一模板一共有多少个。 */
function ownedCount(templateId: string) {
  return (game.profile?.bag ?? []).filter((item: { templateId: string }) => item.templateId === templateId).reduce((sum: number, item: { count: number }) => sum + Number(item.count || 0), 0)
}

/** 打开拍卖、社交、公会或队伍面板时按需读取列表数据。 */
async function loadSide() {
  if (game.panel === 'market') game.market = (await api<{ orders: any[] }>('/api/market')).orders
  if (game.panel === 'social') {
    game.mail = (await api<{ mail: any[] }>('/api/mail')).mail
    game.friends = (await api<{ friends: any[] }>('/api/friends')).friends
  }
  if (game.panel === 'guild') game.guilds = (await api<{ guilds: any[] }>('/api/guilds')).guilds
  if (game.panel === 'party') game.party = (await api<{ party: any }>('/api/party')).party
}

/** 将技能元素标识转换成属性面板里的中文说明。 */
const elements: Record<string, string> = {
  physical: '物理',
  fire: '火',
  water: '水',
  thunder: '雷',
  wind: '风',
  earth: '土',
}

/** 当前角色档案或快照视图，用于派生面板与位置状态。 */
const who = computed(() => game.profile)
/** 从角色当前地图 ID 查找中文地图名称。 */
const mapName = computed(() => game.config?.maps?.find((item: { id: string }) => item.id === who.value?.mapId)?.name ?? '')
/** 当前武器允许放入技能栏的技能模板列表。 */
const weaponSkills = computed(() => (game.config?.skills ?? []).filter((skill: { weapon?: string }) => skill.weapon === who.value?.weaponType))
/** 按角色技能栏 ID 顺序解析技能模板，保持栏位对应关系。 */
const barSkills = computed(() => (who.value?.loadout ?? []).map((id: string) => (game.config?.skills ?? []).find((skill: { id: string }) => skill.id === id)))
/** 任务按主线、支线分成两栏。空的一栏不显示。 */
function questLanes<T extends { kind?: string }>(list: T[] | undefined) {
  const rows = list ?? []
  return [
    { id: 'main' as const, label: '主线', rows: rows.filter((quest) => quest.kind === 'main') },
    { id: 'side' as const, label: '支线', rows: rows.filter((quest) => quest.kind !== 'main') },
  ].filter((lane) => lane.rows.length)
}
/** 当前设为主战的灵伴，面板展示与技能入口共用。 */
const activePet = computed(() => who.value?.pets?.find((pet: { active: boolean }) => pet.active) ?? null)
/** 累计五项自行分配的潜力，用于角色属性展示。 */
const spentPoints = computed(() => {
  const attrs = who.value?.attributes
  if (!attrs) return 0
  return (attrs.vit ?? 0) + (attrs.mag ?? 0) + (attrs.str ?? 0) + (attrs.agi ?? 0) + (attrs.end ?? 0)
})
/** 将服务器战斗面板整理成伤害、防御、暴击等成对展示行。 */
const combatRows = computed(() => {
  const stats = who.value?.stats
  if (!stats) return []
  return [
    ['伤害', stats.atk, '灵力', Math.round((stats.spell ?? 0) * 10) / 10],
    ['防御', stats.def, '速度', stats.speed],
    ['暴击', `${Math.round(stats.critRate * 1000) / 10}%`, '出手', `${Math.round(stats.attackInterval / 100) / 10} 秒`],
  ]
})

/** 将当前值与上限转换为零到百分之百的界面进度条宽度。 */
function barWidth(current: number, max: number) {
  if (!max) return '0%'
  return `${Math.min(100, Math.max(0, (current / max) * 100))}%`
}
/** 按固定顺序展示武器、采集、制造与料理熟练。 */
const lifeTracks = computed(() => {
  const tracks = who.value?.tracks
  if (!tracks) return []
  return [
    ['武器熟练', tracks.weapon],
    ['采集', tracks.gather],
    ['制造', tracks.craft],
    ['料理', tracks.cook],
  ]
})

/** 把熟练经验转换成进度条宽度，缺失上限时显示为空进度。 */
function trackWidth(track: { exp: number; expMax: number } | undefined) {
  if (!track?.expMax) return '0%'
  return `${Math.min(100, (track.exp / track.expMax) * 100)}%`
}

/** 从技能配置组合消耗、冷却、倍率和特殊效果说明。 */
function skillNote(skill: { mp?: number; cooldown?: number; coefficient?: number; aoe?: boolean; healPct?: number; forceCrit?: boolean; tauntMs?: number; element?: string }) {
  const bits = []
  if (skill.mp) bits.push(`法力 ${skill.mp}`)
  if (skill.cooldown) bits.push(`冷却 ${Math.round(skill.cooldown / 100) / 10} 秒`)
  if (skill.coefficient) bits.push(`威力 ${skill.coefficient}`)
  if (skill.element && skill.element !== 'physical') bits.push(elements[skill.element] ?? skill.element)
  if (skill.aoe) bits.push('范围')
  if (skill.healPct) bits.push(`治疗 ${Math.round(skill.healPct * 100)}%`)
  if (skill.forceCrit) bits.push('必定暴击')
  if (skill.tauntMs) bits.push('嘲讽')
  return bits.join(' · ')
}

/** 将增益类型、比例和剩余秒数转换成面板提示。 */
function buffText(buff: { stat: string; value: number; until: number }) {
  const name: Record<string, string> = { atkPct: '攻击', defPct: '防御', critRate: '暴击', speedPct: '移速' }
  const left = Math.max(0, Math.ceil((buff.until - Date.now()) / 1000))
  return `${name[buff.stat] ?? buff.stat} +${Math.round(buff.value * 1000) / 10}% · ${left} 秒`
}

/** 把技能放入当前选中栏位；已有技能通过互换位置保持四栏不重复。 */
async function putSkill(skillId: string) {
  const ids = [...(who.value?.loadout ?? [])]
  if (ids.length !== 4) return
  const from = ids.indexOf(skillId)
  const to = skillSlot.value
  if (from === to) return
  if (from >= 0) ids[from] = ids[to]
  ids[to] = skillId
  await game.act('/api/skills', { ids })
  skillSlot.value = (to + 1) % 4
}

/** 打开背包面板并切到已装备物品页签。 */
function openGear() {
  bagTab.value = 'gear'
  game.panel = 'bag'
}

/** 向服务器申请分配一点指定属性，余额与面板由响应更新。 */
function addAttr(key: string) {
  return game.act('/api/attributes', { [key]: 1 })
}

/** 申请重置潜力分配，客户端读取服务器返回的新余额。 */
function washAttrs() {
  return game.act('/api/attributes/reset', {})
}

watch(() => game.panel, (panel) => {
  pickedId.value = null
  if (panel === 'info') infoTab.value = 'sheet'
  if (panel === 'pets') {
    const pets = game.profile?.pets ?? []
    const deployed = pets.find((pet: any) => pet.deployed)
    const active = pets.find((pet: any) => pet.active)
    pickedId.value = deployed?.id ?? active?.id ?? pets[0]?.id ?? null
  }
  void loadSide()
})

/** 将装备品质标识转换为统一的颜色样式类。 */
function qualityClass(quality: string | null) {
  return quality ? `q-${quality}` : ''
}

/** 按当前筛选显示背包格子。 */
const shownBag = computed(() => {
  const bag = game.profile?.bag ?? []
  if (bagFilter.value === 'all') return bag
  if (bagFilter.value === 'quest') return bag.filter((item: { bound?: boolean; kind?: string }) => item.bound || item.kind === 'quest')
  return bag.filter((item: { kind?: string; bound?: boolean }) => item.kind === bagFilter.value && !(bagFilter.value === 'material' && item.bound))
})

/** 把属性行收成对照用的数字。百分比和元素按原值相加。 */
function statTotals(item: { lines?: { stat: string; value: number }[] } | null) {
  const totals: Record<string, number> = {}
  for (const line of item?.lines ?? []) totals[line.stat] = (totals[line.stat] ?? 0) + Number(line.value)
  return totals
}

/** 属性在对比里怎么读。百分比单独标出来。 */
function formatDelta(stat: string, name: string, value: number) {
  const rounded = Math.round(value * 10) / 10
  const head = `${rounded > 0 ? '+' : ''}${rounded}`
  if (stat.endsWith('Pct') || stat.startsWith('elem_') || stat === 'critRate' || stat === 'critDmg') {
    return `${name} ${rounded > 0 ? '+' : ''}${Math.round(value * 1000) / 10}%`
  }
  return `${name} ${head}`
}

/** 从装备、背包和仓库中解析当前选中的物品实例。 */
const picked = computed(() => {
  const id = pickedId.value
  if (!id || !game.profile) return null
  const worn = Object.values(game.profile.equipment ?? {}).find((item: any) => item?.id === id)
  return worn || game.profile.bag?.find((item: any) => item.id === id) || game.profile.bank?.find((item: any) => item.id === id) || null
})

/** 和同一部位身上那件比。正数是换上会多出来的。 */
const compareRows = computed(() => {
  const item = picked.value
  if (!item || item.kind !== 'equipment' || item.location === 'equipped') return []
  const worn = game.profile?.equipment?.[item.slot]
  const next = statTotals(item)
  const prev = statTotals(worn)
  const labels: Record<string, string> = { atk: '攻击', def: '防御', hp: '生命', mp: '法力', speed: '速度', critRate: '暴击', critDmg: '暴伤' }
  const names: Record<string, string> = {}
  for (const line of [...(worn?.lines ?? []), ...(item.lines ?? [])]) names[line.stat] = labels[line.stat] || line.name
  const stats = new Set([...Object.keys(next), ...Object.keys(prev)])
  const rows = []
  for (const stat of stats) {
    const delta = Math.round(((next[stat] ?? 0) - (prev[stat] ?? 0)) * 10) / 10
    if (!delta) continue
    rows.push({ stat, up: delta > 0, text: formatDelta(stat, names[stat] || stat, delta) })
  }
  return rows
})
/** 选中的武器如果会换套路，给一句提醒。 */
const weaponShift = computed(() => {
  const item = picked.value
  if (!item || item.slot !== 'weapon' || item.location === 'equipped' || !item.weaponType) return ''
  if (item.weaponType === game.profile?.weaponType) return ''
  return '换上后，技能会改成这把武器的套路'
})

/** 根据当前选中 ID 查找角色拥有的灵兽实例。 */
const pickedPet = computed(() => game.profile?.pets?.find((pet: any) => pet.id === pickedId.value) ?? null)
const deployedBeast = computed(() => game.profile?.pets?.find((pet: any) => pet.role === 'beast' && pet.deployed) ?? null)
const carriedPet = computed(() => game.profile?.pets?.find((pet: any) => pet.active) ?? null)
const shownPets = computed(() => {
  const pets = game.profile?.pets ?? []
  if (petLane.value === 'all') return pets
  return pets.filter((pet: any) => pet.role === petLane.value)
})
function bagOf(templateId: string) {
  return (game.profile?.bag ?? []).find((item: { templateId: string }) => item.templateId === templateId)
}
const slips = computed(() => {
  const defs = game.config?.items ?? []
  const names = new Map((game.config?.skills ?? []).map((skill: { id: string; name: string }) => [skill.id, skill.name]))
  return (game.profile?.bag ?? []).flatMap((item: { id: string; templateId: string; name: string; count: number }) => {
    const teach = defs.find((row: { id: string; teach?: string }) => row.id === item.templateId)?.teach
    if (!teach) return []
    return [{ ...item, teach, skillName: names.get(teach) || item.name }]
  })
})
const fuseChoices = computed(() => (game.profile?.pets ?? []).filter((pet: { id: string; role?: string; busy?: boolean }) => pet.role === 'beast' && pet.id !== pickedPet.value?.id && !pet.busy))
function petMark(pet: { busy?: boolean; deployed?: boolean; active?: boolean; role?: string; level?: number }) {
  if (pet.busy) return '派遣中'
  if (pet.deployed) return '出战'
  if (pet.active && pet.role === 'pet') return '携带'
  if (pet.active) return '光环'
  return 'Lv.' + pet.level
}

/** 上架时的旅税和到手金币。标价是整单，不是单价。 */
const listTax = computed(() => {
  const total = Math.floor(Number(price.value) || 0)
  const rate = Number(game.config?.constants?.marketTax ?? 0.05)
  const tax = Math.floor(total * rate)
  return { total, tax, net: total - tax }
})
/** 同类在售里最低的单件价格，给标价做参照。 */
const priceHint = computed(() => {
  const item = picked.value
  if (!item) return ''
  const peers = (game.market ?? []).filter((order: { templateId: string }) => order.templateId === item.templateId)
  if (!peers.length) return '跳市上还没有同类'
  const unit = Math.min(...peers.map((order: { price: number; count: number }) => order.price / Math.max(1, order.count)))
  return `同类最低大约 ${Math.round(unit)} /个`
})

/** 按当前制造或料理页签筛选可显示的配方。 */
const recipes = computed(() => {
  const all = game.config?.recipes ?? []
  if (craftTab.value === 'all') return all
  return all.filter((recipe: any) => recipe.life === craftTab.value)
})

/** 在当前配方列表中解析选中的制造目标。 */
const pickedRecipe = computed(() => recipes.value.find((recipe: any) => recipe.id === pickedId.value) ?? null)
/** 解析选中的派遣路线，显示资源与等级条件。 */
const pickedNode = computed(() => game.config?.dispatchNodes?.find((node: any) => node.id === nodeId.value) ?? null)
/** 解析选中的派遣时长，显示预计时间和奖励倍率。 */
const pickedDuration = computed(() => game.config?.durations?.find((item: any) => item.id === durationId.value) ?? null)

watch(craftTab, () => {
  if (pickedId.value && !recipes.value.some((recipe: any) => recipe.id === pickedId.value)) pickedId.value = null
})

/** 去找发布任务的人。不在这张图上时，先走向能过去的门。 */
function goNpc(id: string) {
  const npc = (game.config?.npcs ?? []).find((item: { id: string }) => item.id === id)
  if (!npc) return
  const mapId = game.snap?.mapId || game.profile?.mapId || ''
  game.panel = ''
  if (npc.mapId === mapId) {
    game.go(npc.x, npc.y, 'interact', npcFocus(npc.id))
    return
  }
  const here = (game.config?.maps ?? []).find((item: { id: string }) => item.id === mapId)
  const door = here?.portals?.find((portal: { to: string }) => portal.to === npc.mapId) ?? here?.portals?.find((portal: { to: string }) => portal.to === 'town_starfall')
  if (door) game.go(door.x, door.y, 'interact', portalFocus(door.x, door.y))
  else game.panel = 'travel'
}

function whole(value: unknown) {
  const n = Number(value)
  return Number.isFinite(n) ? Math.round(n) : 0
}

/** 切换当前选中实例，再次点击同一实例时取消选中。 */
function pick(id: string) {
  pickedId.value = pickedId.value === id ? null : id
}

</script>

<template>
  <!-- 功能弹窗：统一角色、装备、派遣、制造、拍卖、公会、社交、任务和队伍操作。 -->
  <Transition name="sheet">
  <section v-if="game.panel" class="panel" :class="['win-' + game.panel, { wide: game.panel === 'travel' }]">
    <div class="panel-head">
      <div class="panel-title">
        <h2>{{ title }}</h2>
        <small v-if="kicker">{{ kicker }}</small>
      </div>
      <button class="btn light" @click="game.panel = ''">关闭</button>
    </div>

    <div v-if="game.panel === 'bag'" class="seg">
      <button :class="{ on: bagTab === 'gear' }" @click="bagTab = 'gear'; pickedId = null">装备</button>
      <button :class="{ on: bagTab === 'bag' }" @click="bagTab = 'bag'; pickedId = null">背包 {{ game.profile.bag.length }}</button>
      <button :class="{ on: bagTab === 'bank' }" @click="bagTab = 'bank'; pickedId = null">仓库 {{ game.profile.bank.length }}</button>
    </div>
    <div v-if="game.panel === 'bag' && bagTab === 'bag'" class="seg thin">
      <button :class="{ on: bagFilter === 'all' }" @click="bagFilter = 'all'">全部</button>
      <button :class="{ on: bagFilter === 'equipment' }" @click="bagFilter = 'equipment'">装备</button>
      <button :class="{ on: bagFilter === 'consumable' }" @click="bagFilter = 'consumable'">药品</button>
      <button :class="{ on: bagFilter === 'material' }" @click="bagFilter = 'material'">材料</button>
      <button :class="{ on: bagFilter === 'quest' }" @click="bagFilter = 'quest'">任务</button>
    </div>
    <div v-else-if="game.panel === 'craft'" class="seg">
      <button :class="{ on: craftTab === 'all' }" @click="craftTab = 'all'">全部</button>
      <button :class="{ on: craftTab === 'craft' }" @click="craftTab = 'craft'">制造</button>
      <button :class="{ on: craftTab === 'cook' }" @click="craftTab = 'cook'">料理</button>
    </div>
    <div v-else-if="game.panel === 'market'" class="seg">
      <button :class="{ on: marketTab === 'browse' }" @click="marketTab = 'browse'; pickedId = null">在售</button>
      <button :class="{ on: marketTab === 'sell' }" @click="marketTab = 'sell'; pickedId = null">我要卖</button>
    </div>
    <div v-else-if="game.panel === 'social'" class="seg">
      <button :class="{ on: socialTab === 'friend' }" @click="socialTab = 'friend'">好友</button>
      <button :class="{ on: socialTab === 'mail' }" @click="socialTab = 'mail'">邮箱</button>
    </div>
    <div v-else-if="game.panel === 'info'" class="seg">
      <button :class="{ on: infoTab === 'sheet' }" @click="infoTab = 'sheet'">资料</button>
      <button :class="{ on: infoTab === 'skill' }" @click="infoTab = 'skill'">技能</button>
      <button :class="{ on: infoTab === 'log' }" @click="infoTab = 'log'">消息</button>
    </div>

    <div class="panel-body">
      <template v-if="game.panel === 'bag'">
        <div v-if="bagTab === 'gear'" class="gear">
          <button
            v-for="slot in slotOrder"
            :key="slot"
            class="slot"
            :class="{ on: game.profile.equipment[slot] && pickedId === game.profile.equipment[slot].id, vacant: !game.profile.equipment[slot] }"
            @click="game.profile.equipment[slot] && pick(game.profile.equipment[slot].id)"
          >
            <IconMedal v-if="game.profile.equipment[slot]" :src="itemIcon(game.profile.equipment[slot].templateId)" />
            <span class="slot-name">{{ game.profile.equipment[slot]?.name ?? slotNames[slot] }}</span>
            <small>{{ game.profile.equipment[slot] ? slotNames[slot] : '空' }}</small>
          </button>
        </div>

        <div v-else-if="bagTab === 'bag'" class="bag-grid">
          <p v-if="!shownBag.length" class="empty">这一类是空的。换个筛选，或去森林打史莱姆。</p>
          <button v-for="item in shownBag" :key="item.id" class="slot" :class="{ on: pickedId === item.id }" @click="pick(item.id)">
            <IconMedal :src="itemIcon(item.templateId)" />
            <span class="slot-name" :class="qualityClass(item.quality)">{{ item.name }}</span>
            <b v-if="item.count > 1" class="qty">{{ item.count }}</b>
          </button>
        </div>

        <div v-else class="bag-grid">
          <p v-if="!game.profile.bank.length" class="empty">仓库是空的。在背包里点一件东西，再选择存仓。</p>
          <button v-for="item in game.profile.bank" :key="item.id" class="slot" :class="{ on: pickedId === item.id }" @click="pick(item.id)">
            <IconMedal :src="itemIcon(item.templateId)" />
            <span class="slot-name">{{ item.name }}</span>
            <b v-if="item.count > 1" class="qty">{{ item.count }}</b>
          </button>
        </div>
      </template>

      <template v-else-if="game.panel === 'pets'">
        <div class="deploy-bar">
          <div class="deploy-slot">
            <small>出战召唤兽</small>
            <div v-if="deployedBeast" class="deploy-who">
              <ActorFigure class="mini-fig" :look="lookForPet(deployedBeast.templateId)" />
              <span>
                <b>{{ deployedBeast.name }}</b>
                <small>Lv.{{ deployedBeast.level }} · 元息 {{ deployedBeast.vigor }}/{{ deployedBeast.vigorMax }}</small>
              </span>
            </div>
            <p v-else class="empty">空着。选一只召唤兽，再按出战。</p>
          </div>
          <div class="deploy-slot">
            <small>携带光环</small>
            <div v-if="carriedPet" class="deploy-who">
              <ActorFigure class="mini-fig" :look="lookForPet(carriedPet.templateId)" />
              <span>
                <b>{{ carriedPet.name }}</b>
                <small>{{ carriedPet.roleName }} · Lv.{{ carriedPet.level }}</small>
              </span>
            </div>
            <p v-else class="empty">还没携带。宠物会给光环，召唤兽也可以顶上。</p>
          </div>
        </div>
        <div class="choice">
          <button type="button" :class="{ on: petLane === 'all' }" @click="petLane = 'all'">全部</button>
          <button type="button" :class="{ on: petLane === 'beast' }" @click="petLane = 'beast'">召唤兽</button>
          <button type="button" :class="{ on: petLane === 'pet' }" @click="petLane = 'pet'">宠物</button>
        </div>
        <p v-if="!game.profile.pets.length" class="empty">还没有灵兽。到翠叶森林把森林猫打到残血，再契约。猫会作为召唤兽出战。</p>
        <p v-else-if="!shownPets.length" class="empty">这一栏是空的。</p>
        <div v-else class="bag-grid">
          <button
            v-for="pet in shownPets"
            :key="pet.id"
            class="slot pet"
            :class="{ on: pickedId === pet.id, busy: pet.busy, beast: pet.role === 'beast', companion: pet.role === 'pet' }"
            @click="pick(pet.id)"
          >
            <ActorFigure class="mini-fig" :look="lookForPet(pet.templateId)" />
            <span class="slot-name">{{ pet.name }}</span>
            <small>{{ pet.roleName }} · {{ petMark(pet) }}</small>
          </button>
        </div>
      </template>

      <template v-else-if="game.panel === 'dispatch'">
        <p v-if="!game.profile.pets.length" class="empty">先契约一只灵伴，才能把它们派出去。</p>
        <template v-else>
          <p class="lede">点一只没出门的灵伴，再选地点和时长。短途侦察只要一分钟。</p>
          <div class="bag-grid">
            <button
              v-for="pet in game.profile.pets"
              :key="pet.id"
              class="slot"
              :class="{ on: pickedId === pet.id, busy: pet.busy }"
              :disabled="pet.busy"
              @click="pick(pet.id)"
            >
              <IconMedal :src="portrait(pet.templateId)" />
              <span class="slot-name">{{ pet.name }}</span>
              <small>{{ pet.busy ? '外出中' : '可派遣' }}</small>
            </button>
          </div>
          <div class="choice">
            <button
              v-for="node in game.config.dispatchNodes"
              :key="node.id"
              :class="{ on: nodeId === node.id }"
              :disabled="game.profile.level < node.unlockLevel"
              @click="nodeId = node.id"
            >
              {{ node.name }}
              <small v-if="game.profile.level < node.unlockLevel">{{ node.unlockLevel }}级</small>
            </button>
          </div>
          <div class="choice">
            <button v-for="item in game.config.durations" :key="item.id" :class="{ on: durationId === item.id }" @click="durationId = item.id">{{ item.name }}</button>
          </div>
        </template>
        <h3>进行中</h3>
        <p v-if="!game.profile.dispatches.length" class="empty">现在没有灵伴在外面。</p>
        <div v-for="job in game.profile.dispatches" :key="job.id" class="item">
          <div>
            <b>{{ job.petName }} · {{ job.nodeName }}</b>
            <small>{{ job.ready ? '已经回来了' : '还在路上' }}</small>
          </div>
          <div class="row">
            <button v-if="game.profile.devTools && !job.ready" class="btn light" @click="game.act('/api/dispatches/finish', { id: job.id })">立刻完成</button>
            <button class="btn lantern" :disabled="!job.ready" @click="game.act('/api/dispatches/claim', { id: job.id })">领取</button>
          </div>
        </div>
      </template>

      <template v-else-if="game.panel === 'craft'">
        <div class="list">
          <button v-for="recipe in recipes" :key="recipe.id" class="item" :class="{ on: pickedId === recipe.id }" @click="pick(recipe.id)">
            <span class="item-main">
              <IconMedal :src="itemIcon(recipe.output.itemId)" />
              <span>
                <b>{{ recipe.name }}</b>
                <small>{{ recipe.life === 'cook' ? '料理' : '制造' }} {{ recipe.skill }} 级 · {{ recipe.gold }} 金币</small>
                <span class="ings">
                  <span v-for="input in recipe.inputs" :key="input.itemId">
                    <IconMedal tiny :src="itemIcon(input.itemId)" />
                    ×{{ input.count }}
                  </span>
                </span>
              </span>
            </span>
          </button>
        </div>
      </template>

      <template v-else-if="game.panel === 'market'">
        <template v-if="marketTab === 'browse'">
          <p class="lede market-board">旅团跳市 · 旅税 {{ Math.round((game.config?.constants?.marketTax ?? 0.05) * 100) }}% · {{ game.market.length }} 单在售</p>
          <p v-if="!game.market.length" class="empty">拍卖行还没有货。先从背包挑一件挂上去。</p>
          <div class="list">
            <div v-for="order in game.market" :key="order.id" class="item lot">
              <div class="item-main">
                <IconMedal :src="itemIcon(order.templateId)" />
                <div>
                  <b :class="qualityClass(order.extra?.quality)">{{ order.name }} ×{{ order.count }}</b>
                  <small>{{ order.sellerName }} · 支付 {{ order.price }} · 约 {{ Math.round(order.price / Math.max(1, order.count)) }}/个 · 卖家到手 {{ order.price - Math.floor(order.price * (game.config?.constants?.marketTax ?? 0.05)) }}</small>
                </div>
              </div>
              <button v-if="order.sellerId === game.profile.id" class="btn light" @click="game.act('/api/market/cancel', { id: order.id }).then(loadSide)">取回</button>
              <button v-else class="btn" @click="game.act('/api/market/buy', { id: order.id }).then(loadSide)">购买</button>
            </div>
          </div>
        </template>
        <template v-else>
          <p v-if="!picked" class="lede">点一件能交易的东西。标价是买家支付的整单，成交扣 5% 旅税。</p>
          <div class="bag-grid">
            <button v-for="item in game.profile.bag.filter((row: any) => !row.bound)" :key="item.id" class="slot" :class="{ on: pickedId === item.id }" @click="pick(item.id)">
              <IconMedal :src="itemIcon(item.templateId)" />
              <span class="slot-name">{{ item.name }}</span>
              <b v-if="item.count > 1" class="qty">{{ item.count }}</b>
            </button>
          </div>
          <p v-if="!game.profile.bag.filter((row: any) => !row.bound).length" class="empty">背包里没有可以上架的东西。</p>
        </template>
      </template>

      <template v-else-if="game.panel === 'guild'">
        <div v-if="game.profile.guild">
          <h3>{{ game.profile.guild.name }} · {{ game.profile.guild.level }} 级</h3>
          <p>{{ game.profile.guild.notice }}</p>
          <div v-for="member in game.profile.guild.members" :key="member.id" class="item">{{ member.name }} · {{ member.role === 'leader' ? '会长' : '成员' }}</div>
          <div class="row" style="margin-top:10px">
            <input v-model.number="donate" type="number" min="50" style="width:100px" />
            <button class="btn" @click="game.act('/api/guilds/donate', { gold: donate })">捐献</button>
            <button class="btn light" @click="game.act('/api/guilds/leave')">离开</button>
          </div>
        </div>
        <div v-else class="row">
          <input v-model="guildName" placeholder="公会名" />
          <button class="btn lantern" @click="game.act('/api/guilds', { name: guildName })">创建（200 金币）</button>
        </div>
        <h3>可加入</h3>
        <p v-if="!game.guilds.length" class="empty">还没有别人的公会。</p>
        <div v-for="guild in game.guilds" :key="guild.id" class="item">
          <span>{{ guild.name }} · {{ guild.level }} 级 · {{ guild.members }} 人 · 会长 {{ guild.leaderName }}</span>
          <button class="btn" @click="game.act('/api/guilds/join', { guildId: guild.id })">加入</button>
        </div>
      </template>

      <template v-else-if="game.panel === 'travel'">
        <WorldMap />
      </template>

      <template v-else-if="game.panel === 'stall'">
        <StallBoard />
      </template>

      <template v-else-if="game.panel === 'trade'">
        <TradeBoard />
      </template>

      <template v-else-if="game.panel === 'quest'">
        <h3>进行中</h3>
        <p v-if="!game.profile.quests.length" class="empty">还没有接任务。去场景里点一个人的「对话」。</p>
        <section v-for="lane in questLanes(game.profile.quests)" :key="'on-' + lane.id" class="quest-lane" :class="lane.id">
          <h3><em>{{ lane.label }}</em></h3>
          <article v-for="quest in lane.rows" :key="quest.id" class="item quest-card" :class="lane.id">
            <div>
              <b>{{ quest.name }}</b>
              <small>{{ lane.label }} · 交给 {{ quest.turnin || '委托人' }}</small>
              <div v-for="(goal, index) in quest.objectives" :key="index" class="qline">
                <span>{{ goal.text }}</span>
                <span>{{ goal.current }}/{{ goal.need }}</span>
                <span class="qbar"><i :style="{ width: Math.min(100, (goal.current / Math.max(1, goal.need)) * 100) + '%' }" /></span>
              </div>
            </div>
            <button class="btn" @click="game.pursue(quest.id); game.panel = ''">前往</button>
          </article>
        </section>
        <h3>可以去接</h3>
        <p v-if="!game.profile.available?.length" class="empty">暂时没有新的委托。先把身上的做完，或者再升一级。</p>
        <section v-for="lane in questLanes(game.profile.available)" :key="'open-' + lane.id" class="quest-lane" :class="lane.id">
          <h3><em>{{ lane.label }}</em></h3>
          <div v-for="quest in lane.rows" :key="quest.id" class="item quest-card" :class="lane.id">
            <div>
              <b>{{ quest.name }}</b>
              <small>{{ lane.label }} · 去找 {{ quest.giver }} 对话</small>
            </div>
            <button class="btn light" @click="goNpc(quest.giverId)">找人</button>
          </div>
        </section>
      </template>

      <template v-else-if="game.panel === 'shop'">
        <p class="lede">杂货由米娜出售。灵契球打到三成血再契约，捕兽索七成血就能野捕。兽纹帖刻指定的一招。</p>
        <div class="list">
          <div v-for="good in shopGoods" :key="good.itemId" class="item">
            <div class="item-main">
              <IconMedal :src="itemIcon(good.itemId)" />
              <div>
                <b>{{ good.name }}</b>
                <small>{{ good.desc }}</small>
                <small>买 {{ good.price }} · 卖店 {{ good.sell || '不收' }} · 身上 {{ ownedCount(good.itemId) }}</small>
              </div>
            </div>
            <button class="btn" :disabled="game.profile.gold < good.price" @click="game.act('/api/shop/buy', { npcId: shopNpc.id, itemId: good.itemId, count: 1 })">买 1 个</button>
          </div>
        </div>
      </template>

      <template v-else-if="game.panel === 'party'">
        <div v-if="game.party">
          <p class="lede">队长带着队伍。古代遗迹会跟队长进同一条线路。</p>
          <div v-for="member in game.party.members" :key="member.id" class="item">
            <span>{{ member.name }}<small v-if="member.id === game.party.leaderId"> · 队长</small></span>
          </div>
          <div class="row" style="margin-top: 10px">
            <button class="btn light" @click="game.act('/api/party/leave', {}).then(loadSide)">离开队伍</button>
          </div>
        </div>
        <p v-else class="empty">你还没有队伍。在场景里点其他旅人的「组队」，或在这里填写名字。</p>
        <div class="row" style="margin-top: 10px">
          <input v-model="partyName" placeholder="对方名字" @keydown.enter="game.act('/api/party/invite', { name: partyName }).then(loadSide)" />
          <button class="btn" @click="game.act('/api/party/invite', { name: partyName }).then(loadSide)">邀请</button>
          <button class="btn light" @click="game.act('/api/party/join', {}).then(loadSide)">接受邀请</button>
        </div>
      </template>

      <template v-else-if="game.panel === 'info' && who">
        <template v-if="infoTab === 'sheet'">
          <div class="mh">
            <div class="mh-top">
              <div class="mh-portrait">
                <img :src="weaponIcon[who.weaponType] || '/art/fi-ui-emblem.jpg?v=q2'" alt="" />
                <small>{{ who.weaponName }}</small>
              </div>
              <div>
                <b class="mh-name">{{ who.name }}</b>
                <div class="mh-meta">
                  <span>{{ who.raceName || '人族' }}</span>
                  <span>等级 {{ who.level }}</span>
                  <span>{{ mapName || '路上' }}</span>
                  <span>{{ who.gold }} 金币</span>
                  <span v-if="who.guild">{{ who.guild.name }}</span>
                  <span v-if="activePet">灵伴 {{ activePet.name }}</span>
                </div>
                <div class="mh-bar">
                  <em>气血</em>
                  <i><b class="hp" :style="{ width: barWidth(who.hp, who.maxHp) }" /></i>
                  <small>{{ whole(who.hp) }}/{{ whole(who.maxHp) }}</small>
                </div>
                <div class="mh-bar">
                  <em>魔法</em>
                  <i><b class="mp" :style="{ width: barWidth(who.mp, who.maxMp) }" /></i>
                  <small>{{ whole(who.mp) }}/{{ whole(who.maxMp) }}</small>
                </div>
                <div class="mh-bar">
                  <em>经验</em>
                  <i><b class="xp" :style="{ width: barWidth(who.exp, who.expMax) }" /></i>
                  <small>{{ whole(who.exp) }}/{{ whole(who.expMax) }}</small>
                </div>
              </div>
            </div>
            <div class="mh-combat">
              <template v-for="(pair, index) in combatRows" :key="index">
                <span><em>{{ pair[0] }}</em><b>{{ pair[1] }}</b></span>
                <span><em>{{ pair[2] }}</em><b>{{ pair[3] }}</b></span>
              </template>
            </div>
            <div class="mh-points">
              <div class="mh-pot">
                <em>潜力</em>
                <b>{{ who.unspent }}</b>
                <small v-if="who.raceGrowth">每级 {{ who.raceGrowth }}</small>
                <button v-if="spentPoints > 0" class="btn light" type="button" @click="washAttrs">洗点</button>
              </div>
              <div v-for="row in who.attributeRows" :key="row.key" class="mh-attr">
                <em>{{ row.name }}</em>
                <button class="plus" type="button" :disabled="!who.unspent" :title="row.hint" @click="addAttr(row.key)">+</button>
                <b>{{ row.value }}</b>
                <small>{{ row.gain || row.hint }}</small>
              </div>
            </div>
          </div>
          <div v-for="[label, track] in lifeTracks" :key="label" class="track">
            <b>{{ label }} {{ track.level }} 级<small> {{ whole(track.exp) }}/{{ whole(track.expMax) }}</small></b>
            <div class="qbar light"><i :style="{ width: trackWidth(track) }" /></div>
          </div>
          <h3>身上</h3>
          <div v-for="slot in slotOrder" :key="slot" class="item">
            <span><small>{{ slotNames[slot] }}</small> {{ who.equipment?.[slot]?.name ?? '空着' }}</span>
          </div>
          <div v-if="who.buffs?.length" class="list">
            <h3>状态</h3>
            <div v-for="buff in who.buffs" :key="buff.id" class="item">{{ buffText(buff) }}</div>
          </div>
          <div class="row" style="margin-top: 8px">
            <button class="btn light" @click="openGear">去换装</button>
          </div>
        </template>
        <template v-else-if="infoTab === 'skill'">
          <p class="lede">现在用的是{{ who.weaponName }}。先点下面的栏位，再点这里的技能。换武器会换成那把武器的默认四招。</p>
          <button v-for="skill in weaponSkills" :key="skill.id" type="button" class="item" @click="putSkill(skill.id)">
            <span>
              <b>{{ skill.name }}<template v-if="who.loadout?.includes(skill.id)"> · 栏位 {{ who.loadout.indexOf(skill.id) + 1 }}</template></b>
              <small>{{ skill.desc }}</small>
              <small>{{ skillNote(skill) }}</small>
            </span>
          </button>
        </template>
        <template v-else>
          <h3>你的消息</h3>
          <p v-if="!game.log.length" class="empty">还没有掉落、任务或系统消息。</p>
          <div v-for="(line, index) in game.log" :key="index" class="item">{{ line }}</div>
          <h3>传闻</h3>
          <p v-if="!game.news.length" class="empty">世界还很安静。</p>
          <div v-for="item in game.news" :key="item.t" class="item">{{ item.text }}</div>
        </template>
      </template>

      <template v-else-if="game.panel === 'social'">
        <template v-if="socialTab === 'friend'">
          <div class="row">
            <input v-model="friendName" placeholder="对方名字" @keydown.enter="api('/api/friends/request', { name: friendName }).then(loadSide)" />
            <button class="btn" @click="api('/api/friends/request', { name: friendName }).then(loadSide)">申请</button>
          </div>
          <p v-if="!game.friends.length" class="empty">还没有好友。</p>
          <div v-for="friend in game.friends" :key="friend.id" class="item">
            <span>{{ friend.name }} · {{ friend.status === 'accepted' ? (friend.online ? '在线' : '离线') : '待处理' }}</span>
            <button v-if="friend.incoming" class="btn" @click="api('/api/friends/accept', { id: friend.id }).then(loadSide)">同意</button>
          </div>
        </template>
        <template v-else>
          <p v-if="!game.mail.length" class="empty">邮箱是空的。</p>
          <div v-for="letter in game.mail" :key="letter.id" class="mail">
            <div><b>{{ letter.title }}</b><div>{{ letter.body }}</div></div>
            <button v-if="!letter.claimed" class="btn" @click="game.act('/api/mail/claim', { id: letter.id }).then(loadSide)">领取</button>
          </div>
        </template>
      </template>
    </div>

    <div v-if="game.panel === 'bag' && picked" class="detail">
      <div class="item-main">
        <IconMedal :src="itemIcon(picked.templateId)" />
        <div>
          <b :class="qualityClass(picked.quality)">{{ picked.name }}</b>
          <small>{{ picked.desc }}</small>
          <small v-for="line in picked.lines ?? []" :key="line.stat + line.name">{{ formatDelta(line.stat, line.name, line.value) }}</small>
          <small v-if="picked.sell > 0">商人收购 {{ picked.sell }} 金币</small>
          <small v-if="weaponShift" class="shift">{{ weaponShift }}</small>
          <small v-for="row in compareRows" :key="row.stat" :class="row.up ? 'up' : 'down'">对比身上 {{ row.text }}</small>
        </div>
      </div>
      <div class="row">
        <button v-if="picked.kind === 'equipment' && picked.location !== 'equipped'" class="btn" @click="game.act('/api/equip', { itemId: picked.id })">装备</button>
        <button v-if="picked.location === 'equipped'" class="btn light" @click="game.act('/api/unequip', { slot: picked.slot }).then(() => pickedId = null)">卸下</button>
        <button v-if="picked.kind === 'consumable' && picked.location === 'bag'" class="btn" @click="game.act('/api/use', { itemId: picked.id })">使用</button>
        <button v-if="picked.location === 'bag' && !picked.bound" class="btn light" @click="game.act('/api/storage', { itemId: picked.id, direction: 'deposit', count: picked.count }).then(() => pickedId = null)">存仓</button>
        <button v-if="picked.location === 'bank'" class="btn" @click="game.act('/api/storage', { itemId: picked.id, direction: 'withdraw', count: picked.count }).then(() => pickedId = null)">取出</button>
        <button v-if="picked.location === 'bag' && !picked.bound && picked.sell > 0" class="btn light" @click="game.act('/api/shop/sell', { itemId: picked.id, count: 1 })">卖 1 个</button>
      </div>
    </div>

    <div v-else-if="game.panel === 'pets' && pickedPet" class="detail">
      <div class="deploy-who">
        <ActorFigure class="detail-fig" :look="lookForPet(pickedPet.templateId)" />
        <div>
          <b>{{ pickedPet.name }} · Lv.{{ pickedPet.level }}/{{ pickedPet.levelCap }}</b>
          <small>{{ pickedPet.roleName }} · {{ pickedPet.battle }} · 派遣 {{ pickedPet.work }}</small>
          <small>长势 {{ Number(pickedPet.growth).toFixed(2) }} · 信赖 {{ pickedPet.loyalty }} · <em class="mood" :class="pickedPet.mood">{{ pickedPet.moodName }}</em> · 元息 {{ pickedPet.vigor }}/{{ pickedPet.vigorMax }}</small>
        </div>
      </div>
      <p>{{ pickedPet.desc }}</p>
      <div class="qbar light"><i :style="{ width: (pickedPet.expMax ? Math.min(100, (pickedPet.exp / pickedPet.expMax) * 100) : 0) + '%' }" /></div>
      <div v-if="pickedPet.roots?.length" class="roots">
        <div v-for="root in pickedPet.roots" :key="root.key" class="root-line">
          <span>{{ root.name }}</span>
          <i class="track"><b :style="{ width: Math.min(100, (root.value / 80) * 100) + '%' }" /></i>
          <em>{{ root.value }}</em>
        </div>
      </div>
      <small v-if="pickedPet.panel">生命 {{ pickedPet.panel.hp }} · 爪击 {{ pickedPet.panel.atk }} · 甲 {{ pickedPet.panel.def }} · 灵息 {{ pickedPet.panel.spell }} · 身法 {{ pickedPet.panel.speed }}</small>
      <div class="skill-pills">
        <em v-for="skill in pickedPet.skills" :key="skill.id" :title="skill.desc">{{ skill.name }}</em>
        <em v-if="!pickedPet.skills?.length">还没有兽技</em>
      </div>
      <div class="row">
        <template v-if="pickedPet.role === 'beast'">
          <button v-if="!pickedPet.deployed" class="btn lantern" :disabled="pickedPet.busy || pickedPet.vigor < 1 || pickedPet.loyalty < 15" @click="game.act('/api/pets/deploy', { petId: pickedPet.id })">出战</button>
          <button v-else class="btn light" @click="game.act('/api/pets/rest', { petId: pickedPet.id })">休息</button>
        </template>
        <button v-if="!pickedPet.active" class="btn" :disabled="pickedPet.busy" @click="game.act('/api/pets/active', { petId: pickedPet.id })">携带</button>
        <button v-else class="btn light" @click="game.act('/api/pets/active', { petId: null })">卸下光环</button>
        <button class="btn" :disabled="!bagOf('beast_grain')" @click="bagOf('beast_grain') && game.act('/api/pets/feed', { petId: pickedPet.id, itemId: bagOf('beast_grain').id })">喂干粮</button>
        <button class="btn" :disabled="!bagOf('vigor_fruit')" @click="bagOf('vigor_fruit') && game.act('/api/pets/feed', { petId: pickedPet.id, itemId: bagOf('vigor_fruit').id })">喂元息果</button>
        <button v-if="pickedPet.role === 'beast'" class="btn" :disabled="!bagOf('beast_seal') || pickedPet.skills.length >= pickedPet.slots" @click="bagOf('beast_seal') && game.act('/api/pets/teach', { petId: pickedPet.id, itemId: bagOf('beast_seal').id })">刻印</button>
      </div>
      <p v-if="pickedPet.role === 'beast'" class="hint">打完一场元息 -1。信赖低于 40 会犹豫，低于 15 不肯出战。喂干粮或元息果会立刻写回。</p>
      <div v-if="pickedPet.role === 'beast' && slips.length" class="slip-row">
        <select v-if="pickedPet.skills.length >= pickedPet.slots" v-model="coverSkill">
          <option value="">栏满了，覆掉哪一招</option>
          <option v-for="skill in pickedPet.skills" :key="skill.id" :value="skill.id">{{ skill.name }}</option>
        </select>
        <button
          v-for="slip in slips"
          :key="slip.id"
          class="btn"
          :disabled="pickedPet.skills.length >= pickedPet.slots && !coverSkill"
          @click="game.act('/api/pets/teach', { petId: pickedPet.id, itemId: slip.id, cover: coverSkill })"
        >{{ slip.skillName }} ×{{ slip.count }}</button>
      </div>
      <div v-if="pickedPet.role === 'beast' && fuseChoices.length" class="slip-row">
        <select v-model="fuseOffer">
          <option value="">合灵：献上一只召唤兽</option>
          <option v-for="pet in fuseChoices" :key="pet.id" :value="pet.id">{{ pet.name }} · 长势 {{ Number(pet.growth).toFixed(2) }}</option>
        </select>
        <button class="btn lantern" :disabled="!fuseOffer" @click="game.act('/api/pets/fuse', { keepId: pickedPet.id, offerId: fuseOffer }).then(() => { fuseOffer = '' }).catch(() => {})">合灵</button>
      </div>
    </div>

    <div v-else-if="game.panel === 'dispatch' && game.profile.pets.length" class="detail">
      <b v-if="pickedPet">{{ pickedPet.name }} · {{ pickedNode?.name }} · {{ pickedDuration?.name }}</b>
      <b v-else>先点一只灵伴</b>
      <div class="row">
        <button class="btn lantern" :disabled="!pickedPet" @click="game.act('/api/dispatches', { petId: pickedId, nodeId, durationId }).then(() => pickedId = null)">出发</button>
      </div>
    </div>

    <div v-else-if="game.panel === 'craft' && pickedRecipe" class="detail">
      <div class="item-main">
        <IconMedal :src="itemIcon(pickedRecipe.output.itemId)" />
        <div>
          <b>{{ pickedRecipe.name }}</b>
          <small>{{ pickedRecipe.life === 'cook' ? '料理' : '制造' }} {{ pickedRecipe.skill }} 级 · 花费 {{ pickedRecipe.gold }} 金币</small>
        </div>
      </div>
      <div class="row">
        <button class="btn lantern" @click="game.act('/api/craft', { recipeId: pickedRecipe.id })">制作</button>
      </div>
    </div>

    <div v-else-if="game.panel === 'info' && infoTab === 'skill'" class="detail">
      <div class="skill-bar">
        <button v-for="(skill, index) in barSkills" :key="index" type="button" :class="{ on: skillSlot === index }" @click="skillSlot = index">
          <small>{{ index + 1 }}</small>
          <b>{{ skill?.name ?? '空' }}</b>
        </button>
      </div>
      <small v-if="barSkills[skillSlot]">{{ barSkills[skillSlot].desc }} · {{ skillNote(barSkills[skillSlot]) }}</small>
    </div>

    <div v-else-if="game.panel === 'market' && marketTab === 'sell' && picked" class="detail">
      <div class="row">
        <label class="field"><span>{{ picked.name }} · 数量</span><input v-model.number="count" type="number" min="1" :max="picked.count" /></label>
        <label class="field"><span>整单价格</span><input v-model.number="price" type="number" min="1" /></label>
        <button class="btn lantern" @click="game.act('/api/market/list', { itemId: picked.id, count, price }).then(() => { pickedId = null; loadSide() })">上架</button>
        <small>买家支付 {{ listTax.total }}，旅税 {{ listTax.tax }}，你到手 {{ listTax.net }}。{{ priceHint }}</small>
      </div>
    </div>
  </section>
  </Transition>
</template>
