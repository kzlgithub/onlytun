<template>
  <div class="page-shell group-rules-page">
    <div class="mode-title-action">
      <div class="mode-toggle-pill" :class="{ active: groupStore.modeEnabled }">
        <span class="mode-toggle-label">设备组模式</span>
        <el-switch
          :model-value="groupStore.modeEnabled"
          :loading="modeSaving"
          @change="toggleMode"
        />
        <el-tag :type="groupStore.modeEnabled ? 'primary' : 'info'" effect="light" round>
          {{ groupStore.modeEnabled ? '已开启' : '已关闭' }}
        </el-tag>
      </div>
    </div>

    <el-card class="panel-card group-tabs-card" shadow="never">
      <div class="group-card-toolbar">
        <div class="group-tab-switch" role="tablist" aria-label="设备组规则视图">
          <button :class="{ active: activeTab === 'rules' }" type="button" @click="activeTab = 'rules'">规则</button>
          <button :class="{ active: activeTab === 'groups' }" type="button" @click="activeTab = 'groups'">分组</button>
        </div>
        <div class="toolbar group-toolbar">
          <el-input v-model="keyword" class="search-input" clearable :placeholder="searchPlaceholder" />
          <el-button :loading="manualRefreshing || groupStore.refreshing" round @click="manualRefresh">
            刷新
          </el-button>
          <template v-if="activeTab === 'groups'">
            <el-button type="primary" round :disabled="modeDisabled" @click="openGroupDialog('ingress')">新增入口组</el-button>
            <el-button round :disabled="modeDisabled" @click="openGroupDialog('egress')">新增出口组</el-button>
          </template>
          <el-button v-else type="primary" round :disabled="modeDisabled" @click="openRuleDialog()">新增组规则</el-button>
        </div>
      </div>

      <div v-if="activeTab === 'groups'" class="tab-content">
        <div class="section-head">
          <div>
            <h3 class="section-title">设备分组</h3>
            <p class="section-meta">
              {{ groupStore.ingressGroups.length }} 个入口组 · {{ groupStore.egressGroups.length }} 个出口组 ·
              每台入口机只属于一个入口组
            </p>
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
              <div class="group-card-main">
                <div class="group-card-title-row">
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
                <div class="member-preview-list">
                  <div v-for="machine in group.members || []" :key="machine.id" class="member-preview-item" :class="{ offline: !machine.online }">
                    <span class="member-status-dot" :class="{ online: machine.online }"></span>
                    <div class="member-preview-text">
                      <strong>{{ machine.name || machine.ip || machine.id }}</strong>
                      <span>{{ memberSummary(machine) }}</span>
                    </div>
                  </div>
                  <div v-if="!(group.members || []).length" class="member-preview-empty">暂无成员，点击“成员”添加入口机</div>
                </div>
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
              <div class="group-card-main">
                <div class="group-card-title-row">
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
                <div class="member-preview-list">
                  <div v-for="machine in group.members || []" :key="machine.id" class="member-preview-item" :class="{ offline: !machine.online }">
                    <span class="member-status-dot" :class="{ online: machine.online }"></span>
                    <div class="member-preview-text">
                      <strong>{{ machine.name || machine.ip || machine.id }}</strong>
                      <span>{{ memberSummary(machine) }}</span>
                    </div>
                  </div>
                  <div v-if="!(group.members || []).length" class="member-preview-empty">暂无成员，点击“成员”添加出口机</div>
                </div>
              </div>
            </div>
            <el-empty v-if="filteredEgressGroups.length === 0" description="暂无出口组" />
          </div>
        </section>
      </div>
      </div>

      <div v-else class="tab-content">
        <div class="section-head">
          <div>
            <h3 class="section-title">设备组转发规则</h3>
            <p class="section-meta">
              {{ groupStore.modeEnabled ? '设备组模式开启后，仅设备组规则会下发到 Agent。' : '当前设备组模式关闭，组规则暂不下发。' }}
            </p>
          </div>
        </div>

      <el-table :data="filteredRules" row-key="id" v-loading="initialLoading" empty-text="暂无设备组规则">
        <el-table-column label="规则名称" min-width="160">
          <template #default="{ row }">
            <span class="rule-name">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column label="路径" min-width="360">
          <template #default="{ row }">
            <div class="path-line">
              <span class="path-text">
                {{ row.ingress_group_name || row.ingress_group_id }}:{{ row.ingress_port }}
                →
                {{ row.egress_group_name || row.egress_group_id }}
                →
                {{ row.target_addr }}:{{ row.target_port }}
              </span>
              <el-tooltip content="查看路径详情" placement="top">
                <el-button class="path-detail-button" type="primary" text circle @click="openRuleDetail(row)">
                  <el-icon><View /></el-icon>
                </el-button>
              </el-tooltip>
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
      </div>
    </el-card>

    <el-dialog v-model="detailDialog.visible" title="路径详情" width="880px" class="route-detail-dialog">
      <div v-if="detailRule" class="route-detail">
        <div class="route-detail-summary">
          <div>
            <span>规则</span>
            <strong>{{ detailRule.name }}</strong>
          </div>
          <div>
            <span>入口端口</span>
            <strong>{{ detailRule.ingress_port }}</strong>
          </div>
          <div>
            <span>目标</span>
            <strong>{{ detailRule.target_addr }}:{{ detailRule.target_port }}</strong>
          </div>
        </div>

        <div class="route-map-head">
          <div>
            <span>入口组</span>
            <strong>{{ detailIngressGroup?.name || detailRule.ingress_group_name || '入口组' }}</strong>
          </div>
          <em>按入口机稳定映射出口机</em>
          <div>
            <span>出口组</span>
            <strong>{{ detailEgressGroup?.name || detailRule.egress_group_name || '出口组' }}</strong>
          </div>
        </div>

        <div class="route-map-list">
          <div v-for="route in detailRouteMappings" :key="route.ingress.id" class="route-map-card">
            <div class="route-map-node" :class="{ offline: !route.ingress.online }">
              <span class="member-status-dot" :class="{ online: route.ingress.online }"></span>
              <div>
                <small>入口机</small>
                <strong>{{ route.ingress.name || route.ingress.ip || route.ingress.id }}</strong>
                <span>{{ route.ingress.ip || '未上报IP' }}:{{ detailRule.ingress_port }}</span>
              </div>
              <el-tag :type="route.ingress.online ? 'success' : 'info'" effect="light" round>
                {{ route.ingress.online ? '在线' : '离线' }}
              </el-tag>
            </div>

            <div class="route-map-arrow">→</div>

            <div v-if="route.egress" class="route-map-node" :class="{ offline: !route.egress.online }">
              <span class="member-status-dot" :class="{ online: route.egress.online }"></span>
              <div>
                <small>出口机</small>
                <strong>{{ route.egress.name || route.egress.ip || route.egress.id }}</strong>
                <span>{{ route.egress.tunnel_advertise_addr || route.egress.ip || '未上报接入地址' }}</span>
              </div>
              <el-tag v-if="route.egress.is_ix" type="warning" effect="light" round>IX</el-tag>
              <el-tag :type="route.egress.online ? 'success' : 'info'" effect="light" round>
                {{ route.egress.online ? '在线' : '离线' }}
              </el-tag>
            </div>
            <div v-else class="route-map-node offline">
              <span class="member-status-dot"></span>
              <div>
                <strong>无可用出口</strong>
                <span>出口组没有在线或可连接地址</span>
              </div>
            </div>
          </div>
          <div v-if="detailRouteMappings.length === 0" class="route-node-empty">该入口组暂无成员</div>
        </div>
      </div>
    </el-dialog>

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
import { View } from '@element-plus/icons-vue';
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
const detailDialog = reactive({ visible: false, rule: null });

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
const searchPlaceholder = computed(() =>
  activeTab.value === 'groups' ? '搜索分组、规则、目标地址' : '搜索规则、分组、目标地址',
);

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
const detailRule = computed(() => detailDialog.rule);
const detailIngressGroup = computed(() => findGroupById(detailRule.value?.ingress_group_id));
const detailEgressGroup = computed(() => findGroupById(detailRule.value?.egress_group_id));
const detailIngressMembers = computed(() => detailIngressGroup.value?.members || []);
const detailEgressMembers = computed(() => detailEgressGroup.value?.members || []);
const detailOnlineEgressMembers = computed(() =>
  detailEgressMembers.value.filter((machine) => machine.online && (machine.ip || machine.tunnel_advertise_addr)),
);
const detailRouteMappings = computed(() => {
  const rule = detailRule.value;
  if (!rule) return [];
  return detailIngressMembers.value.map((ingress) => ({
    ingress,
    egress: pickMappedEgress(rule, ingress),
  }));
});

