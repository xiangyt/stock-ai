<template>
  <section class="form-section software-config-section">
    <h3>软件配置</h3>
    <p class="section-desc">管理东方财富、同花顺等数据源的 Cookie 与启用状态</p>

    <div v-if="loading" class="config-loading">加载中...</div>

    <div v-else class="software-list">
      <div
        v-for="item in mergedItems"
        :key="item.software_name"
        class="software-item"
        :class="{ disabled: !item.enabled }"
      >
        <div class="software-header">
          <div class="software-info">
            <span class="software-name">{{ item.display_name || item.software_name }}</span>
            <span class="software-desc">{{ item.description }}</span>
          </div>
          <label class="switch">
            <input
              type="checkbox"
              :checked="item.enabled"
              :disabled="savingMap[item.software_name]"
              @change="toggleEnabled(item)"
            />
            <span class="slider"></span>
          </label>
        </div>

        <div class="form-group cookie-group">
          <label>Cookie</label>
          <textarea
            v-model="item.cookie"
            rows="3"
            placeholder="粘贴对应软件的 Cookie 字符串（可选）"
            :disabled="!item.enabled || savingMap[item.software_name]"
          />
        </div>

        <div class="form-group extra-group" v-if="item.software_name === 'eastmoney' || item.software_name === 'ths2'">
          <label>扩展配置 (JSON)</label>
          <textarea
            v-model="item.extraText"
            rows="2"
            placeholder='{"key":"value"}'
            :disabled="!item.enabled || savingMap[item.software_name]"
          />
        </div>

        <div class="software-footer">
          <span class="updated-at" v-if="item.updated_at">更新于 {{ item.updated_at }}</span>
          <button
            class="btn-save-small"
            :disabled="savingMap[item.software_name]"
            @click="saveItem(item)"
          >
            {{ savingMap[item.software_name] ? '保存中...' : '保存' }}
          </button>
        </div>
      </div>
    </div>

    <p v-if="errorMsg" class="error-msg">{{ errorMsg }}</p>
    <p v-if="successMsg" class="success-msg">{{ successMsg }}</p>
  </section>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import * as api from '../api/software-config'
import type { SoftwareConfigItem, SoftwareMeta } from '../api/software-config'

interface EditableItem extends SoftwareConfigItem {
  description: string
  extraText: string
}

const supported = ref<SoftwareMeta[]>([])
const configs = ref<SoftwareConfigItem[]>([])
const loading = ref(false)
const savingMap = ref<Record<string, boolean>>({})
const errorMsg = ref('')
const successMsg = ref('')

const mergedItems = computed<EditableItem[]>(() => {
  const configMap = new Map(configs.value.map((c) => [c.software_name, c]))
  return supported.value.map((meta) => {
    const cfg = configMap.get(meta.name)
    return {
      software_name: meta.name,
      display_name: meta.display_name,
      description: meta.description,
      cookie: cfg?.cookie ?? '',
      extra: cfg?.extra ?? '',
      extraText: cfg?.extra ?? '',
      enabled: cfg?.enabled ?? true,
      updated_at: cfg?.updated_at ?? '',
    }
  })
})

onMounted(async () => {
  loading.value = true
  try {
    const [metaList, cfgList] = await Promise.all([
      api.listSupportedSoftware(),
      api.listSoftwareConfigs(),
    ])
    supported.value = metaList
    configs.value = cfgList
  } catch (e: any) {
    errorMsg.value = e.message || '加载配置失败'
  } finally {
    loading.value = false
  }
})

function clearMessages() {
  errorMsg.value = ''
  successMsg.value = ''
}

async function saveItem(item: EditableItem) {
  clearMessages()
  savingMap.value[item.software_name] = true

  try {
    let extra: Record<string, string> | undefined
    if (item.extraText.trim()) {
      try {
        extra = JSON.parse(item.extraText)
      } catch {
        throw new Error(`${item.display_name} 的扩展配置不是有效 JSON`)
      }
    }

    const updated = await api.updateSoftwareConfig(item.software_name, {
      cookie: item.cookie,
      enabled: item.enabled,
      extra,
    })

    // 刷新本地缓存
    const idx = configs.value.findIndex((c) => c.software_name === item.software_name)
    if (idx >= 0) {
      configs.value[idx] = updated
    } else {
      configs.value.push(updated)
    }

    successMsg.value = `${item.display_name} 保存成功`
  } catch (e: any) {
    errorMsg.value = e.message || '保存失败'
  } finally {
    savingMap.value[item.software_name] = false
  }
}

async function toggleEnabled(item: EditableItem) {
  item.enabled = !item.enabled
  await saveItem(item)
}
</script>

<style scoped>
.software-config-section {
  margin: 0;
}

.software-config-section h3 {
  font-size: 16px; font-weight: 700; color: #333; margin-bottom: 6px;
}

.software-config-section .section-desc {
  font-size: 12.5px; color: #aaa; margin-bottom: 16px;
}

.config-loading {
  padding: 20px 0;
  color: #888;
  font-size: 13px;
}

.software-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.software-item {
  border: 1px solid #e8e8e8;
  border-radius: 10px;
  padding: 16px;
  background: #fafafa;
  transition: opacity 0.2s;
}

.software-item.disabled {
  opacity: 0.65;
}

.software-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 12px;
}

.software-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.software-name {
  font-size: 14px;
  font-weight: 700;
  color: #333;
}

.software-desc {
  font-size: 12px;
  color: #888;
}

.cookie-group textarea,
.extra-group textarea {
  width: 100%;
  padding: 9px 12px;
  border: 1.5px solid #d9d9d9;
  border-radius: 8px;
  font-size: 13px;
  outline: none;
  resize: vertical;
  box-sizing: border-box;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
}

.cookie-group textarea:focus,
.extra-group textarea:focus {
  border-color: #1677ff;
  box-shadow: 0 0 0 3px rgba(22, 119, 255, 0.08);
}

.software-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 12px;
}

.updated-at {
  font-size: 11.5px;
  color: #aaa;
}

.btn-save-small {
  padding: 6px 16px;
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  background: #1677ff;
  border: none;
  border-radius: 6px;
  cursor: pointer;
}

.btn-save-small:hover:not(:disabled) {
  background: #0958d9;
}

.btn-save-small:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

/* Switch 开关 */
.switch {
  position: relative;
  display: inline-block;
  width: 44px;
  height: 24px;
  flex-shrink: 0;
}

.switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: #ccc;
  border-radius: 24px;
  transition: 0.2s;
}

.slider:before {
  position: absolute;
  content: '';
  height: 18px;
  width: 18px;
  left: 3px;
  bottom: 3px;
  background-color: white;
  border-radius: 50%;
  transition: 0.2s;
}

.switch input:checked + .slider {
  background-color: #1677ff;
}

.switch input:checked + .slider:before {
  transform: translateX(20px);
}

.switch input:disabled + .slider {
  opacity: 0.55;
  cursor: not-allowed;
}

.error-msg {
  color: #cf1322;
  font-size: 13px;
  text-align: center;
  min-height: 20px;
  margin-top: 10px;
}

.success-msg {
  color: #16a34a;
  font-size: 13px;
  text-align: center;
  min-height: 20px;
  margin-top: 10px;
}
</style>
