<template>
  <div class="page-shell settings-page">
    <el-card class="panel-card settings-card" shadow="never">
      <template #header>
        <div>
          <h3 class="section-title">设置</h3>
          <p class="section-meta">修改面板管理密码，保存后需要重新登录。</p>
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
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue';
import { ElMessage } from 'element-plus';
import { useRouter } from 'vue-router';
import { authApi } from '../api';
import { useAuthStore } from '../stores/auth';

const router = useRouter();
const authStore = useAuthStore();
const formRef = ref(null);
const submitting = ref(false);
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
</script>

<style scoped>
.settings-card {
  max-width: 560px;
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
  max-width: 420px;
}

:deep(.el-input__wrapper) {
  border-radius: 12px;
}
</style>
