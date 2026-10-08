/**
 * code-intel-stream-reconnect.ts — FE-CV-TASK-050-11
 *
 * Ref-counted push stream (`codeIntel.subscribe`, one per environment) with reconnect backoff,
 * following startMcpEvents in mcp-slice.ts.
 *
 * - `changed` invalidates the worktree cache (stale chip); `changed{resync}` also bumps the
 *   resync counter so open views reload. A gateway-initiated close reconnects and resyncs.
 * - Backoff 1 s -> 30 s, reset after 30 s of healthy connection.
 * - Unsupported transport (local target, no stream) -> polling mode; timers are always cleared.
 *
 * @module store/slices/code-intel-stream-reconnect
 */

import { publishCodeIntelEvent } from '../../lib/code-intel-event-bus'
import type { CodeIntelPushEvent as BusEvent } from '../../lib/code-intel-event-bus'
import type { CodeIntelPushEvent } from '../../../../shared/code-intel-types'
import type { CodeIntelSubscribeCallbacks } from '../../../../shared/code-intel-bridge'
import type { CodeIntelEventsState } from './code-intel'

export const BACKOFF_INITIAL_MS = 1_000
export const BACKOFF_MAX_MS = 30_000
export const HEALTHY_DURATION_MS = 30_000

export type CodeIntelStreamDeps = {
  subscribe: (environmentId: string | null, callbacks: CodeIntelSubscribeCallbacks) => () => void
  invalidateWorktree: (worktreeId: string) => void
  triggerResync: () => void
  setEventsState: (state: CodeIntelEventsState) => void
}

type StreamState = {
  environmentId: string | null
  refCount: number
  unsubscribe: (() => void) | null
  backoffMs: number
  reconnectTimer: ReturnType<typeof setTimeout> | null
  healthyTimer: ReturnType<typeof setTimeout> | null
  /** Set after an unexpected close: the next successful open must reload views. */
  resyncOnReconnect: boolean
  disposed: boolean
}

const streams = new Map<string, StreamState>()
const LOCAL_KEY = '\0local'

/** The internal bus keeps its own (looser) event shape for older consumers. */
function toBusEvent(event: CodeIntelPushEvent): BusEvent {
  switch (event.event) {
    case 'changed':
      return { event: 'changed', worktreeId: event.worktreeId, reason: event.reason, resync: event.resync }
    case 'reindexProgress':
      return {
        event: 'reindexProgress',
        worktreeId: event.worktreeId,
        percent: event.percent,
        overall: event.running ? 'BUILDING' : 'UNKNOWN'
      }
    case 'qualityProgress':
      return {
        event: 'qualityProgress',
        worktreeId: event.worktreeId,
        runId: event.runId,
        percent: event.percent,
        stage: event.stage,
        stepIndex: event.stepIndex,
        stepCount: event.stepCount,
        message: event.message
      }
    case 'qualityFinished':
      return {
        event: 'qualityFinished',
        worktreeId: event.worktreeId,
        runId: event.runId,
        success: event.success,
        error: event.error,
        status: event.status,
        headCommit: event.headCommit
      }
    case 'gateChanged':
      return {
        event: 'gateChanged',
        worktreeId: event.worktreeId,
        gate: event.gate,
        headCommit: event.headCommit,
        profile: event.profile
      }
  }
}

function clearTimers(state: StreamState): void {
  if (state.reconnectTimer) {clearTimeout(state.reconnectTimer)}
  if (state.healthyTimer) {clearTimeout(state.healthyTimer)}
  state.reconnectTimer = null
  state.healthyTimer = null
}

function connect(state: StreamState, deps: CodeIntelStreamDeps): void {
  if (state.disposed) {return}

  if (state.resyncOnReconnect) {
    state.resyncOnReconnect = false
    deps.triggerResync()
  }
  deps.setEventsState('streaming')
  state.healthyTimer = setTimeout(() => {
    state.backoffMs = BACKOFF_INITIAL_MS
  }, HEALTHY_DURATION_MS)

  state.unsubscribe = deps.subscribe(state.environmentId, {
    onEvent: (event) => {
      if (state.disposed) {return}
      if (event.event === 'changed') {
        // Why: a plain `changed` only marks cached data stale; the UI shows a chip instead of reloading.
        deps.invalidateWorktree(event.worktreeId)
        if (event.resync) {deps.triggerResync()}
      }
      publishCodeIntelEvent(toBusEvent(event))
    },
    onClose: () => {
      if (state.disposed) {return}
      if (state.healthyTimer) {clearTimeout(state.healthyTimer)}
      state.healthyTimer = null
      state.unsubscribe = null
      if (state.refCount <= 0) {return}
      state.resyncOnReconnect = true
      const delay = state.backoffMs
      state.backoffMs = Math.min(state.backoffMs * 2, BACKOFF_MAX_MS)
      state.reconnectTimer = setTimeout(() => {
        state.reconnectTimer = null
        connect(state, deps)
      }, delay)
    },
    onUnsupported: () => {
      if (state.disposed) {return}
      clearTimers(state)
      state.unsubscribe = null
      deps.setEventsState('polling')
    }
  })
}

/**
 * Retain the push stream for an environment (null = local target, which falls back to polling).
 * Returns an idempotent release function.
 */
export function retainCodeIntelStream(
  environmentId: string | null,
  deps: CodeIntelStreamDeps
): () => void {
  const key = environmentId ?? LOCAL_KEY
  let state = streams.get(key)
  if (!state) {
    state = {
      environmentId,
      refCount: 0,
      unsubscribe: null,
      backoffMs: BACKOFF_INITIAL_MS,
      reconnectTimer: null,
      healthyTimer: null,
      resyncOnReconnect: false,
      disposed: false
    }
    streams.set(key, state)
    state.refCount++
    connect(state, deps)
  } else {
    state.refCount++
  }

  const owned = state
  let released = false
  return () => {
    if (released) {return}
    released = true
    owned.refCount--
    if (owned.refCount <= 0) {
      owned.disposed = true
      owned.unsubscribe?.()
      owned.unsubscribe = null
      clearTimers(owned)
      if (streams.get(key) === owned) {streams.delete(key)}
      deps.setEventsState('idle')
    }
  }
}

/** For testing: drop every stream and timer. */
export function resetCodeIntelStreams(): void {
  for (const state of streams.values()) {
    state.disposed = true
    state.unsubscribe?.()
    clearTimers(state)
  }
  streams.clear()
}
