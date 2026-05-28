<template>
  <div class="push-mgmt-page">
    <header class="page-header">
      <div class="header-meta">
        <h1>🤖 机器人配置</h1>
        <p>配置消息推送机器人，接收策略信号通知</p>
      </div>
      <div class="header-actions">
        <div class="sl-search">
          <input
            v-model="searchQuery"
            type="text"
            placeholder="搜索机器人名称"
            class="search-input"
            @keyup.enter="onSearch"
          />
          <button class="search-btn" @click="onSearch">🔍</button>
        </div>
        <button class="btn-add" @click="openCreateModal">+ 添加机器人</button>
      </div>
    </header>

    <!-- 加载中 -->
    <div v-if="loading" class="loading-state">加载机器人配置...</div>

    <!-- 机器人表格 -->
    <table v-else-if="filteredBots.length > 0" class="push-table">
      <thead>
        <tr>
          <th>ID</th>
          <th>名称</th>
          <th>渠道</th>
          <th>目标地址</th>
          <th>状态</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="bot in filteredBots" :key="bot.id" :class="{ 'disabled-row': bot.status === 0 }">
          <td>{{ bot.id }}</td>
          <td class="name-cell">{{ bot.name }}</td>
          <td>
            <span :class="'channel-tag channel-' + bot.channel">
              {{ ChannelLabels[bot.channel] || bot.channel }}
            </span>
          </td>
          <td class="url-cell" :data-tooltip="bot.webhook_url || bot.token || '-'">
            <span class="url-text">{{ formatTarget(bot) }}</span>
          </td>
          <td>
            <span :class="'status-tag status-' + (bot.status === 1 ? 'on' : 'off')">
              {{ bot.status === 1 ? '已启用' : '已禁用' }}
            </span>
          </td>
          <td class="actions-cell">
            <button class="btn-sm btn-info" @click="onTest(bot)" title="测试推送">测试</button>
            <button class="btn-sm btn-ok" @click="openEditModal(bot)" title="编辑">编辑</button>
            <button
              v-if="bot.status === 1"
              class="btn-sm btn-warn"
              @click="onToggleStatus(bot, 0)"
              title="禁用"
            >禁用</button>
            <button
              v-else
              class="btn-sm btn-success"
              @click="onToggleStatus(bot, 1)"
              title="启用"
            >启用</button>
            <button class="btn-sm btn-danger" @click="onDelete(bot)" title="删除">删除</button>
          </td>
        </tr>
      </tbody>
    </table>

    <p v-else class="empty-hint">
      {{ bots.length === 0 ? '暂无机器人配置，点击上方「添加机器人」开始配置' : '没有匹配的机器人' }}
    </p>

    <!-- ====== 添加/编辑弹窗 ====== -->
    <teleport to="body">
      <div v-if="showFormModal" class="modal-overlay" @click.self="closeFormModal">
        <div class="modal-box form-modal">
          <h3>{{ isEditing ? '编辑机器人 — ' + editTarget?.name : '添加推送机器人' }}</h3>

          <div class="form-group">
            <label>名称 <span class="required">*</span></label>
            <input
              ref="formNameInput"
              v-model="formData.name"
              type="text"
              placeholder="给机器人起个名字"
              maxlength="50"
              @keyup.enter="confirmForm"
            />
          </div>

          <div class="form-group">
            <label>渠道 <span class="required">*</span></label>
            <select v-model="formData.channel" :disabled="isEditing">
              <option value="">请选择渠道</option>
              <option value="wecom">企微（企业微信）</option>
              <option value="dingtalk">钉钉</option>
              <option value="feishu">飞书</option>
            </select>
          </div>

          <!-- 企微 / 飞书: Webhook URL -->
          <div v-if="formData.channel === 'wecom' || formData.channel === 'feishu'" class="form-group">
            <label>Webhook 地址 <span class="required">*</span></label>
            <input
              v-model="formData.webhook_url"
              type="text"
              placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
            />
          </div>

          <!-- 钉钉: Webhook + Secret -->
          <template v-if="formData.channel === 'dingtalk'">
            <div class="form-group">
              <label>Webhook 地址 <span class="required">*</span></label>
              <input
                v-model="formData.webhook_url"
                type="text"
                placeholder="https://oapi.dingtalk.com/robot/send?access_token=xxx"
              />
            </div>
            <div class="form-group">
              <label>加签密钥（可选）</label>
              <input
                v-model="formData.secret"
                type="text"
                placeholder="钉钉机器人加签 Secret，留空则不使用加签"
              />
            </div>
          </template>

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

    <!-- ====== 测试推送弹窗 ====== -->
    <teleport to="body">
      <div v-if="showTestModal" class="modal-overlay" @click.self="showTestModal = false">
        <div class="modal-box test-modal">
          <h3>🧪 测试推送 — {{ testTarget?.name }}</h3>
          <div v-if="testLoading" class="test-loading">正在发送测试消息...</div>
          <div v-else class="test-result" :class="{ success: testResult?.success, fail: !testResult?.success }">
            <span class="result-icon">{{ testResult?.success ? '✅' : '❌' }}</span>
            <p>{{ testResult?.message || '未知错误' }}</p>
          </div>
          <div class="modal-actions">
            <button class="btn-modal-cancel" @click="showTestModal = false">{{ testResult ? '关闭' : '取消' }}</button>
            <button
              v-if="!testLoading && (!testResult || !testResult.success)"
              class="btn-login-sm"
              @click="retryTest"
            >重试</button>
          </div>
        </div>
      </div>
    </teleport>
    <!-- ====== 确认弹窗（删除 / 禁用-启用）====== -->
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
import { ref, computed, onMounted } from 'vue'
import * as pushApi from '../api/bot'
import type { PushBotItem, CreatePushBotReq, UpdatePushBotReq } from '../api/bot'
import { ChannelLabels } from '../api/bot'

