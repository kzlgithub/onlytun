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
          <div class="card-heading">
            <div>
              <h3 class="section-title">面板更新</h3>
              <p class="section-meta">在线更新面板程序，配置和数据库会保留。</p>
            </div>
          </div>
        </template>

        <div class="update-panel">
          <div class="update-hero">
            <div class="update-mark brand-mark" aria-hidden="true">
              <span></span>
              <span></span>
            </div>
            <div class="update-copy">
              <span class="update-eyebrow">Panel Update</span>
              <h4>让面板保持最新</h4>
              <p>更新会短暂重启 onlytun-panel，页面会自动进入等待状态，直到服务恢复。</p>
            </div>
          </div>

          <div class="version-panel">
            <span>当前版本</span>
            <strong>{{ panelVersion }}</strong>
          </div>

          <div class="update-footer">
            <p>建议在没有批量修改规则或更新隧道机时执行。</p>
            <el-button type="primary" :loading="updatingPanel" round @click="updatePanel">
              立即更新
            </el-button>
          </div>
        </div>
      </el-card>
    </div>

    <div v-if="panelUpdate.visible" class="panel-update-lock">
      <div class="panel-update-lock-card">
        <div class="lock-orbit">
          <span></span>
        </div>
        <h2>{{ panelUpdateTitle }}</h2>
        <p>{{ panelUpdate.message }}</p>
        <div class="lock-steps">
          <div
            v-for="step in panelUpdateSteps"
            :key="step.key"
            class="lock-step"
            :class="{ active: step.active, done: step.done }"
          >
            <span class="step-dot"></span>
            <span>{{ step.label }}</span>
          </div>
        </div>
        <el-button v-if="panelUpdate.phase === 'failed'" type="primary" round @click="reloadPage">
          刷新页面
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue';
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
const PANEL_UPDATE_LOCK_KEY = 'onlytun_panel_update_lock';
const panelUpdate = reactive({
  visible: false,
  phase: 'idle',
  message: '',
});
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

const panelUpdateTitle = computed(() => {
  if (panelUpdate.phase === 'done') return '面板更新完成';
  if (panelUpdate.phase === 'failed') return '面板更新需要确认';
  return '面板正在更新';
});

const panelUpdateSteps = computed(() => [
  {
    key: 'scheduled',
    label: '下发更新',
    active: panelUpdate.phase === 'starting',
    done: ['restarting', 'checking', 'done'].includes(panelUpdate.phase),
  },
  {
    key: 'restart',
    label: '服务重启',
    active: panelUpdate.phase === 'restarting',
    done: ['checking', 'done'].includes(panelUpdate.phase),
  },
  {
    key: 'verify',
    label: '恢复校验',
    active: panelUpdate.phase === 'checking',
    done: panelUpdate.phase === 'done',
  },
]);

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
    '确定要更新面板吗？更新期间页面会锁定，直到服务恢复。',
    '更新面板',
    {
      type: 'warning',
      confirmButtonText: '开始更新',
      cancelButtonText: '取消',
    },
  );

  updatingPanel.value = true;
  const fromVersion = panelVersion.value;
  startPanelUpdateLock('starting', '正在向面板服务下发更新任务，请不要关闭窗口。', fromVersion);
  try {
    try {
      await panelApi.updatePanel({ silentError: true, timeout: 5000 });
    } catch (error) {
      if (error?.response) {
        throw error;
      }
    }
    panelUpdate.phase = 'restarting';
    panelUpdate.message = '更新任务已启动，正在下载并替换面板服务，网络较慢时可能需要几分钟。';
    const nextVersion = await waitForPanelReady(fromVersion);
    panelVersion.value = nextVersion;
    finishPanelUpdateLock(nextVersion);
    ElMessage.success(`面板已更新到 ${nextVersion}`);
  } catch (error) {
    failPanelUpdateLock(error);
  } finally {
    updatingPanel.value = false;
  }
}

