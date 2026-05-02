<template>
  <div class="backtest-page">
    <!-- ====== 统一页面头部 ====== -->
    <header class="page-header">
      <h1>📊 策略回测</h1>
      <p>运行回测模拟交易，查看收益曲线与归因分析</p>
    </header>

    <!-- ====== 工具栏卡片 ====== -->
    <section class="toolbar-card">
      <div class="toolbar-row">
        <button class="back-btn" @click="$emit('goBack')" title="返回">‹ 返回</button>
        <input v-model="strategyName" class="strategy-name" placeholder="策略名称" />
        <span class="toolbar-sep">|</span>
        <label>日期：</label>
        <input type="date" v-model="startDate" class="date-input" />
        <span>至</span>
        <input type="date" v-model="endDate" class="date-input" />
        <span class="toolbar-sep">|</span>
        <label>本金：</label>
        <input type="number" v-model.number="initialCapital" class="capital-input" min="1000" step="1000" />
        <span class="toolbar-right">
          <span class="status-badge" :class="runStatus">
            <span class="status-dot"></span>{{ statusText }}
          </span>
          <span v-if="elapsedTime">{{ elapsedTime }}</span>
        </span>
      </div>
      <div class="toolbar-actions">
        <button class="btn-primary" :disabled="runStatus === 'running'" @click="runBacktest">
          {{ runStatus === 'running' ? '运行中...' : '📊 模拟交易' }}
        </button>
        <button class="btn-outline" @click="activeTab = 'analysis'">🔬 归因分析</button>
        <button class="btn-ghost" @click="shareToCommunity">📤 分享到社区</button>
        <button class="btn-ghost" @click="exportReport">导出 ▾</button>
      </div>
    </section>

    <!-- ====== Tab 导航 ====== -->
    <nav class="tab-nav">
      <button
        v-for="t in tabs" :key="t.key"
        :class="['tab-btn', { active: activeTab === t.key }]"
        @click="activeTab = t.key"
      >
        <span class="tab-icon">{{ t.icon }}</span>{{ t.label }}
      </button>
    </nav>

    <!-- ====== Tab 内容区 ====== -->
    <main class="tab-content">

      <!-- ========== 收益概述 ========== -->
      <div v-if="activeTab === 'overview'" class="tab-overview">
        <!-- 收益指标网格 -->
        <section class="stats-section">
          <h3 class="section-title">收益概述</h3>
          <div class="stats-grid">
            <div v-for="s in statsData" :key="s.key" class="stat-item" :class="s.cls">
              <span class="stat-label">{{ s.label }}</span>
              <span class="stat-value" :style="{ color: s.color }">{{ s.value }}</span>
            </div>
          </div>
        </section>

        <!-- 累计收益曲线图 -->
        <section class="chart-section">
          <div class="chart-header">
            <span class="chart-title">累计收益曲线</span>
            <div class="chart-toolbar">
              <span class="toolbar-label">缩放：</span>
              <button
                v-for="z in zoomOptions" :key="z"
                :class="['zoom-btn', { active: currentZoom === z }]"
                @click="currentZoom = z"
              >{{ z === '1M' ? '1月' : z === '3M' ? '3月' : z === '1Y' ? '1年' : '全部' }}</button>
            </div>
          </div>
          <div class="chart-body">
            <svg :viewBox="`0 0 ${chartW} ${chartH}`" preserveAspectRatio="none">
              <!-- 网格线 -->
              <line v-for="(y, i) in gridYs" :key="'g'+i" x1="yAxisW" :y1="y" x2="chartW - marginR" :y2="y" class="grid-line" />
              <!-- Y轴标签 -->
              <text v-for="(yt, i) in yTickLabels" :key="'yt'+i" :x="yAxisW - 6" :y="i * yStep + paddingTop + 4" text-anchor="end" class="axis-label">{{ yt }}</text>
              <!-- X轴标签 -->
              <text v-for="(d, i) in xLabels" :key="'xt'+i" :x="yAxisW + i * xStep" :y="chartH - paddingBottom + 4" text-anchor="middle" class="axis-label">{{ d }}</text>
              <!-- 零线 -->
              <line :x1="yAxisW" :y1="zeroY" :x2="chartW - marginR" :y2="zeroY" class="zero-line" />
              <!-- 基准线（红色） -->
              <polyline :points="benchPoints" fill="none" class="line-bench" />
              <!-- 策略线（蓝色） -->
              <polyline :points="stratPoints" fill="none" class="line-strategy" />
              <!-- 超额收益填充区域 -->
              <polygon :points="excessAreaPts" fill="url(#excessGrad)" opacity="0.15" />
              <!-- 数据点标记 -->
              <circle v-for="(d, i) in mockData" :key="'p'+i" :cx="xPos(i)" :cy="yPos(d.strategy)" r="3" class="dot-strategy" />
            </svg>
            <defs>
              <linearGradient id="excessGrad" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stop-color="#1677ff" stop-opacity="0.6"/>
                <stop offset="100%" stop-color="#1677ff" stop-opacity="0"/>
              </linearGradient>
            </defs>
          </div>
          <div class="chart-legend">
            <label v-for="lg in legendItems" :key="lg.key"><span class="leg-dot" :style="{ background: lg.color }"></span>{{ lg.label }}</label>
          </div>
        </section>

        <!-- 回撤柱状图 -->
        <section class="chart-section">
          <div class="chart-header">
            <span class="chart-title">最大回撤</span>
          </div>
          <div class="chart-body compact">
            <svg :viewBox="`0 0 ${chartW} ${ddChartH}`" preserveAspectRatio="none">
              <line :x1="yAxisW" :y1="ddZeroY" :x2="chartW - marginR" :y2="ddZeroY" class="zero-line" />
              <g class="x-axis">
                <text v-for="(d, i) in xLabels" :key="'dx'+i" :x="yAxisW + i * xStep" :y="ddChartH - paddingBottom + 4" text-anchor="middle" class="axis-label">{{ d }}</text>
              </g>
              <rect v-for="(d, i) in mockData" :key="'dd'+i"
                :x="yAxisW + i * xStep - barW/2"
                :y="d.excess >= 0 ? ddZeroY - ddBarH(d.excess) : ddZeroY"
                :width="barW"
                :height="Math.abs(ddBarH(d.excess))"
                :fill="d.excess >= 0 ? '#1677ff' : '#cf1322'"
                :rx="1.5"
                class="dd-bar" />
            </svg>
          </div>
        </section>
      </div>

      <!-- ========== 交易详情 ========== -->
      <div v-else-if="activeTab === 'trades'" class="tab-panel">
        <section class="panel-section">
          <h3 class="section-title">📋 交易详情</h3>
          <table class="data-table">
            <thead><tr><th>日期</th><th>标的</th><th>方向</th><th>价格</th><th>数量</th><th>盈亏</th></tr></thead>
            <tbody>
              <tr v-for="(t, i) in tradeData" :key="i" :class="{ buy: t.dir === '买', sell: t.dir === '卖' }">
                <td>{{ t.date }}</td><td>{{ t.stock }}</td><td>{{ t.dir }}</td><td>{{ t.price }}</td><td>{{ t.qty }}</td><td :class="Number(t.pnl) >= 0 ? 'pos' : 'neg'">{{ t.pnl }}</td></tr>
            </tbody>
          </table>
          <div v-if="tradeData.length === 0" class="empty-hint">暂无交易数据，请先运行回测</div>
        </section>
      </div>

      <!-- ========== 每日持仓 ========== -->
      <div v-else-if="activeTab === 'holdings'" class="tab-panel">
        <section class="panel-section">
          <h3 class="section-title">📈 每日持仓 & 收益</h3>
          <div class="placeholder-content">
            <div class="ph-card"><span class="ph-icon">📊</span><p>每日持仓明细和累计收益曲线</p><p class="ph-sub">运行回测后将展示每日持仓变化及盈亏分布</p></div>
          </div>
        </section>
      </div>

      <!-- ========== 日志输出 ========== -->
      <div v-else-if="activeTab === 'logs'" class="tab-panel">
        <section class="panel-section">
          <h3 class="section-title">📝 日志输出</h3>
          <div class="log-console">
            <div v-for="(log, i) in backtestLogs" :key="i" :class="['log-line', 'log-' + log.level]">
              <span class="log-time">{{ log.time }}</span>
              <span class="log-tag">[{{ log.level.toUpperCase() }}]</span>
              <span class="log-msg">{{ log.message }}</span>
            </div>
            <div v-if="backtestLogs.length === 0" class="log-empty">暂无日志输出</div>
          </div>
        </section>
      </div>

      <!-- ========== 性能分析 ========== -->
      <div v-else-if="activeTab === 'analysis'" class="tab-panel">
        <section class="panel-section">
          <h3 class="section-title">⚡ 性能分析</h3>
          <div class="analysis-grid">
            <div class="analysis-card"><h4>📈 收益分布</h4><div class="mini-ph">直方图区域</div></div>
            <div class="analysis-card"><h4>🎯 月度收益热力图</h4><div class="mini-ph">热力图区域</div></div>
            <div class="analysis-card"><h4>📊 回撤分析</h4><div class="mini-ph">回撤曲线</div></div>
            <div class="analysis-card"><h4>🔄 滚动夏普</h4><div class="mini-ph">滚动指标</div></div>
          </div>
        </section>
      </div>

      <!-- ========== 策略代码 ========== -->
      <div v-else-if="activeTab === 'code'" class="tab-panel">
        <section class="panel-section">
          <h3 class="section-title">💻 策略代码</h3>
          <pre class="code-block"><code>{{ strategyCodePreview }}</code></pre>
        </section>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'

