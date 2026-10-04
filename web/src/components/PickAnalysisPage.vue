<template>
  <div class="pick-page">
    <div class="page-header">
      <h1>选股复盘</h1>
      <p>回放单个策略在指定时间段内每天选出的股票，查看它们在选出之后 N 个交易日的走势（收益以选股日收盘价为基准）</p>
    </div>

    <!-- ====== 查询栏 ====== -->
    <div class="filter-bar">
      <div class="fb-item fb-strategy">
        <label>策略</label>
        <SearchSingleSelect
          v-model="strategyId"
          :options="strategyOptions"
          placeholder="请选择策略"
          searchable
        />
      </div>

      <div class="fb-item">
        <label>日期区间</label>
        <div class="fb-range">
          <input v-model="startDate" type="date" class="fb-input" :max="endDate" />
          <span class="fb-sep">至</span>
          <input v-model="endDate" type="date" class="fb-input" :max="today" />
        </div>
      </div>

      <div class="fb-item">
        <label>观察天数</label>
        <input v-model.number="holdDays" type="number" class="fb-input fb-num" min="1" max="60" />
      </div>

      <button class="btn-run" :disabled="loading || !strategyId" @click="runAnalysis">
        {{ loading ? '复盘进行中...' : '运行复盘' }}
      </button>
      <button v-if="loading" class="btn-cancel" @click="cancelAnalysis">停止</button>
    </div>

    <p v-if="error" class="msg-error">{{ error }}</p>
    <p v-if="loading" class="msg-tip">
      ⏳ 正在按交易日逐个回放策略的选股条件：已完成 {{ progress.done }} / {{ progress.total }} 个交易日...
    </p>
    <p v-if="dayErrors.length" class="msg-warn">
      {{ dayErrors.length }} 个交易日复盘失败：<br />
      <span v-for="(msg, i) in dayErrors" :key="i">{{ msg }}</span>
    </p>

    <!-- ====== 概览 ====== -->
    <div v-if="result" class="summary-row">
      <div class="sum-card">
        <span class="sc-label">复盘区间</span>
        <span class="sc-value">{{ result.start_date || '-' }} ~ {{ result.end_date || '-' }}</span>
      </div>
      <div class="sum-card">
        <span class="sc-label">扫描交易日</span>
        <span class="sc-value">{{ result.scan_days }} 天</span>
      </div>
      <div class="sum-card">
        <span class="sc-label">命中次数</span>
        <span class="sc-value">{{ result.total_picks }} 次</span>
      </div>
      <div class="sum-card">
        <span class="sc-label">观察窗口</span>
        <span class="sc-value">T+1 ~ T+{{ result.hold_days }}</span>
      </div>
      <div class="sum-card">
        <span class="sc-label">T+{{ lastValidOffset }} 平均收益</span>
        <span class="sc-value" :style="{ color: avgReturn >= 0 ? '#ef232a' : '#00943e' }">
          {{ fmtPct(avgReturn) }}
        </span>
      </div>
      <div class="sum-card">
        <span class="sc-label">T+{{ lastValidOffset }} 胜率</span>
        <span class="sc-value">{{ fmtWinRate(winRate) }}</span>
      </div>
    </div>

    <!-- ====== 主体：命中列表 + 图表/表格 ====== -->
    <div v-if="result" class="content">
      <aside class="pick-list">
        <div class="pl-header">
          <span>命中记录（{{ flatPicks.length }}）</span>
          <div class="pl-actions">
            <button title="每只股票只勾选最后一次被命中的记录" @click="selectLatestPerStock">去重</button>
            <button @click="selectAll">全选</button>
            <button @click="clearSelection">清空</button>
          </div>
        </div>

        <div v-if="flatPicks.length === 0" class="pl-empty">所选区间内没有命中任何股票</div>

        <div v-else class="pl-body">
          <div v-for="day in result.days" :key="day.date" class="pl-day">
            <div class="pl-day-head">
              <span class="pl-day-date">{{ day.date }}</span>
              <span class="pl-day-count">命中 {{ day.pick_count }} / 扫描 {{ day.scanned }}</span>
              <button class="pl-day-btn" @click="toggleDay(day.picks)">切换本日</button>
            </div>
            <label
              v-for="pick in day.picks ?? []"
              :key="pickKey(pick)"
              class="pl-item"
              :class="{ active: selectedKeys.includes(pickKey(pick)) }"
            >
              <input
                type="checkbox"
                :checked="selectedKeys.includes(pickKey(pick))"
                @change="togglePick(pickKey(pick))"
              />
              <span class="pl-item-name">
                {{ pick.code }}
                <i>{{ pick.name }}</i>
              </span>
              <span class="pl-item-ret" :style="{ color: colorOf(finalReturn(pick)) }">
                {{ fmtPct(finalReturn(pick)) }}
              </span>
            </label>
          </div>
        </div>
      </aside>

      <section class="view-area">
        <div class="va-toolbar">
          <div class="seg-group">
            <button :class="['seg', { active: viewMode === 'chart' }]" @click="viewMode = 'chart'">📈 折线图</button>
            <button :class="['seg', { active: viewMode === 'table' }]" @click="viewMode = 'table'">📋 表格</button>
          </div>

          <span class="toolbar-sep">|</span>

          <div class="seg-group">
            <button
              v-for="opt in dimOptions"
              :key="opt.value"
              :class="['seg', { active: dim === opt.value }]"
              @click="dim = opt.value"
            >
              {{ opt.label }}
            </button>
          </div>

          <span v-if="viewMode === 'chart'" class="va-hint">虚线为同期上证指数</span>

          <span class="va-hint">已选 {{ selectedKeys.length }} 条</span>

          <template v-if="viewMode === 'table' && flatPicks.length">
            <div class="va-search">
              <input
                v-model="tableKeyword"
                placeholder="搜索代码 / 名称"
                @keydown.esc="tableKeyword = ''"
              />
              <button v-if="tableKeyword" class="va-search-clear" @click="tableKeyword = ''">✕</button>
            </div>

            <button
              class="va-export"
              :disabled="exportPicks.length === 0"
              :title="exportPicks.length ? `导出 ${exportPicks.length} 条记录（已勾选且符合搜索条件）` : '请先勾选需要导出的记录'"
              @click="exportTable"
            >导出 Excel</button>
          </template>
        </div>

        <PickReturnsChart
          v-if="viewMode === 'chart'"
          class="fill-card"
          :series="chartSeries"
          :labels="chartLabels"
          :dim-label="dimLabel"
          :benchmark="chartBenchmark"
        />
        <PickReturnsTable
          v-else
          class="fill-card"
          :picks="filteredPicks"
          :labels="labels"
          :dim="dim"
          :selected="selectedKeys"
          @toggle="togglePick"
        />
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import SearchSingleSelect from './SearchSingleSelect.vue'
import PickReturnsChart, { type PickSeries } from './PickReturnsChart.vue'
import PickReturnsTable from './PickReturnsTable.vue'
import { fetchStrategies } from '../api/strategies'
import { fetchIndexDaily, type IndexDailyPoint } from '../api/kline'
import * as XLSX from 'xlsx-js-style'
import {
  fetchStrategyPickDay,
  fetchStrategyPickDays,
  pickDimValue,
  pickKey,
  type PickDayResult,
  type PriceDim,
  type StockPick,
  type StrategyPicksResult,
} from '../api/picks'

