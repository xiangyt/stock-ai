<template>
  <div class="dc-mgmt-page">
    <header class="page-header">
      <h1>📡 数据采集</h1>
      <p>管理后台数据采集定时任务（仅管理员可访问）</p>
    </header>

    <!-- 加载中 -->
    <div v-if="loading" class="loading-state">加载任务列表...</div>

    <!-- 错误 -->
    <div v-if="errorMsg" class="error-msg">{{ errorMsg }}</div>

    <!-- 任务表格 -->
    <table v-if="!loading && tasks.length > 0" class="dc-table">
      <thead>
        <tr>
          <th>ID</th>
          <th>名称</th>
          <th>Cron 表达式</th>
          <th>执行参数</th>
          <th>推送机器人</th>
          <th>状态</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="task in tasks" :key="task.id" :class="{ 'disabled-row': !task.is_active }">
          <td class="id-cell">{{ task.id }}</td>
          <td class="name-cell">{{ task.name }}</td>
          <td>
            <span class="cron-tag">{{ task.cron_expr }}</span>
          </td>
          <td>
            <span class="params-tag" :title="task.params">{{ formatParams(task.params) }}</span>
          </td>
          <td>
            <div v-if="task.bots && task.bots.length > 0" class="bot-tags">
              <span v-for="bot in task.bots" :key="bot.id" class="bot-tag">
                <span class="bot-channel" :class="'channel-' + bot.channel">{{ ChannelLabels[bot.channel as ChannelType] || bot.channel }}</span>
                <span class="bot-name">{{ bot.name }}</span>
              </span>
            </div>
            <span v-else class="no-bots">-</span>
          </td>
          <td>
            <span :class="'status-tag status-' + (task.is_active ? 'on' : 'off')">
              {{ task.is_active ? '已启用' : '已禁用' }}
            </span>
          </td>
          <td class="actions-cell">
            <button class="btn-sm btn-info" :disabled="executingId === task.id" @click="onExecute(task)" title="立即执行">
              {{ executingId === task.id ? '执行中...' : '执行' }}
            </button>
            <button class="btn-sm btn-ok" @click="openEditModal(task)" title="配置">配置</button>
            <button
              v-if="task.is_active"
              class="btn-sm btn-warn"
              :disabled="togglingId === task.id"
              @click="onToggleActive(task, false)"
              title="禁用"
            >禁用</button>
            <button
              v-else
              class="btn-sm btn-success"
              :disabled="togglingId === task.id"
              @click="onToggleActive(task, true)"
              title="启用"
            >启用</button>
          </td>
        </tr>
      </tbody>
    </table>

    <p v-if="!loading && tasks.length === 0" class="empty-hint">暂无数据采集任务</p>

    <!-- ====== 配置弹窗 ====== -->
    <teleport to="body">
      <div v-if="showFormModal" class="modal-overlay" @click.self="closeFormModal">
        <div class="modal-box form-modal">
          <h3>配置任务 — {{ editingTask?.name }}</h3>

          <div class="form-group">
            <label>Cron 表达式 <span class="required">*</span></label>
            <input
              v-model="editForm.cron_expr"
              type="text"
              placeholder="0 */30 * * * *"
            />
            <p class="field-hint">6 段秒级 Cron 表达式：秒 分 时 日 月 周</p>
          </div>

          <div class="form-group">
            <label>执行参数（JSON 格式）<span class="required">*</span></label>
            <textarea
              v-model="editForm.params"
              rows="4"
              placeholder='{"source": "eastmoney"}'
            ></textarea>
            <p class="field-hint" :class="{ 'field-error': paramsError }">{{ paramsError || '合法的 JSON 格式' }}</p>
          </div>

          <div class="checkbox-group">
            <span class="checkbox-label">
              <input
                type="checkbox"
                v-model="editForm.is_active"
              />
              启用此任务
            </span>
          </div>

          <div class="form-group">
            <label>推送机器人（可多选）</label>
            <SearchMultiSelect
              v-model="editForm.bot_ids"
              :options="botOptions"
              placeholder="选择推送机器人..."
              empty-text="没有可用的机器人"
              :max-count="5"
            >
              <template #option="{ option }">
                <span class="bot-channel-opt" :class="'channel-' + (option.channel || '')">
                  {{ ChannelLabels[option.channel as ChannelType] || option.channel }}
                </span>
                <span class="ms-option-label">{{ option.label }}</span>
              </template>
            </SearchMultiSelect>
          </div>

          <div v-if="saveError" class="error-msg">{{ saveError }}</div>

          <div class="modal-actions">
            <button class="btn-modal-cancel" @click="closeFormModal">取消</button>
            <button
              class="btn-login-sm"
              :disabled="saving"
              @click="onSave"
            >{{ saving ? '保存中...' : '保存' }}</button>
          </div>
        </div>
      </div>
    </teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import {
  listDataCollectTasks,
  updateDataCollectTask,
  updateDataCollectBots,
  executeDataCollectTask,
  type DataCollectTaskItem,
  type BotInfo,
} from '../api/datacollect'
import { listPushBots, type PushBotItem, type ChannelType, ChannelLabels } from '../api/bot'
import SearchMultiSelect from './SearchMultiSelect.vue'

