<template>
  <div class="page-shell">
    <el-card class="panel-card" shadow="never">
      <template #header>
        <div class="page-header">
          <div>
            <h3 style="margin: 0">{{ ruleInfo?.name || '规则详情' }}</h3>
            <p class="page-subtitle">{{ routePath }}</p>
          </div>
          <div class="toolbar">
            <el-button @click="router.push('/rules')">返回规则列表</el-button>
          </div>
        </div>
      </template>

      <el-tabs v-model="activeRange" @tab-change="loadStats">
        <el-tab-pane label="按天" name="day" />
        <el-tab-pane label="按周" name="week" />
      </el-tabs>

      <TrafficLineChart :points="chartPoints" />
    </el-card>

    <section class="summary-grid">
      <div class="summary-box">
        <p class="summary-label">总上行</p>
        <p class="summary-value">{{ formatBytes(stats.total_up || 0) }}</p>
      </div>
      <div class="summary-box">
        <p class="summary-label">总下行</p>
        <p class="summary-value">{{ formatBytes(stats.total_down || 0) }}</p>
      </div>
      <div class="summary-box">
        <p class="summary-label">合计</p>
        <p class="summary-value">{{ formatBytes((stats.total_up || 0) + (stats.total_down || 0)) }}</p>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import TrafficLineChart from '../components/TrafficLineChart.vue';
import { useMachineStore } from '../stores/machine';
import { useRuleStore } from '../stores/rule';
import { formatBytes } from '../utils/format';

const route = useRoute();
const router = useRouter();
const machineStore = useMachineStore();
const ruleStore = useRuleStore();

const activeRange = ref('day');
const stats = ref({
  points: [],
  total_up: 0,
  total_down: 0,
});

const ruleInfo = computed(() => ruleStore.ruleMap[route.params.id] || null);
const routePath = computed(() => {
  if (!ruleInfo.value) {
    return '正在加载规则信息...';
  }

  const ingress = machineStore.machineMap[ruleInfo.value.ingress_machine_id];
  const egress = machineStore.machineMap[ruleInfo.value.egress_machine_id];
  const ingressName = ingress?.name || '入口机';
  const egressName = egress?.name || '出口机';
  return `${ingressName}:${ruleInfo.value.ingress_port} → ${egressName} → ${ruleInfo.value.target_addr}:${ruleInfo.value.target_port}`;
});

const chartPoints = computed(() =>
  (stats.value.points || []).map((point) => ({
    time:
      activeRange.value === 'day'
        ? new Date(point.time).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
        : new Date(point.time).toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' }),
    bytes_up: point.bytes_up || 0,
    bytes_down: point.bytes_down || 0,
  })),
);

async function loadStats() {
  stats.value = await ruleStore.fetchStats(route.params.id, activeRange.value);
}

onMounted(async () => {
  await Promise.all([machineStore.fetchMachines(), ruleStore.fetchRules()]);
  await loadStats();
});
</script>
