/**
 * code-intel-stream-reconnect.ts — FE-CV-TASK-050-11
 *
 * Manages the code-intel push stream with exponential backoff reconnection.
 * Follows the pattern of startMcpEvents in mcp-slice.ts.
 *
 * Rules:
 * - Ref-counted: only one stream per environment
 * - Backoff: 1s → 30s, resets after 30s of healthy connection
 * - resync events increment codeIntelResyncCounter
 * - onUnsupported/RATE_LIMITED → polling mode
 * - Clears all timers on reset/dispose
 *
 * @module store/slices/code-intel-stream-reconnect
 */

import { publishCodeIntelEvent } from '../../lib/code-intel-event-bus'
import type { CodeIntelPushEvent } from '../../lib/code-intel-event-bus'

// ---------------------------------------------------------------------------
// Backoff constants
// ---------------------------------------------------------------------------

const BACKOFF_INITIAL_MS = 1_000
const BACKOFF_MAX_MS = 30_000
const HEALTHY_DURATION_MS = 30_000

// ---------------------------------------------------------------------------
// Stream state
// ---------------------------------------------------------------------------

type StreamState = {
  environmentId: string
  refCount: number
  unsubscribe: (() => void) | null
  backoffMs: number
  reconnectTimer: ReturnType<typeof setTimeout> | null
  healthyTimer: ReturnType<typeof setTimeout> | null
  startedAt: number
}

const streams = new Map<string, StreamState>()

// ---------------------------------------------------------------------------
// Callbacks (injected by store to avoid circular deps)
// ---------------------------------------------------------------------------

export type StreamCallbacks = {
  triggerResync: () => void
  setPollingMode: (environmentId: string) => void
  getSubscribe: (environmentId: string, method: string, params: object, callbacks: {
    onResponse: (raw: unknown) => void
    onClose: (code?: number) => void
  }) => (() => void) | null
}

let _callbacks: StreamCallbacks | null = null

export function initCodeIntelStreamCallbacks(callbacks: StreamCallbacks): void {
  _callbacks = callbacks
}

// ---------------------------------------------------------------------------
// Frame parser
// ---------------------------------------------------------------------------

function parseFrame(raw: unknown): CodeIntelPushEvent | null {
  if (typeof raw !== 'object' || raw === null) return null
  const frame = raw as Record<string, unknown>
  const event = frame.event

  if (!event || typeof event !== 'string') return null

  const worktreeId = typeof frame.worktreeId === 'string' ? frame.worktreeId : ''

  switch (event) {
    case 'changed':
      return {
        event: 'changed',
        worktreeId,
        reason: typeof frame.reason === 'string' ? frame.reason : undefined,
        resync: frame.resync === true
      }
    case 'reindexProgress':
      return {
        event: 'reindexProgress',
        worktreeId,
        percent: typeof frame.percent === 'number' ? frame.percent : null,
        overall: typeof frame.overall === 'string' ? frame.overall : 'UNKNOWN'
      }
    case 'qualityProgress':
      return {
        event: 'qualityProgress',
        worktreeId,
        runId: typeof frame.runId === 'string' ? frame.runId : '',
        percent: typeof frame.percent === 'number' ? frame.percent : null
      }
    case 'qualityFinished':
      return {
        event: 'qualityFinished',
        worktreeId,
        runId: typeof frame.runId === 'string' ? frame.runId : '',
        success: frame.success === true,
        error: typeof frame.error === 'string' ? frame.error : null
      }
    case 'gateChanged':
      return { event: 'gateChanged', worktreeId, gate: frame.gate }
    default:
      // Unknown event — silently drop (U4)
      return null
  }
}

// ---------------------------------------------------------------------------
// Stream lifecycle
// ---------------------------------------------------------------------------

function connect(state: StreamState): void {
  if (!_callbacks) return

  const healthyStart = Date.now()

  const unsub = _callbacks.getSubscribe(
    state.environmentId,
    'codeIntel.subscribe',
    {},
    {
      onResponse: (raw) => {
        // Ack frame: null → ignore
        if (raw === null) return

        const parsed = parseFrame(raw)
        if (!parsed) return

        // Publish to bus first
        publishCodeIntelEvent(parsed)

        // resync → trigger resync
        if (parsed.event === 'changed' && parsed.resync) {
          _callbacks?.triggerResync()
        }

        // Reset healthy timer on any successful frame
        if (state.healthyTimer) clearTimeout(state.healthyTimer)
        state.healthyTimer = setTimeout(() => {
          state.backoffMs = BACKOFF_INITIAL_MS
        }, HEALTHY_DURATION_MS)
      },

      onClose: (code) => {
        if (state.healthyTimer) clearTimeout(state.healthyTimer)

        const isUnsupported = code === 1008 // Policy violation / unsupported
        const isRateLimited = code === 1013 // Try again later

        if (isUnsupported || isRateLimited) {
          _callbacks?.setPollingMode(state.environmentId)
          return
        }

        // Reconnect with backoff
        if (state.refCount > 0) {
          state.reconnectTimer = setTimeout(() => {
            state.backoffMs = Math.min(state.backoffMs * 2, BACKOFF_MAX_MS)
            connect(state)
          }, state.backoffMs)
        }
      }
    }
  )

  state.unsubscribe = unsub
}

/**
 * Retain the push stream for an environment.
 * Returns a release function.
 */
export function retainCodeIntelStream(environmentId: string): () => void {
  let state = streams.get(environmentId)
  if (!state) {
    state = {
      environmentId,
      refCount: 0,
      unsubscribe: null,
      backoffMs: BACKOFF_INITIAL_MS,
      reconnectTimer: null,
      healthyTimer: null,
      startedAt: Date.now()
    }
    streams.set(environmentId, state)
    connect(state)
  }

  state.refCount++

  return () => {
    if (!state) return
    state.refCount--
    if (state.refCount <= 0) {
      // Clean up
      state.unsubscribe?.()
      if (state.reconnectTimer) clearTimeout(state.reconnectTimer)
      if (state.healthyTimer) clearTimeout(state.healthyTimer)
      streams.delete(environmentId)
    }
  }
}

/** For testing: reset all streams. */
export function resetCodeIntelStreams(): void {
  for (const state of streams.values()) {
    state.unsubscribe?.()
    if (state.reconnectTimer) clearTimeout(state.reconnectTimer)
    if (state.healthyTimer) clearTimeout(state.healthyTimer)
  }
  streams.clear()
}
