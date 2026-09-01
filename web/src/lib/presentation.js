export function statusSeverity(status) {
  return (
    {
      online: 'success',
      authenticating: 'warn',
      error: 'danger',
      offline: 'secondary',
    }[status] || 'info'
  )
}

export function statusLabel(status) {
  return status ? status.replaceAll('-', ' ') : 'Unknown'
}

export function eventSeverity(kind) {
  if (kind.includes('error') || kind.includes('rejected')) return 'danger'
  if (kind.includes('event_persisted')) return 'success'
  if (kind.includes('login') || kind.includes('connected')) return 'info'
  return 'secondary'
}

export function formatTime(value) {
  if (!value) return '—'
  const time = new Date(value)
  if (Number.isNaN(time.getTime())) return '—'
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'medium',
  }).format(time)
}

export function formatShortTime(value) {
  if (!value) return 'Never'
  const time = new Date(value)
  if (Number.isNaN(time.getTime())) return '—'
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  }).format(time)
}

export function matchesDeviceEvent(event, query) {
  const term = query.trim().toLocaleLowerCase()
  if (!term) return true
  return [event.vendorEventId, event.terminalSerialNumber, event.dataFormat]
    .filter(Boolean)
    .join(' ')
    .toLocaleLowerCase()
    .includes(term)
}

export function matchesTerminal(terminal, query) {
  const term = query.trim().toLocaleLowerCase()
  if (!term) return true
  return [
    terminal.serialNumber,
    terminal.pushSdkSerial,
    terminal.status,
    terminal.lastError,
  ]
    .filter(Boolean)
    .join(' ')
    .toLocaleLowerCase()
    .includes(term)
}

export function isAuthenticationError(error) {
  return error instanceof Error && error.message === 'authentication required'
}
