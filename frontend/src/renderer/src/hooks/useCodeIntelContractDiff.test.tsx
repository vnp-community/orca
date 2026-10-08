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
import { parseContractDiff, useCodeIntelContractDiff } from './useCodeIntelContractDiff'
import type { CodeIntelCallFn } from './useCodeIntelQuery'
import { CONTRACT_DIFF_MIXED } from '../test-support/contract-findings-fixtures'

async function flush(): Promise<void> {
  await act(async () => {
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

describe('useCodeIntelContractDiff', () => {
  it('sends kinds and detail and returns the parsed diff', async () => {
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue({ ok: true, result: CONTRACT_DIFF_MIXED, meta: null })
    const { result } = renderHook(() =>
      useCodeIntelContractDiff('wt', null, { kinds: ['proto', 'route'], detail: 'summary' }, call)
    )
    await flush()
    expect(call.mock.calls[0][1]).toBe('codeIntel.contractDiff')
    expect(call.mock.calls[0][2]).toEqual({ detail: 'summary', kinds: ['proto', 'route'] })
    expect(result.current.diff?.summary.breaking).toBe(2)
  })

  it('surfaces errors for inline display', async () => {
    const call = vi.fn<CodeIntelCallFn>().mockResolvedValue({
      ok: false,
      error: { kind: 'index-missing', code: 'CODEINTEL_INDEX_MISSING', message: 'x', retryable: false }
    })
    const { result } = renderHook(() => useCodeIntelContractDiff('wt', null, {}, call))
    await flush()
    expect(result.current.status).toBe('error')
    expect(result.current.error?.kind).toBe('index-missing')
  })

  it('tolerates missing arrays on the wire', () => {
    const diff = parseContractDiff({ changes: [{ id: 'a', kind: 'route', name: 'n' }] })
    expect(diff.changes[0].details).toEqual({})
    expect(diff.changes[0].consumers).toEqual([])
    expect(diff.migrations).toEqual([])
    expect(diff.totalCount).toBe(1)
  })
})
