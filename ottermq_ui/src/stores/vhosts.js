import { defineStore } from 'pinia'
import api from 'src/services/api'

export const useVHostsStore = defineStore('vhosts', {
  state: () => ({
    items: [],
    selected: '/',
    loading: false,
    error: null,
  }),
  getters: {
    names: (state) => state.items.map(v => v.name),
  },
  actions: {
    async fetch() {
      this.loading = true
      this.error = null
      try {
        const { data } = await api.get('/vhosts')
        this.items = Array.isArray(data?.vhosts) ? data.vhosts : []
        if (!this.names.includes(this.selected)) {
          this.selected = this.names.includes('/') ? '/' : (this.names[0] ?? '/')
        }
      } catch (err) {
        this.error = err?.response?.data?.error || err.message
        this.items = []
      } finally {
        this.loading = false
      }
    },
    setSelected(name) {
      this.selected = name
    },
  },
})
