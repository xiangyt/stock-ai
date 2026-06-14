<template>
  <div class="strategy-builder">

    <!-- 页面顶部标题栏（独立于卡片） -->
    <div class="page-top-bar">
      <div class="ptb-left">
        <button class="back-btn" @click="$emit('goBack')" title="返回列表">‹</button>
        <span class="page-title">{{ editingId ? '编辑策略' : '新建策略' }}</span>
      </div>
      <div class="ptb-right">
        <!-- 策略名称（仅自己的策略可点击编辑） -->
        <template v-if="isOwner && isEditingName">
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
          v-else-if="isOwner"
          class="inline-name-text"
          @click="startEditName"
          :title="'点击编辑名称'"
        >{{ strategyName || '未命名策略' }}</span>
        <span
          v-else
          class="inline-name-text readonly"
        >{{ strategyName || '未命名策略' }}</span>
        <button
          v-if="isOwner"
          class="btn-save-sm"
          @click="saveStrategy"
          :disabled="signals.length === 0"
          title="保存策略"
        >💾 保存</button>
      </div>
    </div>

    <!-- ========== Section 1: AI 输入区 ========== -->
    <section v-if="isOwner" class="sec-input">
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
            <span class="ai-tool-label" @click="scrollToIndicators">🔍 条件选股</span>
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

    <!-- ========== Section 2: 条件选股（指标平铺网格） ========== -->
    <section class="sec-signals">
      <div class="sec-header-row">
        <div class="sec-left">
          <h3 class="sec-title">条件选股</h3>
          <span class="sig-count-tag" v-if="signals.length > 0">{{ signals.length }} 个条件</span>
        </div>
        <div class="sec-right">
          <button class="btn-sec-sm" v-if="isOwner && signals.length > 0" @click="onClearClick">清空全部</button>
          <button class="btn-sec-sm" v-if="isOwner" @click="exportJSON" :disabled="signals.length === 0" title="导出信号">导出</button>
          <label v-if="isOwner" class="btn-sec-sm" title="导入信号">导入
            <input type="file" accept=".json" @change="importJSON" style="display:none" />
          </label>
        </div>
      </div>

      <!-- 分类 + 指标平铺区域（四列横排，仅自己的策略可编辑） -->
      <div class="indicators-flat-area" v-if="isOwner && !indicatorsLoading">
        <template v-for="(inds, cat) in allData" :key="cat">
          <div class="cat-column">
            <!-- 分类标题 -->
            <div class="cat-section-header">{{ catLabels[cat as Category] }}</div>
            <!-- 该分类下所有指标按钮 -->
            <div class="indicator-grid">
              <button
                v-for="ind in inds" :key="ind.id"
                :class="['ind-drop-btn', { expanded: expandedIndicatorID === ind.id }]"
                @click="toggleExpandIndicator(ind.id)"
              >
                {{ ind.name }}
                <span class="drop-arrow">{{ expandedIndicatorID === ind.id ? '▲' : '▾' }}</span>
              </button>
            </div>
          </div>
        </template>
      </div>

      <!-- 指标数据加载中 -->
      <div v-if="indicatorsLoading" class="indicators-loading">
        <span class="loading-spinner"></span>
        正在加载指标数据...
      </div>

      <!-- 展开的指标面板（inline 紧跟在对应位置或统一展示，仅自己的策略可编辑） -->
      <transition name="expand-down">
        <div v-if="isOwner && expandedIndicatorID && expandedInd" class="ind-expand-panel">
          <!-- 面板头部 -->
          <div class="expand-header">
            <span class="expand-ind-name">{{ expandedInd.name }}</span>
            <span class="expand-ind-desc">{{ expandedInd.description }}</span>
            <button class="expand-close-btn" @click="expandedIndicatorID = null">✕</button>
          </div>

          <!-- 内置信号列表（signal_id 第6位='0'，一键添加） -->
          <template v-if="builtinSigs.length > 0">
            <div class="expand-label">内置信号</div>
            <div class="preset-list-compact">
              <button
                v-for="sig in builtinSigs" :key="sig.signal_id"
                class="preset-mini"
                @click="addBuiltinSignal(sig)"
                @mouseenter="hoveredSigID = sig.signal_id; updateTooltipPos($event)"
                @mouseleave="hoveredSigID = ''"
              >
                <span class="pm-name">{{ sig.alias || sig.name }}</span>
              </button>
            </div>
          </template>

          <!-- 自定义信号区（signal_id 第6位='1'，表单配置模式） -->
          <template v-if="customSigs.length > 0">
          <div class="expand-label">自定义信号</div>
          <div class="quick-add-form">
            <div class="qf-row">
            <!-- 步骤1: 选择自定义信号 -->
            <div class="qf-operator">
              <select v-model="customSignalID" class="qf-op-select qf-select-sig">
                <option v-for="sig in customSigs" :key="sig.signal_id" :value="sig.signal_id">{{ sig.name }}</option>
              </select>
            </div>
            <!-- 步骤2: 选择操作符 -->
            <div class="qf-operator">
              <select v-model="customOperator" class="qf-op-select" @change="onOperatorChange">
                <option v-for="op in currentCustomOperators" :key="op.operator" :value="op.operator">{{ op.label }} {{ op.label !== operatorSymbol(op.operator) ? `(${operatorSymbol(op.operator)})` : '' }}</option>
              </select>
            </div>
            <!-- 步骤3: 根据选中操作符动态渲染参数输入框 -->
            <div class="qf-params">
              <template v-for="p in currentOpParams" :key="p.key">
                <!-- 数值型参数 → 标签 + 数字输入框 + 单位 -->
                <div v-if="isNumberLike(p.type)" class="qf-param-field">
                  <span class="qf-param-label">{{ p.label }}</span>
                  <input
                    type="number"
                    v-model.number="paramValues[p.key]"
                    :placeholder="'默认' + p.default"
                    class="qf-input"
                    step="any"
                    :min="p.min" :max="p.max"
                  />
                  <span v-if="p.unit" class="qf-param-unit">{{ p.unit }}</span>
                </div>
                <!-- 单选枚举 → 标签 + 下拉框 -->
                <div v-else-if="p.type === 'select'" class="qf-param-field">
                  <span class="qf-param-label">{{ p.label }}</span>
                  <select
                    v-model="paramValues[p.key]"
                    class="qf-input qf-select"
                  >
                    <option value="">请选择...</option>
                    <option v-for="o in p.options" :key="o.value" :value="o.value">{{ o.label }}</option>
                  </select>
                </div>
                <!-- 多选枚举 → 标签 + checkbox 组 -->
                <div v-else-if="isMultiSelect(p.type)" class="qf-param-field">
                  <span class="qf-param-label">{{ p.label }}</span>
                  <div class="qf-multi-select">
                    <label
                      v-for="o in p.options" :key="o.value"
                      class="qf-checkbox"
                      :class="{ checked: (multiVals[p.key] || []).includes(o.value) }"
                    >
                      <input
                        type="checkbox"
                        :value="o.value"
                        :checked="(multiVals[p.key] || []).includes(o.value)"
                        @change="toggleMultiVal(p.key, o.value)"
                      />
                      {{ o.label }}
                    </label>
                  </div>
                </div>
              </template>
              <span v-if="currentOpParams.length === 0" class="qf-no-params">该操作符无需额外参数</span>
            </div>
            <button
              class="qf-add-btn"
              :disabled="!canQuickAdd"
              @click="addCustomSignal"
            >
              ✅ 添加到策略
            </button>
            </div>
            <div class="qf-sig-desc" v-if="currentCustomSig?.description">{{ currentCustomSig.description }}</div>
          </div>
          </template>

          <!-- 快速添加成功提示 -->
          <transition name="fade-fast">
            <span v-if="addSuccessMsg" class="add-success-msg-inline">{{ addSuccessMsg }}</span>
          </transition>
        </div>
      </transition>

      <!-- 内置信号 fixed tooltip（脱离父容器 overflow 裁剪，放在 transition 外） -->
      <Teleport to="body" :disabled="!hoveredSigID">
        <div v-if="hoveredSigID && hoveredSigDesc" class="builtin-tooltip"
          :style="{ top: tooltipPos.top, left: tooltipPos.left, maxWidth: '380px' }">
          {{ hoveredSigDesc }}
        </div>
      </Teleport>

      <!-- 空状态 -->
      <div v-if="signals.length === 0" class="empty-signals">
        <div class="empty-icon">📭</div>
        <p>还没有信号条件</p>
        <p class="empty-sub">点击上方指标的 ▾ 展开并选择信号条件，或使用 AI 输入框自动生成</p>
      </div>

      <!-- 已添加信号标签行 -->
      <div v-if="signals.length > 0" class="signals-chips-area">
        <transition-group name="sig-chip" tag="div" class="chips-row">
          <div v-for="(s, i) in signals" :key="s.uid"
            class="sig-chip" :class="'chip-' + s.category">
            <span class="chip-bar"></span>
            <span class="chip-name">{{ s.name === s.indicator_name ? s.name : `${s.indicator_name}: ${s.name}` }}</span>
            <span v-if="s.operator !== 'none'" class="chip-op">{{ s.opSym }} {{ s.paramText }}</span>
            <button v-if="isOwner" class="chip-del" @click="removeSignal(i)">✕</button>
          </div>
        </transition-group>
      </div>

      <!-- 底部操作栏 -->
      <div v-if="signals.length > 0" class="sec-footer">
        <div class="logic-toggle">
          <span class="logic-label">逻辑关系：</span>
          <button :class="['logic-btn', { active: logicalOp === 'AND' }]" :disabled="!isOwner" @click="isOwner && (logicalOp = 'AND')">AND</button>
          <button :class="['logic-btn', { active: logicalOp === 'OR' }]" :disabled="!isOwner" @click="isOwner && (logicalOp = 'OR')">OR</button>
        </div>
      </div>
    </section>

    <!-- ========== Section 3: 结果预览表 ========== -->
    <section class="sec-results">
      <div class="results-head">
        <div class="results-left">
          <h3 class="results-title">选出股票 <strong>{{ screenResult ? screenResult.passed.length : 0 }}</strong> / {{ screenResult?.total ?? 0 }}</h3>
          <div class="results-tabs">
            <button :class="['rtab', { active: !screenError }]" @click="screenError = ''">≡ 股票列表</button>
            <button class="rtab dim">⊞ 多股同列</button>
          </div>
        </div>
        <div class="results-right">
          <input type="date" v-model="runDate" class="date-picker" />
          <button class="btn-res-action backtest" @click="onGoBacktest">📊 历史回测</button>
          <button class="btn-res-action run" @click="runFilter" :disabled="isScreening || signals.length === 0">
            {{ isScreening ? '⏳ 筛选中...' : '🔍 运行筛选' }}
          </button>
        </div>
      </div>

      <div class="results-toolbar">
        <button class="tb-tool">＋ 加自选</button>
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
          <input type="text" placeholder="代码/名称" class="tb-search-in" v-model.trim="searchKeyword" />
          <span class="tb-search-icon">🔍</span>
        </div>
      </div>

      <div class="results-table-wrap">
        <table class="results-table">
          <thead>
            <tr>
              <th class="col-cb"><input type="checkbox" :checked="allSelected" :indeterminate.prop="someSelected" @change="toggleAll" /></th>
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
            <!-- 筛选中 -->
            <tr v-if="isScreening">
              <td colspan="11" style="text-align:center; padding:40px 20px; color:#999;">
                <span class="loading-spinner"></span> 正在筛选 {{ screenResult?.total ?? 0 }} 只股票...
              </td>
            </tr>
            <!-- 有结果（含搜索过滤） -->
            <template v-else-if="screenResult && screenResult.passed.length > 0 && paginatedData.length > 0">
              <tr v-for="(stock, idx) in paginatedData" :key="stock.code">
                <td class="col-cb"><input type="checkbox" :checked="selectedRows.has((currentPage - 1) * pageSize + idx)" @change="toggleRow((currentPage - 1) * pageSize + idx)" /></td>
                <td>{{ (currentPage - 1) * pageSize + idx + 1 }}</td>
                <td class="code-col">{{ stock.code }}</td>
                <td class="name-col" @mouseenter="showKLine($event, stock)" @mouseleave="hideKLine">
                  <span class="stock-name-hover" :title="stock.name + ' — 悬浮查看K线图'">{{ stock.name }}</span>
                </td>
                <td>{{ stock.price?.toFixed(2) ?? '-' }}</td>
                <td>-</td>
                <td>-</td>
                <td>-</td>
                <td>-</td>
                <td>-</td>
                <td><span class="match-tag" :title="stock.message">✓ {{ stock.message || '通过' }}</span></td>
              </tr>
            </template>
            <!-- 搜索无匹配 -->
            <tr v-else-if="screenResult && screenResult.passed.length > 0 && paginatedData.length === 0">
              <td colspan="11" style="text-align:center; padding:40px 20px; color:#bbb;">
                🔍 未找到与「{{ searchKeyword }}」匹配的股票
              </td>
            </tr>
            <!-- 无结果 -->
            <tr v-else-if="screenResult && !screenError">
              <td colspan="11" style="text-align:center; padding:40px 20px; color:#bbb;">
                {{ screenResult.total > 0 ? '😔 没有符合条件的股票，请尝试调整条件' : '🔍 运行筛选后显示结果' }}
              </td>
            </tr>
            <!-- 错误 -->
            <tr v-else-if="screenError">
              <td colspan="11" style="text-align:center; padding:30px; color:#cf1322;">
                ⚠️ {{ screenError }}
              </td>
            </tr>
            <!-- 初始状态 -->
            <tr v-else>
              <td colspan="11" style="text-align:center; padding:60px 20px; color:#bbb; font-size:14px;">
                🔍 运行筛选后显示结果
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 分页栏 -->
      <div v-if="screenResult && screenResult.passed.length > 0" class="pagination-bar">
        <div class="pag-left">
          <select class="page-size-select" :value="pageSize" @change="onPageSizeChange">
            <option v-for="sz in pageSizes" :key="sz" :value="sz">{{ sz }} 条/页</option>
          </select>
          <span class="pag-info">共 {{ filteredData.length }} 条</span>
        </div>
        <div class="pag-center">
          <button class="pag-btn" :disabled="currentPage <= 1" @click="prevPage">‹ 上一页</button>
          <template v-for="p in visiblePages" :key="p">
            <span v-if="p === '...'" class="pag-ellipsis">...</span>
            <button v-else :class="['pag-btn', { active: p === currentPage }]" @click="goPage(p)">{{ p }}</button>
          </template>
          <button class="pag-btn" :disabled="currentPage >= totalPage" @click="nextPage">下一页 ›</button>
        </div>
        <div class="pag-right">
          <span>前往第</span>
          <input type="number" class="pag-jump-input" :min="1" :max="totalPage" v-model.number="jumpPageInput" @keyup.enter="goJumpPage" />
          <span>页 / 共 {{ totalPage }} 页</span>
        </div>
      </div>
    </section>

    <!-- 通用确认弹窗 -->
    <teleport to="body">
      <div class="modal-overlay" v-if="showConfirm" @click.self="onConfirmCancel">
        <div class="modal-box">
          <div class="modal-title">{{ confirmTitle }}</div>
          <p class="modal-body">{{ confirmBody }}</p>
          <div class="modal-actions">
            <button class="btn-modal-cancel" @click="onConfirmCancel">取消</button>
            <button class="btn-modal-danger" @click="onConfirmOk">{{ confirmOkText }}</button>
          </div>
        </div>
      </div>
    </teleport>

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
import { reactive, ref, computed, nextTick, onMounted, watch } from 'vue'
import * as indicatorsApi from '../api/indicators'
import KLineTooltip from './KLineTooltip.vue'
import type {
  IndicatorMeta, Category, CompareOperator,
  SignalDef, SignalConfig, SignalOperatorOption, ParamDef, EnumOption,
} from '../api/indicators'
import { categoryLabels as catLabels, operatorSymbols, isCustomSignal } from '../api/indicators'

