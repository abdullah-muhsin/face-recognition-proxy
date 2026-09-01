<script setup>
import { useGatewayStore } from '../stores/gateway'
import {
  formatShortTime,
  formatTime,
  isAuthenticationError,
  matchesRecord,
} from '../lib/presentation'

const gateway = useGatewayStore()
const router = useRouter()
const route = useRoute()
const toast = useToast()
const filter = ref('')
const displayedAttendance = computed(() =>
  gateway.records.filter((record) => matchesRecord(record, filter.value)),
)
const currentPageStart = computed(() =>
  gateway.recordsTotal === 0 ? 0 : gateway.attendanceOffset + 1,
)
const currentPageEnd = computed(() =>
  Math.min(
    gateway.attendanceOffset + gateway.records.length,
    gateway.recordsTotal,
  ),
)

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
    await gateway.loadAttendance(event.first)
  } catch (error) {
    await handleFailure(error, 'Could not load attendance')
  }
}
</script>

<template>
  <div
    class="mb-7 flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between"
  >
    <div>
      <p class="text-sm font-medium text-cyan-700">Attendance records</p>
      <h1 class="mt-1 text-3xl font-semibold tracking-tight text-slate-950">
        Received attendance
      </h1>
      <p class="mt-2 text-sm text-slate-500">
        {{ gateway.recordsTotal }} accepted record{{
          gateway.recordsTotal === 1 ? '' : 's'
        }}
        stored by the gateway.
      </p>
    </div>
    <div class="flex w-full flex-col gap-3 sm:w-auto sm:flex-row">
      <span class="relative"
        ><i
          class="pi pi-search absolute left-3 top-1/2 -translate-y-1/2 text-sm text-slate-400" /><InputText
          v-model="filter"
          class="w-full !pl-9 sm:w-72"
          placeholder="Filter loaded page" /></span
      ><Button
        label="Refresh"
        icon="pi pi-refresh"
        :loading="gateway.refreshing"
        @click="refresh"
      />
    </div>
  </div>
  <Card class="border border-slate-200 shadow-sm"
    ><template #content>
      <div
        class="mb-5 flex flex-col gap-2 text-sm text-slate-500 sm:flex-row sm:items-center sm:justify-between"
      >
        <span
          >Showing {{ currentPageStart }}–{{ currentPageEnd }} of
          {{ gateway.recordsTotal }}</span
        ><span v-if="filter" class="text-cyan-700"
          >{{ displayedAttendance.length }} matching record{{
            displayedAttendance.length === 1 ? '' : 's'
          }}
          on this page</span
        >
      </div>
      <DataTable
        v-if="displayedAttendance.length"
        :value="displayedAttendance"
        striped-rows
        scrollable
        scroll-height="flex"
        class="text-sm"
        ><Column header="Occurred"
          ><template #body="{ data }"
            ><div>
              <p class="font-medium text-slate-800">
                {{ formatTime(data.occurredAt) }}
              </p>
              <p class="mt-1 text-xs text-slate-400">
                Received {{ formatShortTime(data.receivedAt) }}
              </p>
            </div></template
          ></Column
        ><Column header="Employee"
          ><template #body="{ data }"
            ><div>
              <p class="font-medium text-slate-800">
                {{ data.employeeNumber }}
              </p>
              <p v-if="data.employeeName" class="mt-1 text-xs text-slate-500">
                {{ data.employeeName }}
              </p>
            </div></template
          ></Column
        ><Column field="verificationMethod" header="Verification" /><Column
          header="Status"
          ><template #body="{ data }"
            ><Tag
              :value="data.attendanceStatus || 'Not specified'"
              severity="info"
              rounded /></template></Column
        ><Column field="terminalSerialNumber" header="Terminal" /><Column
          field="sourceFormat"
          header="Format"
      /></DataTable>
      <div
        v-else-if="gateway.recordsLoading"
        class="flex items-center justify-center gap-3 py-16 text-sm font-medium text-slate-500"
      >
        <ProgressSpinner stroke-width="4" class="h-6 w-6" /> Loading attendance
        records
      </div>
      <div
        v-else
        class="rounded-xl border border-dashed border-slate-200 bg-slate-50 px-6 py-16 text-center"
      >
        <i class="pi pi-clock text-3xl text-slate-400" />
        <p class="mt-4 font-medium text-slate-700">
          {{
            filter
              ? 'No matching records on this page'
              : 'No attendance records received yet'
          }}
        </p>
        <p class="mt-2 text-sm text-slate-500">
          {{
            filter
              ? 'Clear the page filter or move to another page.'
              : 'The list will populate when an accepted attendance event reaches the gateway.'
          }}
        </p>
      </div>
      <Paginator
        v-if="gateway.recordsTotal > gateway.pageSize"
        class="mt-6"
        :first="gateway.attendanceOffset"
        :rows="gateway.pageSize"
        :total-records="gateway.recordsTotal"
        @page="changePage"
      /> </template
  ></Card>
</template>
