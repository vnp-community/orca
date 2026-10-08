// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook } from '@testing-library/react'

vi.mock('@/store', async () => {
  const { createCodeIntelTestStore } = await import('../test-support/code-intel-test-store')
  return { useAppStore: createCodeIntelTestStore() }
})
vi.mock('@/lib/code-intel-worktree-selector', () => ({
  useCodeIntelSelector: () => ({ state: 'ready', worktreeId: 'wt', projectId: 'p', environmentId: null })
}))

import { useAppStore } from '@/store'
import { useCodeIntelFindings } from './useCodeIntelFindings'
import { useFindingDismissal } from './useFindingDismissal'
import type { CodeIntelCallFn, CodeIntelCallOutcome } from './useCodeIntelQuery'
import { FINDINGS_MIXED, makeFinding } from '../test-support/contract-findings-fixtures'

const page = (
  findings: unknown[],
  next?: string,
  dismissedCount = 0
): CodeIntelCallOutcome => ({
  ok: true,
  result: { findings, dismissedCount, indexFreshness: { state: 'fresh', dirtyFiles: 0, unindexedFiles: [], generatedAt: 'x' } },
  meta: { nextPageToken: next } as never
})

async function flush(): Promise<void> {
  await act(async () => {
    await Promise.resolve()
    await Promise.resolve()
    await Promise.resolve()
  })
}

beforeEach(() => {
  useAppStore.setState({
    codeIntelSupportState: { state: 'enabled', effective: { codeIntelEnabled: true } },
    codeIntelWorktreeState: {},
    codeIntelResyncCounter: 0
  })
})
afterEach(() => cleanup())

describe('useCodeIntelFindings', () => {
  it('sends scope=changed by default and exposes dismissedCount / indexFreshness', async () => {
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(page(FINDINGS_MIXED, undefined, 3))
    const { result } = renderHook(() => useCodeIntelFindings('wt', null, {}, {}, call))
    await flush()
    expect(call.mock.calls[0][1]).toBe('codeIntel.findings')
    expect(call.mock.calls[0][2]).toMatchObject({ scope: 'changed', limit: 100 })
    expect(result.current.findings).toHaveLength(4)
    expect(result.current.dismissedCount).toBe(3)
    expect(result.current.indexFreshness?.state).toBe('fresh')
  })

  it('paginates with the opaque token and does not duplicate findingKeys', async () => {
    const call = vi
      .fn<CodeIntelCallFn>()
      .mockResolvedValueOnce(page([FINDINGS_MIXED[0], FINDINGS_MIXED[1]], 'tok'))
      .mockResolvedValueOnce(page([FINDINGS_MIXED[1], FINDINGS_MIXED[2]]))
    const { result } = renderHook(() => useCodeIntelFindings('wt', null, {}, {}, call))
    await flush()
    expect(result.current.hasNextPage).toBe(true)
    act(() => result.current.loadMore())
    await flush()
    expect(call.mock.calls[1][2]).toMatchObject({ pageToken: 'tok' })
    expect(result.current.findings.map((f) => f.findingKey)).toEqual([
      'layer_violation:a->b',
      'hotspot:order.go',
      'dead:util'
    ])
    expect(result.current.hasNextPage).toBe(false)
  })

  it('refetches with includeDismissed and severities when filters change', async () => {
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue(page([]))
    const { rerender } = renderHook(
      ({ f }) => useCodeIntelFindings('wt', null, f, {}, call),
      { initialProps: { f: {} as Parameters<typeof useCodeIntelFindings>[2] } }
    )
    await flush()
    rerender({ f: { includeDismissed: true, severities: ['error'] } })
    await flush()
    expect(call.mock.calls[1][2]).toMatchObject({ includeDismissed: true, severities: ['error'] })
  })

  it('does not call any channel when support is disabled', async () => {
    useAppStore.setState({ codeIntelSupportState: { state: 'disabled', effective: null } as never })
    const call = vi.fn<CodeIntelCallFn>()
    renderHook(() => useCodeIntelFindings('wt', null, {}, {}, call))
    await flush()
    expect(call).not.toHaveBeenCalled()
  })
})

