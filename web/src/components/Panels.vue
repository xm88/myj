<script setup lang="ts">
/** 功能弹窗：统一角色、装备、派遣、制造、拍卖、公会、社交、任务和队伍操作。 */

import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { api } from '../api'
import ActorFigure from './ActorFigure.vue'
import IconMedal from './IconMedal.vue'
import WorldMap from './WorldMap.vue'
import AlbumBoard from './AlbumBoard.vue'
import StallBoard from './StallBoard.vue'
import TradeBoard from './TradeBoard.vue'
import { itemIcon, portrait, weaponIcon } from '../game/art'
import { lookForPet, lookForSelf, paletteOf, sashOf } from '../game/look'
import { verseLane, verseLanes } from '../game/verse'
import MountGlyph from './MountGlyph.vue'
import { npcFocus, portalFocus } from '../game/npc'
import { companionOrders } from '../game/orders'
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
/** 驿站短信的收件人和正文。不带附件。 */
const noteName = ref('')
const noteBody = ref('')
/** 向公会捐赠的金币输入，最低值由服务器校验。 */
const donate = ref(50)
/** 公会窗口先看堂口，再看名册和可加入的名录。 */
const guildTab = ref<'hall' | 'roster' | 'list'>('hall')
/** 会长正在改的公告。 */
const noticeDraft = ref('')
/** 坐骑预览里点中的那一匹，还没亮出。 */
const mountPick = ref('')
/** 当前选择的灵伴派遣时长模板 ID。 */
const durationId = ref('scout')
/** 当前选择的灵伴派遣路线模板 ID。 */
const nodeId = ref('node_green_herb')
/** 背包面板当前显示装备、背包或仓库页签。 */
const bagTab = ref<'gear' | 'bag' | 'bank'>('bag')
/** 背包里再按种类筛一层，方便在材料堆里找装备。 */
const bagFilter = ref<'all' | 'equipment' | 'consumable' | 'material' | 'quest'>('all')
/** 背包排序。默认保持服务器给的顺序。 */
const bagSort = ref<'shelf' | 'kind' | 'stack'>('shelf')
/** 仓库当前打开的一间。三间共用原来的格子，存仓时进这一间。 */
const bankShelf = ref<'sundry' | 'stock' | 'keeps'>('sundry')
const shelfRows = [
  { id: 'sundry' as const, name: '杂物仓' },
  { id: 'stock' as const, name: '材料仓' },
  { id: 'keeps' as const, name: '珍藏仓' },
]
/** 商店先看货架，也可以直接把背包里能卖的卖掉。 */
const shopLane = ref<'buy' | 'sell'>('buy')
/** 货架再按买得起、药品或材料收一收。 */
const shopFilter = ref<'all' | 'afford' | 'potion' | 'material'>('all')
/** 任务日志按进行中的主线、支线，或只看可接。 */
const questFilter = ref<'all' | 'main' | 'side' | 'open'>('all')
/** 刚刚做成的配方，用来在列表上闪一下。 */
const craftFlash = ref('')
/** 当前选中的物品、灵伴或配方 ID；切换面板时清空。 */
const pickedId = ref<string | null>(null)
/** 制造列表当前选择全部、制造或料理配方。 */
const craftTab = ref<'all' | 'craft' | 'cook' | 'blend'>('all')
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
  guild: '公会',
  mount: '坐骑',
  dye: '染色',
  looks: '衣橱',
  cast: '抛竿',
  trail: '引路',
  dungeon: '遗迹门槛',
  travel: '世界地图',
  social: '好友',
  slate: '名帖',
  quest: '任务',
  party: '队伍',
  shop: '商店',
  info: '角色',
  stall: '摆摊',
  trade: '当面交易',
  album: '旅册',
}
/** 根据当前打开的面板选择窗口标题。 */
const title = computed(() => titles[game.panel] ?? '窗口')
/** 窗口标题下的一行说明，只写和这一扇窗有关的事。 */
const kicker = computed(() => {
  const lines: Record<string, string> = {
    bag: '双击就能穿上或喝掉。仓库分三间，格子还是原来的',
    pets: '召唤兽出战进回合，宠物携带给光环',
    quest: '报酬写在条目上。钉上的任务排到追踪最前',
    shop: '买得出价，也能把背包里收的东西卖给米娜',
    craft: '配方仍看背包。合剂可以连仓库一起算，不涨熟练',
    party: '号令管出手。队长定的目标只告诉队员往哪边走',
    guild: '点卯和捐献只涨公会。战斗和金币收入不跟着变',
    social: '私聊走原来的频道。驿站短信不带金币和东西',
    slate: '一日三件事和挂在名字上的称谓。不加属性',
    mount: '亮出来给自己和旁边的人看。步子还是原来的',
    dye: '只改披风的颜色。攻击、防御和步子不读它',
    looks: '记下三套样子。换上只改衣色、绦带和坐骑预览',
    cast: '星落镇或月光湖抛一次竿。不涨采集熟练',
    trail: '一天互传一句。路谊不加金币、经验和属性',
    dungeon: '先看门槛，再决定进不进。线路和等级门没改',
    stall: '买家支付标价，旅税从卖家手里扣。招牌和架子不改税',
    trade: '两边锁定之后才能确认',
    album: '旅迹、异兽、灵兽，还有踏过的地方',
  }
  return lines[game.panel] ?? ''
})

