<script setup>
import { useGatewayStore } from '../stores/gateway'
import {
  formatShortTime,
  formatTime,
  isAuthenticationError,
} from '../lib/presentation'

const gateway = useGatewayStore()
const router = useRouter()
const route = useRoute()
const toast = useToast()
const payloadVisible = ref(false)
const payloadLoading = ref(false)
const selectedEvent = ref(null)
const selectedPayload = ref(null)
const categories = [
  { value: 'all', label: 'All' },
  { value: 'event', label: 'Event' },
  { value: 'operation', label: 'Operation' },
  { value: 'exception', label: 'Exception' },
  { value: 'alarm', label: 'Alarm' },
]
const sources = [
  { value: 'all', label: 'All sources' },
  { value: 'pushsdk', label: 'PushSDK delivery' },
  { value: 'isapi', label: 'Retained ISAPI history' },
]
const category = ref(gateway.deviceEventQuery.category)
const subtype = ref(gateway.deviceEventQuery.subtype)
const terminal = ref(gateway.deviceEventQuery.terminal)
const source = ref(gateway.deviceEventQuery.source)

const selectedCategory = computed(() =>
  categories.find((item) => item.value === category.value),
)
const terminalOptions = computed(() => gateway.overview.terminals)
const currentQuery = computed(() => ({
  category: category.value,
  subtype: subtype.value,
  terminal: terminal.value,
  source: source.value,
}))
const selectedSubtypeLabel = computed(() => {
  const selected = gateway.deviceEventSubtypes.find(
    (item) => item.code === subtype.value,
  )
  return selected ? subtypeLabel(selected) : ''
})
const currentViewLabel = computed(() => {
  if (category.value === 'all') return 'All archived events'
  return `${selectedCategory.value.label} events`
})
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
const pictureSource = computed(() => {
  const picture = selectedPayload.value?.picture
  if (!picture) return null
  return `data:${picture.contentType};base64,${picture.dataBase64}`
})
const selectedEventContext = computed(() => {
  const accessEvent = selectedEvent.value?.accessEvent
  if (!accessEvent) return []
  return [
    {
      label: 'Event type',
      value: `${accessEvent.majorEventType}/${accessEvent.subEventType}`,
    },
    { label: 'Documented subtype', value: accessEvent.subtypeLabel },
    { label: 'Vendor description', value: accessEvent.eventDescription },
    { label: 'State', value: accessEvent.eventState },
    { label: 'Device', value: accessEvent.deviceName },
    { label: 'Source IP', value: accessEvent.sourceIpAddress },
    { label: 'Source MAC', value: accessEvent.sourceMacAddress },
    { label: 'Channel', value: accessEvent.channelId },
    { label: 'Terminal short serial', value: accessEvent.shortSerialNumber },
    { label: 'Event serial', value: accessEvent.eventSerialNumber },
    { label: 'Previous event serial', value: accessEvent.frontSerialNumber },
    { label: 'Active post count', value: accessEvent.activePostCount },
    { label: 'User type', value: accessEvent.userType },
    {
      label: 'Declared verification mode',
      value: accessEvent.currentVerifyMode,
    },
    { label: 'Current event', value: booleanText(accessEvent.currentEvent) },
    { label: 'Mask', value: accessEvent.mask },
    { label: 'Pictures declared', value: accessEvent.picturesNumber },
    {
      label: 'Password verification enabled',
      value: booleanText(accessEvent.purePwdVerifyEnable),
    },
    { label: 'Face rectangle', value: faceRectText(accessEvent.faceRect) },
  ].filter(
    (item) =>
      item.value !== null && item.value !== undefined && item.value !== '',
  )
})
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
    await gateway.loadDeviceEvents(0, currentQuery.value)
  } catch (error) {
    await handleFailure(error, 'Could not reload device events')
  }
}

async function changePage(event) {
  try {
    await gateway.loadDeviceEvents(event.first, currentQuery.value)
  } catch (error) {
    await handleFailure(error, 'Could not load device events')
  }
}

