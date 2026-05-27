<template>
  <div class="page-shell dashboard-page">
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
        subtitle="所有规则今日累计流量"
        :icon="DataAnalysis"
      />
    </section>

    <el-card class="panel-card panel-health" shadow="never">
      <template #header>
        <div class="page-header">
          <div>
            <h3 class="section-title">面板机状态</h3>
          </div>
          <el-button :loading="loading" round @click="loadData">立即刷新</el-button>
        </div>
      </template>

      <div class="gauge-grid">
        <GaugeCard
          title="CPU"
          subtitle="处理器占用"
          :value="panelMetrics.cpu_percent"
          color="#2f80ed"
        />
        <GaugeCard
          title="内存"
          subtitle="运行内存占用"
          :value="panelMetrics.mem_percent"
          color="#12a87f"
        />
        <GaugeCard
          title="硬盘"
          subtitle="系统盘使用率"
          :value="panelMetrics.disk_percent"
          color="#ef8f21"
        />
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';
import { Connection, DataAnalysis, Monitor, Promotion } from '@element-plus/icons-vue';
import GaugeCard from '../components/GaugeCard.vue';
import StatCard from '../components/StatCard.vue';
import { panelApi } from '../api';
import { useMachineStore } from '../stores/machine';
import { useRuleStore } from '../stores/rule';
import { formatBytes } from '../utils/format';

const route = useRoute();
const machineStore = useMachineStore();
const ruleStore = useRuleStore();
const loading = ref(false);
const panelMetrics = ref({
  cpu_percent: 0,
  mem_percent: 0,
  disk_percent: 0,
});
let timer;

const demoMode = computed(() => route.query.demo === '1');
const demoMachines = [
  { id: 'demo-ingress-1', role: 'ingress', online: true },
  { id: 'demo-ingress-2', role: 'ingress', online: true },
  { id: 'demo-ingress-3', role: 'ingress', online: false },
  { id: 'demo-egress-1', role: 'egress', online: true },
  { id: 'demo-egress-2', role: 'egress', online: true },
];
const demoRules = [
  { id: 'demo-rule-1', enabled: true },
  { id: 'demo-rule-2', enabled: true },
  { id: 'demo-rule-3', enabled: false },
  { id: 'demo-rule-4', enabled: true },
];
const demoTotals = {
  'demo-rule-1': 38.4 * 1024 ** 3,
  'demo-rule-2': 16.7 * 1024 ** 3,
  'demo-rule-3': 2.1 * 1024 ** 3,
  'demo-rule-4': 74.6 * 1024 ** 3,
};

const machines = computed(() => (demoMode.value ? demoMachines : machineStore.machines));
const rules = computed(() => (demoMode.value ? demoRules : ruleStore.rules));
const dayTotals = computed(() => (demoMode.value ? demoTotals : ruleStore.dayTotals));
const totalIngress = computed(() => machines.value.filter((item) => item.role === 'ingress').length);
const onlineIngress = computed(() => machines.value.filter((item) => item.role === 'ingress' && item.online).length);
const totalEgress = computed(() => machines.value.filter((item) => item.role === 'egress').length);
const onlineEgress = computed(() => machines.value.filter((item) => item.role === 'egress' && item.online).length);
const enabledRuleCount = computed(() => rules.value.filter((item) => item.enabled).length);
const todayTraffic = computed(() =>
  Object.values(dayTotals.value).reduce((sum, value) => sum + (value || 0), 0),
);

async function loadData() {
  loading.value = true;
  try {
    if (demoMode.value) {
      panelMetrics.value = nextDemoMetrics();
      return;
    }

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

function nextDemoMetrics() {
  const now = Date.now() / 1000;
  return {
    cpu_percent: 31 + Math.sin(now / 4) * 7,
    mem_percent: 48 + Math.cos(now / 5) * 5,
    disk_percent: 36 + Math.sin(now / 7) * 2,
  };
}

onMounted(async () => {
  await loadData();
  timer = window.setInterval(loadData, demoMode.value ? 1200 : 5000);
});

onBeforeUnmount(() => {
  window.clearInterval(timer);
});
</script>

<style scoped>
.dashboard-page {
  gap: 20px;
}

.panel-health {
  overflow: hidden;
}

.section-title {
  margin: 0;
  color: #132238;
  font-size: 20px;
}

.gauge-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 18px;
}

@media (max-width: 1080px) {
  .gauge-grid {
    grid-template-columns: 1fr;
  }
}
</style>
