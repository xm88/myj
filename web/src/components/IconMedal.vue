<script setup lang="ts">
/** 徽章图标：使用处理后的透明素材地址展示图标，地址变化时重新加载。 */

import { ref, watch } from 'vue'
import { medalUrl } from '../game/art'

/** 图标组件的图片地址和尺寸选项。 */
const props = defineProps<{ src: string; tiny?: boolean }>()
/** 实际显示的图标地址，可被透明底色处理后的地址替换。 */
const shown = ref(props.src)

watch(
  () => props.src,
  (src) => {
    shown.value = src
    void medalUrl(src).then((url) => {
      if (props.src === src) shown.value = url
    })
  },
  { immediate: true },
)
</script>

<template>
  <!-- 徽章图标：使用处理后的透明素材地址展示图标，地址变化时重新加载。 -->
  <img class="icon" :class="{ tiny }" :src="shown" alt="" />
</template>
