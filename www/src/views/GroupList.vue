<template>
  <div class="page">
    <h2>群组管理</h2>
    <n-data-table :columns="columns" :data="groups" :bordered="false" :loading="loading" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, h } from 'vue'
import { NDataTable } from 'naive-ui'
import { useUserStore } from '../stores/user'

const userStore = useUserStore()
const loading = ref(false)
const groups = ref<any[]>([])

const columns = [
  { title: 'ID', key: 'id' },
  { title: '名称', key: 'name' },
  { title: '类型', key: 'type' },
]

async function fetchGroups() {
  loading.value = true
  try {
    const res = await fetch('/api/v1/groups/mini', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'x-token': userStore.token },
      body: '{}'
    })
    const json = await res.json()
    groups.value = json.data?.items || []
  } catch { /* ignore */ }
  finally { loading.value = false }
}

onMounted(fetchGroups)
</script>

<style scoped>
.page { padding: 24px; }
</style>
