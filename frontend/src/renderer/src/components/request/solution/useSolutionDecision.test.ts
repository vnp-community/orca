// @vitest-environment happy-dom
import { act, renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { useSolutionDecision } from './useSolutionDecision'
import type { SolutionDecisionActions } from './useSolutionDecision'
import type { Approval, Solution } from '../../../../../shared/request-types'

const solution: Solution = { id: 's1', requestId: 'r1', kind: 'solution', status: 'ready', options: [{ id: 'o1', title: 'A' }, { id: 'o2', title: 'B' }] }
const approval = { id: 'ap1', requestId: 'r1', version: 4, subjectDigest: 'old', status: 'pending' } as Approval
const ok = (value: unknown = {}) => ({ ok: true as const, value })
const err = (code: string, kind = 'unknown') => ({ ok: false as const, error: { kind, code, message: `${code}: x` } as never })

function setup(over: Partial<SolutionDecisionActions> = {}, sol = solution) {
  const actions: SolutionDecisionActions = {
    choose: vi.fn().mockResolvedValue(ok({ approvalDigest: 'fresh' })),
    generate: vi.fn().mockResolvedValue(ok()),
    approve: vi.fn().mockResolvedValue(ok()),
    reject: vi.fn().mockResolvedValue(ok()),
    ...over
  }
  const onSettled = vi.fn()
  const hook = renderHook(() => useSolutionDecision({ solution: sol, approval, actions, onSettled }))
  return { actions, onSettled, ...hook }
}

describe('useSolutionDecision', () => {
  it('chooses then approves with the fresh digest and approval version', async () => {
    const { result, actions, onSettled } = setup()
    await act(() => result.current.approveSelected('o2'))
    expect(actions.choose).toHaveBeenCalledWith({ solutionId: 's1', optionId: 'o2' })
    const arg = (actions.approve as ReturnType<typeof vi.fn>).mock.calls[0][0]
    expect(arg.approval).toMatchObject({ id: 'ap1', version: 4, subjectDigest: 'fresh' })
    expect(onSettled).toHaveBeenCalled()
  })

  it('skips choose when the option is already chosen', async () => {
    const { result, actions } = setup({}, { ...solution, chosenOptionId: 'o1' })
    await act(() => result.current.approveSelected('o1'))
    expect(actions.choose).not.toHaveBeenCalled()
    expect(actions.approve).toHaveBeenCalled()
  })

  it('retry after a failed approve does not call choose again', async () => {
    const approve = vi.fn().mockResolvedValueOnce(err('X', 'network')).mockResolvedValueOnce(ok())
    const { result, actions } = setup({ approve })
    await act(() => result.current.approveSelected('o2'))
    expect(result.current.chosenButNotApproved).toBe(true)
    expect(result.current.failure?.errorClass).toBe('network')
    await act(() => result.current.approveSelected('o2'))
    expect(actions.choose).toHaveBeenCalledTimes(1)
    expect(approve).toHaveBeenCalledTimes(2)
    expect(result.current.chosenButNotApproved).toBe(false)
  })

  it('conflict refetches and flags; not-approver is remembered per approval', async () => {
    const approve = vi.fn().mockResolvedValueOnce(err('APPROVAL_VERSION_CONFLICT', 'conflict')).mockResolvedValueOnce(err('APPROVAL_NOT_APPROVER', 'unknown'))
    const { result, onSettled } = setup({ approve })
    await act(() => result.current.approveSelected('o1'))
    expect(result.current.conflicted).toBe(true)
    expect(onSettled).toHaveBeenCalled()
    await act(() => result.current.approveSelected('o1'))
    expect(result.current.isDenied('ap1')).toBe(true)
  })

  it('reject + regenerate sends feedback clamped to 2000', async () => {
    const { result, actions } = setup()
    const comment = 'x'.repeat(2500)
    await act(() => result.current.reject(comment, true))
    expect(actions.reject).toHaveBeenCalledWith({ approval, comment })
    const gen = (actions.generate as ReturnType<typeof vi.fn>).mock.calls[0][0]
    expect([...gen.feedback]).toHaveLength(2000)
    expect(gen.idempotencyKey).toBeTruthy()
  })

  it('failed reject does not regenerate', async () => {
    const { result, actions } = setup({ reject: vi.fn().mockResolvedValue(err('APPROVAL_COMMENT_REQUIRED', 'validation')) })
    const out = await act(() => result.current.reject('long enough reason', true))
    expect(out.ok).toBe(false)
    expect(actions.generate).not.toHaveBeenCalled()
  })

  it('ignores a second submit while one is in flight', async () => {
    let release: (v: unknown) => void = () => {}
    const approve = vi.fn().mockReturnValue(new Promise((r) => { release = r }))
    const { result } = setup({ approve }, { ...solution, kind: 'diagnosis' })
    let first: Promise<unknown>
    act(() => { first = result.current.approveSelected() })
    await act(async () => { await result.current.approveSelected() })
    expect(approve).toHaveBeenCalledTimes(1)
    await act(async () => { release(ok()); await first })
  })

  it('forwards the rationale to choose and stops before approve for a high-risk Decision', async () => {
    const decision = { id: 'd1', status: 'chosen', risk_level: 'high', version: 2 }
    const { result, actions } = setup({ choose: vi.fn().mockResolvedValue(ok({ approvalDigest: 'fresh', decision })) })
    let outcome: Awaited<ReturnType<typeof result.current.approveSelected>> | undefined
    await act(async () => { outcome = await result.current.approveSelected('o2', undefined, { rationale: '  because reasons  ' }) })
    expect(actions.choose).toHaveBeenCalledWith({ solutionId: 's1', optionId: 'o2', rationale: 'because reasons' })
    expect(outcome).toMatchObject({ ok: true, needsConfirmation: true, decision: { id: 'd1', riskLevel: 'high' } })
    expect(actions.approve).not.toHaveBeenCalled()
    // After confirmation the retry must not call choose again and uses the remembered digest.
    await act(() => result.current.approveSelected('o2'))
    expect(actions.choose).toHaveBeenCalledTimes(1)
    expect((actions.approve as ReturnType<typeof vi.fn>).mock.calls[0][0].approval.subjectDigest).toBe('fresh')
  })

  it('approves straight away for a normal-risk Decision', async () => {
    const { result, actions } = setup({ choose: vi.fn().mockResolvedValue(ok({ decision: { id: 'd', status: 'chosen', risk_level: 'normal' } })) })
    await act(() => result.current.approveSelected('o2'))
    expect(actions.approve).toHaveBeenCalled()
  })
})
