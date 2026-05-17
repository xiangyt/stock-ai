<template>
  <div class="data-table-wrapper">
    <!-- 加载状态 -->
    <div v-if="loading" class="dt-loading">加载中...</div>

    <!-- 空状态 -->
    <div v-else-if="isEmpty" class="dt-empty">
      <slot name="empty">
        <div class="dt-empty-default">
          <div class="dt-empty-icon">📭</div>
          <p>暂无数据</p>
        </div>
      </slot>
    </div>

    <!-- 表格 -->
    <table v-else class="data-table" :class="{ 'dt-sticky-header': stickyHeader }" :style="{ tableLayout }">
      <thead>
        <slot name="header">
          <tr><th>-</th></tr>
        </slot>
      </thead>
      <tbody>
        <slot :items="data">
          <!-- 父组件渲染行 -->
        </slot>
      </tbody>
    </table>

    <!-- 底部（分页等） -->
    <slot name="footer" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  loading?: boolean
  data?: any[]
  rowKey?: string
  stickyHeader?: boolean
  tableLayout?: 'auto' | 'fixed'
}>(), {
  loading: false,
  data: () => [],
  rowKey: 'id',
  stickyHeader: false,
  tableLayout: 'auto',
})

const isEmpty = computed(() => !props.loading && props.data.length === 0)
</script>

<style scoped>
.data-table-wrapper {
  width: 100%;
}

/* ====== 表格基础 ====== */
.data-table {
  width: 100%;
  border-collapse: collapse;
  background: var(--dt-bg, #fff);
  border-radius: var(--dt-radius, 12px);
  overflow: hidden;
  box-shadow: var(--dt-shadow, 0 1px 4px rgba(0,0,0,.06));
}

.data-table thead th {
  background: var(--dt-th-bg, #fafafa);
  padding: var(--dt-th-padding, 10px 12px);
  text-align: left;
  font-size: var(--dt-th-fs, 13px);
  font-weight: var(--dt-th-fw, 600);
  color: var(--dt-th-color, #666);
  border-bottom: 1px solid var(--dt-border, #eee);
  white-space: nowrap;
  vertical-align: middle;
}

.data-table tbody td {
  padding: var(--dt-td-padding, 10px 12px);
  font-size: var(--dt-td-fs, 13.5px);
  color: var(--dt-td-color, #333);
  border-bottom: 1px solid var(--dt-td-border, #f3f3f3);
  vertical-align: middle;
}

.data-table tbody tr:hover td {
  background: var(--dt-hover-bg, #f9fbff);
}

/* 行状态 */
.data-table tbody tr.dt-disabled-row td {
  opacity: var(--dt-disabled-opacity, .5);
}
.data-table tbody tr.dt-selected td,
.data-table tbody tr.selected td {
  background: var(--dt-selected-bg, #e6f4ff);
}

/* 展开行 */
.data-table tbody tr.dt-expand-row td {
  background: #fafbfc;
  padding: 0;
  border-bottom: 1px solid var(--dt-td-border, #f3f3f3);
}
.data-table tbody tr.dt-expand-row:hover td {
  background: #fafbfc;
}

/* Sticky 表头 */
.dt-sticky-header thead th {
  position: sticky;
  top: 0;
  z-index: 2;
}

/* ====== 加载状态 ====== */
.dt-loading {
  text-align: center;
  color: #999;
  padding: 60px 0;
  font-size: 14px;
}

/* ====== 空状态 ====== */
.dt-empty {
  text-align: center;
  padding: 60px 0;
}
.dt-empty-default {
  color: #bbb;
  font-size: 14px;
}
.dt-empty-default .dt-empty-icon {
  font-size: 32px;
  margin-bottom: 8px;
}
</style>