async function selectCategory(value) {
  if (value === category.value) return
  category.value = value
  subtype.value = null
  await refresh()
}

async function selectSubtype(value) {
  if (value === subtype.value) return
  subtype.value = value
  await refresh()
}

async function selectTerminal() {
  await refresh()
}

async function selectSource() {
  await refresh()
}

function categoryLabel(accessEvent) {
  const category = categories.find(
    (item) => item.value === accessEvent.category,
  )
  return category ? category.label : `Major type ${accessEvent.majorEventType}`
}

function accessEventTitle(accessEvent) {
  return accessEvent.subtypeLabel || `Code ${accessEvent.subEventType}`
}

function subtypeLabel(value) {
  return value.label ? `${value.code} · ${value.label}` : `Code ${value.code}`
}

function booleanText(value) {
  if (value === true) return 'true'
  if (value === false) return 'false'
  return null
}

function faceRectText(faceRect) {
  if (!faceRect) return null
  const values = ['x', 'y', 'width', 'height']
    .filter((key) => faceRect[key] !== null && faceRect[key] !== undefined)
    .map((key) => `${key}=${faceRect[key]}`)
  return values.length ? values.join(' · ') : null
}

async function inspectPayload(event) {
  selectedEvent.value = event
  selectedPayload.value = null
  payloadVisible.value = true
  payloadLoading.value = true
  try {
    selectedPayload.value = await gateway.loadDeviceEventPayload(
      event.source,
      event.id,
    )
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
  link.download = `${selectedPayload.value.sourceRecordId}.${payloadExtension(selectedPayload.value.dataFormat)}`
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
  <section class="overflow-hidden rounded-lg border border-slate-300 bg-white">
    <div class="border-b border-slate-200 bg-slate-50 px-4 pt-3 sm:px-5">
      <div class="flex flex-wrap items-center gap-1" role="tablist">
        <Button
          v-for="item in categories"
          :key="item.value"
          :label="item.label"
          size="small"
          :text="category !== item.value"
          :severity="category === item.value ? 'info' : 'secondary'"
          :aria-selected="category === item.value"
          role="tab"
          @click="selectCategory(item.value)"
        />
      </div>
      <div
        class="flex flex-col gap-3 py-3 sm:flex-row sm:items-center sm:justify-between"
      >
        <div class="flex flex-wrap items-center gap-2">
          <span
            v-if="category !== 'all'"
            class="text-xs font-medium uppercase tracking-wide text-slate-500"
            >Sub type</span
          >
          <div
            v-if="category !== 'all'"
            class="flex max-w-full flex-wrap items-center gap-1"
            role="group"
            aria-label="Access-event subtype"
          >
            <Button
              label="All"
              size="small"
              :outlined="subtype !== null"
              :severity="subtype === null ? 'info' : 'secondary'"
              @click="selectSubtype(null)"
            />
            <Button
              v-for="item in gateway.deviceEventSubtypes"
              :key="item.code"
              :label="subtypeLabel(item)"
              size="small"
              :outlined="subtype !== item.code"
              :severity="subtype === item.code ? 'info' : 'secondary'"
              @click="selectSubtype(item.code)"
            />
          </div>
          <span v-else class="text-sm text-slate-500"
            >All retained source records and classification states</span
          >
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <label class="sr-only" for="event-terminal">Terminal</label>
          <select
            id="event-terminal"
            v-model="terminal"
            class="h-9 min-w-48 rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-700 shadow-sm outline-none focus:border-cyan-500 focus:ring-2 focus:ring-cyan-100"
            @change="selectTerminal"
          >
            <option value="">All terminals</option>
            <option
              v-for="item in terminalOptions"
              :key="item.serialNumber"
              :value="item.serialNumber"
            >
              {{ item.serialNumber }}
            </option>
          </select>
          <label class="sr-only" for="event-source">Archive source</label>
          <select
            id="event-source"
            v-model="source"
            class="h-9 min-w-48 rounded-md border border-slate-300 bg-white px-3 text-sm text-slate-700 shadow-sm outline-none focus:border-cyan-500 focus:ring-2 focus:ring-cyan-100"
            @change="selectSource"
          >
            <option v-for="item in sources" :key="item.value" :value="item.value">
              {{ item.label }}
            </option>
          </select>
          <Button
            label="Reload archive"
            icon="pi pi-refresh"
            size="small"
            :loading="gateway.eventsLoading"
            @click="refresh"
          />
        </div>
      </div>
    </div>
    <div
      class="flex flex-col gap-1 border-b border-slate-200 px-4 py-3 text-xs text-slate-600 sm:flex-row sm:items-center sm:justify-between"
    >
      <span
        >{{ currentViewLabel }} · rows {{ currentPageStart }}–{{
          currentPageEnd
        }}
        of {{ gateway.deviceEventsTotal }}</span
      >
      <span v-if="selectedSubtypeLabel" class="text-cyan-700">{{
        selectedSubtypeLabel
      }}</span>
    </div>
    <DataTable
      v-if="gateway.deviceEvents.length"
      :value="gateway.deviceEvents"
      size="small"
      striped-rows
      scrollable
      scroll-height="flex"
      class="text-sm"
    >
      <Column header="Archived">
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
      <Column header="Access event">
        <template #body="{ data }">
          <template v-if="data.accessEvent">
            <div class="flex items-center gap-2">
              <Tag
                :value="categoryLabel(data.accessEvent)"
                severity="info"
                rounded
              />
              <code class="text-xs text-slate-500"
                >{{ data.accessEvent.majorEventType }}/{{
                  data.accessEvent.subEventType
                }}</code
              >
              <Tag
                v-if="data.accessEvent.eventState"
                :value="data.accessEvent.eventState"
                severity="secondary"
                rounded
              />
            </div>
            <p
              class="mt-1 max-w-64 truncate font-medium text-slate-700"
              :title="accessEventTitle(data.accessEvent)"
            >
              {{ accessEventTitle(data.accessEvent) }}
            </p>
            <p
              v-if="data.accessEvent.currentVerifyMode"
              class="mt-1 max-w-64 truncate text-xs text-slate-500"
              :title="data.accessEvent.currentVerifyMode"
            >
              Declared: {{ data.accessEvent.currentVerifyMode }}
            </p>
          </template>
          <Tag
            v-else
            value="Unclassified raw event"
            severity="secondary"
            rounded
          />
        </template>
      </Column>
      <Column header="Occurred">
        <template #body="{ data }">
          <span v-if="data.accessEvent?.occurredAt">{{
            formatTime(data.accessEvent.occurredAt)
          }}</span>
          <span v-else class="text-slate-400">—</span>
        </template>
      </Column>
      <Column header="Identity">
        <template #body="{ data }">
          <template v-if="data.accessEvent">
            <p
              v-if="data.accessEvent.employeeName"
              class="font-medium text-slate-700"
            >
              {{ data.accessEvent.employeeName }}
            </p>
            <p
              v-if="data.accessEvent.employeeNumber"
              class="text-xs text-slate-500"
            >
              Employee {{ data.accessEvent.employeeNumber }}
            </p>
            <p
              v-if="data.accessEvent.cardNumber"
              class="text-xs text-slate-500"
            >
              Card {{ data.accessEvent.cardNumber }}
            </p>
            <span
              v-if="
                !data.accessEvent.employeeName &&
                !data.accessEvent.employeeNumber &&
                !data.accessEvent.cardNumber
              "
              class="text-slate-400"
              >—</span
            >
          </template>
          <span v-else class="text-slate-400">—</span>
        </template>
      </Column>
      <Column field="terminalSerialNumber" header="Terminal" />
      <Column header="Source record">
        <template #body="{ data }">
          <code
            class="block max-w-48 truncate text-xs text-slate-700"
            :title="data.sourceRecordId"
            >{{ data.sourceRecordId }}</code
          >
          <Tag
            :value="data.source === 'isapi' ? 'Retained ISAPI' : 'PushSDK delivery'"
            severity="secondary"
            rounded
            class="mt-1"
          />
        </template>
      </Column>
      <Column header="Device IP">
        <template #body="{ data }">
          <code
            v-if="data.accessEvent?.sourceIpAddress"
            class="text-xs text-slate-600"
            >{{ data.accessEvent.sourceIpAddress }}</code
          >
          <span v-else class="text-slate-400">—</span>
        </template>
      </Column>
      <Column header="Payload">
        <template #body="{ data }">
          <div class="flex items-center gap-1">
            <Tag
              :value="data.payloadAvailable ? 'Captured' : 'Not retained'"
              :severity="data.payloadAvailable ? 'success' : 'secondary'"
              rounded
            />
            <Button
              v-if="data.payloadAvailable"
              label="Inspect"
              icon="pi pi-code"
              text
              size="small"
              @click="inspectPayload(data)"
            />
          </div>
        </template>
      </Column>
    </DataTable>
    <div
      v-else-if="gateway.eventsLoading"
      class="flex items-center justify-center gap-3 px-4 py-16 text-sm text-slate-500"
    >
      <ProgressSpinner stroke-width="4" class="h-6 w-6" /> Loading device events
    </div>
    <div v-else class="px-4 py-12 text-center">
      <p class="font-medium text-slate-700">No events match this view.</p>
      <p class="mt-1 text-sm text-slate-500">
        Choose another category, subtype, or terminal to widen the query.
      </p>
    </div>
    <Paginator
      v-if="gateway.deviceEventsTotal > gateway.pageSize"
      class="border-t border-slate-200"
      :first="gateway.deviceEventsOffset"
      :rows="gateway.pageSize"
      :total-records="gateway.deviceEventsTotal"
      @page="changePage"
    />
  </section>

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
            Source record ID
          </p>
          <code class="mt-1 block break-all text-xs text-slate-700">{{
            selectedPayload.sourceRecordId
          }}</code>
        </div>
        <div>
          <p class="text-xs font-medium uppercase tracking-wide text-slate-400">
            Archive source
          </p>
          <p class="mt-1 font-medium text-slate-700">
            {{ selectedPayload.source === 'isapi' ? 'Retained ISAPI history' : 'PushSDK delivery' }}
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
      <section
        v-if="selectedEventContext.length"
        class="mb-5 rounded-xl border border-slate-200"
        aria-label="Declared access-event context"
      >
        <div class="border-b border-slate-200 bg-slate-50 px-4 py-3">
          <h3 class="text-sm font-semibold text-slate-800">
            Declared access-event context
          </h3>
          <p class="mt-1 text-xs text-slate-500">
            Values are projected directly from the terminal event and do not
            interpret the authentication outcome.
          </p>
        </div>
        <dl class="grid gap-x-5 gap-y-4 p-4 sm:grid-cols-2 lg:grid-cols-3">
          <div v-for="item in selectedEventContext" :key="item.label">
            <dt
              class="text-xs font-medium uppercase tracking-wide text-slate-400"
            >
              {{ item.label }}
            </dt>
            <dd class="mt-1 break-words font-medium text-slate-700">
              {{ item.value }}
            </dd>
          </div>
        </dl>
      </section>
      <Message severity="info" :closable="false">
        The exact event bytes are shown below. Valid UTF-8 is rendered verbatim
        without parsing or formatting. In mixed or binary payloads, non-text
        bytes appear as <code>\xHH</code> so no bytes are hidden; downloading
        always yields the exact source bytes.
      </Message>
      <figure
        v-if="pictureSource"
        class="mt-5 overflow-hidden rounded-xl border border-slate-200 bg-slate-50"
      >
        <div
          class="flex flex-wrap items-center justify-between gap-2 border-b border-slate-200 px-4 py-3"
        >
          <figcaption class="text-sm font-medium text-slate-800">
            Terminal-supplied picture
          </figcaption>
          <span class="text-xs text-slate-500">
            {{ selectedPayload.picture.fileName }} · exact JPEG part
          </span>
        </div>
        <img
          :src="pictureSource"
          alt="Terminal-supplied access-event picture"
          class="max-h-[32rem] w-full bg-slate-950 object-contain"
        />
      </figure>
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
