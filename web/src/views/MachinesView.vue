<template>
  <div class="page-shell machines-page">
    <el-card class="panel-card" shadow="never">
      <template #header>
        <div class="page-header machines-header">
          <div>
            <h3 class="section-title">隧道机</h3>
            <p class="section-meta">
              {{ onlineCount }}/{{ displayMachines.length }} 在线
              <span v-if="demoMode || localDemoMode"> · 演示卡片</span>
              <span v-if="localDemoMode"> · 本地可操作</span>
            </p>
          </div>
          <div class="toolbar">
            <el-input
              v-model="keyword"
              class="search-input"
              clearable
              placeholder="搜索名称、IP、角色"
            />
            <el-button @click="openInstallDialog('ingress')">入口机安装命令</el-button>
            <el-button type="primary" plain @click="openInstallDialog('egress')">
              出口机安装命令
            </el-button>
            <el-button v-if="localDemoMode" type="success" plain @click="addDemoMachine('ingress')">
              新增入口演示
            </el-button>
            <el-button v-if="localDemoMode" type="success" plain @click="addDemoMachine('egress')">
              新增出口演示
            </el-button>
            <el-button v-if="localDemoMode" plain @click="resetDemoMachines">重置演示</el-button>
            <el-button :loading="manualRefreshing" round @click="manualRefresh">立即刷新</el-button>
          </div>
        </div>
      </template>

      <div v-loading="initialLoading" class="machine-groups">
        <MachineGroup
          title="入口机"
          role="ingress"
          :machines="filteredIngressMachines"
          :rule-store="ruleStore"
          @copy-ip="copyIP"
          @rename="handleRename"
          @update-script="handleUpdateScript"
          @delete="handleDelete"
        />

        <MachineGroup
          title="出口机"
          role="egress"
          :machines="filteredEgressMachines"
          :rule-store="ruleStore"
          @copy-ip="copyIP"
          @rename="handleRename"
          @update-script="handleUpdateScript"
          @delete="handleDelete"
        />
      </div>
    </el-card>

    <el-dialog
      v-model="installDialog.visible"
      :title="installDialog.role === 'egress' ? '出口机安装命令' : '入口机安装命令'"
      width="720px"
      class="install-dialog"
    >
      <div class="install-dialog-body">
        <p class="dialog-tip">复制下面这一整行命令到服务器执行即可。</p>
        <div class="code-block command-modal-block">
          <pre>{{ currentInstallCommand || '暂无安装命令' }}</pre>
        </div>
      </div>
      <template #footer>
        <el-button @click="installDialog.visible = false">关闭</el-button>
        <el-button type="primary" :disabled="!currentInstallCommand" @click="copyInstallCommand">
          复制命令
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, defineComponent, h, onBeforeUnmount, onMounted, reactive, ref } from 'vue';
import {
  ElButton,
  ElDropdown,
  ElDropdownItem,
  ElDropdownMenu,
  ElMessage,
  ElMessageBox,
  ElProgress,
  ElTag,
  ElTooltip,
} from 'element-plus';
import { ArrowDown, CopyDocument, RefreshRight } from '@element-plus/icons-vue';
import { useMachineStore } from '../stores/machine';
import { useRuleStore } from '../stores/rule';
import { formatBytes, formatSpeed, roleLabel } from '../utils/format';

const machineStore = useMachineStore();
const ruleStore = useRuleStore();

const keyword = ref('');
const initialLoading = ref(false);
const manualRefreshing = ref(false);
const refreshing = ref(false);
const nowTick = ref(Date.now());
const installDialog = reactive({
  visible: false,
  role: 'ingress',
});

let timer;
let loadingPromise = null;
let lastTotalsAt = 0;
let lastRulesAt = 0;

const demoMode = new URLSearchParams(window.location.search).get('demo') === '1';
const localDemoMode = new URLSearchParams(window.location.search).get('localDemo') === '1';
const demoMachines = ref(loadDemoMachines());
const displayMachines = computed(() => {
  if (localDemoMode) {
    return demoMachines.value;
  }
  return demoMode ? [...machineStore.machines, ...demoMachines.value] : machineStore.machines;
});
const onlineCount = computed(() => displayMachines.value.filter((item) => item.online).length);

const filteredMachines = computed(() => {
  const q = keyword.value.trim().toLowerCase();
  if (!q) {
    return displayMachines.value;
  }
  return displayMachines.value.filter((item) => {
    const haystack = [
      item.name,
      item.ip,
      item.os,
      item.role,
      roleLabel(item.role),
      item.id,
    ]
      .filter(Boolean)
      .join(' ')
      .toLowerCase();
    return haystack.includes(q);
  });
});

const filteredIngressMachines = computed(() => filteredMachines.value.filter((item) => item.role === 'ingress'));
const filteredEgressMachines = computed(() => filteredMachines.value.filter((item) => item.role === 'egress'));
const currentInstallCommand = computed(() => machineStore.installCommands[installDialog.role] || '');

