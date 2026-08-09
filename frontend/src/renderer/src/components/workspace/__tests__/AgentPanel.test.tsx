// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { registerTraceSink, type TraceEvent } from '../../../../../shared/trace'
import { peekOpenAgentOrchSpan } from '@/lib/agent-orchestration-active-spans'

const updateAgentStatus = vi.fn()
const setRemoteAgentSession = vi.fn()
let mockSession: { sessionId: string; status: string; errorMessage?: string } | undefined

vi.mock('../../../store', () => ({
  useAppStore: (selector: (state: unknown) => unknown) =>
    selector({
      remoteAgentSessions: mockSession ? { 'wt-1': mockSession } : {},
      setRemoteAgentSession,
      updateAgentStatus
    })
}))

const start = vi.fn()
const stop = vi.fn()
const resume = vi.fn()
const onStatusChanged = vi.fn(() => vi.fn())

// Why: augment the real happy-dom `window` in place — replacing it wholesale
// (e.g. spreading its own enumerable props into a plain object) silently drops
// host APIs (matchMedia, ResizeObserver, ...) that Radix UI's Select needs.
;(window as unknown as { api: unknown }).api = {
  agentOrchestration: { start, stop, resume, onStatusChanged }
}

import { AgentPanel } from '../AgentPanel'

afterEach(() => cleanup())

describe('AgentPanel tracing (TASK-FE-002.2)', () => {
  let events: TraceEvent[]
  let unregister: () => void

  beforeEach(() => {
    vi.clearAllMocks()
    mockSession = undefined
    events = []
    unregister = registerTraceSink((event) => events.push(event))
  })

  afterEach(() => unregister())

  function agentEvents(flow: string): TraceEvent[] {
    return events.filter((e) => e.flow === flow)
  }

  it('starts a ui:agentOrch.spawn span and forwards traceId, staying open on status "started"', async () => {
    start.mockResolvedValue({ sessionId: 'sess-1', status: 'started' })
    render(<AgentPanel worktreeId="wt-1" />)

    fireEvent.click(screen.getByText('Start Agent'))

    await waitFor(() => expect(start).toHaveBeenCalled())
    expect(start).toHaveBeenCalledWith(
      expect.objectContaining({ worktreeId: 'wt-1', traceId: expect.any(String) })
    )
    const spawnEvents = agentEvents('ui:agentOrch.spawn')
    expect(spawnEvents.some((e) => e.level === 'start')).toBe(true)
    expect(spawnEvents.some((e) => e.level === 'ok')).toBe(false)
    // Span stays open in the registry, waiting for statusChanged.
    expect(peekOpenAgentOrchSpan('wt-1')).toBeDefined()
  })

  it('ok()s the spawn span immediately when the agent is already running', async () => {
    start.mockResolvedValue({ sessionId: 'sess-1', status: 'already-running' })
    render(<AgentPanel worktreeId="wt-1" />)

    fireEvent.click(screen.getByText('Start Agent'))

    await waitFor(() =>
      expect(agentEvents('ui:agentOrch.spawn').some((e) => e.level === 'ok')).toBe(true)
    )
    expect(peekOpenAgentOrchSpan('wt-1')).toBeUndefined()
  })

  it('fail()s the spawn span and clears the registry when the IPC call rejects', async () => {
    start.mockRejectedValue(new Error('boom'))
    render(<AgentPanel worktreeId="wt-1" />)

    fireEvent.click(screen.getByText('Start Agent'))

    await waitFor(() =>
      expect(agentEvents('ui:agentOrch.spawn').some((e) => e.level === 'fail')).toBe(true)
    )
    expect(peekOpenAgentOrchSpan('wt-1')).toBeUndefined()
    const failEvent = agentEvents('ui:agentOrch.spawn').find((e) => e.level === 'fail')
    // No secrets: only worktreeId/agentType, never env/credential fields.
    expect(Object.keys(failEvent?.fields ?? {}).sort()).toEqual(['agentType', 'err', 'worktreeId'])
  })

  it('ok()s the ui:agentOrch.stop span immediately on success', async () => {
    mockSession = { sessionId: 'sess-1', status: 'running' }
    stop.mockResolvedValue(undefined)
    render(<AgentPanel worktreeId="wt-1" />)

    fireEvent.click(screen.getByText('Stop'))

    await waitFor(() => expect(stop).toHaveBeenCalled())
    expect(stop).toHaveBeenCalledWith(
      expect.objectContaining({ sessionId: 'sess-1', traceId: expect.any(String) })
    )
    const stopEvents = agentEvents('ui:agentOrch.stop')
    expect(stopEvents.some((e) => e.level === 'start')).toBe(true)
    expect(stopEvents.some((e) => e.level === 'ok')).toBe(true)
  })

  it('fail()s the stop span when the IPC call rejects', async () => {
    mockSession = { sessionId: 'sess-1', status: 'running' }
    stop.mockRejectedValue(new Error('stop failed'))
    render(<AgentPanel worktreeId="wt-1" />)

    fireEvent.click(screen.getByText('Stop'))

    await waitFor(() =>
      expect(agentEvents('ui:agentOrch.stop').some((e) => e.level === 'fail')).toBe(true)
    )
  })

  it('starts a ui:agentOrch.resume span that stays open when resumed:true', async () => {
    mockSession = { sessionId: 'sess-1', status: 'stopped' }
    resume.mockResolvedValue({ resumed: true })
    render(<AgentPanel worktreeId="wt-1" />)

    fireEvent.click(screen.getByText('Resume'))

    await waitFor(() => expect(resume).toHaveBeenCalled())
    expect(resume).toHaveBeenCalledWith(
      expect.objectContaining({ sessionId: 'sess-1', traceId: expect.any(String) })
    )
    const resumeEvents = agentEvents('ui:agentOrch.resume')
    expect(resumeEvents.some((e) => e.level === 'start')).toBe(true)
    expect(resumeEvents.some((e) => e.level === 'ok')).toBe(false)
    expect(peekOpenAgentOrchSpan('wt-1')).toBeDefined()
  })

  it('fail()s (not ok()s) the resume span when resumed:false', async () => {
    mockSession = { sessionId: 'sess-1', status: 'stopped' }
    resume.mockResolvedValue({ resumed: false })
    render(<AgentPanel worktreeId="wt-1" />)

    fireEvent.click(screen.getByText('Resume'))

    await waitFor(() =>
      expect(agentEvents('ui:agentOrch.resume').some((e) => e.level === 'fail')).toBe(true)
    )
    expect(agentEvents('ui:agentOrch.resume').some((e) => e.level === 'ok')).toBe(false)
    expect(peekOpenAgentOrchSpan('wt-1')).toBeUndefined()
  })

  it('fail()s the resume span and clears the registry when the IPC call rejects', async () => {
    mockSession = { sessionId: 'sess-1', status: 'stopped' }
    resume.mockRejectedValue(new Error('resume failed'))
    render(<AgentPanel worktreeId="wt-1" />)

    fireEvent.click(screen.getByText('Resume'))

    await waitFor(() =>
      expect(agentEvents('ui:agentOrch.resume').some((e) => e.level === 'fail')).toBe(true)
    )
    expect(peekOpenAgentOrchSpan('wt-1')).toBeUndefined()
  })
})
