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
      component: () => import('../layouts/OperatorLayout.vue'),
      meta: { requiresAuth: true },
      children: [
        { path: '', redirect: { name: 'overview' } },
        {
          path: 'overview',
          name: 'overview',
          component: () => import('../views/OverviewView.vue'),
          meta: {
            label: 'Overview',
            description: 'Gateway posture at a glance',
          },
        },
        {
          path: 'events',
          name: 'events',
          component: () => import('../views/DeviceEventsView.vue'),
          meta: {
            label: 'Device events',
            description: 'Exact PushSDK event payloads received from terminals',
          },
        },
        {
          path: 'terminals',
          name: 'terminals',
          component: () => import('../views/TerminalsView.vue'),
          meta: {
            label: 'Terminals',
            description: 'Connected PushSDK devices',
          },
        },
        {
          path: 'monitor',
          name: 'monitor',
          component: () => import('../views/MonitorView.vue'),
          meta: {
            label: 'Live monitor',
            description: 'Metadata-only gateway activity',
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
  if (to.meta.guestOnly && gateway.authenticated) return { name: 'overview' }
  return true
})

export default router
