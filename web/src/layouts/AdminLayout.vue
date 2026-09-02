<script setup>
import { useGatewayStore } from '../stores/gateway'
import { isAuthenticationError, monitorConnection } from '../lib/presentation'

const gateway = useGatewayStore()
const router = useRouter()
const route = useRoute()
const toast = useToast()
const mobileNavigationVisible = ref(false)

const navigation = [
  { to: '/board', label: 'Gateway board', icon: 'pi pi-table' },
  { to: '/events', label: 'Event archive', icon: 'pi pi-code' },
  { to: '/terminals', label: 'Terminal registry', icon: 'pi pi-server' },
  { to: '/activity', label: 'Gateway activity', icon: 'pi pi-list' },
]

const title = computed(() => route.meta.label || 'PushSDK gateway')
const administratorInitials = computed(() =>
  (gateway.administratorName || 'AD').slice(0, 2).toUpperCase(),
)
const connection = computed(() => monitorConnection(gateway.socketState))

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
      detail: 'The administrator session has ended.',
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
  <main class="min-h-screen bg-slate-100 text-slate-900">
    <Drawer
      v-model:visible="mobileNavigationVisible"
      position="left"
      class="!w-80"
    >
      <template #header>
        <div class="flex items-center gap-3">
          <span
            class="grid h-10 w-10 place-items-center rounded-lg bg-slate-950 text-lg text-cyan-300"
            ><i class="pi pi-shield"
          /></span>
          <div>
            <p class="font-semibold text-slate-950">PushSDK gateway</p>
            <p class="text-xs text-slate-500">Administration console</p>
          </div>
        </div>
      </template>
      <nav class="space-y-1" aria-label="Administration navigation">
        <Button
          v-for="item in navigation"
          :key="item.to"
          text
          class="w-full !justify-start"
          :class="
            route.path === item.to
              ? '!bg-slate-200 !text-slate-950'
              : '!text-slate-600'
          "
          :icon="item.icon"
          :label="item.label"
          @click="navigate(item.to)"
        />
      </nav>
    </Drawer>

    <aside
      class="fixed inset-y-0 left-0 hidden w-60 flex-col border-r border-slate-300 bg-slate-950 px-3 py-4 text-slate-100 lg:flex"
    >
      <div class="mb-6 flex items-center gap-3 px-2">
        <span
          class="grid h-9 w-9 place-items-center rounded-lg bg-cyan-300 text-lg text-slate-950"
          ><i class="pi pi-shield"
        /></span>
        <div>
          <p class="font-semibold tracking-tight">PushSDK gateway</p>
          <p class="text-xs text-slate-400">Administration console</p>
        </div>
      </div>
      <nav class="space-y-1" aria-label="Administration navigation">
        <Button
          v-for="item in navigation"
          :key="item.to"
          text
          class="w-full !justify-start"
          :class="
            route.path === item.to
              ? '!bg-white/15 !text-cyan-200'
              : '!text-slate-300 hover:!bg-white/5 hover:!text-white'
          "
          :icon="item.icon"
          :label="item.label"
          @click="navigate(item.to)"
        />
      </nav>
      <div class="mt-auto border-t border-white/10 px-2 pt-4">
        <div
          class="flex items-center gap-1 text-xs font-medium uppercase tracking-wide text-slate-500"
        >
          Data boundary
          <Button
            v-tooltip.right="
              'Payloads stay in the event archive. Credentials are never exposed in this console.'
            "
            icon="pi pi-info-circle"
            text
            rounded
            size="small"
            class="!h-5 !w-5 !p-0 !text-slate-400 hover:!text-cyan-200"
            aria-label="About the data boundary"
          />
        </div>
      </div>
    </aside>

    <section class="lg:pl-60">
      <header
        class="sticky top-0 z-20 border-b border-slate-300 bg-slate-100/95 px-4 py-3 backdrop-blur sm:px-6 lg:px-8"
      >
        <div
          class="mx-auto flex max-w-[100rem] items-center justify-between gap-4"
        >
          <div class="flex min-w-0 items-center gap-3">
            <Button
              icon="pi pi-bars"
              text
              rounded
              class="lg:!hidden"
              aria-label="Open navigation"
              @click="mobileNavigationVisible = true"
            />
            <div class="flex min-w-0 items-center gap-1">
              <h1 class="truncate text-sm font-semibold text-slate-950">
                {{ title }}
              </h1>
              <Button
                v-tooltip.bottom="
                  route.meta.description || 'Administrative gateway visibility'
                "
                icon="pi pi-info-circle"
                text
                rounded
                size="small"
                class="!h-6 !w-6 !shrink-0 !p-0 !text-slate-500 hover:!text-slate-950"
                aria-label="About this view"
              />
            </div>
          </div>
          <div class="flex items-center gap-2">
            <Tag
              :value="connection.label"
              :severity="connection.severity"
              rounded
              class="hidden sm:inline-flex"
            />
            <Button
              icon="pi pi-refresh"
              text
              rounded
              aria-label="Refresh administrative data"
              :loading="gateway.refreshing"
              @click="refresh"
            />
            <Avatar
              :label="administratorInitials"
              shape="circle"
              class="!bg-slate-950 !text-xs !font-semibold !text-cyan-300"
            />
            <Button
              label="Sign out"
              icon="pi pi-sign-out"
              text
              class="hidden sm:inline-flex"
              @click="signOut"
            />
          </div>
        </div>
      </header>
      <section class="mx-auto max-w-[100rem] px-4 py-5 sm:px-6 lg:px-8">
        <RouterView />
      </section>
    </section>
  </main>
</template>
