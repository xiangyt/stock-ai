<template>
  <div class="pick-chart-wrap">
    <div v-show="isEmpty" class="pc-empty">
      <div class="pc-empty-icon">📈</div>
      <p>请在左侧勾选要叠加显示的选股记录</p>
    </div>
    <div v-show="!isEmpty" ref="chartEl" class="pc-chart"></div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import * as echarts from 'echarts'

// ========== Props ==========

/** 一条折线（一只「股票@选股日」的后续走势） */
export interface PickSeries {
  key: string
  label: string
  values: (number | null)[]   // 与 labels 等长，缺失填 null
}

const props = defineProps<{
  series: PickSeries[]
  labels: string[]            // X 轴刻度：T+1 ~ T+N
  dimLabel: string            // 当前维度中文名，用于 tooltip 说明
  benchmark?: (number | null)[] // 同期上证指数收益曲线，与 labels 等长
}>()

// ========== 配色 ==========

const UP = '#ef232a'
const DOWN = '#00943e'
const LINE_COLORS = [
  '#1677ff', '#fa8c16', '#722ed1', '#13c2c2', '#eb2f96',
  '#2f54eb', '#a0d911', '#fa541c', '#36cfc9', '#f5222d',
  '#9254de', '#73d13d',
]

// ========== 状态 ==========

const chartEl = ref<HTMLDivElement | null>(null)
let chart: echarts.ECharts | null = null
let resizeObserver: ResizeObserver | null = null

const isEmpty = computed(() => props.series.length === 0)

// ========== 图表配置 ==========

/** 是否存在基准曲线（上证指数） */
const hasBenchmark = computed(() =>
  !!props.benchmark && props.benchmark.some(v => v !== null && Number.isFinite(v as number)),
)

const chartOption = computed<echarts.EChartsCoreOption>(() => {
  const series: any[] = props.series.map((s, i) => ({
    name: s.label,
    type: 'line',
    data: s.values,
    smooth: false,
    symbol: 'circle',
    symbolSize: 5,
    connectNulls: true,
    lineStyle: { width: 1.6, color: LINE_COLORS[i % LINE_COLORS.length] },
    itemStyle: { color: LINE_COLORS[i % LINE_COLORS.length] },
  }))

  // 基准曲线：同期上证指数收益，置于最上层便于对照
  if (hasBenchmark.value) {
    series.push({
      name: `上证指数·${props.dimLabel || '收盘'}`,
      type: 'line',
      data: props.benchmark,
      smooth: false,
      symbol: 'circle',
      symbolSize: 5,
      connectNulls: true,
      z: 10,
      lineStyle: { width: 2.2, color: '#1a1a2e', type: 'dashed' },
      itemStyle: { color: '#1a1a2e' },
      markLine: {
        silent: true,
        symbol: 'none',
        lineStyle: { color: '#bbb', type: 'dashed', width: 1 },
        data: [{ yAxis: 0 }],
      },
    })
  }

  return {
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'cross' },
      backgroundColor: 'rgba(255,255,255,.96)',
      borderColor: '#e5e5e5',
      textStyle: { color: '#333', fontSize: 12 },
      formatter: (params: any[]) => {
        const rows = params
          .filter(p => p.value !== null && p.value !== undefined && !Number.isNaN(p.value))
          .sort((a, b) => b.value - a.value)
          .map(p => {
            const color = p.value >= 0 ? UP : DOWN
            return `<div style="display:flex;gap:8px;align-items:center">
              <span style="width:8px;height:8px;border-radius:50%;background:${p.color}"></span>
              <span style="flex:1">${p.marker.slice(0, 0)}${p.seriesName}</span>
              <b style="color:${color}">${p.value >= 0 ? '+' : ''}${Number(p.value).toFixed(2)}%</b>
            </div>`
          })
          .join('')
        if (!rows) return ''
        return `<div style="font-size:12px;margin-bottom:4px;color:#888">${params[0].axisValue} · ${props.dimLabel}价收益</div>${rows}`
      },
    },
    legend: {
      type: 'scroll',
      bottom: 0,
      itemWidth: 14,
      itemHeight: 8,
      textStyle: { fontSize: 12, color: '#555' },
    },
    grid: { left: 56, right: 24, top: 24, bottom: 56 },
    xAxis: {
      type: 'category',
      data: props.labels,
      boundaryGap: false,
      axisLine: { lineStyle: { color: '#ddd' } },
      // 横轴为实际交易日，日期较多时自动隐藏重叠标签
      axisLabel: { color: '#888', fontSize: 12, hideOverlap: true },
    },
    yAxis: {
      type: 'value',
      name: '收益率(%)',
      nameTextStyle: { color: '#999', fontSize: 12 },
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { lineStyle: { color: '#f0f0f0' } },
      axisLabel: { formatter: '{value}%', color: '#888', fontSize: 12 },
    },
    series,
  }
})

// ========== 生命周期 ==========

function render() {
  if (isEmpty.value || !chartEl.value) return
  if (!chart) {
    chart = echarts.init(chartEl.value)
    // 容器尺寸自校正：首次 init 时布局可能尚未稳定（测到错误宽度），
    // ResizeObserver 首次回调及后续任何尺寸变化都会触发 resize 修正
    resizeObserver = new ResizeObserver(() => {
      if (!chart || !chartEl.value) return
      const w = chartEl.value.clientWidth
      const h = chartEl.value.clientHeight
      if (w > 0 && h > 0 && (chart.getWidth() !== w || chart.getHeight() !== h)) {
        chart.resize()
      }
    })
    resizeObserver.observe(chartEl.value)
  }
  chart.setOption(chartOption.value, true)
  // 容器可能刚从 v-show 隐藏状态变为可见，需重新计算尺寸
  chart.resize()
}

function onResize() {
  chart?.resize()
}

watch([() => props.series, () => props.labels, () => props.dimLabel, () => props.benchmark], () => {
  render()
}, { deep: true, flush: 'post' })

onMounted(() => {
  window.addEventListener('resize', onResize)
  render()
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  resizeObserver?.disconnect()
  resizeObserver = null
  chart?.dispose()
  chart = null
})
</script>

<style scoped>
.pick-chart-wrap {
  width: 100%;
  min-height: 320px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 1px 4px rgba(0,0,0,.06);
}
.pc-chart {
  flex: 1;
  min-height: 320px;
  width: 100%;
}
.pc-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 420px;
  color: #bbb;
  font-size: 14px;
}
.pc-empty-icon {
  font-size: 32px;
  margin-bottom: 8px;
}
</style>
