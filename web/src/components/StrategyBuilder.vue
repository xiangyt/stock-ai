<template>
  <div class="strategy-builder">
    <!-- ====== 左侧：添加信号面板（三级选择器）====== -->
    <section class="add-panel">
      <h2 class="panel-title">➕ 添加信号条件</h2>

      <!-- Step 1: 选择分类 -->
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

      <!-- Step 2: 选择指标 -->
      <div class="step-block" v-if="state.category">
        <div class="step-label">
          <span class="step-num">②</span> 指标
          <span class="step-count">{{ indicatorsInCat.length }} 个可选</span>
        </div>
        <div class="indicator-scroll">
          <button
            v-for="ind in indicatorsInCat"
            :key="ind.id"
            :class="['ind-btn', { selected: state.indicator?.id === ind.id, 'has-presets': ind.presets.length > 0 }]"
            @click="selectIndicator(ind)"
          >
            <span class="ind-name">{{ ind.name }}</span>
            <span class="ind-meta">
              <span class="type-badge" :class="ind.valueType">{{ valueTypeLabels[ind.valueType] }}</span>
              <span v-if="ind.presets.length > 0" class="preset-count">{{ ind.presets.length }}个模板</span>
              <span v-else class="preset-count dim">自定义</span>
            </span>
          </button>
        </div>
      </div>

      <!-- Step 3: 选择信号模板 ★ 新增中间层 -->
      <div class="step-block" v-if="state.indicator">
        <div class="step-label">
          <span class="step-num">③</span> 信号类型
          <span class="step-hint" v-if="state.indicator!.presets.length > 0">
            该指标有 {{ state.indicator!.presets.length }} 种预设信号
          </span>
          <span class="step-hint dim" v-else>无预设模板，将使用自由组合模式</span>
        </div>

        <!-- 有预设信号时：展示信号模板卡片 -->
        <template v-if="state.indicator!.presets.length > 0">
          <div class="preset-grid">
            <button
              v-for="p in state.indicator!.presets"
              :key="p.id"
              :class="['preset-card', { selected: state.preset?.id === p.id }]"
              @click="selectPreset(p)"
            >
              <div class="preset-name">{{ p.name }}</div>
              <div class="preset-desc">{{ p.description }}</div>
              <div class="preset-default-op">
                默认操作符：<code>{{ getOperatorSymbol(p.defaultOperator) }} {{ getOperatorLabel(p.defaultOperator) }}</code>
              </div>
            </button>
          </div>
          <!-- "自定义模式"入口 -->
          <button
            :class="['custom-mode-btn', { active: state.customMode }]"
            @click="enterCustomMode"
          >
            ⚙️ 自定义模式 — 手动选操作符和参数
          </button>
        </template>

        <!-- 无预设时：直接进入自定义 -->
        <div v-else class="no-preset-hint">
          <p>该指标暂无预设模板，请手动配置操作符和参数</p>
          <button class="btn-small" @click="enterCustomMode">开始自定义 →</button>
        </div>
      </div>

      <!-- Step 4: 选择/确认 操作符 -->
      <div class="step-block" v-if="showOperatorStep">
        <div class="step-label">
          <span class="step-num">④</span> 操作符
          <span class="step-hint" v-if="!state.customMode && state.preset">
            当前：{{ getOperatorSymbol(state.preset.defaultOperator!) }}
            {{ getOperatorLabel(state.preset.defaultOperator!) }}
            （可切换）
          </span>
          <span class="step-hint" v-else>从该指标支持的操作符中选择</span>
        </div>
        <div class="op-list">
          <button
            v-for="op in availableOperators"
            :key="op.operator"
            :class="['op-btn', { selected: state.selectedOp?.operator === op.operator }]"
            @click="selectOperator(op)"
          >
            <span class="op-sym">{{ op.symbol }}</span>
            <span class="op-lbl">{{ op.label }}</span>
            <span class="op-ex">{{ op.example }}</span>
          </button>
        </div>
      </div>

      <!-- Step 5: 参数输入 -->
      <div class="step-block params-block" v-if="state.selectedOp && state.selectedOp!.params.length > 0">
        <div class="step-label"><span class="step-num">⑤</span> 参数设置</div>
        <div class="params-grid">
          <div v-for="param in state.selectedOp!.params" :key="param.key" class="param-item">
            <label class="param-label">
              {{ param.label }}
              <span v-if="param.unit" class="param-unit">({{ param.unit }})</span>
              <span v-if="param.required" class="req">*</span>
            </label>
            <!-- 数值/区间/天数 -->
            <input
              v-if="isNumberLike(param.type)"
              type="number"
              v-model.number="paramValues[param.key]"
              :placeholder="param.placeholder || `默认: ${param.default}`"
              :min="param.min ?? undefined"
              :max="param.max ?? undefined"
              :step="param.step || (param.type === 'days' ? 1 : 0.01)"
              class="param-input"
            />
            <!-- 枚举单选 -->
            <select v-else-if="param.type === 'select'" v-model="paramValues[param.key]" class="param-input">
              <option value="">请选择...</option>
              <option v-for="o in getEnumOpts(state.indicator!.id)" :key="o.value" :value="o.value">{{ o.label }}</option>
            </select>
            <!-- 枚举多选 -->
            <div v-else-if="param.type === 'multiSelect'" class="multi-check">
              <label v-for="o in getEnumOpts(state.indicator!.id)" :key="o.value" class="chk-item">
                <input type="checkbox" :value="o.value" v-model="multiVals[param.key]" />
                {{ o.label }}
              </label>
            </div>
            <p v-if="param.description" class="param-tip">{{ param.description }}</p>
          </div>
        </div>
      </div>

      <!-- 添加按钮 -->
      <button
        class="add-btn"
        :disabled="!canAdd"
        @click="addSignal"
      >
        ✅ 添加到策略
      </button>
    </section>

    <!-- ====== 右侧：策略预览面板 ====== -->
    <section class="strategy-panel">
      <h2 class="panel-title">
        📋 当前策略
        <span class="sig-count">{{ signals.length }} 条信号</span>
      </h2>

      <!-- 空状态 -->
      <div v-if="signals.length === 0" class="empty">
        <div class="empty-icon">📭</div>
        <p>还没有信号条件</p>
        <p class="empty-sub">从左侧依次选择：分类 → 指标 → 信号模板 → 操作符 → 参数</p>
      </div>

      <!-- 信号列表 -->
      <transition-group name="sig-list" tag="ul" v-else class="sig-list">
        <li v-for="(s, i) in signals" :key="s.uid" class="sig-card">
          <div class="sig-top">
            <span class="sig-idx">#{{ i + 1 }}</span>
            <span class="sig-cat" :class="s.category">{{ catLabel(s.category) }}</span>
            <button class="sig-rm" @click="removeSignal(i)" title="移除">✕</button>
          </div>
          <div class="sig-body">
            <strong>{{ s.name }}</strong>
            <code class="sig-op">{{ s.opSym }} {{ s.paramText }}</code>
          </div>
          <pre class="sig-json">{{ sigJSON(s) }}</pre>
        </li>
      </transition-group>

      <!-- 底部操作栏 -->
      <div v-if="signals.length > 0" class="strategy-bar">
        <div class="logic-row">
          <label>逻辑关系：</label>
          <select v-model="logicalOp" class="sel-op">
            <option value="AND">AND（全部满足）</option>
            <option value="OR">OR（任一满足）</option>
          </select>
        </div>
        <div class="bar-btns">
          <button class="btn-sec" @click="clearAll">清空全部</button>
          <button class="btn-pri" @click="exportJSON">导出 JSON</button>
        </div>
      </div>

      <!-- 完整策略 JSON -->
      <div v-if="signals.length > 0" class="full-json-wrap">
        <details open>
          <summary>完整策略 JSON</summary>
          <pre class="full-json">{{ fullStrategyJSON() }}</pre>
        </details>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, computed, watch } from 'vue'
