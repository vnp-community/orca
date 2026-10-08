/**
 * Tests for code-intel-quality-wire-parsers.ts and code-intel-quality-errors.ts (FE-CV-TASK-087-01)
 */

import { describe, it, expect } from 'vitest'
import {
  parseCiComparison,
  parseCoverageReport,
  parseQualityCoverageResponse,
  parseQualityFinding,
  parseQualityFindingsResponse,
  parseQualityGate,
  parseQualityGateResponse,
  parseQualityProfileResponse,
  parseQualityRun,
  parseQualityTrendPoint,
  parseQualityTrendResponse,
  parseRunnableProfile
} from './code-intel-quality-wire-parsers'
import {
  parseEnvNotReadyData,
  parseProfileUnknownData,
  parseRetryAfter,
  parseRunInProgressData,
  parseWaiverExpiryData
} from './code-intel-quality-errors'
import { parseCodeIntelErrorMessage } from './code-intel-error-codes'
import { parseCodeIntelPushEvent } from './code-intel-parsers'

describe('parseQualityGate', () => {
  it('parses a valid gate with string observed/threshold', () => {
    const g = parseQualityGate({
      verdict: 'fail',
      mode: 'inform',
      profile: 'full@repo/v3',
      reasons: [
        { check: 'coverage', observed: '0.62', threshold: '0.80', result: 'fail', code: 'coverage_below', params: { a: 'b', n: 1 }, waivedCount: 2 }
      ],
      basedOn: { runIds: ['r1'], indexCommit: 'abc', stale: true, headCommit: 'h1', profileVersion: 3 }
    })
    expect(g.verdict).toBe('fail')
    expect(g.mode).toBe('inform')
    expect(g.reasons[0]).toMatchObject({ observed: '0.62', threshold: '0.80', waivedCount: 2, params: { a: 'b' } })
    expect(g.basedOn).toMatchObject({ stale: true, runIds: ['r1'], profileVersion: 3 })
  })

  it('maps unknown enums to unknown and never to pass', () => {
    const g = parseQualityGate({ verdict: 'weird', mode: 'x', reasons: [{ result: 'nope' }] })
    expect(g.verdict).toBe('unknown')
    expect(g.mode).toBe('unknown')
    expect(g.reasons[0].result).toBe('unknown')
  })

  it('tolerates non-object input and numeric observed values', () => {
    expect(parseQualityGate(null).verdict).toBe('unknown')
    expect(parseQualityGate(null).reasons).toEqual([])
    expect(parseQualityGate({ reasons: [{ observed: 3, threshold: 0 }] }).reasons[0].observed).toBe('3')
  })
})

describe('parseQualityRun', () => {
  it('parses steps, flags and ci block', () => {
    const run = parseQualityRun({
      id: 'r1',
      worktreeId: 'w',
      headCommit: 'h',
      scope: 'changed',
      profile: 'full',
      status: 'interrupted',
      source: 'ci',
      startedAt: 't1',
      finishedAt: null,
      dirty: true,
      workTreeChangedDuringRun: true,
      scopeWidened: true,
      summary: { error: 2, warning: -1, stepsTotal: 3 },
      steps: [{ id: 's', status: 'env_not_ready', failureKind: 'env', envReason: 'node_modules', errorCount: 'x' }],
      ci: { provider: 'gha', headSha: 'sha', fetchedAt: 'f' }
    })
    expect(run.status).toBe('interrupted')
    expect(run.summary).toMatchObject({ error: 2, warning: 0, stepsTotal: 3 })
    expect(run.steps[0]).toMatchObject({ status: 'env_not_ready', failureKind: 'env', envReason: 'node_modules', errorCount: 0 })
    expect(run).toMatchObject({ dirty: true, workTreeChangedDuringRun: true, scopeWidened: true })
    expect(run.ci?.provider).toBe('gha')
  })

  it('unknown enums become unknown; missing arrays become []', () => {
    const run = parseQualityRun({ status: 'zzz', scope: 'q', steps: 'no' })
    expect(run.status).toBe('unknown')
    expect(run.scope).toBe('unknown')
    expect(run.steps).toEqual([])
    expect(run.indexBasis).toEqual([])
    expect(run.workTreeChangedDuringRun).toBe(false)
  })
})