defineEmits<{ goBack: [] }>()

// ========== 工具栏状态 ==========
const strategyName = ref('小市值策略')
const startDate = ref('2026-04-01')
const endDate = ref('2026-04-30')
const initialCapital = ref(100000)
const runStatus = ref<'idle' | 'running' | 'done'>('done')
const elapsedTime = ref('00分03秒')

const statusText = computed(() => {
  if (runStatus.value === 'running') return '回测运行中...'
  if (runStatus.value === 'done') return '已完成'
  return '未运行'
})

// ========== Tab 导航 ==========
type TabKey = 'overview' | 'trades' | 'holdings' | 'logs' | 'analysis' | 'code'
const activeTab = ref<TabKey>('overview')

interface TabItem { key: TabKey; label: string; icon: string }
const tabs: TabItem[] = [
  { key: 'overview', label: '收益概述', icon: '⊕' },
  { key: 'trades', label: '交易详情', icon: '📋' },
  { key: 'holdings', label: '每日持仓', icon: '📈' },
  { key: 'logs', label: '日志输出', icon: '📝' },
  { key: 'analysis', label: '性能分析', icon: '⚡' },
  { key: 'code', label: '策略代码', icon: '💻' },
]

// ========== 操作方法 ==========
function runBacktest() {
  if (runStatus.value === 'running') return
  runStatus.value = 'running'
  setTimeout(() => { runStatus.value = 'done'; elapsedTime.value = '00分03秒' }, 2000)
}
function shareToCommunity() { alert('分享功能开发中...') }
function exportReport() { alert('导出功能开发中...') }

