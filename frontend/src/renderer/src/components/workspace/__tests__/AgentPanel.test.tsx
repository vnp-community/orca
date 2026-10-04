// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, fireEvent, cleanup, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { registerTraceSink, type TraceEvent } from '../../../../../shared/trace'
import { peekOpenAgentOrchSpan } from '@/lib/agent-orchestration-active-spans'

// Why vi.hoisted: vi.mock factories are hoisted above ordinary top-level
// `const`/`let` — referencing those directly inside a factory throws
// "Cannot access ... before initialization"; vi.hoisted runs before that
// hoisting so the factories can safely close over these.
const {
  updateAgentStatus,
  setRemoteAgentSession,
  sessionBox,
  resolveRuntimeAgentProvider,
  startRuntimeAgentSession,
  resumeRuntimeAgentSession,
  stopRuntimeAgentSession,
  subscribeRuntimeAgentStatus
} = vi.hoisted(() => ({
  updateAgentStatus: vi.fn(),
  setRemoteAgentSession: vi.fn(),
  sessionBox: {
    current: undefined as
      | { sessionId: string; status: string; errorMessage?: string; startedAt?: number }
      | undefined
  },
  resolveRuntimeAgentProvider: vi.fn(),
  startRuntimeAgentSession: vi.fn(),
  resumeRuntimeAgentSession: vi.fn(),
  stopRuntimeAgentSession: vi.fn(),
  subscribeRuntimeAgentStatus: vi.fn()
}))

vi.mock('../../../store', () => {
  const state = {
    get remoteAgentSessions() {
      return sessionBox.current ? { 'wt-1': sessionBox.current } : {}
    },
    worktreesByRepo: {
      'repo-1': [{ id: 'wt-1', path: '/repo/wt-1', branch: 'main', isMain: false }]
    },
    settings: {},
    setRemoteAgentSession,
    updateAgentStatus
  }
  const useAppStore = (selector: (s: unknown) => unknown) => selector(state)
  useAppStore.getState = () => state
  return { useAppStore }
})

vi.mock('../../../context/WorkspaceContext', () => ({
  useWorkspace: () => ({ project: { id: 'proj-1', devServerId: 'ds-1' } })
}))

vi.mock('../../../hooks/useAuthSession', () => ({
  useAuthUser: () => ({ id: 'user-1', email: 'u@x.com', name: 'U', role: 'developer' })
}))

vi.mock('../../../lib/connection-context', () => ({
  getConnectionId: () => 'conn-1'
}))

vi.mock('../../../runtime/runtime-rpc-client', () => ({
  getActiveRuntimeTarget: () => ({ kind: 'environment', environmentId: 'env-1' })
}))

vi.mock('../../../runtime/runtime-agent-orchestration-client', () => ({
  resolveRuntimeAgentProvider,
  startRuntimeAgentSession,
  resumeRuntimeAgentSession,
  stopRuntimeAgentSession,
  subscribeRuntimeAgentStatus
}))

function agentSession(overrides: Partial<Record<string, unknown>> = {}) {
  return {
    id: 'sess-1',
    ptyId: 'pty-1',
    worktreeId: 'wt-1',
    devServerId: 'ds-1',
    userId: 'user-1',
    modelId: 'model-1',
    accountId: 'acct-1',
    status: 'spawning',
    startedAtUnixMs: 0,
    lastActiveAtUnixMs: 0,
    ...overrides
  }
}

import { AgentPanel } from '../AgentPanel'

afterEach(() => cleanup())