// ========== 数据状态 ==========
const bots = ref<PushBotItem[]>([])
const loading = ref(false)

// ========== 搜索过滤 ==========
const searchQuery = ref('')
const filteredBots = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return bots.value
  return bots.value.filter(b => b.name.toLowerCase().includes(q))
})
function onSearch() {
  // 前端过滤，无需调接口
}

// ---- 表单弹窗 ----
const showFormModal = ref(false)
const isEditing = ref(false)
const editTarget = ref<PushBotItem | null>(null)
const submitting = ref(false)
const formError = ref('')
const formNameInput = ref<HTMLInputElement | null>(null)

// 表单数据初始化
function emptyForm(): CreatePushBotReq & UpdatePushBotReq {
  return { name: '', channel: '' as any, webhook_url: '', token: '', secret: '' }
}
const formData = ref<CreatePushBotReq & UpdatePushBotReq>(emptyForm())

// 表单验证
const isFormValid = computed(() => {
  const d = formData.value
  if (!d.name.trim() || !d.channel) return false
  // 所有渠道: Webhook 必填
  return !!d.webhook_url?.trim()
})

// ---- 测试推送弹窗 ----
const showTestModal = ref(false)
const testTarget = ref<PushBotItem | null>(null)
const testLoading = ref(false)
const testResult = ref<{ success: boolean; message: string } | null>(null)

// ---- 通用确认弹窗 ----
const showConfirmModal = ref(false)
const confirmTitle = ref('')
const confirmMessage = ref('')
const confirmOkText = ref('确定')
const confirmIsDanger = ref(false)
const confirmLoading = ref(false)
const confirmError = ref('')
let confirmAction: (() => Promise<void>) | null = null

/** 打开确认弹框 */
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

/** 关闭确认弹框 */
function closeConfirmModal() {
  showConfirmModal.value = false
  confirmAction = null
}

/** 执行确认操作 */
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

// ========== 数据操作 ==========

async function loadBots() {
  loading.value = true
  try {
    bots.value = await pushApi.listPushBots()
  } catch (e) {
    console.error('加载机器人配置失败:', e)
  } finally {
    loading.value = false
  }
}

// ---- 创建/编辑 ----

function openCreateModal() {
  isEditing.value = false
  editTarget.value = null
  formData.value = emptyForm()
  formError.value = ''
  showFormModal.value = true
}