// ========== 状态 ==========

const loading = ref(true)
const errorMsg = ref('')
const tasks = ref<DataCollectTaskItem[]>([])
const togglingId = ref<number | null>(null)
const executingId = ref<number | null>(null)

// 弹窗
const showFormModal = ref(false)
const editingTask = ref<DataCollectTaskItem | null>(null)
const saving = ref(false)
const saveError = ref('')
const paramsError = ref('')

const editForm = ref({
  cron_expr: '',
  params: '',
  is_active: true,
  bot_ids: [] as number[],
})

// 机器人选项
const botItems = ref<PushBotItem[]>([])
const botOptions = computed(() =>
  botItems.value.map((b) => ({
    value: b.id,
    label: b.name,
    channel: b.channel,
  }))
)

// ========== 辅助函数 ==========

function formatParams(params: string): string {
  if (!params) return '-'
  try {
    const obj = JSON.parse(params)
    const short = JSON.stringify(obj)
    return short.length > 50 ? short.slice(0, 47) + '...' : short
  } catch {
    return params.length > 50 ? params.slice(0, 47) + '...' : params
  }
}

function getBotIDs(bots: BotInfo[]): number[] {
  return bots.map((b) => b.id)
}

// ========== 数据加载 ==========

async function loadTasks() {
  loading.value = true
  errorMsg.value = ''
  try {
    tasks.value = await listDataCollectTasks()
  } catch (e: any) {
    errorMsg.value = '加载失败: ' + (e.message || '未知错误')
  } finally {
    loading.value = false
  }
}

async function loadBots() {
  try {
    botItems.value = await listPushBots()
  } catch {
    // 静默失败
  }
}

// ========== 启停切换 ==========

async function onToggleActive(task: DataCollectTaskItem, active: boolean) {
  togglingId.value = task.id
  errorMsg.value = ''
  try {
    await updateDataCollectTask(task.id, { is_active: active })
    task.is_active = active
  } catch (e: any) {
    errorMsg.value = '操作失败: ' + (e.message || '未知错误')
  } finally {
    togglingId.value = null
  }
}

// ========== 立即执行 ==========

async function onExecute(task: DataCollectTaskItem) {
  executingId.value = task.id
  errorMsg.value = ''
  try {
    await executeDataCollectTask(task.id)
  } catch (e: any) {
    errorMsg.value = '执行失败: ' + (e.message || '未知错误')
  } finally {
    executingId.value = null
  }
}

// ========== 配置弹窗 ==========

function openEditModal(task: DataCollectTaskItem) {
  editingTask.value = task
  editForm.value = {
    cron_expr: task.cron_expr,
    params: task.params,
    is_active: task.is_active,
    bot_ids: getBotIDs(task.bots),
  }
  paramsError.value = ''
  saveError.value = ''
  showFormModal.value = true
}

function closeFormModal() {
  showFormModal.value = false
  editingTask.value = null
  saveError.value = ''
  paramsError.value = ''
}

// 实时校验 JSON
watch(
  () => editForm.value.params,
  (val) => {
    if (!val || val.trim() === '') {
      paramsError.value = '参数不能为空'
      return
    }
    try {
      JSON.parse(val)
      paramsError.value = ''
    } catch {
      paramsError.value = 'JSON 格式无效'
    }
  },
  { immediate: false }
)

async function onSave() {
  if (!editingTask.value) return

  // 校验
  if (!editForm.value.cron_expr.trim()) {
    saveError.value = 'Cron 表达式不能为空'
    return
  }
  if (paramsError.value) {
    saveError.value = '请修正 JSON 格式错误'
    return
  }

  saving.value = true
  saveError.value = ''

  try {
    const id = editingTask.value.id

    // 更新任务配置
    const updated = await updateDataCollectTask(id, {
      cron_expr: editForm.value.cron_expr,
      params: editForm.value.params,
      is_active: editForm.value.is_active,
    })

    // 更新关联机器人
    await updateDataCollectBots(id, editForm.value.bot_ids)

    // 更新本地数据
    const idx = tasks.value.findIndex((t) => t.id === id)
    if (idx >= 0) {
      tasks.value[idx] = updated
      // 刷新机器人列表
      await loadTasks()
    }

    closeFormModal()
  } catch (e: any) {
    saveError.value = '保存失败: ' + (e.message || '未知错误')
  } finally {
    saving.value = false
  }
}

