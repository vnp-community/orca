// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cleanup, renderHook } from '@testing-library/react'

const h = vi.hoisted(() => ({ args: [] as unknown[], available: true, env: 'dev' }))
vi.mock('@/store', () => ({
  useAppStore: (sel: (s: unknown) => unknown) =>
    sel({ reviewUiByWorktree: { wt: { storageEnv: h.env } } })
}))
vi.mock('../../../hooks/useCodeIntelStorage', () => ({
  useCodeIntelStorage: (args: unknown) => {
    h.args.push(args)
    return { available: h.available }
  }
}))

import { resetReviewLensAvailability, useUnavailableReviewLenses } from './review-lens-availability'
import { useStorageLensProbe } from './use-storage-lens-probe'

beforeEach(() => {
  h.args.length = 0
  h.available = true
  h.env = 'dev'
  resetReviewLensAvailability()
})
afterEach(cleanup)

describe('useStorageLensProbe', () => {
  it('hides the Storage tab before it is opened when the channel is unsupported', () => {
    h.available = false
    renderHook(() => useStorageLensProbe('wt', 'env1', true))
    expect(renderHook(() => useUnavailableReviewLenses('wt')).result.current.has('storage')).toBe(
      true
    )
    // Same params as the lens's first load, so the lens reuses the cached answer.
    expect(h.args[0]).toEqual({
      worktreeId: 'wt',
      environmentId: 'env1',
      env: 'dev',
      enabled: true
    })
  })

  it('leaves the tab alone when supported or while disabled', () => {
    renderHook(() => useStorageLensProbe('wt', null, true))
    h.available = false
    renderHook(() => useStorageLensProbe('wt', null, false))
    expect(renderHook(() => useUnavailableReviewLenses('wt')).result.current.size).toBe(0)
    expect(h.args.at(-1)).toMatchObject({ enabled: false })
  })

  it('probes with the environment the lens will open on', () => {
    h.env = 'prod'
    renderHook(() => useStorageLensProbe('wt', null, true))
    expect(h.args[0]).toMatchObject({ env: 'prod' })
  })
})
