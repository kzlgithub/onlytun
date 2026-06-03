<template>
  <div class="page-shell group-rules-page">
    <div class="mode-toggle-row">
      <div class="mode-toggle-pill" :class="{ active: groupStore.modeEnabled }">
        <span class="mode-toggle-label">
          {{ groupStore.modeEnabled ? '设备组模式运行中' : '设备组模式已关闭' }}
        </span>
        <el-switch
          :model-value="groupStore.modeEnabled"
          :loading="modeSaving"
          @change="toggleMode"
        />
      </div>
    </div>

    <el-card class="panel-card group-tabs-card" shadow="never">
      <el-tabs v-model="activeTab" class="group-tabs">
        <el-tab-pane label="分组" name="groups">
        <div class="page-header group-header">
          <div>
            <h3 class="section-title">设备组规则</h3>
            <p class="section-meta">
              {{ groupStore.ingressGroups.length }} 个入口组 · {{ groupStore.egressGroups.length }} 个出口组 ·
              {{ groupStore.rules.length }} 条组规则
            </p>
          </div>
          <div class="toolbar">
            <el-input v-model="keyword" class="search-input" clearable placeholder="搜索组、规则、目标地址" />
            <el-button :loading="manualRefreshing || groupStore.refreshing" round @click="manualRefresh">
              立即刷新
            </el-button>
            <el-button round :disabled="modeDisabled" @click="openGroupDialog('ingress')">新增入口组</el-button>
            <el-button round :disabled="modeDisabled" @click="openGroupDialog('egress')">新增出口组</el-button>
            <el-button v-if="false" type="primary" :disabled="modeDisabled" @click="openRuleDialog()">新增组规则</el-button>
          </div>
        </div>

      <div v-loading="initialLoading" class="group-grid">
        <section class="group-panel">
          <div class="group-panel-head">
            <span>入口组</span>
            <small>每台入口机只属于一个入口组</small>
          </div>
          <div class="group-card-list">
            <div v-for="group in filteredIngressGroups" :key="group.id" class="group-card">
              <div>
                <strong>{{ group.name }}</strong>
                <p>{{ group.machine_count || 0 }} 台入口机</p>
              </div>
              <div class="group-actions">
                <el-button link type="primary" :disabled="modeDisabled" @click="openMembersDialog(group)">成员</el-button>
                <el-button link type="primary" :disabled="modeDisabled" @click="openGroupDialog(group.role, group)">编辑</el-button>
                <el-button link type="danger" :disabled="modeDisabled" @click="deleteGroup(group)">删除</el-button>
              </div>
            </div>
            <el-empty v-if="filteredIngressGroups.length === 0" description="暂无入口组" />
          </div>
        </section>

        <section class="group-panel">
          <div class="group-panel-head">
            <span>出口组</span>
            <small>在线出口机会自动做稳定均衡</small>
          </div>
          <div class="group-card-list">
            <div v-for="group in filteredEgressGroups" :key="group.id" class="group-card">
              <div>
                <strong>{{ group.name }}</strong>
                <p>{{ group.machine_count || 0 }} 台出口机</p>
              </div>
              <div class="group-actions">
                <el-button link type="primary" :disabled="modeDisabled" @click="openMembersDialog(group)">成员</el-button>
                <el-button link type="primary" :disabled="modeDisabled" @click="openGroupDialog(group.role, group)">编辑</el-button>
                <el-button link type="danger" :disabled="modeDisabled" @click="deleteGroup(group)">删除</el-button>
              </div>
            </div>
            <el-empty v-if="filteredEgressGroups.length === 0" description="暂无出口组" />
          </div>
        </section>
      </div>
        </el-tab-pane>

        <el-tab-pane label="规则" name="rules">
        <div class="table-card-header">
          <div>
            <h3 class="section-title">设备组转发规则</h3>
            <p class="section-meta">单条转发规则优先；冲突入口机会自动跳过。</p>
          </div>
          <div class="toolbar">
            <el-input v-model="keyword" class="search-input" clearable placeholder="搜索规则、分组、目标地址" />
            <el-button :loading="manualRefreshing || groupStore.refreshing" round @click="manualRefresh">
              立即刷新
            </el-button>
            <el-button type="primary" :disabled="modeDisabled" @click="openRuleDialog()">新增组规则</el-button>
          </div>
        </div>

      <el-table :data="filteredRules" row-key="id" v-loading="initialLoading">
        <el-table-column label="规则名称" min-width="160">
          <template #default="{ row }">
            <span class="rule-name">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column label="路径" min-width="360">
          <template #default="{ row }">
            <div class="path-line">
              {{ row.ingress_group_name || row.ingress_group_id }}:{{ row.ingress_port }}
              →
              {{ row.egress_group_name || row.egress_group_id }}
              →
              {{ row.target_addr }}:{{ row.target_port }}
            </div>
          </template>
        </el-table-column>
        <el-table-column label="协议" width="100">
          <template #default="{ row }">
            <el-tag effect="light" round>{{ protocolLabel(row.protocol) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <el-switch
              :model-value="row.enabled"
              :disabled="modeDisabled"
              inline-prompt
              active-text="开"
              inactive-text="关"
              @change="toggleRule(row)"
            />
          </template>
        </el-table-column>
        <el-table-column label="生效入口" width="120">
          <template #default="{ row }">
            <el-tag :type="row.effective_machines > 0 ? 'success' : 'warning'" effect="light" round>
              {{ row.effective_machines }}/{{ row.ingress_machine_count }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="冲突" width="90">
          <template #default="{ row }">
            <span :class="{ danger: row.conflict_machines > 0 }">{{ row.conflict_machines || 0 }}</span>
          </template>
        </el-table-column>
        <el-table-column label="出口在线" width="110">
          <template #default="{ row }">
            {{ row.online_egress_count || 0 }}
          </template>
        </el-table-column>
        <el-table-column label="今日流量" width="130">
          <template #default="{ row }">
            {{ formatBytes(row.today_bytes || 0) }}
          </template>
        </el-table-column>
        <el-table-column label="流量上限" width="130">
          <template #default="{ row }">
            <el-tag v-if="row.limit_exceeded" type="danger" effect="light" round>已超限</el-tag>
            <span v-else>{{ row.traffic_limit_bytes > 0 ? formatBytes(row.traffic_limit_bytes) : '无限制' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link :disabled="modeDisabled" @click="openRuleDialog(row)">编辑</el-button>
            <el-button type="danger" link :disabled="modeDisabled" @click="deleteRule(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <el-dialog v-model="groupDialog.visible" :title="groupDialog.id ? '编辑设备组' : '新增设备组'" width="520px">
      <el-form ref="groupFormRef" :model="groupForm" :rules="groupRules" label-position="top">
        <el-form-item label="组名称" prop="name">
          <el-input v-model="groupForm.name" placeholder="例如：上海入口组" />
        </el-form-item>
        <el-form-item label="角色">
          <el-tag effect="light" round>{{ groupForm.role === 'egress' ? '出口组' : '入口组' }}</el-tag>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="groupForm.remark" type="textarea" :rows="3" placeholder="可选备注" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="groupDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitGroup">保存</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="membersDialog.visible" title="管理组成员" width="680px">
      <div class="member-dialog-head">
        <strong>{{ membersDialog.group?.name }}</strong>
        <span>只显示同角色隧道机，保存后会覆盖该组成员。</span>
      </div>
      <el-checkbox-group v-model="memberSelection" class="member-list">
        <el-checkbox v-for="machine in memberOptions" :key="machine.id" :label="machine.id" border>
          {{ machine.name }} · {{ machine.ip || '未上报 IP' }}
        </el-checkbox>
      </el-checkbox-group>
      <template #footer>
        <el-button @click="membersDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitMembers">保存成员</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="ruleDialog.visible" :title="ruleDialog.id ? '编辑设备组规则' : '新增设备组规则'" width="720px">
      <el-form ref="ruleFormRef" :model="ruleForm" :rules="ruleFormRules" label-position="top" class="rule-form">
        <el-form-item label="规则名称" prop="name">
          <el-input v-model="ruleForm.name" placeholder="例如：美国出口组规则" />
        </el-form-item>
        <div class="form-grid">
          <el-form-item label="入口组" prop="ingress_group_id">
            <el-select v-model="ruleForm.ingress_group_id" placeholder="选择入口组">
              <el-option v-for="group in groupStore.ingressGroups" :key="group.id" :label="group.name" :value="group.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="入口端口" prop="ingress_port">
            <el-input-number v-model="ruleForm.ingress_port" :min="1" :max="65535" controls-position="right" />
          </el-form-item>
          <el-form-item label="出口组" prop="egress_group_id">
            <el-select v-model="ruleForm.egress_group_id" placeholder="选择出口组">
              <el-option v-for="group in groupStore.egressGroups" :key="group.id" :label="group.name" :value="group.id" />
            </el-select>
          </el-form-item>
          <el-form-item label="协议" prop="protocol">
            <el-radio-group v-model="ruleForm.protocol">
              <el-radio-button label="tcp">TCP</el-radio-button>
              <el-radio-button label="udp">UDP</el-radio-button>
              <el-radio-button label="both">TCP+UDP</el-radio-button>
            </el-radio-group>
          </el-form-item>
        </div>
        <div class="form-grid">
          <el-form-item label="目标地址" prop="target_addr">
            <el-input v-model="ruleForm.target_addr" placeholder="IP 或域名" />
          </el-form-item>
          <el-form-item label="目标端口" prop="target_port">
            <el-input-number v-model="ruleForm.target_port" :min="1" :max="65535" controls-position="right" />
          </el-form-item>
          <el-form-item label="流量上限">
            <el-input-number v-model="limitGB" :min="0" :step="10" controls-position="right" />
            <span class="limit-tip">GB，0 表示无限制</span>
          </el-form-item>
          <el-form-item label="默认启用">
            <el-switch v-model="ruleForm.enabled" inline-prompt active-text="启用" inactive-text="关闭" />
          </el-form-item>
        </div>
        <el-form-item label="备注">
          <el-input v-model="ruleForm.remark" type="textarea" :rows="3" placeholder="可选备注" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="ruleDialog.visible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitRule">保存规则</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { useGroupRuleStore } from '../stores/groupRule';
import { useMachineStore } from '../stores/machine';
import { formatBytes, protocolLabel } from '../utils/format';

const groupStore = useGroupRuleStore();
const machineStore = useMachineStore();

const keyword = ref('');
const activeTab = ref('rules');
const initialLoading = ref(false);
const manualRefreshing = ref(false);
const submitting = ref(false);
const modeSaving = ref(false);
const groupFormRef = ref(null);
const ruleFormRef = ref(null);
const memberSelection = ref([]);
const limitGB = ref(0);

const groupDialog = reactive({ visible: false, id: '', role: 'ingress' });
const membersDialog = reactive({ visible: false, group: null });
const ruleDialog = reactive({ visible: false, id: '' });

const groupForm = reactive({ name: '', role: 'ingress', remark: '' });
const ruleForm = reactive({
  name: '',
  ingress_group_id: '',
  egress_group_id: '',
  ingress_port: 1,
  target_addr: '',
  target_port: 1,
  protocol: 'tcp',
  traffic_limit_bytes: 0,
  enabled: true,
  remark: '',
});

let timer;

const modeDisabled = computed(() => !groupStore.modeEnabled);

const groupRules = {
  name: [{ required: true, message: '请输入组名称', trigger: 'blur' }],
};

const ruleFormRules = {
  name: [{ required: true, message: '请输入规则名称', trigger: 'blur' }],
  ingress_group_id: [{ required: true, message: '请选择入口组', trigger: 'change' }],
  egress_group_id: [{ required: true, message: '请选择出口组', trigger: 'change' }],
  target_addr: [{ required: true, message: '请输入目标地址', trigger: 'blur' }],
};

const filteredIngressGroups = computed(() => filterGroups(groupStore.ingressGroups));
const filteredEgressGroups = computed(() => filterGroups(groupStore.egressGroups));
const filteredRules = computed(() => {
  const q = keyword.value.trim().toLowerCase();
  if (!q) return groupStore.rules;
  return groupStore.rules.filter((rule) =>
    [
      rule.name,
      rule.ingress_group_name,
      rule.egress_group_name,
      rule.target_addr,
      rule.protocol,
      String(rule.ingress_port),
      String(rule.target_port),
    ]
      .filter(Boolean)
      .join(' ')
      .toLowerCase()
      .includes(q),
  );
});

const memberOptions = computed(() => {
  const role = membersDialog.group?.role;
  if (!role) return [];
  return machineStore.machines.filter((machine) => machine.role === role);
});

function filterGroups(groups) {
  const q = keyword.value.trim().toLowerCase();
  if (!q) return groups;
  return groups.filter((group) => [group.name, group.remark, group.role].filter(Boolean).join(' ').toLowerCase().includes(q));
}

async function loadData(options = {}) {
  if (options.initial) initialLoading.value = true;
  try {
    await Promise.all([
      groupStore.fetchAll({ initial: options.initial }),
      machineStore.fetchMachines(),
    ]);
  } finally {
    initialLoading.value = false;
    manualRefreshing.value = false;
  }
}

async function manualRefresh() {
  manualRefreshing.value = true;
  await loadData();
}

async function toggleMode(enabled) {
  modeSaving.value = true;
  try {
    await ElMessageBox.confirm(
      enabled
        ? '开启后，全体单条转发规则会在运行层面失效，入口机只会下发设备组规则。确定开启吗？'
        : '关闭后，设备组规则会停止下发，入口机恢复使用单条转发规则。确定关闭吗？',
      enabled ? '开启设备组规则模式' : '关闭设备组规则模式',
      { type: enabled ? 'warning' : 'info' },
    );
    await groupStore.setMode(enabled);
    ElMessage.success(enabled ? '设备组规则模式已开启，单条转发规则将停止下发' : '设备组规则模式已关闭，恢复单条转发规则下发');
    await loadData();
  } finally {
    modeSaving.value = false;
  }
}

function openGroupDialog(role, group = null) {
  if (!ensureModeEnabled()) return;
  groupDialog.visible = true;
  groupDialog.id = group?.id || '';
  groupDialog.role = group?.role || role;
  groupForm.name = group?.name || '';
  groupForm.role = group?.role || role;
  groupForm.remark = group?.remark || '';
}

async function submitGroup() {
  await groupFormRef.value?.validate();
  submitting.value = true;
  try {
    const payload = { ...groupForm };
    if (groupDialog.id) await groupStore.updateGroup(groupDialog.id, payload);
    else await groupStore.createGroup(payload);
    groupDialog.visible = false;
    ElMessage.success('设备组已保存');
  } finally {
    submitting.value = false;
  }
}

function openMembersDialog(group) {
  if (!ensureModeEnabled()) return;
  membersDialog.visible = true;
  membersDialog.group = group;
  memberSelection.value = machineStore.machines
    .filter((machine) => machine.role === group.role && machine.group_id === group.id)
    .map((machine) => machine.id);
}

async function submitMembers() {
  submitting.value = true;
  try {
    await groupStore.setGroupMembers(membersDialog.group.id, memberSelection.value);
    await machineStore.fetchMachines();
    membersDialog.visible = false;
    ElMessage.success('组成员已更新');
  } finally {
    submitting.value = false;
  }
}

async function deleteGroup(group) {
  if (!ensureModeEnabled()) return;
  await ElMessageBox.confirm(`确定删除设备组「${group.name}」吗？`, '删除设备组', { type: 'warning' });
  await groupStore.deleteGroup(group.id);
  ElMessage.success('设备组已删除');
}

function openRuleDialog(rule = null) {
  if (!ensureModeEnabled()) return;
  ruleDialog.visible = true;
  ruleDialog.id = rule?.id || '';
  Object.assign(ruleForm, {
    name: rule?.name || '',
    ingress_group_id: rule?.ingress_group_id || '',
    egress_group_id: rule?.egress_group_id || '',
    ingress_port: rule?.ingress_port || 1,
    target_addr: rule?.target_addr || '',
    target_port: rule?.target_port || 1,
    protocol: rule?.protocol || 'tcp',
    traffic_limit_bytes: rule?.traffic_limit_bytes || 0,
    enabled: rule?.enabled ?? true,
    remark: rule?.remark || '',
  });
  limitGB.value = rule?.traffic_limit_bytes ? Math.round(rule.traffic_limit_bytes / 1024 / 1024 / 1024) : 0;
}

async function submitRule() {
  await ruleFormRef.value?.validate();
  submitting.value = true;
  try {
    const payload = {
      ...ruleForm,
      traffic_limit_bytes: Math.max(0, Number(limitGB.value || 0)) * 1024 * 1024 * 1024,
    };
    const data = ruleDialog.id
      ? await groupStore.updateRule(ruleDialog.id, payload)
      : await groupStore.createRule(payload);
    ruleDialog.visible = false;
    if (data?.conflict_rule) {
      ElMessage.warning('入口组端口已被其它组规则占用，本规则已保存为关闭状态');
    } else {
      ElMessage.success('设备组规则已保存');
    }
  } finally {
    submitting.value = false;
  }
}

async function toggleRule(rule) {
  if (!ensureModeEnabled()) {
    rule.enabled = !rule.enabled;
    return;
  }
  try {
    await groupStore.toggleRule(rule.id);
    await groupStore.fetchRules();
  } catch {
    rule.enabled = !rule.enabled;
  }
}

async function deleteRule(rule) {
  if (!ensureModeEnabled()) return;
  await ElMessageBox.confirm(`确定删除组规则「${rule.name}」吗？`, '删除规则', { type: 'warning' });
  await groupStore.deleteRule(rule.id);
  ElMessage.success('设备组规则已删除');
}

function ensureModeEnabled() {
  if (groupStore.modeEnabled) {
    return true;
  }
  ElMessage.warning('请先开启设备组规则模式');
  return false;
}

onMounted(async () => {
  await loadData({ initial: true });
  timer = window.setInterval(() => loadData(), 3000);
});

onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer);
});
</script>

<style scoped>
.group-rules-page {
  display: grid;
  gap: 12px;
}

.mode-toggle-row {
  display: flex;
  justify-content: flex-end;
  margin-top: -10px;
  margin-bottom: -2px;
}

.mode-toggle-pill {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 7px 10px 7px 14px;
  border: 1px solid rgba(113, 135, 166, 0.16);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.76);
  color: #64748b;
  box-shadow: 0 10px 24px rgba(19, 34, 56, 0.05);
  transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease, box-shadow 0.18s ease;
}

.mode-toggle-pill.active {
  border-color: rgba(31, 111, 235, 0.22);
  background: linear-gradient(135deg, rgba(237, 246, 255, 0.94), rgba(255, 255, 255, 0.9));
  color: #1f6feb;
  box-shadow: 0 12px 26px rgba(31, 111, 235, 0.1);
}

.mode-toggle-label {
  font-size: 13px;
  font-weight: 700;
  white-space: nowrap;
}

.mode-toggle-pill :deep(.el-switch) {
  height: 22px;
}

.mode-toggle-pill :deep(.el-switch__core) {
  width: 42px;
  min-width: 42px;
  height: 22px;
  border: none;
  background: #cbd5e1;
}

.mode-toggle-pill :deep(.el-switch.is-checked .el-switch__core) {
  background: linear-gradient(135deg, #1f6feb, #4aa3ff);
}

.mode-toggle-pill :deep(.el-switch__action) {
  width: 18px;
  height: 18px;
}

.group-tabs-card {
  overflow: hidden;
}

.group-tabs :deep(.el-tabs__header) {
  margin: 0 0 20px;
}

.group-tabs :deep(.el-tabs__nav-wrap::after) {
  display: none;
}

.group-tabs :deep(.el-tabs__nav-scroll) {
  display: flex;
}

.group-tabs :deep(.el-tabs__nav) {
  gap: 6px;
  padding: 5px;
  border: 1px solid rgba(113, 135, 166, 0.16);
  border-radius: 999px;
  background: #f3f7fc;
}

.group-tabs :deep(.el-tabs__active-bar) {
  display: none;
}

.group-tabs :deep(.el-tabs__item) {
  height: 34px;
  padding: 0 18px;
  border-radius: 999px;
  color: #64748b;
  font-weight: 700;
  transition: color 0.18s ease, background 0.18s ease, box-shadow 0.18s ease;
}

.group-tabs :deep(.el-tabs__item.is-active) {
  color: #1f6feb;
  background: #fff;
  box-shadow: 0 8px 20px rgba(31, 111, 235, 0.13);
}

.group-tabs :deep(.el-tabs__item:hover) {
  color: #1f6feb;
}

.group-tabs :deep(#tab-rules) {
  order: 1;
}

.group-tabs :deep(#tab-groups) {
  order: 2;
}

.group-header {
  display: grid;
  gap: 18px;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.search-input {
  width: 300px;
}

.group-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
}

.group-panel {
  border: 1px solid rgba(113, 135, 166, 0.16);
  border-radius: 22px;
  background: linear-gradient(180deg, #fbfdff, #f4f8ff);
  padding: 18px;
}

.group-panel-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 12px;
}

.group-panel-head span {
  font-size: 18px;
  font-weight: 800;
  color: #132238;
}

.group-panel-head small {
  color: #8292a8;
}

.group-card-list {
  display: grid;
  gap: 12px;
}

.group-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid rgba(113, 135, 166, 0.16);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.86);
}

