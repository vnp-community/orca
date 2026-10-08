/**
 * Tests for code-intel-types.ts (FE-CV-TASK-050-01)
 *
 * Fixture assignment + expectTypeOf: the shapes follow CONTRACT-codeintel-ui-api §2.2/§4.
 */

import { describe, it, expect, expectTypeOf } from 'vitest'
import type {
  ChangeOverlay,
  CodeIntelEnvelope,
  CodeIntelPushEvent,
  ContractDiff,
  Finding,
  IndexOverall,
  IndexStatus,
  PushChanged,
  PushGateChanged,
  PushQualityFinished,
  PushQualityProgress,
  PushReindexProgress,
  ReviewState,
  SymbolKind,
  WithUnknown,
  WorktreeSel
} from './code-intel-types'
import {
  CHANGE_OVERLAY_EMPTY_UNBORN,
  CHANGE_OVERLAY_SMALL,
  INDEX_STATUS_FIXTURES,
  REVIEW_STATE_INITIAL
} from '../renderer/src/test-support/code-intel-fixtures'

describe('§2.1 WorktreeSel', () => {
  it('requires projectId and worktreeId (PQ-04)', () => {
    const sel: WorktreeSel = { projectId: 'p-1', worktreeId: 'repo::/wt' }
    expectTypeOf(sel).toEqualTypeOf<{ projectId: string; worktreeId: string }>()
  })
})

describe('§2.2 CodeIntelEnvelope', () => {
  it('accepts a full fixture; data is optional', () => {
    const env: CodeIntelEnvelope<{ value: number }> = {
      worktreeId: 'wt-1',
      view: 'structure',
      sources: [{ tool: 'gitnexus', version: '1', indexedAt: null, commit: 'abc' }],
      headCommit: 'abc',
      stale: false,
      truncated: false,
      totalCount: 10,
      etag: '"e1"',
      fromCache: false,
      generatedAt: '2026-10-07T00:00:00Z',
      data: { value: 42 }
    }
    expectTypeOf(env.data).toEqualTypeOf<{ value: number } | undefined>()
    expect(env.data?.value).toBe(42)
  })

  it('notModified is the literal true and carries no data', () => {
    expectTypeOf<CodeIntelEnvelope<string>['notModified']>().toEqualTypeOf<true | undefined>()
  })
})

describe('§4.1 IndexStatus', () => {
  it('overall has exactly the 9 uppercase values', () => {
    const all: IndexOverall[] = [
      'OFFLINE',
      'UNKNOWN',
      'NOT_INSTALLED',
      'BUILDING',
      'MISSING',
      'DEGRADED',
      'OVERLAY',
      'STALE',
      'READY'
    ]
    expect(new Set(all).size).toBe(9)
    expect(Object.values(INDEX_STATUS_FIXTURES).map((s) => s.overall).sort()).toEqual([...all].sort())
  })

  it('status is flat: tools[] and indexBasis[] (not an array of statuses)', () => {
    const status: IndexStatus = INDEX_STATUS_FIXTURES.ready
    expectTypeOf(status.tools).toBeArray()
    expectTypeOf(status.indexBasis).toBeArray()
    expect(status.tools[0]?.tool).toBe('gitnexus')
  })

  it('activeJob.percent may be null', () => {
    expectTypeOf<NonNullable<IndexStatus['activeJob']>['percent']>().toEqualTypeOf<number | null>()
  })
})

describe('§4.1 SymbolKind', () => {
  it('has exactly 13 values', () => {
    const kinds: SymbolKind[] = [
      'function',
      'method',
      'type',
      'value',
      'file',
      'folder',
      'route',
      'component',
      'namespace',
      'import',
      'cluster',
      'flow',
      'doc'
    ]
    expect(new Set(kinds).size).toBe(13)
  })
})

describe('§4.3 ChangeOverlay', () => {
  it('summary fixture compiles and exposes risk + limits', () => {
    const overlay: ChangeOverlay = CHANGE_OVERLAY_SMALL
    expect(overlay.risk.level).toBe('MEDIUM')
    expect(overlay.limits.totalCounts.files).toBe(3)
  })

  it('emptyReason is only "unborn-head"', () => {
    expectTypeOf<ChangeOverlay['emptyReason']>().toEqualTypeOf<'unborn-head' | undefined>()
    expect(CHANGE_OVERLAY_EMPTY_UNBORN.emptyReason).toBe('unborn-head')
  })

  it('changed file status admits unknown', () => {
    expectTypeOf<ChangeOverlay['changedFiles'][number]['status']>().toEqualTypeOf<
      WithUnknown<'added' | 'modified' | 'deleted' | 'renamed' | 'copied' | 'untracked'>
    >()
  })
})

describe('§4.5 Finding', () => {
  it('severity admits unknown (U4)', () => {
    expectTypeOf<Finding['severity']>().toEqualTypeOf<'error' | 'warning' | 'info' | 'unknown'>()
  })
})

describe('§4.5 ContractDiff', () => {
  it('uses changes[] + summary (not the legacy breakingChanges)', () => {
    expectTypeOf<ContractDiff['summary']>().toEqualTypeOf<{
      breaking: number
      risky: number
      compatible: number
      unknown: number
    }>()
    expectTypeOf<ContractDiff>().toHaveProperty('changes')
  })
})

describe('§4.6 ReviewState', () => {
  it('version 0 means no record yet', () => {
    const state: ReviewState = REVIEW_STATE_INITIAL
    expect(state.version).toBe(0)
    expect(state.readingProgress.version).toBe(1)
  })
})

describe('§5 push events (normalized internal names)', () => {
  it('PushQualityProgress carries percent: number | null', () => {
    expectTypeOf<PushQualityProgress['percent']>().toEqualTypeOf<number | null>()
  })

  it('PushGateChanged gate can be unknown', () => {
    expectTypeOf<PushGateChanged['gate']>().toEqualTypeOf<'pass' | 'warn' | 'fail' | 'unknown'>()
  })

  it('CodeIntelPushEvent is the union of all 5', () => {
    expectTypeOf<CodeIntelPushEvent>().toEqualTypeOf<
      PushChanged | PushReindexProgress | PushQualityProgress | PushQualityFinished | PushGateChanged
    >()
  })
})

describe('WithUnknown helper', () => {
  it('adds unknown to a literal union', () => {
    expectTypeOf<WithUnknown<'a' | 'b'>>().toEqualTypeOf<'a' | 'b' | 'unknown'>()
  })
})
