<template>
  <div class="sub-mgmt-page">
    <header class="page-header">
      <div class="header-meta">
        <h1>🔔 策略订阅</h1>
        <p>订阅策略后自动执行选股并推送结果到指定机器人</p>
      </div>
      <div class="header-actions">
        <div class="sl-search">
          <input
            v-model="searchQuery"
            type="text"
            placeholder="搜索订阅名称"
            class="search-input"
            @keyup.enter="onSearch"
          />
          <button class="search-btn" @click="onSearch">🔍</button>
        </div>
        <button class="btn-add" @click="openCreateModal">+ 新建订阅</button>
      </div>
    </header>

    <!-- 加载中 -->
    <div v-if="loading" class="loading-state">加载订阅列表...</div>

    <!-- 订阅表格 -->
    <table v-else-if="filteredSubs.length > 0" class="sub-table">
      <thead>
        <tr>
          <th>名称</th>
          <th>关联策略</th>
          <th>监控范围</th>
          <th>执行频率</th>
          <th>交易时段</th>
          <th>推送机器人</th>
          <th>状态</th>
          <th>上次执行</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="sub in filteredSubs" :key="sub.id" :class="{ 'disabled-row': !sub.is_active }">
          <td class="name-cell">{{ sub.name }}</td>
          <td>
            <span class="strategy-tag">{{ sub.strategy_name || '-' }}</span>
          </td>
          <td>
            <span :class="'scope-tag scope-' + sub.scope">
              {{ ScopeLabels[sub.scope as MonitorScope] || sub.scope }}
            </span>
          </td>
          <td>
            <span v-if="sub.preset_type === 'custom'" class="cron-tag">{{ sub.cron_expr || '-' }}</span>
            <span v-else class="preset-tag">{{ PresetTypeLabels[sub.preset_type as PresetType] || sub.preset_type }}</span>
          </td>
          <td>{{ sub.trading_hours_only ? '是' : '否' }}</td>
          <td>
            <div v-if="sub.bots && sub.bots.length > 0" class="bot-tags">
              <span v-for="bot in sub.bots" :key="bot.id" class="bot-tag">
                <span class="bot-channel" :class="'channel-' + bot.channel">{{ ChannelLabels[bot.channel as ChannelType] || bot.channel }}</span>
                <span class="bot-name">{{ bot.name }}</span>
              </span>
            </div>
            <span v-else>-</span>
          </td>
          <td>
            <span :class="'status-tag status-' + (sub.is_active ? 'on' : 'off')">
              {{ sub.is_active ? '运行中' : '已停用' }}
            </span>
          </td>
          <td class="time-cell">{{ formatTime(sub.last_run_at) }}</td>
          <td class="actions-cell">
            <button
              class="btn-sm btn-info"
              :disabled="!sub.is_active || runningId === sub.id"
              @click="onTriggerRun(sub)"
              title="手动执行"
            >{{ runningId === sub.id ? '执行中...' : '执行' }}</button>
            <button class="btn-sm btn-ok" @click="openEditModal(sub)" title="编辑">编辑</button>
            <button
              v-if="sub.is_active"
              class="btn-sm btn-warn"
              @click="onToggleActive(sub, false)"
              title="停用"
            >停用</button>
            <button
              v-else
              class="btn-sm btn-success"
              @click="onToggleActive(sub, true)"
              title="启用"
            >启用</button>
            <button class="btn-sm btn-danger" @click="onDelete(sub)" title="删除">删除</button>
          </td>
        </tr>
      </tbody>
    </table>

    <!-- 分页 -->
    <div v-if="filteredSubs.length > 0 && total > pageSize" class="pag-bar">
      <span class="pag-info">共 {{ total }} 条</span>
      <button class="pag-btn" :disabled="page <= 1" @click="page--; loadSubs()">‹ 上一页</button>
      <span class="pag-current">{{ page }} / {{ totalPages }}</span>
      <button class="pag-btn" :disabled="page >= totalPages" @click="page++; loadSubs()">下一页 ›</button>
    </div>

    <p v-if="!loading && filteredSubs.length === 0" class="empty-hint">
      {{ subs.length === 0 ? '暂无订阅，点击上方「新建订阅」开始配置' : '没有匹配的订阅' }}
    </p>

    <!-- ====== 创建/编辑弹窗 ====== -->
    <teleport to="body">
      <div v-if="showFormModal" class="modal-overlay" @click.self="closeFormModal">
        <div class="modal-box form-modal">
          <h3>{{ isEditing ? '编辑订阅' : '新建订阅' }}</h3>

          <div class="form-group">
            <label>订阅名称 <span class="required">*</span></label>
            <input v-model="form.name" type="text" placeholder="给订阅起个名字" maxlength="50" />
          </div>

          <div class="form-group">
            <label>关联策略 <span class="required">*</span></label>
            <SearchSingleSelect
              :model-value="form.strategy_id || ''"
              :options="strategySelectOptions"
              placeholder="请选择策略"
              searchable
              @update:model-value="form.strategy_id = $event"
            />
          </div>

          <div class="form-group">
            <label>监控范围</label>
            <SearchSingleSelect
              :model-value="form.scope"
              :options="scopeOptions"
              placeholder="请选择"
              @update:model-value="form.scope = $event"
            />
          </div>

          <div v-if="form.scope === 'custom'" class="form-group">
            <label>自选股票代码 <span class="required">*</span></label>
            <textarea
              v-model="customStocksText"
              placeholder="输入 6 位股票代码，用逗号分隔，如：000001,600036,300750"
              rows="3"
            />
            <p class="field-hint">仅支持 6 位纯数字代码，逗号分隔</p>
          </div>

          <div class="form-group">
            <label>执行频率 <span class="required">*</span></label>
            <SearchSingleSelect
              :model-value="form.preset_type || ''"
              :options="presetTypeOptions"
              placeholder="请选择"
              @update:model-value="form.preset_type = $event"
            />
          </div>

          <div v-if="form.preset_type === 'custom'" class="form-group">
            <label>Cron 表达式 <span class="required">*</span></label>
            <input v-model="form.cron_expr" type="text" placeholder="如: */5 9-15 * * 1-5" />
            <p class="field-hint">标准 5 段或 6 段（含秒）Cron 表达式</p>
          </div>

          <div class="checkbox-group">
            <span class="checkbox-label">
              <input type="checkbox" v-model="form.trading_hours_only" />
              <span>仅在交易时段执行</span>
            </span>
          </div>

          <div class="form-group">
            <label>推送机器人</label>
            <SearchMultiSelect
              :model-value="form.bot_ids"
              :options="botSelectOptions"
              placeholder="搜索机器人..."
              :max-count="5"
              @update:model-value="form.bot_ids = $event"
            >
              <template #option="{ option }">
                <span class="bot-channel" :class="'channel-' + option.channel">{{ ChannelLabels[option.channel as ChannelType] || option.channel }}</span>
                <span class="ms-option-label">{{ option.label }}</span>
              </template>
            </SearchMultiSelect>
            <p class="field-hint">最多关联 5 个机器人，请先在「机器人配置」中启用</p>
          </div>

          <p v-if="formError" class="error-msg">{{ formError }}</p>

          <div class="modal-actions">
            <button class="btn-modal-cancel" @click="closeFormModal">取消</button>
            <button
              class="btn-login-sm"
              :disabled="submitting || !isFormValid"
              @click="confirmForm"
            >
              {{ submitting ? '提交中...' : (isEditing ? '保存修改' : '创建') }}
            </button>
          </div>
        </div>
      </div>
    </teleport>

    <!-- ====== 确认弹窗 ====== -->
    <teleport to="body">
      <div v-if="showConfirmModal" class="modal-overlay" @click.self="showConfirmModal = false">
        <div class="modal-box confirm-modal">
          <h3>{{ confirmTitle }}</h3>
          <p class="confirm-msg">{{ confirmMessage }}</p>
          <div v-if="confirmError" class="error-msg">{{ confirmError }}</div>
          <div class="modal-actions">
            <button class="btn-modal-cancel" @click="closeConfirmModal">{{ confirmLoading ? '关闭' : '取消' }}</button>
            <button
              class="btn-login-sm"
              :class="{ 'btn-danger-sm': confirmIsDanger }"
              :disabled="confirmLoading"
              @click="executeConfirm"
            >{{ confirmLoading ? '处理中...' : confirmOkText }}</button>
          </div>
        </div>
      </div>
    </teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import SearchSingleSelect from './SearchSingleSelect.vue'
