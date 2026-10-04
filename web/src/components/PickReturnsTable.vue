<template>
  <div class="pick-table-wrap">
    <div v-if="picks.length === 0" class="pt-empty">
      <div class="pt-empty-icon">📭</div>
      <p>暂无命中记录</p>
    </div>

    <div v-else class="pt-scroll">
      <table class="pt-table">
        <thead>
          <tr>
            <th class="pt-col-check"></th>
            <th>代码</th>
            <th>名称</th>
            <th>信号日</th>
            <th class="pt-num">基准价</th>
            <th
              v-for="label in labels"
              :key="label"
              class="pt-num"
              title="选出日后第 n 个交易日的收益率（基准 = 选出日收盘价），单元格下方为实际日期"
            >{{ label }}</th>
            <th class="pt-num">区间最高</th>
            <th class="pt-num">区间最低</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="pick in picks"
            :key="pickKey(pick)"
            :class="{ 'pt-selected': selected.includes(pickKey(pick)) }"
            @click="emit('toggle', pickKey(pick))"
          >
            <td class="pt-col-check">
              <input
                type="checkbox"
                :checked="selected.includes(pickKey(pick))"
                @click.stop
                @change="emit('toggle', pickKey(pick))"
              />
            </td>
            <td class="pt-code pt-link-hover" @mouseenter="showLinks($event, pick)" @mouseleave="hideLinks">
              <span class="stock-name-hover">{{ pick.code }}</span>
            </td>
            <td
              class="pt-name"
              :title="pick.message || pick.name"
              @mouseenter="showKLine($event, pick)"
              @mouseleave="hideKLine"
            >
              <span class="stock-name-hover">{{ pick.name }}</span>
            </td>
            <td class="pt-date">{{ pick.date }}</td>
            <td class="pt-num">{{ pick.base_close.toFixed(2) }}</td>
            <td
              v-for="(label, i) in labels"
              :key="label"
              class="pt-num"
              :style="{ color: cellColor(cellValue(pick, i)) }"
            >
              <div class="pt-cell">
                <span>{{ fmtCell(cellValue(pick, i)) }}</span>
                <i v-if="cellDate(pick, i)" class="pt-cell-date">{{ cellDate(pick, i) }}</i>
              </div>
            </td>
            <td class="pt-num" :style="{ color: cellColor(pick.max_pct) }">{{ fmtCell(pick.max_pct) }}</td>
            <td class="pt-num" :style="{ color: cellColor(pick.min_pct) }">{{ fmtCell(pick.min_pct) }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- K 线图悬浮弹窗（悬浮股票名称时展示） -->
    <KLineTooltip
      :visible="klineVisible"
      :stock-code="klineStockCode"
      :stock-name="klineStockName"
      :x="klineX"
      :y="klineY"
      @mouseenter="onKLineEnter"
      @mouseleave="hideKLine"
    />

    <!-- 股票代码悬浮：外部行情站点跳转 -->
    <StockLinkTooltip
      :visible="linkVisible"
      :stock-code="linkStockCode"
      :stock-name="linkStockName"
      :x="linkX"
      :y="linkY"
      @mouseenter="onLinkEnter"
      @mouseleave="hideLinks"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onBeforeUnmount } from 'vue'
import KLineTooltip from './KLineTooltip.vue'
import StockLinkTooltip from './StockLinkTooltip.vue'
import { pickDimValue, pickKey, type PriceDim, type StockPick } from '../api/picks'

// ========== Props & Emits ==========

const props = defineProps<{
  picks: StockPick[]
  labels: string[]        // T+1 ~ T+N 表头
  dim: PriceDim           // 当前价格维度
  selected: string[]      // 已勾选的选股记录 key
}>()

const emit = defineEmits<{ toggle: [key: string] }>()

// ========== 方法 ==========

/** 取第 i 个交易日（T+i+1）在所选维度下的收益率(%) */
function cellValue(pick: StockPick, i: number): number | null {
  const day = pick.returns[i]
  return day ? pickDimValue(day, props.dim) : null
}

/** 第 i 个交易日的实际日期（MM-DD），无数据返回空 */
function cellDate(pick: StockPick, i: number): string {
  const day = pick.returns[i]
  return day?.date.slice(5) ?? ''
}

function fmtCell(value: number | null): string {
  if (value === null || !Number.isFinite(value)) return '-'
  return `${value >= 0 ? '+' : ''}${value.toFixed(2)}%`
}

function cellColor(value: number | null): string {
  if (value === null || !Number.isFinite(value)) return '#bbb'
  return value >= 0 ? '#ef232a' : '#00943e'
}

// ========== K 线悬浮弹窗 ==========

const klineVisible = ref(false)
const klineStockCode = ref('')
const klineStockName = ref('')
const klineX = ref(0)
const klineY = ref(0)
let klineTimer: ReturnType<typeof setTimeout> | null = null
let klineHideTimer: ReturnType<typeof setTimeout> | null = null

