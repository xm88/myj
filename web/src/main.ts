/** 网页入口：注册 Pinia 状态仓库、引入全局样式并将根组件挂载到 app 容器。 */

import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import './styles.css'

createApp(App).use(createPinia()).mount('#app')
