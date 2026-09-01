<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import Avatar from 'primevue/avatar'
import Button from 'primevue/button'
import Card from 'primevue/card'
import Column from 'primevue/column'
import DataTable from 'primevue/datatable'
import Divider from 'primevue/divider'
import Drawer from 'primevue/drawer'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Paginator from 'primevue/paginator'
import Password from 'primevue/password'
import ProgressSpinner from 'primevue/progressspinner'
import Tag from 'primevue/tag'
import Toast from 'primevue/toast'

const toast = useToast()
const pageSize = 25
const monitorLimit = 100
const monitorRefreshDelay = 350
const monitorReconnectDelay = 3000

const navigation = [
  { key: 'overview', label: 'Overview', icon: 'pi pi-home', description: 'Gateway posture at a glance' },
  { key: 'attendance', label: 'Attendance', icon: 'pi pi-clock', description: 'Received employee events' },
  { key: 'terminals', label: 'Terminals', icon: 'pi pi-desktop', description: 'Connected PushSDK devices' },
  { key: 'monitor', label: 'Live monitor', icon: 'pi pi-wave-pulse', description: 'Metadata-only gateway activity' },
]

const appReady = ref(false)
const authenticated = ref(false)
const signingIn = ref(false)
const refreshing = ref(false)
const recordsLoading = ref(false)
const mobileNavigationVisible = ref(false)
const activeView = ref('overview')
const signInUsername = ref('')
const signInPassword = ref('')
const operatorName = ref('')
const errorMessage = ref('')
const overview = ref({ attendanceTotal: 0, terminals: [] })
const records = ref([])
const recordsTotal = ref(0)
const attendanceOffset = ref(0)
const attendanceFilter = ref('')
const terminalFilter = ref('')
const monitor = ref([])
const socketState = ref('disconnected')
const lastUpdatedAt = ref(null)

let socket
let monitorReconnectTimer
let monitorRefreshTimer

const onlineTerminals = computed(() => overview.value.terminals.filter((terminal) => terminal.status === 'online').length)
const terminalsWithErrors = computed(() => overview.value.terminals.filter((terminal) => terminal.lastError).length)
const offlineTerminals = computed(() => overview.value.terminals.filter((terminal) => terminal.status === 'offline').length)
const displayedAttendance = computed(() => records.value.filter((record) => matchesRecord(record, attendanceFilter.value)))
const displayedTerminals = computed(() => overview.value.terminals.filter((terminal) => matchesTerminal(terminal, terminalFilter.value)))
const latestMonitorEvents = computed(() => monitor.value.slice(0, 6))
const monitorConnectionLabel = computed(() => ({
  connected: 'Live connected',
  connecting: 'Connecting',
  reconnecting: 'Reconnecting',
  disconnected: 'Disconnected',
  error: 'Connection error',
}[socketState.value] || 'Disconnected'))
const monitorConnectionSeverity = computed(() => ({
  connected: 'success',
  connecting: 'warn',
  reconnecting: 'warn',
  disconnected: 'secondary',
  error: 'danger',
}[socketState.value] || 'secondary'))
const operatorInitials = computed(() => {
  const source = operatorName.value || signInUsername.value || 'OP'
  return source.slice(0, 2).toUpperCase()
})
const currentPageStart = computed(() => recordsTotal.value === 0 ? 0 : attendanceOffset.value + 1)
const currentPageEnd = computed(() => Math.min(attendanceOffset.value + records.value.length, recordsTotal.value))

function activeNavigation() {
  return navigation.find((item) => item.key === activeView.value) || navigation[0]
}

function navigate(view) {
  activeView.value = view
  mobileNavigationVisible.value = false
}

async function request(url, options = {}) {
  const response = await fetch(url, { credentials: 'same-origin', ...options })
  const data = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(data.error || 'Request failed')
  return data
}

function setError(message, summary = 'Gateway request failed') {
  errorMessage.value = message
  toast.add({ severity: 'error', summary, detail: message, life: 5000 })
}

function clearError() {
  errorMessage.value = ''
}

function handleAuthenticationFailure(exception) {
  if (exception.message !== 'authentication required') return false
  disconnectMonitor()
  authenticated.value = false
  operatorName.value = ''
  return true
}

