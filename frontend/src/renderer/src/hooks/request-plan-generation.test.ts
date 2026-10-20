import { beforeEach, describe, expect, it, vi } from 'vitest'

const callRequestRpc = vi.fn()
vi.mock('../runtime/request-rpc-client', () => ({
  callRequestRpc: (...a: unknown[]) => callRequestRpc(...a)
}))

import {
  PLAN_RUN_POLL_INTERVAL_MS,
  PLAN_RUN_OPENSPEC_TIMEOUT_MS,
  PLAN_RUN_TIMEOUT_MS,
  generatePlanPropose,
  generatePlanProposeCommit
} from './request-plan-generation'

const ok = (value: unknown) => ({ ok: true, value })
const netFail = { ok: false, error: { kind: 'network', code: 'NETWORK_ERROR', message: 'offline' } }
const PROPOSAL = { title: 'Plan', tasks: [{ title: 'a', taskType: 'task' }] }

function script(steps: Record<string, unknown[]>) {
  callRequestRpc.mockImplementation(async (method: string, params: { mode?: string }) => {
    const key = method === 'request.generatePlan' ? `${method}:${params.mode}` : method
    const queue = steps[key]
    if (!queue || queue.length === 0) {
      throw new Error(`unexpected call ${key}`)
    }
    return queue.length > 1 ? queue.shift() : queue[0]
  })
}

const instantSleep = () => Promise.resolve()

beforeEach(() => {
  callRequestRpc.mockReset()
})

describe('generatePlanProposeCommit', () => {
  it('starts a background run, polls request.planProposal and commits the proposal it returns', async () => {
    script({
      'request.generatePlan:propose': [ok({ runId: 'run1', proposal: null, rawAiResponse: '' })],
      'request.planProposal': [
        ok({ runId: 'run1', status: 'running', proposal: null }),
        ok({ runId: 'run1', status: 'succeeded', proposal: PROPOSAL, rawAiResponse: 'raw' })
      ],
      'request.generatePlan:commit': [ok({ planTaskId: 'p1', alreadyExists: false })]
    })
    const sleep = vi.fn(instantSleep)
    const res = await generatePlanProposeCommit('r1', { sleep })
    expect(res).toEqual({ ok: true, value: { planTaskId: 'p1', alreadyExists: false } })
    expect(sleep).toHaveBeenCalledWith(PLAN_RUN_POLL_INTERVAL_MS)
    expect(callRequestRpc).toHaveBeenCalledWith('request.planProposal', { id: 'r1', runId: 'run1' })
    expect(callRequestRpc).toHaveBeenLastCalledWith(
      'request.generatePlan',
      { id: 'r1', mode: 'commit', proposal: PROPOSAL, rawAiResponse: 'raw' },
      { idempotent: true }
    )
  })

  it('passes feedback to the propose call only', async () => {
    script({
      'request.generatePlan:propose': [ok({ runId: 'run1', proposal: null })],
      'request.planProposal': [ok({ status: 'succeeded', proposal: PROPOSAL })],
      'request.generatePlan:commit': [ok({})]
    })
    await generatePlanProposeCommit('r1', { sleep: instantSleep, feedback: 'smaller steps' })
    expect(callRequestRpc.mock.calls[0][1]).toEqual({
      id: 'r1',
      mode: 'propose',
      feedback: 'smaller steps'
    })
  })

  it('surfaces a failed run as a classified error with its backend code and never commits', async () => {
    script({
      'request.generatePlan:propose': [ok({ runId: 'run1', proposal: null })],
      'request.planProposal': [
        ok({
          status: 'failed',
          errorCode: 'REQUEST_AI_NOT_CONFIGURED',
          errorMessage: 'log in',
          proposal: null
        })
      ]
    })
    const res = await generatePlanProposeCommit('r1', { sleep: instantSleep })
    expect(res.ok).toBe(false)
    if (res.ok) {
      return
    }
    expect(res.error).toMatchObject({ kind: 'no_ai', backendCode: 'REQUEST_AI_NOT_CONFIGURED' })
    expect(
      callRequestRpc.mock.calls.some(([, p]) => (p as { mode?: string }).mode === 'commit')
    ).toBe(false)
  })

  it('commits straight away when an older server returns the proposal in the propose answer', async () => {
    script({
      'request.generatePlan:propose': [ok({ proposal: PROPOSAL })],
      'request.generatePlan:commit': [ok({ planTaskId: 'p1' })]
    })
    const res = await generatePlanProposeCommit('r1', { sleep: instantSleep })
    expect(res.ok).toBe(true)
    expect(callRequestRpc.mock.calls.map(([m]) => m)).toEqual([
      'request.generatePlan',
      'request.generatePlan'
    ])
  })

  it('stops after propose when a plan already exists, and returns a propose error unchanged', async () => {
    script({ 'request.generatePlan:propose': [ok({ alreadyExists: true })] })
    expect(await generatePlanProposeCommit('r1', { sleep: instantSleep })).toEqual({
      ok: true,
      value: { alreadyExists: true }
    })
    callRequestRpc.mockReset()
    const denied = {
      ok: false,
      error: { kind: 'invalid_state', code: 'x', message: 'REQUEST_PLAN_SOLUTION_NOT_APPROVED: no' }
    }
    script({ 'request.generatePlan:propose': [denied] })
    expect(await generatePlanProposeCommit('r1', { sleep: instantSleep })).toBe(denied)
  })

  it('tolerates a short network blip while polling but gives up after three in a row', async () => {
    script({
      'request.generatePlan:propose': [ok({ runId: 'run1', proposal: null })],
      'request.planProposal': [netFail, ok({ status: 'succeeded', proposal: PROPOSAL })],
      'request.generatePlan:commit': [ok({})]
    })
    expect((await generatePlanProposeCommit('r1', { sleep: instantSleep })).ok).toBe(true)
    callRequestRpc.mockReset()
    script({
      'request.generatePlan:propose': [ok({ runId: 'run1', proposal: null })],
      'request.planProposal': [netFail]
    })
    const res = await generatePlanProposeCommit('r1', { sleep: instantSleep })
    expect(res).toMatchObject({ ok: false, error: { kind: 'network' } })
    expect(callRequestRpc.mock.calls.filter(([m]) => m === 'request.planProposal')).toHaveLength(3)
  })

  it('times out with REQUEST_AI_COMPLETE_TIMEOUT when the run never finishes', async () => {
    script({
      'request.generatePlan:propose': [ok({ runId: 'run1', proposal: null })],
      'request.planProposal': [ok({ status: 'running', proposal: null })]
    })
    let clock = 0
    const sleep = async (ms: number) => {
      clock += ms
    }
    const res = await generatePlanProposeCommit('r1', { sleep, now: () => clock })
    expect(res).toMatchObject({
      ok: false,
      error: { kind: 'unavailable', backendCode: 'REQUEST_AI_COMPLETE_TIMEOUT' }
    })
    expect(clock).toBeGreaterThanOrEqual(PLAN_RUN_TIMEOUT_MS)
  })

  it('waits as long as an agent-authored OpenSpec change may take, and tells the caller the engine', async () => {
    script({
      'request.generatePlan:propose': [ok({ runId: 'run1', proposal: null })],
      'request.planProposal': [ok({ status: 'running', engine: 'openspec', proposal: null })]
    })
    let clock = 0
    const sleep = async (ms: number) => {
      clock += ms
    }
    const engines: string[] = []
    const res = await generatePlanProposeCommit('r1', {
      sleep,
      now: () => clock,
      onEngine: (e) => engines.push(e)
    })
    expect(res).toMatchObject({ ok: false, error: { backendCode: 'REQUEST_AI_COMPLETE_TIMEOUT' } })
    expect(clock).toBeGreaterThanOrEqual(PLAN_RUN_OPENSPEC_TIMEOUT_MS)
    expect(clock).toBeLessThan(PLAN_RUN_OPENSPEC_TIMEOUT_MS + 10_000)
    expect(engines).toEqual(['openspec'])
  })

  it('reports a succeeded run without a proposal instead of committing nothing', async () => {
    script({
      'request.generatePlan:propose': [ok({ runId: 'run1', proposal: null })],
      'request.planProposal': [ok({ status: 'succeeded', proposal: null })]
    })
    const res = await generatePlanProposeCommit('r1', { sleep: instantSleep })
    expect(res).toMatchObject({ ok: false, error: { backendCode: 'REQUEST_PLAN_AI_INVALID_JSON' } })
  })
})