describe('parseQualityFinding', () => {
  it('keeps contract fields and defaults endLine to line', () => {
    const f = parseQualityFinding({
      fingerprint: 'fp',
      fpVersion: 1,
      ruleId: 'no-unused-vars',
      severity: 'warning',
      category: 'lint',
      file: 'a/b.ts',
      line: 4,
      column: 2,
      message: 'm',
      tool: 'oxlint',
      toolVersion: '1',
      inScope: true,
      waiver: { by: 'u', reason: 'r', expiresAt: 'e' }
    })
    expect(f).toMatchObject({ endLine: 4, column: 2, endColumn: 0, inScope: true, waiver: { by: 'u' } })
    expect(f.fixHint).toBeUndefined()
  })

  it('unknown severity/category → unknown', () => {
    const f = parseQualityFinding({ severity: 'critical', category: 'misc' })
    expect(f.severity).toBe('unknown')
    expect(f.category).toBe('unknown')
  })
})

describe('profiles', () => {
  it('RunnableProfile fails closed on ready', () => {
    expect(parseRunnableProfile({ id: 'go-test' }).ready).toBe(false)
    const p = parseRunnableProfile({ id: 'x', ready: true, heavy: true, scopes: ['changed'], missing: [{ check: 'c', reason: 'r', hint: 'h' }] })
    expect(p).toMatchObject({ ready: true, heavy: true, scopes: ['changed'] })
    expect(p.missing[0].hint).toBe('h')
  })

  it('parses profile response with thresholds', () => {
    const r = parseQualityProfileResponse({
      profile: { name: 'full', mode: 'inform', version: 2, definition: { coverage: { required: true, diffCoverageWarnBelow: 0.8, diffCoverageFailBelow: null } } },
      origin: 'repo',
      version: 2,
      runnableProfiles: [{ id: 'full', ready: true }]
    })
    expect(r.profile.definition.coverage).toEqual({ required: true, diffCoverageWarnBelow: 0.8, diffCoverageFailBelow: null })
    expect(r.runnableProfiles).toHaveLength(1)
  })
})

describe('gate response and CI comparison', () => {
  it('parses all nine relations and unknown fallback', () => {
    const relations = [
      'agree_pass', 'agree_fail', 'local_pass_ci_fail', 'local_fail_ci_pass', 'local_only', 'ci_only',
      'ci_pending', 'sha_mismatch', 'not_comparable'
    ]
    for (const relation of relations) {
      expect(parseCiComparison({ relation }).relation).toBe(relation)
    }
    expect(parseCiComparison({ relation: 'new_one' }).relation).toBe('unknown')
    expect(parseCiComparison({ relation: 'local_pass_ci_fail', reasonsHint: ['a', 1] }).reasonsHint).toEqual(['a'])
  })

  it('parses the response wrapper', () => {
    const r = parseQualityGateResponse({ gate: { verdict: 'pass' }, waivers: [{ id: 'w', subjectKind: 'check' }], comparison: [{}] })
    expect(r.gate.verdict).toBe('pass')
    expect(r.waivers[0].subjectKind).toBe('check')
    expect(r.comparison).toHaveLength(1)
  })
})

