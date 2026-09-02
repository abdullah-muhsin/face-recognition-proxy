<script setup>
import { useGatewayStore } from '../stores/gateway'
import {
  formatTime,
  isAuthenticationError,
  matchesTerminal,
  statusLabel,
  statusSeverity,
} from '../lib/presentation'

const gateway = useGatewayStore()
const router = useRouter()
const route = useRoute()
const toast = useToast()
const filter = ref('')
const syncRuns = ref({})
const syncTimers = new Map()

onBeforeUnmount(() => {
  for (const timer of syncTimers.values()) window.clearTimeout(timer)
  syncTimers.clear()
})
const displayedTerminals = computed(() =>
  gateway.overview.terminals.filter((terminal) =>
    matchesTerminal(terminal, filter.value),
  ),
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

function syncRunFor(terminal) {
  return syncRuns.value[terminal.serialNumber] ?? terminal.latestAccessEventSync ?? null
}

function syncIsActive(terminal) {
  const run = syncRunFor(terminal)
  return run?.status === 'awaiting_time' || run?.status === 'running'
}

function syncSummary(terminal) {
  const run = syncRunFor(terminal)
  if (!run) return '—'
  if (run.status === 'awaiting_time') return 'Waiting for terminal time'
  if (run.status === 'running') {
    const total = run.totalMatches ?? '…'
    return `${run.pagesCompleted} pages · ${run.recordsImported + run.recordsDuplicate}/${total}`
  }
  if (run.status === 'completed') {
    return `${run.recordsImported} imported · ${run.recordsDuplicate} already retained`
  }
  return run.failure || 'Stopped'
}

function trackSync(run) {
  syncRuns.value = { ...syncRuns.value, [run.terminalSerialNumber]: run }
  if (run.status !== 'awaiting_time' && run.status !== 'running') return
  const prior = syncTimers.get(run.terminalSerialNumber)
  if (prior) window.clearTimeout(prior)
  syncTimers.set(
    run.terminalSerialNumber,
    window.setTimeout(async () => {
      syncTimers.delete(run.terminalSerialNumber)
      try {
        trackSync(await gateway.loadAccessEventSync(run.uuid))
        await gateway.loadDeviceEvents()
      } catch (error) {
        if (isAuthenticationError(error)) {
          await router.replace({ name: 'login', query: { redirect: route.fullPath } })
          return
        }
        toast.add({
          severity: 'error',
          summary: 'Could not update event sync',
          detail: error.message,
          life: 5000,
        })
      }
    }, 1500),
  )
}

async function syncRetainedEvents(terminal) {
  try {
    const run = await gateway.queueAccessEventSync(terminal.serialNumber)
    trackSync(run)
    toast.add({
      severity: 'info',
      summary: 'Retained event sync queued',
      detail: 'The gateway will use the terminal’s own clock and page its retained archive through PushSDK.',
      life: 5000,
    })
  } catch (error) {
    if (isAuthenticationError(error)) {
      await router.replace({ name: 'login', query: { redirect: route.fullPath } })
      return
    }
    toast.add({
      severity: 'error',
      summary: 'Could not queue retained event sync',
      detail: error.message,
      life: 5000,
    })
  }
}
</script>

<template>
  <div
    class="mb-4 flex w-full flex-col gap-2 sm:w-auto sm:flex-row sm:justify-end"
  >
    <span class="relative"
      ><i
        class="pi pi-search absolute left-3 top-1/2 -translate-y-1/2 text-xs text-slate-400" /><InputText
        v-model="filter"
        class="w-full !pl-8 sm:w-80"
        placeholder="Filter serial, state, or error" /></span
    ><Button
      label="Reload registry"
      icon="pi pi-refresh"
      :loading="gateway.refreshing"
      @click="refresh"
    />
  </div>

  <section class="overflow-hidden rounded-lg border border-slate-300 bg-white">
    <div
      class="grid divide-y divide-slate-200 sm:grid-cols-3 sm:divide-x sm:divide-y-0"
    >
      <div class="p-3">
        <p class="text-xs font-medium uppercase tracking-wide text-slate-500">
          Online
        </p>
        <p class="mt-1 text-xl font-semibold tabular-nums text-emerald-700">
          {{ gateway.onlineTerminals }}
        </p>
      </div>
      <div class="p-3">
        <p class="text-xs font-medium uppercase tracking-wide text-slate-500">
          Offline
        </p>
        <p class="mt-1 text-xl font-semibold tabular-nums text-slate-800">
          {{ gateway.offlineTerminals }}
        </p>
      </div>
      <div class="p-3">
        <p class="text-xs font-medium uppercase tracking-wide text-slate-500">
          Last errors
        </p>
        <p class="mt-1 text-xl font-semibold tabular-nums text-amber-700">
          {{ gateway.terminalsWithErrors }}
        </p>
      </div>
    </div>
  </section>

  <section
    class="mt-5 overflow-hidden rounded-lg border border-slate-300 bg-white"
  >
    <div class="border-b border-slate-200 bg-slate-50 px-4 py-3">
      <h2 class="text-sm font-semibold text-slate-950">Registered terminals</h2>
      <p class="mt-0.5 text-xs text-slate-500">
        State is durable and reflects the latest valid protocol interaction.
      </p>
    </div>
    <DataTable
      v-if="displayedTerminals.length"
      :value="displayedTerminals"
      size="small"
      striped-rows
      scrollable
      class="text-sm"
    >
      <Column header="Terminal serial">
        <template #body="{ data }">
          <code class="text-xs text-slate-800">{{ data.serialNumber }}</code>
        </template>
      </Column>
      <Column header="PushSDK serial">
        <template #body="{ data }">
          <code class="text-xs text-slate-700">{{ data.pushSdkSerial }}</code>
        </template>
      </Column>
      <Column header="State">
        <template #body="{ data }">
          <Tag
            :value="statusLabel(data.status)"
            :severity="statusSeverity(data.status)"
            rounded
          />
        </template>
      </Column>
      <Column header="Last seen">
        <template #body="{ data }">
          <span class="whitespace-nowrap text-xs text-slate-600">{{
            formatTime(data.lastSeenAt)
          }}</span>
        </template>
      </Column>
      <Column header="Last error">
        <template #body="{ data }">
          <code
            class="whitespace-pre-wrap text-xs"
            :class="data.lastError ? 'text-red-700' : 'text-slate-400'"
            >{{ data.lastError || '—' }}</code
          >
        </template>
      </Column>
      <Column header="Retained history">
        <template #body="{ data }">
          <p
            class="max-w-64 text-xs"
            :class="syncRunFor(data)?.status === 'failed' ? 'text-red-700' : 'text-slate-600'"
          >
            {{ syncSummary(data) }}
          </p>
        </template>
      </Column>
      <Column header="Actions" frozen align-frozen="right">
        <template #body="{ data }">
          <Button
            :label="syncIsActive(data) ? 'Syncing' : 'Sync retained events'"
            icon="pi pi-history"
            size="small"
            outlined
            :loading="syncIsActive(data)"
            :disabled="data.status !== 'online' || syncIsActive(data)"
            :title="data.status === 'online' ? 'Read the complete retained access-event archive through this terminal’s PushSDK session.' : 'The terminal must be online before a retained-event sync can be queued.'"
            @click="syncRetainedEvents(data)"
          />
        </template>
      </Column>
    </DataTable>
    <p v-else class="px-4 py-8 text-sm text-slate-500">
      {{
        filter
          ? 'No terminal rows match this filter.'
          : 'No terminal mappings are configured.'
      }}
    </p>
  </section>
</template>
