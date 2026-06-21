<template>
  <Teleport to="body">
    <div
      v-if="visible"
      ref="tooltipRef"
      class="kline-tooltip"
      :style="positionStyle"
      @mouseenter="$emit('mouseenter')"
      @mouseleave="$emit('mouseleave')"
    >
      <!-- 标题栏 -->
      <div class="kt-header">
        <div class="kt-title-left">
          <span class="kt-name">{{ stockName }}</span>
          <span class="kt-code">{{ stockCode }}</span>
        </div>
        <div class="kt-period-tabs">
          <button
            v-for="p in periods"
            :key="p.value"
            :class="['kt-tab', { active: currentPeriod === p.value }]"
            @click="switchPeriod(p.value)"
          >{{ p.label }}</button>
        </div>
      </div>

      <!-- 最新价信息 -->
      <div class="kt-price-bar" v-if="latestInfo">
        <div class="kt-price-left">
          <span class="kt-date">{{ latestInfo.date }}</span>
          <span class="kt-price" :style="{ color: latestInfo.change >= 0 ? '#ef232a' : '#00943e' }">
            {{ latestInfo.close.toFixed(2) }}
          </span>
          <span class="kt-change" :style="{ color: latestInfo.change >= 0 ? '#ef232a' : '#00943e' }">
            {{ latestInfo.change >= 0 ? '+' : '' }}{{ latestInfo.changePct.toFixed(2) }}%
          </span>
        </div>
        <div class="kt-ohlc-row">
          <span class="kt-ohlc-item" :style="{ color: ohlcColor(latestInfo.open) }">开 {{ latestInfo.open.toFixed(2) }}</span>
          <span class="kt-ohlc-item" :style="{ color: ohlcColor(latestInfo.high) }">高 {{ latestInfo.high.toFixed(2) }}</span>
          <span class="kt-ohlc-item" :style="{ color: ohlcColor(latestInfo.close) }">收 {{ latestInfo.close.toFixed(2) }}</span>
          <span class="kt-ohlc-item" :style="{ color: ohlcColor(latestInfo.low) }">低 {{ latestInfo.low.toFixed(2) }}</span>
        </div>
        <div class="kt-ma-info">
          <span style="color:#1a1a1a">MA5:{{ latestInfo.ma.ma5?.toFixed(2) ?? '-' }}</span>
          <span style="color:#FF9033; margin-left:6px">MA10:{{ latestInfo.ma.ma10?.toFixed(2) ?? '-' }}</span>
          <span style="color:#C71585; margin-left:6px">MA20:{{ latestInfo.ma.ma20?.toFixed(2) ?? '-' }}</span>
          <span style="color:#0066FF; margin-left:6px">MA60:{{ latestInfo.ma.ma60?.toFixed(2) ?? '-' }}</span>
        </div>
      </div>

      <!-- 加载状态 -->
      <div v-if="loading" class="kt-loading">
        <span class="kt-spinner"></span> 加载中...
      </div>

      <!-- 图表区域 -->
      <div v-else-if="chartData && chartData.items.length > 0" class="kt-chart-wrap">
        <!-- 主图：K 线 + 均线 -->
        <div ref="mainChartRef" class="kt-main-chart"></div>
        <!-- 副图：成交量（顶部信息栏 + 图表） -->
        <div class="kt-sub-chart">
          <div v-if="subVolumeVal" class="kt-sub-label-bar">{{ subVolumeVal }}</div>
          <div ref="volumeChartRef" class="kt-sub-canvas"></div>
        </div>
        <!-- 副图：MACD -->
        <div class="kt-sub-chart">
          <div v-if="subMacdVal" class="kt-sub-label-bar">
            <span style="color:#1a1a1a">DIF:{{ subMacdVal.dif?.toFixed(2) ?? '-' }}</span>&nbsp;
            <span style="color:#FFD700">DEA:{{ subMacdVal.dea?.toFixed(2) ?? '-' }}</span>&nbsp;
            <span style="color:#C71585">MACD:{{ subMacdVal.macd?.toFixed(2) ?? '-' }}</span>
          </div>
          <div ref="macdChartRef" class="kt-sub-canvas"></div>
        </div>
        <!-- 副图：KDJ -->
        <div class="kt-sub-chart">
          <div v-if="subKdjVal" class="kt-sub-label-bar">
            <span style="color:#1a1a1a">K:{{ subKdjVal.k?.toFixed(2) ?? '-' }}</span>&nbsp;
            <span style="color:#FFD700">D:{{ subKdjVal.d?.toFixed(2) ?? '-' }}</span>&nbsp;
            <span style="color:#C71585">J:{{ subKdjVal.j?.toFixed(2) ?? '-' }}</span>
          </div>
          <div ref="kdjChartRef" class="kt-sub-canvas"></div>
        </div>
      </div>

      <!-- 无数据 -->
      <div v-else class="kt-empty">暂无 K 线数据</div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import * as echarts from 'echarts'
