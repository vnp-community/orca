// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook } from '@testing-library/react'
import type { QualityCall } from '../store/slices/code-intel-quality-slice-context'

const call = vi.fn<QualityCall>()

vi.mock('@/store', async () => {
  const { createCodeIntelQualityTestStore } =
    await import('../test-support/code-intel-quality-test-store')
  return { useAppStore: createCodeIntelQualityTestStore((...args) => call(...args)) }
})

import { useAppStore } from '@/store'
import { useQualityFindingsForFile } from './useQualityFindingsForFile'
import { pickLatestFinishedLocalRun } from './useLatestQualityRun'

const finding = (file: string) => ({
  fingerprint: `fp-${file}`,
  ruleId: 'r',
  severity: 'warning',
  category: 'lint',
  file,
  line: 1,
  endLine: 1,
  column: 0,
  endColumn: 0,
  message: 'm',
  tool: 't',
  toolVersion: '1',
  stepId: 's',
  inScope: true
})

async function flush(): Promise<void> {
  await act(async () => {
    for (let i = 0; i < 6; i++) {
      await Promise.resolve()
    }
  })
}

beforeEach(() => {
  call.mockReset()
  call.mockImplementation((_w, _m, params) =>
    Promise.resolve({
      ok: true as const,
      result: { findings: [finding((params as { file: string }).file)] }
    })
  )
  useAppStore.setState({ codeIntelQualityByWorktree: {} })
})
afterEach(() => cleanup())

describe('useQualityFindingsForFile', () => {
  it('does not call the backend when disabled', async () => {
    const { result } = renderHook(() => useQualityFindingsForFile('wt', 'a.ts', { enabled: false }))
    await flush()
    expect(call).not.toHaveBeenCalled()
    expect(result.current).toBeNull()
  })

  it('loads a file once with file, runId and limit, then serves the cache', async () => {
    const { result, rerender } = renderHook(() =>
      useQualityFindingsForFile('wt', 'a.ts', { enabled: true, runId: 'r1' })
    )
    await flush()
    expect(call).toHaveBeenCalledTimes(1)
    expect(call.mock.calls[0][2]).toEqual({ file: 'a.ts', limit: 500, runId: 'r1' })
    expect(result.current).toHaveLength(1)
    const first = result.current
    rerender()
    expect(result.current).toBe(first)
    expect(call).toHaveBeenCalledTimes(1)
  })

  it('reloads after the cache is invalidated', async () => {
    const { result } = renderHook(() => useQualityFindingsForFile('wt', 'a.ts', { enabled: true }))
    await flush()
    act(() => useAppStore.getState().invalidateQuality('wt'))
    await flush()
    expect(call).toHaveBeenCalledTimes(2)
    expect(result.current).toHaveLength(1)
  })

  it('keeps at most 16 files per worktree', async () => {
    for (let i = 0; i < 18; i++) {
      await useAppStore.getState().loadQualityFindingsForFile('wt', `f${i}.ts`)
    }
    expect(
      Object.keys(useAppStore.getState().codeIntelQualityByWorktree.wt.findingsByFile)
    ).toHaveLength(16)
  })
})

describe('pickLatestFinishedLocalRun', () => {
  it('ignores CI and unfinished runs and picks the newest', () => {
    const run = (id: string, over: object) =>
      ({
        id,
        status: 'succeeded',
        source: 'local',
        finishedAt: '2026-10-01T00:00:00Z',
        ...over
      }) as never
    expect(
      pickLatestFinishedLocalRun([
        run('a', {}),
        run('b', { finishedAt: '2026-10-05T00:00:00Z' }),
        run('c', { source: 'ci', finishedAt: '2026-10-09T00:00:00Z' }),
        run('d', { status: 'running' })
      ])?.id
    ).toBe('b')
    expect(pickLatestFinishedLocalRun(undefined)).toBeNull()
  })
})
