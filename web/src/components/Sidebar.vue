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
        :class="['nav-item', { active: props.current === item.key }]"
        @click="$emit('change', item.key)"
      >
        <span class="nav-icon">{{ item.icon }}</span>
        <span class="nav-label">{{ item.label }}</span>
      </button>
    </nav>

    <!-- 左下角用户信息区域 -->
    <div class="user-area" @click.stop="toggleUserMenu">
      <div class="user-avatar">
        <template v-if="userInfo?.avatar">
          <img :src="userInfo.avatar" alt="" class="avatar-img" />
        </template>
        <template v-else>
          {{ (userInfo?.nickname || userInfo?.username || '?').charAt(0).toUpperCase() }}
        </template>
      </div>
      <span class="user-name">{{ userInfo?.nickname || userInfo?.username || '未登录' }}</span>
      <span class="user-arrow">{{ showUserMenu ? '▲' : '▼' }}</span>

      <!-- 下拉菜单 -->
      <teleport to="body">
        <div
          v-if="showUserMenu"
          class="user-menu-overlay"
          @click.self="showUserMenu = false"
        >
          <div class="user-menu" :style="menuStyle">
            <!-- 用户信息头部 -->
            <div class="menu-user-header">
              <div class="menu-avatar">
                <template v-if="userInfo?.avatar">
                  <img :src="userInfo.avatar" alt="" class="avatar-img" />
                </template>
                <template v-else>
                  {{ (userInfo?.nickname || userInfo?.username || '?').charAt(0).toUpperCase() }}
                </template>
              </div>
              <div class="menu-user-info">
                <div class="menu-username">{{ userInfo?.nickname || userInfo?.username || '' }}</div>
                <div class="menu-role">{{ roleLabel }}</div>
              </div>
            </div>
            <div class="menu-divider"></div>
            <!-- 菜单项 -->
            <button class="menu-item" @click.stop="onProfile">
              <span>👤 个人主页</span>
            </button>
            <button class="menu-item danger" @click.stop="onLogout">
              <span>🚪 退出登录</span>
            </button>
          </div>
        </div>
      </teleport>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue'

const props = defineProps<{
  current: string
  userInfo?: { nickname?: string; username?: string; avatar?: string; role?: string } | null
}>()

const emit = defineEmits<{ change: [key: string]; logout: []; profile: [] }>()

// 扁平化一级菜单（管理员不展示策略订阅和持仓管理）
const menuItems = computed(() => {
  const isAdmin = props.userInfo?.role === 'admin'
  const base: { key: string; label: string; icon: string }[] = [
    { key: 'strategy-list', label: '策略列表', icon: '📂' },
    { key: 'strategy-backtest', label: '策略回测', icon: '📊' },
  ]
  if (!isAdmin) {
    base.push({ key: 'strategy-subscribe', label: '策略订阅', icon: '🔔' })
    base.push({ key: 'monitor-config', label: '盯盘助手', icon: '📈' })
    base.push({ key: 'my-positions', label: '持仓管理', icon: '💼' })
  }
  base.push({ key: 'bot-config', label: '机器人配置', icon: '🤖' })
  if (isAdmin) {
    base.push({ key: 'account-management', label: '账户管理', icon: '👥' })
    base.push({ key: 'data-collect', label: '数据采集', icon: '📡' })
  }
  return base
})

// ========== 用户下拉菜单 ==========
const showUserMenu = ref(false)
const menuStyle = reactive<Record<string, string>>({})

function toggleUserMenu(e?: MouseEvent) {
  if (showUserMenu.value) {
    showUserMenu.value = false
    return
  }
  // 计算菜单位置（在按钮上方弹出）
  const target = e?.currentTarget as HTMLElement
  if (target) {
    const rect = target.getBoundingClientRect()
    menuStyle.left = `${rect.left}px`
    menuStyle.bottom = `${window.innerHeight - rect.top + 4}px`
  }
  showUserMenu.value = true
}

function onLogout() {
  showUserMenu.value = false
  // 延迟触发，让菜单动画先关闭
  setTimeout(() => emit('logout'), 100)
}

function onProfile() {
  showUserMenu.value = false
  setTimeout(() => emit('profile'), 100)
}

