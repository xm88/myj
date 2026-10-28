/** 声明 Vite 提供的前端类型，供构建环境和资源导入的类型检查使用。 */

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<object, object, unknown>
  export default component
}
