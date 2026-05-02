<template>
  <div class="account-mgmt-page">
    <header class="page-header">
      <h1>👥 账户管理</h1>
      <p>管理系统用户、重置密码、启用/禁用账号</p>
    </header>

    <!-- 加载中 -->
    <div v-if="loading" class="loading-state">加载用户列表...</div>

    <!-- 用户表格 -->
    <table v-else class="user-table">
      <thead>
        <tr>
          <th>ID</th>
          <th>用户名</th>
          <th>昵称</th>
          <th>角色</th>
          <th>状态</th>
          <th>创建时间</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="u in users" :key="u.id" :class="{ 'disabled-row': u.status === 0 }">
          <td>{{ u.id }}</td>
          <td>{{ u.username }}</td>
          <td>{{ u.nickname || '-' }}</td>
          <td>
            <span :class="'role-tag role-' + (u.role === 'admin' ? 'admin' : 'user')">
              {{ u.role === 'admin' ? '管理员' : '普通用户' }}
            </span>
          </td>
          <td>
            <span :class="'status-tag status-' + (u.status === 1 ? 'on' : 'off')">
              {{ u.status === 1 ? '正常' : '已禁用' }}
            </span>
          </td>
          <td>{{ u.created_at }}</td>
          <td class="actions-cell">
            <button
              v-if="u.status === 1"
              class="btn-sm btn-warn"
              @click="onToggleStatus(u, 0)"
              title="禁用"
            >禁用</button>
            <button
              v-else
              class="btn-sm btn-ok"
              @click="onToggleStatus(u, 1)"
              title="启用"
            >启用</button>
            <button class="btn-sm btn-info" @click="openResetPwd(u)">重置密码</button>
          </td>
        </tr>
      </tbody>
    </table>

    <p v-if="!loading && users.length === 0" class="empty-hint">暂无用户数据</p>

    <!-- 重置密码弹窗 -->
    <teleport to="body">
      <div v-if="showResetModal" class="modal-overlay" @click.self="closeResetModal">
        <div class="modal-box">
          <h3>重置密码 — {{ resetTarget?.username }}</h3>
          <div class="form-group">
            <label>新密码（至少6位）</label>
            <input
              ref="resetPwdInput"
              v-model="newPassword"
              type="password"
              placeholder="请输入新密码"
              autocomplete="new-password"
              @keyup.enter="confirmReset"
            />
          </div>
          <p v-if="resetError" class="error-msg">{{ resetError }}</p>
          <div class="modal-actions">
            <button class="btn-modal-cancel" @click="closeResetModal">取消</button>
            <button class="btn-login-sm" :disabled="resetting || newPassword.length < 6" @click="confirmReset">
              {{ resetting ? '处理中...' : '确认重置' }}
            </button>
          </div>
        </div>
      </div>
    </teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import * as adminApi from '../api/admin'
import type { AdminUserItem } from '../api/admin'

const users = ref<AdminUserItem[]>([])
const loading = ref(false)

// ---- 重置密码弹窗 ----
const showResetModal = ref(false)
const resetTarget = ref<AdminUserItem | null>(null)
const newPassword = ref('')
const resetting = ref(false)
const resetError = ref('')
const resetPwdInput = ref<HTMLInputElement | null>(null)

async function loadUsers() {
  loading.value = true
  try {
    users.value = await adminApi.listUsers()
  } catch (e) {
    console.error('加载用户失败:', e)
  } finally {
    loading.value = false
  }
}

async function onToggleStatus(u: AdminUserItem, newStatus: number) {
  const action = newStatus === 1 ? '启用' : '禁用'
  if (!confirm(`确定${action}用户「${u.username}」？`)) return
  try {
    await adminApi.toggleUserStatus(u.id, newStatus)
    await loadUsers()
  } catch (e: any) {
    alert(e.message || `${action}失败`)
  }
}

function openResetPwd(u: AdminUserItem) {
  resetTarget.value = u
  newPassword.value = ''
  resetError.value = ''
  showResetModal.value = true
}

function closeResetModal() {
  showResetModal.value = false
  resetTarget.value = null
  newPassword.value = ''
}

async function confirmReset() {
  if (!resetTarget.value) return
  if (newPassword.value.length < 6) {
    resetError.value = '密码至少6位'
    return
  }
  resetting.value = true
  resetError.value = ''
  try {
    await adminApi.resetUserPassword(resetTarget.value.id, newPassword.value)
    closeResetModal()
    alert('密码已重置')
  } catch (e: any) {
    resetError.value = e.message || '重置失败'
  } finally {
    resetting.value = false
  }
}

onMounted(loadUsers)
</script>

<style scoped>
.account-mgmt-page {}
.loading-state { text-align: center; color: #999; padding: 60px 0; font-size: 14px; }

.user-table {
  width: 100%; border-collapse: collapse;
  background: #fff; border-radius: 12px; overflow: hidden;
  box-shadow: 0 1px 4px rgba(0,0,0,.06);
}
.user-table th {
  background: #fafafa; padding: 10px 14px; text-align: left;
  font-size: 13px; font-weight: 600; color: #666; border-bottom: 1px solid #eee;
}
.user-table td {
  padding: 10px 14px; font-size: 13.5px; border-bottom: 1px solid #f3f3f3;
}
.user-table tr:hover td { background: #f9fbff; }
.user-table tr.disabled-row td { opacity: .45; }

.actions-cell { white-space: nowrap; }

.btn-sm {
  padding: 4px 12px; font-size: 12.5px; border: 1px solid #d9d9d9;
  border-radius: 5px; background: #fff; cursor: pointer; margin-right: 6px;
  transition: all .15s;
}
.btn-sm:last-child { margin-right: 0; }
.btn-ok { color: #1677ff; border-color: #91caff; background: #f0f7ff; }
.btn-ok:hover { background: #d6e8ff; }
.btn-warn { color: #ff6b00; border-color: #ffd591; background: #fff7e6; }
.btn-warn:hover { background: #ffe7ba; }
.btn-info { color: #555; }
.btn-info:hover { border-color: #aaa; background: #f5f5f5; }

/* 标签 */
.role-tag, .status-tag {
  display: inline-block; padding: 2px 10px; border-radius: 10px;
  font-size: 11.5px; font-weight: 600;
}
.role-admin { background: #e6f4ff; color: #1677ff; }
.role-user { background: #f5f5f5; color: #888; }
.status-on { background: #f0fdf4; color: #16a34a; }
.status-off { background: #fef2f2; color: #dc2626; }

.empty-hint { text-align: center; color: #bbb; padding: 40px 0; font-size: 14px; }
.error-msg { color: #cf1322; font-size: 13px; margin-bottom: 10px; text-align: center; min-height: 20px; }

/* Modal */
.modal-overlay {
  position: fixed; inset: 0; z-index: 999; background: rgba(0,0,0,.35);
  display: flex; align-items: center; justify-content: center;
}
.modal-box {
  background: #fff; border-radius: 16px; padding: 28px 32px; width: 400px;
  box-shadow: 0 4px 24px rgba(0,0,0,.12);
}
.modal-box h3 { font-size: 17px; margin-bottom: 18px; text-align: center; }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; font-size: 13px; font-weight: 600; color: #555; margin-bottom: 6px; }
.form-group input {
  width: 100%; padding: 10px 12px; border: 1.5px solid #d9d9d9; border-radius: 8px;
  font-size: 14px; outline: none; box-sizing: border-box;
}
.form-group input:focus { border-color: #1677ff; box-shadow: 0 0 0 3px rgba(22,119,255,.08); }

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
</style>
