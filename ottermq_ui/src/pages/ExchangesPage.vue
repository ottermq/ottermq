<template>
  <q-page padding>
    <div class="container">

      <div class="text-h6 q-mb-md">Exchanges</div>

        <div class="row q-gutter-sm q-mb-md">
          <q-btn label="Add Exchange" color="primary" icon="add" @click="showCreateDialog = true" />
        </div>

        <q-table
          :rows="store.items"
          :columns="columns"
          row-key="name"
          flat bordered
          :loading="store.loading"
          @row-click="(_, row) => select(row.name)"
        >
          <template #body-cell-actions="props">
            <q-td :props="props">
              <q-btn size="sm" color="negative" label="Delete" @click.stop="del(props.row.name)" />
            </q-td>
          </template>
        </q-table>

        <div v-if="store.selected" class="q-mt-lg">
          <div class="text-subtitle1 q-mb-sm">Bindings for <b>{{ store.selected }}</b></div>

          <q-form class="row q-gutter-sm q-mb-md" @submit.prevent="addBinding">
            <q-input v-model="routingKey" label="Routing key" dense outlined />
            <q-input v-model="queueName" label="Queue" dense outlined />
            <q-btn label="Bind" color="primary" type="submit" />
          </q-form>

          <q-table
            :rows="store.bindings"
            :columns="bindingCols"
            row-key="routingKey-queue"
            flat bordered
          >
            <template #body-cell-actions="props">
              <q-td :props="props">
                <q-btn size="sm" label="Unbind" @click="unbind(props.row)" />
              </q-td>
            </template>
          </q-table>

          <q-separator class="q-my-md" />

          <div class="text-subtitle1 q-mb-sm">Publish Message</div>
          <q-form class="column q-gutter-sm" @submit.prevent="publish">
            <q-input v-model="pubRoutingKey" label="Routing key" dense outlined />
            <q-input v-model="message" type="textarea" autogrow label="Message" dense outlined />
            <q-btn label="Publish" color="primary" type="submit" />
          </q-form>
        </div>

        <!-- Create Exchange Dialog -->
        <q-dialog v-model="showCreateDialog" persistent>
          <q-card style="min-width: 500px">
            <q-card-section>
              <div class="text-h6">Create Exchange</div>
            </q-card-section>

            <q-card-section class="q-pt-none">
              <q-form @submit.prevent="createExchange">
                <q-input
                  v-model="newExchange.name"
                  label="Exchange Name *"
                  outlined
                  dense
                  class="q-mb-md"
                  :rules="[val => !!val || 'Name is required']"
                />

                <q-select
                  v-model="newExchange.vhost"
                  label="Virtual Host"
                  :options="['/']"
                  outlined
                  dense
                  class="q-mb-md"
                />

                <q-select
                  v-model="newExchange.type"
                  label="Exchange Type *"
                  :options="types"
                  outlined
                  dense
                  class="q-mb-md"
                />

                <div class="row q-col-gutter-md q-mb-md">
                  <div class="col-6">
                    <q-checkbox v-model="newExchange.durable" label="Durable" />
                  </div>
                  <div class="col-6">
                    <q-checkbox v-model="newExchange.auto_delete" label="Auto Delete" />
                  </div>
                </div>

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
import { onMounted, ref } from 'vue'
import { useExchangesStore } from 'src/stores/exchanges'
import { Notify } from 'quasar'

const store = useExchangesStore()

const showCreateDialog = ref(false)
const creating = ref(false)
const newExchange = ref({
  name: '',
  vhost: '/',
  type: 'direct',
  durable: false,
  auto_delete: false
})

const columns = [
  { name: 'vhost', label: 'VHost', field: 'vhost' },
  { name: 'name', label: 'Name', field: 'name' },
  { name: 'type', label: 'Type', field: 'type' },
  { name: 'actions', label: 'Actions', field: 'actions' }
]

const bindingCols = [
  { name: 'routingKey', label: 'Routing Key', field: 'routingKey' },
  { name: 'queue', label: 'Queue', field: 'queue' },
  { name: 'actions', label: 'Actions', field: 'actions' }
]

const types = ['direct', 'fanout', 'topic', 'headers']

const routingKey = ref('')
const queueName = ref('')

const pubRoutingKey = ref('')
const message = ref('')

function select(name) {
  store.select(name)
  store.fetchBindings(name)
}

function resetForm() {
  newExchange.value = {
    name: '',
    vhost: '/',
    type: 'direct',
    durable: false,
    auto_delete: false
  }
}

async function createExchange() {
  if (!newExchange.value.name) return
  
  creating.value = true
  try {
    await store.addExchange(newExchange.value)
    showCreateDialog.value = false
    resetForm()
  } finally {
    creating.value = false
  }
}

async function del(name) {
  await store.deleteExchange(name)
}

async function addBinding() {
  if (!store.selected) return
  await store.addBinding(store.selected, routingKey.value, queueName.value)
  routingKey.value = ''; queueName.value = ''
}

async function unbind(row) {
  await store.deleteBinding(store.selected, row.routingKey, row.queue)
}

async function publish() {
  if (!store.selected) return
  await store.publish(store.selected, pubRoutingKey.value, message.value)
  Notify.create({ type: 'positive', message: 'Message published!' })
  pubRoutingKey.value = ''; message.value = ''
}

onMounted(store.fetch)
</script>
