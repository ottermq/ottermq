<template>
  <q-page padding>
    <div class="container">
        <div class="text-h6 q-mb-md">Queues</div>
        <div class="row q-gutter-sm q-mb-md">
          <q-btn label="Add Queue" color="primary" icon="add" @click="showCreateDialog = true" />
        </div>

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
              :class="props.row.state === 'running'? 'small-square--green' : 'small-square--red'"
              />
              {{ props.row.state || '-' }}
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

          <!-- Create Queue Dialog -->
          <q-dialog v-model="showCreateDialog" persistent>
            <q-card style="min-width: 500px">
              <q-card-section>
                <div class="text-h6">Create Queue</div>
              </q-card-section>

              <q-card-section class="q-pt-none">
                <q-form @submit.prevent="createQueue">
                  <q-input
                    v-model="newQueue.name"
                    label="Queue Name *"
                    outlined
                    dense
                    class="q-mb-md"
                    :rules="[val => !!val || 'Name is required']"
                  />

                  <q-select
                    v-model="newQueue.vhost"
                    label="Virtual Host"
                    :options="['/']"
                    outlined
                    dense
                    class="q-mb-md"
                  />

                  <div class="row q-col-gutter-md q-mb-md">
                    <div class="col-6">
                      <q-checkbox v-model="newQueue.durable" label="Durable" />
                    </div>
                    <div class="col-6">
                      <q-checkbox v-model="newQueue.auto_delete" label="Auto Delete" />
                    </div>
                  </div>

                  <q-input
                    v-model.number="newQueue.max_length"
                    label="Max Length (optional)"
                    type="number"
                    outlined
                    dense
                    class="q-mb-md"
                    hint="Maximum number of messages in queue"
                  />

                  <q-input
                    v-model.number="newQueue.message_ttl"
                    label="Message TTL (ms, optional)"
                    type="number"
                    outlined
                    dense
                    class="q-mb-md"
                    hint="Message time-to-live in milliseconds"
                  />

                  <q-expansion-item
                    label="Dead Letter Exchange Configuration"
                    icon="warning"
                    class="q-mb-md"
                  >
                    <q-card>
                      <q-card-section>
                        <q-input
                          v-model="newQueue.x_dead_letter_exchange"
                          label="Dead Letter Exchange"
                          outlined
                          dense
                          class="q-mb-md"
                          hint="Exchange to route expired/rejected messages"
                        />
                        <q-input
                          v-model="newQueue.x_dead_letter_routing_key"
                          label="Dead Letter Routing Key"
                          outlined
                          dense
                          hint="Routing key for dead lettered messages"
                        />
                      </q-card-section>
                    </q-card>
                  </q-expansion-item>

                  <q-card-actions align="right">
                    <q-btn flat label="Cancel" color="primary" v-close-popup @click="resetForm" />
                    <q-btn label="Create" type="submit" color="primary" :loading="creating" />
                  </q-card-actions>
                </q-form>
              </q-card-section>
            </q-card>
          </q-dialog>
      </div>
  </q-page>
</template>

<script setup>
import { onMounted, ref, computed, watch } from 'vue'
import { useQueuesStore } from 'src/stores/queues'

const store = useQueuesStore()
const selectedName = ref('')
const showCreateDialog = ref(false)
const creating = ref(false)

const newQueue = ref({
  name: '',
  vhost: '/',
  durable: false,
  auto_delete: false,
  max_length: null,
  message_ttl: null,
  x_dead_letter_exchange: '',
  x_dead_letter_routing_key: ''
})

const columns = [
  { name: 'vhost', label: 'VHost', field: 'vhost' },
  { name: 'name', label: 'Name', field: 'name', align: 'left' },
  { name: 'status', label: 'Status', field: 'state' },
  { name: 'messages_ready', label: 'Ready', field: 'messages_ready', align: 'right' },
  { name: 'messages_unacked', label: 'Unacked', field: 'messages_unacked', align: 'right' },
  { name: 'messages_total', label: 'Total', field: 'messages_total', align: 'right' },
  { name: 'consumers', label: 'Consumers', field: 'consumers', align: 'right' },
  { name: 'durable', label: 'D', field: 'durable', align: 'center' },
  { name: 'auto_delete', label: 'AD', field: 'auto_delete', align: 'center' },
  { name: 'actions', label: 'Actions', field: 'actions', align: 'center' }
]

const rows = computed(() =>
  store.items.map(q => ({
    ...q,
    durable: q.durable ? '✓' : '',
    auto_delete: q.auto_delete ? '✓' : ''
  }))
)

function select(row) {
  store.select(row.name)
}

watch(() => store.selected, v => { selectedName.value = v || '' })

function resetForm() {
  newQueue.value = {
    name: '',
    vhost: '/',
    durable: false,
    auto_delete: false,
    max_length: null,
    message_ttl: null,
    x_dead_letter_exchange: '',
    x_dead_letter_routing_key: ''
  }
}

async function createQueue() {
  if (!newQueue.value.name) return
  
  creating.value = true
  try {
    await store.addQueue(newQueue.value)
    showCreateDialog.value = false
    resetForm()
  } finally {
    creating.value = false
  }
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
