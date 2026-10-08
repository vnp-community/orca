/**
 * code-intel-fixtures.ts — FE-CV-TASK-050-08
 *
 * Shared test fixtures for code-intel review-frontend features.
 * Used by both 050-series unit tests and 073-02 (fake backend).
 *
 * Rules:
 * - No absolute paths or secrets
 * - No duplicate fixture names with 073-02
 * - All paths use relative format (worktreeId::path)
 *
 * @module test-support/code-intel-fixtures
 */

import type { IndexOverall, IndexStatus, ReviewState } from '../../../shared/code-intel-types'

export * from './code-intel-symbol-fixtures'
export * from './code-intel-view-fixtures'

// ---------------------------------------------------------------------------
// §4.1 IndexStatus: one fixture per overall state (all 9)
// ---------------------------------------------------------------------------

function indexStatus(overall: IndexOverall, extra: Partial<IndexStatus> = {}): IndexStatus {
  return { overall, tools: [], scopeMismatch: false, indexBasis: [], ...extra }
}

export const INDEX_STATUS_FIXTURES: Record<Lowercase<IndexOverall>, IndexStatus> = {
  offline: indexStatus('OFFLINE'),
  unknown: indexStatus('UNKNOWN'),
  not_installed: indexStatus('NOT_INSTALLED'),
  building: indexStatus('BUILDING', { activeJob: { id: 'job-1', stage: 'parse', percent: null } }),
  missing: indexStatus('MISSING', { errorCode: 'CODEINTEL_INDEX_MISSING' }),
  degraded: indexStatus('DEGRADED', { scopeMismatch: true }),
  overlay: indexStatus('OVERLAY', {
    tools: [
      {
        tool: 'gitnexus',
        available: true,
        supported: true,
        state: 'stale',
        indexScope: 'exact',
        freshness: 'fresh_base',
        dirtySinceIndex: true,
        changedFilesNotInIndex: 3
      }
    ]
  }),
  stale: indexStatus('STALE', {
    tools: [
      {
        tool: 'codegraph',
        available: true,
        supported: true,
        state: 'stale',
        indexScope: 'repo_root',
        freshness: 'stale'
      }
    ]
  }),
  ready: indexStatus('READY', {
    tools: [
      {
        tool: 'gitnexus',
        available: true,
        supported: true,
        state: 'ready',
        indexScope: 'exact',
        freshness: 'fresh',
        stats: { files: 1024 }
      }
    ],
    indexBasis: [
      {
        tool: 'gitnexus',
        indexScope: 'exact',
        freshness: 'fresh',
        dirtySinceIndex: false,
        changedFilesNotInIndex: 0,
        refreshState: 'idle',
        indexPolicy: 'auto_in_place'
      }
    ]
  })
}

// ---------------------------------------------------------------------------
// Error wires (contract §2.3: code lives in error.message; error.code is the RPC-level code)
// ---------------------------------------------------------------------------

export const ERROR_WIRES = {
  timeout: {
    ok: false,
    error: {
      code: 'internal',
      message: 'CODEINTEL_TIMEOUT: read timed out | {"retryAfterMs":5000,"inProgress":true}'
    }
  },
  versionConflict: {
    ok: false,
    error: {
      code: 'internal',
      message: 'CODEINTEL_VERSION_CONFLICT: expectedVersion is stale | {"currentVersion":3}'
    }
  },
  ambiguousSymbol: {
    ok: false,
    error: {
      code: 'internal',
      message:
        'CODEINTEL_AMBIGUOUS_SYMBOL: more than one match | {"candidates":[{"uid":"a","name":"Foo","kind":"type","filePath":"a.ts","line":1}]}'
    }
  },
  reindexCooldown: {
    ok: false,
    error: {
      code: 'internal',
      message: 'CODEINTEL_REINDEX_COOLDOWN: wait before reindexing | {"retryAfterSeconds":300}'
    }
  },
  reindexInProgress: {
    ok: false,
    error: {
      code: 'internal',
      message: 'CODEINTEL_REINDEX_IN_PROGRESS: job running | {"jobId":"job-1","stage":"parse"}'
    }
  }
} as const

// ---------------------------------------------------------------------------
// ReviewState: version 0 means "no record yet"
// ---------------------------------------------------------------------------

export const REVIEW_STATE_INITIAL: ReviewState = {
  baseCommit: '',
  headCommit: '',
  readingProgress: { version: 1, entries: {}, lastFocusedKey: null },
  notes: { anchors: {}, sentBatches: [] },
  turnMarkers: [],
  status: 'open',
  version: 0
}