// ========== 收益指标数据 ==========
interface StatItem { key: string; label: string; value: string; color?: string; cls?: string }
const statsData = computed<StatItem[]>(() => [
  { key: 'total_return', label: '策略收益', value: '-0.84%', color: '#cf1322', cls: 'neg' },
  { key: 'annual_return', label: '策略年化收益', value: '-9.51%', color: '#cf1322', cls: 'neg' },
  { key: 'excess_return', label: '超额收益', value: '-8.21%', color: '#cf1322', cls: 'neg' },
  { key: 'benchmark_return', label: '基准收益', value: '8.03%', color: '#16a34a', cls: 'pos' },
  { key: 'alpha', label: '阿尔法', value: '-0.609' },
  { key: 'beta', label: '贝塔', value: '0.323' },
  { key: 'sharpe_ratio', label: '夏普比率', value: '-0.635' },
  { key: 'win_rate', label: '胜率', value: '0.400' },
  { key: 'profit_loss', label: '盈亏比', value: '0.760' },
  { key: 'max_drawdown', label: '最大回撤', value: '3.76%' },
  { key: 'sortino_ratio', label: '索提诺比率', value: '-1.064' },
  { key: 'avg_daily_excess', label: '日均超额收益', value: '-0.40%' },
  { key: 'max_excess_dd', label: '超额收益最大回撤', value: '10.13%' },
  { key: 'excess_sharpe', label: '超额收益夏普比率', value: '-2.930' },
  { key: 'daily_win_rate', label: '日胜率', value: '0.429' },
  { key: 'win_count', label: '盈利次数', value: '4' },
  { key: 'loss_count', label: '亏损次数', value: '6' },
  { key: 'info_ratio', label: '信息比率', value: '-6.842' },
  { key: 'strat_vol', label: '策略波动率', value: '0.213' },
  { key: 'bench_vol', label: '基准波动率', value: '0.165' },
])

