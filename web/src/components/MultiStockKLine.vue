<template>
  <div class="multi-kline-container">
    <!-- K线图网格 -->
    <div
      class="mk-grid"
      :style="{ gridTemplateColumns: `repeat(${gridColumns}, 1fr)` }"
    >
      <div v-for="(stock, idx) in stocks" :key="stock.code" class="mk-card">
        <!-- 卡片标题栏：名称+代码 | 日K周K月K -->
        <div class="mk-card-header">
          <div class="mk-stock-info">
            <span class="mk-name">{{ stock.name }}</span>
            <span class="mk-code">{{ stock.code }}</span>
          </div>
        </div>

        <!-- 数据摘要行：日期 价格 涨跌幅 MA -->
        <div class="mk-data-row" v-if="getFullData(idx)">
          <div class="mdr-left">
            <span class="mdr-date">{{ getFullData(idx)!.date }}</span>
            <span class="mdr-price" :style="{ color: getFullData(idx)!.change >= 0 ? '#ef232a' : '#00943e' }">
              {{ getFullData(idx)!.close.toFixed(2) }}
            </span>
            <span class="mdr-chg" :style="{ color: getFullData(idx)!.change >= 0 ? '#ef232a' : '#00943e' }">
              {{ getFullData(idx)!.change >= 0 ? '+' : '' }}{{ getFullData(idx)!.changePct.toFixed(2) }}%
            </span>
          </div>
          <div class="mdr-right">
            <template v-if="getFullData(idx)!.ma || getHoverMa(idx)">
              <span class="mdr-ma mdr-ma5">MA5:<b>{{ getDisplayMa(idx, 'ma5') }}</b></span>
              <span class="mdr-ma mdr-ma10">MA10:<b>{{ getDisplayMa(idx, 'ma10') }}</b></span>
              <span class="mdr-ma mdr-ma20">MA20:<b>{{ getDisplayMa(idx, 'ma20') }}</b></span>
              <span class="mdr-ma mdr-ma60">MA60:<b>{{ getDisplayMa(idx, 'ma60') }}</b></span>
            </template>
          </div>
        </div>

        <!-- 加载状态 -->
        <div v-if="loadingSet.has(stock.code)" class="mk-card-loading">
          <span class="mk-spinner"></span> 加载中...
        </div>

        <!-- 图表 -->
        <div v-else class="mk-chart-wrap">
          <div :ref="(el) => setChartRef(idx, el as HTMLDivElement)" class="mk-chart"></div>
        </div>
      </div>
    </div>

    <!-- 空状态 -->
    <div v-if="stocks.length === 0 && !loading" class="mk-empty">
      暂无股票数据，请先运行选股筛选
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import * as echarts from 'echarts'
import type { ECharts } from 'echarts'
import { fetchKLine, type KLineResponse, type KLinePeriod, type MAData } from '../api/kline'

// ========== Props ==========

interface StockItem {
  code: string
  name: string
}

const props = defineProps<{
  stocks: StockItem[]
  period?: KLinePeriod   // 外部控制的周期（工具栏联动）
  columns?: number       // 外部控制的列数，默认2
}>()

// ========== 周期配置 ==========

// ========== 状态 ==========

/** 当前周期：优先使用外部prop，否则用内部状态 */
const currentPeriod = computed(() => props.period ?? 'daily' as KLinePeriod)
const gridColumns = computed(() => props.columns ?? 2)
const loading = ref(false)
const loadingCount = ref(0)
const loadingSet = ref(new Set<string>()) // 正在加载的股票代码

/** 每只股票的K线数据 */
const klineDataMap = ref<Map<string, KLineResponse>>(new Map())

/** 图表实例数组 */
const chartInstances = ref<(ECharts | null)[]>([])
const chartRefs = ref<(HTMLDivElement | null)[]>([])

/** 鼠标悬停时的 MA 值（key=stock index） */
const hoverMaMap = ref<Map<number, MAData>>(new Map())

// ========== A 股配色 ==========

const COLORS = {
  up: '#ef232a',
  down: '#00943e',
  unch: '#888888',
  ma5: '#1a1a1a',
  ma10: '#FF9033',
  ma20: '#C71585',
  ma60: '#0066FF',
}

// ========== 方法 ==========

function setChartRef(idx: number, el: HTMLDivElement | null) {
  chartRefs.value[idx] = el
}

/** 完整数据摘要（用于卡片头部信息行） */
interface FullData {
  date: string
  close: number
  change: number
  changePct: number
  ma?: MAData
}

