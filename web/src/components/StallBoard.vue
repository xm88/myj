<script setup lang="ts">
/** 摆摊：自己开摊上架，或查看附近摊位并购买。价格和归属由服务器校验。 */

import { onMounted, ref, watch } from 'vue'
import { api } from '../api'
import { useGame } from '../stores/game'

const game = useGame()
const stalls = ref<any[]>([])
const detail = ref<any>(null)
const title = ref('路边摊')
const price = ref(12)
const count = ref(1)
const picked = ref('')

const mine = () => game.profile?.stall
/** 和拍卖同一档旅税。买一件时按这一件分到的价钱计算，向下取整。 */
function due(price: number, count = 1) {
  const rate = Number(game.config?.constants?.marketTax ?? 0.05)
  const n = Math.max(1, Number(count) || 1)
  const pay = n <= 1 ? Math.floor(Number(price) || 0) : Math.floor((Number(price) || 0) / n)
  const tax = Math.floor(pay * rate)
  return { pay, tax, net: pay - tax }
}
const bag = () => (game.profile?.bag ?? []).filter((item: { bound?: boolean; kind: string }) => !item.bound && item.kind !== 'quest')

async function loadList() {
  const data = await api<{ stalls: any[] }>('/api/stalls')
  stalls.value = data.stalls ?? []
}

async function openDetail(id: string) {
  game.stallFocus = id
  detail.value = await api('/api/stall?id=' + encodeURIComponent(id))
}

async function refresh() {
  await loadList()
  const id = game.stallFocus || mine()?.id
  if (id) {
    try {
      detail.value = await api('/api/stall?id=' + encodeURIComponent(id))
    } catch {
      detail.value = mine()
    }
  }
}

onMounted(() => {
  void refresh()
})
watch(() => game.stallFocus, (id) => {
  if (id) void openDetail(id)
})

async function openStall() {
  await game.act('/api/stall/open', { title: title.value })
  await refresh()
}

async function listGoods() {
  if (!picked.value) return
  await game.act('/api/stall/list', { itemId: picked.value, count: count.value, price: price.value })
  picked.value = ''
  await refresh()
}

async function unlist(id: string) {
  await game.act('/api/stall/unlist', { id })
  await refresh()
}

async function closeStall() {
  await game.act('/api/stall/close')
  detail.value = null
  game.stallFocus = ''
  await refresh()
}

async function buy(id: string) {
  await game.act('/api/stall/buy', { id, count: 1 })
  await refresh()
}
</script>

<template>
  <div class="street">
    <section v-if="mine()">
      <h3>{{ mine().title }} · 我的摊</h3>
      <p class="hint">人会站在原地。收摊后才能走路。标价是这一摞的总价。</p>
      <article v-for="good in mine().goods" :key="good.id" class="item">
        <div>
          <b>{{ good.name }} <small>×{{ good.count }}</small></b>
          <div class="quote">
            <span>总价 <b>{{ good.price }}</b></span>
            <span>一件 <b>{{ due(good.price, good.count).pay }}</b></span>
            <span>旅税 <b>{{ due(good.price, good.count).tax }}</b></span>
            <span>到手 <b class="net">{{ due(good.price, good.count).net }}</b></span>
          </div>
        </div>
        <button class="btn light" @click="unlist(good.id)">下架</button>
      </article>
      <p v-if="!mine().goods?.length" class="empty">摊上还是空的。</p>
      <div class="row">
        <select v-model="picked">
          <option value="">选背包里的东西</option>
          <option v-for="item in bag()" :key="item.id" :value="item.id">{{ item.name }} ×{{ item.count }}</option>
        </select>
        <input v-model.number="count" type="number" min="1" max="99" />
        <input v-model.number="price" type="number" min="1" />
        <button class="btn" :disabled="!picked" @click="listGoods">上架</button>
        <button class="btn danger" @click="closeStall">收摊</button>
      </div>
      <p v-if="picked" class="quote">
        <span>买家支付这一摞 <b>{{ Math.floor(Number(price) || 0) }}</b></span>
        <span>买一件 <b>{{ due(price, count).pay }}</b></span>
        <span>旅税 <b>{{ due(price, count).tax }}</b></span>
        <span>到手 <b class="net">{{ due(price, count).net }}</b></span>
      </p>
    </section>
    <section v-else>
      <h3>在这里摆摊</h3>
      <p class="hint">站在想摆的地方再开。副本里不能摆。</p>
      <div class="row">
        <input v-model="title" maxlength="8" placeholder="摊名" />
        <button class="btn lantern" @click="openStall">开摊</button>
      </div>
    </section>

    <h3>附近的摊</h3>
    <p v-if="!stalls.length" class="empty">旁边没有摊。走到界边时，对面的摊也会出现在这里。</p>
    <button v-for="stall in stalls" :key="stall.id" class="item as-btn" @click="openDetail(stall.id)">
      <div>
        <b>{{ stall.title }}</b>
        <small>{{ stall.owner }}<template v-if="stall.seam"> · 界外</template><template v-if="stall.mine"> · 我的</template></small>
      </div>
    </button>
    <section v-if="detail && !detail.mine">
      <h3>{{ detail.title }} · {{ detail.owner }}</h3>
      <article v-for="good in detail.goods" :key="good.id" class="item">
        <div>
          <b>{{ good.name }} <small>×{{ good.count }}</small></b>
          <div class="quote">
            <span>你支付 <b>{{ due(good.price, good.count).pay }}</b></span>
            <span>旅税 <b>{{ due(good.price, good.count).tax }}</b></span>
            <span>对方到手 <b class="net">{{ due(good.price, good.count).net }}</b></span>
          </div>
        </div>
        <button class="btn lantern" @click="buy(good.id)">买下</button>
      </article>
      <p v-if="!detail.goods?.length" class="empty">这家暂时没有货。</p>
    </section>
  </div>
</template>
