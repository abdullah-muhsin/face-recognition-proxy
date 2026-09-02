<script setup>
import { useGatewayStore } from '../stores/gateway'
import {
  eventSeverity,
  formatShortTime,
  formatTime,
  monitorConnection,
  statusLabel,
  statusSeverity,
} from '../lib/presentation'

const gateway = useGatewayStore()
const router = useRouter()
const latestActivity = computed(() => gateway.monitor.slice(0, 8))
const connection = computed(() => monitorConnection(gateway.socketState))
</script>

<template>
  <div
    class="mb-4 flex flex-col gap-3 xl:flex-row xl:items-end xl:justify-between"
  >
    <div>
      <p
        class="text-xs font-semibold uppercase tracking-[0.18em] text-cyan-700"
      >
        Administration
      </p>
      <h1 class="mt-1 text-2xl font-semibold tracking-tight text-slate-950">
        Gateway board
      </h1>
      <p class="mt-1 text-sm text-slate-600">
        Current terminal state, durable event collection, and retained protocol
        activity.
      </p>
    </div>
    <div class="flex flex-wrap items-center gap-2 text-xs text-slate-500">
      <Tag :value="connection.label" :severity="connection.severity" rounded />
      <span v-if="gateway.lastUpdatedAt">
        Data refreshed {{ formatShortTime(gateway.lastUpdatedAt) }}
      </span>
      <span v-else>Waiting for gateway data</span>
    </div>
  </div>

  <section class="overflow-hidden rounded-lg border border-slate-300 bg-white">
    <div
      class="grid divide-y divide-slate-200 sm:grid-cols-2 sm:divide-x sm:divide-y-0 xl:grid-cols-4"
    >
      <div class="p-4">
        <p class="text-xs font-medium uppercase tracking-wide text-slate-500">
          Retained events
        </p>
        <p class="mt-1 text-2xl font-semibold tabular-nums text-slate-950">
          {{ gateway.overview.deviceEventTotal }}
        </p>
        <p class="mt-1 text-xs text-slate-500">
          Exact source values in archive
        </p>
      </div>
      <div class="p-4">
        <p class="text-xs font-medium uppercase tracking-wide text-slate-500">
          Terminal mappings
        </p>
        <p class="mt-1 text-2xl font-semibold tabular-nums text-slate-950">
          {{ gateway.overview.terminals.length }}
        </p>
        <p class="mt-1 text-xs text-slate-500">Configured device identities</p>
      </div>
      <div class="p-4">
        <p class="text-xs font-medium uppercase tracking-wide text-slate-500">
          Online terminals
        </p>
        <p class="mt-1 text-2xl font-semibold tabular-nums text-emerald-700">
          {{ gateway.onlineTerminals }}
        </p>
        <p class="mt-1 text-xs text-slate-500">Confirmed by PushSDK traffic</p>
      </div>
      <div class="p-4">
        <p class="text-xs font-medium uppercase tracking-wide text-slate-500">
          Recorded errors
        </p>
        <p class="mt-1 text-2xl font-semibold tabular-nums text-amber-700">
          {{ gateway.terminalsWithErrors }}
        </p>
        <p class="mt-1 text-xs text-slate-500">Terminal rows with last error</p>
      </div>
    </div>
  </section>

  <div class="mt-5 grid gap-5 xl:grid-cols-[1.45fr_1fr]">
    <section
      class="overflow-hidden rounded-lg border border-slate-300 bg-white"
    >
      <div
        class="flex items-center justify-between gap-4 border-b border-slate-200 bg-slate-50 px-4 py-3"
      >
        <div>
          <h2 class="text-sm font-semibold text-slate-950">Terminal state</h2>
          <p class="mt-0.5 text-xs text-slate-500">
            Current database state and last observed PushSDK interaction.
          </p>
        </div>
        <Button
          label="Registry"
          icon="pi pi-arrow-right"
          icon-pos="right"
          text
          size="small"
          @click="router.push('/terminals')"
        />
      </div>
      <DataTable
        v-if="gateway.overview.terminals.length"
        :value="gateway.overview.terminals"
        size="small"
        striped-rows
        class="text-sm"
      >
        <Column header="Terminal">
          <template #body="{ data }">
            <code class="text-xs text-slate-800">{{ data.serialNumber }}</code>
            <p class="mt-1 text-xs text-slate-500">
              PushSDK: {{ data.pushSdkSerial }}
            </p>
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
            <span
              class="text-xs"
              :class="data.lastError ? 'text-red-700' : 'text-slate-400'"
            >
              {{ data.lastError || '—' }}
            </span>
          </template>
        </Column>
      </DataTable>
      <p v-else class="px-4 py-6 text-sm text-slate-500">
        No terminal mappings are configured.
      </p>
    </section>

    <section
      class="overflow-hidden rounded-lg border border-slate-300 bg-white"
    >
      <div
        class="flex items-center justify-between gap-4 border-b border-slate-200 bg-slate-50 px-4 py-3"
      >
        <div>
          <h2 class="text-sm font-semibold text-slate-950">Latest activity</h2>
          <p class="mt-0.5 text-xs text-slate-500">
            Live stream backed by retained gateway activity.
          </p>
        </div>
        <Button
          label="Log"
          icon="pi pi-arrow-right"
          icon-pos="right"
          text
          size="small"
          @click="router.push('/activity')"
        />
      </div>
      <div v-if="latestActivity.length" class="divide-y divide-slate-100">
        <article
          v-for="(event, index) in latestActivity"
          :key="event.id || `${event.at}-${index}`"
          class="grid grid-cols-[0.55rem_1fr] gap-3 px-4 py-3"
        >
          <span
            class="mt-1.5 h-2 w-2 rounded-full"
            :class="
              eventSeverity(event.kind) === 'danger'
                ? 'bg-red-500'
                : eventSeverity(event.kind) === 'success'
                  ? 'bg-emerald-500'
                  : 'bg-cyan-600'
            "
          />
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
              <code class="text-xs font-medium text-slate-800">{{
                event.kind
              }}</code>
              <span class="text-xs text-slate-400">{{
                formatShortTime(event.at)
              }}</span>
            </div>
            <p class="mt-1 text-xs leading-5 text-slate-600">
              {{ event.message }}
            </p>
            <p class="mt-1 truncate text-xs text-slate-400">
              {{ event.terminal || 'gateway' }}
            </p>
          </div>
        </article>
      </div>
      <p v-else class="px-4 py-6 text-sm text-slate-500">
        No gateway activity has been received in this browser session.
      </p>
    </section>
  </div>
</template>
