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
            <el-button :loading="manualRefreshing || refreshing" round @click="manualRefresh">立即刷新</el-button>
            <el-button round @click="openImportDialog">批量导入</el-button>
            <el-button round :disabled="selectedRules.length === 0" @click="openExportDialog">
              导出选中
            </el-button>
            <el-button type="primary" @click="openCreateDialog">新增规则</el-button>
          </div>
        </div>
      </template>

      <el-table
        :data="filteredRules"
        v-loading="initialLoading"
        row-key="id"
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="46" />
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

    <el-dialog v-model="importDialogVisible" title="批量导入规则" width="720px" class="batch-dialog">
      <div class="batch-tip">
        每行一条，格式：<code>名称#入口机端口#目标地址#目标端口</code>。空行会自动跳过。
      </div>
      <div class="batch-options">
        <el-select v-model="importDefaults.ingress_machine_id" placeholder="选择入口机">
          <el-option
            v-for="machine in machineStore.onlineIngressMachines"
            :key="machine.id"
            :label="`${machine.name} (${machine.ip || '未上报IP'})`"
            :value="machine.id"
          />
        </el-select>
        <el-select v-model="importDefaults.egress_machine_id" placeholder="选择出口机">
          <el-option
            v-for="machine in machineStore.onlineEgressMachines"
            :key="machine.id"
            :label="`${machine.name} (${machine.ip || '未上报IP'})`"
            :value="machine.id"
          />
        </el-select>
        <el-select v-model="importDefaults.protocol" placeholder="协议" class="protocol-select">
          <el-option label="TCP" value="tcp" />
          <el-option label="UDP" value="udp" />
          <el-option label="TCP+UDP" value="both" />
        </el-select>
      </div>
      <el-input
        v-model="importText"
        type="textarea"
        :rows="12"
      />
      <template #footer>
        <div class="dialog-footer-actions">
          <el-button @click="importDialogVisible = false">取消</el-button>
          <el-button type="primary" :loading="importing" @click="importRules">开始导入</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="exportDialogVisible" title="导出选中规则" width="680px" class="batch-dialog">
      <div class="batch-tip">
        已选择 {{ selectedRules.length }} 条规则，导出格式与导入格式一致。
      </div>
      <el-input v-model="exportText" type="textarea" :rows="12" readonly />
      <template #footer>
        <div class="dialog-footer-actions">
          <el-button @click="downloadExport">下载 TXT</el-button>
          <el-button type="primary" @click="copyExport">复制内容</el-button>
        </div>
      </template>
    </el-dialog>
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
const selectedRules = ref([]);
const importDialogVisible = ref(false);
const exportDialogVisible = ref(false);
const importing = ref(false);
const importText = ref('');
const exportText = ref('');
const importDefaults = ref({
  ingress_machine_id: '',
  egress_machine_id: '',
  protocol: 'tcp',
});
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

