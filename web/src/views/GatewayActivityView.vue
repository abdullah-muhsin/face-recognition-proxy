<script setup>
import { useGatewayStore } from '../stores/gateway'
import {
  eventSeverity,
  formatTime,
  isAuthenticationError,
  matchesGatewayActivity,
  monitorConnection,
} from '../lib/presentation'

const gateway = useGatewayStore()
const router = useRouter()
const route = useRoute()
const toast = useToast()
const filter = ref('')
const connection = computed(() => monitorConnection(gateway.socketState))
const displayedActivities = computed(() =>
  gateway.gatewayActivities.filter((activity) =>
    matchesGatewayActivity(activity, filter.value),
  ),
)
const currentPageStart = computed(() =>
  gateway.gatewayActivitiesTotal === 0
    ? 0
    : gateway.gatewayActivitiesOffset + 1,
)
const currentPageEnd = computed(() =>
  Math.min(
    gateway.gatewayActivitiesOffset + gateway.gatewayActivities.length,
    gateway.gatewayActivitiesTotal,
  ),
)

const formatFields = (fields) => JSON.stringify(fields, null, 2)

async function handleFailure(error, summary) {
  if (isAuthenticationError(error)) {
    await router.replace({ name: 'login', query: { redirect: route.fullPath } })
    return
  }
  toast.add({ severity: 'error', summary, detail: error.message, life: 5000 })
}

async function reload() {
  try {
    await gateway.loadGatewayActivities(0)
  } catch (error) {
    await handleFailure(error, 'Could not load gateway activity')
  }
}

async function changePage(event) {
  try {
    await gateway.loadGatewayActivities(event.first)
  } catch (error) {
    await handleFailure(error, 'Could not load gateway activity')
  }
}
</script>

<template>
  <div
    class="mb-4 flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between"
  >
    <div>
      <p
        class="text-xs font-semibold uppercase tracking-[0.18em] text-cyan-700"
      >
        Administrative history
      </p>
      <h1 class="mt-1 text-2xl font-semibold tracking-tight text-slate-950">
        Gateway activity
      </h1>
      <p class="mt-1 text-sm text-slate-600">
        PostgreSQL-backed protocol and administration activity. Device payloads
        remain only in Event archive.
      </p>
    </div>
    <div class="flex flex-wrap items-center gap-2">
      <Tag :value="connection.label" :severity="connection.severity" rounded />
      <Button
        label="Reconnect stream"
        icon="pi pi-sync"
        outlined
        size="small"
        @click="gateway.connectMonitor"
      />
      <Button
        label="Reload archive"
        icon="pi pi-refresh"
        size="small"
        :loading="gateway.activityLoading"
        @click="reload"
      />
    </div>
  </div>

  <section class="overflow-hidden rounded-lg border border-slate-300 bg-white">
    <div
      class="grid divide-y divide-slate-200 sm:grid-cols-3 sm:divide-x sm:divide-y-0"
    >
      <div class="p-3">
        <p class="text-xs font-medium uppercase tracking-wide text-slate-500">
          Archived rows
        </p>
        <p class="mt-1 text-xl font-semibold tabular-nums text-slate-950">
          {{ gateway.gatewayActivitiesTotal }}
        </p>
      </div>
      <div class="p-3">
        <p class="text-xs font-medium uppercase tracking-wide text-slate-500">
          Current page
        </p>
        <p class="mt-1 text-xl font-semibold tabular-nums text-slate-950">
          {{ currentPageStart }}–{{ currentPageEnd }}
        </p>
      </div>
      <div class="p-3">
        <p class="text-xs font-medium uppercase tracking-wide text-slate-500">
          Live stream cache
        </p>
        <p class="mt-1 text-xl font-semibold tabular-nums text-slate-950">
          {{ gateway.monitor.length }}
        </p>
      </div>
    </div>
  </section>

  <section
    class="mt-5 overflow-hidden rounded-lg border border-slate-300 bg-white"
  >
    <div
      class="flex flex-col gap-3 border-b border-slate-200 bg-slate-50 px-4 py-3 sm:flex-row sm:items-center sm:justify-between"
    >
      <div>
        <h2 class="text-sm font-semibold text-slate-950">Activity archive</h2>
        <p class="mt-0.5 text-xs text-slate-500">
          Filter applies to the currently loaded database page.
        </p>
      </div>
      <span class="relative w-full sm:w-80">
        <i
          class="pi pi-search absolute left-3 top-1/2 -translate-y-1/2 text-xs text-slate-400"
        />
        <InputText
          v-model="filter"
          class="w-full !pl-8"
          placeholder="Filter kind, source, or message"
        />
      </span>
    </div>
    <DataTable
      v-if="displayedActivities.length"
      :value="displayedActivities"
      size="small"
      striped-rows
      scrollable
      class="text-sm"
    >
      <Column header="Time">
        <template #body="{ data }">
          <span class="whitespace-nowrap text-xs text-slate-600">{{
            formatTime(data.at)
          }}</span>
        </template>
      </Column>
      <Column header="Kind">
        <template #body="{ data }">
          <Tag :value="data.kind" :severity="eventSeverity(data.kind)" />
        </template>
      </Column>
      <Column header="Source">
        <template #body="{ data }">
          <code class="text-xs text-slate-700">{{
            data.terminal || 'gateway'
          }}</code>
        </template>
      </Column>
      <Column header="Message">
        <template #body="{ data }">
          <p class="text-xs leading-5 text-slate-700">{{ data.message }}</p>
        </template>
      </Column>
      <Column header="Fields">
        <template #body="{ data }">
          <details v-if="data.fields && Object.keys(data.fields).length">
            <summary class="cursor-pointer text-xs font-medium text-cyan-700">
              Inspect JSON
            </summary>
            <pre
              class="mt-2 max-w-md overflow-auto rounded bg-slate-950 p-3 text-xs leading-5 text-slate-100"
              >{{ formatFields(data.fields) }}</pre>
          </details>
          <span v-else class="text-xs text-slate-400">—</span>
        </template>
      </Column>
    </DataTable>
    <div
      v-else-if="gateway.activityLoading"
      class="flex items-center justify-center gap-3 px-4 py-16 text-sm text-slate-500"
    >
      <ProgressSpinner stroke-width="4" class="h-6 w-6" /> Loading activity
      archive
    </div>
    <p v-else class="px-4 py-8 text-sm text-slate-500">
      {{
        filter
          ? 'No activities match this page filter.'
          : 'No gateway activity has been retained yet.'
      }}
    </p>
    <Paginator
      v-if="gateway.gatewayActivitiesTotal > gateway.pageSize"
      class="border-t border-slate-200"
      :first="gateway.gatewayActivitiesOffset"
      :rows="gateway.pageSize"
      :total-records="gateway.gatewayActivitiesTotal"
      @page="changePage"
    />
  </section>
</template>
