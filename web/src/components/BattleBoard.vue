<script setup lang="ts">
/** 战斗界面：没开战时仍显示出手拍；开战后面板改成回合、出手顺序和动作。 */

import { computed, ref, watch } from 'vue'
import IconMedal from './IconMedal.vue'
import { mapThumb, portrait } from '../game/art'
import { useGame } from '../stores/game'

/** 共享游戏状态仓库，组件通过它读取服务端数据和提交操作。 */
const game = useGame()
/** 动作菜单当前展开的是技能还是药品。 */
const menu = ref<'skill' | 'item' | ''>('')

/** 当前选中或正在追击的怪物快照。 */
const foe = computed(() => {
  const intent = game.intent
  const id = intent && intent.kind !== 'move' ? intent.id : game.snap?.self?.targetId
  const fromOrder = (game.snap?.bout?.order ?? []).find((actor: { kind: string; alive: boolean }) => actor.kind === 'monster' && actor.alive)
  return (game.snap?.entities ?? []).find((entity: { id: string }) => entity.id === (id || fromOrder?.id)) ?? fromOrder ?? null
})
/** 角色当前设为主战的灵伴，用于战斗搭档展示。 */
const pet = computed(() => game.profile?.pets?.find((item: { active: boolean }) => item.active) ?? null)
/** 服务器推送的自身战斗快照，缺失时部分视图使用档案坐标。 */
const self = computed(() => game.snap?.self)
/** 本场回合。没有时界面退回原来的出手拍。 */
const bout = computed(() => game.snap?.bout ?? null)
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
const won = computed(() => bout.value?.phase === 'victory' || (!chasing.value && !bout.value && !foe.value?.alive))
/** 出手条：间隔走完就满，表示下一拍普通攻击准备好了。 */
const readyPct = computed(() => {
  const interval = Number(self.value?.interval) || 0
  const swing = Number(self.value?.swing) || 0
  if (!interval) return self.value?.targetId ? 100 : 0
  return Math.min(100, Math.max(0, (1 - swing / interval) * 100))
})
/** 守势剩余时，出手条改成守势提示。 */
const guarding = computed(() => Number(self.value?.guard) > 0 || Boolean(bout.value?.order?.find((actor: { id: string; effects?: { id: string }[] }) => actor.id === game.profile?.id && actor.effects?.some((ef) => ef.id === 'guard'))))
/** 这一场已经打出的普通攻击拍数。 */
const beats = computed(() => Number(self.value?.beats) || 0)
/** 轮到自己时才能点动作。 */
const yours = computed(() => Boolean(bout.value?.yours))
/** 出手顺序，用来标出当前是谁。 */
const order = computed(() => bout.value?.order ?? [])
/** 被拉进这场的队友由服务器选动作，本人的回合仍要点。 */
const companions = computed(() => order.value.some((actor: { companion?: boolean; id: string }) => actor.companion && actor.id !== game.profile?.id))
/** 自己身上还在倒数的回合状态。 */
const selfEffects = computed(() => order.value.find((actor: { id: string }) => actor.id === game.profile?.id)?.effects ?? [])
/** 目标身上的回合状态。 */
const foeEffects = computed(() => order.value.find((actor: { id: string }) => actor.id === foe.value?.id)?.effects ?? [])
/** 战斗画面上跟着自己或目标的飘字。 */
function hits(id: string | undefined) {
  if (!id) return []
  return game.floats.filter((item) => item.entityId === id)
}
/** 优先使用实时快照中的地图，再回退到持久化角色档案。 */
const mapId = computed(() => game.snap?.mapId || game.profile?.mapId || 'town_starfall')
/** 判断当前意图是否为追击攻击或契约目标。 */
const chasing = computed(() => game.intent?.kind === 'fight' || game.intent?.kind === 'capture')
/** 药品只列出能直接喝的消耗品。 */
const drinks = computed(() => (game.profile?.bag ?? []).filter((item: { kind: string; templateId: string }) => item.kind === 'consumable' && item.templateId !== 'capture_orb'))
/** 技能栏和模板合在一起，给回合里的技能菜单用。 */
const skills = computed(() => {
  const loadout: string[] = game.profile?.loadout ?? []
  const all = game.config?.skills ?? []
  return loadout.map((id: string, index: number) => {
    const skill = all.find((item: { id: string }) => item.id === id)
    return { slot: index + 1, name: skill?.name ?? '技能', mp: skill?.mp ?? 0 }
  })
})
/** 缓存最近目标名称，目标消失后仍能显示战斗结束提示。 */
const lastName = ref('')
watch(foe, (next) => {
  if (next?.name) lastName.value = next.name
})
watch(bout, (next) => {
  if (!next?.yours) menu.value = ''
})

/** 回合提示：轮到谁，或这一场已经怎么结束。 */
const turnText = computed(() => {
  if (!bout.value) return ''
  if (bout.value.phase === 'victory') return `第 ${bout.value.round} 回合 · 胜利`
  if (bout.value.phase === 'defeat') return `第 ${bout.value.round} 回合 · 战败`
  if (bout.value.phase === 'fled') return `第 ${bout.value.round} 回合 · 已脱身`
  if (bout.value.yours) return `第 ${bout.value.round} 回合 · 轮到你`
  return `第 ${bout.value.round} 回合 · ${bout.value.awaitName || '对方'} 行动`
})

