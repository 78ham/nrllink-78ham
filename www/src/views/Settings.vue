<template>
  <div class="page">
    <h2>系统设置</h2>
    <n-card title="个人信息" style="margin-bottom:16px;">
      <n-descriptions>
        <n-descriptions-item label="呼号">{{ userStore.user?.callsign }}</n-descriptions-item>
        <n-descriptions-item label="手机">{{ userStore.user?.phone }}</n-descriptions-item>
        <n-descriptions-item label="角色">{{ userStore.user?.roles?.join(', ') }}</n-descriptions-item>
      </n-descriptions>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import { NCard, NDescriptions, NDescriptionsItem } from 'naive-ui'
import { useUserStore } from '../stores/user'
const userStore = useUserStore()
</script>

<style scoped>
.page { padding: 24px; }
</style>
