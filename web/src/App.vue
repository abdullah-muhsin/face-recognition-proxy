<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

const authenticated = ref(false)
const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)
const overview = ref({ attendanceTotal: 0, terminals: [] })
const records = ref([])
const monitor = ref([])
const socketState = ref('disconnected')
let socket

const onlineTerminals = computed(() => overview.value.terminals.filter((terminal) => terminal.status === 'online').length)

async function request(url, options = {}) {
  const response = await fetch(url, { credentials: 'same-origin', ...options })
  const data = await response.json().catch(() => ({}))
  if (!response.ok) throw new Error(data.error || 'Request failed')
  return data
}

async function signIn() {
  error.value = ''
  loading.value = true
  try {
    await request('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: username.value, password: password.value }),
    })
    password.value = ''
    authenticated.value = true
    await refresh()
    connectMonitor()
  } catch (exception) {
    error.value = exception.message
  } finally {
    loading.value = false
  }
}

async function refresh() {
  try {
    const [currentOverview, page] = await Promise.all([
      request('/api/v1/admin/overview'),
      request('/api/v1/admin/attendance?limit=50&offset=0'),
    ])
    overview.value = currentOverview
    records.value = page.records
    return true
  } catch (exception) {
    if (exception.message === 'authentication required') authenticated.value = false
    else error.value = exception.message
    return false
  }
}

function connectMonitor() {
  if (socket) socket.close()
  socketState.value = 'connecting'
  const scheme = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  socket = new WebSocket(`${scheme}//${window.location.host}/ws/v1/monitor`)
  socket.onopen = () => { socketState.value = 'connected' }
  socket.onclose = () => { socketState.value = 'disconnected' }
  socket.onerror = () => { socketState.value = 'error' }
  socket.onmessage = (message) => {
    const event = JSON.parse(message.data)
    monitor.value.unshift(event)
    monitor.value = monitor.value.slice(0, 100)
    if (event.kind === 'attendance.received' || event.kind === 'pushsdk.login' || event.kind === 'pushsdk.logout') refresh()
  }
}

async function signOut() {
  await request('/api/v1/auth/logout', { method: 'POST' })
  if (socket) socket.close()
  authenticated.value = false
  overview.value = { attendanceTotal: 0, terminals: [] }
  records.value = []
  monitor.value = []
}

function formatTime(value) {
  return value ? new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value)) : '—'
}

onMounted(async () => {
  if (await refresh()) {
    authenticated.value = true
    connectMonitor()
  }
})
onBeforeUnmount(() => { if (socket) socket.close() })
</script>

<template>
  <main class="shell">
    <section v-if="!authenticated" class="login-card">
      <p class="eyebrow">PushSDK gateway</p>
      <h1>Operator console</h1>
      <p>Sign in to view registered terminals and received attendance events.</p>
      <form @submit.prevent="signIn">
        <label>Username <input v-model="username" autocomplete="username" required /></label>
        <label>Password <input v-model="password" type="password" autocomplete="current-password" required /></label>
        <p v-if="error" class="error">{{ error }}</p>
        <button :disabled="loading">{{ loading ? 'Signing in…' : 'Sign in' }}</button>
      </form>
    </section>

    <template v-else>
      <header>
        <div><p class="eyebrow">PushSDK gateway</p><h1>Live operations</h1></div>
        <div class="actions"><span class="connection" :data-state="socketState">Monitor {{ socketState }}</span><button class="secondary" @click="refresh">Refresh</button><button class="secondary" @click="signOut">Sign out</button></div>
      </header>
      <p v-if="error" class="error">{{ error }}</p>
      <section class="cards">
        <article><span>Attendance records</span><strong>{{ overview.attendanceTotal }}</strong></article>
        <article><span>Registered terminals</span><strong>{{ overview.terminals.length }}</strong></article>
        <article><span>Online terminals</span><strong>{{ onlineTerminals }}</strong></article>
      </section>
      <section class="panel">
        <h2>Terminals</h2>
        <table><thead><tr><th>Terminal</th><th>PushSDK serial</th><th>Status</th><th>Last seen</th><th>Last error</th></tr></thead>
          <tbody><tr v-for="terminal in overview.terminals" :key="terminal.serialNumber"><td>{{ terminal.serialNumber }}</td><td>{{ terminal.pushSdkSerial }}</td><td><span class="status" :data-status="terminal.status">{{ terminal.status }}</span></td><td>{{ formatTime(terminal.lastSeenAt) }}</td><td>{{ terminal.lastError || '—' }}</td></tr></tbody>
        </table>
      </section>
      <section class="panel">
        <h2>Attendance events</h2>
        <p v-if="records.length === 0" class="empty">No attendance records have been received.</p>
        <table v-else><thead><tr><th>Time</th><th>Employee</th><th>Verification</th><th>Attendance status</th><th>Terminal</th><th>Format</th></tr></thead>
          <tbody><tr v-for="record in records" :key="record.id"><td>{{ formatTime(record.occurredAt) }}</td><td>{{ record.employeeNumber }}<small v-if="record.employeeName">{{ record.employeeName }}</small></td><td>{{ record.verificationMethod }}</td><td>{{ record.attendanceStatus || '—' }}</td><td>{{ record.terminalSerialNumber }}</td><td>{{ record.sourceFormat }}</td></tr></tbody>
        </table>
      </section>
      <section class="panel monitor"><h2>Live monitor</h2><p class="note">Protocol bodies and images are never broadcast or stored in this console.</p>
        <p v-if="monitor.length === 0" class="empty">Waiting for gateway activity.</p>
        <ol v-else><li v-for="(event, index) in monitor" :key="`${event.at}-${index}`"><time>{{ formatTime(event.at) }}</time><b>{{ event.kind }}</b><span>{{ event.terminal || 'gateway' }} — {{ event.message }}</span></li></ol>
      </section>
    </template>
  </main>
</template>
