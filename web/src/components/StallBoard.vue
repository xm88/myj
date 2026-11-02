<script setup lang="ts">
/** 摆摊：自己开摊上架，或查看附近摊位并购买。价格和归属由服务器校验。 */

import { onMounted, ref, watch } from 'vue'
import { api } from '../api'
import { useGame } from '../stores/game'

const game = useGame()
const stalls = ref<any[]>([])
const detail = ref<any>(null)
const title = ref('路边摊')
const sign = ref('')
const price = ref(12)
const count = ref(1)
const picked = ref('')
const listedPop = ref(false)
let listTimer = 0

const mine = () => game.profile?.stall
/** 按兵器、丹食、材料、杂货把一摊货分成架子。空架不画。 */
function shelfGroups(goods: { shelf?: string }[] | undefined) {
  const rows = (game.config?.stallShelves ?? []) as { id: string; name: string }[]
  const list = goods ?? []
  const grouped = rows
    .map((shelf) => ({ ...shelf, goods: list.filter((good) => good.shelf === shelf.id) }))
    .filter((shelf) => shelf.goods.length)
  const known = new Set(rows.map((shelf) => shelf.id))
  const rest = list.filter((good) => !known.has(good.shelf))
  if (rest.length) grouped.push({ id: 'rest', name: '杂货架', goods: rest })
  return grouped
}
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
  sign.value = String(mine()?.sign || '')
  void refresh()
})
watch(() => game.stallFocus, (id) => {
  if (id) void openDetail(id)
})

async function openStall() {
  await game.act('/api/stall/open', { title: title.value, sign: sign.value })
  await refresh()
}

async function saveSign() {
  await game.act('/api/stall/sign', { sign: sign.value })
  await refresh()
}

async function listGoods() {
  if (!picked.value) return
  listedPop.value = true
  window.clearTimeout(listTimer)
  listTimer = window.setTimeout(() => {
    listedPop.value = false
  }, 380)
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
      <p v-if="mine().sign" class="stall-board">{{ mine().sign }}</p>
      <p class="hint">人会站在原地。收摊后才能走路。标价是这一摞的总价。招牌和架子不改旅税。</p>
      <div class="row">
        <input v-model="sign" maxlength="16" placeholder="招牌，最多 16 个字" />
        <button class="btn light" @click="saveSign">改招牌</button>
      </div>
      <section v-for="shelf in shelfGroups(mine().goods)" :key="shelf.id" class="shelf">
        <h3 class="shelf-name">{{ shelf.name }}</h3>
        <article v-for="good in shelf.goods" :key="good.id" class="item">
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
      </section>
      <p v-if="!mine().goods?.length" class="empty">摊上还是空的。</p>
      <div class="row">
        <select v-model="picked">
          <option value="">选背包里的东西</option>
          <option v-for="item in bag()" :key="item.id" :value="item.id">{{ item.name }} ×{{ item.count }}</option>
        </select>
        <input v-model.number="count" type="number" min="1" max="99" />
        <input v-model.number="price" type="number" min="1" />
        <button class="btn" :class="{ pop: listedPop }" :disabled="!picked" @click="listGoods">上架</button>
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
        <input v-model="sign" maxlength="16" placeholder="招牌，可以空着" />
        <button class="btn lantern" @click="openStall">开摊</button>
      </div>
    </section>

    <h3>附近的摊</h3>
    <p v-if="!stalls.length" class="empty">旁边没有摊。走到界边时，对面的摊也会出现在这里。</p>
    <button v-for="stall in stalls" :key="stall.id" class="item as-btn" @click="openDetail(stall.id)">
      <div>
        <b>{{ stall.title }}</b>
        <small v-if="stall.sign" class="stall-board">{{ stall.sign }}</small>
        <small>{{ stall.owner }}<template v-if="stall.seam"> · 界外</template><template v-if="stall.mine"> · 我的</template> · {{ stall.listed ? stall.listed + ' 件' : '空摊' }}<template v-if="stall.shelves"> · {{ stall.shelves }}</template><template v-else-if="stall.sample"> · {{ stall.sample }}</template></small>
      </div>
    </button>
    <section v-if="detail && !detail.mine">
      <h3>{{ detail.title }} · {{ detail.owner }}</h3>
      <p v-if="detail.sign" class="stall-board">{{ detail.sign }}</p>
      <section v-for="shelf in shelfGroups(detail.goods)" :key="shelf.id" class="shelf">
        <h3 class="shelf-name">{{ shelf.name }}</h3>
        <article v-for="good in shelf.goods" :key="good.id" class="item">
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
      </section>
      <p v-if="!detail.goods?.length" class="empty">这家暂时没有货。</p>
    </section>
  </div>
</template>