import SearchMultiSelect from './SearchMultiSelect.vue'
import * as subApi from '../api/subscriptions'
import type {
  SubscriptionListItem, SubscriptionDetail,
  MonitorScope, PresetType, BotInfo,
} from '../api/subscriptions'
import { ScopeLabels, PresetTypeLabels } from '../api/subscriptions'
import * as pushApi from '../api/bot'
import type { PushBotItem, ChannelType } from '../api/bot'
import { ChannelLabels } from '../api/bot'
import * as strategyApi from '../api/strategies'

// ========== 数据状态 ==========
const subs = ref<SubscriptionListItem[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

// ========== 搜索过滤 ==========
const searchQuery = ref('')
const filteredSubs = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return subs.value
  return subs.value.filter(s =>
    s.name.toLowerCase().includes(q) ||
    (s.strategy_name || '').toLowerCase().includes(q)
  )
})
function onSearch() {
  // 前端过滤，无需调接口
}

// ========== 加载列表 ==========
async function loadSubs() {
  loading.value = true
  try {
    const resp = await subApi.listSubscriptions(page.value, pageSize.value)
    subs.value = resp.list || []
    total.value = resp.total || 0
  } catch (e: any) {
    console.error('加载订阅列表失败:', e)
  } finally {
    loading.value = false
  }
}

