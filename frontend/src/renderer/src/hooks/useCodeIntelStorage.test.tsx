// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook } from '@testing-library/react'

vi.mock('@/store', async () => {
  const { createCodeIntelTestStore } = await import('../test-support/code-intel-test-store')
  return { useAppStore: createCodeIntelTestStore() }
})
vi.mock('@/lib/code-intel-worktree-selector', () => ({
  useCodeIntelSelector: () => ({
    state: 'ready',
    worktreeId: 'wt',
    projectId: 'p',
    environmentId: null
  })
}))

import { useAppStore } from '@/store'
import { isStorageLensAvailable, useCodeIntelStorage } from './useCodeIntelStorage'
import type { CodeIntelCallFn, CodeIntelCallOutcome } from './useCodeIntelQuery'
import { sampleStorageMap } from '../components/review-map/storage/storage-map.fixture'

const ok = (result: unknown): CodeIntelCallOutcome => ({ ok: true, result, meta: null })
async function flush() {
  await act(async () => {
    for (let i = 0; i < 6; i++) {
      await Promise.resolve()
    }
  })
}
function setSupport(state: 'enabled' | 'disabled') {
  useAppStore.setState({
    codeIntelSupportState: { state, effective: { codeIntelEnabled: state === 'enabled' } } as never
  })
}

beforeEach(() => {
  setSupport('enabled')
  useAppStore.setState({ codeIntelWorktreeState: {}, codeIntelResyncCounter: 0 })
})
afterEach(cleanup)

describe('useCodeIntelStorage', () => {
  it('defaults to dev and drops the stale response when env changes', async () => {
    let releaseDev: (o: CodeIntelCallOutcome) => void = () => undefined
    const call = vi.fn<CodeIntelCallFn>((_w, _m, p) =>
      p.env === 'dev'
        ? new Promise((r) => (releaseDev = r))
        : Promise.resolve(ok(sampleStorageMap({ asOfCommit: 'prod' })))
    )
    const { result, rerender } = renderHook(
      (p: { env?: 'dev' | 'prod' }) =>
        useCodeIntelStorage({ worktreeId: 'wt', environmentId: null, ...p, callFn: call }),
      {
        initialProps: {} as { env?: 'dev' | 'prod' }
      }
    )
    await flush()
    expect(call.mock.calls[0][2]).toEqual({ env: 'dev' })
    rerender({ env: 'prod' })
    await flush()
    expect(result.current.data?.asOfCommit).toBe('prod')
    await act(async () => releaseDev(ok(sampleStorageMap({ asOfCommit: 'dev' }))))
    await flush()
    expect(result.current.data?.asOfCommit).toBe('prod')
  })

  it('sends includeLegacy only when asked', async () => {
    const call = vi.fn<CodeIntelCallFn>(async () => ok(sampleStorageMap()))
    renderHook(() =>
      useCodeIntelStorage({
        worktreeId: 'wt',
        environmentId: null,
        includeLegacy: true,
        callFn: call
      })
    )
    await flush()
    expect(call.mock.calls[0][2]).toEqual({ env: 'dev', includeLegacy: true })
  })

  it('marks the lens unavailable on unsupported errors', async () => {
    const call = vi.fn<CodeIntelCallFn>(async () => ({
      ok: false,
      error: { kind: 'unsupported', code: 'CODEINTEL_UNAVAILABLE', message: 'x', retryable: false }
    }))
    const { result } = renderHook(() =>
      useCodeIntelStorage({ worktreeId: 'wt', environmentId: null, callFn: call })
    )
    await flush()
    expect(result.current.available).toBe(false)
    expect(isStorageLensAvailable('timeout')).toBe(true)
    expect(isStorageLensAvailable('disabled')).toBe(false)
  })

  it('makes no call when the feature is off', async () => {
    const call = vi.fn<CodeIntelCallFn>()
    setSupport('disabled')
    renderHook(() => useCodeIntelStorage({ worktreeId: 'wt', environmentId: null, callFn: call }))
    await flush()
    expect(call).not.toHaveBeenCalled()
  })

  it('makes no call when the hook is disabled', async () => {
    const call = vi.fn<CodeIntelCallFn>()
    renderHook(() =>
      useCodeIntelStorage({ worktreeId: 'wt', environmentId: null, enabled: false, callFn: call })
    )
    await flush()
    expect(call).not.toHaveBeenCalled()
  })
})