import {
  getAllIndicators, categoryLabels, valueTypeLabels,
  enumOptions as enumOptsRaw, enumOptions as eo,
  Category, ValueType,
  type IndicatorWithPresets, type PresetSignal, type OperatorOption,
  type ParamDef, CompareOperator,
} from '../mock/indicators'

// ========== 状态 ==========

const allData = computed(() => getAllIndicators())

const state = reactive({
  category: null as Category | null,
  indicator: null as IndicatorWithPresets | null,
  preset: null as PresetSignal | null,
  customMode: false,       // 是否在"自定义模式"
  selectedOp: null as OperatorOption | null,
})

const paramValues = reactive<Record<string, any>>({})
const multiVals = reactive<Record<string, string[]>>({})

// 信号列表
interface Sig {
  uid: number
  id: string; name: string; category: Category
  operator: CompareOperator; opSym: string; opLbl: string
  params: Record<string, any>; paramText: string
}
const signals = ref<Sig[]>([])
let uidCounter = 0
const logicalOp = ref<'AND' | 'OR'>('AND')

// emit
const emit = defineEmits<{
  addSignals: [signals: Sig[]]
}>()

// ========== 计算属性 ==========

const indicatorsInCat = computed(() => allData.value[state.category!] || [])

/** 是否显示操作符选择步骤 */
const showOperatorStep = computed(() => {
  if (!state.indicator) return false
  // 选了 preset 或 进入 customMode 都要显示
  return !!(state.preset || state.customMode)
})

