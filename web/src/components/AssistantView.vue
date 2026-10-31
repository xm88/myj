<script setup lang="ts">
/** 助手界面：集中展示离线派遣和轻量业务操作，服务器开启安全模式避免角色挨打。 */

import { onBeforeUnmount, onMounted, ref } from 'vue'
import { api } from '../api'
import { useGame } from '../stores/game'

/** 共享游戏状态仓库，组件通过它读取服务端数据和提交操作。 */
const game = useGame()
/** 界面倒计时的当前时间，仅用于显示，派遣完成由服务器判断。 */
const clock = ref(Date.now())
/** 循环或倒计时的定时器句柄，组件卸载或进程关闭时需要清理。 */
let timer = 0

onMounted(async () => {
  game.news = (await api<{ news: { t: number; text: string }[] }>('/api/news')).news
  timer = window.setInterval(() => {
    clock.value = Date.now()
  }, 1000)
})
onBeforeUnmount(() => window.clearInterval(timer))

/** 将派遣结束时间转换为剩余分钟，已结束时显示可领取状态。 */
function remain(endsAt: number) {
  const left = Math.max(0, endsAt - clock.value)
  const min = Math.floor(left / 60000)
  const sec = Math.floor((left % 60000) / 1000)
  return `${min}:${String(sec).padStart(2, '0')}`
}
</script>

<template>
  <!-- 助手界面：集中展示离线派遣和轻量业务操作，服务器开启安全模式避免角色挨打。 -->
  <main class="assistant">
    <div class="sheet">
      <header class="row" style="justify-content:space-between;margin-bottom:12px">
        <div class="row" style="align-items:center">
          <img class="mark" src="/art/fi-ui-emblem.jpg?v=q2" alt="" />
          <h2>旅团助手</h2>
        </div>
        <div class="row">
          <button class="btn lantern" @click="game.setMode('play')">回到冒险</button>
          <button class="btn light" @click="game.logout()">离开</button>
        </div>
      </header>
      <section class="hero" v-if="game.profile">
        <div class="muted">Lv.{{ game.profile.level }}</div>
        <h2>{{ game.profile.name }}</h2>
        <div>金币 {{ game.profile.gold }} · {{ game.profile.weather?.name }} · {{ game.profile.periodName }}</div>
        <p class="lede">{{ game.profile.weather?.desc }}</p>
      </section>
      <div class="phone-cards">
        <section class="phone-card">
          <div class="card-top">
            <b>派遣</b>
            <small>灵兽 {{ game.profile?.pets?.length ?? 0 }} 只</small>
          </div>
          <article v-for="job in game.profile?.dispatches ?? []" :key="job.id" class="phone-job">
            <b>{{ job.petName }} · {{ job.nodeName }}</b>
            <div class="progress"><i :style="{ width: job.ready ? '100%' : '55%' }" /></div>
            <div>{{ job.ready ? '已经回来了' : '剩余 ' + remain(job.endsAt) }}</div>
            <div class="row">
              <button v-if="game.profile.devTools && !job.ready" class="btn light" @click="game.act('/api/dispatches/finish', { id: job.id })">测试：立刻完成</button>
              <button class="btn lantern" :disabled="!job.ready" @click="game.act('/api/dispatches/claim', { id: job.id })">领取</button>
            </div>
          </article>
          <p v-if="!game.profile?.dispatches?.length" class="muted">还没有派遣。回冒险找莎雅，或在派遣面板把灵兽派出去。</p>
          <button class="btn" @click="game.setMode('play'); game.panel = 'dispatch'">打开派遣</button>
        </section>
        <button class="phone-card" type="button" @click="game.setMode('play'); game.panel = 'market'">
          <div class="card-top"><b>拍卖</b><small>跳市</small></div>
          <p>看在售、上架材料。成交仍按原来的旅税结算。</p>
        </button>
        <button class="phone-card" type="button" @click="game.setMode('play'); game.panel = 'guild'">
          <div class="card-top"><b>公会</b><small>{{ game.profile?.guild?.name ?? '未加入' }}</small></div>
          <p>{{ game.profile?.guild ? '捐献、公告和成员都在这里。' : '还没有公会。点这里去创建或加入。' }}</p>
        </button>
      </div>
      <div class="phone-row">
        <button class="tile" @click="game.setMode('play'); game.panel = 'bag'"><b>背包</b><div class="muted">{{ game.profile?.bag?.length ?? 0 }} 格</div></button>
        <button class="tile" @click="game.setMode('play'); game.panel = 'info'"><b>角色</b><div class="muted">{{ game.profile?.weaponName ?? '技能' }}</div></button>
      </div>
      <section class="hero" style="margin-top:14px">
        <h3>今日传闻</h3>
        <div v-if="!game.news.length" class="muted">世界还很安静。</div>
        <div v-for="item in game.news" :key="item.t">{{ item.text }}</div>
      </section>
    </div>
  </main>
</template>
