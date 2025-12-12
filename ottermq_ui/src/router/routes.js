const routes = [
  
  {
    path: '/',
    component: () => import('layouts/MainLayout.vue'),
    children: [
      { path: '', component: () => import('pages/OverviewPage.vue'), meta: { requiresAuth: true } },
      { path: 'overview', component: () => import('pages/OverviewPage.vue'), meta: { requiresAuth: true } },
      { path: 'connections', component: () => import('pages/ConnectionsPage.vue'), meta: { requiresAuth: true } },
      { path: 'channels', component: () => import('pages/ChannelsPage.vue'), meta: { requiresAuth: true } },
      { path: 'exchanges', component: () => import('pages/ExchangesPage.vue'), meta: { requiresAuth: true } },
      { path: 'queues', component: () => import('pages/QueuesPage.vue'), meta: { requiresAuth: true } },
      { path: 'login', component: () => import('pages/LoginPage.vue'), meta: { requiresAuth: false } },
    ],
  },

  // Always leave this as last one,
  // but you can also remove it
  {
    path: '/:catchAll(.*)*',
    component: () => import('pages/ErrorNotFound.vue'),
  },
]

export default routes