function openEditModal(bot: PushBotItem) {
  isEditing.value = true
  editTarget.value = bot
  formData.value = {
    name: bot.name,
    channel: bot.channel,
    webhook_url: bot.webhook_url,
    token: bot.token,
    secret: bot.secret,
  }
  formError.value = ''
  showFormModal.value = true
}

function closeFormModal() {
  showFormModal.value = false
  editTarget.value = null
  formData.value = emptyForm()
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
      await pushApi.updatePushBot(editTarget.value.id, { ...formData.value })
    } else {
      await pushApi.createPushBot({ ...formData.value })
    }
    closeFormModal()
    await loadBots()
  } catch (e: any) {
    formError.value = e.message || '操作失败'
  } finally {
    submitting.value = false
  }
}

// ---- 删除 ----

function onDelete(bot: PushBotItem) {
  openConfirm({
    title: '删除机器人',
    message: `确定删除机器人「${bot.name}」？删除后无法恢复。`,
    okText: '删除',
    isDanger: true,
    onOk: async () => {
      await pushApi.deletePushBot(bot.id)
      await loadBots()
    },
  })
}

// ---- 状态切换 ----

function onToggleStatus(bot: PushBotItem, newStatus: number) {
  const action = newStatus === 1 ? '启用' : '禁用'
  openConfirm({
    title: `${action}机器人`,
    message: `确定${action}机器人「${bot.name}」？`,
    okText: action,
    isDanger: newStatus === 0,
    onOk: async () => {
      await pushApi.togglePushBotStatus(bot.id, newStatus)
      await loadBots()
    },
  })
}

// ---- 测试推送 ----

async function onTest(bot: PushBotItem) {
  testTarget.value = bot
  testResult.value = null
  testLoading.value = true
  showTestModal.value = true

  try {
    testResult.value = await pushApi.testPushBot(bot.id)
  } catch (e: any) {
    testResult.value = { success: false, message: e.message || '请求异常' }
  } finally {
    testLoading.value = false
  }
}

function retryTest() {
  if (testTarget.value) {
    testResult.value = null
    onTest(testTarget.value)
  }
}

// ========== 工具函数 ==========

/** 格式化目标地址显示 */
function formatTarget(bot: PushBotItem): string {
  const url = bot.webhook_url || ''
  if (!url) return '-'

  try {
    const u = new URL(url)
    return u.hostname + u.pathname.slice(0, 20) + (u.pathname.length > 20 ? '...' : '')
  } catch {
    return url.length > 35 ? url.slice(0, 35) + '...' : url
  }
}

onMounted(loadBots)
</script>

