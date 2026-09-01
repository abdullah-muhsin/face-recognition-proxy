<script setup>
import { useGatewayStore } from '../stores/gateway'
import {
  eventSeverity,
  formatShortTime,
  isAuthenticationError,
  statusLabel,
  statusSeverity,
} from '../lib/presentation'

const gateway = useGatewayStore()
const router = useRouter()
const route = useRoute()
const toast = useToast()
const latestMonitorEvents = computed(() => gateway.monitor.slice(0, 6))
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
</script>

<template>
  <div
    class="mb-7 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between"
  >
    <div>
      <p class="text-sm font-medium text-cyan-700">Gateway snapshot</p>
      <h1 class="mt-1 text-3xl font-semibold tracking-tight text-slate-950">
        Operations at a glance
      </h1>
      <p class="mt-2 text-sm text-slate-500">
        {{
          gateway.lastUpdatedAt
            ? `Last refreshed ${formatShortTime(gateway.lastUpdatedAt)}`
            : 'Loading the latest gateway state'
        }}
      </p>
    </div>
    <Button
      label="Refresh data"
      icon="pi pi-refresh"
      :loading="gateway.refreshing"
      @click="refresh"
    />
  </div>

  <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
    <Card class="border border-slate-200 shadow-sm"
      ><template #content
        ><div class="flex items-start justify-between">
          <div>
            <p class="text-sm font-medium text-slate-500">Device events</p>
            <p
              class="mt-3 text-3xl font-semibold tracking-tight text-slate-950"
            >
              {{ gateway.overview.deviceEventTotal }}
            </p>
            <p class="mt-2 text-xs text-slate-500">
              Raw source payloads retained
            </p>
          </div>
          <span
            class="grid h-11 w-11 place-items-center rounded-xl bg-cyan-50 text-cyan-700"
            ><i class="pi pi-code text-lg"
          /></span></div></template
    ></Card>
    <Card class="border border-slate-200 shadow-sm"
      ><template #content
        ><div class="flex items-start justify-between">
          <div>
            <p class="text-sm font-medium text-slate-500">
              Configured terminals
            </p>
            <p
              class="mt-3 text-3xl font-semibold tracking-tight text-slate-950"
            >
              {{ gateway.overview.terminals.length }}
            </p>
            <p class="mt-2 text-xs text-slate-500">Known PushSDK devices</p>
          </div>
          <span
            class="grid h-11 w-11 place-items-center rounded-xl bg-indigo-50 text-indigo-700"
            ><i class="pi pi-desktop text-lg"
          /></span></div></template
    ></Card>
    <Card class="border border-slate-200 shadow-sm"
      ><template #content
        ><div class="flex items-start justify-between">
          <div>
            <p class="text-sm font-medium text-slate-500">Online now</p>
            <p
              class="mt-3 text-3xl font-semibold tracking-tight text-slate-950"
            >
              {{ gateway.onlineTerminals }}
            </p>
            <p class="mt-2 text-xs text-slate-500">Active terminal sessions</p>
          </div>
          <span
            class="grid h-11 w-11 place-items-center rounded-xl bg-emerald-50 text-emerald-700"
            ><i class="pi pi-check-circle text-lg"
          /></span></div></template
    ></Card>
    <Card class="border border-slate-200 shadow-sm"
      ><template #content
        ><div class="flex items-start justify-between">
          <div>
            <p class="text-sm font-medium text-slate-500">Needs attention</p>
            <p
              class="mt-3 text-3xl font-semibold tracking-tight text-slate-950"
            >
              {{ gateway.terminalsWithErrors }}
            </p>
            <p class="mt-2 text-xs text-slate-500">Terminal error states</p>
          </div>
          <span
            class="grid h-11 w-11 place-items-center rounded-xl bg-amber-50 text-amber-700"
            ><i class="pi pi-exclamation-triangle text-lg"
          /></span></div></template
    ></Card>
  </div>

  <div class="mt-6 grid gap-6 xl:grid-cols-[1.35fr_0.65fr]">
    <Card class="border border-slate-200 shadow-sm">
      <template #title
        ><div class="flex items-center justify-between gap-4">
          <span>Terminal posture</span
          ><Button
            label="View terminals"
            icon="pi pi-arrow-right"
            icon-pos="right"
            text
            size="small"
            @click="router.push('/terminals')"
          /></div
      ></template>
      <template #subtitle
        >Connection status from the most recent PushSDK interaction.</template
      >
      <template #content>
        <DataTable
          v-if="gateway.overview.terminals.length"
          :value="gateway.overview.terminals"
          striped-rows
          class="text-sm"
        >
          <Column field="serialNumber" header="Terminal"
            ><template #body="{ data }"
              ><div>
                <p class="font-medium text-slate-900">
                  {{ data.serialNumber }}
                </p>
                <p class="mt-1 text-xs text-slate-500">
                  PushSDK {{ data.pushSdkSerial }}
                </p>
              </div></template
            ></Column
          >
          <Column header="Status"
            ><template #body="{ data }"
              ><Tag
                :value="statusLabel(data.status)"
                :severity="statusSeverity(data.status)"
                rounded /></template
          ></Column>
          <Column header="Last seen"
            ><template #body="{ data }">{{
              formatShortTime(data.lastSeenAt)
            }}</template></Column
          >
        </DataTable>
        <div
          v-else
          class="rounded-xl border border-dashed border-slate-200 bg-slate-50 px-6 py-12 text-center"
        >
          <i class="pi pi-desktop text-2xl text-slate-400" />
          <p class="mt-3 font-medium text-slate-700">No terminals configured</p>
          <p class="mt-1 text-sm text-slate-500">
            Add a terminal mapping before waiting for PushSDK registration.
          </p>
        </div>
      </template>
    </Card>
    <Card class="border border-slate-200 shadow-sm">
      <template #title
        ><div class="flex items-center justify-between gap-4">
          <span>Live activity</span
          ><Tag
            :value="monitorConnectionLabel"
            :severity="monitorConnectionSeverity"
            rounded
          /></div
      ></template>
      <template #subtitle>Most recent metadata-only gateway activity.</template>
      <template #content>
        <div v-if="latestMonitorEvents.length" class="space-y-4">
          <article
            v-for="(event, index) in latestMonitorEvents"
            :key="`${event.at}-${index}`"
            class="flex gap-3"
          >
            <span
              class="mt-1.5 h-2.5 w-2.5 shrink-0 rounded-full"
              :class="
                eventSeverity(event.kind) === 'danger'
                  ? 'bg-red-500'
                  : eventSeverity(event.kind) === 'success'
                    ? 'bg-emerald-500'
                    : 'bg-cyan-500'
              "
            />
            <div class="min-w-0">
              <p class="truncate text-sm font-medium text-slate-800">
                {{ event.kind }}
              </p>
              <p class="mt-1 text-sm leading-5 text-slate-500">
                {{ event.message }}
              </p>
              <p class="mt-1 text-xs text-slate-400">
                {{ formatShortTime(event.at) }} ·
                {{ event.terminal || 'Gateway' }}
              </p>
            </div>
          </article>
        </div>
        <div
          v-else
          class="rounded-xl border border-dashed border-slate-200 bg-slate-50 px-5 py-10 text-center"
        >
          <i class="pi pi-wave-pulse text-2xl text-slate-400" />
          <p class="mt-3 font-medium text-slate-700">Waiting for activity</p>
          <p class="mt-1 text-sm text-slate-500">
            The monitor will update as the gateway receives activity.
          </p>
        </div>
        <Button
          class="mt-5 w-full"
          label="Open live monitor"
          icon="pi pi-wave-pulse"
          outlined
          @click="router.push('/monitor')"
        />
      </template>
    </Card>
  </div>
</template>