function handleSelectionChange(rows) {
  selectedRules.value = rows;
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

function openImportDialog() {
  if (!importDefaults.value.ingress_machine_id && machineStore.onlineIngressMachines.length === 1) {
    importDefaults.value.ingress_machine_id = machineStore.onlineIngressMachines[0].id;
  }
  if (!importDefaults.value.egress_machine_id && machineStore.onlineEgressMachines.length === 1) {
    importDefaults.value.egress_machine_id = machineStore.onlineEgressMachines[0].id;
  }
  importDialogVisible.value = true;
}

function openExportDialog() {
  if (selectedRules.value.length === 0) {
    ElMessage.warning('请先选择要导出的规则');
    return;
  }
  exportText.value = selectedRules.value.map(formatRuleLine).join('\n');
  exportDialogVisible.value = true;
}

function formatRuleLine(rule) {
  return [rule.name, rule.ingress_port, rule.target_addr, rule.target_port].join('#');
}

function parseImportText(text) {
  const rows = [];
  const errors = [];
  const seenPorts = new Set();
  const lines = text.split(/\r?\n/);

  lines.forEach((line, index) => {
    const lineNo = index + 1;
    const raw = line.trim();
    if (!raw) {
      return;
    }

    const parts = raw.split('#').map((item) => item.trim());
    if (parts.length !== 4) {
      errors.push(`第 ${lineNo} 行格式错误，应为 4 段`);
      return;
    }

    const [name, ingressPortRaw, targetAddr, targetPortRaw] = parts;
    const ingressPort = Number(ingressPortRaw);
    const targetPort = Number(targetPortRaw);
    if (!name) {
      errors.push(`第 ${lineNo} 行名称不能为空`);
    }
    if (!Number.isInteger(ingressPort) || ingressPort < 1 || ingressPort > 65535) {
      errors.push(`第 ${lineNo} 行入口端口必须是 1-65535 的整数`);
    }
    if (!targetAddr) {
      errors.push(`第 ${lineNo} 行目标地址不能为空`);
    }
    if (!Number.isInteger(targetPort) || targetPort < 1 || targetPort > 65535) {
      errors.push(`第 ${lineNo} 行目标端口必须是 1-65535 的整数`);
    }

    const portKey = `${importDefaults.value.ingress_machine_id}:${ingressPort}:${importDefaults.value.protocol}`;
    if (seenPorts.has(portKey)) {
      errors.push(`第 ${lineNo} 行入口端口在本次导入内容中重复`);
    }
    seenPorts.add(portKey);

    rows.push({
      name,
      ingress_port: ingressPort,
      target_addr: targetAddr,
      target_port: targetPort,
    });
  });

  return { rows, errors };
}

function ensureImportDefaults() {
  if (!importDefaults.value.ingress_machine_id) {
    ElMessage.warning('请选择入口机');
    return false;
  }
  if (!importDefaults.value.egress_machine_id) {
    ElMessage.warning('请选择出口机');
    return false;
  }
  if (!importDefaults.value.protocol) {
    ElMessage.warning('请选择协议');
    return false;
  }
  return true;
}

async function importRules() {
  if (!ensureImportDefaults()) {
    return;
  }

  const { rows, errors } = parseImportText(importText.value);
  if (errors.length > 0) {
    ElMessage.error(errors.slice(0, 3).join('；'));
    return;
  }
  if (rows.length === 0) {
    ElMessage.warning('没有可导入的规则');
    return;
  }

  importing.value = true;
  let created = 0;
  try {
    for (const row of rows) {
      await ruleStore.createRule({
        ...row,
        ingress_machine_id: importDefaults.value.ingress_machine_id,
        egress_machine_id: importDefaults.value.egress_machine_id,
        protocol: importDefaults.value.protocol,
        enabled: true,
        traffic_limit_bytes: 0,
        remark: '',
      }, { refresh: false });
      created += 1;
    }
    ElMessage.success(`已导入 ${created} 条规则`);
    importDialogVisible.value = false;
    importText.value = '';
    await loadData();
  } catch (error) {
    ElMessage.error(`已导入 ${created} 条，后续导入失败：${error.response?.data?.error || error.message}`);
  } finally {
    importing.value = false;
  }
}

async function copyExport() {
  if (!exportText.value) {
    return;
  }
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(exportText.value);
    } else {
      copyTextFallback(exportText.value);
    }
  } catch {
    copyTextFallback(exportText.value);
  }
  ElMessage.success('导出内容已复制');
}

function copyTextFallback(text) {
  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.style.position = 'fixed';
  textarea.style.opacity = '0';
  document.body.appendChild(textarea);
  textarea.select();
  document.execCommand('copy');
  document.body.removeChild(textarea);
}

function downloadExport() {
  const blob = new Blob([exportText.value], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `onlytun-rules-${new Date().toISOString().slice(0, 10)}.txt`;
  link.click();
  URL.revokeObjectURL(url);
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

.batch-tip {
  margin-bottom: 14px;
  padding: 12px 14px;
  border-radius: 14px;
  color: #5f728c;
  background: #f6f9fd;
  border: 1px solid rgba(84, 112, 150, 0.12);
  line-height: 1.7;
}

.batch-tip code {
  padding: 2px 6px;
  border-radius: 7px;
  color: #1f6feb;
  background: rgba(64, 158, 255, 0.1);
}

.batch-options {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) 130px;
  gap: 12px;
  margin-bottom: 14px;
}

.dialog-footer-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

:deep(.batch-dialog .el-dialog__body) {
  padding-top: 12px;
}

@media (max-width: 900px) {
  .rules-header {
    flex-direction: column;
  }

  .toolbar,
  .search-input {
    width: 100%;
  }

  .batch-options {
    grid-template-columns: 1fr;
  }
}
</style>
