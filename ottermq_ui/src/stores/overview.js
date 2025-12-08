import { defineStore } from 'pinia'
import api from 'src/services/api'

export const useOverviewStore = defineStore('overview', {
  state: () => ({
    data: null,
    chartsData: null,
    loading: false,
    error: null,
    lastUpdate: null
  }),

  getters: {
    brokerVersion: (state) => `${state.data?.broker?.product || ''} ${state.data?.broker?.version || ''}`.trim(),
    nodeName: (state) => state.data?.node?.name || '-',
    uptime: (state) => {
      const secs = state.data?.node?.uptime_secs
      if (!secs) return '-'
      const d = Math.floor(secs / 86400)
      const h = Math.floor((secs % 86400) / 3600)
      const m = Math.floor((secs % 3600) / 60)
      return `${d}d ${h}h ${m}m`
    },
    totalMessages: (state) => state.data?.message_stats?.messages_total || 0,
    readyMessages: (state) => state.data?.message_stats?.messages_ready || 0,
    unackedMessages: (state) => state.data?.message_stats?.messages_unacknowledged || 0
  },

  actions: {
    async fetch() {
      this.loading = true
      this.error = null
      try {
        const { data } = await api.get('/overview')
        this.data = data
        this.lastUpdate = new Date()
      } catch (e) {
        this.error = e?.response?.data?.error || e.message
        this.data = null
      } finally {
        this.loading = false
      }
    },

    async fetchCharts() {
      try {
        const { data } = await api.get('/overview/charts')
        this.chartsData = data
      } catch (e) {
        console.error('Failed to fetch chart data:', e)
      }
    }
  }
})