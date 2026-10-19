import { useAppStore } from '../store'
import {
  getActiveRuntimeTarget,
  subscribeRuntimeStreamChannel
} from '../runtime/runtime-rpc-client'

// One `task.activity.subscribe` stream per task, shared by every view of that task and kept for a
// short while after the last view closes. Why: the gateway only reaps a stream when the whole socket
// closes (the web client's unsubscribe is local), so subscribing on every sheet open would pile up
// server-side NATS subscriptions. Re-opening a task also shows its last output right away.

export type AgentOutputStatus = 'connecting' | 'live' | 'reconnecting' | 'unavailable'

export type AgentOutputSnapshot = {
  /** Cumulative output of the latest run: each frame carries everything so far. */
  stdout: string
  stderr: string
  runId: string | null
  receivedAt: number | null
  status: AgentOutputStatus
}

const RECONNECT_BASE_MS = 1_000
const RECONNECT_MAX_MS = 15_000
// Consecutive failed attempts before giving up; an acknowledged connection resets the count.
const MAX_CONSECUTIVE_FAILURES = 8
// Tasks without a viewer whose stream stays open (bounds both server subscriptions and memory).
const MAX_IDLE_ENTRIES = 6
// A frame is the whole run so far; only the tail is ever shown, so only the tail is kept.
const MAX_KEPT_CHARS = 512 * 1024

type Entry = {
  taskId: string
  snapshot: AgentOutputSnapshot
  listeners: Set<() => void>
  unsubscribe: (() => void) | null
  retryTimer: ReturnType<typeof setTimeout> | null
  attempt: number
  disposed: boolean
  idleSince: number | null
}

const entries = new Map<string, Entry>()

type ActivityFrame = {
  eventType?: unknown
  payload?: { stdout?: unknown; stderr?: unknown; request_id?: unknown } | null
}

function asFrame(event: unknown): ActivityFrame | null {
  if (typeof event !== 'object' || event === null) {
    return null
  }
  // Some dialects wrap the pushed value as `args: [frame]`.
  const wrapped = Array.isArray((event as { args?: unknown }).args)
    ? (event as { args: unknown[] }).args[0]
    : undefined
  return (typeof wrapped === 'object' && wrapped !== null ? wrapped : event) as ActivityFrame
}

function keepTail(text: string): string {
  return text.length > MAX_KEPT_CHARS ? text.slice(text.length - MAX_KEPT_CHARS) : text
}

// Gateways without the channel answer method_not_found; retrying would never help.
function isUnsupportedChannel(error: unknown): boolean {
  const code = (error as { code?: unknown } | null)?.code
  return code === 'method_not_found' || code === 'unsupported' || code === 'forbidden'
}

function update(entry: Entry, patch: Partial<AgentOutputSnapshot>): void {
  entry.snapshot = { ...entry.snapshot, ...patch }
  entry.listeners.forEach((l) => l())
}

// A stream that cannot work is not kept for a future view; viewers see the status and the next open retries.
function markUnavailable(entry: Entry): void {
  update(entry, { status: 'unavailable' })
  if (entry.listeners.size === 0) {
    dispose(entry)
  }
}

function onEvent(entry: Entry, event: unknown): void {
  const frame = asFrame(event)
  if (!frame || frame.eventType !== 'agent_output_partial' || !frame.payload) {
    return
  }
  const { stdout, stderr, request_id: requestId } = frame.payload
  update(entry, {
    stdout: typeof stdout === 'string' ? keepTail(stdout) : '',
    stderr: typeof stderr === 'string' ? keepTail(stderr) : '',
    runId: typeof requestId === 'string' ? requestId : null,
    receivedAt: Date.now()
  })
}

function scheduleReconnect(entry: Entry): void {
  if (entry.disposed) {
    return
  }
  entry.unsubscribe = null
  // Closed while nobody was looking: forget it, the next open starts a fresh stream.
  if (entry.listeners.size === 0) {
    dispose(entry)
    return
  }
  entry.attempt += 1
  if (entry.attempt > MAX_CONSECUTIVE_FAILURES) {
    markUnavailable(entry)
    return
  }
  update(entry, { status: 'reconnecting' })
  const delay = Math.min(RECONNECT_BASE_MS * 2 ** (entry.attempt - 1), RECONNECT_MAX_MS)
  entry.retryTimer = setTimeout(() => void connect(entry), delay)
}

async function connect(entry: Entry): Promise<void> {
  entry.retryTimer = null
  try {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    // Desktop-local has no push client for backend-go channels (see subscribeRuntimeStreamChannel).
    if (target.kind === 'local') {
      markUnavailable(entry)
      return
    }
    const sub = await subscribeRuntimeStreamChannel<unknown, unknown>(
      target,
      'task.activity.subscribe',
      { taskId: entry.taskId },
      (event) => onEvent(entry, event),
      { onClose: () => scheduleReconnect(entry) }
    )
    if (entry.disposed) {
      sub.unsubscribe()
      return
    }
    entry.unsubscribe = sub.unsubscribe
    entry.attempt = 0
    update(entry, { status: 'live' })
  } catch (error) {
    if (entry.disposed) {
      return
    }
    if (isUnsupportedChannel(error) || !window.api) {
      markUnavailable(entry)
      return
    }
    scheduleReconnect(entry)
  }
}

function dispose(entry: Entry): void {
  entry.disposed = true
  if (entry.retryTimer) {
    clearTimeout(entry.retryTimer)
  }
  entry.unsubscribe?.()
  entries.delete(entry.taskId)
}

function trimIdleEntries(): void {
  const idle = [...entries.values()].filter((e) => e.idleSince !== null)
  idle
    .sort((a, b) => (a.idleSince ?? 0) - (b.idleSince ?? 0))
    .slice(0, Math.max(0, idle.length - MAX_IDLE_ENTRIES))
    .forEach(dispose)
}

export type AgentOutputHandle = { snapshot: () => AgentOutputSnapshot; release: () => void }

/** Starts (or joins) the stream of a task; call release() when the view goes away. */
export function watchTaskAgentOutput(taskId: string, onChange: () => void): AgentOutputHandle {
  let entry = entries.get(taskId)
  if (!entry) {
    entry = {
      taskId,
      snapshot: { stdout: '', stderr: '', runId: null, receivedAt: null, status: 'connecting' },
      listeners: new Set(),
      unsubscribe: null,
      retryTimer: null,
      attempt: 0,
      disposed: false,
      idleSince: null
    }
    entries.set(taskId, entry)
    void connect(entry)
  }
  const joined = entry
  joined.listeners.add(onChange)
  joined.idleSince = null
  let released = false
  return {
    snapshot: () => joined.snapshot,
    release: () => {
      if (released) {
        return
      }
      released = true
      joined.listeners.delete(onChange)
      if (joined.listeners.size > 0 || joined.disposed) {
        return
      }
      // Nobody is looking: a failing stream is not worth keeping; a healthy or still-connecting one waits for a
      // re-open (React strict mode remounts right away, and a second subscribe would leak server-side).
      if (joined.snapshot.status === 'live' || joined.snapshot.status === 'connecting') {
        joined.idleSince = Date.now()
        trimIdleEntries()
      } else {
        dispose(joined)
      }
    }
  }
}

export function resetTaskAgentOutputStoreForTests(): void {
  ;[...entries.values()].forEach(dispose)
}
