<template>
  <n-layout position="absolute" style="top:0;bottom:0;left:0;right:0;">
    <n-layout-sider bordered content-style="display:flex;flex-direction:column;" :width="220" :collapsed-width="64" collapse-mode="width" :collapsed="collapsed" @click="collapsed = !collapsed">
      <div class="logo">{{ collapsed ? 'NRL' : 'NRLLink' }}</div>
      <n-menu :collapsed="collapsed" :collapsed-width="64" :collapsed-icon-size="22" :options="displayOptions" @update:value="navigate" />
    </n-layout-sider>
    <n-layout>
      <n-layout-header bordered style="height:48px;display:flex;align-items:center;justify-content:space-between;padding:0 16px;">
        <span class="user-info">{{ userStore.user?.callsign || '未登录' }}</span>
        <n-button quaternary size="small" @click="handleLogout">退出</n-button>
      </n-layout-header>
      <n-alert v-if="userStore.user?.default_admin === true" type="warning" :closable="false" style="border-radius:0;">
        当前正在使用系统默认管理员账号，请在【用户管理】创建您自己的管理员账号后删除此默认账号
      </n-alert>
      <n-layout-content :native-scrollbar="false" style="height:calc(100% - 48px);background:var(--bg-primary);">
        <router-view />
      </n-layout-content>
    </n-layout>
  </n-layout>
</template>

<script setup lang="ts">
import { ref, h, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { NLayout, NLayoutSider, NLayoutHeader, NLayoutContent, NMenu, NButton, NAlert } from 'naive-ui'
import { useUserStore } from '../stores/user'
import type { MenuOption } from 'naive-ui'

const router = useRouter()
const userStore = useUserStore()
const collapsed = ref(false)

const menuOptions: MenuOption[] = [
  { label: '概览', key: '/dashboard', icon: () => h('span', '📊') },
  { label: '设备', key: '/devices', icon: () => h('span', '📡') },
  { label: '群组', key: '/groups', icon: () => h('span', '👥') },
  { label: '语音', key: '/voice', icon: () => h('span', '🎙️') },
  { label: '设置', key: '/settings', icon: () => h('span', '⚙️') },
]

const adminMenuOptions: MenuOption[] = [
  { label: '用户管理', key: '/users', icon: () => h('span', '🪪') },
]

const displayOptions = computed<MenuOption[]>(() =>
  userStore.user?.roles?.includes('admin') ? [...menuOptions, ...adminMenuOptions] : menuOptions
)

function navigate(key: string) { router.push(key) }

async function handleLogout() {
  userStore.logout()
  router.push('/login')
}

onMounted(async () => {
  if (!userStore.token) { router.push('/login'); return }
  const u = await userStore.fetchUser()
  if (u?.must_change_pwd) router.push('/force-password')
})
</script>

<style scoped>
.logo { padding: 16px; font-size: 20px; font-weight: 700; color: var(--accent); text-align: center; border-bottom: 1px solid var(--border); }
.user-info { color: var(--text-primary); font-size: 14px; }
</style>