import type { ECharts } from 'echarts'
import { fetchKLine, type KLineResponse, type KLineItem, type KLinePeriod } from '../api/kline'

// ========== Props / Emits ==========

const props = defineProps<{
  visible: boolean
  stockCode: string
  stockName: string
  x?: number
  y?: number
}>()

const emit = defineEmits<{
  mouseenter: []
  mouseleave: []
}>()

// ========== 周期配置 ==========

const periods = [
  { label: '日K', value: 'daily' as KLinePeriod },
  { label: '周K', value: 'weekly' as KLinePeriod },
  { label: '月K', value: 'monthly' as KLinePeriod },
]

// ========== 状态 ==========

const currentPeriod = ref<KLinePeriod>('daily')
const loading = ref(false)
const chartData = ref<KLineResponse | null>(null)

/** 主图鼠标悬停的数据索引，默认显示最后一天 */
const hoverIndex = ref(-1)

const tooltipRef = ref<HTMLDivElement>()
const mainChartRef = ref<HTMLDivElement>()
const volumeChartRef = ref<HTMLDivElement>()
const macdChartRef = ref<HTMLDivElement>()
const kdjChartRef = ref<HTMLDivElement>()

let mainChart: ECharts | null = null
let volumeChart: ECharts | null = null
let macdChart: ECharts | null = null
let kdjChart: ECharts | null = null

// ========== 定位样式 ==========

const positionStyle = computed(() => {
  if (props.x !== undefined && props.y !== undefined) {
    const viewportW = window.innerWidth
    const viewportH = window.innerHeight
    const w = 860
    const h = 620
    // 始终在鼠标右侧展开，往上偏移让整体居中显示
    let x = props.x + 12
    let y = props.y - h / 2 - 50
    // 超出右边界时向左收缩，但不翻转到左侧
    if (x + w > viewportW - 20) x = viewportW - w - 20
    // 超出下边界时向上对齐
    if (y + h > viewportH - 20) y = viewportH - h - 20
    // 确保不超出左/上边界
    if (x < 10) x = 10
    if (y < 10) y = 10
    return {
      left: `${x}px`,
      top: `${y}px`,
    }
  }
  return {}
})

// ========== 最新价信息 ==========

interface LatestInfo {
  date: string
  open: number
  close: number
  high: number
  low: number
  change: number
  changePct: number
  ma: { ma5?: number; ma10?: number; ma20?: number; ma60?: number }
}

const latestInfo = computed<LatestInfo | null>(() => {
  const data = chartData.value
  const items = data?.items
  if (!items || items.length === 0) return null
  // hoverIndex < 0 时显示最后一天，否则跟随鼠标悬停
  const idx = (hoverIndex.value >= 0 && hoverIndex.value < items.length) ? hoverIndex.value : items.length - 1
  const cur = items[idx]
  const prevIdx = idx > 0 ? idx - 1 : 0
  const prevClose = items[prevIdx]?.close ?? cur.close
  const change = cur.close - prevClose
  const changePct = prevClose !== 0 ? (change / prevClose) * 100 : 0
  // 从 indicators 数组取 MA 值
  const maData = data?.indicators?.ma
  return {
    date: cur.date,
    open: cur.open,
    close: cur.close,
    high: cur.high,
    low: cur.low,
    change,
    changePct,
    ma: {
      ma5: maData?.ma5?.[idx],
      ma10: maData?.ma10?.[idx],
      ma20: maData?.ma20?.[idx],
      ma60: maData?.ma60?.[idx],
    },
  }
})

// ========== 副图左上角数值（跟随 hoverIndex） ==========
const idxVal = computed(() => {
  const items = chartData.value?.items
  if (!items || items.length === 0) return -1
  return (hoverIndex.value >= 0 && hoverIndex.value < items.length) ? hoverIndex.value : items.length - 1
})

const subVolumeVal = computed(() => {
  const data = chartData.value
  const i = idxVal.value
  if (!data?.items[i]) return null
  const v = Math.round(data.items[i].volume / 100) // 股 → 手（1手=100股）
  return formatVol(v) + '手'
})