async function loadData(options = {}) {
  if (localDemoMode) {
    initialLoading.value = false;
    manualRefreshing.value = false;
    refreshing.value = false;
    return Promise.resolve();
  }

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

  const now = Date.now();
  const includeRules = initial || manual || now - lastRulesAt >= 3000;
  const includeDayTotals = initial || manual || now - lastTotalsAt > 30000;
  const tasks = [machineStore.fetchMachines()];
  if (includeRules) {
    tasks.push(ruleStore.fetchRules({ includeDayTotals }));
  }
  loadingPromise = Promise.all(tasks);

  try {
    await loadingPromise;
    if (includeRules) {
      lastRulesAt = Date.now();
    }
    if (includeDayTotals) {
      lastTotalsAt = Date.now();
    }
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

async function openInstallDialog(role) {
  installDialog.role = role;
  installDialog.visible = true;
  if (localDemoMode) {
    machineStore.installCommands[role] =
      `bash <(curl -fsSL https://raw.githubusercontent.com/kzlgithub/onlytun/main/scripts/install.sh) --token 'demo-token-${role}' --role ${role} --panel 'http://127.0.0.1:8080'`;
    return;
  }
  if (!machineStore.installCommands[role]) {
    await machineStore.fetchInstallCommands();
  }
}

async function copyInstallCommand() {
  await copyText(currentInstallCommand.value, '安装命令已复制');
}

async function copyIP(machine) {
  await copyText(machine.ip || '', 'IP 已复制');
}

async function copyText(text, successText) {
  if (!text) {
    ElMessage.warning('当前没有可复制的内容');
    return;
  }
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
    } else {
      const textarea = document.createElement('textarea');
      textarea.value = text;
      textarea.setAttribute('readonly', '');
      textarea.style.position = 'fixed';
      textarea.style.top = '-9999px';
      document.body.appendChild(textarea);
      textarea.select();
      const ok = document.execCommand('copy');
      document.body.removeChild(textarea);
      if (!ok) {
        throw new Error('copy failed');
      }
    }
    ElMessage.success(successText);
  } catch {
    ElMessage.error('复制失败，请手动复制');
  }
}

async function handleRename(machine) {
  if (localDemoMode && machine.fake) {
    const { value } = await ElMessageBox.prompt('请输入新的机器名称', '修改演示名称', {
      confirmButtonText: '保存',
      cancelButtonText: '取消',
      inputValue: machine.name,
      inputValidator: (value) => Boolean(value && value.trim()),
      inputErrorMessage: '机器名称不能为空',
    });
    updateDemoMachine(machine.id, { name: value.trim() });
    ElMessage.success('演示名称已更新');
    return;
  }

  if (machine.fake) {
    ElMessage.info('演示卡片不支持改名');
    return;
  }

  const { value } = await ElMessageBox.prompt('请输入新的机器名称', '修改名称', {
    confirmButtonText: '保存',
    cancelButtonText: '取消',
    inputValue: machine.name,
    inputValidator: (value) => Boolean(value && value.trim()),
    inputErrorMessage: '机器名称不能为空',
  });

  await machineStore.updateMachine(machine.id, { name: value.trim() });
  ElMessage.success('名称已更新');
}

async function handleUpdateScript(machine) {
  if (localDemoMode && machine.fake) {
    const taskId = `demo-update-${Date.now()}`;
    updateDemoMachine(machine.id, {
      last_update_task: {
        id: taskId,
        status: 'pending',
        kind: 'agent',
        requested_at: new Date().toISOString(),
      },
    });
    window.setTimeout(() => {
      updateDemoMachine(machine.id, {
        last_update_task: {
          id: taskId,
          status: 'running',
          kind: 'agent',
          requested_at: new Date().toISOString(),
          started_at: new Date().toISOString(),
        },
      });
    }, 1600);
    window.setTimeout(() => {
      updateDemoMachine(machine.id, {
        last_update_task: {
          id: taskId,
          status: 'success',
          kind: 'agent',
          requested_at: new Date().toISOString(),
          finished_at: new Date().toISOString(),
        },
        agent_version: 'v1.2.5',
      });
      ElMessage.success('演示更新已完成');
    }, 5200);
    ElMessage.success('演示更新任务已下发');
    return;
  }

  if (machine.fake) {
    ElMessage.info('演示卡片不支持下发更新');
    return;
  }

  await ElMessageBox.confirm(
    `确定要更新隧道机“${machine.name}”吗？该机器上的 Agent 会短暂重启。`,
    '更新脚本',
    {
      type: 'warning',
      confirmButtonText: '下发更新',
      cancelButtonText: '取消',
    },
  );

  await machineStore.updateScript(machine.id);
  ElMessage.success('更新任务已下发，Agent 下一次同步配置时会执行');
}

