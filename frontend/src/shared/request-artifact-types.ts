/**
 * Request artifact types — FE-REQ-TASK-036-01
 *
 * Clarification, Decision, impact/risk, task readiness and execution result as
 * seen by the UI (camelCase via the gateway). Enums the parser does not know
 * become 'unknown'. Channel names are provisional (see request-rpc-methods).
 *
 * @module shared/request-artifact-types
 */

import type { GraphRisk } from './graph-types'

export type ClarificationQuestionKind = 'text' | 'single_choice' | 'multi_choice' | 'file' | 'boolean'
export type ClarificationSource =
  | 'readiness'
  | 'solution_open_question'
  | 'plan_assumption'
  | 'task_blocked'
  | 'unknown'
export type ClarificationStatus = 'open' | 'answered' | 'expired' | 'cancelled' | 'unknown'

export type ClarificationQuestion = {
  id: string
  seq: number
  questionKey: string
  kind: ClarificationQuestionKind
  prompt: string
  reason?: string
  options?: { value: string; label: string }[]
  suggestedDefault?: unknown
  required: boolean
  answer?: unknown
}

export type Clarification = {
  id: string
  displayId: string
  requestId: string
  source: ClarificationSource
  sourceRef?: string
  status: ClarificationStatus
  resumeStatus?: string
  round: number
  questions: ClarificationQuestion[]
  dueAt?: string
  assigneeIds?: string[]
  version: number
}

export type AnswerPayload = {
  clarificationId: string
  answers: { questionId: string; valueJson: string; acceptDefault: boolean }[]
  complete: boolean
  expectedVersion: number
}

export type DecisionStatus = 'open' | 'chosen' | 'effective' | 'superseded' | 'unknown'
export type Decision = {
  id: string
  displayId: string
  subjectKind: string
  subjectId: string
  subjectDigest: string
  chosenOptionId?: string
  recommendedOptionId?: string
  rationale: string
  /** Server-assessed (DecisionRisk.Assess); distinct from ImpactSummary.level. */
  riskLevel: 'normal' | 'high'
  riskReasons: string[]
  status: DecisionStatus
  chooserId?: string
  confirmedBy?: string
  version: number
}

export type ImpactStatus = 'collecting' | 'ready' | 'partial' | 'failed' | 'unknown'
export type ImpactSummary = {
  assessmentId: string
  digest: string
  level: GraphRisk
  score: number | null
  topReasons: string[]
  confidence: 'low' | 'medium' | 'high' | null
  indexAgeCommits?: number
  assessedAt: string | null
  tool: string | null
  stale: boolean
  mode: 'shadow' | 'enforce'
  status: ImpactStatus
  hardRules: string[]
  narrative?: string
}

export type ImpactFinding = {
  id: string
  dimension: string
  level: GraphRisk
  title: string
  evidenceRef?: string
  nodeIds?: string[]
}

export type ImpactComparison = {
  optionId: string
  dimensions: Record<string, { level: GraphRisk; score: number | null; note?: string }>
}

export type RiskAcceptance = {
  id: string
  assessmentId: string
  findingId: string
  rationale: string
  acceptedBy: string
  createdAt: string
}

export type ImpactDriftItem = { taskId: string; expected: string; actual: string }
export type ImpactDrift = { phaseId: string; drifted: boolean; items: ImpactDriftItem[] }

export type TaskReadinessOutcome = 'ready' | 'needs_info' | 'spec_defect' | 'env_defect' | 'unknown'
export type TaskReadinessFinding = { code: string; tier: string; path?: string; message: string }
export type TaskReadinessReport = {
  taskId: string
  outcome: TaskReadinessOutcome
  tier?: string
  findings: TaskReadinessFinding[]
  specDigest?: string
  durationMs?: number
  checkedAt?: string
}

export type ExecutionFailureClass =
  | 'retryable'
  | 'needs_info'
  | 'spec_defect'
  | 'env_defect'
  | 'agent_defect'
  | 'unknown'

export type ExecutionResult = {
  taskId: string
  attempt: number
  parseStatus: 'ok' | 'missing' | 'invalid'
  status?: 'done' | 'blocked' | 'failed' | 'needs_info'
  summary?: string
  filesChanged: string[]
  checksRun: { id: string; exit: number }[]
  verdict?: { status: 'passed' | 'failed'; findings: { code: string; message: string }[] }
  failureClass?: ExecutionFailureClass
  stdoutTail?: string
  outputs?: Record<string, unknown>
}

// Why: strings are provisional until the CONTRACT section 7 names them.
export const ARTIFACT_EVENT_TYPES = [
  'clarification.requested',
  'clarification.answered',
  'clarification.expired',
  'clarification.cancelled',
  'decision.recorded',
  'decision.confirmed',
  'impact.assessed',
  'impact.drift_detected',
  'risk.accepted',
  'readiness.reported',
  'execution.verified'
] as const