/** 获取最新完整数据 */
function getFullData(idx: number): FullData | null {
  const stock = props.stocks[idx]
  if (!stock) return null
  const data = klineDataMap.value.get(stock.code)
  const items = data?.items
  if (!items || items.length === 0) return null
  const cur = items[items.length - 1]
  const prevClose = items.length > 1 ? items[items.length - 2].close : cur.open
  const change = cur.close - prevClose
  const changePct = prevClose !== 0 ? (change / prevClose) * 100 : 0

  return { date: cur.date, close: cur.close, change, changePct, ma: data.indicators?.ma }
}

/** 格式化 MA 值：取数组最后一项 */
function fmtMa(arr?: number[]): string {
  if (!arr || arr.length === 0) return '--'
  const v = arr[arr.length - 1]
  // MA 的 0 值视为无效
  if (v === 0) return '--'
  return v.toFixed(2)
}

/** 获取鼠标悬停的 MA 数据 */
function getHoverMa(idx: number): MAData | undefined {
  return hoverMaMap.value.get(idx)
}

/** 获取 MA 显示值：优先用 hover 值，否则取最后一日 */
function getDisplayMa(idx: number, key: keyof MAData): string {
  const hover = hoverMaMap.value.get(idx)
  const hoverArr = hover?.[key]
  if (hoverArr && hoverArr.length > 0 && hoverArr[hoverArr.length - 1] !== 0) {
    return fmtMa(hoverArr)
  }
  const data = klineDataMap.value.get(props.stocks[idx]?.code ?? '')
  const maArr = data?.indicators?.ma?.[key]
  return fmtMa(maArr ?? [])
}

/** 并发加载所有K线数据 */
async function loadAllData() {
  const stockList = props.stocks
  if (stockList.length === 0) {
    klineDataMap.value.clear()
    disposeAllCharts()
    return
  }

  loading.value = true
  loadingCount.value = stockList.length
  const newLoadingSet = new Set<string>(stockList.map(s => s.code))
  loadingSet.value = newLoadingSet

  // 清空旧数据
  klineDataMap.value = new Map()
  hoverMaMap.value.clear()
  disposeAllCharts()

  // 并发请求所有K线数据
  const promises = stockList.map(async (stock) => {
    try {
      const data = await fetchKLine(stock.code, currentPeriod.value, 250)
      klineDataMap.value.set(stock.code, data)
    } catch (e) {
      console.error(`[MultiStockKLine] 加载 ${stock.code} 失败:`, e)
    } finally {
      newLoadingSet.delete(stock.code)
      loadingSet.value = new Set(newLoadingSet)
      loadingCount.value = newLoadingSet.size
    }
  })

  await Promise.all(promises)

  loading.value = false
  loadingCount.value = 0

  // 数据全部加载完毕后渲染图表
  await nextTick()
  renderAllCharts()
}

/** 渲染所有图表 */
function renderAllCharts() {
  props.stocks.forEach((stock, idx) => {
    const data = klineDataMap.value.get(stock.code)
    if (!data || data.items.length === 0) return
    const container = chartRefs.value[idx]
    if (!container || container.clientWidth === 0) return

    // 延迟渲染确保容器尺寸就绪
    requestAnimationFrame(() => {
      renderSingleChart(idx, data!)
    })
  })
}

