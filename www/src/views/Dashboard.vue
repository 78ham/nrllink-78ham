<template>
  <div class="dashboard">
    <h2>系统概览</h2>
    <div class="stats-grid">
      <n-card v-for="s in stats" :key="s.label" class="stat-card">
        <n-statistic :title="s.label" :value="s.value" :tabular-numbers="true" />
      </n-card>
    </div>
    <div class="charts">
      <n-card title="在线设备趋势" class="chart-card">
        <v-chart :option="chartOption" autoresize style="height:300px" />
      </n-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NCard, NStatistic } from 'naive-ui'
import VChart from 'vue-echarts'
import { use } from 'echarts/core'
import { LineChart } from 'echarts/charts'
import { GridComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

use([LineChart, GridComponent, TooltipComponent, CanvasRenderer])

const stats = ref([
  { label: '在线设备', value: 0 },
  { label: '用户数', value: 0 },
  { label: '群组数', value: 0 },
  { label: '今日流量', value: '0 MB' }
])

const chartOption = ref({
  tooltip: { trigger: 'axis' },
  grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
  xAxis: { type: 'category', data: [] as string[], axisLine: { lineStyle: { color: '#2a2f3e' } } },
  yAxis: { type: 'value', splitLine: { lineStyle: { color: '#2a2f3e' } } },
  series: [{ type: 'line', data: [] as number[], smooth: true, lineStyle: { color: '#00d4aa' }, areaStyle: { color: 'rgba(0,212,170,0.1)' } }]
})

async function fetchStats() {
  try {
    const res = await fetch('/api/v1/stats')
    const json = await res.json()
    if (json.data?.items) {
      stats.value[0].value = json.data.items.online_dev_number || 0
      stats.value[1].value = json.data.items.user_number || 0
    }
  } catch { /* ignore */ }
}

onMounted(fetchStats)
</script>

<style scoped>
.dashboard { padding: 24px; }
.stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 16px; margin-bottom: 24px; }
.stat-card { background: var(--bg-card); border: 1px solid var(--border); }
.charts { display: grid; grid-template-columns: 1fr; gap: 16px; }
.chart-card { background: var(--bg-card); border: 1px solid var(--border); }
</style>
