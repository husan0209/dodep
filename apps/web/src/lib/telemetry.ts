type TelemetryPayload = Record<string, string | number | boolean | null | undefined>

export type TelemetryEvent =
  | 'page_view'
  | 'sportsbook_filter_changed'
  | 'odds_selected'
  | 'betslip_opened'
  | 'bet_placed'
  | 'casino_filter_changed'
  | 'casino_search_used'
  | 'casino_game_play_clicked'
  | 'deposit_started'
  | 'deposit_submitted'
  | 'withdraw_submitted'
  | 'auth_login_submitted'
  | 'auth_register_submitted'

interface StoredTelemetryEvent {
  event: TelemetryEvent
  timestamp: string
  payload: TelemetryPayload
}

const MAX_STORED_EVENTS = 200
const DEDUPE_WINDOW_MS = 1200
let lastSignature = ''
let lastTrackedAt = 0

export function trackEvent(event: TelemetryEvent, payload: TelemetryPayload = {}) {
  if (typeof window === 'undefined') {
    return
  }

  const normalizedPayload = Object.fromEntries(
    Object.entries(payload).map(([key, value]) => [key, value ?? null])
  )

  const entry: StoredTelemetryEvent = {
    event,
    timestamp: new Date().toISOString(),
    payload: normalizedPayload,
  }

  // In React StrictMode (dev), effects can run twice.
  // Drop immediate duplicates to keep baseline metrics clean.
  const signature = `${event}:${JSON.stringify(normalizedPayload)}`
  const now = Date.now()
  if (signature === lastSignature && now - lastTrackedAt < DEDUPE_WINDOW_MS) {
    return
  }
  lastSignature = signature
  lastTrackedAt = now

  try {
    const raw = window.localStorage.getItem('dod.telemetry.events')
    const parsed = raw ? (JSON.parse(raw) as StoredTelemetryEvent[]) : []
    const next = [...parsed.slice(-MAX_STORED_EVENTS + 1), entry]
    window.localStorage.setItem('dod.telemetry.events', JSON.stringify(next))
  } catch {
    // Ignore storage errors in privacy mode / full quota.
  }

  if (process.env.NODE_ENV !== 'production') {
    // eslint-disable-next-line no-console
    console.info('[telemetry]', event, normalizedPayload)
  }
}

