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
        <div class="topbar-title-row">
          <h2>{{ currentTitle }}</h2>
          <button
            v-if="todayTrafficSummary"
            type="button"
            class="today-traffic-summary"
            aria-label="查看最近五天流量"
            title="点击查看最近五天流量"
            @click.stop.prevent="openRecentTrafficDialog"
          >
            <span>今日总流量：{{ formatCompactBytes(todayTrafficSummary.total) }}</span>
          </button>
        </div>
      </header>

      <main class="content-area">
        <router-view />
      </main>
    </section>

    <el-dialog
      v-model="recentTrafficDialogVisible"
      :title="recentTrafficDialogTitle"
      width="560px"
      class="traffic-history-dialog"
    >
      <div class="traffic-history" v-loading="recentTrafficLoading">
        <div class="traffic-history-total">
          <span>最近 5 天总流量</span>
          <strong>{{ formatCompactBytes(recentTrafficTotal) }}</strong>
        </div>
        <div class="traffic-history-list">
          <div v-for="point in recentTrafficPoints" :key="point.date" class="traffic-history-item">
            <div class="traffic-history-row">
              <span>{{ formatTrafficDate(point.date) }}</span>
              <strong>{{ formatCompactBytes(point.total) }}</strong>
            </div>
            <div class="traffic-history-bar">
              <span :style="{ width: trafficBarWidth(point.total) }"></span>
            </div>
          </div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useAuthStore } from '../stores/auth';
import { useGroupRuleStore } from '../stores/groupRule';
import { useRuleStore } from '../stores/rule';
import { panelApi, statsApi } from '../api';
import { formatBytes } from '../utils/format';

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const ruleStore = useRuleStore();
const groupRuleStore = useGroupRuleStore();
const panelVersion = ref('unknown');
const recentTrafficDialogVisible = ref(false);
const recentTrafficLoading = ref(false);
const recentTrafficPoints = ref([]);
const recentTrafficTotal = ref(0);

const activePath = computed(() => {
  if (route.path.startsWith('/rules/') && route.path.endsWith('/stats')) {
    return '/rules';
  }
  return route.path;
});

const currentTitle = computed(() => route.meta?.title || 'OnlyTun');

const todayTrafficSummary = computed(() => {
  if (route.path === '/rules') {
    return { total: summarizeSingleRulesTotal() };
  }
  if (route.path === '/group-rules') {
    return { total: summarizeGroupRulesTotal() };
  }
  return null;
});

const recentTrafficDialogTitle = computed(() =>
  route.path === '/group-rules' ? '设备组规则最近五天流量' : '转发规则最近五天流量',
);

const recentTrafficMax = computed(() =>
  recentTrafficPoints.value.reduce((max, point) => Math.max(max, Number(point.total || 0)), 0),
);

function summarizeSingleRulesTotal() {
  return ruleStore.rules.reduce(
    (total, rule) => {
      const up = Number(ruleStore.dayUpTotals[rule.id] || rule.today_bytes_up || 0);
      const down = Number(ruleStore.dayDownTotals[rule.id] || rule.today_bytes_down || 0);
      const fallbackTotal = Number(ruleStore.dayTotals[rule.id] || rule.today_bytes || 0);
      return total + (up + down || fallbackTotal);
    },
    0,
  );
}

function summarizeGroupRulesTotal() {
  return groupRuleStore.rules.reduce(
    (total, rule) => {
      const up = Number(rule.today_bytes_up || 0);
      const down = Number(rule.today_bytes_down || 0);
      const fallbackTotal = Number(rule.today_bytes || 0);
      return total + (up + down || fallbackTotal);
    },
    0,
  );
}

function formatCompactBytes(value) {
  return formatBytes(value).replace(/\s+/g, '');
}

function currentTrafficScope() {
  if (route.path === '/rules') {
    return 'rules';
  }
  if (route.path === '/group-rules') {
    return 'group_rules';
  }
  return '';
}

async function openRecentTrafficDialog() {
  const scope = currentTrafficScope();
  if (!scope) return;

  recentTrafficDialogVisible.value = true;
  recentTrafficLoading.value = true;
  try {
    const { data } = await statsApi.getRecentTraffic(scope, 5);
    recentTrafficPoints.value = data.points || [];
    recentTrafficTotal.value = data.total || 0;
  } finally {
    recentTrafficLoading.value = false;
  }
}

function formatTrafficDate(input) {
  const date = new Date(input);
  if (Number.isNaN(date.getTime())) {
    return '-';
  }
  return date.toLocaleDateString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
  });
}

function trafficBarWidth(value) {
  const total = Number(value || 0);
  const max = recentTrafficMax.value;
  if (max <= 0 || total <= 0) {
    return '0%';
  }
  return `${Math.max((total / max) * 100, 6)}%`;
}

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

.topbar-title-row {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-wrap: wrap;
  min-width: 0;
}

.today-traffic-summary {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  min-height: 30px;
  padding: 6px 13px;
  border: 1px solid rgba(64, 158, 255, 0.14);
  border-radius: 999px;
  background:
    linear-gradient(135deg, rgba(238, 247, 255, 0.96), rgba(247, 251, 255, 0.88)),
    rgba(255, 255, 255, 0.8);
  color: #52657f;
  font-size: 12px;
  line-height: 1;
  font-family: inherit;
  cursor: pointer;
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.86),
    0 10px 26px rgba(64, 158, 255, 0.08);
  transition: transform 0.18s ease, border-color 0.18s ease, box-shadow 0.18s ease, color 0.18s ease;
}

.today-traffic-summary:hover {
  color: #1f6feb;
  border-color: rgba(64, 158, 255, 0.28);
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.9),
    0 12px 30px rgba(64, 158, 255, 0.13);
  transform: translateY(-1px);
}

.traffic-history {
  min-height: 252px;
}

.traffic-history-total {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 18px;
  margin-bottom: 14px;
  border: 1px solid rgba(64, 158, 255, 0.14);
  border-radius: 18px;
  background: linear-gradient(135deg, rgba(238, 247, 255, 0.96), rgba(250, 253, 255, 0.96));
  color: #64748b;
}

.traffic-history-total strong {
  color: #132238;
  font-size: 22px;
  letter-spacing: -0.03em;
}

.traffic-history-list {
  display: grid;
  gap: 10px;
}

.traffic-history-item {
  padding: 12px 14px;
  border: 1px solid rgba(113, 135, 166, 0.12);
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.82);
}

.traffic-history-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin-bottom: 9px;
  color: #64748b;
  font-size: 13px;
}

.traffic-history-row strong {
  color: #1d2b42;
}

.traffic-history-bar {
  height: 8px;
  overflow: hidden;
  border-radius: 999px;
  background: #e9eff7;
}

.traffic-history-bar span {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, #7cc4ff, #3f8cff);
  transition: width 0.28s ease;
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