/** 从配置中选择提供商店货物的 NPC。 */
const shopNpc = computed(() => (game.config?.npcs ?? []).find((npc: any) => npc.shop?.length))
/** 把商店价格与物品模板的名称、说明合成商品列表。 */
const shopGoods = computed(() => {
  const items = game.config?.items ?? []
  return (shopNpc.value?.shop ?? []).map((good: { itemId: string; price: number }) => {
    const item = items.find((row: { id: string; sell?: number; kind?: string }) => row.id === good.itemId)
    return { ...good, name: item?.name ?? good.itemId, desc: item?.desc ?? '', sell: item?.sell ?? 0, kind: item?.kind ?? 'material' }
  })
})
/** 背包里某一模板一共有多少个。 */
function ownedCount(templateId: string) {
  return (game.profile?.bag ?? []).filter((item: { templateId: string }) => item.templateId === templateId).reduce((sum: number, item: { count: number }) => sum + Number(item.count || 0), 0)
}
/** 背包和仓库加在一起。合剂先用背包，不够再动仓库。 */
function heldCount(templateId: string) {
  const piles = [...(game.profile?.bag ?? []), ...(game.profile?.bank ?? [])]
  return piles.filter((item: { templateId: string }) => item.templateId === templateId).reduce((sum: number, item: { count: number }) => sum + Number(item.count || 0), 0)
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

function itemShelf(item: { bin?: string }) {
  return item.bin === 'stock' || item.bin === 'keeps' ? item.bin : 'sundry'
}
function shelfCount(id: string) {
  return (game.profile?.bank ?? []).filter((item: { bin?: string }) => itemShelf(item) === id).length
}
const shelfItems = computed(() => (game.profile?.bank ?? []).filter((item: { bin?: string }) => itemShelf(item) === bankShelf.value))
const shelfLabel = computed(() => shelfRows.find((row) => row.id === bankShelf.value)?.name ?? '杂物仓')
const bankCap = computed(() => Number(game.config?.constants?.bankCap ?? 80))

/** 存、取或在三间仓之间挪。数量按整摞走，不另收费。 */
function storeItem(direction: 'deposit' | 'withdraw' | 'shift', bin?: string) {
  if (!picked.value) return
  const body: { itemId: string; direction: string; count: number; bin?: string } = {
    itemId: picked.value.id,
    direction,
    count: picked.value.count,
  }
  if (direction === 'deposit') body.bin = bankShelf.value
  if (direction === 'shift' && bin) body.bin = bin
  void game.act('/api/storage', body).then(() => {
    pickedId.value = null
  })
}

const dyeRows = computed(() => (game.config?.dyes ?? game.profile?.dyes ?? []) as { id: string; name: string; coat: string; trim: string; hair: string; blurb: string }[])
const dyePick = ref('')
const wornDyeId = computed(() => String(game.profile?.dye || 'tea'))
const shownDye = computed(() => dyeRows.value.find((row) => row.id === (dyePick.value || wornDyeId.value)) ?? dyeRows.value.find((row) => row.id === 'tea'))
const sashRows = computed(() => (game.config?.sashes ?? game.profile?.sashes ?? []) as { id: string; name: string; color: string; blurb: string }[])
const sashPick = ref('')
const wornSashId = computed(() => String(game.profile?.sash || 'plain'))
const shownSash = computed(() => sashRows.value.find((row) => row.id === (sashPick.value || wornSashId.value)) ?? sashRows.value.find((row) => row.id === 'plain'))
const previewLook = computed(() => lookForSelf(game.profile?.weaponType, shownDye.value, sashOf(shownSash.value?.id, sashRows.value)))
const trailName = ref('')
const lookName = ref('')
const lookPick = ref('')
const savedLooks = computed(() => (game.profile?.looks ?? []) as { id: string; name: string; dye?: string; mount?: string; sash?: string; label?: string }[])
const shownSaved = computed(() => savedLooks.value.find((row) => row.id === lookPick.value))
const wardrobeLook = computed(() => {
  const row = shownSaved.value
  const dyeId = row ? (row.dye || 'tea') : wornDyeId.value
  const sashId = sashPick.value || (row ? row.sash : game.profile?.sash)
  return lookForSelf(game.profile?.weaponType, dyeRows.value.find((item) => item.id === dyeId), sashOf(sashId, sashRows.value))
})
const wardrobeMount = computed(() => (shownSaved.value ? shownSaved.value.mount || '' : String(game.profile?.mount || '')))
const clock = ref(Date.now())
let castClock = 0
onMounted(() => {
  castClock = window.setInterval(() => {
    clock.value = Date.now()
  }, 1000)
})
onBeforeUnmount(() => window.clearInterval(castClock))
const castUntil = computed(() => Number(game.snap?.self?.castUntil || game.profile?.castUntil || 0))
const castLeft = computed(() => Math.max(0, castUntil.value - clock.value))
const castReadyNow = computed(() => castLeft.value <= 0)

async function applyDye(id: string) {
  await game.act('/api/dye', { id: id === 'tea' ? '' : id })
  dyePick.value = ''
}

function studied(id: string) {
  return (game.profile?.manual?.studied ?? []).includes(id)
}
async function studySkill(id: string) {
  await game.act('/api/manual', { id })
}
const bookLanes = computed(() => {
  const grouped: Record<string, any[]> = { root: [], guard: [], edge: [] }
  for (const skill of weaponSkills.value) grouped[verseLane(skill)]?.push(skill)
  return verseLanes
    .map((lane) => ({ ...lane, skills: grouped[lane.id] ?? [] }))
    .filter((lane) => lane.skills.length)
})

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

/** 按当前筛选显示背包格子，再按种类或数量排。 */
const shownBag = computed(() => {
  const bag = [...(game.profile?.bag ?? [])]
  const filtered = bagFilter.value === 'all'
    ? bag
    : bagFilter.value === 'quest'
      ? bag.filter((item: { bound?: boolean; kind?: string }) => item.bound || item.kind === 'quest')
      : bag.filter((item: { kind?: string; bound?: boolean }) => item.kind === bagFilter.value && !(bagFilter.value === 'material' && item.bound))
  if (bagSort.value === 'kind') filtered.sort((a: { kind?: string; name?: string }, b: { kind?: string; name?: string }) => String(a.kind).localeCompare(String(b.kind)) || String(a.name).localeCompare(String(b.name), 'zh'))
  if (bagSort.value === 'stack') filtered.sort((a: { count?: number }, b: { count?: number }) => Number(b.count || 0) - Number(a.count || 0))
  return filtered
})
/** 货架按买得起和种类收过之后的商品。 */
const shownGoods = computed(() => shopGoods.value.filter((good: { price: number; kind?: string }) => {
  if (shopFilter.value === 'afford' && Number(game.profile?.gold ?? 0) < good.price) return false
  if (shopFilter.value === 'potion' && good.kind !== 'consumable') return false
  if (shopFilter.value === 'material' && good.kind !== 'material') return false
  return true
}))
/** 商人肯收的背包物品。 */
const sellables = computed(() => (game.profile?.bag ?? []).filter((item: { bound?: boolean; sell?: number }) => !item.bound && Number(item.sell) > 0))
/** 这些消耗品不在背包里双击直接用。 */
const heldBack = new Set(['capture_orb', 'beast_grain', 'vigor_fruit', 'beast_seal'])

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

/** 材料模板的中文名。配置里没有时退回标识。 */
function itemName(id: string) {
  return game.config?.items?.find((row: { id: string; name?: string }) => row.id === id)?.name ?? id
}

/** 金币够不够买这么多个。 */
function canAfford(price: number, count = 1) {
  return Number(game.profile?.gold ?? 0) >= price * count
}

/** 配方现在做不了的原因。空字符串表示可以做。 */
function craftReady(recipe: { life: string; skill: number; gold: number; inputs: { itemId: string; count: number }[] }) {
  const track = game.profile?.tracks?.[recipe.life]
  if (Number(track?.level ?? 1) < recipe.skill) return '等级不够'
  if (!canAfford(recipe.gold)) return '金币不够'
  for (const input of recipe.inputs) {
    if (ownedCount(input.itemId) < input.count) return '材料不够'
  }
  return ''
}

/** 合剂看背包加仓库，不看生活等级。 */
function blendReady(blend: { gold: number; inputs: { itemId: string; count: number }[] }) {
  if (!canAfford(blend.gold)) return '金币不够'
  for (const input of blend.inputs) {
    if (heldCount(input.itemId) < input.count) return '材料不够'
  }
  return ''
}

/** 做成之后在这一行上留一下名字，方便看出刚才出了什么。 */
async function makeRecipe(recipe: { id: string; name: string; life: string; skill: number; gold: number; inputs: { itemId: string; count: number }[] }) {
  const block = craftReady(recipe)
  if (block) {
    game.toast(block)
    return
  }
  await game.act('/api/craft', { recipeId: recipe.id })
  craftFlash.value = recipe.id
}

/** 双击背包：装备直接穿上，普通药品直接喝。 */
async function quickItem(item: { id: string; kind?: string; location?: string; templateId?: string }) {
  pickedId.value = item.id
  if (item.kind === 'equipment' && item.location === 'bag') {
    await game.act('/api/equip', { itemId: item.id })
    return
  }
  if (item.kind === 'consumable' && item.location === 'bag' && item.templateId && !heldBack.has(item.templateId)) {
    await game.act('/api/use', { itemId: item.id })
  }
}

/** 单击选中，连点第二下走快捷。 */
function onBagClick(item: { id: string; kind?: string; location?: string; templateId?: string }, ev: MouseEvent) {
  if (ev.detail >= 2) {
    void quickItem(item)
    return
  }
  pick(item.id)
}

/** 公会经验条：500 一点一级，10 级封顶。和捐献用的是同一条。 */
const guildPace = computed(() => {
  const guild = game.profile?.guild
  if (!guild) return { into: 0, width: '0%', full: false }
  const full = Number(guild.level) >= 10
  const into = full ? 500 : Number(guild.exp) % 500
  return { into, width: `${Math.min(100, (into / 500) * 100)}%`, full }
})
/** 坐骑图鉴。等级不够的只能看剪影说明，不能亮出。 */
const mountRows = computed(() => (game.config?.mounts ?? game.profile?.mounts ?? []) as { id: string; name: string; level: number; blurb: string }[])
const shownMount = computed(() => mountRows.value.find((row) => row.id === (mountPick.value || game.profile?.mount)) ?? mountRows.value[0])
const selfLook = computed(() => lookForSelf(game.profile?.weaponType, paletteOf(game.profile?.dye, dyeRows.value), sashOf(game.profile?.sash, sashRows.value)))
/** 古代遗迹的门槛，怪物名单从地图刷新里收，不另写一份掉落。 */
const ruins = computed(() => (game.config?.maps ?? []).find((item: { id: string }) => item.id === 'dungeon_ruins') as {
  name?: string
  desc?: string
  unlockLevel?: number
  levelMin?: number
  levelMax?: number
  spawns?: { monsterId?: string }[]
} | undefined)
const ruinsFoes = computed(() => {
  const seen = new Set<string>()
  const rows: { id: string; name: string; level?: number; role?: string }[] = []
  for (const spawn of ruins.value?.spawns ?? []) {
    const id = String(spawn.monsterId || '')
    if (!id || seen.has(id)) continue
    seen.add(id)
    const mob = game.config?.monsters?.find((item: { id: string }) => item.id === id) as { name?: string; level?: number; role?: string } | undefined
    rows.push({ id, name: mob?.name ?? id, level: mob?.level, role: mob?.role })
  }
  return rows
})
const ruinsHere = computed(() => (game.snap?.mapId || game.profile?.mapId) === 'dungeon_ruins')
const ruinsTown = computed(() => {
  const id = game.snap?.mapId || game.profile?.mapId
  const row = (game.config?.maps ?? []).find((item: { id: string }) => item.id === id) as { kind?: string } | undefined
  return row?.kind === 'town'
})
const ruinsReady = computed(() => Number(game.profile?.level ?? 1) >= Number(ruins.value?.unlockLevel ?? 25))
function foeMark(role?: string) {
  if (role === 'boss') return '首领'
  if (role === 'elite') return '精英'
  return '普通'
}
async function saveNotice() {
  await game.act('/api/guilds/notice', { text: noticeDraft.value })
}
function openWhisper(name: string) {
  game.whisperName = name
  game.whisperTick += 1
  game.panel = ''
}
async function sendNote() {
  const name = noteName.value.trim()
  const body = noteBody.value.trim()
  if (!name || !body) return
  await game.act('/api/mail/send', { name, body })
  noteBody.value = ''
  await loadSide()
}
async function showMount(id: string) {
  await game.act('/api/mount', { id })
}
async function enterRuins() {
  await game.act('/api/travel', { mapId: ruinsHere.value ? 'town_starfall' : 'dungeon_ruins' })
  game.panel = ''
}

/** 任务列表按主线、支线或可接收一下。 */
function shownLanes<T extends { kind?: string }>(list: T[] | undefined) {
  return questLanes(list).filter((lane) => questFilter.value === 'all' || questFilter.value === 'open' || lane.id === questFilter.value)
}

</script>

<template>
  <!-- 功能弹窗：统一角色、装备、派遣、制造、拍卖、公会、社交、任务和队伍操作。 -->
  <Transition name="sheet">
  <section v-if="game.panel" class="panel" :class="['win-' + game.panel, { wide: game.panel === 'travel' || game.panel === 'album' }]">
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
      <button :class="{ on: bagTab === 'bank' }" @click="bagTab = 'bank'; pickedId = null">仓库 {{ game.profile.bank.length }}/{{ bankCap }}</button>
    </div>
    <div v-if="game.panel === 'bag' && bagTab === 'bank'" class="seg thin">
      <button v-for="row in shelfRows" :key="row.id" :class="{ on: bankShelf === row.id }" @click="bankShelf = row.id; pickedId = null">{{ row.name }} {{ shelfCount(row.id) }}</button>
    </div>
    <div v-if="game.panel === 'bag' && bagTab === 'bag'" class="seg thin">
      <button :class="{ on: bagFilter === 'all' }" @click="bagFilter = 'all'">全部</button>
      <button :class="{ on: bagFilter === 'equipment' }" @click="bagFilter = 'equipment'">装备</button>
      <button :class="{ on: bagFilter === 'consumable' }" @click="bagFilter = 'consumable'">药品</button>
      <button :class="{ on: bagFilter === 'material' }" @click="bagFilter = 'material'">材料</button>
      <button :class="{ on: bagFilter === 'quest' }" @click="bagFilter = 'quest'">任务</button>
    </div>
    <div v-if="game.panel === 'bag' && bagTab === 'bag'" class="seg thin">
      <button :class="{ on: bagSort === 'shelf' }" @click="bagSort = 'shelf'">原序</button>
      <button :class="{ on: bagSort === 'kind' }" @click="bagSort = 'kind'">种类</button>
      <button :class="{ on: bagSort === 'stack' }" @click="bagSort = 'stack'">数量</button>
    </div>
    <div v-else-if="game.panel === 'craft'" class="seg">
      <button :class="{ on: craftTab === 'all' }" @click="craftTab = 'all'">全部</button>
      <button :class="{ on: craftTab === 'craft' }" @click="craftTab = 'craft'">制造</button>
      <button :class="{ on: craftTab === 'cook' }" @click="craftTab = 'cook'">料理</button>
      <button :class="{ on: craftTab === 'blend' }" @click="craftTab = 'blend'">合剂</button>
    </div>
    <div v-else-if="game.panel === 'market'" class="seg">
      <button :class="{ on: marketTab === 'browse' }" @click="marketTab = 'browse'; pickedId = null">在售</button>
      <button :class="{ on: marketTab === 'sell' }" @click="marketTab = 'sell'; pickedId = null">我要卖</button>
    </div>
    <div v-else-if="game.panel === 'social'" class="seg">
      <button :class="{ on: socialTab === 'friend' }" @click="socialTab = 'friend'">好友</button>
      <button :class="{ on: socialTab === 'mail' }" @click="socialTab = 'mail'">驿站</button>
    </div>
    <div v-else-if="game.panel === 'guild'" class="seg">
      <button :class="{ on: guildTab === 'hall' }" @click="guildTab = 'hall'">堂口</button>
      <button :class="{ on: guildTab === 'roster' }" @click="guildTab = 'roster'">名册</button>
      <button :class="{ on: guildTab === 'list' }" @click="guildTab = 'list'">名录</button>
    </div>
    <div v-else-if="game.panel === 'info'" class="seg">
      <button :class="{ on: infoTab === 'sheet' }" @click="infoTab = 'sheet'">资料</button>
      <button :class="{ on: infoTab === 'skill' }" @click="infoTab = 'skill'">心法</button>
      <button :class="{ on: infoTab === 'log' }" @click="infoTab = 'log'">消息</button>
    </div>

    <div class="panel-body">
      <AlbumBoard v-if="game.panel === 'album'" />
      <template v-else-if="game.panel === 'bag'">
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
          <button v-for="item in shownBag" :key="item.id" class="slot" :class="{ on: pickedId === item.id }" @click="onBagClick(item, $event)">
            <IconMedal :src="itemIcon(item.templateId)" />
            <span class="slot-name" :class="qualityClass(item.quality)">{{ item.name }}</span>
            <small v-if="item.temper" class="temper-mark">淬{{ item.temper }}</small>
            <b v-if="item.count > 1" class="qty">{{ item.count }}</b>
          </button>
        </div>

        <div v-else class="bag-grid">
          <p class="empty shelf-note">三间加起来仍是 {{ bankCap }} 格。存进来的进「{{ shelfLabel }}」，不另收费。</p>
          <p v-if="!shelfItems.length" class="empty">这间是空的。在背包里点一件东西，再存进{{ shelfLabel }}。</p>
          <button v-for="item in shelfItems" :key="item.id" class="slot" :class="{ on: pickedId === item.id }" @click="pick(item.id)">
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
        <div v-if="craftTab === 'blend'" class="list blend-bench">
          <p class="lede">两味。材料先扣背包，不够再扣仓库。不涨熟练，原来的配方还在。</p>
          <article v-for="blend in game.config?.blends ?? []" :key="blend.id" class="item">
            <span class="item-main">
              <IconMedal :src="itemIcon(blend.output.itemId)" />
              <span>
                <b>{{ blend.name }}</b>
                <small>{{ blend.gold }} 金币 · {{ blendReady(blend) || '可以合' }}</small>
                <small>{{ blend.blurb }}</small>
                <span class="ings">
                  <span v-for="input in blend.inputs" :key="input.itemId" :class="{ short: heldCount(input.itemId) < input.count }">
                    <IconMedal tiny :src="itemIcon(input.itemId)" />
                    {{ itemName(input.itemId) }} {{ heldCount(input.itemId) }}/{{ input.count }}
                  </span>
                </span>
              </span>
            </span>
            <button class="btn lantern" :disabled="!!blendReady(blend)" @click="game.act('/api/blend', { id: blend.id })">{{ blendReady(blend) || '合' }}</button>
          </article>
        </div>
        <div v-else class="list">
          <button v-for="recipe in recipes" :key="recipe.id" class="item" :class="{ on: pickedId === recipe.id, flash: craftFlash === recipe.id, short: craftReady(recipe) }" @click="pick(recipe.id)">
            <span class="item-main">
              <IconMedal :src="itemIcon(recipe.output.itemId)" />
              <span>
                <b>{{ recipe.name }}</b>
                <small>{{ recipe.life === 'cook' ? '料理' : '制造' }} {{ recipe.skill }} 级 · {{ recipe.gold }} 金币 · {{ craftReady(recipe) || '可以做' }}</small>
                <span class="ings">
                  <span v-for="input in recipe.inputs" :key="input.itemId" :class="{ short: ownedCount(input.itemId) < input.count }">
                    <IconMedal tiny :src="itemIcon(input.itemId)" />
                    {{ itemName(input.itemId) }} {{ ownedCount(input.itemId) }}/{{ input.count }}
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
        <template v-if="guildTab === 'hall'">
          <div v-if="game.profile.guild" class="hall-board">
            <h3>{{ game.profile.guild.name }} · {{ game.profile.guild.level }} 级</h3>
            <p class="notice">{{ game.profile.guild.notice }}</p>
            <div class="hall-meter">
              <span>{{ guildPace.full ? '已经满级' : '距下一级' }}</span>
              <i><b :style="{ width: guildPace.width }" /></i>
              <small>{{ guildPace.full ? '10 / 10' : guildPace.into + ' / 500' }}</small>
            </div>
            <p class="lede">捐献仍是 50 金币起，500 经验一级。点卯一天一次，只加 20 公会经验。</p>
            <div v-if="game.profile.guild.role === 'leader'" class="row">
              <input v-model="noticeDraft" maxlength="40" placeholder="改一句公告" />
              <button class="btn" @click="saveNotice">张贴</button>
            </div>
            <div class="row">
              <input v-model.number="donate" type="number" min="50" style="width:100px" />
              <button class="btn" @click="game.act('/api/guilds/donate', { gold: donate })">捐献</button>
              <button class="btn lantern" :disabled="game.profile.guild.rolled" @click="game.act('/api/guilds/roll')">{{ game.profile.guild.rolled ? '今日已点卯' : '点卯' }}</button>
              <button class="btn light" @click="game.act('/api/guilds/leave')">离开</button>
            </div>
          </div>
          <div v-else class="hall-board">
            <h3>还没有堂口</h3>
            <p class="lede">创建要 200 金币。有成员时会长不能直接走。没有公会岛，也不加战斗。</p>
            <div class="row">
              <input v-model="guildName" maxlength="8" placeholder="公会名" />
              <button class="btn lantern" @click="game.act('/api/guilds', { name: guildName }).then(() => { guildTab = 'hall' })">创建</button>
            </div>
          </div>
        </template>
        <template v-else-if="guildTab === 'roster'">
          <p v-if="!game.profile.guild" class="empty">先创建或加入，名册才会出现。</p>
          <div v-for="member in game.profile.guild?.members ?? []" :key="member.id" class="item">
            <span>{{ member.name }} · {{ member.level }} 级<small> · {{ member.role === 'leader' ? '会长' : '成员' }} · 捐献 {{ member.contrib || 0 }}</small></span>
          </div>
        </template>
        <template v-else>
          <p v-if="game.profile.guild" class="lede">你已经在「{{ game.profile.guild.name }}」。要换，先离开。</p>
          <p v-if="!game.guilds.length" class="empty">还没有别人的公会。</p>
          <div v-for="guild in game.guilds" :key="guild.id" class="item">
            <span>{{ guild.name }} · {{ guild.level }} 级 · {{ guild.members }} 人 · 会长 {{ guild.leaderName }}</span>
            <button v-if="!game.profile.guild" class="btn" @click="game.act('/api/guilds/join', { guildId: guild.id }).then(loadSide)">加入</button>
          </div>
        </template>
      </template>

      <template v-else-if="game.panel === 'dye'">
        <div class="mount-stage">
          <div class="ride stage">
            <ActorFigure :look="previewLook" />
          </div>
          <div v-if="shownDye">
            <h3>{{ shownDye.name }}</h3>
            <p class="lede">{{ shownDye.blurb }}</p>
            <p class="lede">旁边的人也能看见。攻击、防御、生命和步子不读这个。</p>
            <div class="row">
              <button class="btn lantern" :disabled="wornDyeId === shownDye.id" @click="applyDye(shownDye.id)">染上</button>
              <button class="btn light" :disabled="!game.profile.dye" @click="applyDye('tea')">染回暖茶</button>
            </div>
          </div>
        </div>
        <div class="mount-grid">
          <button v-for="row in dyeRows" :key="row.id" type="button" class="mount-card dye-card" :class="{ on: shownDye?.id === row.id, worn: wornDyeId === row.id }" @click="dyePick = row.id">
            <i class="dye-swatch" :style="{ background: row.coat }" />
            <b>{{ row.name }}</b>
            <small>{{ wornDyeId === row.id ? '现在这身' : '可预览' }}</small>
          </button>
        </div>
      </template>

      <template v-else-if="game.panel === 'looks'">
        <div class="mount-stage">
          <div class="ride stage" :class="{ on: !!wardrobeMount }">
            <MountGlyph v-if="wardrobeMount" :id="wardrobeMount" />
            <ActorFigure :look="wardrobeLook" />
          </div>
          <div>
            <h3>{{ shownSaved?.name || '现在这身' }}</h3>
            <p class="lede">{{ shownSaved?.label || '衣色、绦带和坐骑预览可以收成一套。最多三套。' }}</p>
            <p class="lede">换上不加攻击、防御、生命，步子也不变。坐骑等级不够时，只换衣色和绦带。</p>
            <div class="row">
              <input v-model="lookName" maxlength="6" placeholder="给这身起个名" @keydown.enter="game.act('/api/look/save', { name: lookName }).then(() => { lookName = '' })" />
              <button class="btn lantern" @click="game.act('/api/look/save', { name: lookName }).then(() => { lookName = '' })">记下</button>
              <button v-if="shownSaved" class="btn" @click="game.act('/api/look/wear', { id: shownSaved.id })">换上</button>
              <button v-if="shownSaved" class="btn light" @click="game.act('/api/look/drop', { id: shownSaved.id }).then(() => { lookPick = '' })">拿掉</button>
            </div>
          </div>
        </div>
        <h3>绦带</h3>
        <p v-if="shownSash" class="lede">{{ shownSash.blurb }}</p>
        <div class="mount-grid">
          <button v-for="row in sashRows" :key="row.id" type="button" class="mount-card dye-card" :class="{ on: shownSash?.id === row.id, worn: wornSashId === row.id }" @click="sashPick = row.id">
            <i class="dye-swatch" :style="{ background: row.color || '#d08a3c' }" />
            <b>{{ row.name }}</b>
            <small>{{ wornSashId === row.id ? '系着' : '可系' }}</small>
          </button>
        </div>
        <div class="row">
          <button class="btn lantern" :disabled="!shownSash || wornSashId === shownSash.id" @click="game.act('/api/sash', { id: shownSash?.id || 'plain' }).then(() => { sashPick = ''; lookPick = '' })">系上</button>
        </div>
        <h3>已记下</h3>
        <p v-if="!savedLooks.length" class="empty">还没有记下的样子。先染色或系绦，再起个名。</p>
        <div v-for="row in savedLooks" :key="row.id" class="item" :class="{ on: shownSaved?.id === row.id }">
          <span>{{ row.name }}<small>{{ row.label }}</small></span>
          <button class="btn light" @click="lookPick = row.id">看</button>
        </div>
      </template>

      <template v-else-if="game.panel === 'cast'">
        <div class="cast-board">
          <svg class="rod" viewBox="0 0 88 48" aria-hidden="true">
            <path d="M10 36 Q36 10 78 8" fill="none" stroke="#6b3a1e" stroke-width="3" stroke-linecap="round" />
            <path d="M78 8 Q66 22 48 34" fill="none" stroke="#d5eef2" stroke-width="1.4" />
            <circle cx="48" cy="34" r="3.2" fill="#7eb8c9" stroke="#2a1810" stroke-width="1" />
          </svg>
          <div>
            <h3>抛一竿</h3>
            <p class="lede">站在星落镇或月光湖边上。钓上一条银鳞鱼，竿要歇一会儿。</p>
            <p class="lede">不涨采集熟练，也不动地图上的鱼点。战斗里不能抛。</p>
            <p class="lede">{{ castReadyNow ? '竿歇好了。' : '竿还要歇 ' + Math.ceil(castLeft / 1000) + ' 秒。' }}</p>
            <button class="btn lantern" :disabled="!castReadyNow" @click="game.act('/api/cast')">抛竿</button>
          </div>
        </div>
      </template>

      <template v-else-if="game.panel === 'trail'">
        <p class="lede">和一位旅人结路谊。等级高的当引路人，另一位跟着学。一天各传一句，两句都到了路谊才往前，最高 {{ game.profile?.trail?.bondMax || 6 }}。不加金币、经验和属性。</p>
        <p v-if="game.profile?.trail?.peerName" class="lede">
          你是{{ game.profile.trail.roleName }}，对方是 {{ game.profile.trail.peerName }}。
          路谊 {{ game.profile.trail.bond }}/{{ game.profile.trail.bondMax }}。
          {{ game.profile.trail.said ? '今天这句已经传了。' : '今天还可以传一句。' }}
          {{ game.profile.trail.peerLine ? '对方今天说：' + game.profile.trail.peerLine : (game.profile.trail.peerSaid ? '对方也传过了。' : '还在等对方。') }}
        </p>
        <div v-else class="row">
          <input v-model="trailName" placeholder="对方名字" />
          <button class="btn" @click="game.act('/api/trail/ask', { name: trailName, role: 'lead' })">我来引路</button>
          <button class="btn light" @click="game.act('/api/trail/ask', { name: trailName, role: 'walk' })">请你引路</button>
        </div>
        <div v-if="game.profile?.trail?.incoming" class="item">
          <span>{{ game.profile.trail.incoming.name }} 请你当{{ game.profile.trail.incoming.roleName }}</span>
          <span class="row">
            <button class="btn" @click="game.act('/api/trail/accept')">答应</button>
            <button class="btn light" @click="game.act('/api/trail/decline')">回绝</button>
          </span>
        </div>
        <div v-if="game.profile?.trail?.outgoing" class="item">
          <span>已请 {{ game.profile.trail.outgoing.name }}，你想当{{ game.profile.trail.outgoing.roleName }}</span>
          <button class="btn light" @click="game.act('/api/trail/cancel')">收回</button>
        </div>
        <div v-for="line in game.profile?.trail?.lines ?? []" :key="line.id" class="item">
          <span>{{ line.text }}</span>
          <button class="btn" :disabled="game.profile?.trail?.said" @click="game.act('/api/trail/line', { id: line.id })">传这句</button>
        </div>
        <div v-if="game.profile?.trail?.peerName" class="row">
          <button class="btn light" @click="game.act('/api/trail/leave')">散了路谊</button>
        </div>
        <p class="lede">路谊到 {{ game.profile?.trail?.titleAt || 3 }} 可以在名帖挂「认得路的人」。</p>
      </template>

      <template v-else-if="game.panel === 'mount'">
        <div class="mount-stage">
          <div class="ride stage" :class="{ on: shownMount }">
            <MountGlyph v-if="shownMount" :id="shownMount.id" />
            <ActorFigure :look="selfLook" />
          </div>
          <div v-if="shownMount">
            <h3>{{ shownMount.name }}</h3>
            <p class="lede">{{ shownMount.blurb }}</p>
            <p class="lede">{{ game.profile.level >= shownMount.level ? '你的等级够了，可以亮出。' : '要 ' + shownMount.level + ' 级才能亮出。' }}</p>
            <div class="row">
              <button class="btn lantern" :disabled="game.profile.level < shownMount.level" @click="showMount(shownMount.id)">亮出</button>
              <button class="btn light" :disabled="!game.profile.mount" @click="showMount('')">收起</button>
            </div>
          </div>
        </div>
        <div class="mount-grid">
          <button v-for="row in mountRows" :key="row.id" type="button" class="mount-card" :class="{ on: shownMount?.id === row.id, locked: game.profile.level < row.level, worn: game.profile.mount === row.id }" @click="mountPick = row.id">
            <MountGlyph :id="row.id" />
            <b>{{ row.name }}</b>
            <small>{{ game.profile.mount === row.id ? '亮着' : game.profile.level < row.level ? row.level + ' 级' : '可预览' }}</small>
          </button>
        </div>
      </template>

      <template v-else-if="game.panel === 'dungeon'">
        <article class="gate-board">
          <h3>{{ ruins?.name || '古代遗迹' }}</h3>
          <p class="lede">{{ ruins?.desc }}</p>
          <ul class="gate-facts">
            <li>门槛 {{ ruins?.unlockLevel ?? 25 }} 级 · 建议 {{ ruins?.levelMin }}–{{ ruins?.levelMax }}</li>
            <li>{{ game.party ? '跟队长进同一条线路' : '现在一个人一条线路' }}</li>
            <li>里面不能摆摊。离开回到星落镇</li>
          </ul>
          <h3>会遇见</h3>
          <p v-if="!ruinsFoes.length" class="empty">门槛上还没写出会见谁。</p>
          <div v-for="foe in ruinsFoes" :key="foe.id" class="item">
            <span>{{ foe.name }}<small> · {{ foeMark(foe.role) }}<template v-if="foe.level"> · {{ foe.level }} 级</template></small></span>
          </div>
          <p v-if="!ruinsTown && !ruinsHere" class="lede">你在野外。先回星落镇，再用这里或航标进门。</p>
          <p v-else-if="!ruinsReady && !ruinsHere" class="lede">还差 {{ (ruins?.unlockLevel ?? 25) - (game.profile?.level ?? 1) }} 级。门槛不会因为预览坐骑而放开。</p>
          <div class="row">
            <button class="btn lantern" :disabled="ruinsHere ? false : !(ruinsTown && ruinsReady)" @click="enterRuins">{{ ruinsHere ? '离开遗迹' : '进入遗迹' }}</button>
            <button class="btn light" @click="game.panel = 'travel'">返回地图</button>
          </div>
        </article>
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
        <div class="seg thin quest-seg">
          <button :class="{ on: questFilter === 'all' }" @click="questFilter = 'all'">全部</button>
          <button :class="{ on: questFilter === 'main' }" @click="questFilter = 'main'">主线</button>
          <button :class="{ on: questFilter === 'side' }" @click="questFilter = 'side'">支线</button>
          <button :class="{ on: questFilter === 'open' }" @click="questFilter = 'open'">可接</button>
        </div>
        <template v-if="questFilter !== 'open'">
          <h3>进行中</h3>
          <p v-if="!game.profile.quests.length" class="empty">还没有接任务。去场景里点一个人的「对话」。</p>
          <p v-else-if="!shownLanes(game.profile.quests).length" class="empty">这一类没有进行中的任务。</p>
          <section v-for="lane in shownLanes(game.profile.quests)" :key="'on-' + lane.id" class="quest-lane" :class="lane.id">
            <h3><em>{{ lane.label }}</em></h3>
            <article v-for="quest in lane.rows" :key="quest.id" class="item quest-card" :class="[lane.id, { pinned: game.pinnedQuest === quest.id }]">
              <div>
                <b>{{ quest.name }}</b>
                <small>{{ lane.label }} · 交给 {{ quest.turnin || '委托人' }}</small>
                <small v-if="quest.desc" class="qdesc">{{ quest.desc }}</small>
                <small v-if="quest.reward" class="qreward">报酬 {{ quest.reward }}</small>
                <div v-for="(goal, index) in quest.objectives" :key="index" class="qline">
                  <span>{{ goal.text }}</span>
                  <span>{{ goal.current }}/{{ goal.need }}</span>
                  <span class="qbar"><i :style="{ width: Math.min(100, (goal.current / Math.max(1, goal.need)) * 100) + '%' }" /></span>
                </div>
              </div>
              <div class="row">
                <button class="btn light" @click="game.pinQuest(quest.id)">{{ game.pinnedQuest === quest.id ? '已钉' : '钉上' }}</button>
                <button class="btn" @click="game.pursue(quest.id); game.panel = ''">前往</button>
              </div>
            </article>
          </section>
        </template>
        <template v-if="questFilter === 'all' || questFilter === 'open'">
          <h3>可以去接</h3>
          <p v-if="!game.profile.available?.length" class="empty">暂时没有新的委托。先把身上的做完，或者再升一级。</p>
          <section v-for="lane in questLanes(game.profile.available)" :key="'open-' + lane.id" class="quest-lane" :class="lane.id">
            <h3><em>{{ lane.label }}</em></h3>
            <div v-for="quest in lane.rows" :key="quest.id" class="item quest-card" :class="lane.id">
              <div>
                <b>{{ quest.name }}</b>
                <small>{{ lane.label }} · 去找 {{ quest.giver }} 对话</small>
                <small v-if="quest.desc" class="qdesc">{{ quest.desc }}</small>
                <small v-if="quest.reward" class="qreward">报酬 {{ quest.reward }}</small>
              </div>
              <button class="btn light" @click="goNpc(quest.giverId)">找人</button>
            </div>
          </section>
        </template>
      </template>

      <template v-else-if="game.panel === 'shop'">
        <div class="seg thin">
          <button :class="{ on: shopLane === 'buy' }" @click="shopLane = 'buy'">购买</button>
          <button :class="{ on: shopLane === 'sell' }" @click="shopLane = 'sell'">出售</button>
        </div>
        <template v-if="shopLane === 'buy'">
          <p class="lede">杂货由米娜出售。火砂用来淬炼。灵契球打到三成血再契约，捕兽索七成血就能野捕。兽纹帖刻指定的一招。</p>
          <div class="seg thin">
            <button :class="{ on: shopFilter === 'all' }" @click="shopFilter = 'all'">全部</button>
            <button :class="{ on: shopFilter === 'afford' }" @click="shopFilter = 'afford'">买得起</button>
            <button :class="{ on: shopFilter === 'potion' }" @click="shopFilter = 'potion'">药品</button>
            <button :class="{ on: shopFilter === 'material' }" @click="shopFilter = 'material'">材料</button>
          </div>
          <p v-if="!shownGoods.length" class="empty">这一栏没有货。换个筛选，或先去挣点金币。</p>
          <div class="list">
            <div v-for="good in shownGoods" :key="good.itemId" class="item">
              <div class="item-main">
                <IconMedal :src="itemIcon(good.itemId)" />
                <div>
                  <b>{{ good.name }}</b>
                  <small>{{ good.desc }}</small>
                  <small>买 {{ good.price }} · 卖店 {{ good.sell || '不收' }} · 身上 {{ ownedCount(good.itemId) }}</small>
                </div>
              </div>
              <div class="row">
                <button class="btn" :disabled="!canAfford(good.price)" @click="game.act('/api/shop/buy', { npcId: shopNpc.id, itemId: good.itemId, count: 1 })">买 1</button>
                <button class="btn light" :disabled="!canAfford(good.price, 5)" @click="game.act('/api/shop/buy', { npcId: shopNpc.id, itemId: good.itemId, count: 5 })">买 5</button>
              </div>
            </div>
          </div>
        </template>
        <template v-else>
          <p class="lede">卖的是背包里商人肯收的东西。装备一次卖一件。</p>
          <p v-if="!sellables.length" class="empty">背包里没有能卖的。任务物品和绑定的不会出现在这里。</p>
          <div class="list">
            <div v-for="item in sellables" :key="item.id" class="item">
              <div class="item-main">
                <IconMedal :src="itemIcon(item.templateId)" />
                <div>
                  <b>{{ item.name }}<template v-if="item.count > 1"> ×{{ item.count }}</template></b>
                  <small>收购 {{ item.sell }} /个 · 全卖 {{ item.sell * item.count }} 金币</small>
                </div>
              </div>
              <div class="row">
                <button class="btn light" @click="game.act('/api/shop/sell', { itemId: item.id, count: 1 })">卖 1</button>
                <button v-if="item.count > 1" class="btn" @click="game.act('/api/shop/sell', { itemId: item.id, count: item.count })">卖光</button>
              </div>
            </div>
          </div>
        </template>
      </template>

      <template v-else-if="game.panel === 'party'">
        <div class="order-picks">
          <button v-for="order in companionOrders" :key="order.id" type="button" :class="{ on: (game.profile?.companionOrder || 'steady') === order.id }" @click="game.act('/api/party/order', { order: order.id })">
            <b>{{ order.name }}</b>
            <small>{{ order.hint }}</small>
          </button>
        </div>
        <p class="lede">现在是「{{ game.profile?.companionOrderName || '稳打' }}」。没组队时，你自己超时没选也会听这个。</p>
        <div v-if="game.party">
          <p class="lede">队长带着队伍。古代遗迹会跟队长进同一条线路。目标不改步子，也不改号令。</p>
          <p v-if="game.party.follow" class="stall-board">{{ game.party.follow.text }}</p>
          <div class="order-picks">
            <button
              v-for="aim in game.config?.partyAims ?? []"
              :key="aim.id"
              type="button"
              :class="{ on: game.party.aim === aim.id }"
              :disabled="game.party.leaderId !== game.profile.id"
              @click="game.act('/api/party/aim', { aim: aim.id }).then((result) => { game.party = result?.party ?? game.party })"
            >
              <b>{{ aim.name }}</b>
              <small>{{ aim.hint }}</small>
            </button>
          </div>
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
                  <span v-if="who.titleName">{{ who.titleName }}</span>
                  <span>等级 {{ who.level }}</span>
                  <span>{{ mapName || '路上' }}</span>
                  <span>{{ who.gold }} 金币</span>
                  <span v-if="who.guild">{{ who.guild.name }}</span>
                  <span v-for="face in (who.regards ?? []).filter((row) => row.n > 0)" :key="face.id">{{ face.name }} {{ face.tierName }}</span>
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
          <p class="lede">现在是{{ who.weaponName }}。心法修为就是武器熟练，{{ who.tracks?.weapon?.level ?? 1 }} 级。先温习再记到下面的栏位。温习不改威力，换武器仍换成那把武器的默认四招。</p>
          <section v-for="lane in bookLanes" :key="lane.id" class="verse-lane">
            <h3>{{ lane.name }}<small> {{ lane.hint }}</small></h3>
            <div v-for="skill in lane.skills" :key="skill.id" class="item verse" :class="{ studied: studied(skill.id), open: game.profile.manual?.focus === skill.id }">
              <div>
                <b>{{ skill.name }}<template v-if="who.loadout?.includes(skill.id)"> · 栏位 {{ who.loadout.indexOf(skill.id) + 1 }}</template></b>
                <small v-if="studied(skill.id)">{{ skill.desc }}</small>
                <small v-else>还没温习。翻开才看得到口诀，栏上的招仍能用。</small>
                <small>{{ skillNote(skill) }}</small>
              </div>
              <div class="row">
                <button class="btn light" type="button" @click="studySkill(skill.id)">{{ studied(skill.id) ? '再看' : '温习' }}</button>
                <button class="btn" type="button" @click="putSkill(skill.id)">记到 {{ skillSlot + 1 }}</button>
              </div>
            </div>
          </section>
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
          <p class="lede">加上好友就能私聊、寄短信，也可以互相同行。同行只是称谓，不加属性。</p>
          <div class="row">
            <input v-model="friendName" placeholder="对方名字" @keydown.enter="game.act('/api/friends/request', { name: friendName }).then(loadSide)" />
            <button class="btn" @click="game.act('/api/friends/request', { name: friendName }).then(loadSide)">申请</button>
          </div>
          <p v-if="!game.friends.length" class="empty">还没有好友。</p>
          <div v-for="friend in game.friends" :key="friend.id" class="item">
            <span>
              {{ friend.name }}
              · {{ friend.status === 'accepted' ? (friend.online ? '在线' : '离线') : (friend.incoming ? '想加你' : '等答应') }}
              <small v-if="friend.bonded">同行</small>
              <small v-else-if="friend.pledged">已邀同行</small>
            </span>
            <span class="row">
              <button v-if="friend.incoming" class="btn" @click="game.act('/api/friends/accept', { id: friend.id }).then(loadSide)">同意</button>
              <button v-if="friend.incoming" class="btn light" @click="game.act('/api/friends/remove', { id: friend.id }).then(loadSide)">回绝</button>
              <button v-if="friend.status === 'accepted'" class="btn" @click="openWhisper(friend.name)">私聊</button>
              <button v-if="friend.status === 'accepted' && !friend.bonded && !friend.pledged" class="btn light" @click="game.act('/api/friends/pledge', { name: friend.name }).then(loadSide)">邀同行</button>
              <button v-if="friend.pledgeIn && !friend.bonded" class="btn" @click="game.act('/api/friends/pledge', { name: friend.name }).then(loadSide)">答应</button>
              <button v-if="friend.bonded" class="btn light" @click="game.act('/api/friends/break').then(loadSide)">散了</button>
              <button v-if="friend.outgoing" class="btn light" @click="game.act('/api/friends/remove', { id: friend.id }).then(loadSide)">撤回</button>
              <button v-if="friend.status === 'accepted'" class="btn light" @click="game.act('/api/friends/remove', { id: friend.id }).then(loadSide)">删除</button>
            </span>
          </div>
        </template>
        <template v-else>
          <p class="lede">驿站代转短信。玩家的信没有附件。系统寄来的东西，仍然在这里领。</p>
          <div class="row">
            <input v-model="noteName" placeholder="好友名字" />
            <input v-model="noteBody" maxlength="60" placeholder="写一句，不带东西" @keydown.enter="sendNote" />
            <button class="btn" @click="sendNote">寄出</button>
          </div>
          <p v-if="!game.mail.length" class="empty">驿站还没有信。</p>
          <div v-for="letter in game.mail" :key="letter.id" class="mail" :class="{ fresh: !letter.read }">
            <div>
              <b>{{ letter.from || '驿站' }} · {{ letter.title }}</b>
              <div>{{ letter.body }}</div>
            </div>
            <span class="row">
              <button v-if="!letter.claimed" class="btn" @click="game.act('/api/mail/claim', { id: letter.id }).then(loadSide)">领取</button>
              <button v-else-if="!letter.read" class="btn light" @click="game.act('/api/mail/read', { id: letter.id }).then(loadSide)">读过</button>
            </span>
          </div>
        </template>
      </template>

      <template v-else-if="game.panel === 'slate'">
        <h3>今日小事</h3>
        <p class="lede">和公会点卯分开。做完只解锁称谓「今日有闲」，不加金币、经验，也不改战斗。</p>
        <div v-for="goal in game.profile.day?.goals ?? []" :key="goal.id" class="item">
          <span>{{ goal.done ? '已记' : '未记' }} · {{ goal.name }}<small>{{ goal.hint }}</small></span>
        </div>
        <div class="row">
          <button class="btn lantern" :disabled="!game.profile.day?.done || game.profile.day?.kept" @click="game.act('/api/day/claim')">
            {{ game.profile.day?.kept ? '今日已收下' : '收下称谓' }}
          </button>
        </div>
        <h3>称谓</h3>
        <p v-if="game.profile.bondName" class="lede">同行的人：{{ game.profile.bondName }}</p>
        <p class="lede">挂在地图名字上面。旁边的人也能看见。不加攻击、防御、生命或速度。</p>
        <div v-for="row in game.profile.titles ?? []" :key="row.id" class="item" :class="{ on: row.on }">
          <span>{{ row.name }}<small>{{ row.need }}</small></span>
          <button v-if="row.on" class="btn light" @click="game.act('/api/title', { id: '' })">卸下</button>
          <button v-else class="btn" :disabled="!row.open" @click="game.act('/api/title', { id: row.id })">{{ row.open ? '挂上' : '未解锁' }}</button>
        </div>
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
          <small v-if="picked.kind === 'equipment'" class="temper-line">
            淬炼 {{ picked.temper || 0 }}/{{ picked.temperMax || 6 }}
            <template v-if="(picked.temper || 0) < (picked.temperMax || 6)"> · 火砂 ×{{ picked.temperGrit }}（身上 {{ ownedCount('temper_grit') }}）· {{ picked.temperGold }} 金币 · 大约 {{ picked.temperChance }}%</template>
            <template v-else> · 已经淬满</template>
          </small>
          <small v-if="weaponShift" class="shift">{{ weaponShift }}</small>
          <small v-for="row in compareRows" :key="row.stat" :class="row.up ? 'up' : 'down'">对比身上 {{ row.text }}</small>
        </div>
      </div>
      <div class="row">
        <button v-if="picked.kind === 'equipment' && (picked.temper || 0) < (picked.temperMax || 6) && picked.location !== 'bank'" class="btn lantern" @click="game.act('/api/temper', { itemId: picked.id })">淬炼</button>
        <button v-if="picked.kind === 'equipment' && picked.location !== 'equipped'" class="btn" @click="game.act('/api/equip', { itemId: picked.id })">装备</button>
        <button v-if="picked.location === 'equipped'" class="btn light" @click="game.act('/api/unequip', { slot: picked.slot }).then(() => pickedId = null)">卸下</button>
        <button v-if="picked.kind === 'consumable' && picked.location === 'bag'" class="btn" @click="game.act('/api/use', { itemId: picked.id })">使用</button>
        <button v-if="picked.location === 'bag' && !picked.bound" class="btn light" @click="storeItem('deposit')">存进{{ shelfLabel }}</button>
        <button v-if="picked.location === 'bank'" class="btn" @click="storeItem('withdraw')">取出</button>
        <button v-for="row in shelfRows.filter((row) => row.id !== itemShelf(picked))" v-show="picked.location === 'bank'" :key="row.id" class="btn light" @click="storeItem('shift', row.id)">挪到{{ row.name }}</button>
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
          <small>{{ pickedRecipe.life === 'cook' ? '料理' : '制造' }} {{ pickedRecipe.skill }} 级 · 花费 {{ pickedRecipe.gold }} 金币 · {{ craftReady(pickedRecipe) || '材料齐了' }}</small>
        </div>
      </div>
      <div class="row">
        <button class="btn lantern" :disabled="!!craftReady(pickedRecipe)" @click="makeRecipe(pickedRecipe)">{{ craftReady(pickedRecipe) || '制作' }}</button>
      </div>
    </div>

    <div v-else-if="game.panel === 'info' && infoTab === 'skill'" class="detail">
      <div class="skill-bar">
        <button v-for="(skill, index) in barSkills" :key="index" type="button" :class="{ on: skillSlot === index }" @click="skillSlot = index">
          <small>{{ index + 1 }}</small>
          <b>{{ skill?.name ?? '空' }}</b>
          <small v-if="skill && studied(skill.id)">已温习</small>
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
