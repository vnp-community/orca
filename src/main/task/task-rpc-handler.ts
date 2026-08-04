/**
 * Task RPC Methods (TDD-18)
 *
 * Factory function — inject services at bootstrap.
 * 18 RPC methods covering CRUD, tree ops, DAG edges, grants, AI, and agent execution.
 *
 * Permission checks:
 * - view ops: task.get, getChildren, getAncestors, getSubtree, getDependencies → view
 * - edit ops: task.update, addEdge, removeEdge → edit
 * - manage ops: task.delete → manage
 * - comment: task.addComment → comment
 * - execute: task.execute → execute
 * - grant: task.grant → manage
 *
 * @module main/task/task-rpc-handler
 */

import { z } from 'zod'
import { defineMethod } from '../runtime/rpc/core'
import type { RpcMethod } from '../runtime/rpc/core'
import type { TaskService } from './TaskService'
import type { TaskGrantService } from './TaskGrantService'
import type { TaskAIPlanner } from './TaskAIPlanner'
import type { TaskAgentExecutor } from './TaskAgentExecutor'
import { TASK_PERMISSION_ORDER } from '../../shared/task-types'
import type { TaskPermission } from '../../shared/task-types'

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Throws if resolved permission is below required level */
async function requirePermission(
  grantService: TaskGrantService,
  userId: string,
  taskId: string,
  required: TaskPermission
): Promise<void> {
  const perm = await grantService.resolvePermission(userId, taskId)
  const level = perm ? (TASK_PERMISSION_ORDER[perm] ?? 0) : 0
  const requiredLevel = TASK_PERMISSION_ORDER[required] ?? 0
  if (level < requiredLevel) {
    throw new Error(`TASK_PERMISSION_DENIED: need "${required}" on task "${taskId}"`)
  }
}

// ── Param schemas ─────────────────────────────────────────────────────────────

const TaskIdParam = z.object({ taskId: z.string().min(1) })

const CreateParam = z.object({
  projectId: z.string().optional(),
  parentId: z.string().optional(),
  title: z.string().min(1),
  description: z.string().optional(),
  type: z.enum(['epic', 'story', 'task', 'subtask', 'bug', 'spike']).optional(),
  priority: z.enum(['critical', 'high', 'medium', 'low']).optional(),
  labels: z.array(z.string()).optional(),
  visibility: z.enum(['private', 'team', 'company']).optional(),
  assigneeId: z.string().optional(),
  estimatedHours: z.number().positive().optional(),
  aiContext: z.string().optional(),
  promptTemplate: z.string().optional(),
  dueDate: z.string().optional(), // ISO date string
})

const UpdateParam = z.object({
  taskId: z.string().min(1),
  patch: z.object({
    title: z.string().optional(),
    description: z.string().optional(),
    status: z.enum(['backlog', 'todo', 'in_progress', 'review', 'done', 'blocked', 'cancelled']).optional(),
    priority: z.enum(['critical', 'high', 'medium', 'low']).optional(),
    labels: z.array(z.string()).optional(),
    assigneeId: z.string().optional(),
    estimatedHours: z.number().positive().optional(),
    progressPercent: z.number().int().min(0).max(100).optional(),
    aiContext: z.string().optional(),
    visibility: z.enum(['private', 'team', 'company']).optional(),
    dueDate: z.string().optional(),
  }),
})

const ListParam = z.object({
  projectId: z.string().optional(),
  parentId: z.string().optional(),
  assigneeId: z.string().optional(),
  status: z.enum(['backlog', 'todo', 'in_progress', 'review', 'done', 'blocked', 'cancelled']).optional(),
  type: z.enum(['epic', 'story', 'task', 'subtask', 'bug', 'spike']).optional(),
  limit: z.number().int().positive().max(500).optional(),
})

