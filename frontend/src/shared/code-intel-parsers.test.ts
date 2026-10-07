/**
 * Tests for code-intel-parsers.ts (FE-CV-TASK-050-03)
 */

import { describe, it, expect } from 'vitest'
import {
  parseCodeIntelErrorMessage,
  parseCodeIntelEnvelope,
  parseIndexStatus,
  parseCodeIntelPushEvent,
  CODE_INTEL_ERROR_CODES,
  CODE_INTEL_ERROR_KIND_BY_CODE,
  ALL_ERROR_KINDS
} from './code-intel-parsers'

describe('parseCodeIntelErrorMessage', () => {
  it('parses text only message', () => {
    const r = parseCodeIntelErrorMessage('Something went wrong')
    expect(r.code).toBeNull()
    expect(r.text).toBe('Something went wrong')
    expect(r.data).toBeNull()
  })

  it('parses text + JSON suffix', () => {
    const r = parseCodeIntelErrorMessage('Index not ready {"code":"CODEINTEL_INDEX_NOT_READY"}')
    expect(r.code).toBe('CODEINTEL_INDEX_NOT_READY')
    expect(r.text).toBe('Index not ready')
  })

  it('returns data:null for JSON suffix > 2KiB', () => {
    const large = JSON.stringify({ code: 'X', padding: 'a'.repeat(2500) })
    const r = parseCodeIntelErrorMessage(`msg ${large}`)
    expect(r.data).toBeNull()
  })

  it('handles malformed JSON gracefully', () => {
    const r = parseCodeIntelErrorMessage('err {not valid json}')
    expect(r.data).toBeNull()
    expect(r.code).toBeNull()
  })

  it('unknown CODEINTEL_* code preserved in data.code', () => {
    const r = parseCodeIntelErrorMessage('oops {"code":"CODEINTEL_FUTURE_CODE"}')
    expect(r.code).toBe('CODEINTEL_FUTURE_CODE')
  })
})

describe('CODE_INTEL_ERROR_KIND_BY_CODE mapping', () => {
  it('DISABLED maps to disabled', () => {
    expect(CODE_INTEL_ERROR_KIND_BY_CODE[CODE_INTEL_ERROR_CODES.DISABLED]).toBe('disabled')
  })

  it('method_not_found maps to unsupported', () => {
    expect(CODE_INTEL_ERROR_KIND_BY_CODE['method_not_found']).toBe('unsupported')
  })

  it('forbidden maps to forbidden', () => {
    expect(CODE_INTEL_ERROR_KIND_BY_CODE['forbidden']).toBe('forbidden')
  })

  it('all defined kinds are valid CodeIntelErrorKind values', () => {
    const validKinds = new Set(ALL_ERROR_KINDS)
    for (const kind of Object.values(CODE_INTEL_ERROR_KIND_BY_CODE)) {
      expect(validKinds.has(kind)).toBe(true)
    }
  })
})

describe('parseCodeIntelEnvelope', () => {
  it('parses a full envelope', () => {
    const raw = {
      worktreeId: 'wt-1', view: 'main', sources: ['src/'],
      headCommit: 'abc', stale: false, truncated: false,
      totalCount: 5, etag: 'e1', data: [1, 2, 3]
    }
    const result = parseCodeIntelEnvelope(raw, (d) => d as number[])
    expect(result.worktreeId).toBe('wt-1')
    expect(result.data).toEqual([1, 2, 3])
    expect(result.totalCount).toBe(5)
  })

  it('uses safe defaults for missing fields', () => {
    const result = parseCodeIntelEnvelope({}, (d) => d)
    expect(result.stale).toBe(false)
    expect(result.truncated).toBe(false)
    expect(result.totalCount).toBe(0)
    expect(result.sources).toEqual([])
    expect(result.headCommit).toBeNull()
  })

  it('notModified: data is undefined', () => {
    const raw = { worktreeId: 'wt-1', view: 'v', sources: [], headCommit: null, stale: false, truncated: false, totalCount: 0, etag: null, notModified: true }
    const result = parseCodeIntelEnvelope(raw, () => 'should not call' as unknown)
    expect(result.notModified).toBe(true)
    expect(result.data).toBeUndefined()
  })

  it('throws for non-object raw', () => {
    expect(() => parseCodeIntelEnvelope('string', (d) => d)).toThrow()
  })
})

describe('parseIndexStatus', () => {
  it('parses valid status', () => {
    const raw = {
      worktreeId: 'wt-1', overall: 'READY', lastIndexedAt: '2024-01-01T00:00:00Z',
      fileCoverage: 0.9, linesIndexed: 1000, running: false, percent: null, error: null
    }
    const result = parseIndexStatus(raw)
    expect(result.overall).toBe('READY')
    expect(result.fileCoverage).toBe(0.9)
  })

  it('unknown overall value → UNKNOWN', () => {
    const result = parseIndexStatus({ overall: 'WEIRD' })
    expect(result.overall).toBe('UNKNOWN')
  })

  it('lowercase overall → coerced to uppercase READY', () => {
    const result = parseIndexStatus({ overall: 'ready' })
    expect(result.overall).toBe('READY')
  })

  it('null/missing input → safe defaults', () => {
    const result = parseIndexStatus(null)
    expect(result.overall).toBe('UNKNOWN')
    expect(result.running).toBe(false)
    expect(result.percent).toBeNull()
  })
})

describe('parseCodeIntelPushEvent', () => {
  it('parses changed event', () => {
    const r = parseCodeIntelPushEvent({ event: 'changed', worktreeId: 'wt-1', reason: 'commit', resync: false })
    expect(r?.event).toBe('changed')
  })

  it('changed with unknown reason → unknown', () => {
    const r = parseCodeIntelPushEvent({ event: 'changed', worktreeId: 'wt-1', reason: 'cosmic_ray', resync: false }) as never
    expect((r as { reason: string }).reason).toBe('unknown')
  })

  it('parses reindexProgress with null percent', () => {
    const r = parseCodeIntelPushEvent({ event: 'reindexProgress', worktreeId: 'wt-1', percent: null, running: true })
    expect(r?.event).toBe('reindexProgress')
  })

  it('parses qualityFinished', () => {
    const r = parseCodeIntelPushEvent({ event: 'qualityFinished', worktreeId: 'wt-1', runId: 'r-1', success: true, error: null })
    expect(r?.event).toBe('qualityFinished')
  })

  it('parses gateChanged', () => {
    const r = parseCodeIntelPushEvent({ event: 'gateChanged', worktreeId: 'wt-1', gate: 'fail' })
    expect(r?.event).toBe('gateChanged')
  })

  it('returns null for unknown event name', () => {
    expect(parseCodeIntelPushEvent({ event: 'unknown_event' })).toBeNull()
  })

  it('returns null for non-object input', () => {
    expect(parseCodeIntelPushEvent('string')).toBeNull()
    expect(parseCodeIntelPushEvent(null)).toBeNull()
  })
})