function startPanelUpdateLock(phase, message, fromVersion) {
  panelUpdate.visible = true;
  panelUpdate.phase = phase;
  panelUpdate.message = message;
  localStorage.setItem(
    PANEL_UPDATE_LOCK_KEY,
    JSON.stringify({
      startedAt: Date.now(),
      fromVersion: fromVersion || 'unknown',
    }),
  );
}

function finishPanelUpdateLock(version) {
  panelUpdate.phase = 'done';
  panelUpdate.message = `服务已恢复，当前版本 ${version || 'unknown'}。`;
  localStorage.removeItem(PANEL_UPDATE_LOCK_KEY);
  window.setTimeout(() => {
    panelUpdate.visible = false;
  }, 1200);
}

function failPanelUpdateLock(error) {
  panelUpdate.phase = 'failed';
  panelUpdate.message =
    error?.message || '暂时无法确认面板是否完成更新，请等待片刻后刷新页面重新检查。';
  localStorage.removeItem(PANEL_UPDATE_LOCK_KEY);
}

function reloadPage() {
  window.location.reload();
}

function sleep(ms) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

async function waitForPanelReady(fromVersion) {
  const startedAt = Date.now();
  let sawUnavailable = false;

  while (Date.now() - startedAt < 20 * 60 * 1000) {
    await sleep(2000);
    panelUpdate.phase = sawUnavailable ? 'checking' : 'restarting';

    try {
      const { data } = await panelApi.version({ silentError: true, timeout: 3000 });
      const nextVersion = data.version || 'unknown';
      panelUpdate.phase = 'checking';
      panelUpdate.message = `面板已响应，正在校验版本：${nextVersion}`;

      if (fromVersion && fromVersion !== 'unknown' && nextVersion !== fromVersion) {
        return nextVersion;
      }
      if (sawUnavailable && Date.now() - startedAt > 8000) {
        return nextVersion;
      }
    } catch {
      sawUnavailable = true;
      panelUpdate.phase = 'restarting';
      panelUpdate.message = '服务正在重启，页面会自动等待恢复。';
    }
  }

  throw new Error('等待面板恢复超时，请稍后刷新页面或检查 onlytun-panel 服务日志。');
}

function resumePanelUpdateLock() {
  const raw = localStorage.getItem(PANEL_UPDATE_LOCK_KEY);
  if (!raw) return;

  try {
    const state = JSON.parse(raw);
    if (!state.startedAt || Date.now() - state.startedAt > 25 * 60 * 1000) {
      localStorage.removeItem(PANEL_UPDATE_LOCK_KEY);
      return;
    }

    panelUpdate.visible = true;
    panelUpdate.phase = 'checking';
    panelUpdate.message = '检测到面板正在更新，正在等待服务恢复。';
    waitForPanelReady(state.fromVersion)
      .then((version) => {
        panelVersion.value = version;
        finishPanelUpdateLock(version);
      })
      .catch(failPanelUpdateLock);
  } catch {
    localStorage.removeItem(PANEL_UPDATE_LOCK_KEY);
  }
}

onMounted(async () => {
  await loadPanelVersion();
  resumePanelUpdateLock();
});
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

.password-form {
  max-width: 420px;
}

:deep(.el-input__wrapper) {
  border-radius: 12px;
}