// ========== 常量 ==========

const DIM_OPTIONS: { value: PriceDim; label: string }[] = [
  { value: 'open', label: '开盘 O' },
  { value: 'high', label: '最高 H' },
  { value: 'low', label: '最低 L' },
  { value: 'close', label: '收盘 C' },
]
const dimOptions = DIM_OPTIONS

/** 默认查询区间的日历天数（约 10 个交易日） */
const DEFAULT_RANGE_DAYS = 14

// ========== 查询条件 ==========

const strategies = ref<{ id: number; name: string }[]>([])
const strategyId = ref<number | null>(null)
const startDate = ref('')
const endDate = ref('')
const holdDays = ref(5)

const today = computed(() => formatDate(new Date()))
const strategyOptions = computed(() =>
  strategies.value.map(s => ({ value: s.id as any, label: s.name })),
)

// ========== 结果状态 ==========

const loading = ref(false)
const error = ref('')
const result = ref<StrategyPicksResult | null>(null)

/** 按天轮询进度 */
const progress = ref({ done: 0, total: 0 })
/** 单日复盘失败的记录 */
const dayErrors = ref<string[]>([])
/** 递增的运行令牌，用于停止旧的轮询 */
let runToken = 0

// ========== 视图状态 ==========

const viewMode = ref<'chart' | 'table'>('chart')
const dim = ref<PriceDim>('close')
const selectedKeys = ref<string[]>([])

