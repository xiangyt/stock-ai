<template>
  <div class="strategy-list-page">
    <!-- ====== 头部：标题（与账户管理/个人主页统一风格）====== -->
    <header class="page-header">
      <h1>📋 策略列表</h1>
      <p>管理你的选股策略、回测与信号配置</p>
    </header>

    <!-- ====== 工具栏：左侧按钮 + 右侧搜索 ====== -->
    <div class="sl-toolbar">
      <div class="toolbar-left">
        <button class="btn-primary" @click="$emit('goNew')">＋ 新建策略</button>
        <!-- 选中后显示的操作按钮 -->
        <template v-if="selectedIds.size > 0">
          <button v-if="selectedIds.size === 1" class="btn-outline btn-rename" @click="renameSelected" title="重命名选中项">✏ 重命名</button>
          <button class="btn-outline btn-danger" @click="batchDelete" title="删除选中">✂ 删除</button>
        </template>
      </div>
      <div class="toolbar-right">
        <div class="sl-search">
          <input
            v-model="searchQuery"
            type="text"
            placeholder="策略名称"
            class="search-input"
          />
          <button class="search-btn" @click="onSearch">🔍</button>
        </div>
      </div>
    </div>

    <!-- ====== 空状态 ====== -->
    <div v-if="strategies.length === 0" class="empty-state-outer">
      <div class="empty-content">
        <div class="empty-icon">📭</div>
        <p>还没有保存的策略</p>
        <button class="btn-create-sm" @click="$emit('goNew')">➕ 创建第一个策略</button>
      </div>
    </div>

    <!-- ====== 表格视图 ====== -->
    <div v-else-if="viewMode === 'table'" class="table-area">
      <div class="table-label">全部策略</div>

      <table class="strategy-table">
        <thead>
          <tr>
            <th class="col-check">
              <input
                type="checkbox"
                :checked="isAllChecked"
                @change="toggleSelectAll"
              />
            </th>
            <th class="col-name">策略名称</th>
            <th class="col-backtest">回测次数</th>
            <th class="col-last-run">最后运行时间</th>
            <th class="col-public">是否公开</th>
            <th class="col-created">创建时间</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="s in pagedStrategies"
            :key="s.id"
            :class="{ selected: selectedIds.has(s.id) }"
            @click="toggleSelect(s.id)"
          >
            <td class="col-check" @click.stop>
              <input type="checkbox" :checked="selectedIds.has(s.id)" @change="toggleSelect(s.id)" />
            </td>
            <td class="col-name" @click.stop>
              <div class="name-cell">
                <span class="file-icon">📄</span>
                <template v-if="editingId === s.id && selectedIds.has(s.id)">
                  <input
                    ref="nameInputEl"
                    v-model="editingName"
                    class="name-edit-input"
                    @keyup.enter="saveRename(s)"
                    @keyup.escape="cancelRename"
                    @click.stop
                  />
                  <button class="name-action-btn ok" @click.stop="saveRename(s)" title="确认">✓</button>
                  <button class="name-action-btn cancel" @click.stop="cancelRename" title="取消">✕</button>
                </template>
                <template v-else>
                  <span class="name-text" @click="$emit('load', s)" @dblclick.stop="startEditName(s)" title="点击进入详情，双击编辑名称">{{ s.name }}</span>
                </template>
              </div>
            </td>
            <td class="col-backtest">{{ s.backtestCount ?? 0 }}</td>
            <td class="col-last-run">{{ s.lastRunAt ? formatTimeFull(s.lastRunAt) : '—' }}</td>
            <td class="col-public">
              <span :class="['public-tag', s.isPublic ? 'tag-yes' : 'tag-no']">{{ s.isPublic ? '公开' : '私有' }}</span>
            </td>
            <td class="col-created">{{ formatTimeFull(s.createdAt) }}</td>
          </tr>
        </tbody>
      </table>

      <div v-if="strategies.length > pageSize" class="pagination-bar">
        <select v-model.number="pageSize" class="page-size-select">
          <option :value="10">展示 10 行</option>
          <option :value="20">展示 20 行</option>
          <option :value="50">展示 50 行</option>
          <option :value="100">展示 100 行</option>
        </select>
      </div>
    </div>

    <!-- ====== 卡片列表视图 ====== -->
    <div v-else class="card-grid-area">
      <transition-group name="card-list" tag="div" class="card-grid">
        <div
          v-for="s in pagedStrategies"
          :key="s.id"
          class="strategy-card"
          :class="{ selected: selectedIds.has(s.id) }"
          @click="$emit('load', s)"
        >
          <!-- 顶部：选择 + 名称 + 操作 -->
          <div class="card-top">
            <label class="card-check" @click.stop>
              <input type="checkbox" :checked="selectedIds.has(s.id)" @change="toggleSelect(s.id)" />
            </label>
            <span class="card-file-icon">📄</span>
            <span class="card-name-text">{{ s.name }}</span>
            <div class="card-top-actions">
              <button class="card-action-sm edit-btn" @click.stop="$emit('load', s)" title="编辑详情">✏️</button>
              <button class="card-action-sm del-btn" @click.stop="handleDeleteSingle(s.id, s.name)" title="删除">🗑️</button>
            </div>
          </div>
          <!-- 分类 + 统计信息 -->
          <div class="card-meta-row">
            <span class="category-tag">{{ getCategoryLabel(s) }}</span>
            <span class="meta-badge">{{ s.logicalOp }} · {{ s.signals.length }} 条信号</span>
          </div>
          <!-- 信号预览 chips -->
          <div class="card-signals">
            <div
              v-for="(sig, idx) in s.signals.slice(0, 5)"
              :key="idx"
              class="signal-chip"
              :class="'cat-' + sig.category"
            >
              <span class="chip-ind">{{ sig.name }}</span>
              <span class="chip-op">{{ sig.opSym }}</span>
              <span v-if="sig.paramText" class="chip-param">{{ sig.paramText }}</span>
            </div>
            <span v-if="s.signals.length > 5" class="more-chip">+{{ s.signals.length - 5 }}</span>
          </div>
          <!-- 底部：时间 + 回测计数 -->
          <div class="card-bottom">
            <span class="card-time">更新于 {{ formatTimeShort(s.updatedAt) }}</span>
            <div class="card-stats-mini">
              <span class="stat-pill bt-pill">回测 {{ s.backtestCount ?? 0 }}</span>
            </div>
          </div>
        </div>
      </transition-group>

      <!-- 分页 -->
      <div v-if="strategies.length > pageSize" class="pagination-bar">
        <select v-model.number="pageSize" class="page-size-select">
          <option :value="6">每页 6 个</option>
          <option :value="12">每页 12 个</option>
          <option :value="24">每页 24 个</option>
        </select>
      </div>
    </div>

    <!-- 删除确认弹窗 -->
    <teleport to="body">
      <div class="modal-overlay" v-if="deleteConfirm.show" @click.self="deleteConfirm.show = false">
        <div class="modal-box">
          <div class="modal-title">⚠️ 删除策略</div>
          <p class="modal-body">
            确定要删除 <strong>{{ deleteConfirm.count }}</strong> 个策略吗？此操作不可撤销。
          </p>
          <div class="modal-actions">
            <button class="btn-modal-cancel" @click="deleteConfirm.show = false">取消</button>
            <button class="btn-modal-danger" @click="confirmBatchDelete">确认删除</button>
          </div>
        </div>
      </div>
    </teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue'

