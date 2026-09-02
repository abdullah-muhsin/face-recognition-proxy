import { createRouter, createWebHistory } from 'vue-router'
import { useGatewayStore } from '../stores/gateway'

const router = createRouter({
  history: createWebHistory('/app/'),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
      meta: { guestOnly: true },
    },
    {
      path: '/',
      component: () => import('../layouts/AdminLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        { path: '', redirect: { name: 'board' } },
        {
          path: 'board',
          name: 'board',
          component: () => import('../views/GatewayBoardView.vue'),
          meta: {
            label: 'Gateway board',
            description: 'Administrative gateway visibility',
          },
        },
        {
          path: 'events',
          name: 'events',
          component: () => import('../views/DeviceEventsView.vue'),
          meta: {
            label: 'Event archive',
            description: 'Exact PushSDK payloads retained from terminals',
          },
        },
        {
          path: 'terminals',
          name: 'terminals',
          component: () => import('../views/TerminalsView.vue'),
          meta: {
            label: 'Terminal registry',
            description: 'Configured PushSDK terminal state',
          },
        },
        {
          path: 'activity',
          name: 'activity',
          component: () => import('../views/GatewayActivityView.vue'),
          meta: {
            label: 'Gateway activity',
            description: 'Retained protocol and administration activity',
          },
        },
      ],
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('../views/NotFoundView.vue'),
    },
  ],
})

router.beforeEach(async (to) => {
  const gateway = useGatewayStore()
  await gateway.initialize()

  if (
    to.matched.some((record) => record.meta.requiresAuth) &&
    !gateway.authenticated
  ) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }
  if (to.meta.guestOnly && gateway.authenticated) return { name: 'board' }
  return true
})

export default router
