// @vitest-environment happy-dom
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { act, cleanup, fireEvent, render, renderHook, screen } from '@testing-library/react'
import type { QualityCall } from '../../../../store/slices/code-intel-quality-slice-context'

const clientCall = vi.fn()
vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() }
}))
vi.mock('../../../../runtime/code-intel-client', () => ({
  getCodeIntelClient: () => ({ call: clientCall })
}))
vi.mock('@/store', async () => {
  const { createCodeIntelQualityTestStore } =
    await import('../../../../test-support/code-intel-quality-test-store')
  return {
    useAppStore: createCodeIntelQualityTestStore((async () => ({
      ok: true,
      result: {}
    })) as QualityCall)
  }
})

import { useAppStore } from '@/store'
import { makeQualityFinding } from '../../../../test-support/quality-finding-fixtures'
import { useQualityWaive } from '../../../../hooks/useQualityWaive'
import { QualityWaivePopover } from './QualityWaivePopover'
import type { QualityFinding } from '../../../../../../shared/code-intel-quality-types'

const NOW = new Date('2026-10-07T12:00:00Z')
const err = (kind: string, data: Record<string, unknown> | null = null) =>
  Promise.resolve({
    ok: false as const,
    error: { kind, code: null, message: '', data, retryable: false }
  })

beforeEach(() => {
  clientCall.mockReset()
  useAppStore.setState({ codeIntelQualityByWorktree: {} })
})
afterEach(() => cleanup())

function hook(patch = vi.fn(() => vi.fn())) {
  return { patch, ...renderHook(() => useQualityWaive('wt', patch)) }
}

describe('useQualityWaive', () => {
  it('sends a waive with reason and expiry (no scope), patches optimistically, then invalidates and reloads the gate', async () => {
    clientCall.mockResolvedValue({ ok: true, result: { waiver: {} } })
    const { result, patch } = hook()
    const finding = makeQualityFinding({ fingerprint: 'fp1' })
    let outcome: unknown
    await act(async () => {
      outcome = await result.current.waive(finding, {
        reason: ' because ',
        expiresAt: '2026-10-20T00:00:00Z'
      })
    })
    expect(outcome).toEqual({ ok: true })
    expect(clientCall).toHaveBeenCalledWith(
      'wt',
      'codeIntel.quality.waive',
      {
        subjectKind: 'finding',
        subjectKey: 'fp1',
        action: 'waive',
        reason: 'because',
        expiresAt: '2026-10-20T00:00:00Z'
      },
      {}
    )
    expect(patch).toHaveBeenCalledWith('fp1', expect.objectContaining({ reason: 'because' }))
    expect(useAppStore.getState().codeIntelQualityByWorktree.wt.epoch).toBeGreaterThan(0)
  })

  it('restores the row and reports the reason when the call fails', async () => {
    const restore = vi.fn()
    clientCall.mockImplementation(() => err('forbidden'))
    const { result } = hook(vi.fn(() => restore))
    let outcome: { ok: boolean; message?: string } = { ok: true }
    await act(async () => {
      outcome = await result.current.waive(makeQualityFinding({}), {
        reason: 'r',
        expiresAt: '2026-10-20T00:00:00Z'
      })
    })
    expect(outcome.ok).toBe(false)
    expect(outcome.message).toBeTruthy()
    expect(restore).toHaveBeenCalled()
  })

  it('maps validation with maxDays to the expiry message', async () => {
    clientCall.mockImplementation(() => err('validation', { maxDays: 30 }))
    const { result } = hook()
    let outcome: { ok: boolean; message?: string } = { ok: true }
    await act(async () => {
      outcome = await result.current.waive(makeQualityFinding({}), {
        reason: 'r',
        expiresAt: '2026-12-20T00:00:00Z'
      })
    })
    expect(outcome.message).toContain('30')
  })

  it('rejects missing reason or expiry without calling the backend', async () => {
    const { result } = hook()
    await act(async () => {
      expect(
        (await result.current.waive(makeQualityFinding({}), { reason: '  ', expiresAt: 'x' })).ok
      ).toBe(false)
      expect(
        (await result.current.waive(makeQualityFinding({}), { reason: 'r', expiresAt: '' })).ok
      ).toBe(false)
    })
    expect(clientCall).not.toHaveBeenCalled()
  })

  it('a double submit sends one request', async () => {
    let resolve: (v: unknown) => void = () => undefined
    clientCall.mockImplementation(() => new Promise((r) => (resolve = r)))
    const { result } = hook()
    const finding = makeQualityFinding({ fingerprint: 'dup' })
    await act(async () => {
      void result.current.waive(finding, { reason: 'r', expiresAt: '2026-10-20T00:00:00Z' })
      void result.current.waive(finding, { reason: 'r', expiresAt: '2026-10-20T00:00:00Z' })
    })
    expect(clientCall).toHaveBeenCalledTimes(1)
    await act(async () => resolve({ ok: true, result: {} }))
  })

  it('revoke sends action revoke', async () => {
    clientCall.mockResolvedValue({ ok: true, result: {} })
    const { result } = hook()
    const finding: QualityFinding = makeQualityFinding({
      fingerprint: 'w',
      waiver: { by: 'u', reason: 'old', expiresAt: '2026-10-20T00:00:00Z' }
    })
    await act(async () => {
      await result.current.revoke(finding)
    })
    expect(clientCall.mock.calls[0][2]).toMatchObject({ action: 'revoke', subjectKey: 'w' })
  })
})

describe('QualityWaivePopover', () => {
  const waiveApi = (over = {}) => ({
    busyFingerprint: null,
    waive: vi.fn(async () => ({ ok: true as const })),
    revoke: vi.fn(async () => ({ ok: true as const })),
    ...over
  })

  function open(waive = waiveApi(), finding = makeQualityFinding({ fingerprint: 'fp' })) {
    render(<QualityWaivePopover finding={finding} waive={waive} now={() => NOW} />)
    fireEvent.click(screen.getByRole('button', { name: /waive/i }))
    return waive
  }

  it('requires a reason and an expiry before anything is sent', async () => {
    const waive = open()
    fireEvent.click(screen.getByRole('button', { name: /^Save|Waive finding|Submit/i }))
    expect(waive.waive).not.toHaveBeenCalled()
    expect(await screen.findByRole('alert')).toBeTruthy()
  })

  it('sends the typed reason with a 7-day expiry', async () => {
    const waive = open()
    fireEvent.change(screen.getByRole('textbox'), { target: { value: 'known false positive' } })
    fireEvent.click(screen.getByRole('radio', { name: /7/ }))
    fireEvent.click(screen.getByRole('button', { name: /^Save|Waive finding|Submit/i }))
    await vi.waitFor(() => expect(waive.waive).toHaveBeenCalledTimes(1))
    const input = (waive.waive.mock.calls[0] as unknown[])[1] as {
      reason: string
      expiresAt: string
    }
    expect(input.reason).toBe('known false positive')
    expect(new Date(input.expiresAt).getTime() - NOW.getTime()).toBeLessThanOrEqual(30 * 86_400_000)
  })

  it('shows Revoke instead of the form for an already waived finding', () => {
    const waive = waiveApi()
    render(
      <QualityWaivePopover
        finding={makeQualityFinding({
          waiver: { by: 'u', reason: 'r', expiresAt: '2026-10-20T00:00:00Z' }
        })}
        waive={waive}
        now={() => NOW}
      />
    )
    fireEvent.click(screen.getByRole('button'))
    expect(waive.revoke).toHaveBeenCalledTimes(1)
  })
})
