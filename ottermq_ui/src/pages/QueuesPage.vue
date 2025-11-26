<template>
  <q-page padding>
    <div class="container">
        <div class="text-h6 q-mb-md">Queues</div>
        <q-form class="row q-gutter-sm q-mb-md" @submit.prevent="create">
            <q-input v-model="newQueue" label="Queue name" dense outlined />
            <q-btn label="Add" color="primary" type="submit" />
          </q-form>

          <q-table
            :rows="rows"
            :columns="columns"
            row-key="name"
            flat bordered
            :loading="store.loading"
            @row-click="(_, row) => select(row)"
          >
            <template #body-cell-status="props">
            <q-td :props="props">
              <span 
              class="small-square q-mx-xs"
              :class="props.row.status === 'running'? 'small-square--green' : 'small-square--red'"
              />
              {{ props.row.status || '-' }}
            </q-td>
          </template>

            <template #body-cell-actions="props">
              <q-td :props="props">
                <q-btn size="sm" color="negative" label="Delete" @click.stop="del(props.row.name)" />
              </q-td>
            </template>
          </q-table>

          <div v-if="store.selected" class="q-mt-md">
            <q-form class="row q-gutter-sm" @submit.prevent="consume">
              <q-input v-model="selectedName" label="Selected queue" dense outlined readonly />
              <q-btn label="Get message" color="primary" type="submit" />
            </q-form>

            <q-card v-if="store.lastMessage" class="q-mt-md">
              <q-card-section>
                <div class="text-subtitle2">Message</div>
                <pre class="q-mt-sm">{{ store.lastMessage }}</pre>
              </q-card-section>
            </q-card>
          </div>
      </div>
  </q-page>
</template>

<script setup>
import { onMounted, ref, computed, watch } from 'vue'
import { useQueuesStore } from 'src/stores/queues'

const store = useQueuesStore()
const newQueue = ref('')
const selectedName = ref('')

const columns = [
  { name: 'vhost', label: 'VHost', field: 'vhost' },
  { name: 'name', label: 'Name', field: 'name' },
  { name: 'status', label: 'Status', field: 'status' },
  { name: 'messages', label: 'Ready', field: 'messages', align: 'right' },
  { name: 'unacked', label: 'Unacked', field: 'unacked', align: 'right' },
  { name: 'persisted', label: 'Persisted', field: 'persisted', align: 'right' },
  { name: 'total', label: 'Total', field: 'total', align: 'right' },
  { name: 'actions', label: 'Actions', field: 'actions' }
]

const rows = computed(() =>
  store.items.map(q => ({
    ...q,
    status: 'running',
    unacked: (q.unacked || 0),
    persisted: (q.messages_persistent || 0),
    total: (q.messages || 0)
  }))
)

function select(row) {
  store.select(row.name)
}

watch(() => store.selected, v => { selectedName.value = v || '' })

async function create() {
  if (!newQueue.value) return
  await store.addQueue(newQueue.value)
  newQueue.value = ''
}

async function del(name) {
  await store.deleteQueue(name)
}

async function consume() {
  if (!selectedName.value) return
  await store.get(selectedName.value)
}

onMounted(store.fetch)
</script>

<style scoped lang="scss">

</style>
