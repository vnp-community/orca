/**
 * Workspace RPC Methods (TDD-19)
 *
 * 4 methods for workspace lifecycle management:
 * - workspace.init     → initWorkspace
 * - workspace.teardown → teardownWorkspace
 * - workspace.refreshFileTree → refreshFileTree
 * - workspace.refreshGitStatus → refreshGitStatus
 *
 * @module main/workspace/workspace-rpc-handler
 */

import { z } from 'zod'
import { defineMethod } from '../runtime/rpc/core'
import type { RpcMethod } from '../runtime/rpc/core'
import type { WorkspaceService } from './WorkspaceService'

export function createWorkspaceMethods(workspaceService: WorkspaceService): RpcMethod[] {
  return [
    // ── workspace.init ────────────────────────────────────────────────────────

    defineMethod({
      name: 'workspace.init',
      params: z.object({
        projectId: z.string().min(1),
      }),
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        return workspaceService.initWorkspace(params.projectId, userId)
      },
    }),

    // ── workspace.teardown ────────────────────────────────────────────────────

    defineMethod({
      name: 'workspace.teardown',
      params: z.object({
        projectId: z.string().min(1),
      }),
      handler: async (params) => {
        await workspaceService.teardownWorkspace(params.projectId)
        return { ok: true }
      },
    }),

    // ── workspace.refreshFileTree ─────────────────────────────────────────────

    defineMethod({
      name: 'workspace.refreshFileTree',
      params: z.object({
        projectId: z.string().min(1),
        path: z.string().optional(),
      }),
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        return workspaceService.refreshFileTree(params.projectId, userId, params.path)
      },
    }),

    // ── workspace.refreshGitStatus ────────────────────────────────────────────

    defineMethod({
      name: 'workspace.refreshGitStatus',
      params: z.object({
        projectId: z.string().min(1),
        worktreePath: z.string().min(1),
      }),
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        return workspaceService.refreshGitStatus(params.projectId, userId, params.worktreePath)
      },
    }),
  ]
}
