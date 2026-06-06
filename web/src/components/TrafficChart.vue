<template>
  <div class="traffic-chart-wrapper">
    <div v-if="hasData" ref="chartRef" class="traffic-chart"></div>
    <div v-else class="traffic-chart-empty">{{ t('tunnel.trend.empty') }}</div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import uPlot from 'uplot'
import 'uplot/dist/uPlot.min.css'

const props = defineProps({
  points: { type: Array, required: true },
})

const { t } = useI18n()
const chartRef = ref(null)
let chart = null

const hasData = computed(() => props.points.length > 0)

function formatBytes(bytes) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function buildData() {
  return [
    props.points.map((p) => p.timestamp),
    props.points.map((p) => p.bytesIn),
    props.points.map((p) => p.bytesOut),
  ]
}

function initChart() {
  if (!chartRef.value || !hasData.value) return
  if (chart) {
    chart.destroy()
    chart = null
  }

  const data = buildData()
  const opts = {
    width: chartRef.value.clientWidth,
    height: 140,
    pxAlign: false,
    cursor: { drag: { x: false, y: false } },
    select: { show: false },
    legend: { show: true },
    scales: { x: { time: true }, y: { auto: true } },
    axes: [
      {
        space: 60,
        values: [[3600, '{HH}:{mm}']],
      },
      {
        size: 60,
        values: (u, vals) => vals.map((v) => formatBytes(v)),
      },
    ],
      series: [
      { label: '', value: '{HH}:{mm}' },
      {
        label: t('tunnel.traffic.in'),
        stroke: '#38bdf8',
        fill: 'rgba(56,189,248,0.15)',
      },
      {
        label: t('tunnel.traffic.out'),
        stroke: '#34d399',
        fill: 'rgba(52,211,153,0.15)',
      },
    ],
  }

  chart = new uPlot(opts, data, chartRef.value)
}

function refreshChart() {
  if (!chart) {
    initChart()
    return
  }
  if (!hasData.value) {
    chart.destroy()
    chart = null
    return
  }
  chart.setData(buildData())
}

watch(() => props.points, refreshChart, { deep: true })

onMounted(() => {
  initChart()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  if (chart) {
    chart.destroy()
    chart = null
  }
})

function handleResize() {
  if (chart && chartRef.value) {
    chart.setSize({ width: chartRef.value.clientWidth, height: 140 })
  }
}
</script>

<style scoped>
.traffic-chart-wrapper {
  width: 100%;
}
.traffic-chart {
  width: 100%;
}
.traffic-chart-empty {
  padding: 1rem;
  text-align: center;
  color: #64748b;
  font-size: 0.875rem;
}
</style>