const subMacdVal = computed(() => {
  const data = chartData.value?.indicators?.macd
  const i = idxVal.value
  if (!data) return null
  return { dif: data.dif?.[i], dea: data.dea?.[i], macd: data.macd?.[i] }
})

const subKdjVal = computed(() => {
  const data = chartData.value?.indicators?.kdj
  const i = idxVal.value
  if (!data) return null
  return { k: data.k?.[i], d: data.d?.[i], j: data.j?.[i] }
})

/** OHLC 着色：价比昨收则红，低于则绿 */
function ohlcColor(val: number): string {
  const data = chartData.value
  const items = data?.items
  if (!items || items.length === 0) return '#333'
  const idx = (hoverIndex.value >= 0 && hoverIndex.value < items.length) ? hoverIndex.value : items.length - 1
  const prevClose = idx > 0 ? items[idx - 1].close : items[0].open
  if (val > prevClose) return '#ef232a'
  if (val < prevClose) return '#00943e'
  return '#333'
}

// ========== 数据加载 ==========

async function loadData() {
  if (!props.stockCode) return
  loading.value = true
  hoverIndex.value = -1 // 重置悬停索引，默认显示最后一天
  try {
    chartData.value = await fetchKLine(props.stockCode, currentPeriod.value, 250)
    // Teleport 弹窗需等浏览器完成布局后再 init echarts，否则容器尺寸为 0
    await nextTick()
    requestAnimationFrame(() => {
      renderCharts()
    })
  } catch (e) {
    console.error('[KLineTooltip] 加载失败:', e)
    chartData.value = null
  } finally {
    loading.value = false
  }
}

function switchPeriod(period: KLinePeriod) {
  if (currentPeriod.value === period) return
  currentPeriod.value = period
  loadData()
}

// ========== 图表渲染 ==========

/** A 股配色 */
const COLORS = {
  up: '#ef232a',       // 涨红
  down: '#00943e',     // 跌绿
  unch: '#888888',     // 平盘
  ma5: '#1a1a1a',       // MA5 黑色
  ma10: '#FF9033',      // MA10 橙色
  ma20: '#C71585',      // MA20 紫红色
  ma60: '#0066FF',      // MA60 蓝色
  dif: '#1a1a1a',       // DIF 黑色
  dea: '#FFD700',      // DEA 黄色
  macdUp: '#ef232a',   // MACD柱上涨 红色
  macdDown: '#00943e', // MACD柱下跌
  kLine: '#1a1a1a',    // K线 黑色
  dLine: '#FFD700',     // D线 黄色
  jLine: '#C71585',     // J线 紫红色
}

function getCandleColor(item: KLineItem): string {
  if (item.open === item.close) return COLORS.unch
  return item.close >= item.open ? COLORS.up : COLORS.down
}

function getBarColor(item: KLineItem): string {
  if (!chartData.value || chartData.value.items.length < 2) return COLORS.up
  const idx = chartData.value.items.indexOf(item)
  if (idx <= 0) return COLORS.up
  const prevClose = chartData.value.items[idx - 1].close
  return item.close >= prevClose ? COLORS.up : COLORS.down
}

function renderCharts(retryCount = 0) {
  disposeCharts()
  const data = chartData.value
  if (!data || data.items.length === 0) return

  // 检查 DOM 容器是否已就位（Teleport 到 body 后需要等布局完成）
  if (!mainChartRef.value || mainChartRef.value.clientWidth === 0) {
    if (retryCount < 10) {
      requestAnimationFrame(() => renderCharts(retryCount + 1))
    }
    return
  }

  const dates = data.items.map(i => i.date)
  const klineData = data.items.map(i => [i.open, i.close, i.low, i.high])
  // 提取指标数据
  const ind = data.indicators

  // 主图：K线 + MA均线（MA线叠加在蜡烛图上）
  renderMainChart(dates, klineData, data.items, ind?.ma)

  // 成交量
  renderVolumeChart(dates, data.items)

  // MACD
  renderMACDChart(dates, ind?.macd)

  // KDJ
  renderKDJChart(dates, ind?.kdj)

  // 联动：echarts.connect 统一管理 dataZoom 缩放 + axisPointer 十字线对齐
  const instances = [mainChart, volumeChart, macdChart, kdjChart].filter((c): c is ECharts => c != null)
  if (instances.length > 1) {
    echarts.connect(instances)
  }

  // 确保尺寸正确后重绘
  requestAnimationFrame(() => {
    mainChart?.resize()
    volumeChart?.resize()
    macdChart?.resize()
    kdjChart?.resize()
  })
}

