<template>
  <q-card flat bordered>
    <q-card-section>
      <div class="text-h6">Message Rates (last 60s)</div>
      <div class="text-caption text-grey-7">Publish • Deliver • Ack per second</div>
    </q-card-section>
    <q-card-section class="q-pt-none">
      <apexchart
        v-if="hasData"
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
    required: true
  }
})

const computedRateSeries = (points, name) => {
    if (!points || points.length < 2) return { name, data: [] };

    const now = Date.now();
    let selectedWindow = 1; // TODO: make this dynamic (1min, 10min, 1hr)
    const cutoff = now - selectedWindow * 60 * 1000;

    const recent = points
        .filter(p => new Date(p.timestamp).getTime() >= cutoff)
        .sort((a, b) => new Date(a.timestamp) - new Date(b.timestamp));

    if (recent.length < 2) return { name, data: [] };

    const data = [];
    for (let i = 1; i < recent.length; i++) {
        const prev = recent[i - 1];
        const curr = recent[i];

        const t1 = new Date(prev.timestamp).getTime();
        const t2 = new Date(curr.timestamp).getTime();
        const deltaTime = (t2 - t1) / 1000; // in secs

        if (deltaTime <= 0) continue;

        const rate = (curr.value - prev.value) / deltaTime;
        data.push({ 
            x: t2, 
            y: Math.max(0, Number(rate.toFixed(2))) // clamp negative, round 
        });
    }

    return { name, data };
}

const series = computed(() => {  
  return [
    computedRateSeries(props.chartData.publish, 'Publish'),
    computedRateSeries(props.chartData.deliver, 'Deliver'),
    computedRateSeries(props.chartData.ack, 'Ack')
  ].filter(s => s.data.length > 0)
})

const hasData = computed(() => series.value.some(s => s.data.length > 0))

const chartOptions = {
  chart: {
    type: 'line',
    height: 300,
    toolbar: { show: false },
    animations: {
      enabled: true,
      easing: 'linear',
      dynamicAnimation: { speed: 800 }
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
  colors: ['#1976D2', '#9C27B0', '#21BA45'],
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
