/**
 * Task readiness and execution-result parsers — FE-REQ-TASK-036-01
 *
 * @module shared/request-task-run-parsers
 */

import type {
  ExecutionFailureClass,
  ExecutionResult,
  TaskReadinessOutcome,
  TaskReadinessReport
} from './request-artifact-types'
import { jsonish, num, oneOf, pick, rec, str, strings } from './request-artifact-wire-fields'

export function parseTaskReadinessReport(raw: unknown): TaskReadinessReport | null {
  const o = rec(raw)
  const taskId = o ? str(pick(o, 'taskId', 'task_id')) : undefined
  if (!o || !taskId) {
    return null
  }
  const findings = (Array.isArray(o.findings) ? o.findings : []).flatMap((x) => {
    const r = rec(x)
    const code = r ? str(r.code) : undefined
    return r && code
      ? [
          {
            code,
            tier: str(r.tier) ?? '',
            message: str(r.message) ?? '',
            ...(str(r.path) ? { path: str(r.path) } : {})
          }
        ]
      : []
  })
  const rep: TaskReadinessReport = {
    taskId,
    outcome: oneOf<TaskReadinessOutcome>(
      o.outcome,
      ['ready', 'needs_info', 'spec_defect', 'env_defect'],
      'unknown'
    ),
    findings
  }
  const tier = str(o.tier)
  if (tier) {
    rep.tier = tier
  }
  const digest = str(pick(o, 'specDigest', 'spec_digest'))
  if (digest) {
    rep.specDigest = digest
  }
  const dur = num(pick(o, 'durationMs', 'duration_ms'))
  if (dur !== undefined) {
    rep.durationMs = dur
  }
  const at = str(pick(o, 'checkedAt', 'checked_at'))
  if (at) {
    rep.checkedAt = at
  }
  return rep
}

export function parseExecutionResult(raw: unknown): ExecutionResult | null {
  const o = rec(raw)
  const taskId = o ? str(pick(o, 'taskId', 'task_id')) : undefined
  if (!o || !taskId) {
    return null
  }
  const verdictRaw = rec(o.verdict)
  const r: ExecutionResult = {
    taskId,
    attempt: num(o.attempt) ?? 1,
    parseStatus: oneOf(
      pick(o, 'parseStatus', 'parse_status'),
      ['ok', 'missing', 'invalid'] as const,
      'invalid'
    ),
    filesChanged: strings(pick(o, 'filesChanged', 'files_changed')),
    checksRun: (Array.isArray(pick(o, 'checksRun', 'checks_run'))
      ? (pick(o, 'checksRun', 'checks_run') as unknown[])
      : []
    ).flatMap((x) => {
      const c = rec(x)
      const id = c ? str(c.id) : undefined
      return c && id ? [{ id, exit: num(c.exit) ?? -1 }] : []
    })
  }
  const status = o.status
  if (status === 'done' || status === 'blocked' || status === 'failed' || status === 'needs_info') {
    r.status = status
  }
  const summary = str(o.summary)
  if (summary) {
    r.summary = summary
  }
  if (verdictRaw) {
    r.verdict = {
      status: verdictRaw.status === 'passed' ? 'passed' : 'failed',
      findings: (Array.isArray(verdictRaw.findings) ? verdictRaw.findings : []).flatMap((x) => {
        const f = rec(x)
        const code = f ? str(f.code) : undefined
        return f && code ? [{ code, message: str(f.message) ?? '' }] : []
      })
    }
  }
  const fc = pick(o, 'failureClass', 'failure_class')
  if (fc !== undefined) {
    r.failureClass = oneOf<ExecutionFailureClass>(
      fc,
      ['retryable', 'needs_info', 'spec_defect', 'env_defect', 'agent_defect'],
      'unknown'
    )
  }
  const tail = str(pick(o, 'stdoutTail', 'stdout_tail'))
  if (tail) {
    r.stdoutTail = tail
  }
  const outputs = rec(jsonish(o.outputs))
  if (outputs) {
    r.outputs = outputs
  }
  if (o.source === 'execution_record' || o.source === 'last_execution_output') {
    r.source = o.source
  }
  if (o.redacted === true) {
    r.redacted = true
  }
  if (o.truncated === true) {
    r.truncated = true
  }
  return r
}