<style scoped>
.push-mgmt-page {}
.loading-state { text-align: center; color: #999; padding: 60px 0; font-size: 14px; }

/* ====== 头部布局 ====== */
.page-header {
  display: flex; justify-content: space-between; align-items: center;
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

/* ====== 表格 ====== */
.push-table {
  width: 100%; border-collapse: collapse;
  background: #fff; border-radius: 12px; overflow: hidden;
  box-shadow: 0 1px 4px rgba(0,0,0,.06);
}
.push-table th {
  background: #fafafa; padding: 10px 14px; text-align: center;
  font-size: 13px; font-weight: 600; color: #666; border-bottom: 1px solid #eee;
}
.push-table td {
  padding: 10px 14px; font-size: 13.5px; border-bottom: 1px solid #f3f3f3;
  text-align: center;
}
.push-table tr:hover td { background: #f9fbff; }
.push-table tr.disabled-row td { opacity: .45; }

.name-cell { font-weight: 600; color: #333; }
.url-cell {
  max-width: 220px;
  position: relative;
}
.url-text {
  display: block; overflow: hidden;
  text-overflow: ellipsis; white-space: nowrap;
  color: #888; font-size: 12.5px;
}
/* hover 气泡提示 */
.url-cell:hover::after {
  content: attr(data-tooltip);
  position: absolute;
  bottom: 100%;
  left: 50%;
  transform: translateX(-50%);
  background: #333;
  color: #fff;
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 12px;
  white-space: nowrap;
  z-index: 100;
  margin-bottom: 6px;
  pointer-events: none;
}
.url-cell:hover::before {
  content: '';
  position: absolute;
  bottom: 100%;
  left: 50%;
  transform: translateX(-50%);
  border: 5px solid transparent;
  border-top-color: #333;
  z-index: 101;
  margin-bottom: -4px;
  pointer-events: none;
}

.actions-cell { white-space: nowrap; }

/* 渠道标签 */
.channel-tag {
  display: inline-block; padding: 2px 10px; border-radius: 10px;
  font-size: 11.5px; font-weight: 600;
}
.channel-wecom { background: #f0f5ff; color: #2f54eb; }
.channel-dingtalk { background: #e8f8ff; color: #0891c5; }
.channel-feishu { background: #e6fffb; color: #13c2c2; }

/* 状态标签 */
.status-tag {
  display: inline-block; padding: 2px 10px; border-radius: 10px;
  font-size: 11.5px; font-weight: 600;
}
.status-on { background: #f0fdf4; color: #16a34a; }
.status-off { background: #fef2f2; color: #dc2626; }

/* 按钮 */
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
.btn-ok { color: #1677ff; border-color: #91caff; background: #f0f7ff; }
.btn-ok:hover { background: #d6e8ff; }
.btn-success { color: #52c41a; border-color: #b7eb8f; background: #f6ffed; }
.btn-success:hover { background: #d9f7be; }
.btn-warn { color: #ff6b00; border-color: #ffd591; background: #fff7e6; }
.btn-warn:hover { background: #ffe7ba; }
.btn-danger { color: #cf1322; border-color: #ffa39e; background: #fff1f0; }
.btn-danger:hover { background: #ffccc7; }
.btn-info { color: #555; }
.btn-info:hover { border-color: #aaa; background: #f5f5f5; }

.empty-hint { text-align: center; color: #bbb; padding: 60px 0; font-size: 14px; }
.error-msg { color: #cf1322; font-size: 13px; margin-bottom: 10px; text-align: center; min-height: 20px; }
.required { color: #cf1322; }
.field-hint { font-size: 11.5px; color: #aaa; margin-top: 3px; line-height: 1.4; }

/* ====== Modal（与账户管理一致）====== */
.modal-overlay {
  position: fixed; inset: 0; z-index: 999; background: rgba(0,0,0,.35);
  display: flex; align-items: center; justify-content: center;
}
.modal-box {
  background: #fff; border-radius: 16px; padding: 28px 32px; width: 420px;
  box-shadow: 0 4px 24px rgba(0,0,0,.12);
}
.modal-box h3 { font-size: 17px; margin-bottom: 18px; text-align: center; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

.form-modal { width: 440px; max-height: 85vh; overflow-y: auto; }
.form-group { margin-bottom: 16px; }
.form-group label {
  display: block; font-size: 13px; font-weight: 600; color: #555; margin-bottom: 6px;
}
.form-group input,
.form-group select {
  width: 100%; padding: 10px 12px; border: 1.5px solid #d9d9d9; border-radius: 8px;
  font-size: 14px; outline: none; box-sizing: border-box;
  background: #fff; color: #333;
}
.form-group input:focus,
.form-group select:focus { border-color: #1677ff; box-shadow: 0 0 0 3px rgba(22,119,255,.08); }
.form-group select { cursor: pointer; appearance: auto; }

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

/* ====== 测试结果 ====== */
.test-modal { width: 380px; text-align: center; }
.test-loading { padding: 24px 0; color: #999; font-size: 14px; }
.test-result { padding: 20px 0; }
.result-icon { font-size: 36px; display: block; margin-bottom: 10px; }
.test-result p { font-size: 14px; word-break: break-all; }
.test-result.success p { color: #16a34a; }
.test-result.fail p { color: #cf1322; }

/* ====== 确认弹框 ====== */
.confirm-modal { width: 380px; text-align: center; }
.confirm-msg {
  font-size: 14.5px; color: #555; line-height: 1.6;
  padding: 8px 0 4px; word-break: break-all;
}
.btn-danger-sm {
  padding: 7px 22px; font-size: 14px; font-weight: 600;
  color: #fff; background: #cf1322; border: none; border-radius: 6px; cursor: pointer;
}
.btn-danger-sm:hover:not(:disabled) { background: #a80f1c; }
.btn-danger-sm:disabled { opacity: .5; cursor: not-allowed; }
</style>
