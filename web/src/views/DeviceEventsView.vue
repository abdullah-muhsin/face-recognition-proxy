<script setup>
import { useGatewayStore } from '../stores/gateway'
import {
  formatShortTime,
  formatTime,
  isAuthenticationError,
  matchesDeviceEvent,
} from '../lib/presentation'

const gateway = useGatewayStore()
const router = useRouter()
const route = useRoute()
const toast = useToast()
const filter = ref('')
const payloadVisible = ref(false)
const payloadLoading = ref(false)
const selectedEvent = ref(null)
const selectedPayload = ref(null)

const displayedEvents = computed(() =>
  gateway.deviceEvents.filter((event) =>
    matchesDeviceEvent(event, filter.value),
  ),
)
const currentPageStart = computed(() =>
  gateway.deviceEventsTotal === 0 ? 0 : gateway.deviceEventsOffset + 1,
)
const currentPageEnd = computed(() =>
  Math.min(
    gateway.deviceEventsOffset + gateway.deviceEvents.length,
    gateway.deviceEventsTotal,
  ),
)
const payloadCaptured = computed(
  () => typeof selectedPayload.value?.payloadBase64 === 'string',
)
const payloadBytes = computed(() => {
  if (!payloadCaptured.value) return null
  try {
    const binary = window.atob(selectedPayload.value.payloadBase64)
    return Uint8Array.from(binary, (character) => character.charCodeAt(0))
  } catch {
    return null
  }
})
const decodedPayloadText = computed(() => {
  if (!payloadBytes.value) return null
  try {
    return new TextDecoder('utf-8', { fatal: true }).decode(payloadBytes.value)
  } catch {
    return null
  }
})
const payloadIsUtf8 = computed(() => decodedPayloadText.value !== null)
const readablePayloadText = computed(() => {
  if (!payloadBytes.value) return null
  return decodedPayloadText.value ?? escapedByteView(payloadBytes.value)
})

function escapedByteView(bytes) {
  let value = ''
  for (const byte of bytes) {
    if (byte === 0x0a || byte === 0x09 || (byte >= 0x20 && byte <= 0x7e)) {
      value += String.fromCharCode(byte)
      continue
    }
    value += `\\x${byte.toString(16).padStart(2, '0').toUpperCase()}`
  }
  return value
}

async function handleFailure(error, summary) {
  if (isAuthenticationError(error)) {
    await router.replace({ name: 'login', query: { redirect: route.fullPath } })
    return
  }
  toast.add({ severity: 'error', summary, detail: error.message, life: 5000 })
}

async function refresh() {
  try {
    await gateway.refresh()
  } catch (error) {
    await handleFailure(error, 'Gateway request failed')
  }
}

async function changePage(event) {
  try {
    await gateway.loadDeviceEvents(event.first)
  } catch (error) {
    await handleFailure(error, 'Could not load device events')
  }
}

async function inspectPayload(event) {
  selectedEvent.value = event
  selectedPayload.value = null
  payloadVisible.value = true
  payloadLoading.value = true
  try {
    selectedPayload.value = await gateway.loadDeviceEventPayload(event.id)
  } catch (error) {
    await handleFailure(error, 'Could not load raw payload')
    payloadVisible.value = false
  } finally {
    payloadLoading.value = false
  }
}

function payloadExtension(dataFormat) {
  return {
    jsonData: 'json',
    xmlData: 'xml',
    boundaryData: 'bin',
    noData: 'bin',
  }[dataFormat]
}

function downloadPayload() {
  if (!selectedPayload.value || !payloadBytes.value) return
  const blob = new Blob([payloadBytes.value], {
    type: 'application/octet-stream',
  })
  const link = document.createElement('a')
  const objectURL = URL.createObjectURL(blob)
  link.href = objectURL
  link.download = `${selectedPayload.value.vendorEventId}.${payloadExtension(selectedPayload.value.dataFormat)}`
  link.click()
  URL.revokeObjectURL(objectURL)
}

async function copyReadablePayload() {
  if (!selectedPayload.value || readablePayloadText.value === null) return
  try {
    await navigator.clipboard.writeText(readablePayloadText.value)
    toast.add({
      severity: 'success',
      summary: 'Copied',
      detail: 'Visible decoded payload copied.',
      life: 2500,
    })
  } catch {
    toast.add({
      severity: 'error',
      summary: 'Copy failed',
      detail: 'Your browser did not permit clipboard access.',
      life: 4000,
    })
  }
}
</script>

