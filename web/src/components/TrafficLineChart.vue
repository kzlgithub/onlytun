<template>
  <div ref="chartRef" class="chart-shell"></div>
</template>

<script setup>
import * as echarts from 'echarts';
import { onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { formatBytes } from '../utils/format';

const props = defineProps({
  points: {
    type: Array,
    default: () => [],
  },
});

const chartRef = ref(null);
let chart;

function buildOption(points) {
  return {
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(19, 34, 56, 0.92)',
      borderWidth: 0,
      textStyle: {
        color: '#eff6ff',
      },
      formatter(params) {
        const title = params[0]?.axisValueLabel || '';
        const rows = params
          .map((item) => `${item.marker}${item.seriesName}: ${formatBytes(item.value)}`)
          .join('<br/>');
        return `${title}<br/>${rows}`;
      },
    },
    grid: {
      left: 48,
      right: 24,
      top: 24,
      bottom: 48,
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: points.map((item) => item.time),
      axisLine: {
        lineStyle: {
          color: '#c8d3e6',
        },
      },
      axisLabel: {
        color: '#72829d',
      },
    },
    yAxis: {
      type: 'value',
      axisLine: {
        show: false,
      },
      splitLine: {
        lineStyle: {
          color: 'rgba(114, 130, 157, 0.18)',
        },
      },
      axisLabel: {
        color: '#72829d',
        formatter(value) {
          return formatBytes(value);
        },
      },
    },
    series: [
      {
        name: '上行',
        type: 'line',
        smooth: true,
        showSymbol: false,
        color: '#409EFF',
        lineStyle: {
          width: 3,
        },
        areaStyle: {
          color: 'rgba(64, 158, 255, 0.12)',
        },
        data: points.map((item) => item.bytes_up || 0),
      },
      {
        name: '下行',
        type: 'line',
        smooth: true,
        showSymbol: false,
        color: '#67C23A',
        lineStyle: {
          width: 3,
        },
        areaStyle: {
          color: 'rgba(103, 194, 58, 0.12)',
        },
        data: points.map((item) => item.bytes_down || 0),
      },
    ],
  };
}

function render() {
  if (!chartRef.value) {
    return;
  }

  if (!chart) {
    chart = echarts.init(chartRef.value);
  }

  chart.setOption(buildOption(props.points));
}

onMounted(() => {
  render();
  window.addEventListener('resize', render);
});

watch(
  () => props.points,
  () => {
    render();
  },
  { deep: true },
);

onBeforeUnmount(() => {
  window.removeEventListener('resize', render);
  chart?.dispose();
});
</script>
