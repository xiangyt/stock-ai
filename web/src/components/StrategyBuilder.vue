<template>
  <div class="strategy-builder">

    <!-- 页面顶部标题栏（独立于卡片） -->
    <div class="page-top-bar">
      <div class="ptb-left">
        <button class="back-btn" @click="$emit('goBack')" title="返回列表">‹</button>
        <span class="page-title">{{ editingId ? '编辑策略' : '新建策略' }}</span>
      </div>
      <div class="ptb-right">
        <!-- 策略名称（单击可编辑） -->
        <template v-if="isEditingName">
          <input
            v-model="strategyName"
            class="inline-name-input"
            @keyup.enter="isEditingName = false"
            @blur="isEditingName = false"
            placeholder="输入策略名称"
            ref="nameInputRef"
          />
        </template>
        <span
          v-else
          class="inline-name-text"
          @click="startEditName"
          :title="'点击编辑名称'"
        >{{ strategyName || '未命名策略' }}</span>
        <button class="btn-save-sm" @click="saveStrategy" :disabled="signals.length === 0" title="保存策略">💾 保存</button>
        <button class="btn-save-sm btn-outline-sm" @click="exportJSON" title="导出策略JSON">导出</button>
        <label class="btn-save-sm btn-outline-sm" title="导入策略JSON">导入
          <input type="file" accept=".json" @change="importJSON" style="display:none" ref="importFileRef" />
        </label>
      </div>
    </div>

    <!-- ========== Section 1: AI 输入区 ========== -->
    <section class="sec-input">
      <!-- AI 文本输入 -->
      <div class="ai-input-area">
        <textarea
          v-model="aiText"
          placeholder="例如：&#10;• MACD金叉且PE在20-50倍之间&#10;• 高ROE(>15%)的小盘成长股，市值小于50亿"
          rows="3"
          @keydown.meta.enter="handleAISubmit"
          @keydown.ctrl.enter="handleAISubmit"
        ></textarea>
        <div class="ai-toolbar">
          <div class="ai-tools-left">
            <span class="ai-tool-label">☰ A股 ▾</span>
            <span class="ai-tool-label" :class="{ active: showAddPanel }" @click="toggleAddPanel">🔍 条件选股</span>
            <span class="ai-tool-label dim">★ 我的收藏</span>
          </div>
          <div class="ai-tools-right">
            <span class="ai-hint-text">AI识别输入框</span>
            <button class="btn-ai-send" :disabled="!aiText.trim()" @click="handleAISubmit">
              ⚡ 发送
            </button>
          </div>
        </div>
      </div>
    </section>

    <!-- ========== Section 2: 信号选择器 ========== -->
    <section class="sec-signals">
      <div class="sec-header-row">
        <div class="sec-left">
          <h3 class="sec-title">已选条件</h3>
          <span class="sig-count-tag" v-if="signals.length > 0">{{ signals.length }} 个条件</span>
        </div>
        <div class="sec-right">
          <button class="btn-add-cond" @click="toggleAddPanel">
            ＋ 添加条件
          </button>
        </div>
      </div>

      <!-- 可展开的五步选择器面板 -->
      <transition name="slide-down">
        <div v-if="showAddPanel" class="add-panel-inline">
          <div class="add-panel-inner">
            <!-- Step 1: 分类 -->
            <div class="step-block">
              <div class="step-label"><span class="step-num">①</span> 分类</div>
              <div class="cat-tabs">
                <button
                  v-for="(label, cat) in categoryLabels"
                  :key="cat"
                  :class="['cat-tab', { active: state.category === cat }]"
                  @click="selectCategory(cat as Category)"
                >{{ label }}</button>
              </div>
            </div>

            <!-- Step 2: 指标 -->
            <div class="step-block" v-if="state.category">
              <div class="step-label"><span class="step-num">②</span> 指标
                <span class="step-count">{{ indicatorsInCat.length }} 个可选</span>
              </div>
              <div class="indicator-scroll">
                <button v-for="ind in indicatorsInCat" :key="ind.id"
                  :class="['ind-btn', { selected: state.indicator?.id === ind.id, 'has-presets': ind.presets.length > 0 }]"
                  @click="selectIndicator(ind)">
                  <div class="ind-main">
                    <span class="ind-name">{{ ind.name }}</span>
                    <span v-if="state.indicator?.id === ind.id" class="ind-desc">{{ ind.description }}</span>
                  </div>
                  <span class="ind-meta">
                    <span class="type-badge" :class="ind.valueType">{{ valueTypeLabels[ind.valueType] }}</span>
                    <span v-if="ind.presets.length > 0" class="preset-count">{{ ind.presets.length }}个模板</span>
                    <span v-else class="preset-count dim">自定义</span>
                  </span>
                </button>
              </div>
            </div>

            <!-- Step 3: 信号模板 -->
            <div class="step-block" v-if="state.indicator">
              <div class="step-label"><span class="step-num">③</span> 信号类型
                <span class="step-hint" v-if="state.indicator!.presets.length > 0">该指标有 {{ state.indicator!.presets.length }} 种预设信号</span>
                <span class="step-hint dim" v-else>无预设模板，将使用自由组合模式</span>
              </div>
              <template v-if="state.indicator!.presets.length > 0">
                <div class="preset-grid">
                  <button v-for="p in state.indicator!.presets" :key="p.id"
                    :class="['preset-card', { selected: state.preset?.id === p.id }]" @click="selectPreset(p)">
                    <div class="preset-name">{{ p.name }}</div>
                    <div class="preset-desc">{{ p.description }}</div>
                  </button>
                </div>
                <button :class="['custom-mode-btn', { active: state.customMode }]" @click="enterCustomMode">
                  ⚙️ 自定义模式
                </button>
              </template>
              <div v-else class="no-preset-hint">
                <p>该指标暂无预设模板，请手动配置</p>
                <button class="btn-small" @click="enterCustomMode">开始自定义 →</button>
              </div>
            </div>

            <!-- Step 4: 操作符 -->
            <div class="step-block" v-if="showOperatorStep">
              <div class="step-label"><span class="step-num">④</span> 操作符</div>
              <template v-if="groupedOperators.number.length > 0">
                <div class="op-group-label">数值比较</div>
                <div class="op-list">
                  <button v-for="op in groupedOperators.number" :key="op.operator"
                    :class="['op-btn', { selected: state.selectedOp?.operator === op.operator }]"
                    @click="selectOperator(op)">
                    <span class="op-sym">{{ op.symbol }}</span><span class="op-lbl">{{ op.label }}</span>
                  </button>
                </div>
              </template>
              <template v-if="groupedOperators.series.length > 0">
                <div class="op-group-label">序列分析</div>
                <div class="op-list">
                  <button v-for="op in groupedOperators.series" :key="op.operator"
                    :class="['op-btn', { selected: state.selectedOp?.operator === op.operator }]"
                    @click="selectOperator(op)">
                    <span class="op-sym">{{ op.symbol }}</span><span class="op-lbl">{{ op.label }}</span>
                  </button>
                </div>
              </template>
              <template v-if="groupedOperators.enum.length > 0 || groupedOperators.bool.length > 0">
                <div class="op-group-label">枚举/布尔</div>
                <div class="op-list">
                  <button v-for="op in [...groupedOperators.enum, ...groupedOperators.bool]" :key="op.operator"
                    :class="['op-btn', { selected: state.selectedOp?.operator === op.operator }]"
                    @click="selectOperator(op)">
                    <span class="op-sym">{{ op.symbol }}</span><span class="op-lbl">{{ op.label }}</span>
                  </button>
                </div>
              </template>
            </div>

            <!-- Step 5: 参数 -->
            <div class="step-block params-block" v-if="state.selectedOp && state.selectedOp!.params.length > 0">
              <div class="step-label"><span class="step-num">⑤</span> 参数设置</div>
              <div class="params-grid">
                <div v-for="param in state.selectedOp!.params" :key="param.key" class="param-item">
                  <label class="param-label">{{ param.label }}<span v-if="param.unit">({{ param.unit }})</span></label>
                  <input v-if="isNumberLike(param.type)" type="number" v-model.number="paramValues[param.key]"
                    :placeholder="param.placeholder || `默认: ${param.default}`" class="param-input" />
                  <select v-else-if="param.type === 'select'" v-model="paramValues[param.key]" class="param-input">
                    <option value="">请选择...</option>
                    <option v-for="o in getEnumOpts(state.indicator!.id)" :key="o.value" :value="o.value">{{ o.label }}</option>
                  </select>
                  <p v-if="param.description" class="param-tip">{{ param.description }}</p>
                </div>
              </div>
            </div>

            <!-- 添加按钮 -->
            <div class="add-btn-wrap">
              <button class="add-btn" :disabled="!canAdd" @click="addSignal">✅ 添加到策略</button>
              <transition name="fade-fast">
                <span v-if="addSuccessMsg" class="add-success-msg">{{ addSuccessMsg }}</span>
              </transition>
            </div>
          </div>
        </div>
      </transition>

      <!-- 空状态 -->
      <div v-if="signals.length === 0 && !showAddPanel" class="empty-signals">
        <div class="empty-icon">📭</div>
        <p>还没有信号条件</p>
        <p class="empty-sub">点击「＋添加条件」或使用上方 AI 输入框自动生成</p>
      </div>

      <!-- 已添加信号标签行 -->
      <div v-if="signals.length > 0" class="signals-chips-area">
        <transition-group name="sig-chip" tag="div" class="chips-row">
          <div v-for="(s, i) in signals" :key="s.uid"
            class="sig-chip" :class="'chip-' + s.category">
            <span class="chip-bar"></span>
            <span class="chip-name">{{ s.name }}</span>
            <span class="chip-op">{{ s.opSym }} {{ s.paramText }}</span>
            <button class="chip-del" @click="removeSignal(i)">✕</button>
          </div>
        </transition-group>
      </div>

      <!-- 底部操作栏 -->
      <div v-if="signals.length > 0" class="sec-footer">
        <div class="logic-toggle">
          <span class="logic-label">逻辑关系：</span>
          <button :class="['logic-btn', { active: logicalOp === 'AND' }]" @click="logicalOp = 'AND'">AND</button>
          <button :class="['logic-btn', { active: logicalOp === 'OR' }]" @click="logicalOp = 'OR'">OR</button>
        </div>
        <div class="footer-actions">
          <button class="btn-sec-sm" @click="showClearConfirm = true">清空全部</button>
        </div>
      </div>
    </section>

    <!-- ========== Section 3: 结果预览表 ========== -->
    <section class="sec-results">
      <div class="results-head">
        <div class="results-left">
          <h3 class="results-title">选出股票 <strong>{{ mockResults.length }}</strong></h3>
          <div class="results-tabs">
            <button class="rtab active">≡ 股票列表</button>
            <button class="rtab dim">⊞ 多股同列</button>
            <button class="rtab dim">📊 可视化分析</button>
          </div>
        </div>
        <div class="results-right">
          <button class="btn-res-action" @click="exportJSON">导出 ▾</button>
          <button class="btn-res-action run" @click="runFilter">🔍 运行筛选</button>
        </div>
      </div>

      <div class="results-toolbar">
        <button class="tb-tool">＋ 加自选</button>
        <button class="tb-tool">＋ 加板块</button>
        <select class="tb-select"><option>相关</option><option>涨跌</option><option>市值</option></select>
        <div class="tb-sort-tabs">
          <button class="st active">相关</button>
          <button class="st">概览</button>
          <button class="st">表现</button>
          <button class="st">技术</button>
          <button class="st">估值</button>
          <button class="st">财务</button>
        </div>
        <div class="tb-search">
          🔍 搜索 <input type="text" placeholder="代码/名称" class="tb-search-in" />
        </div>
        <div class="tb-extra">
          我的选择 ▾ &nbsp; ⬇ 导数据
        </div>
      </div>

      <div class="results-table-wrap">
        <table class="results-table">
          <thead>
            <tr>
              <th class="col-cb"><input type="checkbox" /></th>
              <th>序号</th>
              <th>股票代码</th>
              <th>股票简称</th>
              <th>现价(元)</th>
              <th>涨跌幅(%)</th>
              <th>最低价</th>
              <th>最高价</th>
              <th>开盘价</th>
              <th>成交量</th>
              <th>匹配信号</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(r, i) in mockResults" :key="i" :class="{ odd: i % 2 !== 0 }">
              <td class="col-cb"><input type="checkbox" /></td>
              <td>{{ i + 1 }}</td>
              <td class="code-col">{{ r.code }}</td>
              <td class="name-col">{{ r.name }}</td>
              <td>{{ r.price.toFixed(2) }}</td>
              <td :class="{ up: r.change > 0, down: r.change < 0 }">{{ r.change > 0 ? '+' : '' }}{{ r.change.toFixed(2) }}%</td>
              <td>{{ r.low.toFixed(2) }}</td>
              <td>{{ r.high.toFixed(2) }}</td>
              <td>{{ r.open.toFixed(2) }}</td>
              <td>{{ r.volume }}</td>
              <td class="match-tags">
                <span v-for="(tag, ti) in r.matchedSignals" :key="ti" class="match-tag">{{ tag }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <!-- 清空确认弹窗 -->
    <teleport to="body">
      <div class="modal-overlay" v-if="showClearConfirm" @click.self="showClearConfirm = false">
        <div class="modal-box">
          <div class="modal-title">⚠️ 确认清空</div>
          <p class="modal-body">确定要清空所有信号条件吗？此操作不可撤销。</p>
          <div class="modal-actions">
            <button class="btn-modal-cancel" @click="showClearConfirm = false">取消</button>
            <button class="btn-modal-danger" @click="confirmClear">确认清空</button>
          </div>
        </div>
      </div>
    </teleport>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, computed, nextTick } from 'vue'
