/**
 * Tests for code-intel-parsers.ts (FE-CV-TASK-050-03)
 * Covers the §2.3 error table, envelope, IndexStatus and push-event parsing.
 */

import { describe, it, expect } from 'vitest'
import {
  ALL_ERROR_KINDS,
  CODE_INTEL_ERROR_CODES,
  CODE_INTEL_ERROR_KIND_BY_CODE,
  codeIntelErrorKindForCode,
  parseCodeIntelEnvelope,
  parseCodeIntelErrorMessage,
  parseCodeIntelPushEvent,
  parseIndexStatus
} from './code-intel-parsers'

describe('parseCodeIntelErrorMessage', () => {
  it('parses CODE: text | {json}', () => {
    const r = parseCodeIntelErrorMessage(
      'CODEINTEL_TIMEOUT: read timed out | {"retryAfterMs":5000,"inProgress":true}'
    )
    expect(r.code).toBe('CODEINTEL_TIMEOUT')
    expect(r.text).toBe('read timed out')
    expect(r.data).toEqual({ retryAfterMs: 5000, inProgress: true })
  })

  it('parses CODE: text without data', () => {
    const r = parseCodeIntelErrorMessage('CODEINTEL_DISABLED: code intelligence is off')
    expect(r).toEqual({ code: 'CODEINTEL_DISABLED', text: 'code intelligence is off', data: null })
  })

  it('keeps code null for messages without the prefix', () => {
    const r = parseCodeIntelErrorMessage('Something went wrong')
    expect(r).toEqual({ code: null, text: 'Something went wrong', data: null })
  })

  it('returns data: null for a JSON suffix > 2 KiB but keeps the code', () => {
    const large = JSON.stringify({ padding: 'a'.repeat(2500) })
    const r = parseCodeIntelErrorMessage(`CODEINTEL_TOOL_FAILED: boom | ${large}`)
    expect(r.code).toBe('CODEINTEL_TOOL_FAILED')
    expect(r.data).toBeNull()
  })

  it('returns data: null for malformed JSON without throwing', () => {
    const r = parseCodeIntelErrorMessage('CODEINTEL_TOOL_FAILED: boom | {not json}')
    expect(r.code).toBe('CODEINTEL_TOOL_FAILED')
    expect(r.data).toBeNull()
  })

  it('unknown CODEINTEL_* code is preserved and maps to unknown', () => {
    const r = parseCodeIntelErrorMessage('CODEINTEL_FUTURE_CODE: hi')
    expect(r.code).toBe('CODEINTEL_FUTURE_CODE')
    expect(codeIntelErrorKindForCode(r.code)).toBe('unknown')
  })

  it('does not break on a gRPC-style message', () => {
    const r = parseCodeIntelErrorMessage('rpc error: code = Unavailable desc = CODEINTEL_X: m')
    expect(r.code).toBeNull()
  })
})

