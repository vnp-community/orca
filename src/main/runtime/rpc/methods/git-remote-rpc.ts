/**
 * git-remote-rpc.ts — Backend RPC routing layer cho git v6 (TDD-20)
 *
 * Routes client requests → ProjectServerRouter → relay → git-remote-handler-v6.ts
 * (relay tự chọn v5/v6 qua git-remote-handler-index.ts compile selector)
 *
 * KHÔNG implement git logic ở đây — chỉ routing + authorization.
 *
 * Pattern: router.getRelayForProject(projectId, userId) → relay.call('git.xxx', { cwd, ... })
 * cwd = params.worktreePath (passed from client, defaults to project root if omitted)
 */
import { z } from 'zod'
import { defineMethod } from '../core'
import type { RpcMethod } from '../core'
import type { ProjectServerRouter } from '../../../project/ProjectServerRouter'

export function createGitRemoteV6Methods(
  projectRouter: ProjectServerRouter,
): RpcMethod[] {
  return [
    defineMethod({
      name: 'git.status',
      params: z.object({ projectId: z.string(), worktreePath: z.string().optional() }),
      handler: async (params, ctx) => {
        const relay = await projectRouter.getRelayForProject(params.projectId, ctx.userId ?? '')
        return relay.call('git.status', { cwd: params.worktreePath })
      },
    }),

    defineMethod({
      name: 'git.diff',
      params: z.object({
        projectId: z.string(),
        worktreePath: z.string().optional(),
        staged: z.boolean().optional(),
        file: z.string().optional(),
      }),
      handler: async (params, ctx) => {
        const relay = await projectRouter.getRelayForProject(params.projectId, ctx.userId ?? '')
        return relay.call('git.diff', { cwd: params.worktreePath, staged: params.staged, file: params.file })
      },
    }),

    defineMethod({
      name: 'git.add',
      params: z.object({ projectId: z.string(), worktreePath: z.string().optional(), files: z.array(z.string()) }),
      handler: async (params, ctx) => {
        const relay = await projectRouter.getRelayForProject(params.projectId, ctx.userId ?? '')
        return relay.call('git.add', { cwd: params.worktreePath, files: params.files })
      },
    }),

    defineMethod({
      name: 'git.restore',
      params: z.object({
        projectId: z.string(),
        worktreePath: z.string().optional(),
        files: z.array(z.string()),
        staged: z.boolean().optional(),
      }),
      handler: async (params, ctx) => {
        const relay = await projectRouter.getRelayForProject(params.projectId, ctx.userId ?? '')
        return relay.call('git.restore', { cwd: params.worktreePath, files: params.files, staged: params.staged })
      },
    }),

    defineMethod({
      name: 'git.commit',
      params: z.object({
        projectId: z.string(),
        worktreePath: z.string().optional(),
        message: z.string().min(1),
      }),
      handler: async (params, ctx) => {
        const relay = await projectRouter.getRelayForProject(params.projectId, ctx.userId ?? '')
        return relay.call('git.commit', { cwd: params.worktreePath, message: params.message })
      },
    }),

    defineMethod({
      name: 'git.push',
      params: z.object({
        projectId: z.string(),
        worktreePath: z.string().optional(),
        remote: z.string().optional(),
        branch: z.string().optional(),
        force: z.boolean().optional(),
      }),
      handler: async (params, ctx) => {
        const relay = await projectRouter.getRelayForProject(params.projectId, ctx.userId ?? '')
        return relay.call('git.push', {
          cwd: params.worktreePath,
          remote: params.remote,
          branch: params.branch,
          force: params.force,
        })
      },
    }),

    defineMethod({
      name: 'git.pull',
      params: z.object({
        projectId: z.string(),
        worktreePath: z.string().optional(),
        remote: z.string().optional(),
        branch: z.string().optional(),
        rebase: z.boolean().optional(),
      }),
      handler: async (params, ctx) => {
        const relay = await projectRouter.getRelayForProject(params.projectId, ctx.userId ?? '')
        return relay.call('git.pull', {
          cwd: params.worktreePath,
          remote: params.remote,
          branch: params.branch,
          rebase: params.rebase,
        })
      },
    }),

    defineMethod({
      name: 'git.branch.list',
      params: z.object({
        projectId: z.string(),
        worktreePath: z.string().optional(),
        remote: z.boolean().optional(),
      }),
      handler: async (params, ctx) => {
        const relay = await projectRouter.getRelayForProject(params.projectId, ctx.userId ?? '')
        return relay.call('git.branch.list', { cwd: params.worktreePath, remote: params.remote })
      },
    }),

    defineMethod({
      name: 'git.checkout',
      params: z.object({
        projectId: z.string(),
        worktreePath: z.string().optional(),
        branch: z.string(),
        create: z.boolean().optional(),
      }),
      handler: async (params, ctx) => {
        const relay = await projectRouter.getRelayForProject(params.projectId, ctx.userId ?? '')
        return relay.call('git.checkout', { cwd: params.worktreePath, branch: params.branch, create: params.create })
      },
    }),
  ]
}