const memberOptions = computed(() => {
  const role = membersDialog.group?.role;
  if (!role) return [];
  return machineStore.machines.filter((machine) => machine.role === role);
});

function findGroupById(id) {
  if (!id) return null;
  return groupStore.groups.find((group) => group.id === id) || null;
}

function pickMappedEgress(rule, ingress) {
  const egresses = detailOnlineEgressMembers.value;
  if (!rule?.id || !ingress?.id || egresses.length === 0) return null;
  const hash = fnv32a(`${ingress.id}:${rule.id}`);
  return egresses[hash % egresses.length];
}

function fnv32a(value) {
  let hash = 0x811c9dc5;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 0x01000193) >>> 0;
  }
  return hash >>> 0;
}

function filterGroups(groups) {
  const q = keyword.value.trim().toLowerCase();
  if (!q) return groups;
  return groups.filter((group) => [group.name, group.remark, group.role].filter(Boolean).join(' ').toLowerCase().includes(q));
}

function memberSummary(machine) {
  const parts = [];
  if (machine.is_ix) {
    parts.push('IX');
  }
  parts.push(machine.ip || '未上报 IP');
  if (machine.tunnel_advertise_addr) {
    parts.push(machine.tunnel_advertise_addr);
  }
  parts.push(machine.agent_version || 'unknown');
  return parts.join(' · ');
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

function openRuleDetail(rule) {
  detailDialog.rule = rule;
  detailDialog.visible = true;
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
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.mode-title-action {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex: 0 0 auto;
  min-height: 36px;
  margin-top: -62px;
  margin-bottom: 26px;
}

.mode-toggle-pill {
  display: inline-flex;
  align-items: center;
  gap: 9px;
  box-sizing: border-box;
  min-height: 34px;
  padding: 5px 8px 5px 13px;
  border: 1px solid rgba(113, 135, 166, 0.16);
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.86);
  color: #64748b;
  box-shadow: 0 10px 24px rgba(19, 34, 56, 0.06);
  transition: border-color 0.18s ease, background 0.18s ease, color 0.18s ease, box-shadow 0.18s ease;
}

.mode-toggle-pill.active {
  border-color: rgba(31, 111, 235, 0.22);
  background: linear-gradient(135deg, rgba(237, 246, 255, 0.94), rgba(255, 255, 255, 0.9));
  color: #1f6feb;
  box-shadow: 0 12px 26px rgba(31, 111, 235, 0.1);
}

.mode-toggle-pill :deep(.el-tag) {
  height: 22px;
  padding: 0 9px;
  border: none;
  font-weight: 700;
}

.mode-toggle-label {
  font-size: 13px;
  font-weight: 700;
  line-height: 20px;
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

.group-card-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 18px;
  border-bottom: 1px solid rgba(113, 135, 166, 0.12);
}

.group-tab-switch {
  display: inline-flex;
  gap: 6px;
  padding: 5px;
  border: 1px solid rgba(113, 135, 166, 0.16);
  border-radius: 999px;
  background: #f3f7fc;
}

.group-tab-switch button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 34px;
  line-height: 34px;
  padding: 0 18px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: #64748b;
  font-weight: 700;
  cursor: pointer;
  transition: color 0.18s ease, background 0.18s ease, box-shadow 0.18s ease;
}