const EdgeParam = z.object({
  fromTaskId: z.string().min(1),
  toTaskId: z.string().min(1),
  edgeType: z.enum(['depends_on', 'blocks', 'relates_to', 'duplicates']),
  traceId: z.string().optional(), // [NEW CR-TRACE-018]
})

const CommentParam = z.object({
  taskId: z.string().min(1),
  content: z.string().min(1),
  type: z.enum(['comment', 'activity']).optional(),
})

const GrantParam = z.object({
  taskId: z.string().min(1),
  scope: z.enum(['user', 'team', 'role', 'everyone']),
  scopeId: z.string().optional(),
  permission: z.enum(['view', 'comment', 'edit', 'execute', 'manage']),
  applyTree: z.boolean().optional(),
  expiresAt: z.string().optional(), // ISO date
})

const ResolvePermParam = z.object({
  taskId: z.string().min(1),
  targetUserId: z.string().optional(),
})

const AiDecomposeParam = z.object({
  taskId: z.string().min(1),
  projectId: z.string().min(1),
  traceId: z.string().optional(), // [NEW CR-TRACE-018]
})

const AiApplyParam = z.object({
  taskId: z.string().min(1),
  subtasks: z.array(z.object({
    title: z.string().min(1),
    description: z.string().optional(),
    type: z.enum(['epic', 'story', 'task', 'subtask', 'bug', 'spike']).optional(),
    estimatedHours: z.number().positive().optional(),
  })),
})

const ExecuteParam = z.object({
  taskId: z.string().min(1),
  projectId: z.string().min(1),
  worktreePath: z.string().min(1),
  accountId: z.string().optional(),
  traceId: z.string().optional(), // [NEW CR-TRACE-018]
})

// ── Factory ───────────────────────────────────────────────────────────────────

