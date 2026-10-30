<script setup lang="ts">
/** 登录与角色创建：游客、注册、登录及种族武器选择共用服务器提供的规则。 */

import { ref } from 'vue'
import { api } from '../api'
import IconMedal from '../components/IconMedal.vue'
import { weaponIcon } from '../game/art'
import { useGame } from '../stores/game'

/** 共享游戏状态仓库，组件通过它读取服务端数据和提交操作。 */
const game = useGame()
/** 登录或注册的邮箱输入。 */
const email = ref('')
/** 登录或注册的密码输入，仅提交给鉴权接口。 */
const password = ref('')
/** 新角色名称输入，合法字符与重名规则由服务器校验。 */
const name = ref('')
/** 创建角色时选择的初始武器类型。 */
const weapon = ref('sword_shield')
/** 创建角色时选择的种族 ID。 */
const race = ref('human')
/** 从服务器读取的可选初始武器列表。 */
const weapons = ref<{ id: string; name: string; blurb: string }[]>([])
/** 从服务器读取的三族简介与成长说明。 */
const races = ref<{ id: string; name: string; blurb: string; growth: string }[]>([])

api<{ weapons: { id: string; name: string; blurb: string }[]; races?: { id: string; name: string; blurb: string; growth: string }[] }>('/api/weapons')
  .then((data) => {
    weapons.value = data.weapons
    races.value = data.races ?? []
  })
  .catch(() => {
    game.toast('武器列表没有加载出来')
  })

/** 提交登录、注册或游客请求，保存令牌后刷新角色并连接世界。 */
async function enter(path: string, body?: object) {
  try {
    const result = await api<{ token: string }>(path, body ?? {})
    game.saveToken(result.token)
    await game.refresh()
    if (!game.needCharacter) game.connect()
  } catch (err) {
    game.toast(err instanceof Error ? err.message : '进不去')
  }
}

/** 提交角色名字、种族与武器选择，创建成功后进入游戏界面。 */
async function create() {
  try {
    await api('/api/characters', { name: name.value, weaponType: weapon.value, race: race.value })
    await game.refresh()
    game.connect()
  } catch (err) {
    game.toast(err instanceof Error ? err.message : '创建失败')
  }
}
</script>

<template>
  <!-- 登录与角色创建：游客、注册、登录及种族武器选择共用服务器提供的规则。 -->
  <main class="gate">
    <section class="card" v-if="game.needCharacter">
      <header class="win-bar">创建角色</header>
      <img class="emblem" src="/art/fi-ui-emblem.jpg?v=q2" alt="" />
      <h1>你是哪一位旅人</h1>
      <p class="lede">先选种族。每级怎么长，从这里定下，以后不能改。武器还能换。</p>
      <label class="field"><span>名字</span><input v-model="name" maxlength="12" placeholder="两个字到十二个字" /></label>
      <div class="weapons">
        <button v-for="item in races" :key="item.id" type="button" class="weapon" :class="{ on: race === item.id }" @click="race = item.id">
          <b>{{ item.name }}</b>
          <small>{{ item.blurb }}</small>
          <small>{{ item.growth }}</small>
        </button>
      </div>
      <div class="weapons">
        <button v-for="item in weapons" :key="item.id" class="weapon" :class="{ on: weapon === item.id }" @click="weapon = item.id">
          <IconMedal :src="weaponIcon[item.id]" />
          <b>{{ item.name }}</b>
          <small>{{ item.blurb }}</small>
        </button>
      </div>
      <div class="row" style="margin-top: 16px">
        <button class="btn lantern" @click="create">进入星落镇</button>
      </div>
    </section>
    <section class="card" v-else>
      <header class="win-bar">浮岛旅团</header>
      <img class="emblem" src="/art/fi-ui-emblem.jpg?v=q2" alt="" />
      <h1>进入游戏</h1>
      <p class="lede">点地名走路，点名字旁边的按钮对话或战斗。同一个角色，也可以用助手收派遣。</p>
      <TransitionGroup name="toast" tag="div" class="toasts" style="position:static;margin-bottom:12px">
        <div v-for="item in game.toasts" :key="item.id" class="toast">{{ item.text }}</div>
      </TransitionGroup>
      <div class="row">
        <label class="field" style="flex:1"><span>邮箱</span><input v-model="email" autocomplete="username" /></label>
        <label class="field" style="flex:1"><span>密码</span><input v-model="password" type="password" autocomplete="current-password" /></label>
      </div>
      <div class="row">
        <button class="btn" @click="enter('/api/auth/login', { email, password })">登录</button>
        <button class="btn light" @click="enter('/api/auth/register', { email, password })">注册</button>
        <button class="btn lantern" @click="enter('/api/auth/guest')">游客进入</button>
      </div>
    </section>
  </main>
</template>