export interface SavedStrategy {
  id: number
  name: string
  backtestCount: number
  lastRunAt: string | null
  isPublic: boolean
  createdAt: string
  updatedAt: string
}

const props = defineProps<{
  strategies: SavedStrategy[]
}>()

const emit = defineEmits<{
  load: [s: SavedStrategy]
  goNew: []
  deleted: [ids: number[]]
  rename: [id: number, newName: string]
  search: [keyword: string]
}>()

// ========== 视图模式 ==========
const viewMode = ref<'table' | 'list'>('table')

// ========== 搜索（后端搜索） ==========
const searchQuery = ref('')

function onSearch() {
  emit('search', searchQuery.value.trim())
}

// ========== 排序（按更新时间倒序，数据由后端搜索返回） ==========
const sortedStrategies = computed(() => {
  return [...props.strategies].sort((a, b) =>
    new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()
  )
})

// ========== 选择 ==========
const selectedIds = reactive(new Set<number>())
const editingId = ref<number | null>(null)
const editingName = ref('')

function toggleSelect(id: number) {
  if (selectedIds.has(id)) { selectedIds.delete(id) }
  else { selectedIds.add(id) }
}

function toggleSelectAll(e: Event) {
  const checked = (e.target as HTMLInputElement).checked
  if (checked) { for (const s of sortedStrategies.value) selectedIds.add(s.id) }
  else { selectedIds.clear() }
}

