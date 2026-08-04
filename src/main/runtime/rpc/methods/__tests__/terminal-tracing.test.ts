/**
 * Tests for terminal RPC handler tracing (TASK-BE-003.1/003.4).
 *
 * Covers `terminal.create`, `terminal.split`, `terminal.resizeForClient` in
 * `src/main/runtime/rpc/methods/terminal.ts` — instrumented with
 * `Tracers.terminalCreate` (shared by create + split, no separate
 * `terminal:split` tracer) and `Tracers.terminalResize`.
 *
 * @module main/runtime/rpc/methods/__tests__/terminal-tracing.test
 */

import { describe, expect, it, vi } from 'vitest'
import { RpcDispatcher } from '../../dispatcher'
import type { RpcRequest } from '../../core'
import type { OrcaRuntimeService } from '../../../orca-runtime'
import { TERMINAL_METHODS } from '../terminal'
import { registerTraceSink, type TraceEvent } from '../../../../../shared/trace'

function makeRequest(method: string, params?: unknown): RpcRequest {
  return { id: 'req-1', authToken: 'tok', method, params }
}

function captureTraceEvents(): { events: TraceEvent[]; stop: () => void } {
  const events: TraceEvent[] = []
  const unregister = registerTraceSink((e) => events.push(e))
  return { events, stop: unregister }
}

