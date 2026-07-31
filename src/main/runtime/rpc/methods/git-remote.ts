/**
 * git-remote.ts — Server-side Remote Git RPC methods (TASK-044)
 *
 * Routes client git requests through ProjectServerRouter to the relay.
 * 18 RPC methods: status, diff, add, restore, commit, push, pull, fetch,
 * branch.list/create/delete, checkout, log, worktree.list/add/remove,
 * generateCommitMessage, pr.create.
 *
 * Security: all routing happens through ProjectServerRouter which validates
 * project membership before exposing the relay.
 *
 * Task auto-advance: git.commit parses #TG-xxx refs from message → updates
 * task status to 'review' (non-fatal; never blocks commit).
 *
 * @module main/runtime/rpc/methods/git-remote
 */

import { z } from 'zod'
import { defineMethod } from '../core'
import type { RpcMethod } from '../core'
import type { ProjectServerRouter } from '../../../project/ProjectServerRouter'
import type { AIProviderService } from '../../../ai-providers/AIProviderService'
import type { TaskService } from '../../../task/TaskService'
import type { TaskGrantService } from '../../../task/TaskGrantService'
import type { GitExecResult } from '../../../../relay/git-remote-handler'

// ── Helpers ───────────────────────────────────────────────────────────────────

/** Regex to extract #TG-xxx task refs from commit messages */
const TASK_REF_RE = /#TG-([a-zA-Z0-9][a-zA-Z0-9-]{5,})/g

/** Auto-advance tasks referenced by commit to 'review'. Non-fatal. */
async function autoAdvanceTasks(
  message: string,
  taskService: TaskService,
  taskGrantService: TaskGrantService,
  userId: string
): Promise<void> {
  const refs = [...message.matchAll(TASK_REF_RE)].map(m => m[1]!)
  if (refs.length === 0) return

  await Promise.allSettled(
    refs.map(async (ref) => {
      try {
        const perm = await taskGrantService.resolvePermission(userId, ref)
        if (!perm || !['edit', 'execute', 'manage'].includes(perm)) return
        const task = await taskService.get(ref)
        if (!task) return
        if (['done', 'cancelled'].includes(task.status)) return
        await taskService.update(ref, { status: 'review' })
        await taskService.addComment(
          ref, userId,
          `Auto-advanced to review by commit: ${message.slice(0, 80)}`,
          'activity'
        )
      } catch {
        // Non-fatal — log but don't propagate
        console.warn(`[git-remote] auto-advance task ${ref} failed (non-fatal)`)
      }
    })
  )
}

// ── Common schemas ─────────────────────────────────────────────────────────────

const ProjectWorktreeParam = z.object({
  projectId: z.string().min(1),
  worktreePath: z.string().min(1),
})

const FilesParam = ProjectWorktreeParam.extend({
  files: z.array(z.string().min(1)).min(1),
})

// ── Factory ───────────────────────────────────────────────────────────────────