// ========== 表单弹窗 ==========
const showFormModal = ref(false)
const isEditing = ref(false)
const editTarget = ref<SubscriptionListItem | null>(null)
const submitting = ref(false)
const formError = ref('')

// 策略选项（供下拉选择）
const strategyOptions = ref<{ id: number; name: string }[]>([])

// 可用机器人（已启用）
const availableBots = ref<PushBotItem[]>([])

interface FormData {
  name: string
  strategy_id: number
  scope: MonitorScope
  preset_type: PresetType | ''
  cron_expr: string
  trading_hours_only: boolean
  bot_ids: number[]
}

function emptyForm(): FormData {
  return {
    name: '',
    strategy_id: 0,
    scope: 'all',
    preset_type: '',
    cron_expr: '',
    trading_hours_only: true,
    bot_ids: [],
  }
}
const form = ref<FormData>(emptyForm())
const customStocksText = ref('')

// ========== 下拉框选项 ==========

const strategySelectOptions = computed(() =>
  strategyOptions.value.map((s) => ({ value: s.id, label: s.name })),
)

const botSelectOptions = computed(() =>
  availableBots.value.map((b) => ({ value: b.id, label: b.name, channel: b.channel })),
)

const scopeOptions = [
  { value: 'all', label: '全部A股' },
  { value: 'held', label: '我的持仓' },
  { value: 'custom', label: '自选股票' },
]

const presetTypeOptions = computed(() =>
  Object.entries(PresetTypeLabels).map(([key, label]) => ({ value: key, label })),
)

// 表单校验
const isFormValid = computed(() => {
  const f = form.value
  if (!f.name.trim() || !f.strategy_id || !f.preset_type) return false
  if (f.scope === 'custom') {
    const codes = customStocksText.value
      .split(/[,，\s]+/)
      .map((c) => c.trim())
      .filter(Boolean)
    if (codes.length === 0) return false
  }
  if (f.preset_type === 'custom' && !f.cron_expr.trim()) return false
  return true
})

// 解析自定义股票代码
function parseCustomStocks(): string[] {
  return customStocksText.value
    .split(/[,，\s]+/)
    .map((c) => c.trim())
    .filter((c) => /^\d{6}$/.test(c))
}

async function openCreateModal() {
  isEditing.value = false
  editTarget.value = null
  form.value = emptyForm()
  customStocksText.value = ''
  formError.value = ''
  await loadFormData()
  showFormModal.value = true
}

