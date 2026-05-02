<template>
  <div class="login-page">
    <div class="login-card">
      <div class="login-header">
        <span class="login-logo">🔶</span>
        <h1 class="login-title">AI 选股</h1>
        <p class="login-subtitle">智能选股策略平台</p>
      </div>

      <form class="login-form" @submit.prevent="handleSubmit">
        <div class="form-group">
          <label for="username">用户名</label>
          <input
            id="username"
            v-model="form.username"
            type="text"
            placeholder="请输入用户名"
            autocomplete="username"
            :disabled="loading"
          />
        </div>

        <div class="form-group">
          <label for="password">密码</label>
          <input
            id="password"
            v-model="form.password"
            type="password"
            placeholder="请输入密码"
            autocomplete="current-password"
            :disabled="loading"
          />
        </div>

        <p v-if="errorMsg" class="error-msg">{{ errorMsg }}</p>

        <button type="submit" class="btn-login" :disabled="loading || !canSubmit">
          {{ loading ? '登录中...' : '登 录' }}
        </button>
      </form>

      <div class="login-footer">
        <span>还没有账号？</span>
        <button type="button" class="link-btn" @click="openRegister">注册新账号</button>
      </div>
    </div>

    <!-- 注册弹窗 -->
    <teleport to="body">
      <div v-if="showRegister" class="modal-overlay" @click.self="showRegister = false">
        <div class="modal-box register-box">
          <h2 class="register-title">📝 注册账号</h2>
          <form @submit.prevent="handleRegister">
            <div class="form-group">
              <label>用户名</label>
              <input v-model="regForm.username" type="text" placeholder="设置用户名" autocomplete="new-username" :disabled="regLoading" />
            </div>
            <div class="form-group">
              <label>密码</label>
              <input v-model="regForm.password" type="password" placeholder="至少6位" autocomplete="new-password" :disabled="regLoading" />
            </div>
            <div class="form-group">
              <label>昵称(可选)</label>
              <input v-model="regForm.nickname" type="text" placeholder="显示名称" :disabled="regLoading" />
            </div>
            <p v-if="regError" class="error-msg">{{ regError }}</p>
            <div class="register-actions">
              <button type="button" class="btn-modal-cancel" @click="showRegister = false">取消</button>
              <button type="submit" class="btn-login-sm" :disabled="regLoading">{{ regLoading ? '注册中...' : '注 册' }}</button>
            </div>
          </form>
        </div>
      </div>
    </teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import * as authApi from '../api/auth'
import { setToken } from '../utils/auth'

const emit = defineEmits<{ login: [user: authApi.UserInfo] }>()

// ========== 登录表单 ==========
const form = ref({ username: '', password: '' })
const loading = ref(false)
const errorMsg = ref('')

const canSubmit = computed(() => form.value.username.trim() && form.value.password.length >= 0)

async function handleSubmit() {
  loading.value = true
  errorMsg.value = ''
  try {
    const resp = await authApi.login(form.value.username, form.value.password)
    setToken(resp.token)
    emit('login', resp.user)
  } catch (e) {
    errorMsg.value = (e as Error).message || '登录失败'
  } finally {
    loading.value = false
  }
}

// ========== 注册表单 ==========
const showRegister = ref(false)
const regForm = ref({ username: '', password: '', nickname: '' })
const regLoading = ref(false)
const regError = ref('')

function openRegister() {
  regForm.value = { username: '', password: '', nickname: '' }
  regError.value = ''
  showRegister.value = true
}

async function handleRegister() {
  if (regForm.value.password.length < 6) { regError.value = '密码至少6位'; return }
  regLoading.value = true; regError.value = ''
  try {
    const user = await authApi.register(regForm.value.username, regForm.value.password, regForm.value.nickname)
    showRegister.value = false
    // 注册成功后自动登录
    form.value = { username: regForm.value.username, password: '' }
    await handleSubmit()
  } catch (e) {
    regError.value = (e as Error).message || '注册失败'
  } finally {
    regLoading.value = false
  }
}
</script>

<style scoped>
.login-page {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: calc(100vh - 40px);
  background: #f0f2f5;
}
.login-card {
  background: #fff;
  border-radius: 16px;
  padding: 40px 36px;
  width: 380px;
  box-shadow: 0 4px 24px rgba(0,0,0,.08);
}
.login-header { text-align: center; margin-bottom: 32px; }
.login-logo { font-size: 40px; display: block; margin-bottom: 8px; }
.login-title { font-size: 24px; font-weight: 700; color: #1a1a2e; margin-bottom: 4px; }
.login-subtitle { font-size: 13.5px; color: #999; }

.form-group { margin-bottom: 18px; }
.form-group label { display: block; font-size: 13px; font-weight: 600; color: #555; margin-bottom: 6px; }
.form-group input {
  width: 100%; padding: 10px 12px; border: 1.5px solid #d9d9d9; border-radius: 8px;
  font-size: 14px; outline: none; transition: border-color .15s; box-sizing: border-box;
}
.form-group input:focus { border-color: #1677ff; box-shadow: 0 0 0 3px rgba(22,119,255,.08); }
.form-group input:disabled { background: #f5f5f5; }

.error-msg { color: #cf1322; font-size: 13px; margin-bottom: 10px; text-align: center; min-height: 20px; }

.btn-login {
  width: 100%; padding: 11px; font-size: 15px; font-weight: 700;
  color: #fff; background: #1677ff; border: none; border-radius: 8px;
  cursor: pointer; transition: background .15s;
}
.btn-login:hover:not(:disabled) { background: #0958d9; }
.btn-login:disabled { opacity: .6; cursor: not-allowed; }

.login-footer {
  text-align: center; margin-top: 20px; font-size: 13px; color: #999;
}
.link-btn {
  color: #1677ff; background: none; border: none; cursor: pointer;
  font-size: 13px; font-weight: 500;
}
.link-btn:hover { text-decoration: underline; }

/* Modal */
.modal-overlay {
  position: fixed; inset: 0; z-index: 999; background: rgba(0,0,0,.35);
  display: flex; align-items: center; justify-content: center;
}
.register-box { padding: 28px; width: 380px; background: #fff; border-radius: 16px; box-shadow: 0 4px 24px rgba(0,0,0,.12); }
.register-title { font-size: 17px; font-weight: 700; margin-bottom: 20px; text-align: center; }
.register-actions { display: flex; justify-content: flex-end; gap: 10px; margin-top: 4px; }
.btn-login-sm {
  padding: 7px 22px; font-size: 14px; font-weight: 600;
  color: #fff; background: #1677ff; border: none; border-radius: 6px; cursor: pointer;
}
.btn-login-sm:hover:not(:disabled) { background: #0958d9; }
.btn-login-sm:disabled { opacity: .6; cursor: not-allowed; }
.btn-modal-cancel {
  padding: 7px 18px; border: 1px solid #d9d9d9; border-radius: 6px; background: #fff;
  font-size: 13px; cursor: pointer; color: #666;
}
</style>
