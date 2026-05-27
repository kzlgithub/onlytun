<template>
  <div class="page-shell settings-page">
    <div class="settings-grid">
      <el-card class="panel-card settings-card" shadow="never">
        <template #header>
          <div class="card-heading">
            <div>
              <h3 class="section-title">修改密码</h3>
              <p class="section-meta">修改后会退出当前会话，需要重新登录。</p>
            </div>
          </div>
        </template>

        <el-form ref="formRef" :model="form" :rules="rules" label-position="top" class="password-form">
          <el-form-item label="当前密码" prop="old_password">
            <el-input v-model="form.old_password" type="password" show-password placeholder="请输入当前密码" />
          </el-form-item>
          <el-form-item label="新密码" prop="new_password">
            <el-input v-model="form.new_password" type="password" show-password placeholder="至少 8 位" />
          </el-form-item>
          <el-form-item label="确认新密码" prop="confirm_password">
            <el-input v-model="form.confirm_password" type="password" show-password placeholder="再次输入新密码" />
          </el-form-item>
          <el-button type="primary" :loading="submitting" @click="submitPassword">保存新密码</el-button>
        </el-form>
      </el-card>

      <el-card class="panel-card settings-card update-card" shadow="never">
        <template #header>
          <div class="card-heading card-heading-inline">
            <div>
              <h3 class="section-title">面板更新</h3>
              <p class="section-meta">从 GitHub Release 拉取最新面板二进制，配置和数据库会保留。</p>
            </div>
            <span class="soft-status">systemd</span>
          </div>
        </template>

        <div class="update-panel">
          <div class="update-list">
            <div class="update-item">
              <span>当前版本</span>
              <strong>{{ panelVersion }}</strong>
            </div>
            <div class="update-item">
              <span>更新来源</span>
              <strong>GitHub Release</strong>
            </div>
            <div class="update-item">
              <span>服务方式</span>
              <strong>systemd 自动重启</strong>
            </div>
          </div>

          <p class="update-note">
            更新会短暂重启 onlytun-panel，配置和数据库会保留。建议避开正在创建规则或批量更新隧道机的时候执行。
          </p>

          <div class="update-footer">
            <el-button type="primary" :loading="updatingPanel" round @click="updatePanel">立即更新</el-button>
          </div>
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox } from 'element-plus';
import { useRouter } from 'vue-router';
import { authApi, panelApi } from '../api';
import { useAuthStore } from '../stores/auth';

const router = useRouter();
const authStore = useAuthStore();
const formRef = ref(null);
const submitting = ref(false);
const updatingPanel = ref(false);
const versionLoading = ref(false);
const panelVersion = ref('unknown');
const form = reactive({
  old_password: '',
  new_password: '',
  confirm_password: '',
});

const rules = {
  old_password: [{ required: true, message: '请输入当前密码', trigger: 'blur' }],
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 8, message: '新密码至少 8 位', trigger: 'blur' },
  ],
  confirm_password: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    {
      validator: (_, value, callback) => {
        if (value !== form.new_password) {
          callback(new Error('两次输入的新密码不一致'));
          return;
        }
        callback();
      },
      trigger: 'blur',
    },
  ],
};

async function loadPanelVersion() {
  versionLoading.value = true;
  try {
    const { data } = await panelApi.version({ silentError: true });
    panelVersion.value = data.version || 'unknown';
  } finally {
    versionLoading.value = false;
  }
}

async function submitPassword() {
  await formRef.value.validate();
  submitting.value = true;
  try {
    await authApi.changePassword({
      old_password: form.old_password,
      new_password: form.new_password,
    });
    ElMessage.success('密码已修改，请重新登录');
    authStore.logout();
    router.push('/login');
  } finally {
    submitting.value = false;
  }
}

async function updatePanel() {
  await ElMessageBox.confirm(
    '确定要更新面板吗？服务会短暂重启，当前页面可能需要稍后刷新。',
    '更新面板',
    {
      type: 'warning',
      confirmButtonText: '开始更新',
      cancelButtonText: '取消',
    },
  );

  updatingPanel.value = true;
  try {
    await panelApi.updatePanel();
    ElMessage.success('面板更新任务已启动，稍后刷新版本查看状态');
  } finally {
    updatingPanel.value = false;
  }
}

onMounted(loadPanelVersion);
</script>

<style scoped>
.settings-page {
  display: grid;
  gap: 20px;
  align-content: start;
}

.settings-page .section-title {
  margin: 0;
  color: #132238;
  font-size: 20px;
  letter-spacing: -0.02em;
}

.settings-page .section-meta {
  margin: 6px 0 0;
  color: #72829d;
  font-size: 13px;
  line-height: 1.6;
}

.settings-grid {
  display: grid;
  grid-template-columns: minmax(340px, 0.95fr) minmax(420px, 1.05fr);
  gap: 20px;
  align-items: stretch;
}

.settings-card {
  min-height: 100%;
}

.card-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.card-heading-inline {
  align-items: center;
}

.soft-status {
  padding: 6px 10px;
  border-radius: 999px;
  color: #46b389;
  font-size: 12px;
  background: rgba(70, 179, 137, 0.1);
  border: 1px solid rgba(70, 179, 137, 0.18);
}

.password-form {
  max-width: 420px;
}

:deep(.el-input__wrapper) {
  border-radius: 12px;
}

.update-panel {
  min-height: 236px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.update-list {
  display: grid;
  gap: 10px;
}

.update-item {
  min-height: 46px;
  padding: 0 14px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  background: #f8fbff;
  border: 1px solid rgba(84, 112, 150, 0.1);
}

.update-item span {
  color: #6d7f99;
  font-size: 13px;
}

.update-item strong {
  color: #132238;
  font-size: 14px;
  text-align: right;
}

.update-note {
  margin: 0;
  padding: 12px 14px;
  border-radius: 14px;
  color: #6d7f99;
  line-height: 1.7;
  background: rgba(64, 158, 255, 0.06);
}

.update-footer {
  margin-top: auto;
  display: flex;
  justify-content: flex-end;
}

@media (max-width: 980px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }

}
</style>