describe('findings, trend and coverage responses', () => {
  it('findings response carries paging fields', () => {
    const r = parseQualityFindingsResponse({ findings: [{ ruleId: 'a' }], totalCount: 9, truncated: true, outsideScopeCount: 3, nextPageToken: 't' })
    expect(r).toMatchObject({ totalCount: 9, truncated: true, outsideScopeCount: 3, nextPageToken: 't' })
    expect(parseQualityFindingsResponse(undefined).findings).toEqual([])
  })

  it('trend metrics keep absent keys absent (never 0)', () => {
    const p = parseQualityTrendPoint({ turnKey: 'p:1', verdict: 'warn', counts: { error: 1 }, metrics: { diffCoverage: 0.5, testsFailed: 'x' } })
    expect(p.metrics).toEqual({ diffCoverage: 0.5 })
    expect('newCycles' in p.metrics).toBe(false)
    expect(parseQualityTrendResponse({ points: [{}, {}], totalCount: 1 }).totalCount).toBe(2)
  })

  it('coverage: null report and diff, ratios stay null when unreadable', () => {
    expect(parseCoverageReport(null)).toBeNull()
    expect(parseQualityCoverageResponse({ report: null, reason: 'no_coverage' })).toEqual({ report: null, reason: 'no_coverage' })
    const report = parseCoverageReport({
      source: 'estimated',
      diff: { changedExecutable: 10, covered: 4, uncovered: 6, diffCoverage: 'x', partial: true, excludedFiles: [{ path: 'a', reason: 'gen' }] },
      files: [{ path: 'a.ts', stmts: 10, covered: 4, pct: 0.4, uncoveredRanges: [[1, 2], ['x', 3]] }],
      totals: { pct: 0.4 }
    })
    expect(report?.source).toBe('estimated')
    expect(report?.diff?.diffCoverage).toBeNull()
    expect(report?.diff?.partial).toBe(true)
    expect(report?.files[0].uncoveredRanges).toEqual([[1, 2]])
    expect(report?.totals).toEqual({ pct: 0.4 })
  })
})

describe('error data readers', () => {
  it('reads the JSON suffix of contract error messages', () => {
    const env = parseCodeIntelErrorMessage('CODEINTEL_ENV_NOT_READY: x | {"missing":["node_modules",{"check":"go","reason":"absent","hint":"install"}]}')
    expect(parseEnvNotReadyData(env.data)?.missing).toEqual([
      { check: 'node_modules', reason: '' },
      { check: 'go', reason: 'absent', hint: 'install' }
    ])
    expect(parseRunInProgressData({ runId: 'r9' })).toEqual({ runId: 'r9' })
    expect(parseProfileUnknownData({ available: ['a', 3, 'b'] })?.available).toEqual(['a', 'b'])
    expect(parseWaiverExpiryData({ maxDays: 30 })).toEqual({ maxDays: 30 })
    expect(parseRetryAfter({ retryAfterMs: 1500 })).toBe(2)
    expect(parseRetryAfter({ retryAfterSeconds: 7 })).toBe(7)
  })

  it('corrupt data yields undefined', () => {
    expect(parseEnvNotReadyData(null)).toBeUndefined()
    expect(parseEnvNotReadyData({ missing: 'x' })).toBeUndefined()
    expect(parseRunInProgressData({ runId: 5 })).toBeUndefined()
    expect(parseProfileUnknownData({})).toBeUndefined()
    expect(parseWaiverExpiryData({ maxDays: 'x' })).toBeUndefined()
    expect(parseRetryAfter(undefined)).toBeUndefined()
  })
})

describe('quality push events (contract §5)', () => {
  it('carries stage/step/message, terminal status and gate verdict details', () => {
    expect(
      parseCodeIntelPushEvent({ event: 'quality.progress', worktreeId: 'w', runId: 'r', stage: 'lint', stepIndex: 1, stepCount: 4, percent: null, message: 'm' })
    ).toMatchObject({ event: 'qualityProgress', stage: 'lint', stepIndex: 1, stepCount: 4, percent: null, message: 'm' })
    expect(
      parseCodeIntelPushEvent({ event: 'quality.finished', worktreeId: 'w', runId: 'r', status: 'interrupted', headCommit: 'h' })
    ).toMatchObject({ event: 'qualityFinished', status: 'interrupted', success: false, headCommit: 'h' })
    expect(
      parseCodeIntelPushEvent({ event: 'quality.gateChanged', worktreeId: 'w', verdict: 'FAIL', previousVerdict: null, profile: 'p@repo/v1' })
    ).toMatchObject({ event: 'gateChanged', gate: 'fail', previousVerdict: null, profile: 'p@repo/v1' })
  })
})
