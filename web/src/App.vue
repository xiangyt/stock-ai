<template>
  <!-- ====== 未登录：显示登录页 ====== -->
  <div v-if="!loggedIn" class="login-layout">
    <LoginPage @login="onLoginSuccess" />
  </div>

  <!-- ====== 已登录：显示主界面 ====== -->
  <div v-else class="app-layout">
    <Sidebar
      :current="sidebarActive"
      :user-info="currentUser"
      @change="onPageChange"
      @logout="onLogout"
      @profile="onProfile"
    />

    <main class="main-content">
      <!-- ====== 策略列表页面 ====== -->
      <div v-if="currentPage === 'strategy-list'" class="page strategy-list-page">
        <StrategyList
          :strategies="savedStrategies"
          :total="strategyTotal"
          :page="strategyPage"
          :pageSize="strategyPageSize"
          @load="onLoadStrategy"
          @goNew="onGoNew"
          @deleted="onDeleteStrategies"
          @rename="onRenameStrategy"
          @search="onSearchStrategies"
          @pageChange="onStrategyPageChange"
        />
      </div>

      <!-- ====== 策略详情/编辑页面（新建 + 编辑） ====== -->
      <div v-else-if="currentPage === 'strategy-new' || currentPage === 'strategy-edit'" class="page strategy-edit-page">
        <StrategyBuilder ref="builderRef" @saved="onStrategySaved" @goBack="onBackFromEdit" @goBacktest="onGoBacktest" />
      </div>

      <!-- ====== 策略回测页面 ====== -->
      <div v-else-if="currentPage === 'strategy-backtest'" class="page backtest-page-wrapper">
        <BacktestPage :default-strategy-id="pendingStrategyId" @goBack="onBackFromBacktest" @goToEdit="onGoToEditFromBacktest" />
      </div>

      <!-- ====== 策略订阅页面 ====== -->
      <div v-else-if="currentPage === 'strategy-subscribe'" class="page subscribe-page">
        <header class="page-header">
          <h1>🔔 策略订阅</h1>
          <p>订阅社区策略或接收信号推送</p>
        </header>
        <div class="placeholder-content">
          <div class="ph-card">
            <span class="ph-icon">🚧</span>
            <p>策略订阅功能开发中...</p>
            <p class="ph-sub">即将支持：策略市场浏览 / 信号订阅 / 推送通知</p>
          </div>
        </div>
      </div>

      <!-- ====== 持仓管理页面 ====== -->
      <div v-else-if="currentPage === 'my-positions'" class="page positions-page">
        <header class="page-header">
          <h1>💼 持仓管理</h1>
          <p>管理你的股票持仓组合，跟踪盈亏表现</p>
        </header>
        <div class="placeholder-content">
          <div class="ph-card">
            <span class="ph-icon">🚧</span>
            <p>持仓管理功能开发中...</p>
            <p class="ph-sub">即将支持：持仓录入 / 盈亏追踪 / 风险预警</p>
          </div>
        </div>
      </div>

      <!-- ====== 账户管理页面（仅管理员）====== -->
      <div v-else-if="currentPage === 'account-management'" class="page">
        <AccountManagementPage />
      </div>

      <!-- ====== 机器人配置页面 ====== -->
      <div v-else-if="currentPage === 'bot-config'" class="page">
        <BotConfigPage />
      </div>

      <!-- ====== 个人主页 ====== -->
      <div v-else-if="currentPage === 'profile'" class="page">
        <ProfilePage :current-user="currentUser" @saved="onProfileSaved" @goBack="onBackFromProfile" />
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed, nextTick, onMounted } from 'vue'
import Sidebar from './components/Sidebar.vue'
import LoginPage from './components/LoginPage.vue'
import StrategyBuilder from './components/StrategyBuilder.vue'
import StrategyList from './components/StrategyList.vue'
import BacktestPage from './components/BacktestPage.vue'
import AccountManagementPage from './components/AccountManagementPage.vue'
import BotConfigPage from './components/BotConfigPage.vue'
import ProfilePage from './components/ProfilePage.vue'
import * as strategyApi from './api/strategies'
import * as authApi from './api/auth'
import { getToken, removeToken, isLoggedIn as checkIsLoggedIn } from './utils/auth'
import type { StrategyDetail } from './api/strategies'

