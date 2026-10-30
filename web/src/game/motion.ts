/** 只跟界面有关的短闪烁。不读战斗公式，也不改任何操作。 */

import { onUnmounted, ref, watch, type WatchSource } from 'vue'

/** 数值往下掉时亮一下，给血条和受击用。第一次出现不算。 */
export function useDropFlash(source: WatchSource<number>, ms = 280) {
  const hot = ref(false)
  let timer = 0
  let last = Number.NaN
  watch(source, (next) => {
    const value = Number(next)
    if (!Number.isFinite(value)) return
    if (Number.isFinite(last) && value < last) {
      hot.value = true
      window.clearTimeout(timer)
      timer = window.setTimeout(() => {
        hot.value = false
      }, ms)
    }
    last = value
  }, { immediate: true })
  onUnmounted(() => window.clearTimeout(timer))
  return hot
}
