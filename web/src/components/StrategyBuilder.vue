<template>
  <div class="strategy-builder">

    <!-- 页面顶部标题栏（独立于卡片） -->
    <div class="page-top-bar">
      <div class="ptb-left">
        <button class="back-btn" @click="$emit('goBack')" title="返回列表">‹</button>
        <span class="page-title">{{ editingId ? '编辑策略' : '新建策略' }}</span>
      </div>
      <div class="ptb-right">
        <!-- 策略名称（仅自己的策略可点击编辑） -->
        <template v-if="isOwner && isEditingName">
          <input
            v-model="strategyName"
            class="inline-name-input"
            @keyup.enter="handleNameConfirm"
            @blur="handleNameConfirm"
            placeholder="输入策略名称"
            ref="nameInputRef"
          />
        </template>
        <span
          v-else-if="isOwner"
          class="inline-name-text"
          @click="startEditName"
          :title="'点击编辑名称'"
        >{{ strategyName || '未命名策略' }}</span>
        <span
          v-else
          class="inline-name-text readonly"
        >{{ strategyName || '未命名策略' }}</span>
        <button
          v-if="isOwner"
          class="btn-save-sm"
          @click="saveStrategy"
          :disabled="signals.length === 0"
          title="保存策略"
        >💾 保存</button>
      </div>
    </div>

    <!-- ========== Section 1: AI 输入区 ========== -->
    <section v-if="isOwner" class="sec-input">
      <!-- AI 文本输入 -->
      <div class="ai-input-area">
        <textarea
          v-model="aiText"
          placeholder="例如：MACD金叉且PE在20-50倍之间 ｜ 高ROE(>15%)小盘成长股"
          :rows="aiExpanded ? 4 : 1"
          @keydown.meta.enter="handleAISubmit"
          @keydown.ctrl.enter="handleAISubmit"
        ></textarea>
        <div class="ai-toolbar">
          <div class="ai-tools-left">
            <span class="ai-tool-label" :class="{ active: aiExpanded }" @click="aiExpanded = !aiExpanded">{{ aiExpanded ? '收起 ▲' : '展开 ▼' }}</span>
            <span class="ai-tool-label">☰ A股 ▾</span>
            <span class="ai-tool-label" @click="scrollToIndicators">🔍 条件选股</span>
            <span class="ai-tool-label dim">★ 我的收藏</span>
          </div>
          <div class="ai-tools-right">
            <span class="ai-hint-text">AI识别输入框</span>
            <button class="btn-ai-send" :disabled="!aiText.trim()" @click="handleAISubmit">
              ⚡ 发送
            </button>
          </div>
        </div>
      </div>
    </section>

    <!-- ========== Section 2: 条件选股（指标平铺网格） ========== -->
    <section class="sec-signals">
      <!-- 页签 + 操作按钮同行 -->
      <div class="sec-header-row">
        <div class="signal-tabs-inline">
          <button
            :class="['signal-tab', { active: activeTab === 'buy_signals' }]"
            @click="activeTab = 'buy_signals'"
          >
            信号买入
            <span v-if="signals.length > 0" class="tab-badge">{{ signals.length }}</span>
          </button>
          <button
            :class="['signal-tab', { active: activeTab === 'sell_signals' }]"
            @click="activeTab = 'sell_signals'"
          >
            信号卖出
            <span v-if="sellSignals.length > 0" class="tab-badge">{{ sellSignals.length }}</span>
          </button>
          <button
            :class="['signal-tab', { active: activeTab === 'position' }]"
            @click="activeTab = 'position'"
          >
            仓位管理
          </button>
          <button
            :class="['signal-tab', { active: activeTab === 'exit_rules' }]"
            @click="activeTab = 'exit_rules'"
          >
            卖出规则
          </button>
        </div>
        <div class="sec-right" v-if="isOwner && isSignalTab">
          <button class="btn-sec-sm" v-if="(activeTab === 'buy_signals' ? signals.length > 0 : sellSignals.length > 0)" @click="onClearClick">清空全部</button>
          <button class="btn-sec-sm" @click="exportJSON" :disabled="signals.length === 0 && sellSignals.length === 0" title="导出信号">导出</button>
          <label class="btn-sec-sm" title="导入信号">导入
            <input type="file" accept=".json" @change="importJSON" style="display:none" />
          </label>
        </div>
      </div>

      <!-- 信号页签内容：指标网格 + 信号选择 -->
      <template v-if="isSignalTab">

      <!-- 分类 + 指标平铺区域（四列横排，仅自己的策略可编辑） -->
      <div class="indicators-flat-area" v-if="isOwner && !indicatorsLoading">
        <template v-for="(inds, cat) in allData" :key="cat">
          <div class="cat-column">
            <!-- 分类标题 -->
            <div class="cat-section-header">{{ catLabels[cat as Category] }}</div>
            <!-- 该分类下所有指标按钮 -->
            <div class="indicator-grid">
              <button
                v-for="ind in inds" :key="ind.id"
                :class="['ind-drop-btn', { expanded: expandedIndicatorID === ind.id }]"
                @click="toggleExpandIndicator(ind.id)"
              >
                {{ ind.name }}
                <span class="drop-arrow">{{ expandedIndicatorID === ind.id ? '▲' : '▾' }}</span>
              </button>
            </div>
          </div>
        </template>
      </div>

      <!-- 指标数据加载中 -->
      <div v-if="indicatorsLoading" class="indicators-loading">
        <span class="loading-spinner"></span>
        正在加载指标数据...
      </div>

      <!-- 展开的指标面板（inline 紧跟在对应位置或统一展示，仅自己的策略可编辑） -->
      <transition name="expand-down">
        <div v-if="isOwner && expandedIndicatorID && expandedInd" class="ind-expand-panel">
          <!-- 面板头部 -->
          <div class="expand-header">
            <span class="expand-ind-name">{{ expandedInd.name }}</span>
            <span class="expand-ind-desc">{{ expandedInd.description }}</span>
            <button class="expand-close-btn" @click="expandedIndicatorID = null">✕</button>
          </div>

          <!-- 内置信号列表（signal_id 第6位='0'，一键添加） -->
          <template v-if="builtinSigs.length > 0">
            <div class="expand-label">内置信号</div>
            <div class="preset-list-compact">
              <button
                v-for="sig in builtinSigs" :key="sig.signal_id"
                class="preset-mini"
                @click="addBuiltinSignal(sig)"
                @mouseenter="hoveredSigID = sig.signal_id; updateTooltipPos($event)"
                @mouseleave="hoveredSigID = ''"
              >
                <span class="pm-name">{{ sig.alias || sig.name }}</span>
              </button>
            </div>
          </template>

          <!-- 自定义信号区（signal_id 第6位='1'，表单配置模式） -->
          <template v-if="customSigs.length > 0">
          <div class="expand-label">自定义信号</div>
          <div class="quick-add-form">
            <div class="qf-row">
            <!-- 步骤1: 选择自定义信号 -->
            <div class="qf-operator">
              <select v-model="customSignalID" class="qf-op-select qf-select-sig">
                <option v-for="sig in customSigs" :key="sig.signal_id" :value="sig.signal_id">{{ sig.name }}</option>
              </select>
            </div>
            <!-- 步骤2: 选择操作符 -->
            <div class="qf-operator">
              <select v-model="customOperator" class="qf-op-select" @change="onOperatorChange">
                <option v-for="op in currentCustomOperators" :key="op.operator" :value="op.operator">{{ op.label }} {{ op.label !== operatorSymbol(op.operator) ? `(${operatorSymbol(op.operator)})` : '' }}</option>
              </select>
            </div>
            <!-- 步骤3: 根据选中操作符动态渲染参数输入框 -->
            <div class="qf-params">
              <template v-for="p in currentOpParams" :key="p.key">
                <!-- 数值型参数 → 标签 + 数字输入框 + 单位 -->
                <div v-if="isNumberLike(p.type)" class="qf-param-field">
                  <span class="qf-param-label">{{ p.label }}</span>
                  <input
                    type="number"
                    v-model.number="paramValues[p.key]"
                    :placeholder="'默认' + p.default"
                    class="qf-input"
                    step="any"
                    :min="p.min" :max="p.max"
                  />
                  <span v-if="p.unit" class="qf-param-unit">{{ p.unit }}</span>
                </div>
                <!-- 单选枚举 → 标签 + 下拉框 -->
                <div v-else-if="p.type === 'select'" class="qf-param-field">
                  <span class="qf-param-label">{{ p.label }}</span>
                  <select
                    v-model="paramValues[p.key]"
                    class="qf-input qf-select"
                  >
                    <option value="">请选择...</option>
                    <option v-for="o in p.options" :key="o.value" :value="o.value">{{ o.label }}</option>
                  </select>
                </div>
                <!-- 多选枚举 → 标签 + checkbox 组 -->
                <div v-else-if="isMultiSelect(p.type)" class="qf-param-field">
                  <span class="qf-param-label">{{ p.label }}</span>
                  <div class="qf-multi-select">
                    <label
                      v-for="o in p.options" :key="o.value"
                      class="qf-checkbox"
                      :class="{ checked: (multiVals[p.key] || []).includes(o.value) }"
                    >
                      <input
                        type="checkbox"
                        :value="o.value"
                        :checked="(multiVals[p.key] || []).includes(o.value)"
                        @change="toggleMultiVal(p.key, o.value)"
                      />
                      {{ o.label }}
                    </label>
                  </div>
                </div>
              </template>
              <span v-if="currentOpParams.length === 0" class="qf-no-params">该操作符无需额外参数</span>
            </div>
            <button
              :class="['qf-add-btn', 'qf-add-btn--' + activeTab]"
              :disabled="!canQuickAdd"
              @click="addCustomSignal"
            >
              {{ activeTab === 'buy_signals' ? '📈 添加为买入信号' : '📉 添加为卖出信号' }}
            </button>
            </div>
            <div class="qf-sig-desc" v-if="currentCustomSig?.description">{{ currentCustomSig.description }}</div>
          </div>
          </template>

          <!-- 快速添加成功提示 -->
          <transition name="fade-fast">
            <span v-if="addSuccessMsg" class="add-success-msg-inline">{{ addSuccessMsg }}</span>
          </transition>
        </div>
      </transition>

      <!-- 内置信号 fixed tooltip（脱离父容器 overflow 裁剪，放在 transition 外） -->
      <Teleport to="body" :disabled="!hoveredSigID">
        <div v-if="hoveredSigID && hoveredSigDesc" class="builtin-tooltip"
          :style="{ top: tooltipPos.top, left: tooltipPos.left, maxWidth: '380px' }">
          {{ hoveredSigDesc }}
        </div>
      </Teleport>

      <!-- 空状态（根据活跃页签显示） -->
      <div v-if="activeTabSignals.length === 0" class="empty-signals">
        <div class="empty-icon">📭</div>
        <p>还没有{{ activeTab === 'buy_signals' ? '买入' : '卖出' }}信号条件</p>
        <p class="empty-sub">点击上方指标的 ▾ 展开并选择信号条件，或使用 AI 输入框自动生成</p>
      </div>

      <!-- 已添加信号标签行（根据活跃页签显示） -->
      <div v-if="activeTabSignals.length > 0" class="signals-chips-area">
        <transition-group name="sig-chip" tag="div" class="chips-row">
          <div v-for="(s, i) in activeTabSignals" :key="i"
            class="sig-chip" :class="'chip-' + s.category">
            <span class="chip-bar"></span>
            <span class="chip-name">{{ s.name === s.indicator_name ? s.name : `${s.indicator_name}: ${s.name}` }}</span>
            <span v-if="s.operator !== 'none'" class="chip-op">{{ s.opSym }} {{ s.paramText }}</span>
            <button v-if="isOwner" class="chip-del" @click="removeActiveTabSignal(i)">✕</button>
          </div>
        </transition-group>
      </div>

      <!-- 底部操作栏（仅买入页签显示） -->
      <div v-if="activeTab === 'buy_signals' && signals.length > 0" class="sec-footer">
        <div class="logic-toggle">
          <span class="logic-label">逻辑关系：</span>
          <button :class="['logic-btn', { active: logicalOp === 'AND' }]" :disabled="!isOwner" @click="isOwner && (logicalOp = 'AND')">AND</button>
          <button :class="['logic-btn', { active: logicalOp === 'OR' }]" :disabled="!isOwner" @click="isOwner && (logicalOp = 'OR')">OR</button>
        </div>
      </div>
      </template>

      <!-- 仓位管理页签内容 -->
      <div v-if="activeTab === 'position' && isOwner" class="rules-body">
        <div class="rules-subsection">
          <h4 class="rules-subtitle">📦 仓位管理</h4>
          <div class="rules-pos-grid">
            <label class="rule-item">
              <span>最大持仓</span>
              <input type="number" v-model.number="positionRules.max_positions" class="rule-input" step="1" min="1" max="50" @change="markRulesDirty" />
              <span class="rule-unit">只</span>
            </label>
            <label class="rule-item">
              <span>单票上限</span>
              <input type="number" v-model.number="positionRules.max_single_pct" class="rule-input" step="5" min="0" max="100" @change="markRulesDirty" />
              <span class="rule-unit">%</span>
            </label>
            <label class="rule-item">
              <span>分配方式</span>
              <select v-model="positionRules.allocation" class="rule-select" @change="markRulesDirty">
                <option value="equal">等权分配</option>
                <option value="signal_weighted">信号加权</option>
                <option value="volatility_weighted">波动率加权</option>
                <option value="risk_parity">风险平价</option>
              </select>
            </label>
          </div>
        </div>
      </div>

      <!-- 卖出规则页签内容 -->
      <div v-if="activeTab === 'exit_rules' && isOwner" class="rules-body">
        <!-- 卖出规则 -->
        <div class="rules-subsection">
          <h4 class="rules-subtitle">📐 卖出规则</h4>
          <div class="rules-list">
            <!-- 滑点 -->
            <div class="rule-row">
              <label class="rule-check">
                <input type="checkbox" checked disabled />
                <span class="rule-label">滑点</span>
                <span class="rule-help" data-tooltip="模拟实际交易中成交价与触发价的偏差，卖出时以低于触发价滑点%的价格成交，使回测更接近真实">?</span>
              </label>
              <div class="rule-params-wrap">
                <input type="number" v-model.number="exitRules.slippage_pct" class="rule-input-sm" step="0.1" min="0" max="5" @change="markRulesDirty" />
                <span class="rule-unit">%</span>
              </div>
            </div>

            <div v-for="rule in exitRules.rules" :key="rule.type" class="rule-row">
              <label class="rule-check">
                <input type="checkbox" v-model="rule.enabled" @change="markRulesDirty" />
                <span class="rule-label">{{ ruleName(rule.type) }}</span>
                <span class="rule-help" :data-tooltip="ruleDesc(rule.type)">?</span>
              </label>
              <template v-if="rule.enabled">
                <div class="rule-params-wrap">
                <template v-if="rule.type === 'stop_loss'">
                  <input type="number" v-model.number="rule.params.threshold_pct" class="rule-input-sm" step="1" @change="markRulesDirty" />
                  <span class="rule-unit">%</span>
                </template>
                <template v-else-if="rule.type === 'take_profit'">
                  <input type="number" v-model.number="rule.params.threshold_pct" class="rule-input-sm" step="1" @change="markRulesDirty" />
                  <span class="rule-unit">%</span>
                </template>
                <template v-else-if="rule.type === 'time_exit'">
                  <input type="number" v-model.number="rule.params.hold_days" class="rule-input-sm" step="1" min="1" @change="markRulesDirty" />
                  <span class="rule-unit">天</span>
                </template>
                <template v-else-if="rule.type === 'trailing_stop'">
                  <span class="rule-param-label">激活</span>
                  <input type="number" v-model.number="rule.params.activation_pct" class="rule-input-sm" step="1" @change="markRulesDirty" />
                  <span class="rule-unit">%</span>
                  <span class="rule-param-label">回撤</span>
                  <input type="number" v-model.number="rule.params.trail_pct" class="rule-input-sm" step="0.5" @change="markRulesDirty" />
                  <span class="rule-unit">%</span>
                </template>
                <template v-else-if="rule.type === 'segment_profit'">
                  <div class="segment-list">
                    <div v-for="(lv, li) in rule.params.levels" :key="li" class="segment-level">
                      <span class="seg-idx">#{{ Number(li) + 1 }}</span>
                      <span class="seg-label">涨</span>
                      <input type="number" v-model.number="lv.threshold_pct" class="rule-input-xs" step="1" @change="markRulesDirty" />
                      <span class="seg-unit">% 卖</span>
                      <input type="number" v-model.number="lv.sell_ratio" class="rule-input-xs" step="0.1" min="0.1" max="1" @change="markRulesDirty" />
                      <span class="seg-unit">成</span>
                      <button v-if="rule.params.levels.length > 1" class="btn-level-del" @click="rule.params.levels.splice(li,1); markRulesDirty()">✕</button>
                    </div>
                    <button v-if="rule.params.levels.length < 5" class="btn-level-add" @click="rule.params.levels.push({ threshold_pct: 30, sell_ratio: 0.5 }); markRulesDirty()">+ 添加档位</button>
                  </div>
                </template>
                </div>
              </template>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- ========== Section 3: 结果预览区 ========== -->
    <section class="sec-results">
      <!-- 头部工具栏（共享） -->
      <div class="results-head">
        <div class="results-left">
          <h3 class="results-title">选出股票 <strong>{{ screenResult ? screenResult.passed.length : 0 }}</strong> / {{ screenResult?.total ?? 0 }}</h3>
          <div class="results-tabs">
            <button :class="['rtab', { active: resultViewMode === 'list' }]" @click="resultViewMode = 'list'">≡ 股票列表</button>
            <button :class="['rtab', { active: resultViewMode === 'multiKline', dim: !screenResult?.passed.length }]" @click="screenResult?.passed.length && (resultViewMode = 'multiKline')">⊞ 多股同列</button>
          </div>
        </div>
        <div class="results-right">
          <input type="date" v-model="runDate" class="date-picker" :max="today" />
          <button class="btn-res-action backtest" @click="onGoBacktest">📊 历史回测</button>
          <button class="btn-res-action run" @click="runFilter" :disabled="isScreening || signals.length === 0">
            {{ isScreening ? '⏳ 筛选中...' : '🔍 运行筛选' }}
          </button>
        </div>
      </div>

      <!-- 共享工具栏（一键加入自选 + 搜索 + 视图切换） -->
      <div class="results-toolbar">
        <button class="btn-batch-fav" @click="batchAddFavorites"
          :disabled="isAddingFav || !(resultViewMode === 'list' ? filteredData.length : screenResult?.passed.length)">
          {{ isAddingFav ? '⏳ 添加中...' : '＋ 一键加入自选' }}
        </button>
        <!-- 列表模式：概览/财务切换 -->
        <div v-if="resultViewMode === 'list'" class="tb-sort-tabs">
          <button :class="['st', { active: tableTab === 'overview' }]" @click="tableTab = 'overview'">概览</button>
          <button :class="['st', { active: tableTab === 'financial' }]" @click="tableTab = 'financial'">财务</button>
        </div>
        <!-- K线模式：周期切换 -->
        <template v-else>
          <div class="tb-sort-tabs mk-period-tabs-inline">
            <button class="mk-tab-sm" :class="{ active: multiKlinePeriod === 'daily' }"
              @click="multiKlinePeriod = 'daily'">日K</button>
            <button class="mk-tab-sm" :class="{ active: multiKlinePeriod === 'weekly' }"
              @click="multiKlinePeriod = 'weekly'">周K</button>
            <button class="mk-tab-sm" :class="{ active: multiKlinePeriod === 'monthly' }"
              @click="multiKlinePeriod = 'monthly'">月K</button>
          </div>
        </template>
        <div class="tb-search">
          <!-- K线模式：列数选择（搜索框左侧） -->
          <select v-if="resultViewMode === 'multiKline'" v-model.number="multiKlineColumns" class="mk-col-select-inline">
            <option :value="2">2 列</option>
            <option :value="3">3 列</option>
            <option :value="4">4 列</option>
          </select>
          <input type="text" placeholder="代码/名称" class="tb-search-in" v-model.trim="searchKeyword" />
          <span class="tb-search-icon">🔍</span>
        </div>
      </div>

      <!-- ====== 视图 A：股票列表 ====== -->
      <template v-if="resultViewMode === 'list'">

      <div class="results-table-wrap" :class="{ 'financial-mode': tableTab === 'financial' }">
        <table class="results-table">
          <colgroup>
            <col class="col-cb" />
            <col class="col-idx" />
            <col class="col-code" />
            <col class="col-name" />
            <!-- 概览列 -->
            <col class="col-price" />
            <col class="col-pct" />
            <col class="col-num" />
            <col class="col-num" />
            <col class="col-num" />
            <col class="col-industry" />
            <col class="col-sector" />
            <col class="col-links" />
            <!-- 财务列（9列） -->
            <col class="col-fin-bvps" />
            <col class="col-fin-eps" />
            <col class="col-fin-roe" />
            <col class="col-fin-roa" />
            <col class="col-fin-gm" />
            <col class="col-fin-nm" />
            <col class="col-fin-dr" />
            <col class="col-fin-ps" />
            <col class="col-fin-pb" />
          </colgroup>
          <thead>
            <tr>
              <th class="col-cb">
                <input type="checkbox" :checked="allSelected" :indeterminate="Boolean(someSelected)" @change="toggleAll" />
              </th>
              <th class="col-idx">序号</th>
              <th class="col-code">股票代码</th>
              <th class="col-name">股票简称</th>

              <!-- 概览表头（v-show 避免 DOM 重建抖动） -->
              <th v-show="tableTab === 'overview'" class="col-price sortable" :class="{ active: sortField === 'price', [sortOrder]: sortField === 'price' }" @click="toggleSort('price')">现价(元)<span class="sort-icon">{{ sortField === 'price' ? (sortOrder === 'asc' ? '↑' : '↓') : '↕' }}</span></th>
              <th v-show="tableTab === 'overview'" class="col-pct">涨跌幅(%)</th>
              <th v-show="tableTab === 'overview'" class="col-num sortable" :class="{ active: sortField === 'pe_ttm', [sortOrder]: sortField === 'pe_ttm' }" @click="toggleSort('pe_ttm')">市盈率TTM<span class="sort-icon">{{ sortField === 'pe_ttm' ? (sortOrder === 'asc' ? '↑' : '↓') : '↕' }}</span></th>
              <th v-show="tableTab === 'overview'" class="col-num sortable" :class="{ active: sortField === 'circulate_market_cap', [sortOrder]: sortField === 'circulate_market_cap' }" @click="toggleSort('circulate_market_cap')">流通市值(亿)<span class="sort-icon">{{ sortField === 'circulate_market_cap' ? (sortOrder === 'asc' ? '↑' : '↓') : '↕' }}</span></th>
              <th v-show="tableTab === 'overview'" class="col-num sortable" :class="{ active: sortField === 'total_market_cap', [sortOrder]: sortField === 'total_market_cap' }" @click="toggleSort('total_market_cap')">总市值(亿)<span class="sort-icon">{{ sortField === 'total_market_cap' ? (sortOrder === 'asc' ? '↑' : '↓') : '↕' }}</span></th>
              <th v-show="tableTab === 'overview'" class="col-industry">所属东财行业</th>
              <th v-show="tableTab === 'overview'" class="col-sector">细分行业</th>
              <th v-show="tableTab === 'overview'" class="col-links">外部链接</th>

              <!-- 财务表头（v-show） -->
              <th v-show="tableTab === 'financial'" class="sortable" :class="{ active: sortField === 'bvps', [sortOrder]: sortField === 'bvps' }" @click="toggleSort('bvps')">每股净资产(元)<span class="sort-icon">{{ sortField === 'bvps' ? (sortOrder === 'asc' ? '↑' : '↓') : '↕' }}</span></th>
              <th v-show="tableTab === 'financial'" class="sortable" :class="{ active: sortField === 'basic_eps', [sortOrder]: sortField === 'basic_eps' }" @click="toggleSort('basic_eps')">基本每股收益(元)<span class="sort-icon">{{ sortField === 'basic_eps' ? (sortOrder === 'asc' ? '↑' : '↓') : '↕' }}</span></th>
              <th v-show="tableTab === 'financial'">净资产收益率(%)</th>
              <th v-show="tableTab === 'financial'">总资产收益率(%)</th>
              <th v-show="tableTab === 'financial'">毛利率(%)</th>
              <th v-show="tableTab === 'financial'">净利率(%)</th>
              <th v-show="tableTab === 'financial'">资产负债率(%)</th>
              <th v-show="tableTab === 'financial'">市销率TTM</th>
              <th v-show="tableTab === 'financial'" class="sortable" :class="{ active: sortField === 'pb', [sortOrder]: sortField === 'pb' }" @click="toggleSort('pb')">市净率<span class="sort-icon">{{ sortField === 'pb' ? (sortOrder === 'asc' ? '↑' : '↓') : '↕' }}</span></th>
            </tr>
          </thead>
          <tbody>
            <!-- 筛选中 -->
            <tr v-if="isScreening">
              <td :colspan="13" style="text-align:center; padding:40px 20px; color:#999;">
                <span class="loading-spinner"></span> 正在筛选 {{ screenResult?.total ?? 0 }} 只股票...
              </td>
            </tr>
            <!-- 有结果（含搜索过滤） -->
            <template v-else-if="screenResult && screenResult.passed.length > 0 && paginatedData.length > 0">
              <tr v-for="(stock, idx) in paginatedData" :key="stock.code">
                <td class="col-cb"><input type="checkbox" :checked="selectedRows.has((currentPage - 1) * pageSize + idx)" @change="toggleRow((currentPage - 1) * pageSize + idx)" /></td>
                <td class="col-idx">{{ (currentPage - 1) * pageSize + idx + 1 }}</td>
              <td class="col-code">{{ stock.code }}</td>
              <td class="col-name" @mouseenter="showKLine($event, stock)" @mouseleave="hideKLine">
                <span class="stock-name-hover" :title="stock.name + ' — 悬浮查看K线图'">{{ stock.name }}</span>
              </td>

                <!-- 概览列（v-show 避免 DOM 重建抖动） -->
                <td v-show="tableTab === 'overview'" class="col-price">{{ stock.price?.toFixed(2) ?? '-' }}</td>
                <td v-show="tableTab === 'overview'" :class="['col-pct', stock.change_pct > 0 ? 'up' : stock.change_pct < 0 ? 'down' : '']">
                  {{ stock.change_pct != null ? (stock.change_pct > 0 ? '+' : '') + stock.change_pct.toFixed(2) + '%' : '-' }}
                </td>
                <td v-show="tableTab === 'overview'" class="col-num">{{ stock.pe_ttm != null ? stock.pe_ttm.toFixed(2) : '-' }}</td>
                <td v-show="tableTab === 'overview'" class="col-num">{{ stock.circulate_market_cap > 0 ? (stock.circulate_market_cap / 1e8).toFixed(2) : '-' }}</td>
                <td v-show="tableTab === 'overview'" class="col-num">{{ stock.total_market_cap > 0 ? (stock.total_market_cap / 1e8).toFixed(2) : '-' }}</td>
                <td v-show="tableTab === 'overview'" class="col-industry">{{ stock.industry || '-' }}</td>
                <td v-show="tableTab === 'overview'" class="col-sector">{{ stock.sector || '-' }}</td>
                <td v-show="tableTab === 'overview'" class="col-links">
                  <a :href="getEastMoneyUrl(stock.code)" target="_blank" class="ext-link" title="东方财富">东财</a>
                  <a :href="getTHSUrl(stock.code)" target="_blank" class="ext-link" title="同花顺">同花顺</a>
                  <a :href="getTencentUrl(stock.code)" target="_blank" class="ext-link" title="腾讯自选股">腾讯</a>
                </td>

                <!-- 财务列（v-show） -->
                <td v-show="tableTab === 'financial'">{{ stock.bvps > 0 ? stock.bvps.toFixed(2) : '-' }}</td>
                <td v-show="tableTab === 'financial'">{{ stock.basic_eps != 0 ? stock.basic_eps.toFixed(3) : '-' }}</td>
                <td v-show="tableTab === 'financial'">{{ stock.roe != 0 ? stock.roe.toFixed(2) + '%' : '-' }}</td>
                <td v-show="tableTab === 'financial'">{{ stock.roa != 0 ? stock.roa.toFixed(2) + '%' : '-' }}</td>
                <td v-show="tableTab === 'financial'">{{ stock.gross_margin != 0 ? stock.gross_margin.toFixed(2) + '%' : '-' }}</td>
                <td v-show="tableTab === 'financial'">{{ stock.net_margin != 0 ? stock.net_margin.toFixed(2) + '%' : '-' }}</td>
                <td v-show="tableTab === 'financial'">{{ stock.debt_ratio != 0 ? stock.debt_ratio.toFixed(2) + '%' : '-' }}</td>
                <td v-show="tableTab === 'financial'">{{ stock.ps_ttm > 0 ? stock.ps_ttm.toFixed(2) : '-' }}</td>
                <td v-show="tableTab === 'financial'">{{ stock.pb > 0 ? stock.pb.toFixed(2) : '-' }}</td>
              </tr>
            </template>
            <!-- 搜索无匹配 -->
            <tr v-else-if="screenResult && screenResult.passed.length > 0 && paginatedData.length === 0">
              <td :colspan="13" style="text-align:center; padding:40px 20px; color:#bbb;">
                🔍 未找到与「{{ searchKeyword }}」匹配的股票
              </td>
            </tr>
            <!-- 无结果 -->
            <tr v-else-if="screenResult && !screenError">
              <td :colspan="13" style="text-align:center; padding:40px 20px; color:#bbb;">
                {{ screenResult.total > 0 ? '😔 没有符合条件的股票，请尝试调整条件' : '🔍 运行筛选后显示结果' }}
              </td>
            </tr>
            <!-- 错误 -->
            <tr v-else-if="screenError">
              <td :colspan="13" style="text-align:center; padding:30px; color:#cf1322;">
                ⚠️ {{ screenError }}
              </td>
            </tr>
            <!-- 初始状态 -->
            <tr v-else>
              <td :colspan="13" style="text-align:center; padding:60px 20px; color:#bbb; font-size:14px;">
                🔍 运行筛选后显示结果
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      </template>

      <!-- ====== 视图 B：多股同列K线 ====== -->
      <MultiStockKLine
        v-else-if="resultViewMode === 'multiKline'"
        :stocks="paginatedData.map((s: any) => ({ code: s.code, name: s.name }))"
        :period="multiKlinePeriod"
        :columns="multiKlineColumns"
        :key="`${searchKeyword}-${currentPage}-${pageSize}-${multiKlineColumns}`"
      />

      <!-- 空状态（无数据时切换到多股同列） -->
      <div v-else-if="resultViewMode === 'multiKline'" class="multi-kline-empty">
        <div class="mk-empty-content">
          <span class="mk-empty-icon">📊</span>
          <p>暂无股票数据</p>
          <p class="mk-empty-hint">请先运行选股筛选，再切换到多股同列视图</p>
        </div>
      </div>

      <!-- 分页栏（两种模式完全一致） -->
      <div v-if="screenResult && screenResult.passed.length > 0" class="pagination-bar">
        <div class="pag-left">
          <select class="page-size-select" :value="pageSize" @change="onPageSizeChange">
            <option v-for="sz in pageSizes" :key="sz" :value="sz">{{ sz }} 条/页</option>
          </select>
          <span class="pag-info">共 {{ filteredData.length }} 条</span>
        </div>
        <div class="pag-center">
          <button class="pag-btn" :disabled="currentPage <= 1" @click="prevPage">‹ 上一页</button>
          <template v-for="p in visiblePages" :key="p">
            <span v-if="p === '...'" class="pag-ellipsis">...</span>
            <button v-else :class="['pag-btn', { active: p === currentPage }]" @click="typeof p === 'number' && goPage(p)">{{ p }}</button>
          </template>
          <button class="pag-btn" :disabled="currentPage >= totalPage" @click="nextPage">下一页 ›</button>
        </div>
        <div class="pag-right">
          <span>前往第</span>
          <input type="number" class="pag-jump-input" :min="1" :max="totalPage" v-model.number="jumpPageInput" @keyup.enter="goJumpPage" />
          <span>页 / 共 {{ totalPage }} 页</span>
        </div>
      </div>
    </section>

    <!-- 通用确认弹窗 -->
    <teleport to="body">
      <div class="modal-overlay" v-if="showConfirm" @click.self="onConfirmCancel">
        <div class="modal-box">
          <div class="modal-title">{{ confirmTitle }}</div>
          <p class="modal-body">{{ confirmBody }}</p>
          <div class="modal-actions">
            <button class="btn-modal-cancel" @click="onConfirmCancel">取消</button>
            <button class="btn-modal-danger" @click="onConfirmOk">{{ confirmOkText }}</button>
          </div>
        </div>
      </div>
    </teleport>

    <!-- K 线图悬浮弹窗 -->
    <KLineTooltip
      :visible="klineVisible"
      :stock-code="klineStockCode"
      :stock-name="klineStockName"
      :x="klineX"
      :y="klineY"
      @mouseenter="onKLineEnter"
      @mouseleave="hideKLine"
    />
  </div>

  <!-- 保存成功 toast -->
  <Teleport to="body">
    <transition name="toast-slide">
      <div v-if="saveSuccessMsg" class="save-toast">{{ saveSuccessMsg }}</div>
    </transition>
    <transition name="toast-slide">
      <div v-if="favToastMsg" :class="['save-toast', favToastOk ? 'fav-toast-ok' : 'fav-toast-err']">{{ favToastMsg }}</div>
    </transition>
  </Teleport>
