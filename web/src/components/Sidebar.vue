<template>
  <aside class="sidebar">
    <div class="sidebar-logo">
      <span class="logo-icon">🔶</span>
      <span class="logo-text">AI选股</span>
    </div>
    <nav class="sidebar-nav">
      <button
        v-for="item in menuItems"
        :key="item.key"
        :class="['nav-item', { active: current === item.key }]"
        @click="$emit('change', item.key)"
      >
        <span class="nav-icon">{{ item.icon }}</span>
        <span class="nav-label">{{ item.label }}</span>
      </button>
    </nav>
    <div class="sidebar-footer">
      <span class="version">v0.1.0</span>
    </div>
  </aside>
</template>

<script setup lang="ts">
defineProps<{ current: string }>()
defineEmits<{ change: [key: string] }>()

const menuItems = [
  { key: 'ai-picker', label: 'AI 选股', icon: '🤖' },
  { key: 'my-strategy', label: '我的策略', icon: '📋' },
  { key: 'my-positions', label: '我的持仓', icon: '💼' },
]
</script>

<style scoped>
.sidebar {
  width: 200px;
  min-height: 100vh;
  background: linear-gradient(180deg, #1a1a2e 0%, #16213e 100%);
  color: #ccc;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
}
.sidebar-logo {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 20px 18px 16px;
}
.logo-icon { font-size: 24px; }
.logo-text {
  font-size: 17px;
  font-weight: 700;
  color: #fff;
  letter-spacing: 0.5px;
}
.sidebar-nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 0 10px;
  flex: 1;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 11px 14px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: #aaa;
  cursor: pointer;
  font-size: 14px;
  text-align: left;
  transition: all 0.15s;
  width: 100%;
}
.nav-item:hover {
  background: rgba(255,255,255,0.06);
  color: #ddd;
}
.nav-item.active {
  background: rgba(22, 119, 255, 0.15);
  color: #fff;
  font-weight: 600;
}
.nav-item.active .nav-icon { filter: grayscale(0); }
.nav-icon { font-size: 17px; width: 24px; text-align: center; }
.nav-label { white-space: nowrap; }

.sidebar-footer {
  padding: 16px 18px;
  border-top: 1px solid rgba(255,255,255,0.06);
}
.version {
  font-size: 11px;
  color: #555;
}
</style>
