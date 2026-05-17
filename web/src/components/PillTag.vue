<template>
  <span class="pill-tag" :class="['pill-' + variant, { 'pill-custom': customColor }]" :style="tagStyle">
    <slot>{{ label }}</slot>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  variant?: 'success' | 'danger' | 'primary' | 'warning' | 'info' | 'muted' | 'default'
  label?: string
  color?: string   // 自定义背景色
}>(), {
  variant: 'default',
})

const customColor = computed(() => !!props.color)

const tagStyle = computed(() => {
  if (!props.color) return {}
  return {
    background: props.color + '18',
    color: props.color,
  }
})
</script>

<style scoped>
.pill-tag {
  display: inline-block;
  padding: 2px 10px;
  border-radius: 10px;
  font-size: 11.5px;
  font-weight: 600;
  line-height: 1.5;
  white-space: nowrap;
}

/* Variants */
.pill-default { background: #f5f5f5; color: #555; }
.pill-primary { background: #f0f7ff; color: #1677ff; }
.pill-success { background: #f0fdf4; color: #16a34a; }
.pill-danger  { background: #fef2f2; color: #dc2626; }
.pill-warning { background: #fff7e6; color: #d46b08; }
.pill-info    { background: #f0f5ff; color: #2f54eb; }
.pill-muted   { background: #f9fafb; color: #999; }
</style>
