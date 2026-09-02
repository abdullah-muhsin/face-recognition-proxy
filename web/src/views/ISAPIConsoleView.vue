<script setup>
import { useGatewayStore } from '../stores/gateway'
import { formatTime, isAuthenticationError } from '../lib/presentation'

const gateway = useGatewayStore()
const router = useRouter()
const route = useRoute()
const toast = useToast()

const selectedTerminal = ref('')
const method = ref('GET')
const url = ref('/ISAPI/System/deviceInfo')
const dataFormat = ref('noData')
const payload = ref('')
const expiresInSeconds = ref(60)
const submitting = ref(false)
const selectedCommand = ref(null)
const selectedPayload = ref(null)
const detailVisible = ref(false)
const detailLoading = ref(false)

const methods = ['GET', 'POST', 'PUT', 'DELETE']
const formats = ['noData', 'jsonData', 'xmlData', 'boundaryData']
const onlineTerminals = computed(() =>
  gateway.overview.terminals.filter((terminal) => terminal.status === 'online'),
)
const textPayload = computed(
  () => dataFormat.value === 'jsonData' || dataFormat.value === 'xmlData',
)
const boundaryPayload = computed(() => dataFormat.value === 'boundaryData')
const payloadPlaceholder = computed(() => {
  if (boundaryPayload.value)
    return 'Base64 of the complete multipart/form-data payload'
  if (dataFormat.value === 'jsonData') return '{\n  "Example": true\n}'
  return '<Example />'
})
const pageStart = computed(() =>
  gateway.isapiCommandsTotal === 0 ? 0 : gateway.isapiCommandsOffset + 1,
)
const pageEnd = computed(() =>
  Math.min(
    gateway.isapiCommandsOffset + gateway.isapiCommands.length,
    gateway.isapiCommandsTotal,
  ),
)
const responseBytes = computed(() =>
  base64Bytes(selectedPayload.value?.responseDataBase64),
)
const requestBytes = computed(() =>
  base64Bytes(selectedPayload.value?.requestDataBase64),
)
const requestText = computed(() => readableBytes(requestBytes.value))
const responseText = computed(() => readableBytes(responseBytes.value))

onMounted(() => {
  gateway
    .loadISAPICommands()
    .catch((error) =>
      handleFailure(error, 'Could not load ISAPI command history'),
    )
})

function base64Bytes(value) {
  if (typeof value !== 'string') return null
  try {
    return Uint8Array.from(window.atob(value), (character) =>
      character.charCodeAt(0),
    )
  } catch {
    return null
  }
}

function readableBytes(bytes) {
  if (!bytes) return null
  try {
    return new TextDecoder('utf-8', { fatal: true }).decode(bytes)
  } catch {
    let view = ''
    for (const byte of bytes) {
      if (byte === 0x0a || byte === 0x09 || (byte >= 0x20 && byte <= 0x7e)) {
        view += String.fromCharCode(byte)
      } else {
        view += `\\x${byte.toString(16).padStart(2, '0').toUpperCase()}`
      }
    }
    return view
  }
}

function statusSeverity(status) {
  return {
    queued: 'info',
    sent: 'warn',
    completed: 'success',
    expired: 'secondary',
  }[status]
}

function payloadLabel(format) {
  return {
    noData: 'No data',
    jsonData: 'JSON text',
    xmlData: 'XML text',
    boundaryData: 'Multipart bytes',
  }[format]
}

function setFormat(value) {
  if (dataFormat.value === value) return
  dataFormat.value = value
  payload.value = ''
}

async function handleFailure(error, summary) {
  if (isAuthenticationError(error)) {
    await router.replace({ name: 'login', query: { redirect: route.fullPath } })
    return
  }
  toast.add({ severity: 'error', summary, detail: error.message, life: 5000 })
}

async function submit() {
  const input = {
    method: method.value,
    url: url.value,
    dataFormat: dataFormat.value,
    expiresInSeconds: expiresInSeconds.value,
  }
  if (textPayload.value) input.textData = payload.value
  if (boundaryPayload.value) input.dataBase64 = payload.value

  submitting.value = true
  try {
    const command = await gateway.queueISAPICommand(
      selectedTerminal.value,
      input,
    )
    selectedCommand.value = command
    toast.add({
      severity: 'success',
      summary: 'Command queued',
      detail: `${command.method} ${command.url}`,
      life: 3500,
    })
  } catch (error) {
    await handleFailure(error, 'Could not queue ISAPI command')
  } finally {
    submitting.value = false
  }
}

