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
  enabled: false,
})

const backfillSerial = ref('')
const backfillPreview = ref(null)
const mappingsVerified = ref(false)
function localDateTime(date) {
  const pad = (value) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}
const backfillRange = ref({
  startsAt: localDateTime(new Date(Date.now() - 30 * 86400000)),
  endsAt: localDateTime(new Date()),
})
watch(
  [
    backfillSerial,
    () => backfillRange.value.startsAt,
    () => backfillRange.value.endsAt,
  ],
  () => {
    backfillPreview.value = null
    mappingsVerified.value = false
  },
)
function backfillInput() {
  return {
    startsAt: new Date(backfillRange.value.startsAt).toISOString(),
    endsAt: new Date(backfillRange.value.endsAt).toISOString(),
  }
}
async function previewBackfill() {
  loading.value = true
  backfillPreview.value = null
  mappingsVerified.value = false
  try {
    backfillPreview.value = await gateway.previewEventBackfill(
      backfillSerial.value,
      backfillInput(),
    )
  } catch (error) {
    report(error)
  } finally {
    loading.value = false
  }
}
async function queueBackfill() {
  loading.value = true
  try {
    const result = await gateway.queueEventBackfill(backfillSerial.value, {
      startsAt: backfillPreview.value.startsAt,
      endsAt: backfillPreview.value.endsAt,
      previewToken: backfillPreview.value.token,
    })
    backfillPreview.value = null
    mappingsVerified.value = false
    toast.add({
      severity: 'success',
      summary: `Backfill #${result.backfillId}: ${result.queuedCount} events queued`,
      life: 6000,
    })
    await loadData()
  } catch (error) {
    backfillPreview.value = null
    mappingsVerified.value = false
    report(error)
  } finally {
    loading.value = false
  }
}

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
async function removeDestination(destination) {
  loading.value = true
  try {
    await gateway.deleteDeliveryRoute(destination.terminalSerialNumber)
    if (serial.value === destination.terminalSerialNumber) {
      serial.value = ''
      selectTerminal()
    }
    if (backfillSerial.value === destination.terminalSerialNumber) {
      backfillSerial.value = ''
    }
    await loadData()
    toast.add({
      severity: 'success',
      summary: 'Destination removed',
      life: 3000,
    })
  } catch (error) {
    report(error)
  } finally {
    loading.value = false
  }
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
        enabled: route.enabled,
      }
    : {
        endpointUrl: '',
        signingKeyId: '',
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
          Face Authentication Completed events (5/75) are delivered while
          enabled. Use Historical backfill to submit events already stored in
          the archive.
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
              <Button
                class="mt-2"
                label="Remove"
                icon="pi pi-trash"
                size="small"
                severity="secondary"
                :disabled="loading"
                @click="removeDestination(route)"
              />
            </td>
          </tr>
          <tr v-if="configuration.routes.length === 0">
            <td colspan="5" class="p-2 text-slate-500">
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
        Destination and key changes require pending deliveries to finish. Each
        delivery retains its original destination.
      </p>
    </div>
  </section>
  <section
    class="mt-5 overflow-hidden rounded-lg border border-slate-300 bg-white"
  >
    <div class="border-b border-slate-200 bg-slate-50 px-4 py-3">
      <h2 class="text-sm font-semibold text-slate-950">Historical backfill</h2>
      <p class="mt-1 text-xs text-slate-500">
        Preview Face Authentication Completed scans from both archives. Original
        scan dates are preserved. The school applies its teacher mappings and
        preserves existing Late or Absent attendance.
      </p>
    </div>
    <div class="p-4">
      <form @submit.prevent="previewBackfill">
        <fieldset :disabled="loading" class="grid gap-4 md:grid-cols-3">
          <label class="grid min-w-0 content-start gap-1 text-sm"
            >Terminal
            <select
              v-model="backfillSerial"
              class="w-full min-w-0 rounded border border-slate-300 p-2"
              required
            >
              <option value="">Select enabled destination</option>
              <option
                v-for="destination in configuration.routes.filter(
                  (item) => item.enabled,
                )"
                :key="destination.terminalSerialNumber"
                :value="destination.terminalSerialNumber"
              >
                {{ destination.terminalSerialNumber }}
              </option>
            </select>
          </label>
          <label class="grid content-start gap-1 text-sm"
            >From ({{ displayTimezone }})<InputText
              v-model="backfillRange.startsAt"
              type="datetime-local"
              required
          /></label>
          <label class="grid content-start gap-1 text-sm"
            >Until, exclusive ({{ displayTimezone }})<InputText
              v-model="backfillRange.endsAt"
              type="datetime-local"
              required
          /></label>
          <div class="md:col-span-3">
            <Button
              type="submit"
              label="Preview backfill"
              :disabled="loading || !backfillSerial"
              :loading="loading"
            />
          </div>
        </fieldset>
      </form>
      <p class="mt-3 text-xs text-slate-500">
        Select a past range of up to 31 days. Previewing creates no deliveries
        or attendance.
      </p>
      <div v-if="backfillPreview" class="mt-4 border-t border-slate-200 pt-4">
        <p class="text-sm">
          {{ backfillPreview.eligible }} events ready:
          {{ backfillPreview.pushSdkEvents }} PushSDK,
          {{ backfillPreview.isapiEvents }} ISAPI. Already queued:
          {{ backfillPreview.alreadyQueued }}. Matching copies across sources:
          {{ backfillPreview.duplicateSources }}. Invalid identities:
          {{ backfillPreview.invalid }}.
        </p>
        <p class="mt-2 break-all text-xs text-slate-500">
          Destination: {{ backfillPreview.route.endpointUrl }}
        </p>
        <div class="mt-3 overflow-auto">
          <table class="w-full text-left text-sm">
            <thead>
              <tr class="border-b border-slate-200 bg-slate-50 text-slate-600">
                <th class="p-2">Device person ID</th>
                <th class="p-2">Names in archive</th>
                <th class="p-2">Events</th>
                <th class="p-2">First scan</th>
                <th class="p-2">Last scan</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="identity in backfillPreview.identities"
                :key="identity.employeeNumber"
                class="border-b border-slate-200"
              >
                <td class="p-2">
                  <code>{{ identity.employeeNumber }}</code>
                </td>
                <td class="p-2">{{ identity.names.join(' / ') }}</td>
                <td class="p-2">{{ identity.events }}</td>
                <td class="p-2">{{ formatTime(identity.firstScan) }}</td>
                <td class="p-2">{{ formatTime(identity.lastScan) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <label class="mt-4 flex items-start gap-2 text-sm"
          ><input
            v-model="mappingsVerified"
            type="checkbox"
            :disabled="loading || backfillPreview.eligible === 0"
            class="mt-1"
          />I have verified the school teacher mappings for these identities and
          scan dates.</label
        >
        <Button
          class="mt-3"
          label="Queue previewed events"
          :loading="loading"
          :disabled="
            loading || !mappingsVerified || backfillPreview.eligible === 0
          "
          @click="queueBackfill"
        />
      </div>
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
              #{{ delivery.id }} /
              {{ delivery.retainedEventId ? 'ISAPI' : 'PushSDK' }} #{{
                delivery.retainedEventId ?? delivery.deviceEventId
              }}
              <p v-if="delivery.backfillId" class="text-xs text-slate-500">
                Backfill #{{ delivery.backfillId }}
              </p>
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
