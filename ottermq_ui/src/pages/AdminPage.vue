<template>
  <q-page padding>
    <div class="q-mb-lg">
      <div class="text-h5 q-mb-md">Users</div>

      <q-table
        :rows="store.users"
        :columns="userColumns"
        row-key="username"
        flat
        bordered
        :loading="store.loading"
      >
        <template #top-right>
          <q-btn color="primary" label="Add User" icon="add" @click="openAddUser" />
        </template>

        <template #body-cell-role="props">
          <q-td :props="props">
            <q-badge :color="props.value === 'admin' ? 'negative' : 'primary'" :label="props.value" />
          </q-td>
        </template>

        <template #body-cell-actions="props">
          <q-td :props="props" class="q-gutter-xs">
            <q-btn
              flat dense round icon="key" size="sm" color="warning"
              @click="openChangePassword(props.row.username)"
            >
              <q-tooltip>Change password</q-tooltip>
            </q-btn>
            <q-btn
              flat dense round icon="delete" size="sm" color="negative"
              :disable="props.row.username === authStore.username"
              @click="confirmDeleteUser(props.row.username)"
            >
              <q-tooltip>Delete user</q-tooltip>
            </q-btn>
          </q-td>
        </template>
      </q-table>
    </div>

    <div>
      <div class="text-h5 q-mb-md">Permissions</div>

      <q-table
        :rows="store.permissions"
        :columns="permColumns"
        row-key="id"
        flat
        bordered
        :loading="store.loading"
      >
        <template #top-right>
          <q-btn color="primary" label="Grant Permission" icon="add" @click="openGrantPermission" />
        </template>

        <template #body-cell-actions="props">
          <q-td :props="props">
            <q-btn
              flat dense round icon="delete" size="sm" color="negative"
              @click="confirmRevokePermission(props.row.vhost, props.row.username)"
            >
              <q-tooltip>Revoke</q-tooltip>
            </q-btn>
          </q-td>
        </template>
      </q-table>
    </div>

    <!-- Add User Dialog -->
    <q-dialog v-model="addUserDialog">
      <q-card style="min-width: 380px">
        <q-card-section><div class="text-h6">Add User</div></q-card-section>
        <q-card-section class="q-gutter-sm">
          <q-input v-model="newUser.username" label="Username" outlined dense :rules="[v => !!v || 'Required']" />
          <q-input v-model="newUser.password" label="Password" type="password" outlined dense :rules="[v => !!v || 'Required']" />
          <q-input v-model="newUser.confirmPassword" label="Confirm Password" type="password" outlined dense :rules="[v => v === newUser.password || 'Passwords do not match']" />
          <q-select
            v-model="newUser.role"
            :options="roleOptions"
            label="Role"
            outlined dense
            emit-value map-options
          />
        </q-card-section>
        <q-card-actions align="right">
          <q-btn flat label="Cancel" v-close-popup />
          <q-btn color="primary" label="Add" @click="submitAddUser" />
        </q-card-actions>
      </q-card>
    </q-dialog>

    <!-- Change Password Dialog -->
    <q-dialog v-model="changePasswordDialog">
      <q-card style="min-width: 380px">
        <q-card-section><div class="text-h6">Change Password — {{ changePwTarget }}</div></q-card-section>
        <q-card-section class="q-gutter-sm">
          <q-input v-model="changePw.password" label="New Password" type="password" outlined dense :rules="[v => !!v || 'Required']" />
          <q-input v-model="changePw.confirmPassword" label="Confirm Password" type="password" outlined dense :rules="[v => v === changePw.password || 'Passwords do not match']" />
        </q-card-section>
        <q-card-actions align="right">
          <q-btn flat label="Cancel" v-close-popup />
          <q-btn color="primary" label="Save" @click="submitChangePassword" />
        </q-card-actions>
      </q-card>
    </q-dialog>

    <!-- Grant Permission Dialog -->
    <q-dialog v-model="grantPermDialog">
      <q-card style="min-width: 380px">
        <q-card-section><div class="text-h6">Grant Permission</div></q-card-section>
        <q-card-section class="q-gutter-sm">
          <q-select
            v-model="newPerm.username"
            :options="usernames"
            label="User"
            outlined dense
            :rules="[v => !!v || 'Required']"
          />
          <q-select
            v-model="newPerm.vhost"
            :options="vhostsStore.names"
            label="VHost"
            outlined dense
            :rules="[v => !!v || 'Required']"
          />
        </q-card-section>
        <q-card-actions align="right">
          <q-btn flat label="Cancel" v-close-popup />
          <q-btn color="primary" label="Grant" @click="submitGrantPermission" />
        </q-card-actions>
      </q-card>
    </q-dialog>
  </q-page>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useQuasar } from 'quasar'
