/**
 * WorkflowTypes — Core type definitions for workflow orchestration (TDD-17)
 *
 * Defines the DAG-based workflow execution model:
 * - WorkflowDefinition: the static blueprint
 * - WorkflowExecution: a runtime instance with status tracking
 * - WorkflowStep: a single node in the DAG
 *
 * @module main/workflow/WorkflowTypes
 */

/** Step types supported by the orchestrator */
export type WorkflowStepType = 'agent' | 'shell' | 'webhook' | 'notification' | 'condition'

/** Execution lifecycle status */
export type WorkflowStatus = 'pending' | 'running' | 'paused' | 'completed' | 'failed' | 'cancelled'

/** Per-step configuration — type-specific fields are opaque to the DAG engine */
/**
 * Explicit AI provider override for a single 'agent' step (BUG-BE-HLD-008).
 * `accountId` must reference a row in orca_ai_provider_accounts belonging to the
 * dev server the step's serverSpec resolves to — StepExecutors validates this at
 * dispatch time (WORKFLOW_STEP_PROVIDER_NOT_FOUND if the account doesn't exist).
 */
export type WorkflowStepProviderConfig = {
  accountId: string
  /** Overrides the account's configured model for this step only. */
  model?: string
}

export type WorkflowStepConfig = {
  type: WorkflowStepType
  // agent:        { prompt: string; worktreePath: string; trustPreset?: string; provider?: WorkflowStepProviderConfig }
  // shell:        { script: string; env?: Record<string, string> }
  // webhook:      { url: string; method?: string; body?: unknown }
  // notification: { channel: string; message: string }
  // condition:    { expression: string }
  /** 'agent' steps only — see WorkflowStepProviderConfig. Absent = use project default (ProviderResolver priority chain). */
  provider?: WorkflowStepProviderConfig
  [key: string]: unknown
}

/** A single node in the workflow DAG */
export type WorkflowStep = {
  id: string
  name: string
  /** 'project:<projectId>' or 'server:<devServerId>' */
  serverSpec: string
  /** Step IDs this step must wait for before executing */
  dependsOn?: string[]
  config: WorkflowStepConfig
  /** Execution timeout in ms (default: 30 min) */
  timeout?: number
  /** If true, workflow continues to next wave even if this step fails */
  continueOnError?: boolean
}

/** Static workflow blueprint */
export type WorkflowDefinition = {
  steps: WorkflowStep[]
  inputs?: Record<string, unknown>
}

/** A running or completed workflow execution instance */
export type WorkflowExecution = {
  id: string
  definition: WorkflowDefinition
  status: WorkflowStatus
  inputs: Record<string, unknown>
  /** Index of the wave currently executing (0-based) */
  currentWave: number
  /** User or system that triggered this execution */
  triggeredBy: string
  projectId?: string
  startedAt?: Date
  completedAt?: Date
  pausedAt?: Date // [NEW BUG-BE-HLD-009] set on pause(), cleared on resumeFromPause()
  errorMessage?: string
  createdAt: Date
}

/** Result produced by a step executor */
export type StepOutput = {
  exitCode: number
  stdout?: string
  stderr?: string
  data?: Record<string, unknown>
}

/** Filters for listing workflow executions */
export type ListExecutionsFilter = {
  projectId?: string
  triggeredBy?: string
  status?: WorkflowStatus
  limit?: number
}

/** Thrown when a cycle is detected in the step dependency graph */
export class WorkflowCycleError extends Error {
  constructor(public readonly cycle: string[]) {
    super(`Workflow cycle detected: ${cycle.join(' → ')}`)
    this.name = 'WorkflowCycleError'
  }
}