// ========== 登录状态管理 ==========
const currentUser = ref<authApi.UserInfo | null>(null)
const loggedIn = ref(checkIsLoggedIn())

/** 检查登录状态（页面刷新时用 token 换用户信息） */
async function checkAuth() {
  const token = getToken()
  if (!token) { loggedIn.value = false; currentUser.value = null; return }
  try {
    const user = await authApi.getMe()
    loggedIn.value = true
    currentUser.value = user
  } catch (e) {
    // token 过期或无效
    removeToken()
    loggedIn.value = false
    currentUser.value = null
  }
}

/** 登录成功回调 */
function onLoginSuccess(user: authApi.UserInfo) {
  loggedIn.value = true
  currentUser.value = user
}

/** 退出登录 */
function onLogout() {
  removeToken()
  loggedIn.value = false
  currentUser.value = null
  currentPage.value = 'strategy-list'
}

/** 跳转个人主页 */
function onProfile() {
  currentPage.value = 'profile'
}

/** 个人主页保存成功 — 刷新用户信息 */
async function onProfileSaved(updatedUser: any) {
  // 刷新当前用户信息
  try {
    currentUser.value = await authApi.getMe()
  } catch (e) { /* ignore */ }
}

// ========== 页面路由 ==========
const currentPage = ref('strategy-list')

// 侧边栏高亮：新建/编辑页时仍显示「策略列表」为选中
const sidebarActive = computed(() => {
  if (currentPage.value === 'strategy-new' || currentPage.value === 'strategy-edit') return 'strategy-list'
  return currentPage.value
})

function onPageChange(key: string) {
  pendingStrategyId.value = null
  currentPage.value = key
}

// ========== 组件引用 ==========
const builderRef = ref<InstanceType<typeof StrategyBuilder> | null>(null)

// ========== 已保存策略（从后端 API 获取） ==========

/** 前端展示用的策略格式（兼容 StrategyList 组件接口） */
interface SavedStrategy {
  id: number
  name: string
  backtestCount: number
  lastRunAt: string | null
  isPublic: boolean
  createdAt: string
  updatedAt: string
}

const savedStrategies = ref<SavedStrategy[]>([])
const loadingStrategies = ref(false)
const currentKeyword = ref('')
const strategyPage = ref(1)
const strategyPageSize = ref(10)
const strategyTotal = ref(0)

/** 从后端加载策略列表（支持关键词搜索+分页） */
async function loadStrategies(keyword?: string) {
  loadingStrategies.value = true
  if (keyword !== undefined) currentKeyword.value = keyword
  try {
    const resp = await strategyApi.fetchStrategies(currentKeyword.value, strategyPage.value, strategyPageSize.value)
    const list = Array.isArray(resp.list) ? resp.list : []
    savedStrategies.value = list.map(toFrontendFormat)
    strategyTotal.value = resp.total ?? list.length
  } catch (e) {
    console.error('加载策略列表失败:', e)
    savedStrategies.value = []
  } finally {
    loadingStrategies.value = false }
}

/** 分页变化时重新请求 */
function onStrategyPageChange(page: number, pageSize: number) {
  strategyPage.value = page
  strategyPageSize.value = pageSize
  loadStrategies()
}

/** 将后端 StrategyListItem 转为前端 SavedStrategy 格式 */
function toFrontendFormat(item: any): SavedStrategy {
  return {
    id: item.id,
    name: item.name ?? '',
    backtestCount: item.backtest_count ?? 0,
    lastRunAt: item.last_run_at ?? null,
    isPublic: !!item.is_public,
    createdAt: item.created_at ?? '',
    updatedAt: item.updated_at ?? '',
  }
}

/** 搜索事件处理 */
function onSearchStrategies(keyword: string) {
  strategyPage.value = 1
  loadStrategies(keyword)
}

// 页面挂载时：检查登录状态 + 加载策略列表
onMounted(() => { checkAuth().then(() => { if (loggedIn.value) loadStrategies() }) })

// 每次切换到策略列表页时重新加载
watch(currentPage, (val) => {
  if (val === 'strategy-list') loadStrategies()
})

