// @vitest-environment happy-dom
import { cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({ callRequestRpc: (...a: unknown[]) => callRequestRpc(...a) }))

beforeEach(() => {
  callRequestRpc.mockReset()
})
afterEach(cleanup)
const ok = (value: unknown) => ({ ok: true, value })
const fail = (kind: string, message = 'm') => ({ ok: false, error: { kind, code: 'x', message } })
import { useImpactAssessment } from './useImpactAssessment'

const summary = { assessment_id: 'a1', digest: 'dg', level: 'high', status: 'ready' }
const params = { requestId: 'r1', subjectType: 'solution_option', subjectId: 's1', solutionId: 'sol' }

function routes(map: Record<string, unknown>) {
  callRequestRpc.mockImplementation(async (method: string) => map[method] ?? fail('unsupported'))
}

describe('useImpactAssessment', () => {
  it('loads summary, findings and comparison', async () => {
    routes({
      'impact.get': ok(summary),
      'impact.findings': ok({ findings: [{ id: 'f1', level: 'high', title: 'T' }] }),
      'impact.compare': ok({ comparison: [{ option_id: 'o1', dimensions: {} }] })
    })
    const { result } = renderHook(() => useImpactAssessment(params))
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(result.current.summary?.level).toBe('high')
    expect(result.current.findings).toHaveLength(1)
    expect(result.current.comparison).toHaveLength(1)
    expect(callRequestRpc).toHaveBeenCalledWith('impact.findings', { assessmentId: 'a1', minLevel: 'medium' })
  })

  it.each([
    ['unsupported', 'unsupported'], ['forbidden', 'forbidden'], ['no_dev_server', 'noDevServer'], ['pending', 'collecting'], ['network', 'error']
  ])('maps %s to status %s', async (kind, expected) => {
    routes({ 'impact.get': fail(kind) })
    const { result } = renderHook(() => useImpactAssessment(params))
    await waitFor(() => expect(result.current.status).toBe(expected))
  })

  it('guards accept (10) and override (20) in the client without calling the network', async () => {
    routes({ 'impact.get': ok(summary), 'impact.findings': ok({ findings: [] }) })
    const { result } = renderHook(() => useImpactAssessment(params))
    await waitFor(() => expect(result.current.status).toBe('ready'))
    callRequestRpc.mockClear()
    expect(await result.current.acceptRisk({ findingId: 'f', rationale: '  short ' })).toMatchObject({ ok: false, error: { kind: 'validation' } })
    expect(await result.current.override({ gate: 'plan', reason: 'x'.repeat(19) })).toMatchObject({ ok: false })
    expect(callRequestRpc).not.toHaveBeenCalled()
    routes({ 'impact.accept': ok({}), 'risk.override': ok({}) })
    await result.current.acceptRisk({ findingId: 'f', rationale: 'long enough rationale' })
    expect(callRequestRpc).toHaveBeenCalledWith('impact.accept', { assessmentId: 'a1', findingId: 'f', rationale: 'long enough rationale', assessmentDigest: 'dg' })
    await result.current.override({ gate: 'plan', reason: 'x'.repeat(20) })
    expect(callRequestRpc).toHaveBeenCalledWith('risk.override', { requestId: 'r1', gate: 'plan', reason: 'x'.repeat(20) })
  })

  it('an empty answer means not assessed (ready, no summary)', async () => {
    routes({ 'impact.get': ok({}) })
    const { result } = renderHook(() => useImpactAssessment(params))
    await waitFor(() => expect(result.current.status).toBe('ready'))
    expect(result.current.summary).toBeNull()
  })
})