async function openEditModal(sub: SubscriptionListItem) {
  isEditing.value = true
  editTarget.value = sub
  formError.value = ''
  await loadFormData()

  // 加载详情填充表单
  try {
    const detail = await subApi.getSubscription(sub.id)
    form.value = {
      name: detail.name,
      strategy_id: detail.strategy_id,
      scope: detail.scope as MonitorScope,
      preset_type: detail.preset_type as PresetType,
      cron_expr: detail.cron_expr || '',
      trading_hours_only: detail.trading_hours_only,
      bot_ids: detail.bots.map((b) => b.id),
    }
    // 解析 custom_stocks JSON
    if (detail.custom_stocks) {
      try {
        const codes = JSON.parse(detail.custom_stocks)
        customStocksText.value = Array.isArray(codes) ? codes.join(', ') : ''
      } catch {
        customStocksText.value = ''
      }
    } else {
      customStocksText.value = ''
    }
  } catch (e: any) {
    formError.value = e.message || '加载详情失败'
  }

  showFormModal.value = true
}

async function loadFormData() {
  // 加载策略选项
  try {
    const resp = await strategyApi.fetchStrategies('', 1, 100)
    strategyOptions.value = (resp.list || []).map((s: any) => ({ id: s.id, name: s.name }))
  } catch (e) {
    console.error('加载策略列表失败:', e)
  }
  // 加载已启用机器人
  try {
    const bots = await pushApi.listPushBots()
    availableBots.value = bots.filter((b) => b.status === 1)
  } catch (e) {
    console.error('加载机器人列表失败:', e)
  }
}

function closeFormModal() {
  showFormModal.value = false
  editTarget.value = null
  form.value = emptyForm()
  customStocksText.value = ''
}

async function confirmForm() {
  if (!isFormValid.value) {
    formError.value = '请填写完整必填信息'
    return
  }

  submitting.value = true
  formError.value = ''

  try {
    if (isEditing.value && editTarget.value) {
      // 编辑模式：更新订阅本体（不含 bot_ids）
      const updateData: subApi.UpdateSubscriptionReq = {
        name: form.value.name.trim(),
        strategy_id: form.value.strategy_id,
        scope: form.value.scope,
        preset_type: form.value.preset_type as PresetType,
        cron_expr: form.value.cron_expr.trim() || undefined,
        trading_hours_only: form.value.trading_hours_only,
      }
      if (form.value.scope === 'custom') {
        updateData.custom_stocks = parseCustomStocks()
        if (updateData.custom_stocks!.length === 0) {
          formError.value = '请输入有效的股票代码'
          submitting.value = false
          return
        }
      }
      await subApi.updateSubscription(editTarget.value.id, updateData)
      // 单独更新机器人关联
      await subApi.updateSubscriptionBots(editTarget.value.id, form.value.bot_ids)
    } else {
      // 创建模式：一次性提交全部字段
      const data: subApi.CreateSubscriptionReq = {
        name: form.value.name.trim(),
        strategy_id: form.value.strategy_id,
        scope: form.value.scope,
        preset_type: form.value.preset_type as PresetType,
        cron_expr: form.value.cron_expr.trim() || undefined,
        trading_hours_only: form.value.trading_hours_only,
        bot_ids: form.value.bot_ids.length > 0 ? form.value.bot_ids : undefined,
      }
      if (form.value.scope === 'custom') {
        data.custom_stocks = parseCustomStocks()
        if (data.custom_stocks!.length === 0) {
          formError.value = '请输入有效的股票代码'
          submitting.value = false
          return
        }
      }
      await subApi.createSubscription(data)
    }

    closeFormModal()
    await loadSubs()
  } catch (e: any) {
    formError.value = e.message || '操作失败'
  } finally {
    submitting.value = false
  }
}

// ========== 操作 ==========

// 启停
const runningId = ref<number | null>(null)

function onToggleActive(sub: SubscriptionListItem, active: boolean) {
  const action = active ? '启用' : '停用'
  openConfirm({
    title: `${action}订阅`,
    message: `确定${action}订阅「${sub.name}」？`,
    okText: action,
    isDanger: !active,
    onOk: async () => {
      await subApi.setSubscriptionActive(sub.id, active)
      await loadSubs()
    },
  })
}