/** 当前可用操作符列表 */
const availableOperators = computed<OperatorOption[]>(() => {
  if (!state.indicator) return []
  // 自由模式下返回全部操作符；preset 模式下也返回全部（允许覆盖默认）
  return state.indicator.operators || []
})

/** 是否可以添加信号 */
const canAdd = computed(() => {
  if (!state.indicator || !state.selectedOp) return false
  for (const p of state.selectedOp.params) {
    if (p.required) {
      if (p.type === 'multiSelect') {
        if (!multiVals[p.key] || multiVals[p.key].length === 0) return false
      } else if (paramValues[p.key] === undefined || paramValues[p.key] === '') {
        return false
      }
    }
  }
  return true
})

// ========== 方法 ==========

function selectCategory(cat: Category) {
  state.category = cat
  resetFrom(1)
}

function selectIndicator(ind: IndicatorWithPresets) {
  state.indicator = ind
  state.preset = null
  state.customMode = false
  state.selectedOp = null
  clearParams()
}

function selectPreset(p: PresetSignal) {
  state.preset = p
  state.customMode = false
  clearParams()

  // 自动选中默认操作符 + 填充默认参数
  const defaultOp = findOpByComparator(p.defaultOperator)
  if (defaultOp) {
    state.selectedOp = defaultOp
  }
  // 填入默认参数值
  for (const [k, v] of Object.entries(p.defaultParams)) {
    if (Array.isArray(v)) {
      multiVals[k] = [...v]
    } else {
      paramValues[k] = v
    }
  }
}

function enterCustomMode() {
  state.customMode = true
  state.preset = null
  state.selectedOp = null
  clearParams()
}

function selectOperator(op: OperatorOption) {
  state.selectedOp = op
  clearParams()
  // 填入默认值
  for (const p of op.params) {
    if (p.type === 'multiSelect') {
      multiVals[p.key] = []
    } else if (p.default !== undefined) {
      paramValues[p.key] = p.default
    }
  }
}

function resetFrom(step: number) {
  if (step <= 2) { state.indicator = null }
  if (step <= 3) { state.preset = null; state.customMode = false }
  if (step <= 4) { state.selectedOp = null }
  clearParams()
}

function clearParams() {
  for (const k of Object.keys(paramValues)) delete paramValues[k]
  for (const k of Object.keys(multiVals)) multiVals[k] = []
}