// ========== Props ==========
interface BuilderProps {
  currentUserId?: number
}
const props = withDefaults(defineProps<BuilderProps>(), { currentUserId: 0 })

// ========== 策略归属 ==========
/** 当前编辑策略的创建者 UID（0 = 新建策略，始终视为自己的） */
const strategyOwnerId = ref(0)
/** 是否是自己的策略 */
const isOwner = computed(() => {
  if (!editingId.value) return true  // 新建策略
  if (props.currentUserId === 0) return true  // 未传 currentUserId，兼容模式
  return strategyOwnerId.value === props.currentUserId
})

interface Sig {
  uid: number
  indicator_id: string       // 指标 ID（5位, 如 "03001"）
  indicator_name: string     // 指标名称（如 "筹码分布"）
  signal_id: string          // 8位数字信号ID（如 "03001001", "04001001"）
  name: string               // 显示名
  category: Category
  operator: CompareOperator
  opSym: string              // 操作符符号 (>)
  opLbl: string              // 操作符中文标签
  params: Record<string, any>
  paramText: string          // 参数可读文本
}

// ========== 指标数据（从后端 API 加载） ==========

/** 全量指标数据，按分类分组 */
const allData = ref<Record<Category, IndicatorMeta[]>>({
  technical: [],
  market: [],
  fundamental: [],
  financial: [],
})
/** 枚举选项映射（从 API 获取，用于 listing_board / industry 等枚举型指标） */
const enumOptions = ref<Record<string, EnumOption[]>>({})
/** 指标数据加载状态 */
const indicatorsLoading = ref(true)
/** 加载指标数据 */
let indicatorsLoadPromise: Promise<void> | null = null

