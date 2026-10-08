import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createAgentTurnRecordQueue } from './agent-turn-record-queue'
import type { AgentTurnRecordParams } from './agent-turn-record-params'

function params(id = 't1'): AgentTurnRecordParams {
  return {
    projectId: 'p', worktreeId: 'w', clientTurnId: id, agentType: 'claude', endedAt: 'x',
    endHeadCommit: 'h', treeDirtyEnd: false, filesChangedCount: 0, filesDigest: 'd', promptDigest: 'd'
  }
}

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

describe('createAgentTurnRecordQueue', () => {
  it('sends once for duplicate enqueues, including after success', async () => {
    const send = vi.fn().mockResolvedValue(undefined)
    const q = createAgentTurnRecordQueue(send, () => ({ kind: 'unknown' }))
    q.enqueue(params())
    q.enqueue(params())
    await vi.advanceTimersByTimeAsync(0)
    q.enqueue(params())
    expect(send).toHaveBeenCalledTimes(1)
    expect(q.pending()).toBe(0)
  })

  it('retries transient errors at 2s, 6s, 18s then gives up', async () => {
    const send = vi.fn().mockRejectedValue(new Error('x'))
    const q = createAgentTurnRecordQueue(send, () => ({ kind: 'offline' }))
    q.enqueue(params())
    await vi.advanceTimersByTimeAsync(0)
    expect(send).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(2000)
    expect(send).toHaveBeenCalledTimes(2)
    await vi.advanceTimersByTimeAsync(6000)
    expect(send).toHaveBeenCalledTimes(3)
    await vi.advanceTimersByTimeAsync(18000)
    expect(send).toHaveBeenCalledTimes(4)
    await vi.advanceTimersByTimeAsync(60000)
    expect(send).toHaveBeenCalledTimes(4)
    expect(q.pending()).toBe(0)
  })

  it.each(['disabled', 'forbidden', 'unsupported', 'validation', 'quality-disabled'])(
    'does not retry permanent kind %s',
    async (kind) => {
      const send = vi.fn().mockRejectedValue(new Error('x'))
      const q = createAgentTurnRecordQueue(send, () => ({ kind }))
      q.enqueue(params())
      await vi.advanceTimersByTimeAsync(60000)
      expect(send).toHaveBeenCalledTimes(1)
      expect(q.pending()).toBe(0)
    }
  )

  it('succeeds on a later retry', async () => {
    const send = vi.fn().mockRejectedValueOnce(new Error('x')).mockResolvedValue(undefined)
    const q = createAgentTurnRecordQueue(send, () => ({ kind: 'unknown' }))
    q.enqueue(params())
    await vi.advanceTimersByTimeAsync(2100)
    expect(send).toHaveBeenCalledTimes(2)
    expect(q.pending()).toBe(0)
  })

  it('never throws into the caller, even when the classifier throws', async () => {
    const send = vi.fn().mockRejectedValue(new Error('x'))
    const q = createAgentTurnRecordQueue(send, () => {
      throw new Error('classifier')
    })
    expect(() => q.enqueue(params())).not.toThrow()
    await vi.advanceTimersByTimeAsync(0)
  })

  it('dispose cancels pending retries', async () => {
    const send = vi.fn().mockRejectedValue(new Error('x'))
    const q = createAgentTurnRecordQueue(send, () => ({ kind: 'offline' }))
    q.enqueue(params())
    await vi.advanceTimersByTimeAsync(0)
    q.dispose()
    await vi.advanceTimersByTimeAsync(60000)
    expect(send).toHaveBeenCalledTimes(1)
    q.enqueue(params('t2'))
    expect(send).toHaveBeenCalledTimes(1)
  })
})