.update-panel {
  min-height: 260px;
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.update-hero {
  min-height: 122px;
  padding: 22px;
  border-radius: 22px;
  display: flex;
  align-items: center;
  gap: 18px;
  background:
    radial-gradient(circle at 12% 18%, rgba(64, 158, 255, 0.18), transparent 30%),
    linear-gradient(135deg, #f7fbff 0%, #ffffff 58%, #f5f9ff 100%);
  border: 1px solid rgba(84, 112, 150, 0.12);
}

.update-mark {
  width: 58px;
  height: 58px;
  flex: 0 0 auto;
}

.brand-mark {
  border-radius: 18px;
  position: relative;
  background: #f7fbff;
  border: 1px solid rgba(31, 111, 235, 0.16);
  box-shadow: 0 14px 30px rgba(31, 111, 235, 0.1);
}

.brand-mark span {
  position: absolute;
  display: block;
  border-radius: 999px;
}

.brand-mark span:first-child {
  width: 31px;
  height: 14px;
  left: 13px;
  top: 16px;
  background: #1f6feb;
}

.brand-mark span:last-child {
  width: 14px;
  height: 31px;
  right: 13px;
  bottom: 11px;
  background: #46b389;
}

.update-copy {
  min-width: 0;
}

.update-eyebrow {
  display: inline-flex;
  margin-bottom: 7px;
  color: #409eff;
  font-size: 12px;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.update-copy h4 {
  margin: 0;
  color: #132238;
  font-size: 22px;
  letter-spacing: -0.04em;
}

.update-copy p {
  margin: 8px 0 0;
  color: #6d7f99;
  line-height: 1.65;
}

.version-panel {
  min-height: 72px;
  padding: 0 18px;
  border-radius: 18px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  background: #ffffff;
  border: 1px solid rgba(84, 112, 150, 0.12);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.8);
}

.version-panel span {
  color: #6d7f99;
  font-size: 13px;
}

.version-panel strong {
  padding: 8px 13px;
  border-radius: 999px;
  color: #132238;
  font-size: 14px;
  background: #f4f8fd;
  border: 1px solid rgba(84, 112, 150, 0.12);
}

.update-footer {
  margin-top: auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.update-footer p {
  margin: 0;
  color: #7a8ba3;
  font-size: 13px;
  line-height: 1.6;
}

.panel-update-lock {
  position: fixed;
  inset: 0;
  z-index: 3000;
  display: grid;
  place-items: center;
  padding: 24px;
  background:
    radial-gradient(circle at 30% 20%, rgba(64, 158, 255, 0.14), transparent 34%),
    rgba(244, 248, 253, 0.9);
  backdrop-filter: blur(14px);
}

.panel-update-lock-card {
  width: min(520px, 100%);
  min-height: 360px;
  padding: 36px;
  border-radius: 28px;
  text-align: center;
  background: rgba(255, 255, 255, 0.94);
  border: 1px solid rgba(126, 154, 190, 0.18);
  box-shadow: 0 24px 70px rgba(42, 72, 112, 0.18);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 18px;
}

.panel-update-lock-card h2 {
  margin: 0;
  color: #132238;
  font-size: 26px;
  letter-spacing: -0.04em;
}

.panel-update-lock-card p {
  margin: 0;
  max-width: 390px;
  color: #6d7f99;
  line-height: 1.7;
}

.lock-orbit {
  width: 72px;
  height: 72px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: linear-gradient(135deg, #e8f3ff, #f7fbff);
  border: 1px solid rgba(64, 158, 255, 0.18);
}

.lock-orbit span {
  width: 42px;
  height: 42px;
  border-radius: 50%;
  border: 4px solid rgba(64, 158, 255, 0.18);
  border-top-color: #409eff;
  animation: update-spin 0.9s linear infinite;
}

.lock-steps {
  width: 100%;
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
  margin: 10px 0 2px;
}

.lock-step {
  min-height: 42px;
  border-radius: 999px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: #7a8ba3;
  background: #f5f8fc;
  border: 1px solid rgba(126, 154, 190, 0.16);
  font-size: 13px;
  transition: all 0.2s ease;
}

.lock-step.active,
.lock-step.done {
  color: #1f74d8;
  background: rgba(64, 158, 255, 0.1);
  border-color: rgba(64, 158, 255, 0.24);
}

.lock-step.done {
  color: #2ca37a;
  background: rgba(70, 179, 137, 0.1);
  border-color: rgba(70, 179, 137, 0.22);
}

.step-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: currentColor;
}

@keyframes update-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 980px) {
  .settings-grid {
    grid-template-columns: 1fr;
  }

}
</style>