function renderMainChart(dates: string[], klineData: number[][], items: KLineItem[], maData?: MAData) {
  if (!mainChartRef.value) return
  mainChart = echarts.init(mainChartRef.value)

  // 将 MA 的 0 值替换为 null（ECharts 遇到 null 自动跳过不绘制）
  const n = (arr?: number[]) => arr?.map(v => v === 0 ? null as any : v)

  const series: echarts.EChartsOption['series'][] = [
    {
      type: 'candlestick',
      data: klineData,
      itemStyle: {
        color: COLORS.up,
        color0: COLORS.down,
        borderColor: COLORS.up,
        borderColor0: COLORS.down,
      },
      barWidth: '55%',
    },
    // MA5
    {
      type: 'line',
      name: 'MA5',
      data: n(maData?.ma5),
      lineStyle: { width: 1, color: COLORS.ma5 },
      symbol: 'none',
      smooth: false,
    },
    // MA10
    {
      type: 'line',
      name: 'MA10',
      data: n(maData?.ma10),
      lineStyle: { width: 1, color: COLORS.ma10 },
      symbol: 'none',
    },
    // MA20
    {
      type: 'line',
      name: 'MA20',
      data: n(maData?.ma20),
      lineStyle: { width: 1, color: COLORS.ma10 },
      symbol: 'none',
    },
    // MA20
    {
      type: 'line',
      name: 'MA20',
      data: n(maData?.ma20),
      lineStyle: { width: 1, color: COLORS.ma20 },
      symbol: 'none',
    },
    // MA60
    {
      type: 'line',
      name: 'MA60',
      data: n(maData?.ma60),
      lineStyle: { width: 1, color: COLORS.ma60 },
      symbol: 'none',
    },
  ]

  mainChart.setOption({
    grid: { top: 15, right: 50, bottom: 25, left: 48 },
    axisPointer: {
      link: [{ xAxisIndex: 'all' }],
      axisLabel: { show: false },
    },
    xAxis: {
      type: 'category',
      data: dates,
      show: false,
      axisLabel: { fontSize: 10 },
    },
    yAxis: {
      type: 'value',
      scale: true,
      axisLabel: { fontSize: 10, formatter: '{value}' },
      splitLine: { show: true, lineStyle: { color: '#f0f0f0', type: 'dashed' } },
    },
    tooltip: {
      trigger: 'axis',
      confine: true,
      // 使用 dataIndex 从原始 items 取值，避免混合系列时 p.data 结构异常
      formatter(params: any) {
        const idx = params[0]?.dataIndex
        const item = idx != null && items[idx] ? items[idx] : null
        if (!item) return ''
        const prevClose = idx > 0 ? items[idx - 1].close : item.open
        // 辅助：价比昨收则着色
        const c = (val: number) => val > prevClose ? '#ef232a' : val < prevClose ? '#00943e' : '#333'
        const chg = item.close - prevClose
        const chgPct = prevClose !== 0 ? (chg / prevClose) * 100 : 0
        return `<b>${params[0]?.axisValue}</b><br/>`
          + `<span style="color:${c(item.open)}">开: ${item.open.toFixed(2)}</span>&nbsp;&nbsp;`
          + `<span style="color:${c(item.high)}">高: ${item.high.toFixed(2)}</span><br/>`
          + `<span style="color:${c(item.close)}">收: ${item.close.toFixed(2)}</span>&nbsp;&nbsp;`
          + `<span style="color:${c(item.low)}">低: ${item.low.toFixed(2)}</span><br/>`
          + `涨跌: <span style="color:${chg >= 0 ? '#ef232a' : '#00943e'}">${chg >= 0 ? '+' : ''}${chgPct.toFixed(2)}%</span><br/>`
          + `量: <span style="color:#666">${Math.round(item.volume / 100).toLocaleString()}手</span>`
      },
    },
    dataZoom: [
      { type: 'inside', start: 70, end: 100, groupId: 'klineGroup' },
    ],
    series,
  } as echarts.EChartsOption)

  // 用 updateAxisPointer 替代 mouseover，更可靠地跟踪悬停位置
  // mouseover 在快速滑动/空白区域/副图切换时会丢失事件
  mainChart.on('updateAxisPointer', (params: any) => {
    if (params.dataIndex != null && params.dataIndex >= 0) {
      hoverIndex.value = params.dataIndex
    }
  })
  // 鼠标离开图表区域时恢复默认（最后一天）
  mainChart.on('globalout', () => {
    hoverIndex.value = -1
  })
}