const isAllChecked = computed(() =>
  sortedStrategies.value.length > 0 && sortedStrategies.value.every(s => selectedIds.has(s.id))
)
const isIndeterminate = computed(() =>
  selectedIds.size > 0 && !isAllChecked.value
)

// ========== 名称编辑 ==========
function renameSelected() {
  if (selectedIds.size !== 1) return
  const id = [...selectedIds][0]
  const s = props.strategies.find(item => item.id === id)
  if (s) startEditName(s)
}

function startEditName(s: SavedStrategy) {
  selectedIds.add(s.id)
  editingId.value = s.id
  editingName.value = s.name
  setTimeout(() => {
    const els = document.querySelectorAll('.name-edit-input')
    const el = els[els.length - 1] as HTMLInputElement | undefined
    el?.focus(); el?.select()
  }, 50)
}

function saveRename(s: SavedStrategy) {
  const newName = editingName.value.trim()
  if (newName && newName !== s.name) emit('rename', s.id, newName)
  cancelRename()
}
function cancelRename() { editingId.value = null; editingName.value = '' }

// ========== 删除 ==========
const deleteConfirm = reactive({ show: false, ids: [] as number[], count: 0 })

function handleDeleteSingle(id: number, name: string) {
  deleteConfirm.ids = [id]
  deleteConfirm.count = 1
  deleteConfirm.show = true
}

function batchDelete() {
  if (selectedIds.size === 0) return
  deleteConfirm.ids = [...selectedIds]
  deleteConfirm.count = selectedIds.size
  deleteConfirm.show = true
}

function confirmBatchDelete() {
  emit('deleted', deleteConfirm.ids)
  selectedIds.clear()
  deleteConfirm.show = false
}

// ========== 分页 ==========
const pageSize = ref(10)
const pagedStrategies = computed(() => {
  return sortedStrategies.value.slice(0, pageSize.value)
})

// ========== 工具函数 ==========
function formatTimeFull(iso: string): string {
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) +
    ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds())
}

function formatTimeShort(iso: string): string {
  const d = new Date(iso)
  const pad = (n: number) => String(n).padStart(2, '0')
  return (d.getMonth()+1) + '/' + d.getDate() + ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes())
}

function getCategoryLabel(s: SavedStrategy): string {
  const cats = new Set(s.signals.map(sig => sig.category))
  if (cats.has('technical') && cats.size <= 2) return '技术面'
  if (cats.has('fundamental')) return '基本面'
  if (cats.has('market')) return '市场情绪'
  if (cats.has('financial')) return '财务指标'
  if (cats.size >= 3) return '综合'
  return 'Code'
}
</script>

<style scoped>
.strategy-list-page {
  max-width: 100%;
  display: flex;
  flex-direction: column;
  gap: 0;
}

/* ====== 头部（使用全局 .page-header 样式）====== */