.group-tab-switch button.active {
  color: #1f6feb;
  background: #fff;
  box-shadow: 0 8px 20px rgba(31, 111, 235, 0.13);
}

.group-tab-switch button:hover {
  color: #1f6feb;
}

.group-toolbar {
  justify-content: flex-end;
}

.tab-content {
  padding-top: 22px;
}

.section-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
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
  padding: 14px 16px;
  border: 1px solid rgba(113, 135, 166, 0.16);
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.86);
}

.group-card-main {
  display: grid;
  gap: 12px;
}

.group-card-title-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
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

.member-preview-list {
  display: grid;
  gap: 8px;
}

.member-preview-item {
  display: flex;
  align-items: center;
  gap: 9px;
  min-width: 0;
  padding: 8px 10px;
  border: 1px solid rgba(113, 135, 166, 0.12);
  border-radius: 14px;
  background: rgba(247, 251, 255, 0.9);
}

.member-preview-item.offline {
  opacity: 0.62;
  background: rgba(241, 245, 249, 0.82);
}

.member-status-dot {
  flex: 0 0 auto;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #cbd5e1;
  box-shadow: 0 0 0 4px rgba(148, 163, 184, 0.12);
}

.member-status-dot.online {
  background: #45c66f;
  box-shadow: 0 0 0 4px rgba(69, 198, 111, 0.14);
}

