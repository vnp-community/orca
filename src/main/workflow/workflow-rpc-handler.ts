/**
 * Workflow RPC Methods (TDD-17)
 *
 * Factory function — inject orchestrator and templateResolver at bootstrap.
 * 7 RPC methods:
 *   workflow.execute, workflow.getExecution, workflow.listExecutions,
 *   workflow.cancel, workflow.template.create, workflow.template.list,
 *   workflow.template.resolve
 *
 * Access control:
 * - All ops require authentication (userId from ctx.userId)
 * - workflow.cancel: only the triggeredBy user (or admin)
 * - workflow.template.create: any authenticated user
 *
 * @module main/workflow/workflow-rpc-handler
 */

import { z } from 'zod'
import { defineMethod } from '../runtime/rpc/core'
import type { RpcMethod } from '../runtime/rpc/core'
import type { WorkflowOrchestrator } from './WorkflowOrchestrator'
import type { TemplateResolver } from './TemplateResolver'

// ── Param schemas ─────────────────────────────────────────────────────────────

const WorkflowStepConfigSchema = z.object({
  type: z.enum(['agent', 'shell', 'webhook', 'notification', 'condition']),
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
})

const GetExecutionParam = z.object({
  executionId: z.string().min(1),
})

const ListExecutionsParam = z.object({
  projectId: z.string().optional(),
  triggeredBy: z.string().optional(),
  status: z.enum(['pending', 'running', 'completed', 'failed', 'cancelled']).optional(),
  limit: z.number().int().positive().max(500).optional(),
})

const CancelParam = z.object({
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
  templateResolver: TemplateResolver
): RpcMethod[] {
  return [
    // ── workflow.execute ───────────────────────────────────────────────────

    defineMethod({
      name: 'workflow.execute',
      params: ExecuteParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? 'system'
        const execution = await orchestrator.execute(
          params.definition as Parameters<typeof orchestrator.execute>[0],
          params.inputs ?? {},
          userId,
          params.projectId
        )
        // Return execution ID immediately (non-blocking)
        return { executionId: execution.id, status: execution.status }
      },
    }),

    // ── workflow.getExecution ──────────────────────────────────────────────

    defineMethod({
      name: 'workflow.getExecution',
      params: GetExecutionParam,
      handler: async (params) => {
        const execution = await orchestrator.getExecution(params.executionId)
        if (!execution) throw new Error(`EXECUTION_NOT_FOUND: ${params.executionId}`)
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
        if (!execution) throw new Error(`EXECUTION_NOT_FOUND: ${params.executionId}`)
        if (execution.triggeredBy !== userId) {
          throw new Error('WORKFLOW_CANCEL_DENIED: only the triggering user can cancel this execution')
        }
        await orchestrator.cancel(params.executionId)
        return { cancelled: true }
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