describe('§2.3 code -> kind table', () => {
  // [code, kind] rows copied from the contract table
  const rows: [string, string][] = [
    ['CODEINTEL_DISABLED', 'disabled'],
    ['CODEINTEL_QUALITY_GATE_DISABLED', 'quality-disabled'],
    ['CODEINTEL_AI_REVIEW_DISABLED', 'ai-disabled'],
    ['CODEINTEL_UNAVAILABLE', 'unsupported'],
    ['CODEINTEL_NOT_AUTHORIZED', 'forbidden'],
    ['CODEINTEL_NOT_FOUND', 'not-found'],
    ['CODEINTEL_INVALID_PARAMS', 'validation'],
    ['CODEINTEL_PATH_NOT_ALLOWED', 'path-not-allowed'],
    ['CODEINTEL_WORKTREE_NOT_FOUND', 'no-binding'],
    ['CODEINTEL_WORKTREE_REF_UNSUPPORTED', 'no-binding'],
    ['CODEINTEL_NO_DEV_SERVER', 'no-binding'],
    ['CODEINTEL_DEV_SERVER_NOT_APPROVED', 'no-binding'],
    ['CODEINTEL_DEV_SERVER_MODE_UNSUPPORTED', 'no-binding'],
    ['CODEINTEL_DEV_SERVER_OFFLINE', 'offline'],
    ['CODEINTEL_TOOL_UNAVAILABLE', 'tool-unavailable'],
    ['CODEINTEL_INDEX_MISSING', 'index-missing'],
    ['CODEINTEL_REPO_NOT_REGISTERED', 'repo-not-registered'],
    ['CODEINTEL_AMBIGUOUS_SYMBOL', 'ambiguous'],
    ['CODEINTEL_SYMBOL_NOT_FOUND', 'not-found'],
    ['CODEINTEL_TIMEOUT', 'timeout'],
    ['CODEINTEL_REINDEX_IN_PROGRESS', 'reindex-in-progress'],
    ['CODEINTEL_REINDEX_COOLDOWN', 'rate-limited'],
    ['CODEINTEL_OUTPUT_TOO_LARGE', 'too-large'],
    ['CODEINTEL_RESPONSE_TOO_LARGE', 'too-large'],
    ['CODEINTEL_TOOL_FAILED', 'tool-failed'],
    ['CODEINTEL_RESULT_INVALID', 'tool-failed'],
    ['CODEINTEL_AGENT_UNSUPPORTED', 'unsupported'],
    ['CODEINTEL_RATE_LIMITED', 'rate-limited'],
    ['CODEINTEL_CONCURRENCY_LIMIT', 'rate-limited'],
    ['CODEINTEL_VERSION_CONFLICT', 'conflict'],
    ['CODEINTEL_PAYLOAD_TOO_LARGE', 'validation'],
    ['CODEINTEL_PROFILE_INVALID', 'validation'],
    ['CODEINTEL_PROFILE_UNKNOWN', 'profile-unknown'],
    ['CODEINTEL_ENV_NOT_READY', 'env-not-ready'],
    ['CODEINTEL_RUN_IN_PROGRESS', 'run-in-progress'],
    ['CODEINTEL_RUN_NOT_FOUND', 'not-found'],
    ['CODEINTEL_RUN_CANCELLED', 'run-cancelled'],
    ['CODEINTEL_WAIVER_EXPIRY_INVALID', 'validation'],
    ['CODEINTEL_AI_NO_RELAY', 'ai-error'],
    ['CODEINTEL_AI_BAD_OUTPUT', 'ai-error'],
    ['CODEINTEL_SECRET_LEAK_BLOCKED', 'tool-failed'],
    ['CODEINTEL_AUTHZ_UNAVAILABLE', 'unknown'],
    ['CODEINTEL_INTERNAL', 'unknown']
  ]

  it.each(rows)('%s -> %s', (code, kind) => {
    expect(CODE_INTEL_ERROR_KIND_BY_CODE[code]).toBe(kind)
  })

  it('every code constant is covered by the table', () => {
    const covered = new Set(rows.map(([code]) => code))
    for (const code of Object.values(CODE_INTEL_ERROR_CODES)) {
      expect(covered.has(code), code).toBe(true)
    }
  })

  it('RPC-level codes map without the CODEINTEL_ prefix', () => {
    expect(CODE_INTEL_ERROR_KIND_BY_CODE['method_not_found']).toBe('unsupported')
    expect(CODE_INTEL_ERROR_KIND_BY_CODE['forbidden']).toBe('forbidden')
    expect(CODE_INTEL_ERROR_KIND_BY_CODE['internal']).toBeUndefined()
  })

  it('all mapped kinds are valid kinds', () => {
    const valid = new Set<string>(ALL_ERROR_KINDS)
    for (const kind of Object.values(CODE_INTEL_ERROR_KIND_BY_CODE)) {
      expect(valid.has(kind), kind).toBe(true)
    }
  })
})