function showKLine(e: MouseEvent, pick: StockPick) {
  // 取消待执行的隐藏（鼠标从弹窗移回名称时）
  if (klineHideTimer) { clearTimeout(klineHideTimer); klineHideTimer = null }
  if (klineTimer) clearTimeout(klineTimer)
  klineTimer = setTimeout(() => {
    klineStockCode.value = pick.code
    klineStockName.value = pick.name
    klineX.value = e.clientX
    klineY.value = e.clientY
    klineVisible.value = true
  }, 350) // 延迟显示，避免快速划过时闪烁
}

function hideKLine() {
  if (klineTimer) { clearTimeout(klineTimer); klineTimer = null }
  // 延迟 200ms 隐藏，给用户时间从名称移动到弹窗
  if (!klineHideTimer) {
    klineHideTimer = setTimeout(() => {
      klineVisible.value = false
      klineHideTimer = null
    }, 200)
  }
}

// ========== 外部行情链接悬浮弹窗（悬浮股票代码） ==========

const linkVisible = ref(false)
const linkStockCode = ref('')
const linkStockName = ref('')
const linkX = ref(0)
const linkY = ref(0)
let linkTimer: ReturnType<typeof setTimeout> | null = null
let linkHideTimer: ReturnType<typeof setTimeout> | null = null

function showLinks(e: MouseEvent, pick: StockPick) {
  if (linkHideTimer) { clearTimeout(linkHideTimer); linkHideTimer = null }
  if (linkTimer) clearTimeout(linkTimer)
  linkTimer = setTimeout(() => {
    linkStockCode.value = pick.code
    linkStockName.value = pick.name
    linkX.value = e.clientX
    linkY.value = e.clientY
    linkVisible.value = true
  }, 180)
}

function hideLinks() {
  if (linkTimer) { clearTimeout(linkTimer); linkTimer = null }
  if (!linkHideTimer) {
    linkHideTimer = setTimeout(() => {
      linkVisible.value = false
      linkHideTimer = null
    }, 200)
  }
}

/** 弹窗 mouseenter 时取消隐藏 */
function onLinkEnter() {
  if (linkHideTimer) { clearTimeout(linkHideTimer); linkHideTimer = null }
}

/** 弹窗 mouseenter 时取消隐藏 */
function onKLineEnter() {
  if (klineHideTimer) { clearTimeout(klineHideTimer); klineHideTimer = null }
}

onBeforeUnmount(() => {
  if (klineTimer) clearTimeout(klineTimer)
  if (klineHideTimer) clearTimeout(klineHideTimer)
  if (linkTimer) clearTimeout(linkTimer)
  if (linkHideTimer) clearTimeout(linkHideTimer)
})
</script>

<style scoped>
.pick-table-wrap {
  width: 100%;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 1px 4px rgba(0,0,0,.06);
  overflow: hidden;
}
.pt-scroll {
  flex: 1;
  min-height: 0;
  max-height: none;
  overflow: auto;
}
.pt-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13.5px;
}
.pt-table thead th {
  position: sticky;
  top: 0;
  z-index: 2;
  background: #fafafa;
  padding: 10px 12px;
  text-align: left;
  font-size: 12.5px;
  font-weight: 600;
  color: #666;
  border-bottom: 1px solid #eee;
  white-space: nowrap;
}
.pt-table tbody td {
  padding: 9px 12px;
  color: #333;
  border-bottom: 1px solid #f3f3f3;
  white-space: nowrap;
}
.pt-table tbody tr {
  cursor: pointer;
}
.pt-table tbody tr:hover td {
  background: #f9fbff;
}
.pt-table tbody tr.pt-selected td {
  background: #e6f4ff;
}
.pt-num {
  text-align: right;
}
/* 覆盖 thead th 的 text-align:left，让数字列表头与右对齐的数据对齐 */
.pt-table thead th.pt-num {
  text-align: right;
}
.pt-cell {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  line-height: 1.25;
}
.pt-cell-date {
  font-style: normal;
  font-size: 10.5px;
  color: #c0c4cc;
  font-variant-numeric: tabular-nums;
}
.pt-col-check {
  width: 32px;
  text-align: center;
}
.pt-code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  color: #555;
}
.pt-link-hover {
  cursor: pointer;
}
.pt-name {
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
}
.stock-name-hover {
  border-bottom: 1px dashed #1677ff;
  transition: color .12s;
}
.stock-name-hover:hover {
  color: #1677ff;
}
.pt-date {
  color: #888;
}
.pt-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 300px;
  color: #bbb;
  font-size: 14px;
}
.pt-empty-icon {
  font-size: 32px;
  margin-bottom: 8px;
}
</style>
