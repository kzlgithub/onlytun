<template>
  <div class="layout-root">
    <aside class="sidebar">
      <div class="brand">
        <div class="brand-logo-stack">
          <div class="brand-mark">
            <span></span>
            <span></span>
          </div>
          <small>{{ panelVersion }}</small>
        </div>
        <div>
          <h1>OnlyTun</h1>
        </div>
      </div>

      <el-menu
        :default-active="activePath"
        class="nav-menu"
        background-color="transparent"
        text-color="#52657d"
        active-text-color="#1f6feb"
        router
      >
        <el-menu-item index="/dashboard">
          <el-icon><DataLine /></el-icon>
          <span>仪表盘</span>
        </el-menu-item>
        <el-menu-item index="/machines">
          <el-icon><Monitor /></el-icon>
          <span>隧道机</span>
        </el-menu-item>
        <el-menu-item index="/rules">
          <el-icon><Connection /></el-icon>
          <span>转发规则</span>
        </el-menu-item>
        <el-menu-item index="/group-rules">
          <el-icon><Share /></el-icon>
          <span>&#35774;&#22791;&#32452;&#35268;&#21017;</span>
        </el-menu-item>
        <el-menu-item index="/settings">
          <el-icon><Setting /></el-icon>
          <span>设置</span>
        </el-menu-item>
      </el-menu>

      <el-button class="sidebar-logout" type="danger" plain @click="handleLogout">
        退出登录
      </el-button>
    </aside>

    <section class="main-area">
      <header class="topbar">
        <div>
          <h2>{{ currentTitle }}</h2>
        </div>
      </header>

      <main class="content-area">
        <router-view />
      </main>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useAuthStore } from '../stores/auth';
import { panelApi } from '../api';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const panelVersion = ref('unknown');

const activePath = computed(() => {
  if (route.path.startsWith('/rules/') && route.path.endsWith('/stats')) {
    return '/rules';
  }
  return route.path;
});

const currentTitle = computed(() => route.meta?.title || 'OnlyTun');

function handleLogout() {
  authStore.logout();
  router.push('/login');
}

onMounted(async () => {
  try {
    const { data } = await panelApi.version({ silentError: true });
    panelVersion.value = data.version || 'unknown';
  } catch {
    panelVersion.value = 'unknown';
  }
});
</script>

<style scoped>
.layout-root {
  min-height: 100vh;
  display: grid;
  grid-template-columns: 280px minmax(0, 1fr);
}

.sidebar {
  position: sticky;
  top: 0;
  height: 100vh;
  padding: 28px 22px;
  display: flex;
  flex-direction: column;
  background:
    linear-gradient(180deg, rgba(255, 255, 255, 0.94), rgba(244, 248, 255, 0.96)),
    radial-gradient(circle at 20% 0%, rgba(64, 158, 255, 0.16), transparent 32%);
  border-right: 1px solid rgba(84, 112, 150, 0.14);
  color: #18263a;
}

.brand {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  margin-bottom: 32px;
  padding: 8px 8px 18px;
}

.brand-logo-stack {
  display: grid;
  justify-items: center;
  gap: 6px;
  flex: 0 0 auto;
}

.brand-logo-stack small {
  color: #9aa8ba;
  font-size: 11px;
  line-height: 1;
  letter-spacing: 0.02em;
}

.brand-mark {
  width: 52px;
  height: 52px;
  border-radius: 16px;
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
  width: 28px;
  height: 12px;
  left: 12px;
  top: 14px;
  background: #1f6feb;
}

.brand-mark span:last-child {
  width: 12px;
  height: 28px;
  right: 12px;
  bottom: 10px;
  background: #46b389;
}

.brand h1 {
  margin: 0;
  font-size: 24px;
  letter-spacing: -0.03em;
}

.nav-menu {
  border-right: none;
}

:deep(.el-menu-item) {
  height: 50px;
  border-radius: 14px;
  margin-bottom: 8px;
  font-size: 15px;
  color: #52657d;
}

:deep(.el-menu-item:hover) {
  background: rgba(31, 111, 235, 0.07);
}

:deep(.el-menu-item.is-active) {
  background: #eaf2ff;
  color: #1f6feb;
  box-shadow: inset 3px 0 0 #1f6feb;
}

.sidebar-logout {
  margin-top: auto;
  width: 100%;
  height: 42px;
  border-radius: 13px;
}

.main-area {
  min-width: 0;
  padding: 28px 30px;
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 8px 4px 22px;
}

.topbar h2 {
  margin: 0;
  font-size: 32px;
  color: #132238;
  letter-spacing: -0.04em;
}

.content-area {
  min-width: 0;
}

@media (max-width: 980px) {
  .layout-root {
    grid-template-columns: 1fr;
  }

  .sidebar {
    position: static;
    height: auto;
  }

  .main-area {
    padding: 18px;
  }

  .topbar {
    padding-top: 0;
  }
}
</style>
