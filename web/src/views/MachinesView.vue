<template>
  <div class="page-shell">
    <el-card class="panel-card" shadow="never">
      <template #header>
        <div class="page-header">
          <div>
            <h3 style="margin: 0">隧道机列表</h3>
          </div>
          <div class="toolbar">
            <span v-if="refreshing" class="refresh-hint">正在更新</span>
            <el-button :loading="manualRefreshing" @click="manualRefresh">立即刷新</el-button>
          </div>
        </div>
      </template>

      <el-table :data="machineStore.machines" v-loading="initialLoading" row-key="id">
        <el-table-column prop="name" label="机器名称" min-width="170" />
        <el-table-column label="角色" width="110">
          <template #default="{ row }">
            <el-tag :type="row.role === 'ingress' ? 'primary' : 'success'" round>
              {{ roleLabel(row.role) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="ip" label="IP" min-width="150" />
        <el-table-column label="在线状态" width="120">
          <template #default="{ row }">
            <span>
              <span :class="['status-dot', row.online ? 'online' : 'offline']"></span>
              {{ row.online ? '在线' : '离线' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="os" label="操作系统" min-width="140" />
        <el-table-column label="CPU%" width="90">
          <template #default="{ row }">
            {{ Number(row.cpu_percent || 0).toFixed(1) }}%
          </template>
        </el-table-column>
        <el-table-column label="内存%" width="90">
          <template #default="{ row }">
            {{ Number(row.mem_percent || 0).toFixed(1) }}%
          </template>
        </el-table-column>
        <el-table-column label="最后心跳" min-width="160">
          <template #default="{ row }">
            <el-tooltip :content="formatDateTime(row.last_heartbeat)">
              <span>{{ formatRelativeTime(row.last_heartbeat) }}</span>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column prop="rule_count" label="关联规则数" width="120" />
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link @click="handleRename(row)">改名</el-button>
            <el-button type="danger" link @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <section class="two-column install-dock">
      <InstallCommandCard
        title="入口机安装命令"
        :command="machineStore.installCommands.ingress"
      />
      <InstallCommandCard
        title="出口机安装命令"
        :command="machineStore.installCommands.egress"
      />
    </section>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { ElMessageBox, ElMessage } from 'element-plus';
import InstallCommandCard from '../components/InstallCommandCard.vue';
import { useMachineStore } from '../stores/machine';
import { formatDateTime, formatRelativeTime, roleLabel } from '../utils/format';

const machineStore = useMachineStore();
const initialLoading = ref(false);
const manualRefreshing = ref(false);
const refreshing = ref(false);
let timer;
let loadingPromise = null;

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

  const tasks = [machineStore.fetchMachines()];
  if (initial || manual) {
    tasks.push(machineStore.fetchInstallCommands());
  }

  loadingPromise = Promise.all(tasks);
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

async function handleRename(machine) {
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

async function handleDelete(machine) {
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
  await loadData({ initial: true });
  timer = window.setInterval(loadData, 30000);
});

onBeforeUnmount(() => {
  window.clearInterval(timer);
});
</script>

<style scoped>
.refresh-hint {
  font-size: 13px;
  color: #8a99ad;
}
</style>
