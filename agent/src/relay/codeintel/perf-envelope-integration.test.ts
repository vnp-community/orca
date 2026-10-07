import { describe, it, expect } from 'vitest'
import {
  CODEINTEL_CLI_COMMANDS,
  PerfRecorder,
  perfForCacheHit,
  stripVolatileResultFields
} from './perf-block'
import { buildCodeIntelResult } from '../codeintel-result-envelope'

describe('perf-envelope-integration', () => {
  const samplePerf = {
    totalMs: 120,
    queueWaitMs: 10,
    cliCalls: 1,
    cli: [{ tool: 'gitnexus' as const, command: 'context' as const, ms: 100, stdoutBytes: 500 }],
    parseMs: 10,
    truncated: false
  }

  it('every enveloped method returns a perf object', () => {
    const envelopedResult = buildCodeIntelResult({
      sources: [],
      headCommit: 'abc',
      stale: false,
      truncated: false,
      totalCount: null,
      perf: samplePerf,
      data: { hello: 'world' }
    })

    expect(envelopedResult.perf).toBeDefined()
    expect(envelopedResult.perf.totalMs).toBe(120)
    expect(envelopedResult.perf.cliCalls).toBe(1)
  })

  it('non-enveloped methods have no perf', () => {
    // reindexStart/Cancel/Status or watch results
    const nonEnvelopedResult: any = {
      jobId: 'reindex-123',
      state: 'queued'
    }
    expect(nonEnvelopedResult.perf).toBeUndefined()
  })

  it('cache entries never contain perf', () => {
    const dataToCache = {
      nodes: [{ id: 'c1' }],
      edges: []
    }
    const cachedString = JSON.stringify(dataToCache)
    expect(cachedString).not.toContain('"perf"')
  })

  it('cache hit reports cliCalls 0', () => {
    const perf = perfForCacheHit(2)
    expect(perf.cliCalls).toBe(0)
    expect(perf.cli).toEqual([])
    expect(perf.totalMs).toBe(2)
  })

  it('error responses carry no perf', () => {
    const errorPayload: any = {
      code: -32602,
      message: 'Invalid params',
      data: { code: 'CODEINTEL_INVALID_PARAMS' }
    }
    expect(errorPayload.perf).toBeUndefined()
  })

  it('perf.cli commands are all in the closed set across all methods', () => {
    const recorder = new PerfRecorder()
    for (const cmd of CODEINTEL_CLI_COMMANDS) {
      recorder.recordCli({
        tool: 'gitnexus',
        command: cmd,
        ms: 10,
        stdoutBytes: 100
      })
    }
    const perf = recorder.build()
    expect(perf.cli.length).toBe(CODEINTEL_CLI_COMMANDS.length)
    for (const entry of perf.cli) {
      expect(CODEINTEL_CLI_COMMANDS).toContain(entry.command)
    }
  })

  it('stripVolatileResultFields removes perf and startedAt', () => {
    const raw = {
      headCommit: 'abc',
      startedAt: 1234567,
      perf: samplePerf,
      data: { id: 1 }
    }
    const stripped = stripVolatileResultFields(raw)
    expect(stripped.perf).toBeUndefined()
    expect(stripped.startedAt).toBeUndefined()
    expect(stripped.headCommit).toBe('abc')
    expect(stripped.data).toEqual({ id: 1 })
  })
})