describe('terminal RPC tracing (CR-TRACE-003)', () => {
  it('terminal.create emits a terminal:create span with ok() containing ptyId', async () => {
    const runtime = {
      getRuntimeId: () => 'test-runtime',
      createTerminal: vi
        .fn()
        .mockResolvedValue({ handle: 'h-1', worktreeId: 'wt-1', title: null, ptyId: 'pty-1' })
    } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: TERMINAL_METHODS })
    const { events, stop } = captureTraceEvents()

    const response = await dispatcher.dispatch(
      makeRequest('terminal.create', { worktree: 'id:wt-1' }),
      { userId: 'user-1' }
    )
    stop()

    expect(response).toMatchObject({ ok: true })
    const created = events.filter((e) => e.flow === 'terminal:create')
    expect(created[0]?.level).toBe('start')
    const ok = created.find((e) => e.level === 'ok')
    expect(ok?.fields).toMatchObject({ ptyId: 'pty-1' })
  })

  it('terminal.create resumes the span id from params.traceId when provided', async () => {
    const runtime = {
      getRuntimeId: () => 'test-runtime',
      createTerminal: vi
        .fn()
        .mockResolvedValue({ handle: 'h-1', worktreeId: 'wt-1', title: null, ptyId: 'pty-1' })
    } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: TERMINAL_METHODS })
    const { events, stop } = captureTraceEvents()

    await dispatcher.dispatch(
      makeRequest('terminal.create', { worktree: 'id:wt-1', traceId: 'resume-terminal-1' }),
      { userId: 'user-1' }
    )
    stop()

    const created = events.filter((e) => e.flow === 'terminal:create')
    expect(created.length).toBeGreaterThan(0)
    expect(created.every((e) => e.id === 'resume-terminal-1')).toBe(true)
  })

  it('terminal.create generates a fresh span id when params.traceId is absent (backward compatible)', async () => {
    const runtime = {
      getRuntimeId: () => 'test-runtime',
      createTerminal: vi
        .fn()
        .mockResolvedValue({ handle: 'h-1', worktreeId: 'wt-1', title: null, ptyId: 'pty-1' })
    } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: TERMINAL_METHODS })
    const { events, stop } = captureTraceEvents()

    await dispatcher.dispatch(makeRequest('terminal.create', { worktree: 'id:wt-1' }), {
      userId: 'user-1'
    })
    stop()

    const created = events.filter((e) => e.flow === 'terminal:create')
    expect(created[0]?.id).toBeTruthy()
    expect(created[0]?.id).not.toBe('resume-terminal-1')
  })

  it('terminal.create calls span.fail() on createTerminal rejection, then rethrows', async () => {
    const runtime = {
      getRuntimeId: () => 'test-runtime',
      createTerminal: vi.fn().mockRejectedValue(new Error('spawn_failed'))
    } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: TERMINAL_METHODS })
    const { events, stop } = captureTraceEvents()

    const response = await dispatcher.dispatch(
      makeRequest('terminal.create', { worktree: 'id:wt-1' }),
      { userId: 'user-1' }
    )
    stop()

    expect(response).toMatchObject({ ok: false })
    const failEvent = events.find((e) => e.flow === 'terminal:create' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.err).toContain('spawn_failed')
  })

  it('terminal.create calls span.fail() with UNAUTHORIZED when ctx.userId is missing, then throws', async () => {
    const runtime = {
      getRuntimeId: () => 'test-runtime',
      createTerminal: vi.fn()
    } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: TERMINAL_METHODS })
    const { events, stop } = captureTraceEvents()

    const response = await dispatcher.dispatch(
      makeRequest('terminal.create', { worktree: 'id:wt-1' })
    )
    stop()

    expect(response).toMatchObject({ ok: false })
    expect(runtime.createTerminal).not.toHaveBeenCalled()
    const failEvent = events.find((e) => e.flow === 'terminal:create' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.err).toContain('UNAUTHORIZED')
  })

  it('terminal.split reuses Tracers.terminalCreate — no separate terminal:split tracer emitted', async () => {
    const runtime = {
      getRuntimeId: () => 'test-runtime',
      splitTerminal: vi.fn().mockResolvedValue({ handle: 'h-2', tabId: 'tab-1', paneRuntimeId: 1 })
    } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: TERMINAL_METHODS })
    const { events, stop } = captureTraceEvents()

    const response = await dispatcher.dispatch(
      makeRequest('terminal.split', { terminal: 'h-1', direction: 'vertical' })
    )
    stop()

    expect(response).toMatchObject({ ok: true })
    expect(events.some((e) => e.flow === 'terminal:create')).toBe(true)
    expect(events.some((e) => e.flow === 'terminal:split')).toBe(false)
  })

  it('terminal.resizeForClient emits a terminal:resize span distinct from terminal:create', async () => {
    const runtime = {
      getRuntimeId: () => 'test-runtime',
      resolveLiveLeafForHandle: () => ({ ptyId: 'pty-1' }),
      resizeForClient: vi.fn().mockResolvedValue({ cols: 80, rows: 24 })
    } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: TERMINAL_METHODS })
    const { events, stop } = captureTraceEvents()

    const response = await dispatcher.dispatch(
      makeRequest('terminal.resizeForClient', {
        terminal: 'h-1',
        mode: 'restore',
        clientId: 'client-1'
      })
    )
    stop()

    expect(response).toMatchObject({ ok: true })
    const resizeEvents = events.filter((e) => e.flow === 'terminal:resize')
    expect(resizeEvents.length).toBeGreaterThan(0)
    expect(events.some((e) => e.flow === 'terminal:create')).toBe(false)
    const ok = resizeEvents.find((e) => e.level === 'ok')
    expect(ok?.fields).toMatchObject({ ptyId: 'pty-1' })
  })

  it('terminal.resizeForClient calls span.fail("no_connected_pty", ...) before throwing when no leaf/ptyId is resolved', async () => {
    const runtime = {
      getRuntimeId: () => 'test-runtime',
      resolveLiveLeafForHandle: () => null,
      resizeForClient: vi.fn().mockResolvedValue({ cols: 80, rows: 24 })
    } as unknown as OrcaRuntimeService
    const dispatcher = new RpcDispatcher({ runtime, methods: TERMINAL_METHODS })
    const { events, stop } = captureTraceEvents()

    const response = await dispatcher.dispatch(
      makeRequest('terminal.resizeForClient', {
        terminal: 'h-1',
        mode: 'restore',
        clientId: 'client-1'
      })
    )
    stop()

    expect(response).toMatchObject({ ok: false })
    const failEvent = events.find((e) => e.flow === 'terminal:resize' && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.err).toContain('no_connected_pty')
  })
})
