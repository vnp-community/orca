// ─── @orca/trace — Browser Adapter ───────────────────────────────────────────
// Browser-specific initialization for the trace system.
//
// Call `initBrowserTrace(dispatch)` once at app startup (e.g. in main-web-bootstrap.tsx).
// It:
//   1. Overrides the trace-enabled predicate to check localStorage
//   2. Registers a sink that dispatches events to the Zustand trace store
//   3. Connects to the backend SSE stream to receive server-side trace events
//
// Toggle trace in browser DevTools console:
//   localStorage.setItem('ORCA_TRACE', '1'); location.reload()   → enable console output
//   localStorage.removeItem('ORCA_TRACE')                        → disable console output
//
// The TracePanel receives ALL events (start/step/ok AND fail) regardless of the
// ORCA_TRACE flag — fail events always flow through for diagnostics.

import { setTraceEnabledPredicate, registerTraceSink } from './index'
import type { TraceEvent } from './index'

export type TraceDispatch = (event: TraceEvent) => void

// ─── SSE client ───────────────────────────────────────────────────────────────
// Connects to /api/trace-stream to receive backend trace events.
// Auto-reconnects on disconnect (EventSource handles this natively).

const SSE_URL = '/api/trace-stream'
let _sseSource: EventSource | null = null

function startSseClient(dispatch: TraceDispatch): () => void {
  if (!('EventSource' in window)) {
    console.warn('[Trace] EventSource not supported in this browser — backend push disabled')
    return () => undefined
  }

  const source = new EventSource(SSE_URL, {
    // Include cookies for session auth; also sends X-Orca-Trace-Client via URL param
    withCredentials: true,
  })
  _sseSource = source

  source.onmessage = (e) => {
    try {
      const event = JSON.parse(e.data as string) as TraceEvent
      dispatch(event)
    } catch {
      // malformed event — ignore
    }
  }

  source.onerror = () => {
    // EventSource auto-reconnects after error — no action needed
  }

  return () => {
    source.close()
    _sseSource = null
  }
}

// ─── Init ─────────────────────────────────────────────────────────────────────

let _initialized = false
let _cleanup: (() => void) | null = null

/**
 * Initialize browser-side tracing.
 * Safe to call multiple times — only runs once.
 *
 * @param dispatch  Callback that receives each TraceEvent from both:
 *   - Frontend trace calls (via shared/trace sink)
 *   - Backend SSE push (via /api/trace-stream)
 *   Typically: (e) => useAppStore.getState().addTraceEvent(e)
 */
export function initBrowserTrace(dispatch: TraceDispatch): () => void {
  if (_initialized) return () => _cleanup?.()
  _initialized = true

  // 1. Override enabled predicate → check localStorage
  setTraceEnabledPredicate(() => {
    try {
      return localStorage.getItem('ORCA_TRACE') === '1'
    } catch {
      return false
    }
  })

  // 2. Register frontend trace sink → Zustand store
  const unregisterSink = registerTraceSink(dispatch)

  // 3. Connect to backend SSE stream → also dispatches to same store
  const stopSse = startSseClient(dispatch)

  _cleanup = () => {
    unregisterSink()
    stopSse()
    _initialized = false
    _cleanup = null
  }

  return _cleanup
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

export function enableBrowserTrace(): void {
  try { localStorage.setItem('ORCA_TRACE', '1') } catch { /* ignore */ }
}

export function disableBrowserTrace(): void {
  try { localStorage.removeItem('ORCA_TRACE') } catch { /* ignore */ }
}

export function isBrowserTraceEnabled(): boolean {
  try { return localStorage.getItem('ORCA_TRACE') === '1' } catch { return false }
}

/** True if the SSE stream to the backend is currently open */
export function isBackendStreamConnected(): boolean {
  return _sseSource?.readyState === EventSource.OPEN
}
