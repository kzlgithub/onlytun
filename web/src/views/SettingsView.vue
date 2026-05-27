<template>
  <div class="page-shell settings-page">
    <section class="settings-hero">
      <div>
        <p class="eyebrow">Control Center</p>
        <h3>设置</h3>
        <p>管理面板安全信息、版本更新和运行维护入口。</p>
      </div>
      <div class="version-card">
        <span>当前面板版本</span>
        <strong>{{ panelVersion }}</strong>
      </div>
    </section>

    <div class="settings-grid">
      <el-card class="panel-card settings-card password-card" shadow="never">
        <template #header>
          <div class="card-heading">
            <span class="heading-dot green"></span>
            <div>
              <h3 class="section-title">修改密码</h3>
              <p class="section-meta">保存后会退出当前会话，需要重新登录。</p>
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
          <div class="card-heading">
            <span class="heading-dot blue"></span>
            <div>
              <h3 class="section-title">面板更新</h3>
              <p class="section-meta">从 Release 下载新版二进制，数据库与配置会保留。</p>
            </div>
          </div>
        </template>

        <div class="update-panel-box">
          <div class="update-visual">
            <span class="pulse-ring"></span>
            <strong>{{ panelVersion }}</strong>
            <small>running</small>
          </div>
          <div class="update-copy">
            <h4>热更新面板服务</h4>
            <p>更新会短暂重启 onlytun-panel。建议避开正在创建规则或批量更新隧道机的时候执行。</p>
          </div>
        </div>

        <div class="update-actions">
          <el-button :loading="versionLoading" @click="loadPanelVersion">刷新版本</el-button>
          <el-button type="primary" :loading="updatingPanel" @click="updatePanel">立即更新面板</el-button>
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
    const { data } = await panelApi.version();
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
}

.settings-hero {
  display: flex;
  align-items: stretch;
  justify-content: space-between;
  gap: 18px;
  padding: 24px;
  border-radius: 24px;
  background:
    radial-gradient(circle at 8% 0%, rgba(64, 158, 255, 0.18), transparent 34%),
    linear-gradient(135deg, #ffffff 0%, #f6faff 100%);
  border: 1px solid rgba(84, 112, 150, 0.14);
  box-shadow: 0 18px 42px rgba(31, 44, 62, 0.07);
}

.settings-hero h3 {
  margin: 2px 0 8px;
  color: #132238;
  font-size: 28px;
  letter-spacing: -0.04em;
}

.settings-hero p {
  margin: 0;
  color: #6d7f99;
}

.eyebrow {
  text-transform: uppercase;
  font-size: 12px;
  letter-spacing: 0.12em;
  color: #409eff !important;
  font-weight: 700;
}

.version-card {
  min-width: 190px;
  padding: 18px 20px;
  border-radius: 20px;
  background: #132238;
  color: #ffffff;
  display: flex;
  flex-direction: column;
  justify-content: center;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.08);
}

.version-card span {
  color: rgba(255, 255, 255, 0.58);
  font-size: 12px;
}

.version-card strong {
  margin-top: 8px;
  font-size: 26px;
  letter-spacing: -0.04em;
}

.settings-grid {
  display: grid;
  grid-template-columns: minmax(360px, 0.9fr) minmax(420px, 1.1fr);
  gap: 20px;
}

.settings-card {
  min-height: 100%;
}

.card-heading {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}

.heading-dot {
  width: 12px;
  height: 12px;
  margin-top: 7px;
  border-radius: 999px;
}

.heading-dot.green {
  background: #46b389;
  box-shadow: 0 0 0 6px rgba(70, 179, 137, 0.13);
}

.heading-dot.blue {
  background: #409eff;
  box-shadow: 0 0 0 6px rgba(64, 158, 255, 0.13);
}

.section-title {
  margin: 0;
  font-size: 20px;
  color: #132238;
}

.section-meta {
  margin: 6px 0 0;
  color: #72829d;
  font-size: 13px;
}

.password-form {
  max-width: 430px;
}

:deep(.el-input__wrapper) {
  border-radius: 12px;
}

.update-panel-box {
  display: grid;
  grid-template-columns: 150px minmax(0, 1fr);
  gap: 20px;
  align-items: center;
}

.update-visual {
  position: relative;
  height: 150px;
  border-radius: 28px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  color: #ffffff;
  background:
    radial-gradient(circle at 30% 20%, rgba(255, 255, 255, 0.24), transparent 34%),
    linear-gradient(145deg, #1f6feb 0%, #46b389 100%);
}

.pulse-ring {
  position: absolute;
  width: 92px;
  height: 92px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.34);
}

.update-visual strong {
  position: relative;
  font-size: 24px;
}

.update-visual small {
  position: relative;
  margin-top: 4px;
  opacity: 0.7;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.update-copy h4 {
  margin: 0;
  color: #132238;
  font-size: 20px;
}

.update-copy p {
  margin: 10px 0 0;
  color: #6d7f99;
  line-height: 1.7;
}

.update-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 22px;
}

@media (max-width: 980px) {
  .settings-hero,
  .settings-grid,
  .update-panel-box {
    grid-template-columns: 1fr;
  }

  .settings-hero {
    flex-direction: column;
  }
}
</style>
