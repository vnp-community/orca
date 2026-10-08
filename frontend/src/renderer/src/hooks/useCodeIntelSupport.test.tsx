// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, renderHook } from '@testing-library/react'

vi.mock('@/store', async () => {
  const { createCodeIntelTestStore } = await import('../test-support/code-intel-test-store')
  return { useAppStore: createCodeIntelTestStore() }
})

import { useAppStore } from '@/store'
import { useCodeIntelSupport } from './useCodeIntelSupport'

const settings = (effective: Record<string, boolean>) => ({ ok: true as const, result: { effective } })
const failure = (kind: string) => ({ ok: false as const, error: { kind, code: null, message: kind } })

async function flush() {
  await act(async () => {
    await Promise.resolve()
    await Promise.resolve()
  })
}

function mount(callSettings: ReturnType<typeof vi.fn>, isReviewTabOpen = true) {
  const call = callSettings as unknown as (env: string | null) => Promise<never>
  return renderHook(() =>
    useCodeIntelSupport({ worktreeId: 'wt', environmentId: 'env-1', isReviewTabOpen, callSettings: call })
  )
}

beforeEach(() => {
  vi.useFakeTimers()
  useAppStore.setState({ codeIntelSupportState: { state: 'unknown' } })
})
afterEach(() => {
  cleanup()
  vi.useRealTimers()
})

describe('useCodeIntelSupport mapping', () => {
  it('effective.codeIntelEnabled -> enabled with flags', async () => {
    const call = vi.fn().mockResolvedValue(
      settings({ codeIntelEnabled: true, qualityGateEnabled: true, aiReviewEnabled: false })
    )
    mount(call)
    await flush()
    const s = useAppStore.getState().codeIntelSupportState
    expect(s.state).toBe('enabled')
    expect(s.effective).toMatchObject({ codeIntelEnabled: true, qualityGateEnabled: true, aiReviewEnabled: false })
  })

  it('effective.codeIntelEnabled=false -> disabled', async () => {
    mount(vi.fn().mockResolvedValue(settings({ codeIntelEnabled: false })))
    await flush()
    expect(useAppStore.getState().codeIntelSupportState.state).toBe('disabled')
  })

  it.each([
    ['disabled', 'disabled'],
    ['unsupported', 'unsupported'],
    ['forbidden', 'unsupported']
  ])('error kind %s -> %s', async (kind, expected) => {
    mount(vi.fn().mockResolvedValue(failure(kind)))
    await flush()
    expect(useAppStore.getState().codeIntelSupportState.state).toBe(expected)
  })

  it.each(['offline', 'timeout', 'rate-limited', 'unknown'])(
    'transient error %s keeps the previous state (unknown on first load)',
    async (kind) => {
      const call = vi.fn().mockResolvedValue(failure(kind))
      mount(call)
      await flush()
      expect(useAppStore.getState().codeIntelSupportState.state).toBe('unknown')

      useAppStore.setState({ codeIntelSupportState: { state: 'enabled', effective: { codeIntelEnabled: true } } })
      await act(async () => {
        await vi.advanceTimersByTimeAsync(60_000)
      })
      expect(useAppStore.getState().codeIntelSupportState.state).toBe('enabled')
    }
  )
})

describe('useCodeIntelSupport polling', () => {
  it('calls once on open, then every 60 s, and stops on unmount', async () => {
    const call = vi.fn().mockResolvedValue(settings({ codeIntelEnabled: true }))
    const { unmount } = mount(call)
    await flush()
    expect(call).toHaveBeenCalledTimes(1)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000)
    })
    expect(call).toHaveBeenCalledTimes(2)
    unmount()
    expect(vi.getTimerCount()).toBe(0)
    await vi.advanceTimersByTimeAsync(120_000)
    expect(call).toHaveBeenCalledTimes(2)
  })

  it('does nothing while the review tab is closed', async () => {
    const call = vi.fn().mockResolvedValue(settings({ codeIntelEnabled: true }))
    mount(call, false)
    await flush()
    expect(call).not.toHaveBeenCalled()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('only ever calls the injected settings channel', async () => {
    const call = vi.fn().mockResolvedValue(settings({ codeIntelEnabled: false }))
    mount(call)
    await flush()
    expect(call).toHaveBeenCalledWith('env-1')
  })
})