// ========== 图表配置 & Mock 数据 ==========
const chartW = 1100
const chartH = 220
const ddChartH = 120
const yAxisW = 50
const marginR = 20
const paddingTop = 18
const paddingBottom = 24

const currentZoom = ref('1M')
const zoomOptions: string[] = ['1M', '3M', '1Y', 'all']

const legendItems = [
  { key: 'strategy', label: '策略收益', color: '#1677ff' },
  { key: 'bench', label: '沪深300', color: '#cf1322' },
  { key: 'excess', label: '超额收益', color: '#faad14' },
]

// Mock 数据生成 (30天)
const generateMockData = () => {
  const data: { date: string; strategy: number; bench: number; excess: number }[] = []
  let stratVal = 0, benchVal = 0
  for (let i = 0; i < 30; i++) {
    const d = new Date('2026-04-01'); d.setDate(d.getDate() + i)
    const dateStr = `${d.getMonth()+1}-${String(d.getDate()).padStart(2,'0')}`
    const sChg = (Math.random() - 0.48) * 3
    const bChg = (Math.random() - 0.45) * 2.5
    stratVal += sChg
    benchVal += bChg
    data.push({ date: dateStr, strategy: stratVal, bench: benchVal, excess: stratVal - benchVal })
  }
  return data
}

const mockData = generateMockData()

// Y轴计算
const yMin = computed(() => Math.min(...mockData.map(d => Math.min(d.strategy, d.bench, d.excess))) - 2)
const yMax = computed(() => Math.max(...mockData.map(d => Math.max(d.strategy, d.bench, d.excess))) + 2)
const yRange = computed(() => yMax.value - yMin.value || 1)

function yPos(val: number): number {
  return paddingTop + ((yMax.value - val) / yRange.value) * (chartH - paddingTop - paddingBottom)
}
function xPos(i: number): number { return yAxisW + i * xStep.value }
const zeroY = computed(() => yPos(0))

const yStep = computed(() => (chartH - paddingTop - paddingBottom) / 5)
const yTickLabels = computed(() => Array.from({ length: 6 }, (_, i) => (yMax.value - i * yRange.value / 5).toFixed(1) + '%'))
const gridYs = computed(() => Array.from({ length: 5 }, (_, i) => paddingTop + (i + 1) * yStep.value))

// X轴
const xLabels = mockData.filter((_, i) => i % 3 === 0 || i === mockData.length - 1).map(d => d.date)
const xStep = computed(() => (chartW - yAxisW - marginR) / Math.max(mockData.length - 1, 1))

