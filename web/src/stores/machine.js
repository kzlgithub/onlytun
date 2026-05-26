import { defineStore } from 'pinia';
import { machineApi } from '../api';

function isSameValue(left, right) {
  if (left === right) {
    return true;
  }
  if (typeof left === 'object' || typeof right === 'object') {
    return JSON.stringify(left) === JSON.stringify(right);
  }
  return false;
}

function hasMachineChanged(current, incoming) {
  const keys = new Set([...Object.keys(current), ...Object.keys(incoming)]);
  for (const key of keys) {
    if (!isSameValue(current[key], incoming[key])) {
      return true;
    }
  }
  return false;
}

export const useMachineStore = defineStore('machines', {
  state: () => ({
    machines: [],
    installCommands: {
      ingress: '',
      egress: '',
    },
    loading: false,
    commandLoading: false,
  }),
  getters: {
    ingressMachines: (state) => state.machines.filter((item) => item.role === 'ingress'),
    egressMachines: (state) => state.machines.filter((item) => item.role === 'egress'),
    onlineIngressMachines() {
      return this.ingressMachines.filter((item) => item.online);
    },
    onlineEgressMachines() {
      return this.egressMachines.filter((item) => item.online);
    },
    machineMap: (state) =>
      state.machines.reduce((acc, item) => {
        acc[item.id] = item;
        return acc;
      }, {}),
  },
  actions: {
    async fetchMachines() {
      this.loading = true;
      try {
        const { data } = await machineApi.list();
        this.mergeMachines(data.machines || []);
        return this.machines;
      } finally {
        this.loading = false;
      }
    },
    mergeMachines(incomingMachines) {
      const currentById = new Map(this.machines.map((item) => [item.id, item]));
      let structureChanged = this.machines.length !== incomingMachines.length;

      const nextMachines = incomingMachines.map((incoming, index) => {
        const current = currentById.get(incoming.id);
        if (!current) {
          structureChanged = true;
          return incoming;
        }

        if (this.machines[index]?.id !== incoming.id) {
          structureChanged = true;
        }

        if (hasMachineChanged(current, incoming)) {
          Object.assign(current, incoming);
        }
        return current;
      });

      if (structureChanged) {
        this.machines = nextMachines;
      }
    },
    async fetchInstallCommands() {
      this.commandLoading = true;
      try {
        const { data } = await machineApi.installScript();
        this.installCommands = {
          ingress: data.ingress_command || '',
          egress: data.egress_command || '',
        };
        return this.installCommands;
      } finally {
        this.commandLoading = false;
      }
    },
    async updateMachine(id, payload) {
      const { data } = await machineApi.update(id, payload);
      const updated = data.machine;
      if (updated) {
        const current = this.machines.find((item) => item.id === id);
        if (current) {
          Object.assign(current, updated);
        }
      }
      return updated;
    },
    async deleteMachine(id) {
      await machineApi.remove(id);
      this.machines = this.machines.filter((item) => item.id !== id);
    },
  },
});
