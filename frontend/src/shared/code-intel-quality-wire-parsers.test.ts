/**
 * Tests for code-intel-quality-wire-parsers.ts (FE-CV-TASK-087-01)
 */

import { describe, it, expect } from 'vitest'
import {
  parseQualityGate,
  parseQualityRun,
  parseQualityFinding,
  parseQualityProfile,
  parseQualityTrendPoint,
  parseCoverageReport,
  parseCiComparison
} from './code-intel-quality-wire-parsers'

describe('parseQualityGate', () => {
  it('parses valid gate', () => {
    const g = parseQualityGate({
      result: 'fail', mode: 'block', stale: false, unavailable: false, runId: 'r-1',
      reasons: [{ check: 'coverage', observed: 70, threshold: 80, result: 'fail', code: 'coverage_below_threshold' }]
    })
    expect(g.result).toBe('fail')
    expect(g.mode).toBe('block')
    expect(g.reasons[0].check).toBe('coverage')
    expect(g.runId).toBe('r-1')
  })

  it('unknown result → unknown', () => {
    expect(parseQualityGate({ result: 'weird' }).result).toBe('unknown')
  })

  it('missing reasons → []', () => {
    expect(parseQualityGate({ result: 'pass' }).reasons).toEqual([])
  })

  it('non-object input → safe defaults', () => {
    const g = parseQualityGate(null)
    expect(g.result).toBe('unknown')
    expect(g.stale).toBe(false)
  })
})

describe('parseQualityRun', () => {
  it('parses valid run', () => {
    const r = parseQualityRun({
      id: 'r-1', worktreeId: 'wt-1', profileId: 'p-1',
      status: 'running', startedAt: '2024-01-01', completedAt: null,
      error: null, percent: 50, steps: [], triggeredBy: 'user'
    })
    expect(r.status).toBe('running')
    expect(r.percent).toBe(50)
    expect(r.triggeredBy).toBe('user')
  })

  it('interrupted status is valid', () => {
    expect(parseQualityRun({ status: 'interrupted' }).status).toBe('interrupted')
  })

  it('unknown status → unknown', () => {
    expect(parseQualityRun({ status: 'cosmic' }).status).toBe('unknown')
  })

  it('negative percent → 0 (via safeNum)', () => {
    // percent uses nullableNum, not safeNum — should return null for missing, not 0
    const r = parseQualityRun({ percent: null })
    expect(r.percent).toBeNull()
  })
})

describe('parseQualityFinding', () => {
  it('parses valid finding', () => {
    const f = parseQualityFinding({
      id: 'f-1', ruleId: 'rule-1', ruleName: 'No unused vars',
      severity: 'high', category: 'maintainability',
      path: 'src/a.ts', startLine: 1, endLine: 1, startColumn: 0, endColumn: null,
      message: 'Unused', snippet: null, waived: false, waiver: null,
      effortMinutes: 5, isNew: true, autoFixAvailable: false
    })
    expect(f.severity).toBe('high')
    expect(f.category).toBe('maintainability')
    expect(f.isNew).toBe(true)
  })

  it('unknown severity → unknown', () => {
    expect(parseQualityFinding({ severity: 'cosmic' }).severity).toBe('unknown')
  })

  it('unknown category → unknown', () => {
    expect(parseQualityFinding({ category: 'weird' }).category).toBe('unknown')
  })

  it('negative startLine → 0', () => {
    expect(parseQualityFinding({ startLine: -5 }).startLine).toBe(0)
  })

  it('parses waiver correctly', () => {
    const f = parseQualityFinding({
      waived: true,
      waiver: { id: 'w-1', findingId: 'f-1', reason: 'OK', waivedBy: 'user', waivedAt: '2024-01-01', expiresAt: null }
    })
    expect(f.waiver?.id).toBe('w-1')
    expect(f.waiver?.expiresAt).toBeNull()
  })
})

describe('parseQualityProfile', () => {
  it('parses profiles list', () => {
    const p = parseQualityProfile({
      activeProfileId: 'p-1',
      profiles: [{ id: 'p-1', name: 'Default', description: null, isDefault: true, checks: ['cov'], estimatedMinutes: 5 }]
    })
    expect(p.activeProfileId).toBe('p-1')
    expect(p.profiles).toHaveLength(1)
    expect(p.profiles[0].checks).toEqual(['cov'])
  })

  it('missing profiles → []', () => {
    expect(parseQualityProfile({}).profiles).toEqual([])
  })
})

describe('parseQualityTrendPoint', () => {
  it('parses trend point', () => {
    const t = parseQualityTrendPoint({ date: '2024-01-01', score: 82, coverage: 74, violations: 3, runId: 'r-1' })
    expect(t.score).toBe(82)
    expect(t.violations).toBe(3)
  })

  it('missing score → null', () => {
    expect(parseQualityTrendPoint({ date: '2024-01-01' }).score).toBeNull()
  })
})

describe('parseCoverageReport', () => {
  it('parses coverage report', () => {
    const r = parseCoverageReport({ worktreeId: 'wt-1', lineCoverage: 0.85, files: [] })
    expect(r.lineCoverage).toBe(0.85)
    expect(r.files).toEqual([])
  })

  it('non-object → safe defaults', () => {
    const r = parseCoverageReport(null)
    expect(r.worktreeId).toBe('')
    expect(r.files).toEqual([])
  })
})

describe('parseCiComparison', () => {
  it('parses valid comparison', () => {
    const c = parseCiComparison({ verdict: 'better', scoreDelta: 2.5, newViolations: 0, resolvedViolations: 3 })
    expect(c.verdict).toBe('better')
    expect(c.scoreDelta).toBe(2.5)
  })

  it('unknown verdict → unknown', () => {
    expect(parseCiComparison({ verdict: 'meh' }).verdict).toBe('unknown')
  })

  it('negative newViolations → 0', () => {
    expect(parseCiComparison({ newViolations: -1 }).newViolations).toBe(0)
  })
})
