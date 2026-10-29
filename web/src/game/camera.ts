/** 自由镜头：把相邻地块按世界原点铺开，镜头跟着人走，而不是一张图撑满屏幕。 */

export const defaultSpan = 1100

export interface Chunk {
  id: string
  name?: string
  kind?: string
  width: number
  height: number
  originX: number
  originY: number
}

export interface Frame {
  camX: number
  camY: number
  scale: number
  viewW: number
  viewH: number
  span: number
}

/** 镜头中心是人的世界坐标，横向看见 span 这么宽。 */
export function frameOf(camX: number, camY: number, viewW: number, viewH: number, span = defaultSpan): Frame {
  const width = viewW > 0 ? viewW : 960
  const height = viewH > 0 ? viewH : 540
  return { camX, camY, scale: width / span, viewW: width, viewH: height, span }
}

/** 世界坐标换成镜头里的像素。 */
export function worldToScreen(frame: Frame, x: number, y: number) {
  return {
    x: (x - frame.camX) * frame.scale + frame.viewW / 2,
    y: (y - frame.camY) * frame.scale + frame.viewH / 2,
  }
}

/** 镜头里的像素换回世界坐标。 */
export function screenToWorld(frame: Frame, x: number, y: number) {
  return {
    x: frame.camX + (x - frame.viewW / 2) / frame.scale,
    y: frame.camY + (y - frame.viewH / 2) / frame.scale,
  }
}

/** 找出包含这个世界点的地块。当前所在的地块优先，避免缝上点到重叠的邻块。 */
export function chunkAt(chunks: Chunk[], x: number, y: number, prefer?: string) {
  const hit = (chunk: Chunk) => x >= chunk.originX && y >= chunk.originY && x <= chunk.originX + chunk.width && y <= chunk.originY + chunk.height
  if (prefer) {
    const current = chunks.find((chunk) => chunk.id === prefer)
    if (current && hit(current)) return current
  }
  return chunks.find(hit) ?? null
}

/** 门靠哪一条边。和服务器的判定用同一组阈值。 */
export function gateEdge(x: number, y: number, width: number, height: number) {
  if (x <= 80) return 'west'
  if (x >= width - 80) return 'east'
  if (y <= 80) return 'north'
  if (y >= height - 80) return 'south'
  return ''
}

/** 往门外多走一步，让服务器判定过界。 */
export function pastGate(edge: string, x: number, y: number) {
  if (edge === 'west') return { x: x - 20, y }
  if (edge === 'east') return { x: x + 20, y }
  if (edge === 'north') return { x, y: y - 20 }
  if (edge === 'south') return { x, y: y + 20 }
  return { x, y }
}