async function loadIndicators() {
  // 正在加载中 → 返回同一个 Promise，避免重复请求
  if (indicatorsLoadPromise) return indicatorsLoadPromise
  // 已加载则直接返回
  if (allData.value.technical.length > 0) return Promise.resolve()
  indicatorsLoading.value = true
  indicatorsLoadPromise = (async () => {
    try {
      const data = await indicatorsApi.fetchIndicators()
      const grouped: Record<string, IndicatorMeta[]> = { technical: [], market: [], fundamental: [], financial: [] }
      for (const ind of data.indicators) {
        const cat = ind.category as string
        if (grouped[cat]) grouped[cat].push(ind)
      }
      for (const cat in grouped) {
        grouped[cat].sort((a, b) => a.id.localeCompare(b.id))
      }
      allData.value = grouped as Record<Category, IndicatorMeta[]>
      enumOptions.value = data.enum_options
    } catch (e) {
      console.error('加载指标数据失败:', e)
    } finally {
      indicatorsLoading.value = false
      indicatorsLoadPromise = null
    }
  })()
  return indicatorsLoadPromise
}

// 组件挂载时加载指标
onMounted(() => { loadIndicators() })

/** 当前展开的指标 ID */
const expandedIndicatorID = ref<string | null>(null)
const expandedInd = computed(() => {
  if (!expandedIndicatorID.value) return null
  for (const cats of Object.values(allData.value)) {
    const found = cats.find(i => i.id === expandedIndicatorID.value)
    if (found) return found
  }
  return null
})

// ============================================================================
//  展开指标的信号拆分（内置 vs 自定义，两种独立交互模型）
// ============================================================================

/** 当前展开指标下的内置信号列表（signal_id 第6位='0'，一键添加模式） */
const builtinSigs = ref<SignalDef[]>([])

/** 当前展开指标下的自定义信号列表（signal_id 第6位='1'，表单配置模式） */
const customSigs = ref<SignalDef[]>([])

/** 自定义表单状态（仅用于自定义信号区） */
const selectedSignalID = ref<string | null>(null)  // 当前选中的内置信号ID（用于高亮）
const customSignalID = ref<string>('')
const customOperator = ref<CompareOperator>('gt')
const paramValues = reactive<Record<string, any>>({})
const multiVals = reactive<Record<string, string[]>>({})

/** 内置信号 fixed tooltip 状态 */
const hoveredSigID = ref<string>('')
const hoveredSigDesc = computed(() => {
  if (!hoveredSigID.value) return ''
  const sig = builtinSigs.value.find(s => s.signal_id === hoveredSigID.value)
  return sig?.description ?? ''
})
const tooltipPos = ref({ top: '0px', left: '0px' })

/** 更新 fixed tooltip 位置 — 左对齐按钮，靠右留边距 */
function updateTooltipPos(e: MouseEvent) {
  const el = e.currentTarget as HTMLElement
  const rect = el.getBoundingClientRect()
  const w = Math.min(380, window.innerWidth - 16)
  // 左边缘对齐按钮左边缘（+4px 偏移），而非居中
  let left = rect.left + 4
  if (left < 8) left = 8
  if (left + w > window.innerWidth - 8) left = window.innerWidth - w - 8
  tooltipPos.value = {
    top: `${Math.round(rect.bottom + 6)}px`,
    left: `${Math.round(left)}px`,
  }
}

/** 监听指标切换：一次性拆分内置/自定义信号 + 初始化表单 */
watch(expandedIndicatorID, (newID) => {
  // 重置所有状态
  builtinSigs.value = []
  customSigs.value = []
  customSignalID.value = ''
  customOperator.value = 'gt'
  clearParams()
  selectedSignalID.value = null

  if (!newID || !expandedInd.value) return

  // 按 signal_id 来源位拆分为两个独立数组
  for (const sig of expandedInd.value.signals) {
    if (!isCustomSignal(sig.signal_id)) {
      // 内置信号（第6位='0'）：只要有 operators 就加入（用于查看/备用）
      // 即使没有 default_config，也记录到 builtinSigs 供后续检查
      builtinSigs.value.push(sig)
    } else if (sig.operators && sig.operators.length > 0) {
      // 自定义信号（第6位='1'）：只要有 operators 就加入（有参数或无参数均可）
      customSigs.value.push(sig)
    } else if (sig.default_config) {
      // 兜底：如果没有 operators 但有 default_config，也加入自定义列表
      customSigs.value.push(sig)
    }
  }

  // 自定义表单默认选中第一个
  if (customSigs.value.length > 0) {
    const first = customSigs.value[0]
    customSignalID.value = first.signal_id
    // operator 和参数由 watch(customSignalID) 通过 default_config 初始化
  }
})

