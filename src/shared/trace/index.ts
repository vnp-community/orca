// ─── @orca/trace — Core ───────────────────────────────────────────────────────
// Isomorphic structured trace/observability for Orca.
// Works in Node.js (server/main/relay) AND browser (React frontend).
//
// Environment control:
//   Node.js:  ORCA_TRACE=1 env var
//   Browser:  localStorage.setItem('ORCA_TRACE', '1')
//
// Usage:
//   import { createTracer } from '../../shared/trace'
//   const tracer = createTracer('devServer:browseDir')
//   const span = tracer.start({ devServerId: 'dev-01', path: '~' })
//   span.step('relay', { session: 'connected' })
//   span.ok({ entries: 12 })      // or span.fail(err)

// ─── Public types ─────────────────────────────────────────────────────────────

export type TraceLevel = 'start' | 'step' | 'ok' | 'fail'

export interface TraceEvent {
  /** Span identifier (short random string) */
  id: string
  /** Tracer flow name e.g. 'devServer:browseDir' */
  flow: string
  /** Event type within the span */
  level: TraceLevel
  /** Step label for 'step' events, empty otherwise */
  label?: string
  /** Arbitrary key/value context fields */
  fields: TraceFields
  /** Unix timestamp (ms) when this event was emitted */
  ts: number
  /** Wall-clock ms since span started (populated for step/ok/fail) */
  elapsedMs?: number
}

export type TraceFields = Record<string, string | number | boolean | undefined>

export interface TraceSpan {
  readonly id: string
  step(label: string, fields?: TraceFields): void
  ok(fields?: TraceFields): void
  fail(err: unknown, fields?: TraceFields): void
}

export interface Tracer {
  start(fields?: TraceFields): TraceSpan
}

// ─── Sink registry (platform-agnostic) ───────────────────────────────────────
//
// Platforms (Node.js / browser) register sinks at startup.
// Each sink receives every TraceEvent and can do whatever it wants:
// write to console, dispatch to Zustand store, ship to a backend, etc.

type TraceSink = (event: TraceEvent) => void

const sinks: TraceSink[] = []

/** Register a sink that will receive all future trace events. */
export function registerTraceSink(sink: TraceSink): () => void {
  sinks.push(sink)
  return () => {
    const i = sinks.indexOf(sink)
    if (i >= 0) sinks.splice(i, 1)
  }
}

// ─── Enable flag ──────────────────────────────────────────────────────────────
//
// The isTraceEnabled() function is intentionally overridable so that the
// browser adapter can swap in a localStorage-based check.

let _isEnabled: () => boolean = () => {
  // Default: Node.js env var
  if (typeof process !== 'undefined' && process.env) {
    const v = process.env['ORCA_TRACE'] ?? ''
    return v === '1' || v === 'true' || v === '*'
  }
  return false
}

/** Override the trace-enabled predicate (used by browser adapter). */
export function setTraceEnabledPredicate(fn: () => boolean): void {
  _isEnabled = fn
}

export function isTraceEnabled(): boolean {
  return _isEnabled()
}

// ─── Internal helpers ─────────────────────────────────────────────────────────

function shortId(): string {
  return Math.random().toString(36).slice(2, 8)
}

function serializeFields(fields: TraceFields): string {
  return Object.entries(fields)
    .filter(([, v]) => v !== undefined)
    .map(([k, v]) => {
      const s = String(v)
      return s.includes(' ') ? `${k}='${s}'` : `${k}=${s}`
    })
    .join(' ')
}

function formatError(err: unknown): string {
  if (err instanceof Error) return err.message
  return String(err)
}

function emit(event: TraceEvent): void {
  // Console output
  const extra = serializeFields(event.fields)
  const extraStr = extra ? ' ' + extra : ''

  if (event.level === 'start') {
    if (isTraceEnabled()) {
      console.log(`[TRACE] ${event.flow} id=${event.id}${extraStr}`)
    }
  } else if (event.level === 'step') {
    if (isTraceEnabled()) {
      console.log(`[TRACE] ${event.flow} id=${event.id} step=${event.label ?? ''}${extraStr}`)
    }
  } else if (event.level === 'ok') {
    if (isTraceEnabled()) {
      const elapsed = event.elapsedMs !== undefined ? ` durationMs=${event.elapsedMs}` : ''
      console.log(`[TRACE] ${event.flow} id=${event.id} OK${extraStr}${elapsed}`)
    }
  } else {
    // fail — always log regardless of ORCA_TRACE flag
    const elapsed = event.elapsedMs !== undefined ? ` durationMs=${event.elapsedMs}` : ''
    console.error(`[TRACE] ${event.flow} id=${event.id} FAIL${extraStr}${elapsed}`)
  }

  // Dispatch to registered sinks (browser store, remote shipper, etc.)
  for (const sink of sinks) {
    try { sink(event) } catch { /* sink errors must not break callers */ }
  }
}

// ─── Public API ───────────────────────────────────────────────────────────────

/**
 * Create a named tracer for a specific flow.
 *
 * @param flow  Colon-separated name: 'subsystem:operation' e.g. 'devServer:browseDir'
 */
export function createTracer(flow: string): Tracer {
  return {
    start(fields: TraceFields = {}): TraceSpan {
      const id = shortId()
      const startMs = Date.now()

      emit({ id, flow, level: 'start', fields, ts: startMs })

      return {
        id,

        step(label: string, stepFields: TraceFields = {}): void {
          emit({
            id, flow, level: 'step', label, fields: stepFields,
            ts: Date.now(), elapsedMs: Date.now() - startMs
          })
        },

        ok(okFields: TraceFields = {}): void {
          emit({
            id, flow, level: 'ok', fields: okFields,
            ts: Date.now(), elapsedMs: Date.now() - startMs
          })
        },

        fail(err: unknown, failFields: TraceFields = {}): void {
          const errMsg = formatError(err)
          emit({
            id, flow, level: 'fail',
            fields: { err: errMsg, ...failFields },
            ts: Date.now(), elapsedMs: Date.now() - startMs
          })
        }
      }
    }
  }
}
