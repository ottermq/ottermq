import { defineStore } from "pinia"
import api from "src/services/api"

export const useChannelsStore = defineStore("channels", {
  state: () => ({
    items: [],
    loading: false,
    error: null,
  }),
  actions: {
    async fetch() {
      this.loading = true;
      this.error = null
      try {
        const {data} = await api.get("/channels")
        this.items = Array.isArray(data?.channels) ? data.channels : []
        // Sort by vhost -> connection_name -> number (channel number)
        this.items.sort((a, b) => {
          // First: vhost (case-insensitive asc)
          const vhostCompare = (a.vhost || '').toLowerCase().localeCompare((b.vhost || '').toLowerCase())
          if (vhostCompare !== 0) return vhostCompare
          
          // Second: connection_name (case-insensitive asc)
          const connNameCompare = (a.connection_name || '').toLowerCase().localeCompare((b.connection_name || '').toLowerCase())
          if (connNameCompare !== 0) return connNameCompare
          
          // Third: channel number (asc)
          return (a.number || 0) - (b.number || 0)
        })
      } catch (e) {
        this.error = e?.response?.data?.error || e.message
        this.items = []
      } finally { this.loading = false }
    }
  }
})
