import { defineStore } from 'pinia';
import { machineApi } from '../api';

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
        this.machines = data.machines || [];
        return this.machines;
      } finally {
        this.loading = false;
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
        this.machines = this.machines.map((item) => (item.id === id ? { ...item, ...updated } : item));
      }
      return updated;
    },
    async deleteMachine(id) {
      await machineApi.remove(id);
      this.machines = this.machines.filter((item) => item.id !== id);
    },
  },
});
