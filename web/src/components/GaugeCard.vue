<template>
  <div class="gauge-card">
    <div class="gauge-head">
      <span>{{ title }}</span>
      <strong>{{ safeValue.toFixed(1) }}%</strong>
    </div>
    <div ref="chartRef" class="gauge-chart"></div>
  </div>
</template>

<script setup>
import * as echarts from 'echarts/core';
import { GaugeChart } from 'echarts/charts';
import { CanvasRenderer } from 'echarts/renderers';
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';

echarts.use([GaugeChart, CanvasRenderer]);

const props = defineProps({
  title: { type: String, required: true },
  value: { type: Number, default: 0 },
  color: { type: String, default: '#409eff' },
});

const chartRef = ref(null);
let chart;

const safeValue = computed(() => Math.max(0, Math.min(100, Number(props.value || 0))));

function renderGauge() {
  if (!chartRef.value) {
    return;
  }
  if (!chart) {
    chart = echarts.init(chartRef.value);
  }
  chart.setOption({
    series: [
      {
        type: 'gauge',
        min: 0,
        max: 100,
        radius: '96%',
        progress: {
          show: true,
          width: 12,
          itemStyle: { color: props.color },
        },
        axisLine: {
          lineStyle: {
            width: 12,
            color: [[1, '#e9eef6']],
          },
        },
        axisTick: { show: false },
        splitLine: { show: false },
        axisLabel: { show: false },
        pointer: {
          width: 4,
          length: '58%',
          itemStyle: { color: '#44546a' },
        },
        anchor: {
          show: true,
          size: 7,
          itemStyle: { color: '#44546a' },
        },
        detail: { show: false },
        data: [{ value: safeValue.value }],
      },
    ],
  });
}

onMounted(() => {
  renderGauge();
  window.addEventListener('resize', resizeGauge);
});

watch(() => [props.value, props.color], renderGauge);

onBeforeUnmount(() => {
  window.removeEventListener('resize', resizeGauge);
  chart?.dispose();
});

function resizeGauge() {
  chart?.resize();
}
</script>

<style scoped>
.gauge-card {
  min-height: 210px;
  padding: 18px;
  border-radius: 20px;
  border: 1px solid rgba(84, 112, 150, 0.13);
  background: linear-gradient(180deg, #ffffff 0%, #f8fbff 100%);
  box-shadow: 0 14px 30px rgba(31, 44, 62, 0.07);
}

.gauge-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: #52657d;
}

.gauge-head strong {
  color: #132238;
  font-size: 22px;
}

.gauge-chart {
  height: 150px;
}
</style>
