<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="stock-link-tooltip"
      :style="positionStyle"
      @mouseenter="$emit('mouseenter')"
      @mouseleave="$emit('mouseleave')"
    >
      <div class="slt-title">
        <span class="slt-name">{{ stockName }}</span>
        <span class="slt-code">{{ stockCode }}</span>
      </div>

      <a class="slt-item" :href="eastMoneyUrl" target="_blank" rel="noopener" @click="$emit('mouseleave')">
        <span class="slt-badge slt-badge-em">东财</span>
        <span class="slt-label">东方财富</span>
        <span class="slt-arrow">↗</span>
      </a>
      <a class="slt-item" :href="thsUrl" target="_blank" rel="noopener" @click="$emit('mouseleave')">
        <span class="slt-badge slt-badge-ths">同花顺</span>
        <span class="slt-label">同花顺 iwencai</span>
        <span class="slt-arrow">↗</span>
      </a>
      <a class="slt-item" :href="tencentUrl" target="_blank" rel="noopener" @click="$emit('mouseleave')">
        <span class="slt-badge slt-badge-tx">腾讯</span>
        <span class="slt-label">腾讯自选股</span>
        <span class="slt-arrow">↗</span>
      </a>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { getEastMoneyUrl, getTHSUrl, getTencentUrl } from '../utils/stockLinks'

const props = defineProps<{
  visible: boolean
  stockCode: string
  stockName: string
  x?: number
  y?: number
}>()

defineEmits<{
  mouseenter: []
  mouseleave: []
}>()

const eastMoneyUrl = computed(() => getEastMoneyUrl(props.stockCode))
const thsUrl = computed(() => getTHSUrl(props.stockCode))
const tencentUrl = computed(() => getTencentUrl(props.stockCode))

const positionStyle = computed(() => {
  if (props.x === undefined || props.y === undefined) return {}
  const w = 190
  const h = 150
  let x = props.x + 12
  let y = props.y + 12
  if (x + w > window.innerWidth - 10) x = props.x - w - 12
  if (y + h > window.innerHeight - 10) y = window.innerHeight - h - 10
  if (x < 10) x = 10
  if (y < 10) y = 10
  return { left: `${x}px`, top: `${y}px` }
})
</script>

<style scoped>
.stock-link-tooltip {
  position: fixed;
  z-index: 9999;
  width: 190px;
  padding: 8px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 6px 24px rgba(0, 0, 0, 0.16), 0 1px 4px rgba(0, 0, 0, 0.06);
  animation: slt-fade-in 0.14s ease-out;
  user-select: none;
}
@keyframes slt-fade-in {
  from { opacity: 0; transform: translateY(3px); }
  to { opacity: 1; transform: translateY(0); }
}

.slt-title {
  display: flex;
  align-items: baseline;
  gap: 6px;
  padding: 2px 6px 8px;
  border-bottom: 1px solid #f0f0f0;
  margin-bottom: 6px;
}
.slt-name {
  font-size: 13px;
  font-weight: 600;
  color: #1a1a2e;
}
.slt-code {
  font-size: 11.5px;
  color: #999;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}

.slt-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 6px;
  text-decoration: none;
  color: #333;
  transition: background 0.15s;
}
.slt-item:hover {
  background: #f0f5ff;
}
.slt-item + .slt-item {
  margin-top: 2px;
}
.slt-badge {
  flex-shrink: 0;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
  color: #fff;
}
.slt-badge-em { background: #e6432e; }
.slt-badge-ths { background: #f5a623; }
.slt-badge-tx { background: #1677ff; }
.slt-label {
  flex: 1;
  font-size: 12.5px;
  color: #555;
}
.slt-arrow {
  font-size: 11px;
  color: #bbb;
}
</style>
