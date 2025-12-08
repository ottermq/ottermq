<template>
  <q-card flat bordered>
    <q-card-section>
      <div class="text-h6">Queued Messages</div>
      <div class="text-caption text-grey-7">Ready / Unacked / Total</div>
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
      name: 'Ready',
      data: props.chartData.ready?.map(d => ({
        x: new Date(d.timestamp).getTime(),
        y: Math.round(d.value)
      })) || []
    },
    {
      name: 'Unacked',
      data: props.chartData.unacked?.map(d => ({
        x: new Date(d.timestamp).getTime(),
        y: Math.round(d.value)
      })) || []
    },
    {
      name: 'Total',
      data: props.chartData.total?.map(d => ({
        x: new Date(d.timestamp).getTime(),
        y: Math.round(d.value)
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
  colors: ['#21BA45', '#F2C037', '#1976D2'],
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
