import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { isAuthenticationError } from '../lib/presentation'

const pageSize = 25
const monitorLimit = 100
const monitorRefreshDelay = 350
const monitorReconnectDelay = 3000

export const useGatewayStore = defineStore('gateway', () => {
  const initialized = ref(false)
  const authenticated = ref(false)
  const operatorName = ref('')
  const initializing = ref(false)
  const refreshing = ref(false)
  const eventsLoading = ref(false)
  const bootstrapError = ref('')
  const overview = ref({ deviceEventTotal: 0, terminals: [] })
  const deviceEvents = ref([])
  const deviceEventsTotal = ref(0)
  const deviceEventsOffset = ref(0)
  const monitor = ref([])
  const socketState = ref('disconnected')
  const lastUpdatedAt = ref(null)

  let socket
  let initializationPromise
  let monitorReconnectTimer
  let monitorRefreshTimer

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

  async function loadDeviceEvents(offset = deviceEventsOffset.value) {
    eventsLoading.value = true
    try {
      const page = await request(
        `/api/v1/admin/events?limit=${pageSize}&offset=${offset}`,
      )
      deviceEventsOffset.value = offset
      deviceEvents.value = page.events
      deviceEventsTotal.value = page.total
    } finally {
      eventsLoading.value = false
    }
  }

  function loadDeviceEventPayload(id) {
    return request(`/api/v1/admin/events/${id}/payload`)
  }

  async function refresh() {
    refreshing.value = true
    try {
      await Promise.all([loadOverview(), loadDeviceEvents()])
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
    operatorName.value = username
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

  function scheduleMonitorRefresh() {
    if (monitorRefreshTimer) return
    monitorRefreshTimer = window.setTimeout(() => {
      monitorRefreshTimer = undefined
      refresh().catch((error) => {
        if (isAuthenticationError(error)) clearSession()
      })
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
      if (
        ['device.event_persisted', 'pushsdk.login', 'pushsdk.logout'].includes(
          event.kind,
        )
      )
        scheduleMonitorRefresh()
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

  function clearSession() {
    disconnectMonitor()
    authenticated.value = false
    operatorName.value = ''
    overview.value = { deviceEventTotal: 0, terminals: [] }
    deviceEvents.value = []
    deviceEventsTotal.value = 0
    deviceEventsOffset.value = 0
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
    operatorName,
    initializing,
    refreshing,
    eventsLoading,
    bootstrapError,
    overview,
    deviceEvents,
    deviceEventsTotal,
    deviceEventsOffset,
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
    connectMonitor,
    dispose,
  }
})
