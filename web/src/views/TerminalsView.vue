<script setup>
import { useGatewayStore } from '../stores/gateway'
import {
  formatTime,
  isAuthenticationError,
  matchesTerminal,
  statusLabel,
  statusSeverity,
} from '../lib/presentation'

const gateway = useGatewayStore()
const router = useRouter()
const route = useRoute()
const toast = useToast()
const filter = ref('')
const displayedTerminals = computed(() =>
  gateway.overview.terminals.filter((terminal) =>
    matchesTerminal(terminal, filter.value),
  ),
)

async function refresh() {
  try {
    await gateway.refresh()
  } catch (error) {
    if (isAuthenticationError(error)) {
      await router.replace({
        name: 'login',
        query: { redirect: route.fullPath },
      })
      return
    }
    toast.add({
      severity: 'error',
      summary: 'Gateway request failed',
      detail: error.message,
      life: 5000,
    })
  }
}
</script>

<template>
  <div
    class="mb-7 flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between"
  >
    <div>
      <p class="text-sm font-medium text-cyan-700">Terminal directory</p>
      <h1 class="mt-1 text-3xl font-semibold tracking-tight text-slate-950">
        Connected devices
      </h1>
      <p class="mt-2 text-sm text-slate-500">
        Every terminal configured to register through this gateway.
      </p>
    </div>
    <div class="flex w-full flex-col gap-3 sm:w-auto sm:flex-row">
      <span class="relative"
        ><i
          class="pi pi-search absolute left-3 top-1/2 -translate-y-1/2 text-sm text-slate-400" /><InputText
          v-model="filter"
          class="w-full !pl-9 sm:w-72"
          placeholder="Find a terminal" /></span
      ><Button
        label="Refresh"
        icon="pi pi-refresh"
        :loading="gateway.refreshing"
        @click="refresh"
      />
    </div>
  </div>
  <div class="mb-6 grid gap-4 sm:grid-cols-3">
    <Card class="border border-slate-200 shadow-sm"
      ><template #content
        ><p class="text-sm font-medium text-slate-500">Online</p>
        <p class="mt-3 text-3xl font-semibold tracking-tight text-emerald-700">
          {{ gateway.onlineTerminals }}
        </p></template
      ></Card
    ><Card class="border border-slate-200 shadow-sm"
      ><template #content
        ><p class="text-sm font-medium text-slate-500">Offline</p>
        <p class="mt-3 text-3xl font-semibold tracking-tight text-slate-700">
          {{ gateway.offlineTerminals }}
        </p></template
      ></Card
    ><Card class="border border-slate-200 shadow-sm"
      ><template #content
        ><p class="text-sm font-medium text-slate-500">With last error</p>
        <p class="mt-3 text-3xl font-semibold tracking-tight text-amber-700">
          {{ gateway.terminalsWithErrors }}
        </p></template
      ></Card
    >
  </div>
  <Card class="border border-slate-200 shadow-sm"
    ><template #content>
      <DataTable
        v-if="displayedTerminals.length"
        :value="displayedTerminals"
        striped-rows
        class="text-sm"
        ><Column header="Terminal"
          ><template #body="{ data }"
            ><div>
              <p class="font-medium text-slate-900">{{ data.serialNumber }}</p>
              <p class="mt-1 text-xs text-slate-500">
                PushSDK serial: {{ data.pushSdkSerial }}
              </p>
            </div></template
          ></Column
        ><Column header="Status"
          ><template #body="{ data }"
            ><Tag
              :value="statusLabel(data.status)"
              :severity="statusSeverity(data.status)"
              rounded /></template></Column
        ><Column header="Last seen"
          ><template #body="{ data }">{{
            formatTime(data.lastSeenAt)
          }}</template></Column
        ><Column header="Last error"
          ><template #body="{ data }"
            ><span
              :class="data.lastError ? 'text-red-700' : 'text-slate-400'"
              >{{ data.lastError || '—' }}</span
            ></template
          ></Column
        ></DataTable
      >
      <div
        v-else
        class="rounded-xl border border-dashed border-slate-200 bg-slate-50 px-6 py-16 text-center"
      >
        <i class="pi pi-desktop text-3xl text-slate-400" />
        <p class="mt-4 font-medium text-slate-700">
          {{ filter ? 'No matching terminals' : 'No terminals configured' }}
        </p>
        <p class="mt-2 text-sm text-slate-500">
          {{
            filter
              ? 'Try another terminal serial or status.'
              : 'Add a terminal mapping and restart the gateway to begin registration.'
          }}
        </p>
      </div>
    </template></Card
  >
</template>
