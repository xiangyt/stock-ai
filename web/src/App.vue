<template>
  <div class="app-layout">
    <Sidebar :current="currentPage" @change="onPageChange" />

    <main class="main-content">
      <!-- AI 选股页面 -->
      <div v-if="currentPage === 'ai-picker'" class="page ai-page">
        <header class="page-header">
          <h1>🤖 AI 智能选股</h1>
          <p>用自然语言描述你的选股策略，AI 自动解析并执行</p>
        </header>
        <div class="placeholder-content">
          <div class="ph-card">
            <span class="ph-icon">🚧</span>
            <p>AI 选股功能开发中...</p>
            <p class="ph-sub">即将支持：自然语言 → 策略信号 → 自动筛选 → 结果排序</p>
          </div>
        </div>
      </div>

      <!-- 我的策略页面（核心功能） -->
      <div v-else-if="currentPage === 'my-strategy'" class="strategy-page">
        <!-- 顶部 AI 输入框 -->
        <AIInput
          ref="aiInputRef"
          @parse="onAIParse"
          @acceptAll="onAIAcceptAll"
        />
        <!-- 策略构建器 -->
        <StrategyBuilder ref="builderRef" />
      </div>

      <!-- 我的持仓页面 -->
      <div v-else-if="currentPage === 'my-positions'" class="page positions-page">
        <header class="page-header">
          <h1>💼 我的持仓</h1>
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
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import Sidebar from './components/Sidebar.vue'
import AIInput from './components/AIInput.vue'
import StrategyBuilder from './components/StrategyBuilder.vue'

const currentPage = ref('my-strategy')
const aiInputRef = ref<InstanceType<typeof AIInput> | null>(null)
const builderRef = ref<InstanceType<typeof StrategyBuilder> | null>(null)

function onPageChange(key: string) {
  currentPage.value = key
}

/** AI 解析回调 */
function onAIParse(signals: any[]) {
  // 解析时仅展示结果，不自动添加
}

/** AI 全部接受回调 — 调用 builder 的 acceptAISignals */
function onAIAcceptAll(signals: any[]) {
  if (builderRef.value) {
    builderRef.value.acceptAISignals(signals)
  }
}
</script>

<style>
/* 全局重置 */
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

html {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial,
    'Noto Sans SC', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}
body {
  background: #f5f6f8;
  min-height: 100vh;
  color: #1a1a2e;
}
#app { max-width: 100%; }

/* ===== 布局 ===== */
.app-layout {
  display: flex;
  min-height: 100vh;
}
.main-content {
  flex: 1;
  padding: 24px 28px;
  overflow-y: auto;
  max-width: calc(100vw - 200px);
}

/* ===== 页面通用 ===== */
.page-header {
  margin-bottom: 20px;
}
.page-header h1 {
  font-size: 22px;
  font-weight: 700;
  margin-bottom: 4px;
}
.page-header p {
  font-size: 13.5px;
  color: #888;
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
  border-radius: 12px;
  padding: 48px 40px;
}
.ph-icon { font-size: 44px; display: block; margin-bottom: 14px; }
.ph-card p {
  font-size: 15px;
  color: #666;
  margin-bottom: 6px;
}
.ph-sub { font-size: 13px !important; color: #aaa !important; }

/* 策略页不需要额外 header */
.strategy-page .page-header {
  /* 由 AIInput 和 Builder 自行展示标题 */
}
</style>
