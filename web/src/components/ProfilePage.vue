<template>
  <div class="profile-page">
    <header class="page-header">
      <h1>👤 个人主页</h1>
      <p>编辑你的账户信息</p>
    </header>

    <div class="profile-card">
      <!-- 基本信息 -->
      <section class="form-section">
        <h3>基本信息</h3>
        <div class="form-group">
          <label>用户名</label>
          <input :value="currentUser?.username" disabled class="input-disabled" />
          <span class="input-hint">用户名不可修改</span>
        </div>
        <div class="form-group">
          <label>昵称</label>
          <input v-model="nickname" placeholder="设置显示名称" />
        </div>
        <div class="form-group">
          <label>头像 URL</label>
          <input v-model="avatar" placeholder="https://..." autocomplete="off" />
        </div>
      </section>

      <!-- 修改密码 -->
      <section class="form-section">
        <h3>修改密码</h3>
        <p class="section-desc">留空则不修改密码</p>
        <div class="form-group">
          <label>当前密码</label>
          <input v-model="oldPassword" type="password" placeholder="输入当前密码（修改密码时必填）" autocomplete="new-password" />
        </div>
        <div class="form-row">
          <div class="form-group flex-1">
            <label>新密码</label>
            <input v-model="newPassword" type="password" placeholder="至少6位" autocomplete="new-password" />
          </div>
          <div class="form-group flex-1">
            <label>确认新密码</label>
            <input v-model="confirmPassword" type="password" placeholder="再次输入新密码" autocomplete="new-password" />
          </div>
        </div>
      </section>

      <!-- 操作 -->
      <div class="action-bar">
        <button class="btn-save" :disabled="saving" @click="onSave">
          {{ saving ? '保存中...' : '💾 保存修改' }}
        </button>
        <button class="btn-cancel" @click="$emit('goBack')">取消</button>
      </div>

      <p v-if="errorMsg" class="error-msg">{{ errorMsg }}</p>
      <p v-if="successMsg" class="success-msg">{{ successMsg }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import * as authApi from '../api/auth'

const props = defineProps<{
  currentUser?: { id: number; username: string; nickname: string; avatar: string; role: string } | null
}>()

const emit = defineEmits<{ saved: []; goBack: [] }>()

const nickname = ref('')
const avatar = ref('')
const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const saving = ref(false)
const errorMsg = ref('')
const successMsg = ref('')

watch(() => props.currentUser, (u) => {
  if (u) {
    nickname.value = u.nickname || ''
    avatar.value = u.avatar || ''
  }
}, { immediate: true })

function clearMessages() { errorMsg.value = ''; successMsg.value = '' }

async function onSave() {
  clearMessages()
  saving.value = true

  try {
    if (newPassword.value || confirmPassword.value) {
      if (newPassword.value !== confirmPassword.value) {
        errorMsg.value = '两次输入的新密码不一致'
        saving.value = false
        return
      }
      if (newPassword.value.length < 6) {
        errorMsg.value = '新密码至少6位'
        saving.value = false
        return
      }
      if (!oldPassword.value) {
        errorMsg.value = '修改密码需要填写当前密码'
        saving.value = false
        return
      }
    }

    const updated = await authApi.updateAccount({
      nickname: nickname.value,
      avatar: avatar.value,
      old_password: oldPassword.value,
      new_password: newPassword.value,
    })

    successMsg.value = '保存成功'
    oldPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
    emit('saved', updated)
  } catch (e: any) {
    errorMsg.value = e.message || '保存失败'
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.profile-page {}
.profile-card {
  max-width: 560px; background: #fff; border-radius: 16px;
  box-shadow: 0 2px 8px rgba(0,0,0,.04); padding: 32px;
}

.form-section { margin-bottom: 28px; }
.form-section h3 {
  font-size: 16px; font-weight: 700; color: #333; margin-bottom: 12px;
  padding-bottom: 8px; border-bottom: 1px solid #f0f0f0;
}
.section-desc { font-size: 12.5px; color: #aaa; margin-bottom: 12px; }

.form-group { margin-bottom: 14px; }
.form-group label {
  display: block; font-size: 13px; font-weight: 600; color: #555; margin-bottom: 6px;
}
.form-group input {
  width: 100%; padding: 9px 12px; border: 1.5px solid #d9d9d9; border-radius: 8px;
  font-size: 13.5px; outline: none; transition: border-color .15s; box-sizing: border-box;
}
.form-group input:focus:not(.input-disabled) { border-color: #1677ff; box-shadow: 0 0 0 3px rgba(22,119,255,.08); }
.input-disabled { background: #f5f5f5 !important; color: #aaa; cursor: not-allowed; }
.input-hint { display: block; font-size: 11.5px; color: #bbb; margin-top: 3px; }

.form-row { display: flex; gap: 16px; }
.flex-1 { flex: 1; min-width: 0; }

.action-bar {
  display: flex; gap: 10px; margin-top: 4px; padding-top: 20px;
  border-top: 1px solid #f0f0f0;
}
.btn-save {
  padding: 9px 24px; font-size: 14px; font-weight: 600;
  color: #fff; background: #1677ff; border: none; border-radius: 8px; cursor: pointer;
}
.btn-save:hover:not(:disabled) { background: #0958d9; }
.btn-save:disabled { opacity: .55; cursor: not-allowed; }
.btn-cancel {
  padding: 9px 18px; font-size: 14px; border: 1px solid #d9d9d9; border-radius: 8px;
  background: #fff; cursor: pointer; color: #666;
}
.btn-cancel:hover { border-color: #bbb; }

.error-msg { color: #cf1322; font-size: 13px; text-align: center; min-height: 20px; margin-top: 10px; }
.success-msg { color: #16a34a; font-size: 13px; text-align: center; min-height: 20px; margin-top: 10px; }
</style>
