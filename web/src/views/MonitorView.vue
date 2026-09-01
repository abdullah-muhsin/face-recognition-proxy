<script setup>
import { useGatewayStore } from '../stores/gateway'
import { eventSeverity, formatTime } from '../lib/presentation'

const gateway = useGatewayStore()
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
      <p class="text-sm font-medium text-cyan-700">Live monitor</p>
      <h1 class="mt-1 text-3xl font-semibold tracking-tight text-slate-950">
        Gateway activity
      </h1>
      <p class="mt-2 text-sm text-slate-500">
        A live, metadata-only operational feed. Protocol bodies and biometric
        data are deliberately excluded.
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
          </div>
        </article>
      </div>
      <div
        v-else
        class="rounded-xl border border-dashed border-slate-200 bg-slate-50 px-6 py-20 text-center"
      >
        <i class="pi pi-wave-pulse text-3xl text-slate-400" />
        <p class="mt-4 font-medium text-slate-700">
          Waiting for gateway activity
        </p>
        <p class="mt-2 text-sm text-slate-500">
          Keep this view open while testing terminal registration or device
          events.
        </p>
      </div>
    </template></Card
  >
</template>