.member-preview-text {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}

.member-preview-text strong {
  overflow: hidden;
  max-width: 160px;
  color: #17243a;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.member-preview-text span {
  overflow: hidden;
  color: #75869d;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.member-preview-empty {
  padding: 9px 10px;
  border: 1px dashed rgba(113, 135, 166, 0.22);
  border-radius: 14px;
  color: #8292a8;
  font-size: 12px;
  background: rgba(255, 255, 255, 0.52);
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
  align-items: center;
  color: #3d516d;
  display: flex;
  gap: 10px;
  line-height: 1.7;
  min-width: 0;
}

.path-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.path-detail-button {
  flex: 0 0 auto;
  width: 26px;
  height: 26px;
  padding: 0;
  color: #409eff;
  background: rgba(64, 158, 255, 0.08);
}

.path-detail-button:hover {
  background: rgba(64, 158, 255, 0.16);
}

.route-detail {
  display: grid;
  gap: 20px;
}

.route-detail-summary {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) 150px minmax(0, 1.35fr);
  gap: 14px;
}

.route-detail-summary div {
  display: grid;
  gap: 6px;
  min-width: 0;
  padding: 16px 18px;
  border: 1px solid rgba(113, 135, 166, 0.14);
  border-radius: 18px;
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(244, 249, 255, 0.92)),
    radial-gradient(circle at 100% 0%, rgba(64, 158, 255, 0.1), transparent 34%);
  box-shadow: 0 10px 28px rgba(18, 42, 76, 0.05);
}

.route-detail-summary span,
.route-detail-panel-head span {
  color: #8292a8;
  font-size: 12px;
}

.route-detail-summary strong,
.route-detail-panel-head strong {
  overflow: hidden;
  color: #132238;
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.route-map-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  padding: 0 4px;
}

.route-map-head div {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.route-map-head em {
  color: #8ca0b8;
  font-size: 12px;
  font-style: normal;
}

.route-map-list {
  display: grid;
  gap: 12px;
}

.route-map-card {
  align-items: center;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 28px minmax(0, 1fr);
  gap: 10px;
  padding: 13px;
  border: 1px solid rgba(113, 135, 166, 0.14);
  border-radius: 20px;
  background: linear-gradient(180deg, rgba(251, 253, 255, 0.96), rgba(244, 248, 255, 0.94));
}

.route-map-node {
  align-items: center;
  display: flex;
  gap: 9px;
  min-width: 0;
  min-height: 58px;
  padding: 10px 12px;
  border: 1px solid rgba(113, 135, 166, 0.12);
  border-radius: 16px;
  background: #fff;
}

.route-map-node.offline {
  opacity: 0.58;
  background: #f1f5f9;
}

.route-map-node div {
  min-width: 0;
}

.route-map-node strong,
.route-map-node span {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.route-map-node small {
  display: block;
  margin-bottom: 2px;
  color: #9aa9bc;
  font-size: 11px;
  font-weight: 700;
}

.route-map-node strong {
  color: #17243a;
  font-size: 13px;
}

.route-map-node span {
  color: #75869d;
  font-size: 12px;
}

.route-node-empty {
  padding: 14px;
  border: 1px dashed rgba(113, 135, 166, 0.22);
  border-radius: 14px;
  color: #8292a8;
  background: rgba(255, 255, 255, 0.62);
}

.route-map-arrow {
  align-self: center;
  color: #7ea8dd;
  font-size: 22px;
  font-weight: 800;
  text-align: center;
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
  .group-card-toolbar,
  .section-head {
    align-items: stretch;
    flex-direction: column;
  }

  .group-toolbar {
    justify-content: flex-start;
  }

  .mode-toggle-pill {
    align-self: flex-end;
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
