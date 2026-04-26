<template>
  <div class="ai-input-bar">
    <div class="input-wrapper">
      <div class="input-header">
        <span class="input-icon">🤖</span>
        <span class="input-title">AI 智能识别</span>
        <span class="input-hint">用自然语言描述你的选股条件，AI 自动解析为策略信号</span>
      </div>
      <textarea
        v-model="text"
        placeholder="例如：
• MACD金叉且PE在20-50倍之间
• RSI超卖 + 底背离，最近3天内出现
• 高ROE(>15%)的小盘成长股，市值小于50亿"
        rows="4"
        @keydown.meta.enter="handleSubmit"
        @keydown.ctrl.enter="handleSubmit"
      ></textarea>
      <div class="input-actions">
        <button class="btn-analyze" @click="handleSubmit" :disabled="!text.trim()">
          <span>⚡</span> 解析为策略信号
        </button>
        <button class="btn-clear" @click="text = ''" v-if="text">清空</button>
      </div>
    </div>

    <!-- 解析结果 -->
    <transition name="fade">
      <div v-if="parsedResult" class="parse-result">
        <div class="result-header">
          <span class="result-badge">AI 解析结果</span>
          <button class="btn-accept-all" @click="handleAcceptAll">全部添加到策略 →</button>
        </div>
        <ul class="result-list">
          <li v-for="(item, i) in parsedResult" :key="i" class="result-item">
            <span class="result-check">✓</span>
            <strong>{{ item.indicatorName }}</strong>
            <code>{{ item.operatorLabel }} {{ item.paramSummary }}</code>
          </li>
        </ul>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const text = ref('')
const emit = defineEmits<{
  parse: [signals: ParsedSignal[]]
  acceptAll: [signals: ParsedSignal[]]
}>()

export interface ParsedSignal {
  indicatorID: string
  indicatorName: string
  category: string
  operator: string
  operatorSymbol: string
  operatorLabel: string
  params: Record<string, any>
  paramSummary: string
}

let parsedResult = ref<ParsedSignal[] | null>(null)

function handleSubmit() {
  if (!text.value.trim()) return

  // TODO: 对接真实 AI 解析接口
  // 当前用简单的关键词匹配模拟
  const t = text.value.toLowerCase()
  const results: ParsedSignal[] = []

  // 简单模拟：根据关键词生成预设信号
  if (t.includes('macd') && (t.includes('金叉') || t.includes('golden'))) {
    results.push({
      indicatorID: 'macd_cross', indicatorName: 'MACD交叉', category: 'technical',
      operator: 'cross_up', operatorSymbol: '↑↑', operatorLabel: '金叉/上穿',
      params: { within_days: 3 }, paramSummary: '近3天'
    })
  }
  if (t.includes('pe') || (t.includes('市盈率'))) {
    const betweenMatch = t.match(/(\d+)\s*[-~到]\s*(\d+)/)
    if (betweenMatch) {
      results.push({
        indicatorID: 'pe_ttm', indicatorName: '市盈率(TTM)', category: 'financial',
        operator: 'between', operatorSymbol: '[~]', operatorLabel: '区间内',
        params: { min_value: Number(betweenMatch[1]), max_value: Number(betweenMatch[2]) },
        paramSummary: `[${betweenMatch[1]}, ${betweenMatch[2]}]倍`
      })
    }
  }
  if (t.includes('rsi') && (t.includes('超卖') || t.includes('oversold'))) {
    results.push({
      indicatorID: 'rsi6', indicatorName: 'RSI6', category: 'technical',
      operator: '<', operatorSymbol: '<', operatorLabel: '小于',
      params: { value_number: 30 }, paramSummary: '< 30'
    })
  }
  if (t.includes('roe') && (t.match(/\d+/))) {
    const roeVal = t.match(/roe[^0-9]*(\d+)/i)?.[1] || '15'
    results.push({
      indicatorID: 'roe_w', indicatorName: 'ROE(加权)', category: 'financial',
      operator: '>=', operatorSymbol: '>=', operatorLabel: '大于等于',
      params: { value_number: Number(roeVal) }, paramSummary: `>= ${roeVal}%`
    })
  }
  if (t.includes('小盘') || t.includes('小市值')) {
    results.push({
      indicatorID: 'circulate_market_cap', indicatorName: '流通市值', category: 'market',
      operator: '<', operatorSymbol: '<', operatorLabel: '小于',
      params: { value_number: 50 }, paramSummary: '< 50亿'
    })
  }
  if (t.includes('底背离')) {
    results.push({
      indicatorID: 'macd_divergence', indicatorName: 'MACD背离', category: 'technical',
      operator: '↗', operatorSymbol: '↗', operatorLabel: '底背离',
      params: { lookback_days: 26 }, paramSummary: ''
    })
  }

  parsedResult.value = results.length > 0 ? results : null
  emit('parse', results)
}

