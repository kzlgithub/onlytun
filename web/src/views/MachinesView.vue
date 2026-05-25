<template>
  <div class="page-shell">
    <section class="two-column">
      <InstallCommandCard
        title="入口机安装命令"
        subtitle="在入口机上执行后，将自动注册并开始拉取面板配置。"
        :command="machineStore.installCommands.ingress"
      />
      <InstallCommandCard
        title="出口机安装命令"
        subtitle="在出口机上执行后，会监听隧道端口并接受入口机连接。"
        :command="machineStore.installCommands.egress"
      />
    </section>

    <el-card class="panel-card" shadow="never">
      <template #header>
        <div class="page-header">
          <div>
            <h3 style="margin: 0">隧道机列表</h3>
            <p class="page-subtitle">每 30 秒自动刷新，随时查看节点在线状态与资源占用。</p>
          </div>
          <el-button :loading="loading" @click="loadData">立即刷新</el-button>
        </div>
      </template>

      <el-table :data="machineStore.machines" v-loading="loading">
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
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { ElMessageBox, ElMessage } from 'element-plus';
import InstallCommandCard from '../components/InstallCommandCard.vue';
import { useMachineStore } from '../stores/machine';
import { formatDateTime, formatRelativeTime, roleLabel } from '../utils/format';

const machineStore = useMachineStore();
const loading = ref(false);
let timer;

async function loadData() {
  loading.value = true;
  try {
    await Promise.all([machineStore.fetchMachines(), machineStore.fetchInstallCommands()]);
  } finally {
    loading.value = false;
  }
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
  await loadData();
  timer = window.setInterval(loadData, 30000);
});

onBeforeUnmount(() => {
  window.clearInterval(timer);
});
</script>
