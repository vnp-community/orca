/**
 * task-execution-result-listener — writes agent/orchestration run outcomes
 * back onto OrcaTask (Source→Plan→Execute pipeline linkage, migration 0016;
 * docs/guides/task-automation-orchestration-integration.md §9.4.5).
 *
 * Neither execution path has a discrete completion *event* to subscribe to:
 * - Path (a): TaskAgentExecutor already `await`s agentSpawner.spawn() itself
 *   — that resolved/rejected Promise IS the completion signal. There is no
 *   backend 'agent.complete' emitter (only a frontend WorkspaceContext one).
 * - Path (b): coordinator.ts's `merge_ready` MessageType is a documented
 *   no-op case in Coordinator.processMessages() — it carries no completion
 *   semantics. The real signal is the Promise Coordinator.run()/
 *   runFromExistingRun() resolves with once checkConvergence() succeeds.
 *
 * So this module is a pair of plain write-back functions, called directly by
 * whichever code already holds that resolved outcome (TaskAgentExecutor for
 * (a), TaskOrchestrationBridge for (b)) — not a registered event handler.
 *
 * @module main/task/task-execution-result-listener
 */

import type { TaskService } from './TaskService'
import type { TaskStatus } from '../../shared/task-types'
import type { CoordinatorStatus, MessageRow } from '../runtime/orchestration/types'

/** Shape of what Coordinator.run()/runFromExistingRun() resolves with. */
export type OrchestrationRunOutcome = {
  runId: string
  status: CoordinatorStatus
  completedTasks: string[]
  failedTasks: string[]
  escalations: MessageRow[]
}

/**
 * Path (a) completion: a single agent session finished (or failed to spawn).
 * Writes status and — on success — the agent session id the run happened under.
 */
export async function recordAgentSessionCompletion(
  taskService: TaskService,
  taskId: string,
  outcome: { status: TaskStatus; agentSessionId?: string }
): Promise<void> {
  await taskService.update(taskId, {
    status: outcome.status,
    ...(outcome.agentSessionId ? { agentSessionId: outcome.agentSessionId } : {}),
  })
}

/**
 * Path (b) completion: a coordinator run converged (completed or failed).
 * Writes status and clears activeExecutionTaskId — the run it pointed to is over.
 */
export async function recordOrchestrationRunCompletion(
  taskService: TaskService,
  taskId: string,
  outcome: OrchestrationRunOutcome
): Promise<void> {
  const status: TaskStatus = outcome.status === 'completed' ? 'review' : 'blocked'
  await taskService.update(taskId, { status, activeExecutionTaskId: null })
}
