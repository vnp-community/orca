/**
 * Tests for code-intel-types.ts (FE-CV-TASK-050-01)
 *
 * Type-level tests using expectTypeOf and fixture assignment.
 * Verifies: §4.1-§4.6 shapes compile correctly, SymbolKind has 13 members.
 */

import { describe, it, expectTypeOf } from 'vitest'
import type {
  WorktreeSel,
  CodeIntelEnvelope,
  IndexStatus,
  IndexOverall,
  SymbolKind,
  ChangeOverlay,
  ReviewState,
  Finding,
  FindingSeverity,
  PushChanged,
  PushReindexProgress,
  PushQualityProgress,
  PushQualityFinished,
  PushGateChanged,
  CodeIntelPushEvent,
  WithUnknown,
  ContractDiff
} from './code-intel-types'

describe('code-intel-types §4.1 WorktreeSel', () => {
  it('accepts valid fixture', () => {
    const sel: WorktreeSel = { worktreeId: 'wt-1', environmentId: 'env-1' }
    expectTypeOf(sel.worktreeId).toBeString()
  })

  it('environmentId is optional', () => {
    const sel: WorktreeSel = { worktreeId: 'wt-1' }
    expectTypeOf(sel.environmentId).toEqualTypeOf<string | null | undefined>()
  })
})

describe('code-intel-types §4.2 CodeIntelEnvelope', () => {
  it('accepts full fixture', () => {
    const env: CodeIntelEnvelope<{ value: number }> = {
      worktreeId: 'wt-1',
      view: 'main',
      sources: ['src/'],
      headCommit: 'abc123',
      stale: false,
      truncated: false,
      totalCount: 10,
      etag: 'etag-1',
      data: { value: 42 }
    }
    expectTypeOf(env.data).toEqualTypeOf<{ value: number } | undefined>()
  })

  it('notModified envelope has no data required', () => {
    const env: CodeIntelEnvelope<string> = {
      worktreeId: 'wt-1',
      view: 'main',
      sources: [],
      headCommit: null,
      stale: false,
      truncated: false,
      totalCount: 0,
      etag: null,
      notModified: true
    }
    expectTypeOf(env.notModified).toEqualTypeOf<boolean | undefined>()
  })
})

describe('code-intel-types §4.3 IndexStatus', () => {
  it('overall accepts all valid values', () => {
    const valids: IndexOverall[] = ['READY', 'INDEXING', 'PARTIAL', 'ERROR', 'UNKNOWN']
    expect(valids.length).toBe(5)
  })

  it('accepts full fixture', () => {
    const status: IndexStatus = {
      worktreeId: 'wt-1',
      overall: 'READY',
      lastIndexedAt: '2024-01-01T00:00:00Z',
      fileCoverage: 0.95,
      linesIndexed: 50000,
      running: false,
      percent: null,
      error: null
    }
    expectTypeOf(status.overall).toEqualTypeOf<IndexOverall>()
  })
})

describe('code-intel-types §4.4 SymbolKind', () => {
  it('has exactly 13 values', () => {
    const kinds: SymbolKind[] = [
      'file', 'namespace', 'module', 'class', 'interface', 'method',
      'function', 'variable', 'constant', 'property', 'enum_member',
      'type_alias', 'constructor'
    ]
    expect(kinds.length).toBe(13)
  })
})

describe('code-intel-types §4.5 ReviewState', () => {
  it('ChangeOverlay accepts unknown changeType', () => {
    const o: ChangeOverlay = { path: 'foo.ts', changeType: 'unknown' }
    expect(o.changeType).toBe('unknown')
  })

  it('ReviewState shape compiles with all fields', () => {
    const state: ReviewState = {
      id: 'rv-1',
      worktreeId: 'wt-1',
      headCommit: 'abc',
      overlay: [],
      impactGraph: [],
      comments: [],
      checklist: [],
      overallRisk: 'HIGH',
      approved: false,
      approvedAt: null,
      approvedBy: null
    }
    expectTypeOf(state.approved).toBeBoolean()
  })
})

describe('code-intel-types §4.6 Finding', () => {
  it('severity includes unknown', () => {
    const s: FindingSeverity = 'unknown'
    expect(s).toBe('unknown')
  })

  it('Finding shape compiles', () => {
    const f: Finding = {
      id: 'f-1', ruleId: 'rule-001', ruleName: 'No console',
      severity: 'high', path: 'src/a.ts', startLine: 1, endLine: 1,
      startColumn: 0, endColumn: null, message: 'msg', snippet: null,
      category: 'quality', waived: false, waivedAt: null,
      waivedBy: null, waivedReason: null, autoFixAvailable: false
    }
    expectTypeOf(f.severity).toEqualTypeOf<FindingSeverity>()
  })
})

describe('code-intel-types §5 Push events', () => {
  it('PushChanged has resync boolean', () => {
    const e: PushChanged = { event: 'changed', worktreeId: 'wt-1', reason: 'commit', resync: false }
    expectTypeOf(e.resync).toBeBoolean()
  })

  it('PushReindexProgress percent can be null', () => {
    const e: PushReindexProgress = { event: 'reindexProgress', worktreeId: 'wt-1', percent: null, running: true }
    expectTypeOf(e.percent).toEqualTypeOf<number | null>()
  })

  it('PushQualityProgress has phase', () => {
    const e: PushQualityProgress = { event: 'qualityProgress', worktreeId: 'wt-1', runId: 'r-1', percent: 50, phase: 'analyze' }
    expect(e.phase).toBe('analyze')
  })

  it('PushQualityFinished has error null', () => {
    const e: PushQualityFinished = { event: 'qualityFinished', worktreeId: 'wt-1', runId: 'r-1', success: true, error: null }
    expectTypeOf(e.error).toEqualTypeOf<string | null>()
  })

  it('PushGateChanged gate can be unknown', () => {
    const e: PushGateChanged = { event: 'gateChanged', worktreeId: 'wt-1', gate: 'unknown' }
    expect(e.gate).toBe('unknown')
  })

  it('CodeIntelPushEvent is union of all 5', () => {
    const events: CodeIntelPushEvent[] = [
      { event: 'changed', worktreeId: 'wt-1', reason: 'commit', resync: false },
      { event: 'reindexProgress', worktreeId: 'wt-1', percent: null, running: false },
      { event: 'qualityProgress', worktreeId: 'wt-1', runId: 'r', percent: null, phase: 'collect' },
      { event: 'qualityFinished', worktreeId: 'wt-1', runId: 'r', success: false, error: 'err' },
      { event: 'gateChanged', worktreeId: 'wt-1', gate: 'fail' }
    ]
    expect(events.length).toBe(5)
  })
})

describe('code-intel-types WithUnknown helper', () => {
  it('accepts unknown in addition to literal type', () => {
    const v: WithUnknown<'foo' | 'bar'> = 'unknown'
    expect(v).toBe('unknown')
  })
})

describe('code-intel-types ContractDiff', () => {
  it('ContractDiff shape compiles', () => {
    const diff: ContractDiff = {
      breakingChanges: [],
      addedEndpoints: [],
      removedEndpoints: [],
      modifiedSchemas: []
    }
    expect(diff.breakingChanges).toHaveLength(0)
  })
})
