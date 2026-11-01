/** 心法分栏。和 server/kit.go 的 verseLane 同一套，不读伤害公式。 */

export type VerseLane = 'root' | 'guard' | 'edge'

export function verseLane(skill: {
  healPct?: number
  buff?: unknown
  shieldFlat?: number
  tauntMs?: number
  aoe?: boolean
  forceCrit?: boolean
}): VerseLane {
  if (skill.healPct || skill.buff || skill.shieldFlat || skill.tauntMs) return 'guard'
  if (skill.aoe || skill.forceCrit) return 'edge'
  return 'root'
}

export const verseLanes: { id: VerseLane; name: string; hint: string }[] = [
  { id: 'root', name: '根基', hint: '直接打出去的招' },
  { id: 'guard', name: '守心', hint: '护身、回复和嘲讽' },
  { id: 'edge', name: '手法', hint: '范围，或必定打中要害' },
]