</template>

<script setup lang="ts">
import { reactive, ref, computed, nextTick, onMounted, watch } from 'vue'
import * as indicatorsApi from '../api/indicators'
import KLineTooltip from './KLineTooltip.vue'
import MultiStockKLine from './MultiStockKLine.vue'
import type {
  IndicatorMeta, Category, CompareOperator,
  SignalDef, SignalConfig, SignalOperatorOption, ParamDef, EnumOption,
} from '../api/indicators'
import { categoryLabels as catLabels, operatorSymbols, isCustomSignal } from '../api/indicators'

// ========== Props ==========
interface BuilderProps {
  currentUserId?: number
}
const props = withDefaults(defineProps<BuilderProps>(), { currentUserId: 0 })

// ========== 策略归属 ==========
/** 当前编辑策略的创建者 UID（0 = 新建策略，始终视为自己的） */
const strategyOwnerId = ref(0)
/** 是否是自己的策略 */
const isOwner = computed(() => {
  if (!editingId.value) return true  // 新建策略
  if (props.currentUserId === 0) return true  // 未传 currentUserId，兼容模式
  return strategyOwnerId.value === props.currentUserId
})

interface Sig {
  uid: number
  indicator_id: string       // 指标 ID（5位, 如 "03001"）
  indicator_name: string     // 指标名称（如 "筹码分布"）
  signal_id: string          // 8位数字信号ID（如 "03001001", "04001001"）
  name: string               // 显示名
  category: Category
  operator: CompareOperator
  opSym: string              // 操作符符号 (>)
  opLbl: string              // 操作符中文标签
  params: Record<string, any>
  paramText: string          // 参数可读文本
}