/** 基准指数（上证指数）日线 OHLC：date(YYYY-MM-DD) → 行情点 */
const indexPoints = ref<Record<string, IndexDailyPoint>>({})

const dimLabel = computed(() => {
  const hit = DIM_OPTIONS.find(o => o.value === dim.value)
  return hit ? hit.label.replace(/ [OCHL]$/, '') : '收盘'
})

/** X 轴/表头：T+1 ~ T+N */
const labels = computed(() => {
  const n = result.value?.hold_days ?? holdDays.value
  return Array.from({ length: n }, (_, i) => `T+${i + 1}`)
})

/** 扁平化的命中列表 */
const flatPicks = computed<StockPick[]>(() => {
  if (!result.value) return []
  return result.value.days.flatMap(d => d.picks ?? [])
})

/** 已勾选的命中记录 */
const selectedPicks = computed<StockPick[]>(() =>
  flatPicks.value.filter(p => selectedKeys.value.includes(pickKey(p))),
)

/** 表格搜索关键字（代码 / 名称模糊匹配） */
const tableKeyword = ref('')

/** 按代码或名称模糊过滤后的表格数据 */
const filteredPicks = computed<StockPick[]>(() => {
  const kw = tableKeyword.value.trim().toLowerCase()
  if (!kw) return flatPicks.value
  return flatPicks.value.filter(p =>
    p.code.toLowerCase().includes(kw) || (p.name || '').toLowerCase().includes(kw),
  )
})

/** 导出范围：已勾选 且 命中当前搜索条件 */
const exportPicks = computed<StockPick[]>(() =>
  filteredPicks.value.filter(p => selectedKeys.value.includes(pickKey(p))),
)

/** 最后一个有数据的交易日序号（窗口末端，数据不足时取实际最大值） */
const lastValidOffset = computed(() => {
  let offset = 0
  for (const pick of flatPicks.value) {
    offset = Math.max(offset, pick.returns.length)
  }
  return Math.min(offset, labels.value.length)
})

/** 参与统计的记录：优先使用勾选项，否则使用全部命中 */
const statPicks = computed<StockPick[]>(() =>
  selectedPicks.value.length > 0 ? selectedPicks.value : flatPicks.value,
)

/** T+last 的平均收益率(%) */
const avgReturn = computed(() => {
  const values = statPicks.value
    .map(p => finalReturn(p))
    .filter((v): v is number => v !== null && Number.isFinite(v))
  if (values.length === 0) return Number.NaN
  return values.reduce((sum, v) => sum + v, 0) / values.length
})

/** T+last 的胜率(%) */
const winRate = computed(() => {
  const values = statPicks.value
    .map(p => finalReturn(p))
    .filter((v): v is number => v !== null && Number.isFinite(v))
  if (values.length === 0) return Number.NaN
  return values.filter(v => v > 0).length / values.length * 100
})

