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

import type { IndexStatus, ChangeOverlay } from '../../../../shared/code-intel-types'

// ---------------------------------------------------------------------------
// §4.1 — IndexStatus fixtures
// ---------------------------------------------------------------------------

/** All 9 overall states for IndexStatus */
export const INDEX_STATUS_FIXTURES: Record<string, IndexStatus> = {
  ready: {
    overall: 'READY',
    filesIndexed: 1024,
    filesTotal: 1024,
    progressPercent: 100,
    errorMessage: null,
    lastIndexedAt: '2024-01-01T12:00:00Z'
  },
  indexing: {
    overall: 'INDEXING',
    filesIndexed: 512,
    filesTotal: 1024,
    progressPercent: 50,
    errorMessage: null,
    lastIndexedAt: null
  },
  partial: {
    overall: 'PARTIAL',
    filesIndexed: 900,
    filesTotal: 1024,
    progressPercent: 87,
    errorMessage: 'Some files could not be indexed',
    lastIndexedAt: '2024-01-01T11:00:00Z'
  },
  error: {
    overall: 'ERROR',
    filesIndexed: 0,
    filesTotal: 1024,
    progressPercent: 0,
    errorMessage: 'Index failed: out of memory',
    lastIndexedAt: null
  },
  notIndexed: {
    overall: 'NOT_INDEXED',
    filesIndexed: 0,
    filesTotal: 0,
    progressPercent: 0,
    errorMessage: null,
    lastIndexedAt: null
  },
  unknown: {
    overall: 'UNKNOWN',
    filesIndexed: 0,
    filesTotal: 0,
    progressPercent: 0,
    errorMessage: null,
    lastIndexedAt: null
  },
  cooldown: {
    overall: 'REINDEX_COOLDOWN',
    filesIndexed: 1024,
    filesTotal: 1024,
    progressPercent: 100,
    errorMessage: null,
    lastIndexedAt: '2024-01-01T11:59:00Z'
  },
  queued: {
    overall: 'QUEUED',
    filesIndexed: 0,
    filesTotal: 0,
    progressPercent: 0,
    errorMessage: null,
    lastIndexedAt: null
  },
  disabled: {
    overall: 'DISABLED',
    filesIndexed: 0,
    filesTotal: 0,
    progressPercent: 0,
    errorMessage: null,
    lastIndexedAt: null
  }
}

// ---------------------------------------------------------------------------
// §4.3 — ChangeOverlay fixtures
// ---------------------------------------------------------------------------

/** Small overlay (3 files) */
export const CHANGE_OVERLAY_SMALL: ChangeOverlay[] = [
  { path: 'src/a.ts', changeType: 'modified', oldPath: null },
  { path: 'src/b.ts', changeType: 'added', oldPath: null },
  { path: 'src/c.ts', changeType: 'deleted', oldPath: null }
]

/** Overlay with rename */
export const CHANGE_OVERLAY_WITH_RENAME: ChangeOverlay[] = [
  { path: 'src/new-name.ts', changeType: 'renamed', oldPath: 'src/old-name.ts' }
]

/** Overlay with truncated flag */
export const CHANGE_OVERLAY_TRUNCATED: ChangeOverlay[] = [
  ...CHANGE_OVERLAY_SMALL,
  // Simulated truncation: backend returns a marker
  { path: '...', changeType: 'truncated' as never, oldPath: null }
]

/** Empty overlay (no changes) */
export const CHANGE_OVERLAY_EMPTY: ChangeOverlay[] = []

// ---------------------------------------------------------------------------
// Error scenario wires (for failNext usage with 073-02 fake backend)
// ---------------------------------------------------------------------------

export const ERROR_WIRES = {
  timeout: {
    ok: false,
    error: {
      code: 'timeout',
      message: 'CODEINTEL_TIMEOUT | {"retryAfterMs":5000,"inProgress":true}'
    }
  },
  versionConflict: {
    ok: false,
    error: {
      code: 'conflict',
      message: 'CODEINTEL_VERSION_CONFLICT | {"expected":2,"actual":3}'
    }
  },
  ambiguousSymbol: {
    ok: false,
    error: {
      code: 'ambiguous',
      message: 'CODEINTEL_AMBIGUOUS_SYMBOL | {"candidates":["a.ts::Foo","b.ts::Foo"]}'
    }
  },
  reindexCooldown: {
    ok: false,
    error: {
      code: 'cooldown',
      message: 'CODEINTEL_REINDEX_COOLDOWN | {"nextAllowedAt":"2024-01-01T12:05:00Z"}'
    }
  }
} as const

// ---------------------------------------------------------------------------
// ReviewState fixture
// ---------------------------------------------------------------------------

export const REVIEW_STATE_INITIAL = {
  version: 0,
  headCommit: null,
  approved: false,
  approvedBy: null,
  approvedAt: null,
  checklistItems: [],
  comments: []
}