function handleAcceptAll() {
  if (parsedResult.value) {
    emit('acceptAll', parsedResult.value)
    parsedResult.value = null
    text.value = ''
  }
}

/** 暴露给父组件的方法 */
function clear() { text.value = ''; parsedResult.value = null }
defineExpose({ clear })
</script>

<style scoped>
.ai-input-bar {
  margin-bottom: 20px;
}
.input-wrapper {
  background: #fff;
  border: 1.5px solid #e0e0e0;
  border-radius: 12px;
  overflow: hidden;
  transition: border-color 0.2s;
}
.input-wrapper:focus-within {
  border-color: #1677ff;
  box-shadow: 0 0 0 3px rgba(22,119,255,0.08);
}
.input-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px 0;
}
.input-icon { font-size: 18px; }
.input-title {
  font-size: 14px;
  font-weight: 700;
  color: #1677ff;
}
.input-hint {
  font-size: 12px;
  color: #999;
  margin-left: auto;
}
textarea {
  display: block;
  width: 100%;
  padding: 12px 16px;
  border: none;
  outline: none;
  resize: vertical;
  font-size: 13.5px;
  line-height: 1.65;
  color: #333;
  font-family: inherit;
  min-height: 90px;
}
textarea::placeholder { color: #bbb; }
.input-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 8px 12px 12px;
  border-top: 1px solid #f5f5f5;
  background: #fafafa;
}
.btn-analyze {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 7px 18px;
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  background: linear-gradient(135deg, #1677ff, #0958d9);
  border: none;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.15s;
}
.btn-analyze:hover:not(:disabled) { box-shadow: 0 3px 10px rgba(22,119,255,0.35); }
.btn-analyze:disabled { opacity: 0.45; cursor: not-allowed; }
.btn-clear {
  padding: 7px 14px;
  font-size: 13px;
  border: 1px solid #ddd;
  border-radius: 6px;
  background: #fff;
  cursor: pointer;
  color: #666;
}
.btn-clear:hover { border-color: #999; color: #333; }

/* 解析结果 */
.parse-result {
  margin-top: 10px;
  background: #fff;
  border: 1.5px solid #b7eb8f;
  border-radius: 10px;
  overflow: hidden;
}
.result-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  background: #f6ffed;
  border-bottom: 1px solid #d9f7be;
}
.result-badge {
  font-size: 12px;
  font-weight: 700;
  color: #389e0d;
}
.btn-accept-all {
  font-size: 12px;
  font-weight: 600;
  color: #1677ff;
  background: none;
  border: none;
  cursor: pointer;
}
.btn-accept-all:hover { text-decoration: underline; }
.result-list {
  list-style: none;
  padding: 8px 12px;
  margin: 0;
}
.result-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 0;
  font-size: 13px;
}
.result-check { color: #52c41a; font-weight: 700; }
.result-item strong { color: #1a1a2e; }
.result-item code {
  padding: 1px 6px;
  background: #f5f5f5;
  border-radius: 4px;
  font-family: monospace;
  font-size: 12px;
  color: #555;
  margin-left: 4px;
}

.fade-enter-active { transition: opacity 0.25s ease; }
.fade-leave-active { transition: opacity 0.15s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