/** 切换自定义信号时：从 operators/default_config 初始化操作符和参数 */
watch(customSignalID, async (newSigId) => {
  if (!newSigId) return
  const sig = currentCustomSig.value
  if (!sig) return

  // 自定义信号必定有 operators
  if (sig.default_config) {
    customOperator.value = sig.default_config.operator as CompareOperator
    // 等待 currentOpParams 随 operator 更新
    await nextTick()
    const cfgParams = sig.default_config.params || {}
    for (const p of currentOpParams.value) {
      if (p.type === 'multi_select' || p.type === 'select_multi') {
        multiVals[p.key] = [...(cfgParams[p.key] || [])]
      } else {
        paramValues[p.key] = cfgParams[p.key] ?? p.default
      }
    }
  } else {
    customOperator.value = sig.operators[0].operator as CompareOperator
    clearParams()
  }
})

// ============================================================================
//  自定义表单计算属性（仅依赖 customSigs / customSignalID / customOperator）
// ============================================================================

/** 当前选中的自定义信号定义 */
const currentCustomSig = computed((): SignalDef | undefined => {
  if (!customSignalID.value || customSigs.value.length === 0) return undefined
  return customSigs.value.find(s => s.signal_id === customSignalID.value)
})

/** 当前自定义信号的可用操作符 */
const currentCustomOperators = computed((): SignalOperatorOption[] => {
  return currentCustomSig.value?.operators || []
})

/** 当前操作符的参数定义 */
const currentOpParams = computed((): ParamDef[] => {
  const sig = currentCustomSig.value
  if (!sig) return []
  const op = sig.operators.find(o => o.operator === customOperator.value)
  return op?.params || []
})

/** 自定义添加按钮是否可用 */
const canQuickAdd = computed(() => {
  if (!customOperator.value) return false
  const params = currentOpParams.value
  for (const p of params) {
    if (!p.required) continue
    // 多选类型：检查 multiVals
    if (p.type === 'multi_select' || p.type === 'select_multi') {
      if (!multiVals[p.key] || multiVals[p.key].length === 0) return false
    } else {
      // 其他类型（数字、单选等）：检查 paramValues
      const val = paramValues[p.key]
      if (val === undefined || val === '') return false
    }
  }
  return true
})

/** 已选信号列表 */
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

/** 筛选日期（YYYY-MM-DD），默认为今天 */
const today = new Date().toISOString().split('T')[0]
const runDate = ref(today)
const showConfirm = ref(false)
const confirmTitle = ref('')
const confirmBody = ref('')
const confirmOkText = ref('确认')
let confirmCallback: (() => void) | null = null
function showConfirmModal(opts: { title: string; body: string; okText?: string; onOk: () => void }) {
  confirmTitle.value = opts.title
  confirmBody.value = opts.body
  confirmOkText.value = opts.okText || '确认'
  confirmCallback = opts.onOk
  showConfirm.value = true
}
function onConfirmOk() {
  showConfirm.value = false
  confirmCallback?.()
  confirmCallback = null
}
function onConfirmCancel() {
  showConfirm.value = false
  confirmCallback = null
}
interface BuilderEmits {
  (e: 'addSignals', signals: Sig[]): void
  (e: 'saved', strategy: { id: number; name: string }): void
  (e: 'goBack'): void
  (e: 'goBacktest', strategyId: number | null): void
}
const emit = defineEmits<BuilderEmits>()
const editingId = ref<number | null>(null) // 后端数字 ID，null = 新建模式

/** 脏标记：是否进行了信号增删操作（未保存） */
const isDirty = ref(false)

/** 标记脏状态 */
function markDirty() { isDirty.value = true }
/** 清除脏状态 */
function clearDirty() { isDirty.value = false }

// 策略名称内联编辑
function startEditName() {
  isEditingName.value = true
  nextTick(() => {
    nameInputRef.value?.focus()
    nameInputRef.value?.select()
  })
}

/** 滚动到条件选股区域 */
function scrollToIndicators() {
  const el = document.querySelector('.sec-signals')
  if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' })
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
        signal_id: s.signal_id,
        operator: s.operator,
        params: s.params,
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
    clearDirty()
  } catch (e) {
    console.error('保存策略失败:', e)
    alert('保存失败: ' + (e as Error).message)
  }
}

/** 判断当前是否有未保存的修改（信号增删操作） */
function hasUnsavedChanges(): boolean {
  return isDirty.value
}

/** 点击历史回测按钮 */
function onGoBacktest() {
  const needsSave = hasUnsavedChanges()
  if (needsSave) {
    showConfirmModal({
      title: '💾 保存提示',
      body: '当前有未保存的策略内容，是否保存后再前往回测？',
      okText: '保存并跳转',
      onOk: async () => {
        await saveStrategy()
        emit('goBacktest', editingId.value ?? null)
      },
    })
  } else {
    emit('goBacktest', editingId.value ?? null)
  }
}

// ========== 方法 ==========

/** 切换指标展开/收起（状态初始化由 watch expandedIndicatorID 统一处理） */
function toggleExpandIndicator(indID: string) {
  if (expandedIndicatorID.value === indID) {
    expandedIndicatorID.value = null
    return
  }
  expandedIndicatorID.value = indID
}

/** 选择信号模板并直接添加（使用默认配置） */
function selectSignalQuick(sig: SignalDef) {
  if (!expandedInd.value || !sig.default_config) return
  selectedSignalID.value = sig.signal_id

  const ind = expandedInd.value
  const cfg = sig.default_config

  // 使用 default_config 中的默认操作符和参数，直接构建 Sig
  const text = formatSignalParamText(cfg, ind)

  const newSig: Sig = {
    uid: ++uidCounter,
    indicator_id: ind.id,
    indicator_name: ind.name,
    signal_id: cfg.signal_id,
    name: sig.alias || sig.name || ind.name,
    category: ind.category,
    operator: cfg.operator,
    opSym: operatorSymbols[cfg.operator] || cfg.operator,
    opLbl: findOpLabel(ind, cfg.operator),
    params: { ...cfg.params },
    paramText: text,
  }
  signals.value.push(newSig)
  markDirty()
  emit('addSignals', [newSig])
}

/** 添加内置信号（一键添加，无需配置） */
function addBuiltinSignal(sig: SignalDef) {
  if (!expandedInd.value) return
  const ind = expandedInd.value

  // 使用 default_config（如果有），否则使用空配置（无参内置信号如 513 战法）
  const cfg: SignalConfig = sig.default_config || {
    signal_id: sig.signal_id,
    operator: 'none',  // 无参信号使用特殊操作符
    params: {},
  }
  const text = formatSignalParamText(cfg, ind)

  const newSig: Sig = {
    uid: ++uidCounter,
    indicator_id: ind.id,
    indicator_name: ind.name,
    signal_id: cfg.signal_id,
    name: sig.alias || sig.name,
    category: ind.category,
    operator: cfg.operator,
    opSym: cfg.operator === 'none' ? '' : (operatorSymbols[cfg.operator] || cfg.operator),
    opLbl: findOpLabel(ind, cfg.operator),
    params: { ...cfg.params },
    paramText: text,
  }
  signals.value.push(newSig)
  markDirty()
  emit('addSignals', [newSig])
}

