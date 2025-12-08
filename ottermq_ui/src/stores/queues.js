import { defineStore } from 'pinia'
import api from 'src/services/api'

export const useQueuesStore = defineStore('queues', {
  state: () => ({
items: [],
loading: false,
error: null,
selected: null,
lastMessage: null,
  }),
  actions: {
    async fetch() {
      this.loading = true;
      this.error = null;
      try {
        const {data} = await api.get('/queues')
        this.items = Array.isArray(data?.queues) ? data.queues : []
      } catch (err) {
        this.error = err?.response?.data?.error || err.message
        this.items = []
      } finally {
        this.loading = false
      }
    },
    async addQueue (queueData) {
        // Build the request payload matching CreateQueueRequest structure
        const payload = {
          passive: false,
          durable: queueData.durable || false,
          auto_delete: queueData.auto_delete || false,
          arguments: {}
        }

        // Add optional fields only if they have values
        if (queueData.max_length != null && queueData.max_length > 0) {
          payload.max_length = queueData.max_length
        }

        if (queueData.message_ttl != null && queueData.message_ttl > 0) {
          payload.message_ttl = queueData.message_ttl
        }

        if (queueData.x_dead_letter_exchange) {
          payload['x-dead-letter-exchange'] = queueData.x_dead_letter_exchange
        }

        if (queueData.x_dead_letter_routing_key) {
          payload['x-dead-letter-routing-key'] = queueData.x_dead_letter_routing_key
        }

        const vhost = queueData.vhost || '/'
        const encodedVhost = encodeURIComponent(vhost)
        const encodedName = encodeURIComponent(queueData.name)
        
        await api.post(`/queues/${encodedVhost}/${encodedName}`, payload)
        await this.fetch()
    },
    async deleteQueue (name) {
        const vhost = '/'
        const encodedVhost = encodeURIComponent(vhost)
        const encodedName = encodeURIComponent(name)
        await api.delete(`/queues/${encodedVhost}/${encodedName}`)
        await this.fetch()
    },
    async get(queue) {
        const {data} = await api.post(`/queues/${encodeURIComponent(queue)}/get`, {
          queue: queue,
          vhost: "/",
          ackMode: "ack", // "ack" | "noack" | "reject" | "reject_requeue"
        })
        this.lastMessage = data?.message ?? null
        await this.fetch()
        return this.lastMessage
    },
    select(queue) {
      this.selected = queue
    }
  },
})
