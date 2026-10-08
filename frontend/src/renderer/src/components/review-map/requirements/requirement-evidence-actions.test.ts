import { describe, expect, it, vi } from 'vitest'
import { createRequirementEvidenceActions } from './requirement-evidence-actions'
import { traceWire } from './requirement-trace.fixture'

const ctx = { projectId: 'p', worktreeId: 'wt' }

describe('createRequirementEvidenceActions', () => {
  it('confirm sends the contract params and returns the parsed trace', async () => {
    const call = vi.fn().mockResolvedValue({ ok: true, result: { trace: traceWire() } })
    const result = await createRequirementEvidenceActions(ctx, { call }).confirm('k', { kind: 'change', ref: 'a.ts' })
    expect(call).toHaveBeenCalledWith('codeIntel.quality.trace.confirm', {
      projectId: 'p', worktreeId: 'wt', requirementKey: 'k', evidenceKind: 'change', evidenceRef: 'a.ts',
      linkKind: 'confirm', scope: 'worktree'
    })
    expect(result).toMatchObject({ ok: true, trace: { subject: { taskId: 't1' } } })
  })

  it('reject uses linkKind reject', async () => {
    const call = vi.fn().mockResolvedValue({ ok: true, result: { trace: traceWire() } })
    await createRequirementEvidenceActions(ctx, { call }).reject('k', { kind: 'test', ref: 't.ts' })
    expect(call.mock.calls[0][1]).toMatchObject({ linkKind: 'reject', evidenceKind: 'test' })
  })

  it('linkTask sends the id and unlinkTask sends an empty taskId', async () => {
    const call = vi.fn().mockResolvedValue({ ok: true, result: { trace: traceWire() } })
    const actions = createRequirementEvidenceActions(ctx, { call })
    await actions.linkTask('task-7')
    await actions.unlinkTask()
    expect(call.mock.calls[0][0]).toBe('codeIntel.quality.trace.link')
    expect(call.mock.calls[0][1]).toMatchObject({ taskId: 'task-7' })
    expect(call.mock.calls[1][1]).toMatchObject({ taskId: '' })
  })

  it('is not optimistic: failures return the error kind and no trace', async () => {
    const call = vi.fn().mockResolvedValue({ ok: false, error: { kind: 'forbidden' } })
    expect(await createRequirementEvidenceActions(ctx, { call }).confirm('k', { kind: 'change', ref: 'a' })).toEqual({
      ok: false,
      kind: 'forbidden'
    })
  })

  it('rejects a response without a trace and survives thrown errors', async () => {
    const actions = createRequirementEvidenceActions(ctx, {
      call: vi.fn().mockResolvedValueOnce({ ok: true, result: {} }).mockRejectedValueOnce(new Error('x'))
    })
    expect(await actions.linkTask('a')).toEqual({ ok: false, kind: 'invalid_response' })
    expect(await actions.linkTask('a')).toEqual({ ok: false, kind: 'unknown' })
  })

  it('ignores a duplicate write while the same one is in flight', async () => {
    let resolve: (v: unknown) => void = () => {}
    const call = vi.fn().mockReturnValue(new Promise((r) => (resolve = r)))
    const actions = createRequirementEvidenceActions(ctx, { call })
    const first = actions.confirm('k', { kind: 'change', ref: 'a' })
    const second = await actions.confirm('k', { kind: 'change', ref: 'a' })
    expect(second).toMatchObject({ ok: false, busy: true })
    expect(call).toHaveBeenCalledTimes(1)
    resolve({ ok: true, result: { trace: traceWire() } })
    await first
    await actions.confirm('k', { kind: 'change', ref: 'a' })
    expect(call).toHaveBeenCalledTimes(2)
  })
})