import {
  getAllIndicators, categoryLabels, valueTypeLabels,
  Category, ValueType,
  type IndicatorWithPresets, type PresetSignal, type OperatorOption,
  CompareOperator,
} from '../mock/indicators'

// ========== 类型定义 ==========

interface Sig {
  uid: number
  id: string; name: string; category: Category
  operator: CompareOperator; opSym: string; opLbl: string
  params: Record<string, any>; paramText: string
}

// ========== 状态 ==========
const allData = computed(() => getAllIndicators())
const state = reactive({
  category: null as Category | null,
  indicator: null as IndicatorWithPresets | null,
  preset: null as PresetSignal | null,
  customMode: false,
  selectedOp: null as OperatorOption | null,
})
const paramValues = reactive<Record<string, any>>({})
const multiVals = reactive<Record<string, string[]>>({})
const signals = ref<Sig[]>([])
let uidCounter = 0
const logicalOp = ref<'AND' | 'OR'>('AND')
const strategyName = ref('')
const isEditingName = ref(false)
const nameInputRef = ref<HTMLInputElement | null>(null)
const importFileRef = ref<HTMLInputElement | null>(null)
const editingIdx = ref<number | null>(null)
const editParams = reactive<Record<string, any>>({})
const expandedJSON = reactive(new Set<number>())
const addSuccessMsg = ref('')
let successTimer: ReturnType<typeof setTimeout> | null = null
const showClearConfirm = ref(false)
interface BuilderEmits {
  (e: 'addSignals', signals: Sig[]): void
  (e: 'saved', strategy: { id: number; name: string }): void
  (e: 'goBack'): void
}
const emit = defineEmits<BuilderEmits>()
const editingId = ref<number | null>(null) // 后端数字 ID，null = 新建模式
const showAddPanel = ref(false)

