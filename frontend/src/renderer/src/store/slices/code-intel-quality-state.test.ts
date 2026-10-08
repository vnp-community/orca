/**
 * Tests for code-intel-quality-state.ts (FE-CV-TASK-087-02)
 */

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createCodeIntelQualitySlice, QUALITY_RUN_POLL_INTERVAL_MS } from './code-intel-quality-state'
import type { CodeIntelQualitySlice } from './code-intel-quality-state'
import type { QualityCall, QualitySliceData } from './code-intel-quality-slice-context'

type Reply = ReturnType<QualityCall>

function makeSlice(call: QualityCall, events: QualitySliceData['codeIntelEventsState'] = 'streaming') {
  let state = { codeIntelQualityByWorktree: {}, codeIntelEventsState: events } as QualitySliceData
  let slice = {} as CodeIntelQualitySlice
  const set = (fn: (prev: QualitySliceData) => Partial<QualitySliceData>) => {
    state = { ...state, ...fn(state) }
  }
  slice = createCodeIntelQualitySlice(set, () => state, { call, now: () => 1000 })
  return { slice, state: () => state, wt: (id: string) => state.codeIntelQualityByWorktree[id] }
}

const ok = (result: unknown): Reply => Promise.resolve({ ok: true, result })
const fail = (kind: string, data: Record<string, unknown> | null = null, message = 'm'): Reply =>
  Promise.resolve({ ok: false, error: { kind, code: null, message, data, retryable: false } as never })

const runBody = (status: string, id = 'run-1') => ({ run: { id, status, profile: 'full', scope: 'changed' } })

describe('startQualityRun', () => {
  it('locks synchronously in phase starting and ignores a second start', async () => {
    const call = vi.fn(() => ok(runBody('running')))
    const { slice, wt } = makeSlice(call)
    const first = slice.startQualityRun('wt', { profile: 'full', scope: 'changed' })
    expect(wt('wt').run).toMatchObject({ phase: 'starting', runId: null })
    await slice.startQualityRun('wt', { profile: 'full', scope: 'changed' })
    await first
    expect(call).toHaveBeenCalledTimes(1)
    expect(call).toHaveBeenCalledWith('wt', 'codeIntel.quality.start', { profile: 'full', scope: 'changed' })
    expect(wt('wt').run).toMatchObject({ phase: 'running', runId: 'run-1' })
  })

  it('a queued response keeps phase queued', async () => {
    const { slice, wt } = makeSlice(() => ok(runBody('queued')))
    await slice.startQualityRun('wt', { profile: 'full', scope: 'changed' })
    expect(wt('wt').run?.phase).toBe('queued')
  })

  it('RUN_IN_PROGRESS attaches to the existing run instead of failing', async () => {
    const { slice, wt } = makeSlice(() => fail('run-in-progress', { runId: 'other' }))
    await slice.startQualityRun('wt', { profile: 'full', scope: 'changed' })
    expect(wt('wt').run).toMatchObject({ runId: 'other', phase: 'running' })
    expect(wt('wt').runError).toBeNull()
  })

  it('ENV_NOT_READY keeps missing[] as a persistent inline error', async () => {
    const { slice, wt } = makeSlice(() => fail('env-not-ready', { missing: [{ check: 'go', reason: 'absent', hint: 'install' }] }))
    await slice.startQualityRun('wt', { profile: 'full', scope: 'changed' })
    expect(wt('wt').run).toBeNull()
    expect(wt('wt').runError).toMatchObject({ kind: 'env-not-ready', missing: [{ check: 'go', hint: 'install' }] })
  })

  it('PROFILE_UNKNOWN keeps available[]; RATE_LIMITED keeps retryAfterSeconds', async () => {
    const calls: string[] = []
    const a = makeSlice((_w, method) => {
      calls.push(method)
      return method.endsWith('start') ? fail('profile-unknown', { available: ['a', 'b'] }) : ok({ runnableProfiles: [] })
    })
    a.slice.setQualityUi('wt', { profile: 'x' })
    await a.slice.startQualityRun('wt', { profile: 'x', scope: 'changed' })
    expect(calls).toContain('codeIntel.quality.profile.get')
    expect(a.wt('wt').runError?.available).toEqual(['a', 'b'])
    expect(a.wt('wt').ui.profile).toBeNull()
    const b = makeSlice(() => fail('rate-limited', { retryAfterSeconds: 9 }))
    await b.slice.startQualityRun('wt', { profile: 'x', scope: 'changed' })
    expect(b.wt('wt').runError).toMatchObject({ kind: 'rate-limited', retryAfterSeconds: 9 })
  })

  it('forbidden / offline are reported with their kind', async () => {
    const { slice, wt } = makeSlice(() => fail('forbidden'))
    await slice.startQualityRun('wt', { profile: 'x', scope: 'changed' })
    expect(wt('wt').runError?.kind).toBe('forbidden')
  })
})

