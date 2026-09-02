import { defineStore } from 'pinia'
import api from 'src/services/api'

export const useAdminStore = defineStore('admin', {
  state: () => ({
    users: [],
    permissions: [],
    loading: false,
    error: null,
  }),
  actions: {
    async fetchUsers() {
      this.loading = true
      this.error = null
      try {
        const { data } = await api.get('/admin/users')
        this.users = Array.isArray(data?.users) ? data.users : []
      } catch (err) {
        this.error = err?.response?.data?.error || err.message
        this.users = []
      } finally {
        this.loading = false
      }
    },
    async addUser(username, password, confirmPassword, role) {
      await api.post('/admin/users', {
        username,
        password,
        confirm_password: confirmPassword,
        role,
      })
      await this.fetchUsers()
    },
    async deleteUser(username) {
      await api.delete(`/admin/users/${encodeURIComponent(username)}`)
      await this.fetchUsers()
    },
    async changePassword(username, password, confirmPassword) {
      await api.put(`/admin/users/${encodeURIComponent(username)}/password`, {
        password,
        confirm_password: confirmPassword,
      })
    },
    async fetchPermissions() {
      this.loading = true
      this.error = null
      try {
        const { data } = await api.get('/admin/permissions')
        this.permissions = Array.isArray(data?.permissions) ? data.permissions : []
      } catch (err) {
        this.error = err?.response?.data?.error || err.message
        this.permissions = []
      } finally {
        this.loading = false
      }
    },
    async grantPermission(vhost, username) {
      await api.put(`/admin/permissions/${encodeURIComponent(vhost)}/${encodeURIComponent(username)}`)
      await this.fetchPermissions()
    },
    async revokePermission(vhost, username) {
      await api.delete(`/admin/permissions/${encodeURIComponent(vhost)}/${encodeURIComponent(username)}`)
      await this.fetchPermissions()
    },
  },
})