// 手动执行
function onTriggerRun(sub: SubscriptionListItem) {
  openConfirm({
    title: '手动执行',
    message: `确定立即执行订阅「${sub.name}」？结果将推送到关联的机器人。`,
    okText: '执行',
    onOk: async () => {
      runningId.value = sub.id
      try {
        await subApi.triggerSubscriptionRun(sub.id)
        await loadSubs()
        runningId.value = null
      } catch (e: any) {
        runningId.value = null
        throw e
      }
    },
  })
}

// 删除
function onDelete(sub: SubscriptionListItem) {
  openConfirm({
    title: '删除订阅',
    message: `确定删除订阅「${sub.name}」？删除后无法恢复。`,
    okText: '删除',
    isDanger: true,
    onOk: async () => {
      await subApi.deleteSubscription(sub.id)
      await loadSubs()
    },
  })
}

// ========== 通用确认弹窗 ==========
const showConfirmModal = ref(false)
const confirmTitle = ref('')
const confirmMessage = ref('')
const confirmOkText = ref('确定')
const confirmIsDanger = ref(false)
const confirmLoading = ref(false)
const confirmError = ref('')
let confirmAction: (() => Promise<void>) | null = null

function openConfirm(opts: {
  title: string
  message: string
  okText?: string
  isDanger?: boolean
  onOk: () => Promise<void>
}) {
  confirmTitle.value = opts.title
  confirmMessage.value = opts.message
  confirmOkText.value = opts.okText || '确定'
  confirmIsDanger.value = opts.isDanger ?? false
  confirmError.value = ''
  confirmLoading.value = false
  confirmAction = opts.onOk
  showConfirmModal.value = true
}

function closeConfirmModal() {
  showConfirmModal.value = false
  confirmAction = null
}

async function executeConfirm() {
  if (!confirmAction || confirmLoading.value) return
  confirmLoading.value = true
  confirmError.value = ''
  try {
    await confirmAction()
    closeConfirmModal()
  } catch (e: any) {
    confirmError.value = e.message || '操作失败'
  } finally {
    confirmLoading.value = false
  }
}

// ========== 工具函数 ==========

function formatTime(t: string): string {
  if (!t) return '-'
  return t.replace('T', ' ').slice(0, 16)
}

onMounted(() => {
  loadSubs()
})
</script>