import { useAdminStore } from 'src/stores/admin'
import { useAuthStore } from 'src/stores/auth'
import { useVHostsStore } from 'src/stores/vhosts'

const $q = useQuasar()
const store = useAdminStore()
const authStore = useAuthStore()
const vhostsStore = useVHostsStore()

onMounted(() => {
  store.fetchUsers()
  store.fetchPermissions()
  vhostsStore.fetch()
})

// --- Tables ---

const userColumns = [
  { name: 'username', label: 'Username', field: 'username', align: 'left', sortable: true },
  { name: 'role', label: 'Role', field: 'role', align: 'left' },
  { name: 'actions', label: '', field: 'actions', align: 'right' },
]

const permColumns = [
  { name: 'username', label: 'User', field: 'username', align: 'left', sortable: true },
  { name: 'vhost', label: 'VHost', field: 'vhost', align: 'left', sortable: true },
  { name: 'actions', label: '', field: 'actions', align: 'right' },
]

const usernames = computed(() => store.users.map(u => u.username))
const roleOptions = [
  { label: 'Admin', value: 1 },
  { label: 'Viewer', value: 2 },
]

// --- Add User ---

const addUserDialog = ref(false)
const newUser = ref({ username: '', password: '', confirmPassword: '', role: 2 })

function openAddUser() {
  newUser.value = { username: '', password: '', confirmPassword: '', role: 2 }
  addUserDialog.value = true
}

async function submitAddUser() {
  try {
    await store.addUser(newUser.value.username, newUser.value.password, newUser.value.confirmPassword, newUser.value.role)
    addUserDialog.value = false
    $q.notify({ type: 'positive', message: 'User created' })
  } catch (e) {
    $q.notify({ type: 'negative', message: e?.response?.data?.error || e.message })
  }
}

// --- Delete User ---

function confirmDeleteUser(username) {
  $q.dialog({
    title: 'Delete User',
    message: `Delete user "${username}"?`,
    cancel: true,
    persistent: true,
  }).onOk(async () => {
    try {
      await store.deleteUser(username)
      $q.notify({ type: 'positive', message: 'User deleted' })
    } catch (e) {
      $q.notify({ type: 'negative', message: e?.response?.data?.error || e.message })
    }
  })
}

// --- Change Password ---

const changePasswordDialog = ref(false)
const changePwTarget = ref('')
const changePw = ref({ password: '', confirmPassword: '' })

function openChangePassword(username) {
  changePwTarget.value = username
  changePw.value = { password: '', confirmPassword: '' }
  changePasswordDialog.value = true
}

async function submitChangePassword() {
  try {
    await store.changePassword(changePwTarget.value, changePw.value.password, changePw.value.confirmPassword)
    changePasswordDialog.value = false
    $q.notify({ type: 'positive', message: 'Password updated' })
  } catch (e) {
    $q.notify({ type: 'negative', message: e?.response?.data?.error || e.message })
  }
}

// --- Grant Permission ---

const grantPermDialog = ref(false)
const newPerm = ref({ username: '', vhost: '' })

function openGrantPermission() {
  newPerm.value = { username: '', vhost: vhostsStore.selected }
  grantPermDialog.value = true
}

async function submitGrantPermission() {
  try {
    await store.grantPermission(newPerm.value.vhost, newPerm.value.username)
    grantPermDialog.value = false
    $q.notify({ type: 'positive', message: 'Permission granted' })
  } catch (e) {
    $q.notify({ type: 'negative', message: e?.response?.data?.error || e.message })
  }
}

// --- Revoke Permission ---

function confirmRevokePermission(vhost, username) {
  $q.dialog({
    title: 'Revoke Permission',
    message: `Remove ${username}'s access to "${vhost}"?`,
    cancel: true,
    persistent: true,
  }).onOk(async () => {
    try {
      await store.revokePermission(vhost, username)
      $q.notify({ type: 'positive', message: 'Permission revoked' })
    } catch (e) {
      $q.notify({ type: 'negative', message: e?.response?.data?.error || e.message })
    }
  })
}
</script>