describe('parseCodeIntelEnvelope', () => {
  const full = {
    repo: 'orca',
    worktreeId: 'wt-1',
    view: 'structure',
    sources: [{ tool: 'gitnexus', version: '1.2', indexedAt: '2026-10-07T00:00:00Z', commit: 'abc', lineBase: 1 }],
    headCommit: 'abc',
    stale: false,
    truncated: true,
    totalCount: 5,
    etag: '"e1"',
    fromCache: true,
    generatedAt: '2026-10-07T00:00:01Z',
    nextPageToken: 'tok',
    data: [1, 2, 3]
  }

  it('parses a full envelope', () => {
    const r = parseCodeIntelEnvelope(full, (d) => d as number[])
    expect(r.data).toEqual([1, 2, 3])
    expect(r.sources).toEqual([
      { tool: 'gitnexus', version: '1.2', indexedAt: '2026-10-07T00:00:00Z', commit: 'abc', lineBase: 1 }
    ])
    expect(r.nextPageToken).toBe('tok')
    expect(r.fromCache).toBe(true)
    expect(r.etag).toBe('"e1"')
    expect(r.truncated).toBe(true)
  })

  it('drops sources with an unknown tool', () => {
    const r = parseCodeIntelEnvelope({ ...full, sources: [{ tool: 'x' }, 'str'] }, (d) => d)
    expect(r.sources).toEqual([])
  })

  it('uses safe defaults for missing fields', () => {
    const r = parseCodeIntelEnvelope({}, (d) => d)
    expect(r).toMatchObject({
      stale: false,
      truncated: false,
      totalCount: 0,
      sources: [],
      headCommit: null,
      etag: '',
      fromCache: false
    })
  })

  it('notModified: never calls parseData and has no data key', () => {
    let called = false
    const r = parseCodeIntelEnvelope({ ...full, notModified: true, data: undefined }, () => {
      called = true
      return 1
    })
    expect(r.notModified).toBe(true)
    expect('data' in r).toBe(false)
    expect(called).toBe(false)
  })

  it('throws for a non-object', () => {
    expect(() => parseCodeIntelEnvelope('string', (d) => d)).toThrow()
  })
})

describe('parseIndexStatus', () => {
  it('parses the flat status with tools and indexBasis', () => {
    const r = parseIndexStatus({
      overall: 'READY',
      tools: [
        { tool: 'gitnexus', available: true, supported: true, state: 'ready', indexScope: 'exact', freshness: 'fresh' },
        { tool: 'bogus' }
      ],
      scopeMismatch: true,
      activeJob: { id: 'j1', stage: 'parse', percent: null },
      indexBasis: [
        {
          tool: 'codegraph',
          indexScope: 'weird',
          freshness: 'fresh',
          dirtySinceIndex: true,
          changedFilesNotInIndex: 2,
          refreshState: 'running',
          indexPolicy: 'off'
        }
      ],
      binding: { id: 'b1', projectId: 'p', repoId: 'r', indexScope: 'exact', version: 3 }
    })
    expect(r.overall).toBe('READY')
    expect(r.tools).toHaveLength(1)
    expect(r.tools[0]).toMatchObject({ tool: 'gitnexus', state: 'ready' })
    expect(r.scopeMismatch).toBe(true)
    expect(r.activeJob).toEqual({ id: 'j1', stage: 'parse', percent: null })
    expect(r.indexBasis[0].indexScope).toBe('unknown')
    expect(r.indexBasis[0].refreshState).toBe('running')
    expect(r.binding).toMatchObject({ id: 'b1', indexScope: 'exact', version: 3 })
  })

  it.each(['OFFLINE', 'UNKNOWN', 'NOT_INSTALLED', 'BUILDING', 'MISSING', 'DEGRADED', 'OVERLAY', 'STALE', 'READY'])(
    'accepts overall %s',
    (overall) => {
      expect(parseIndexStatus({ overall }).overall).toBe(overall)
    }
  )

  it('unknown overall -> UNKNOWN', () => {
    expect(parseIndexStatus({ overall: 'WEIRD' }).overall).toBe('UNKNOWN')
  })

  it('lowercase overall is coerced to uppercase', () => {
    expect(parseIndexStatus({ overall: 'ready' }).overall).toBe('READY')
  })

  it('unknown tool state / scope -> unknown', () => {
    const r = parseIndexStatus({
      overall: 'READY',
      tools: [{ tool: 'codegraph', state: 'weird', indexScope: 'x', freshness: 'y' }]
    })
    expect(r.tools[0]).toMatchObject({ state: 'unknown', indexScope: 'unknown', freshness: 'unknown' })
  })

  it('null / garbage input -> safe defaults', () => {
    for (const input of [null, undefined, 'x', 3]) {
      const r = parseIndexStatus(input)
      expect(r).toEqual({ overall: 'UNKNOWN', tools: [], scopeMismatch: false, indexBasis: [] })
    }
  })
})

