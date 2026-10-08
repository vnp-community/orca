import { describe, expect, it } from 'vitest'
import {
  classifyReviewError,
  normalizeChangeOverlay,
  normalizeIndexStatus,
  normalizeReviewState
} from './review-shell-data'

describe('normalizeIndexStatus', () => {
  it('unknown overall never becomes READY; percent null stays null', () => {
    expect(normalizeIndexStatus({ overall: 'WEIRD' }).overall).toBe('UNKNOWN')
    expect(normalizeIndexStatus(null).overall).toBe('UNKNOWN')
    expect(
      normalizeIndexStatus({
        overall: 'BUILDING',
        activeJob: { id: 'j', stage: 's', percent: null }
      }).activeJob?.percent
    ).toBeNull()
  })
  it('missing arrays become empty', () => {
    expect(normalizeIndexStatus({ overall: 'READY' })).toMatchObject({
      tools: [],
      indexBasis: [],
      scopeMismatch: false
    })
  })
})

describe('normalizeChangeOverlay', () => {
  it('fills empty arrays when the backend sent a summary-only payload', () => {
    const o = normalizeChangeOverlay({
      scope: { baseRef: 'm', mode: 'worktree', includesUncommitted: true },
      risk: { level: 'HIGH', incomplete: true }
    })
    expect(o.changedFiles).toEqual([])
    expect(o.limits).toEqual({ truncated: {}, totalCounts: {} })
    expect(o.risk).toMatchObject({ level: 'HIGH', incomplete: true, reasons: [] })
  })
  it('unwraps the envelope', () => {
    expect(
      normalizeChangeOverlay({ data: { changedFiles: [{ path: 'a', status: 'added' }] } })
        .changedFiles
    ).toHaveLength(1)
  })
  it('no risk -> null', () => {
    expect(normalizeChangeOverlay({}).risk).toBeNull()
  })
})

describe('normalizeReviewState', () => {
  it('missing record becomes version 0 with empty progress for the requested pair', () => {
    expect(normalizeReviewState({}, 'b', 'h')).toMatchObject({
      baseCommit: 'b',
      headCommit: 'h',
      version: 0,
      readingProgress: { entries: {}, lastFocusedKey: null }
    })
  })
  it('drops malformed entries but keeps valid ones and unknown fields (notes) verbatim', () => {
    const s = normalizeReviewState(
      {
        version: 3,
        notes: { anchors: {} },
        readingProgress: {
          entries: { a: { state: 'seen', at: 1 }, b: { state: 'x', at: 1 }, c: 'no' },
          lastFocusedKey: 'a'
        }
      },
      'b',
      'h'
    )
    expect(s.readingProgress.entries).toEqual({ a: { state: 'seen', at: 1 } })
    expect(s.notes).toEqual({ anchors: {} })
    expect(s.version).toBe(3)
  })
})

describe('classifyReviewError', () => {
  it.each([
    ['CODEINTEL_NO_DEV_SERVER: x', 'no-binding'],
    ['CODEINTEL_WORKTREE_NOT_FOUND: x', 'no-binding'],
    ['CODEINTEL_INDEX_MISSING: x | {"tool":"gitnexus"}', 'index-missing'],
    ['CODEINTEL_TOOL_UNAVAILABLE: x', 'tool-unavailable'],
    ['CODEINTEL_REPO_NOT_REGISTERED: x', 'repo-not-registered'],
    ['CODEINTEL_PATH_NOT_ALLOWED: x', 'path-not-allowed'],
    ['CODEINTEL_REINDEX_COOLDOWN: x', 'rate-limited'],
    ['CODEINTEL_REINDEX_IN_PROGRESS: x', 'reindex-in-progress'],
    ['CODEINTEL_VERSION_CONFLICT: x', 'conflict'],
    ['CODEINTEL_DEV_SERVER_OFFLINE: x', 'offline'],
    ['CODEINTEL_TIMEOUT: x', 'timeout'],
    ['CODEINTEL_RESPONSE_TOO_LARGE: x', 'too-large'],
    ['something else', 'unknown']
  ])('%s -> %s', (message, kind) => {
    expect(classifyReviewError({ message }).kind).toBe(kind)
  })
  it('falls back to the coarse 050 client kind and reads retryAfterSeconds', () => {
    expect(classifyReviewError({ kind: 'offline', message: 'x' }).kind).toBe('offline')
    expect(classifyReviewError({ kind: 'forbidden', message: 'x' }).kind).toBe('forbidden')
    expect(
      classifyReviewError({
        code: 'CODEINTEL_REINDEX_COOLDOWN',
        message: 'x',
        data: { retryAfterSeconds: 120 }
      }).retryAfterSeconds
    ).toBe(120)
  })
})
