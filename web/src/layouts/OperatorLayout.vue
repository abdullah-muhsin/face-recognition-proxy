<script setup>
import { useGatewayStore } from '../stores/gateway'
import { isAuthenticationError } from '../lib/presentation'

const gateway = useGatewayStore()
const router = useRouter()
const route = useRoute()
const toast = useToast()
const mobileNavigationVisible = ref(false)

const navigation = [
  { to: '/overview', label: 'Overview', icon: 'pi pi-home' },
  { to: '/events', label: 'Device events', icon: 'pi pi-code' },
  { to: '/terminals', label: 'Terminals', icon: 'pi pi-desktop' },
  { to: '/monitor', label: 'Live monitor', icon: 'pi pi-wave-pulse' },
]

const title = computed(() => route.meta.label || 'PushSDK gateway')
const description = computed(
  () => route.meta.description || 'Operations workspace',
)
const operatorInitials = computed(() =>
  (gateway.operatorName || 'OP').slice(0, 2).toUpperCase(),
)
const monitorConnectionLabel = computed(
  () =>
    ({
      connected: 'Live connected',
      connecting: 'Connecting',
      reconnecting: 'Reconnecting',
      disconnected: 'Disconnected',
      error: 'Connection error',
    })[gateway.socketState] || 'Disconnected',
)
const monitorConnectionSeverity = computed(
  () =>
    ({
      connected: 'success',
      connecting: 'warn',
      reconnecting: 'warn',
      disconnected: 'secondary',
      error: 'danger',
    })[gateway.socketState] || 'secondary',
)

watch(
  () => gateway.authenticated,
  async (authenticated) => {
    if (!authenticated)
      await router.replace({
        name: 'login',
        query: { redirect: route.fullPath },
      })
  },
)

function navigate(destination) {
  mobileNavigationVisible.value = false
  router.push(destination)
}

async function refresh() {
  try {
    await gateway.refresh()
  } catch (error) {
    if (isAuthenticationError(error)) {
      await router.replace({
        name: 'login',
        query: { redirect: route.fullPath },
      })
      return
    }
    toast.add({
      severity: 'error',
      summary: 'Gateway request failed',
      detail: error.message,
      life: 5000,
    })
  }
}

async function signOut() {
  try {
    await gateway.signOut()
    toast.add({
      severity: 'success',
      summary: 'Signed out',
      detail: 'The operator session has ended.',
      life: 3000,
    })
    await router.replace({ name: 'login' })
  } catch (error) {
    toast.add({
      severity: 'error',
      summary: 'Sign out failed',
      detail: error.message,
      life: 5000,
    })
  }
}
</script>

<template>
  <main class="min-h-screen bg-slate-50 text-slate-900">
    <Drawer
      v-model:visible="mobileNavigationVisible"
      position="left"
      class="!w-80"
    >
      <template #header
        ><div class="flex items-center gap-3">
          <span
            class="grid h-10 w-10 place-items-center rounded-xl bg-slate-950 text-lg text-cyan-300"
            ><i class="pi pi-bolt"
          /></span>
          <div>
            <p class="font-semibold text-slate-950">PushSDK gateway</p>
            <p class="text-xs text-slate-500">Operations workspace</p>
          </div>
        </div></template
      >
      <nav class="space-y-1" aria-label="Operator console navigation">
        <Button
          v-for="item in navigation"
          :key="item.to"
          text
          class="w-full !justify-start"
          :class="
            route.path === item.to
              ? '!bg-slate-100 !text-slate-950'
              : '!text-slate-600'
          "
          :icon="item.icon"
          :label="item.label"
          @click="navigate(item.to)"
        />
      </nav>
      <Divider />
      <div class="rounded-xl bg-slate-50 p-4">
        <div class="mb-2 flex items-center gap-2">
          <i class="pi pi-shield text-cyan-700" /><span
            class="text-sm font-semibold text-slate-800"
            >Privacy boundary</span
          >
        </div>
        <p class="text-xs leading-5 text-slate-500">
          Protocol payloads, images, and credentials are not sent to this
          interface.
        </p>
      </div>
    </Drawer>

    <aside
      class="fixed inset-y-0 left-0 hidden w-72 flex-col border-r border-slate-200 bg-white px-4 py-6 lg:flex"
    >
      <div class="mb-9 flex items-center gap-3 px-2">
        <span
          class="grid h-11 w-11 place-items-center rounded-2xl bg-slate-950 text-xl text-cyan-300"
          ><i class="pi pi-bolt"
        /></span>
        <div>
          <p class="font-semibold tracking-tight text-slate-950">
            PushSDK gateway
          </p>
          <p class="text-xs text-slate-500">Operations workspace</p>
        </div>
      </div>
      <nav class="space-y-1" aria-label="Operator console navigation">
        <Button
          v-for="item in navigation"
          :key="item.to"
          text
          class="w-full !justify-start"
          :class="
            route.path === item.to
              ? '!bg-slate-100 !text-slate-950'
              : '!text-slate-600'
          "
          :icon="item.icon"
          :label="item.label"
          @click="navigate(item.to)"
        />
      </nav>
      <div class="mt-auto rounded-2xl bg-slate-950 p-5 text-slate-100">
        <div class="mb-3 flex items-center gap-2 text-cyan-300">
          <i class="pi pi-shield" /><span class="text-sm font-semibold"
            >Privacy boundary</span
          >
        </div>
        <p class="text-xs leading-5 text-slate-400">
          Protocol payloads, images, and credentials remain outside this
          interface.
        </p>
      </div>
    </aside>

    <section class="lg:pl-72">
      <header
        class="sticky top-0 z-20 border-b border-slate-200 bg-slate-50/90 px-4 py-4 backdrop-blur sm:px-6 lg:px-10"
      >
        <div class="mx-auto flex max-w-7xl items-center justify-between gap-4">
          <div class="flex min-w-0 items-center gap-3">
            <Button
              icon="pi pi-bars"
              text
              rounded
              class="lg:!hidden"
              aria-label="Open navigation"
              @click="mobileNavigationVisible = true"
            />
            <div class="min-w-0">
              <p class="truncate text-sm font-semibold text-slate-950">
                {{ title }}
              </p>
              <p class="hidden truncate text-xs text-slate-500 sm:block">
                {{ description }}
              </p>
            </div>
          </div>
          <div class="flex items-center gap-2 sm:gap-3">
            <Tag
              :value="monitorConnectionLabel"
              :severity="monitorConnectionSeverity"
              rounded
              class="hidden sm:inline-flex"
            /><Button
              icon="pi pi-refresh"
              text
              rounded
              aria-label="Refresh console"
              :loading="gateway.refreshing"
              @click="refresh"
            /><Avatar
              :label="operatorInitials"
              shape="circle"
              class="!bg-slate-950 !text-xs !font-semibold !text-cyan-300"
            /><Button
              label="Sign out"
              icon="pi pi-sign-out"
              text
              class="hidden sm:inline-flex"
              @click="signOut"
            />
          </div>
        </div>
      </header>
      <section class="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-10 lg:py-8">
        <RouterView />
      </section>
    </section>
  </main>
</template>