async function signIn() {
  clearError()
  signingIn.value = true
  try {
    await request('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: signInUsername.value, password: signInPassword.value }),
    })
    operatorName.value = signInUsername.value
    signInPassword.value = ''
    authenticated.value = true
    await refreshConsole()
    connectMonitor()
    toast.add({ severity: 'success', summary: 'Signed in', detail: 'The operator console is ready.', life: 3000 })
  } catch (exception) {
    setError(exception.message, 'Sign in failed')
  } finally {
    signingIn.value = false
  }
}

async function loadOverview() {
  overview.value = await request('/api/v1/admin/overview')
}

async function loadAttendance(offset = attendanceOffset.value) {
  recordsLoading.value = true
  try {
    const page = await request(`/api/v1/admin/attendance?limit=${pageSize}&offset=${offset}`)
    attendanceOffset.value = offset
    records.value = page.records
    recordsTotal.value = page.total
  } finally {
    recordsLoading.value = false
  }
}

async function refreshConsole() {
  clearError()
  refreshing.value = true
  try {
    await Promise.all([loadOverview(), loadAttendance()])
    lastUpdatedAt.value = new Date()
  } catch (exception) {
    if (!handleAuthenticationFailure(exception)) setError(exception.message)
  } finally {
    refreshing.value = false
  }
}

async function changeAttendancePage(event) {
  clearError()
  try {
    await loadAttendance(event.first)
  } catch (exception) {
    if (!handleAuthenticationFailure(exception)) setError(exception.message, 'Could not load attendance')
  }
}

function scheduleMonitorRefresh() {
  if (monitorRefreshTimer) return
  monitorRefreshTimer = window.setTimeout(() => {
    monitorRefreshTimer = undefined
    refreshConsole()
  }, monitorRefreshDelay)
}

function connectMonitor() {
  disconnectMonitor()
  if (!authenticated.value) return

  socketState.value = 'connecting'
  const scheme = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const connection = new WebSocket(`${scheme}//${window.location.host}/ws/v1/monitor`)
  socket = connection

  connection.onopen = () => {
    if (socket !== connection) return
    socketState.value = 'connected'
  }
  connection.onerror = () => {
    if (socket !== connection) return
    socketState.value = 'error'
  }
  connection.onclose = () => {
    if (socket !== connection) return
    socket = undefined
    if (!authenticated.value) {
      socketState.value = 'disconnected'
      return
    }
    socketState.value = 'reconnecting'
    monitorReconnectTimer = window.setTimeout(connectMonitor, monitorReconnectDelay)
  }
  connection.onmessage = (message) => {
    const event = JSON.parse(message.data)
    monitor.value.unshift(event)
    monitor.value = monitor.value.slice(0, monitorLimit)
    if (['attendance.received', 'pushsdk.login', 'pushsdk.logout'].includes(event.kind)) scheduleMonitorRefresh()
  }
}

function disconnectMonitor() {
  if (monitorReconnectTimer) {
    window.clearTimeout(monitorReconnectTimer)
    monitorReconnectTimer = undefined
  }
  if (monitorRefreshTimer) {
    window.clearTimeout(monitorRefreshTimer)
    monitorRefreshTimer = undefined
  }
  if (socket) {
    const connection = socket
    socket = undefined
    connection.close()
  }
  socketState.value = 'disconnected'
}

async function signOut() {
  clearError()
  try {
    await request('/api/v1/auth/logout', { method: 'POST' })
  } catch (exception) {
    setError(exception.message, 'Sign out failed')
    return
  }
  disconnectMonitor()
  authenticated.value = false
  operatorName.value = ''
  overview.value = { attendanceTotal: 0, terminals: [] }
  records.value = []
  recordsTotal.value = 0
  monitor.value = []
  attendanceOffset.value = 0
  attendanceFilter.value = ''
  terminalFilter.value = ''
  toast.add({ severity: 'success', summary: 'Signed out', detail: 'The operator session has ended.', life: 3000 })
}

function statusSeverity(status) {
  return {
    online: 'success',
    authenticating: 'warn',
    error: 'danger',
    offline: 'secondary',
  }[status] || 'info'
}

