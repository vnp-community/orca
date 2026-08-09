// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderHook } from '@testing-library/react'
import {
  peekOpenAgentOrchSpan,
  registerOpenAgentOrchSpan,
  takeOpenAgentOrchSpan
} from '@/lib/agent-orchestration-active-spans'
import { createTracer, registerTraceSink, type TraceEvent, type TraceSpan } from '../../../../shared/trace'

const updateAgentStatus = vi.fn()

vi.mock('../../store', () => ({
  useAppStore: (selector: (state: { updateAgentStatus: typeof updateAgentStatus }) => unknown) =>
    selector({ updateAgentStatus })
}))

type StatusChangedEvent = {
  worktreeId: string
  sessionId?: string
  status: 'starting' | 'running' | 'stopped' | 'error'
  errorMessage?: string
}
type StatusChangedCallback = (event: StatusChangedEvent) => void

let statusChangedCallback: StatusChangedCallback | null = null
const onStatusChanged = vi.fn((callback: StatusChangedCallback) => {
  statusChangedCallback = callback
  return vi.fn()
})

import { useAgentOrchestrationEvents } from '../use-agent-orchestration-events'

const testTracer = createTracer('test:agentOrchEvents')

function openSpan(): TraceSpan {
  return testTracer.start()
}

describe('useAgentOrchestrationEvents tracing (TASK-FE-002.3)', () => {
  let events: TraceEvent[]
  let unregister: () => void

  beforeEach(() => {
    vi.clearAllMocks()
    statusChangedCallback = null
    events = []
    unregister = registerTraceSink((event) => events.push(event))
    ;(window as unknown as { api: unknown }).api = {
      agentOrchestration: { onStatusChanged }
    }
    renderHook(() => useAgentOrchestrationEvents())
    expect(statusChangedCallback).not.toBeNull()
  })

  afterEach(() => unregister())

  it("ok()s the open span and clears the registry on status 'running'", () => {
    const span = openSpan()
    registerOpenAgentOrchSpan('wt-1', span)

    statusChangedCallback!({ worktreeId: 'wt-1', sessionId: 'sess-1', status: 'running' })

    expect(updateAgentStatus).toHaveBeenCalledWith(
      expect.objectContaining({ worktreeId: 'wt-1', status: 'running' })
    )
    const okEvent = events.find((e) => e.id === span.id && e.level === 'ok')
    expect(okEvent).toBeDefined()
    expect(takeOpenAgentOrchSpan('wt-1')).toBeUndefined()
  })

  it("does not throw when status is 'running' with no open span for that worktree", () => {
    expect(() =>
      statusChangedCallback!({ worktreeId: 'wt-no-span', status: 'running' })
    ).not.toThrow()
    expect(updateAgentStatus).toHaveBeenCalled()
  })

  it("fail()s the open span on status 'error'", () => {
    const span = openSpan()
    registerOpenAgentOrchSpan('wt-2', span)

    statusChangedCallback!({ worktreeId: 'wt-2', status: 'error', errorMessage: 'boom' })

    const failEvent = events.find((e) => e.id === span.id && e.level === 'fail')
    expect(failEvent).toBeDefined()
    expect(failEvent?.fields.err).toBe('boom')
    expect(takeOpenAgentOrchSpan('wt-2')).toBeUndefined()
  })

  it("does not throw when status is 'error' with no open span for that worktree", () => {
    expect(() =>
      statusChangedCallback!({ worktreeId: 'wt-no-span-2', status: 'error' })
    ).not.toThrow()
  })

  it("step()s the open span on status 'starting' without removing it from the registry", () => {
    const span = openSpan()
    registerOpenAgentOrchSpan('wt-3', span)

    statusChangedCallback!({ worktreeId: 'wt-3', status: 'starting' })

    const stepEvent = events.find((e) => e.id === span.id && e.level === 'step')
    expect(stepEvent?.label).toBe('statusChanged')
    // Span is still open — not removed by the 'starting' branch.
    expect(peekOpenAgentOrchSpan('wt-3')).toBe(span)
  })

  it("does not touch the registry on status 'stopped'", () => {
    const span = openSpan()
    registerOpenAgentOrchSpan('wt-4', span)

    statusChangedCallback!({ worktreeId: 'wt-4', status: 'stopped' })

    // No step/ok/fail emitted for this span from the 'stopped' branch.
    expect(events.some((e) => e.id === span.id && e.level !== 'start')).toBe(false)
    expect(peekOpenAgentOrchSpan('wt-4')).toBe(span)
  })

  it('never opens a dedicated tracer span for the statusChanged event itself', () => {
    const span = openSpan()
    registerOpenAgentOrchSpan('wt-5', span)
    const startCountBefore = events.filter((e) => e.level === 'start').length

    statusChangedCallback!({ worktreeId: 'wt-5', sessionId: 'sess-5', status: 'running' })

    const startCountAfter = events.filter((e) => e.level === 'start').length
    expect(startCountAfter).toBe(startCountBefore)
  })
})
