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
    async addQueue (name) {
        await api.post('/queues', {
          name: name,
          vhost: "/",
          Durable: false,
          AutoDelete: false,
          Exclusive: false,
          NoWait: false,
          Arguments: {}
        })
        await this.fetch()
    },
    async deleteQueue (name) {
        await api.delete(`/queues/${encodeURIComponent(name)}`)
        await this.fetch()
    },
    async consume(queue) {
        const {data} = await api.post(`/queues/${encodeURIComponent(queue)}/consume`)
        this.lastMessage = data?.message ?? null
        await this.fetch()
        return this.lastMessage
    },
    select(queue) {
      this.selected = queue
    }
  },
})
