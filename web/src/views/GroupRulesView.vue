<template>
  <div class="page-shell group-rules-page">
    <div class="mode-title-action">
      <div
        class="mode-toggle-pill"
        :class="{ active: groupStore.modeEnabled, saving: modeSaving }"
        role="switch"
        :aria-checked="String(groupStore.modeEnabled)"
        :aria-disabled="String(modeSaving)"
        tabindex="0"
        @click="requestModeToggle"
        @keydown.enter.prevent="requestModeToggle"
        @keydown.space.prevent="requestModeToggle"
      >
        <span class="mode-toggle-dot"></span>
        <span class="mode-toggle-label">{{ groupStore.modeEnabled ? '设备组模式运行中' : '设备组模式已停止' }}</span>
        <span class="mode-switch" aria-hidden="true">
          <span class="mode-switch-track">
            <span class="mode-switch-thumb"></span>
          </span>
        </span>
      </div>
    </div>

    <el-card class="panel-card group-tabs-card" shadow="never">
      <div class="group-card-toolbar">
        <div class="group-tab-switch" role="tablist" aria-label="设备组规则视图">
          <button :class="{ active: activeTab === 'rules' }" type="button" @click="activeTab = 'rules'">规则</button>
          <button :class="{ active: activeTab === 'groups' }" type="button" @click="activeTab = 'groups'">分组</button>
        </div>
        <span class="group-tab-summary">{{ tabSummary }}</span>
        <div class="toolbar group-toolbar">
          <el-input v-model="keyword" class="search-input" clearable :placeholder="searchPlaceholder" />
          <el-button :loading="manualRefreshing || groupStore.refreshing" round @click="manualRefresh">
            立即刷新
          </el-button>
          <template v-if="activeTab === 'groups'">
            <el-button type="primary" round :disabled="modeDisabled" @click="openGroupDialog('ingress')">新增入口组</el-button>
            <el-button round :disabled="modeDisabled" @click="openGroupDialog('egress')">新增出口组</el-button>
          </template>
          <template v-else>
            <el-button round :disabled="modeDisabled" @click="openImportDialog">批量导入</el-button>
            <el-button round :disabled="selectedRules.length === 0" @click="openExportDialog">导出选中</el-button>
            <el-button
              round
              type="danger"
              plain
              :disabled="modeDisabled || selectedRules.length === 0"
              :loading="batchDeleting"
              @click="batchDeleteRules"
            >
              删除选中
            </el-button>
            <el-button type="primary" round :disabled="modeDisabled" @click="openRuleDialog()">新增组规则</el-button>
          </template>
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
      <el-table
        ref="ruleTableRef"
        :data="filteredRules"
        row-key="id"
        v-loading="initialLoading"
        empty-text="暂无设备组规则"
        @selection-change="handleSelectionChange"
      >
        <el-table-column type="selection" width="38" />
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

    <el-dialog v-model="importDialog.visible" title="批量导入设备组规则" width="720px" class="batch-dialog">
      <div class="batch-tip">
        每行一条，格式：<code>名称#入口端口#目标地址#目标端口</code>。空行会自动跳过；端口冲突的规则会导入但保持关闭。
      </div>
      <div class="batch-options">
        <el-select v-model="importDefaults.ingress_group_id" placeholder="选择入口组">
          <el-option v-for="group in groupStore.ingressGroups" :key="group.id" :label="group.name" :value="group.id" />
        </el-select>
        <el-select v-model="importDefaults.egress_group_id" placeholder="选择出口组">
          <el-option v-for="group in groupStore.egressGroups" :key="group.id" :label="group.name" :value="group.id" />
        </el-select>
        <el-select v-model="importDefaults.protocol" placeholder="协议" class="protocol-select">
          <el-option label="TCP" value="tcp" />
          <el-option label="UDP" value="udp" />
          <el-option label="TCP+UDP" value="both" />
        </el-select>
        <el-switch v-model="importDefaults.enabled" inline-prompt active-text="启用" inactive-text="关闭" />
      </div>
      <el-input v-model="importText" type="textarea" :rows="12" />
      <template #footer>
        <div class="dialog-footer-actions">
          <el-button @click="importDialog.visible = false">取消</el-button>
          <el-button type="primary" :loading="importing" @click="importRules">开始导入</el-button>
        </div>
      </template>
    </el-dialog>

    <el-dialog v-model="exportDialog.visible" title="导出选中设备组规则" width="680px" class="batch-dialog">
      <div class="batch-tip">
        已选择 {{ selectedRules.length }} 条设备组规则，导出格式与导入格式一致。
      </div>
      <el-input v-model="exportText" type="textarea" :rows="12" readonly />
      <template #footer>
        <div class="dialog-footer-actions">
          <el-button @click="downloadExport">下载 TXT</el-button>
          <el-button type="primary" @click="copyExport">复制内容</el-button>
        </div>
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
          <el-form-item prop="target_addr">
            <template #label>
              <span class="label-with-action">
                <span>目标地址</span>
                <el-button type="primary" link class="detect-link" @click.stop.prevent="detectGroupTarget">
                  识别
                </el-button>
              </span>
            </template>
            <el-input v-model="ruleForm.target_addr" placeholder="支持 IP、域名或代理链接" />
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
import { computed, onMounted, reactive, ref } from 'vue';
import { useRoute } from 'vue-router';
import { ElMessage, ElMessageBox } from 'element-plus';
import { View } from '@element-plus/icons-vue';
import { useGroupRuleStore } from '../stores/groupRule';
import { useMachineStore } from '../stores/machine';
import { formatBytes, protocolLabel } from '../utils/format';
import { parseTarget } from '../utils/targetParser';