/**
 * 折线图横轴：所有已勾选项涉及的实际交易日（去重升序）。
 * 不同选股日的票按各自日期错开，例如 A 占 d1~d5、B 占 d2~d6。
 */
const chartDates = computed<string[]>(() => {
  const set = new Set<string>()
  for (const pick of selectedPicks.value) {
    for (const day of pick.returns) {
      set.add(day.date)
    }
  }
  return [...set].sort()
})

/** 横轴显示标签：MM-DD */
const chartLabels = computed(() => chartDates.value.map(d => d.slice(5)))

/** 图表数据：一条线 = 一只「股票@选股日」，按日期对齐到统一横轴 */
const chartSeries = computed<PickSeries[]>(() => {
  const dates = chartDates.value
  return selectedPicks.value.map(pick => {
    const byDate = new Map<string, number | null>()
    for (const day of pick.returns) {
      byDate.set(day.date, pickDimValue(day, dim.value))
    }
    return {
      key: pickKey(pick),
      label: `${pick.code} ${pick.name || ''} ${pick.date.slice(5)}`.trim(),
      values: dates.map(d => (byDate.has(d) ? byDate.get(d) ?? null : null)),
    }
  })
})

/** 取指数某交易日在指定维度下的价格（开/高/低/收） */
function indexPrice(point: IndexDailyPoint | undefined, d: PriceDim): number | null {
  if (!point) return null
  switch (d) {
    case 'open': return point.open
    case 'high': return point.high
    case 'low': return point.low
    case 'close': return point.close
    default: return null
  }
}

/**
 * 基准曲线：同期上证指数收益，按统一横轴日期对齐。
 *
 * 与个股口径完全一致：基准价为「选股日 A」的**收盘价**（不随维度变化），
 * 区间各日收益则取当前维度（开/高/低/收）的价格相对基准价计算；多个勾选项在同一日期上取平均值。
 * 指数数据缺失时返回 null，图表不绘制。
 */
const chartBenchmark = computed<(number | null)[]>(() => {
  const dates = chartDates.value
  const points = indexPoints.value
  if (dates.length === 0 || Object.keys(points).length === 0) return []

  const d = dim.value
  const sums = new Map<string, { sum: number; count: number }>()
  for (const pick of selectedPicks.value) {
    const base = points[pick.date]?.close ?? null
    if (!base || base <= 0) continue
    for (const day of pick.returns) {
      const price = indexPrice(points[day.date], d)
      if (!price || price <= 0) continue
      const bucket = sums.get(day.date) ?? { sum: 0, count: 0 }
      bucket.sum += (price / base - 1) * 100
      bucket.count++
      sums.set(day.date, bucket)
    }
  }
  return dates.map(d => {
    const bucket = sums.get(d)
    if (!bucket || bucket.count === 0) return null
    return Number((bucket.sum / bucket.count).toFixed(3))
  })
})

// ========== 方法 ==========

/** 取窗口末端（有数据的最后一个交易日）在所选维度下的收益率(%) */
function finalReturn(pick: StockPick): number | null {
  if (pick.returns.length === 0) return null
  const idx = Math.min(pick.returns.length, lastValidOffset.value) - 1
  const day = pick.returns[idx]
  if (!day) return null
  return pickDimValue(day, dim.value)
}