/** 内置信号的简短描述（从 default_config 提取，枚举值用 label 替代） */
function formatBuiltinDesc(sig: SignalDef): string {
  const cfg = sig.default_config
  if (!cfg) return ''
  if (!sig.operators) return cfg.operator || ''
  const op = sig.operators.find(o => o.operator === cfg.operator)
  const opLabel = op?.label ?? cfg.operator
  const params = cfg.params || {}
  // 枚举型：提取 values 并映射为 label
  if (params.values && Array.isArray(params.values)) {
    const vals = params.values as string[]
    // 从操作符参数定义中找枚举选项
    const enumOpts = op?.params?.find(p =>
      p.type === 'multi_select' || p.type === 'select_multi' || p.type === 'select'
    )?.options
    if (enumOpts && enumOpts.length > 0) {
      const valToLabel = new Map(enumOpts.map(o => [o.value, o.label]))
      const labels = vals.map(v => valToLabel.get(v) || v)
      return `${opLabel} ${labels.join(',')}`
    }
    return `${opLabel} ${vals.join(',')}`
  }
  // 数值型：取 threshold
  if (params.threshold != null) return `${opLabel} ${params.threshold}`
  // range 型
  if (params.min != null && params.max != null) return `${opLabel} ${params.min}~${params.max}`
  return opLabel
}

/** 添加自定义信号（从表单收集操作符+参数） */
function addCustomSignal() {
  if (!expandedInd.value || !currentCustomSig.value) return
  const ind = expandedInd.value
  const sig = currentCustomSig.value
  const op = sig.operators.find(o => o.operator === customOperator.value)
  if (!op) return

  // 收集参数值
  const collected: Record<string, any> = {}
  if (op.params) {
    for (const p of op.params) {
      if (p.type === 'multi_select' || p.type === 'select_multi') { collected[p.key] = [...(multiVals[p.key] || [])] }
      else if (paramValues[p.key] !== undefined) { collected[p.key] = paramValues[p.key] }
      else if (p.default !== undefined) { collected[p.key] = p.default }
    }
  }

  // 构建可读文本
  const text = formatSignalParamText(
    { signal_id: sig.signal_id, operator: customOperator.value, params: collected } as SignalConfig,
    ind,
  )

  const newSig: Sig = {
    uid: ++uidCounter,
    indicator_id: ind.id,
    indicator_name: ind.name,
    signal_id: sig.signal_id,
    name: sig.alias || sig.name || ind.name,
    category: ind.category,
    operator: customOperator.value,
    opSym: operatorSymbols[customOperator.value] || customOperator.value,
    opLbl: op.label || findOpLabel(ind, customOperator.value),
    params: collected,
    paramText: text,
  }
  signals.value.push(newSig)
  markDirty()
  emit('addSignals', [newSig])
  clearParams()
}

function showAddSuccess(msg: string) {
  addSuccessMsg.value = msg
  if (successTimer) clearTimeout(successTimer)
  successTimer = setTimeout(() => { addSuccessMsg.value = '' }, 2500)
}
function clearParams() {
  for (const k of Object.keys(paramValues)) delete paramValues[k]
  for (const k of Object.keys(multiVals)) multiVals[k] = []
}
function onOperatorChange() {
  clearParams()
}
function isNumberLike(t: string): boolean { return ['number', 'range', 'threshold', 'days'].includes(t) }
function isMultiSelect(t: string): boolean { return ['multi_select', 'select_multi'].includes(t) }

/** 切换多选项的选中状态 */
function toggleMultiVal(key: string, value: string) {
  if (!multiVals[key]) multiVals[key] = []
  const idx = multiVals[key].indexOf(value)
  if (idx >= 0) {
    multiVals[key].splice(idx, 1)
  } else {
    multiVals[key].push(value)
  }
}

/** 获取操作符的显示符号 */
function operatorSymbol(op: CompareOperator): string { return operatorSymbols[op] || op }

/** 从信号的操作符列表中找操作符标签 */
function findOpLabel(ind: IndicatorMeta, op: CompareOperator): string {
  // 遍历所有信号的操作符查找标签
  for (const sig of ind.signals) {
    if (!sig.operators) continue
    const found = sig.operators.find(o => o.operator === op)
    if (found) return found.label
  }
  return op
}

/** 将 SignalConfig 格式化为可读参数文本 */
function formatSignalParamText(cfg: SignalConfig, ind: IndicatorMeta): string {
  const params = cfg.params || {}
  // 辅助：从信号定义中查找枚举选项的 value→label 映射
  const findEnumLabels = (sigId: string, key: string): Map<string, string> | null => {
    for (const sig of ind.signals) {
      if (sig.signal_id === sigId) {
        if (!sig.operators) continue
        for (const op of sig.operators) {
          if (!op.params) continue
          for (const p of op.params) {
            if (p.key === key && p.options) return new Map(p.options.map(o => [o.value, o.label]))
          }
        }
      }
    }
    return null
  }

  switch (cfg.operator) {
    case 'gt':   return `${params.threshold ?? ''}${ind.unit}`
    case 'gte':  return `${params.threshold ?? ''}${ind.unit}`
    case 'lt':   return `${params.threshold ?? ''}${ind.unit}`
    case 'lte':  return `${params.threshold ?? ''}${ind.unit}`
    case 'between': case 'not_between':
      return `${params.min ?? ''}~${params.max ?? ''}${ind.unit}`
    case 'eq': {
      const labelMap = findEnumLabels(cfg.signal_id!, 'threshold')
      if (labelMap && params.threshold !== undefined) {
        return labelMap.get(String(params.threshold)) || String(params.threshold)
      }
      return String(params.threshold ?? '')
    }
    case 'neq': {
      const neqLabelMap = findEnumLabels(cfg.signal_id!, 'threshold')
      if (neqLabelMap && params.threshold !== undefined) {
        return neqLabelMap.get(String(params.threshold)) || String(params.threshold)
      }
      return String(params.threshold ?? '')
    }
    case 'in': case 'not_in': {
      const vals = params.values as string[] | undefined
      if (!vals || vals.length === 0) return '{}'
      const labelMap = findEnumLabels(cfg.signal_id!, 'values')
      if (labelMap) {
        return `{${vals.map(v => labelMap.get(v) || v).join(',')}}`
      }
      return `{${vals.join(',')}}`
    }
    case 'custom': {
      const start = params.lookback_start
      const end = params.lookback_end
      // 从信号定义中动态读取参数的 label 和 unit
      let sLabel = '起始天数', eLabel = '结束天数'
      let sUnit = '天前', eUnit = '天前'
      for (const sig of ind.signals) {
        if (sig.signal_id === cfg.signal_id) {
          for (const op of sig.operators) {
            if (!op.params) continue
            for (const p of op.params) {
              if (p.key === 'lookback_start') { if (p.label) sLabel = p.label; if (p.unit) sUnit = p.unit }
              if (p.key === 'lookback_end')   { if (p.label) eLabel = p.label; if (p.unit) eUnit = p.unit }
            }
          }
        }
      }
      return `${sLabel}${start ?? 0}${sUnit}, ${eLabel}${end ?? 0}${eUnit}`
    }
    default:
      return Object.entries(params).map(([k, v]) => `${k}=${v}`).join(', ')
  }
}
function onClearClick() {
  showConfirmModal({
    title: '⚠️ 确认清空',
    body: '确定要清空所有信号条件吗？此操作不可撤销。',
    onOk: () => { signals.value = []; markDirty() },
  })
}