// ========== 指标数据（从后端 API 加载） ==========

/** 全量指标数据，按分类分组 */
const allData = ref<Record<Category, IndicatorMeta[]>>({
  technical: [],
  market: [],
  fundamental: [],
  financial: [],
})
/** 枚举选项映射（从 API 获取，用于 listing_board / industry 等枚举型指标） */
const enumOptions = ref<Record<string, EnumOption[]>>({})
/** 指标数据加载状态 */
const indicatorsLoading = ref(true)
/** 加载指标数据 */
let indicatorsLoadPromise: Promise<void> | null = null

async function loadIndicators() {
  // 正在加载中 → 返回同一个 Promise，避免重复请求
  if (indicatorsLoadPromise) return indicatorsLoadPromise
  // 已加载则直接返回
  if (allData.value.technical.length > 0) return Promise.resolve()
  indicatorsLoading.value = true
  indicatorsLoadPromise = (async () => {
    try {
      const data = await indicatorsApi.fetchIndicators()
      const grouped: Record<string, IndicatorMeta[]> = { technical: [], market: [], fundamental: [], financial: [] }
      for (const ind of data.indicators) {
        const cat = ind.category as string
        if (grouped[cat]) grouped[cat].push(ind)
      }
      for (const cat in grouped) {
        grouped[cat].sort((a, b) => a.id.localeCompare(b.id))
      }
      allData.value = grouped as Record<Category, IndicatorMeta[]>
      enumOptions.value = data.enum_options
    } catch (e) {
      console.error('加载指标数据失败:', e)
    } finally {
      indicatorsLoading.value = false
      indicatorsLoadPromise = null
    }
  })()
  return indicatorsLoadPromise
}

// 组件挂载时加载指标
onMounted(() => { loadIndicators() })

/** 当前展开的指标 ID */
const expandedIndicatorID = ref<string | null>(null)
const expandedInd = computed(() => {
  if (!expandedIndicatorID.value) return null
  for (const cats of Object.values(allData.value)) {
    const found = cats.find(i => i.id === expandedIndicatorID.value)
    if (found) return found
  }
  return null
})

// ============================================================================
//  展开指标的信号拆分（内置 vs 自定义，两种独立交互模型）
// ============================================================================

/** 当前展开指标下的内置信号列表（signal_id 第6位='0'，一键添加模式） */
const builtinSigs = ref<SignalDef[]>([])

/** 当前展开指标下的自定义信号列表（signal_id 第6位='1'，表单配置模式） */
const customSigs = ref<SignalDef[]>([])

/** 自定义表单状态（仅用于自定义信号区） */
const selectedSignalID = ref<string | null>(null)  // 当前选中的内置信号ID（用于高亮）
const customSignalID = ref<string>('')
const customOperator = ref<CompareOperator>('gt')
const paramValues = reactive<Record<string, any>>({})
const multiVals = reactive<Record<string, string[]>>({})

/** 内置信号 fixed tooltip 状态 */
const hoveredSigID = ref<string>('')
const hoveredSigDesc = computed(() => {
  if (!hoveredSigID.value) return ''
  const sig = builtinSigs.value.find(s => s.signal_id === hoveredSigID.value)
  return sig?.description ?? ''
})
const tooltipPos = ref({ top: '0px', left: '0px' })

/** 更新 fixed tooltip 位置 — 左对齐按钮，靠右留边距 */
function updateTooltipPos(e: MouseEvent) {
  const el = e.currentTarget as HTMLElement
  const rect = el.getBoundingClientRect()
  const w = Math.min(380, window.innerWidth - 16)
  // 左边缘对齐按钮左边缘（+4px 偏移），而非居中
  let left = rect.left + 4
  if (left < 8) left = 8
  if (left + w > window.innerWidth - 8) left = window.innerWidth - w - 8
  tooltipPos.value = {
    top: `${Math.round(rect.bottom + 6)}px`,
    left: `${Math.round(left)}px`,
  }
}

/** 监听指标切换：一次性拆分内置/自定义信号 + 初始化表单 */
watch(expandedIndicatorID, (newID) => {
  // 重置所有状态
  builtinSigs.value = []
  customSigs.value = []
  customSignalID.value = ''
  customOperator.value = 'gt'
  clearParams()
  selectedSignalID.value = null

  if (!newID || !expandedInd.value) return

  // 按 signal_id 来源位拆分为两个独立数组
  for (const sig of expandedInd.value.signals) {
    if (!isCustomSignal(sig.signal_id)) {
      // 内置信号（第6位='0'）：只要有 operators 就加入（用于查看/备用）
      // 即使没有 default_config，也记录到 builtinSigs 供后续检查
      builtinSigs.value.push(sig)
    } else if (sig.operators && sig.operators.length > 0) {
      // 自定义信号（第6位='1'）：只要有 operators 就加入（有参数或无参数均可）
      customSigs.value.push(sig)
    } else if (sig.default_config) {
      // 兜底：如果没有 operators 但有 default_config，也加入自定义列表
      customSigs.value.push(sig)
    }
  }

  // 自定义表单默认选中第一个
  if (customSigs.value.length > 0) {
    const first = customSigs.value[0]
    customSignalID.value = first.signal_id
    // operator 和参数由 watch(customSignalID) 通过 default_config 初始化
  }
})

/** 切换自定义信号时：从 operators/default_config 初始化操作符和参数 */
watch(customSignalID, async (newSigId) => {
  if (!newSigId) return
  const sig = currentCustomSig.value
  if (!sig) return

  // 自定义信号必定有 operators
  if (sig.default_config) {
    customOperator.value = sig.default_config.operator as CompareOperator
    // 等待 currentOpParams 随 operator 更新
    await nextTick()
    const cfgParams = sig.default_config.params || {}
    for (const p of currentOpParams.value) {
      if (p.type === 'multi_select' || p.type === 'select_multi') {
        multiVals[p.key] = [...(cfgParams[p.key] || [])]
      } else {
        paramValues[p.key] = cfgParams[p.key] ?? p.default
      }
    }
  } else {
    customOperator.value = sig.operators[0].operator as CompareOperator
    clearParams()
  }
})

// ============================================================================
//  自定义表单计算属性（仅依赖 customSigs / customSignalID / customOperator）
// ============================================================================

/** 当前选中的自定义信号定义 */
const currentCustomSig = computed((): SignalDef | undefined => {
  if (!customSignalID.value || customSigs.value.length === 0) return undefined
  return customSigs.value.find(s => s.signal_id === customSignalID.value)
})

/** 当前自定义信号的可用操作符 */
const currentCustomOperators = computed((): SignalOperatorOption[] => {
  return currentCustomSig.value?.operators || []
})

/** 当前操作符的参数定义 */
const currentOpParams = computed((): ParamDef[] => {
  const sig = currentCustomSig.value
  if (!sig) return []
  const op = sig.operators.find(o => o.operator === customOperator.value)
  return op?.params || []
})

/** 自定义添加按钮是否可用 */
const canQuickAdd = computed(() => {
  if (!customOperator.value) return false
  const params = currentOpParams.value
  for (const p of params) {
    if (!p.required) continue
    // 多选类型：检查 multiVals
    if (p.type === 'multi_select' || p.type === 'select_multi') {
      if (!multiVals[p.key] || multiVals[p.key].length === 0) return false
    } else {
      // 其他类型（数字、单选等）：检查 paramValues
      const val = paramValues[p.key]
      if (val === undefined || val === '') return false
    }
  }
  return true
})