<style scoped>
.sub-mgmt-page {}
.loading-state { text-align: center; color: #999; padding: 60px 0; font-size: 14px; }

/* ====== 头部布局 ====== */
.page-header {
  display: flex; justify-content: space-between; align-items: flex-start;
  margin-bottom: 20px; flex-wrap: wrap; gap: 12px;
}
.header-meta h1 { font-size: 22px; font-weight: 700; margin-bottom: 4px; }
.header-meta p { font-size: 14px; color: #999; }
.header-actions {
  display: flex; align-items: center; gap: 10px; flex-wrap: wrap;
}

/* ====== 搜索框（与策略列表统一）====== */
.sl-search {
  display: flex; align-items: center;
  border: 1px solid #d9d9d9; border-radius: 6px;
  overflow: hidden; transition: border-color .15s;
}
.sl-search:focus-within { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.08); }
.search-input {
  border: none; outline: none; padding: 6px 12px;
  font-size: 13px; width: 180px; color: #333; background: transparent;
}
.search-input::placeholder { color: #bbb; }
.search-btn {
  border: none; background: transparent; cursor: pointer;
  padding: 6px 10px; font-size: 14px; border-left: 1px solid #eee; transition: background .12s;
}
.search-btn:hover { background: #f5f5f5; }

/* ====== 表格（与 push-table 统一风格）====== */
.sub-table {
  width: 100%; border-collapse: collapse;
  background: #fff; border-radius: 12px; overflow: hidden;
  box-shadow: 0 1px 4px rgba(0,0,0,.06);
}
.sub-table th {
  background: #fafafa; padding: 10px 14px; text-align: center;
  font-size: 13px; font-weight: 600; color: #666; border-bottom: 1px solid #eee;
  white-space: nowrap;
}
.sub-table td {
  padding: 10px 14px; font-size: 13.5px; border-bottom: 1px solid #f3f3f3;
  vertical-align: middle; text-align: center;
}
.sub-table tr:hover td { background: #f9fbff; }
.sub-table tr.disabled-row td { opacity: .45; }

.name-cell { font-weight: 600; color: #333; max-width: 140px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.time-cell { font-size: 12.5px; color: #888; white-space: nowrap; }
.actions-cell { white-space: nowrap; }

/* 渠道标签（与 push-table 统一）*/
.channel-tag {
  display: inline-block; padding: 2px 10px; border-radius: 10px;
  font-size: 11.5px; font-weight: 600;
}
.channel-wecom { background: #f0f5ff; color: #2f54eb; }
.channel-dingtalk { background: #e8f8ff; color: #0891c5; }
.channel-feishu { background: #e6fffb; color: #13c2c2; }

/* 状态标签（与 push-table 统一）*/
.status-tag {
  display: inline-block; padding: 2px 10px; border-radius: 10px;
  font-size: 11.5px; font-weight: 600;
}
.status-on { background: #f0fdf4; color: #16a34a; }
.status-off { background: #fef2f2; color: #dc2626; }

/* 监控范围标签 */
.scope-tag {
  display: inline-block; padding: 2px 10px; border-radius: 10px;
  font-size: 11.5px; font-weight: 600;
}
.scope-all { background: #f0f5ff; color: #2f54eb; }
.scope-held { background: #f0fdf4; color: #16a34a; }
.scope-custom { background: #fff7e6; color: #d46b08; }

/* 策略标签 */
.strategy-tag {
  display: inline-block; padding: 2px 10px; border-radius: 10px;
  font-size: 11.5px; font-weight: 600;
  background: #f0f7ff; color: #1677ff;
}

/* 预设频率标签 */
.preset-tag {
  display: inline-block; padding: 2px 10px; border-radius: 10px;
  font-size: 11.5px; font-weight: 600;
  background: #f5f5f5; color: #555;
}

/* Cron 标签 */
.cron-tag {
  display: inline-block; padding: 2px 10px; border-radius: 10px;
  font-size: 12px; font-weight: 500;
  background: #f5f5f5; color: #555;
  font-family: 'SF Mono', Monaco, monospace;
}

/* 表格中的机器人标签 */
.bot-tags { display: flex; flex-wrap: wrap; gap: 4px; justify-content: center; }
.bot-tag {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 1px 6px; border-radius: 4px;
  background: #f0f7ff; font-size: 11.5px; font-weight: 500;
}
.bot-tag .bot-channel { padding: 0 4px; border-radius: 3px; font-size: 10px; font-weight: 600; }
.bot-tag .bot-name { color: #1677ff; }

/* ====== 分页（与 push-table 统一风格）====== */
.pag-bar {
  display: flex; align-items: center; gap: 10px;
  margin-top: 14px; padding: 0 4px; justify-content: flex-end;
}
.pag-info { font-size: 13px; color: #999; margin-right: auto; }
.pag-btn {
  padding: 4px 12px; font-size: 13px; border: 1px solid #d9d9d9;
  border-radius: 5px; background: #fff; cursor: pointer; color: #333;
  transition: all .15s;
}
.pag-btn:hover:not(:disabled) { border-color: #1677ff; color: #1677ff; }
.pag-btn:disabled { opacity: .4; cursor: not-allowed; }
.pag-current { font-size: 13px; color: #555; }

/* ====== 按钮（与 push-table 统一）====== */
.btn-add {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 8px 18px; margin-top: 8px;
  background: #1677ff; color: #fff; border: none; border-radius: 8px;
  font-size: 14px; font-weight: 600; cursor: pointer; transition: all .15s;
}
.btn-add:hover { background: #0958d9; transform: translateY(-1px); box-shadow: 0 2px 8px rgba(22,119,255,.25); }

.btn-sm {
  padding: 4px 12px; font-size: 12.5px; border: 1px solid #d9d9d9;
  border-radius: 5px; background: #fff; cursor: pointer; margin-right: 6px;
  transition: all .15s;
}
.btn-sm:last-child { margin-right: 0; }
.btn-sm:disabled { opacity: .4; cursor: not-allowed; }
.btn-ok { color: #1677ff; border-color: #91caff; background: #f0f7ff; }
.btn-ok:hover:not(:disabled) { background: #d6e8ff; }
.btn-warn { color: #ff6b00; border-color: #ffd591; background: #fff7e6; }
.btn-warn:hover:not(:disabled) { background: #ffe7ba; }
.btn-danger { color: #cf1322; border-color: #ffa39e; background: #fff1f0; }
.btn-danger:hover:not(:disabled) { background: #ffccc7; }
.btn-success { color: #52c41a; border-color: #b7eb8f; background: #f6ffed; }
.btn-success:hover:not(:disabled) { background: #d9f7be; }
.btn-info { color: #555; }
.btn-info:hover:not(:disabled) { border-color: #aaa; background: #f5f5f5; }

.empty-hint { text-align: center; color: #bbb; padding: 60px 0; font-size: 14px; }
.error-msg { color: #cf1322; font-size: 13px; margin-bottom: 10px; text-align: center; min-height: 20px; }
.required { color: #cf1322; }
.field-hint { font-size: 11.5px; color: #aaa; margin-top: 3px; line-height: 1.4; }

/* ====== Modal（与 BotConfigPage 统一）====== */
.modal-overlay {
  position: fixed; inset: 0; z-index: 999; background: rgba(0,0,0,.35);
  display: flex; align-items: center; justify-content: center;
}
.modal-box {
  background: #fff; border-radius: 16px; padding: 28px 32px; width: 420px;
  box-shadow: 0 4px 24px rgba(0,0,0,.12);
}
.modal-box h3 { font-size: 17px; margin-bottom: 18px; text-align: center; }

.form-modal { width: 480px; max-height: 85vh; overflow-y: auto; }
.form-group { margin-bottom: 16px; }
.form-group label {
  display: block; font-size: 13px; font-weight: 600; color: #555; margin-bottom: 6px;
}
.form-group input[type="text"],
.form-group textarea,
.form-group select {
  width: 100%; padding: 10px 12px; border: 1.5px solid #d9d9d9; border-radius: 8px;
  font-size: 14px; outline: none; box-sizing: border-box;
  background: #fff; color: #333;
}
.form-group input[type="text"]:focus,
.form-group textarea:focus,
.form-group select:focus { border-color: #1677ff; box-shadow: 0 0 0 3px rgba(22,119,255,.08); }
.form-group select { cursor: pointer; appearance: auto; }
.form-group textarea { resize: vertical; font-family: inherit; }

.checkbox-group { margin: 16px 0; }
.checkbox-label {
  display: flex; align-items: center; gap: 10px;
  font-size: 14px; font-weight: 500; color: #333;
}
.checkbox-label input[type="checkbox"] {
  width: 16px; height: 16px; accent-color: #1677ff; cursor: pointer;
  margin: 0; flex-shrink: 0;
  vertical-align: middle;
  position: relative;
  top: -0.5px;
}

/* 机器人多选（SearchMultiSelect 中的渠道标签）*/
.multi-select .bot-channel {
  padding: 0 5px; border-radius: 3px; font-size: 10px; font-weight: 600;
  flex-shrink: 0;
}

.modal-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 4px; }
.btn-login-sm {
  padding: 7px 22px; font-size: 14px; font-weight: 600;
  color: #fff; background: #1677ff; border: none; border-radius: 6px; cursor: pointer;
}
.btn-login-sm:hover:not(:disabled) { background: #0958d9; }
.btn-login-sm:disabled { opacity: .5; cursor: not-allowed; }
.btn-modal-cancel {
  padding: 7px 18px; border: 1px solid #d9d9d9; border-radius: 6px; background: #fff;
  font-size: 13px; cursor: pointer; color: #666;
}
.btn-modal-cancel:hover { background: #f5f5f5; }

/* 确认弹窗 */
.confirm-modal { width: 380px; text-align: center; }
.confirm-msg { font-size: 14.5px; color: #555; line-height: 1.6; padding: 8px 0 4px; }
.btn-danger-sm {
  padding: 7px 22px; font-size: 14px; font-weight: 600;
  color: #fff; background: #cf1322; border: none; border-radius: 6px; cursor: pointer;
}
.btn-danger-sm:hover:not(:disabled) { background: #a80f1c; }
.btn-danger-sm:disabled { opacity: .5; cursor: not-allowed; }
</style>
