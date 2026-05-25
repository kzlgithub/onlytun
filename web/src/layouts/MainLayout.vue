<template>
  <div class="layout-root">
    <aside class="sidebar">
      <div class="brand">
        <div class="brand-mark">OT</div>
        <div>
          <h1>OnlyTun</h1>
          <p>Private Tunnel Panel</p>
        </div>
      </div>

      <el-menu
        :default-active="activePath"
        class="nav-menu"
        background-color="transparent"
        text-color="#d9e4f6"
        active-text-color="#ffffff"
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
      </el-menu>
    </aside>

    <section class="main-area">
      <header class="topbar">
        <div>
          <h2>{{ currentTitle }}</h2>
          <p>{{ subtitle }}</p>
        </div>
        <el-button type="danger" plain @click="handleLogout">
          退出登录
        </el-button>
      </header>

      <main class="content-area">
        <router-view />
      </main>
    </section>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useAuthStore } from '../stores/auth';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();

const activePath = computed(() => {
  if (route.path.startsWith('/rules/') && route.path.endsWith('/stats')) {
    return '/rules';
  }
  return route.path;
});

const currentTitle = computed(() => route.meta?.title || 'OnlyTun');
const subtitle = computed(() => {
  if (route.path === '/dashboard') return '实时掌握整体运行状态和今日流量走势';
  if (route.path === '/machines') return '管理入口机与出口机，快速获取安装命令';
  if (route.path.startsWith('/rules')) return '维护转发链路、查看流量和会话情况';
  return 'OnlyTun 管理面板';
});

function handleLogout() {
  authStore.logout();
  router.push('/login');
}
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
  background:
    radial-gradient(circle at top, rgba(64, 158, 255, 0.22), transparent 34%),
    linear-gradient(180deg, #10223b 0%, #112746 60%, #0d1c33 100%);
  color: #eff6ff;
}

.brand {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 28px;
  padding: 10px 8px 18px;
}

.brand-mark {
  width: 52px;
  height: 52px;
  border-radius: 18px;
  display: grid;
  place-items: center;
  font-weight: 800;
  letter-spacing: 0.06em;
  color: #132238;
  background: linear-gradient(135deg, #d8efff, #86c4ff 65%, #c7f7d8);
  box-shadow: 0 16px 30px rgba(64, 158, 255, 0.28);
}

.brand h1 {
  margin: 0;
  font-size: 24px;
}

.brand p {
  margin: 6px 0 0;
  font-size: 12px;
  color: rgba(239, 246, 255, 0.72);
}

.nav-menu {
  border-right: none;
}

:deep(.el-menu-item) {
  height: 54px;
  border-radius: 16px;
  margin-bottom: 8px;
  font-size: 15px;
}

:deep(.el-menu-item.is-active) {
  background: linear-gradient(135deg, rgba(64, 158, 255, 0.28), rgba(103, 194, 58, 0.18));
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.06);
}

.main-area {
  min-width: 0;
  padding: 28px 30px;
}

.topbar {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 8px 4px 26px;
}

.topbar h2 {
  margin: 0;
  font-size: 32px;
  color: #132238;
}

.topbar p {
  margin: 8px 0 0;
  color: #72829d;
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
