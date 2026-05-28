<template>
  <div class="portfolio-page">
    <!-- ====== 头部 ====== -->
    <header class="page-header">
      <h1>💼 持仓管理</h1>
      <p>管理你的股票持仓组合，跟踪盈亏表现</p>
    </header>

    <!-- ====== 工具栏：建仓 | 统计卡片 | 设置 ====== -->
    <div class="sl-toolbar">
      <div class="toolbar-left">
        <button class="btn-add" @click="openOpenModal()">＋ 建仓</button>
      </div>
      <div class="toolbar-center" v-if="summary">
        <div class="stat-chips">
          <div class="stat-chip">
            <span class="stat-label">持仓中</span>
            <span class="stat-val">{{ summary.holding_count }}</span>
            <span class="stat-unit">只</span>
          </div>
          <div class="stat-chip">
            <span class="stat-label">已清仓</span>
            <span class="stat-val stat-gray">{{ summary.closed_count }}</span>
            <span class="stat-unit">只</span>
          </div>
          <div class="stat-chip">
            <span class="stat-label">总投入</span>
            <span class="stat-val cost-color">¥{{ formatMoney(summary.total_cost) }}</span>
          </div>
          <div class="stat-chip">
            <span class="stat-label">总股数</span>
            <span class="stat-val">{{ summary.total_quantity.toLocaleString() }}</span>
            <span class="stat-unit">股</span>
          </div>
          <div class="stat-chip stat-chip-select">
            <select v-model="statusFilter" @change="loadPositions()" class="filter-select">
              <option value="">全部</option>
              <option value="holding">持有中</option>
              <option value="closed">已清仓</option>
            </select>
          </div>
        </div>
      </div>
      <div class="toolbar-right">
        <button class="btn-icon config-btn" @click="showConfig = true" title="交易设置">⚙️</button>
      </div>
    </div>

    <!-- ====== 空状态 ====== -->
    <div v-if="positions.length === 0 && !loading" class="empty-state-outer">
      <div class="empty-content">
        <div class="empty-icon">📭</div>
        <p>暂无持仓记录</p>
        <button class="btn-create-sm" @click="openOpenModal()">➕ 建立第一个持仓</button>
      </div>
    </div>

    <!-- ====== 持仓表格 ====== -->
    <div v-else class="table-area">
      <table class="portfolio-table">
        <thead>
          <tr>
            <th>股票代码</th>
            <th>股票名称</th>
            <th class="num-col">数量(股)</th>
            <th class="num-col">成本价</th>
            <th class="num-col">总成本</th>
            <th>状态</th>
            <th class="col-actions">操作</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="pos in positions" :key="pos.id">
          <tr
            :class="{ 'row-closed': pos.status === 'closed' }"
          >
            <!-- 基本信息（可点击展开交易记录） -->
            <td class="code-cell" @click="toggleExpand(pos.id)">
              <span class="expand-arrow">{{ expandedId === pos.id ? '▼' : '▶' }}</span>
              {{ pos.stock_code }}
            </td>
            <td class="name-cell" @click="toggleExpand(pos.id)">{{ pos.stock_name || '-' }}</td>
            <td class="num-col" @click="toggleExpand(pos.id)">{{ pos.quantity.toLocaleString() }}</td>
            <td class="num-col" @click="toggleExpand(pos.id)">¥{{ pos.avg_cost.toFixed(2) }}</td>
            <td class="num-col" @click="toggleExpand(pos.id)">¥{{ formatMoney(pos.total_cost) }}</td>
            <td @click="toggleExpand(pos.id)">
              <span :class="['status-badge', pos.status === 'holding' ? 'badge-holding' : 'badge-closed']">
                {{ pos.status === 'holding' ? '持有中' : '已清仓' }}
              </span>
            </td>
            <td class="col-actions" @click.stop>
              <template v-if="pos.status === 'holding'">
                <button class="btn-sm btn-buy" @click="openBuyModal(pos)" title="加仓">加仓</button>
                <button class="btn-sm btn-sell" @click="openSellModal(pos)" title="减仓">减仓</button>
                <button class="btn-sm btn-close" @click="openCloseModal(pos)" title="清仓">清仓</button>
                <button class="btn-sm btn-del" @click="handleDelete(pos)" title="删除">删除</button>
              </template>
              <template v-else>
                <button class="btn-sm btn-del" @click="handleDelete(pos)" title="删除">删除</button>
              </template>
            </td>
          </tr>
          <!-- 展开行：交易记录明细 -->
          <tr v-if="expandedId === pos.id && pos.trades" class="expand-row">
            <td colspan="7">
              <div class="trade-detail">
                <div class="trade-title">交易记录（{{ pos.trades?.length || 0 }}笔）</div>
                <table class="trade-table" v-if="pos.trades && pos.trades.length > 0">
                  <thead>
                    <tr>
                      <th>日期</th>
                      <th>类型</th>
                      <th class="num-col">数量(股)</th>
                      <th class="num-col">价格</th>
                      <th class="num-col">金额</th>
                      <th class="num-col">手续费</th>
                      <th>备注</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="t in pos.trades" :key="t.id"
                        :class="{ 'trade-buy': t.trade_type === 1, 'trade-sell': t.trade_type === 2 }">
                      <td>{{ t.trade_date }}</td>
                      <td>
                        <span :class="['type-tag', t.trade_type === 1 ? 'tag-buy' : 'tag-sell']">
                          {{ t.trade_type_name }}
                        </span>
                      </td>
                      <td class="num-col">{{ t.quantity.toLocaleString() }}</td>
                      <td class="num-col">¥{{ t.price.toFixed(2) }}</td>
                      <td class="num-col">¥{{ formatMoney(t.amount) }}</td>
                      <td class="num-col">¥{{ formatMoney(t.commission) }}</td>
                      <td class="note-cell">{{ t.note || '-' }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </td>
          </tr>
          </template>
        </tbody>
      </table>

      <!-- 分页 -->
      <div class="pagination-area" v-if="listTotal > pageSize">
        <button class="page-btn" :disabled="currentPage <= 1" @click="goPage(currentPage - 1)">‹</button>
        <span class="page-info">{{ currentPage }} / {{ totalPages }}</span>
        <button class="page-btn" :disabled="currentPage >= totalPages" @click="goPage(currentPage + 1)">›</button>
      </div>
    </div>

    <!-- ====== 建仓弹窗 ====== -->
    <div v-if="showOpenModal" class="modal-overlay" @click.self="closeModals()">
      <div class="modal-box">
        <div class="modal-header">
          <h3>📦 建仓</h3>
          <button class="modal-close" @click="closeModals()">✕</button>
        </div>
        <form @submit.prevent="handleSubmitOpen" class="modal-form">
          <div v-if="errorMsg" class="form-error">{{ errorMsg }}</div>
          <label class="form-group">
            <span class="form-label">股票代码 *</span>
            <input v-model="formOpen.stock_code" maxlength="6" placeholder="6位数字，如 000001" class="form-input" required />
          </label>
          <div class="form-row">
            <label class="form-group flex-1">
              <span class="form-label">买入数量(股) *</span>
              <input v-model.number="formOpen.quantity" type="number" min="1" placeholder="100" class="form-input" required />
            </label>
            <label class="form-group flex-1">
              <span class="form-label">成交价格 *</span>
              <input v-model.number="formOpen.price" type="number" step="0.01" min="0.01" placeholder="10.50" class="form-input" required />
            </label>
          </div>
          <label class="form-group">
            <span class="form-label">建仓价格</span>
            <input v-model.number="formOpen.entry_price" type="number" step="0.01" min="0.01" placeholder="记录计划建仓价格，选填" class="form-input" />
            <span class="form-desc">用于记录你的目标建仓价位，方便后续对比实际成交价格</span>
          </label>
          <label class="form-group">
            <span class="form-label">交易日期 *</span>
            <input v-model="formOpen.trade_date" type="date" class="form-input" required />
          </label>
          <label class="form-group">
            <span class="form-label">备注</span>
            <input v-model="formOpen.note" placeholder="选填" class="form-input" />
          </label>
          <div class="modal-actions">
            <button type="button" class="btn-outline" @click="closeModals()">取消</button>
            <button type="submit" class="btn-primary" :disabled="submitting">
              {{ submitting ? '提交中...' : '确认建仓' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- ====== 加仓弹窗 ====== -->
    <div v-if="showBuyModal" class="modal-overlay" @click.self="closeModals()">
      <div class="modal-box">
        <div class="modal-header">
          <h3>📈 加仓 — {{ currentPos?.stock_code }} {{ currentPos?.stock_name }}</h3>
          <button class="modal-close" @click="closeModals()">✕</button>
        </div>
        <form @submit.prevent="handleSubmitBuy" class="modal-form">
          <div v-if="errorMsg" class="form-error">{{ errorMsg }}</div>
          <div class="form-hint">当前持仓: {{ currentPos?.quantity?.toLocaleString() }} 股，成本价 ¥{{ currentPos?.avg_cost?.toFixed(2) }}</div>
          <div class="form-row">
            <label class="form-group flex-1">
              <span class="form-label">加仓数量(股) *</span>
              <input v-model.number="formTrade.quantity" type="number" min="1" class="form-input" required />
            </label>
            <label class="form-group flex-1">
              <span class="form-label">成交价格 *</span>
              <input v-model.number="formTrade.price" type="number" step="0.01" min="0.01" class="form-input" required />
            </label>
          </div>
          <label class="form-group">
            <span class="form-label">交易日期 *</span>
            <input v-model="formTrade.trade_date" type="date" class="form-input" required />
          </label>
          <label class="form-group">
            <span class="form-label">备注</span>
            <input v-model="formTrade.note" placeholder="选填" class="form-input" />
          </label>
          <div class="modal-actions">
            <button type="button" class="btn-outline" @click="closeModals()">取消</button>
            <button type="submit" class="btn-primary btn-buy-action" :disabled="submitting">
              {{ submitting ? '提交中...' : '确认加仓' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- ====== 减仓弹窗 ====== -->
    <div v-if="showSellModal" class="modal-overlay" @click.self="closeModals()">
      <div class="modal-box">
        <div class="modal-header">
          <h3>📉 减仓 — {{ currentPos?.stock_code }} {{ currentPos?.stock_name }}</h3>
          <button class="modal-close" @click="closeModals()">✕</button>
        </div>
        <form @submit.prevent="handleSubmitSell" class="modal-form">
          <div v-if="errorMsg" class="form-error">{{ errorMsg }}</div>
          <div class="form-hint">当前持仓: {{ currentPos?.quantity?.toLocaleString() }} 股（减仓需小于此数）</div>
          <div class="form-row">
            <label class="form-group flex-1">
              <span class="form-label">卖出数量(股) *</span>
              <input v-model.number="formTrade.quantity" type="number" min="1" :max="(currentPos?.quantity||1)-1" class="form-input" required />
            </label>
            <label class="form-group flex-1">
              <span class="form-label">成交价格 *</span>
              <input v-model.number="formTrade.price" type="number" step="0.01" min="0.01" class="form-input" required />
            </label>
          </div>
          <label class="form-group">
            <span class="form-label">交易日期 *</span>
            <input v-model="formTrade.trade_date" type="date" class="form-input" required />
          </label>
          <label class="form-group">
            <span class="form-label">备注</span>
            <input v-model="formTrade.note" placeholder="选填" class="form-input" />
          </label>
          <div class="modal-actions">
            <button type="button" class="btn-outline" @click="closeModals()">取消</button>
            <button type="submit" class="btn-primary btn-sell-action" :disabled="submitting">
              {{ submitting ? '提交中...' : '确认减仓' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- ====== 清仓弹窗 ====== -->
    <div v-if="showCloseModal" class="modal-overlay" @click.self="closeModals()">
      <div class="modal-box modal-small">
        <div class="modal-header">
          <h3>❌ 清仓 — {{ currentPos?.stock_code }} {{ currentPos?.stock_name }}</h3>
          <button class="modal-close" @click="closeModals()">✕</button>
        </div>
        <form @submit.prevent="handleSubmitClose" class="modal-form">
          <div v-if="errorMsg" class="form-error">{{ errorMsg }}</div>
          <div class="form-hint warn">
            将以当前价格卖出全部 {{ currentPos?.quantity?.toLocaleString() }} 股，持仓将标记为「已清仓」
          </div>
          <label class="form-group">
            <span class="form-label">清仓价格 *</span>
            <input v-model.number="formClose.price" type="number" step="0.01" min="0.01" class="form-input" required />
          </label>
          <label class="form-group">
            <span class="form-label">交易日期 *</span>
            <input v-model.number="formClose.trade_date" type="date" class="form-input" required />
          </label>
          <label class="form-group">
            <span class="form-label">备注</span>
            <input v-model="formClose.note" placeholder="选填" class="form-input" />
          </label>
          <div class="modal-actions">
            <button type="button" class="btn-outline" @click="closeModals()">取消</button>
            <button type="submit" class="btn-danger" :disabled="submitting">
              {{ submitting ? '提交中...' : '确认清仓' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- ====== 交易配置弹窗 ====== -->
    <div v-if="showConfig" class="modal-overlay" @click.self="showConfig = false">
      <div class="modal-box modal-small">
        <div class="modal-header">
          <h3>⚙️ 交易设置</h3>
          <button class="modal-close" @click="showConfig = false">✕</button>
        </div>
        <form @submit.prevent="handleSubmitConfig" class="modal-form">
          <div v-if="errorMsg" class="form-error">{{ errorMsg }}</div>
          <label class="form-group">
            <span class="form-label">手续费率 (万分之x)</span>
            <input v-model.number="configForm.commission_rate" type="number" step="0.1" min="0.1" max="10" class="form-input" required />
            <span class="form-desc">例如：输入 2.5 表示万分之2.5（默认券商标准费率）</span>
          </label>
          <label class="form-group">
            <span class="form-label">最低手续费（免五）</span>
            <div class="switch-row">
              <label class="switch-wrap">
                <input type="checkbox" v-model="configForm.min_commission" />
                <span class="switch-slider"></span>
              </label>
              <span class="switch-text">{{ configForm.min_commission ? '不免五（最低收费5元）' : '免五（无最低收费）' }}</span>
            </div>
          </label>
          <div class="modal-actions">
            <button type="button" class="btn-outline" @click="showConfig = false">取消</button>
            <button type="submit" class="btn-primary" :disabled="configSubmitting">保存</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from 'vue'
import * as portfolioApi from '../api/portfolio'
import type { PositionDetail, TradeConfig, PositionListResp } from '../api/portfolio'

// ========== 数据状态 ==========

const positions = ref<PositionDetail[]>([])
const summary = ref<PositionListResp['summary'] | null>(null)
const loading = ref(false)
const statusFilter = ref('')
const currentPage = ref(1)
const pageSize = ref(20)
const listTotal = ref(0)
const expandedId = ref<number | null>(null)

// 弹窗控制
const showOpenModal = ref(false)
const showBuyModal = ref(false)
const showSellModal = ref(false)
const showCloseModal = ref(false)
const showConfig = ref(false)
const submitting = ref(false)
const configSubmitting = ref(false)
const errorMsg = ref('')

// 当前操作中的持仓
const currentPos = ref<PositionDetail | null>(null)

// 表单数据
const formOpen = reactive({
  stock_code: '',
  quantity: undefined as number | undefined,
  price: undefined as number | undefined,
  entry_price: undefined as number | undefined,
  trade_date: todayStr(),
  note: '',
})

const formTrade = reactive({
  quantity: undefined as number | undefined,
  price: undefined as number | undefined,
  trade_date: todayStr(),
  note: '',
})

const formClose = reactive({
  price: undefined as number | undefined,
  trade_date: todayStr(),
  note: '',
})

const configForm = reactive<Partial<TradeConfig>>({
  commission_rate: 2.5,
  min_commission: true,
})

// ========== 计算属性 ==========

const totalPages = computed(() => Math.ceil(listTotal.value / pageSize.value))

// ========== 工具函数 ==========

function todayStr(): string {
  return new Date().toISOString().slice(0, 10)
}

function formatMoney(v: number): string {
  if (!v || isNaN(v)) return '0.00'
  return v.toFixed(2).replace(/\B(?=(\d{3})+(?!\d))/g, ',')
}

// ========== 加载数据 ==========

async function loadPositions() {
  loading.value = true
  try {
    const resp = await portfolioApi.fetchPositions(statusFilter.value, currentPage.value, pageSize.value)
    positions.value = resp.list ?? []
    listTotal.value = resp.total ?? 0
    if (resp.summary) {
      summary.value = resp.summary as PositionListResp['summary']
    }
    // 默认收起展开行
    expandedId.value = null
  } catch (e) {
    console.error('加载持仓列表失败:', e)
    positions.value = []
  } finally {
    loading.value = false
  }
}

async function loadConfig() {
  try {
    const cfg = await portfolioApi.fetchTradeConfig()
    Object.assign(configForm, cfg)
  } catch (e) {
    console.error('加载交易配置失败:', e)
  }
}

// 展开/折叠交易记录
async function toggleExpand(id: number) {
  if (expandedId.value === id) {
    expandedId.value = null
    return
  }
  // 如果还没有 trades 数据，去拉详情
  const pos = positions.value.find(p => p.id === id)
  if (pos && !pos.trades) {
    try {
      const detail = await portfolioApi.fetchPositionById(id)
      const idx = positions.value.findIndex(p => p.id === id)
      if (idx !== -1) positions.value[idx] = detail
    } catch (e) { /* ignore */ }
  }
  expandedId.value = id
}

// ========== 分页 ==========

function goPage(p: number) {
  currentPage.value = p
  loadPositions()
}

// ========== 弹窗操作 ==========

function resetForms() {
  Object.assign(formOpen, { stock_code: '', quantity: undefined, price: undefined, entry_price: undefined, trade_date: todayStr(), note: '' })
  Object.assign(formTrade, { quantity: undefined, price: undefined, trade_date: todayStr(), note: '' })
  Object.assign(formClose, { price: undefined, trade_date: todayStr(), note: '' })
  submitting.value = false
  errorMsg.value = ''
  currentPos.value = null
}

function closeModals() {
  showOpenModal.value = false
  showBuyModal.value = false
  showSellModal.value = false
  showCloseModal.value = false
  resetForms()
}

function openOpenModal() {
  resetForms()
  showOpenModal.value = true
}

function openBuyModal(pos: PositionDetail) {
  resetForms()
  currentPos.value = pos
  showBuyModal.value = true
}

function openSellModal(pos: PositionDetail) {
  resetForms()
  currentPos.value = pos
  showSellModal.value = true
}

function openCloseModal(pos: PositionDetail) {
  resetForms()
  currentPos.value = pos
  showCloseModal.value = true
}

// ========== 提交操作 ==========

async function handleSubmitOpen() {
  if (submitting.value) return
  submitting.value = true
  try {
    await portfolioApi.openPosition({ ...formOpen } as portfolioApi.OpenPositionPayload)
    closeModals()
    await loadPositions()
  } catch (e: any) {
    errorMsg.value = e.message || '建仓失败'
  } finally {
    submitting.value = false
  }
}

async function handleSubmitBuy() {
  if (submitting.value || !currentPos.value) return
  submitting.value = true
  try {
    await portfolioApi.buyMore(currentPos.value!.id, { ...formTrade } as portfolioApi.TradePayload)
    closeModals()
    await loadPositions()
  } catch (e: any) {
    errorMsg.value = e.message || '加仓失败'
  } finally {
    submitting.value = false
  }
}

async function handleSubmitSell() {
  if (submitting.value || !currentPos.value) return
  submitting.value = true
  try {
    await portfolioApi.sellPartial(currentPos.value!.id, { ...formTrade } as portfolioApi.TradePayload)
    closeModals()
    await loadPositions()
  } catch (e: any) {
    errorMsg.value = e.message || '减仓失败'
  } finally {
    submitting.value = false
  }
}

async function handleSubmitClose() {
  if (submitting.value || !currentPos.value) return
  submitting.value = true
  try {
    await portfolioApi.closePosition(
      currentPos.value!.id,
      formClose.price!,
      formClose.trade_date,
      formClose.note,
    )
    closeModals()
    await loadPositions()
  } catch (e: any) {
    errorMsg.value = e.message || '清仓失败'
  } finally {
    submitting.value = false
  }
}

async function handleDelete(pos: PositionDetail) {
  if (!confirm(`确定删除 ${pos.stock_code} 的持仓记录吗？`)) return
  try {
    await portfolioApi.deletePosition(pos.id)
    await loadPositions()
  } catch (e: any) {
    errorMsg.value = e.message || '删除失败'
  }
}

async function handleSubmitConfig() {
  if (configSubmitting.value) return
  configSubmitting.value = true
  try {
    await portfolioApi.updateTradeConfig(configForm as TradeConfig)
    showConfig.value = false
  } catch (e: any) {
    errorMsg.value = e.message || '保存失败'
  } finally {
    configSubmitting.value = false
  }
}

// ========== 生命周期 ==========

onMounted(() => {
  loadPositions()
  loadConfig()
})
</script>

<style scoped>
/* ====== 页面布局 ====== */
.portfolio-page {
  max-width: 100%;
}

/* ====== 头部（使用全局 .page-header 样式）====== */

/* ====== 工具栏（与策略列表 sl-toolbar 统一）====== */
.sl-toolbar {
  display: flex; justify-content: space-between; align-items: center;
  margin-bottom: 14px; flex-wrap: wrap; gap: 8px;
}
.toolbar-right { display: flex; align-items: center; gap: 8px; }

/* ====== 按钮（与策略列表 btn-add 统一）====== */
.btn-add {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 8px 18px;
  background: #1677ff; color: #fff; border: none; border-radius: 8px;
  font-size: 14px; font-weight: 600; cursor: pointer; transition: all .15s;
}
.btn-add:hover { background: #0958d9; transform: translateY(-1px); box-shadow: 0 2px 8px rgba(22,119,255,.25); }

.config-btn {
  width: 36px; height: 36px;
  border-radius: 8px;
  border: 1px solid #d9d9d9;
  background: #fff;
  cursor: pointer;
  font-size: 16px;
  transition: all .15s;
  display: flex;
  align-items: center;
  justify-content: center;
}
.config-btn:hover { background: #f5f5f5; border-color: #1677ff; }

/* ====== 统计芯片（嵌在 toolbar 中间）====== */
.toolbar-center {
  flex: 1;
  display: flex;
  justify-content: center;
  min-width: 0;
}
.stat-chips {
  display: inline-flex;
  align-items: stretch;
  background: #f5f7fa;
  border-radius: 8px;
  height: 36px;
  overflow: hidden;
}
.stat-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 0 16px;
  font-size: 13px;
  white-space: nowrap;
  border-right: 1px solid #e0e4e8;
}
.stat-chip:last-child { border-right: none; }
.stat-chip .stat-label { font-size: 12px; color: #888; }
.stat-chip .stat-val { font-size: 16px; font-weight: 700; color: #1a1a2e; }
.stat-chip .stat-val.stat-gray { color: #aaa; }
.stat-chip .cost-color { color: #e74c3c; }
.stat-chip .stat-unit { font-size: 11px; color: #bbb; }

.stat-chip-select {
  padding: 0 8px;
  display: flex;
  align-items: center;
}
.stat-chip-select .filter-select {
  padding: 4px 8px;
  border: none;
  font-size: 12px;
  outline: none;
  color: #555;
  background: transparent;
  cursor: pointer;
  appearance: none;
  -webkit-appearance: none;
  -moz-appearance: none;
  text-align: center;
  text-align-last: center;
}
.stat-chip-select .filter-select:focus { outline: none; background: transparent; }
.stat-chip-select .filter-select option { background: #fff; color: #333; }

/* ====== 按钮 ====== */
.btn-primary {
  padding: 8px 18px;
  background: #1677ff;
  color: #fff;
  border: none;
  border-radius: 6px;
  font-size: 14px;
  cursor: pointer;
  transition: all .12s;
}
.btn-primary:hover { background: #0958d9; }
.btn-primary:disabled { opacity: 0.55; cursor: not-allowed; }
.btn-outline {
  padding: 8px 18px;
  background: #fff;
  color: #555;
  border: 1px solid #ddd;
  border-radius: 6px;
  font-size: 14px;
  cursor: pointer;
}
.btn-outline:hover { border-color: #1677ff; color: #1677ff; }
.btn-danger {
  padding: 8px 18px;
  background: #cf1322;
  color: #fff;
  border: none;
  border-radius: 6px;
  font-size: 14px;
  cursor: pointer;
}
.btn-danger:hover { background: #a8071a; }
.btn-create-sm {
  padding: 8px 24px; font-size: 13px; font-weight: 600;
  color: #fff; background: #1677ff; border: 1px solid #1677ff;
  border-radius: 5px; cursor: pointer; transition: .15s;
}
.btn-create-sm:hover { background: #0958d9; }
.btn-icon { cursor: pointer; }
.btn-sm {
  padding: 4px 10px;
  border: 1px solid #ddd;
  border-radius: 4px;
  background: #fff;
  font-size: 12px;
  cursor: pointer;
  margin-right: 4px;
  transition: all .1s;
}
.btn-buy { border-color: #52c41a; color: #389e0d; }
.btn-buy:hover { background: #f6ffed; }
.btn-sell { border-color: #ff7a45; color: #d46b08; }
.btn-sell:hover { background: #fff7e6; }
.btn-close { border-color: #ff4d4f; color: #cf1322; }
.btn-close:hover { background: #fff2f0; }
.btn-del { border-color: #ffa39e; color: #cf1322; background: #fff1f0; }
.btn-del:hover { background: #ffccc7; }

.btn-buy-action { background: #52c41a !important; }
.btn-buy-action:hover { background: #389e0d !important; }
.btn-sell-action { background: #fa8c16 !important; }
.btn-sell-action:hover { background: #d46b08 !important; }

/* ====== 表格（与策略列表统一）====== */
.table-area {
  background: #fff;
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 1px 4px rgba(0,0,0,.06);
}
.portfolio-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
.portfolio-table thead th {
  padding: 10px 14px;
  text-align: center;
  font-weight: 600;
  color: #666;
  background: #fafafa;
  border-bottom: 1px solid #eee;
  white-space: nowrap;
  font-size: 13px;
}
.portfolio-table tbody tr {
  border-bottom: 1px solid #f3f3f3;
  transition: background .1s;
}
.portfolio-table tbody tr:hover { background: #f9fbff; }
.portfolio-table tbody td {
  padding: 10px 14px;
  vertical-align: middle;
  color: #333;
  font-size: 13.5px;
  text-align: center;
}
.row-closed td { opacity: 0.55; }
.num-col { text-align: center; }
.col-actions { text-align: center; white-space: nowrap; }

.code-cell { font-family: 'SF Mono', Monaco, Consolas, monospace; font-weight: 600; letter-spacing: 0.5px; cursor: pointer; user-select: none; }
.name-cell { font-weight: 500; cursor: pointer; }
.expand-arrow { display: inline-block; width: 16px; color: #bbb; font-size: 10px; margin-right: 4px; }

/* 状态标签 */
.status-badge {
  display: inline-block;
  padding: 2px 10px;
  border-radius: 10px;
  font-size: 12px;
  font-weight: 600;
}
.badge-holding { background: #e6f4ff; color: #1677ff; }
.badge-closed { background: #f5f5f5; color: #999; }

/* ====== 展开行（交易记录）====== */
.expand-row td {
  padding: 0;
  background: #fafbfc;
  border-bottom: 2px solid #e8eaed;
}
.trade-detail { padding: 14px 20px; }
.trade-title { font-size: 13px; font-weight: 600; color: #666; margin-bottom: 8px; }
.trade-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12.5px;
}
.trade-table th {
  background: #f0f2f5;
  padding: 7px 10px;
  text-align: left;
  font-weight: 500;
  color: #777;
  border-bottom: 1px solid #e8e8e8;
  font-size: 12px;
}
.trade-table td {
  padding: 7px 10px;
  border-bottom: 1px solid #f0f0f0;
  color: #444;
}
.trade-table tr.trade-buy td { background: rgba(82,196,26,.03); }
.trade-table tr.trade-sell td { background: rgba(255,122,69,.04); }

.type-tag {
  display: inline-block;
  padding: 1px 8px;
  border-radius: 4px;
  font-size: 11.5px;
  font-weight: 600;
}
.tag-buy { background: #f6ffed; color: #389e0d; border: 1px solid #b7eb8f; }
.tag-sell { background: #fff7e6; color: #d46b08; border: 1px solid #ffd591; }

.note-cell { color: #999; font-size: 12px; max-width: 150px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

/* ====== 空状态（与策略列表统一）====== */
.empty-state-outer {
  text-align: center; padding: 80px 20px;
  background: #fff; border: 1px solid #e8e8e8; border-radius: 10px;
}
.empty-content { text-align: center; }
.empty-icon { font-size: 48px; display: block; margin-bottom: 12px; }
.empty-content p { font-size: 14px; color: #999; margin-bottom: 12px; }

/* ====== 分页 ====== */
.pagination-area {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 12px;
  padding: 14px;
  border-top: 1px solid #f0f0f0;
}
.page-btn {
  width: 30px; height: 30px;
  border: 1px solid #ddd;
  border-radius: 6px;
  background: #fff;
  cursor: pointer;
  font-size: 16px;
  line-height: 28px;
  text-align: center;
  transition: all .1s;
}
.page-btn:hover:not(:disabled) { border-color: #1677ff; color: #1677ff; }
.page-btn:disabled { opacity: 0.35; cursor: not-allowed; }
.page-info { font-size: 13px; color: #666; }

/* ====== 弹窗 ====== */
.modal-overlay {
  position: fixed; inset: 0; z-index: 999;
  background: rgba(0,0,0,.35);
  display: flex; align-items: center; justify-content: center;
  animation: fade-in .15s ease-out;
}
@keyframes fade-in { from { opacity: 0; } to { opacity: 1; } }

.modal-box {
  background: #fff;
  border-radius: 16px;
  width: 480px;
  max-width: 90vw;
  max-height: 85vh;
  overflow-y: auto;
  box-shadow: 0 12px 40px rgba(0,0,0,.18);
  animation: slide-up .2s ease-out;
}
.modal-small { width: 400px; }
@keyframes slide-up { from { transform: translateY(16px); opacity: 0; } to { transform: translateY(0); opacity: 1; } }

.modal-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 18px 22px 0;
  margin-bottom: 18px;
}
.modal-header h3 { font-size: 17px; font-weight: 700; }
.modal-close {
  width: 28px; height: 28px; border: none; background: none;
  border-radius: 6px; font-size: 16px; cursor: pointer; color: #999;
  display: flex; align-items: center; justify-content: center;
}
.modal-close:hover { background: #f5f5f5; color: #333; }

.modal-form { padding: 0 22px 22px; }
.form-group { display: block; margin-bottom: 14px; }
.form-label { display: block; font-size: 13px; font-weight: 600; color: #555; margin-bottom: 5px; }
.form-input {
  width: 100%; padding: 9px 12px;
  border: 1px solid #ddd; border-radius: 6px;
  font-size: 14px; outline: none;
  transition: border-color .12s;
}
.form-input:focus { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.1); }
.form-desc { display: block; font-size: 12px; color: #aaa; margin-top: 4px; }
.form-hint {
  font-size: 12.5px; color: #888;
  background: #f6f8fa; padding: 8px 12px; border-radius: 6px;
  margin-bottom: 14px;
}
.form-hint.warn { color: #d46b08; background: #fff7e6; }
.form-error {
  color: #cf1322; background: #fff2f0;
  border: 1px solid #ffccc7; border-radius: 6px;
  padding: 8px 12px; font-size: 13px;
  margin-bottom: 14px;
}

.form-row { display: flex; gap: 12px; }
.flex-1 { flex: 1; }

.modal-actions {
  display: flex; justify-content: flex-end; gap: 8px;
  margin-top: 18px; padding-top: 14px; border-top: 1px solid #f0f0f0;
}

/* 开关样式 */
.switch-row { display: flex; align-items: center; gap: 10px; }
.switch-wrap { position: relative; display: inline-block; width: 42px; height: 24px; }
.switch-wrap input { opacity: 0; width: 0; height: 0; }
.switch-slider {
  position: absolute; inset: 0;
  background: #ccc; border-radius: 24px;
  cursor: pointer; transition: .2s;
}
.switch-slider::before {
  content: ''; position: absolute;
  left: 3px; top: 3px;
  width: 18px; height: 18px;
  background: #fff; border-radius: 50%;
  transition: .2s;
}
.switch-wrap input:checked + .switch-slider { background: #1677ff; }
.switch-wrap input:checked + .switch-slider::before { transform: translateX(18px); }
.switch-text { font-size: 13px; color: #666; }
</style>