function act(action: string, extra: Record<string, unknown> = {}) {
  if (!yours.value) return
  menu.value = ''
  game.bout(action, extra)
}
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
      <div v-if="selfEffects.length" class="effects">
        <em v-for="ef in selfEffects" :key="ef.id">{{ ef.name }} {{ ef.rounds }}</em>
      </div>
      <div v-if="pet" class="pet-row">
        <IconMedal :src="portrait(pet.templateId)" />
        <span>{{ pet.name }}<small>主战</small></span>
      </div>
      <p v-else class="hint">还没有主战灵伴</p>
    </div>
    <div class="battle-mid">
      <div v-if="bout" class="cadence">
        <div class="order">
          <span v-for="actor in order" :key="actor.id" :class="{ now: actor.id === bout.await, down: actor.alive === false }">{{ actor.name }}</span>
        </div>
        <small>{{ turnText }}{{ game.autoFight ? ' · 连招' : '' }}{{ companions ? ' · 队友由同伴决定' : '' }}</small>
      </div>
      <div v-else class="cadence">
        <div class="swing" :title="guarding ? '守势中，这一拍少挨打' : '普通攻击的出手节奏'">
          <i :class="{ guard: guarding }" :style="{ width: readyPct + '%' }" />
        </div>
        <small>{{ guarding ? '守势' : (beats ? '第 ' + beats + ' 拍' : '等待出手') }}{{ game.autoFight ? ' · 连招' : '' }}</small>
      </div>
      <div class="battle-log">
        <p v-if="bout">{{ yours ? '轮到你了。选一个动作。' : (bout.phase === 'choose' ? '等待这一手结束。' : turnText) }}</p>
        <p v-else-if="chasing">正在靠近{{ foe?.name ? ' ' + foe.name : '' }}。走进距离后会排成回合。</p>
        <p v-else-if="won">{{ lastName ? lastName + ' 已被击败。' : '战斗结束。' }}</p>
        <p v-for="(line, index) in (bout?.log ?? []).slice().reverse()" :key="'b' + index">{{ line }}</p>
        <p v-for="(line, index) in game.battleLog" :key="index" :class="{ crit: line.includes('暴击'), guarded: line.includes('守势') }">{{ line }}</p>
      </div>
      <div v-if="bout && yours" class="row bout-actions">
        <button class="btn lantern" @click="act('attack')">攻击</button>
        <button class="btn" @click="menu = menu === 'skill' ? '' : 'skill'">技能</button>
        <button class="btn" @click="menu = menu === 'item' ? '' : 'item'">物品</button>
        <button class="btn" :class="{ lantern: guarding }" @click="act('defend')">防御</button>
        <button class="btn danger" @click="act('flee')">逃跑</button>
      </div>
      <div v-if="menu === 'skill'" class="row">
        <button v-for="skill in skills" :key="skill.slot" class="btn" @click="act('skill', { slot: skill.slot })">{{ skill.name }}<small v-if="skill.mp"> {{ skill.mp }}</small></button>
      </div>
      <div v-if="menu === 'item'" class="row">
        <button v-for="item in drinks" :key="item.id" class="btn" @click="act('item', { itemId: item.id })">{{ item.name }} ×{{ item.count }}</button>
        <p v-if="!drinks.length" class="hint">背包里没有能在战斗中用的药。</p>
      </div>
      <div class="row">
        <button v-if="!bout" class="btn" :class="{ lantern: guarding }" @click="game.guard()">守势</button>
        <button class="btn" :class="{ lantern: game.autoFight }" @click="game.setAuto(!game.autoFight)">{{ game.autoFight ? '连招中' : '连招' }}</button>
        <button v-if="!bout" class="btn danger" @click="game.flee()">逃跑</button>
        <button class="btn" :class="{ lantern: won }" @click="game.view = 'scene'">{{ won ? '胜利，返回地图' : '返回地图' }}</button>
        <button class="btn lantern" :disabled="!canCapture || !foe" @click="foe && game.capture(foe.id)">契约</button>
      </div>
    </div>
    <div class="fighter">
      <template v-if="foe">
        <IconMedal class="portrait" :src="portrait(foe.templateId || 'slime')" />
        <b>{{ foe.name }}</b>
        <small>{{ foeLevel ? foeLevel + '级 · ' : '' }}{{ foe.alive === false ? '已被击败' : (foe.capturable ? '可以契约' : '战斗中') }}</small>
        <div class="bar"><i class="hp foe" :style="{ width: foePct + '%' }" /></div>
        <small>{{ foe.hp }} / {{ foe.maxHp }}</small>
        <div v-if="foeEffects.length" class="effects">
          <em v-for="ef in foeEffects" :key="ef.id">{{ ef.name }} {{ ef.rounds }}</em>
        </div>
        <span v-for="hit in hits(foe.id)" :key="hit.id" class="float foe" :class="{ crit: hit.crit, heal: hit.kind === 'heal' }">{{ hit.text }}</span>
      </template>
      <p v-else class="hint">{{ lastName ? lastName + ' 已被击败' : '这场已经结束' }}</p>
    </div>
  </div>
</template>