// 折线 points
const benchPoints = computed(() => mockData.map((_, i) => `${xPos(i)},${yPos(mockData[i].bench)}`).join(' '))
const stratPoints = computed(() => mockData.map((_, i) => `${xPos(i)},${yPos(mockData[i].strategy)}`).join(' '))
const excessAreaPts = computed(() =>
  mockData.map((d, i) => `${xPos(i)},${yPos(d.excess)}`).join(' ') +
  ` ${xPos(mockData.length - 1)},${zeroY.value}` +
  ` ${xPos(0)},${zeroY.value}`
)

// 回撤柱状图
const barW = 8
const ddMaxAbs = 12
const ddZeroY = ddChartH / 2
function ddBarH(v: number): number { return (Math.abs(v) / ddMaxAbs) * (ddChartH / 2 - 10) }

// ========== 交易数据（Mock）==========
const tradeData = ref([
  { date: '2026-04-02', stock: '000001.SZ', dir: '买', price: '11.23', qty: '900', pnl: '+320' },
  { date: '2026-04-05', stock: '600519.SH', dir: '买', price: '1680.00', qty: '50', pnl: '+156' },
  { date: '2026-04-08', stock: '300750.SZ', dir: '卖', price: '215.60', qty: '200', pnl: '-89' },
  { date: '2026-04-12', stock: '000001.SZ', dir: '卖', price: '11.45', qty: '900', pnl: '+198' },
  { date: '2026-04-16', stock: '002594.SZ', dir: '买', price: '22.80', qty: '400', pnl: '-45' },
])

// ========== 日志（Mock）==========
const backtestLogs = ref([
  { time: '14:32:01', level: 'info', message: '回测引擎启动，加载策略信号...' },
  { time: '14:32:02', level: 'info', message: '日期范围：2026-04-01 ~ 2026-04-30，共22个交易日' },
  { time: '14:32:02', level: 'info', message: '初始资金：¥100,000.00' },
  { time: '14:32:05', level: 'warn', message: '2026-04-08 触发止损，卖出 300750.SZ @ 215.60' },
  { time: '14:32:07', level: 'info', message: '回测完成！总收益率：-0.84%，最大回撤：3.76%' },
])

// ========== 策略代码预览 ==========
const strategyCodePreview = `# 小市值选股策略
# 逻辑：选择市值最小的前 N 只股票，等权持有

def initialize(context):
    context.n_stocks = 10
    context.rebalance_freq = "monthly"

def handle_data(context, data):
    if is_rebalance_day():
        # 按市值排序，取最小的 N 只
        stocks = sort_by_market_cap(ascending=True)
        target = stocks[:context.n_stocks]
        # 等权分配资金
        weight = 1.0 / len(target)
        for stock in target:
            order_target_percent(stock, weight)`
</script>

<style scoped>
.backtest-page {}