describe('cancelQualityRun', () => {
  it('moves to cancelling and only finished(cancelled) confirms', async () => {
    const call = vi.fn((_w: string, method: string) => (method.endsWith('start') ? ok(runBody('running')) : ok({ run: {} })))
    const { slice, wt } = makeSlice(call)
    await slice.startQualityRun('wt', { profile: 'full', scope: 'changed' })
    await slice.cancelQualityRun('wt')
    expect(wt('wt').run?.phase).toBe('cancelling')
    slice.applyQualityPushEvent({ event: 'qualityFinished', worktreeId: 'wt', runId: 'run-1', success: false, error: null, status: 'cancelled' })
    expect(wt('wt').run).toMatchObject({ phase: 'finished', status: 'cancelled' })
  })

  it('no-ops without an active run', async () => {
    const call = vi.fn(() => ok({}))
    const { slice } = makeSlice(call)
    await slice.cancelQualityRun('wt')
    expect(call).not.toHaveBeenCalled()
  })

  it('restores running when cancel is rejected', async () => {
    const call = vi.fn((_w: string, method: string) => (method.endsWith('start') ? ok(runBody('running')) : fail('forbidden')))
    const { slice, wt } = makeSlice(call)
    await slice.startQualityRun('wt', { profile: 'full', scope: 'changed' })
    await slice.cancelQualityRun('wt')
    expect(wt('wt').run?.phase).toBe('running')
    expect(wt('wt').runError?.kind).toBe('forbidden')
  })
})

