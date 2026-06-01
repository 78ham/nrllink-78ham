<template>
  <div class="page">
    <h2>设备管理</h2>
    <n-data-table :columns="columns" :data="devices" :bordered="false" :loading="loading" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NDataTable, NTag } from 'naive-ui'
import { useUserStore } from '../stores/user'

const userStore = useUserStore()
const loading = ref(false)
const devices = ref<any[]>([])

const columns = [
  { title: '呼号', key: 'callsign' },
  { title: 'SSID', key: 'ssid' },
  { title: '型号', key: 'dev_model' },
  { title: '状态', key: 'is_online', render: (r: any) => h(NTag, { type: r.is_online ? 'success' : 'default' }, { default: () => r.is_online ? '在线' : '离线' }) },
  { title: '群组', key: 'group_id' },
  { title: 'IP', key: 'qth' },
]

async function fetchDevices() {
  loading.value = true
  try {
    const res = await fetch('/api/v1/devices', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'x-token': userStore.token },
      body: '{}'
    })
    const json = await res.json()
    if (json.data?.items) {
      devices.value = Object.values(json.data.items)
    }
  } catch { /* ignore */ }
  finally { loading.value = false }
}

onMounted(fetchDevices)
</script>

<style scoped>
.page { padding: 24px; }
</style>
