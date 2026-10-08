import { describe, expect, it } from 'vitest'
import { classifyFiles, compareChecks, describeFailureRoute, secretScanState } from './execution-result-comparison'
import type { ExecutionResult } from '../../../../../shared/request-artifact-types'

const verdict = (...findings: [string, string][]): ExecutionResult['verdict'] => ({ status: findings.length ? 'failed' : 'passed', findings: findings.map(([code, message]) => ({ code, message })) })

describe('compareChecks', () => {
  it('shows "not re-run yet" without a verdict', () => {
    expect(compareChecks([{ id: 'lint', exit: 0 }], undefined)).toEqual([{ id: 'lint', agentExit: 0, orcaPassed: null, mismatch: false }])
  })
  it('flags a mismatch when Orca disagrees with the agent', () => {
    const rows = compareChecks([{ id: 'lint', exit: 0 }, { id: 'test', exit: 0 }, { id: 'build', exit: 1 }], verdict(['CHECK_MISMATCH', 'test exited 1 on re-run'], ['CHECK_FAILED', 'build failed']))
    expect(rows.find((r) => r.id === 'lint')).toMatchObject({ orcaPassed: true, mismatch: false })
    expect(rows.find((r) => r.id === 'test')).toMatchObject({ orcaPassed: false, mismatch: true })
    expect(rows.find((r) => r.id === 'build')).toMatchObject({ orcaPassed: false, mismatch: false })
  })
})

describe('classifyFiles', () => {
  it('marks only files named in SCOPE_VIOLATION findings as out of scope', () => {
    const v = verdict(['SCOPE_VIOLATION', 'touched src/secret.ts outside scope'])
    expect(classifyFiles(['src/a.ts', 'src/secret.ts'], v)).toEqual([{ path: 'src/a.ts', inScope: true }, { path: 'src/secret.ts', inScope: false }])
    expect(classifyFiles(['x'], undefined)).toEqual([{ path: 'x', inScope: true }])
  })
})

describe('describeFailureRoute / secretScanState', () => {
  it('routes each failure class', () => {
    expect(describeFailureRoute('retryable', 2).params).toEqual({ count: 2 })
    for (const c of ['spec_defect', 'needs_info', 'env_defect', 'agent_defect', 'unknown'] as const) {expect(describeFailureRoute(c).messageKey).toContain(c)}
  })
  it('defaults to notRun and never claims passed without a finding', () => {
    expect(secretScanState(undefined)).toBe('notRun')
    expect(secretScanState(verdict())).toBe('notRun')
    expect(secretScanState(verdict(['SECRET_SCAN_PASSED', '']))).toBe('passed')
    expect(secretScanState(verdict(['SECRET_FOUND', 'token in .env']))).toBe('failed')
    expect(secretScanState(verdict(['SECRET_SCAN_SKIPPED', 'no tool']))).toBe('notRun')
  })
})
