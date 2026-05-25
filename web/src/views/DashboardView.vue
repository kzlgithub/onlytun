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
            <h3 style="margin: 0">系统概览</h3>
            <p class="page-subtitle">每 5 秒自动刷新一次，帮助你快速判断当前负载和规则活跃度。</p>
          </div>
          <el-button :loading="loading" @click="loadData">立即刷新</el-button>
        </div>
      </template>

      <div class="summary-grid">
        <div class="summary-box">
          <p class="summary-label">入口机在线率</p>
          <p class="summary-value">{{ ingressOnlineRate }}</p>
        </div>
        <div class="summary-box">
          <p class="summary-label">出口机在线率</p>
          <p class="summary-value">{{ egressOnlineRate }}</p>
        </div>
        <div class="summary-box">
          <p class="summary-label">规则启用占比</p>
          <p class="summary-value">{{ ruleEnabledRate }}</p>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { Connection, DataAnalysis, Monitor, Promotion } from '@element-plus/icons-vue';
import StatCard from '../components/StatCard.vue';
import { useMachineStore } from '../stores/machine';
import { useRuleStore } from '../stores/rule';
import { formatBytes } from '../utils/format';

const machineStore = useMachineStore();
const ruleStore = useRuleStore();
const loading = ref(false);
let timer;

const totalIngress = computed(() => machineStore.ingressMachines.length);
const onlineIngress = computed(() => machineStore.onlineIngressMachines.length);
const totalEgress = computed(() => machineStore.egressMachines.length);
const onlineEgress = computed(() => machineStore.onlineEgressMachines.length);
const enabledRuleCount = computed(() => ruleStore.enabledRules.length);
const todayTraffic = computed(() =>
  Object.values(ruleStore.dayTotals).reduce((sum, value) => sum + (value || 0), 0),
);

const ingressOnlineRate = computed(() =>
  totalIngress.value ? `${Math.round((onlineIngress.value / totalIngress.value) * 100)}%` : '0%',
);
const egressOnlineRate = computed(() =>
  totalEgress.value ? `${Math.round((onlineEgress.value / totalEgress.value) * 100)}%` : '0%',
);
const ruleEnabledRate = computed(() =>
  ruleStore.rules.length ? `${Math.round((enabledRuleCount.value / ruleStore.rules.length) * 100)}%` : '0%',
);

async function loadData() {
  loading.value = true;
  try {
    await Promise.all([
      machineStore.fetchMachines(),
      ruleStore.fetchRules({ includeDayTotals: true }),
    ]);
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