async function handleDelete(machine) {
  if (localDemoMode && machine.fake) {
    await ElMessageBox.confirm(`确定删除演示卡片“${machine.name}”吗？`, '删除演示卡片', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    });
    demoMachines.value = demoMachines.value.filter((item) => item.id !== machine.id);
    saveDemoMachines();
    ElMessage.success('演示卡片已删除');
    return;
  }

  if (machine.fake) {
    ElMessage.info('演示卡片不会写入数据库，无需删除');
    return;
  }

  await ElMessageBox.confirm(
    `确定删除隧道机“${machine.name}”吗？如果仍有关联启用规则，后端会拒绝删除。`,
    '删除确认',
    {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    },
  );

  await machineStore.deleteMachine(machine.id);
  ElMessage.success('删除成功');
  await loadData();
}

onMounted(async () => {
  await loadData({ initial: machineStore.machines.length === 0 });
  if (localDemoMode) {
    machineStore.installCommands.ingress =
      "bash <(curl -fsSL https://raw.githubusercontent.com/kzlgithub/onlytun/main/scripts/install.sh) --token 'demo-token-ingress' --role ingress --panel 'http://127.0.0.1:8080'";
    machineStore.installCommands.egress =
      "bash <(curl -fsSL https://raw.githubusercontent.com/kzlgithub/onlytun/main/scripts/install.sh) --token 'demo-token-egress' --role egress --panel 'http://127.0.0.1:8080'";
  } else {
    await machineStore.fetchInstallCommands();
  }
  timer = window.setInterval(() => {
    nowTick.value = Date.now();
    loadData();
  }, 1000);
});

onBeforeUnmount(() => {
  window.clearInterval(timer);
});

const MachineGroup = defineComponent({
  name: 'MachineGroup',
  props: {
    title: { type: String, required: true },
    role: { type: String, required: true },
    machines: { type: Array, required: true },
    ruleStore: { type: Object, required: true },
  },
  emits: ['copy-ip', 'rename', 'update-script', 'delete'],
  setup(props, { emit }) {
    const renderMachine = (machine) => {
      const updating = isUpdating(machine.last_update_task);
      return h(
        'article',
        {
          class: [
            'machine-card',
            machine.online ? 'is-online' : 'is-offline',
            updating ? 'is-updating' : '',
            'is-expanded',
          ],
        },
        [
          h('div', { class: 'machine-card-top' }, [
            h('div', { class: 'machine-title-wrap' }, [
              h('span', { class: ['machine-state', machine.online ? 'online' : 'offline'] }),
              h('div', { class: 'machine-title-main' }, [
                h('h4', { class: 'machine-name' }, machine.name || '未命名机器'),
                h('div', { class: 'machine-ip-line' }, [
                  h('span', { class: 'machine-ip-text' }, machine.ip || '--'),
                  h(
                    ElTooltip,
                    { content: '复制 IP', placement: 'top' },
                    {
                      default: () =>
                        h(
                          ElButton,
                          {
                            class: 'copy-ip-btn',
                            link: true,
                            icon: CopyDocument,
                            disabled: updating,
                            onClick: () => emit('copy-ip', machine),
                          },
                          () => '',
                        ),
                    },
                  ),
                  h(
                    'span',
                    { class: ['agent-version-inline', hasAgentUpdate(machine) ? 'has-update' : ''] },
                    agentVersionText(machine),
                  ),
                ]),
              ]),
            ]),
            h(ElTag, { type: machine.online ? 'success' : 'danger', effect: 'light', round: true }, () =>
              machine.online ? '在线' : '离线',
            ),
          ]),

          updating
            ? h('div', { class: 'machine-update-state' }, [
                h('div', { class: 'update-title-row' }, [
                  h(RefreshRight, { class: 'update-spin-icon' }),
                  h('strong', updateTaskLabel(machine.last_update_task)),
                ]),
                h('div', { class: 'update-steps' }, [
                  updateStep('下发任务', true),
                  updateStep('Agent 执行', machine.last_update_task?.status === 'running'),
                  updateStep('重启回报', false),
                ]),
              ])
            : h('div', null, [
                h('div', { class: 'compact-metrics' }, [
                  metricPill('上传', machineTraffic(machine).up, 'blue'),
                  metricPill('下载', machineTraffic(machine).down, 'green'),
                ]),

                h('div', { class: 'machine-detail' }, [
                  progressRow('CPU', Number(machine.cpu_percent || 0), 'cpu'),
                  progressRow('内存', Number(machine.mem_percent || 0), 'mem'),
                  progressRow('硬盘', optionalPercent(machine.disk_percent), 'disk'),
                  detailRow('网速', machineSpeed(machine, props.ruleStore)),
                  detailRow('在线时间', machineOnlineTime(machine, nowTick.value)),
                ]),
              ]),

          !updating
            ? h(
                'div',
                { class: 'machine-actions' },
                h(
                  ElDropdown,
                  {
                    trigger: 'click',
                    onCommand: (command) => {
                      if (command === 'rename') emit('rename', machine);
                      if (command === 'update') emit('update-script', machine);
                      if (command === 'delete') emit('delete', machine);
                    },
                  },
                  {
                    default: () =>
                      h(ElButton, { class: 'more-action-btn' }, () => [
                        h('span', '操作'),
                        h(ArrowDown, { class: 'action-arrow' }),
                      ]),
                    dropdown: () =>
                      h(ElDropdownMenu, null, () => [
                        h(ElDropdownItem, { command: 'rename' }, () => '改名'),
                        h(ElDropdownItem, { command: 'update', disabled: !machine.online }, () => '更新脚本'),
                        h(ElDropdownItem, { command: 'delete', class: 'danger-dropdown-item' }, () => '删除'),
                      ]),
                  },
                ),
              )
            : null,
        ],
      );
    };

    return () =>
      h('section', { class: 'machine-group' }, [
        h('div', { class: 'group-title-row' }, [
          h('h3', { class: 'group-title' }, props.title),
          h('span', { class: 'group-count' }, `${props.machines.filter((item) => item.online).length}/${props.machines.length} 在线`),
        ]),
        props.machines.length
          ? h('div', { class: 'machine-card-grid' }, props.machines.map(renderMachine))
          : h('div', { class: 'empty-group' }, `暂无${props.title}`),
      ]);
  },
});

