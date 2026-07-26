<template>
  <div class="multi-select" :class="{ 'is-open': open, 'is-disabled': disabled }" ref="rootEl">
    <div class="multi-select-input" @click="openInput">
      <span v-for="val in modelValue" :key="val" class="ms-tag">
        {{ labelMap[val] ?? val }}
        <span class="ms-tag-close" @click.stop="remove(val)">&times;</span>
      </span>
      <input
        ref="inputEl"
        v-model="searchText"
        type="text"
        class="ms-input"
        :placeholder="modelValue.length === 0 ? placeholder : ''"
        @focus="open = true"
        @keydown.tab="open = false"
      />
    </div>
    <div v-if="open" class="ms-dropdown">
      <div
        v-for="opt in filteredOptions"
        :key="opt.value"
        class="ms-option"
        :class="{ selected: modelValue.includes(opt.value) }"
        @click.stop="toggle(opt.value)"
      >
        <input
          type="checkbox"
          :checked="modelValue.includes(opt.value)"
          tabindex="-1"
          style="pointer-events: none;"
        />
        <slot name="option" :option="opt" :selected="modelValue.includes(opt.value)">
          <span class="ms-option-label">{{ opt.label }}</span>
        </slot>
      </div>
      <p v-if="filteredOptions.length === 0" class="ms-empty">{{ emptyText }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'

export interface MultiSelectOption {
  value: any
  label: string
  [key: string]: any
}

const props = withDefaults(defineProps<{
  modelValue: any[]
  options: MultiSelectOption[]
  placeholder?: string
  emptyText?: string
  maxCount?: number
  disabled?: boolean
}>(), {
  placeholder: '搜索...',
  emptyText: '无匹配结果',
  maxCount: Infinity,
  disabled: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: any[]]
}>()

const rootEl = ref<HTMLElement | null>(null)
const open = ref(false)
const searchText = ref('')

const labelMap = computed(() => {
  const m: Record<any, string> = {}
  props.options.forEach(o => { m[o.value] = o.label })
  return m
})

const filteredOptions = computed(() => {
  if (!searchText.value) return props.options
  const kw = searchText.value.toLowerCase()
  return props.options.filter(o => o.label.toLowerCase().includes(kw))
})

function openInput() {
  if (!props.disabled) open.value = true
}

function toggle(val: any) {
  const idx = props.modelValue.indexOf(val)
  if (idx >= 0) {
    emit('update:modelValue', props.modelValue.filter(v => v !== val))
  } else if (props.modelValue.length < props.maxCount) {
    emit('update:modelValue', [...props.modelValue, val])
  }
}

function remove(val: any) {
  emit('update:modelValue', props.modelValue.filter(v => v !== val))
}

function onClickOutside(e: MouseEvent) {
  if (rootEl.value && !rootEl.value.contains(e.target as Node)) {
    open.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', onClickOutside)
})
onUnmounted(() => {
  document.removeEventListener('click', onClickOutside)
})
</script>

<style scoped>
.multi-select {
  position: relative;
  border: 1.5px solid #d9d9d9;
  border-radius: 8px;
  background: #fff;
  transition: border-color .15s;
  cursor: pointer;
}
.multi-select:focus-within,
.multi-select.is-open {
  border-color: #1677ff;
  box-shadow: 0 0 0 3px rgba(22,119,255,.08);
  border-radius: 8px 8px 0 0;
}
.multi-select.is-disabled {
  opacity: .5;
  cursor: not-allowed;
}

.multi-select-input {
  display: flex; flex-wrap: wrap; align-items: center;
  min-height: 40px; padding: 4px 8px; gap: 4px;
}

.ms-tag {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 2px 6px 2px 8px; background: #f0f7ff; color: #1677ff;
  border-radius: 4px; font-size: 12px; font-weight: 500;
}
.ms-tag-close {
  cursor: pointer; font-size: 14px; line-height: 1; color: #999;
  width: 16px; height: 16px; display: flex; align-items: center; justify-content: center;
  border-radius: 50%;
}
.ms-tag-close:hover { background: #d6e8ff; color: #1677ff; }

.ms-input {
  flex: 1; min-width: 80px; border: none !important; outline: none;
  font-size: 13px; padding: 4px 0 !important; background: transparent;
  box-shadow: none !important;
  margin: 0;
}
.ms-input::placeholder { color: #bbb; }

.ms-dropdown {
  position: absolute; top: 100%; left: -1.5px; right: -1.5px;
  background: #fff; border: 1.5px solid #1677ff; border-top: none;
  border-radius: 0 0 8px 8px; max-height: 200px; overflow-y: auto;
  box-shadow: 0 4px 12px rgba(0,0,0,.08);
  z-index: 10;
}

.ms-option {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 12px; cursor: pointer; font-size: 13px;
  transition: background .1s;
}
.ms-option:hover { background: #f5f8ff; }
.ms-option.selected { background: #f0f7ff; }

.ms-option input[type="checkbox"] {
  width: 14px; height: 14px; accent-color: #1677ff; flex-shrink: 0;
  pointer-events: none;
}

.ms-option-label { flex: 1; color: #333; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ms-empty { text-align: center; color: #bbb; padding: 16px 0; font-size: 13px; }
</style>
