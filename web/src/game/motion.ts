/** 只跟界面有关的短闪烁和走位朝向。不读战斗公式，也不改任何操作。 */

import { onUnmounted, ref, watch, type WatchSource } from 'vue'
import type { Face } from './look'

/** 别人或怪物这一拍朝哪边、脚还在不在动。 */
export interface Gait {
  face: Face
  step: boolean
}

/** 根据两次快照的位移决定朝向。停下后脚步再留一小会儿。 */
export function useGait(source: WatchSource<Array<{ id: string; x: number; y: number }>>) {
  const gait = ref<Record<string, Gait>>({})
  const last = new Map<string, { x: number; y: number; moved: number; face: Face }>()
  watch(source, (list) => {
    const now = Date.now()
    const next: Record<string, Gait> = {}
    const seen = new Set<string>()
    for (const item of list) {
      seen.add(item.id)
      const prev = last.get(item.id)
      let face: Face = prev?.face ?? 'right'
      let step = false
      if (prev) {
        const dx = item.x - prev.x
        const dy = item.y - prev.y
        if (Math.hypot(dx, dy) > 1.2) {
          face = Math.abs(dx) >= Math.abs(dy) ? (dx > 0 ? 'right' : 'left') : (dy > 0 ? 'down' : 'up')
          last.set(item.id, { x: item.x, y: item.y, moved: now, face })
          step = true
        } else {
          step = now - prev.moved < 320
          last.set(item.id, { ...prev, x: item.x, y: item.y })
        }
      } else {
        last.set(item.id, { x: item.x, y: item.y, moved: 0, face })
      }
      next[item.id] = { face, step }
    }
    for (const id of last.keys()) {
      if (!seen.has(id)) last.delete(id)
    }
    gait.value = next
  }, { immediate: true })
  return gait
}

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