.sl-search {
  display: flex; align-items: center;
  border: 1px solid #d9d9d9; border-radius: 6px;
  overflow: hidden; transition: border-color .15s;
}
.sl-search:focus-within { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.08); }
.search-input {
  border: none; outline: none; padding: 6px 12px;
  font-size: 13px; width: 200px; color: #333; background: transparent;
}
.search-input::placeholder { color: #bbb; }
.search-btn {
  border: none; background: transparent; cursor: pointer;
  padding: 6px 12px; font-size: 14px; border-left: 1px solid #eee; transition: background .12s;
}
.search-btn:hover { background: #f5f5f5; }

/* ====== 工具栏 ====== */
.sl-toolbar {
  display: flex; justify-content: space-between; align-items: center;
  margin-bottom: 14px; flex-wrap: wrap; gap: 8px;
}
.toolbar-left { display: flex; gap: 8px; align-items: center; }
.toolbar-right { display: flex; align-items: center; }

/* 视图切换 */
.view-toggle {
  display: flex; border: 1px solid #d9d9d9; border-radius: 6px; overflow: hidden;
}
.view-btn {
  padding: 5px 14px; border: none; font-size: 12.5px; cursor: pointer;
  background: #fff; color: #666; transition: .15s; white-space: nowrap;
}
.view-btn:first-child { border-right: 1px solid #d9d9d9; }
.view-btn.active { background: #1677ff; color: #fff; font-weight: 600; }
.view-btn:not(.active):hover { background: #f5f5f5; }

.btn-primary {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 7px 18px; font-size: 13px; font-weight: 600;
  color: #fff; background: #1677ff; border: 1px solid #1677ff;
  border-radius: 5px; cursor: pointer; transition: .15s; white-space: nowrap;
}
.btn-primary:hover { background: #0958d9; border-color: #0958d9; }
.btn-outline {
  padding: 7px 14px; font-size: 13px; font-weight: 500;
  color: #555; background: #fff; border: 1px solid #d9d9d9;
  border-radius: 5px; cursor: pointer; transition: .15s; white-space: nowrap;
}
.btn-outline:hover { border-color: #1677ff; color: #1677ff; }
.btn-danger:hover { border-color: #cf1322; color: #cf1322 !important; }
.btn-rename:hover { border-color: #d46b08; color: #d46b08 !important; }

.toolbar-right { font-size: 12.5px; color: #888; }
.link-apply { color: #1677ff; text-decoration: none; }
.link-apply:hover { text-decoration: underline; }

/* ====== 空状态 ====== */
.empty-state-outer {
  text-align: center; padding: 80px 20px;
  background: #fff; border: 1px solid #e8e8e8; border-radius: 10px;
}
.empty-content { text-align: center; }
.empty-icon { font-size: 48px; display: block; margin-bottom: 12px; }
.empty-content p { font-size: 14px; color: #999; margin-bottom: 12px; }
.btn-create-sm {
  padding: 8px 24px; font-size: 13px; font-weight: 600;
  color: #fff; background: #1677ff; border: 1px solid #1677ff;
  border-radius: 5px; cursor: pointer; transition: .15s;
}
.btn-create-sm:hover { background: #0958d9; }

/* ==================== 表格视图 ==================== */
.table-area {
  background: #fff; border: 1px solid #e8e8e8;
  border-radius: 8px; overflow: hidden;
}
.table-label {
  padding: 10px 16px; font-size: 13px; color: #666;
  border-bottom: 1px solid #f0f0f0; font-weight: 500;
}

.strategy-table {
  width: 100%; border-collapse: collapse; font-size: 13px;
  table-layout: fixed;
}
.strategy-table thead th {
  padding: 8px 10px; text-align: left; font-weight: 500;
  color: #666; background: #fafafa; border-bottom: 1px solid #eee;
  white-space: nowrap; font-size: 12px;
  position: sticky; top: 0; z-index: 2;
}
.strategy-table tbody tr {
  border-bottom: 1px solid #f5f5f5; cursor: pointer; transition: background .1s;
}
.strategy-table tbody tr:hover { background: #fafbff; }
.strategy-table tbody tr.selected { background: #e6f4ff; }
.strategy-table tbody td { padding: 8px 10px; vertical-align: middle; color: #333; }

.col-check { width: 32px; text-align: center; }
.col-check input[type="checkbox"] { accent-color: #1677ff; cursor: pointer; width: 14px; height: 14px; }
.col-name { min-width: auto; }
.col-backtest { min-width: auto; text-align: center; }
.col-last-run { min-width: 140px; }
.col-public { min-width: 80px; text-align: center; }
.col-created { min-width: 150px; }

/* 名称单元格 */
.name-cell { display: flex; align-items: center; gap: 8px; }
.file-icon { font-size: 17px; flex-shrink: 0; }
.name-text { font-weight: 500; color: #1a1a2e; cursor: pointer; transition: color .12s; }
.name-text:hover { color: #1677ff; }

/* 编辑输入 */
.name-edit-input {
  flex: 1; max-width: 240px; padding: 3px 8px;
  border: 1.5px solid #1677ff; border-radius: 4px;
  font-size: 13px; outline: none; color: #1a1a2e; font-weight: 500; background: #fff;
}
.name-edit-input:focus { box-shadow: 0 0 0 2px rgba(22,119,255,.08); }
.name-action-btn {
  border: none; background: transparent; cursor: pointer;
  font-size: 13px; padding: 2px 5px; border-radius: 3px; transition: .12s; line-height: 1;
}
.name-action-btn.ok { color: #52c41a; }
.name-action-btn.ok:hover { background: #f6ffed; }
.name-action-btn.cancel { color: #cf1322; }
.name-action-btn.cancel:hover { background: #fff1f0; }

/* 分类 tag */
.category-tag {
  display: inline-block; padding: 2px 12px; border-radius: 3px;
  font-size: 12px; font-weight: 600; color: #0958d9;
  background: #e6f4ff; border: 1px solid #bae0ff;
}
.col-backtest { font-family: 'SF Mono', Monaco, monospace; font-size: 13px; color: #666; text-align: center; }

/* 是否公开 tag */
.public-tag {
  display: inline-block; padding: 2px 10px; border-radius: 10px;
  font-size: 12px; font-weight: 600;
}
.tag-yes { color: #389e0d; background: #f6ffed; }
.tag-no { color: #8c8c8c; background: #f5f5f5; }

/* 分页 */
.pagination-bar {
  display: flex; justify-content: center; padding: 12px 16px;
  border-top: 1px solid #f0f0f0;
}
.page-size-select {
  padding: 4px 10px; border: 1px solid #d9d9d9; border-radius: 4px;
  font-size: 12.5px; color: #555; outline: none; cursor: pointer; background: #fff;
}
.page-size-select:focus { border-color: #1677ff; }

/* ==================== 卡片列表视图 ==================== */
.card-grid-area { width: 100%; }

.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 14px;
}

.strategy-card {
  background: #fff; border: 1.5px solid #e8e8e8; border-radius: 12px;
  padding: 16px 18px; cursor: pointer; transition: all .18s;
  display: flex; flex-direction: column; gap: 10px;
  position: relative;
}
.strategy-card:hover { border-color: #1677ff; box-shadow: 0 4px 16px rgba(22,119,255,.08); transform: translateY(-1px); }
.strategy-card.selected { border-color: #1677ff; background: #fbfdff; box-shadow: 0 0 0 2px rgba(22,119,255,.08); }

/* 顶部：checkbox + 名字 */
.card-top { display: flex; align-items: center; gap: 8px; }
.card-check input[type="checkbox"] { accent-color: #1677ff; cursor: pointer; width: 15px; height: 15px; }
.card-file-icon { font-size: 17px; flex-shrink: 0; }
.card-name-text {
  font-size: 15px; font-weight: 700; color: #1a1a2e;
  flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.card-top-actions { display: flex; gap: 2px; opacity: 0; transition: opacity .15s; flex-shrink: 0; }
.strategy-card:hover .card-top-actions { opacity: 1; }
.card-action-sm {
  border: none; background: transparent; cursor: pointer; font-size: 13px;
  padding: 3px 6px; border-radius: 4px; transition: .12s;
}
.edit-btn:hover { background: #e6f4ff; }
.del-btn:hover { background: #fff1f0; }

/* 元数据行 */
.card-meta-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.meta-badge { font-size: 11.5px; color: #999; }

/* 信号 chips */
.card-signals { display: flex; flex-wrap: wrap; gap: 4px; min-height: 26px; }
.signal-chip {
  display: inline-flex; align-items: center; gap: 2px;
  padding: 2px 7px; border-radius: 10px; font-size: 11px;
  background: #f5f5f5; max-width: 160px; overflow: hidden;
}
.signal-chip .chip-ind { font-weight: 600; color: #333; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.signal-chip .chip-op {
  font-family: monospace; font-weight: 700; font-size: 10px;
  padding: 1px 4px; border-radius: 3px; color: #fff;
  background: linear-gradient(135deg, #1677ff, #0958d9); flex-shrink: 0;
}
.signal-chip .chip-param {
  color: #666; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-size: 10.5px;
}
.signal-chip.cat-technical   { border-left: 3px solid #08979c; }
.signal-chip.cat-market     { border-left: 3px solid #0958d9; }
.signal-chip.cat-fundamental{ border-left: 3px solid #d46b08; }
.signal-chip.cat-financial  { border-left: 3px solid #52c41a; }
.more-chip { font-size: 11px; color: #bbb; font-style: italic; padding: 2px 6px; }

/* 底部 */
.card-bottom {
  display: flex; justify-content: space-between; align-items: center;
  padding-top: 8px; border-top: 1px solid #f0f0f0; margin-top: auto;
}
.card-time { font-size: 11.5px; color: #bbb; }
.card-stats-mini { display: flex; gap: 6px; }
.stat-pill {
  font-size: 10.5px; padding: 1px 8px; border-radius: 8px; font-weight: 600;
}
.bt-pill   { background: #fff7e6; color: #d46b08; }

/* 卡片动画 */
.card-list-enter-active { transition: all .28s cubic-bezier(.23,1,.32,1); }
.card-list-leave-active { transition: all .18s ease-in; }
.card-list-enter-from { opacity: 0; transform: scale(.96) translateY(8px); }
.card-list-leave-to { opacity: 0; transform: scale(.96) translateY(-6px); }

/* ====== Modal ====== */
.modal-overlay {
  position: fixed; inset: 0; z-index: 999; background: rgba(0,0,0,.35);
  display: flex; align-items: center; justify-content: center;
}
.modal-box {
  background: #fff; border-radius: 10px; padding: 24px; width: 380px; max-width: 90vw;
  box-shadow: 0 12px 40px rgba(0,0,0,.18);
}
.modal-title { font-size: 16px; font-weight: 700; margin-bottom: 10px; }
.modal-body { font-size: 14px; color: #666; line-height: 1.6; margin-bottom: 18px; }
.modal-body strong { color: #1a1a2e; }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; }
.btn-modal-cancel {
  padding: 7px 18px; border: 1px solid #d9d9d9; border-radius: 5px; background: #fff;
  font-size: 13px; cursor: pointer; color: #666; transition: .12s;
}
.btn-modal-cancel:hover { border-color: #aaa; }
.btn-modal-danger {
  padding: 7px 18px; border: none; border-radius: 5px; background: #cf1322; color: #fff;
  font-size: 13px; font-weight: 600; cursor: pointer;
}
.btn-modal-danger:hover { background: #a8071a; }

tbody tr { animation: fade-in .15s ease-out; }
@keyframes fade-in { from { opacity: 0; } to { opacity: 1; } }

/* 响应式 */
@media (max-width: 800px) {
  .sl-header { flex-direction: column; gap: 10px; align-items: stretch; }
  .sl-search { width: 100%; }
  .search-input { flex: 1; }
  .sl-toolbar { flex-direction: column; align-items: stretch; }
  .toolbar-center { justify-content: flex-start; }
  .strategy-table { font-size: 12px; }
  .col-name { min-width: 160px; word-break: break-all; }
  .card-grid { grid-template-columns: 1fr; }
}
</style>
