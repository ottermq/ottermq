<template>
  <q-layout view="lHh Lpr lFf">
    <q-header elevated class="bg-primary text-white">
      <q-toolbar>
        <q-toolbar-title class="row items-center q-pl-md">
          <div class="text-h4 text-weight-medium">OtterMQ</div> 
          <div v-if="overview.data" class="q-ml-md text-subtitle2">
            <span class="q-pa-xs q-ml-sm text-grey-2 text-weight-light ">
              {{ overview.data.broker.version }}
            </span>
            <span class="q-pa-xs q-ml-sm text-grey-2 text-weight-light">
              {{ goVersion }}
            </span>
          </div>
        </q-toolbar-title>

        <div class="q-mr-md text-subtitle1">
          User: <strong>{{username}}</strong>
        </div>
        <q-btn flat label="logout" @click="logout" />
      </q-toolbar>
    </q-header>

    <q-page-container>
      <q-tabs
        v-model="tab"
        class="bg-primary text-white"
        active-color="white"
        indicator-color="white"
        align="left"
        inline-label
        >
        <q-route-tab to="/overview" name="overview" label="Overview" />
        <q-route-tab to="/connections" name="connections" label="Connections" />
        <q-route-tab to="/channels" name="channels" label="Channels" />
        <q-route-tab to="/exchanges" name="exchanges" label="Exchanges" />
        <q-route-tab to="/queues" name="queues" label="Queues" />
      </q-tabs>

      <router-view />
    </q-page-container>
  </q-layout>
</template>

<script setup>
import { onMounted, ref, watch, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from 'src/stores/auth'
import { useOverviewStore } from 'src/stores/overview'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const overview = useOverviewStore()

const username = ref(auth.username || 'guest')


// Sync tab with route
const tab = ref(route.path.split('/')[1] || 'overview')
watch(() => route.path, (p) => {
  tab.value = (p.split('/')[1] || 'overview')
})



// Clean Go version: 
const goVersion = computed(() => {
  const full = overview.data?.broker.go_version
  if (!full) return ''
  const parts = full.split(' ')
  let ver = parts.length >= 2 ? parts[0] : full
  if (ver.startsWith('go')) {
    ver = ver.slice(2)
  }
  return `Go ${ver}`
})

// fetch overview data for header info
onMounted(() => {
  overview.fetch()
})


function logout() {
  auth.logout()
  router.push('/login')
}
</script>

<style scoped lang="scss">
header .container {
  max-width: 1200px;
  margin: 0 auto;
}

.small-square { 
    height: .7em; 
    width: .7em; 
    border: 1px solid lightslategray;
    padding: 2px; 
    margin-right: 4px; 
    display: inline-block; 
}
.small-square--green { background: #0f0}
.small-square--red { background: #c10015}
.q-toolbar-title {
  padding-left: 8px !important;
}
.q-toolbar span {
  font-family: Arial, Helvetica, sans-serif;
  font-size: smaller;
  border-radius: 4px;
  background-color: rgba(255, 255, 255, 0.3);
}
</style>
