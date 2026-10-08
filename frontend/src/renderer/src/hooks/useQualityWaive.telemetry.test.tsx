// @vitest-environment happy-dom
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { act, renderHook } from '@testing-library/react'
import type { QualityCall } from '../store/slices/code-intel-quality-slice-context'

const clientCall = vi.fn()
const track = vi.hoisted(() => vi.fn())
vi.mock('@/lib/telemetry', () => ({ track }))
vi.mock('sonner', () => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() }
}))
vi.mock('../runtime/code-intel-client', () => ({
  getCodeIntelClient: () => ({ call: clientCall })
}))
vi.mock('@/store', async () => {
  const { createCodeIntelQualityTestStore } =
    await import('../test-support/code-intel-quality-test-store')
  return {
    useAppStore: createCodeIntelQualityTestStore((async () => ({
      ok: true,
      result: {}
    })) as QualityCall)
  }
})

import { eventSchemas } from '../../../shared/telemetry-events'
import { makeQualityFinding } from '../test-support/quality-finding-fixtures'
import { useQualityWaive } from './useQualityWaive'

beforeEach(() => {
  clientCall.mockReset()
  track.mockClear()
})

describe('useQualityWaive — quality_finding_triaged', () => {
  it('emits an enum-only event after a successful waive, never the free-text reason', async () => {
    clientCall.mockResolvedValue({ ok: true, result: { waiver: {} } })
    const { result } = renderHook(() =>
      useQualityWaive(
        'wt',
        vi.fn(() => vi.fn())
      )
    )
    const finding = makeQualityFinding({
      fingerprint: 'fp1',
      severity: 'error',
      inScope: true,
      tool: 'golangci-lint'
    })
    await act(async () => {
      await result.current.waive(finding, {
        reason: 'SECRET reason',
        expiresAt: '2026-10-20T00:00:00Z'
      })
    })
    const triaged = track.mock.calls.filter(([name]) => name === 'quality_finding_triaged')
    expect(triaged).toHaveLength(1)
    const props = triaged[0][1]
    expect(props).toMatchObject({
      action: 'waive',
      reason: 'other',
      severity: 'error',
      blocking: true
    })
    expect(eventSchemas.quality_finding_triaged.safeParse(props).success).toBe(true)
    expect(JSON.stringify(props)).not.toContain('SECRET')
  })

  it('emits revoke, maps unknown severity to info, and stays silent when the call fails', async () => {
    clientCall.mockResolvedValueOnce({ ok: true, result: {} })
    const { result } = renderHook(() =>
      useQualityWaive(
        'wt',
        vi.fn(() => vi.fn())
      )
    )
    const waived = makeQualityFinding({
      fingerprint: 'fp2',
      severity: 'mystery' as never,
      inScope: false,
      waiver: { by: 'a', reason: 'r', expiresAt: '2026-10-20T00:00:00Z' }
    })
    await act(async () => {
      await result.current.revoke(waived)
    })
    expect(track).toHaveBeenCalledWith(
      'quality_finding_triaged',
      expect.objectContaining({ action: 'revoke', severity: 'info', blocking: false })
    )
    track.mockClear()
    clientCall.mockResolvedValueOnce({
      ok: false,
      error: { kind: 'forbidden', code: null, message: '', data: null, retryable: false }
    })
    await act(async () => {
      await result.current.revoke({ ...waived, fingerprint: 'fp3' })
    })
    expect(track.mock.calls.filter(([name]) => name === 'quality_finding_triaged')).toHaveLength(0)
  })
})