export function createTaskMethods(
  taskService: TaskService,
  grantService: TaskGrantService,
  aiPlanner: TaskAIPlanner,
  executor: TaskAgentExecutor
): RpcMethod[] {
  return [
    // ── task.create ──────────────────────────────────────────────────────────

    defineMethod({
      name: 'task.create',
      params: CreateParam,
      handler: async (params) => {
        return taskService.create({
          ...params,
          dueDate: params.dueDate ? new Date(params.dueDate) : undefined,
        })
      },
    }),

    // ── task.get ─────────────────────────────────────────────────────────────

    defineMethod({
      name: 'task.get',
      params: TaskIdParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        await requirePermission(grantService, userId, params.taskId, 'view')
        const task = await taskService.get(params.taskId)
        if (!task) throw new Error(`TASK_NOT_FOUND: ${params.taskId}`)
        return task
      },
    }),

    // ── task.update ──────────────────────────────────────────────────────────

    defineMethod({
      name: 'task.update',
      params: UpdateParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        await requirePermission(grantService, userId, params.taskId, 'edit')
        const { dueDate, ...rest } = params.patch
        await taskService.update(params.taskId, {
          ...rest,
          dueDate: dueDate ? new Date(dueDate) : undefined,
        })
        return { updated: true }
      },
    }),

    // ── task.delete ──────────────────────────────────────────────────────────

    defineMethod({
      name: 'task.delete',
      params: TaskIdParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        await requirePermission(grantService, userId, params.taskId, 'manage')
        await taskService.delete(params.taskId)
        return { deleted: true }
      },
    }),

    // ── task.list ────────────────────────────────────────────────────────────

    defineMethod({
      name: 'task.list',
      params: ListParam,
      handler: async (params) => {
        return taskService.list(params)
      },
    }),

    // ── task.getChildren ─────────────────────────────────────────────────────

    defineMethod({
      name: 'task.getChildren',
      params: TaskIdParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        await requirePermission(grantService, userId, params.taskId, 'view')
        return taskService.getChildren(params.taskId)
      },
    }),

    // ── task.getAncestors ────────────────────────────────────────────────────

    defineMethod({
      name: 'task.getAncestors',
      params: TaskIdParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        await requirePermission(grantService, userId, params.taskId, 'view')
        return taskService.getAncestors(params.taskId)
      },
    }),

    // ── task.getSubtree ──────────────────────────────────────────────────────

    defineMethod({
      name: 'task.getSubtree',
      params: TaskIdParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        await requirePermission(grantService, userId, params.taskId, 'view')
        return taskService.getSubtree(params.taskId)
      },
    }),

    // ── task.addEdge ─────────────────────────────────────────────────────────

    defineMethod({
      name: 'task.addEdge',
      params: EdgeParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        await requirePermission(grantService, userId, params.fromTaskId, 'edit')
        await taskService.addEdge(params.fromTaskId, params.toTaskId, params.edgeType)
        return { added: true }
      },
    }),

    // ── task.removeEdge ──────────────────────────────────────────────────────

    defineMethod({
      name: 'task.removeEdge',
      params: EdgeParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        await requirePermission(grantService, userId, params.fromTaskId, 'edit')
        await taskService.removeEdge(params.fromTaskId, params.toTaskId, params.edgeType)
        return { removed: true }
      },
    }),

    // ── task.getDependencies ─────────────────────────────────────────────────

    defineMethod({
      name: 'task.getDependencies',
      params: TaskIdParam,
      handler: async (params) => {
        return taskService.getDependencies(params.taskId)
      },
    }),

    // ── task.recalculateProgress ─────────────────────────────────────────────

    defineMethod({
      name: 'task.recalculateProgress',
      params: TaskIdParam,
      handler: async (params) => {
        const progress = await taskService.recalculateProgress(params.taskId)
        return { progressPercent: progress }
      },
    }),

    // ── task.addComment ──────────────────────────────────────────────────────

    defineMethod({
      name: 'task.addComment',
      params: CommentParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        await requirePermission(grantService, userId, params.taskId, 'comment')
        await taskService.addComment(
          params.taskId,
          userId,
          params.content,
          params.type ?? 'comment'
        )
        return { added: true }
      },
    }),

    // ── task.grant ───────────────────────────────────────────────────────────

    defineMethod({
      name: 'task.grant',
      params: GrantParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        await requirePermission(grantService, userId, params.taskId, 'manage')
        const grantId = await grantService.grantPermission({
          taskId: params.taskId,
          scope: params.scope,
          scopeId: params.scopeId,
          permission: params.permission,
          applyTree: params.applyTree,
          grantedBy: userId,
          expiresAt: params.expiresAt ? new Date(params.expiresAt) : undefined,
        })
        return { grantId }
      },
    }),

    // ── task.resolvePermission ───────────────────────────────────────────────

    defineMethod({
      name: 'task.resolvePermission',
      params: ResolvePermParam,
      handler: async (params, ctx) => {
        const userId = params.targetUserId ?? ctx.userId ?? ''
        const permission = await grantService.resolvePermission(userId, params.taskId)
        return { permission }
      },
    }),

    // ── task.aiDecompose ─────────────────────────────────────────────────────

    defineMethod({
      name: 'task.aiDecompose',
      params: AiDecomposeParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        await requirePermission(grantService, userId, params.taskId, 'edit')
        return aiPlanner.decompose(params.taskId, params.projectId, userId)
      },
    }),

    // ── task.aiApply ─────────────────────────────────────────────────────────

    defineMethod({
      name: 'task.aiApply',
      params: AiApplyParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        await requirePermission(grantService, userId, params.taskId, 'edit')
        return aiPlanner.applyDecomposition(params.taskId, params.subtasks)
      },
    }),

    // ── task.execute ─────────────────────────────────────────────────────────

    defineMethod({
      name: 'task.execute',
      params: ExecuteParam,
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        // executor.executeTask handles permission check internally
        await executor.executeTask({
          taskId: params.taskId,
          projectId: params.projectId,
          userId,
          worktreePath: params.worktreePath,
          accountId: params.accountId,
        })
        return { started: true }
      },
    }),
  ]
}
