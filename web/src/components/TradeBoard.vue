<script setup lang="ts">
/** 当面交易：左右两栏分别是自己和对方放上的东西，锁定后再确认。 */

import { ref } from 'vue'
import { useGame } from '../stores/game'

const game = useGame()
const gold = ref(0)
const picked = ref('')
const count = ref(1)

const bag = () => (game.profile?.bag ?? []).filter((item: { bound?: boolean; kind: string }) => !item.bound && item.kind !== 'quest')

function setName(ev: Event) {
  game.callName = (ev.target as HTMLInputElement).value
}

async function invite() {
  const name = game.callName || ''
  if (!name) {
    game.toast('先在场景里点一个旅人')
    return
  }
  await game.act('/api/trade/invite', { name })
}

async function offerGold() {
  await game.act('/api/trade/offer', { gold: gold.value })
}

async function offerItem() {
  if (!picked.value) return
  await game.act('/api/trade/offer', { itemId: picked.value, count: count.value })
  picked.value = ''
}

async function removeItem(itemId: string) {
  await game.act('/api/trade/offer', { remove: itemId })
}
</script>

<template>
  <div class="street" v-if="game.trade">
    <p
      class="trade-state"
      :class="{ ready: game.trade.ready, wait: game.trade.you?.locked && !game.trade.ready }"
    >
      <template v-if="game.trade.ready">双方已锁定。再点确认才会成交。</template>
      <template v-else-if="game.trade.you?.locked">你已锁定，等对方锁上。改栏里的东西会解开锁定。</template>
      <template v-else-if="game.trade.them?.locked">对方已锁定。你锁上之后才能确认。</template>
      <template v-else>放入物品或金币，然后锁定。两边都锁定后才能确认。</template>
    </p>
    <div class="trade-panes">
      <section>
        <h3>我 · {{ game.trade.you?.name }}</h3>
        <p>金币 {{ game.trade.you?.gold || 0 }}</p>
        <article v-for="item in game.trade.you?.items ?? []" :key="item.itemId" class="item">
          <div><b>{{ item.name }}</b><small>×{{ item.count }}</small></div>
          <button class="btn light" :disabled="game.trade.you?.locked" @click="removeItem(item.itemId)">拿回</button>
        </article>
        <p v-if="!(game.trade.you?.items ?? []).length" class="empty">这一栏还是空的。</p>
        <div class="seals">
          <em :class="{ on: game.trade.you?.locked }">{{ game.trade.you?.locked ? '已锁定' : '未锁定' }}</em>
          <em :class="{ on: game.trade.you?.confirmed, wait: game.trade.you?.locked && !game.trade.you?.confirmed }">{{ game.trade.you?.confirmed ? '已确认' : '未确认' }}</em>
        </div>
      </section>
      <section>
        <h3>对方 · {{ game.trade.them?.name }}</h3>
        <p>金币 {{ game.trade.them?.gold || 0 }}</p>
        <article v-for="item in game.trade.them?.items ?? []" :key="item.itemId" class="item">
          <div><b>{{ item.name }}</b><small>×{{ item.count }}</small></div>
        </article>
        <p v-if="!(game.trade.them?.items ?? []).length" class="empty">对方还没有放入东西。</p>
        <div class="seals">
          <em :class="{ on: game.trade.them?.locked }">{{ game.trade.them?.locked ? '已锁定' : '未锁定' }}</em>
          <em :class="{ on: game.trade.them?.confirmed, wait: game.trade.them?.locked && !game.trade.them?.confirmed }">{{ game.trade.them?.confirmed ? '已确认' : '未确认' }}</em>
        </div>
      </section>
    </div>
    <div class="row">
      <select v-model="picked" :disabled="game.trade.you?.locked">
        <option value="">从背包放入</option>
        <option v-for="item in bag()" :key="item.id" :value="item.id">{{ item.name }} ×{{ item.count }}</option>
      </select>
      <input v-model.number="count" type="number" min="1" max="99" :disabled="game.trade.you?.locked" />
      <button class="btn" :disabled="!picked || game.trade.you?.locked" @click="offerItem">放入</button>
    </div>
    <div class="row">
      <input v-model.number="gold" type="number" min="0" :disabled="game.trade.you?.locked" />
      <button class="btn" :disabled="game.trade.you?.locked" @click="offerGold">放入金币</button>
      <button class="btn" :class="{ lantern: game.trade.you?.locked }" @click="game.act('/api/trade/lock')">{{ game.trade.you?.locked ? '已锁定' : '锁定' }}</button>
      <button class="btn lantern" :disabled="!game.trade.ready" @click="game.act('/api/trade/confirm')">确认</button>
      <button class="btn danger" @click="game.act('/api/trade/cancel')">取消</button>
    </div>
  </div>
  <div v-else class="street">
    <p class="hint">走到另一个旅人旁边，点他的名字发起当面交易。也可以先点「交谈」记下一个名字。</p>
    <div class="row">
      <input :value="game.callName" placeholder="对方名字" @input="setName" />
      <button class="btn lantern" @click="invite">发起交易</button>
    </div>
  </div>
</template>