/** 已选信号列表 */
const signals = ref<Sig[]>([])
let uidCounter = 0
const logicalOp = ref<'AND' | 'OR'>('AND')
const strategyName = ref('')
const isEditingName = ref(false)
const nameInputRef = ref<HTMLInputElement | null>(null)
const addSuccessMsg = ref('')
const saveSuccessMsg = ref('')
let saveSuccessTimer: ReturnType<typeof setTimeout> | null = null

/** 筛选日期（YYYY-MM-DD），默认为今天 */
const today = new Date().toISOString().split('T')[0]
const runDate = ref(today)
const showConfirm = ref(false)
const confirmTitle = ref('')
const confirmBody = ref('')
const confirmOkText = ref('确认')
let confirmCallback: (() => void) | null = null
function showConfirmModal(opts: { title: string; body: string; okText?: string; onOk: () => void }) {
  confirmTitle.value = opts.title
  confirmBody.value = opts.body
  confirmOkText.value = opts.okText || '确认'
  confirmCallback = opts.onOk
  showConfirm.value = true
}
function onConfirmOk() {
  showConfirm.value = false
  confirmCallback?.()
  confirmCallback = null
}
function onConfirmCancel() {
  showConfirm.value = false
  confirmCallback = null
}
interface BuilderEmits {
  (e: 'addSignals', signals: Sig[]): void
  (e: 'saved', strategy: { id: number; name: string }): void
  (e: 'goBack'): void
  (e: 'goBacktest', strategyId: number | null): void
}
const emit = defineEmits<BuilderEmits>()
const editingId = ref<number | null>(null) // 后端数字 ID，null = 新建模式

/** 脏标记：是否进行了信号增删操作（未保存） */
const isDirty = ref(false)

// ===== 卖出规则 & 仓位管理 =====
const rulesDirty = ref(false)
const exitRules = reactive<strategyApi.ExitRules>({
  rules: [
    { type: 'stop_loss', enabled: true, params: { threshold_pct: -8 }, priority: 1 },
    { type: 'take_profit', enabled: true, params: { threshold_pct: 20 }, priority: 2 },
    { type: 'time_exit', enabled: false, params: { hold_days: 10 }, priority: 3 },
    { type: 'trailing_stop', enabled: false, params: { trail_pct: 5, activation_pct: 10 }, priority: 2 },
    { type: 'segment_profit', enabled: false, params: { levels: [{ threshold_pct: 10, sell_ratio: 0.5 }, { threshold_pct: 20, sell_ratio: 0.5 }] }, priority: 2 },
  ],
  slippage_pct: 0.3,
})
const positionRules = reactive<strategyApi.PositionRules>({
  max_positions: 5, max_single_pct: 20, allocation: 'equal',
})

function markRulesDirty() { rulesDirty.value = true }
function ruleName(type: string): string {
  const map: Record<string, string> = {
    stop_loss: '止损', take_profit: '止盈', time_exit: '到期退出',
    trailing_stop: '移动止盈', segment_profit: '分段止盈',
  }
  return map[type] || type
}

function ruleDesc(type: string): string {
  const map: Record<string, string> = {
    stop_loss: '股价跌破止损线后立即平仓，控制最大亏损幅度',
    take_profit: '股价达到目标收益后自动止盈卖出',
    time_exit: '持仓超过指定自然日天数后，到期以当日收盘价强制平仓',
    trailing_stop: '从最高点回撤超过指定幅度后触发卖出，锁定利润',
    segment_profit: '分档位分批卖出，每达到一个收益目标卖出一定比例仓位',
  }
  return map[type] || ''
}

// ===== 信号退出选择器（复用条件选股模块） =====
// 已改为页签模式，sellSignals 独立管理卖出信号

/** 当前活跃的页签 */
type TabKey = 'buy_signals' | 'sell_signals' | 'exit_rules' | 'position'
const activeTab = ref<TabKey>('buy_signals')

/** 是否为信号类页签 */
const isSignalTab = computed(() => activeTab.value === 'buy_signals' || activeTab.value === 'sell_signals')

/** 卖出信号列表 */
interface SellSig {
  _key: number
  signal_id: string
  indicator_id: string
  indicator_name: string
  name: string
  category: Category
  operator: string  // 使用 string 以兼容 'none' 等特殊操作符
  opSym: string
  params: Record<string, any>
  paramText: string
}
let sellKeyCounter = 0
const sellSignals = ref<SellSig[]>([])

/** 当前活跃页签对应的信号数组 */
const activeTabSignals = computed(() => {
  return activeTab.value === 'buy_signals' ? signals.value : sellSignals.value
})

/** 从活跃页签移除信号 */
function removeActiveTabSignal(idx: number) {
  if (activeTab.value === 'buy_signals') {
    signals.value.splice(idx, 1)
  } else {
    sellSignals.value.splice(idx, 1)
  }
  markDirty()
}

/** 标记脏状态 */
function markDirty() { isDirty.value = true }
/** 清除脏状态 */
function clearDirty() { isDirty.value = false }

// 策略名称内联编辑
const editingNameBefore = ref('')   // 编辑前的原始名称，用于判断是否变更

function startEditName() {
  editingNameBefore.value = strategyName.value
  isEditingName.value = true
  nextTick(() => {
    nameInputRef.value?.focus()
    nameInputRef.value?.select()
  })
}

/** Enter 或 失焦时立即重命名（仅当有编辑 ID 且名称变更时） */
async function handleNameConfirm() {
  isEditingName.value = false
  const newName = strategyName.value.trim()
  if (!newName || newName === editingNameBefore.value) return
  if (!editingId.value) {
    // 新建模式还未保存，只更新内存
    return
  }
  try {
    await strategyApi.renameStrategy(editingId.value, newName)
    editingNameBefore.value = newName
  } catch (e) {
    console.error('重命名失败:', e)
    strategyName.value = editingNameBefore.value  // 回滚
    alert('重命名失败: ' + (e as Error).message)
  }
}

