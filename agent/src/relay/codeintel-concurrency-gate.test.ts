import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createConcurrencyGate } from './codeintel-concurrency-gate'
import { CodeIntelError } from './codeintel-errors'

describe('codeintel-concurrency-gate', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('limits total concurrency', async () => {
    const gate = createConcurrencyGate({ maxTotal: 3, perTool: {}, queueMax: 10, queueWaitMs: 10000 })
    const release1 = await gate.acquire('any')
    const release2 = await gate.acquire('any')
    const release3 = await gate.acquire('any')
    
    let acquire4Resolved = false
    gate.acquire('any').then(() => { acquire4Resolved = true })
    
    await Promise.resolve() // flush microtasks
    expect(acquire4Resolved).toBe(false)
    
    release1()
    await Promise.resolve()
    expect(acquire4Resolved).toBe(true)
  })

  it('limits per tool concurrency', async () => {
    const gate = createConcurrencyGate({ maxTotal: 5, perTool: { gitnexus: 2 }, queueMax: 10, queueWaitMs: 10000 })
    const release1 = await gate.acquire('gitnexus')
    const release2 = await gate.acquire('gitnexus')
    
    let acquire3Resolved = false
    gate.acquire('gitnexus').then(() => { acquire3Resolved = true })
    
    await Promise.resolve()
    expect(acquire3Resolved).toBe(false)
    
    let otherResolved = false
    gate.acquire('other').then(() => { otherResolved = true })
    await Promise.resolve()
    expect(otherResolved).toBe(true)
    
    release1()
    await Promise.resolve()
    expect(acquire3Resolved).toBe(true)
  })

  it('rejects immediately when queue is full', async () => {
    const gate = createConcurrencyGate({ maxTotal: 1, perTool: {}, queueMax: 2, queueWaitMs: 10000 })
    await gate.acquire('t') // active = 1
    
    // push 2 to queue
    gate.acquire('t').catch(() => {})
    gate.acquire('t').catch(() => {})
    
    // 3rd push should reject immediately
    await expect(gate.acquire('t')).rejects.toThrowError('Queue full')
  })

  it('times out in queue', async () => {
    const gate = createConcurrencyGate({ maxTotal: 1, perTool: {}, queueMax: 2, queueWaitMs: 10000 })
    await gate.acquire('t') // active = 1
    
    const p = gate.acquire('t')
    vi.advanceTimersByTime(10000)
    
    await expect(p).rejects.toThrowError(CodeIntelError)
    await p.catch(err => {
      expect(err.data.reason).toBe('queue_wait')
    })
  })

  it('handles abort signal while queuing', async () => {
    const gate = createConcurrencyGate({ maxTotal: 1, perTool: {}, queueMax: 2, queueWaitMs: 10000 })
    await gate.acquire('t')
    
    const ac = new AbortController()
    const p = gate.acquire('t', ac.signal)
    
    ac.abort()
    await expect(p).rejects.toThrowError(/aborted/)
  })
})
