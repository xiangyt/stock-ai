<template>
  <div class="monitor-page">
    <header class="page-header">
      <h1>📈 盯盘监控</h1>
      <p>配置监控规则，持仓异动自动推送到指定机器人</p>
    </header>

    <div class="monitor-toolbar">
      <button class="btn-add" @click="openCreate">+ 新建监控</button>
      <div class="toolbar-right">共 {{ configs.length }} 条配置</div>
    </div>

    <div v-if="loading" class="loading-state">加载中...</div>

    <table v-else-if="configs.length > 0" class="monitor-table">
      <thead>
        <tr>
          <th>名称</th>
          <th>监控范围</th>
          <th>启用规则</th>
          <th>冷却策略</th>
          <th>推送机器人</th>
          <th>状态</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="cfg in configs" :key="cfg.id" :class="{ 'disabled-row': !cfg.is_active }">
          <td class="name-cell">{{ cfg.name }}</td>
          <td>
            <span :class="'scope-tag scope-' + cfg.scope">
              {{ cfg.scope === 'held' ? '我的持仓' : '自选' + (cfg.stocks?.length || 0) + '只' }}
            </span>
          </td>
          <td>
            <div class="rule-tags">
              <span v-if="cfg.rule" class="rule-tag">
                {{ RuleTypeLabels[cfg.rule.type] || cfg.rule.type }}
              </span>
            </div>
          </td>
          <td>{{ cfg.cooldown?.interval_minutes }}min / 日{{ cfg.cooldown?.daily_max }}次</td>
          <td>
            <div v-if="cfg.bots?.length" class="bot-tags">
              <span v-for="bot in cfg.bots" :key="bot.id" class="bot-tag">
                <span :class="'bot-channel channel-' + bot.channel">{{ channelLabel(bot.channel) }}</span>
                <span class="bot-name">{{ bot.name }}</span>
              </span>
            </div>
            <span v-else>-</span>
          </td>
          <td>
            <span :class="'status-tag status-' + (cfg.is_active ? 'on' : 'off')">
              {{ cfg.is_active ? '运行中' : '已停用' }}
            </span>
          </td>
          <td class="actions-cell">
            <button class="btn-sm btn-ok" @click="openEdit(cfg)">编辑</button>
            <button
              :class="'btn-sm ' + (cfg.is_active ? 'btn-warn' : 'btn-success')"
              @click="onToggle(cfg)"
            >{{ cfg.is_active ? '停用' : '启用' }}</button>
            <button class="btn-sm btn-danger" @click="onDelete(cfg)">删除</button>
          </td>
        </tr>
      </tbody>
    </table>

    <div v-else class="empty-hint">暂无监控配置，点击「+ 新建监控」开始</div>

    <!-- ====== 表单弹窗 ====== -->
    <div v-if="showForm" class="modal-overlay" @click.self="closeForm">
      <div class="modal-box form-modal">
        <h3>{{ isEditing ? '编辑监控' : '新建监控' }}</h3>
        <div v-if="formError" class="error-msg">{{ formError }}</div>

        <!-- 名称 -->
        <div class="form-group">
          <label>名称 <span class="required">*</span></label>
          <input v-model="form.name" type="text" placeholder="如：我的持仓监控" />
        </div>

        <!-- 监控范围 -->
        <div class="form-group">
          <label>监控范围</label>
          <select v-model="form.scope">
            <option value="held">我的持仓</option>
            <option value="custom">自选股票</option>
          </select>
        </div>

        <div v-if="form.scope === 'custom'" class="form-group">
          <label>自选股票代码 <span class="required">*</span></label>
          <input v-model="stocksText" type="text" placeholder="000001, 600036, 300750" />
          <div class="field-hint">6位数字代码，逗号分隔</div>
        </div>

        <!-- ====== 告警规则（单选） ====== -->
        <div class="form-group">
          <label>告警规则 <span class="required">*</span></label>
          <select v-model="selectedRuleType" class="rule-type-select" @change="onRuleTypeChange">
            <option value="" disabled>-- 选择规则类型 --</option>
            <option v-for="opt in availableRuleTypes" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </option>
          </select>

          <!-- 当日涨幅参数 -->
          <div v-if="selectedRuleType === 'daily_change'" class="rule-params" style="margin-top:12px">
            <div v-for="dk in dailyChangeKeys" :key="dk.key" class="param-row">
              <input
                type="checkbox"
                :checked="!!(currentRule.params as any)[dk.key + '_enabled']"
                @change="(e: Event) => (currentRule.params as any)[dk.key + '_enabled'] = (e.target as HTMLInputElement).checked"
                style="width:16px;height:16px;accent-color:#1677ff;cursor:pointer;flex-shrink:0"
              />
              <label class="param-label" style="min-width:90px">{{ dk.label }}</label>
              <input
                v-model.number="(currentRule.params as any)[dk.key]"
                type="number"
                step="0.5"
                class="param-input"
                :disabled="!(currentRule.params as any)[dk.key + '_enabled']"
              />
            </div>
          </div>

          <!-- 急拉急跌参数 -->
          <div v-if="selectedRuleType === 'rapid_move'" class="rule-params" style="margin-top:12px">
            <div class="param-row">
              <label class="param-label">时间窗口(分钟)</label>
              <input v-model.number="currentRule.params.minutes" type="number" min="1" class="param-input" />
            </div>
            <div class="param-row">
              <label class="param-label">涨跌幅阈值(%)</label>
              <input v-model.number="currentRule.params.amplitude_pct" type="number" step="0.1" class="param-input" />
            </div>
            <div class="param-row">
              <label class="param-label">方向</label>
              <label class="param-choice">
                <input type="checkbox" v-model="currentRule.params.up_enabled" /> 上涨
              </label>
              <label class="param-choice">
                <input type="checkbox" v-model="currentRule.params.down_enabled" /> 下跌
              </label>
            </div>
          </div>

          <!-- 量比异动参数 -->
          <div v-if="selectedRuleType === 'volume_ratio'" class="rule-params" style="margin-top:12px">
            <div class="param-row">
              <label class="param-label">量比大于</label>
              <input v-model.number="currentRule.params.min_ratio" type="number" step="0.5" class="param-input" />
            </div>
            <div class="field-hint">当日成交量 / 近5日均量 ≥ 此值时告警</div>
          </div>

          <!-- 封单监控参数 -->
          <div v-if="selectedRuleType === 'seal_board'" class="rule-params" style="margin-top:12px">
            <div class="param-row">
              <label class="param-label">封单小于(手)</label>
              <input v-model.number="currentRule.params.min_lots" type="number" min="1" class="param-input" />
            </div>
            <div class="field-hint">涨停时监控买一量，跌停时监控卖一量，共用同一阈值</div>
          </div>
        </div>

        <!-- 冷却策略 -->
        <div class="form-group">
          <label>冷却策略</label>
          <div class="cooldown-row">
            <span>同一股票</span>
            <input v-model.number="form.cooldown.interval_minutes" type="number" min="1" max="60" style="width:60px" />
            <span>分钟内不重复告警</span>
          </div>
          <div class="cooldown-row" style="margin-top:8px">
            <span>每天同一规则最多推送</span>
            <input v-model.number="form.cooldown.daily_max" type="number" min="1" max="20" style="width:60px" />
            <span>次</span>
          </div>
        </div>

        <!-- 推送模板 -->
        <div class="form-group">
          <label>推送模板 <span class="field-tip-icon">?<span class="field-tip-tooltip var-list-tip">
            <div class="var-row" v-for="(desc, key) in AllTemplateVars" :key="key">
              <code>${ {{ key }} }</code><span>{{ desc }}</span>
            </div>
          </span></span></label>
          <input v-model="form.template" type="text" :placeholder="DefaultRuleTemplates[selectedRuleType] || ''" />
        </div>

        <!-- 推送机器人（与订阅一致的多选下拉） -->
        <div class="form-group">
          <label>推送机器人 <span class="field-tip-icon">?<span class="field-tip-tooltip">最多关联 5 个机器人，请先在「机器人配置」中启用</span></span></label>
          <SearchMultiSelect
            v-model="form.bot_ids"
            :options="botSelectOptions"
            placeholder="搜索机器人..."
            :max-count="5"
          >
            <template #option="{ option }">
              <span class="bot-channel" :class="'channel-' + option.channel">{{ ChannelLabels[option.channel as ChannelType] || option.channel }}</span>
              <span class="ms-option-label">{{ option.label }}</span>
            </template>
          </SearchMultiSelect>
        </div>

        <div class="modal-actions">
          <button class="btn-modal-cancel" @click="closeForm">取消</button>
          <button class="btn-login-sm" :disabled="submitting" @click="confirmForm">
            {{ submitting ? '保存中...' : '保存' }}
          </button>
        </div>
      </div>
    </div>

    <!-- ====== 确认弹窗 ====== -->
    <div v-if="showConfirm" class="modal-overlay" @click.self="closeConfirm">
      <div class="modal-box confirm-modal">
        <h3>{{ confirmTitle }}</h3>
        <p class="confirm-msg">{{ confirmMsg }}</p>
        <div v-if="confirmErr" class="error-msg">{{ confirmErr }}</div>
        <div class="modal-actions">
          <button class="btn-modal-cancel" @click="closeConfirm">取消</button>
          <button
            :class="confirmDanger ? 'btn-danger-sm' : 'btn-login-sm'"
            :disabled="confirmLoading"
            @click="execConfirm"
          >{{ confirmLoading ? '处理中...' : confirmOk }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  listMonitorConfigs, createMonitorConfig, updateMonitorConfig, deleteMonitorConfig,
  setMonitorConfigActive, DefaultCooldown, DefaultRuleTemplates, AllTemplateVars,
  RuleTypeLabels, DailyChangeSubKeys,
  type MonitorConfigDetail, type MonitorRule, type MonitorCooldown, type RuleType,
} from '../api/monitor-config'
import { listPushBots, ChannelLabels, type PushBotItem, type ChannelType } from '../api/bot'
import SearchMultiSelect from './SearchMultiSelect.vue'

