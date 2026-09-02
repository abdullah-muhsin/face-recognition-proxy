import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { isAuthenticationError } from '../lib/presentation'

const pageSize = 25
const monitorLimit = 100
const monitorRefreshDelay = 350
const monitorReconnectDelay = 3000
const snapshotActivityKinds = new Set([
  'device.event_persisted',
  'pushsdk.auth_info',
  'pushsdk.login',
  'pushsdk.logout',
  'pushsdk.session_resumed',
])
const commandActivityKinds = new Set([
  'admin.isapi_command_queued',
  'pushsdk.command_sent',
  'pushsdk.command_completed',
  'pushsdk.command_expired',
])

export const useGatewayStore = defineStore('gateway', () => {
  const initialized = ref(false)
  const authenticated = ref(false)
  const administratorName = ref('')
  const initializing = ref(false)
  const refreshing = ref(false)
  const eventsLoading = ref(false)
  const activityLoading = ref(false)
  const isapiCommandsLoading = ref(false)
  const bootstrapError = ref('')
  const overview = ref({ deviceEventTotal: 0, terminals: [] })
  const deviceEvents = ref([])
  const deviceEventsTotal = ref(0)
  const deviceEventsOffset = ref(0)
  const deviceEventQuery = ref({ category: 'all', subtype: null, terminal: '' })
  const deviceEventSubtypes = ref([])
  const gatewayActivities = ref([])
  const gatewayActivitiesTotal = ref(0)
  const gatewayActivitiesOffset = ref(0)
  const isapiCommands = ref([])
  const isapiCommandsTotal = ref(0)
  const isapiCommandsOffset = ref(0)
  const monitor = ref([])
  const socketState = ref('disconnected')
  const lastUpdatedAt = ref(null)

  let socket
  let initializationPromise
  let monitorReconnectTimer
  let activityRefreshTimer
  let snapshotRefreshTimer

  const onlineTerminals = computed(
    () =>
      overview.value.terminals.filter(
        (terminal) => terminal.status === 'online',
      ).length,
  )
  const offlineTerminals = computed(
    () =>
      overview.value.terminals.filter(
        (terminal) => terminal.status === 'offline',
      ).length,
  )
  const terminalsWithErrors = computed(
    () =>
      overview.value.terminals.filter((terminal) => terminal.lastError).length,
  )

  async function request(url, options = {}) {
    const response = await fetch(url, {
      credentials: 'same-origin',
      ...options,
    })
    const data = await response.json().catch(() => ({}))
    if (!response.ok) throw new Error(data.error || 'Request failed')
    return data
  }

  async function loadOverview() {
    overview.value = await request('/api/v1/admin/overview')
  }

  async function loadDeviceEvents(
    offset = deviceEventsOffset.value,
    query = deviceEventQuery.value,
  ) {
    eventsLoading.value = true
    try {
      const parameters = new URLSearchParams({
        limit: String(pageSize),
        offset: String(offset),
        category: query.category,
      })
      if (query.subtype !== null)
        parameters.set('subtype', String(query.subtype))
      if (query.terminal !== '') parameters.set('terminal', query.terminal)
      const page = await request(`/api/v1/admin/events?${parameters}`)
      deviceEventsOffset.value = offset
      deviceEventQuery.value = { ...query }
      deviceEvents.value = page.events
      deviceEventsTotal.value = page.total
      deviceEventSubtypes.value = page.subtypes
    } finally {
      eventsLoading.value = false
    }
  }

  function loadDeviceEventPayload(id) {
    return request(`/api/v1/admin/events/${id}/payload`)
  }

  async function loadGatewayActivities(offset = gatewayActivitiesOffset.value) {
    activityLoading.value = true
    try {
      const page = await request(
        `/api/v1/admin/activity?limit=${pageSize}&offset=${offset}`,
      )
      gatewayActivitiesOffset.value = offset
      gatewayActivities.value = page.activities
      gatewayActivitiesTotal.value = page.total
    } finally {
      activityLoading.value = false
    }
  }

  async function loadISAPICommands(
    offset = isapiCommandsOffset.value,
    terminal = '',
  ) {
    isapiCommandsLoading.value = true
    try {
      const parameters = new URLSearchParams({
        limit: String(pageSize),
        offset: String(offset),
      })
      if (terminal !== '') parameters.set('terminal', terminal)
      const page = await request(`/api/v1/admin/isapi-commands?${parameters}`)
      isapiCommandsOffset.value = offset
      isapiCommands.value = page.commands
      isapiCommandsTotal.value = page.total
    } finally {
      isapiCommandsLoading.value = false
    }
  }

  function loadISAPICommandPayload(uuid) {
    return request(`/api/v1/admin/isapi-commands/${uuid}`)
  }

  async function queueISAPICommand(serial, input) {
    const command = await request(
      `/api/v1/admin/terminals/${encodeURIComponent(serial)}/isapi-commands`,
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(input),
      },
    )
    await loadISAPICommands(0)
    return command
  }

  async function refresh() {
    refreshing.value = true
    try {
      await Promise.all([
        loadOverview(),
        loadDeviceEvents(),
        loadGatewayActivities(),
        loadISAPICommands(),
      ])
      lastUpdatedAt.value = new Date()
    } finally {
      refreshing.value = false
    }
  }

  async function initialize() {
    if (initialized.value) return authenticated.value
    if (initializationPromise) return initializationPromise

    initializing.value = true
    initializationPromise = (async () => {
      try {
        await refresh()
        authenticated.value = true
        connectMonitor()
        return true
      } catch (error) {
        if (!isAuthenticationError(error)) bootstrapError.value = error.message
        clearSession()
        return false
      } finally {
        initialized.value = true
        initializing.value = false
        initializationPromise = undefined
      }
    })()
    return initializationPromise
  }

  async function signIn({ username, password }) {
    bootstrapError.value = ''
    await request('/api/v1/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password }),
    })
    authenticated.value = true
    administratorName.value = username
    try {
      await refresh()
      connectMonitor()
    } catch (error) {
      if (isAuthenticationError(error)) clearSession()
      throw error
    }
  }

  async function signOut() {
    try {
      await request('/api/v1/auth/logout', { method: 'POST' })
    } catch (error) {
      if (!isAuthenticationError(error)) throw error
    }
    clearSession()
  }

  function clearSessionOnAuthenticationError(error) {
    if (isAuthenticationError(error)) clearSession()
  }

  function scheduleActivityRefresh() {
    if (activityRefreshTimer) return
    activityRefreshTimer = window.setTimeout(() => {
      activityRefreshTimer = undefined
      loadGatewayActivities().catch(clearSessionOnAuthenticationError)
    }, monitorRefreshDelay)
  }

  function scheduleSnapshotRefresh() {
    if (snapshotRefreshTimer) return
    snapshotRefreshTimer = window.setTimeout(() => {
      snapshotRefreshTimer = undefined
      Promise.all([loadOverview(), loadDeviceEvents()])
        .then(() => {
          lastUpdatedAt.value = new Date()
        })
        .catch(clearSessionOnAuthenticationError)
    }, monitorRefreshDelay)
  }

  function addMonitorEvent(event) {
    const history = event.id
      ? monitor.value.filter((current) => current.id !== event.id)
      : monitor.value
    monitor.value = [event, ...history].slice(0, monitorLimit)
  }

  function connectMonitor() {
    disconnectMonitor()
    monitor.value = []
    if (!authenticated.value) return

    socketState.value = 'connecting'
    const scheme = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const connection = new WebSocket(
      `${scheme}//${window.location.host}/ws/v1/monitor`,
    )
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
      monitorReconnectTimer = window.setTimeout(
        connectMonitor,
        monitorReconnectDelay,
      )
    }
    connection.onmessage = (message) => {
      const event = JSON.parse(message.data)
      addMonitorEvent(event)
      scheduleActivityRefresh()
      if (snapshotActivityKinds.has(event.kind)) scheduleSnapshotRefresh()
      if (commandActivityKinds.has(event.kind))
        loadISAPICommands().catch(clearSessionOnAuthenticationError)
    }
  }

  function disconnectMonitor() {
    if (monitorReconnectTimer) {
      window.clearTimeout(monitorReconnectTimer)
      monitorReconnectTimer = undefined
    }
    if (activityRefreshTimer) {
      window.clearTimeout(activityRefreshTimer)
      activityRefreshTimer = undefined
    }
    if (snapshotRefreshTimer) {
      window.clearTimeout(snapshotRefreshTimer)
      snapshotRefreshTimer = undefined
    }
    if (socket) {
      const connection = socket
      socket = undefined
      connection.close()
    }
    socketState.value = 'disconnected'
  }

  function clearSession() {
    disconnectMonitor()
    authenticated.value = false
    administratorName.value = ''
    overview.value = { deviceEventTotal: 0, terminals: [] }
    deviceEvents.value = []
    deviceEventsTotal.value = 0
    deviceEventsOffset.value = 0
    deviceEventQuery.value = { category: 'all', subtype: null, terminal: '' }
    deviceEventSubtypes.value = []
    gatewayActivities.value = []
    gatewayActivitiesTotal.value = 0
    gatewayActivitiesOffset.value = 0
    isapiCommands.value = []
    isapiCommandsTotal.value = 0
    isapiCommandsOffset.value = 0
    monitor.value = []
    lastUpdatedAt.value = null
  }

  function dispose() {
    disconnectMonitor()
  }

  return {
    pageSize,
    initialized,
    authenticated,
    administratorName,
    initializing,
    refreshing,
    eventsLoading,
    activityLoading,
    isapiCommandsLoading,
    bootstrapError,
    overview,
    deviceEvents,
    deviceEventsTotal,
    deviceEventsOffset,
    deviceEventQuery,
    deviceEventSubtypes,
    gatewayActivities,
    gatewayActivitiesTotal,
    gatewayActivitiesOffset,
    isapiCommands,
    isapiCommandsTotal,
    isapiCommandsOffset,
    monitor,
    socketState,
    lastUpdatedAt,
    onlineTerminals,
    offlineTerminals,
    terminalsWithErrors,
    initialize,
    signIn,
    signOut,
    refresh,
    loadDeviceEvents,
    loadDeviceEventPayload,
    loadGatewayActivities,
    loadISAPICommands,
    loadISAPICommandPayload,
    queueISAPICommand,
    connectMonitor,
    dispose,
  }
})
