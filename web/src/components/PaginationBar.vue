<template>
  <div class="pag-bar" :class="{ 'pag-bar-embedded': embedded }">
    <!-- 左侧：信息 + 每页条数 -->
    <div class="pag-left">
      <span class="pag-info">共 {{ total }} 条</span>
      <select v-if="showPageSize" :value="pageSize" @change="$emit('update:pageSize', Number(($event.target as HTMLSelectElement).value))" class="pag-size-select">
        <option v-for="s in pageSizeOptions" :key="s" :value="s">{{ s }} 条/页</option>
      </select>
    </div>

    <!-- 中间：页码按钮 -->
    <div class="pag-center">
      <button class="pag-btn" :disabled="page <= 1" @click="$emit('update:page', page - 1)">‹ 上一页</button>
      <template v-if="showNumbers">
        <template v-for="p in pageNumbers" :key="p">
          <span v-if="p === '...'" class="pag-ellipsis">...</span>
          <button v-else :class="['pag-btn', 'pag-num', { active: p === page }]" @click="$emit('update:page', p as number)">{{ p }}</button>
        </template>
      </template>
      <button class="pag-btn" :disabled="page >= totalPages" @click="$emit('update:page', page + 1)">下一页 ›</button>
    </div>

    <!-- 右侧：跳页 -->
    <div v-if="showJump" class="pag-right">
      <span>跳至</span>
      <input v-model.number="jumpInput" type="number" class="pag-jump-input" min="1" :max="totalPages" @keyup.enter="doJump" />
      <span>页</span>
      <button class="pag-btn pag-go" @click="doJump">GO</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'

const props = withDefaults(defineProps<{
  page: number
  pageSize: number
  total: number
  showPageSize?: boolean
  showJump?: boolean
  showNumbers?: boolean
  embedded?: boolean        // 嵌入表格底部（带 border-top）
  pageSizeOptions?: number[]
}>(), {
  showPageSize: false,
  showJump: false,
  showNumbers: false,
  embedded: false,
  pageSizeOptions: () => [10, 20, 50, 100],
})

defineEmits<{
  'update:page': [page: number]
  'update:pageSize': [size: number]
}>()

const jumpInput = ref(props.page)
watch(() => props.page, (v) => { jumpInput.value = v })

const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))

const pageNumbers = computed(() => {
  const total = totalPages.value
  const cur = props.page
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)
  const pages: (number | string)[] = [1]
  if (cur > 3) pages.push('...')
  for (let i = Math.max(2, cur - 1); i <= Math.min(total - 1, cur + 1); i++) {
    pages.push(i)
  }
  if (cur < total - 2) pages.push('...')
  pages.push(total)
  return pages
})

function doJump() {
  let p = Math.round(jumpInput.value) || 1
  p = Math.min(p, totalPages.value)
  p = Math.max(p, 1)
  jumpInput.value = p
}
</script>

<style scoped>
.pag-bar {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 16px;
  padding: 0 4px;
  flex-wrap: wrap;
}
.pag-bar-embedded {
  margin-top: 0;
  padding: 10px 16px;
  border-top: 1px solid #f0f0f0;
  justify-content: flex-end;
}

.pag-left {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-right: auto;
}
.pag-info { font-size: 13px; color: #999; }
.pag-size-select {
  font-size: 12px;
  padding: 3px 6px;
  border: 1px solid #d9d9d9;
  border-radius: 4px;
  background: #fff;
  color: #555;
  cursor: pointer;
  outline: none;
}

.pag-center {
  display: flex;
  align-items: center;
  gap: 4px;
}
.pag-right {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #666;
}

.pag-btn {
  padding: 4px 12px;
  font-size: 13px;
  border: 1px solid #d9d9d9;
  border-radius: 6px;
  background: #fff;
  cursor: pointer;
  color: #333;
  transition: all .15s;
}
.pag-btn:hover:not(:disabled) { border-color: #1677ff; color: #1677ff; }
.pag-btn:disabled { opacity: .4; cursor: not-allowed; }

.pag-num {
  min-width: 32px;
  text-align: center;
}
.pag-num.active {
  background: #1677ff;
  color: #fff;
  border-color: #1677ff;
}
.pag-ellipsis {
  padding: 0 4px;
  color: #bbb;
  font-size: 13px;
}

.pag-jump-input {
  width: 48px;
  padding: 3px 6px;
  font-size: 13px;
  border: 1px solid #d9d9d9;
  border-radius: 4px;
  text-align: center;
  outline: none;
}
.pag-jump-input:focus { border-color: #1677ff; }

.pag-go {
  padding: 3px 10px;
  font-size: 12px;
  font-weight: 600;
}
</style>