// ========== 列表 ==========
const configs = ref<MonitorConfigDetail[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  try {
    const resp = await listMonitorConfigs(1, 50)
    configs.value = resp.data || []
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

// ========== 表单 ==========
const showForm = ref(false)
const isEditing = ref(false)
const editingId = ref(0)
const submitting = ref(false)
const formError = ref('')
const stocksText = ref('')
const availableBots = ref<PushBotItem[]>([])
const botSelectOptions = computed(() =>
  availableBots.value.map((b) => ({ value: b.id, label: b.name, channel: b.channel })),
)
const selectedRuleType = ref<RuleType | ''>('')
const currentRule = ref<MonitorRule>({ type: 'daily_change', params: {} })

const dailyChangeKeys = DailyChangeSubKeys as readonly { key: string; label: string }[]

const form = ref<{
  name: string
  scope: 'held' | 'custom'
  cooldown: MonitorCooldown
  template: string
  bot_ids: number[]
}>({
  name: '',
  scope: 'held',
  cooldown: { ...DefaultCooldown },
  template: '',
  bot_ids: [],
})

const availableRuleTypes = ref<{ value: RuleType; label: string }[]>([
  { value: 'daily_change', label: '当日涨幅监控' },
  { value: 'rapid_move', label: '急拉急跌监控' },
  { value: 'volume_ratio', label: '量比异动监控' },
  { value: 'seal_board', label: '涨跌停封单监控' },
])

function defaultParams(type: RuleType): Record<string, number | boolean> {
  switch (type) {
    case 'daily_change':
      return {
        surge_big_enabled: true, surge_big: 9,
        surge_small_enabled: true, surge_small: 5,
        limit_up_enabled: true, limit_up: 9.8,
        limit_down_enabled: true, limit_down: -9.8,
        drop_small_enabled: true, drop_small: -5,
        drop_big_enabled: true, drop_big: -9,
      }
    case 'rapid_move':
      return { minutes: 5, amplitude_pct: 3, up_enabled: true, down_enabled: true }
    case 'volume_ratio':
      return { min_ratio: 3 }
    case 'seal_board':
      return { min_lots: 1000 }
    default:
      return {}
  }
}

function onRuleTypeChange() {
  if (!selectedRuleType.value) return
  currentRule.value = {
    type: selectedRuleType.value,
    params: { ...defaultParams(selectedRuleType.value) },
  }
  // 自动填充默认模板（仅新建或模板为空时）
  if (!isEditing.value || !form.value.template) {
    form.value.template = DefaultRuleTemplates[selectedRuleType.value] || ''
  }
}

// 机器人多选
function resetForm() {
  form.value = {
    name: '',
    scope: 'held',
    cooldown: { ...DefaultCooldown },
    template: '',
    bot_ids: [],
  }
  stocksText.value = ''
  selectedRuleType.value = ''
  currentRule.value = { type: 'daily_change', params: {} }
  formError.value = ''
}

async function openCreate() {
  isEditing.value = false
  editingId.value = 0
  resetForm()
  await loadBots()
  showForm.value = true
}

async function openEdit(cfg: MonitorConfigDetail) {
  isEditing.value = true
  editingId.value = cfg.id
  formError.value = ''
  await loadBots()

  form.value = {
    name: cfg.name,
    scope: cfg.scope,
    rules: [],
    cooldown: cfg.cooldown ? { ...cfg.cooldown } : { ...DefaultCooldown },
    template: cfg.template || '',
    bot_ids: cfg.bots?.map((b) => b.id) || [],
  }
  stocksText.value = (cfg.stocks || []).join(', ')

  // 填充单条规则
  if (cfg.rule) {
    const r = cfg.rule
    selectedRuleType.value = r.type as RuleType
    currentRule.value = { type: r.type as RuleType, params: { ...r.params } }
  } else {
    selectedRuleType.value = ''
    currentRule.value = { type: 'daily_change', params: {} }
  }
  showForm.value = true
}

function closeForm() {
  showForm.value = false
}

async function loadBots() {
  try {
    const bots = await listPushBots()
    availableBots.value = bots.filter((b) => b.status === 1)
  } catch (e) {
    console.error(e)
  }
}

async function confirmForm() {
  if (!form.value.name.trim()) { formError.value = '请输入监控名称'; return }
  if (!selectedRuleType.value) { formError.value = '请选择告警规则类型'; return }
  if (form.value.scope === 'custom') {
    const codes = parseStocks()
    if (codes.length === 0) { formError.value = '自选模式下必须指定股票代码'; return }
    if (codes.some((c) => !/^\d{6}$/.test(c))) { formError.value = '股票代码必须为6位数字'; return }
  }

  submitting.value = true
  formError.value = ''

  try {
    const stocks = form.value.scope === 'custom' ? parseStocks() : []
    const rule: MonitorRule = { type: currentRule.value.type, params: { ...currentRule.value.params } }
    if (isEditing.value) {
      await updateMonitorConfig(editingId.value, {
        name: form.value.name.trim(),
        scope: form.value.scope,
        stocks,
        rule,
        cooldown: form.value.cooldown,
        template: form.value.template || undefined,
      })
      // 单独更新机器人关联
      if (form.value.bot_ids.length > 0) {
        await updateMonitorConfigBots(editingId.value, form.value.bot_ids)
      }
    } else {
      await createMonitorConfig({
        name: form.value.name.trim(),
        scope: form.value.scope,
        stocks: stocks.length > 0 ? stocks : undefined,
        rule,
        cooldown: form.value.cooldown,
        template: form.value.template || undefined,
        bot_ids: form.value.bot_ids.length > 0 ? form.value.bot_ids : undefined,
      })
    }
    closeForm()
    await load()
  } catch (e: any) {
    formError.value = e.message || '操作失败'
  } finally {
    submitting.value = false
  }
}

function parseStocks(): string[] {
  return stocksText.value
    .split(/[,，\s]+/)
    .map((s) => s.trim())
    .filter(Boolean)
}

// ========== 操作 ==========
function onToggle(cfg: MonitorConfigDetail) {
  const action = cfg.is_active ? '停用' : '启用'
  openConfirm({
    title: `${action}监控`,
    msg: `确定${action}监控「${cfg.name}」？`,
    ok: action,
    danger: !cfg.is_active,
    action: async () => {
      await setMonitorConfigActive(cfg.id, !cfg.is_active)
      await load()
    },
  })
}

function onDelete(cfg: MonitorConfigDetail) {
  openConfirm({
    title: '删除监控',
    msg: `确定删除监控「${cfg.name}」？删除后无法恢复。`,
    ok: '删除',
    danger: true,
    action: async () => {
      await deleteMonitorConfig(cfg.id)
      await load()
    },
  })
}

// ========== 确认弹窗 ==========
const showConfirm = ref(false)
const confirmTitle = ref('')
const confirmMsg = ref('')
const confirmOk = ref('确定')
const confirmDanger = ref(false)
const confirmLoading = ref(false)
const confirmErr = ref('')
let confirmAction: (() => Promise<void>) | null = null

function openConfirm(opts: {
  title: string; msg: string; ok: string; danger?: boolean; action: () => Promise<void>
}) {
  confirmTitle.value = opts.title
  confirmMsg.value = opts.msg
  confirmOk.value = opts.ok
  confirmDanger.value = opts.danger ?? false
  confirmErr.value = ''
  confirmLoading.value = false
  confirmAction = opts.action
  showConfirm.value = true
}

function closeConfirm() { showConfirm.value = false; confirmAction = null }

async function execConfirm() {
  if (!confirmAction || confirmLoading.value) return
  confirmLoading.value = true
  confirmErr.value = ''
  try { await confirmAction(); closeConfirm() }
  catch (e: any) { confirmErr.value = e.message || '操作失败' }
  finally { confirmLoading.value = false }
}

// ========== 工具 ==========
const channelNames: Record<string, string> = {
  dingtalk: '钉钉', feishu: '飞书', wecom: '企微', qq: 'QQ',
}
function channelLabel(ch: string) { return channelNames[ch] || ch }

onMounted(() => load())
</script>

<style scoped>
.monitor-page {}
.loading-state { text-align: center; color: #999; padding: 60px 0; font-size: 14px; }
.empty-hint { text-align: center; color: #bbb; padding: 60px 0; font-size: 14px; }

.monitor-toolbar {
  display: flex; justify-content: space-between; align-items: center;
  margin-bottom: 14px; gap: 8px;
}
.toolbar-right { font-size: 12.5px; color: #888; }

.monitor-table {
  width: 100%; border-collapse: collapse;
  background: #fff; border-radius: 12px; overflow: hidden;
  box-shadow: 0 1px 4px rgba(0,0,0,.06);
}
.monitor-table th {
  background: #fafafa; padding: 10px 14px; text-align: center;
  font-size: 13px; font-weight: 600; color: #666; border-bottom: 1px solid #eee;
}
.monitor-table td {
  padding: 10px 14px; font-size: 13.5px; border-bottom: 1px solid #f3f3f3;
  text-align: center;
}
.monitor-table tr:hover td { background: #f9fbff; }
.monitor-table tr.disabled-row td { opacity: .45; }
.name-cell { font-weight: 600; color: #333; }

.scope-tag {
  display: inline-block; padding: 2px 10px; border-radius: 10px;
  font-size: 11.5px; font-weight: 600;
}
.scope-held { background: #f0fdf4; color: #16a34a; }
.scope-custom { background: #fff7e6; color: #d46b08; }

.rule-tags { display: flex; flex-wrap: wrap; gap: 4px; justify-content: center; }
.rule-tag {
  display: inline-block; padding: 2px 8px; border-radius: 4px;
  font-size: 11px; font-weight: 500;
  background: #f0f7ff; color: #1677ff;
}

.bot-tags { display: flex; flex-wrap: wrap; gap: 4px; justify-content: center; }
.bot-tag {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 1px 6px; border-radius: 4px;
  background: #f0f7ff; font-size: 11.5px; font-weight: 500;
}
.bot-channel {
  padding: 0 4px; border-radius: 3px; font-size: 10px; font-weight: 600;
}
.channel-dingtalk { background: #e8f8ff; color: #0891c5; }
.channel-feishu { background: #e6fffb; color: #13c2c2; }
.channel-wecom { background: #f0f5ff; color: #2f54eb; }
.channel-qq { background: #f5f5f5; color: #555; }
.bot-name { color: #1677ff; }

.status-tag {
  display: inline-block; padding: 2px 10px; border-radius: 10px;
  font-size: 11.5px; font-weight: 600;
}
.status-on { background: #f0fdf4; color: #16a34a; }
.status-off { background: #fef2f2; color: #dc2626; }
.actions-cell { white-space: nowrap; }

/* ====== 规则参数 ====== */
.rule-params { display: flex; flex-direction: column; gap: 8px; }
.param-row {
  display: flex; align-items: center; gap: 8px;
}
.param-label { font-size: 12.5px; color: #555; min-width: 100px; }
.param-input {
  width: 80px; padding: 5px 8px; border: 1px solid #d9d9d9; border-radius: 4px;
  font-size: 13px; text-align: center; box-sizing: border-box;
  background: #fff; color: #333; outline: none;
}
.param-input:focus { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.08); }
.param-input:disabled { background: #f5f5f5; color: #bbb; cursor: not-allowed; }

select.param-input {
  appearance: none; -webkit-appearance: none;
  width: 80px; height: 30px;
  padding: 0 20px 0 8px;
  font-size: 13px; line-height: 30px; text-align: left;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='10' height='6'%3E%3Cpath d='M0 0l5 6 5-6z' fill='%23999'/%3E%3C/svg%3E");
  background-repeat: no-repeat; background-position: right 6px center;
  cursor: pointer;
}

.param-choice {
  display: inline-flex; align-items: center; gap: 4px;
  font-size: 12.5px; color: #555; cursor: pointer;
}
.param-choice input[type="checkbox"] {
  width: 16px; height: 16px; accent-color: #1677ff; cursor: pointer;
}

.rule-type-select {
  width: 100%; padding: 10px 12px; border: 1.5px solid #d9d9d9; border-radius: 8px;
  font-size: 14px; outline: none; background: #fff; color: #333;
}
.rule-type-select:focus { border-color: #1677ff; }

/* ====== 多选下拉 ====== */
.multi-select { position: relative; }
.multi-select-trigger {
  display: flex; flex-wrap: wrap; gap: 4px; min-height: 38px;
  padding: 6px 10px; border: 1.5px solid #d9d9d9; border-radius: 8px;
  cursor: pointer; align-items: center;
}
.multi-select.open .multi-select-trigger { border-color: #1677ff; }
.multi-select-trigger .placeholder { color: #bbb; font-size: 13px; }
.selected-tag {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 2px 8px; border-radius: 4px;
  background: #f0f7ff; font-size: 12px;
}
.remove-tag {
  margin-left: 2px; cursor: pointer; color: #999; font-size: 14px;
}
.remove-tag:hover { color: #cf1322; }

.multi-select-dropdown {
  position: absolute; top: 100%; left: 0; right: 0; z-index: 100;
  max-height: 200px; overflow-y: auto;
  background: #fff; border: 1px solid #d9d9d9; border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0,0,0,.1); margin-top: 4px;
}
.dropdown-item {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 12px; font-size: 13px; cursor: pointer;
}
.dropdown-item:hover { background: #f5f5f5; }
.dropdown-item.checked { background: #f0f7ff; }
.dropdown-item input[type="checkbox"] {
  width: 16px; height: 16px; accent-color: #1677ff; cursor: pointer;
}

/* 频道标签 */
.bot-channel-sm {
  padding: 0 5px; border-radius: 3px; font-size: 10px; font-weight: 600;
}

/* 冷却 */
.cooldown-row {
  display: flex; align-items: center; gap: 8px; font-size: 13px; color: #555;
}
.cooldown-row input {
  padding: 4px 8px; border: 1px solid #d9d9d9; border-radius: 4px;
  font-size: 13px; text-align: center;
}

/* ====== 通用组件 ====== */
:deep(.btn-add) {
  display: inline-flex; align-items: center; gap: 6px;
  padding: 8px 18px; background: #1677ff; color: #fff;
  border: none; border-radius: 8px; font-size: 14px; font-weight: 600;
  cursor: pointer; transition: all .15s;
}
:deep(.btn-add:hover) { background: #0958d9; }

:deep(.btn-sm) {
  padding: 4px 12px; font-size: 12.5px; border: 1px solid #d9d9d9;
  border-radius: 5px; background: #fff; cursor: pointer; margin-right: 6px;
}
:deep(.btn-sm:last-child) { margin-right: 0; }
:deep(.btn-ok) { color: #1677ff; border-color: #91caff; background: #f0f7ff; }
:deep(.btn-ok:hover:not(:disabled)) { background: #d6e8ff; }
:deep(.btn-warn) { color: #ff6b00; border-color: #ffd591; background: #fff7e6; }
:deep(.btn-warn:hover:not(:disabled)) { background: #ffe7ba; }
:deep(.btn-danger) { color: #cf1322; border-color: #ffa39e; background: #fff1f0; }
:deep(.btn-danger:hover:not(:disabled)) { background: #ffccc7; }
:deep(.btn-success) { color: #52c41a; border-color: #b7eb8f; background: #f6ffed; }
:deep(.btn-success:hover:not(:disabled)) { background: #d9f7be; }

.required { color: #cf1322; }
.field-hint { font-size: 11.5px; color: #aaa; margin-top: 3px; line-height: 1.4; }

/* label 旁悬浮提示图标 */
.field-tip-icon {
  position: relative;
  display: inline-flex; align-items: center; justify-content: center;
  width: 15px; height: 15px; border-radius: 50%;
  background: #e8e8e8; color: #999;
  font-size: 10px; font-weight: 700; line-height: 1;
  cursor: default; vertical-align: middle; margin-left: 4px;
}
.field-tip-tooltip {
  position: absolute; top: 50%; left: calc(100% + 8px);
  transform: translateY(-50%);
  min-width: 160px; max-width: 280px; padding: 6px 10px;
  background: #333; color: #fff;
  font-size: 12px; line-height: 1.5; border-radius: 6px;
  white-space: normal; font-weight: 400;
  opacity: 0; pointer-events: none; transition: opacity .15s;
  z-index: 1000;
}
.field-tip-tooltip::after {
  content: '';
  position: absolute; top: 50%; left: -10px;
  transform: translateY(-50%);
  border: 5px solid transparent;
  border-right-color: #333;
}
.field-tip-icon:hover { background: #1677ff; color: #fff; }
.field-tip-icon:hover .field-tip-tooltip { opacity: 1; }

/* 变量列表 tooltip（推送模板专用） */
.var-list-tip {
  min-width: 260px; max-width: 340px;
  padding: 10px 12px;
}
.var-row {
  display: flex; gap: 8px; align-items: baseline;
  padding: 1px 0;
}
.var-row code {
  font-family: monospace; font-size: 11px;
  color: #91caff; font-weight: 600;
  white-space: nowrap;
  min-width: 90px;
  display: inline-block;
}
.var-row span {
  font-size: 11.5px; color: #ddd; line-height: 1.4;
  flex: 1;
}

.error-msg { color: #cf1322; font-size: 13px; margin-bottom: 10px; text-align: center; }

.modal-overlay {
  position: fixed; inset: 0; z-index: 999; background: rgba(0,0,0,.35);
  display: flex; align-items: center; justify-content: center;
}
.modal-box {
  background: #fff; border-radius: 16px; padding: 28px 32px;
  box-shadow: 0 4px 24px rgba(0,0,0,.12);
}
.modal-box h3 { font-size: 17px; margin-bottom: 18px; text-align: center; }
.form-modal { width: 520px; max-height: 85vh; overflow-y: auto; }
.confirm-modal { width: 380px; text-align: center; }
.confirm-msg { font-size: 14.5px; color: #555; line-height: 1.6; padding: 8px 0 4px; }

.form-group { margin-bottom: 16px; }
.form-group > label {
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

.modal-actions { display: flex; justify-content: flex-end; gap: 10px; }

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

.btn-danger-sm {
  padding: 7px 22px; font-size: 14px; font-weight: 600;
  color: #fff; background: #cf1322; border: none; border-radius: 6px; cursor: pointer;
}
.btn-danger-sm:hover:not(:disabled) { background: #a80f1c; }
.btn-danger-sm:disabled { opacity: .5; cursor: not-allowed; }
</style>