/* ===== 工具栏 ===== */
.toolbar-card {
  background: #fff; border-radius: 12px;
  box-shadow: 0 1px 4px rgba(0,0,0,.06); padding: 14px 18px;
  margin-bottom: 14px;
}
.toolbar-row {
  display: flex; align-items: center; gap: 8px; flex-wrap: wrap;
  font-size: 13px; color: #555; flex-wrap: wrap;
}
.back-btn {
  border: 1px solid #d9d9d9; border-radius: 6px; background: #fff;
  font-size: 13px; cursor: pointer; padding: 4px 10px; transition: .15s;
  white-space: nowrap;
}
.back-btn:hover { background: #f5f5f5; }
.strategy-name {
  border: none; border-bottom: 1.5px solid transparent;
  font-size: 15px; font-weight: 700; outline: none;
  width: 160px; background: transparent; color: #1a1a2e;
  transition: border-color .15s;
}
.strategy-name:focus { border-bottom-color: #1677ff; }
.date-input, .capital-input {
  border: 1px solid #d9d9d9; border-radius: 6px; padding: 4px 8px;
  font-size: 13px; outline: none; transition: border-color .15s;
}
.date-input:focus, .capital-input:focus { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.08); }
.capital-input { width: 110px; }
.toolbar-sep { color: #ddd; }
.toolbar-right { display: inline-flex; gap: 6px; align-items: center; }
.status-badge {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 3px 10px; border-radius: 10px; font-size: 12px; font-weight: 600;
}
.status-badge.done { background: #f6ffed; color: #389e0d; }
.status-badge.running { background: #e6f4ff; color: #1677ff; }
.status-badge.idle { background: #f5f5f5; color: #999; }
.status-dot { width: 7px; height: 7px; border-radius: 50%; }
.status-badge.done .status-dot { background: #52c41a; }
.status-badge.running .status-dot { animation: pulse 1s infinite; background: #1677ff; }
@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: .3; } }

.toolbar-actions {
  display: flex; gap: 8px; margin-top: 10px;
  padding-top: 10px; border-top: 1px solid #f0f0f0;
}

/* 按钮复用 StrategyList 风格 */
.btn-primary {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 7px 18px; font-size: 13px; font-weight: 600;
  color: #fff; background: #1677ff; border: 1px solid #1677ff;
  border-radius: 5px; cursor: pointer; transition: .15s; white-space: nowrap;
}
.btn-primary:hover:not(:disabled) { background: #0958d9; }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }
.btn-outline {
  padding: 7px 14px; font-size: 13px; font-weight: 500;
  color: #555; background: #fff; border: 1px solid #d9d9d9;
  border-radius: 5px; cursor: pointer; transition: .15s; white-space: nowrap;
}
.btn-outline:hover { border-color: #1677ff; color: #1677ff; }
.btn-ghost {
  padding: 7px 12px; font-size: 13px; background: transparent; border: none;
  color: #666; cursor: pointer; border-radius: 5px; transition: .15s;
}
.btn-ghost:hover { background: #f5f5f5; color: #333; }

/* ===== Tab 导航 ===== */
.tab-nav {
  display: flex; gap: 2px; margin-bottom: 16px;
  border-bottom: 1px solid #e8e8e8; padding-bottom: 0;
}
.tab-btn {
  display: flex; align-items: center; gap: 4px;
  padding: 8px 16px; font-size: 13px; font-weight: 500;
  color: #666; background: transparent; border: none;
  cursor: pointer; position: relative; transition: all .15s;
  white-space: nowrap;
}
.tab-btn:hover { color: #333; }
.tab-btn.active {
  color: #1677ff; font-weight: 700;
}
.tab-btn.active::after {
  content: ''; position: absolute; bottom: -1px; left: 0; right: 0;
  height: 2px; background: #1677ff; border-radius: 1px 1px 0 0;
}
.tab-icon { font-size: 14px; }

/* ===== 内容区 ===== */
.tab-content { min-height: 400px; }
.tab-panel { animation: fade-in .15s ease-out; }
@keyframes fade-in { from { opacity: 0; } to { opacity: 1; } }

/* ===== 收益指标网格 ===== */
.stats-section { margin-bottom: 20px; }
.section-title {
  font-size: 16px; font-weight: 700; color: #333;
  margin-bottom: 12px; padding-bottom: 8px; border-bottom: 1px solid #f0f0f0;
}
.stats-grid {
  display: grid; grid-template-columns: repeat(3, 1fr); gap: 1px;
  background: #f0f0f0; border: 1px solid #e8e8e8; border-radius: 10px; overflow: hidden;
}
.stat-item {
  background: #fff; padding: 12px 14px; display: flex; justify-content: space-between; align-items: center;
}
.stat-label { font-size: 12.5px; color: #888; }
.stat-value { font-size: 15px; font-weight: 700; font-family: 'SF Mono', Monaco, monospace; }
.stat-item.neg .stat-value { color: #cf1322; }
.stat-item.pos .stat-value { color: #16a34a; }

/* ===== 图表区域 ===== */
.chart-section {
  background: #fff; border-radius: 12px;
  box-shadow: 0 1px 4px rgba(0,0,0,.06); padding: 16px 18px;
  margin-bottom: 20px;
}
.chart-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.chart-title { font-size: 14px; font-weight: 600; color: #333; }
.chart-toolbar { display: flex; align-items: center; gap: 4px; }
.toolbar-label { font-size: 12px; color: #888; }
.zoom-btn {
  padding: 3px 10px; font-size: 12px; border: 1px solid #d9d9d9;
  border-radius: 4px; background: #fff; cursor: pointer; transition: .15s; color: #666;
}
.zoom-btn.active { background: #1677ff; color: #fff; border-color: #1677ff; }
.zoom-btn:not(.active):hover { border-color: #1677ff; color: #1677ff; }

.chart-body { overflow-x: auto; }
.chart-body.compact svg { height: auto; aspect-ratio: 1100/120; }
.chart-body svg { display: block; width: 100%; max-width: 100%; height: auto; }

.grid-line { stroke: #f5f5f5; stroke-width: 1; }
.zero-line { stroke: #e0e0e0; stroke-width: 1; stroke-dasharray: 4 3; }
.axis-label { fill: #bbb; font-size: 11px; }

.line-strategy { stroke: #1677ff; stroke-width: 2; stroke-linejoin: round; }
.line-bench { stroke: #cf1322; stroke-width: 1.5; stroke-dasharray: 4 3; }
.dot-strategy { fill: #1677ff; }
.dd-bar { transition: opacity .12s; }
.dd-bar:hover { opacity: .7; }

.chart-legend {
  display: flex; gap: 16px; margin-top: 8px; justify-content: center;
}
.chart-legend label { font-size: 12px; color: #666; display: flex; align-items: center; gap: 4px; cursor: pointer; }
.leg-dot { width: 10px; height: 10px; border-radius: 2px; display: inline-block; }

/* ===== 面板内容 ===== */
.panel-section { background: #fff; border-radius: 12px; box-shadow: 0 1px 4px rgba(0,0,0,.06); padding: 20px 22px; }
.empty-hint { text-align: center; color: #bbb; padding: 30px 0; font-size: 14px; }

/* 交易表格 */
.data-table {
  width: 100%; border-collapse: collapse; font-size: 13px;
}
.data-table th {
  background: #fafafa; padding: 8px 12px; text-align: left;
  font-weight: 600; color: #666; border-bottom: 1px solid #eee;
}
.data-table td { padding: 8px 12px; border-bottom: 1px solid #f5f5f5; color: #333; }
.data-table tr.buy td:nth-child(3) { color: #cf1322; }
.data-table tr.sell td:nth-child(3) { color: #16a34a; }
.data-table .pos { color: #cf1322; font-weight: 600; }
.data-table .neg { color: #16a34a; font-weight: 600; }

/* 日志 */
.log-console {
  background: #1a1a2e; border-radius: 8px; padding: 12px 14px;
  max-height: 360px; overflow-y: auto; font-family: 'SF Mono', Monaco, monospace; font-size: 12.5px;
}
.log-line { line-height: 1.8; color: #ccc; }
.log-time { color: #666; margin-right: 8px; }
.log-tag { color: #faad14; margin-right: 8px; font-weight: 600; font-size: 11px; }
.log-msg { color: #eee; }
.log-log-info { /* default */ }
.log-log-warn .log-msg { color: #faad14; }
.log-log-error .log-msg { color: #cf1322; }
.log-empty { text-align: center; color: #666; padding: 40px 0; }

/* 性能分析 */
.analysis-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 14px; }
.analysis-card {
  background: #fafafa; border: 1px solid #eee; border-radius: 10px; padding: 18px;
}
.analysis-card h4 { font-size: 14px; font-weight: 600; color: #444; margin-bottom: 10px; }
.mini-ph {
  height: 120px; background: #f5f5f5; border-radius: 6px;
  display: flex; align-items: center; justify-content: center;
  color: #bbb; font-size: 13px;
}

/* 代码块 */
.code-block {
  background: #1a1a2e; color: #e0e0e0; border-radius: 8px; padding: 18px;
  overflow-x: auto; font-family: 'SF Mono', Monaco, monospace; font-size: 13px; line-height: 1.65;
  white-space: pre;
}
</style>