describe('generatePlanPropose', () => {
  it('starts the run with feedback and a key and does not commit or poll', async () => {
    callRequestRpc.mockResolvedValue(ok({ runId: 'run9', proposal: null }))
    const res = await generatePlanPropose('r1', { feedback: 'split it', idempotencyKey: 'k1' })
    expect(res).toEqual({ ok: true, value: { runId: 'run9' } })
    expect(callRequestRpc).toHaveBeenCalledTimes(1)
    expect(callRequestRpc).toHaveBeenCalledWith(
      'request.generatePlan',
      { id: 'r1', mode: 'propose', feedback: 'split it', idempotencyKey: 'k1' },
      { idempotent: true }
    )
  })

  it('leaves an empty feedback and a missing key out of the payload', async () => {
    callRequestRpc.mockResolvedValue(ok({ runId: 'run9' }))
    await generatePlanPropose('r1', { feedback: '' })
    expect(callRequestRpc).toHaveBeenCalledWith('request.generatePlan', {
      id: 'r1',
      mode: 'propose'
    })
  })

  it('answers an empty run id when an older server returns none', async () => {
    callRequestRpc.mockResolvedValue(ok({ proposal: { title: 'x' } }))
    expect(await generatePlanPropose('r1', {})).toEqual({ ok: true, value: { runId: '' } })
  })

  it('returns the failure as it is', async () => {
    callRequestRpc.mockResolvedValue(netFail)
    expect(await generatePlanPropose('r1', { feedback: 'x' })).toEqual(netFail)
  })
})

describe('generatePlanProposeCommit with a key', () => {
  it('sends the key on the propose call only', async () => {
    script({
      'request.generatePlan:propose': [
        ok({ runId: 'run1', proposal: PROPOSAL, rawAiResponse: '' })
      ],
      'request.generatePlan:commit': [ok({ planTaskId: 'p1' })]
    })
    await generatePlanProposeCommit('r1', { feedback: 'f', idempotencyKey: 'k' })
    expect(callRequestRpc).toHaveBeenNthCalledWith(
      1,
      'request.generatePlan',
      { id: 'r1', mode: 'propose', feedback: 'f', idempotencyKey: 'k' },
      { idempotent: true }
    )
    const commit = callRequestRpc.mock.calls[1][1] as Record<string, unknown>
    expect(commit).not.toHaveProperty('idempotencyKey')
  })
})