// ========== 生命周期 ==========

onMounted(async () => {
  await Promise.all([loadTasks(), loadBots()])
})
</script>

<style scoped>
.dc-mgmt-page {}
.loading-state { text-align: center; color: #999; padding: 60px 0; font-size: 14px; }

/* ====== 表格 ====== */
.dc-table {
  width: 100%; border-collapse: collapse;
  background: #fff; border-radius: 12px; overflow: hidden;
  box-shadow: 0 1px 4px rgba(0,0,0,.06);
}
.dc-table th {
  background: #fafafa; padding: 10px 14px; text-align: center;
  font-size: 13px; font-weight: 600; color: #666; border-bottom: 1px solid #eee;
  white-space: nowrap;
}
.dc-table td {
  padding: 10px 14px; font-size: 13.5px; border-bottom: 1px solid #f3f3f3;
  vertical-align: middle; text-align: center;
}
.dc-table tr:hover td { background: #f9fbff; }
.dc-table tr.disabled-row td { opacity: .45; }

.id-cell { font-weight: 600; color: #888; font-family: 'SF Mono', Monaco, monospace; font-size: 12px; }
.name-cell { font-weight: 600; color: #333; max-width: 140px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.actions-cell { white-space: nowrap; }

/* Cron 标签 */
.cron-tag {
  display: inline-block; padding: 2px 10px; border-radius: 10px;
  font-size: 12px; font-weight: 500;
  background: #f5f5f5; color: #555;
  font-family: 'SF Mono', Monaco, monospace;
}

/* 参数标签 */
.params-tag {
  display: inline-block; padding: 2px 10px; border-radius: 10px;
  font-size: 12px; font-weight: 500;
  background: #f5f5f5; color: #555;
  font-family: 'SF Mono', Monaco, monospace;
  max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  cursor: default;
}

/* 状态标签 */
.status-tag {
  display: inline-block; padding: 2px 10px; border-radius: 10px;
  font-size: 11.5px; font-weight: 600;
}
.status-on { background: #f0fdf4; color: #16a34a; }
.status-off { background: #fef2f2; color: #dc2626; }

/* 表格中的机器人标签 */
.bot-tags { display: flex; flex-wrap: wrap; gap: 4px; justify-content: center; }
.bot-tag {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 1px 6px; border-radius: 4px;
  background: #f0f7ff; font-size: 11.5px; font-weight: 500;
}
.bot-tag .bot-channel { padding: 0 4px; border-radius: 3px; font-size: 10px; font-weight: 600; }
.bot-tag .bot-name { color: #1677ff; }

.no-bots { color: #bbb; }

/* 渠道标签（与 push-table 统一）*/
.channel-wecom { background: #f0f5ff; color: #2f54eb; }
.channel-dingtalk { background: #e8f8ff; color: #0891c5; }
.channel-feishu { background: #e6fffb; color: #13c2c2; }

/* 按钮 */
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
.btn-success { color: #52c41a; border-color: #b7eb8f; background: #f6ffed; }
.btn-success:hover:not(:disabled) { background: #d9f7be; }
.btn-info { color: #555; }
.btn-info:hover:not(:disabled) { border-color: #aaa; background: #f5f5f5; }

.empty-hint { text-align: center; color: #bbb; padding: 60px 0; font-size: 14px; }
.error-msg { color: #cf1322; font-size: 13px; margin-bottom: 10px; text-align: center; min-height: 20px; }
.required { color: #cf1322; }
.field-hint { font-size: 11.5px; color: #aaa; margin-top: 3px; line-height: 1.4; }
.field-error { color: #dc2626 !important; }

/* ====== Modal ====== */
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
.form-group textarea {
  width: 100%; padding: 10px 12px; border: 1.5px solid #d9d9d9; border-radius: 8px;
  font-size: 14px; outline: none; box-sizing: border-box;
  background: #fff; color: #333;
}
.form-group input[type="text"]:focus,
.form-group textarea:focus { border-color: #1677ff; box-shadow: 0 0 0 3px rgba(22,119,255,.08); }
.form-group textarea { resize: vertical; font-family: 'SF Mono', Monaco, monospace; }

.checkbox-group { margin: 16px 0; }
.checkbox-label {
  display: flex; align-items: center; gap: 10px;
  font-size: 14px; font-weight: 500; color: #333;
}
.checkbox-label input[type="checkbox"] {
  width: 16px; height: 16px; accent-color: #1677ff; cursor: pointer;
  margin: 0; flex-shrink: 0;
}

/* 机器人多选中的渠道标签 */
.bot-channel-opt {
  padding: 0 5px; border-radius: 3px; font-size: 10px; font-weight: 600;
  margin-right: 6px; flex-shrink: 0;
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
</style>