function metricPill(label, value, tone) {
  return h('div', { class: ['metric-pill', tone] }, [
    h('span', { class: 'metric-label' }, label),
    h('strong', value),
  ]);
}

function progressRow(label, value, type) {
  const hasValue = Number.isFinite(value);
  return h('div', { class: 'meter-row' }, [
    h('span', { class: 'meter-label' }, label),
    hasValue
      ? h(ElProgress, {
          percentage: Math.max(0, Math.min(100, Number(value.toFixed(1)))),
          strokeWidth: 10,
          showText: true,
          class: `meter-progress ${type}`,
        })
      : h('span', { class: 'unknown-value' }, '--'),
  ]);
}

function detailRow(label, value) {
  return h('div', { class: 'detail-row' }, [
    h('span', { class: 'detail-label' }, label),
    h('strong', value || '--'),
  ]);
}

function updateStep(label, active) {
  return h('span', { class: ['update-step', active ? 'active' : ''] }, [
    h('span', { class: 'update-step-dot' }),
    h('span', label),
  ]);
}

function isUpdating(task) {
  return task?.status === 'pending' || task?.status === 'running';
}

function hasAgentUpdate(machine) {
  const current = String(machine.agent_version || '').trim();
  const latest = String(machine.agent_latest_version || '').trim();
  return Boolean(current && latest && current !== latest);
}

function agentVersionText(machine) {
  const current = String(machine.agent_version || 'unknown').trim();
  const latest = String(machine.agent_latest_version || '').trim();
  if (latest && current !== latest) {
    return `${current}-->${latest}`;
  }
  return current;
}

function updateTaskLabel(task) {
  if (!task) {
    return '未更新';
  }
  const status = task.status || '';
  if (status === 'pending') return '等待执行';
  if (status === 'running') return '正在更新 agent';
  if (status === 'success') return '已更新';
  if (status === 'failed') return '失败';
  return status || '未知';
}

function optionalPercent(value) {
  const numeric = Number(value);
  return value !== undefined && value !== null && Number.isFinite(numeric) ? numeric : Number.NaN;
}

function relatedRules(machine, ruleStore) {
  return ruleStore.rules.filter((rule) =>
    machine.role === 'ingress'
      ? rule.ingress_machine_id === machine.id
      : rule.egress_machine_id === machine.id,
  );
}

function machineTraffic(machine) {
  if (machine.demo_net) {
    return {
      up: formatBytes(machine.demo_net.up || 0),
      down: formatBytes(machine.demo_net.down || 0),
    };
  }
  const up = Number(machine.net_bytes_up || 0);
  const down = Number(machine.net_bytes_down || 0);
  return {
    up: formatBytes(up),
    down: formatBytes(down),
  };
}

function machineSpeed(machine, ruleStore) {
  if (machine.demo_speed) {
    return `↑ ${formatSpeed(machine.demo_speed.up || 0)}  ↓ ${formatSpeed(machine.demo_speed.down || 0)}`;
  }

  const speed = relatedRules(machine, ruleStore).reduce(
    (sum, rule) => {
      const rate = ruleStore.rateMap[rule.id] || {};
      sum.up += rate.up || 0;
      sum.down += rate.down || 0;
      return sum;
    },
    { up: 0, down: 0 },
  );
  return `↑ ${formatSpeed(speed.up)}  ↓ ${formatSpeed(speed.down)}`;
}

function machineOnlineTime(machine, now) {
  if (!machine.online) {
    return '离线';
  }
  if (Number(machine.uptime_seconds) > 0) {
    return formatDuration(Number(machine.uptime_seconds));
  }
  const since = new Date(machine.online_since || machine.last_heartbeat).getTime();
  if (!Number.isFinite(since) || Number.isNaN(since)) {
    return '--';
  }
  const seconds = Math.max(Math.floor((now - since) / 1000), 0);
  return formatDuration(seconds);
}