/** 滚动到条件选股区域 */
function scrollToIndicators() {
  const el = document.querySelector('.sec-signals')
  if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

// AI 输入文本
const aiText = ref('')
const aiExpanded = ref(false)

// ========== API 保存逻辑 ==========
import * as strategyApi from '../api/strategies'

/** 保存策略到后端（新建或更新） */
async function saveStrategy() {
  if (signals.value.length === 0) { alert('请先添加至少一个信号条件'); return }
  const name = strategyName.value.trim()
  if (!name) { alert('请输入策略名称'); return }

  try {
    // 构建 exit_rules：合并基础规则 + 卖出信号
    const baseRules = exitRules.rules.filter((r: any) => r.type !== 'signal_exit')
    const sellSignalRules = sellSignals.value.map(ss => ({
      type: 'signal_exit',
      enabled: true,
      params: { signal_id: ss.signal_id, operator: ss.operator, params: ss.params },
      priority: 5,
    }))
    const mergedExitRules = {
      rules: [...baseRules, ...sellSignalRules],
      slippage_pct: exitRules.slippage_pct,
    }

    const payload: Record<string, any> = {
      name,
      logical_op: logicalOp.value,
      signals: signals.value.map(s => ({
        signal_id: s.signal_id,
        operator: s.operator,
        params: s.params,
      })),
      description: '',
      exit_rules: mergedExitRules,
      position_rules: { ...positionRules },
    }

    let result
    if (editingId.value) {
      // 更新已有策略
      result = await strategyApi.updateStrategy(editingId.value, payload as any)
    } else {
      // 创建新策略
      result = await strategyApi.createStrategy(payload as any)
      editingId.value = result.id  // 保存后进入编辑模式
    }

    emit('saved', { id: result.id, name: result.name })
    clearDirty()
    rulesDirty.value = false
    saveSuccessMsg.value = '保存成功'
    if (saveSuccessTimer) clearTimeout(saveSuccessTimer)
    saveSuccessTimer = setTimeout(() => { saveSuccessMsg.value = '' }, 2500)
  } catch (e) {
    console.error('保存策略失败:', e)
    alert('保存失败: ' + (e as Error).message)
  }
}

/** 判断当前是否有未保存的修改（信号增删操作） */
function hasUnsavedChanges(): boolean {
  return isDirty.value || rulesDirty.value
}

/** 点击历史回测按钮 */
function onGoBacktest() {
  const needsSave = hasUnsavedChanges()
  if (needsSave) {
    showConfirmModal({
      title: '💾 保存提示',
      body: '当前有未保存的策略内容，是否保存后再前往回测？',
      okText: '保存并跳转',
      onOk: async () => {
        await saveStrategy()
        emit('goBacktest', editingId.value ?? null)
      },
    })
  } else {
    emit('goBacktest', editingId.value ?? null)
  }
}

// ========== 方法 ==========

/** 切换指标展开/收起（状态初始化由 watch expandedIndicatorID 统一处理） */
function toggleExpandIndicator(indID: string) {
  if (expandedIndicatorID.value === indID) {
    expandedIndicatorID.value = null
    return
  }
  expandedIndicatorID.value = indID
}

/** 添加内置信号（一键添加，无需配置） */
function addBuiltinSignal(sig: SignalDef) {
  if (!expandedInd.value) return
  const ind = expandedInd.value

  // 使用 default_config（如果有），否则使用空配置（无参内置信号如 513 战法）
  const cfg: SignalConfig = sig.default_config || {
    signal_id: sig.signal_id,
    operator: 'none' as CompareOperator,  // 无参信号使用特殊操作符
    params: {},
  }
  const text = formatSignalParamText(cfg, ind)
  const isNoneOp = (cfg.operator as string) === 'none'

  if (activeTab.value === 'sell_signals') {
    // 卖出信号
    sellSignals.value.push({
      _key: ++sellKeyCounter,
      signal_id: cfg.signal_id,
      indicator_id: ind.id,
      indicator_name: ind.name,
      name: sig.alias || sig.name,
      category: ind.category,
      operator: cfg.operator,
      opSym: isNoneOp ? '' : ((operatorSymbols as any)[cfg.operator] || cfg.operator),
      params: { ...cfg.params },
      paramText: text,
    })
    markDirty()
    return
  }

  const newSig: Sig = {
    uid: ++uidCounter,
    indicator_id: ind.id,
    indicator_name: ind.name,
    signal_id: cfg.signal_id,
    name: sig.alias || sig.name,
    category: ind.category,
    operator: cfg.operator,
    opSym: isNoneOp ? '' : ((operatorSymbols as any)[cfg.operator] || cfg.operator),
    opLbl: findOpLabel(ind, cfg.operator),
    params: { ...cfg.params },
    paramText: text,
  }
  signals.value.push(newSig)
  markDirty()
  emit('addSignals', [newSig])
}

/** 添加自定义信号（从表单收集操作符+参数） */
function addCustomSignal() {
  if (!expandedInd.value || !currentCustomSig.value) return
  const ind = expandedInd.value
  const sig = currentCustomSig.value
  const op = sig.operators.find(o => o.operator === customOperator.value)
  if (!op) return

  // 收集参数值
  const collected: Record<string, any> = {}
  if (op.params) {
    for (const p of op.params) {
      if (p.type === 'multi_select' || p.type === 'select_multi') { collected[p.key] = [...(multiVals[p.key] || [])] }
      else if (paramValues[p.key] !== undefined) { collected[p.key] = paramValues[p.key] }
      else if (p.default !== undefined) { collected[p.key] = p.default }
    }
  }

  // 构建可读文本
  const text = formatSignalParamText(
    { signal_id: sig.signal_id, operator: customOperator.value, params: collected } as SignalConfig,
    ind,
  )

  if (activeTab.value === 'sell_signals') {
    // 卖出信号
    sellSignals.value.push({
      _key: ++sellKeyCounter,
      signal_id: sig.signal_id,
      indicator_id: ind.id,
      indicator_name: ind.name,
      name: sig.alias || sig.name || ind.name,
      category: ind.category,
      operator: customOperator.value,
      opSym: operatorSymbols[customOperator.value] || customOperator.value,
      params: collected,
      paramText: text,
    })
    markDirty()
    clearParams()
    return
  }

  const newSig: Sig = {
    uid: ++uidCounter,
    indicator_id: ind.id,
    indicator_name: ind.name,
    signal_id: sig.signal_id,
    name: sig.alias || sig.name || ind.name,
    category: ind.category,
    operator: customOperator.value,
    opSym: operatorSymbols[customOperator.value] || customOperator.value,
    opLbl: op.label || findOpLabel(ind, customOperator.value),
    params: collected,
    paramText: text,
  }
  signals.value.push(newSig)
  markDirty()
  emit('addSignals', [newSig])
  clearParams()
}
function clearParams() {
  for (const k of Object.keys(paramValues)) delete paramValues[k]
  for (const k of Object.keys(multiVals)) multiVals[k] = []
}
function onOperatorChange() {
  clearParams()
}
function isNumberLike(t: string): boolean { return ['number', 'range', 'threshold', 'days'].includes(t) }
function isMultiSelect(t: string): boolean { return ['multi_select', 'select_multi'].includes(t) }

/** 切换多选项的选中状态 */
function toggleMultiVal(key: string, value: string) {
  if (!multiVals[key]) multiVals[key] = []
  const idx = multiVals[key].indexOf(value)
  if (idx >= 0) {
    multiVals[key].splice(idx, 1)
  } else {
    multiVals[key].push(value)
  }
}

/** 获取操作符的显示符号 */
function operatorSymbol(op: CompareOperator): string { return operatorSymbols[op] || op }

/** 从信号的操作符列表中找操作符标签 */
function findOpLabel(ind: IndicatorMeta, op: CompareOperator): string {
  // 遍历所有信号的操作符查找标签
  for (const sig of ind.signals) {
    if (!sig.operators) continue
    const found = sig.operators.find(o => o.operator === op)
    if (found) return found.label
  }
  return op
}

/** 将 SignalConfig 格式化为可读参数文本 */
function formatSignalParamText(cfg: SignalConfig, ind: IndicatorMeta): string {
  const params = cfg.params || {}
  // 辅助：从信号定义中查找枚举选项的 value→label 映射
  const findEnumLabels = (sigId: string, key: string): Map<string, string> | null => {
    for (const sig of ind.signals) {
      if (sig.signal_id === sigId) {
        if (!sig.operators) continue
        for (const op of sig.operators) {
          if (!op.params) continue
          for (const p of op.params) {
            if (p.key === key && p.options) return new Map(p.options.map(o => [o.value, o.label]))
          }
        }
      }
    }
    return null
  }

  switch (cfg.operator) {
    case 'gt':   return `${params.threshold ?? ''}${ind.unit}${formatDaysSuffix(cfg, ind)}`
    case 'gte':  return `${params.threshold ?? ''}${ind.unit}${formatDaysSuffix(cfg, ind)}`
    case 'lt':   return `${params.threshold ?? ''}${ind.unit}${formatDaysSuffix(cfg, ind)}`
    case 'lte':  return `${params.threshold ?? ''}${ind.unit}${formatDaysSuffix(cfg, ind)}`
    case 'between': case 'not_between':
      return `${params.min ?? ''}~${params.max ?? ''}${ind.unit}${formatDaysSuffix(cfg, ind)}`
    case 'eq': {
      const labelMap = findEnumLabels(cfg.signal_id!, 'threshold')
      if (labelMap && params.threshold !== undefined) {
        return labelMap.get(String(params.threshold)) || String(params.threshold)
      }
      return String(params.threshold ?? '')
    }
    case 'neq': {
      const neqLabelMap = findEnumLabels(cfg.signal_id!, 'threshold')
      if (neqLabelMap && params.threshold !== undefined) {
        return neqLabelMap.get(String(params.threshold)) || String(params.threshold)
      }
      return String(params.threshold ?? '')
    }
    case 'in': case 'not_in': {
      const vals = params.values as string[] | undefined
      if (!vals || vals.length === 0) return '{}'
      const labelMap = findEnumLabels(cfg.signal_id!, 'values')
      if (labelMap) {
        return `{${vals.map(v => labelMap.get(v) || v).join(',')}}`
      }
      return `{${vals.join(',')}}`
    }
    case 'custom': {
      // 遍历信号定义中的所有 param，拼接 label + value + unit
      const parts: string[] = []
      for (const sig of ind.signals) {
        if (sig.signal_id === cfg.signal_id) {
          for (const op of sig.operators) {
            if (!op.params) continue
            for (const p of op.params) {
              const val = params[p.key]
              const label = p.label || p.key
              const unit = p.unit || ''
              parts.push(`${label}${val ?? p.default ?? ''}${unit}`)
            }
          }
          break
        }
      }
      return parts.length > 0 ? parts.join(', ') : Object.entries(params).map(([k, v]) => `${k}=${v}`).join(', ')
    }
    default:
      return Object.entries(params).map(([k, v]) => `${k}=${v}`).join(', ')
  }
}
/** 当 params.days > 0 时返回取值天数后缀，如 " (2天前)" */
function formatDaysSuffix(cfg: SignalConfig, ind: IndicatorMeta): string {
  const days = (cfg.params as Record<string, any>)?.days
  if (!days || Number(days) <= 0) return ''
  // 从信号定义的 days 参数中读取 unit
  let unit = '天前'
  for (const sig of ind.signals) {
    if (sig.signal_id === cfg.signal_id) {
      for (const op of sig.operators) {
        if (!op.params) continue
        for (const p of op.params) {
          if (p.key === 'days' && p.unit) { unit = p.unit; break }
        }
      }
    }
  }
  return ` (${days}${unit})`
}
function onClearClick() {
  const tabLabel = activeTab.value === 'buy_signals' ? '买入信号' : '卖出信号'
  const count = activeTab.value === 'buy_signals' ? signals.value.length : sellSignals.value.length
  showConfirmModal({
    title: '⚠️ 确认清空',
    body: `确定要清空全部${tabLabel}（${count} 个条件）吗？此操作不可撤销。`,
    onOk: () => {
      if (activeTab.value === 'buy_signals') {
        signals.value = []
      } else {
        sellSignals.value = []
      }
      markDirty()
    },
  })
}


/** 根据 signal_id 查找信号名称 */
function findSignalName(ind: IndicatorMeta, signalId: string): string {
  const sig = ind.signals.find(s => s.signal_id === signalId)
  return sig ? (sig.alias || sig.name) : signalId
}

/** 从 signal_id 提取 indicator_id（前5位） */
function getIndicatorID(signalId: string): string {
  return signalId.length >= 5 ? signalId.substring(0, 5) : signalId
}

/** 补全单个信号的前端字段 */
function enrichSignal(raw: any): Sig {
  const indId = getIndicatorID(raw.signal_id)
  let ind: IndicatorMeta | null = null
  for (const cat of ['technical', 'market', 'fundamental', 'financial']) {
    ind = allData.value[cat as Category]?.find(i => i.id === indId) || null
    if (ind) break
  }
  if (!ind) {
    return {
      uid: ++uidCounter,
      indicator_id: indId,
      indicator_name: indId,
      signal_id: raw.signal_id,
      name: raw.signal_id,
      category: 'technical',
      operator: raw.operator,
      opSym: (operatorSymbols as Record<string, string>)[raw.operator as string] || raw.operator,
      opLbl: raw.operator,
      params: { ...raw.params },
      paramText: JSON.stringify(raw.params),
    }
  }
  const text = formatSignalParamText(raw, ind)
  return {
    uid: ++uidCounter,
    indicator_id: indId,
    indicator_name: ind.name,
    signal_id: raw.signal_id,
    name: findSignalName(ind, raw.signal_id),
    category: ind.category,
    operator: raw.operator,
    opSym: (operatorSymbols as Record<string, string>)[raw.operator as string] || raw.operator,
    opLbl: findOpLabel(ind, raw.operator),
    params: { ...raw.params },
    paramText: text,
  }
}

function handleAISubmit() { /* TODO: 对接 AI 解析 */ }

function exportJSON() {
  const json = JSON.stringify({
    buy_signals: signals.value.map(s => ({
      signal_id: s.signal_id,
      operator: s.operator,
      params: s.params,
    })),
    sell_signals: sellSignals.value.map(ss => ({
      signal_id: ss.signal_id,
      operator: ss.operator,
      params: ss.params,
    })),
  }, null, 2)
  const blob = new Blob([json], { type: 'application/json' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `策略信号_${Date.now()}.json`
  a.click()
  URL.revokeObjectURL(a.href)
}

async function importJSON(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = async () => {
    try {
      const data = JSON.parse(reader.result as string)
      const rawSignals = Array.isArray(data) ? data : (data.conditions || [])
      if (rawSignals.length === 0) {
        alert('文件中没有找到有效的信号数据')
        return
      }
      // 如果当前已有信号，用通用弹窗提示覆盖
      if (signals.value.length > 0) {
        showConfirmModal({
          title: '⚠️ 确认导入',
          body: `导入将会覆盖当前已有的 ${signals.value.length} 个信号条件，是否继续？`,
          okText: '确认导入',
          onOk: () => { doImport(rawSignals) },
        })
        return
      }
      await doImport(rawSignals)
    } catch (err) {
      console.error('[StrategyBuilder] 导入失败:', err)
      alert('导入失败：文件格式不正确')
    } finally {
      ;(e.target as HTMLInputElement).value = ''
    }
  }
  reader.readAsText(file)
}
async function doImport(rawSignals: any[]) {
  await loadIndicators()
  signals.value = []
  uidCounter = 0
  for (const raw of rawSignals) {
    signals.value.push(enrichSignal(raw))
  }
  markDirty()
  console.warn('[StrategyBuilder] 导入成功', signals.value)
}
// ========== 筛选执行 ==========
const isScreening = ref(false)
const isAddingFav = ref(false)
const favToastMsg = ref('')
const favToastOk = ref(true)
let favToastTimer: ReturnType<typeof setTimeout> | null = null

function showFavToast(msg: string, ok = true) {
  favToastMsg.value = msg
  favToastOk.value = ok
  if (favToastTimer) clearTimeout(favToastTimer)
  favToastTimer = setTimeout(() => { favToastMsg.value = '' }, 3500)
}
const screenResult = ref<{ passed: any[]; rejected: any[]; total: number } | null>(null)
const screenError = ref('')

// ========== 表格选择状态 ==========
const selectedRows = ref<Set<number>>(new Set())
const allSelected = computed(() =>
  screenResult.value && screenResult.value.passed.length > 0 && selectedRows.value.size === screenResult.value.passed.length
)
const someSelected = computed((): boolean => selectedRows.value.size > 0 && !!allSelected.value === false)
/** indeterminate 状态（绕过 Vue 模板类型推断） */
const indeterminate = computed(() => Boolean(someSelected.value))

function toggleAll() {
  if (!screenResult.value) return
  if (allSelected.value) {
    selectedRows.value.clear()
  } else {
    selectedRows.value = new Set(screenResult.value.passed.map((_, i) => i))
  }
}
function toggleRow(idx: number) {
  if (selectedRows.value.has(idx)) {
    selectedRows.value.delete(idx)
  } else {
    selectedRows.value.add(idx)
  }
}

// ========== 前端排序 & 分页 ==========
const sortField = ref<string>('')
const sortOrder = ref<'asc' | 'desc'>('asc')

/** 切换排序：同一字段三次循环 (asc → desc → 无排序) */
function toggleSort(field: string) {
  if (sortField.value === field) {
    if (sortOrder.value === 'asc') {
      sortOrder.value = 'desc'
    } else {
      sortField.value = ''
      sortOrder.value = 'asc'
    }
  } else {
    sortField.value = field
    sortOrder.value = 'asc'
  }
}

const searchKeyword = ref('')
const currentPage = ref(1)
const pageSize = ref(12)
const pageSizes = [12, 24, 48, 96]

// 表格视图：overview=概览 financial=财务（与信号 activeTab 区分）
const tableTab = ref<'overview' | 'financial'>('overview')

// 结果视图模式：list=股票列表 multiKline=多股同列
const resultViewMode = ref<'list' | 'multiKline'>('list')

// 多股同列周期（与工具栏联动）
const multiKlinePeriod = ref<'daily' | 'weekly' | 'monthly'>('daily')
// 多股同列列数（与工具栏联动）
const multiKlineColumns = ref(2)

// ========== K 线图悬浮 ==========
const klineVisible = ref(false)
const klineStockCode = ref('')
const klineStockName = ref('')
const klineX = ref(0)
const klineY = ref(0)
let klineTimer: ReturnType<typeof setTimeout> | null = null   // 显示延迟计时器
let klineHideTimer: ReturnType<typeof setTimeout> | null = null // 隐藏延迟计时器

function showKLine(e: MouseEvent, stock: any) {
  // 取消待执行的隐藏（鼠标从弹窗移回名称时）
  if (klineHideTimer) { clearTimeout(klineHideTimer); klineHideTimer = null }
  if (klineTimer) clearTimeout(klineTimer)
  klineTimer = setTimeout(() => {
    klineStockCode.value = stock.code
    klineStockName.value = stock.name
    klineX.value = e.clientX
    klineY.value = e.clientY
    klineVisible.value = true
  }, 350) // 延迟显示，避免快速划过时闪烁
}

function hideKLine() {
  // 取消显示计时器
  if (klineTimer) { clearTimeout(klineTimer); klineTimer = null }
  // 延迟 200ms 隐藏，给用户时间从名称移动到弹窗
  if (!klineHideTimer) {
    klineHideTimer = setTimeout(() => {
      klineVisible.value = false
      klineHideTimer = null
    }, 200)
  }
}

/** 弹窗mouseenter时取消隐藏 */
function onKLineEnter() {
  if (klineHideTimer) { clearTimeout(klineHideTimer); klineHideTimer = null }
}

/** 根据纯数字代码推导交易所前缀 */
function getExchangePrefix(code: string): string {
  const c = code.charAt(0)
  if (c === '6') return 'sh'
  if (c === '0' || c === '3') return 'sz'
  if (c === '8' || c === '9') return 'bj'
  return 'sz'
}

function getEastMoneyUrl(code: string): string {
  return `https://quote.eastmoney.com/concept/${getExchangePrefix(code)}${code}.html#chart-k-cyq`
}

function getTHSUrl(code: string): string {
  return `https://www.iwencai.com/screener/result?w=${code}&querytype=stock&sign=1781436668603`
}

function getTencentUrl(code: string): string {
  return `https://gu.qq.com/${getExchangePrefix(code)}${code}/gp`
}

/** 先过滤，后排序 */
const filteredData = computed(() => {
  if (!screenResult.value) return []
  const kw = searchKeyword.value.toLowerCase()
  if (!kw) return screenResult.value.passed
  return screenResult.value.passed.filter((s: any) =>
    (s.code ?? '').toLowerCase().includes(kw) || (s.name ?? '').toLowerCase().includes(kw)
  )
})

/** 过滤后再排序 */
const sortedData = computed(() => {
  const list = [...filteredData.value]
  if (!sortField.value) return list

  return list.sort((a: any, b: any) => {
    let va: any, vb: any
    switch (sortField.value) {
      case 'price':
        va = a.price ?? 0
        vb = b.price ?? 0
        break
      case 'pe_ttm':
        va = a.pe_ttm ?? Number.MAX_VALUE
        vb = b.pe_ttm ?? Number.MAX_VALUE
        break
      case 'circulate_market_cap':
        va = a.circulate_market_cap ?? 0
        vb = b.circulate_market_cap ?? 0
        break
      case 'total_market_cap':
        va = a.total_market_cap ?? 0
        vb = b.total_market_cap ?? 0
        break
      case 'bvps':
        va = a.bvps ?? 0
        vb = b.bvps ?? 0
        break
      case 'basic_eps':
        va = a.basic_eps ?? Number.MIN_SAFE_INTEGER
        vb = b.basic_eps ?? Number.MIN_SAFE_INTEGER
        break
      case 'pb':
        va = a.pb ?? Number.MAX_VALUE
        vb = b.pb ?? Number.MAX_VALUE
        break
      default:
        return 0
    }
    if (typeof va === 'number' && typeof vb === 'number') {
      return sortOrder.value === 'asc' ? va - vb : vb - va
    }
    return 0
  })
})

const paginatedData = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return sortedData.value.slice(start, start + pageSize.value)
})

const totalPage = computed(() => {
  if (!filteredData.value.length) return 1
  return Math.ceil(filteredData.value.length / pageSize.value)
})

// 页码显示逻辑（省略号）
const visiblePages = computed(() => {
  const total = totalPage.value
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)
  const cur = currentPage.value
  const pages: (number | string)[] = [1]
  if (cur > 3) pages.push('...')
  for (let i = Math.max(2, cur - 1); i <= Math.min(total - 1, cur + 1); i++) pages.push(i)
  if (cur < total - 2) pages.push('...')
  pages.push(total)
  return pages
})

