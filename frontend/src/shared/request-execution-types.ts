/**
 * Request execution read models — CR-REQ-038 / FE-REQ-TASK-037-02
 *
 * The flow of a request type, the summary of a run and the activity feed, as the gateway sends them (camelCase).
 * Enums the parser does not know keep their text (feed kinds) or become 'unknown' (closed sets).
 *
 * @module shared/request-execution-types
 */

import type {
  AnalysisKind,
  ExecutionGate,
  OpenSpecProfile,
  PlanKind,
  StartGate
} from './request-flow-registry'
import type { RequestStatus } from './request-types'

export type RequestFlowView = {
  type: string
  analysisKind: AnalysisKind
  analysisGate: 'solution' | 'findings' | 'answer' | null
  planKind: PlanKind
  /** For the size asked about; the registry's own phase rule says when a size L is needed. */
  hasPhases: boolean
  startGate: StartGate
  executionGates: readonly ExecutionGate[]
  completesAfterAnalysis: boolean
  humanConfirmRequired: boolean
  openSpecProfile: OpenSpecProfile
  /** 'backend' when it came from request.flow, 'fallback' when the built-in table answered. */
  source: 'backend' | 'fallback'
}

export type FailureCause =
  | 'timed_out'
  | 'model_error'
  | 'verify_failed'
  | 'env_defect'
  | 'spec_defect'
  | 'agent_error'
  | 'needs_info'
  | 'cancelled'
  /** CR-REQ-039: the Request's starting branch no longer exists, so no worktree could be created from it. */
  | 'base_ref_not_found'
  | 'unknown'

export type ExecutionProgress = {
  phasesTotal: number
  phasesDone: number
  tasksTotal: number
  tasksDone: number
  tasksFailed: number
  tasksRunning: number
}

export type ExecutionRunningTask = {
  taskId: string
  phaseId?: string
  title: string
  attempt: number
  maxAttempts: number
  startedAt?: string
}

export type ExecutionFailedTask = {
  taskId: string
  phaseId?: string
  title: string
  attempts: number
  maxAttempts: number
  lastCause: FailureCause
  lastError?: string
  lastAt?: string
}

export type NeedsYouKind =
  | 'approval'
  | 'clarification'
  | 'failed_task'
  | 'pre_deploy'
  | 'drift'
  | 'unknown'

export type ExecutionNeedsYou = {
  kind: NeedsYouKind
  refId: string
  subjectType?: string
  title?: string
  phaseId?: string
  since?: string
  dueAt?: string
}

export type HandoverState = 'none' | 'archiving' | 'ready' | 'failed'

export type ExecutionHandover = {
  state: HandoverState
  archiveAttempts: number
  summaryRef?: string
}

export type ExecutionSummary = {
  requestId: string
  status: RequestStatus
  engine?: 'native' | 'openspec'
  executionMode?: 'step_by_step' | 'continuous'
  progress: ExecutionProgress
  currentPhaseId?: string
  nextPhaseId?: string
  running: ExecutionRunningTask[]
  failed: ExecutionFailedTask[]
  needsYou: ExecutionNeedsYou[]
  handover: ExecutionHandover
  maxAttempts: number
  startedAt?: string
}

export type ActivitySeverity = 'info' | 'attention' | 'error'
export type ActivityActorKind = 'user' | 'ai' | 'agent' | 'system'

export type RequestActivityEntry = {
  id: string
  /** For example `task.failed` or `approval.decided`; a kind this build does not know is kept as is. */
  kind: string
  occurredAt: string
  actorKind: ActivityActorKind
  actorId?: string
  subjectType?: string
  subjectId?: string
  /** English fallback line; screens translate from kind and details. */
  summary: string
  attempt?: number
  severity: ActivitySeverity
  details: Record<string, unknown>
}

export type RequestActivityPage = {
  entries: RequestActivityEntry[]
  nextPageToken?: string
}

export type RequestActivityFilter = {
  kinds?: string[]
  minSeverity?: ActivitySeverity
  actorKind?: ActivityActorKind
  subjectId?: string
}