function statusLabel(status) {
  return status ? status.replaceAll('-', ' ') : 'Unknown'
}

function eventSeverity(kind) {
  if (kind.includes('error') || kind.includes('rejected')) return 'danger'
  if (kind.includes('attendance')) return 'success'
  if (kind.includes('login') || kind.includes('connected')) return 'info'
  return 'secondary'
}

function formatTime(value) {
  if (!value) return '—'
  const time = new Date(value)
  if (Number.isNaN(time.getTime())) return '—'
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' }).format(time)
}

function formatShortTime(value) {
  if (!value) return 'Never'
  const time = new Date(value)
  if (Number.isNaN(time.getTime())) return '—'
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit' }).format(time)
}

function matchesRecord(record, query) {
  const term = query.trim().toLocaleLowerCase()
  if (!term) return true
  return [
    record.employeeNumber,
    record.employeeName,
    record.terminalSerialNumber,
    record.verificationMethod,
    record.attendanceStatus,
    record.sourceFormat,
  ].filter(Boolean).join(' ').toLocaleLowerCase().includes(term)
}

function matchesTerminal(terminal, query) {
  const term = query.trim().toLocaleLowerCase()
  if (!term) return true
  return [terminal.serialNumber, terminal.pushSdkSerial, terminal.status, terminal.lastError]
    .filter(Boolean)
    .join(' ')
    .toLocaleLowerCase()
    .includes(term)
}

onMounted(async () => {
  try {
    await refreshConsole()
    if (authenticated.value) connectMonitor()
  } finally {
    appReady.value = true
  }
})

onBeforeUnmount(disconnectMonitor)
</script>

