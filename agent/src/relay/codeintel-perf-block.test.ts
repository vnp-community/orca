import { describe, it, expect } from 'vitest'
import { PerfRecorder, perfForCacheHit } from './codeintel-perf-block'

describe('codeintel-perf-block', () => {
  it('records queue wait, cli calls and parse time', () => {
    let currentTime = 1000
    const nowFn = () => currentTime
    
    const recorder = new PerfRecorder(nowFn)
    recorder.recordQueueWait(50)
    recorder.recordParse(15)
    
    recorder.recordCli({ tool: 'gitnexus', command: 'cypher', ms: 200, stdoutBytes: 1024 })
    recorder.recordCli({ tool: 'codegraph', command: 'files', ms: 50, stdoutBytes: 128, rssPeakKb: 4096 })
    
    currentTime = 1300
    
    const perf = recorder.build()
    expect(perf.totalMs).toBe(300)
    expect(perf.queueWaitMs).toBe(50)
    expect(perf.parseMs).toBe(15)
    expect(perf.cliCalls).toBe(2)
    expect(perf.cli[0]).toEqual({ tool: 'gitnexus', command: 'cypher', ms: 200, stdoutBytes: 1024 })
    expect(perf.cli[1]).toEqual({ tool: 'codegraph', command: 'files', ms: 50, stdoutBytes: 128, rssPeakKb: 4096 })
    expect(perf.truncated).toBe(false)
  })

  it('rejects a command outside the closed set', () => {
    const recorder = new PerfRecorder()
    expect(() => recorder.recordCli({ tool: 'gitnexus', command: 'invalid-cmd', ms: 10, stdoutBytes: 0 }))
      .toThrow(/Invalid perf command: invalid-cmd/)
  })

  it('perfForCacheHit has cliCalls 0 and empty cli', () => {
    const perf = perfForCacheHit()
    expect(perf.cliCalls).toBe(0)
    expect(perf.cli).toEqual([])
    expect(perf.totalMs).toBe(0)
  })

  it('JSON.stringify(perf) contains no path-like or symbol-like strings', () => {
    const recorder = new PerfRecorder()
    recorder.recordCli({ tool: 'gitnexus', command: 'cypher', ms: 10, stdoutBytes: 100 })
    const perf = recorder.build()
    const str = JSON.stringify(perf)
    expect(str).not.toMatch(/\//)
    expect(str).not.toMatch(/\\/)
    // Just verifying it basically looks like a bunch of numbers and whitelisted strings
    expect(Object.keys(perf)).toEqual(['totalMs', 'queueWaitMs', 'cliCalls', 'cli', 'parseMs', 'truncated'])
  })

  it('build is idempotent and does not alias internal arrays', () => {
    const recorder = new PerfRecorder()
    recorder.recordCli({ tool: 'gitnexus', command: 'query', ms: 10, stdoutBytes: 100 })
    
    const p1 = recorder.build()
    const p2 = recorder.build()
    
    expect(p1).toEqual(p2)
    expect(p1).not.toBe(p2)
    expect(p1.cli).not.toBe(p2.cli)
  })
})