function removeSignal(idx: number) { signals.value.splice(idx, 1); markDirty() }


/** 根据 signal_id 查找信号名称 */
function findSignalName(ind: IndicatorMeta, signalId: string): string {
  const sig = ind.signals.find(s => s.signal_id === signalId)
  return sig ? (sig.alias || sig.name) : signalId
}

/** 从 signal_id 提取 indicator_id（前5位） */
function getIndicatorID(signalId: string): string {
  return signalId.length >= 5 ? signalId.substring(0, 5) : signalId
}

/** 补全单个信号的前端字段 */
function enrichSignal(raw: any): Sig {
  const indId = getIndicatorID(raw.signal_id)
  let ind: IndicatorMeta | null = null
  for (const cat of ['technical', 'market', 'fundamental', 'financial']) {
    ind = allData.value[cat as Category]?.find(i => i.id === indId) || null
    if (ind) break
  }
  if (!ind) {
    return {
      uid: ++uidCounter,
      indicator_id: indId,
      indicator_name: indId,
      signal_id: raw.signal_id,
      name: raw.signal_id,
      category: 'technical',
      operator: raw.operator,
      opSym: operatorSymbols[raw.operator] || raw.operator,
      opLbl: raw.operator,
      params: { ...raw.params },
      paramText: JSON.stringify(raw.params),
    }
  }
  const text = formatSignalParamText(raw, ind)
  return {
    uid: ++uidCounter,
    indicator_id: indId,
    indicator_name: ind.name,
    signal_id: raw.signal_id,
    name: findSignalName(ind, raw.signal_id),
    category: ind.category,
    operator: raw.operator,
    opSym: operatorSymbols[raw.operator] || raw.operator,
    opLbl: findOpLabel(ind, raw.operator),
    params: { ...raw.params },
    paramText: text,
  }
}

function handleAISubmit() { /* TODO: 对接 AI 解析 */ }

function exportJSON() {
  const json = JSON.stringify(
    signals.value.map(s => ({
      signal_id: s.signal_id,
      operator: s.operator,
      params: s.params,
    })),
    null,
    2
  )
  const blob = new Blob([json], { type: 'application/json' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `策略信号_${Date.now()}.json`
  a.click()
  URL.revokeObjectURL(a.href)
}

async function importJSON(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = async () => {
    try {
      const data = JSON.parse(reader.result as string)
      const rawSignals = Array.isArray(data) ? data : (data.conditions || [])
      if (rawSignals.length === 0) {
        alert('文件中没有找到有效的信号数据')
        return
      }
      // 如果当前已有信号，用通用弹窗提示覆盖
      if (signals.value.length > 0) {
        showConfirmModal({
          title: '⚠️ 确认导入',
          body: `导入将会覆盖当前已有的 ${signals.value.length} 个信号条件，是否继续？`,
          okText: '确认导入',
          onOk: () => { doImport(rawSignals) },
        })
        return
      }
      await doImport(rawSignals)
    } catch (err) {
      console.error('[StrategyBuilder] 导入失败:', err)
      alert('导入失败：文件格式不正确')
    } finally {
      ;(e.target as HTMLInputElement).value = ''
    }
  }
  reader.readAsText(file)
}
async function doImport(rawSignals: any[]) {
  await loadIndicators()
  signals.value = []
  uidCounter = 0
  for (const raw of rawSignals) {
    signals.value.push(enrichSignal(raw))
  }
  markDirty()
  console.warn('[StrategyBuilder] 导入成功', signals.value)
}
// ========== 筛选执行 ==========
const isScreening = ref(false)
const screenResult = ref<{ passed: any[]; rejected: any[]; total: number } | null>(null)
const screenError = ref('')

// ========== 表格选择状态 ==========
const selectedRows = ref<Set<number>>(new Set())
const allSelected = computed(() =>
  screenResult.value && screenResult.value.passed.length > 0 && selectedRows.value.size === screenResult.value.passed.length
)
const someSelected = computed(() => selectedRows.value.size > 0 && !allSelected.value)

function toggleAll() {
  if (!screenResult.value) return
  if (allSelected.value) {
    selectedRows.value.clear()
  } else {
    selectedRows.value = new Set(screenResult.value.passed.map((_, i) => i))
  }
}
function toggleRow(idx: number) {
  if (selectedRows.value.has(idx)) {
    selectedRows.value.delete(idx)
  } else {
    selectedRows.value.add(idx)
  }
}

// ========== 前端分页 ==========
const searchKeyword = ref('')
const currentPage = ref(1)
const pageSize = ref(20)
const pageSizes = [10, 20, 50, 100]

// ========== K 线图悬浮 ==========
const klineVisible = ref(false)
const klineStockCode = ref('')
const klineStockName = ref('')
const klineX = ref(0)
const klineY = ref(0)
let klineTimer: ReturnType<typeof setTimeout> | null = null   // 显示延迟计时器
let klineHideTimer: ReturnType<typeof setTimeout> | null = null // 隐藏延迟计时器

function showKLine(e: MouseEvent, stock: any) {
  // 取消待执行的隐藏（鼠标从弹窗移回名称时）
  if (klineHideTimer) { clearTimeout(klineHideTimer); klineHideTimer = null }
  if (klineTimer) clearTimeout(klineTimer)
  klineTimer = setTimeout(() => {
    klineStockCode.value = stock.code
    klineStockName.value = stock.name
    klineX.value = e.clientX
    klineY.value = e.clientY
    klineVisible.value = true
  }, 350) // 延迟显示，避免快速划过时闪烁
}

function hideKLine() {
  // 取消显示计时器
  if (klineTimer) { clearTimeout(klineTimer); klineTimer = null }
  // 延迟 200ms 隐藏，给用户时间从名称移动到弹窗
  if (!klineHideTimer) {
    klineHideTimer = setTimeout(() => {
      klineVisible.value = false
      klineHideTimer = null
    }, 200)
  }
}

/** 弹窗mouseenter时取消隐藏 */
function onKLineEnter() {
  if (klineHideTimer) { clearTimeout(klineHideTimer); klineHideTimer = null }
}

/** 先过滤，后分页 */
const filteredData = computed(() => {
  if (!screenResult.value) return []
  const kw = searchKeyword.value.toLowerCase()
  if (!kw) return screenResult.value.passed
  return screenResult.value.passed.filter((s: any) =>
    (s.code ?? '').toLowerCase().includes(kw) || (s.name ?? '').toLowerCase().includes(kw)
  )
})

const paginatedData = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredData.value.slice(start, start + pageSize.value)
})

const totalPage = computed(() => {
  if (!filteredData.value.length) return 1
  return Math.ceil(filteredData.value.length / pageSize.value)
})

// 页码显示逻辑（省略号）
const visiblePages = computed(() => {
  const total = totalPage.value
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)
  const cur = currentPage.value
  const pages: (number | string)[] = [1]
  if (cur > 3) pages.push('...')
  for (let i = Math.max(2, cur - 1); i <= Math.min(total - 1, cur + 1); i++) pages.push(i)
  if (cur < total - 2) pages.push('...')
  pages.push(total)
  return pages
})