function formatDuration(seconds) {
  if (seconds < 60) {
    return `${seconds} 秒`;
  }
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return `${minutes} 分钟`;
  }
  const hours = Math.floor(minutes / 60);
  const days = Math.floor(hours / 24);
  if (days > 0) {
    return `${days} 天 ${hours % 24} 小时`;
  }
  return `${hours} 小时 ${minutes % 60} 分钟`;
}

function addDemoMachine(role) {
  const index = demoMachines.value.filter((item) => item.role === role).length + 1;
  const now = Date.now();
  const isIngress = role === 'ingress';
  demoMachines.value.unshift({
    id: `demo-${role}-${now}`,
    fake: true,
    name: `${isIngress ? '入口' : '出口'}演示 · ${index}`,
    role,
    ip: isIngress ? `203.0.113.${20 + index}` : `198.51.100.${20 + index}`,
    online: true,
    os: 'Ubuntu 22.04',
    agent_version: 'v1.2.5',
    agent_latest_version: 'v1.2.6',
    cpu_percent: Math.min(95, 8 + index * 7),
    mem_percent: Math.min(92, 18 + index * 9),
    disk_percent: Math.min(88, 24 + index * 6),
    uptime_seconds: index * 37 * 60,
    rule_count: Math.max(1, index * 2),
    demo_traffic: (32 + index * 18) * 1024 ** 3,
    demo_net: { up: (18 + index * 11) * 1024 ** 3, down: (46 + index * 21) * 1024 ** 3 },
    demo_speed: { up: (300 + index * 180) * 1024, down: (700 + index * 260) * 1024 },
    demo_info: `${2 + index * 2} Cores · ${4 + index * 4} GB · ${80 + index * 40} GB`,
    online_since: new Date(now - index * 37 * 60 * 1000).toISOString(),
    last_heartbeat: new Date(now - 3 * 1000).toISOString(),
  });
  saveDemoMachines();
  ElMessage.success('演示卡片已新增');
}

async function resetDemoMachines() {
  await ElMessageBox.confirm('确定重置成本地默认演示卡片吗？', '重置演示', {
    type: 'warning',
    confirmButtonText: '重置',
    cancelButtonText: '取消',
  });
  demoMachines.value = buildDemoMachines();
  saveDemoMachines();
  ElMessage.success('演示卡片已重置');
}

function updateDemoMachine(id, patch) {
  const current = demoMachines.value.find((item) => item.id === id);
  if (!current) {
    return;
  }
  Object.assign(current, patch);
  saveDemoMachines();
}

function loadDemoMachines() {
  if (!localStorage) {
    return buildDemoMachines();
  }
  try {
    const raw = localStorage.getItem('onlytun_demo_machines');
    if (!raw) {
      return buildDemoMachines();
    }
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed) || parsed.length === 0) {
      return buildDemoMachines();
    }
    if (
      parsed.some((item) => !item.demo_net || !item.demo_speed || !item.agent_version || !item.agent_latest_version) ||
      !parsed.some((item) => item.last_update_task?.status === 'running')
    ) {
      return buildDemoMachines();
    }
    return parsed;
  } catch {
    return buildDemoMachines();
  }
}

function saveDemoMachines() {
  if (!localStorage) {
    return;
  }
  localStorage.setItem('onlytun_demo_machines', JSON.stringify(demoMachines.value));
}