/** 渲染单个图表 */
function renderSingleChart(idx: number, data: KLineResponse) {
  const container = chartRefs.value[idx]
  if (!container) return

  // 销毁旧实例
  if (chartInstances.value[idx]) {
    chartInstances.value[idx].dispose()
    chartInstances.value[idx] = null
  }

  const chart = echarts.init(container)
  chartInstances.value[idx] = chart

  const dates = data.items.map(i => i.date)
  const klineData = data.items.map(i => [i.open, i.close, i.low, i.high])
  const volumeData = data.items.map(i => i.volume)
  const maData = data.indicators?.ma

  // 将 MA 的 0 值替换为 null
  const n = (arr?: number[]) => arr?.map(v => v === 0 ? null as any : v)

  // 成交量配色（根据涨跌）
  const volColors = data.items.map((item, i) =>
    i > 0 ? (item.close >= data.items[i - 1].close ? COLORS.up : COLORS.down) : COLORS.up
  )

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
    { type: 'line', name: 'MA5', data: n(maData?.ma5), lineStyle: { width: 1, color: COLORS.ma5 }, symbol: 'none' },
    { type: 'line', name: 'MA10', data: n(maData?.ma10), lineStyle: { width: 1, color: COLORS.ma10 }, symbol: 'none' },
    { type: 'line', name: 'MA20', data: n(maData?.ma20), lineStyle: { width: 1, color: COLORS.ma20 }, symbol: 'none' },
    { type: 'line', name: 'MA60', data: n(maData?.ma60), lineStyle: { width: 1, color: COLORS.ma60 }, symbol: 'none' },
    // 成交量
    {
      type: 'bar',
      name: '成交量',
      data: volumeData,
      xAxisIndex: 1,
      yAxisIndex: 1,
      itemStyle: { color: (params: any) => volColors[params.dataIndex] ?? '#ccc' },
      barWidth: '55%',
    },
  ]

  chart.setOption({
    grid: [
      // K线图网格（主图）—— 占比 ~70%
      { top: 22, right: 58, bottom: '32%', left: 62 },
      // 成交量网格（副图）—— 占比 ~30%
      { top: '70%', right: 58, bottom: 36, left: 62 },
    ],
    xAxis: [
      {
        type: 'category',
        data: dates,
        axisLabel: { fontSize: 9, show: false },
        gridIndex: 0,
      },
      {
        type: 'category',
        data: dates,
        axisLabel: { fontSize: 9, interval: 25 },
        gridIndex: 1,
        axisTick: { show: false },
        axisLine: { show: true, lineStyle: { color: '#e8e8e8' } },
      },
    ],
    yAxis: [
      {
        type: 'value',
        scale: true,
        position: 'right',
        axisLabel: { fontSize: 9, formatter: '{value}' },
        splitLine: { show: true, lineStyle: { color: '#f0f0f0', type: 'dashed' } },
        splitNumber: 3,
        gridIndex: 0,
      },
      {
        type: 'value',
        scale: true,
        position: 'right',
        axisLabel: { fontSize: 8, formatter: (v: number) => v >= 1e8 ? (v / 1e8).toFixed(0) + '亿' : v >= 1e4 ? (v / 1e4).toFixed(0) + '万' : v.toString() },
        splitLine: { show: false },
        splitNumber: 2,
        gridIndex: 1,
      },
    ],
    tooltip: {
      trigger: 'axis',
      confine: true,
      formatter(params: any) {
        const i = params[0]?.dataIndex
        const item = i != null && data.items[i] ? data.items[i] : null
        if (!item) return ''
        const prevClose = i > 0 ? data.items[i - 1].close : item.open
        const c = (val: number) => val > prevClose ? '#ef232a' : val < prevClose ? '#00943e' : '#333'
        const chg = item.close - prevClose
        const chgPct = prevClose !== 0 ? (chg / prevClose) * 100 : 0
        return `<b>${params[0]?.axisValue}</b><br/>`
          + `<span style="color:${c(item.open)}">开: ${item.open.toFixed(2)}</span>&nbsp;`
          + `<span style="color:${c(item.high)}">高: ${item.high.toFixed(2)}</span><br/>`
          + `<span style="color:${c(item.close)}">收: ${item.close.toFixed(2)}</span>&nbsp;`
          + `<span style="color:${c(item.low)}">低: ${item.low.toFixed(2)}</span><br/>`
          + `成交量: ${(item.volume / 10000).toFixed(0)}万股<br/>`
          + `涨跌: <span style="color:${chg >= 0 ? '#ef232a' : '#00943e'}">${chg >= 0 ? '+' : ''}${chgPct.toFixed(2)}%</span>`
      },
    },
    dataZoom: [
      { type: 'inside', xAxisIndex: [0, 1], start: 70, end: 100, filterMode: 'weakFilter' },
    ],
    series,
  } as echarts.EChartsOption)

  // 绑定鼠标事件：动态更新卡片右上角的 MA 值
  // 使用 updateAxisPointer 而非 mousemove —— 前者在轴指针移动时始终触发，
  // 而 mousemove 仅在悬停到 K 线柱体时才携带 dataIndex
  chart.on('updateAxisPointer', (event: any) => {
    const i = event.dataIndex ?? event.axesInfo?.[0]?.value
    // 尝试从 axesInfo 取 index
    let idxVal = i
    if (idxVal === undefined && event.axesInfo) {
      const axis = event.axesInfo.find((a: any) => a.axisIndex === 0)
      if (axis != null && axis.value != null) {
        idxVal = dates.indexOf(axis.value)
        if (idxVal < 0) idxVal = undefined
      }
    }
    if (idxVal == null || !maData) {
      hoverMaMap.value.delete(idx)
      return
    }
    hoverMaMap.value.set(idx, {
      ma5: maData.ma5?.slice(0, Number(idxVal) + 1),
      ma10: maData.ma10?.slice(0, Number(idxVal) + 1),
      ma20: maData.ma20?.slice(0, Number(idxVal) + 1),
      ma60: maData.ma60?.slice(0, Number(idxVal) + 1),
    })
  })
  chart.on('globalout', () => {
    hoverMaMap.value.delete(idx)
  })
}

