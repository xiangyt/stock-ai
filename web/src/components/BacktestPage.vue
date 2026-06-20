<template>
  <div class="backtest-page">
    <!-- ====== 页面头部 ====== -->
    <header class="page-header">
      <h1>📊 策略回测</h1>
    </header>

    <!-- ====== 工具栏 ====== -->
    <section class="toolbar-card">
      <div class="toolbar-row">
        <button v-if="props.defaultStrategyId" class="back-btn" @click="onGoBack">‹ 返回编辑</button>
        <!-- 策略选择 -->
        <div class="strategy-select-wrap" ref="strategySelectRef">
          <div
            class="strategy-select-trigger"
            @click.stop="toggleStrategyDropdown"
            :class="{ open: showStrategyDropdown }"
          >
            <span v-if="selectedStrategy" class="sel-text">{{ selectedStrategy.name }}</span>
            <span v-else class="sel-placeholder">选择策略</span>
            <span class="sel-arrow">▾</span>
          </div>
          <teleport to="body">
            <div v-if="showStrategyDropdown" class="sd-overlay" @click.self="showStrategyDropdown = false">
              <div class="sd-dropdown" :style="dropdownStyle">
                <input
                  type="text" class="sd-search-input" placeholder="搜索策略..."
                  v-model.trim="strategySearchKeyword" ref="sdSearchInputRef"
                  @click.stop @keydown.escape="showStrategyDropdown = false"
                  @keydown.down.prevent="moveSelDown" @keydown.up.prevent="moveSelUp"
                  @keydown.enter="selectHighlighted"
                />
                <div class="sd-list" ref="sdListRef">
                  <template v-if="filteredStrategies.length > 0">
                    <div
                      v-for="(s, idx) in filteredStrategies" :key="s.id"
                      :class="['sd-item', { active: selectedStrategy?.id === s.id, highlighted: highlightIdx === idx }]"
                      @mousedown.prevent="selectStrategy(s)"
                    >
                      <span class="sd-name">{{ s.name }}</span>
                      <span class="sd-meta">{{ s.signals?.length || 0 }} 个信号</span>
                    </div>
                  </template>
                  <div v-else class="sd-empty">无匹配策略</div>
                </div>
              </div>
            </div>
          </teleport>
        </div>

        <span class="toolbar-sep">|</span>
        <label>日期：</label>
        <input type="date" v-model="startDate" class="date-picker" :max="endDate || undefined" />
        <span>至</span>
        <input type="date" v-model="endDate" class="date-picker" :max="formatDate(new Date())" />

        <span class="toolbar-sep">|</span>
        <label>本金：</label>
        <input type="number" v-model.number="initialCapital" class="capital-input" min="1000" step="1000" />

        <span class="toolbar-right">
          <span class="status-badge" :class="runStatus">
            <span class="status-dot"></span>{{ statusText }}
          </span>
          <span v-if="elapsedTime" class="elapsed">{{ elapsedTime }}</span>
          <span v-if="runStatus === 'failed' && errorMessage" class="error-hint" :title="errorMessage">⚠️ {{ errorMessage.slice(0, 30) }}{{ errorMessage.length > 30 ? '...' : '' }}</span>
        </span>
      </div>

      <div class="toolbar-actions">
        <button class="btn-outline" @click="showRulesPanel = !showRulesPanel">
          {{ showRulesPanel ? '🔽 隐藏规则' : '📐 规则配置' }}
        </button>
        <button class="btn-primary" :disabled="isRunning || !selectedStrategy" @click="runBacktest">
          {{ isRunning ? '运行中...' : '📊 模拟交易' }}
        </button>

        <div class="toolbar-right">
          <!-- 回测历史选择 -->
          <div class="history-select-wrap" ref="historySelectRef">
            <button
              class="btn-outline history-btn"
              @click.stop="toggleHistoryDropdown"
              :class="{ active: showHistoryDropdown }"
            >
              <span class="history-prefix">📜 回测历史</span>
              <span v-if="currentRunId && historyRunLabel" class="history-text">{{ historyRunLabel }}</span>
              <span v-else class="history-placeholder">未选择</span>
              <span class="history-arrow" :class="{ open: showHistoryDropdown }">▾</span>
            </button>
            <teleport to="body">
              <div v-if="showHistoryDropdown" class="sd-overlay" @click.self="showHistoryDropdown = false">
                <div class="sd-dropdown history-dropdown" :style="historyDropdownStyle">
                  <div class="sd-list" ref="sdHistoryListRef">
                    <template v-if="historyRuns.length > 0">
                      <div
                        v-for="r in historyRuns" :key="r.id"
                        :class="['sd-item', { active: currentRunId === r.id }]"
                        @mousedown.prevent="loadHistoryRun(r)"
                      >
                        <span class="sd-name">{{ formatHistoryLabel(r) }}</span>
                        <span class="sd-meta" :class="'history-status-' + r.status">{{ statusLabel(r.status) }}</span>
                      </div>
                    </template>
                    <div v-else class="sd-empty">暂无回测记录</div>
                  </div>
                </div>
              </div>
            </teleport>
          </div>
        </div>
      </div>

      <!-- 规则配置面板 -->
      <div v-if="showRulesPanel" class="rules-panel">
        <div class="rules-section">
          <h4 class="rules-title">📐 卖出规则</h4>
          <div class="rules-list">
            <div v-for="(rule, ri) in exitRulesOverride.rules" :key="rule.type" class="rule-row">
              <label class="rule-check">
                <input type="checkbox" v-model="rule.enabled" />
                <span class="rule-label">{{ ruleName(rule.type) }}</span>
              </label>
              <template v-if="rule.enabled">
                <!-- stop_loss -->
                <template v-if="rule.type === 'stop_loss'">
                  <input type="number" v-model.number="rule.params.threshold_pct" class="rule-input-sm" step="1" />
                  <span class="rule-unit">%</span>
                </template>
                <!-- take_profit -->
                <template v-else-if="rule.type === 'take_profit'">
                  <input type="number" v-model.number="rule.params.threshold_pct" class="rule-input-sm" step="1" />
                  <span class="rule-unit">%</span>
                </template>
                <!-- time_exit -->
                <template v-else-if="rule.type === 'time_exit'">
                  <input type="number" v-model.number="rule.params.hold_days" class="rule-input-sm" step="1" min="1" />
                  <span class="rule-unit">天</span>
                </template>
                <!-- trailing_stop -->
                <template v-else-if="rule.type === 'trailing_stop'">
                  <span class="rule-param-label">激活</span>
                  <input type="number" v-model.number="rule.params.activation_pct" class="rule-input-sm" step="1" />
                  <span class="rule-unit">%</span>
                  <span class="rule-param-label">回撤</span>
                  <input type="number" v-model.number="rule.params.trail_pct" class="rule-input-sm" step="0.5" />
                  <span class="rule-unit">%</span>
                </template>
                <!-- segment_profit -->
                <template v-else-if="rule.type === 'segment_profit'">
                  <span class="rule-param-label">档位</span>
                  <div v-for="(lv, li) in rule.params.levels" :key="li" class="segment-level">
                    <span>涨</span><input type="number" v-model.number="lv.threshold_pct" class="rule-input-xs" step="1" /><span>%卖</span>
                    <input type="number" v-model.number="lv.sell_ratio" class="rule-input-xs" step="0.1" min="0.1" max="1" /><span>成</span>
                    <button v-if="rule.params.levels.length > 1" class="btn-level-del" @click="rule.params.levels.splice(li,1)">✕</button>
                  </div>
                  <button class="btn-level-add" @click="rule.params.levels.push({ threshold_pct: 30, sell_ratio: 0.5 })">+ 添加档位</button>
                </template>
                <!-- signal_exit -->
                <template v-else-if="rule.type === 'signal_exit'">
                  <input type="text" v-model="rule.params.signal_id" class="rule-input-sm" placeholder="信号ID" style="width:80px" />
                  <select v-model="rule.params.operator" class="rule-select">
                    <option value="gt">大于</option>
                    <option value="lt">小于</option>
                    <option value="cross_below">下穿</option>
                    <option value="cross_above">上穿</option>
                  </select>
                </template>
              </template>
            </div>
          </div>
          <div class="rules-footer">
            <span>滑点</span>
            <input type="number" v-model.number="exitRulesOverride.slippage_pct" class="rule-input-sm" step="0.1" min="0" max="5" />
            <span class="rule-unit">%</span>
          </div>
        </div>
        <div class="rules-section">
          <h4 class="rules-title">📦 仓位管理</h4>
          <div class="rules-grid">
            <label class="rule-item">
              <span>最大持仓</span>
              <input type="number" v-model.number="positionRulesOverride.max_positions" class="rule-input" step="1" min="1" max="50" />
              <span class="rule-unit">只</span>
            </label>
            <label class="rule-item">
              <span>单票上限</span>
              <input type="number" v-model.number="positionRulesOverride.max_single_pct" class="rule-input" step="5" min="0" max="100" />
              <span class="rule-unit">%</span>
            </label>
            <label class="rule-item">
              <span>分配方式</span>
              <select v-model="positionRulesOverride.allocation" class="rule-select">
                <option value="equal">等权分配</option>
                <option value="signal_weighted">信号加权</option>
                <option value="volatility_weighted">波动率加权</option>
                <option value="risk_parity">风险平价</option>
              </select>
            </label>
          </div>
        </div>
      </div>
    </section>

    <!-- ====== Tab 导航 ====== -->
    <nav class="tab-nav">
      <button v-for="t in tabs" :key="t.key" :class="['tab-btn', { active: activeTab === t.key }]" @click="activeTab = t.key">
        <span class="tab-icon">{{ t.icon }}</span>{{ t.label }}
      </button>
    </nav>

    <!-- ====== Tab 内容 ====== -->
    <main class="tab-content">

      <!-- ========== 收益概述 ========== -->
      <div v-if="activeTab === 'overview'" class="tab-panel">
        <!-- 空状态 -->
        <div v-if="runStatus === 'idle'" class="empty-state">
          <span class="empty-icon">📊</span>
          <p>选择策略并配置规则后，点击「模拟交易」开始回测</p>
        </div>

        <!-- 加载中 -->
        <div v-else-if="runStatus === 'pending' || runStatus === 'running'" class="empty-state">
          <span class="empty-icon">⏳</span>
          <p>{{ runStatus === 'pending' ? '任务排队中...' : `回测进行中 ${progressPct}%` }}</p>
          <div class="progress-bar"><div class="progress-fill" :style="{ width: progressPct + '%' }"></div></div>
        </div>

        <!-- 失败 -->
        <div v-else-if="runStatus === 'failed'" class="empty-state">
          <span class="empty-icon">❌</span>
          <p>回测失败：{{ errorMessage || '未知错误' }}</p>
        </div>

        <!-- 结果展示 -->
        <template v-else-if="statsData.length > 0">
          <section class="stats-section">
            <h3 class="section-title">收益概述</h3>
            <div class="stats-grid">
              <div v-for="s in statsData" :key="s.key" class="stat-item" :class="s.cls">
                <span class="stat-label">{{ s.label }}</span>
                <span class="stat-value" :style="{ color: s.color }">{{ s.value }}</span>
              </div>
            </div>
          </section>

          <section class="chart-section" v-if="snapshotData.length > 0">
            <div class="chart-header">
              <span class="chart-title">净值曲线</span>
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
                <!-- 策略净值线 -->
                <polyline :points="stratPoints" fill="none" class="line-strategy" />
                <!-- 数据点 -->
                <circle v-for="(d, i) in snapshotData" :key="'p'+i" :cx="xPos(i)" :cy="yPos(d.strategy)" r="2.5" class="dot-strategy" />
              </svg>
            </div>
          </section>
        </template>
      </div>

      <!-- ========== 交易详情 ========== -->
      <div v-else-if="activeTab === 'trades'" class="tab-panel">
        <div v-if="tradeData.length === 0" class="empty-state">
          <span class="empty-icon">📋</span>
          <p>暂无交易数据，请先运行回测</p>
        </div>
        <section v-else class="panel-section">
          <div class="panel-header">
            <h3 class="section-title">交易详情 (共 {{ tradeTotal }} 条)</h3>
            <div class="pager" v-if="tradeTotalPages > 1">
              <button :disabled="tradePage <= 1" @click="loadTradePage(tradePage - 1)">‹</button>
              <span>{{ tradePage }} / {{ tradeTotalPages }}</span>
              <button :disabled="tradePage >= tradeTotalPages" @click="loadTradePage(tradePage + 1)">›</button>
            </div>
          </div>
          <div style="overflow-x: auto;">
            <table class="data-table">
              <thead>
                <tr>
                  <th>日期</th><th>名称</th><th>代码</th><th>方向</th><th>价格</th><th>数量</th><th>金额</th>
                  <th>手续费</th><th>卖出原因</th><th>盈亏</th><th>盈亏%</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(t, i) in tradeData" :key="i" :class="{ buy: t.dir === '买入', sell: t.dir === '卖出' }">
                  <td>{{ t.date }}</td>
                  <td>{{ t.stockName }}</td>
                  <td class="code-col" @mouseenter="showKLine($event, t.stockCode)" @mouseleave="hideKLine">
                    <span class="code-hover">{{ t.stockCode }}</span>
                  </td>
                  <td>{{ t.dir }}</td>
                  <td>{{ t.price }}</td>
                  <td>{{ t.qty }}</td>
                  <td>{{ t.amount }}</td>
                  <td>{{ t.fee }}</td>
                  <td><span v-if="t.reason" :class="'reason-tag reason-' + t.reason">{{ reasonLabel(t.reason) }}</span><span v-else>—</span></td>
                  <td :class="t.pnlClass">{{ t.pnl }}</td>
                  <td :class="t.pnlClass">{{ t.pnlPct }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <!-- ========== 每日持仓 ========== -->
      <div v-else-if="activeTab === 'holdings'" class="tab-panel">
        <div v-if="snapshotData.length === 0" class="empty-state">
          <span class="empty-icon">📈</span>
          <p>暂无持仓数据，请先运行回测</p>
        </div>
        <section v-else class="panel-section">
          <h3 class="section-title">每日持仓快照 (共 {{ rawSnapshots.length }} 个交易日)</h3>
          <div style="max-height: 500px; overflow-y: auto;">
            <table class="data-table">
              <thead>
                <tr><th>日期</th><th>总权益</th><th>现金</th><th>市值</th><th>持仓数</th><th>日收益</th><th>累计收益</th></tr>
              </thead>
              <tbody>
                <tr v-for="s in rawSnapshots" :key="s.snap_date">
                  <td>{{ s.snap_date }}</td>
                  <td>¥{{ s.total_equity.toLocaleString('zh-CN', { maximumFractionDigits: 0 }) }}</td>
                  <td>¥{{ s.cash.toLocaleString('zh-CN', { maximumFractionDigits: 0 }) }}</td>
                  <td>¥{{ s.market_value.toLocaleString('zh-CN', { maximumFractionDigits: 0 }) }}</td>
                  <td>{{ s.position_count }}</td>
                  <td :class="(s.daily_return ?? 0) >= 0 ? 'pos' : 'neg'">{{ s.daily_return != null ? (s.daily_return >= 0 ? '+' : '') + s.daily_return.toFixed(2) + '%' : '—' }}</td>
                  <td :class="(s.cumulative_return ?? 0) >= 0 ? 'pos' : 'neg'">{{ s.cumulative_return != null ? (s.cumulative_return >= 0 ? '+' : '') + s.cumulative_return.toFixed(2) + '%' : '—' }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </div>

      <!-- ========== 运行日志 ========== -->
      <div v-else-if="activeTab === 'logs'" class="tab-panel">
        <div class="log-console">
          <div v-for="(log, i) in backtestLogs" :key="i" :class="['log-line', 'log-' + log.level]">
            <span class="log-time">{{ log.time }}</span>
            <span class="log-tag">[{{ log.level.toUpperCase() }}]</span>
            <span class="log-msg">{{ log.message }}</span>
          </div>
          <div v-if="backtestLogs.length === 0" class="log-empty">暂无日志输出</div>
        </div>
      </div>
    </main>

    <!-- K 线图悬浮弹窗 -->
    <KLineTooltip
      :visible="klineVisible"
      :stock-code="klineStockCode"
      :stock-name="klineStockName"
      :x="klineX"
      :y="klineY"
      @mouseenter="onKLineEnter"
      @mouseleave="hideKLine"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, reactive, nextTick, onUnmounted } from 'vue'
import * as strategyApi from '../api/strategies'
import KLineTooltip from './KLineTooltip.vue'

const props = defineProps<{ defaultStrategyId?: number | null }>()
const emit = defineEmits<{ goBack: []; goToEdit: [strategyId: number] }>()

// ========== 日期工具 ==========
function formatDate(d: Date): string {
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

// ========== 工具栏 ==========
const endDate = ref(formatDate(new Date()))
const startDate = ref(formatDate(new Date(new Date().setMonth(new Date().getMonth() - 6))))
const initialCapital = ref(100000)

const exitRulesOverride = ref<strategyApi.ExitRules>({
  rules: [
    { type: 'stop_loss', enabled: true, params: { threshold_pct: -8 }, priority: 1 },
    { type: 'take_profit', enabled: true, params: { threshold_pct: 20 }, priority: 2 },
    { type: 'time_exit', enabled: false, params: { hold_days: 60 }, priority: 3 },
    { type: 'trailing_stop', enabled: false, params: { trail_pct: 5, activation_pct: 10 }, priority: 2 },
    { type: 'segment_profit', enabled: false, params: { levels: [{ threshold_pct: 10, sell_ratio: 0.5 }, { threshold_pct: 20, sell_ratio: 0.5 }] }, priority: 2 },
    { type: 'signal_exit', enabled: false, params: { signal_id: '', operator: '', params: {} }, priority: 5 },
  ],
  slippage_pct: 0.3,
})
const positionRulesOverride = ref<strategyApi.PositionRules>({
  max_positions: 5, max_single_pct: 20, allocation: 'equal',
})
const showRulesPanel = ref(false)

// ========== 运行状态 ==========
type RunStatus = 'idle' | 'pending' | 'running' | 'done' | 'failed'
const runStatus = ref<RunStatus>('idle')
const currentRunId = ref<number | null>(null)
const progressPct = ref(0)
const elapsedTime = ref('')
const errorMessage = ref('')
let pollTimer: ReturnType<typeof setInterval> | null = null
let startTime = 0

const isRunning = computed(() => runStatus.value === 'pending' || runStatus.value === 'running')
const statusText = computed(() => {
  switch (runStatus.value) {
    case 'pending': return '排队中...'
    case 'running': return `运行中 ${progressPct.value}%`
    case 'done': return '已完成'
    case 'failed': return '失败'
    default: return '未运行'
  }
})

// ========== 策略下拉 ==========
interface StrategyItem { id: number; name: string; signals?: any[] }
const allStrategies = ref<StrategyItem[]>([])
const selectedStrategy = ref<StrategyItem | null>(null)
const showStrategyDropdown = ref(false)
const strategySearchKeyword = ref('')
const highlightIdx = ref(-1)
const strategySelectRef = ref<HTMLElement | null>(null)
const sdSearchInputRef = ref<HTMLInputElement | null>(null)
const sdListRef = ref<HTMLElement | null>(null)
const dropdownStyle = reactive<Record<string, string>>({})

// ========== 回测历史 ==========
const historyRuns = ref<strategyApi.BacktestRun[]>([])
const showHistoryDropdown = ref(false)
const historySelectRef = ref<HTMLElement | null>(null)
const sdHistoryListRef = ref<HTMLElement | null>(null)
const historyDropdownStyle = reactive<Record<string, string>>({})

const historyRunLabel = computed(() => {
  const run = historyRuns.value.find(r => r.id === currentRunId.value)
  return run ? formatHistoryLabel(run) : null
})

function formatHistoryLabel(r: strategyApi.BacktestRun): string {
  const d = r.created_at ? new Date(r.created_at) : null
  const ts = d ? `${String(d.getMonth()+1).padStart(2,'0')}-${String(d.getDate()).padStart(2,'0')} ${String(d.getHours()).padStart(2,'0')}:${String(d.getMinutes()).padStart(2,'0')}` : ''
  const ret = r.total_return != null ? ` ${r.total_return >= 0 ? '+' : ''}${r.total_return.toFixed(2)}%` : ''
  return `${ts}${ret}`
}

function statusLabel(status: string): string {
  const map: Record<string, string> = { done: '✅', running: '⏳', pending: '🕐', failed: '❌' }
  return map[status] || status
}

function toggleHistoryDropdown(e?: MouseEvent) {
  if (showHistoryDropdown.value) { showHistoryDropdown.value = false; return }
  const target = e?.currentTarget as HTMLElement
  if (target) {
    const rect = target.getBoundingClientRect()
    // 右对齐：dropdown 右边缘与按钮右边缘对齐
    historyDropdownStyle.right = `${window.innerWidth - rect.right}px`
    historyDropdownStyle.top = `${rect.bottom + 4}px`
    historyDropdownStyle.minWidth = `${Math.max(rect.width, 280)}px`
  }
  showHistoryDropdown.value = true
}

async function loadHistoryRuns() {
  if (!selectedStrategy.value) return
  try {
    const runs = await strategyApi.getBacktestRuns(selectedStrategy.value.id, 50)
    historyRuns.value = runs || []
  } catch (e) { console.error('加载回测历史失败:', e) }
}

async function loadHistoryRun(run: strategyApi.BacktestRun) {
  showHistoryDropdown.value = false
  currentRunId.value = run.id
  runStatus.value = (run.status as RunStatus) || 'done'
  errorMessage.value = run.error_message || ''
  progressPct.value = run.progress_pct || 0

  if (run.status === 'done') {
    await loadResults(run.id)
  } else if (run.status === 'failed') {
    addLog('error', `回测失败: ${run.error_message || '未知错误'}`)
  } else if (run.status === 'running' || run.status === 'pending') {
    startPolling()
  }
}

const filteredStrategies = computed(() => {
  const kw = strategySearchKeyword.value.toLowerCase()
  if (!kw) return allStrategies.value
  return allStrategies.value.filter(s => s.name.toLowerCase().includes(kw))
})

function toggleStrategyDropdown(e?: MouseEvent) {
  if (showStrategyDropdown.value) { showStrategyDropdown.value = false; return }
  strategySearchKeyword.value = ''; highlightIdx.value = -1
  const target = e?.currentTarget as HTMLElement
  if (target) {
    const rect = target.getBoundingClientRect()
    dropdownStyle.left = `${rect.left}px`; dropdownStyle.top = `${rect.bottom + 4}px`
    dropdownStyle.minWidth = `${Math.max(rect.width, 200)}px`
  }
  showStrategyDropdown.value = true
  nextTick(() => sdSearchInputRef.value?.focus())
}
function selectStrategy(s: StrategyItem) { selectedStrategy.value = s; showStrategyDropdown.value = false; loadHistoryRuns() }
function moveSelDown() { if (filteredStrategies.value.length > 0 && highlightIdx.value < filteredStrategies.value.length - 1) highlightIdx.value++ }
function moveSelUp() { if (highlightIdx.value > 0) highlightIdx.value-- }
function selectHighlighted() {
  if (highlightIdx.value >= 0 && highlightIdx.value < filteredStrategies.value.length) selectStrategy(filteredStrategies.value[highlightIdx.value])
}

async function loadAllStrategies() {
  try {
    const res = await strategyApi.fetchStrategies('', 1, 200)
    allStrategies.value = (res.list || []).map((s: any) => ({ id: s.id, name: s.name, signals: s.signals }))
    if (props.defaultStrategyId) {
      const found = allStrategies.value.find(s => s.id === props.defaultStrategyId)
      if (found) { selectedStrategy.value = found; loadHistoryRuns() }
    }
  } catch (e) { console.error('加载策略列表失败:', e) }
}

onMounted(loadAllStrategies)
onUnmounted(() => { if (pollTimer) clearInterval(pollTimer) })

// ========== Tab ==========
type TabKey = 'overview' | 'trades' | 'holdings' | 'logs'
const activeTab = ref<TabKey>('overview')
const tabs: { key: TabKey; label: string; icon: string }[] = [
  { key: 'overview', label: '收益概述', icon: '📊' },
  { key: 'trades', label: '交易详情', icon: '📋' },
  { key: 'holdings', label: '每日持仓', icon: '📈' },
  { key: 'logs', label: '运行日志', icon: '📝' },
]

// ========== 日志 ==========
interface LogEntry { time: string; level: string; message: string }
const backtestLogs = ref<LogEntry[]>([])
function addLog(level: string, message: string) {
  const now = new Date()
  const time = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}:${String(now.getSeconds()).padStart(2, '0')}`
  backtestLogs.value.push({ time, level, message })
}

// ========== 回测执行 ==========
async function runBacktest() {
  if (isRunning.value || !selectedStrategy.value) return
  runStatus.value = 'pending'; progressPct.value = 0; errorMessage.value = ''; currentRunId.value = null
  clearResults()

  try {
    const res = await strategyApi.initiateBacktest(selectedStrategy.value.id, {
      stock_pool: [], start_date: startDate.value, end_date: endDate.value,
      initial_capital: initialCapital.value,
      exit_rules_override: exitRulesOverride.value,
      position_rules_override: positionRulesOverride.value,
    })
    currentRunId.value = res.run_id
    runStatus.value = 'running'
    startTime = Date.now()
    addLog('info', `回测已发起 (run_id=${res.run_id})，日期 ${startDate.value} ~ ${endDate.value}`)
    startPolling()
  } catch (e: any) {
    runStatus.value = 'failed'
    errorMessage.value = e?.message || '发起回测失败'
    addLog('error', `发起失败: ${errorMessage.value}`)
  }
}

function startPolling() {
  if (pollTimer) clearInterval(pollTimer)
  pollTimer = setInterval(async () => {
    if (!currentRunId.value) return
    try {
      const status = await strategyApi.getBacktestRunStatus(currentRunId.value)
      progressPct.value = status.progress_pct
      const elapsed = Math.floor((Date.now() - startTime) / 1000)
      elapsedTime.value = `${String(Math.floor(elapsed / 60)).padStart(2, '0')}:${String(elapsed % 60).padStart(2, '0')}`

      if (status.status === 'done') {
        runStatus.value = 'done'
        clearInterval(pollTimer!); pollTimer = null
        addLog('info', '回测完成，加载结果...')
        await loadResults(currentRunId.value)
      } else if (status.status === 'failed') {
        runStatus.value = 'failed'
        clearInterval(pollTimer!); pollTimer = null
        await loadRunError(currentRunId.value)
      } else {
        // 运行中：逐步加载交易和快照（进度每前进 5% 刷新一次）
        if (status.progress_pct > 0 && status.progress_pct % 5 === 0) {
          await loadRunningResults(currentRunId.value)
        }
      }
    } catch (e) { console.error('轮询失败:', e) }
  }, 2000) // 2 秒轮询一次，避免过于频繁
}

/** 回测运行中逐步加载中间结果 */
async function loadRunningResults(runId: number) {
  try {
    // 加载交易（第一页，让用户看到已产生的买卖记录）
    if (tradeData.value.length === 0) {
      await loadTradePage(1)
    }
    // 加载快照（净值曲线逐步展示）
    await loadSnapshots(runId)
  } catch (e) { /* 静默忽略中间加载失败 */ }
}

// ========== 结果数据 ==========
interface StatItem { key: string; label: string; value: string; color?: string; cls?: string }
const statsData = ref<StatItem[]>([])

interface TradeRow { date: string; stockName: string; stockCode: string; dir: string; price: string; qty: string; amount: string; fee: string; reason: string | null; pnl: string; pnlPct: string; pnlClass: string }
const tradeData = ref<TradeRow[]>([])
const tradePage = ref(1)
const tradeTotal = ref(0)
const tradeTotalPages = computed(() => Math.max(1, Math.ceil(tradeTotal.value / 20)))

const rawSnapshots = ref<strategyApi.DailySnapshot[]>([])
const snapshotData = ref<{ date: string; strategy: number }[]>([])

function clearResults() {
  statsData.value = []; tradeData.value = []; rawSnapshots.value = []; snapshotData.value = []
  tradePage.value = 1; tradeTotal.value = 0; elapsedTime.value = ''; backtestLogs.value = []
}

async function loadResults(runId: number) {
  try {
    const run = await strategyApi.getBacktestRun(runId)
    // 填充指标
    const items: StatItem[] = []
    const addItem = (key: string, label: string, v: number | null | undefined, suffix = '%', isNegBad = false) => {
      if (v == null) return
      const val = v as number
      items.push({
        key, label,
        value: `${val >= 0 && !isNegBad ? '+' : ''}${val.toFixed(2)}${suffix}`,
        color: val >= 0 ? '#cf1322' : '#16a34a',
        cls: val >= 0 ? 'pos' : 'neg',
      })
    }
    addItem('total_return', '累计收益', run.total_return)
    addItem('annual_return', '年化收益', run.annual_return)
    addItem('max_drawdown', '最大回撤', run.max_drawdown, '%', true)
    if (run.sharpe_ratio != null) items.push({ key: 'sharpe', label: '夏普比率', value: run.sharpe_ratio.toFixed(2) })
    addItem('win_rate', '胜率', run.win_rate)
    if (run.profit_factor != null) items.push({ key: 'profit_factor', label: '盈亏比', value: run.profit_factor.toFixed(2) })
    items.push({ key: 'trade_count', label: '交易次数', value: String(run.trade_count) })
    if (run.final_equity != null) items.push({ key: 'final_equity', label: '最终权益', value: `¥${run.final_equity.toLocaleString('zh-CN', { maximumFractionDigits: 0 })}` })
    if (run.stop_loss_count > 0) items.push({ key: 'stop_loss', label: '止损次数', value: String(run.stop_loss_count), color: '#cf1322' })
    if (run.take_profit_count > 0) items.push({ key: 'take_profit', label: '止盈次数', value: String(run.take_profit_count), color: '#16a34a' })
    statsData.value = items

    addLog('info', `累计收益: ${run.total_return != null ? run.total_return.toFixed(2) + '%' : 'N/A'}，最大回撤: ${run.max_drawdown != null ? run.max_drawdown.toFixed(2) + '%' : 'N/A'}`)

    // 加载交易
    await loadTradePage(1)
    // 加载快照
    await loadSnapshots(runId)
  } catch (e: any) {
    addLog('error', `加载结果失败: ${e?.message || '未知错误'}`)
  }
}

async function loadTradePage(page: number) {
  if (!currentRunId.value) return
  try {
    const res = await strategyApi.getBacktestTrades(currentRunId.value, page, 20)
    tradeData.value = (res.items || []).map(t => ({
      date: t.trade_date,
      stockName: t.stock_name || '',
      stockCode: t.stock_code,
      dir: t.trade_type === 1 ? '买入' : '卖出',
      price: t.price.toFixed(2),
      qty: String(t.quantity),
      amount: t.amount.toLocaleString('zh-CN', { maximumFractionDigits: 0 }),
      fee: t.trade_type === 2 ? `¥${(t.commission + t.stamp_tax).toFixed(2)}` : `¥${t.commission.toFixed(2)}`,
      reason: t.exit_reason || null,
      // 盈亏已包含手续费（后端 ProfitLoss = sellAmount - entryAmount - commission - stampTax）
      pnl: t.profit_loss != null ? `${t.profit_loss >= 0 ? '+' : ''}¥${t.profit_loss.toFixed(0)}` : '—',
      pnlPct: t.profit_loss_pct != null ? `${t.profit_loss_pct >= 0 ? '+' : ''}${t.profit_loss_pct.toFixed(2)}%` : '—',
      pnlClass: t.profit_loss != null ? (t.profit_loss > 0 ? 'pos' : 'neg') : '',
    }))
    tradePage.value = page
    tradeTotal.value = res.total
  } catch (e) { console.error('加载交易失败:', e) }
}

async function loadSnapshots(runId: number) {
  try {
    const res = await strategyApi.getBacktestSnapshots(runId)
    rawSnapshots.value = res.snapshots || []
    snapshotData.value = (res.snapshots || []).map(s => ({
      date: s.snap_date.slice(5),
      strategy: s.cumulative_return ?? 0,
    }))
  } catch (e) { console.error('加载快照失败:', e) }
}

async function loadRunError(runId: number) {
  try {
    const run = await strategyApi.getBacktestRun(runId)
    errorMessage.value = run.error_message || '未知错误'
    addLog('error', `回测失败: ${errorMessage.value}`)
  } catch (e) { /* ignore */ }
}

function reasonLabel(reason: string): string {
  const map: Record<string, string> = {
    stop_loss: '🛑 止损', take_profit: '✅ 止盈', time_exit: '⏰ 到期',
    trailing_stop: '📉 移动止盈', segment_profit: '📊 分段止盈',
    signal_exit: '📡 信号', signal: '📡 信号', force_close: '🏁 清仓',
  }
  return map[reason] || reason
}

function ruleName(type: string): string {
  const map: Record<string, string> = {
    stop_loss: '止损', take_profit: '止盈', time_exit: '到期退出',
    trailing_stop: '移动止盈', segment_profit: '分段止盈',
    signal_exit: '信号退出',
  }
  return map[type] || type
}

function onGoBack() {
  if (selectedStrategy.value) emit('goToEdit', selectedStrategy.value.id)
  else emit('goBack')
}

// ========== K 线悬浮 ==========
const klineVisible = ref(false)
const klineStockCode = ref('')
const klineStockName = ref('')
const klineX = ref(0)
const klineY = ref(0)
let klineTimer: ReturnType<typeof setTimeout> | null = null
let klineHideTimer: ReturnType<typeof setTimeout> | null = null

function showKLine(e: MouseEvent, code: string) {
  if (klineHideTimer) { clearTimeout(klineHideTimer); klineHideTimer = null }
  if (klineTimer) clearTimeout(klineTimer)
  klineTimer = setTimeout(() => {
    klineStockCode.value = code
    klineStockName.value = ''
    klineX.value = e.clientX
    klineY.value = e.clientY
    klineVisible.value = true
  }, 350)
}

function hideKLine() {
  if (klineTimer) { clearTimeout(klineTimer); klineTimer = null }
  if (!klineHideTimer) {
    klineHideTimer = setTimeout(() => {
      klineVisible.value = false
      klineHideTimer = null
    }, 200)
  }
}

function onKLineEnter() {
  if (klineHideTimer) { clearTimeout(klineHideTimer); klineHideTimer = null }
}

// ========== 图表 ==========
const chartW = 1100
const chartH = 220
const yAxisW = 50
const marginR = 20
const paddingTop = 18
const paddingBottom = 24

const yMin = computed(() => {
  if (snapshotData.value.length === 0) return -5
  return Math.min(...snapshotData.value.map(d => d.strategy)) - 2
})
const yMax = computed(() => {
  if (snapshotData.value.length === 0) return 5
  return Math.max(...snapshotData.value.map(d => d.strategy)) + 2
})
const yRange = computed(() => yMax.value - yMin.value || 1)
function yPos(v: number) { return paddingTop + ((yMax.value - v) / yRange.value) * (chartH - paddingTop - paddingBottom) }
function xPos(i: number) { return yAxisW + i * xStep.value }
const zeroY = computed(() => yPos(0))
const yStep = computed(() => (chartH - paddingTop - paddingBottom) / 5)
const yTickLabels = computed(() => Array.from({ length: 6 }, (_, i) => (yMax.value - i * yRange.value / 5).toFixed(1) + '%'))
const gridYs = computed(() => Array.from({ length: 5 }, (_, i) => paddingTop + (i + 1) * yStep.value))
const xLabels = computed(() => {
  const d = snapshotData.value
  return d.filter((_, i) => i % Math.max(1, Math.floor(d.length / 8)) === 0 || i === d.length - 1).map(p => p.date)
})
const xStep = computed(() => snapshotData.value.length > 1 ? (chartW - yAxisW - marginR) / (snapshotData.value.length - 1) : chartW - yAxisW - marginR)
const stratPoints = computed(() => snapshotData.value.map((d, i) => `${xPos(i)},${yPos(d.strategy)}`).join(' '))
</script>

<style scoped>
/* ===== 页面头部（使用全局 .page-header 样式）====== */

.back-btn {
  border: 1px solid #d9d9d9; border-radius: 6px; background: #fff;
  font-size: 13px; cursor: pointer; padding: 4px 10px; transition: .15s; white-space: nowrap;
}
.back-btn:hover { background: #f5f5f5; }

/* ===== 工具栏 ===== */
.toolbar-card { background: #fff; border-radius: 12px; box-shadow: 0 1px 4px rgba(0,0,0,.06); padding: 14px 18px; margin-bottom: 14px; }
.toolbar-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; font-size: 13px; color: #555; }
.strategy-select-wrap { position: relative; display: inline-block; }
.strategy-select-trigger {
  border: 1.5px solid #e0e0e0; border-radius: 6px; padding: 4px 10px 4px 12px;
  font-size: 15px; font-weight: 700; cursor: pointer; display: flex; align-items: center; gap: 6px;
  width: 220px; background: #fff; color: #1a1a2e; transition: all .15s; user-select: none;
}
.strategy-select-trigger:hover { border-color: #1677ff; }
.strategy-select-trigger.open { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.1); }
.sel-text { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 180px; }
.sel-placeholder { color: #bbb; }
.sel-arrow { font-size: 11px; color: #999; flex-shrink: 0; transition: transform .15s; }
.strategy-select-trigger.open .sel-arrow { transform: rotate(180deg); }
.sd-overlay { position: fixed; inset: 0; z-index: 998; }
.sd-dropdown { position: fixed; z-index: 999; background: #fff; border: 1px solid #e8e8e8; border-radius: 8px; box-shadow: 0 6px 24px rgba(0,0,0,.14); padding: 4px 0; animation: sd-fade .12s ease-out; }
@keyframes sd-fade { from { opacity: 0; transform: translateY(3px); } to { opacity: 1; } }
.sd-search-input { margin: 4px 8px 6px; padding: 5px 10px; width: calc(100% - 16px); border: 1px solid #d9d9d9; border-radius: 5px; font-size: 12.5px; outline: none; box-sizing: border-box; }
.sd-search-input:focus { border-color: #1677ff; }
.sd-list { max-height: 260px; overflow-y: auto; padding: 0 4px; }
.sd-item { display: flex; align-items: center; justify-content: space-between; padding: 7px 12px; cursor: pointer; border-radius: 5px; gap: 8px; transition: background .1s; }
.sd-item:hover, .sd-item.highlighted { background: #f5f7fa; }
.sd-item.active { background: #e6f4ff; color: #1677ff; font-weight: 600; }
.sd-name { font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sd-meta { font-size: 11px; color: #aaa; flex-shrink: 0; }
.sd-empty { text-align: center; padding: 20px 16px; color: #bbb; font-size: 13px; }

.date-picker, .capital-input { border: 1px solid #d9d9d9; border-radius: 6px; padding: 4px 8px; font-size: 13px; outline: none; transition: border-color .15s; }
.date-picker:focus, .capital-input:focus { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.08); }
.capital-input { width: 110px; }
.toolbar-sep { color: #ddd; }
.toolbar-right { display: inline-flex; gap: 6px; align-items: center; margin-left: auto; }
.elapsed { font-size: 12px; color: #888; font-family: 'SF Mono', Monaco, monospace; }
.error-hint { font-size: 12px; color: #cf1322; max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; cursor: help; }

.status-badge { display: inline-flex; align-items: center; gap: 4px; padding: 3px 10px; border-radius: 10px; font-size: 12px; font-weight: 600; }
.status-badge.done { background: #f6ffed; color: #389e0d; }
.status-badge.running { background: #e6f4ff; color: #1677ff; }
.status-badge.pending { background: #fffbe6; color: #d48806; }
.status-badge.failed { background: #fff2f0; color: #cf1322; }
.status-badge.idle { background: #f5f5f5; color: #999; }
.status-dot { width: 7px; height: 7px; border-radius: 50%; }
.status-badge.done .status-dot { background: #52c41a; }
.status-badge.running .status-dot { animation: pulse 1s infinite; background: #1677ff; }
.status-badge.pending .status-dot { animation: pulse 1s infinite; background: #d48806; }
.status-badge.failed .status-dot { background: #cf1322; }
@keyframes pulse { 0%, 100% { opacity: 1; } 50% { opacity: .3; } }

.toolbar-actions { display: flex; gap: 8px; margin-top: 10px; padding-top: 10px; border-top: 1px solid #f0f0f0; }
.btn-primary {
  display: inline-flex; align-items: center; gap: 4px; padding: 7px 18px; font-size: 13px; font-weight: 600;
  color: #fff; background: #1677ff; border: 1px solid #1677ff; border-radius: 5px; cursor: pointer; transition: .15s;
}
.btn-primary:hover:not(:disabled) { background: #0958d9; }
.btn-primary:disabled { opacity: .5; cursor: not-allowed; }
.btn-outline {
  padding: 7px 14px; font-size: 13px; font-weight: 500; color: #555; background: #fff;
  border: 1px solid #d9d9d9; border-radius: 5px; cursor: pointer; transition: .15s;
}
.btn-outline:hover { border-color: #1677ff; color: #1677ff; }

/* ===== 规则面板 ===== */
.rules-panel { margin-top: 10px; padding: 12px 14px; background: #fafafa; border: 1px solid #eee; border-radius: 8px; display: flex; flex-direction: column; gap: 12px; }
.rules-title { font-size: 13px; font-weight: 600; color: #555; margin: 0 0 8px; }
.rules-grid { display: flex; gap: 16px; flex-wrap: wrap; }
.rule-item { display: flex; align-items: center; gap: 4px; font-size: 12.5px; color: #666; cursor: pointer; user-select: none; }
.rule-item input[type="checkbox"] { cursor: pointer; }
.rule-input { width: 60px; padding: 2px 6px; border: 1px solid #d9d9d9; border-radius: 4px; font-size: 12.5px; text-align: right; outline: none; }
.rule-input:focus { border-color: #1677ff; }
.rule-input:disabled { background: #f5f5f5; color: #bbb; }
.rule-unit { font-size: 11px; color: #999; }
.rule-select { padding: 2px 6px; border: 1px solid #d9d9d9; border-radius: 4px; font-size: 12.5px; outline: none; cursor: pointer; }

/* ===== Tab ===== */
.tab-nav { display: flex; gap: 2px; margin-bottom: 16px; border-bottom: 1px solid #e8e8e8; }
.tab-btn {
  display: flex; align-items: center; gap: 4px; padding: 8px 16px; font-size: 13px; font-weight: 500;
  color: #666; background: transparent; border: none; cursor: pointer; position: relative; transition: all .15s;
}
.tab-btn:hover { color: #333; }
.tab-btn.active { color: #1677ff; font-weight: 700; }
.tab-btn.active::after { content: ''; position: absolute; bottom: -1px; left: 0; right: 0; height: 2px; background: #1677ff; border-radius: 1px 1px 0 0; }
.tab-icon { font-size: 14px; }
.tab-content { min-height: 400px; }
.tab-panel { animation: fade-in .15s ease-out; }
@keyframes fade-in { from { opacity: 0; } to { opacity: 1; } }

/* ===== 空状态 ===== */
.empty-state { text-align: center; padding: 60px 20px; color: #999; }
.empty-icon { font-size: 48px; display: block; margin-bottom: 12px; }
.empty-state p { font-size: 14px; margin: 0; }
.progress-bar { width: 200px; height: 6px; background: #f0f0f0; border-radius: 3px; margin: 16px auto 0; overflow: hidden; }
.progress-fill { height: 100%; background: #1677ff; border-radius: 3px; transition: width .3s ease; }

/* ===== 收益指标 ===== */
.stats-section { margin-bottom: 20px; }
.section-title { font-size: 16px; font-weight: 700; color: #333; margin: 0 0 12px; padding-bottom: 8px; border-bottom: 1px solid #f0f0f0; }
.stats-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 1px; background: #f0f0f0; border: 1px solid #e8e8e8; border-radius: 10px; overflow: hidden; }
.stat-item { background: #fff; padding: 12px 14px; display: flex; justify-content: space-between; align-items: center; }
.stat-label { font-size: 12.5px; color: #888; }
.stat-value { font-size: 15px; font-weight: 700; font-family: 'SF Mono', Monaco, monospace; }
.stat-item.neg .stat-value { color: #cf1322; }
.stat-item.pos .stat-value { color: #16a34a; }

/* ===== 净值曲线 ===== */
.chart-section { background: #fff; border-radius: 12px; box-shadow: 0 1px 4px rgba(0,0,0,.06); padding: 16px 18px; }
.chart-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.chart-title { font-size: 14px; font-weight: 600; color: #333; }
.chart-body { overflow-x: auto; }
.chart-body svg { display: block; width: 100%; max-width: 100%; height: auto; }
.grid-line { stroke: #f5f5f5; stroke-width: 1; }
.zero-line { stroke: #e0e0e0; stroke-width: 1; stroke-dasharray: 4 3; }
.axis-label { fill: #bbb; font-size: 11px; }
.line-strategy { stroke: #1677ff; stroke-width: 2; stroke-linejoin: round; fill: none; }
.dot-strategy { fill: #1677ff; }

/* ===== 面板 ===== */
.panel-section { background: #fff; border-radius: 12px; box-shadow: 0 1px 4px rgba(0,0,0,.06); padding: 20px 22px; }
.panel-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }
.panel-header .section-title { margin-bottom: 0; padding-bottom: 0; border-bottom: none; }
.pager { display: flex; align-items: center; gap: 8px; font-size: 13px; color: #666; }
.pager button { padding: 2px 8px; border: 1px solid #d9d9d9; border-radius: 4px; background: #fff; cursor: pointer; font-size: 14px; }
.pager button:hover:not(:disabled) { border-color: #1677ff; color: #1677ff; }
.pager button:disabled { opacity: .4; cursor: not-allowed; }

/* ===== 表格 ===== */
.data-table { width: 100%; border-collapse: collapse; font-size: 13px; }
.data-table th { background: #fafafa; padding: 8px 12px; text-align: left; font-weight: 600; color: #666; border-bottom: 1px solid #eee; white-space: nowrap; }
.data-table td { padding: 8px 12px; border-bottom: 1px solid #f5f5f5; color: #333; }
.data-table .pos { color: #cf1322; font-weight: 600; }
.data-table .neg { color: #16a34a; font-weight: 600; }

/* 卖出原因标签 */
.reason-tag { font-size: 11px; padding: 1px 6px; border-radius: 3px; white-space: nowrap; }
.reason-stop_loss { background: #fff2f0; color: #cf1322; }
.reason-take_profit { background: #f6ffed; color: #389e0d; }
.reason-time_exit { background: #fffbe6; color: #d48806; }
.reason-trailing_stop { background: #fff0f6; color: #c41d7f; }
.reason-segment_profit { background: #f9f0ff; color: #722ed1; }
.reason-signal { background: #f5f5f5; color: #999; }
.reason-force_close { background: #e6f4ff; color: #1677ff; }

/* ===== 动态规则列表 ===== */
.rules-list { display: flex; flex-direction: column; gap: 8px; }
.rule-row { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.rule-check { display: flex; align-items: center; gap: 4px; cursor: pointer; min-width: 90px; }
.rule-label { font-size: 13px; font-weight: 500; color: #333; }
.rule-input-sm { width: 56px; padding: 3px 6px; border: 1px solid #d9d9d9; border-radius: 4px; font-size: 13px; text-align: center; }
.rule-input-xs { width: 44px; padding: 2px 4px; border: 1px solid #d9d9d9; border-radius: 4px; font-size: 12px; text-align: center; }
.rule-param-label { font-size: 12px; color: #888; }
.rules-footer { margin-top: 8px; display: flex; align-items: center; gap: 6px; font-size: 13px; color: #666; }
.segment-level { display: inline-flex; align-items: center; gap: 2px; font-size: 12px; color: #666; }
.btn-level-del { padding: 0 4px; border: none; background: transparent; color: #cf1322; cursor: pointer; font-size: 12px; }
.btn-level-add { margin-top: 2px; padding: 1px 8px; border: 1px dashed #d9d9d9; border-radius: 4px; background: transparent; color: #1677ff; cursor: pointer; font-size: 11px; }

/* ===== 回测历史选择器 ===== */
.history-select-wrap { position: relative; display: inline-block; }
.history-btn {
  display: inline-flex; align-items: center; gap: 6px; position: relative; font-weight: 500;
}
.history-btn.active { border-color: #1677ff; color: #1677ff; }
.history-prefix { font-size: 13px; color: #888; flex-shrink: 0; }
.history-text { max-width: 160px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-family: 'SF Mono', Monaco, monospace; font-size: 12px; }
.history-placeholder { color: #bbb; font-size: 12px; }
.history-arrow { font-size: 10px; color: #999; transition: transform .15s; }
.history-arrow.open { transform: rotate(180deg); }
.history-dropdown .sd-list { max-height: 260px; overflow-y: auto; }
.history-status-done { color: #52c41a; }
.history-status-running { color: #1677ff; }
.history-status-pending { color: #d48806; }
.history-status-failed { color: #cf1322; }

/* 代码列悬浮 */
.code-col { cursor: pointer; }
.code-hover {
  padding: 1px 4px; border-radius: 3px; transition: background .15s;
}
.code-hover:hover { background: #e6f4ff; color: #1677ff; }

/* ===== 日志 ===== */
.log-console { background: #1a1a2e; border-radius: 8px; padding: 12px 14px; max-height: 400px; overflow-y: auto; font-family: 'SF Mono', Monaco, monospace; font-size: 12.5px; }
.log-line { line-height: 1.8; color: #ccc; }
.log-time { color: #666; margin-right: 8px; }
.log-tag { margin-right: 8px; font-weight: 600; font-size: 11px; }
.log-info .log-tag { color: #1677ff; }
.log-warn .log-tag { color: #faad14; }
.log-error .log-tag { color: #cf1322; }
.log-info .log-msg { color: #e0e0e0; }
.log-warn .log-msg { color: #faad14; }
.log-error .log-msg { color: #cf1322; }
.log-empty { text-align: center; color: #666; padding: 40px 0; }
</style>
