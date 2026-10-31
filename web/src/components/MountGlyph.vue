<script setup lang="ts">
/** 坐骑预览：旅团自己的矢量侧影。不代表骑上之后会变快。 */

defineProps<{ id: string }>()
</script>

<template>
  <svg class="mount-glyph" viewBox="0 0 120 78" aria-hidden="true">
    <g v-if="id === 'red_pony'" stroke="#2a1810" stroke-width="2" stroke-linejoin="round">
      <path d="M22 46 Q10 40 12 52" fill="none" stroke="#8a3020" stroke-width="3" />
      <ellipse cx="58" cy="42" rx="28" ry="16" fill="#c45a3a" />
      <path d="M78 36 Q98 18 108 28 Q96 34 86 36" fill="#e7c56a" />
      <ellipse cx="96" cy="30" rx="12" ry="10" fill="#d46542" />
      <ellipse cx="106" cy="32" rx="6" ry="4" fill="#e7c56a" />
      <path d="M90 22 Q94 8 102 18" fill="#8a3020" />
      <rect x="40" y="52" width="6" height="16" rx="2" fill="#8a3020" />
      <rect x="54" y="52" width="6" height="16" rx="2" fill="#8a3020" />
      <rect x="74" y="52" width="6" height="16" rx="2" fill="#8a3020" />
      <rect x="86" y="52" width="6" height="16" rx="2" fill="#8a3020" />
      <path d="M46 36 H74 L72 46 H48 Z" fill="#7a3018" stroke="#e7c56a" />
    </g>
    <g v-else-if="id === 'wind_yak'" stroke="#2a1810" stroke-width="2" stroke-linejoin="round">
      <path d="M24 48 Q8 42 10 56" fill="none" stroke="#5c3a22" stroke-width="4" />
      <ellipse cx="62" cy="44" rx="32" ry="18" fill="#8a6238" />
      <ellipse cx="92" cy="34" rx="16" ry="13" fill="#a07040" />
      <path d="M84 24 L88 8 L94 22" fill="#e7c56a" />
      <path d="M98 24 L106 8 L104 24" fill="#e7c56a" />
      <circle cx="112" cy="46" r="5" fill="#e7c56a" />
      <rect x="38" y="56" width="7" height="14" rx="2" fill="#5c3a22" />
      <rect x="54" y="56" width="7" height="14" rx="2" fill="#5c3a22" />
      <rect x="74" y="56" width="7" height="14" rx="2" fill="#5c3a22" />
      <rect x="88" y="56" width="7" height="14" rx="2" fill="#5c3a22" />
      <path d="M48 38 H78 L76 50 H50 Z" fill="#6b3014" stroke="#e7c56a" />
    </g>
    <g v-else-if="id === 'mist_deer'" stroke="#2a1810" stroke-width="2" stroke-linejoin="round">
      <path d="M30 40 Q14 36 16 48" fill="none" stroke="#8a7050" stroke-width="2.4" />
      <ellipse cx="62" cy="42" rx="24" ry="12" fill="#e6d3b0" />
      <ellipse cx="90" cy="32" rx="11" ry="9" fill="#f3e2c4" />
      <path d="M84 24 L80 6 L88 18" fill="none" />
      <path d="M96 24 L104 4 L98 18" fill="none" />
      <circle cx="104" cy="5" r="2.4" fill="#e7c56a" stroke="#e7c56a" />
      <rect x="46" y="50" width="4" height="20" rx="1.5" fill="#cbb892" />
      <rect x="58" y="50" width="4" height="20" rx="1.5" fill="#cbb892" />
      <rect x="74" y="50" width="4" height="20" rx="1.5" fill="#cbb892" />
      <rect x="86" y="50" width="4" height="20" rx="1.5" fill="#cbb892" />
      <circle cx="52" cy="40" r="2" fill="#fff8ea" stroke="none" />
      <circle cx="66" cy="44" r="2" fill="#fff8ea" stroke="none" />
      <path d="M50 38 H74" fill="none" stroke="#7a3018" />
    </g>
    <g v-else stroke="#2a1810" stroke-width="2" stroke-linejoin="round">
      <path d="M24 44 Q8 38 12 52" fill="none" stroke="#5c5848" stroke-width="3" />
      <ellipse cx="58" cy="44" rx="26" ry="15" fill="#8d8a7a" />
      <ellipse cx="90" cy="34" rx="13" ry="11" fill="#a19e8c" />
      <ellipse cx="104" cy="36" rx="7" ry="4.5" fill="#6e6848" />
      <path d="M82 26 L76 6 L88 22 Z" fill="#6e6848" />
      <path d="M96 24 L102 4 L106 22 Z" fill="#6e6848" />
      <rect x="40" y="54" width="6" height="16" rx="2" fill="#5c5848" />
      <rect x="54" y="54" width="6" height="16" rx="2" fill="#5c5848" />
      <rect x="72" y="54" width="6" height="16" rx="2" fill="#5c5848" />
      <rect x="86" y="54" width="6" height="16" rx="2" fill="#5c5848" />
      <path d="M46 38 H72 L70 48 H48 Z" fill="#7a3018" stroke="#e7c56a" />
      <ellipse cx="58" cy="34" rx="6" ry="4" fill="#3f8a45" stroke="#1e4a24" />
    </g>
  </svg>
</template>