// 策略名称内联编辑
function startEditName() {
  isEditingName.value = true
  nextTick(() => {
    nameInputRef.value?.focus()
    nameInputRef.value?.select()
  })
}

// AI 输入文本
const aiText = ref('')

// ========== API 保存逻辑 ==========
import * as strategyApi from '../api/strategies'

/** 保存策略到后端（新建或更新） */
async function saveStrategy() {
  if (signals.value.length === 0) { alert('请先添加至少一个信号条件'); return }
  const name = strategyName.value.trim()
  if (!name) { alert('请输入策略名称'); return }

  try {
    const payload = {
      name,
      logical_op: logicalOp.value,
      signals: signals.value.map(s => ({
        uid: s.uid, id: s.id, name: s.name,
        category: s.category, operator: s.operator,
        opSym: s.opSym, opLbl: s.opLbl,
        params: s.params, paramText: s.paramText,
      })),
      description: '',
    }

    let result
    if (editingId.value) {
      // 更新已有策略
      result = await strategyApi.updateStrategy(editingId.value, payload)
    } else {
      // 创建新策略
      result = await strategyApi.createStrategy(payload)
      editingId.value = result.id  // 保存后进入编辑模式
    }

    emit('saved', { id: result.id, name: result.name })
  } catch (e) {
    console.error('保存策略失败:', e)
    alert('保存失败: ' + (e as Error).message)
  }
}

// ========== 计算属性 ==========
const indicatorsInCat = computed(() => allData.value[state.category!] || [])
const showOperatorStep = computed(() => !!(state.indicator) && !!(state.preset || state.customMode))
const groupedOperators = computed(() => {
  const ops = state.indicator?.operators || []
  return {
    number: ops.filter(o => ['>', '>=', '<', '<=', 'between', 'not_between'].includes(o.operator)),
    series: ops.filter(o => ['cross_up', 'cross_down', 'divergence_pos', 'divergence_neg', 'breakout', 'breakdown'].includes(o.operator)),
    enum: ops.filter(o => ['in', 'not_in', 'contains'].includes(o.operator)),
    bool: ops.filter(o => ['=', '!='].includes(o.operator)),
  }
})
const canAdd = computed(() => {
  if (!state.indicator || !state.selectedOp) return false
  for (const p of state.selectedOp.params) {
    if (p.required && ((p.type === 'multiSelect' && (!multiVals[p.key] || multiVals[p.key].length === 0)) || (paramValues[p.key] === undefined || paramValues[p.key] === ''))) return false
  }
  return true
})