describe('push events', () => {
  async function running() {
    const call = vi.fn((_w: string, method: string) => {
      if (method.endsWith('start')) {
        return ok(runBody('running'))
      }
      if (method.endsWith('gate')) {
        return ok({ gate: { verdict: 'pass' } })
      }
      if (method.endsWith('runs')) {
        return ok({ runs: [] })
      }
      return ok({})
    })
    const s = makeSlice(call)
    await s.slice.startQualityRun('wt', { profile: 'full', scope: 'changed' })
    return { ...s, call }
  }

  it('progress keeps percent null (unknown) and carries stage/step', async () => {
    const { slice, wt } = await running()
    slice.applyQualityPushEvent({ event: 'qualityProgress', worktreeId: 'wt', runId: 'run-1', percent: null, stage: 'lint', stepIndex: 1, stepCount: 4, message: 'go' })
    expect(wt('wt').run).toMatchObject({ percent: null, stage: 'lint', stepIndex: 1, stepCount: 4, message: 'go' })
    slice.applyQualityPushEvent({ event: 'qualityProgress', worktreeId: 'wt', runId: 'run-1', percent: 40 })
    expect(wt('wt').run?.percent).toBe(40)
  })

  it('ignores events for a different run while one is active', async () => {
    const { slice, wt } = await running()
    slice.applyQualityPushEvent({ event: 'qualityProgress', worktreeId: 'wt', runId: 'other', percent: 90 })
    expect(wt('wt').run?.percent).toBeNull()
    slice.applyQualityPushEvent({ event: 'qualityFinished', worktreeId: 'wt', runId: 'other', success: true, error: null })
    expect(wt('wt').run?.phase).toBe('running')
  })

  it('ignores events for a worktree without state', () => {
    const { slice, state } = makeSlice(() => ok({}))
    expect(() => slice.applyQualityPushEvent({ event: 'qualityProgress', worktreeId: 'zz', runId: 'r', percent: 1 })).not.toThrow()
    expect(state().codeIntelQualityByWorktree).toEqual({})
  })

  it('attaches to a run started by another client', async () => {
    const { slice, wt } = makeSlice(() => ok({ gate: {} }))
    await slice.loadQualityGate('wt')
    slice.applyQualityPushEvent({ event: 'qualityProgress', worktreeId: 'wt', runId: 'remote', percent: 10 })
    expect(wt('wt').run).toMatchObject({ runId: 'remote', phase: 'running', percent: 10 })
  })

  it('finished maps interrupted, reloads the gate and bumps the epoch', async () => {
    const { slice, wt, call } = await running()
    await slice.loadQualityGate('wt')
    const epoch = wt('wt').epoch
    slice.applyQualityPushEvent({ event: 'qualityFinished', worktreeId: 'wt', runId: 'run-1', success: false, error: 'x', status: 'interrupted' })
    expect(wt('wt').run).toMatchObject({ phase: 'finished', status: 'interrupted' })
    expect(wt('wt').epoch).toBe(epoch + 1)
    expect(call.mock.calls.filter((c) => c[1] === 'codeIntel.quality.gate')).toHaveLength(2)
  })

  it('gateChanged reloads only the gate; changed marks it stale', async () => {
    const { slice, wt, call } = await running()
    await slice.loadQualityGate('wt')
    slice.applyQualityPushEvent({ event: 'gateChanged', worktreeId: 'wt' })
    expect(call.mock.calls.filter((c) => c[1] === 'codeIntel.quality.gate')).toHaveLength(2)
    expect(call.mock.calls.filter((c) => c[1] === 'codeIntel.quality.runs')).toHaveLength(0)
    slice.applyQualityPushEvent({ event: 'changed', worktreeId: 'wt' })
    expect(wt('wt').gate).not.toBeNull()
  })
})

describe('polling fallback', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => vi.useRealTimers())

  it('polls quality.run every 2s while no stream is open and stops at the end', async () => {
    let status = 'running'
    const call = vi.fn((_w: string, method: string) => {
      if (method.endsWith('start')) {
        return ok(runBody('running'))
      }
      if (method.endsWith('.run')) {
        return ok({ id: 'run-1', status })
      }
      return ok({})
    })
    const { slice, wt } = makeSlice(call, 'polling')
    await slice.startQualityRun('wt', { profile: 'full', scope: 'changed' })
    await vi.advanceTimersByTimeAsync(QUALITY_RUN_POLL_INTERVAL_MS)
    expect(call.mock.calls.filter((c) => c[1] === 'codeIntel.quality.run')).toHaveLength(1)
    status = 'succeeded'
    await vi.advanceTimersByTimeAsync(QUALITY_RUN_POLL_INTERVAL_MS)
    expect(wt('wt').run).toMatchObject({ phase: 'finished', status: 'succeeded' })
    const polls = call.mock.calls.length
    await vi.advanceTimersByTimeAsync(QUALITY_RUN_POLL_INTERVAL_MS * 3)
    expect(call.mock.calls.length).toBe(polls)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('does not poll quality.run while the push stream is open', async () => {
    const call = vi.fn((_w: string, method: string) => (method.endsWith('start') ? ok(runBody('running')) : ok({})))
    const { slice } = makeSlice(call, 'streaming')
    await slice.startQualityRun('wt', { profile: 'full', scope: 'changed' })
    await vi.advanceTimersByTimeAsync(QUALITY_RUN_POLL_INTERVAL_MS * 3)
    expect(call.mock.calls.filter((c) => c[1] === 'codeIntel.quality.run')).toHaveLength(0)
  })

  it('prune clears the timer of a removed worktree', async () => {
    const { slice } = makeSlice((_w, method) => (method.endsWith('start') ? ok(runBody('running')) : ok({})), 'polling')
    await slice.startQualityRun('wt', { profile: 'full', scope: 'changed' })
    expect(vi.getTimerCount()).toBe(1)
    slice.pruneCodeIntelQualityWorktrees([])
    expect(vi.getTimerCount()).toBe(0)
  })
})

