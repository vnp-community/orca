import { describe, it, expect } from 'vitest'
import { PerfRecorder, perfForCacheHit } from './perf-block'

describe('perf-block and PerfRecorder', () => {
  it('records queue wait, cli calls and parse time', () => {
    let mockTime = 1000
    const clock = () => mockTime
    const recorder = new PerfRecorder(clock)

    recorder.recordQueueWait(15.4)
    recorder.recordParse(5.8)
    recorder.recordCli({
      tool: 'gitnexus',
      command: 'cypher',
      ms: 50.2,
      stdoutBytes: 1024.7,
      rssPeakKb: 4096
    })

    mockTime = 1200
    const perf = recorder.build()

    expect(perf.totalMs).toBe(200)
    expect(perf.queueWaitMs).toBe(15)
    expect(perf.parseMs).toBe(6)
    expect(perf.cliCalls).toBe(1)
    expect(perf.cli[0]).toEqual({
      tool: 'gitnexus',
      command: 'cypher',
      ms: 50,
      stdoutBytes: 1025,
      rssPeakKb: 4096
    })
    expect(perf.truncated).toBe(false)
  })

  it('rejects a command outside the closed set', () => {
    const recorder = new PerfRecorder()
    expect(() => {
      recorder.recordCli({
        tool: 'gitnexus',
        command: 'invalid_command' as any,
        ms: 10,
        stdoutBytes: 100
      })
    }).toThrow(/Invalid CodeIntel CLI command/)
  })

  it('perfForCacheHit has cliCalls 0 and empty cli', () => {
    const perf = perfForCacheHit(5)
    expect(perf.cliCalls).toBe(0)
    expect(perf.cli).toEqual([])
    expect(perf.queueWaitMs).toBe(0)
    expect(perf.parseMs).toBe(0)
    expect(perf.totalMs).toBe(5)
    expect(perf.truncated).toBe(false)
  })

  it('JSON.stringify(perf) contains no path-like or symbol-like strings', () => {
    const recorder = new PerfRecorder()
    recorder.recordCli({
      tool: 'codegraph',
      command: 'query',
      ms: 20,
      stdoutBytes: 500
    })
    const jsonStr = JSON.stringify(recorder.build())

    // Contains no path separators or suspicious symbols
    expect(jsonStr).not.toContain('/')
    expect(jsonStr).not.toContain('\\')
    expect(jsonStr).not.toContain('Symbol')
  })

  it('build is idempotent and does not alias internal arrays', () => {
    const recorder = new PerfRecorder()
    recorder.recordCli({
      tool: 'gitnexus',
      command: 'status',
      ms: 10,
      stdoutBytes: 50
    })

    const perf1 = recorder.build()
    const perf2 = recorder.build()

    expect(perf1.cli).not.toBe(perf2.cli)
    perf1.cli.push({ tool: 'gitnexus', command: 'files', ms: 1, stdoutBytes: 1 })
    expect(recorder.build().cli.length).toBe(1)
  })
})
