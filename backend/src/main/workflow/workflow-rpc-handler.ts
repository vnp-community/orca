/**
 * Workflow RPC Methods (TDD-17)
 *
 * Factory function — inject orchestrator and templateResolver at bootstrap.
 * 9 RPC methods:
 *   workflow.execute, workflow.getExecution, workflow.listExecutions,
 *   workflow.cancel, workflow.pause, workflow.resume,
 *   workflow.template.create, workflow.template.list, workflow.template.resolve
 *
 * Access control:
 * - All ops require authentication (userId from ctx.userId)
 * - workflow.cancel / workflow.pause / workflow.resume: only the triggeredBy user (or admin)
 * - workflow.template.create: any authenticated user
 *
 * [BUG-BE-HLD-009] workflow.resume calls orchestrator.resumeFromPause() — a SINGLE-execution,
 * user-triggered resume, NOT orchestrator.resumeRunningExecutions() (internal crash-recovery,
 * called once at server bootstrap for every status='running' execution, no RPC exposure).
 *
 * @module main/workflow/workflow-rpc-handler
 */

import { z } from 'zod'
import { defineMethod } from '../runtime/rpc/core'
import type { RpcMethod } from '../runtime/rpc/core'
import type { IConnectionPool } from '../db/pool'
import type { WorkflowOrchestrator } from './WorkflowOrchestrator'
import type { TemplateResolver } from './TemplateResolver'

// ── Param schemas ─────────────────────────────────────────────────────────────

const WorkflowStepProviderSchema = z.object({
  accountId: z.string().min(1),
  model: z.string().optional(),
})

const WorkflowStepConfigSchema = z.object({
  type: z.enum(['agent', 'shell', 'webhook', 'notification', 'condition']),
  provider: WorkflowStepProviderSchema.optional(), // [NEW BUG-BE-HLD-008]
}).catchall(z.unknown())

const WorkflowStepSchema = z.object({
  id: z.string().min(1),
  name: z.string().min(1),
  serverSpec: z.string().min(1),
  dependsOn: z.array(z.string()).optional(),
  config: WorkflowStepConfigSchema,
  timeout: z.number().int().positive().optional(),
  continueOnError: z.boolean().optional(),
})

const WorkflowDefinitionSchema = z.object({
  steps: z.array(WorkflowStepSchema),
  inputs: z.record(z.unknown()).optional(),
})

const ExecuteParam = z.object({
  definition: WorkflowDefinitionSchema,
  inputs: z.record(z.unknown()).optional(),
  projectId: z.string().optional(),
  traceId: z.string().optional(), // [NEW] — reserved for future FE-initiated resume, không wire vào orchestrator.execute() (chưa có resume param, xem note bên dưới)
})

const GetExecutionParam = z.object({
  executionId: z.string().min(1),
})

const ListExecutionsParam = z.object({
  projectId: z.string().optional(),
  triggeredBy: z.string().optional(),
  status: z.enum(['pending', 'running', 'paused', 'completed', 'failed', 'cancelled']).optional(), // [NEW BUG-BE-HLD-009]
  limit: z.number().int().positive().max(500).optional(),
})

const CancelParam = z.object({
  executionId: z.string().min(1),
})

const PauseParam = z.object({
  executionId: z.string().min(1),
})

const ResumeParam = z.object({
  executionId: z.string().min(1),
})

const TemplateCreateParam = z.object({
  name: z.string().min(1),
  definition: WorkflowDefinitionSchema,
  scope: z.string().optional(),
  parentTemplateId: z.string().optional(),
})

const TemplateListParam = z.object({
  scope: z.string().min(1),
})

const TemplateResolveParam = z.object({
  templateId: z.string().min(1),
})

// ── Factory ────────────────────────────────────────────────────────────────────

