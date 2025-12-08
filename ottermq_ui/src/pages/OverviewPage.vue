<template>
  <q-page class="q-pa-md">
    <!-- Header Bar (RabbitMQ style) -->
    <div class="row items-center q-mb-lg">
      <div class="text-h4 text-weight-medium">
       Overview
      </div>
      <q-space />
      <div class="text-subtitle1 text-grey-7">
        Node: <strong>{{ store.nodeName }}</strong>
        <q-icon name="circle" size="xs" class="q-ml-xs text-positive" />
      </div>
    </div>

    <!-- Top Summary Cards -->
    <div class="row q-col-gutter-md q-mb-lg">
      <div class="col-xs-12 col-sm-6 col-md-3">
        <q-card flat bordered class="overview-card">
          <q-card-section class="text-center">
            <div class="text-h3 text-weight-bold text-primary">
              {{ store.totalMessages.toLocaleString() }}
            </div>
            <div class="text-caption text-grey-7">Total Messages</div>
          </q-card-section>
        </q-card>
      </div>

      <div class="col-xs-12 col-sm-6 col-md-3">
        <q-card flat bordered class="overview-card">
          <q-card-section class="text-center">
            <div class="text-h3 text-weight-bold text-positive">
              {{ store.readyMessages.toLocaleString() }}
            </div>
            <div class="text-caption text-grey-7">Ready</div>
          </q-card-section>
        </q-card>
      </div>

      <div class="col-xs-12 col-sm-6 col-md-3">
        <q-card flat bordered class="overview-card">
          <q-card-section class="text-center">
            <div class="text-h3 text-weight-bold text-orange">
              {{ store.unackedMessages.toLocaleString() }}
            </div>
            <div class="text-caption text-grey-7">Unacknowledged</div>
          </q-card-section>
        </q-card>
      </div>

      <div class="col-xs-12 col-sm-6 col-md-3">
        <q-card flat bordered class="overview-card">
          <q-card-section class="text-center">
            <div class="text-h3 text-weight-bold">
              {{ store.data?.object_totals?.consumers || 0 }}
            </div>
            <div class="text-caption text-grey-7">Consumers</div>
          </q-card-section>
        </q-card>
      </div>
    </div>

    <!-- Charts Section -->
    <div class="row q-col-gutter-md q-mb-lg">
      <div class="col-12 col-md-6">
        <MessageStatsChart :chart-data="store.chartsData?.message_stats" />
      </div>
      <div class="col-12 col-md-6">
        <MessageRatesChart :chart-data="store.chartsData?.message_rates" />
      </div>
    </div>

    <!-- Object Totals Grid -->
    <div class="row q-col-gutter-md q-mb-lg">
      <div class="col-12 col-md-6">
        <q-card flat bordered>
          <q-card-section>
            <div class="text-h6">Object Totals</div>
          </q-card-section>
          <q-card-section class="q-pt-none">
            <q-markup-table flat dense separator="none">
              <tbody>
                <tr v-for="(value, key) in objectTotals" :key="key">
                  <td class="text-weight-medium">{{ labels[key] || key }}</td>
                  <td class="text-right text-weight-bold">{{ value }}</td>
                </tr>
              </tbody>
            </q-markup-table>
          </q-card-section>
        </q-card>
      </div>

      <div class="col-12 col-md-6">
        <q-card flat bordered>
          <q-card-section>
            <div class="text-h6">Node Statistics</div>
          </q-card-section>
          <q-card-section class="q-pt-none">
            <q-markup-table flat dense separator="none">
              <tbody>
                <tr>
                  <td class="text-weight-medium">Uptime</td>
                  <td class="text-right">{{ store.uptime }}</td>
                </tr>
                <tr>
                  <td class="text-weight-medium">Goroutines</td>
                  <td class="text-right">{{ store.data?.node?.goroutines || '-' }}</td>
                </tr>
                <tr>
                  <td class="text-weight-medium">File Descriptors</td>
                  <td class="text-right">{{ store.data?.node?.fd_used || 0 }} / {{ store.data?.node?.fd_limit?.toLocaleString() || '∞' }}</td>
                </tr>
                <tr>
                  <td class="text-weight-medium">Memory Used</td>
                  <td class="text-right">{{ formatBytes(store.data?.node?.memory_usage) }}</td>
                </tr>
              </tbody>
            </q-markup-table>
          </q-card-section>
        </q-card>
      </div>
    </div>

    <!-- Queue Details Table -->
    <q-card flat bordered class="q-mb-lg">
      <q-card-section>
        <div class="text-h6">Queues</div>
      </q-card-section>
      <q-card-section class="q-pt-none">
        <q-table
          :rows="queueRows"
          :columns="queueColumns"
          row-key="name"
          flat
          dense
          hide-pagination
          :rows-per-page-options="[0]"
        />
      </q-card-section>
    </q-card>

    <!-- Last updated -->
    <div class="text-caption text-grey-6 text-center q-mt-md">
      Last updated:
      {{ store.lastUpdate 
        ? new Date(store.lastUpdate).toLocaleString(undefined, { 
            year: 'numeric', month: 'short', day: 'numeric',
            hour: '2-digit', minute: '2-digit', second: '2-digit'
          })
        : 'never' 
      }}
      <q-spinner-dots v-if="store.loading" size="xs" class="q-ml-xs" />
    </div>
  </q-page>
</template>

<script setup>
import { onMounted, onBeforeUnmount, computed } from 'vue'
import { useOverviewStore } from 'src/stores/overview'
import MessageStatsChart from 'src/components/MessageStatsChart.vue'
import MessageRatesChart from 'src/components/MessageRatesChart.vue'
// import { formatBytes } from 'src/utils/filters' // optional, or inline

const store = useOverviewStore()
let timer = null

onMounted(async () => {
  await store.fetch()
  await store.fetchCharts()
  timer = setInterval(async () => {
    await store.fetch()
    await store.fetchCharts()
  }, 10000) // refresh every 10 seconds
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})

// ———————————————————————— Computed data ————————————————————————

const objectTotals = computed(() => ({
  connections: store.data?.object_totals?.connections || 0,
  channels: store.data?.object_totals?.channels || 0,
  exchanges: store.data?.object_totals?.exchanges || 0,
  queues: store.data?.object_totals?.queues || 0,
  consumers: store.data?.object_totals?.consumers || 0
}))

const labels = {
  connections: 'Connections',
  channels: 'Channels',
  exchanges: 'Exchanges',
  queues: 'Queues',
  consumers: 'Consumers'
}

const queueColumns = [
  { name: 'name', label: 'Name', field: 'name', align: 'left' },
  { name: 'vhost', label: 'Virtual Host', field: 'vhost', align: 'left' },
  { name: 'messages', label: 'Ready', field: 'messages', align: 'right' },
  { name: 'unacknowledged', label: 'Unacked', field: 'messages_unacknowledged', align: 'right' },
  { name: 'total', label: 'Total', field: r => r.messages + r.messages_unacknowledged, align: 'right' }
]

const queueRows = computed(() => 
  store.data?.message_stats?.queue_stats || []
)

// Optional helper if you don't have filters globally
function formatBytes(bytes) {
  if (!bytes) return '-'
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(2)} ${sizes[i]}`
}
</script>

<style scoped lang="scss">
.overview-card {
  height: 120px;
  transition: all 0.2s;

  &:hover {
    box-shadow: 0 4px 12px rgba(0,0,0,0.1);
    transform: translateY(-2px);
  }
}
</style>