function renderVolumeChart(dates: string[], items: KLineItem[]) {
  if (!volumeChartRef.value) return
  volumeChart = echarts.init(volumeChartRef.value)

  const volData = items.map((item, idx) => ({
    value: item.volume,
    itemStyle: {
      color: getBarColor(item),
    },
  }))

  volumeChart.setOption({
    grid: { top: 8, right: 50, bottom: 22, left: 48 },
    axisPointer: {
      link: [{ xAxisIndex: 'all' }],
    },
    xAxis: {
      type: 'category',
      data: dates,
      show: false,
      boundaryGap: true,
    },
    yAxis: {
      type: 'value',
      scale: true,
      axisLabel: { fontSize: 9, formatter: (v: number) => formatVol(v) },
      splitLine: { show: false },
    },
    dataZoom: [{ type: 'inside', start: 70, end: 100, groupId: 'klineGroup' }],
    series: [{
      type: 'bar',
      data: volData,
      barWidth: '55%',
    }],
  } as echarts.EChartsOption)
}

function renderMACDChart(dates: string[], macdData?: MACDData) {
  if (!macdChartRef.value) return
  macdChart = echarts.init(macdChartRef.value)

  const macdBars = (macdData?.macd ?? []).map((val) => ({
    value: val,
    itemStyle: { color: (val ?? 0) >= 0 ? COLORS.macdUp : COLORS.macdDown },
  }))

  macdChart.setOption({
    grid: { top: 8, right: 50, bottom: 22, left: 48 },
    axisPointer: {
      link: [{ xAxisIndex: 'all' }],
    },
    xAxis: {
      type: 'category',
      data: dates,
      show: false,
    },
    yAxis: {
      type: 'value',
      scale: true,
      splitNumber: 1,
      axisLabel: { fontSize: 9 },
      splitLine: { show: false },
    },
    dataZoom: [{ type: 'inside', start: 70, end: 100, groupId: 'klineGroup' }],
    series: [
      {
        name: 'MACD',
        type: 'bar',
        data: macdBars,
        barWidth: '45%',
      },
      {
        name: 'DIF',
        type: 'line',
        data: macdData?.dif,
        lineStyle: { width: 1.2, color: COLORS.dif },
        symbol: 'none',
      },
      {
        name: 'DEA',
        type: 'line',
        data: macdData?.dea,
        lineStyle: { width: 1.2, color: COLORS.dea },
        symbol: 'none',
      },
    ],
  } as echarts.EChartsOption)
}

function renderKDJChart(dates: string[], kdjData?: KDJData) {
  if (!kdjChartRef.value) return
  kdjChart = echarts.init(kdjChartRef.value)

  kdjChart.setOption({
    grid: { top: 8, right: 50, bottom: 22, left: 48 },
    axisPointer: {
      link: [{ xAxisIndex: 'all' }],
    },
    xAxis: {
      type: 'category',
      data: dates,
      show: false,
    },
    yAxis: {
      type: 'value',
      scale: true,
      splitNumber: 2,
      axisLabel: { fontSize: 9 },
      splitLine: { show: true, lineStyle: { color: '#f0f0f0', type: 'dashed' } },
    },
    dataZoom: [{ type: 'inside', start: 70, end: 100, groupId: 'klineGroup' }],
    series: [
      {
        name: 'K',
        type: 'line',
        data: kdjData?.k,
        lineStyle: { width: 1.2, color: COLORS.kLine },
        symbol: 'none',
      },
      {
        name: 'D',
        type: 'line',
        data: kdjData?.d,
        lineStyle: { width: 1.2, color: COLORS.dLine },
        symbol: 'none',
      },
      {
        name: 'J',
        type: 'line',
        data: kdjData?.j,
        lineStyle: { width: 1.2, color: COLORS.jLine },
        symbol: 'none',
      },
    ],
  } as echarts.EChartsOption)
}

// ========== 辅助函数 ==========

function formatVol(v: number): string {
  if (v >= 100000000) return `${(v / 100000000).toFixed(1)}亿`
  if (v >= 10000) return `${(v / 10000).toFixed(0)}万`
  return String(v)
}

function formatVolNum(v: number): string {
  return formatVol(v)
}

