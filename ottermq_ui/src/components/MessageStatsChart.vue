<template>
  <q-card flat bordered>
    <q-card-section>
      <div class="text-h6">Queued Messages</div>
      <div class="text-caption text-grey-7">Ready • Unacked • Total</div>
    </q-card-section>
    <q-card-section class="q-pt-none">
      <apexchart
        v-if="chartData"
        type="line"
        height="300"
        :options="chartOptions"
        :series="series"
      />
      <div v-else class="text-center q-pa-md text-grey-6">
        No data available
      </div>
    </q-card-section>
  </q-card>
</template>

<script setup>
import { computed } from 'vue'
import VueApexCharts from 'vue3-apexcharts'

const apexchart = VueApexCharts
const windowFilter = (points) => {
    if (!points || points.length === 0) return [];

    const now = Date.now();
    let selectedWindow = 1; // currently, 1min -- TODO: make this dynamic (1min, 10min, 1hr)
    const window = now - selectedWindow * 60 * 1000;
    return points
        .filter(d => new Date(d.timestamp).getTime() >= window)
        .map(d => ({ 
            x: new Date(d.timestamp).getTime(),
            y: Math.round(d.value)
        }))
}

const props = defineProps({
  chartData: {
    type: Object,
    default: null
  }
})

const series = computed(() => {
  if (!props.chartData) return []
  
  return [
    {
      name: 'Ready',
      data: windowFilter(props.chartData.ready)
    },
    {
      name: 'Unacked',
      data: windowFilter(props.chartData.unacked)
    },
    {
      name: 'Total',
      data: windowFilter(props.chartData.total)
    }
  ]
})

const chartOptions = {
  chart: {
    type: 'line',
    height: 300,
    toolbar: {
      show: false
    },
    animations: {
      enabled: false,
      // easing: 'linear',
      // dynamicAnimation: { speed: 1000 }
    },
    zoom: { enabled: false }
  },
  stroke: {
    curve: 'smooth',
    width: 2
  },
  colors: ['#EDC240', '#AFD8F8', '#CB4B4B'],
  xaxis: {
    type: 'datetime',
    labels: {
      datetimeUTC: false,
      format: 'HH:mm:ss'
    }
  },
  yaxis: {
    title: {
      text: 'Messages'
    },
    labels: {
      formatter: (val) => Math.round(val).toLocaleString()
    }
  },
  tooltip: {
    x: {
      format: 'HH:mm:ss'
    },
    y: {
      formatter: (val) => Math.round(val).toLocaleString()
    }
  },
  legend: {
    position: 'top',
    horizontalAlign: 'center'
  },
  grid: {
    borderColor: '#f1f1f1'
  }
}
</script>
