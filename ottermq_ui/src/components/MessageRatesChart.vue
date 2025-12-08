<template>
  <q-card flat bordered>
    <q-card-section>
      <div class="text-h6">Message Rates</div>
      <div class="text-caption text-grey-7">Messages per second</div>
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
      name: 'Publish',
      data: props.chartData.publish?.map(d => ({
        x: new Date(d.timestamp).getTime(),
        y: parseFloat(d.value.toFixed(2))
      })) || []
    },
    {
      name: 'Deliver',
      data: props.chartData.deliver?.map(d => ({
        x: new Date(d.timestamp).getTime(),
        y: parseFloat(d.value.toFixed(2))
      })) || []
    },
    {
      name: 'Ack',
      data: props.chartData.ack?.map(d => ({
        x: new Date(d.timestamp).getTime(),
        y: parseFloat(d.value.toFixed(2))
      })) || []
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
      enabled: true,
      easing: 'linear',
      dynamicAnimation: {
        speed: 1000
      }
    }
  },
  stroke: {
    curve: 'smooth',
    width: 2
  },
  colors: ['#1976D2', '#9C27B0', '#21BA45'],
  xaxis: {
    type: 'datetime',
    labels: {
      datetimeUTC: false,
      format: 'HH:mm:ss'
    }
  },
  yaxis: {
    title: {
      text: 'Rate (msg/s)'
    },
    labels: {
      formatter: (val) => val.toFixed(2)
    }
  },
  tooltip: {
    x: {
      format: 'HH:mm:ss'
    },
    y: {
      formatter: (val) => `${val.toFixed(2)} msg/s`
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
