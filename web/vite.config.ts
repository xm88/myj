/** 前端开发配置：启用 Vue 编译，并将 REST 与 WebSocket 转发到同一 Go 服务。 */

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 5173 在不少开发机上会被别的项目占用，这里固定到 5180，避免打开到错误的网站。
export default defineConfig({
  plugins: [vue()],
  server: {
    host: '0.0.0.0',
    port: 5180,
    strictPort: true,
    proxy: {
      '/api': 'http://127.0.0.1:8787',
      '/ws': { target: 'ws://127.0.0.1:8787', ws: true },
    },
  },
})
