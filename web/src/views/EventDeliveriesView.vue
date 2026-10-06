<script setup>
import { useGatewayStore } from '../stores/gateway'
import { formatTime } from '../lib/presentation'

const gateway = useGatewayStore()
const toast = useToast()
const loading = ref(false)
const configuration = ref({ routes: [], signingKeyIds: [] })
const page = ref({ deliveries: [], total: 0 })
const offset = ref(0)
const serial = ref('')
const form = ref({
  endpointUrl: '',
  signingKeyId: '',
  enabledFrom: new Date().toISOString(),
  enabled: false,
})

async function reload() {
  loading.value = true
  try {
    const results = await Promise.all([
      gateway.loadDeliveryConfiguration(),
      gateway.loadEventDeliveries(offset.value),
    ])
    configuration.value = results[0]
    page.value = results[1]
  } catch (error) {
    report(error)
  } finally {
    loading.value = false
  }
}
function report(error) {
  toast.add({
    severity: 'error',
    summary: 'Event delivery',
    detail: error.message,
    life: 6000,
  })
}
function selectTerminal() {
  const route = configuration.value.routes.find(
    (item) => item.terminalSerialNumber === serial.value,
  )
  form.value = route
    ? {
        endpointUrl: route.endpointUrl,
        signingKeyId: route.signingKeyId,
        enabledFrom: route.enabledFrom,
        enabled: route.enabled,
      }
    : {
        endpointUrl: '',
        signingKeyId: '',
        enabledFrom: new Date().toISOString(),
        enabled: false,
      }
}
async function save() {
  loading.value = true
  try {
    await gateway.saveDeliveryRoute(serial.value, form.value)
    await reload()
    toast.add({
      severity: 'success',
      summary: 'Delivery configuration saved',
      life: 3000,
    })
  } catch (error) {
    report(error)
  } finally {
    loading.value = false
  }
}
async function retry(id) {
  try {
    await gateway.retryEventDelivery(id)
    await reload()
  } catch (error) {
    report(error)
  }
}
async function paginate(event) {
  offset.value = event.first
  await reload()
}
onMounted(reload)
</script>

