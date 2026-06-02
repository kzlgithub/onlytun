import { defineStore } from 'pinia';
import { groupRuleApi, machineGroupApi } from '../api';

function mergeById(current, incoming) {
  const currentById = new Map(current.map((item) => [item.id, item]));
  let structureChanged = current.length !== incoming.length;
  const next = incoming.map((item, index) => {
    if (current[index]?.id !== item.id) {
      structureChanged = true;
    }
    return { ...(currentById.get(item.id) || {}), ...item };
  });
  return structureChanged ? next : next;
}

export const useGroupRuleStore = defineStore('groupRules', {
  state: () => ({
    groups: [],
    rules: [],
    modeEnabled: false,
    loading: false,
    refreshing: false,
  }),
  getters: {
    ingressGroups: (state) => state.groups.filter((item) => item.role === 'ingress'),
    egressGroups: (state) => state.groups.filter((item) => item.role === 'egress'),
    enabledRules: (state) => state.rules.filter((item) => item.enabled),
  },
  actions: {
    async fetchGroups(options = {}) {
      if (options.initial) this.loading = true;
      else this.refreshing = true;
      try {
        const { data } = await machineGroupApi.list();
        const incoming = data.groups || [];
        this.groups = mergeById(this.groups, incoming);
        return this.groups;
      } finally {
        this.loading = false;
        this.refreshing = false;
      }
    },
    async fetchRules(options = {}) {
      if (options.initial) this.loading = true;
      else this.refreshing = true;
      try {
        const { data } = await groupRuleApi.list();
        const incoming = data.rules || [];
        this.rules = mergeById(this.rules, incoming);
        return this.rules;
      } finally {
        this.loading = false;
        this.refreshing = false;
      }
    },
    async fetchAll(options = {}) {
      if (options.initial) this.loading = true;
      else this.refreshing = true;
      try {
        const [modeResp, groupsResp, rulesResp] = await Promise.all([
          machineGroupApi.mode(),
          machineGroupApi.list(),
          groupRuleApi.list(),
        ]);
        this.modeEnabled = Boolean(modeResp.data.enabled);
        this.groups = mergeById(this.groups, groupsResp.data.groups || []);
        this.rules = mergeById(this.rules, rulesResp.data.rules || []);
      } finally {
        this.loading = false;
        this.refreshing = false;
      }
    },
    async fetchMode() {
      const { data } = await machineGroupApi.mode();
      this.modeEnabled = Boolean(data.enabled);
      return this.modeEnabled;
    },
    async setMode(enabled) {
      const { data } = await machineGroupApi.setMode(enabled);
      this.modeEnabled = Boolean(data.enabled);
      return this.modeEnabled;
    },
    async createGroup(payload) {
      await machineGroupApi.create(payload);
      return this.fetchGroups();
    },
    async updateGroup(id, payload) {
      await machineGroupApi.update(id, payload);
      return this.fetchGroups();
    },
    async deleteGroup(id) {
      await machineGroupApi.remove(id);
      this.groups = this.groups.filter((item) => item.id !== id);
    },
    async setGroupMembers(id, machineIds) {
      await machineGroupApi.setMembers(id, machineIds);
      return this.fetchGroups();
    },
    async createRule(payload) {
      const { data } = await groupRuleApi.create(payload);
      await this.fetchRules();
      return data;
    },
    async updateRule(id, payload) {
      const { data } = await groupRuleApi.update(id, payload);
      await this.fetchRules();
      return data;
    },
    async deleteRule(id) {
      await groupRuleApi.remove(id);
      this.rules = this.rules.filter((item) => item.id !== id);
    },
    async toggleRule(id) {
      const { data } = await groupRuleApi.toggle(id);
      const index = this.rules.findIndex((item) => item.id === id);
      if (index >= 0) {
        this.rules[index] = { ...this.rules[index], ...data };
      }
      return data;
    },
  },
});
