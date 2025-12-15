<template>
  <q-card flat bordered>
    <q-card-section>
      <div class="text-h6">Message Rates (last 60s)</div>
      <div class="text-caption text-grey-7">Publish • Deliver AutoAck • Deliver Manual Ack • Ack per second</div>
    </q-card-section>
    <q-card-section class="q-pt-none">
      <apexchart
        v-if="chartData"
        type="area"
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

const props = defineProps({
  chartData: {
    type: Object,
    default: null
  }
})

const series = computed(() => {
  if (!props.chartData) return []
  
  const now = Date.now()
  const selectedWindow = 1 
  const cutoff = now - selectedWindow * 60 * 1000

  const transformToSeries = (points, name) => {
    if (!points || points.length === 0) return null

    const data = points
      .filter(p => new Date(p.timestamp).getTime() >= cutoff)
      .map(p => ({
        x: new Date(p.timestamp).getTime(),
        y: Math.max(0, Number(p.value.toFixed(2))) // value is already a rate!
      }))

    return data.length > 0 ? { name, data } : null
  }

  return [
    transformToSeries(props.chartData.publish, 'Publish'),
    transformToSeries(props.chartData.deliver_auto_ack, 'Deliver (auto ack)'),
    transformToSeries(props.chartData.deliver_manual_ack, 'Deliver (manual ack)'),
    transformToSeries(props.chartData.ack, 'Ack')
  ].filter(s => s !== null)
})


const chartOptions = {
  chart: {
    type: 'line',
    height: 300,
    toolbar: { show: false },
    animations: {
      enabled: false,
      // easing: 'linear',
      // dynamicAnimation: { speed: 800 }
    },
    zoom: { enabled: false }
  },
  stroke: {
    curve: 'smooth',
    width: 2.5
  },
  fill: {
    type: 'gradient',
    gradient: {
      shadeIntensity: 0.8,
      opacityFrom: 0.6,
      opacityTo: 0.1,
    }
  },
  colors: ['#EDC240','#1976D2', '#9C27B0', '#21BA45'],
  xaxis: {
    type: 'datetime',
    labels: {
      datetimeUTC: false,
      format: 'HH:mm:ss'
    }
  },
  yaxis: {
    title: { text: 'Message rates' },
    min: 0,
    labels: {
      formatter: (val) => val.toFixed(1)
    }
  },
  tooltip: {
    x: { format: 'HH:mm:ss' },
    y: {
      formatter: (val) => `${val.toFixed(2)} msg/s`
    }
  },
  legend: {
    position: 'top',
    horizontalAlign: 'center'
  },
  grid: {
    borderColor: '#f1f1f1',
    strokeDashArray: 4
  },
  dataLabels: { enabled: false }
}
</script>