function disposeCharts() {
  mainChart?.dispose()
  volumeChart?.dispose()
  macdChart?.dispose()
  kdjChart?.dispose()
  mainChart = null
  volumeChart = null
  macdChart = null
  kdjChart = null
}

// ========== 生命周期 ==========

watch(() => props.visible, (v) => {
  if (v && props.stockCode) {
    loadData()
  } else {
    disposeCharts()
    chartData.value = null
  }
})

// 股票切换时重新加载 K 线和指标数据
watch(() => props.stockCode, (code) => {
  if (code && props.visible) {
    loadData()
  }
})

onBeforeUnmount(() => {
  disposeCharts()
})
</script>

<style scoped>
.kline-tooltip {
  position: fixed;
  z-index: 9999;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.18), 0 2px 8px rgba(0, 0, 0, 0.08);
  padding: 12px;
  font-size: 13px;
  width: 860px;
  animation: kt-fade-in 0.18s ease-out;
  user-select: none;
}
@keyframes kt-fade-in {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
}

/* ====== 标题栏 ====== */
.kt-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
  padding-bottom: 8px;
  border-bottom: 1px solid #f0f0f0;
}
.kt-title-left {
  display: flex;
  align-items: baseline;
  gap: 8px;
}
.kt-name {
  font-size: 15px;
  font-weight: 700;
  color: #1a1a2e;
}
.kt-code {
  font-size: 12px;
  color: #999;
  font-family: 'SF Mono', Monaco, monospace;
}

/* ====== 周期切换 ====== */
.kt-period-tabs {
  display: flex;
  gap: 2px;
  background: #f5f5f5;
  border-radius: 4px;
  overflow: hidden;
}
.kt-tab {
  padding: 3px 14px;
  font-size: 12px;
  border: none;
  background: transparent;
  cursor: pointer;
  color: #666;
  transition: .15s;
}
.kt-tab.active {
  background: #1677ff;
  color: #fff;
  font-weight: 600;
}
.kt-tab:not(.active):hover {
  color: #333;
  background: #eee;
}

/* ====== 价格栏 ====== */
.kt-price-bar {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 6px;
  font-size: 12.5px;
  min-height: 24px;
}
.kt-price-left {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}
.kt-date {
  color: #999;
  min-width: 78px;      /* YYYY-MM-DD 固定宽度 */
}
.kt-price {
  font-size: 17px;
  font-weight: 700;
  font-family: 'SF Mono', Monaco, monospace;
  min-width: 56px;      /* X.XX 固定 */
  text-align: right;
}
.kt-change {
  font-size: 12px;
  font-weight: 600;
  font-family: 'SF Mono', Monaco, monospace;
  min-width: 56px;      /* +X.XX% 固定 */
}
.kt-ohlc-row {
  display: flex;
  gap: 0;
  flex-shrink: 0;
}
.kt-ohlc-item {
  display: inline-block;
  min-width: 62px;      /* 开/高/收/低 X.XX 固定 */
  text-align: left;
}
.kt-ma-info {
  margin-left: auto;
  color: #888;
  font-size: 11px;
  letter-spacing: 0.5px;
  flex-shrink: 0;
}

/* ====== 图表区域 ====== */
.kt-chart-wrap {
  display: flex;
  flex-direction: column;
  gap: 0;
}
.kt-main-chart {
  height: 280px;
}
.kt-sub-chart {
  display: flex;
  flex-direction: column;
  border-top: 1px solid #f5f5f5;
  height: 85px;
}
.kt-sub-label-bar {
  flex-shrink: 0;
  height: 20px;
  padding-left: 48px;
  font-size: 10px;
  font-family: 'SF Mono', Monaco, monospace;
  line-height: 20px;
  white-space: nowrap;
  color: #333;
  background: #fafafa;
  border-bottom: 1px solid #f0f0f0;
}
.kt-sub-canvas {
  flex: 1;
  min-height: 0;
}

/* ====== 加载/空状态 ====== */
.kt-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 300px;
  color: #999;
  font-size: 13px;
}
.kt-spinner {
  display: inline-block;
  width: 20px;
  height: 20px;
  border: 2.5px solid #e8e8e8;
  border-top-color: #1677ff;
  border-radius: 50%;
  animation: kt-spin 0.7s linear infinite;
}
@keyframes kt-spin {
  to { transform: rotate(360deg); }
}
.kt-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 200px;
  color: #bbb;
  font-size: 13px;
}
</style>