/** 销毁所有图表实例 */
function disposeAllCharts() {
  chartInstances.value.forEach(chart => chart?.dispose())
  chartInstances.value = []
}

// ========== 生命周期 ==========

let resizeObserver: ResizeObserver | null = null

onMounted(() => {
  // 初始化 ResizeObserver
  resizeObserver = new ResizeObserver(() => {
    chartInstances.value.forEach(chart => chart?.resize())
  })
  if (resizeObserver) {
    resizeObserver.observe(document.documentElement)
  }
  // 组件挂载后立即加载数据
  if (props.stocks.length > 0) {
    loadAllData()
  }
})

// 监听外部周期变化，自动重新加载
watch(() => props.period, (newVal, oldVal) => {
  if (newVal && newVal !== oldVal) {
    loadAllData()
  }
})

// 监听stocks变化

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  disposeAllCharts()
})

// ========== 监听stocks变化 ==========

watch(() => props.stocks, () => {
  loadAllChartsDebounced()
}, { deep: true })

let debounceTimer: ReturnType<typeof setTimeout> | null = null
function loadAllChartsDebounced() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    loadAllData()
  }, 300)
}

defineExpose({ refresh: loadAllData })
</script>

<style scoped>
.multi-kline-container {
  display: flex;
  flex-direction: column;
  gap: 8px;
  height: 100%;
  overflow: auto;
}

/* ====== 网格布局 ====== */
.mk-grid {
  display: grid;
  column-gap: 12px;
  row-gap: 12px;
  padding: 8px;
  overflow-y: auto;
  align-items: start;
  /* 自动填充高度 */
  min-height: 460px;
}

/* ====== 单个卡片 ====== */
.mk-card {
  background: #fff;
  border: 1px solid #e8e8e8;
  border-radius: 8px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-width: 300px;
  min-height: 400px;
}
.mk-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: linear-gradient(to bottom, #f8faff, #fff);
  border-bottom: 1px solid #f0f0f0;
  flex-shrink: 0;
}
.mk-stock-info {
  display: flex;
  align-items: baseline;
  gap: 8px;
}
.mk-name {
  font-size: 13px;
  font-weight: 700;
  color: #1a1a2e;
}
.mk-code {
  font-size: 11px;
  color: #999;
  font-family: 'SF Mono', Monaco, monospace;
}

/* ====== 数据摘要行 ====== */
.mk-data-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 12px;
  background: #fff;
  border-bottom: 1px solid #f5f5f5;
  flex-shrink: 0;
  gap: 8px;
  overflow-x: auto;
}
.mdr-left {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.mdr-right {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.mdr-date {
  font-size: 11px;
  color: #999;
  white-space: nowrap;
}
.mdr-price {
  font-size: 14px;
  font-weight: 700;
  font-family: 'SF Mono', Monaco, monospace;
}
.mdr-chg {
  font-size: 11.5px;
  font-weight: 600;
  font-family: 'SF Mono', Monaco, monospace;
}
.mdr-ma {
  font-size: 10.5px;
  white-space: nowrap;
}
.mdr-ma b { font-weight: 600; }
.mdr-ma5 { color: #1a1a1a; }
.mdr-ma10 { color: #FF9033; }
.mdr-ma20 { color: #C71585; }
.mdr-ma60 { color: #0066FF; }

/* ====== 图表区域 ====== */
.mk-chart-wrap {
  flex: 1;
  min-height: 340px;
  overflow: hidden;
}
.mk-chart {
  width: 100%;
  height: 340px;
}

/* ====== 加载状态 ====== */
.mk-card-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  height: 340px;
  color: #999;
  font-size: 12px;
}
.mk-spinner {
  display: inline-block;
  width: 18px;
  height: 18px;
  border: 2px solid #e0e0e0;
  border-top-color: #1677ff;
  border-radius: 50%;
  animation: mk-spin 0.7s linear infinite;
}
@keyframes mk-spin {
  to { transform: rotate(360deg); }
}

/* ====== 空状态 ====== */
.mk-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 200px;
  color: #bbb;
  font-size: 14px;
}
</style>
