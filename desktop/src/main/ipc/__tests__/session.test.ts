/**
 * Tests for `session:read-terminal-scrollback-sync` restore-path tracing
 * (TASK-BE-003.3/003.4).
 *
 * Covers `registerSessionHandlers()` in `src/main/ipc/session.ts` —
 * instrumented with `Tracers.terminalReattach` (flow `terminal:reattach`,
 * reused — no separate `terminal:reconnect` tracer was minted). This is an
 * in-process Electron IPC round-trip (`ipcMain.on`, synchronous
 * `event.returnValue`), so no `traceId` wire field is threaded through.
 *
 * @module main/ipc/__tests__/session.test
 */

import { describe, expect, it, vi, beforeEach } from 'vitest'
import { ipcMain } from 'electron'
import { registerSessionHandlers } from '../session'
import type { Store } from '../../persistence'
import { registerTraceSink, type TraceEvent } from '../../../shared/trace'

vi.mock('electron', () => ({
  ipcMain: {
    handle: vi.fn(),
    on: vi.fn()
  }
}))

type MockIpcMain = { handle: ReturnType<typeof vi.fn>; on: ReturnType<typeof vi.fn> }

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

function findReadScrollbackHandler(
  store: Store
): (event: { returnValue: unknown }, args: { ref?: unknown } | undefined) => void {
  registerSessionHandlers(store)
  const mock = ipcMain as unknown as MockIpcMain
  const call = mock.on.mock.calls.find(
    (c: unknown[]) => c[0] === 'session:read-terminal-scrollback-sync'
  )
  if (!call) {
    throw new Error('session:read-terminal-scrollback-sync handler was not registered')
  }
  return call[1] as (event: { returnValue: unknown }, args: { ref?: unknown } | undefined) => void
}

describe('session:read-terminal-scrollback-sync tracing (CR-TRACE-003)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('emits a terminalReattach span with restoredBytes on a successful restore', () => {
    const store = {
      readTerminalScrollbackSnapshot: vi.fn().mockReturnValue('hello world')
    } as unknown as Store
    const handler = findReadScrollbackHandler(store)
    const { events, stop } = captureTraceEvents()

    const event = { returnValue: undefined as unknown }
    handler(event, { ref: 'v1-abc' })
    stop()

    expect(event.returnValue).toBe('hello world')
    const reattachEvents = events.filter((e) => e.flow === 'terminal:reattach')
    expect(reattachEvents[0]?.level).toBe('start')
    const ok = reattachEvents.find((e) => e.level === 'ok')
    expect(ok?.fields).toMatchObject({ ref: 'v1-abc', restoredBytes: 'hello world'.length })
  })

  it('emits span.ok() with restoredBytes=0 when the ref is not found (buffer null)', () => {
    const store = {
      readTerminalScrollbackSnapshot: vi.fn().mockReturnValue(null)
    } as unknown as Store
    const handler = findReadScrollbackHandler(store)
    const { events, stop } = captureTraceEvents()

    const event = { returnValue: undefined as unknown }
    handler(event, { ref: 'v1-missing' })
    stop()

    expect(event.returnValue).toBeNull()
    const reattachEvents = events.filter((e) => e.flow === 'terminal:reattach')
    const ok = reattachEvents.find((e) => e.level === 'ok')
    expect(ok?.fields).toMatchObject({ ref: 'v1-missing', restoredBytes: 0 })
  })

  it('does not start a span when ref is missing/invalid', () => {
    const store = {
      readTerminalScrollbackSnapshot: vi.fn()
    } as unknown as Store
    const handler = findReadScrollbackHandler(store)
    const { events, stop } = captureTraceEvents()

    const missingRefEvent = { returnValue: undefined as unknown }
    handler(missingRefEvent, undefined)
    const invalidRefEvent = { returnValue: undefined as unknown }
    handler(invalidRefEvent, { ref: 42 })
    stop()

    expect(missingRefEvent.returnValue).toBeNull()
    expect(invalidRefEvent.returnValue).toBeNull()
    expect(events.filter((e) => e.flow === 'terminal:reattach')).toHaveLength(0)
    expect(store.readTerminalScrollbackSnapshot).not.toHaveBeenCalled()
  })

  it('calls span.fail() and returns null when readTerminalScrollbackSnapshot throws', () => {
    const store = {
      readTerminalScrollbackSnapshot: vi.fn().mockImplementation(() => {
        throw new Error('disk_error')
      })
    } as unknown as Store
    const handler = findReadScrollbackHandler(store)
    const { events, stop } = captureTraceEvents()

    const event = { returnValue: undefined as unknown }
    handler(event, { ref: 'v1-broken' })
    stop()

    expect(event.returnValue).toBeNull()
    const failEvent = events.find((e) => e.flow === 'terminal:reattach' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.err).toContain('disk_error')
  })
})