<template>
  <section class="rounded-lg border border-slate-300 bg-white p-4">
    <div class="flex items-start justify-between gap-4">
      <div>
        <h2 class="font-semibold">Delivery destinations</h2>
        <p class="mt-1 text-sm text-slate-600">
          New live Face Authentication Completed events (5/75) are delivered
          after the activation time. Imported archive events remain in the
          archive.
        </p>
      </div>
      <Button
        label="Refresh"
        icon="pi pi-refresh"
        :loading="loading"
        @click="reload"
      />
    </div>
    <div class="mt-4 overflow-auto">
      <table class="w-full text-left text-sm">
        <thead>
          <tr class="border-b">
            <th class="p-2">Terminal</th>
            <th class="p-2">Destination</th>
            <th class="p-2">Signing key</th>
            <th class="p-2">Starts at</th>
            <th class="p-2">Status</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="route in configuration.routes"
            :key="route.terminalSerialNumber"
            class="border-b"
          >
            <td class="p-2">{{ route.terminalSerialNumber }}</td>
            <td class="p-2 break-all">{{ route.endpointUrl }}</td>
            <td class="p-2">{{ route.signingKeyId }}</td>
            <td class="p-2">{{ formatTime(route.enabledFrom) }}</td>
            <td class="p-2">{{ route.enabled ? 'Enabled' : 'Paused' }}</td>
          </tr>
          <tr v-if="configuration.routes.length === 0">
            <td colspan="5" class="p-2 text-slate-500">
              No destinations configured.
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <form class="mt-5 grid gap-4 md:grid-cols-2" @submit.prevent="save">
      <label class="grid gap-1 text-sm"
        >Terminal<select
          v-model="serial"
          class="rounded border border-slate-300 p-2"
          required
          @change="selectTerminal"
        >
          <option value="">Select terminal</option>
          <option
            v-for="terminal in gateway.overview.terminals"
            :key="terminal.serialNumber"
            :value="terminal.serialNumber"
          >
            {{ terminal.pushSdkSerial }} — {{ terminal.serialNumber }}
          </option>
        </select></label
      >
      <label class="grid gap-1 text-sm"
        >HTTPS destination<InputText
          v-model="form.endpointUrl"
          type="url"
          required
          placeholder="https://creative.itplus.club/api/integrations/pushsdk/events"
      /></label>
      <label class="grid gap-1 text-sm"
        >Signing key<select
          v-model="form.signingKeyId"
          class="rounded border border-slate-300 p-2"
          required
        >
          <option value="">Select configured key</option>
          <option
            v-for="key in configuration.signingKeyIds"
            :key="key"
            :value="key"
          >
            {{ key }}
          </option>
        </select></label
      >
      <label class="grid gap-1 text-sm"
        >Activation time (RFC3339 with offset)<InputText
          v-model="form.enabledFrom"
          required
          placeholder="2026-10-07T06:00:00+03:00"
      /></label>
      <label class="flex items-center gap-2 text-sm"
        ><input v-model="form.enabled" type="checkbox" />Enable delivery</label
      >
      <div>
        <Button
          type="submit"
          label="Save destination"
          :loading="loading"
          :disabled="configuration.signingKeyIds.length === 0"
        />
      </div>
    </form>
    <p
      v-if="configuration.signingKeyIds.length === 0"
      class="mt-3 text-sm text-amber-700"
    >
      Configure a signing key on the server before adding a destination.
    </p>
    <p class="mt-3 text-xs text-slate-500">
      Pausing stops collection of new deliveries and pauses queued deliveries.
      Destination, key and activation time changes require pending deliveries to
      finish. Each delivery retains its original destination.
    </p>
  </section>
  <section
    class="mt-5 overflow-hidden rounded-lg border border-slate-300 bg-white p-4"
  >
    <h2 class="font-semibold">Delivery audit</h2>
    <p class="mt-1 text-sm text-slate-600">
      Delivered means the destination stored the event. Teacher mapping and
      attendance results are reviewed in the school platform.
    </p>
    <div class="mt-3 overflow-auto">
      <table class="w-full text-left text-sm">
        <thead>
          <tr class="border-b">
            <th class="p-2">Delivery / event</th>
            <th class="p-2">Terminal</th>
            <th class="p-2">Destination</th>
            <th class="p-2">State</th>
            <th class="p-2">Attempts</th>
            <th class="p-2">HTTP / error</th>
            <th class="p-2">Next attempt</th>
            <th class="p-2"></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="delivery in page.deliveries"
            :key="delivery.id"
            class="border-b"
          >
            <td class="p-2">
              #{{ delivery.id }} / #{{ delivery.deviceEventId }}
            </td>
            <td class="p-2">{{ delivery.terminalSerialNumber }}</td>
            <td class="p-2 break-all">{{ delivery.endpointUrl }}</td>
            <td class="p-2">{{ delivery.status }}</td>
            <td class="p-2">{{ delivery.attempts }}</td>
            <td class="p-2">
              {{ delivery.lastHttpStatus }} {{ delivery.lastError }}
            </td>
            <td class="p-2">
              {{
                delivery.status === 'pending'
                  ? formatTime(delivery.nextAttemptAt)
                  : '—'
              }}
            </td>
            <td class="p-2">
              <Button
                v-if="delivery.status === 'failed'"
                label="Retry"
                size="small"
                @click="retry(delivery.id)"
              />
            </td>
          </tr>
          <tr v-if="page.deliveries.length === 0">
            <td colspan="8" class="p-2 text-slate-500">
              No deliveries queued.
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <Paginator
      :first="offset"
      :rows="gateway.pageSize"
      :total-records="page.total"
      @page="paginate"
    />
  </section>
</template>