// Mock 结果表格数据
interface MockRow { code: string; name: string; price: number; change: number; low: number; high: number; open: number; volume: string; matchedSignals: string[] }
const mockResults = computed<MockRow[]>(() => {
  if (signals.value.length === 0) return []
  const baseStocks = [
    { code: '000609', name: '*ST中迪', price: 11.87 },
    { code: '002290', name: '禾盛新材', price: 81.35 },
    { code: '600433', name: '冠豪高新', price: 4.00 },
    { code: '600696', name: '*ST岩石', price: 1.38 },
    { code: '600770', name: '综艺股份', price: 6.97 },
    { code: '603318', name: '水发燃气', price: 11.39 },
    { code: '603813', name: '*ST原尚', price: 33.32 },
    { code: '001211', name: '双枪科技', price: 28.18 },
    { code: '600892', name: '*ST大晟', price: 3.68 },
    { code: '002569', name: '*ST步森', price: 14.02 },
  ]
  const sigNames = signals.value.map(s => s.name.slice(0, 6))
  return baseStocks.map((s, i) => ({
    ...s,
    change: (Math.random() * 10 - 4.5),
    low: s.price * (0.95 + Math.random() * 0.04),
    high: s.price * (1.01 + Math.random() * 0.06),
    open: s.price * (0.97 + Math.random() * 0.04),
    volume: `${Math.floor(Math.random() * 50000 + 5000)}手`,
    matchedSignals: sigNames.slice(0, Math.min(sigNames.length, Math.floor(Math.random() * sigNames.length) + 1)),
  })).sort((a, b) => b.change - a.change).slice(0, 7)
})

// ========== 方法 ==========
function toggleAddPanel() { showAddPanel.value = !showAddPanel.value }
function selectCategory(cat: Category) { state.category = cat; resetFrom(1) }
function selectIndicator(ind: IndicatorWithPresets) { state.indicator = ind; state.preset = null; state.customMode = false; state.selectedOp = null; clearParams() }
function selectPreset(p: PresetSignal) { state.preset = p; state.customMode = false; clearParams(); const defaultOp = findOpByComparator(p.defaultOperator); if (defaultOp) state.selectedOp = defaultOp; for (const [k, v] of Object.entries(p.defaultParams)) { Array.isArray(v) ? (multiVals[k] = [...v]) : (paramValues[k] = v) } }
function enterCustomMode() { state.customMode = true; state.preset = null; state.selectedOp = null; clearParams() }
function selectOperator(op: OperatorOption) { state.selectedOp = op; clearParams(); for (const p of op.params) { if (p.type === 'multiSelect') multiVals[p.key] = []; else if (p.default !== undefined) paramValues[p.key] = p.default } }
function resetFrom(step: number) { if (step <= 2) state.indicator = null; if (step <= 3) { state.preset = null; state.customMode = false } if (step <= 4) state.selectedOp = null; clearParams() }
function clearParams() { for (const k of Object.keys(paramValues)) delete paramValues[k]; for (const k of Object.keys(multiVals)) multiVals[k] = [] }
function isNumberLike(t: string): boolean { return ['number', 'range', 'threshold', 'days'].includes(t) }
function getEnumOpts(indID: string): { value: string; label: string }[] {
  const eo = (window as any).__enumOptions || {}
  if (indID === 'listing_board') { const vs = eo.listing_board || ['main','chinext','star','neeq']; const ls = eo.listing_board_labels || vs; return vs.map((v: string, i: number) => ({ value: v, label: ls[i] || v })) }
  if (indID === 'industry') return (eo.industry || []).map((v: string) => ({ value: v, label: v }))
  return []
}
function findOpByComparator(cmp: CompareOperator): OperatorOption | undefined { return state.indicator?.operators.find(o => o.operator === cmp) }
function getOperatorSymbol(cmp: CompareOperator): string { return findOpByComparator(cmp)?.symbol ?? cmp }
function getOperatorLabel(cmp: CompareOperator): string { return findOpByComparator(cmp)?.label ?? '' }
function catLabel(c: Category): string { return categoryLabels[c] }

// ========== 信号操作 ==========
function addSignal() {
  if (!state.indicator || !state.selectedOp) return
  const ind = state.indicator; const op = state.selectedOp
  const collected: Record<string, any> = {}
  for (const p of op.params) { if (p.type === 'multiSelect') collected[p.key] = [...(multiVals[p.key] || [])]; else if (paramValues[p.key] !== undefined) collected[p.key] = paramValues[p.key]; else if (p.default !== undefined) collected[p.key] = p.default }
  let text = ''
  switch (op.operator) {
    case CompareOperator.GT: case CompareOperator.GTE: case CompareOperator.LT: case CompareOperator.LTE: text = `${collected.value_number}${ind.unit}`; break
    case CompareOperator.Between: case CompareOperator.NotBetween: text = `[${collected.min_value}, ${collected.max_value}]${ind.unit}`; break
    case CompareOperator.In: case CompareOperator.NotIn: text = `{${collected.value_list?.join(', ')}}`; break
    default: text = Object.entries(collected).map(([k, v]) => `${k}=${v}`).join(', ')
  }
  const newSig: Sig = { uid: ++uidCounter, id: ind.id, name: ind.name, category: ind.category, operator: op.operator, opSym: op.symbol, opLbl: op.label, params: collected, paramText: text }
  signals.value.push(newSig); emit('addSignals', [newSig])
  addSuccessMsg.value = `✓ 已添加: ${ind.name}`
  if (successTimer) clearTimeout(successTimer)
  successTimer = setTimeout(() => { addSuccessMsg.value = '' }, 2500)
  state.preset = null; state.customMode = false; state.selectedOp = null; clearParams()
}
function removeSignal(idx: number) { signals.value.splice(idx, 1) }
function confirmClear() { signals.value = []; showClearConfirm.value = false }

