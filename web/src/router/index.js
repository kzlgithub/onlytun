import { createRouter, createWebHistory } from 'vue-router';

const routes = [
  {
    path: '/login',
    name: 'login',
    component: () => import('../views/LoginView.vue'),
    meta: { guestOnly: true, title: '登录' },
  },
  {
    path: '/',
    component: () => import('../layouts/MainLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      {
        path: '',
        redirect: '/dashboard',
      },
      {
        path: '/dashboard',
        name: 'dashboard',
        component: () => import('../views/DashboardView.vue'),
        meta: { title: '仪表盘' },
      },
      {
        path: '/machines',
        name: 'machines',
        component: () => import('../views/MachinesView.vue'),
        meta: { title: '隧道机' },
      },
      {
        path: '/rules',
        name: 'rules',
        component: () => import('../views/RulesView.vue'),
        meta: { title: '转发规则' },
      },
      {
        path: '/rules/:id/stats',
        name: 'rule-stats',
        component: () => import('../views/RuleStatsView.vue'),
        meta: { title: '流量详情' },
      },
      {
        path: '/settings',
        name: 'settings',
        component: () => import('../views/SettingsView.vue'),
        meta: { title: '设置' },
      },
    ],
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior() {
    return { top: 0 };
  },
});

router.beforeEach((to) => {
  const token = localStorage.getItem('onlytun_token');
  const demoPreview =
    (to.path === '/dashboard' && to.query.demo === '1') ||
    (import.meta.env.DEV && to.path === '/machines' && to.query.localDemo === '1');

  if (to.meta.requiresAuth && !token && !demoPreview) {
    return { path: '/login', query: { redirect: to.fullPath } };
  }

  if (to.meta.guestOnly && token) {
    return { path: '/dashboard' };
  }

  return true;
});

router.afterEach((to) => {
  const title = to.meta?.title ? `${to.meta.title} - OnlyTun` : 'OnlyTun';
  document.title = title;
});

export default router;