export function registerRemoteGitRpcMethods(
  router: ProjectServerRouter,
  aiProviderService: AIProviderService,
  taskService: TaskService,
  taskGrantService: TaskGrantService
): RpcMethod[] {
  return [
    // ── git.status ──────────────────────────────────────────────────────────

    defineMethod({
      name: 'git.status',
      params: ProjectWorktreeParam,
      handler: async (params, ctx) => {
        const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
        return relay.call('git.exec', {
          cwd: params.worktreePath,
          args: ['status', '--porcelain=v2', '--branch'],
        }) as Promise<GitExecResult>
      },
    }),

    // ── git.diff ────────────────────────────────────────────────────────────

    defineMethod({
      name: 'git.diff',
      params: ProjectWorktreeParam.extend({
        staged: z.boolean().optional(),
        files: z.array(z.string()).optional(),
      }),
      handler: async (params, ctx) => {
        const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
        const args = ['diff']
        if (params.staged) args.push('--staged')
        if (params.files?.length) args.push('--', ...params.files)
        return relay.call('git.exec', { cwd: params.worktreePath, args }) as Promise<GitExecResult>
      },
    }),

    // ── git.add ─────────────────────────────────────────────────────────────

    defineMethod({
      name: 'git.add',
      params: FilesParam,
      handler: async (params, ctx) => {
        const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
        return relay.call('git.exec', {
          cwd: params.worktreePath,
          args: ['add', '--', ...params.files],
        }) as Promise<GitExecResult>
      },
    }),

    // ── git.restore ─────────────────────────────────────────────────────────

    defineMethod({
      name: 'git.restore',
      params: FilesParam,
      handler: async (params, ctx) => {
        const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
        return relay.call('git.exec', {
          cwd: params.worktreePath,
          args: ['restore', '--staged', '--', ...params.files],
        }) as Promise<GitExecResult>
      },
    }),

    // ── git.commit ──────────────────────────────────────────────────────────

    defineMethod({
      name: 'git.commit',
      params: ProjectWorktreeParam.extend({
        message: z.string().min(1),
      }),
      handler: async (params, ctx) => {
        const userId = ctx.userId ?? ''
        const relay = await router.getRelayForProject(params.projectId, userId)
        const result = await relay.call('git.exec', {
          cwd: params.worktreePath,
          args: ['commit', '-m', params.message],
        }) as GitExecResult

        // Task auto-advance (non-fatal)
        autoAdvanceTasks(params.message, taskService, taskGrantService, userId).catch(() => {})

        return result
      },
    }),

    // ── git.push ────────────────────────────────────────────────────────────

    defineMethod({
      name: 'git.push',
      params: ProjectWorktreeParam.extend({
        remote: z.string().optional(),
        branch: z.string().optional(),
        force: z.boolean().optional(),
      }),
      handler: async (params, ctx) => {
        const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
        const args = ['push']
        if (params.force) args.push('--force-with-lease')
        if (params.remote) args.push(params.remote)
        if (params.branch) args.push(params.branch)
        return relay.call('git.execStream', { cwd: params.worktreePath, args }) as Promise<GitExecResult>
      },
    }),

    // ── git.pull ────────────────────────────────────────────────────────────

    defineMethod({
      name: 'git.pull',
      params: ProjectWorktreeParam.extend({
        remote: z.string().optional(),
        branch: z.string().optional(),
        rebase: z.boolean().optional(),
      }),
      handler: async (params, ctx) => {
        const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
        const args = ['pull']
        if (params.rebase) args.push('--rebase')
        if (params.remote) args.push(params.remote)
        if (params.branch) args.push(params.branch)
        return relay.call('git.execStream', { cwd: params.worktreePath, args }) as Promise<GitExecResult>
      },
    }),

    // ── git.fetch ───────────────────────────────────────────────────────────

    defineMethod({
      name: 'git.fetch',
      params: ProjectWorktreeParam.extend({
        remote: z.string().optional(),
        prune: z.boolean().optional(),
      }),
      handler: async (params, ctx) => {
        const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
        const args = ['fetch']
        if (params.prune) args.push('--prune')
        if (params.remote) args.push(params.remote)
        return relay.call('git.exec', { cwd: params.worktreePath, args }) as Promise<GitExecResult>
      },
    }),

    // ── git.branch.list ─────────────────────────────────────────────────────

    defineMethod({
      name: 'git.branch.list',
      params: ProjectWorktreeParam.extend({
        all: z.boolean().optional(),
      }),
      handler: async (params, ctx) => {
        const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
        const args = ['branch', '--format=%(refname:short)|%(upstream:short)|%(HEAD)|%(objectname:short)']
        if (params.all) args.splice(1, 0, '-a')
        return relay.call('git.exec', { cwd: params.worktreePath, args }) as Promise<GitExecResult>
      },
    }),

    // ── git.branch.create ────────────────────────────────────────────────────

    defineMethod({
      name: 'git.branch.create',
      params: ProjectWorktreeParam.extend({
        name: z.string().min(1).regex(/^[a-zA-Z0-9_/.-]+$/, 'Invalid branch name'),
        from: z.string().optional(),
      }),
      handler: async (params, ctx) => {
        const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
        const args = ['branch', params.name]
        if (params.from) args.push(params.from)
        return relay.call('git.exec', { cwd: params.worktreePath, args }) as Promise<GitExecResult>
      },
    }),

    // ── git.branch.delete ────────────────────────────────────────────────────

    defineMethod({
      name: 'git.branch.delete',
      params: ProjectWorktreeParam.extend({
        name: z.string().min(1),
        force: z.boolean().optional(),
      }),
      handler: async (params, ctx) => {
        const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
        const flag = params.force ? '-D' : '-d'
        return relay.call('git.exec', {
          cwd: params.worktreePath,
          args: ['branch', flag, params.name],
        }) as Promise<GitExecResult>
      },
    }),

    // ── git.checkout ────────────────────────────────────────────────────────

    defineMethod({
      name: 'git.checkout',
      params: ProjectWorktreeParam.extend({
        branch: z.string().min(1),
        create: z.boolean().optional(),
      }),
      handler: async (params, ctx) => {
        const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
        const args = ['checkout']
        if (params.create) args.push('-b')
        args.push(params.branch)
        return relay.call('git.exec', { cwd: params.worktreePath, args }) as Promise<GitExecResult>
      },
    }),

    // ── git.log ─────────────────────────────────────────────────────────────

    defineMethod({
      name: 'git.log',
      params: ProjectWorktreeParam.extend({
        limit: z.number().int().positive().max(200).optional(),
      }),
      handler: async (params, ctx) => {
        const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
        const n = params.limit ?? 20
        return relay.call('git.exec', {
          cwd: params.worktreePath,
          args: ['log', `--oneline`, `-${n}`, '--no-walk=sorted'],
        }) as Promise<GitExecResult>
      },
    }),

    // ── git.worktree.list ────────────────────────────────────────────────────

    defineMethod({
      name: 'git.worktree.list',
      params: ProjectWorktreeParam,
      handler: async (params, ctx) => {
        const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
        return relay.call('git.exec', {
          cwd: params.worktreePath,
          args: ['worktree', 'list', '--porcelain'],
        }) as Promise<GitExecResult>
      },
    }),

    // ── git.worktree.add ─────────────────────────────────────────────────────

    defineMethod({
      name: 'git.worktree.add',
      params: ProjectWorktreeParam.extend({
        path: z.string().min(1),
        branch: z.string().min(1),
      }),
      handler: async (params, ctx) => {
        const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
        return relay.call('git.exec', {
          cwd: params.worktreePath,
          args: ['worktree', 'add', params.path, params.branch],
        }) as Promise<GitExecResult>
      },
    }),

    // ── git.worktree.remove ──────────────────────────────────────────────────

    defineMethod({
      name: 'git.worktree.remove',
      params: ProjectWorktreeParam.extend({
        path: z.string().min(1),
        force: z.boolean().optional(),
      }),
      handler: async (params, ctx) => {
        const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')
        const args = ['worktree', 'remove', params.path]
        if (params.force) args.push('--force')
        return relay.call('git.exec', { cwd: params.worktreePath, args }) as Promise<GitExecResult>
      },
    }),

    // ── git.generateCommitMessage ────────────────────────────────────────────

    defineMethod({
      name: 'git.generateCommitMessage',
      params: ProjectWorktreeParam.extend({
        devServerId: z.string().min(1),
        userId: z.string().optional(),
        modelHint: z.string().optional(),
      }),
      handler: async (params, ctx) => {
        const userId = params.userId ?? ctx.userId ?? ''
        const relay = await router.getRelayForProject(params.projectId, userId)

        // Get staged diff
        const diffResult = await relay.call('git.exec', {
          cwd: params.worktreePath,
          args: ['diff', '--staged', '--no-color'],
        }) as GitExecResult

        if (!diffResult.stdout.trim()) {
          throw new Error('GIT_NO_STAGED_CHANGES')
        }

        // Build prompt and call AI via relay
        const prompt = [
          'You are a git commit message generator. Given the following diff, write a concise commit message.',
          'Format: <type>(<scope>): <subject> (max 72 chars on first line)',
          'Optionally add body paragraphs after blank line.',
          '',
          'Diff:',
          diffResult.stdout.slice(0, 8000), // limit context
        ].join('\n')

        const aiResult = await relay.call('ai.complete', { prompt, format: 'text' }) as { content?: string; text?: string }
        const message = (aiResult.content ?? aiResult.text ?? '').trim()
        if (!message) throw new Error('GIT_AI_EMPTY_RESPONSE')

        return { message }
      },
    }),

    // ── git.pr.create ────────────────────────────────────────────────────────

    defineMethod({
      name: 'git.pr.create',
      params: ProjectWorktreeParam.extend({
        title: z.string().min(1),
        body: z.string().optional(),
        base: z.string().min(1),
        draft: z.boolean().optional(),
        head: z.string().optional(),
      }),
      handler: async (params, ctx) => {
        const relay = await router.getRelayForProject(params.projectId, ctx.userId ?? '')

        // Use gh CLI via git.exec wrapper
        const ghArgs = [
          'pr', 'create',
          '--title', params.title,
          '--base', params.base,
        ]
        if (params.body) { ghArgs.push('--body', params.body) }
        if (params.draft) ghArgs.push('--draft')
        if (params.head) { ghArgs.push('--head', params.head) }

        // NOTE: gh is not a git subcommand, so we use a separate relay call
        const result = await relay.call('shell.exec', {
          cwd: params.worktreePath,
          cmd: 'gh',
          args: ghArgs,
        }) as { stdout?: string; stderr?: string; exitCode?: number }

        const prUrl = (result.stdout ?? '').trim()
        return { url: prUrl, exitCode: result.exitCode ?? 0 }
      },
    }),
  ]
}
