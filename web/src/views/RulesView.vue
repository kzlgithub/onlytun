<template>
  <div class="page-shell">
    <el-card class="panel-card" shadow="never">
      <template #header>
        <div class="page-header">
          <div>
            <h3 style="margin: 0">转发规则</h3>
          </div>
          <div class="toolbar">
            <span v-if="refreshing" class="refresh-hint">正在更新</span>
            <el-button :loading="manualRefreshing" @click="manualRefresh">立即刷新</el-button>
            <el-button type="primary" @click="openCreateDialog">新增规则</el-button>
          </div>
        </div>
      </template>

      <el-table :data="ruleStore.rules" v-loading="initialLoading" row-key="id">
        <el-table-column label="规则名称" min-width="180">
          <template #default="{ row }">
            <router-link :to="`/rules/${row.id}/stats`" class="rule-link">
              {{ row.name }}
            </router-link>
          </template>
        </el-table-column>
        <el-table-column label="路径" min-width="320">
          <template #default="{ row }">
            <div class="path-line">
              {{ buildRoutePath(row) }}
            </div>
          </template>
        </el-table-column>
        <el-table-column label="协议" width="110">
          <template #default="{ row }">
            <el-tag effect="light" round>{{ protocolLabel(row.protocol) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-switch
              :model-value="row.enabled"
              inline-prompt
              active-text="开"
              inactive-text="关"
              @change="toggleRule(row)"
            />
          </template>
        </el-table-column>
        <el-table-column label="实时上行速率" width="150">
          <template #default="{ row }">
            {{ formatSpeed(ruleStore.rateMap[row.id]?.up || 0) }}
          </template>
        </el-table-column>
        <el-table-column label="实时下行速率" width="150">
          <template #default="{ row }">
            {{ formatSpeed(ruleStore.rateMap[row.id]?.down || 0) }}
          </template>
        </el-table-column>
        <el-table-column label="活跃连接数" width="120">
          <template #default="{ row }">
            {{ row.realtime_stat?.peak_conns || 0 }}
          </template>
        </el-table-column>
        <el-table-column label="今日流量" width="140">
          <template #default="{ row }">
            {{ formatBytes(ruleStore.dayTotals[row.id] || 0) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link @click="openEditDialog(row)">编辑</el-button>
            <el-button type="danger" link @click="deleteRule(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <RuleFormDialog
      v-model="dialogVisible"
      :rule="editingRule"
      :submitting="submitting"
      :ingress-options="machineStore.onlineIngressMachines"
      :egress-options="machineStore.onlineEgressMachines"
      @submit="submitRule"
    />
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import RuleFormDialog from '../components/RuleFormDialog.vue';
import { useMachineStore } from '../stores/machine';
import { useRuleStore } from '../stores/rule';
import { formatBytes, formatSpeed, protocolLabel } from '../utils/format';

const machineStore = useMachineStore();
const ruleStore = useRuleStore();

const initialLoading = ref(false);
const manualRefreshing = ref(false);
const refreshing = ref(false);
const dialogVisible = ref(false);
const editingRule = ref(null);
const submitting = ref(false);
let timer;
let loadingPromise = null;

function buildRoutePath(rule) {
  const ingress = machineStore.machineMap[rule.ingress_machine_id];
  const egress = machineStore.machineMap[rule.egress_machine_id];
  const ingressName = ingress?.name || '入口机';
  const egressName = egress?.name || '出口机';
  return `${ingressName}:${rule.ingress_port} → ${egressName} → ${rule.target_addr}:${rule.target_port}`;
}

async function loadData(options = {}) {
  if (loadingPromise) {
    if (options.manual) {
      manualRefreshing.value = true;
      try {
        await loadingPromise;
      } finally {
        manualRefreshing.value = false;
      }
    }
    return loadingPromise;
  }

  const { initial = false, manual = false } = options;
  initialLoading.value = initial;
  manualRefreshing.value = manual;
  refreshing.value = !initial && !manual;

  loadingPromise = Promise.all([
    machineStore.fetchMachines(),
    ruleStore.fetchRules({ includeDayTotals: true }),
  ]);

  try {
    await loadingPromise;
  } finally {
    initialLoading.value = false;
    manualRefreshing.value = false;
    refreshing.value = false;
    loadingPromise = null;
  }
}

function manualRefresh() {
  return loadData({ manual: true });
}

function openCreateDialog() {
  editingRule.value = null;
  dialogVisible.value = true;
}

function openEditDialog(rule) {
  editingRule.value = { ...rule };
  dialogVisible.value = true;
}

async function submitRule(form) {
  submitting.value = true;
  try {
    const payload = {
      ...form,
      enabled: editingRule.value?.enabled ?? true,
    };
    if (editingRule.value?.id) {
      await ruleStore.updateRule(editingRule.value.id, payload);
      ElMessage.success('规则已更新');
    } else {
      await ruleStore.createRule(payload);
      ElMessage.success('规则已创建');
    }
    dialogVisible.value = false;
    editingRule.value = null;
    await loadData();
  } finally {
    submitting.value = false;
  }
}

async function toggleRule(rule) {
  await ruleStore.toggleRule(rule.id);
  ElMessage.success(`规则已${rule.enabled ? '禁用' : '启用'}`);
  await loadData();
}

async function deleteRule(rule) {
  await ElMessageBox.confirm(`确定删除规则“${rule.name}”吗？`, '删除确认', {
    type: 'warning',
    confirmButtonText: '删除',
    cancelButtonText: '取消',
  });

  await ruleStore.deleteRule(rule.id);
  ElMessage.success('规则已删除');
  await loadData();
}

onMounted(async () => {
  await loadData({ initial: true });
  timer = window.setInterval(() => loadData(), 5000);
});

onBeforeUnmount(() => {
  window.clearInterval(timer);
});
</script>

<style scoped>
.rule-link {
  color: #409eff;
  font-weight: 600;
}

.path-line {
  color: #3c4a61;
  line-height: 1.6;
}

.refresh-hint {
  font-size: 13px;
  color: #8a99ad;
}
</style>