const groupStore = useGroupRuleStore();
const machineStore = useMachineStore();
const route = useRoute();
const localDemo = computed(() => import.meta.env.DEV && route.query.localDemo === '1');

const keyword = ref('');
const activeTab = ref('rules');
const initialLoading = ref(false);
const manualRefreshing = ref(false);
const submitting = ref(false);
const modeSaving = ref(false);
const groupFormRef = ref(null);
const ruleFormRef = ref(null);
const ruleTableRef = ref(null);
const memberSelection = ref([]);
const selectedRules = ref([]);
const limitGB = ref(0);
const batchDeleting = ref(false);
const importing = ref(false);
const importText = ref('');
const exportText = ref('');

const groupDialog = reactive({ visible: false, id: '', role: 'ingress' });
const membersDialog = reactive({ visible: false, group: null });
const ruleDialog = reactive({ visible: false, id: '' });
const detailDialog = reactive({ visible: false, rule: null });
const importDialog = reactive({ visible: false });
const exportDialog = reactive({ visible: false });

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
const importDefaults = reactive({
  ingress_group_id: '',
  egress_group_id: '',
  protocol: 'tcp',
  enabled: true,
});

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
const tabSummary = computed(() => {
  const hasKeyword = keyword.value.trim().length > 0;
  if (activeTab.value === 'groups') {
    const ingressTotal = groupStore.ingressGroups.length;
    const egressTotal = groupStore.egressGroups.length;
    const filteredTotal = filteredIngressGroups.value.length + filteredEgressGroups.value.length;
    const total = ingressTotal + egressTotal;
    if (hasKeyword) return `筛选 ${filteredTotal} / ${total} 个分组`;
    return `入口组 ${ingressTotal} · 出口组 ${egressTotal}`;
  }
  if (hasKeyword) return `筛选 ${filteredRules.value.length} / ${groupStore.rules.length} 条规则`;
  return `共 ${groupStore.rules.length} 条规则`;
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
  if (localDemo.value) {
    loadDemoData();
    initialLoading.value = false;
    manualRefreshing.value = false;
    return;
  }
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

function loadDemoData() {
  groupStore.modeEnabled = true;
  groupStore.groups = [
    {
      id: 'demo-ingress-1',
      name: '入口组 广州',
      role: 'ingress',
      machine_count: 2,
      members: [
        {
          id: 'demo-i-1',
          name: '广东测试腾讯200M',
          ip: '42.193.145.61',
          online: true,
          agent_version: 'v1.7.0',
        },
        {
          id: 'demo-i-2',
          name: '腾讯云广州 - 800元',
          ip: '106.53.10.133',
          online: true,
          agent_version: 'v1.7.0',
        },
      ],
    },
    {
      id: 'demo-egress-1',
      name: '出口组 香港',
      role: 'egress',
      machine_count: 2,
      members: [
        {
          id: 'demo-e-1',
          name: 'FRC香港出口机',
          ip: '172.81.104.204',
          online: true,
          agent_version: 'v1.7.0',
        },
        {
          id: 'demo-e-2',
          name: 'IX 103.177.162.211',
          ip: '103.177.162.211',
          tunnel_advertise_addr: '103.177.162.211:19999',
          is_ix: true,
          online: true,
          agent_version: 'v1.7.0',
        },
      ],
    },
  ];
  groupStore.rules = [
    {
      id: 'demo-rule-1',
      name: 'joe-01',
      ingress_group_id: 'demo-ingress-1',
      ingress_group_name: '入口组 广州',
      egress_group_id: 'demo-egress-1',
      egress_group_name: '出口组 香港',
      ingress_port: 53328,
      target_addr: '61.219.247.68',
      target_port: 51314,
      protocol: 'tcp',
      enabled: true,
      effective_machines: 2,
      ingress_machine_count: 2,
      conflict_machines: 0,
      online_egress_count: 2,
      today_bytes: 12884901888,
      traffic_limit_bytes: 0,
    },
  ];
  machineStore.machines = [...groupStore.groups.flatMap((group) => group.members || [])];
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

function requestModeToggle() {
  if (modeSaving.value) return;
  toggleMode(!groupStore.modeEnabled).catch(() => {});
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

function handleSelectionChange(rows) {
  selectedRules.value = rows;
}

function openImportDialog() {
  if (!ensureModeEnabled()) return;
  if (!importDefaults.ingress_group_id && groupStore.ingressGroups.length === 1) {
    importDefaults.ingress_group_id = groupStore.ingressGroups[0].id;
  }
  if (!importDefaults.egress_group_id && groupStore.egressGroups.length === 1) {
    importDefaults.egress_group_id = groupStore.egressGroups[0].id;
  }
  importDialog.visible = true;
}

function openExportDialog() {
  if (selectedRules.value.length === 0) {
    ElMessage.warning('请先选择要导出的设备组规则');
    return;
  }
  exportText.value = selectedRules.value.map(formatRuleLine).join('\n');
  exportDialog.visible = true;
}

function formatRuleLine(rule) {
  return [rule.name, rule.ingress_port, rule.target_addr, rule.target_port].join('#');
}

function parseImportText(text) {
  const rows = [];
  const errors = [];
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

    rows.push({
      line_no: lineNo,
      name,
      ingress_port: ingressPort,
      target_addr: targetAddr,
      target_port: targetPort,
    });
  });

  return { rows, errors };
}

function ensureImportDefaults() {
  if (!importDefaults.ingress_group_id) {
    ElMessage.warning('请选择入口组');
    return false;
  }
  if (!importDefaults.egress_group_id) {
    ElMessage.warning('请选择出口组');
    return false;
  }
  if (!importDefaults.protocol) {
    ElMessage.warning('请选择协议');
    return false;
  }
  return true;
}

function describeConflictRule(rule) {
  if (!rule) {
    return '未知规则';
  }
  return `${rule.name || rule.id}（端口 ${rule.ingress_port}，${protocolLabel(rule.protocol)}）`;
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
    ElMessage.warning('没有可导入的设备组规则');
    return;
  }

  importing.value = true;
  let created = 0;
  const disabledConflicts = [];
  try {
    for (const row of rows) {
      const data = await groupStore.createRule({
        ...row,
        ingress_group_id: importDefaults.ingress_group_id,
        egress_group_id: importDefaults.egress_group_id,
        protocol: importDefaults.protocol,
        enabled: importDefaults.enabled,
        traffic_limit_bytes: 0,
        remark: '',
      }, { refresh: false });
      created += 1;
      if (data?.conflict_rule) {
        disabledConflicts.push({
          lineNo: row.line_no,
          name: row.name,
          conflict: data.conflict_rule,
        });
      }
    }

    if (disabledConflicts.length > 0) {
      ElMessage.warning(`已导入 ${created} 条，其中 ${disabledConflicts.length} 条因端口占用保持关闭`);
      try {
        await ElMessageBox.alert(
          disabledConflicts
            .slice(0, 20)
            .map((item) => `第 ${item.lineNo} 行 ${item.name}：被 ${describeConflictRule(item.conflict)} 占用`)
            .join('\n'),
          '端口占用提示',
          {
            confirmButtonText: '知道了',
            customClass: 'import-conflict-alert',
          },
        );
      } catch {
        // Closing the notice does not mean the import failed.
      }
    } else {
      ElMessage.success(`已导入 ${created} 条设备组规则`);
    }
    importDialog.visible = false;
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
  link.download = `onlytun-group-rules-${new Date().toISOString().slice(0, 10)}.txt`;
  link.click();
  URL.revokeObjectURL(url);
}

function openRuleDetail(rule) {
  detailDialog.rule = rule;
  detailDialog.visible = true;
}

function detectGroupTarget() {
  const raw = ruleForm.target_addr.trim();
  if (!raw) {
    ElMessage.warning('请先粘贴代理连接或输入目标地址');
    return;
  }

  const parsed = parseTarget(raw);
  if (!parsed.ok) {
    ElMessage.warning(parsed.message);
    return;
  }

  ruleForm.target_addr = parsed.host;
  ruleForm.target_port = parsed.port;
  ElMessage.success(`已识别：${parsed.host}:${parsed.port}`);
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
    await loadData();
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
    await loadData();
  } catch (error) {
    const conflict = error.response?.data?.conflict_rule;
    if (conflict) {
      ElMessage.warning(`无法启用：被 ${describeConflictRule(conflict)} 占用`);
    }
    rule.enabled = !rule.enabled;
  }
}

async function deleteRule(rule) {
  if (!ensureModeEnabled()) return;
  await ElMessageBox.confirm(`确定删除组规则「${rule.name}」吗？`, '删除规则', { type: 'warning' });
  await groupStore.deleteRule(rule.id);
  ElMessage.success('设备组规则已删除');
  await loadData();
}

async function batchDeleteRules() {
  if (!ensureModeEnabled()) return;
  const rules = [...selectedRules.value];
  if (rules.length === 0) {
    ElMessage.warning('请先选择要删除的设备组规则');
    return;
  }

  const preview = rules
    .slice(0, 6)
    .map((rule) => `“${rule.name}”`)
    .join('、');
  const suffix = rules.length > 6 ? ` 等 ${rules.length} 条设备组规则` : '';
  await ElMessageBox.confirm(
    `确定删除 ${rules.length} 条设备组规则吗？${preview}${suffix} 删除后不可恢复。`,
    '批量删除确认',
    {
      type: 'warning',
      confirmButtonText: '批量删除',
      cancelButtonText: '取消',
    },
  );

  batchDeleting.value = true;
  try {
    await groupStore.deleteRules(rules.map((rule) => rule.id));
    selectedRules.value = [];
    ruleTableRef.value?.clearSelection?.();
    ElMessage.success(`已删除 ${rules.length} 条设备组规则`);
    await loadData();
  } finally {
    batchDeleting.value = false;
  }
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
  gap: 12px;
  box-sizing: border-box;
  min-height: 43px;
  padding: 10px 16px;
  border: 1px solid rgba(59, 130, 246, 0.2);
  border-radius: 999px;
  background:
    linear-gradient(135deg, #e8f4ff 0%, #dbeeff 100%);
  color: #1d4ed8;
  cursor: pointer;
  user-select: none;
  box-shadow:
    0 2px 12px rgba(59, 130, 246, 0.15),
    inset 0 1px 0 rgba(255, 255, 255, 0.8);
  transition:
    border-color 0.3s ease,
    background 0.3s ease,
    color 0.3s ease,
    box-shadow 0.3s ease,
    transform 0.3s ease;
}

.mode-toggle-pill.active {
  border-color: rgba(59, 130, 246, 0.2);
  background:
    linear-gradient(135deg, #e8f4ff 0%, #dbeeff 100%);
  color: #1d4ed8;
  box-shadow:
    0 2px 12px rgba(59, 130, 246, 0.15),
    inset 0 1px 0 rgba(255, 255, 255, 0.8);
}

.mode-toggle-pill:not(.active) {
  border-color: rgba(0, 0, 0, 0.08);
  background:
    linear-gradient(135deg, #f0f0f4 0%, #e8e8ef 100%);
  color: #6b7280;
  box-shadow:
    0 2px 8px rgba(0, 0, 0, 0.06),
    inset 0 1px 0 rgba(255, 255, 255, 0.8);
}

.mode-toggle-pill:hover {
  transform: none;
}

.mode-toggle-pill:focus-visible {
  outline: 3px solid rgba(64, 158, 255, 0.22);
  outline-offset: 3px;
}

.mode-toggle-label {
  font-size: 14px;
  font-weight: 500;
  line-height: 21px;
  letter-spacing: 0.01em;
  white-space: nowrap;
  transition: color 0.3s ease;
}

.mode-toggle-dot {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: #9ca3af;
  box-shadow: none;
  transform: scale(1);
  animation: none;
}

.mode-toggle-pill.active .mode-toggle-dot {
  background: #2563eb;
  animation: modeDotScale 0.4s ease-in-out;
}

.mode-toggle-pill.active .mode-toggle-dot::after {
  position: absolute;
  inset: 0;
  border-radius: 999px;
  background: #3b82f6;
  animation: modePulse 1.2s ease-out infinite;
  content: '';
}

.mode-switch {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex: 0 0 auto;
  margin-left: 4px;
  padding: 0;
}

.mode-switch-track {
  position: relative;
  display: block;
  width: 40px;
  height: 22px;
  border-radius: 11px;
  background:
    #cbced4;
  box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.15);
  transition:
    background 0.3s ease,
    box-shadow 0.3s ease;
}

.mode-toggle-pill.active .mode-switch-track {
  background:
    linear-gradient(135deg, #3b82f6, #2563eb);
  box-shadow:
    0 2px 6px rgba(37, 99, 235, 0.4),
    inset 0 1px 0 rgba(255, 255, 255, 0.2);
}

.mode-switch-thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 18px;
  height: 18px;
  border-radius: 999px;
  background: #ffffff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.15);
  transform: translateX(0);
  transition:
    transform 0.34s cubic-bezier(0.22, 1.12, 0.36, 1),
    box-shadow 0.3s ease;
}

.mode-toggle-pill.active .mode-switch-thumb {
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.2);
  transform: translateX(17px);
}

.mode-toggle-pill:active .mode-switch-thumb {
  transform: translateX(0) scale(0.96);
}

.mode-toggle-pill.active:active .mode-switch-thumb {
  transform: translateX(17px) scale(0.96);
}

.mode-toggle-pill.saving {
  pointer-events: none;
  opacity: 0.78;
}

.mode-toggle-pill.saving .mode-switch-thumb {
  animation: modeThumbBusy 0.8s ease-in-out infinite;
}

@keyframes modeThumbBusy {
  0%,
  100% {
    filter: brightness(1);
  }

  50% {
    filter: brightness(0.92);
  }
}

@keyframes modePulse {
  0% {
    opacity: 0.6;
    transform: scale(1);
  }

  100% {
    opacity: 0;
    transform: scale(2.5);
  }
}

@keyframes modeDotScale {
  0%,
  100% {
    transform: scale(1);
  }

  50% {
    transform: scale(1.2);
  }
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

.group-tab-summary {
  display: inline-flex;
  align-items: center;
  align-self: flex-start;
  min-height: 30px;
  padding: 0 12px;
  border: 1px solid rgba(113, 135, 166, 0.14);
  border-radius: 999px;
  background: rgba(248, 251, 255, 0.86);
  color: #64748b;
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
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

.label-with-action {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.detect-link {
  padding: 0 2px;
  height: auto;
  font-weight: 600;
  vertical-align: baseline;
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
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) 130px 94px;
  gap: 12px;
  align-items: center;
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

:global(.import-conflict-alert .el-message-box__message) {
  white-space: pre-line;
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
  .member-list,
  .batch-options {
    grid-template-columns: 1fr;
  }

  .search-input {
    width: 100%;
  }
}
</style>
