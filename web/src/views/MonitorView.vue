<script setup>
import { useGatewayStore } from '../stores/gateway'
import { eventSeverity, formatTime } from '../lib/presentation'

const gateway = useGatewayStore()
const formatFields = (fields) => JSON.stringify(fields, null, 2)
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
</script>

<template>
  <div
    class="mb-7 flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between"
  >
    <div>
      <p class="text-sm font-medium text-cyan-700">Gateway activity</p>
      <h1 class="mt-1 text-3xl font-semibold tracking-tight text-slate-950">
        Gateway activity
      </h1>
      <p class="mt-2 text-sm text-slate-500">
        PostgreSQL-backed operational history, with new activity streamed live.
        Raw device payloads remain in Device Events and are never copied here.
      </p>
    </div>
    <div class="flex items-center gap-3">
      <Tag
        :value="monitorConnectionLabel"
        :severity="monitorConnectionSeverity"
        rounded
      /><Button
        label="Reconnect"
        icon="pi pi-sync"
        outlined
        @click="gateway.connectMonitor"
      />
    </div>
  </div>
  <Card class="border border-slate-200 shadow-sm"
    ><template #content>
      <div v-if="gateway.monitor.length" class="divide-y divide-slate-100">
        <article
          v-for="(event, index) in gateway.monitor"
          :key="`${event.at}-${index}`"
          class="grid gap-3 py-5 md:grid-cols-[10rem_11rem_1fr] md:items-start"
        >
          <p class="text-sm text-slate-500">{{ formatTime(event.at) }}</p>
          <Tag
            :value="event.kind"
            :severity="eventSeverity(event.kind)"
            class="w-fit"
          />
          <div>
            <p class="text-sm font-medium text-slate-800">
              {{ event.message }}
            </p>
            <p class="mt-1 text-xs text-slate-500">
              Source: {{ event.terminal || 'Gateway' }}
            </p>
            <details
              v-if="event.fields && Object.keys(event.fields).length"
              class="mt-3"
            >
              <summary class="cursor-pointer text-xs font-medium text-cyan-700">
                Activity details
              </summary>
              <pre
                class="mt-2 overflow-x-auto rounded-lg bg-slate-950 p-3 text-xs leading-5 text-slate-100"
                >{{ formatFields(event.fields) }}</pre>
            </details>
          </div>
        </article>
      </div>
      <div
        v-else
        class="rounded-xl border border-dashed border-slate-200 bg-slate-50 px-6 py-20 text-center"
      >
        <i class="pi pi-wave-pulse text-3xl text-slate-400" />
        <p class="mt-4 font-medium text-slate-700">
          No gateway activity retained yet
        </p>
        <p class="mt-2 text-sm text-slate-500">
          New protocol activity is retained in PostgreSQL before it appears
          here, and remains available after a gateway restart.
        </p>
      </div>
    </template></Card
  >
</template>