function handleAISubmit() { /* TODO: 对接 AI 解析 */ }

function exportJSON() {
  const json = JSON.stringify({ name: strategyName.value, logical_op: logicalOp.value, conditions: signals.value.map(s => ({ indicator_id: s.id, name: s.name, operator: s.operator, params: s.params })) }, null, 2)
  const blob = new Blob([json], { type: 'application/json' })
  const a = document.createElement('a'); a.href = URL.createObjectURL(blob); a.download = `${strategyName.value || '未命名策略'}_${Date.now()}.json`; a.click(); URL.revokeObjectURL(a.href)
}

function importJSON(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = () => {
    try {
      const data = JSON.parse(reader.result as string)
      if (data.name) strategyName.value = data.name
      if (data.logical_op) logicalOp.value = data.logical_op
      if (Array.isArray(data.conditions) && data.conditions.length > 0) {
        // 清空现有信号
        signals.value = []
        uidCounter = 0
        for (const c of data.conditions) {
          signals.value.push({
            uid: ++uidCounter,
            id: c.indicator_id || c.id || '',
            name: c.name || '',
            category: 'technical',
            operator: c.operator || '>',
            opSym: c.operator || '>',
            opLbl: c.operator || '>',
            params: c.params || {},
            paramText: JSON.stringify(c.params || {}),
          })
        }
      }
      addSuccessMsg.value = `✅ 成功导入 ${data.name || ''}，共 ${signals.value.length} 条条件`
    } catch { addSuccessMsg.value = '❌ 导入失败：文件格式不正确' }
  }
  reader.readAsText(file)
  // 重置 file input 以便重复选择同一文件
  ;(e.target as HTMLInputElement).value = ''
}
function runFilter() { alert('🔍 筛选功能即将对接后端 API，敬请期待！') }

/** 外部调用 */
function acceptAISignals(aiSignals: any[]) {
  for (const s of aiSignals) { signals.value.push({ uid: ++uidCounter, id: s.indicatorID, name: s.indicatorName, category: s.category as Category, operator: s.operator as CompareOperator, opSym: s.operatorSymbol, opLbl: s.operatorLabel, params: s.params, paramText: s.paramSummary }) }
}
/** 从策略列表加载策略到编辑器 */
function loadStrategyFromOutside(s: { id: string | number; name: string; signals: Sig[]; logicalOp: 'AND' | 'OR' }) {
  editingId.value = typeof s.id === 'number' ? s.id : parseInt(s.id)
  strategyName.value = s.name
  signals.value = JSON.parse(JSON.stringify(s.signals))
  logicalOp.value = s.logicalOp
  uidCounter = Math.max(...signals.value.map(s => s.uid), 0)
}
function resetAllSignals() { editingId.value = null; strategyName.value = ''; signals.value = []; logicalOp.value = 'AND'; uidCounter = 0 }

defineExpose({ acceptAISignals, loadStrategyFromOutside, resetAllSignals })
</script>