describe('useFindingDismissal', () => {
  function setup(dismissOutcome: CodeIntelCallOutcome | (() => Promise<CodeIntelCallOutcome>)) {
    const call = vi.fn<CodeIntelCallFn>(async (_wt, method) => {
      if (method === 'codeIntel.dismissFinding') {
        return typeof dismissOutcome === 'function' ? dismissOutcome() : dismissOutcome
      }
      return page([makeFinding()])
    })
    const hook = renderHook(() => {
      const findings = useCodeIntelFindings('wt', null, {}, {}, call)
      const dismissal = useFindingDismissal({
        worktreeId: 'wt',
        environmentId: null,
        setOverride: findings.setOverride,
        clearOverride: findings.clearOverride,
        reload: findings.reload,
        callFn: call
      })
      return { findings, dismissal }
    })
    return { call, hook }
  }

  it('dismiss is optimistic, sends reason/note/disposition and no extra args', async () => {
    let release!: (o: CodeIntelCallOutcome) => void
    const { call, hook } = setup(() => new Promise((r) => (release = r)))
    await flush()
    let promise!: Promise<unknown>
    act(() => {
      promise = hook.result.current.dismissal.dismiss(makeFinding(), { reason: 'false_positive', note: 'n' })
    })
    expect(hook.result.current.dismissal.pendingKeys.has('layer_violation:a->b')).toBe(true)
    expect(hook.result.current.findings.findings[0].dismissed?.disposition).toBe('ignored')
    expect(hook.result.current.findings.dismissedCount).toBe(1)
    release({ ok: true, result: { findingKey: 'x', dismissed: true } })
    await act(async () => {
      await promise
    })
    const sent = call.mock.calls.find((c) => c[1] === 'codeIntel.dismissFinding')![2]
    expect(sent).toEqual({
      findingKey: 'layer_violation:a->b',
      action: 'dismiss',
      disposition: 'ignored',
      reason: 'false_positive',
      note: 'n'
    })
    expect(hook.result.current.dismissal.pendingKeys.size).toBe(0)
  })

  it('refuses an empty reason without calling the backend', async () => {
    const { call, hook } = setup({ ok: true, result: {} })
    await flush()
    let res: unknown
    await act(async () => {
      res = await hook.result.current.dismissal.dismiss(makeFinding(), { reason: '   ' })
    })
    expect(res).toMatchObject({ ok: false, error: { kind: 'validation' } })
    expect(call.mock.calls.some((c) => c[1] === 'codeIntel.dismissFinding')).toBe(false)
  })

  it('reverts and keeps the error per row on forbidden', async () => {
    const { hook } = setup({
      ok: false,
      error: { kind: 'forbidden', code: 'CODEINTEL_NOT_AUTHORIZED', message: 'no', retryable: false }
    })
    await flush()
    await act(async () => {
      await hook.result.current.dismissal.resolve(makeFinding())
    })
    expect(hook.result.current.findings.findings[0].dismissed).toBeUndefined()
    expect(hook.result.current.dismissal.errorsByKey['layer_violation:a->b'].kind).toBe('forbidden')
  })

  it('restore sends action restore and clears the dismissal', async () => {
    const dismissed = makeFinding({
      dismissed: { by: 'u', at: 'x', reason: 'later', disposition: 'ignored' }
    })
    const call = vi.fn<CodeIntelCallFn>(async (_wt, method) =>
      method === 'codeIntel.dismissFinding' ? { ok: true, result: {} } : page([dismissed], undefined, 1)
    )
    const hook = renderHook(() => {
      const findings = useCodeIntelFindings('wt', null, { includeDismissed: true }, {}, call)
      const dismissal = useFindingDismissal({
        worktreeId: 'wt',
        environmentId: null,
        setOverride: findings.setOverride,
        clearOverride: findings.clearOverride,
        reload: findings.reload,
        callFn: call
      })
      return { findings, dismissal }
    })
    await flush()
    await act(async () => {
      await hook.result.current.dismissal.restore(dismissed)
    })
    expect(call.mock.calls.find((c) => c[1] === 'codeIntel.dismissFinding')![2]).toEqual({
      findingKey: 'layer_violation:a->b',
      action: 'restore'
    })
    expect(hook.result.current.findings.findings[0].dismissed).toBeUndefined()
    expect(hook.result.current.findings.dismissedCount).toBe(0)
  })

  it('reloads on not-found / conflict', async () => {
    const { call, hook } = setup({
      ok: false,
      error: { kind: 'not-found', code: 'CODEINTEL_NOT_FOUND', message: 'gone', retryable: false }
    })
    await flush()
    const before = call.mock.calls.length
    await act(async () => {
      await hook.result.current.dismissal.restore(makeFinding())
    })
    await flush()
    expect(call.mock.calls.filter((c) => c[1] === 'codeIntel.findings').length).toBeGreaterThan(
      before > 0 ? 1 : 0
    )
  })
})
