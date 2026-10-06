<script setup>
import { useGatewayStore } from '../stores/gateway'
import { formatTime, isAuthenticationError } from '../lib/presentation'

const gateway = useGatewayStore()
const toast = useToast()
const router = useRouter()
const currentRoute = useRoute()
const editor = ref(null)
const displayTimezone = Intl.DateTimeFormat().resolvedOptions().timeZone
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
    await loadData()
  } catch (error) {
    report(error)
  } finally {
    loading.value = false
  }
}
async function loadData() {
  const results = await Promise.all([
    gateway.loadDeliveryConfiguration(),
    gateway.loadEventDeliveries(offset.value),
  ])
  configuration.value = results[0]
  page.value = results[1]
}
function report(error) {
  if (isAuthenticationError(error)) {
    router.replace({
      name: 'login',
      query: { redirect: currentRoute.fullPath },
    })
    return
  }
  toast.add({
    severity: 'error',
    summary: 'Event delivery',
    detail: error.message,
    life: 6000,
  })
}
async function editDestination(destination) {
  serial.value = destination.terminalSerialNumber
  selectTerminal()
  await nextTick()
  editor.value.scrollIntoView({ behavior: 'smooth', block: 'center' })
  editor.value.querySelector('input').focus({ preventScroll: true })
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
    await loadData()
    selectTerminal()
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
  loading.value = true
  try {
    await gateway.retryEventDelivery(id)
    await loadData()
  } catch (error) {
    report(error)
  } finally {
    loading.value = false
  }
}
async function paginate(event) {
  offset.value = event.first
  await reload()
}
onMounted(reload)
</script>

<template>
  <section class="overflow-hidden rounded-lg border border-slate-300 bg-white">
    <div
      class="flex flex-wrap items-start justify-between gap-4 border-b border-slate-200 bg-slate-50 px-4 py-3"
    >
      <div>
        <h2 class="text-sm font-semibold text-slate-950">
          Delivery destinations
        </h2>
        <p class="mt-1 text-xs text-slate-500">
          New live Face Authentication Completed events (5/75) are delivered
          after the activation time. Imported archive events remain in the
          archive.
        </p>
      </div>
      <Button
        label="Refresh"
        icon="pi pi-refresh"
        :loading="loading"
        size="small"
        @click="reload"
      />
    </div>
    <p class="px-4 pt-3 text-xs text-slate-500">
      Times displayed in {{ displayTimezone }}.
    </p>
    <div class="overflow-auto px-4 pb-4 pt-2">
      <table class="w-full text-left text-sm">
        <thead>
          <tr class="border-b border-slate-200 bg-slate-50 text-slate-600">
            <th class="p-2">Terminal</th>
            <th class="p-2">Destination</th>
            <th class="p-2">Signing key</th>
            <th class="p-2">Starts at</th>
            <th class="p-2">Status</th>
            <th class="p-2"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="route in configuration.routes"
            :key="route.terminalSerialNumber"
            class="border-b border-slate-200"
          >
            <td class="max-w-64 break-all p-2">
              <code class="text-xs">{{ route.terminalSerialNumber }}</code>
            </td>
            <td class="p-2 break-all">{{ route.endpointUrl }}</td>
            <td class="whitespace-nowrap p-2">{{ route.signingKeyId }}</td>
            <td class="p-2">
              {{ formatTime(route.enabledFrom) }}
            </td>
            <td class="p-2">
              <Tag
                :value="route.enabled ? 'Enabled' : 'Paused'"
                :severity="route.enabled ? 'success' : 'secondary'"
                rounded
              />
            </td>
            <td class="p-2">
              <Button
                label="Edit"
                icon="pi pi-pencil"
                size="small"
                severity="secondary"
                :disabled="loading"
                @click="editDestination(route)"
              />
            </td>
          </tr>
          <tr v-if="configuration.routes.length === 0">
            <td colspan="6" class="p-2 text-slate-500">
              No destinations configured.
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="border-t border-slate-200 p-4">
      <h3 class="mb-3 text-sm font-semibold">Configure destination</h3>
      <form ref="editor" @submit.prevent="save">
        <fieldset :disabled="loading" class="grid min-w-0 gap-4 md:grid-cols-2">
          <label class="grid min-w-0 content-start gap-1 text-sm"
            >Terminal<select
              v-model="serial"
              class="w-full min-w-0 rounded border border-slate-300 p-2"
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
          <label class="grid min-w-0 content-start gap-1 text-sm"
            >HTTPS destination<InputText
              v-model="form.endpointUrl"
              class="w-full min-w-0"
              type="url"
              required
              placeholder="https://school.example/api/integrations/pushsdk/events"
          /></label>
          <label class="grid min-w-0 content-start gap-1 text-sm"
            >Signing key<select
              v-model="form.signingKeyId"
              class="w-full min-w-0 rounded border border-slate-300 p-2"
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
          <label class="grid min-w-0 content-start gap-1 text-sm"
            >Activation time<InputText
              v-model="form.enabledFrom"
              class="w-full min-w-0"
              required
              placeholder="2026-10-07T06:00:00+03:00"
            /><span class="text-xs text-slate-500"
              >Include the timezone offset, for example
              2026-10-07T06:00:00+03:00.</span
            ></label
          >
          <label class="flex items-center gap-2 text-sm md:col-span-2"
            ><input v-model="form.enabled" type="checkbox" />Enable
            delivery</label
          >
          <div class="md:col-span-2">
            <Button
              type="submit"
              label="Save destination"
              :loading="loading"
              :disabled="
                loading || !serial || configuration.signingKeyIds.length === 0
              "
            />
          </div>
        </fieldset>
      </form>
      <p
        v-if="configuration.signingKeyIds.length === 0"
        class="mt-3 text-sm text-amber-700"
      >
        Configure a signing key on the server before adding a destination.
      </p>
      <p class="mt-3 text-xs text-slate-500">
        Pausing stops collection of new deliveries and pauses queued deliveries.
        Destination, key and activation time changes require pending deliveries
        to finish. Each delivery retains its original destination.
      </p>
    </div>
  </section>
  <section
    class="mt-5 overflow-hidden rounded-lg border border-slate-300 bg-white"
  >
    <div class="border-b border-slate-200 bg-slate-50 px-4 py-3">
      <h2 class="text-sm font-semibold text-slate-950">Delivery audit</h2>
      <p class="mt-1 text-xs text-slate-500">
        Delivered means the destination stored the event. Teacher mapping and
        attendance results are reviewed in the school platform.
      </p>
    </div>
    <div class="overflow-auto p-4">
      <table class="w-full text-left text-sm">
        <thead>
          <tr class="border-b border-slate-200 bg-slate-50 text-slate-600">
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
            class="border-b border-slate-200"
          >
            <td class="p-2">
              #{{ delivery.id }} / #{{ delivery.deviceEventId }}
            </td>
            <td class="p-2">
              <code class="text-xs">{{ delivery.terminalSerialNumber }}</code>
            </td>
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
                :disabled="loading"
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
      v-if="page.total > gateway.pageSize"
      :first="offset"
      :rows="gateway.pageSize"
      :total-records="page.total"
      @page="paginate"
    />
  </section>
</template>