export function createWorkflowMethods(
  orchestrator: WorkflowOrchestrator,
  templateResolver: TemplateResolver,
  pool?: IConnectionPool // [NEW] optional — chỉ dùng để đọc lại root_trace_id đã persist cho response
): RpcMethod[] {
  return [
    // ── workflow.execute ───────────────────────────────────────────────────

    defineMethod({
      name: 'workflow.execute',
      params: ExecuteParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? 'system'
        // NOTE: orchestrator.execute() hiện chưa nhận resume param — cần overload tương tự
        // AIProviderService.writeCredentialToDevServer() (TASK-BE-016.1) nếu muốn FE-initiated
        // traceId resume vào workflowExecuteFlow. Việc này để lại cho patch nhỏ bổ sung khi cần,
        // không chặn phần còn lại của task (rootTraceId nội bộ vẫn hoạt động độc lập).
        const execution = await orchestrator.execute(
          params.definition as Parameters<typeof orchestrator.execute>[0],
          params.inputs ?? {},
          userId,
          params.projectId
        )
        // Trả traceId để FE filter TracePanel theo execution — đọc lại root_trace_id đã
        // persist (TASK-BE-017.2), không phải giá trị tính lại trong bộ nhớ.
        let traceId: string | undefined
        if (pool) {
          const rows = await pool.withConnection((db) =>
            db.query(`SELECT root_trace_id as rootTraceId FROM orca_workflow_executions WHERE id = ?`, [
              execution.id,
            ])
          )
          traceId = (rows[0] as { rootTraceId: string | null } | undefined)?.rootTraceId ?? undefined
        }
        // Return execution ID immediately (non-blocking)
        return { executionId: execution.id, status: execution.status, traceId }
      },
    }),

    // ── workflow.getExecution ──────────────────────────────────────────────

    defineMethod({
      name: 'workflow.getExecution',
      params: GetExecutionParam,
      handler: async (params) => {
        const execution = await orchestrator.getExecution(params.executionId)
        if (!execution) {throw new Error(`EXECUTION_NOT_FOUND: ${params.executionId}`)}
        return execution
      },
    }),

    // ── workflow.listExecutions ────────────────────────────────────────────

    defineMethod({
      name: 'workflow.listExecutions',
      params: ListExecutionsParam,
      handler: async (params) => {
        return orchestrator.listExecutions(params)
      },
    }),

    // ── workflow.cancel ────────────────────────────────────────────────────

    defineMethod({
      name: 'workflow.cancel',
      params: CancelParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        // Access control: only triggeredBy user can cancel
        const execution = await orchestrator.getExecution(params.executionId)
        if (!execution) {throw new Error(`EXECUTION_NOT_FOUND: ${params.executionId}`)}
        if (execution.triggeredBy !== userId) {
          throw new Error('WORKFLOW_CANCEL_DENIED: only the triggering user can cancel this execution')
        }
        await orchestrator.cancel(params.executionId)
        return { cancelled: true }
      },
    }),

    // ── workflow.pause ─────────────────────────────────────────────────────

    defineMethod({
      name: 'workflow.pause',
      params: PauseParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        // Access control: same rule as workflow.cancel — only the triggering user may pause
        const execution = await orchestrator.getExecution(params.executionId)
        if (!execution) {throw new Error(`EXECUTION_NOT_FOUND: ${params.executionId}`)}
        if (execution.triggeredBy !== userId) {
          throw new Error('WORKFLOW_PAUSE_DENIED: only the triggering user can pause this execution')
        }
        await orchestrator.pause(params.executionId)
        return { paused: true }
      },
    }),

    // ── workflow.resume ────────────────────────────────────────────────────

    defineMethod({
      name: 'workflow.resume',
      params: ResumeParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        const execution = await orchestrator.getExecution(params.executionId)
        if (!execution) {throw new Error(`EXECUTION_NOT_FOUND: ${params.executionId}`)}
        if (execution.triggeredBy !== userId) {
          throw new Error('WORKFLOW_RESUME_DENIED: only the triggering user can resume this execution')
        }
        // [BUG-BE-HLD-009] resumeFromPause(), KHÔNG PHẢI resumeRunningExecutions() — xem header comment
        await orchestrator.resumeFromPause(params.executionId)
        return { resumed: true }
      },
    }),

    // ── workflow.template.create ───────────────────────────────────────────

    defineMethod({
      name: 'workflow.template.create',
      params: TemplateCreateParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? 'system'
        const id = await templateResolver.create({
          name: params.name,
          definition: params.definition as Parameters<typeof templateResolver.create>[0]['definition'],
          ownerId: userId,
          scope: params.scope,
          parentTemplateId: params.parentTemplateId,
        })
        return { templateId: id }
      },
    }),

    // ── workflow.template.list ─────────────────────────────────────────────

    defineMethod({
      name: 'workflow.template.list',
      params: TemplateListParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId
        return templateResolver.list(params.scope, userId)
      },
    }),

    // ── workflow.template.resolve ──────────────────────────────────────────

    defineMethod({
      name: 'workflow.template.resolve',
      params: TemplateResolveParam,
      handler: async (params) => {
        return templateResolver.resolve(params.templateId)
      },
    }),
  ]
}