.group-card strong {
  color: #16243a;
  font-size: 15px;
}

.group-card p {
  margin: 6px 0 0;
  color: #7d8ea5;
  font-size: 13px;
}

.group-actions {
  white-space: nowrap;
}

.rule-card {
  overflow: hidden;
}

.table-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.rule-name {
  color: #1f6feb;
  font-weight: 700;
}

.path-line {
  color: #3d516d;
  line-height: 1.7;
}

.danger {
  color: #f56c6c;
  font-weight: 700;
}

.member-dialog-head {
  display: grid;
  gap: 4px;
  margin-bottom: 16px;
}

.member-dialog-head span {
  color: #8292a8;
  font-size: 13px;
}

.member-list {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.member-list :deep(.el-checkbox) {
  margin: 0;
  height: auto;
  padding: 10px 12px;
}

.rule-form {
  --el-border-radius-base: 12px;
}

.form-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) 180px;
  gap: 16px;
}

.limit-tip {
  margin-left: 10px;
  color: #8292a8;
  font-size: 13px;
}

@media (max-width: 1080px) {
  .tabs-card-inner {
    align-items: stretch;
    flex-direction: column;
  }

  .tabs-actions {
    justify-content: flex-start;
  }

  .group-grid,
  .form-grid,
  .member-list {
    grid-template-columns: 1fr;
  }

  .search-input {
    width: 100%;
  }
}
</style>
