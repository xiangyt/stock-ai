<template>
  <div class="single-select" :class="{ 'is-open': open, 'is-disabled': disabled }" ref="rootEl">
    <div class="single-select-input" @click="open = true">
      <span v-if="selectedLabel" class="ss-selected">{{ selectedLabel }}</span>
      <input
        v-if="searchable"
        ref="inputEl"
        v-model="searchText"
        type="text"
        class="ss-input"
        :placeholder="selectedLabel ? '' : placeholder"
        @focus="open = true"
        @keydown.tab="open = false"
      />
      <span v-else-if="!selectedLabel" class="ss-placeholder">{{ placeholder }}</span>
    </div>
    <div v-if="open" class="ss-dropdown">
      <div
        v-for="opt in filteredOptions"
        :key="opt.value"
        class="ss-option"
        :class="{ selected: modelValue === opt.value }"
        @click="select(opt.value)"
      >
        <slot name="option" :option="opt" :selected="modelValue === opt.value">
          {{ opt.label }}
        </slot>
      </div>
      <p v-if="filteredOptions.length === 0" class="ss-empty">{{ emptyText }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'

export interface SelectOption {
  value: any
  label: string
  [key: string]: any
}

const props = withDefaults(defineProps<{
  modelValue: any
  options: SelectOption[]
  placeholder?: string
  emptyText?: string
  searchable?: boolean
  disabled?: boolean
}>(), {
  placeholder: '请选择',
  emptyText: '无匹配结果',
  searchable: false,
  disabled: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: any]
}>()

const rootEl = ref<HTMLElement | null>(null)
const open = ref(false)
const searchText = ref('')

const selectedLabel = computed(() => {
  const opt = props.options.find(o => o.value === props.modelValue)
  return opt?.label ?? ''
})

const filteredOptions = computed(() => {
  if (!props.searchable || !searchText.value) return props.options
  const kw = searchText.value.toLowerCase()
  return props.options.filter(o => o.label.toLowerCase().includes(kw))
})

function select(val: any) {
  emit('update:modelValue', val)
  searchText.value = ''
  open.value = false
}

function onClickOutside(e: MouseEvent) {
  if (rootEl.value && !rootEl.value.contains(e.target as Node)) {
    open.value = false
    searchText.value = ''
  }
}

watch(() => props.modelValue, () => {
  searchText.value = ''
})

onMounted(() => {
  document.addEventListener('click', onClickOutside)
})
onUnmounted(() => {
  document.removeEventListener('click', onClickOutside)
})
</script>

<style scoped>
.single-select {
  position: relative;
  border: 1.5px solid #d9d9d9;
  border-radius: 8px;
  background: #fff;
  transition: border-color .15s;
  cursor: pointer;
}
.single-select:focus-within,
.single-select.is-open {
  border-color: #1677ff;
  box-shadow: 0 0 0 3px rgba(22,119,255,.08);
  border-radius: 8px 8px 0 0;
}
.single-select.is-disabled {
  opacity: .5;
  cursor: not-allowed;
}

.single-select-input {
  display: flex; align-items: center;
  min-height: 40px; padding: 0 12px; gap: 6px;
  position: relative;
}
.single-select-input::after {
  content: '\25BC';
  font-size: 10px;
  color: #999;
  margin-left: auto;
  transition: transform .2s;
  flex-shrink: 0;
}
.single-select.is-open .single-select-input::after {
  transform: rotate(180deg);
}

.ss-selected {
  font-size: 14px; color: #333;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  flex-shrink: 0;
  max-width: 55%;
}
.single-select.is-open .ss-selected {
  display: none;
}
.ss-placeholder { color: #bbb; font-size: 14px; flex: 1; }

.ss-input {
  flex: 1; border: none !important; outline: none;
  font-size: 14px; padding: 8px 0 !important; background: transparent;
  box-shadow: none !important;
  cursor: text;
  margin: 0;
}
.ss-input::placeholder { color: #bbb; }

.ss-dropdown {
  position: absolute; top: 100%; left: -1.5px; right: -1.5px;
  background: #fff; border: 1.5px solid #1677ff; border-top: none;
  border-radius: 0 0 8px 8px; max-height: 200px; overflow-y: auto;
  box-shadow: 0 4px 12px rgba(0,0,0,.08);
  z-index: 10;
}

.ss-option {
  padding: 9px 14px; cursor: pointer; font-size: 14px;
  color: #333; transition: background .1s;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.ss-option:hover { background: #f5f8ff; }
.ss-option.selected { background: #f0f7ff; color: #1677ff; font-weight: 600; }

.ss-empty { text-align: center; color: #bbb; padding: 16px 0; font-size: 13px; }
</style>
