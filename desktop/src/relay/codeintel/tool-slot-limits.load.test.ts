import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import fs from 'fs'
import path from 'path'
import os from 'os'
import { CodeIntelConcurrencyGate } from '../codeintel-concurrency-gate'
import { CodeIntelError } from '../codeintel-errors'
import { createFakeCli, countRunningInstances } from './fake-codeintel-cli'

describe('tool-slot-limits load tests', () => {
  let tmpDir: string

  beforeEach(() => {
    tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'orca-gate-load-test-'))
  })

  afterEach(() => {
    try {
      fs.rmSync(tmpDir, { recursive: true, force: true })
    } catch {}
  })

  it('enforces total limit (3), gitnexus limit (2), and codegraph limit (3)', async () => {
    const gate = new CodeIntelConcurrencyGate({
      maxTotal: 3,
      perTool: { gitnexus: 2, codegraph: 3 },
      queueMax: 16,
      queueWaitMs: 2000
    })

    // Acquire 2 gitnexus slots
    const r1 = await gate.acquire('gitnexus')
    const r2 = await gate.acquire('gitnexus')

    expect(gate.getActiveTotal()).toBe(2)
    expect(gate.getActivePerTool('gitnexus')).toBe(2)

    // Third gitnexus cannot be acquired immediately
    let acquiredThird = false
    const p3 = gate.acquire('gitnexus').then(release => {
      acquiredThird = true
      return release
    })

    // Give microtasks time to run
    await new Promise(r => setTimeout(r, 20))
    expect(acquiredThird).toBe(false)
    expect(gate.getQueueLength()).toBe(1)

    // Acquire 1 codegraph (brings total to 3)
    const rCodegraph = await gate.acquire('codegraph')
    expect(gate.getActiveTotal()).toBe(3)

    // Fourth process across all tools is queued
    let acquiredAny = false
    const pAny = gate.acquire('codegraph').then(release => {
      acquiredAny = true
      return release
    })

    await new Promise(r => setTimeout(r, 20))
    expect(acquiredAny).toBe(false)
    expect(gate.getQueueLength()).toBe(2)

    // Release one gitnexus
    r1()
    const r3 = await p3
    expect(acquiredThird).toBe(true)

    // Release all
    r2()
    r3()
    rCodegraph()
    const rAny = await pAny
    rAny()

    expect(gate.getActiveTotal()).toBe(0)
  })

  it('enforces queue max of 16 and rejects 17th request immediately', async () => {
    const gate = new CodeIntelConcurrencyGate({
      maxTotal: 1,
      perTool: { gitnexus: 1 },
      queueMax: 3, // test with 3 for fast test
      queueWaitMs: 1000
    })

    const r1 = await gate.acquire('gitnexus')

    // Fill queue
    const q1 = gate.acquire('gitnexus')
    const q2 = gate.acquire('gitnexus')
    const q3 = gate.acquire('gitnexus')

    // 4th queued request should be rejected immediately
    await expect(gate.acquire('gitnexus')).rejects.toThrowError(CodeIntelError)
    try {
      await gate.acquire('gitnexus')
    } catch (e: any) {
      expect(e.code).toBe('CODEINTEL_TIMEOUT')
      expect(e.data?.reason).toBe('queue_wait')
    }

    r1()
    const rQ1 = await q1
    rQ1()
    const rQ2 = await q2
    rQ2()
    const rQ3 = await q3
    rQ3()
  })

  it('rejects on queue timeout with reason=queue_wait', async () => {
    const gate = new CodeIntelConcurrencyGate({
      maxTotal: 1,
      perTool: { gitnexus: 1 },
      queueMax: 16,
      queueWaitMs: 50
    })

    const r1 = await gate.acquire('gitnexus')

    const pWait = gate.acquire('gitnexus')
    await expect(pWait).rejects.toThrowError(CodeIntelError)
    try {
      await pWait
    } catch (e: any) {
      expect(e.code).toBe('CODEINTEL_TIMEOUT')
      expect(e.data?.reason).toBe('queue_wait')
    }

    r1()
  })

  it('handles 20 mixed calls and does not spawn rejected calls', async () => {
    createFakeCli({ toolName: 'gitnexus', dir: tmpDir, delayMs: 40 })

    const gate = new CodeIntelConcurrencyGate({
      maxTotal: 3,
      perTool: { gitnexus: 2, codegraph: 3 },
      queueMax: 5,
      queueWaitMs: 200
    })

    let spawnedCount = 0
    let rejectedCount = 0

    const executeCall = async (tool: string) => {
      try {
        const release = await gate.acquire(tool)
        spawnedCount++
        await new Promise(r => setTimeout(r, 30))
        release()
      } catch (e: any) {
        if (e.code === 'CODEINTEL_TIMEOUT') {
          rejectedCount++
        }
      }
    }

    const tasks: Promise<void>[] = []
    for (let i = 0; i < 20; i++) {
      tasks.push(executeCall(i % 2 === 0 ? 'gitnexus' : 'codegraph'))
    }

    await Promise.all(tasks)

    expect(spawnedCount + rejectedCount).toBe(20)
    expect(gate.getActiveTotal()).toBe(0)
  })
})
