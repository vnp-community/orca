// @vitest-environment happy-dom
import { renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AgentStatusEntry } from '../../../../../shared/agent-status-types'

const flags = { state: 'enabled', codeIntel: true, quality: true, ai: false }
const call = vi.fn()

type FakeState = {
  agentStatusByPaneKey: Record<string, AgentStatusEntry>
  gitBranchCompareSummaryByWorktree: Record<string, { headOid: string } | null>
  gitStatusByWorktree: Record<string, { path: string; area: string; status: string }[]>
  worktreesByRepo: Record<string, { id: string; projectId?: string }[]>
}

const fake = vi.hoisted(() => {
  const holder = { state: {} as unknown }
  const listeners = new Set<(s: unknown, p: unknown) => void>()
  const subscribe = vi.fn((l: (s: unknown, p: unknown) => void) => {
    listeners.add(l)
    return () => listeners.delete(l)
  })
  return { holder, listeners, subscribe }
})
const { listeners, subscribe } = fake

vi.mock('@/store', () => ({ useAppStore: { getState: () => fake.holder.state, subscribe: fake.subscribe } }))
vi.mock('@/lib/worktree-runtime-owner', () => ({ getRuntimeEnvironmentIdForWorktree: () => null }))
vi.mock('../../../hooks/useQualityFeatureFlags', () => ({ useQualityFeatureFlags: () => flags }))
vi.mock('../../../runtime/code-intel-client', () => ({
  getCodeIntelClient: () => ({ call }),
  classifyCodeIntelError: () => ({ kind: 'unknown' })
}))

import { useAgentTurnBackendRecorder } from './use-agent-turn-backend-recorder'

function entry(s: AgentStatusEntry['state'], at: number): AgentStatusEntry {
  return {
    state: s, prompt: 'do the thing', updatedAt: at, stateStartedAt: at, paneKey: 'p', agentType: 'claude',
    stateHistory: [{ state: 'working', prompt: '', startedAt: 1 }], worktreeId: 'wt',
    ...(s === 'working' ? { toolName: 'Bash', toolInput: 'pnpm test' } : {})
  }
}

function emit(next: FakeState['agentStatusByPaneKey']) {
  const prev = fake.holder.state as FakeState
  const nextState = { ...prev, agentStatusByPaneKey: next }
  fake.holder.state = nextState
  for (const l of listeners) {l(nextState, prev)}
}

beforeEach(() => {
  vi.useFakeTimers()
  call.mockReset()
  call.mockResolvedValue({ ok: true, result: { turn: {} } })
  listeners.clear()
  subscribe.mockClear()
  flags.quality = true
  fake.holder.state = {
    agentStatusByPaneKey: {},
    gitBranchCompareSummaryByWorktree: { wt: { headOid: 'h1' } },
    gitStatusByWorktree: { wt: [{ path: 'a.ts', area: 'unstaged', status: 'modified' }] },
    worktreesByRepo: { r: [{ id: 'wt', projectId: 'proj' }] }
  } satisfies FakeState
})
afterEach(() => vi.useRealTimers())

describe('useAgentTurnBackendRecorder', () => {
  it('does not subscribe or call anything when the quality flag is off', async () => {
    flags.quality = false
    renderHook(() => useAgentTurnBackendRecorder())
    expect(subscribe).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(5000)
    expect(call).not.toHaveBeenCalled()
  })

  it('records one turn after completion with sampled commands and no raw prompt', async () => {
    renderHook(() => useAgentTurnBackendRecorder())
    emit({ p: entry('working', 10) })
    emit({ p: entry('done', 20) })
    expect(call).not.toHaveBeenCalled() // waits for git to settle
    await vi.advanceTimersByTimeAsync(3100)
    expect(call).toHaveBeenCalledTimes(1)
    const [wt, method, params] = call.mock.calls[0]
    expect(wt).toBe('wt')
    expect(method).toBe('codeIntel.quality.turn.record')
    expect(params).toMatchObject({
      projectId: 'proj', worktreeId: 'wt', clientTurnId: 'p:20', endHeadCommit: 'h1', treeDirtyEnd: true,
      filesChangedCount: 1
    })
    expect(params.commandsSummary.commands[0]).toMatchObject({ name: 'pnpm', sub: 'test', count: 1 })
    expect(JSON.stringify(params)).not.toContain('do the thing')
  })

  it('skips recording when the worktree has no projectId', async () => {
    ;(fake.holder.state as FakeState).worktreesByRepo = { r: [{ id: 'wt' }] }
    renderHook(() => useAgentTurnBackendRecorder())
    emit({ p: entry('working', 10) })
    emit({ p: entry('done', 20) })
    await vi.advanceTimersByTimeAsync(4000)
    expect(call).not.toHaveBeenCalled()
  })

  it('swallows RPC errors without throwing and unsubscribes on unmount', async () => {
    call.mockRejectedValue(new Error('boom'))
    const { unmount } = renderHook(() => useAgentTurnBackendRecorder())
    emit({ p: entry('working', 10) })
    emit({ p: entry('done', 20) })
    await vi.advanceTimersByTimeAsync(3100)
    expect(call).toHaveBeenCalledTimes(1)
    unmount()
    expect(listeners.size).toBe(0)
  })
})
