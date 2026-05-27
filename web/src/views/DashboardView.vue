<template>
  <div class="page-shell">
    <section class="stat-grid">
      <StatCard
        title="入口机在线"
        :value="`${onlineIngress}/${totalIngress}`"
        subtitle="在线入口机 / 总入口机"
        :icon="Monitor"
      />
      <StatCard
        title="出口机在线"
        :value="`${onlineEgress}/${totalEgress}`"
        subtitle="在线出口机 / 总出口机"
        :icon="Connection"
      />
      <StatCard
        title="启用规则数"
        :value="String(enabledRuleCount)"
        subtitle="当前已启用的转发链路"
        :icon="Promotion"
      />
      <StatCard
        title="今日总流量"
        :value="formatBytes(todayTraffic)"
        subtitle="所有规则当日累计上行 + 下行"
        :icon="DataAnalysis"
      />
    </section>

    <el-card class="panel-card" shadow="never">
      <template #header>
        <div class="page-header">
          <div>
            <h3 style="margin: 0">面板机状态</h3>
          </div>
          <el-button :loading="loading" @click="loadData">立即刷新</el-button>
        </div>
      </template>

      <div class="panel-gauge-grid">
        <GaugeCard title="CPU" :value="panelMetrics.cpu_percent" color="#409eff" />
        <GaugeCard title="内存" :value="panelMetrics.mem_percent" color="#46b389" />
        <GaugeCard title="硬盘" :value="panelMetrics.disk_percent" color="#f59e0b" />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { Connection, DataAnalysis, Monitor, Promotion } from '@element-plus/icons-vue';
import GaugeCard from '../components/GaugeCard.vue';
import StatCard from '../components/StatCard.vue';
import { panelApi } from '../api';
import { useMachineStore } from '../stores/machine';
import { useRuleStore } from '../stores/rule';
import { formatBytes } from '../utils/format';

const machineStore = useMachineStore();
const ruleStore = useRuleStore();
const loading = ref(false);
const panelMetrics = ref({
  cpu_percent: 0,
  mem_percent: 0,
  disk_percent: 0,
});
let timer;

const totalIngress = computed(() => machineStore.ingressMachines.length);
const onlineIngress = computed(() => machineStore.onlineIngressMachines.length);
const totalEgress = computed(() => machineStore.egressMachines.length);
const onlineEgress = computed(() => machineStore.onlineEgressMachines.length);
const enabledRuleCount = computed(() => ruleStore.enabledRules.length);
const todayTraffic = computed(() =>
  Object.values(ruleStore.dayTotals).reduce((sum, value) => sum + (value || 0), 0),
);

async function loadData() {
  loading.value = true;
  try {
    const [, , metricsResp] = await Promise.all([
      machineStore.fetchMachines(),
      ruleStore.fetchRules({ includeDayTotals: true }),
      panelApi.metrics(),
    ]);
    panelMetrics.value = metricsResp.data || panelMetrics.value;
  } finally {
    loading.value = false;
  }
}

onMounted(async () => {
  await loadData();
  timer = window.setInterval(loadData, 5000);
});

onBeforeUnmount(() => {
  window.clearInterval(timer);
});
</script>

<style scoped>
.panel-gauge-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 18px;
}

@media (max-width: 900px) {
  .panel-gauge-grid {
    grid-template-columns: 1fr;
  }
}
</style>
