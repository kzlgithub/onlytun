<template>
  <div class="page-shell">
    <el-card class="panel-card" shadow="never">
      <template #header>
        <div class="page-header rules-header">
          <div>
            <h3 class="section-title">转发规则</h3>
            <p class="section-meta">
              共 {{ ruleStore.rules.length }} 条，{{ ruleStore.enabledRules.length }} 条启用
            </p>
          </div>
          <div class="toolbar rules-toolbar">
            <el-input
              v-model="keyword"
              class="search-input"
              clearable
              placeholder="搜索规则、路径、协议"
            />
            <el-button :loading="manualRefreshing" round @click="manualRefresh">立即刷新</el-button>
            <el-button type="primary" @click="openCreateDialog">新增规则</el-button>
          </div>
        </div>
      </template>

      <el-table :data="filteredRules" v-loading="initialLoading" row-key="id">
        <el-table-column label="规则名称" min-width="180">
          <template #default="{ row }">
            <router-link :to="`/rules/${row.id}/stats`" class="rule-link">
              {{ row.name }}
            </router-link>
          </template>
        </el-table-column>
        <el-table-column label="路径" min-width="330">
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
        <el-table-column label="活跃连接数" width="120">
          <template #default="{ row }">
            {{ row.realtime_stat?.peak_conns || 0 }}
          </template>
        </el-table-column>
        <el-table-column label="流量上限" width="150">
          <template #default="{ row }">
            <el-tag v-if="row.limit_exceeded" type="danger" effect="light" round>已超限</el-tag>
            <span v-else>{{ row.traffic_limit_bytes > 0 ? formatBytes(row.traffic_limit_bytes) : '无限制' }}</span>
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
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import RuleFormDialog from '../components/RuleFormDialog.vue';
import { useMachineStore } from '../stores/machine';
import { useRuleStore } from '../stores/rule';
import { formatBytes, protocolLabel } from '../utils/format';

const machineStore = useMachineStore();
const ruleStore = useRuleStore();

const keyword = ref('');
const initialLoading = ref(false);
const manualRefreshing = ref(false);
const refreshing = ref(false);
const dialogVisible = ref(false);
const editingRule = ref(null);
const submitting = ref(false);
let timer;
let loadingPromise = null;

const filteredRules = computed(() => {
  const q = keyword.value.trim().toLowerCase();
  if (!q) {
    return ruleStore.rules;
  }
  return ruleStore.rules.filter((rule) => {
    const path = buildRoutePath(rule);
    const haystack = [
      rule.name,
      rule.protocol,
      protocolLabel(rule.protocol),
      rule.target_addr,
      rule.target_port,
      rule.ingress_port,
      path,
      rule.remark,
    ]
      .filter((item) => item !== undefined && item !== null)
      .join(' ')
      .toLowerCase();
    return haystack.includes(q);
  });
});

function buildRoutePath(rule) {
  const ingress = machineStore.machineMap[rule.ingress_machine_id];
  const egress = machineStore.machineMap[rule.egress_machine_id];
  const ingressName = ingress?.name || ingress?.ip || '入口机';
  const egressName = egress?.name || egress?.ip || '出口机';
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
  await loadData({ initial: ruleStore.rules.length === 0 });
  timer = window.setInterval(() => loadData(), 5000);
});

onBeforeUnmount(() => {
  window.clearInterval(timer);
});
</script>

<style scoped>
.rules-header {
  align-items: flex-start;
  flex-direction: column;
}

.section-title {
  margin: 0;
  font-size: 20px;
  color: #132238;
}

.section-meta {
  margin: 6px 0 0;
  font-size: 13px;
  color: #72829d;
}

.search-input {
  width: 260px;
  margin-right: auto;
}

.rules-toolbar {
  width: 100%;
  justify-content: flex-end;
}

.rule-link {
  color: #409eff;
  font-weight: 600;
}

.path-line {
  color: #3c4a61;
  line-height: 1.6;
}

@media (max-width: 900px) {
  .rules-header {
    flex-direction: column;
  }

  .toolbar,
  .search-input {
    width: 100%;
  }
}
</style>
