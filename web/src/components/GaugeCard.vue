<template>
  <article class="gauge-card" :style="cardStyle">
    <div class="gauge-title">
      <div>
        <h4>{{ title }}</h4>
        <p>{{ subtitle }}</p>
      </div>
      <span class="status-pill" :class="statusClass">{{ statusText }}</span>
    </div>

    <div class="gauge-wrap">
      <div ref="chartRef" class="gauge-chart"></div>
      <div class="gauge-value">
        <strong>{{ safeValue.toFixed(1) }}</strong>
        <span>%</span>
      </div>
    </div>
  </article>
</template>

<script setup>
import * as echarts from 'echarts/core';
import { GaugeChart } from 'echarts/charts';
import { CanvasRenderer } from 'echarts/renderers';
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue';

echarts.use([GaugeChart, CanvasRenderer]);

const props = defineProps({
  title: { type: String, required: true },
  subtitle: { type: String, default: '' },
  value: { type: Number, default: 0 },
  color: { type: String, default: '#409eff' },
});

const chartRef = ref(null);
let chart;

const safeValue = computed(() => Math.max(0, Math.min(100, Number(props.value || 0))));
const statusClass = computed(() => {
  if (safeValue.value >= 85) {
    return 'danger';
  }
  if (safeValue.value >= 65) {
    return 'warn';
  }
  return 'good';
});
const statusText = computed(() => {
  if (safeValue.value >= 85) {
    return '紧张';
  }
  if (safeValue.value >= 65) {
    return '偏高';
  }
  return '正常';
});
const cardStyle = computed(() => ({
  '--gauge-color': props.color,
}));

function renderGauge() {
  if (!chartRef.value) {
    return;
  }
  if (!chart) {
    chart = echarts.init(chartRef.value);
  }

  chart.setOption({
    animationDuration: 500,
    series: [
      {
        type: 'gauge',
        startAngle: 210,
        endAngle: -30,
        min: 0,
        max: 100,
        radius: '100%',
        center: ['50%', '56%'],
        pointer: {
          show: true,
          length: '54%',
          width: 5,
          itemStyle: {
            color: '#23344d',
          },
        },
        anchor: {
          show: true,
          size: 12,
          showAbove: true,
          itemStyle: {
            color: '#ffffff',
            borderColor: '#23344d',
            borderWidth: 4,
          },
        },
        progress: {
          show: true,
          roundCap: true,
          width: 16,
          itemStyle: {
            color: props.color,
            shadowColor: props.color,
            shadowBlur: 14,
            shadowOffsetY: 4,
          },
        },
        axisLine: {
          roundCap: true,
          lineStyle: {
            width: 16,
            color: [[1, '#e8eef7']],
          },
        },
        axisTick: { show: false },
        splitLine: {
          distance: -18,
          length: 7,
          lineStyle: {
            width: 2,
            color: 'rgba(82, 101, 125, 0.22)',
          },
        },
        axisLabel: {
          distance: 8,
          color: '#9aa8ba',
          fontSize: 11,
          formatter(value) {
            return value === 0 || value === 50 || value === 100 ? value : '';
          },
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
  min-height: 250px;
  padding: 20px;
  border-radius: 24px;
  border: 1px solid rgba(84, 112, 150, 0.13);
  background:
    radial-gradient(circle at 50% 46%, color-mix(in srgb, var(--gauge-color) 13%, transparent), transparent 34%),
    linear-gradient(180deg, #ffffff 0%, #f8fbff 100%);
  box-shadow: 0 16px 36px rgba(31, 44, 62, 0.07);
}

.gauge-title {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.gauge-title h4 {
  margin: 0;
  color: #132238;
  font-size: 20px;
  letter-spacing: -0.04em;
}

.gauge-title p {
  margin: 6px 0 0;
  color: #7a8aa2;
  font-size: 13px;
}

.status-pill {
  flex: 0 0 auto;
  padding: 6px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 700;
}

.status-pill.good {
  color: #168363;
  background: rgba(70, 179, 137, 0.12);
}

.status-pill.warn {
  color: #a46207;
  background: rgba(245, 158, 11, 0.16);
}

.status-pill.danger {
  color: #c23a3a;
  background: rgba(245, 108, 108, 0.14);
}

.gauge-wrap {
  position: relative;
  height: 184px;
  margin-top: 12px;
}

.gauge-chart {
  height: 100%;
}

.gauge-value {
  position: absolute;
  left: 50%;
  bottom: 8px;
  display: flex;
  align-items: flex-end;
  gap: 3px;
  transform: translateX(-50%);
  pointer-events: none;
}

.gauge-value strong {
  color: #101d2f;
  font-size: 38px;
  line-height: 0.9;
  letter-spacing: -0.06em;
}

.gauge-value span {
  margin-bottom: 3px;
  color: #8b9ab0;
  font-size: 15px;
  font-weight: 700;
}
</style>