describe('loads', () => {
  it('keeps the last gate (marked stale) when a reload fails', async () => {
    let n = 0
    const { slice, wt } = makeSlice(() => (n++ === 0 ? ok({ gate: { verdict: 'pass' } }) : fail('offline')))
    await slice.loadQualityGate('wt')
    expect(wt('wt').gate?.data.gate.verdict).toBe('pass')
    await slice.loadQualityGate('wt', { force: true })
    expect(wt('wt').gate).toMatchObject({ stale: true })
    expect(wt('wt').errors.gate?.kind).toBe('offline')
    expect(wt('wt').loading.gate).toBe(false)
  })

  it('drops a superseded response', async () => {
    const resolvers: ((v: unknown) => void)[] = []
    const call: QualityCall = () => new Promise((resolve) => resolvers.push((v) => resolve({ ok: true, result: v })))
    const { slice, wt } = makeSlice(call)
    const first = slice.loadQualityGate('wt')
    const second = slice.loadQualityGate('wt', { force: true })
    resolvers[1]({ gate: { verdict: 'fail' } })
    await second
    resolvers[0]({ gate: { verdict: 'pass' } })
    await first
    expect(wt('wt').gate?.data.gate.verdict).toBe('fail')
  })

  it('dedupes concurrent loads without force', async () => {
    const call = vi.fn(() => ok({ gate: {} }))
    const { slice } = makeSlice(call)
    await Promise.all([slice.loadQualityGate('wt'), slice.loadQualityGate('wt')])
    expect(call).toHaveBeenCalledTimes(1)
  })

  it('re-attaches to a running local run found in the run list', async () => {
    const { slice, wt } = makeSlice(() => ok({ runs: [{ id: 'live', status: 'running', source: 'local', profile: 'full', scope: 'changed' }] }), 'streaming')
    await slice.loadQualityRuns('wt')
    expect(wt('wt').run).toMatchObject({ runId: 'live', phase: 'running' })
  })

  it('findings per file are capped at 16 files', async () => {
    let t = 0
    const slice0 = makeSlice(() => ok({ findings: [{ ruleId: 'r' }] }))
    for (let i = 0; i < 20; i++) {
      t++
      await slice0.slice.loadQualityFindingsForFile('wt', `f${i}.ts`)
    }
    expect(Object.keys(slice0.wt('wt').findingsByFile)).toHaveLength(16)
    expect(t).toBe(20)
  })

  it('invalidate marks cached data stale and clears per-file findings', async () => {
    const { slice, wt } = makeSlice(() => ok({ gate: {}, findings: [] }))
    await slice.loadQualityGate('wt')
    await slice.loadQualityFindingsForFile('wt', 'a.ts')
    slice.invalidateQuality('wt')
    expect(wt('wt').gate?.stale).toBe(true)
    expect(wt('wt').findingsByFile).toEqual({})
  })

  it('setQualityUi merges into ui', () => {
    const { slice, wt } = makeSlice(() => ok({}))
    slice.setQualityUi('wt', { annotationsOn: false, selectedFingerprint: 'fp' })
    expect(wt('wt').ui).toMatchObject({ annotationsOn: false, selectedFingerprint: 'fp', onlyInScope: true })
  })
})
