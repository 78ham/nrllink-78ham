<template>
  <div class="page">
    <h2>系统设置</h2>
    <n-card title="个人信息" style="margin-bottom:16px;">
      <n-description>
        <n-description-item label="呼号">{{ userStore.user?.callsign }}</n-description-item>
        <n-description-item label="手机">{{ userStore.user?.phone }}</n-description-item>
        <n-description-item label="角色">{{ userStore.user?.roles?.join(', ') }}</n-description-item>
      </n-description>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { NCard } from 'naive-ui'
import { useUserStore } from '../stores/user'
const userStore = useUserStore()
</script>

<style scoped>
.page { padding: 24px; }
</style>