describe('parseCodeIntelPushEvent', () => {
  it('parses changed with a known reason', () => {
    const r = parseCodeIntelPushEvent({ event: 'changed', worktreeId: 'wt-1', reason: 'commit', resync: false })
    expect(r).toMatchObject({ event: 'changed', reason: 'commit', resync: false })
  })

  it('changed with an unknown reason -> unknown', () => {
    const r = parseCodeIntelPushEvent({ event: 'changed', worktreeId: 'wt-1', reason: 'cosmic_ray' })
    expect(r).toMatchObject({ event: 'changed', reason: 'unknown' })
  })

  it('maps contract wire reasons and flags resync', () => {
    expect(parseCodeIntelPushEvent({ event: 'changed', reason: 'index_changed' })).toMatchObject({ reason: 'reindex' })
    expect(parseCodeIntelPushEvent({ event: 'changed', reason: 'head_changed' })).toMatchObject({ reason: 'commit' })
    expect(parseCodeIntelPushEvent({ event: 'changed', reason: 'resync' })).toMatchObject({ resync: true })
    expect(parseCodeIntelPushEvent({ event: 'changed', reason: 'overflow', resync: true })).toMatchObject({
      reason: 'unknown',
      resync: true
    })
  })

  it('reindexProgress keeps percent null', () => {
    const r = parseCodeIntelPushEvent({ event: 'reindexProgress', worktreeId: 'wt-1', percent: null, running: true })
    expect(r).toMatchObject({ event: 'reindexProgress', percent: null, running: true })
  })

  it('normalizes dotted wire names (quality.progress/finished/gateChanged)', () => {
    expect(
      parseCodeIntelPushEvent({ event: 'quality.progress', worktreeId: 'w', runId: 'r', percent: 40 })
    ).toMatchObject({ event: 'qualityProgress', percent: 40 })
    expect(
      parseCodeIntelPushEvent({ event: 'quality.finished', worktreeId: 'w', runId: 'r', status: 'succeeded' })
    ).toMatchObject({ event: 'qualityFinished', success: true, error: null })
    expect(
      parseCodeIntelPushEvent({ event: 'quality.finished', worktreeId: 'w', runId: 'r', status: 'interrupted' })
    ).toMatchObject({ event: 'qualityFinished', success: false })
    expect(
      parseCodeIntelPushEvent({ event: 'quality.gateChanged', worktreeId: 'w', verdict: 'FAIL' })
    ).toMatchObject({ event: 'gateChanged', gate: 'fail' })
  })

  it('parses internal-name events and defaults unknown gate', () => {
    expect(parseCodeIntelPushEvent({ event: 'qualityFinished', worktreeId: 'w', runId: 'r', success: true, error: null }))
      .toMatchObject({ event: 'qualityFinished', success: true })
    expect(parseCodeIntelPushEvent({ event: 'gateChanged', worktreeId: 'w', gate: 'fail' })).toMatchObject({ gate: 'fail' })
    expect(parseCodeIntelPushEvent({ event: 'gateChanged', worktreeId: 'w' })).toMatchObject({ gate: 'unknown' })
  })

  it('returns null for unknown event names and non-objects', () => {
    expect(parseCodeIntelPushEvent({ event: 'unknown_event' })).toBeNull()
    expect(parseCodeIntelPushEvent('string')).toBeNull()
    expect(parseCodeIntelPushEvent(null)).toBeNull()
  })
})