const jumpPageInput = ref(1)

function goPage(p: number) { if (p >= 1 && p <= totalPage.value) currentPage.value = p }
function prevPage() { if (currentPage.value > 1) currentPage.value-- }
function nextPage() { if (currentPage.value < totalPage.value) currentPage.value++ }
function onPageSizeChange(e: Event) {
  pageSize.value = Number((e.target as HTMLSelectElement).value)
  currentPage.value = 1
}
function goJumpPage() { goPage(jumpPageInput.value) }

// 重置分页和选中
watch(totalPage, () => { currentPage.value = 1; selectedRows.value.clear() })
watch(searchKeyword, () => { currentPage.value = 1 })

async function runFilter() {
  if (signals.value.length === 0) { alert('请先添加至少一个信号条件'); return }

  isScreening.value = true
  screenError.value = ''
  screenResult.value = null
  selectedRows.value.clear()
  currentPage.value = 1

  try {
    const res = await indicatorsApi.executeScreen({
      configs: signals.value.map(s => ({
        signal_id: s.signal_id,
        operator: s.operator,
        params: s.params,
      })),
      max_concurrency: 200,
      date: runDate.value,
    })

    screenResult.value = {
      total: res.total,
      passed: res.passed || [],
      rejected: res.rejected || [],
    }
  } catch (e: any) {
    console.error('筛选执行失败:', e)
    screenError.value = e.message || '筛选执行失败'
  } finally {
    isScreening.value = false
  }
}

// ========== 一键加入自选 ==========
async function batchAddFavorites() {
  if (!editingId.value) { showFavToast('请先保存策略', false); return }
  if (!screenResult.value?.passed.length) { showFavToast('请先运行筛选', false); return }

  const codes = filteredData.value.map((s: any) => s.code)
  const dateStr = runDate.value.replace(/-/g, '') // YYYY-MM-DD → YYYYMMDD

  isAddingFav.value = true
  try {
    const resp = await strategyApi.batchAddToFavorites({
      strategy_id: editingId.value,
      date: dateStr,
      stock_codes: codes,
    })

    if (resp.failed && resp.failed.length > 0) {
      // 将失败 code 转为简称
      const nameMap = new Map(screenResult.value.passed.map((s: any) => [s.code, s.name]))
      const failedNames = resp.failed.map(c => `${c}(${nameMap.get(c) || '?'})`)
      showFavToast(`已添加 ${resp.total - resp.failed.length}/${resp.total} 只到「${resp.gname}」，失败: ${failedNames.join(', ')}`, false)
    } else {
      showFavToast(`✅ 成功添加 ${resp.total} 只到「${resp.gname}」`)
    }
  } catch (e: any) {
    console.error('加入自选失败:', e)
    showFavToast('加入自选失败: ' + (e.message || '未知错误'), false)
  } finally {
    isAddingFav.value = false
  }
}

/** 外部调用：接收 AI 解析的信号 */
function acceptAISignals(aiSignals: any[]) {
  for (const s of aiSignals) { signals.value.push({
    uid: ++uidCounter,
    indicator_id: String(s.indicatorID || s.indicator_id),
    signal_id: String(s.signalID || s.signal_id),
    name: String(s.indicatorName || s.name),
    category: (s.category as Category) ?? 'technical',
    operator: (s.operator as CompareOperator) ?? 'gt',
    opSym: (operatorSymbols as Record<string, string>)[String(s.operator)] || String(s.operatorSymbol || s.operator),
    opLbl: String(s.operatorLabel || s.operator),
    params: (s.params && typeof s.params === 'object') ? s.params : {},
    paramText: String(s.paramSummary || ''),
  } as Sig) }
  markDirty()
}
/** 从策略列表加载策略到编辑器 */
async function loadStrategyFromOutside(s: { id: string | number; name: string; signals: any[]; logicalOp: 'AND' | 'OR'; uid?: number }) {
  try {
    console.warn('[StrategyBuilder] loadStrategyFromOutside', JSON.stringify(s))
    editingId.value = typeof s.id === 'number' ? s.id : parseInt(s.id)
    strategyOwnerId.value = s.uid ?? 0
    strategyName.value = s.name
    // 确保指标数据已加载
    await loadIndicators()
    // 补全前端字段
    const enriched = (s.signals || []).map(raw => {
      console.warn('[StrategyBuilder] enriching signal', raw)
      if (raw.name && raw.paramText !== undefined && raw.uid) return { ...raw, uid: ++uidCounter }
      return enrichSignal(raw)
    })
    signals.value = enriched
    logicalOp.value = s.logicalOp || 'AND'
    // 加载卖出规则和仓位管理
    sellSignals.value = []
    sellKeyCounter = 0
    if ((s as any).exit_rules) {
      try {
        const er = JSON.parse((s as any).exit_rules)
        if (er.rules) {
          exitRules.rules.length = 0
          // 分离 signal_exit 规则 → sellSignals
          const nonSignalRules: any[] = []
          for (const rule of er.rules) {
            if (rule.type === 'signal_exit' && rule.enabled && rule.params?.signal_id) {
              // 重建 sellSignal 条目
              const sigName = findSignalNameById(rule.params.signal_id)
              sellSignals.value.push({
                _key: ++sellKeyCounter,
                signal_id: rule.params.signal_id,
                indicator_id: getIndicatorID(rule.params.signal_id),
                indicator_name: sigName ? sigName.indicatorName : '',
                name: sigName ? sigName.sigName : rule.params.signal_id,
                category: sigName ? sigName.category : 'technical',
                operator: rule.params.operator || '',
                opSym: (operatorSymbols as any)[rule.params.operator] || rule.params.operator || '',
                params: { ...(rule.params.params || {}) },
                paramText: '',
              })
            } else {
              nonSignalRules.push(rule)
            }
          }
          exitRules.rules.push(...nonSignalRules)
        }
        if (er.slippage_pct != null) exitRules.slippage_pct = er.slippage_pct
      } catch {}
    }
    if ((s as any).position_rules) {
      try {
        const pr = JSON.parse((s as any).position_rules)
        if (pr.max_positions != null) positionRules.max_positions = pr.max_positions
        if (pr.max_single_pct != null) positionRules.max_single_pct = pr.max_single_pct
        if (pr.allocation) positionRules.allocation = pr.allocation
      } catch {}
    }
    clearDirty()
    rulesDirty.value = false
  } catch (e) {
    console.error('[StrategyBuilder] 加载策略失败:', e)
  }
}

/** 根据 signal_id 在整个指标数据中查找信号名称（跨分类） */
function findSignalNameById(signalId: string): { sigName: string; indicatorName: string; category: Category } | null {
  const indId = getIndicatorID(signalId)
  for (const cat of ['technical', 'market', 'fundamental', 'financial'] as Category[]) {
    const ind = allData.value[cat]?.find(i => i.id === indId)
    if (ind) {
      const sig = ind.signals.find(s => s.signal_id === signalId)
      return {
        sigName: sig ? (sig.alias || sig.name) : signalId,
        indicatorName: ind.name,
        category: cat,
      }
    }
  }
  return null
}
function resetAllSignals() { editingId.value = null; strategyOwnerId.value = 0; strategyName.value = ''; signals.value = []; sellSignals.value = []; sellKeyCounter = 0; logicalOp.value = 'AND'; uidCounter = 0; clearDirty(); rulesDirty.value = false; activeTab.value = 'buy_signals' }

defineExpose({ acceptAISignals, loadStrategyFromOutside, resetAllSignals })
</script>

