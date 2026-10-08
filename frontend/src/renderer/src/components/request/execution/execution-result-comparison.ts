/**
 * Execution result comparison — FE-REQ-TASK-036-08
 *
 * "Agent reported" is never trusted: the verdict is Orca's own re-run. Finding
 * codes (CHECK_MISMATCH, SCOPE_VIOLATION, SECRET_*) are provisional (CR-REQ-029 2.6).
 *
 * @module components/request/execution/execution-result-comparison
 */

import type { ExecutionFailureClass, ExecutionResult } from '../../../../../shared/request-artifact-types'

type Verdict = ExecutionResult['verdict']

export type CheckComparison = { id: string; agentExit: number | null; orcaPassed: boolean | null; mismatch: boolean }

export function compareChecks(agentChecks: readonly { id: string; exit: number }[], verdict: Verdict): CheckComparison[] {
  return agentChecks.map((c) => {
    if (!verdict) {return { id: c.id, agentExit: c.exit, orcaPassed: null, mismatch: false }}
    const mentions = verdict.findings.filter((f) => f.message.includes(c.id) || f.code.includes(c.id))
    const mismatchFlag = mentions.some((f) => f.code === 'CHECK_MISMATCH')
    const failedFlag = mentions.some((f) => f.code.startsWith('CHECK_') && f.code !== 'CHECK_PASSED')
    const orcaPassed = !failedFlag
    return { id: c.id, agentExit: c.exit, orcaPassed, mismatch: mismatchFlag || (c.exit === 0) !== orcaPassed }
  })
}

export function classifyFiles(filesChanged: readonly string[], verdict: Verdict): { path: string; inScope: boolean }[] {
  // Why: scope comes from the backend verdict; the client never guesses a task's scope.
  const violations = (verdict?.findings ?? []).filter((f) => f.code === 'SCOPE_VIOLATION')
  return filesChanged.map((path) => ({ path, inScope: !violations.some((v) => v.message.includes(path)) }))
}

export type FailureRoute = { messageKey: string; fallback: string; params?: Record<string, unknown> }

export function describeFailureRoute(failureClass: ExecutionFailureClass, attemptsLeft?: number): FailureRoute {
  const P = 'auto.components.request.execution.route.'
  switch (failureClass) {
    case 'retryable':
      return { messageKey: `${P}retryable`, fallback: 'Will retry up to {{count}} more times', params: { count: attemptsLeft ?? 0 } }
    case 'spec_defect':
      return { messageKey: `${P}spec_defect`, fallback: 'Returned to the Plan step' }
    case 'needs_info':
      return { messageKey: `${P}needs_info`, fallback: 'Turned into a question' }
    case 'env_defect':
      return { messageKey: `${P}env_defect`, fallback: 'Does not count as an attempt' }
    case 'agent_defect':
      return { messageKey: `${P}agent_defect`, fallback: 'Retries once with a format reminder' }
    default:
      return { messageKey: `${P}unknown`, fallback: 'Unclassified failure' }
  }
}

export type SecretScanState = 'passed' | 'failed' | 'notRun'

export function secretScanState(verdict: Verdict): SecretScanState {
  const codes = (verdict?.findings ?? []).map((f) => f.code)
  if (codes.some((c) => c.startsWith('SECRET_') && c !== 'SECRET_SCAN_PASSED' && c !== 'SECRET_SCAN_SKIPPED')) {return 'failed'}
  if (codes.includes('SECRET_SCAN_PASSED')) {return 'passed'}
  return 'notRun'
}
