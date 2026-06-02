import axios from 'axios';
import { ElMessage } from 'element-plus';
import router from '../router';

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '',
  timeout: 15000,
});

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('onlytun_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error.response?.status;
    const message =
      error.response?.data?.error ||
      error.response?.data?.message ||
      error.message ||
      '请求失败';

    if (error.config?.silentError) {
      return Promise.reject(error);
    }

    if (status === 401) {
      localStorage.removeItem('onlytun_token');
      ElMessage.error('登录已过期，请重新登录');
      if (router.currentRoute.value.path !== '/login') {
        router.push('/login');
      }
    } else {
      ElMessage.error(message);
    }

    return Promise.reject(error);
  },
);

export default api;

export const authApi = {
  login(password) {
    return api.post('/api/login', { password });
  },
  changePassword(payload) {
    return api.post('/api/settings/password', payload);
  },
};

export const panelApi = {
  metrics() {
    return api.get('/api/panel/metrics');
  },
  version(config = {}) {
    return api.get('/api/panel/version', config);
  },
  updatePanel(config = {}) {
    return api.post('/api/panel/update', null, config);
  },
};

export const machineApi = {
  list() {
    return api.get('/api/machines');
  },
  generateToken() {
    return api.post('/api/machines/generate-token');
  },
  update(id, payload) {
    return api.patch(`/api/machines/${id}`, payload);
  },
  updateScript(id) {
    return api.post(`/api/machines/${id}/update-script`);
  },
  remove(id) {
    return api.delete(`/api/machines/${id}`);
  },
  installScript() {
    return api.get('/api/machines/install-script');
  },
};

export const ruleApi = {
  list() {
    return api.get('/api/rules');
  },
  create(payload) {
    return api.post('/api/rules', payload);
  },
  update(id, payload) {
    return api.put(`/api/rules/${id}`, payload);
  },
  remove(id) {
    return api.delete(`/api/rules/${id}`);
  },
  toggle(id) {
    return api.patch(`/api/rules/${id}/toggle`);
  },
};

export const machineGroupApi = {
  list(params = {}) {
    return api.get('/api/machine-groups', { params });
  },
  create(payload) {
    return api.post('/api/machine-groups', payload);
  },
  update(id, payload) {
    return api.put(`/api/machine-groups/${id}`, payload);
  },
  remove(id) {
    return api.delete(`/api/machine-groups/${id}`);
  },
  setMembers(id, machineIds) {
    return api.put(`/api/machine-groups/${id}/members`, { machine_ids: machineIds });
  },
};

export const groupRuleApi = {
  list() {
    return api.get('/api/group-rules');
  },
  create(payload) {
    return api.post('/api/group-rules', payload);
  },
  update(id, payload) {
    return api.put(`/api/group-rules/${id}`, payload);
  },
  remove(id) {
    return api.delete(`/api/group-rules/${id}`);
  },
  toggle(id) {
    return api.patch(`/api/group-rules/${id}/toggle`);
  },
};

export const statsApi = {
  getRuleStats(ruleId, range) {
    return api.get(`/api/stats/${ruleId}`, {
      params: { range },
    });
  },
};
