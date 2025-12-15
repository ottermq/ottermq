import { defineStore } from 'pinia'
import api from 'src/services/api'

export const useExchangesStore = defineStore('exchanges', {
  state: () => ({
    items: [],
    bindings: [],
    loading: false,
    error: null,
    selected: null,
  }),
  actions: {
    async fetch() {
      this.loading = true; 
      this.error = null
      try {
        const {data} = await api.get('/exchanges')
        this.items = Array.isArray(data?.exchanges) ? data.exchanges : []
        console.log('Fetched exchanges:', this.items)
        // Sort by vhost -> type -> name (groups by vhost, then type, then name)
        this.items.sort((a, b) => {
          // First: vhost (case-insensitive)
          const vhostCompare = a.vhost.localeCompare(b.vhost, undefined, { sensitivity: 'base' })
          if (vhostCompare !== 0) return vhostCompare
          
          // Second: type (case-insensitive)
          const typeCompare = a.type.localeCompare(b.type, undefined, { sensitivity: 'base' })
          if (typeCompare !== 0) return typeCompare
          
          // Third: name (case-insensitive)
          return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
        })
        console.log('Sorted exchanges:', this.items)
      } catch (err) {
        this.error = err?.response?.data?.error || err.message
        this.items = []
      } finally { 
        this.loading = false 
      }
    },
    async addExchange(exchangeData) {
      const payload = {
        type: exchangeData.type || 'direct',
        passive: false,
        durable: exchangeData.durable || false,
        auto_delete: exchangeData.auto_delete || false,
        no_wait: false,
        arguments: {}
      }
      
      const vhost = exchangeData.vhost || '/'
      const encodedVhost = encodeURIComponent(vhost)
      const encodedName = encodeURIComponent(exchangeData.name)
      
      await api.post(`/exchanges/${encodedVhost}/${encodedName}`, payload)
      await this.fetch()
    },
    async deleteExchange(name) {
      const vhost = '/'
      const encodedVhost = encodeURIComponent(vhost)
      const encodedName = encodeURIComponent(name)
      await api.delete(`/exchanges/${encodedVhost}/${encodedName}`)
      await this.fetch()
    },
    async fetchBindings(exchange) {
        const vhost = '/'
        const encodedVhost = encodeURIComponent(vhost)
        const encodedExchange = encodeURIComponent(exchange)
        const {data} = await api.get(`/bindings/${encodedVhost}/${encodedExchange}`)
        const list = Array.isArray(data?.bindings) 
        ? data.bindings.map(b => ({
            source: b.source,
            destination_type: b.destination_type,
            queue: b.destination,
            routingKey: b.routing_key,
            arguments: b.arguments,
            propertiesKey: b.properties_key,
        })) : []
        this.bindings = list
    },
    async addBinding(exchange, routingKey, queue) {
      await api.post(`/bindings`, {
        vhost: '/',
        source: exchange, 
        routing_key: routingKey, 
        destination: queue,
        arguments: {}
      })
      await this.fetchBindings(exchange)
    },
    async deleteBinding(exchange, routingKey, queue) {
      await api.delete(`/bindings`, { data: { 
        vhost: "/",
        source: exchange, 
        routing_key: routingKey, 
        destination: queue,
        arguments: {},
      } })
      await this.fetchBindings(exchange)
    },
    async publish(exchange, routingKey, message) {
      const vhost = '/'
      const encodedVhost = encodeURIComponent(vhost)
      const encodedExchange = encodeURIComponent(exchange)
      
      await api.post(`/messages/${encodedVhost}/${encodedExchange}`, {
        routing_key: routingKey, 
        payload: message,
        content_type: "text/plain",
        content_encoding: "utf-8",
        delivery_mode: 1, // 1 - transient, 2 - persistent
        priority: 0,
        correlation_id: "",
        reply_to: "",
        expiration: "",
        message_id: "",
        timestamp: null,
        type: "",
        user_id: "",
        app_id: "",
        headers: null,
        mandatory: false,
        immediate: false,
      })
    },
    select(exchange) { this.selected = exchange },
  }
})
