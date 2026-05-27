<template>
  <div class="login-page">
    <div class="login-backdrop"></div>
    <el-card class="login-card" shadow="never">
      <div class="login-head">
        <div class="login-logo brand-mark">
          <span></span>
          <span></span>
        </div>
        <div>
          <h1>OnlyTun Panel</h1>
          <p>输入管理密码，进入私有隧道控制台</p>
        </div>
      </div>

      <el-form @submit.prevent="handleLogin">
        <el-form-item>
          <el-input
            v-model="password"
            type="password"
            show-password
            size="large"
            placeholder="请输入管理密码"
            @keyup.enter="handleLogin"
          />
        </el-form-item>
        <el-button type="primary" size="large" class="login-button" :loading="authStore.loading" @click="handleLogin">
          登录
        </el-button>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref } from 'vue';
import { ElMessage } from 'element-plus';
import { useRoute, useRouter } from 'vue-router';
import { useAuthStore } from '../stores/auth';

const password = ref('');
const router = useRouter();
const route = useRoute();
const authStore = useAuthStore();

async function handleLogin() {
  if (!password.value) {
    ElMessage.warning('请输入密码');
    return;
  }

  await authStore.login(password.value);
  ElMessage.success('登录成功');
  router.push(route.query.redirect || '/dashboard');
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  position: relative;
  overflow: hidden;
}

.login-backdrop {
  position: absolute;
  inset: 0;
  background:
    radial-gradient(circle at 15% 20%, rgba(64, 158, 255, 0.26), transparent 24%),
    radial-gradient(circle at 85% 18%, rgba(103, 194, 58, 0.18), transparent 20%),
    radial-gradient(circle at 80% 78%, rgba(20, 56, 112, 0.15), transparent 28%);
}

.login-card {
  position: relative;
  z-index: 1;
  width: min(100%, 440px);
  border-radius: 28px;
  border: 1px solid rgba(64, 158, 255, 0.16);
  background: rgba(255, 255, 255, 0.95);
  box-shadow: 0 26px 60px rgba(19, 34, 56, 0.14);
}

.login-head {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 28px;
}

.login-logo {
  width: 60px;
  height: 60px;
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
  width: 32px;
  height: 14px;
  left: 13px;
  top: 16px;
  background: #1f6feb;
}

.brand-mark span:last-child {
  width: 14px;
  height: 32px;
  right: 13px;
  bottom: 11px;
  background: #46b389;
}

.login-head h1 {
  margin: 0;
  font-size: 28px;
}

.login-head p {
  margin: 8px 0 0;
  color: #72829d;
}

.login-button {
  width: 100%;
  height: 48px;
  border-radius: 14px;
}
</style>