// ========== 策略操作 ==========

/** 加载单个策略详情并进入编辑页 */
async function onLoadStrategy(s: SavedStrategy) {
  // 先尝试从后端获取完整详情（含 signals）
  let strategyData = s
  try {
    const detail = await strategyApi.fetchStrategyById(s.id)
    strategyData = { ...toFrontendFormat(detail), ...detail } as SavedStrategy
  } catch (e) {
    console.error('获取策略详情失败:', e)
  }

  currentPage.value = 'strategy-edit'
  nextTick(() => {
    if (builderRef.value && typeof (builderRef.value as any).loadStrategyFromOutside === 'function') {
      ;(builderRef.value as any).loadStrategyFromOutside(strategyData)
    }
  })
}

function onGoNew() {
  currentPage.value = 'strategy-new'
  nextTick(() => {
    if (builderRef.value && typeof (builderRef.value as any).resetAllSignals === 'function') {
      ;(builderRef.value as any).resetAllSignals()
    }
  })
}

/** 批量删除策略 */
async function onDeleteStrategies(ids: number[]) {
  try {
    await strategyApi.batchDeleteStrategies(ids)
    await loadStrategies()
  } catch (e) {
    console.error('删除策略失败:', e)
    alert('删除失败: ' + (e as Error).message)
  }
}

/** 重命名策略 */
async function onRenameStrategy(id: number, newName: string) {
  try {
    await strategyApi.renameStrategy(parseInt(id), newName)
    await loadStrategies()
  } catch (e) {
    console.error('重命名失败:', e)
    alert('重命名失败: ' + (e as Error).message)
  }
}

/** 策略保存成功回调 — 重新加载列表 */
async function onStrategySaved() {
  await loadStrategies()
}

const pendingStrategyId = ref<number | null>(null)

function onGoBacktest(strategyId: number | null) {
  pendingStrategyId.value = strategyId
  currentPage.value = 'strategy-backtest'
}

function onBackFromBacktest() {
  currentPage.value = 'strategy-list'
}

function onGoToEditFromBacktest(strategyId: number) {
  // 从回测页返回编辑页：通过策略 ID 加载详情后进入编辑
  onLoadStrategy({ id: strategyId, name: '', signals: [], logicalOp: 'AND' } as SavedStrategy)
}

function onBackFromProfile() {
  currentPage.value = 'strategy-list'
}

async function onBackFromEdit() {
  await loadStrategies()
  currentPage.value = 'strategy-list'
}
</script>

<style>
/* 全局重置 */
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

html {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto,
    'Helvetica Neue', Arial, 'Noto Sans SC', 'PingFang SC',
    'Microsoft YaHei', sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}
body {
  background: #f0f2f5;
  min-height: 100vh;
  color: #1a1a2e;
}
#app { max-width: 100%; }

/* ===== 登录页布局 ===== */
.login-layout {
  min-height: 100vh;
  background: #f0f2f5;
}

/* ===== 布局 ===== */
.app-layout {
  display: flex;
  min-height: 100vh;
}
.main-content {
  flex: 1;
  padding: 20px 24px;
  overflow-y: auto;
  max-width: calc(100vw - 180px);
}

/* ===== 页面通用 ===== */
.page-header {
  margin-bottom: 24px;
}
.page-header h1 {
  font-size: 24px;
  font-weight: 700;
  margin-bottom: 6px;
}
.page-header p {
  font-size: 14px;
  color: #999;
}

/* 占位内容 */
.placeholder-content {
  display: flex;
  justify-content: center;
  align-items: center;
  min-height: 400px;
}
.ph-card {
  text-align: center;
  background: #fff;
  border: 1px solid #eee;
  border-radius: 16px;
  padding: 56px 48px;
  box-shadow: 0 2px 8px rgba(0,0,0,.04);
}
.ph-icon { font-size: 48px; display: block; margin-bottom: 16px; }
.ph-card p {
  font-size: 15px;
  color: #666;
  margin-bottom: 6px;
}
.ph-sub { font-size: 13px !important; color: #aaa !important; }

/* 编辑页不需要额外 header */
.strategy-edit-page .page-header { display: none; }

/* 回测页面：正常流式布局，不使用负margin hack */
</style>