const roleLabel = computed(() => {
  const r = props.userInfo?.role
  return r === 'admin' ? '管理员' : '普通用户'
})
</script>

<style scoped>
.sidebar {
  width: 180px;
  height: 100vh;
  position: sticky;
  top: 0;
  background: #fff;
  border-right: 1px solid #e8e8e8;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  overflow-y: auto;
}
.sidebar-logo {
  display: flex; align-items: center; gap: 10px; padding: 20px 18px 16px;
}
.logo-icon { font-size: 22px; }
.logo-text { font-size: 17px; font-weight: 700; color: #1a1a2e; letter-spacing: .5px; }

.sidebar-nav {
  display: flex; flex-direction: column; gap: 1px; padding: 4px 8px; flex: 1;
}

.nav-item {
  display: flex; align-items: center; gap: 10px; padding: 11px 14px;
  border: none; border-radius: 6px; background: transparent; color: #555;
  cursor: pointer; font-size: 14px; text-align: left; transition: all .12s;
  width: 100%; position: relative;
}
.nav-item:hover { background: #f5f5f5; color: #333; }
.nav-item.active { background: #e6f4ff; color: #1677ff; font-weight: 600; }
.nav-item.active::before {
  content: ''; position: absolute; left: -8px; top: 6px; bottom: 6px;
  width: 3px; border-radius: 3px; background: #1677ff;
}
.nav-icon { font-size: 15px; width: 22px; text-align: center; }
.nav-label { white-space: nowrap; }

/* ====== 用户区域（左下角）====== */
.user-area {
  display: flex; align-items: center; gap: 8px; padding: 10px 14px 14px;
  cursor: pointer; border-top: 1px solid #f0f0f0;
  transition: background .12s; position: relative;
}
.user-area:hover { background: #fafafa; }

.user-avatar {
  width: 32px; height: 32px; border-radius: 50%;
  background: linear-gradient(135deg, #1677ff, #69b1ff);
  color: #fff; font-size: 14px; font-weight: 700;
  display: flex; align-items: center; justify-content: center;
  flex-shrink: 0; overflow: hidden;
}
.avatar-img { width: 100%; height: 100%; object-fit: cover; }
.user-name {
  flex: 1; font-size: 13.5px; font-weight: 600; color: #333;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.user-arrow { font-size: 9px; color: #bbb; flex-shrink: 0; transition: transform .15s; }

/* ====== 用户菜单（Teleport 到 body）====== */
.user-menu-overlay {
  position: fixed; inset: 0; z-index: 998;
}
.user-menu {
  position: fixed; z-index: 999; min-width: 200px;
  background: #fff; border: 1px solid #e8e8e8; border-radius: 10px;
  box-shadow: 0 6px 24px rgba(0,0,0,.12); padding: 6px 0; animation: menu-fade-in .15s ease-out;
}
@keyframes menu-fade-in {
  from { opacity: 0; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
}

.menu-user-header {
  display: flex; align-items: center; gap: 10px; padding: 14px 16px 10px;
}
.menu-avatar {
  width: 40px; height: 40px; border-radius: 50%;
  background: linear-gradient(135deg, #1677ff, #69b1ff);
  color: #fff; font-size: 17px; font-weight: 700;
  display: flex; align-items: center; justify-content: center;
  flex-shrink: 0; overflow: hidden;
}
.menu-avatar .avatar-img { width: 100%; height: 100%; object-fit: cover; }
.menu-user-info { flex: 1; overflow: hidden; }
.menu-username { font-size: 14px; font-weight: 600; color: #1a1a2e; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.menu-role { font-size: 12px; color: #999; margin-top: 2px; }
.menu-divider { height: 1px; background: #f0f0f0; margin: 4px 12px; }

.menu-item {
  display: flex; align-items: center; gap: 8px; width: 100%;
  padding: 10px 16px; border: none; background: none;
  cursor: pointer; font-size: 13.5px; color: #333;
  transition: background .12s; text-align: left;
}
.menu-item:hover { background: #f5f5f5; }
.menu-item.danger:hover { background: #fff1f0; color: #cf1322; }
</style>