function buildDemoMachines() {
  const now = Date.now();
  const gb = 1024 ** 3;
  const mb = 1024 ** 2;
  return [
    {
      id: 'demo-ingress-shanghai',
      fake: true,
      name: '上海入口 · A1',
      role: 'ingress',
      ip: '203.0.113.21',
      online: true,
      os: 'Ubuntu 22.04',
      agent_version: 'v1.2.5',
      agent_latest_version: 'v1.2.5',
      cpu_percent: 7,
      mem_percent: 18,
      disk_percent: 36,
      uptime_seconds: 9 * 60 * 60,
      rule_count: 8,
      demo_traffic: 128.6 * gb,
      demo_net: { up: 42.1 * gb, down: 128.6 * gb },
      demo_speed: { up: 820 * 1024, down: 1.4 * mb },
      demo_info: '4 Cores · 8 GB · 80 GB',
      online_since: new Date(now - 9 * 60 * 60 * 1000).toISOString(),
      last_heartbeat: new Date(now - 8 * 1000).toISOString(),
    },
    {
      id: 'demo-ingress-guangzhou',
      fake: true,
      name: '广州入口 · B2',
      role: 'ingress',
      ip: '203.0.113.35',
      online: true,
      os: 'Debian 12',
      agent_version: 'v1.2.5',
      agent_latest_version: 'v1.2.6',
      cpu_percent: 32,
      mem_percent: 44,
      disk_percent: 51,
      uptime_seconds: 2 * 24 * 60 * 60 + 41 * 60,
      rule_count: 13,
      demo_traffic: 438.2 * gb,
      demo_net: { up: 122.4 * gb, down: 438.2 * gb },
      demo_speed: { up: 3.8 * mb, down: 5.6 * mb },
      demo_info: '8 Cores · 16 GB · 160 GB',
      online_since: new Date(now - 2 * 24 * 60 * 60 * 1000 - 41 * 60 * 1000).toISOString(),
      last_heartbeat: new Date(now - 2 * 1000).toISOString(),
      last_update_task: {
        id: 'demo-running-update',
        kind: 'agent',
        status: 'running',
        requested_at: new Date(now - 52 * 1000).toISOString(),
        started_at: new Date(now - 18 * 1000).toISOString(),
      },
    },
    {
      id: 'demo-ingress-hongkong',
      fake: true,
      name: '香港入口 · HK-Edge',
      role: 'ingress',
      ip: '203.0.113.49',
      online: true,
      os: 'Ubuntu 24.04',
      agent_version: 'v1.2.4',
      agent_latest_version: 'v1.2.6',
      cpu_percent: 61,
      mem_percent: 57,
      disk_percent: 73,
      uptime_seconds: 16 * 60 * 60,
      rule_count: 21,
      demo_traffic: 1.82 * 1024 * gb,
      demo_net: { up: 612.7 * gb, down: 1.82 * 1024 * gb },
      demo_speed: { up: 9.2 * mb, down: 12.7 * mb },
      demo_info: '16 Cores · 32 GB · 320 GB',
      online_since: new Date(now - 16 * 60 * 60 * 1000).toISOString(),
      last_heartbeat: new Date(now - 4 * 1000).toISOString(),
    },
    {
      id: 'demo-ingress-offline',
      fake: true,
      name: '东京入口 · 维护中',
      role: 'ingress',
      ip: '203.0.113.78',
      online: false,
      os: 'Rocky Linux 9',
      agent_version: 'v1.2.3',
      agent_latest_version: 'v1.2.6',
      cpu_percent: 0,
      mem_percent: 0,
      disk_percent: 42,
      uptime_seconds: 0,
      rule_count: 5,
      demo_traffic: 64.8 * gb,
      demo_net: { up: 18.2 * gb, down: 64.8 * gb },
      demo_speed: { up: 0, down: 0 },
      demo_info: '4 Cores · 8 GB · 120 GB',
      online_since: '',
      last_heartbeat: new Date(now - 22 * 60 * 1000).toISOString(),
    },
    {
      id: 'demo-egress-singapore',
      fake: true,
      name: '新加坡出口 · SG-01',
      role: 'egress',
      ip: '198.51.100.12',
      online: true,
      os: 'Ubuntu 22.04',
      agent_version: 'v1.2.5',
      agent_latest_version: 'v1.2.5',
      cpu_percent: 12,
      mem_percent: 23,
      disk_percent: 28,
      uptime_seconds: 7 * 60 * 60 + 12 * 60,
      rule_count: 12,
      demo_traffic: 612.4 * gb,
      demo_net: { up: 241.4 * gb, down: 612.4 * gb },
      demo_speed: { up: 2.1 * mb, down: 4.9 * mb },
      demo_info: '8 Cores · 16 GB · 200 GB',
      online_since: new Date(now - 7 * 60 * 60 * 1000 - 12 * 60 * 1000).toISOString(),
      last_heartbeat: new Date(now - 1 * 1000).toISOString(),
    },
    {
      id: 'demo-egress-us',
      fake: true,
      name: '美国出口 · LA-Transit',
      role: 'egress',
      ip: '198.51.100.44',
      online: true,
      os: 'Debian 12',
      agent_version: 'v1.2.4',
      agent_latest_version: 'v1.2.6',
      cpu_percent: 46,
      mem_percent: 62,
      disk_percent: 67,
      uptime_seconds: 4 * 24 * 60 * 60,
      rule_count: 17,
      demo_traffic: 2.4 * 1024 * gb,
      demo_net: { up: 1.1 * 1024 * gb, down: 2.4 * 1024 * gb },
      demo_speed: { up: 7.8 * mb, down: 18.5 * mb },
      demo_info: '16 Cores · 64 GB · 1 TB',
      online_since: new Date(now - 4 * 24 * 60 * 60 * 1000).toISOString(),
      last_heartbeat: new Date(now - 5 * 1000).toISOString(),
    },
    {
      id: 'demo-egress-offline',
      fake: true,
      name: '德国出口 · 离线',
      role: 'egress',
      ip: '198.51.100.89',
      online: false,
      os: 'Ubuntu 20.04',
      agent_version: 'v1.2.2',
      agent_latest_version: 'v1.2.6',
      cpu_percent: 0,
      mem_percent: 0,
      disk_percent: 81,
      uptime_seconds: 0,
      rule_count: 4,
      demo_traffic: 91.3 * gb,
      demo_net: { up: 22.6 * gb, down: 91.3 * gb },
      demo_speed: { up: 0, down: 0 },
      demo_info: '4 Cores · 8 GB · 100 GB',
      online_since: '',
      last_heartbeat: new Date(now - 2 * 60 * 60 * 1000).toISOString(),
    },
  ];
}
</script>