describe('AgentPanel tracing (TASK-FE-002.2)', () => {
  let events: TraceEvent[]
  let unregister: () => void

  beforeEach(() => {
    vi.clearAllMocks()
    sessionBox.current = undefined
    events = []
    unregister = registerTraceSink((event) => events.push(event))
    resolveRuntimeAgentProvider.mockResolvedValue({ accountId: 'acct-1', modelId: 'model-1' })
    subscribeRuntimeAgentStatus.mockResolvedValue({ unsubscribe: vi.fn() })
  })

  afterEach(() => unregister())

  function agentEvents(flow: string): TraceEvent[] {
    return events.filter((e) => e.flow === flow)
  }

  it('starts a ui:agentOrch.spawn span, resolving the provider before starting, staying open on ack', async () => {
    startRuntimeAgentSession.mockResolvedValue({ ack: agentSession(), unsubscribe: vi.fn() })
    render(<AgentPanel worktreeId="wt-1" />)

    fireEvent.click(screen.getByText('Start Agent'))

    await waitFor(() => expect(startRuntimeAgentSession).toHaveBeenCalled())
    expect(resolveRuntimeAgentProvider).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        userId: 'user-1',
        projectId: 'proj-1',
        devServerId: 'ds-1',
        agentType: 'claude'
      })
    )
    expect(startRuntimeAgentSession).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        connectionId: 'conn-1',
        worktreeId: 'wt-1',
        userId: 'user-1',
        cwd: '/repo/wt-1',
        modelId: 'model-1',
        accountId: 'acct-1',
        trustPreset: 'standard'
      }),
      expect.any(Function)
    )
    const spawnEvents = agentEvents('ui:agentOrch.spawn')
    expect(spawnEvents.some((e) => e.level === 'start')).toBe(true)
    expect(spawnEvents.some((e) => e.level === 'ok')).toBe(false)
    // Span stays open in the registry, waiting for agent.subscribeStatus.
    expect(peekOpenAgentOrchSpan('wt-1')).toBeDefined()
  })

  it('fail()s the spawn span and clears the registry when aiProvider.resolve rejects', async () => {
    resolveRuntimeAgentProvider.mockRejectedValue(new Error('no account available'))
    render(<AgentPanel worktreeId="wt-1" />)

    fireEvent.click(screen.getByText('Start Agent'))

    await waitFor(() =>
      expect(agentEvents('ui:agentOrch.spawn').some((e) => e.level === 'fail')).toBe(true)
    )
    expect(startRuntimeAgentSession).not.toHaveBeenCalled()
    expect(peekOpenAgentOrchSpan('wt-1')).toBeUndefined()
    const failEvent = agentEvents('ui:agentOrch.spawn').find((e) => e.level === 'fail')
    // No secrets: only worktreeId/agentType, never env/credential fields.
    expect(Object.keys(failEvent?.fields ?? {}).sort()).toEqual(['agentType', 'err', 'worktreeId'])
  })

  it('fail()s the spawn span when agent.start rejects', async () => {
    startRuntimeAgentSession.mockRejectedValue(new Error('boom'))
    render(<AgentPanel worktreeId="wt-1" />)

    fireEvent.click(screen.getByText('Start Agent'))

    await waitFor(() =>
      expect(agentEvents('ui:agentOrch.spawn').some((e) => e.level === 'fail')).toBe(true)
    )
    expect(peekOpenAgentOrchSpan('wt-1')).toBeUndefined()
  })

  it('ok()s the ui:agentOrch.stop span immediately on success', async () => {
    sessionBox.current = { sessionId: 'sess-1', status: 'running' }
    stopRuntimeAgentSession.mockResolvedValue(undefined)
    render(<AgentPanel worktreeId="wt-1" />)

    fireEvent.click(screen.getByText('Stop'))

    await waitFor(() => expect(stopRuntimeAgentSession).toHaveBeenCalled())
    expect(stopRuntimeAgentSession).toHaveBeenCalledWith(expect.anything(), 'sess-1')
    const stopEvents = agentEvents('ui:agentOrch.stop')
    expect(stopEvents.some((e) => e.level === 'start')).toBe(true)
    expect(stopEvents.some((e) => e.level === 'ok')).toBe(true)
  })

  it('fail()s the stop span when agent.stop rejects', async () => {
    sessionBox.current = { sessionId: 'sess-1', status: 'running' }
    stopRuntimeAgentSession.mockRejectedValue(new Error('stop failed'))
    render(<AgentPanel worktreeId="wt-1" />)

    fireEvent.click(screen.getByText('Stop'))

    await waitFor(() =>
      expect(agentEvents('ui:agentOrch.stop').some((e) => e.level === 'fail')).toBe(true)
    )
  })

  it('starts a ui:agentOrch.resume span that stays open, resuming by worktreeId not sessionId', async () => {
    sessionBox.current = { sessionId: 'sess-1', status: 'stopped', startedAt: 123 }
    resumeRuntimeAgentSession.mockResolvedValue({
      ack: agentSession({ status: 'spawning' }),
      unsubscribe: vi.fn()
    })
    render(<AgentPanel worktreeId="wt-1" />)

    fireEvent.click(screen.getByText('Resume'))

    await waitFor(() => expect(resumeRuntimeAgentSession).toHaveBeenCalled())
    expect(resumeRuntimeAgentSession).toHaveBeenCalledWith(
      expect.anything(),
      expect.objectContaining({
        connectionId: 'conn-1',
        worktreeId: 'wt-1',
        userId: 'user-1',
        cwd: '/repo/wt-1'
      }),
      expect.any(Function)
    )
    const resumeEvents = agentEvents('ui:agentOrch.resume')
    expect(resumeEvents.some((e) => e.level === 'start')).toBe(true)
    expect(resumeEvents.some((e) => e.level === 'ok')).toBe(false)
    expect(peekOpenAgentOrchSpan('wt-1')).toBeDefined()
  })

  it('fail()s the resume span and clears the registry when agent.resume rejects', async () => {
    sessionBox.current = { sessionId: 'sess-1', status: 'stopped' }
    resumeRuntimeAgentSession.mockRejectedValue(new Error('resume failed'))
    render(<AgentPanel worktreeId="wt-1" />)

    fireEvent.click(screen.getByText('Resume'))

    await waitFor(() =>
      expect(agentEvents('ui:agentOrch.resume').some((e) => e.level === 'fail')).toBe(true)
    )
    expect(peekOpenAgentOrchSpan('wt-1')).toBeUndefined()
  })
})