async function inspect(command) {
  selectedCommand.value = command
  selectedPayload.value = null
  detailVisible.value = true
  detailLoading.value = true
  try {
    selectedPayload.value = await gateway.loadISAPICommandPayload(command.uuid)
  } catch (error) {
    await handleFailure(error, 'Could not load ISAPI command detail')
    detailVisible.value = false
  } finally {
    detailLoading.value = false
  }
}

async function changePage(event) {
  try {
    await gateway.loadISAPICommands(event.first)
  } catch (error) {
    await handleFailure(error, 'Could not load ISAPI command history')
  }
}
</script>

<template>
  <div class="grid gap-5 xl:grid-cols-[minmax(0,1fr)_minmax(28rem,0.8fr)]">
    <section
      class="overflow-hidden rounded-lg border border-slate-300 bg-white"
    >
      <div class="border-b border-slate-200 bg-slate-50 px-4 py-3 sm:px-5">
        <p class="text-sm font-semibold text-slate-950">Command request</p>
        <p class="mt-0.5 text-xs text-slate-500">
          The exact request is queued for the selected terminal’s next PushSDK
          command poll. It is never sent through a direct-device fallback.
        </p>
      </div>
      <form class="space-y-4 p-4 sm:p-5" @submit.prevent="submit">
        <div class="grid gap-4 md:grid-cols-2">
          <label class="grid gap-1.5 text-sm font-medium text-slate-700">
            Terminal
            <select
              v-model="selectedTerminal"
              required
              class="h-10 rounded-md border border-slate-300 bg-white px-3 text-sm font-normal text-slate-800 shadow-sm outline-none focus:border-cyan-500 focus:ring-2 focus:ring-cyan-100"
            >
              <option disabled value="">Choose an online terminal</option>
              <option
                v-for="terminal in onlineTerminals"
                :key="terminal.serialNumber"
                :value="terminal.serialNumber"
              >
                {{ terminal.serialNumber }}
              </option>
            </select>
          </label>
          <label class="grid gap-1.5 text-sm font-medium text-slate-700">
            Delivery expiry
            <InputNumber
              v-model="expiresInSeconds"
              input-id="isapi-expiry"
              :min="1"
              :max="3600"
              suffix=" seconds"
              :use-grouping="false"
              class="w-full"
            />
          </label>
        </div>

        <div class="grid gap-4 md:grid-cols-[auto_minmax(0,1fr)]">
          <fieldset class="grid gap-1.5">
            <legend class="text-sm font-medium text-slate-700">Method</legend>
            <div
              class="flex flex-wrap gap-1"
              role="group"
              aria-label="ISAPI method"
            >
              <Button
                v-for="item in methods"
                :key="item"
                type="button"
                :label="item"
                size="small"
                :outlined="method !== item"
                :severity="method === item ? 'info' : 'secondary'"
                @click="method = item"
              />
            </div>
          </fieldset>
          <label class="grid gap-1.5 text-sm font-medium text-slate-700">
            ISAPI URL
            <InputText
              v-model="url"
              class="w-full font-mono text-sm"
              maxlength="4096"
              required
              aria-describedby="isapi-url-help"
            />
            <span id="isapi-url-help" class="text-xs font-normal text-slate-500"
              >An exact absolute `/ISAPI/` path. Query text is preserved;
              fragments, relative segments, and implicit URL rewriting are not
              accepted.</span
            >
          </label>
        </div>

        <fieldset class="grid gap-1.5">
          <legend class="text-sm font-medium text-slate-700">
            Data format
          </legend>
          <div
            class="flex flex-wrap gap-1"
            role="group"
            aria-label="ISAPI data format"
          >
            <Button
              v-for="item in formats"
              :key="item"
              type="button"
              :label="payloadLabel(item)"
              size="small"
              :outlined="dataFormat !== item"
              :severity="dataFormat === item ? 'info' : 'secondary'"
              @click="setFormat(item)"
            />
          </div>
        </fieldset>

        <div
          v-if="dataFormat === 'noData'"
          class="rounded-md border border-slate-200 bg-slate-50 px-3 py-4 text-sm text-slate-500"
        >
          This request has no payload bytes.
        </div>
        <label v-else class="grid gap-1.5 text-sm font-medium text-slate-700">
          {{
            boundaryPayload
              ? 'Raw multipart bytes (standard Base64)'
              : 'Payload text'
          }}
          <Textarea
            v-model="payload"
            class="min-h-52 w-full font-mono text-xs"
            :placeholder="payloadPlaceholder"
            :aria-label="
              boundaryPayload ? 'Multipart payload base64' : 'Payload text'
            "
          />
          <span class="text-xs font-normal text-slate-500">
            <template v-if="boundaryPayload">
              The Base64 value must be canonical standard Base64 and represents
              the complete multipart bytes without reconstruction.
            </template>
            <template v-else>
              Text is forwarded as entered in UTF-8; the gateway does not parse,
              format, or repair it.
            </template>
          </span>
        </label>

        <div
          class="flex flex-wrap items-center justify-between gap-3 border-t border-slate-200 pt-4"
        >
          <p class="text-xs text-slate-500">
            Commands require an online configured terminal and expire instead of
            being delivered later without an operator’s chosen deadline.
          </p>
          <Button
            type="submit"
            label="Queue command"
            icon="pi pi-send"
            :loading="submitting"
            :disabled="selectedTerminal === ''"
          />
        </div>
      </form>
    </section>

    <section
      class="overflow-hidden rounded-lg border border-slate-300 bg-white"
    >
      <div class="border-b border-slate-200 bg-slate-50 px-4 py-3 sm:px-5">
        <p class="text-sm font-semibold text-slate-950">Delivery contract</p>
      </div>
      <dl class="divide-y divide-slate-200 text-sm">
        <div
          class="grid gap-1 px-4 py-3 sm:grid-cols-[8rem_1fr] sm:gap-4 sm:px-5"
        >
          <dt class="font-medium text-slate-700">Queue</dt>
          <dd class="text-slate-600">
            The authenticated administrator creates a durable, audited request.
          </dd>
        </div>
        <div
          class="grid gap-1 px-4 py-3 sm:grid-cols-[8rem_1fr] sm:gap-4 sm:px-5"
        >
          <dt class="font-medium text-slate-700">Send</dt>
          <dd class="text-slate-600">
            The terminal receives the exact method, URL, format, and bytes in
            its next `CommandRequest` response.
          </dd>
        </div>
        <div
          class="grid gap-1 px-4 py-3 sm:grid-cols-[8rem_1fr] sm:gap-4 sm:px-5"
        >
          <dt class="font-medium text-slate-700">Result</dt>
          <dd class="text-slate-600">
            A declared `CommandResult` format and Base64 value are retained
            under the same command UUID.
          </dd>
        </div>
        <div
          class="grid gap-1 px-4 py-3 sm:grid-cols-[8rem_1fr] sm:gap-4 sm:px-5"
        >
          <dt class="font-medium text-slate-700">No fallback</dt>
          <dd class="text-slate-600">
            The console never opens a direct HTTP connection to the terminal and
            never infers missing result formats.
          </dd>
        </div>
      </dl>
    </section>
  </div>

  <section
    class="mt-5 overflow-hidden rounded-lg border border-slate-300 bg-white"
  >
    <div
      class="flex flex-col gap-3 border-b border-slate-200 bg-slate-50 px-4 py-3 sm:flex-row sm:items-center sm:justify-between sm:px-5"
    >
      <div>
        <p class="text-sm font-semibold text-slate-950">Command history</p>
        <p class="mt-0.5 text-xs text-slate-500">
          {{ pageStart }}–{{ pageEnd }} of
          {{ gateway.isapiCommandsTotal }} durable command records
        </p>
      </div>
      <Button
        label="Reload history"
        icon="pi pi-refresh"
        size="small"
        :loading="gateway.isapiCommandsLoading"
        @click="gateway.loadISAPICommands()"
      />
    </div>
    <DataTable
      v-if="gateway.isapiCommands.length"
      :value="gateway.isapiCommands"
      size="small"
      striped-rows
      scrollable
      class="text-sm"
    >
      <Column header="Created">
        <template #body="{ data }">
          <div>
            <p class="whitespace-nowrap text-xs text-slate-700">
              {{ formatTime(data.createdAt) }}
            </p>
            <p class="mt-1 text-xs text-slate-400">
              {{ data.createdByUsername }}
            </p>
          </div>
        </template>
      </Column>
      <Column header="Terminal">
        <template #body="{ data }">
          <code class="text-xs text-slate-700">{{
            data.terminalSerialNumber
          }}</code>
        </template>
      </Column>
      <Column header="Command">
        <template #body="{ data }">
          <p class="font-mono text-xs font-semibold text-slate-800">
            {{ data.method }}
          </p>
          <code class="break-all text-xs text-slate-600">{{ data.url }}</code>
        </template>
      </Column>
      <Column header="Format">
        <template #body="{ data }">
          <code class="text-xs text-slate-600">{{ data.dataFormat }}</code>
        </template>
      </Column>
      <Column header="Status">
        <template #body="{ data }">
          <Tag
            :value="data.status"
            :severity="statusSeverity(data.status)"
            rounded
          />
        </template>
      </Column>
      <Column header="Result">
        <template #body="{ data }">
          <Button
            label="Inspect"
            icon="pi pi-code"
            text
            size="small"
            @click="inspect(data)"
          />
        </template>
      </Column>
    </DataTable>
    <p v-else class="px-4 py-10 text-sm text-slate-500">
      No ISAPI commands have been queued.
    </p>
    <Paginator
      v-if="gateway.isapiCommandsTotal > gateway.pageSize"
      :first="gateway.isapiCommandsOffset"
      :rows="gateway.pageSize"
      :total-records="gateway.isapiCommandsTotal"
      class="border-t border-slate-200"
      @page="changePage"
    />
  </section>

  <Dialog
    v-model:visible="detailVisible"
    modal
    :header="
      selectedCommand
        ? `ISAPI command ${selectedCommand.uuid}`
        : 'ISAPI command'
    "
    :style="{ width: 'min(72rem, calc(100vw - 2rem))' }"
  >
    <div v-if="detailLoading" class="grid min-h-64 place-items-center">
      <ProgressSpinner aria-label="Loading ISAPI command detail" />
    </div>
    <template v-else-if="selectedPayload">
      <dl
        class="grid gap-px overflow-hidden rounded-md border border-slate-200 bg-slate-200 text-sm sm:grid-cols-2 lg:grid-cols-4"
      >
        <div class="bg-white px-3 py-2">
          <dt
            class="text-xs font-medium uppercase tracking-wide text-slate-500"
          >
            Terminal
          </dt>
          <dd class="mt-1 break-all font-mono text-xs text-slate-800">
            {{ selectedPayload.terminalSerialNumber }}
          </dd>
        </div>
        <div class="bg-white px-3 py-2">
          <dt
            class="text-xs font-medium uppercase tracking-wide text-slate-500"
          >
            Request
          </dt>
          <dd class="mt-1 font-mono text-xs text-slate-800">
            {{ selectedPayload.method }} {{ selectedPayload.url }}
          </dd>
        </div>
        <div class="bg-white px-3 py-2">
          <dt
            class="text-xs font-medium uppercase tracking-wide text-slate-500"
          >
            Status
          </dt>
          <dd class="mt-1">
            <Tag
              :value="selectedPayload.status"
              :severity="statusSeverity(selectedPayload.status)"
              rounded
            />
          </dd>
        </div>
        <div class="bg-white px-3 py-2">
          <dt
            class="text-xs font-medium uppercase tracking-wide text-slate-500"
          >
            Expiry
          </dt>
          <dd class="mt-1 text-xs text-slate-800">
            {{ formatTime(selectedPayload.expiresAt) }}
          </dd>
        </div>
      </dl>
      <div class="mt-4 grid gap-4 lg:grid-cols-2">
        <div>
          <p class="mb-1.5 text-sm font-semibold text-slate-800">
            Request bytes
          </p>
          <p class="mb-2 text-xs text-slate-500">
            {{ selectedPayload.dataFormat }} · exact stored request bytes
          </p>
          <Textarea
            :model-value="requestText ?? ''"
            readonly
            class="min-h-72 w-full font-mono text-xs"
          />
        </div>
        <div>
          <p class="mb-1.5 text-sm font-semibold text-slate-800">
            Terminal result
          </p>
          <p class="mb-2 text-xs text-slate-500">
            <template v-if="selectedPayload.responseDataAvailable">
              {{ selectedPayload.responseDataFormat }} · exact Base64 source
              retained
            </template>
            <template v-else>No result has been received.</template>
          </p>
          <Textarea
            :model-value="responseText ?? ''"
            readonly
            class="min-h-72 w-full font-mono text-xs"
          />
        </div>
      </div>
      <details
        class="mt-4 rounded-md border border-slate-200 bg-slate-50 px-3 py-2"
      >
        <summary class="cursor-pointer text-sm font-medium text-slate-700">
          Stored Base64 values
        </summary>
        <label class="mt-3 grid gap-1 text-xs font-medium text-slate-600">
          Request
          <Textarea
            :model-value="selectedPayload.requestDataBase64"
            readonly
            class="min-h-24 w-full font-mono text-xs"
          />
        </label>
        <label class="mt-3 grid gap-1 text-xs font-medium text-slate-600">
          Result
          <Textarea
            :model-value="selectedPayload.responseDataBase64 ?? ''"
            readonly
            class="min-h-24 w-full font-mono text-xs"
          />
        </label>
      </details>
    </template>
  </Dialog>
</template>