<style>
.machines-page {
  gap: 18px;
}

.machines-page .machines-header {
  flex-direction: column;
  align-items: stretch;
}

.machines-page .toolbar {
  width: 100%;
  justify-content: flex-end;
}

.machines-page .section-title {
  margin: 0;
  font-size: 20px;
  color: #132238;
}

.machines-page .section-meta {
  margin: 6px 0 0;
  font-size: 13px;
  color: #72829d;
}

.machines-page .search-input {
  width: 300px;
  margin-right: auto;
}

.machines-page .machine-groups {
  display: flex;
  flex-direction: column;
  gap: 26px;
  min-height: 220px;
}

.machines-page .group-title-row {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 10px;
  margin-bottom: 14px;
}

.machines-page .group-title {
  margin: 0;
  font-size: 17px;
  color: #15243a;
}

.machines-page .group-count {
  color: #72829d;
  font-size: 13px;
  transform: translateY(1px);
}

.machines-page .machine-card-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(190px, 1fr));
  gap: 16px;
}

.machines-page .machine-card {
  position: relative;
  border-radius: 18px;
  padding: 16px;
  background: linear-gradient(180deg, #ffffff 0%, #f8fbff 100%);
  border: 1px solid rgba(84, 112, 150, 0.16);
  box-shadow: 0 14px 30px rgba(31, 44, 62, 0.07);
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
}

.machines-page .machine-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 18px 36px rgba(31, 44, 62, 0.1);
}

.machines-page .machine-card.is-online {
  border-color: rgba(70, 179, 137, 0.34);
}

