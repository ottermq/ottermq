<template>
  <q-page padding>
    <div class="container">

      <div class="text-h6 q-mb-md">Channels</div>

        <q-table
          :rows="rows"
          :columns="columns"
          row-key="id"
          flat 
          bordered
          :loading="store.loading"
        >
          <template #body-cell-state="props">
            <q-td :props="props">
              <span 
              class="small-square q-mx-xs"
              :class="stateClass(props.row.state)"
              />
              {{ props.row.state || '-' }}
            </q-td>
          </template>

          <template #body-cell-prefetch_count="props">
            <q-td :props="props">{{ props.row.prefetch_count || 0 }}</q-td>
          </template>

          <template #body-cell-unacked_count="props">
            <q-td :props="props">{{ props.row.unacked_count || 0 }}</q-td>
          </template>

          <template #body-cell-unconfirmed_count="props">
            <q-td :props="props">{{ props.row.unconfirmed_count || 0 }}</q-td>
          </template>
        </q-table>
      </div>
  </q-page>
</template>

<script setup>
import { onMounted, onBeforeUnmount, computed } from 'vue'
import { useChannelsStore } from 'src/stores/channels'

const store = useChannelsStore()
let timer = null

const columns = [
  { name: 'vhost', label: 'VHost', field: 'vhost' },
  { name: 'connection_name', label: 'Connection', field: 'connection_name' },
  { name: 'number', label: 'Channel', field: 'number', align: 'right' },
  { name: 'user', label: 'User', field: 'user' },
  { name: 'state', label: 'State', field: 'state' },
  { name: 'prefetch_count', label: 'Prefetch', field: 'prefetch_count', align: 'right' },
  { name: 'unacked_count', label: 'Unacked', field: 'unacked_count', align: 'right' },
  { name: 'unconfirmed_count', label: 'Unconfirmed', field: 'unconfirmed_count', align: 'right' },
  { name: 'publish_rate', label: 'Publish/s', field: 'publish_rate', align: 'right' },
  { name: 'confirm_rate', label: 'Confirm/s', field: 'confirm_rate', align: 'right' },
  { name: 'unroutable_rate', label: 'Unroutable (drop)', field: 'unroutable_rate', align: 'right' },
  { name: 'deliver_rate', label: 'Deliver/s', field: 'deliver_rate', align: 'right' },
  { name: 'ack_rate', label: 'Ack/s', field: 'ack_rate', align: 'right' },
]

const rows = computed(() => store.items.map(c => ({
  ...c,
  id: `${c.connection_name}-${c.number}`,
  publish_rate: formatRate(c.publish_rate),
  confirm_rate: formatRate(c.confirm_rate),
  unroutable_rate: formatRate(c.unroutable_rate),
  deliver_rate: formatRate(c.deliver_rate),
  ack_rate: formatRate(c.ack_rate),
})))

function formatRate(rate) {
  if (!rate || rate === 0) return '0.0'
  return rate.toFixed(1)
}

function stateClass(state) {
  switch(state) {
    case 'running': return 'small-square--green'
    case 'flow': return 'small-square--yellow'
    case 'closing': return 'small-square--red'
    default: return 'small-square--gray'
  }
}

onMounted(async () => {
  await store.fetch()
  timer = setInterval(store.fetch, 15000)
})

onBeforeUnmount(() => { if (timer) clearInterval(timer) })
</script>

<style scoped lang="scss">

</style> 
