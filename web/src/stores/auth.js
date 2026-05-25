import { defineStore } from 'pinia';
import { authApi } from '../api';

const TOKEN_KEY = 'onlytun_token';

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem(TOKEN_KEY) || '',
    loading: false,
  }),
  getters: {
    isAuthenticated: (state) => Boolean(state.token),
  },
  actions: {
    async login(password) {
      this.loading = true;
      try {
        const { data } = await authApi.login(password);
        this.token = data.token || '';
        localStorage.setItem(TOKEN_KEY, this.token);
        return data;
      } finally {
        this.loading = false;
      }
    },
    logout() {
      this.token = '';
      localStorage.removeItem(TOKEN_KEY);
    },
  },
});