<style scoped>
/* ========== 三段式主布局 ========== */
.strategy-builder {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* ========== 页面顶部标题栏（卡片外） ========== */
.page-top-bar {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 14px;
}
.ptb-left {
  display: flex; align-items: center; gap: 10px;
}
.ptb-right {
  display: flex; align-items: center; gap: 8px;
}
.back-btn {
  width: 30px; height: 30px; border: 1px solid #d9d9d9; border-radius: 6px;
  background: #fff; cursor: pointer; font-size: 16px; color: #555;
  display: flex; align-items: center; justify-content: center; transition: .15s;
}
.back-btn:hover { background: #f5f5f5; color: #1677ff; border-color: #1677ff; }
.page-title { font-size: 20px; font-weight: 700; color: #1a1a2e; }

/* 内联策略名称（顶部栏右侧） */
.inline-name-text {
  font-size: 14px; color: #1677ff; cursor: pointer;
  padding: 4px 10px; border-radius: 4px; border: 1px solid transparent;
  transition: all .12s;
}
.inline-name-text:hover { background: #e6f4ff; border-color: #91caff; }
.inline-name-input {
  padding: 4px 10px; border: 1px solid #1677ff; border-radius: 4px;
  font-size: 14px; outline: none; color: #1a1a2e; font-weight: 500;
  width: 180px; background: #fff;
}
.inline-name-input:focus { box-shadow: 0 0 0 2px rgba(22,119,255,.08); }

/* 顶部栏按钮（统一样式） */
.btn-save-sm {
  padding: 6px 16px; font-size: 13px; font-weight: 500;
  color: #fff; background: #1677ff; border: 1px solid #1677ff;
  border-radius: 5px; cursor: pointer; transition: .12s; white-space: nowrap;
}
.btn-save-sm:hover { background: #0958d9; }
.btn-save-sm:disabled { opacity: 0.45; cursor: not-allowed; }
.btn-save-sm.btn-outline-sm {
  color: #555; background: #fff; border: 1px solid #d9d9d9;
}
.btn-save-sm.btn-outline-sm:hover { color: #1677ff; border-color: #1677ff; }

/* ========== Section 1: AI 输入区 ========== */
.sec-input { background: #fff; border: 1px solid #e8e8e8; border-radius: 10px; overflow: hidden; }

.ai-input-area { padding: 14px 20px 14px; }
.ai-input-area textarea {
  width: 100%; border: 1.5px solid #e0e0e0; border-radius: 8px;
  padding: 12px 16px; font-size: 14px; color: #333; resize: none;
  outline: none; transition: border-color .15s; font-family: inherit; line-height: 1.6;
  box-sizing: border-box;
}
.ai-input-area textarea:focus { border-color: #1677ff; box-shadow: 0 0 0 3px rgba(22,119,255,.06); }
.ai-input-area textarea::placeholder { color: #bbb; }

.ai-toolbar {
  display: flex; justify-content: space-between; align-items: center;
  margin-top: 10px; padding-top: 10px; border-top: 1px solid #f0f0f0;
}
.ai-tools-left, .ai-tools-right { display: flex; align-items: center; gap: 6px; }
.ai-tool-label {
  font-size: 13px; color: #555; padding: 4px 12px; border-radius: 4px;
  cursor: pointer; transition: .12s;
}
.ai-tool-label:hover { background: #f5f5f5; color: #333; }
.ai-tool-label.active { color: #cf1322; font-weight: 600; }
.ai-tool-label.dim { color: #bbb; cursor: default; }
.ai-tool-label.dim:hover { background: none; }
.ai-hint-text { font-size: 12px; color: #999; margin-right: 8px; }
.btn-ai-send {
  padding: 6px 18px; border: none; border-radius: 6px; background: #1677ff; color: #fff;
  font-size: 13px; font-weight: 600; cursor: pointer; transition: .15s; white-space: nowrap;
}
.btn-ai-send:hover:not(:disabled) { background: #0958d9; }
.btn-ai-send:disabled { background: #d9d9d9; cursor: not-allowed; }

/* ========== Section 2: 信号选择器 ========== */
.sec-signals { background: #fff; border: 1px solid #e8e8e8; border-radius: 10px; padding: 16px 20px; }

.sec-header-row {
  display: flex; justify-content: space-between; align-items: center;
  margin-bottom: 12px;
}
.sec-left { display: flex; align-items: center; gap: 10px; }
.sec-title { font-size: 15px; font-weight: 700; color: #1a1a2e; margin: 0; }
.sig-count-tag {
  font-size: 11.5px; padding: 2px 10px; border-radius: 10px;
  background: #fff7e6; color: #d46b08; font-weight: 600;
}
.sec-right { display: flex; gap: 8px; }
.btn-add-cond {
  padding: 6px 16px; border: 1.5px dashed #d9d9d9; border-radius: 6px;
  background: transparent; cursor: pointer; font-size: 13px; color: #555; transition: .15s;
}
.btn-add-cond:hover { border-color: #1677ff; color: #1677ff; }

/* 可展开的添加面板 */
.add-panel-inline {
  margin: 12px 0; border: 1px solid #eee; border-radius: 10px;
  overflow: hidden; background: #fafbfc;
}
.add-panel-inner { padding: 18px 20px; }

/* 步骤样式（复用原有） */
.step-block { margin-bottom: 16px; }
.step-label { display: flex; align-items: center; gap: 8px; font-size: 13.5px; font-weight: 600; color: #444; margin-bottom: 10px; }
.step-num { display: inline-flex; align-items: center; justify-content: center; width: 22px; height: 22px; border-radius: 50%; background: #1677ff; color: #fff; font-size: 12px; font-weight: 700; }
.step-count { margin-left: auto; font-weight: 400; color: #aaa; font-size: 11px; }
.step-hint { margin-left: auto; font-weight: 400; color: #888; font-size: 11px; }
.step-hint.dim { color: #bbb; }

.cat-tabs { display: flex; gap: 6px; flex-wrap: wrap; }
.cat-tab { padding: 7px 16px; border: 1.5px solid #d9d9d9; border-radius: 20px; background: #fafafa; cursor: pointer; font-size: 13px; transition: .15s; }
.cat-tab:hover { border-color: #aaa; }
.cat-tab.active { background: #1677ff; color: #fff; border-color: #1677ff; }

.indicator-scroll { display: flex; flex-direction: column; gap: 6px; max-height: 220px; overflow-y: auto; }
.indicator-scroll::-webkit-scrollbar { width: 4px; }
.indicator-scroll::-webkit-scrollbar-thumb { background: #ddd; border-radius: 3px; }
.ind-btn { display: flex; justify-content: space-between; align-items: flex-start; padding: 11px 14px; border: 1.5px solid #eee; border-radius: 10px; background: #fff; cursor: pointer; transition: .15s; text-align: left; }
.ind-btn:hover { border-color: #bbb; }
.ind-btn.selected { border-color: #1677ff; background: #e6f4ff; }
.ind-main { display: flex; flex-direction: column; gap: 3px; min-width: 0; }
.ind-name { font-size: 14px; font-weight: 600; }
.ind-desc { font-size: 11.5px; color: #888; line-height: 1.35; }
.ind-meta { display: flex; gap: 6px; align-items: center; flex-shrink: 0; margin-left: 8px; }
.type-badge { font-size: 10.5px; padding: 1px 7px; border-radius: 8px; background: #eee; color: #666; white-space: nowrap; }
.preset-count { font-size: 10.5px; color: #1677ff; font-weight: 500; white-space: nowrap; }
.preset-count.dim { color: #ccc; }

.preset-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.preset-card { display: flex; flex-direction: column; gap: 4px; padding: 12px 14px; border: 1.5px solid #e8e8e8; border-radius: 10px; background: #fcfcfc; cursor: pointer; transition: .15s; text-align: left; }
.preset-card:hover { border-color: #bbb; }
.preset-card.selected { border-color: #1677ff; background: #e6f4ff; }
.preset-name { font-size: 14px; font-weight: 700; color: #1a1a2e; }
.preset-desc { font-size: 12px; color: #777; }
.custom-mode-btn { width: 100%; margin-top: 6px; padding: 8px; border: 1.5px dashed #d9d9d9; border-radius: 8px; background: transparent; cursor: pointer; font-size: 12.5px; color: #666; transition: .15s; }
.custom-mode-btn:hover { border-color: #1677ff; color: #1677ff; }
.custom-mode-btn.active { border-style: solid; border-color: #1677ff; background: #e6f4ff; color: #1677ff; font-weight: 600; }

.no-preset-hint { text-align: center; padding: 14px; background: #fafafa; border-radius: 8px; border: 1px dashed #ddd; }
.no-preset-hint p { font-size: 13px; color: #888; margin-bottom: 8px; }
.btn-small { padding: 5px 14px; border: 1px solid #1677ff; border-radius: 6px; background: #fff; color: #1677ff; cursor: pointer; font-size: 12px; }

.op-group-label { font-size: 11.5px; font-weight: 600; color: #999; margin: 10px 0 6px; }
.op-list { display: flex; flex-direction: column; gap: 5px; }
.op-btn { display: flex; align-items: center; gap: 12px; padding: 11px 14px; border: 1.5px solid #eee; border-radius: 10px; cursor: pointer; background: #fff; transition: .15s; text-align: left; }
.op-btn:hover { border-color: #aaa; }
.op-btn.selected { border-color: #1677ff; background: #e6f4ff; }
.op-sym { font-family: monospace; font-size: 19px; font-weight: 700; width: 36px; text-align: center; }
.op-lbl { font-size: 14px; font-weight: 600; }

.params-block { background: #f9f9f9; padding: 12px; border-radius: 8px; border: 1px solid #eee; }
.params-grid { display: flex; flex-direction: column; gap: 10px; }
.param-item { display: flex; flex-direction: column; gap: 3px; }
.param-label { font-size: 12px; font-weight: 500; color: #555; }
.param-unit { color: #999; font-weight: 400; }
.req { color: #cf1322; }
.param-input { padding: 7px 10px; border: 1px solid #d9d9d9; border-radius: 6px; font-size: 13px; outline: none; }
.param-input:focus { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.08); }
.param-tip { font-size: 11px; color: #aaa; margin: 0; }

.add-btn-wrap { position: relative; margin-top: 6px; }
.add-btn { width: 100%; padding: 13px; font-size: 15px; font-weight: 700; color: #fff; background: linear-gradient(135deg, #1677ff, #0958d9); border: none; border-radius: 10px; cursor: pointer; transition: .2s; }
.add-btn:hover:not(:disabled) { transform: translateY(-1px); box-shadow: 0 4px 12px rgba(22,119,255,.35); }
.add-btn:disabled { background: #d9d9d9; cursor: not-allowed; }
.add-success-msg { position: absolute; bottom: -24px; left: 0; right: 0; text-align: center; font-size: 12.5px; color: #389e0d; font-weight: 600; }

/* 空状态 */
.empty-signals { text-align: center; padding: 40px 20px; }
.empty-signals .empty-icon { font-size: 44px; display: block; margin-bottom: 8px; }
.empty-signals p { font-size: 13.5px; color: #bbb; margin: 4px 0; }
.empty-sub { font-size: 12px !important; color: #ccc !important; }

/* 信号标签 chips */
.signals-chips-area { margin-top: 12px; }
.chips-row { display: flex; flex-wrap: wrap; gap: 8px; }
.sig-chip {
  display: inline-flex; align-items: center; gap: 6px; padding: 6px 12px 6px 8px;
  border: 1px solid #e8e8e8; border-radius: 6px; font-size: 13px;
  background: #fff; transition: all .15s; cursor: default;
}
.sig-chip:hover { border-color: #ccc; box-shadow: 0 1px 4px rgba(0,0,0,.06); }
.chip-bar { width: 3px; height: 16px; border-radius: 2px; flex-shrink: 0; }
.chip-technical .chip-bar { background: #08979c; }
.chip-market .chip-bar { background: #0958d9; }
.chip-fundamental .chip-bar { background: #d46b08; }
.chip-financial .chip-bar { background: #52c41a; }
.chip-name { font-weight: 600; color: #1a1a2e; }
.chip-op {
  font-family: monospace; font-size: 11.5px; font-weight: 600;
  padding: 1px 6px; border-radius: 4px; color: #fff;
  background: linear-gradient(135deg, #1677ff, #0958d9);
}
.chip-params { color: #666; font-size: 11.5px; }
.chip-del {
  background: none; border: none; cursor: pointer; font-size: 13px; color: #ccc;
  padding: 0 2px; transition: .12s;
}
.chip-del:hover { color: #cf1322; }

/* 底部操作栏 */
.sec-footer {
  display: flex; justify-content: space-between; align-items: center;
  margin-top: 14px; padding-top: 12px; border-top: 1px solid #f0f0f0;
}
.logic-toggle { display: flex; align-items: center; gap: 8px; font-size: 12.5px; color: #555; }
.logic-label { font-weight: 500; }
.logic-btn { padding: 5px 14px; border: 1px solid #d9d9d9; border-radius: 6px; background: #fff; font-size: 12px; font-weight: 500; cursor: pointer; color: #666; transition: .15s; }
.logic-btn:first-child { border-right: none; }
.logic-btn.active { background: #1677ff; color: #fff; border-color: #1677ff; }
.footer-actions { display: flex; gap: 8px; }
.btn-sec-sm { padding: 5px 14px; border: 1px solid #d9d9d9; border-radius: 6px; background: #fff; font-size: 12px; cursor: pointer; color: #666; }
.btn-sec-sm:hover { border-color: #cf1322; color: #cf1322; }

/* ========== Section 3: 结果预览表 ========== */
.sec-results { background: #fff; border: 1px solid #e8e8e8; border-radius: 10px; overflow: hidden; flex: 1; display: flex; flex-direction: column; min-height: 360px; }

.results-head {
  display: flex; justify-content: space-between; align-items: flex-start;
  padding: 14px 20px; border-bottom: 1px solid #f0f0f0; flex-wrap: wrap; gap: 10px;
}
.results-left { display: flex; align-items: center; gap: 12px; }
.results-title { font-size: 15px; font-weight: 700; color: #1a1a2e; margin: 0; }
.results-title strong { color: #cf1322; } /* 中国红涨 */
.results-tabs { display: flex; gap: 2px; }
.rtab { padding: 4px 12px; border: 1px solid transparent; border-radius: 4px; background: transparent; font-size: 12px; cursor: pointer; color: #777; transition: .12s; }
.rtab.active { background: #e6f4ff; color: #1677ff; border-color: #bae0ff; font-weight: 600; }
.rtab.dim { opacity: 0.5; cursor: default; }

.results-right { display: flex; align-items: center; gap: 8px; }
.res-strategy-name { font-size: 13px; color: #555; max-width: 140px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.btn-res-action {
  padding: 5px 14px; border-radius: 5px; font-size: 12.5px; font-weight: 500;
  cursor: pointer; transition: .15s; white-space: nowrap;
}
.btn-res-action.primary { background: #1677ff; color: #fff; border: 1px solid #1677ff; }
.btn-res-action.primary:hover { background: #0958d9; }
.btn-res-action { background: #fff; color: #555; border: 1px solid #d9d9d9; }
.btn-res-action:hover { border-color: #1677ff; color: #1677ff; }
.btn-res-action.run { background: #52c41a; color: #fff; border: 1px solid #52c41a; }
.btn-res-action.run:hover { background: #389e0d; }

.results-toolbar {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 20px; border-bottom: 1px solid #f0f0f0; font-size: 12px; flex-wrap: wrap;
}
.tb-tool { padding: 4px 10px; border: 1px solid #d9d9d9; border-radius: 4px; background: #fff; cursor: pointer; color: #555; font-size: 12px; transition: .12s; }
.tb-tool:hover { border-color: #1677ff; color: #1677ff; }
.tb-select { padding: 4px 8px; border: 1px solid #d9d9d9; border-radius: 4px; font-size: 12px; color: #555; outline: none; background: #fff; cursor: pointer; }
.tb-sort-tabs { display: flex; gap: 2px; margin-left: 4px; }
.st { padding: 4px 10px; border: none; border-radius: 4px; background: transparent; font-size: 12px; cursor: pointer; color: #666; transition: .12s; }
.st.active { background: #e6f4ff; color: #1677ff; font-weight: 600; }
.st:hover:not(.active) { background: #f5f5f5; }
.tb-search { display: flex; align-items: center; gap: 4px; margin-left: auto; color: #888; font-size: 12px; }
.tb-search-in { padding: 4px 8px; border: 1px solid #d9d9d9; border-radius: 4px; font-size: 12px; width: 110px; outline: none; }
.tb-search-in:focus { border-color: #1677ff; }
.tb-extra { font-size: 11.5px; color: #aaa; margin-left: 8px; }

.results-table-wrap { flex: 1; overflow: auto; }
.results-table { width: 100%; border-collapse: collapse; font-size: 12.5px; }
.results-table thead th {
  padding: 9px 12px; text-align: left; font-weight: 600; color: #555;
  background: #fafafa; border-bottom: 1.5px solid #eee; white-space: nowrap; font-size: 11.5px;
  position: sticky; top: 0; z-index: 2;
}
.results-table tbody td {
  padding: 8px 12px; border-bottom: 1px solid #f5f5f5; white-space: nowrap; color: #444;
}
.results-table tbody tr:nth-child(even) { background: #fafbfc; }
.results-table tbody tr:hover { background: #f0f7ff; }
.col-cb { width: 38px; text-align: center; }
.col-cb input[type="checkbox"] { accent-color: #1677ff; width: 14px; height: 14px; cursor: pointer; }
.code-col { font-family: 'SF Mono', Monaco, monospace; font-weight: 600; color: #333; }
.name-col { font-weight: 600; color: #1a1a2e; }
.up { color: #cf1322; font-weight: 600; } /* 中国红涨 */
.down { color: #52c41a; font-weight: 600; } /* 中国绿跌 */

.match-tags { display: flex; flex-wrap: wrap; gap: 3px; }
.match-tag {
  font-size: 10.5px; padding: 1px 7px; border-radius: 8px;
  background: #f0f0f0; color: #666; font-weight: 500;
}

/* ====== 动画 ====== */
.slide-down-enter-active { transition: all .25s ease-out; overflow: hidden; }
.slide-down-leave-active { transition: all .2s ease-in; overflow: hidden; }
.slide-down-enter-from { opacity: 0; max-height: 0; }
.slide-down-leave-to { opacity: 0; max-height: 0; }

.fade-fast-enter-active { transition: opacity .2s ease; }
.fade-fast-leave-active { transition: opacity .15s ease; }
.fade-fast-enter-from, .fade-fast-leave-to { opacity: 0; }

.sig-chip-enter-active { transition: all .28s cubic-bezier(.23,1,.32,1); }
.sig-chip-leave-active { transition: all .18s ease-in; }
.sig-chip-enter-from { opacity: 0; transform: scale(.94) translateY(4px); }
.sig-chip-leave-to { opacity: 0; transform: scale(.94) translateY(-4px); }

/* ====== Modal 弹窗 ====== */
.modal-overlay { position: fixed; inset: 0; z-index: 999; background: rgba(0,0,0,.35); display: flex; align-items: center; justify-content: center; }
.modal-box { background: #fff; border-radius: 12px; padding: 24px; width: 360px; max-width: 90vw; box-shadow: 0 12px 40px rgba(0,0,0,.18); }
.modal-title { font-size: 17px; font-weight: 700; margin-bottom: 10px; }
.modal-body { font-size: 14px; color: #666; line-height: 1.6; margin-bottom: 18px; }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; }
.btn-modal-cancel { padding: 7px 18px; border: 1px solid #d9d9d9; border-radius: 6px; background: #fff; font-size: 13px; cursor: pointer; color: #666; }
.btn-modal-cancel:hover { border-color: #aaa; }
.btn-modal-danger { padding: 7px 18px; border: none; border-radius: 6px; background: #cf1322; color: #fff; font-size: 13px; font-weight: 600; cursor: pointer; }
.btn-modal-danger:hover { background: #a8071a; }
</style>