<template>
  <div
    class="mb-7 flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between"
  >
    <div>
      <p class="text-sm font-medium text-cyan-700">PushSDK event stream</p>
      <h1 class="mt-1 text-3xl font-semibold tracking-tight text-slate-950">
        Device events
      </h1>
      <p class="mt-2 max-w-3xl text-sm leading-6 text-slate-500">
        Every structurally valid event item received from a terminal. The
        gateway does not classify, transform, or omit its payload.
      </p>
    </div>
    <div class="flex w-full flex-col gap-3 sm:w-auto sm:flex-row">
      <span class="relative"
        ><i
          class="pi pi-search absolute left-3 top-1/2 -translate-y-1/2 text-sm text-slate-400" /><InputText
          v-model="filter"
          class="w-full !pl-9 sm:w-72"
          placeholder="Filter event ID or terminal" /></span
      ><Button
        label="Refresh"
        icon="pi pi-refresh"
        :loading="gateway.refreshing"
        @click="refresh"
      />
    </div>
  </div>

  <Card class="border border-slate-200 shadow-sm">
    <template #content>
      <div
        class="mb-5 flex flex-col gap-2 text-sm text-slate-500 sm:flex-row sm:items-center sm:justify-between"
      >
        <span
          >Showing {{ currentPageStart }}–{{ currentPageEnd }} of
          {{ gateway.deviceEventsTotal }}</span
        >
        <span v-if="filter" class="text-cyan-700"
          >{{ displayedEvents.length }} matching event{{
            displayedEvents.length === 1 ? '' : 's'
          }}
          on this page</span
        >
      </div>
      <DataTable
        v-if="displayedEvents.length"
        :value="displayedEvents"
        striped-rows
        scrollable
        scroll-height="flex"
        class="text-sm"
      >
        <Column header="Received">
          <template #body="{ data }">
            <div>
              <p class="font-medium text-slate-800">
                {{ formatTime(data.receivedAt) }}
              </p>
              <p class="mt-1 text-xs text-slate-400">
                {{ formatShortTime(data.receivedAt) }}
              </p>
            </div>
          </template>
        </Column>
        <Column field="vendorEventId" header="Vendor event ID">
          <template #body="{ data }">
            <code class="font-mono text-xs text-slate-700">{{
              data.vendorEventId
            }}</code>
          </template>
        </Column>
        <Column field="terminalSerialNumber" header="Terminal" />
        <Column header="Data format">
          <template #body="{ data }"
            ><Tag :value="data.dataFormat" severity="info" rounded
          /></template>
        </Column>
        <Column header="Payload">
          <template #body="{ data }">
            <Tag
              :value="data.payloadAvailable ? 'Captured' : 'Unavailable'"
              :severity="data.payloadAvailable ? 'success' : 'secondary'"
              rounded
            />
          </template>
        </Column>
        <Column header="">
          <template #body="{ data }">
            <Button
              label="Inspect"
              icon="pi pi-code"
              text
              @click="inspectPayload(data)"
            />
          </template>
        </Column>
      </DataTable>
      <div
        v-else-if="gateway.eventsLoading"
        class="flex items-center justify-center gap-3 py-16 text-sm font-medium text-slate-500"
      >
        <ProgressSpinner stroke-width="4" class="h-6 w-6" /> Loading device
        events
      </div>
      <div
        v-else
        class="rounded-xl border border-dashed border-slate-200 bg-slate-50 px-6 py-16 text-center"
      >
        <i class="pi pi-code text-3xl text-slate-400" />
        <p class="mt-4 font-medium text-slate-700">
          {{
            filter
              ? 'No matching device events'
              : 'No device events received yet'
          }}
        </p>
        <p class="mt-2 text-sm text-slate-500">
          {{
            filter
              ? 'Clear the page filter or move to another page.'
              : 'The list populates when a configured terminal sends a PushSDK Event request.'
          }}
        </p>
      </div>
      <Paginator
        v-if="gateway.deviceEventsTotal > gateway.pageSize"
        class="mt-6"
        :first="gateway.deviceEventsOffset"
        :rows="gateway.pageSize"
        :total-records="gateway.deviceEventsTotal"
        @page="changePage"
      />
    </template>
  </Card>

  <Dialog
    v-model:visible="payloadVisible"
    modal
    :draggable="false"
    header="Raw device-event payload"
    class="w-[min(96vw,72rem)]"
  >
    <div
      v-if="payloadLoading"
      class="flex items-center justify-center gap-3 py-16 text-sm text-slate-500"
    >
      <ProgressSpinner stroke-width="4" class="h-6 w-6" /> Loading exact source
      payload
    </div>
    <template v-else-if="selectedPayload">
      <div
        class="mb-5 grid gap-3 rounded-xl bg-slate-50 p-4 text-sm sm:grid-cols-3"
      >
        <div>
          <p class="text-xs font-medium uppercase tracking-wide text-slate-400">
            Vendor event ID
          </p>
          <code class="mt-1 block break-all text-xs text-slate-700">{{
            selectedPayload.vendorEventId
          }}</code>
        </div>
        <div>
          <p class="text-xs font-medium uppercase tracking-wide text-slate-400">
            Data format
          </p>
          <p class="mt-1 font-medium text-slate-700">
            {{ selectedPayload.dataFormat }}
          </p>
        </div>
        <div>
          <p class="text-xs font-medium uppercase tracking-wide text-slate-400">
            Terminal
          </p>
          <p class="mt-1 break-all font-medium text-slate-700">
            {{ selectedEvent?.terminalSerialNumber }}
          </p>
        </div>
      </div>
      <Message severity="info" :closable="false">
        The exact event bytes are shown below. Valid UTF-8 is rendered verbatim
        without parsing or formatting. In mixed or binary payloads, non-text
        bytes appear as <code>\xHH</code> so no bytes are hidden; downloading
        always yields the exact source bytes.
      </Message>
      <template v-if="payloadCaptured">
        <Textarea
          :model-value="readablePayloadText"
          readonly
          rows="20"
          class="mt-5 w-full !font-mono !text-xs"
          aria-label="Readable raw device-event bytes"
        />
        <Message
          v-if="!payloadIsUtf8"
          severity="secondary"
          :closable="false"
          class="mt-5"
        >
          Non-text bytes are represented as <code>\xHH</code> in the view.
          Download the source when a byte-exact file is required.
        </Message>
        <div class="mt-5 flex flex-wrap justify-end gap-3">
          <Button
            label="Copy visible payload"
            icon="pi pi-copy"
            outlined
            @click="copyReadablePayload"
          />
          <Button
            label="Download decoded bytes"
            icon="pi pi-download"
            @click="downloadPayload"
          />
        </div>
      </template>
      <Message v-else severity="warn" :closable="false" class="mt-5">
        This event predates raw-payload capture. Its source payload was never
        retained, so none can be shown or reconstructed.
      </Message>
    </template>
  </Dialog>
</template>
