import { defineStore } from 'pinia';
import { ruleApi, statsApi } from '../api';

function normalizeRealtime(rule) {
  return {
    bytes_up: rule.realtime_stat?.bytes_up || 0,
    bytes_down: rule.realtime_stat?.bytes_down || 0,
    peak_conns: rule.realtime_stat?.peak_conns || 0,
  };
}

function isSameValue(left, right) {
  if (left === right) {
    return true;
  }
  if (typeof left === 'object' || typeof right === 'object') {
    return JSON.stringify(left) === JSON.stringify(right);
  }
  return false;
}

function hasRuleChanged(current, incoming) {
  const keys = new Set([...Object.keys(current), ...Object.keys(incoming)]);
  for (const key of keys) {
    if (!isSameValue(current[key], incoming[key])) {
      return true;
    }
  }
  return false;
}

export const useRuleStore = defineStore('rules', {
  state: () => ({
    rules: [],
    loading: false,
    dayTotals: {},
    rateMap: {},
    snapshots: {},
    statsCache: {},
    lastRefreshAt: 0,
  }),
  getters: {
    enabledRules: (state) => state.rules.filter((item) => item.enabled),
    ruleMap: (state) =>
      state.rules.reduce((acc, item) => {
        acc[item.id] = item;
        return acc;
      }, {}),
  },
  actions: {
    async fetchRules(options = {}) {
      const { includeDayTotals = false } = options;
      this.loading = true;
      try {
        const { data } = await ruleApi.list();
        const incomingRules = data.rules || [];
        this.updateRateMap(incomingRules);
        this.mergeRules(incomingRules);
        if (includeDayTotals && incomingRules.length > 0) {
          await this.fetchTodayTotals(incomingRules.map((item) => item.id));
        }
        return this.rules;
      } finally {
        this.lastRefreshAt = Date.now();
        this.loading = false;
      }
    },
    updateRateMap(nextRules) {
      const now = Date.now();
      const elapsed = this.lastRefreshAt ? Math.max((now - this.lastRefreshAt) / 1000, 1) : 0;
      const nextSnapshots = {};
      const nextRateMap = {};

      for (const rule of nextRules) {
        const current = normalizeRealtime(rule);
        const previous = this.snapshots[rule.id];
        if (previous && elapsed > 0) {
          nextRateMap[rule.id] = {
            up: Math.max(current.bytes_up - previous.bytes_up, 0) / elapsed,
            down: Math.max(current.bytes_down - previous.bytes_down, 0) / elapsed,
          };
        } else {
          nextRateMap[rule.id] = { up: 0, down: 0 };
        }
        nextSnapshots[rule.id] = current;
      }

        this.rateMap = nextRateMap;
        this.snapshots = nextSnapshots;
    },
    mergeRules(incomingRules) {
      const currentById = new Map(this.rules.map((item) => [item.id, item]));
      let structureChanged = this.rules.length !== incomingRules.length;

      const nextRules = incomingRules.map((incoming, index) => {
        const current = currentById.get(incoming.id);
        if (!current) {
          structureChanged = true;
          return incoming;
        }

        if (this.rules[index]?.id !== incoming.id) {
          structureChanged = true;
        }

        if (hasRuleChanged(current, incoming)) {
          Object.assign(current, incoming);
        }
        return current;
      });

      if (structureChanged) {
        this.rules = nextRules;
      }
    },
    async fetchTodayTotals(ruleIds) {
      const results = await Promise.all(
        ruleIds.map(async (ruleId) => {
          const { data } = await statsApi.getRuleStats(ruleId, 'day');
          return [ruleId, (data.total_up || 0) + (data.total_down || 0)];
        }),
      );

      this.dayTotals = results.reduce((acc, [ruleId, total]) => {
        acc[ruleId] = total;
        return acc;
      }, {});
    },
    async createRule(payload, options = {}) {
      const { refresh = true } = options;
      const { data } = await ruleApi.create(payload);
      if (refresh) {
        await this.fetchRules({ includeDayTotals: true });
      }
      return data;
    },
    async updateRule(id, payload) {
      const { data } = await ruleApi.update(id, payload);
      await this.fetchRules({ includeDayTotals: true });
      return data;
    },
    async deleteRule(id) {
      await ruleApi.remove(id);
      delete this.dayTotals[id];
      delete this.rateMap[id];
      delete this.snapshots[id];
      this.rules = this.rules.filter((item) => item.id !== id);
    },
    async toggleRule(id) {
      const { data } = await ruleApi.toggle(id);
      const index = this.rules.findIndex((item) => item.id === id);
      if (index >= 0) {
        this.rules[index] = {
          ...this.rules[index],
          ...data,
        };
      }
      return data;
    },
    async fetchStats(ruleId, range = 'day') {
      const { data } = await statsApi.getRuleStats(ruleId, range);
      this.statsCache[`${ruleId}:${range}`] = data;
      return data;
    },
  },
});