function isNumberLike(t: string): boolean {
  return ['number', 'range', 'threshold', 'days'].includes(t)
}

function getEnumOpts(indID: string): { value: string; label: string }[] {
  if (indID === 'listing_board') {
    const vs = eo.listing_board || []; const ls = eo.listing_board_labels || vs
    return vs.map((v, i) => ({ value: v, label: ls[i] || v }))
  }
  if (indID === 'industry') {
    return (eo.industry || []).map((v: string) => ({ value: v, label: v }))
  }
  return []
}

function findOpByComparator(cmp: CompareOperator): OperatorOption | undefined {
  return state.indicator?.operators.find(o => o.operator === cmp)
}

function getOperatorSymbol(cmp: CompareOperator): string {
  return findOpByComparator(cmp)?.symbol ?? cmp
}
function getOperatorLabel(cmp: CompareOperator): string {
  return findOpByComparator(cmp)?.label ?? ''
}

function catLabel(c: Category): string { return categoryLabels[c] }

function addSignal() {
  if (!state.indicator || !state.selectedOp) return

  const ind = state.indicator
  const op = state.selectedOp

  // 收集参数
  const collected: Record<string, any> = {}
  for (const p of op.params) {
    if (p.type === 'multiSelect') {
      collected[p.key] = [...(multiVals[p.key] || [])]
    } else if (paramValues[p.key] !== undefined) {
      collected[p.key] = paramValues[p.key]
    } else if (p.default !== undefined) {
      collected[p.key] = p.default
    }
  }

  // 参数摘要
  let text = ''
  switch (op.operator) {
    case CompareOperator.GT:
    case CompareOperator.GTE:
    case CompareOperator.LT:
    case CompareOperator.LTE:
      text = `${collected.value_number}${ind.unit}`
      break
    case CompareOperator.Between:
    case CompareOperator.NotBetween:
      text = `[${collected.min_value}, ${collected.max_value}]${ind.unit}`
      break
    case CompareOperator.In:
    case CompareOperator.NotIn:
      text = `{${collected.value_list?.join(', ')}}`
      break
    default:
      text = Object.entries(collected).map(([k, v]) => `${k}=${v}`).join(', ')
  }

  signals.value.push({
    uid: ++uidCounter,
    id: ind.id,
    name: ind.name,
    category: ind.category,
    operator: op.operator,
    opSym: op.symbol,
    opLbl: op.label,
    params: collected,
    paramText: text,
  })

  emit('addSignals', [signals.value[signals.value.length - 1]])

  // 重置到步骤3（保留分类和指标，方便连续添加同类信号）
  state.preset = null
  state.customMode = false
  state.selectedOp = null
  clearParams()
}

function removeSignal(idx: number) { signals.value.splice(idx, 1) }

function clearAll() {
  if (confirm('确定清空所有信号？')) signals.value = []
}

function sigJSON(s: Sig): string {
  return JSON.stringify({
    id: `custom_${s.id}_${s.operator}_${Date.now()}`,
    indicator_id: s.id,
    name: s.name,
    operator: s.operator,
    ...s.params,
  }, null, 2)
}

function fullStrategyJSON(): string {
  return JSON.stringify({
    name: '我的自定义策略',
    logical_op: logicalOp.value,
    conditions: signals.value.map(s => ({
      indicator_id: s.id,
      indicator_name: s.name,
      operator: s.operator,
      operator_label: s.opLbl,
      params: s.params,
    })),
    created_at: new Date().toISOString(),
  }, null, 2)
}

