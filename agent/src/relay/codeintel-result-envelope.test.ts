import { describe, it, expect } from 'vitest'
import {
  buildCodeIntelResult,
  computeStale,
  PerfCollector,
  fitResultToLimit
} from './codeintel-result-envelope'
import { CodeIntelError } from './codeintel-errors'

describe('codeintel-result-envelope', () => {
  it('computeStale calculates staleness correctly', () => {
    const sources: any[] = [
      { tool: 'gitnexus', commit: 'abc', lineBase: 1 },
      { tool: 'codegraph', commit: null, lineBase: 1 }
    ]

    expect(computeStale(sources, 'abc', {})).toBe(false)
    expect(computeStale(sources, 'def', {})).toBe(true)
    expect(computeStale(sources, 'abc', { rootMismatch: true })).toBe(true)
    expect(computeStale(sources, 'abc', { worktreeMismatch: true })).toBe(true)
    expect(computeStale(sources, 'abc', { pendingChanges: 1 })).toBe(true)
  })

  it('PerfCollector collects and builds perf object', () => {
    const perf = new PerfCollector()
    perf.recordQueueWait(100)
    perf.recordCli({ tool: 'gitnexus', command: 'cypher', ms: 50, stdoutBytes: 1024 })
    perf.recordParse(10)
    perf.setTruncated()

    const res = perf.build()
    expect(res.queueWaitMs).toBe(100)
    expect(res.cliCalls).toBe(1)
    expect(res.cli[0].command).toBe('cypher')
    expect(res.parseMs).toBe(10)
    expect(res.truncated).toBe(true)
    expect(res.totalMs).toBeGreaterThanOrEqual(0)
  })

  it('PerfCollector throws on invalid command', () => {
    const perf = new PerfCollector()
    expect(() => perf.recordCli({ tool: 'gitnexus', command: 'invalid_cmd', ms: 50, stdoutBytes: 1024 }))
      .toThrow('Invalid perf command: invalid_cmd')
  })

  it('buildCodeIntelResult builds envelope', () => {
    const res = buildCodeIntelResult({
      sources: [],
      headCommit: 'abc',
      stale: false,
      truncated: false,
      totalCount: 10,
      perf: new PerfCollector().build(),
      data: { hello: 'world' }
    })

    expect(res.headCommit).toBe('abc')
    expect(res.data.hello).toBe('world')
    expect(res.warnings).toBeUndefined()
  })

  it('fitResultToLimit truncates array to fit within limit', () => {
    const data = {
      items: Array.from({ length: 100 }, (_, i) => ({ id: i, name: 'very long string to take up space' }))
    }
    const res = buildCodeIntelResult({
      sources: [],
      headCommit: 'abc',
      stale: false,
      truncated: false,
      totalCount: 100,
      perf: new PerfCollector().build(),
      data
    })

    const truncatedRes = fitResultToLimit(res, { prune: [{ path: ['items'], minKeep: 10 }] }, 500)
    expect(truncatedRes.truncated).toBe(true)
    expect(truncatedRes.data.items.length).toBeLessThan(100)
    expect(truncatedRes.data.items.length).toBeGreaterThanOrEqual(10)
  })

  it('fitResultToLimit throws if still too large after truncation', () => {
    const data = {
      items: Array.from({ length: 100 }, (_, i) => ({ id: i, name: 'very long string to take up space' }))
    }
    const res = buildCodeIntelResult({
      sources: [],
      headCommit: 'abc',
      stale: false,
      truncated: false,
      totalCount: 100,
      perf: new PerfCollector().build(),
      data
    })

    expect(() => fitResultToLimit(res, { prune: [{ path: ['items'], minKeep: 99 }] }, 500))
      .toThrowError(CodeIntelError)
  })
})