const jumpPageInput = ref(1)

function goPage(p: number) { if (p >= 1 && p <= totalPage.value) currentPage.value = p }
function prevPage() { if (currentPage.value > 1) currentPage.value-- }
function nextPage() { if (currentPage.value < totalPage.value) currentPage.value++ }
function onPageSizeChange(e: Event) {
  pageSize.value = Number((e.target as HTMLSelectElement).value)
  currentPage.value = 1
}
function goJumpPage() { goPage(jumpPageInput.value) }

// 重置分页和选中
watch(totalPage, () => { currentPage.value = 1; selectedRows.value.clear() })
watch(searchKeyword, () => { currentPage.value = 1 })

async function runFilter() {
  if (signals.value.length === 0) { alert('请先添加至少一个信号条件'); return }

  isScreening.value = true
  screenError.value = ''
  screenResult.value = null
  selectedRows.value.clear()
  currentPage.value = 1

  try {
    const res = await indicatorsApi.executeScreen({
      configs: signals.value.map(s => ({
        signal_id: s.signal_id,
        operator: s.operator,
        params: s.params,
      })),
      max_concurrency: 200,
      date: runDate.value,
    })

    screenResult.value = {
      total: res.total,
      passed: res.passed || [],
      rejected: res.rejected || [],
    }
  } catch (e: any) {
    console.error('筛选执行失败:', e)
    screenError.value = e.message || '筛选执行失败'
  } finally {
    isScreening.value = false
  }
}

/** 外部调用：接收 AI 解析的信号 */
function acceptAISignals(aiSignals: any[]) {
  for (const s of aiSignals) { signals.value.push({
    uid: ++uidCounter,
    indicator_id: s.indicatorID || s.indicator_id,
    signal_id: s.signalID || s.signal_id,
    name: s.indicatorName || s.name,
    category: s.category as Category,
    operator: s.operator as CompareOperator,
    opSym: operatorSymbols[s.operator as CompareOperator] || s.operatorSymbol || s.operator,
    opLbl: s.operatorLabel || s.operator,
    params: s.params || {},
    paramText: s.paramSummary || '',
  }) }
  markDirty()
}
/** 从策略列表加载策略到编辑器 */
async function loadStrategyFromOutside(s: { id: string | number; name: string; signals: any[]; logicalOp: 'AND' | 'OR'; uid?: number }) {
  try {
    console.warn('[StrategyBuilder] loadStrategyFromOutside', JSON.stringify(s))
    editingId.value = typeof s.id === 'number' ? s.id : parseInt(s.id)
    strategyOwnerId.value = s.uid ?? 0
    strategyName.value = s.name
    // 确保指标数据已加载
    await loadIndicators()
    // 补全前端字段
    const enriched = (s.signals || []).map(raw => {
      console.warn('[StrategyBuilder] enriching signal', raw)
      if (raw.name && raw.paramText !== undefined && raw.uid) return { ...raw, uid: ++uidCounter }
      return enrichSignal(raw)
    })
    signals.value = enriched
    clearDirty()
    console.warn('[StrategyBuilder] signals loaded', signals.value)
    logicalOp.value = s.logicalOp || 'AND'
  } catch (e) {
    console.error('[StrategyBuilder] 加载策略失败:', e)
  }
}
function resetAllSignals() { editingId.value = null; strategyOwnerId.value = 0; strategyName.value = ''; signals.value = []; logicalOp.value = 'AND'; uidCounter = 0; clearDirty() }

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
.inline-name-text.readonly {
  cursor: default; color: #555;
}
.inline-name-text.readonly:hover { background: none; border-color: transparent; }
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

