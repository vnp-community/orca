import { useEffect } from 'react'
import { useAppStore } from '../store'
import {
  callRuntimeRpc,
  getActiveRuntimeTarget,
  subscribeRuntimeStreamChannel
} from '../runtime/runtime-rpc-client'
import type { DevServer } from '../../../shared/dev-server-types'

// `devServer.subscribeStatus` pushes `devServer.statusChanged {devServerId, connected, activeExecutions, busy}`:
// a snapshot, ephemeral, no history. So: subscribe, read `devServer.list` once for the current state (events seen
// meanwhile win), then let events overwrite. Without the channel, busy is cleared (unknown beats stale).

const RECONNECT_BASE_MS = 1_000
const RECONNECT_MAX_MS = 15_000
const MAX_CONSECUTIVE_FAILURES = 8
// Settings (and so the runtime target) load a moment after the app starts; desktop-local never gets a target.
const LOCAL_TARGET_RECHECKS = 10
const LOCAL_TARGET_RECHECK_MS = 2_000

type StatusEvent = {
  devServerId?: unknown
  connected?: unknown
  activeExecutions?: unknown
  busy?: unknown
}

function asEvent(raw: unknown): StatusEvent | null {
  if (typeof raw !== 'object' || raw === null) {
    return null
  }
  const wrapped = Array.isArray((raw as { args?: unknown }).args)
    ? (raw as { args: unknown[] }).args[0]
    : undefined
  return (typeof wrapped === 'object' && wrapped !== null ? wrapped : raw) as StatusEvent
}

function isUnsupportedChannel(error: unknown): boolean {
  const code = (error as { code?: unknown } | null)?.code
  return code === 'method_not_found' || code === 'unsupported' || code === 'forbidden'
}

function applyBusy(id: string, connected: boolean, activeExecutions: number, busy: boolean): void {
  useAppStore
    .getState()
    .updateDevServerStatus(id, connected ? 'connected' : 'disconnected', { activeExecutions, busy })
}

function clearBusy(): void {
  const { devServers, updateDevServerStatus } = useAppStore.getState()
  for (const ds of devServers) {
    updateDevServerStatus(ds.id, ds.status, { activeExecutions: undefined, busy: undefined })
  }
}

/** Starts the stream for the whole app session; call once at the root (useDevServersSync). */
export function startDevServerStatusStream(): () => void {
  let cancelled = false
  let attempt = 0
  let timer: ReturnType<typeof setTimeout> | null = null
  let unsubscribe: (() => void) | null = null
  let seen = new Set<string>()
  let localChecks = 0

  const scheduleReconnect = (): void => {
    if (cancelled) {
      return
    }
    unsubscribe = null
    attempt += 1
    if (attempt > MAX_CONSECUTIVE_FAILURES) {
      clearBusy()
      return
    }
    timer = setTimeout(
      () => void connect(),
      Math.min(RECONNECT_BASE_MS * 2 ** (attempt - 1), RECONNECT_MAX_MS)
    )
  }

  const onEvent = (raw: unknown): void => {
    const ev = asEvent(raw)
    if (!ev || typeof ev.devServerId !== 'string') {
      return
    }
    seen.add(ev.devServerId)
    const executions = typeof ev.activeExecutions === 'number' ? ev.activeExecutions : 0
    applyBusy(ev.devServerId, ev.connected === true, executions, ev.busy === true)
  }

  const connect = async (): Promise<void> => {
    timer = null
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      if (target.kind === 'local') {
        localChecks += 1
        if (localChecks < LOCAL_TARGET_RECHECKS) {
          timer = setTimeout(() => void connect(), LOCAL_TARGET_RECHECK_MS)
        }
        return
      }
      seen = new Set()
      const sub = await subscribeRuntimeStreamChannel<unknown, unknown>(
        target,
        'devServer.subscribeStatus',
        {},
        onEvent,
        { onClose: scheduleReconnect }
      )
      if (cancelled) {
        sub.unsubscribe()
        return
      }
      unsubscribe = sub.unsubscribe
      attempt = 0
      const servers = await callRuntimeRpc<DevServer[]>(target, 'devServer.list', null).catch(
        () => [] as DevServer[]
      )
      if (cancelled) {
        return
      }
      for (const s of Array.isArray(servers) ? servers : []) {
        if (!seen.has(s.id) && typeof s.busy === 'boolean') {
          applyBusy(s.id, s.status === 'connected', s.activeExecutions ?? 0, s.busy)
        }
      }
    } catch (error) {
      if (cancelled) {
        return
      }
      if (isUnsupportedChannel(error) || !window.api) {
        clearBusy()
        return
      }
      scheduleReconnect()
    }
  }

  void connect()
  return () => {
    cancelled = true
    if (timer) {
      clearTimeout(timer)
    }
    unsubscribe?.()
  }
}

export function useDevServerStatusStream(): void {
  useEffect(() => startDevServerStatusStream(), [])
}