<style scoped>
/* ========== 三段式主布局 ========== */
.strategy-builder {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* ========== 页面顶部标题栏（卡片外） ========== */
.page-top-bar {
  display: flex; align-items: center; justify-content: space-between;
  margin-bottom: 14px;
}
.ptb-left {
  display: flex; align-items: center; gap: 10px;
}
.ptb-right {
  display: flex; align-items: center; gap: 8px;
}
.back-btn {
  width: 30px; height: 30px; border: 1px solid #d9d9d9; border-radius: 6px;
  background: #fff; cursor: pointer; font-size: 16px; color: #555;
  display: flex; align-items: center; justify-content: center; transition: .15s;
}
.back-btn:hover { background: #f5f5f5; color: #1677ff; border-color: #1677ff; }
.page-title { font-size: 20px; font-weight: 700; color: #1a1a2e; }

/* 内联策略名称（顶部栏右侧） */
.inline-name-text {
  font-size: 14px; color: #1677ff; cursor: pointer;
  padding: 4px 10px; border-radius: 4px; border: 1px solid transparent;
  transition: all .12s;
}
.inline-name-text:hover { background: #e6f4ff; border-color: #91caff; }
.inline-name-text.readonly {
  cursor: default; color: #555;
}
.inline-name-text.readonly:hover { background: none; border-color: transparent; }
.inline-name-input {
  padding: 4px 10px; border: 1px solid #1677ff; border-radius: 4px;
  font-size: 14px; outline: none; color: #1a1a2e; font-weight: 500;
  width: 180px; background: #fff;
}
.inline-name-input:focus { box-shadow: 0 0 0 2px rgba(22,119,255,.08); }

/* 顶部栏按钮（统一样式） */
.btn-save-sm {
  padding: 6px 16px; font-size: 13px; font-weight: 500;
  color: #fff; background: #1677ff; border: 1px solid #1677ff;
  border-radius: 5px; cursor: pointer; transition: .12s; white-space: nowrap;
}
.btn-save-sm:hover { background: #0958d9; }
.btn-save-sm:disabled { opacity: 0.45; cursor: not-allowed; }
.btn-save-sm.btn-outline-sm {
  color: #555; background: #fff; border: 1px solid #d9d9d9;
}
.btn-save-sm.btn-outline-sm:hover { color: #1677ff; border-color: #1677ff; }

/* ========== Section 1: AI 输入区 ========== */
.sec-input { background: #fff; border: 1px solid #e8e8e8; border-radius: 10px; overflow: hidden; }

.ai-input-area { padding: 14px 20px 14px; }
.ai-input-area textarea {
  width: 100%; border: 1.5px solid #e0e0e0; border-radius: 8px;
  padding: 12px 16px; font-size: 14px; color: #333; resize: none;
  outline: none; transition: border-color .15s; font-family: inherit; line-height: 1.6;
  box-sizing: border-box;
}
.ai-input-area textarea:focus { border-color: #1677ff; box-shadow: 0 0 0 3px rgba(22,119,255,.06); }
.ai-input-area textarea::placeholder { color: #bbb; }

.ai-toolbar {
  display: flex; justify-content: space-between; align-items: center;
  margin-top: 10px; padding-top: 10px; border-top: 1px solid #f0f0f0;
}
.ai-tools-left, .ai-tools-right { display: flex; align-items: center; gap: 6px; }
.ai-tool-label {
  font-size: 13px; color: #555; padding: 4px 12px; border-radius: 4px;
  cursor: pointer; transition: .12s;
}
.ai-tool-label.active { color: #1677ff; font-weight: 500; }
.ai-tool-label:hover { background: #f5f5f5; color: #333; }
.ai-tool-label.active { color: #cf1322; font-weight: 600; }
.ai-tool-label.dim { color: #bbb; cursor: default; }
.ai-tool-label.dim:hover { background: none; }
.ai-hint-text { font-size: 12px; color: #999; margin-right: 8px; }
.btn-ai-send {
  padding: 6px 18px; border: none; border-radius: 6px; background: #1677ff; color: #fff;
  font-size: 13px; font-weight: 600; cursor: pointer; transition: .15s; white-space: nowrap;
}
.btn-ai-send:hover:not(:disabled) { background: #0958d9; }
.btn-ai-send:disabled { background: #d9d9d9; cursor: not-allowed; }

/* ========== Section 2: 条件选股（指标平铺网格） ========== */
.sec-signals { background: #fff; border: 1px solid #e8e8e8; border-radius: 10px; padding: 16px 20px; }

.sec-header-row {
  display: flex; justify-content: space-between; align-items: center;
  margin-bottom: 14px;
}
.sec-left { display: flex; align-items: center; gap: 10px; }
.sec-title { font-size: 15px; font-weight: 700; color: #1a1a2e; margin: 0; }
.sig-count-tag {
  font-size: 11.5px; padding: 2px 10px; border-radius: 10px;
  background: #fff7e6; color: #d46b08; font-weight: 600;
}
.sec-right { display: flex; gap: 8px; }
.btn-sec-sm { padding: 5px 14px; border: 1px solid #d9d9d9; border-radius: 6px; background: #fff; font-size: 12px; cursor: pointer; color: #666; }
.btn-sec-sm:hover { border-color: #cf1322; color: #cf1322; }

/* ====== 信号页签（买入/卖出） ====== */
.signal-tabs {
  display: flex; gap: 0; margin-bottom: 14px;
  border-bottom: 2px solid #e8e8e8;
}
.signal-tabs-inline {
  display: flex; gap: 0;
}
.signal-tab {
  padding: 8px 18px; border: none; background: transparent;
  font-size: 14px; font-weight: 500; color: #888; cursor: pointer;
  transition: all .15s; border-bottom: 2px solid transparent;
  margin-bottom: -2px; display: flex; align-items: center; gap: 6px;
}
.signal-tab:hover { color: #1677ff; }
.signal-tab.active {
  color: #1677ff; font-weight: 700;
  border-bottom-color: #1677ff;
}
.tab-badge {
  display: inline-flex; align-items: center; justify-content: center;
  min-width: 20px; height: 20px; padding: 0 4px;
  border-radius: 10px; font-size: 11px; font-weight: 700;
  background: #e6f4ff; color: #1677ff;
}
.signal-tab.active .tab-badge {
  background: #1677ff; color: #fff;
}

/* ====== 指标平铺网格（四列横排） ====== */
.indicators-loading {
  display: flex; align-items: center; justify-content: center;
  gap: 10px; padding: 40px 20px; font-size: 13.5px; color: #999;
}
.loading-spinner {
  width: 18px; height: 18px; border: 2px solid #e0e0e0;
  border-top-color: #1677ff; border-radius: 50%;
  animation: spin 0.7s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
.indicators-flat-area {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 4px;
}
.cat-column {
  display: flex;
  flex-direction: column;
  border: 1px solid #d0d0d0;
  border-radius: 8px;
  padding: 10px 12px;
  background: linear-gradient(to bottom, #f0f9ff, #fff);
}
.cat-section-header {
  font-size: 12.5px; font-weight: 700; color: #888;
  padding-bottom: 6px; letter-spacing: 0.5px;
}
.indicator-grid {
  display: flex; flex-wrap: wrap; gap: 6px;
}
.ind-drop-btn {
  display: inline-flex; align-items: center; gap: 3px;
  padding: 4px 11px; border: 1px solid #e0e0e0; border-radius: 6px;
  background: #fafafa; cursor: pointer; font-size: 13px; font-weight: 500;
  color: #444; transition: all .12s; white-space: nowrap;
}
.ind-drop-btn:hover { border-color: #1677ff; color: #1677ff; background: #f0f7ff; }
.ind-drop-btn.expanded {
  border-color: #1677ff; color: #1677ff; background: #e6f4ff; font-weight: 600;
}
.drop-arrow { font-size: 9px; opacity: .5; transition: transform .2s; }

/* ====== 展开面板 ====== */
.ind-expand-panel {
  margin: 10px 0 14px; padding: 16px 18px;
  border: 1px solid #d0d0d0; border-radius: 10px;
  background: linear-gradient(to bottom, #f0f9ff, #fff);
}
.expand-header {
  display: flex; align-items: baseline; gap: 8px; margin-bottom: 12px;
}
.expand-ind-name { font-size: 15px; font-weight: 700; color: #1a1a2e; }
.expand-ind-desc { font-size: 12px; color: #999; }
.expand-close-btn {
  margin-left: auto; border: none; background: none; cursor: pointer;
  font-size: 14px; color: #bbb; padding: 2px 6px; border-radius: 4px;
}
.expand-close-btn:hover { background: #f0f0f0; color: #666; }
.expand-label {
  font-size: 11.5px; font-weight: 600; color: #888;
  text-transform: uppercase; letter-spacing: 0.5px; margin: 10px 0 6px;
}

/* 预设信号紧凑列表 */
.preset-list-compact { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 8px; }
.preset-mini {
  display: inline-flex; align-items: center; gap: 4px; padding: 5px 10px;
  border: 1.5px solid #e8e8e8; border-radius: 6px; cursor: pointer;
  background: #fafafa; transition: all .12s; text-align: left;
  font-size: 13px; flex: 0 0 calc(16.66% - 6px); max-width: calc(16.66% - 6px);
  position: relative;
}
.preset-mini:hover { border-color: #91caff; background: #f0f7ff; }
.preset-mini.selected { border-color: #1677ff; background: #e6f4ff; }
/* 内置信号按钮：tooltip 改为 Teleport + position:fixed（见 .builtin-tooltip），此处不再需要伪元素 */
.builtin-tooltip {
  position: fixed; z-index: 9999;
  padding: 8px 12px; background: #fff; color: #333; font-size: 12px; line-height: 1.6;
  border: 1px solid #d0d0d0; border-radius: 8px;
  box-shadow: 0 4px 16px rgba(0,0,0,.14);
  pointer-events: none; white-space: normal; word-break: break-word;
}
.pm-name { font-size: 13px; font-weight: 600; color: #1a1a2e; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.pm-desc { font-size: 11.5px; color: #999; margin-left: 4px; }
.pm-add {
  font-size: 12px; font-weight: 600; color: #389e0d; white-space: nowrap;
}

/* 自定义快速添加表单 */
.quick-add-form {
  display: flex; flex-direction: column; gap: 6px;
  padding: 10px 12px;
  border: 1.5px solid #e8e8e8; border-radius: 8px;
  background: #fafafa;
}
.qf-row {
  display: flex; align-items: center; gap: 8px; flex-wrap: wrap;
}
.qf-operator { display: flex; align-items: center; }
.qf-op-select {
  padding: 5px 10px; border: 1px solid #d9d9d9; border-radius: 6px;
  font-size: 13px; font-weight: 500; color: #333; outline: none; cursor: pointer;
  background: #fff; min-width: 80px;
}
.qf-op-select:focus { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.06); }
.qf-params { display: flex; gap: 8px; flex: 1; align-items: center; flex-wrap: wrap; }
.qf-param-field { display: flex; align-items: center; gap: 4px; }
.qf-param-label { font-size: 12.5px; color: #666; white-space: nowrap; }
.qf-param-unit { font-size: 12px; color: #999; white-space: nowrap; }
.qf-input {
  padding: 5px 10px; border: 1px solid #d9d9d9; border-radius: 6px;
  font-size: 13px; outline: none; color: #333; width: 110px;
  background: #fff;
}
.qf-input:focus { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.08); }
.qf-select { width: 130px; }
.qf-select-sig { min-width: 100px; font-weight: 600; color: #1a1a2e; }
.qf-sig-desc { font-size: 12px; color: #888; line-height: 1.5; }
.qf-no-params { font-size: 12px; color: #aaa; font-style: italic; }
.qf-add-btn {
  padding: 5px 16px; font-size: 13px; font-weight: 600; color: #fff;
  background: linear-gradient(135deg, #1677ff, #0958d9); border: none;
  border-radius: 6px; cursor: pointer; transition: all .15s; white-space: nowrap;
}
.qf-add-btn:hover:not(:disabled) { transform: translateY(-1px); }
.qf-add-btn:disabled { background: #d9d9d9; cursor: not-allowed; }
.qf-add-btn--buy_signals { background: linear-gradient(135deg, #1677ff, #0958d9); }
.qf-add-btn--buy_signals:hover:not(:disabled) { box-shadow: 0 2px 8px rgba(22,119,255,.3); }
.qf-add-btn--sell_signals { background: linear-gradient(135deg, #52c41a, #389e0d); }
.qf-add-btn--sell_signals:hover:not(:disabled) { box-shadow: 0 2px 8px rgba(82,196,26,.35); }
/* 多选 checkbox 组 */
.qf-multi-select {
  display: inline-flex; gap: 4px; flex-wrap: wrap;
  padding: 4px 6px; background: #fafafa; border-radius: 6px;
}
.qf-checkbox {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 4px 10px; border-radius: 14px; font-size: 12.5px;
  cursor: pointer; user-select: none; transition: .15s;
  border: 1px solid #d9d9d9; background: #fff; color: #666;
}
.qf-checkbox:hover { border-color: #1677ff; }
.qf-checkbox.checked {
  background: #e6f4ff; border-color: #1677ff; color: #1677ff; font-weight: 600;
}
.qf-checkbox input[type="checkbox"] { display: none; }
.add-success-msg-inline {
  display: block; text-align: center; font-size: 12.5px; color: #389e0d; font-weight: 600;
  margin-top: 6px;
}
.save-toast {
  position: fixed; top: 24px; left: 50%; transform: translateX(-50%);
  z-index: 9999; padding: 10px 24px;
  background: #fff; color: #333; font-size: 14px; font-weight: 600;
  border-radius: 8px; box-shadow: 0 6px 20px rgba(0,0,0,.12), 0 2px 8px rgba(0,0,0,.06);
  display: flex; align-items: center; gap: 8px;
  white-space: nowrap; user-select: none;
}
.toast-slide-enter-active { animation: toastIn .3s ease-out; }
.toast-slide-leave-active { animation: toastOut .25s ease-in; }
@keyframes toastIn {
  from { opacity: 0; transform: translateX(-50%) translateY(-12px); }
  to   { opacity: 1; transform: translateX(-50%) translateY(0); }
}
@keyframes toastOut {
  from { opacity: 1; transform: translateX(-50%) translateY(0); }
  to   { opacity: 0; transform: translateX(-50%) translateY(-12px); }
}

/* 空状态 */
.empty-signals { text-align: center; padding: 40px 20px; }
.empty-signals .empty-icon { font-size: 44px; display: block; margin-bottom: 8px; }
.empty-signals p { font-size: 13.5px; color: #bbb; margin: 4px 0; }
.empty-sub { font-size: 12px !important; color: #ccc !important; }

/* 信号标签 chips */
.signals-chips-area { margin-top: 12px; }
.chips-row { display: flex; flex-wrap: wrap; gap: 8px; }
.sig-chip {
  display: inline-flex; align-items: center; gap: 6px; padding: 6px 12px 6px 8px;
  border: 1px solid #e8e8e8; border-radius: 6px; font-size: 13px;
  background: #fff; transition: all .15s; cursor: default;
}
.sig-chip:hover { border-color: #ccc; box-shadow: 0 1px 4px rgba(0,0,0,.06); }
.chip-bar { width: 3px; height: 16px; border-radius: 2px; flex-shrink: 0; }
.chip-technical .chip-bar { background: #08979c; }
.chip-market .chip-bar { background: #0958d9; }
.chip-fundamental .chip-bar { background: #d46b08; }
.chip-financial .chip-bar { background: #52c41a; }
.chip-name { font-weight: 600; color: #1a1a2e; }
.chip-op {
  font-family: monospace; font-size: 11.5px; font-weight: 600;
  padding: 1px 6px; border-radius: 4px; color: #fff;
  background: linear-gradient(135deg, #1677ff, #0958d9);
}
.chip-params { color: #666; font-size: 11.5px; }
.chip-del {
  background: none; border: none; cursor: pointer; font-size: 13px; color: #ccc;
  padding: 0 2px; transition: .12s;
}
.chip-del:hover { color: #cf1322; }

/* 底部操作栏 */
.sec-footer {
  display: flex; justify-content: space-between; align-items: center;
  margin-top: 14px; padding-top: 12px; border-top: 1px solid #f0f0f0;
}
.logic-toggle { display: flex; align-items: center; gap: 8px; font-size: 12.5px; color: #555; }
.logic-label { font-weight: 500; }
.logic-btn { padding: 5px 14px; border: 1px solid #d9d9d9; border-radius: 6px; background: #fff; font-size: 12px; font-weight: 500; cursor: pointer; color: #666; transition: .15s; }
.logic-btn:first-child { border-right: none; }
.logic-btn.active { background: #1677ff; color: #fff; border-color: #1677ff; }
.footer-actions { display: flex; gap: 8px; }
.btn-sec-sm { padding: 5px 14px; border: 1px solid #d9d9d9; border-radius: 6px; background: #fff; font-size: 12px; cursor: pointer; color: #666; }
.btn-sec-sm:hover { border-color: #cf1322; color: #cf1322; }

/* ========== Section 3: 结果预览表 ========== */
.sec-results { background: #fff; border: 1px solid #e8e8e8; border-radius: 10px; overflow: hidden; flex: 1; display: flex; flex-direction: column; min-height: 360px; }
.sec-results--kline { min-height: 500px; } /* 多股同列需要更高 */

.results-head {
  display: flex; justify-content: space-between; align-items: flex-start;
  padding: 14px 20px; border-bottom: 1px solid #f0f0f0; flex-wrap: wrap; gap: 10px;
}
.results-left { display: flex; align-items: center; gap: 12px; }
.results-title { font-size: 15px; font-weight: 700; color: #1a1a2e; margin: 0; }
.results-title strong { color: #cf1322; } /* 中国红涨 */
.results-tabs { display: flex; gap: 2px; }
.rtab { padding: 4px 12px; border: 1px solid transparent; border-radius: 4px; background: transparent; font-size: 12px; cursor: pointer; color: #777; transition: .12s; }
.rtab.active { background: #e6f4ff; color: #1677ff; border-color: #bae0ff; font-weight: 600; }
.rtab.dim { opacity: 0.5; cursor: default; }

.results-right { display: flex; align-items: center; gap: 8px; }
.res-strategy-name { font-size: 13px; color: #555; max-width: 140px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.date-picker {
  padding: 5px 10px; border: 1px solid #d9d9d9; border-radius: 5px;
  font-size: 12.5px; color: #333; background: #fff; outline: none; cursor: pointer;
}
.date-picker:focus { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.08); }
.btn-res-action {
  padding: 5px 14px; border-radius: 5px; font-size: 12.5px; font-weight: 500;
  cursor: pointer; transition: .15s; white-space: nowrap;
}
.btn-res-action.primary { background: #1677ff; color: #fff; border: 1px solid #1677ff; }
.btn-res-action.primary:hover { background: #0958d9; }
.btn-res-action { background: #fff; color: #555; border: 1px solid #d9d9d9; }
.btn-res-action:hover { border-color: #1677ff; color: #1677ff; }
.btn-res-action.run { background: #52c41a; color: #fff; border: 1px solid #52c41a; }
.btn-res-action.run:hover { background: #389e0d; }
.btn-res-action.backtest { background: #fff; color: #1677ff; border: 1px solid #1677ff; font-weight: 600; }
.btn-res-action.backtest:hover { background: #f0f5ff; }

.results-toolbar {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 20px; border-bottom: 1px solid #f0f0f0; font-size: 12px; flex-wrap: wrap;
}
.tb-tool { padding: 4px 10px; border: 1px solid #d9d9d9; border-radius: 4px; background: #fff; cursor: pointer; color: #555; font-size: 12px; transition: .12s; }
.tb-tool:hover { border-color: #1677ff; color: #1677ff; }
.tb-select { padding: 4px 8px; border: 1px solid #d9d9d9; border-radius: 4px; font-size: 12px; color: #555; outline: none; background: #fff; cursor: pointer; }
.tb-sort-tabs { display: flex; gap: 2px; margin-left: 4px; }
.st { padding: 4px 10px; border: none; border-radius: 4px; background: transparent; font-size: 12px; cursor: pointer; color: #666; transition: .12s; }
.st.active { background: #e6f4ff; color: #1677ff; font-weight: 600; }
.st:hover:not(.active) { background: #f5f5f5; }
.tb-search { display: flex; align-items: center; margin-left: auto; position: relative; }
.tb-search-in { padding: 4px 30px 4px 8px; border: 1px solid #d9d9d9; border-radius: 4px; font-size: 12px; width: 120px; outline: none; }
.tb-search-in:focus { border-color: #1677ff; }
.tb-search-icon {
  position: absolute; right: 8px; pointer-events: none;
  font-size: 12.5px; color: #aaa; user-select: none;
}
.btn-batch-fav {
  flex-shrink: 0; padding: 4px 14px;
  border: 1px solid #d9d9d9; border-radius: 4px;
  background: #fff; color: #1677ff; font-size: 12.5px;
  cursor: pointer; transition: all .15s; white-space: nowrap;
}
.btn-batch-fav:hover:not(:disabled) { border-color: #1677ff; background: #f0f7ff; }
.btn-batch-fav:disabled { opacity: .45; cursor: not-allowed; }
.fav-toast-ok { background: #f6ffed; color: #389e0d; }
.fav-toast-err { background: #fff1f0; color: #cf1322; }
.tb-extra { font-size: 11.5px; color: #aaa; margin-left: 8px; }

.results-table-wrap { flex: 1; overflow-x: auto; }
.results-table { width: 100%; border-collapse: collapse; font-size: 12.5px; table-layout: auto; }
.results-table thead th {
  padding: 9px 12px; text-align: center; font-weight: 600; color: #555;
  background: #fafafa; border-bottom: 1.5px solid #eee; white-space: nowrap; font-size: 11.5px;
  position: sticky; top: 0; z-index: 2;
}

/* ====== 排序样式 ====== */
.results-table thead th.sortable {
  cursor: pointer;
  user-select: none;
  transition: background .15s, color .15s;
}
.results-table thead th.sortable:hover {
  background: #e6f4ff;
  color: #1677ff;
}
.results-table thead th.sortable.active {
  color: #1677ff;
  font-weight: 700;
  background: #f0f5ff;
}
.sort-icon {
  margin-left: 4px;
  font-size: 10px;
  opacity: 0.45;
}
.results-table thead th.sortable:hover .sort-icon,
.results-table thead th.sortable.active .sort-icon {
  opacity: 1;
}

.results-table tbody td {
  padding: 8px 12px; border-bottom: 1px solid #f5f5f5; white-space: nowrap; color: #444; text-align: center;
  height: 40px; overflow: hidden; text-overflow: ellipsis;
}
.results-table tbody tr:nth-child(even) { background: #fafbfc; }
.results-table tbody tr:hover { background: #f0f7ff; }
.col-cb { width: 38px; }
.col-idx { width: 50px; }
.col-code { width: 80px; }
.col-name { width: 100px; }

/* 数据列按列序号定位固定 */
.col-price { min-width: 90px; }
.col-pct { min-width: 90px; }
.col-num { min-width: 90px; }
.col-industry { min-width: 120px; }   /* 概况：所属东财行业 */
.col-sector { min-width: 150px; }
.col-links { min-width: 160px; }

/* 财务列宽度分配（根据数据长度合理分配） */
.col-fin-bvps { min-width: 100px; }  /* 每股净资产(元) */
.col-fin-eps { min-width: 110px; }   /* 基本每股收益(元) */
.col-fin-roe { min-width: 105px; }   /* 净资产收益率(%) */
.col-fin-roa { min-width: 110px; }   /* 总资产收益率(%) */
.col-fin-gm { min-width: 85px; }     /* 毛利率(%) */
.col-fin-nm { min-width: 80px; }     /* 净利率(%) */
.col-fin-dr { min-width: 100px; }    /* 资产负债率(%) */
.col-fin-ps { min-width: 95px; }     /* 市销率TTM */
.col-fin-pb { min-width: 80px; }     /* 市净率 */

/* 链接列不截断 */
.col-links { overflow: visible; }
.results-table td.col-links { text-overflow: clip; }

/* 财务模式无需额外处理：v-show 隐藏后 table-layout:auto 自动分配宽度 */

.col-cb { text-align: center; }
.col-cb input[type="checkbox"] { accent-color: #1677ff; width: 14px; height: 14px; cursor: pointer; }
.results-table thead th.col-idx, .results-table tbody td.col-idx { text-align: center; }
.ext-link {
  display: inline-block;
  padding: 2px 8px;
  margin-right: 4px;
  border-radius: 3px;
  font-size: 12px;
  text-decoration: none;
  background: #f0f5ff;
  color: #1677ff;
  border: 1px solid #d6e4ff;
  transition: all .15s;
}
.ext-link:hover {
  background: #1677ff;
  color: #fff;
  border-color: #1677ff;
}
.stock-name-hover {
  border-bottom: 1px dashed #1677ff;
  transition: color .12s;
}
.stock-name-hover:hover {
  color: #1677ff;
}
.up { color: #cf1322 !important; font-weight: 600; } /* 中国红涨 */
.down { color: #52c41a !important; font-weight: 600; } /* 中国绿跌 */

.match-tags { display: flex; flex-wrap: wrap; gap: 3px; }
.match-tag {
  font-size: 10.5px; padding: 1px 7px; border-radius: 8px;
  background: #f0f0f0; color: #666; font-weight: 500;
}

/* ====== 分页栏 ====== */
.pagination-bar {
  display: flex; align-items: center; justify-content: space-between;
  padding: 10px 20px; border-top: 1px solid #f0f0f0;
  font-size: 12.5px; color: #555;
}
.pag-left, .pag-right { display: flex; align-items: center; gap: 8px; }
.pag-center { display: flex; align-items: center; gap: 4px; }
.page-size-select {
  padding: 3px 6px; border: 1px solid #d9d9d9; border-radius: 4px;
  font-size: 12px; color: #555; outline: none; background: #fff; cursor: pointer;
}
.pag-info { color: #999; }
.pag-btn {
  min-width: 30px; height: 28px; padding: 0 8px; border: 1px solid transparent;
  border-radius: 4px; background: transparent; cursor: pointer;
  font-size: 12.5px; color: #555; transition: all .15s; display: inline-flex;
  align-items: center; justify-content: center;
}
.pag-btn:hover:not(:disabled):not(.active) { background: #f5f5f5; border-color: #d9d9d9; color: #1677ff; }
.pag-btn.active { background: #1677ff; color: #fff; border-color: #1677ff; font-weight: 600; }
.pag-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.pag-ellipsis { color: #bbb; padding: 0 4px; }
.pag-jump-input {
  width: 44px; height: 26px; padding: 0 4px; border: 1px solid #d9d9d9;
  border-radius: 4px; font-size: 12px; text-align: center; outline: none;
}
.pag-jump-input:focus { border-color: #1677ff; }

/* ====== 动画 ====== */
.expand-down-enter-active { transition: all .25s ease-out; }
.expand-down-leave-active { transition: all .2s ease-in; }
.expand-down-enter-from { opacity: 0; max-height: 0; margin-top: 0; padding-top: 0; padding-bottom: 0; }
.expand-down-leave-to { opacity: 0; max-height: 0; margin-top: 0; padding-top: 0; padding-bottom: 0; }

.fade-fast-enter-active { transition: opacity .2s ease; }
.fade-fast-leave-active { transition: opacity .15s ease; }
.fade-fast-enter-from, .fade-fast-leave-to { opacity: 0; }

.sig-chip-enter-active { transition: all .28s cubic-bezier(.23,1,.32,1); }
.sig-chip-leave-active { transition: all .18s ease-in; }
.sig-chip-enter-from { opacity: 0; transform: scale(.94) translateY(4px); }
.sig-chip-leave-to { opacity: 0; transform: scale(.94) translateY(-4px); }

/* ====== Modal 弹窗 ====== */
.modal-overlay { position: fixed; inset: 0; z-index: 999; background: rgba(0,0,0,.35); display: flex; align-items: center; justify-content: center; }
.modal-box { background: #fff; border-radius: 12px; padding: 24px; width: 360px; max-width: 90vw; box-shadow: 0 12px 40px rgba(0,0,0,.18); }
.modal-title { font-size: 17px; font-weight: 700; margin-bottom: 10px; }
.modal-body { font-size: 14px; color: #666; line-height: 1.6; margin-bottom: 18px; }
.modal-actions { display: flex; justify-content: flex-end; gap: 8px; }
.btn-modal-cancel { padding: 7px 18px; border: 1px solid #d9d9d9; border-radius: 6px; background: #fff; font-size: 13px; cursor: pointer; color: #666; }
.btn-modal-cancel:hover { border-color: #aaa; }
.btn-modal-danger { padding: 7px 18px; border: none; border-radius: 6px; background: #cf1322; color: #fff; font-size: 13px; font-weight: 600; cursor: pointer; }
.btn-modal-danger:hover { background: #a8071a; }

/* ===== 卖出规则 & 仓位管理 ===== */
.rules-body {
  margin-top: 0; display: flex; flex-direction: column; gap: 16px;
}
.rules-subsection {
  background: linear-gradient(to bottom, #f0f9ff, #fff);
  border: 1px solid #e8e8e8; border-radius: 8px; padding: 14px 16px;
}
.rules-subtitle { font-size: 14px; font-weight: 700; color: #333; margin: 0 0 10px; }
.rules-list { display: flex; flex-direction: column; gap: 8px; }
.rule-row { display: flex; align-items: center; gap: 8px; min-height: 34px; }
.rule-row .rule-params-wrap { display: flex; align-items: center; gap: 6px; }
.rule-check { display: flex; align-items: center; gap: 6px; cursor: pointer; min-width: 90px; flex-shrink: 0; }
.rule-check input[type="checkbox"] {
  width: 15px; height: 15px; accent-color: #1677ff; cursor: pointer;
}
.rule-label { font-size: 13px; font-weight: 500; color: #333; min-width: 56px; }
.rule-help {
  display: inline-flex; align-items: center; justify-content: center;
  width: 16px; height: 16px; border-radius: 50%;
  border: 1px solid #bbb; color: #999; font-size: 10px; font-weight: 700;
  cursor: help; flex-shrink: 0; line-height: 1;
  transition: all .15s; position: relative;
}
.rule-help:hover { border-color: #1677ff; color: #1677ff; background: #e6f4ff; }
.rule-help::after {
  content: attr(data-tooltip);
  position: absolute; top: calc(100% + 6px); left: 50%;
  transform: translateX(-50%);
  display: inline-block;
  background: #333; color: #fff; font-size: 12px; font-weight: 400;
  padding: 8px 12px; border-radius: 6px;
  white-space: normal;
  min-width: 200px; max-width: 320px;
  line-height: 1.6;
  word-break: normal; overflow-wrap: break-word;
  opacity: 0; visibility: hidden; pointer-events: none;
  transition: opacity .15s, visibility .15s;
  z-index: 999;
}
.rule-help:hover::after { opacity: 1; visibility: visible; }
.rule-input-sm {
  width: 60px; padding: 5px 8px; border: 1px solid #d9d9d9; border-radius: 6px;
  font-size: 13px; text-align: center; color: #333; background: #fff; outline: none;
  transition: border-color .15s;
}
.rule-input-sm:focus { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.06); }
.rule-input-xs {
  width: 44px; padding: 4px 4px; border: 1px solid #d9d9d9; border-radius: 6px;
  font-size: 12px; text-align: center; color: #333; background: #fff; outline: none;
  transition: border-color .15s;
}
.rule-input-xs:focus { border-color: #1677ff; }
.rule-input {
  width: 60px; padding: 5px 8px; border: 1px solid #d9d9d9; border-radius: 6px;
  font-size: 13px; text-align: center; color: #333; background: #fff; outline: none;
  transition: border-color .15s;
}
.rule-input:focus { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.06); }
.rule-param-label { font-size: 12.5px; color: #888; }
.rule-unit { font-size: 12.5px; color: #888; }
.rule-select {
  padding: 5px 10px; border: 1px solid #d9d9d9; border-radius: 6px;
  font-size: 13px; color: #333; background: #fff; outline: none; cursor: pointer;
  transition: border-color .15s;
}
.rule-select:focus { border-color: #1677ff; box-shadow: 0 0 0 2px rgba(22,119,255,.06); }
.rules-pos-grid { display: flex; flex-wrap: wrap; gap: 16px; }
.rules-pos-grid .rule-item {
  display: flex; align-items: center; gap: 6px; font-size: 13px; color: #555;
}
.segment-list { display: flex; flex-direction: column; gap: 4px; margin-left: 0; padding-left: 0; }
.segment-level {
  display: inline-flex; align-items: center; gap: 4px;
  font-size: 12px; color: #555;
  padding: 3px 0; border-radius: 6px;
  white-space: nowrap;
}
.seg-idx { color: #999; font-size: 11px; min-width: 18px; }
.seg-label { font-size: 12px; color: #888; }
.seg-unit { font-size: 12px; color: #888; }
.btn-level-del { padding: 0 4px; border: none; background: transparent; color: #cf1322; cursor: pointer; font-size: 12px; }
.btn-level-add { padding: 4px 12px; border: 1px dashed #bae0ff; border-radius: 6px; background: #f0f7ff; color: #1677ff; cursor: pointer; font-size: 12px; transition: .15s; white-space: nowrap; flex-shrink: 0; }
.btn-level-add:hover { border-color: #1677ff; background: #e6f4ff; }

/* ====== 多股同列空状态 ====== */
.multi-kline-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 400px;
}
.mk-empty-content { text-align: center; }
.mk-empty-icon { font-size: 48px; display: block; margin-bottom: 10px; }
.mk-empty-content p { font-size: 14px; color: #bbb; margin: 4px 0; }
.mk-empty-hint { font-size: 12.5px !important; color: #ccc !important; }

/* ====== K线模式内嵌周期切换 ====== */
.mk-period-tabs-inline {
  display: flex;
  gap: 2px;
  background: #f0f0f0;
  border-radius: 4px;
  overflow: hidden;
}
.mk-tab-sm {
  padding: 4px 14px;
  font-size: 12px;
  border: none;
  background: transparent;
  cursor: pointer;
  color: #666;
  transition: .15s;
}
.mk-tab-sm.active {
  background: #1677ff;
  color: #fff;
  font-weight: 600;
}
.mk-tab-sm:not(.active):hover { color: #333; background: #eee; }
/* K线模式内嵌列数选择 */
.mk-col-select-inline {
  padding: 4px 8px;
  border: 1px solid #d9d9d9;
  border-radius: 4px;
  font-size: 12px;
  outline: none;
  cursor: pointer;
  background: #fff;
  margin-right: 8px;
}
</style>