/* ========== Section 2: 条件选股（指标平铺网格） ========== */
.sec-signals { background: #fff; border: 1px solid #e8e8e8; border-radius: 10px; padding: 16px 20px; }

.sec-header-row {
  display: flex; justify-content: space-between; align-items: center;
  margin-bottom: 14px;
}
.sec-left { display: flex; align-items: center; gap: 10px; }
.sec-title { font-size: 15px; font-weight: 700; color: #1a1a2e; margin: 0; }
.sig-count-tag {
  font-size: 11.5px; padding: 2px 10px; border-radius: 10px;
  background: #fff7e6; color: #d46b08; font-weight: 600;
}
.sec-right { display: flex; gap: 8px; }
.btn-sec-sm { padding: 5px 14px; border: 1px solid #d9d9d9; border-radius: 6px; background: #fff; font-size: 12px; cursor: pointer; color: #666; }
.btn-sec-sm:hover { border-color: #cf1322; color: #cf1322; }

/* ====== 指标平铺网格（四列横排） ====== */
.indicators-loading {
  display: flex; align-items: center; justify-content: center;
  gap: 10px; padding: 40px 20px; font-size: 13.5px; color: #999;
}
.loading-spinner {
  width: 18px; height: 18px; border: 2px solid #e0e0e0;
  border-top-color: #1677ff; border-radius: 50%;
  animation: spin 0.7s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
.indicators-flat-area {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 4px;
}
.cat-column {
  display: flex;
  flex-direction: column;
  border: 1px solid #d0d0d0;
  border-radius: 8px;
  padding: 10px 12px;
  background: #fafafa;
}
.cat-section-header {
  font-size: 12.5px; font-weight: 700; color: #888;
  padding-bottom: 6px; letter-spacing: 0.5px;
}
.indicator-grid {
  display: flex; flex-wrap: wrap; gap: 6px;
}
.ind-drop-btn {
  display: inline-flex; align-items: center; gap: 3px;
  padding: 4px 11px; border: 1px solid #e0e0e0; border-radius: 6px;
  background: #fafafa; cursor: pointer; font-size: 13px; font-weight: 500;
  color: #444; transition: all .12s; white-space: nowrap;
}
.ind-drop-btn:hover { border-color: #1677ff; color: #1677ff; background: #f0f7ff; }
.ind-drop-btn.expanded {
  border-color: #1677ff; color: #1677ff; background: #e6f4ff; font-weight: 600;
}
.drop-arrow { font-size: 9px; opacity: .5; transition: transform .2s; }

/* ====== 展开面板 ====== */
.ind-expand-panel {
  margin: 10px 0 14px; padding: 16px 18px;
  border: 1px solid #d0d0d0; border-radius: 10px;
  background: linear-gradient(to bottom, #f0f9ff, #fff);
}
.expand-header {
  display: flex; align-items: baseline; gap: 8px; margin-bottom: 12px;
}
.expand-ind-name { font-size: 15px; font-weight: 700; color: #1a1a2e; }
.expand-ind-desc { font-size: 12px; color: #999; }
.expand-close-btn {
  margin-left: auto; border: none; background: none; cursor: pointer;
  font-size: 14px; color: #bbb; padding: 2px 6px; border-radius: 4px;
}
.expand-close-btn:hover { background: #f0f0f0; color: #666; }
.expand-label {
  font-size: 11.5px; font-weight: 600; color: #888;
  text-transform: uppercase; letter-spacing: 0.5px; margin: 10px 0 6px;
}

/* 预设信号紧凑列表 */
.preset-list-compact { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 8px; }
.preset-mini {
  display: inline-flex; align-items: center; gap: 4px; padding: 5px 10px;
  border: 1.5px solid #e8e8e8; border-radius: 6px; cursor: pointer;
  background: #fafafa; transition: all .12s; text-align: left;
  font-size: 13px; flex: 0 0 calc(16.66% - 6px); max-width: calc(16.66% - 6px);
  position: relative;
}
.preset-mini:hover { border-color: #91caff; background: #f0f7ff; }
.preset-mini.selected { border-color: #1677ff; background: #e6f4ff; }
/* 内置信号按钮：tooltip 改为 Teleport + position:fixed（见 .builtin-tooltip），此处不再需要伪元素 */
.builtin-tooltip {
  position: fixed; z-index: 9999;
  padding: 8px 12px; background: #fff; color: #333; font-size: 12px; line-height: 1.6;
  border: 1px solid #d0d0d0; border-radius: 8px;
  box-shadow: 0 4px 16px rgba(0,0,0,.14);
  pointer-events: none; white-space: normal; word-break: break-word;
}
.pm-name { font-size: 13px; font-weight: 600; color: #1a1a2e; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.pm-desc { font-size: 11.5px; color: #999; margin-left: 4px; }
.pm-add {
  font-size: 12px; font-weight: 600; color: #389e0d; white-space: nowrap;
}

/* 自定义快速添加表单 */
.quick-add-form {
  display: flex; flex-direction: column; gap: 6px;
  padding: 10px 12px;
  border: 1.5px solid #e8e8e8; border-radius: 8px;
  background: #fafafa;
}
.qf-row {
  display: flex; align-items: center; gap: 8px; flex-wrap: wrap;
}
.qf-operator { display: flex; align-items: center; }
.qf-op-select {
  padding: 5px 10px; border: 1px solid #d9d9d9; border-radius: 6px;
  font-size: 13px; font-weight: 500; color: #333; outline: none; cursor: pointer;
  background: #fff; min-width: 80px;
}
.qf-op-select:focus { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.06); }
.qf-params { display: flex; gap: 8px; flex: 1; align-items: center; flex-wrap: wrap; }
.qf-param-field { display: flex; align-items: center; gap: 4px; }
.qf-param-label { font-size: 12.5px; color: #666; white-space: nowrap; }
.qf-param-unit { font-size: 12px; color: #999; white-space: nowrap; }
.qf-input {
  padding: 5px 10px; border: 1px solid #d9d9d9; border-radius: 6px;
  font-size: 13px; outline: none; color: #333; width: 110px;
  background: #fff;
}
.qf-input:focus { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.08); }
.qf-select { width: 130px; }
.qf-select-sig { min-width: 100px; font-weight: 600; color: #1a1a2e; }
.qf-sig-desc { font-size: 12px; color: #888; line-height: 1.5; }
.qf-no-params { font-size: 12px; color: #aaa; font-style: italic; }
.qf-add-btn {
  padding: 5px 16px; font-size: 13px; font-weight: 600; color: #fff;
  background: linear-gradient(135deg, #1677ff, #0958d9); border: none;
  border-radius: 6px; cursor: pointer; transition: .15s; white-space: nowrap;
}
.qf-add-btn:hover:not(:disabled) { transform: translateY(-1px); box-shadow: 0 2px 8px rgba(22,119,255,.3); }
.qf-add-btn:disabled { background: #d9d9d9; cursor: not-allowed; }
/* 多选 checkbox 组 */
.qf-multi-select {
  display: inline-flex; gap: 4px; flex-wrap: wrap;
  padding: 4px 6px; background: #fafafa; border-radius: 6px;
}
.qf-checkbox {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 4px 10px; border-radius: 14px; font-size: 12.5px;
  cursor: pointer; user-select: none; transition: .15s;
  border: 1px solid #d9d9d9; background: #fff; color: #666;
}
.qf-checkbox:hover { border-color: #1677ff; }
.qf-checkbox.checked {
  background: #e6f4ff; border-color: #1677ff; color: #1677ff; font-weight: 600;
}
.qf-checkbox input[type="checkbox"] { display: none; }
.add-success-msg-inline {
  display: block; text-align: center; font-size: 12.5px; color: #389e0d; font-weight: 600;
  margin-top: 6px;
}

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
.date-picker {
  padding: 5px 10px; border: 1px solid #d9d9d9; border-radius: 5px;
  font-size: 12.5px; color: #333; background: #fff; outline: none; cursor: pointer;
}
.date-picker:focus { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.08); }
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
.btn-res-action.backtest { background: #fff; color: #1677ff; border: 1px solid #1677ff; font-weight: 600; }
.btn-res-action.backtest:hover { background: #f0f5ff; }

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
.tb-search { display: flex; align-items: center; margin-left: auto; position: relative; }
.tb-search-in { padding: 4px 30px 4px 8px; border: 1px solid #d9d9d9; border-radius: 4px; font-size: 12px; width: 120px; outline: none; }
.tb-search-in:focus { border-color: #1677ff; }
.tb-search-icon {
  position: absolute; right: 8px; pointer-events: none;
  font-size: 12.5px; color: #aaa; user-select: none;
}
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
.name-col { font-weight: 600; color: #1a1a2e; cursor: pointer; }
.stock-name-hover {
  border-bottom: 1px dashed #1677ff;
  transition: color .12s;
}
.stock-name-hover:hover {
  color: #1677ff;
}
.up { color: #cf1322; font-weight: 600; } /* 中国红涨 */
.down { color: #52c41a; font-weight: 600; } /* 中国绿跌 */

.match-tags { display: flex; flex-wrap: wrap; gap: 3px; }
.match-tag {
  font-size: 10.5px; padding: 1px 7px; border-radius: 8px;
  background: #f0f0f0; color: #666; font-weight: 500;
}

/* ====== 分页栏 ====== */
.pagination-bar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 20px; border-top: 1px solid #f0f0f0;
  font-size: 12.5px; color: #555;
}
.pag-left, .pag-right { display: flex; align-items: center; gap: 8px; }
.pag-center { display: flex; align-items: center; gap: 4px; }
.page-size-select {
  padding: 3px 6px; border: 1px solid #d9d9d9; border-radius: 4px;
  font-size: 12px; color: #555; outline: none; background: #fff; cursor: pointer;
}
.pag-info { color: #999; }
.pag-btn {
  min-width: 30px; height: 28px; padding: 0 8px; border: 1px solid transparent;
  border-radius: 4px; background: transparent; cursor: pointer;
  font-size: 12.5px; color: #555; transition: all .15s; display: inline-flex;
  align-items: center; justify-content: center;
}
.pag-btn:hover:not(:disabled):not(.active) { background: #f5f5f5; border-color: #d9d9d9; color: #1677ff; }
.pag-btn.active { background: #1677ff; color: #fff; border-color: #1677ff; font-weight: 600; }
.pag-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.pag-ellipsis { color: #bbb; padding: 0 4px; }
.pag-jump-input {
  width: 44px; height: 26px; padding: 0 4px; border: 1px solid #d9d9d9;
  border-radius: 4px; font-size: 12px; text-align: center; outline: none;
}
.pag-jump-input:focus { border-color: #1677ff; }

/* ====== 动画 ====== */
.expand-down-enter-active { transition: all .25s ease-out; }
.expand-down-leave-active { transition: all .2s ease-in; }
.expand-down-enter-from { opacity: 0; max-height: 0; margin-top: 0; padding-top: 0; padding-bottom: 0; }
.expand-down-leave-to { opacity: 0; max-height: 0; margin-top: 0; padding-top: 0; padding-bottom: 0; }

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
