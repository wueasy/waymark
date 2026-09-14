<template>
  <el-tooltip :content="dark ? '切换到浅色模式' : '切换到暗黑模式'" placement="bottom-end">
    <el-button
      class="theme-toggle"
      text
      circle
      :aria-label="dark ? '切换到浅色模式' : '切换到暗黑模式'"
      @click="toggleDark"
    >
      <svg v-if="dark" class="theme-icon" viewBox="0 0 24 24" aria-hidden="true">
        <circle cx="12" cy="12" r="4" />
        <path
          d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41"
        />
      </svg>
      <svg v-else class="theme-icon" viewBox="0 0 24 24" aria-hidden="true">
        <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79Z" />
      </svg>
    </el-button>
  </el-tooltip>
</template>

<script setup>
import { computed } from 'vue'
import { themeStore, toggleDark } from '../store'

const dark = computed(() => themeStore.dark)
</script>

<style scoped>
.theme-toggle {
  font-size: 18px;
}
.theme-icon {
  width: 18px;
  height: 18px;
  display: block;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}
</style>