.machines-page .machine-card.is-offline {
  background: linear-gradient(180deg, #ffffff 0%, #f8fbff 100%);
  border-color: rgba(122, 138, 160, 0.2);
}

.machines-page .machine-card.is-offline::after {
  content: "";
  position: absolute;
  inset: 0;
  z-index: 8;
  pointer-events: none;
  border-radius: inherit;
  background:
    linear-gradient(180deg, rgba(241, 245, 249, 0.58), rgba(226, 232, 240, 0.66)),
    repeating-linear-gradient(
      135deg,
      rgba(148, 163, 184, 0.08) 0,
      rgba(148, 163, 184, 0.08) 1px,
      transparent 1px,
      transparent 8px
    );
  backdrop-filter: grayscale(0.45) saturate(0.65);
}

.machines-page .machine-card.is-updating {
  background:
    radial-gradient(circle at 18% 0%, rgba(64, 158, 255, 0.2), transparent 34%),
    linear-gradient(180deg, #ffffff 0%, #f5f9ff 100%);
  border-color: rgba(64, 158, 255, 0.34);
}

.machines-page .machine-card-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.machines-page .machine-title-wrap {
  display: flex;
  gap: 10px;
  min-width: 0;
  flex: 1 1 auto;
}

.machines-page .machine-title-main {
  min-width: 0;
  flex: 1 1 auto;
}

.machines-page .machine-state {
  width: 11px;
  height: 11px;
  margin-top: 7px;
  border-radius: 999px;
  flex: 0 0 auto;
}

.machines-page .machine-state.online {
  background: #46b389;
  box-shadow: 0 0 0 6px rgba(70, 179, 137, 0.14);
}

.machines-page .machine-state.offline {
  background: #f56c6c;
  box-shadow: 0 0 0 6px rgba(245, 108, 108, 0.14);
}

.machines-page .machine-name {
  margin: 0;
  color: #132238;
  font-size: 17px;
  line-height: 1.35;
  word-break: break-word;
}

.machines-page .machine-ip-line {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  color: #52657d;
  font-size: 13px;
  min-width: 0;
  width: 100%;
}

.machines-page .machine-ip-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.machines-page .agent-version-inline {
  position: absolute;
  top: 44px;
  right: 18px;
  max-width: 150px;
  color: #9aa8ba;
  font-size: 11px;
  text-align: right;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.machines-page .agent-version-inline.has-update {
  color: #1f6feb;
  font-weight: 700;
}

.machines-page .copy-ip-btn {
  padding: 0;
  min-height: auto;
}

.machines-page .compact-metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-top: 16px;
}

.machines-page .metric-pill {
  padding: 10px 12px;
  border-radius: 14px;
  background: #f2f6fc;
}

.machines-page .metric-pill.blue {
  background: rgba(64, 158, 255, 0.1);
}

.machines-page .metric-pill.green {
  background: rgba(70, 179, 137, 0.1);
}

.machines-page .metric-label {
  display: block;
  margin-bottom: 4px;
  color: #72829d;
  font-size: 12px;
}

.machines-page .metric-pill strong {
  color: #15243a;
  font-size: 15px;
}

.machines-page .machine-detail {
  margin-top: 16px;
  padding-top: 14px;
  border-top: 1px solid rgba(84, 112, 150, 0.12);
}

.machines-page .meter-row,
.machines-page .detail-row {
  display: grid;
  grid-template-columns: 58px minmax(0, 1fr);
  align-items: center;
  gap: 10px;
  min-height: 30px;
  color: #42516a;
  font-size: 13px;
}

.machines-page .meter-label,
.machines-page .detail-label {
  color: #6c7c92;
}

.machines-page .unknown-value {
  color: #9aa8ba;
}

.machines-page .machine-actions {
  display: flex;
  justify-content: flex-end;
  gap: 4px;
  margin-top: 14px;
}

.machines-page .more-action-btn {
  height: 32px;
  padding: 0 12px;
  border-radius: 999px;
  border-color: rgba(84, 112, 150, 0.16);
  color: #52657d;
  background: rgba(255, 255, 255, 0.78);
  box-shadow: 0 8px 18px rgba(31, 44, 62, 0.06);
}

.machines-page .more-action-btn:hover {
  border-color: rgba(31, 111, 235, 0.3);
  color: #1f6feb;
  background: #f7fbff;
}

.machines-page .action-arrow {
  width: 13px;
  margin-left: 6px;
}

.machines-page .machine-update-state {
  margin-top: 18px;
  min-height: 250px;
  padding: 20px 16px 22px;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 18px;
  background: rgba(64, 158, 255, 0.08);
  border: 1px solid rgba(64, 158, 255, 0.14);
}

.machines-page .update-title-row {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
}

.machines-page .machine-update-state strong {
  display: block;
  color: #132238;
  font-size: 16px;
}

.machines-page .update-spin-icon {
  width: 28px;
  height: 28px;
  padding: 7px;
  color: #1f6feb;
  border-radius: 12px;
  background: #ffffff;
  box-shadow: 0 10px 22px rgba(31, 111, 235, 0.14);
  animation: update-spin 1.2s linear infinite;
}

.machines-page .update-steps {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 22px;
  position: relative;
}

.machines-page .update-steps::before,
.machines-page .update-steps::after {
  content: "↓";
  position: absolute;
  left: 50%;
  transform: translateX(-50%);
  color: #1f6feb;
  font-size: 16px;
  font-weight: 800;
  animation: update-arrow-flow 1.15s ease-in-out infinite;
}

.machines-page .update-steps::before {
  top: 45px;
}

.machines-page .update-steps::after {
  top: 108px;
  animation-delay: 0.22s;
}

.machines-page .update-step {
  min-width: 128px;
  min-height: 38px;
  padding: 10px 16px;
  border-radius: 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  color: #7a8aa0;
  font-size: 13px;
  font-weight: 700;
  background: rgba(255, 255, 255, 0.78);
  border: 1px solid rgba(84, 112, 150, 0.12);
}

.machines-page .update-step-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: #b8c4d4;
}

.machines-page .update-step.active {
  color: #1f6feb;
  border-color: rgba(31, 111, 235, 0.22);
  background: #eef6ff;
}

.machines-page .update-step.active .update-step-dot {
  background: #1f6feb;
  box-shadow: 0 0 0 4px rgba(31, 111, 235, 0.12);
}

@keyframes update-spin {
  to {
    transform: rotate(360deg);
  }
}

@keyframes update-arrow-flow {
  0% {
    opacity: 0.28;
    transform: translate(-50%, -5px);
  }
  45% {
    opacity: 1;
  }
  100% {
    opacity: 0.28;
    transform: translate(-50%, 5px);
  }
}

.danger-dropdown-item {
  color: #f56c6c;
}

.machines-page .empty-group {
  padding: 34px;
  border-radius: 18px;
  color: #8a99ad;
  background: rgba(248, 251, 255, 0.8);
  border: 1px dashed rgba(84, 112, 150, 0.22);
  text-align: center;
}

.dialog-tip {
  margin: 0 0 12px;
  color: #6f7f96;
}

.command-modal-block {
  max-height: 320px;
}

.command-modal-block pre {
  white-space: pre;
  word-break: normal;
}

.machines-page .meter-progress .el-progress-bar__outer {
  background: #e5eaf2;
}

.machines-page .meter-progress.cpu .el-progress-bar__inner {
  background: #409eff;
}

.machines-page .meter-progress.mem .el-progress-bar__inner {
  background: #46b389;
}

.machines-page .meter-progress.disk .el-progress-bar__inner {
  background: #f59e0b;
}

@media (max-width: 1100px) {
  .machines-page .machine-card-grid {
    grid-template-columns: repeat(2, minmax(220px, 1fr));
  }
}

@media (max-width: 900px) {
  .machines-page .machines-header {
    flex-direction: column;
  }

  .machines-page .toolbar,
  .machines-page .search-input {
    width: 100%;
  }

  .machines-page .search-input {
    margin-right: 0;
  }

  .machines-page .machine-card-grid {
    grid-template-columns: 1fr;
  }
}
</style>
