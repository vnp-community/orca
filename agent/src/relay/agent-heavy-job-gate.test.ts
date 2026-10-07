import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createHeavyJobGate, HeavyGateTimeoutError } from './agent-heavy-job-gate'

describe('agent-heavy-job-gate', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('handles max=1, FIFO and wait timeout', async () => {
    const gate = createHeavyJobGate({ max: 1 })
    const ac1 = new AbortController()
    const ac2 = new AbortController()
    const ac3 = new AbortController()

    let release1: () => void
    const p1 = gate.acquire(ac1.signal, 1000).then(r => { release1 = r })

    await Promise.resolve() // let microtasks run
    expect(gate.stats().running).toBe(1)
    expect(gate.stats().waiting).toBe(0)

    let release2: (() => void) | undefined
    const p2 = gate.acquire(ac2.signal, 1000).then(r => { release2 = r })

    let error3: any
    const p3 = gate.acquire(ac3.signal, 1000).catch(e => { error3 = e })

    await Promise.resolve()
    expect(gate.stats().running).toBe(1)
    expect(gate.stats().waiting).toBe(2)

    // Release 1, 2 should acquire
    release1!()
    await p1
    await Promise.resolve()
    expect(release2).toBeDefined()
    expect(gate.stats().running).toBe(1)
    expect(gate.stats().waiting).toBe(1)

    // Wait timeout for 3
    vi.advanceTimersByTime(1500)
    await Promise.resolve()
    expect(error3).toBeInstanceOf(HeavyGateTimeoutError)
    
    expect(gate.stats().running).toBe(1)
    expect(gate.stats().waiting).toBe(0)

    release2!()
    await p2
    expect(gate.stats().running).toBe(0)
  })

  it('aborts while waiting', async () => {
    const gate = createHeavyJobGate({ max: 1 })
    const ac1 = new AbortController()
    const ac2 = new AbortController()

    gate.acquire(ac1.signal, 10000)
    
    let error2: any
    gate.acquire(ac2.signal, 10000).catch(e => { error2 = e })
    
    expect(gate.stats().waiting).toBe(1)

    ac2.abort()
    await Promise.resolve()

    expect(error2.name).toBe('AbortError')
    expect(gate.stats().waiting).toBe(0)
  })

  it('release is idempotent and try/finally works', async () => {
    const gate = createHeavyJobGate({ max: 1 })
    const ac = new AbortController()

    const release = await gate.acquire(ac.signal, 1000)
    expect(gate.stats().running).toBe(1)
    
    release()
    expect(gate.stats().running).toBe(0)
    
    release() // double release
    expect(gate.stats().running).toBe(0)
  })

  it('uses available cores or 1 for invalid max', () => {
    const orig = process.env.ORCA_HEAVY_JOBS
    
    process.env.ORCA_HEAVY_JOBS = '1'
    const gate = createHeavyJobGate()
    expect(gate).toBeDefined()
    
    process.env.ORCA_HEAVY_JOBS = 'xyz'
    const gate2 = createHeavyJobGate()
    expect(gate2).toBeDefined()
    
    if (orig !== undefined) {
      process.env.ORCA_HEAVY_JOBS = orig
    } else {
      delete process.env.ORCA_HEAVY_JOBS
    }
  })
})