function exportJSON() {
  const blob = new Blob([fullStrategyJSON()], { type: 'application/json' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `strategy_${Date.now()}.json`
  a.click()
  URL.revokeObjectURL(a.href)
}

/** 外部调用：接受 AI 解析结果批量添加 */
function acceptAISignals(aiSignals: any[]) {
  for (const s of aiSignals) {
    signals.value.push({
      uid: ++uidCounter,
      id: s.indicatorID,
      name: s.indicatorName,
      category: s.category as Category,
      operator: s.operator as CompareOperator,
      opSym: s.operatorSymbol,
      opLbl: s.operatorLabel,
      params: s.params,
      paramText: s.paramSummary,
    })
  }
}
defineExpose({ acceptAISignals })
</script>

<style scoped>
.strategy-builder {
  display: grid;
  grid-template-columns: 420px 1fr;
  gap: 20px;
  align-items: start;
}

/* ===== 面板通用 ===== */
.panel-title {
  font-size: 15px;
  font-weight: 700;
  margin: 0 0 14px;
  padding-bottom: 10px;
  border-bottom: 1px solid #f0f0f0;
  display: flex;
  align-items: center;
  gap: 8px;
}
.sig-count { font-size: 12px; color: #999; font-weight: 400; }

/* ===== 左侧：添加面板 ===== */
.add-panel {
  background: #fff;
  border: 1px solid #e8e8e8;
  border-radius: 12px;
  padding: 18px;
}
.step-block { margin-bottom: 16px; }
.step-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12.5px;
  font-weight: 600;
  color: #444;
  margin-bottom: 8px;
}
.step-num {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px; height: 20px;
  border-radius: 50%;
  background: #1677ff;
  color: #fff;
  font-size: 11.5px;
  font-weight: 700;
}
.step-count { margin-left: auto; font-weight: 400; color: #aaa; font-size: 11px; }
.step-hint { margin-left: auto; font-weight: 400; color: #888; font-size: 11px; }
.step-hint.dim { color: #bbb; }

/* 分类 Tab */
.cat-tabs { display: flex; gap: 5px; flex-wrap: wrap; }
.cat-tab {
  padding: 5px 13px; border: 1px solid #d9d9d9; border-radius: 16px;
  background: #fafafa; cursor: pointer; font-size: 12.5px; transition: .15s;
}
.cat-tab:hover { border-color: #aaa; }
.cat-tab.active { background: #1677ff; color: #fff; border-color: #1677ff; }

/* 指标列表 */
.indicator-scroll {
  display: flex; flex-direction: column; gap: 4px;
  max-height: 220px; overflow-y: auto; padding-right: 4px;
}
.indicator-scroll::-webkit-scrollbar { width: 4px; }
.indicator-scroll::-webkit-scrollbar-thumb { background: #ddd; border-radius: 3px; }
.ind-btn {
  display: flex; justify-content: space-between; align-items: center;
  padding: 9px 12px; border: 1.5px solid #eee; border-radius: 8px;
  background: #fafafa; cursor: pointer; transition: .15s; text-align: left;
}
.ind-btn:hover { border-color: #bbb; background: #f5f5f5; }
.ind-btn.selected { border-color: #1677ff; background: #e6f4ff; }
.ind-btn.has-presets.selected { box-shadow: 0 0 0 2px rgba(22,119,255,.1); }
.ind-name { font-size: 13px; font-weight: 600; }
.ind-meta { display: flex; gap: 6px; align-items: center; }
.type-badge {
  font-size: 10.5px; padding: 1px 7px; border-radius: 8px; background: #eee; color: #666;
}
.type-badge.number { background: #e6fffb; color: #08979c; }
.type-badge.bool   { background: #fff7e6; color: #fa8c16; }
.type-badge.enum   { background: #f9f0ff; color: #722ed1; }
.type-badge.series { background: #fff1f0; color: #cf1322; }
.preset-count { font-size: 10.5px; color: #1677ff; font-weight: 500; }
.preset-count.dim { color: #ccc; }

/* ★ 信号模板网格 */
.preset-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 6px; }
.preset-card {
  display: flex; flex-direction: column; gap: 3px;
  padding: 10px 12px; border: 1.5px solid #e8e8e8; border-radius: 8px;
  background: #fcfcfc; cursor: pointer; transition: .15s; text-align: left;
}
.preset-card:hover { border-color: #bbb; background: #f9f9f9; }
.preset-card.selected { border-color: #1677ff; background: #e6f4ff; }
.preset-name { font-size: 13px; font-weight: 700; color: #1a1a2e; }
.preset-desc { font-size: 11.5px; color: #777; line-height: 1.35; }
.preset-default-op { font-size: 11px; color: #999; margin-top: 2px; }
.preset-default-op code {
  padding: 1px 5px; background: #f0f0f0; border-radius: 3px; font-size: 10.5px;
}

/* 自定义模式入口 */
.custom-mode-btn {
  width: 100%; margin-top: 6px; padding: 8px;
  border: 1.5px dashed #d9d9d9; border-radius: 8px;
  background: transparent; cursor: pointer; font-size: 12.5px;
  color: #666; transition: .15s;
}
.custom-mode-btn:hover { border-color: #1677ff; color: #1677ff; }
.custom-mode-btn.active { border-style: solid; border-color: #1677ff; background: #e6f4ff; color: #1677ff; font-weight: 600; }

.no-preset-hint {
  text-align: center; padding: 18px; background: #fafafa; border-radius: 8px;
  border: 1px dashed #ddd;
}
.no-preset-hint p { font-size: 13px; color: #888; margin-bottom: 8px; }
.btn-small { padding: 5px 14px; border: 1px solid #1677ff; border-radius: 6px; background: #fff; color: #1677ff; cursor: pointer; font-size: 12px; }

/* 操作符列表 */
.op-list { display: flex; flex-direction: column; gap: 5px; }
.op-btn {
  display: flex; align-items: center; gap: 10px; padding: 9px 12px;
  border: 1.5px solid #eee; border-radius: 8px; cursor: pointer;
  background: #fafafa; transition: .15s; text-align: left;
}
.op-btn:hover { border-color: #aaa; background: #f5f5f5; }
.op-btn.selected { border-color: #1677ff; background: #e6f4ff; }
.op-sym { font-family: monospace; font-size: 17px; font-weight: 700; width: 32px; text-align: center; }
.op-lbl { font-size: 13px; font-weight: 600; }
.op-ex { font-size: 11.5px; color: #999; margin-left: auto; }

/* 参数表单 */
.params-block { background: #f9f9f9; padding: 12px; border-radius: 8px; border: 1px solid #eee; }
.params-grid { display: flex; flex-direction: column; gap: 10px; }
.param-item { display: flex; flex-direction: column; gap: 3px; }
.param-label { font-size: 12px; font-weight: 500; color: #555; }
.param-unit { color: #999; font-weight: 400; }
.req { color: #cf1322; }
.param-input {
  padding: 7px 10px; border: 1px solid #d9d9d9; border-radius: 6px;
  font-size: 13px; outline: none; transition: border-color .15s;
}
.param-input:focus { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.08); }
.param-tip { font-size: 11px; color: #aaa; margin: 0; }
.multi-check { display: flex; flex-wrap: wrap; gap: 5px; }
.chk-item {
  display: flex; align-items: center; gap: 4px; font-size: 12px; cursor: pointer;
  padding: 3px 8px; border: 1px solid #ddd; border-radius: 12px; background: #fff;
}
.chk-item input[type="checkbox"] { accent-color: #1677ff; }

/* 添加按钮 */
.add-btn {
  width: 100%; padding: 11px; font-size: 14px; font-weight: 700;
  color: #fff; background: linear-gradient(135deg, #1677ff, #0958d9);
  border: none; border-radius: 8px; cursor: pointer; transition: .2s; margin-top: 4px;
}
.add-btn:hover:not(:disabled) { transform: translateY(-1px); box-shadow: 0 4px 12px rgba(22,119,255,.35); }
.add-btn:disabled { background: #d9d9d9; cursor: not-allowed; }

/* ===== 右侧：策略面板 ===== */
.strategy-panel {
  background: #fff; border: 1px solid #e8e8e8; border-radius: 12px; padding: 18px;
}
.empty { text-align: center; padding: 48px 24px; color: #bbb; }
.empty-icon { font-size: 44px; }
.empty p { margin: 4px 0; font-size: 13.5px; }
.empty-sub { font-size: 12px !important; color: #ccc; }

/* 信号列表 */
.sig-list { list-style: none; padding: 0; margin: 0; }
.sig-card {
  background: #fafbfc; border: 1px solid #e8e8e8; border-radius: 8px;
  padding: 11px 13px; margin-bottom: 7px; transition: .15s;
}
.sig-card:hover { border-color: #ccc; }
.sig-top { display: flex; align-items: center; gap: 8px; margin-bottom: 5px; }
.sig-idx { font-size: 11.5px; font-weight: 700; color: #999; }
.sig-cat {
  font-size: 10.5px; padding: 1px 8px; border-radius: 10px; background: #f0f0f0; color: #666;
}
.sig-cat.technical { background: #e6fffb; color: #08979c; }
.sig-cat.market     { background: #e6f7ff; color: #0958d9; }
.sig-cat.fundamental{ background: #fff7e6; color: #d46b08; }
.sig-cat.financial  { background: #fcffe6; color: #7cb305; }
.sig-rm { margin-left: auto; background: none; border: none; color: #bbb; cursor: pointer; font-size: 15px; line-height: 1; padding: 2px 5px; border-radius: 4px; }
.sig-rm:hover { background: #fff1f0; color: #cf1322; }
.sig-body { font-size: 13.5px; }
.sig-body strong { color: #1a1a2e; }
.sig-op {
  display: inline-block; margin-left: 7px; padding: 2px 8px;
  background: #f0f0f0; border-radius: 4px; font-size: 12.5px; color: #555; font-family: monospace;
}
.sig-json {
  margin: 7px 0 0; padding: 7px 9px; background: #1a1a2e; color: #a6e22e;
  border-radius: 6px; font-size: 10.5px; overflow-x: auto; white-space: pre-wrap; word-break: break-all;
}

/* 底部操作栏 */
.strategy-bar {
  display: flex; justify-content: space-between; align-items: center;
  margin-top: 14px; padding-top: 12px; border-top: 1px solid #eee; flex-wrap: wrap; gap: 10px;
}
.logic-row { display: flex; align-items: center; gap: 8px; font-size: 12.5px; color: #555; }
.sel-op { padding: 5px 10px; border: 1px solid #d9d9d9; border-radius: 6px; font-size: 12.5px; outline: none; }
.bar-btns { display: flex; gap: 7px; }
.btn-sec, .btn-pri {
  padding: 5px 15px; border-radius: 6px; font-size: 12.5px; font-weight: 500; cursor: pointer; border: 1px solid #d9d9d9; background: #fff; transition: .15s;
}
.btn-sec:hover { border-color: #1677ff; color: #1677ff; }
.btn-pri { background: #1677ff; color: #fff; border-color: #1677ff; }
.btn-pri:hover { background: #0958d9; }

.full-json-wrap { margin-top: 12px; }
.full-json-wrap summary { cursor: pointer; font-size: 12.5px; color: #666; font-weight: 500; user-select: none; }
.full-json {
  margin: 8px 0 0; padding: 11px; background: #1e1e2e; color: #cdd6f4;
  border-radius: 8px; font-size: 11.5px; overflow-x: auto; white-space: pre-wrap; word-break: break-all; line-height: 1.5;
}

/* 动画 */
.sig-list-enter-active { transition: all .25s ease-out; }
.sig-list-leave-active { transition: all .2s ease-in; }
.sig-list-enter-from { opacity: 0; transform: translateX(20px); }
.sig-list-leave-to { opacity: 0; transform: translateX(-20px); }

/* 响应式 */
@media (max-width: 900px) {
  .strategy-builder { grid-template-columns: 1fr; }
  .indicator-scroll { max-height: 180px; }
  .preset-grid { grid-template-columns: 1fr; }
}
</style>