<template>
  <Toast position="top-right" />

  <main v-if="!appReady" class="grid min-h-screen place-items-center bg-slate-50 px-6">
    <div class="flex items-center gap-3 text-sm font-medium text-slate-600">
      <ProgressSpinner strokeWidth="4" class="h-6 w-6" />
      Preparing operator console
    </div>
  </main>

  <main v-else-if="!authenticated" class="min-h-screen bg-slate-50 px-4 py-8 sm:px-6 lg:px-8">
    <div class="mx-auto grid min-h-[calc(100vh-4rem)] max-w-6xl overflow-hidden rounded-3xl border border-slate-200 bg-white shadow-2xl shadow-slate-200/60 lg:grid-cols-[1.1fr_0.9fr]">
      <section class="order-2 flex flex-col justify-between bg-slate-950 p-8 text-white sm:p-12 lg:order-1">
        <div>
          <div class="mb-12 flex items-center gap-3">
            <span class="grid h-11 w-11 place-items-center rounded-2xl bg-cyan-400 text-xl text-slate-950 shadow-lg shadow-cyan-400/20"><i class="pi pi-bolt" /></span>
            <div>
              <p class="text-sm font-semibold tracking-wide text-cyan-300">PushSDK gateway</p>
              <p class="text-xs text-slate-400">Operations workspace</p>
            </div>
          </div>
          <p class="mb-4 text-sm font-semibold uppercase tracking-[0.22em] text-cyan-300">Device operations</p>
          <h1 class="max-w-md text-4xl font-semibold tracking-tight sm:text-5xl">A clearer view of every device interaction.</h1>
          <p class="mt-6 max-w-lg text-base leading-7 text-slate-300">Monitor terminal health, review received attendance, and follow live gateway activity from one focused workspace.</p>
        </div>
        <div class="mt-12 grid gap-4 sm:grid-cols-3 lg:grid-cols-1 xl:grid-cols-3">
          <div class="rounded-2xl border border-white/10 bg-white/5 p-4">
            <i class="pi pi-shield mb-3 text-cyan-300" />
            <p class="text-sm font-semibold">Private by design</p>
            <p class="mt-1 text-xs leading-5 text-slate-400">The live feed contains metadata only.</p>
          </div>
          <div class="rounded-2xl border border-white/10 bg-white/5 p-4">
            <i class="pi pi-wifi mb-3 text-cyan-300" />
            <p class="text-sm font-semibold">Live state</p>
            <p class="mt-1 text-xs leading-5 text-slate-400">Terminal connectivity is updated as events arrive.</p>
          </div>
          <div class="rounded-2xl border border-white/10 bg-white/5 p-4">
            <i class="pi pi-database mb-3 text-cyan-300" />
            <p class="text-sm font-semibold">Durable records</p>
            <p class="mt-1 text-xs leading-5 text-slate-400">Accepted attendance is stored before the device is acknowledged.</p>
          </div>
        </div>
      </section>

      <section class="order-1 flex items-center p-6 sm:p-12 lg:order-2">
        <div class="mx-auto w-full max-w-sm">
          <div class="mb-8">
            <p class="text-sm font-semibold text-cyan-700">Secure sign in</p>
            <h2 class="mt-2 text-3xl font-semibold tracking-tight text-slate-950">Welcome back</h2>
            <p class="mt-2 text-sm leading-6 text-slate-500">Use the gateway operator credentials configured for this environment.</p>
          </div>
          <form class="space-y-5" @submit.prevent="signIn">
            <label class="block">
              <span class="mb-2 block text-sm font-medium text-slate-700">Username</span>
              <InputText v-model="signInUsername" class="w-full" autocomplete="username" required />
            </label>
            <label class="block">
              <span class="mb-2 block text-sm font-medium text-slate-700">Password</span>
              <Password v-model="signInPassword" class="w-full" input-class="w-full" :feedback="false" toggle-mask autocomplete="current-password" required />
            </label>
            <Message v-if="errorMessage" severity="error" :closable="false">{{ errorMessage }}</Message>
            <Button type="submit" class="w-full" label="Open operator console" icon="pi pi-arrow-right" icon-pos="right" :loading="signingIn" />
          </form>
          <p class="mt-8 text-center text-xs leading-5 text-slate-400">Gateway access is limited to authorised operators. Sessions use a same-site operator cookie.</p>
        </div>
      </section>
    </div>
  </main>

  <main v-else class="min-h-screen bg-slate-50 text-slate-900">
    <Drawer v-model:visible="mobileNavigationVisible" position="left" class="!w-80">
      <template #header>
        <div class="flex items-center gap-3">
          <span class="grid h-10 w-10 place-items-center rounded-xl bg-slate-950 text-lg text-cyan-300"><i class="pi pi-bolt" /></span>
          <div><p class="font-semibold text-slate-950">PushSDK gateway</p><p class="text-xs text-slate-500">Operations workspace</p></div>
        </div>
      </template>
      <nav class="space-y-1" aria-label="Operator console navigation">
        <Button v-for="item in navigation" :key="item.key" text class="w-full !justify-start" :class="activeView === item.key ? '!bg-slate-100 !text-slate-950' : '!text-slate-600'" :icon="item.icon" :label="item.label" @click="navigate(item.key)" />
      </nav>
      <Divider />
      <div class="rounded-xl bg-slate-50 p-4">
        <div class="mb-2 flex items-center gap-2"><i class="pi pi-shield text-cyan-700" /><span class="text-sm font-semibold text-slate-800">Privacy boundary</span></div>
        <p class="text-xs leading-5 text-slate-500">Protocol payloads, images, and credentials are not sent to this interface.</p>
      </div>
    </Drawer>

    <aside class="fixed inset-y-0 left-0 hidden w-72 flex-col border-r border-slate-200 bg-white px-4 py-6 lg:flex">
      <div class="mb-9 flex items-center gap-3 px-2">
        <span class="grid h-11 w-11 place-items-center rounded-2xl bg-slate-950 text-xl text-cyan-300"><i class="pi pi-bolt" /></span>
        <div><p class="font-semibold tracking-tight text-slate-950">PushSDK gateway</p><p class="text-xs text-slate-500">Operations workspace</p></div>
      </div>
      <nav class="space-y-1" aria-label="Operator console navigation">
        <Button v-for="item in navigation" :key="item.key" text class="w-full !justify-start" :class="activeView === item.key ? '!bg-slate-100 !text-slate-950' : '!text-slate-600'" :icon="item.icon" :label="item.label" @click="navigate(item.key)" />
      </nav>
      <div class="mt-auto rounded-2xl bg-slate-950 p-5 text-slate-100">
        <div class="mb-3 flex items-center gap-2 text-cyan-300"><i class="pi pi-shield" /><span class="text-sm font-semibold">Privacy boundary</span></div>
        <p class="text-xs leading-5 text-slate-400">Protocol payloads, images, and credentials remain outside this interface.</p>
      </div>
    </aside>

    <section class="lg:pl-72">
      <header class="sticky top-0 z-20 border-b border-slate-200 bg-slate-50/90 px-4 py-4 backdrop-blur sm:px-6 lg:px-10">
        <div class="mx-auto flex max-w-7xl items-center justify-between gap-4">
          <div class="flex min-w-0 items-center gap-3">
            <Button icon="pi pi-bars" text rounded class="lg:!hidden" aria-label="Open navigation" @click="mobileNavigationVisible = true" />
            <div class="min-w-0">
              <p class="truncate text-sm font-semibold text-slate-950">{{ activeNavigation().label }}</p>
              <p class="hidden truncate text-xs text-slate-500 sm:block">{{ activeNavigation().description }}</p>
            </div>
          </div>
          <div class="flex items-center gap-2 sm:gap-3">
            <Tag :value="monitorConnectionLabel" :severity="monitorConnectionSeverity" rounded class="hidden sm:inline-flex" />
            <Button icon="pi pi-refresh" text rounded aria-label="Refresh console" :loading="refreshing" @click="refreshConsole" />
            <Avatar :label="operatorInitials" shape="circle" class="!bg-slate-950 !text-xs !font-semibold !text-cyan-300" />
            <Button label="Sign out" icon="pi pi-sign-out" text class="hidden sm:inline-flex" @click="signOut" />
          </div>
        </div>
      </header>

      <section class="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:px-10 lg:py-8">
        <Message v-if="errorMessage" severity="error" class="mb-6" :closable="true" @close="clearError">{{ errorMessage }}</Message>

        <template v-if="activeView === 'overview'">
          <div class="mb-7 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <p class="text-sm font-medium text-cyan-700">Gateway snapshot</p>
              <h1 class="mt-1 text-3xl font-semibold tracking-tight text-slate-950">Operations at a glance</h1>
              <p class="mt-2 text-sm text-slate-500">{{ lastUpdatedAt ? `Last refreshed ${formatShortTime(lastUpdatedAt)}` : 'Loading the latest gateway state' }}</p>
            </div>
            <Button label="Refresh data" icon="pi pi-refresh" :loading="refreshing" @click="refreshConsole" />
          </div>

          <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
            <Card class="border border-slate-200 shadow-sm"><template #content><div class="flex items-start justify-between"><div><p class="text-sm font-medium text-slate-500">Attendance records</p><p class="mt-3 text-3xl font-semibold tracking-tight text-slate-950">{{ overview.attendanceTotal }}</p><p class="mt-2 text-xs text-slate-500">Accepted, durable events</p></div><span class="grid h-11 w-11 place-items-center rounded-xl bg-cyan-50 text-cyan-700"><i class="pi pi-clock text-lg" /></span></div></template></Card>
            <Card class="border border-slate-200 shadow-sm"><template #content><div class="flex items-start justify-between"><div><p class="text-sm font-medium text-slate-500">Configured terminals</p><p class="mt-3 text-3xl font-semibold tracking-tight text-slate-950">{{ overview.terminals.length }}</p><p class="mt-2 text-xs text-slate-500">Known PushSDK devices</p></div><span class="grid h-11 w-11 place-items-center rounded-xl bg-indigo-50 text-indigo-700"><i class="pi pi-desktop text-lg" /></span></div></template></Card>
            <Card class="border border-slate-200 shadow-sm"><template #content><div class="flex items-start justify-between"><div><p class="text-sm font-medium text-slate-500">Online now</p><p class="mt-3 text-3xl font-semibold tracking-tight text-slate-950">{{ onlineTerminals }}</p><p class="mt-2 text-xs text-slate-500">Active terminal sessions</p></div><span class="grid h-11 w-11 place-items-center rounded-xl bg-emerald-50 text-emerald-700"><i class="pi pi-check-circle text-lg" /></span></div></template></Card>
            <Card class="border border-slate-200 shadow-sm"><template #content><div class="flex items-start justify-between"><div><p class="text-sm font-medium text-slate-500">Needs attention</p><p class="mt-3 text-3xl font-semibold tracking-tight text-slate-950">{{ terminalsWithErrors }}</p><p class="mt-2 text-xs text-slate-500">Terminal error states</p></div><span class="grid h-11 w-11 place-items-center rounded-xl bg-amber-50 text-amber-700"><i class="pi pi-exclamation-triangle text-lg" /></span></div></template></Card>
          </div>

          <div class="mt-6 grid gap-6 xl:grid-cols-[1.35fr_0.65fr]">
            <Card class="border border-slate-200 shadow-sm">
              <template #title><div class="flex items-center justify-between gap-4"><span>Terminal posture</span><Button label="View terminals" icon="pi pi-arrow-right" icon-pos="right" text size="small" @click="navigate('terminals')" /></div></template>
              <template #subtitle>Connection status from the most recent PushSDK interaction.</template>
              <template #content>
                <DataTable v-if="overview.terminals.length" :value="overview.terminals" striped-rows class="text-sm">
                  <Column field="serialNumber" header="Terminal"><template #body="{ data }"><div><p class="font-medium text-slate-900">{{ data.serialNumber }}</p><p class="mt-1 text-xs text-slate-500">PushSDK {{ data.pushSdkSerial }}</p></div></template></Column>
                  <Column header="Status"><template #body="{ data }"><Tag :value="statusLabel(data.status)" :severity="statusSeverity(data.status)" rounded /></template></Column>
                  <Column header="Last seen"><template #body="{ data }">{{ formatShortTime(data.lastSeenAt) }}</template></Column>
                </DataTable>
                <div v-else class="rounded-xl border border-dashed border-slate-200 bg-slate-50 px-6 py-12 text-center"><i class="pi pi-desktop text-2xl text-slate-400" /><p class="mt-3 font-medium text-slate-700">No terminals configured</p><p class="mt-1 text-sm text-slate-500">Add a terminal mapping before waiting for PushSDK registration.</p></div>
              </template>
            </Card>
            <Card class="border border-slate-200 shadow-sm">
              <template #title><div class="flex items-center justify-between gap-4"><span>Live activity</span><Tag :value="monitorConnectionLabel" :severity="monitorConnectionSeverity" rounded /></div></template>
              <template #subtitle>Most recent metadata-only gateway activity.</template>
              <template #content>
                <div v-if="latestMonitorEvents.length" class="space-y-4">
                  <article v-for="(event, index) in latestMonitorEvents" :key="`${event.at}-${index}`" class="flex gap-3">
                    <span class="mt-1.5 h-2.5 w-2.5 shrink-0 rounded-full" :class="eventSeverity(event.kind) === 'danger' ? 'bg-red-500' : eventSeverity(event.kind) === 'success' ? 'bg-emerald-500' : 'bg-cyan-500'" />
                    <div class="min-w-0"><p class="truncate text-sm font-medium text-slate-800">{{ event.kind }}</p><p class="mt-1 text-sm leading-5 text-slate-500">{{ event.message }}</p><p class="mt-1 text-xs text-slate-400">{{ formatShortTime(event.at) }} · {{ event.terminal || 'Gateway' }}</p></div>
                  </article>
                </div>
                <div v-else class="rounded-xl border border-dashed border-slate-200 bg-slate-50 px-5 py-10 text-center"><i class="pi pi-wave-pulse text-2xl text-slate-400" /><p class="mt-3 font-medium text-slate-700">Waiting for activity</p><p class="mt-1 text-sm text-slate-500">The monitor will update as the gateway receives activity.</p></div>
                <Button class="mt-5 w-full" label="Open live monitor" icon="pi pi-wave-pulse" outlined @click="navigate('monitor')" />
              </template>
            </Card>
          </div>
        </template>

        <template v-else-if="activeView === 'attendance'">
          <div class="mb-7 flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
            <div><p class="text-sm font-medium text-cyan-700">Attendance records</p><h1 class="mt-1 text-3xl font-semibold tracking-tight text-slate-950">Received attendance</h1><p class="mt-2 text-sm text-slate-500">{{ recordsTotal }} accepted record{{ recordsTotal === 1 ? '' : 's' }} stored by the gateway.</p></div>
            <div class="flex w-full flex-col gap-3 sm:w-auto sm:flex-row"><span class="relative"><i class="pi pi-search absolute left-3 top-1/2 -translate-y-1/2 text-sm text-slate-400" /><InputText v-model="attendanceFilter" class="w-full !pl-9 sm:w-72" placeholder="Filter loaded page" /></span><Button label="Refresh" icon="pi pi-refresh" :loading="refreshing" @click="refreshConsole" /></div>
          </div>
          <Card class="border border-slate-200 shadow-sm">
            <template #content>
              <div class="mb-5 flex flex-col gap-2 text-sm text-slate-500 sm:flex-row sm:items-center sm:justify-between"><span>Showing {{ currentPageStart }}–{{ currentPageEnd }} of {{ recordsTotal }}</span><span v-if="attendanceFilter" class="text-cyan-700">{{ displayedAttendance.length }} matching record{{ displayedAttendance.length === 1 ? '' : 's' }} on this page</span></div>
              <DataTable v-if="displayedAttendance.length" :value="displayedAttendance" striped-rows scrollable scroll-height="flex" class="text-sm">
                <Column header="Occurred"><template #body="{ data }"><div><p class="font-medium text-slate-800">{{ formatTime(data.occurredAt) }}</p><p class="mt-1 text-xs text-slate-400">Received {{ formatShortTime(data.receivedAt) }}</p></div></template></Column>
                <Column header="Employee"><template #body="{ data }"><div><p class="font-medium text-slate-800">{{ data.employeeNumber }}</p><p v-if="data.employeeName" class="mt-1 text-xs text-slate-500">{{ data.employeeName }}</p></div></template></Column>
                <Column field="verificationMethod" header="Verification" />
                <Column header="Status"><template #body="{ data }"><Tag :value="data.attendanceStatus || 'Not specified'" severity="info" rounded /></template></Column>
                <Column field="terminalSerialNumber" header="Terminal" />
                <Column field="sourceFormat" header="Format" />
              </DataTable>
              <div v-else-if="recordsLoading" class="flex items-center justify-center gap-3 py-16 text-sm font-medium text-slate-500"><ProgressSpinner stroke-width="4" class="h-6 w-6" /> Loading attendance records</div>
              <div v-else class="rounded-xl border border-dashed border-slate-200 bg-slate-50 px-6 py-16 text-center"><i class="pi pi-clock text-3xl text-slate-400" /><p class="mt-4 font-medium text-slate-700">{{ attendanceFilter ? 'No matching records on this page' : 'No attendance records received yet' }}</p><p class="mt-2 text-sm text-slate-500">{{ attendanceFilter ? 'Clear the page filter or move to another page.' : 'The list will populate when an accepted attendance event reaches the gateway.' }}</p></div>
              <Paginator v-if="recordsTotal > pageSize" class="mt-6" :first="attendanceOffset" :rows="pageSize" :total-records="recordsTotal" @page="changeAttendancePage" />
            </template>
          </Card>
        </template>

        <template v-else-if="activeView === 'terminals'">
          <div class="mb-7 flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
            <div><p class="text-sm font-medium text-cyan-700">Terminal directory</p><h1 class="mt-1 text-3xl font-semibold tracking-tight text-slate-950">Connected devices</h1><p class="mt-2 text-sm text-slate-500">Every terminal configured to register through this gateway.</p></div>
            <div class="flex w-full flex-col gap-3 sm:w-auto sm:flex-row"><span class="relative"><i class="pi pi-search absolute left-3 top-1/2 -translate-y-1/2 text-sm text-slate-400" /><InputText v-model="terminalFilter" class="w-full !pl-9 sm:w-72" placeholder="Find a terminal" /></span><Button label="Refresh" icon="pi pi-refresh" :loading="refreshing" @click="refreshConsole" /></div>
          </div>
          <div class="mb-6 grid gap-4 sm:grid-cols-3">
            <Card class="border border-slate-200 shadow-sm"><template #content><p class="text-sm font-medium text-slate-500">Online</p><p class="mt-3 text-3xl font-semibold tracking-tight text-emerald-700">{{ onlineTerminals }}</p></template></Card>
            <Card class="border border-slate-200 shadow-sm"><template #content><p class="text-sm font-medium text-slate-500">Offline</p><p class="mt-3 text-3xl font-semibold tracking-tight text-slate-700">{{ offlineTerminals }}</p></template></Card>
            <Card class="border border-slate-200 shadow-sm"><template #content><p class="text-sm font-medium text-slate-500">With last error</p><p class="mt-3 text-3xl font-semibold tracking-tight text-amber-700">{{ terminalsWithErrors }}</p></template></Card>
          </div>
          <Card class="border border-slate-200 shadow-sm">
            <template #content>
              <DataTable v-if="displayedTerminals.length" :value="displayedTerminals" striped-rows class="text-sm">
                <Column header="Terminal"><template #body="{ data }"><div><p class="font-medium text-slate-900">{{ data.serialNumber }}</p><p class="mt-1 text-xs text-slate-500">PushSDK serial: {{ data.pushSdkSerial }}</p></div></template></Column>
                <Column header="Status"><template #body="{ data }"><Tag :value="statusLabel(data.status)" :severity="statusSeverity(data.status)" rounded /></template></Column>
                <Column header="Last seen"><template #body="{ data }">{{ formatTime(data.lastSeenAt) }}</template></Column>
                <Column header="Last error"><template #body="{ data }"><span :class="data.lastError ? 'text-red-700' : 'text-slate-400'">{{ data.lastError || '—' }}</span></template></Column>
              </DataTable>
              <div v-else class="rounded-xl border border-dashed border-slate-200 bg-slate-50 px-6 py-16 text-center"><i class="pi pi-desktop text-3xl text-slate-400" /><p class="mt-4 font-medium text-slate-700">{{ terminalFilter ? 'No matching terminals' : 'No terminals configured' }}</p><p class="mt-2 text-sm text-slate-500">{{ terminalFilter ? 'Try another terminal serial or status.' : 'Add a terminal mapping and restart the gateway to begin registration.' }}</p></div>
            </template>
          </Card>
        </template>

        <template v-else>
          <div class="mb-7 flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
            <div><p class="text-sm font-medium text-cyan-700">Live monitor</p><h1 class="mt-1 text-3xl font-semibold tracking-tight text-slate-950">Gateway activity</h1><p class="mt-2 text-sm text-slate-500">A live, metadata-only operational feed. Protocol bodies and biometric data are deliberately excluded.</p></div>
            <div class="flex items-center gap-3"><Tag :value="monitorConnectionLabel" :severity="monitorConnectionSeverity" rounded /><Button label="Reconnect" icon="pi pi-sync" outlined @click="connectMonitor" /></div>
          </div>
          <Card class="border border-slate-200 shadow-sm">
            <template #content>
              <div v-if="monitor.length" class="divide-y divide-slate-100">
                <article v-for="(event, index) in monitor" :key="`${event.at}-${index}`" class="grid gap-3 py-5 md:grid-cols-[10rem_11rem_1fr] md:items-start">
                  <p class="text-sm text-slate-500">{{ formatTime(event.at) }}</p>
                  <Tag :value="event.kind" :severity="eventSeverity(event.kind)" class="w-fit" />
                  <div><p class="text-sm font-medium text-slate-800">{{ event.message }}</p><p class="mt-1 text-xs text-slate-500">Source: {{ event.terminal || 'Gateway' }}</p></div>
                </article>
              </div>
              <div v-else class="rounded-xl border border-dashed border-slate-200 bg-slate-50 px-6 py-20 text-center"><i class="pi pi-wave-pulse text-3xl text-slate-400" /><p class="mt-4 font-medium text-slate-700">Waiting for gateway activity</p><p class="mt-2 text-sm text-slate-500">Keep this view open while testing terminal registration or attendance events.</p></div>
            </template>
          </Card>
        </template>
      </section>
    </section>
  </main>
</template>