function fmtPct(value: number | null): string {
  if (value === null || !Number.isFinite(value)) return '-'
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`
}

function fmtWinRate(value: number): string {
  if (!Number.isFinite(value)) return '-'
  return `${value.toFixed(1)}%`
}

function colorOf(value: number | null): string {
  if (value === null || !Number.isFinite(value)) return '#bbb'
  return value >= 0 ? '#ef232a' : '#00943e'
}

function togglePick(key: string) {
  const idx = selectedKeys.value.indexOf(key)
  if (idx >= 0) {
    selectedKeys.value.splice(idx, 1)
  } else {
    selectedKeys.value.push(key)
  }
}

/** 勾选/取消勾选某一天的全部命中（有未选的则全选，已全选则取消） */
function toggleDay(picks: StockPick[]) {
  const keys = (picks ?? []).map(pickKey)
  const allSelected = keys.length > 0 && keys.every(k => selectedKeys.value.includes(k))
  if (allSelected) {
    selectedKeys.value = selectedKeys.value.filter(k => !keys.includes(k))
  } else {
    selectedKeys.value = [...new Set([...selectedKeys.value, ...keys])]
  }
}

function selectAll() {
  selectedKeys.value = flatPicks.value.map(pickKey)
}

function clearSelection() {
  selectedKeys.value = []
}

/**
 * 去重勾选：每只股票只保留「最后一次被命中」的那条记录。
 * 同一只股票在多个交易日被选中时，取信号日最晚的一条。
 */
function selectLatestPerStock() {
  const lastKey = new Map<string, string>()
  const lastDate = new Map<string, string>()
  for (const pick of flatPicks.value) {
    const prevDate = lastDate.get(pick.code)
    if (!prevDate || pick.date >= prevDate) {
      lastDate.set(pick.code, pick.date)
      lastKey.set(pick.code, pickKey(pick))
    }
  }
  selectedKeys.value = Array.from(lastKey.values())
}

/** 把单个交易日的结果并入汇总结果（按日期升序） */
function mergeDay(day: PickDayResult) {
  const cur = result.value
  if (!cur) return
  const days = [...cur.days, day].sort((a, b) => a.date.localeCompare(b.date))
  cur.days = days
  cur.scan_days = days.length
  cur.total_picks = days.reduce((sum, d) => sum + d.pick_count, 0)
}

/**
 * 运行复盘：先取交易日列表，再逐天请求并在前端汇总。
 * 每个请求只处理一天，避免后端长耗时请求被网关/代理掐断。
 */
async function runAnalysis() {
  if (!strategyId.value) return
  const token = ++runToken
  loading.value = true
  error.value = ''
  dayErrors.value = []
  result.value = null
  selectedKeys.value = []
  indexPoints.value = {}
  tableKeyword.value = ''
  progress.value = { done: 0, total: 0 }

  try {
    // 逐日回放前先取大盘指数日线，作为同期基准曲线
    loadIndexCloses(startDate.value, endDate.value, holdDays.value)

    const meta = await fetchStrategyPickDays(strategyId.value, {
      start_date: startDate.value,
      end_date: endDate.value,
      hold_days: holdDays.value,
    })
    if (token !== runToken) return

    const dates = meta.dates ?? []
    progress.value = { done: 0, total: dates.length }
    result.value = {
      strategy_id: meta.strategy_id,
      strategy_name: meta.strategy_name,
      start_date: meta.start_date,
      end_date: meta.end_date,
      hold_days: meta.hold_days,
      scan_days: 0,
      total_picks: 0,
      days: [],
    }
    if (dates.length === 0) return

    for (const date of dates) {
      if (token !== runToken) return
      try {
        const day = await fetchStrategyPickDay(strategyId.value, {
          date,
          hold_days: meta.hold_days,
        })
        if (token !== runToken) return
        mergeDay(day)
        // 首个有命中的交易日先默认勾上，后续命中日由用户自行勾选
        if (selectedKeys.value.length === 0 && (day.picks?.length ?? 0) > 0) {
          selectedKeys.value = day.picks.slice(0, 10).map(pickKey)
        }
      } catch (e) {
        dayErrors.value = [...dayErrors.value, `${date}：${(e as Error).message || '复盘失败'}`]
      }
      progress.value = { ...progress.value, done: progress.value.done + 1 }
    }
  } catch (e) {
    error.value = (e as Error).message || '复盘失败'
  } finally {
    if (token === runToken) loading.value = false
  }
}

/**
 * 预取上证指数日线，作为同期基准。
 * 区间需覆盖「复盘区间 + 观察窗口」，失败时静默降级（不绘制基准线）。
 */
async function loadIndexCloses(start: string, end: string, hold: number) {
  try {
    // 起始日再往前多取若干自然日，确保覆盖首个选股日的「前一交易日」（A-1）
    const resp = await fetchIndexDaily(shiftDays(start, -20), shiftDays(end, (hold + 8) * 2))
    const map: Record<string, IndexDailyPoint> = {}
    for (const item of resp.items ?? []) {
      if (item.open > 0 && item.high > 0 && item.low > 0 && item.close > 0) map[item.date] = item
    }
    indexPoints.value = map
  } catch (e) {
    console.error('加载上证指数日线失败:', e)
    indexPoints.value = {}
  }
}

/** 日期加/减自然日 */
function shiftDays(dateStr: string, days: number): string {
  const d = new Date(`${dateStr}T00:00:00`)
  d.setDate(d.getDate() + days)
  return formatDate(d)
}

/** 导出的 sheet 定义：OHLC 各一个 sheet 页 */
const EXPORT_SHEETS: { dim: PriceDim; name: string }[] = [
  { dim: 'open', name: '开盘O' },
  { dim: 'high', name: '最高H' },
  { dim: 'low', name: '最低L' },
  { dim: 'close', name: '收盘C' },
]

/** 导出表格数据为 Excel：OHLC 各占一个 sheet 页，每页都含区间最高/最低；只导出已勾选的记录 */
function exportTable() {
  if (!exportPicks.value.length) return
  const n = labels.value.length

  const wb = XLSX.utils.book_new()
  for (const { dim, name } of EXPORT_SHEETS) {
    const head = ['代码', '名称', '信号日', '基准价(元)']
    for (let i = 1; i <= n; i++) head.push(`T+${i}(%)`)
    head.push('区间最高(%)', '区间最低(%)')

    // 收益列（T+1..T+N、区间最高、区间最低）在下表中的列号，用于着色
    const valueCols: number[] = []
    for (let i = 0; i < n; i++) valueCols.push(4 + i)
    valueCols.push(4 + n, 5 + n)

    const rows = exportPicks.value.map(pick => {
      const row: (string | number)[] = [pick.code, pick.name, pick.date, pick.base_close]
      for (let i = 0; i < n; i++) {
        const v = pickDimValue(pick.returns[i], dim)
        row.push(v === null || v === undefined ? '' : Number(v.toFixed(3)))
      }
      row.push(Number(pick.max_pct.toFixed(3)), Number(pick.min_pct.toFixed(3)))
      return row
    })

    const ws = XLSX.utils.aoa_to_sheet([head, ...rows])

    // 表头加粗
    head.forEach((_, c) => {
      const cell = ws[XLSX.utils.encode_cell({ r: 0, c })]
      if (cell) cell.s = { font: { bold: true, color: { rgb: 'FF333333' } }, fill: { fgColor: { rgb: 'FFF2F4F8' } } }
    })
    // 收益单元格按正负着色：涨红跌绿
    rows.forEach((row, r) => {
      for (const c of valueCols) {
        const v = row[c]
        if (typeof v !== 'number' || v === 0) continue
        const cell = ws[XLSX.utils.encode_cell({ r: r + 1, c })]
        if (cell) cell.s = { font: { color: { rgb: v > 0 ? 'FFEF232A' : 'FF00943E' } } }
      }
    })

    ws['!cols'] = head.map(() => ({ wch: 10 }))
    XLSX.utils.book_append_sheet(wb, ws, name)
  }

  const fileName = `选股复盘_${result.value?.strategy_name ?? '策略'}_${startDate.value}_${endDate.value}.xlsx`
  XLSX.writeFile(wb, fileName)
}

/** 停止当前轮询 */
function cancelAnalysis() {
  runToken++
  loading.value = false
}

function formatDate(d: Date): string {
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${d.getFullYear()}-${m}-${day}`
}

