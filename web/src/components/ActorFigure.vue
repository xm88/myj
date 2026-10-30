<script setup lang="ts">
/** 平面角色：同一套参数画在地图、对话和战斗里。 */

import { computed } from 'vue'
import { rankLabel, shadeColor, type Face, type Look } from '../game/look'

const props = withDefaults(defineProps<{
  look: Look
  facing?: Face
  step?: boolean
  pip?: boolean
}>(), {
  facing: 'down',
  step: false,
  pip: false,
})

const shade = computed(() => shadeColor(props.look.coat, -42))
const back = computed(() => props.facing === 'up')
const eyes = computed(() => props.facing !== 'up' && props.look.hat !== 'helm')
const bulk = computed(() => {
  if (props.look.bulk === 'wide') return 'translate(32 0) scale(1.16 1) translate(-32 0)'
  if (props.look.bulk === 'slim') return 'translate(32 0) scale(0.86 1) translate(-32 0)'
  return undefined
})
const pipText = computed(() => (props.pip ? rankLabel(props.look.rank) : ''))
</script>

<template>
  <span
    class="actor"
    :class="[{ step, idle: !step, ['rank-' + look.rank]: look.rank !== 'normal' }]"
    :data-body="look.body"
    :data-hat="look.hat"
    :data-tool="look.tool"
    aria-hidden="true"
  >
    <span class="flip" :class="{ left: facing === 'left' }">
      <svg class="fig" viewBox="0 0 64 80">
        <ellipse cx="32" cy="75" rx="14" ry="3.1" fill="#1c140c" opacity="0.22" />
        <ellipse v-if="look.rank === 'elite'" cx="32" cy="73" rx="18" ry="5.2" fill="rgba(231,180,35,0.28)" stroke="#e7b423" stroke-width="3.4" />
        <ellipse v-if="look.rank === 'boss'" cx="32" cy="73" rx="20" ry="5.6" fill="rgba(141,46,36,0.35)" stroke="#e7c56a" stroke-width="3.6" />
        <ellipse v-if="look.rank === 'world'" cx="32" cy="73" rx="20" ry="5.4" fill="rgba(126,176,212,0.28)" stroke="#7eb0d4" stroke-width="3.4" />
        <polygon v-if="look.rank === 'boss'" points="32,0 40,11 32,8 24,11" fill="#e7c56a" />

        <g v-if="look.body === 'human'">
          <path v-if="look.cape" :d="back ? 'M16 36 Q12 58 18 72 H46 Q52 58 48 36 Z' : 'M20 40 Q14 58 20 70 H44 Q50 58 44 40 Z'" :fill="back ? look.coat : shade" />
          <g :transform="bulk">
            <rect class="leg-a" x="24" y="58" width="6.4" height="13" rx="2" :fill="shade" />
            <rect class="leg-b" x="34" y="58" width="6.4" height="13" rx="2" :fill="shade" />
            <path d="M22 38 H42 L47 62 H17 Z" :fill="look.coat" />
            <rect x="20" y="50" width="24" height="3.2" rx="1" :fill="look.trim" />
            <rect x="15" y="40" width="8" height="12" rx="3" :fill="look.coat" />
            <rect x="41" y="40" width="8" height="12" rx="3" :fill="look.coat" />
            <circle cx="18" cy="52" r="2.5" :fill="look.skin" />
            <circle cx="46" cy="52" r="2.5" :fill="look.skin" />
          </g>
          <circle cx="32" cy="28" r="10" :fill="back ? look.hair : look.skin" />
          <path v-if="!back && look.hat !== 'hood'" d="M22 27 Q24 16 32 16 Q40 16 42 27 Q36 22 32 23 Q28 22 22 27" :fill="look.hair" />
          <polygon v-if="look.ears === 'fin'" points="18,26 12,16 24,30" :fill="look.coat" />
          <polygon v-if="look.ears === 'fin'" points="46,26 52,16 40,30" :fill="look.coat" />
          <g v-if="look.hat === 'cap'">
            <ellipse cx="32" cy="21" rx="11" ry="3" :fill="shade" />
            <path d="M24 21 Q32 11 40 21 Z" :fill="look.coat" />
          </g>
          <g v-else-if="look.hat === 'wide'">
            <ellipse cx="32" cy="20" rx="16" ry="3.2" :fill="look.coat" />
            <path d="M26 20 Q32 8 38 20 Z" :fill="shade" />
          </g>
          <g v-else-if="look.hat === 'hood'">
            <path d="M18 32 Q16 12 32 9 Q48 12 46 32 Q32 24 18 32" :fill="look.coat" />
            <ellipse v-if="!back" cx="32" cy="29" rx="6.4" ry="6" :fill="look.skin" />
          </g>
          <g v-else-if="look.hat === 'helm'">
            <path d="M22 30 Q22 14 32 13 Q42 14 42 30 Z" :fill="shade" />
            <rect x="24" y="26" width="16" height="3.2" rx="1" :fill="look.trim" />
          </g>
          <g v-else-if="look.hat === 'straw'">
            <ellipse cx="32" cy="22" rx="16" ry="3" fill="#f0d48a" />
            <path d="M27 22 Q32 12 37 22 Z" fill="#e6c56a" />
            <rect x="29" y="17" width="6" height="2.4" :fill="look.trim" />
          </g>
          <g v-else-if="look.hat === 'band'">
            <rect x="23" y="24" width="18" height="3.4" rx="1" :fill="look.trim" />
          </g>
          <g v-else-if="look.hat === 'officer'">
            <rect x="24" y="12" width="16" height="8" rx="1" :fill="look.coat" />
            <ellipse cx="32" cy="20" rx="12" ry="3" :fill="shade" />
            <rect x="24" y="17" width="16" height="2.2" :fill="look.trim" />
          </g>
          <g v-else-if="look.hat === 'tall'">
            <rect x="26" y="6" width="12" height="14" rx="1" :fill="look.coat" />
            <ellipse cx="32" cy="20" rx="9" ry="2.6" :fill="shade" />
            <rect x="26" y="16" width="12" height="2.2" :fill="look.trim" />
          </g>
          <g v-if="eyes">
            <circle cx="28.4" cy="29" r="1.15" fill="#24180f" />
            <circle cx="35.6" cy="29" r="1.15" fill="#24180f" />
          </g>
          <g v-if="look.tool === 'sword'">
            <path d="M8 36 L16 34 L18 50 L12 56 L6 48 Z" :fill="look.trim" :stroke="shade" stroke-width="1" />
            <rect x="50" y="30" width="3" height="20" rx="0.6" fill="#d5dbe3" />
            <rect x="47" y="48" width="9" height="2.2" :fill="look.trim" />
            <rect x="50.6" y="50" width="1.8" height="6" :fill="look.hair" />
          </g>
          <g v-else-if="look.tool === 'great'">
            <rect x="49" y="16" width="4.4" height="40" rx="0.8" fill="#d5dbe3" />
            <rect x="46" y="50" width="10" height="2.6" :fill="look.trim" />
            <rect x="50" y="52" width="2.4" height="7" :fill="look.hair" />
          </g>
          <g v-else-if="look.tool === 'hammer'">
            <rect x="51" y="30" width="2.4" height="28" :fill="look.hair" />
            <rect x="45" y="24" width="14" height="8" rx="1" fill="#8d949c" />
          </g>
          <g v-else-if="look.tool === 'staff'">
            <rect x="50" y="10" width="2.4" height="56" :fill="look.hair" />
            <circle cx="51.2" cy="10" r="4" :fill="look.trim" />
          </g>
          <g v-else-if="look.tool === 'bow'">
            <path d="M48 24 Q62 44 48 64" fill="none" :stroke="look.coat" stroke-width="2.4" />
            <path d="M48 24 L48 64" stroke="#f3e2b8" stroke-width="1" />
          </g>
          <g v-else-if="look.tool === 'book'">
            <rect x="46" y="40" width="12" height="14" rx="1" :fill="look.trim" />
            <path d="M52 40 V54" :stroke="shade" stroke-width="1" />
          </g>
          <g v-else-if="look.tool === 'pack'">
            <rect x="46" y="40" width="12" height="14" rx="2" :fill="shade" />
            <rect x="46" y="40" width="12" height="4" :fill="look.trim" />
          </g>
          <g v-else-if="look.tool === 'rod'">
            <path d="M46 46 L60 18" :stroke="look.hair" stroke-width="2" fill="none" stroke-linecap="round" />
            <path d="M60 18 Q64 18 62 24" fill="none" stroke="#d5dbe3" stroke-width="1.4" />
          </g>
          <g v-else-if="look.tool === 'pick'">
            <path d="M48 54 L58 30" :stroke="look.hair" stroke-width="2.2" fill="none" />
            <path d="M52 34 L62 26 L58 38 Z" fill="#8d949c" />
          </g>
          <g v-else-if="look.tool === 'lantern'">
            <rect x="50" y="28" width="2.2" height="36" :fill="look.hair" />
            <rect x="46" y="14" width="10" height="12" rx="1" fill="#f6e2a8" />
            <path d="M48 14 H54 L51 8 Z" :fill="shade" />
          </g>
        </g>

        <g v-else-if="look.body === 'blob'" class="blob">
          <ellipse cx="32" cy="56" rx="18" ry="14" :fill="look.coat" />
          <ellipse cx="32" cy="60" rx="10" ry="6" :fill="look.trim" opacity="0.72" />
          <ellipse cx="24" cy="50" rx="4" ry="2.6" fill="#fff8ea" opacity="0.55" />
          <g v-if="eyes">
            <circle cx="26" cy="56" r="1.6" fill="#24180f" />
            <circle cx="37" cy="56" r="1.6" fill="#24180f" />
            <path d="M28 61 Q32 64 36 61" fill="none" stroke="#24180f" stroke-width="1.1" />
          </g>
        </g>

        <g v-else-if="look.body === 'quad'" :transform="bulk">
          <ellipse v-if="look.bulk === 'wide' && look.ears === 'point'" cx="40" cy="46" rx="9" ry="7" :fill="shade" />
          <path v-if="look.tail" d="M16 52 Q4 46 10 36" :stroke="look.coat" :stroke-width="look.fluff ? 4 : 3" fill="none" stroke-linecap="round" />
          <ellipse v-if="look.fluff" cx="8" cy="34" rx="5" ry="4" :fill="look.trim" />
          <ellipse cx="30" cy="52" rx="16" ry="11" :fill="look.coat" />
          <ellipse cx="32" cy="56" rx="9" ry="5.5" :fill="look.trim" opacity="0.85" />
          <rect class="leg-a" x="18" y="58" width="4.2" height="12" rx="1.4" :fill="shade" />
          <rect class="leg-b" x="26" y="58" width="4.2" height="12" rx="1.4" :fill="shade" />
          <rect class="leg-b" x="36" y="58" width="4.2" height="12" rx="1.4" :fill="shade" />
          <rect class="leg-a" x="44" y="58" width="4.2" height="12" rx="1.4" :fill="shade" />
          <ellipse v-if="look.ears === 'long'" cx="44" cy="30" rx="2.3" ry="9" :fill="look.coat" />
          <ellipse v-if="look.ears === 'long'" cx="51" cy="30" rx="2.3" ry="9" :fill="look.coat" />
          <circle cx="48" cy="44" r="9" :fill="look.coat" />
          <polygon v-if="look.ears === 'point'" points="42,38 47,24 52,38" :fill="look.coat" />
          <circle v-if="look.ears === 'round'" cx="44" cy="36" r="3.5" :fill="look.coat" />
          <circle v-if="look.ears === 'round'" cx="44" cy="36" r="1.6" :fill="look.skin" />
          <ellipse :cx="look.snout === 'long' ? 57 : 55" cy="48" :rx="look.snout === 'long' ? 7 : 4.4" ry="3.3" :fill="look.trim" />
          <polygon v-if="look.ears === 'horn'" points="52,50 55,60 58,50" fill="#f4ead2" />
          <polygon v-if="look.ears === 'horn'" points="58,49 62,58 64,48" fill="#f4ead2" />
          <circle v-if="eyes" cx="50" cy="42" r="1.35" fill="#24180f" />
          <path v-if="look.hat === 'helm'" d="M40 44 Q40 32 48 31 Q56 32 56 44 Z" :fill="shade" />
        </g>

        <g v-else-if="look.body === 'shroom'" :transform="bulk">
          <rect x="27" y="50" width="10" height="20" rx="4" :fill="look.trim" />
          <ellipse cx="32" cy="44" rx="20" ry="13" :fill="look.coat" />
          <ellipse cx="22" cy="42" rx="3.2" ry="2.2" fill="#fff8ea" opacity="0.8" />
          <ellipse cx="36" cy="36" rx="2.8" ry="2" fill="#fff8ea" opacity="0.75" />
          <ellipse cx="42" cy="46" rx="2.2" ry="1.6" fill="#fff8ea" opacity="0.6" />
          <g v-if="eyes">
            <circle cx="28" cy="50" r="1.35" fill="#24180f" />
            <circle cx="36" cy="50" r="1.35" fill="#24180f" />
          </g>
        </g>

        <g v-else-if="look.body === 'bloom'">
          <rect x="30" y="52" width="4" height="18" rx="2" :fill="look.trim" />
          <ellipse cx="16" cy="48" rx="10" ry="5.5" :fill="look.coat" />
          <ellipse cx="48" cy="48" rx="10" ry="5.5" :fill="look.coat" />
          <ellipse cx="32" cy="36" rx="7" ry="10" :fill="look.coat" />
          <circle cx="32" cy="46" r="4" :fill="look.trim" />
        </g>

        <g v-else-if="look.body === 'bird'" :transform="bulk">
          <polygon points="14,46 4,40 6,52" :fill="look.coat" />
          <ellipse cx="32" cy="50" rx="14" ry="9" :fill="look.coat" />
          <ellipse class="wing" cx="30" cy="50" rx="11" ry="5" :fill="shade" />
          <circle cx="44" cy="44" r="7" :fill="look.coat" />
          <polygon v-if="look.ears === 'point'" points="40,40 44,30 48,40" :fill="look.coat" />
          <polygon :points="look.snout === 'long' ? '48,44 62,47 48,51' : '48,45 58,47 48,50'" fill="#e7b15a" />
          <circle v-if="eyes" cx="46" cy="43" r="1.2" fill="#24180f" />
          <rect class="leg-a" x="28" y="58" width="2" height="12" :fill="shade" />
          <rect class="leg-b" x="36" y="58" width="2" height="12" :fill="shade" />
        </g>

        <g v-else-if="look.body === 'crab'" :transform="bulk">
          <ellipse cx="32" cy="54" rx="14" ry="9" :fill="look.coat" />
          <circle cx="12" cy="48" r="6.2" :fill="shade" />
          <circle cx="52" cy="48" r="6.2" :fill="shade" />
          <circle cx="8" cy="44" r="3" :fill="look.coat" />
          <circle cx="56" cy="44" r="3" :fill="look.coat" />
          <rect class="leg-a" x="20" y="60" width="3" height="9" :fill="shade" />
          <rect class="leg-b" x="28" y="62" width="3" height="8" :fill="shade" />
          <rect class="leg-a" x="34" y="62" width="3" height="8" :fill="shade" />
          <rect class="leg-b" x="41" y="60" width="3" height="9" :fill="shade" />
          <circle cx="26" cy="40" r="1.5" fill="#24180f" />
          <circle cx="38" cy="40" r="1.5" fill="#24180f" />
        </g>

        <g v-else-if="look.body === 'coil'">
          <path d="M14 64 C30 64 28 50 18 46 C8 40 22 30 36 34 C52 38 46 54 36 56 C30 57 50 66 58 52" :stroke="look.coat" stroke-width="7" fill="none" stroke-linecap="round" />
          <circle cx="56" cy="46" r="6" :fill="look.coat" />
          <circle v-if="eyes" cx="58" cy="45" r="1.3" fill="#24180f" />
          <polygon points="62,47 68,45 62,52" fill="#e07a6a" />
        </g>

        <g v-else-if="look.body === 'block'" :transform="bulk">
          <rect x="16" y="56" width="32" height="14" rx="2" :fill="look.coat" />
          <rect x="20" y="42" width="24" height="16" rx="2" :fill="shade" />
          <rect x="24" y="26" width="16" height="18" rx="2" :fill="look.coat" />
          <rect x="27" y="34" width="10" height="3.2" rx="1" :fill="look.trim" />
          <rect class="leg-b" x="8" y="46" width="12" height="8" rx="1" :fill="shade" />
          <rect class="leg-a" x="44" y="46" width="12" height="8" rx="1" :fill="shade" />
        </g>

        <g v-else-if="look.body === 'fish'">
          <polygon points="14,50 2,40 2,60" :fill="look.coat" />
          <ellipse cx="34" cy="50" rx="16" ry="9" :fill="look.coat" />
          <ellipse cx="36" cy="54" rx="8" ry="4" :fill="look.trim" opacity="0.75" />
          <polygon points="28,42 20,34 34,44" :fill="shade" />
          <circle v-if="eyes" cx="44" cy="48" r="1.45" fill="#24180f" />
        </g>

        <g v-else-if="look.body === 'wisp'">
          <circle class="wisp" cx="32" cy="42" r="16" :fill="look.coat" opacity="0.32" />
          <circle cx="32" cy="46" r="8" :fill="look.coat" />
          <circle cx="32" cy="46" r="3.2" :fill="look.trim" />
          <circle class="wisp" cx="16" cy="36" r="2.1" :fill="look.trim" />
          <circle class="wisp" cx="48" cy="32" r="1.6" :fill="look.trim" />
        </g>

        <g v-else-if="look.body === 'whale'">
          <polygon points="10,48 0,38 2,54" :fill="look.coat" />
          <ellipse cx="34" cy="50" rx="22" ry="12" :fill="look.coat" />
          <ellipse cx="36" cy="55" rx="12" ry="5" :fill="look.trim" opacity="0.8" />
          <polygon points="24,42 16,30 30,40" :fill="shade" />
          <circle v-if="eyes" cx="48" cy="46" r="1.5" fill="#24180f" />
        </g>
      </svg>
    </span>
    <em v-if="pipText" class="rank-pip" :class="look.rank">{{ pipText }}</em>
  </span>
</template>