/** 初始化默认查询区间 */
function initDefaultRange() {
  const now = new Date()
  const start = new Date(now.getTime() - DEFAULT_RANGE_DAYS * 24 * 3600 * 1000)
  endDate.value = formatDate(now)
  startDate.value = formatDate(start)
}

async function loadStrategies() {
  try {
    const resp = await fetchStrategies('', 1, 100)
    const list = Array.isArray(resp.list) ? resp.list : []
    strategies.value = list.map((s: any) => ({ id: s.id, name: s.name ?? '' }))
  } catch (e) {
    console.error('加载策略列表失败:', e)
  }
}

// ========== 生命周期 ==========

onMounted(() => {
  initDefaultRange()
  loadStrategies()
})
</script>

<style scoped>
.pick-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex: 1;
  min-height: 0; /* 撑满容器，页面级滚动条交由卡片内部 */
}
/* 头部/筛选/概览等固定高度，不参与压缩 */
.page-header,
.filter-bar,
.msg-error,
.msg-tip,
.msg-warn,
.summary-row { flex-shrink: 0; }

/* ====== 查询栏 ====== */
.filter-bar {
  display: flex;
  align-items: flex-end;
  flex-wrap: wrap;
  gap: 14px;
  padding: 14px 16px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 1px 4px rgba(0,0,0,.06);
}
.fb-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.fb-item > label {
  font-size: 12.5px;
  color: #888;
}
.fb-strategy {
  min-width: 260px;
}
.fb-range {
  display: flex;
  align-items: center;
  gap: 8px;
}
.fb-sep { color: #999; font-size: 13px; }
.fb-input {
  height: 40px;
  padding: 0 10px;
  border: 1.5px solid #d9d9d9;
  border-radius: 8px;
  font-size: 14px;
  color: #333;
  outline: none;
  transition: border-color .15s;
}
.fb-input:focus { border-color: #1677ff; }
.fb-num { width: 96px; }

.btn-run {
  height: 40px;
  padding: 0 22px;
  border: none;
  border-radius: 8px;
  background: #1677ff;
  color: #fff;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: background .15s;
}
.btn-run:hover:not(:disabled) { background: #4096ff; }
.btn-run:disabled { background: #c9d7ea; cursor: not-allowed; }

.btn-cancel {
  height: 40px;
  padding: 0 18px;
  border: 1.5px solid #ff7875;
  border-radius: 8px;
  background: #fff;
  color: #cf1322;
  font-size: 14px;
  cursor: pointer;
}
.btn-cancel:hover { background: #fff1f0; }

.msg-error {
  padding: 10px 14px;
  border-radius: 8px;
  background: #fff1f0;
  color: #cf1322;
  font-size: 13.5px;
}
.msg-tip {
  padding: 10px 14px;
  border-radius: 8px;
  background: #e6f4ff;
  color: #1677ff;
  font-size: 13.5px;
}
.msg-warn {
  padding: 10px 14px;
  border-radius: 8px;
  background: #fffbe6;
  color: #ad6800;
  font-size: 13px;
  line-height: 1.8;
}
.msg-warn span { display: block; }

/* ====== 概览 ====== */
.summary-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 12px;
}
.sum-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 14px;
  background: #fff;
  border-radius: 10px;
  box-shadow: 0 1px 4px rgba(0,0,0,.06);
}
.sc-label { font-size: 12.5px; color: #999; }
.sc-value { font-size: 16px; font-weight: 700; color: #1a1a2e; }

/* ====== 主体 ====== */
.content {
  display: flex;
  gap: 14px;
  align-items: stretch; /* 左右两卡片底部对齐 */
  flex: 1;
  min-height: 0; /* 高度受限，滚动发生在卡片内部 */
}

/* -- 左侧命中列表 -- */
.pick-list {
  width: 288px;
  flex-shrink: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 1px 4px rgba(0,0,0,.06);
  overflow: hidden;
}
.pl-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border-bottom: 1px solid #f0f0f0;
  font-size: 13px;
  font-weight: 600;
  color: #333;
}
.pl-actions { display: flex; gap: 6px; }
.pl-actions button {
  border: 1px solid #e0e0e0;
  background: #fff;
  border-radius: 6px;
  padding: 3px 8px;
  font-size: 12px;
  color: #666;
  cursor: pointer;
}
.pl-actions button:hover { border-color: #1677ff; color: #1677ff; }

.pl-body { flex: 1; min-height: 0; overflow-y: auto; }
.pl-day-head {
  position: sticky;
  top: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 12px;
  background: #fafafa;
  border-bottom: 1px solid #f0f0f0;
  font-size: 12.5px;
  color: #666;
}
.pl-day-date { font-weight: 600; color: #333; }
.pl-day-count { flex: 1; color: #999; }
.pl-day-btn {
  border: none;
  background: none;
  color: #1677ff;
  font-size: 12px;
  cursor: pointer;
  padding: 0;
}
.pl-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 12px;
  cursor: pointer;
  border-bottom: 1px solid #f7f7f7;
  font-size: 13px;
}
.pl-item:hover { background: #f9fbff; }
.pl-item.active { background: #e6f4ff; }
.pl-item-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #333;
}
.pl-item-name i { font-style: normal; color: #999; font-size: 12px; }
.pl-item-ret { font-variant-numeric: tabular-nums; font-weight: 600; }
.pl-empty { padding: 30px 12px; text-align: center; color: #bbb; font-size: 13.5px; }

/* -- 右侧视图区 -- */
.view-area { flex: 1; min-width: 0; min-height: 0; display: flex; flex-direction: column; }
.va-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 10px 14px;
  margin-bottom: 12px;
  background: #fff;
  border-radius: 10px;
  box-shadow: 0 1px 4px rgba(0,0,0,.06);
  flex-shrink: 0;
}
/* 图表/表格卡片撑满剩余高度 */
.fill-card { flex: 1; min-height: 0; display: flex; flex-direction: column; overflow: hidden; }
/* 表格搜索框 */
.va-search {
  position: relative;
  display: flex;
  align-items: center;
  margin-left: auto;
}
.va-search input {
  width: 150px;
  padding: 4px 22px 4px 10px;
  border: 1px solid #e0e0e0;
  border-radius: 6px;
  font-size: 12.5px;
  color: #333;
  outline: none;
  transition: border-color .15s;
}
.va-search input:focus { border-color: #1677ff; }
.va-search-clear {
  position: absolute;
  right: 4px;
  border: none;
  background: transparent;
  color: #bbb;
  font-size: 11px;
  cursor: pointer;
  padding: 0 2px;
  line-height: 1;
}
.va-search-clear:hover { color: #666; }

.va-export {
  margin-left: 0;
  border: 1px solid #d6e4ff;
  background: #f0f5ff;
  color: #1d5cff;
  border-radius: 6px;
  padding: 4px 12px;
  font-size: 12.5px;
  cursor: pointer;
  transition: background .15s;
}
.va-export:hover { background: #dce7ff; }
.va-export:disabled {
  background: #f5f5f5;
  border-color: #e5e5e5;
  color: #bbb;
  cursor: not-allowed;
}
.toolbar-sep { color: #e0e0e0; }
.seg-group {
  display: flex;
  border: 1px solid #e0e0e0;
  border-radius: 8px;
  overflow: hidden;
}
.seg {
  border: none;
  background: #fff;
  padding: 6px 12px;
  font-size: 13px;
  color: #555;
  cursor: pointer;
  transition: all .12s;
}
.seg:hover { background: #f5f8ff; }
.seg.active { background: #1677ff; color: #fff; }
.va-checkbox {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: #555;
  cursor: pointer;
}
.va-hint { margin-left: auto; font-size: 12.5px; color: #999; }
</style>
