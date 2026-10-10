/**
 * Data the live UI flow creates behind the UI's back: a scratch repo and a project bound to the dev server. The UI cannot
 * browse the dev server's disk from a headless run, so this is the one non-UI setup step. Everything carries the run prefix
 * and is removed by `dispose()` (open Requests are cancelled first so none is orphaned).
 */
import { execFileSync } from 'node:child_process'
import { mkdtempSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import type { GatewayRpcClient, RpcRow } from './gateway-rpc-client'
import type { LiveStackConfig } from './live-stack-config'

export class LiveStackNotReady extends Error {}

export type LiveProject = {
  id: string
  name: string
  devServerId: string
  /** Cancels still-open Requests of the project, deletes it and its scratch repo. Safe to call twice. */
  dispose: () => Promise<string[]>
}

function makeScratchRepo(prefix: string): string {
  const path = mkdtempSync(join(tmpdir(), `${prefix}-repo-`))
  execFileSync('git', ['init', '-q', '-b', 'main'], { cwd: path })
  execFileSync(
    'git',
    [
      '-c',
      'user.name=e2e',
      '-c',
      'user.email=e2e@example.com',
      'commit',
      '-q',
      '--allow-empty',
      '-m',
      'init'
    ],
    { cwd: path }
  )
  return path
}

export async function provisionLiveProject(
  rpc: GatewayRpcClient,
  cfg: LiveStackConfig,
  // Why: the real-AI specs bring their own fixture repo and remove it themselves after checking it.
  ownRepoPath = ''
): Promise<LiveProject> {
  // Why: after a gateway restart the agent needs a few seconds to reattach; wait instead of failing the run.
  let server: RpcRow | undefined
  for (let i = 0; i < 20; i++) {
    const servers = await rpc.call<RpcRow[]>('devServer.list', {})
    server = servers.find((s) => s.name === cfg.devServerName)
    if (!server || server.status === 'connected') {
      break
    }
    await new Promise((r) => setTimeout(r, 2_000))
  }
  if (!server) {
    throw new LiveStackNotReady(
      `no dev server named '${cfg.devServerName}' (start the agent: deploy/dev/scripts/start-local-agent.sh)`
    )
  }
  if (server.approvalStatus !== 'approved' || server.status !== 'connected') {
    throw new LiveStackNotReady(
      `dev server '${cfg.devServerName}' is ${server.approvalStatus}/${server.status}; it must be approved and connected`
    )
  }
  const repoPath = ownRepoPath || cfg.repoPath || makeScratchRepo(cfg.runPrefix)
  const ownsRepo = !cfg.repoPath && !ownRepoPath
  const name = `${cfg.runPrefix}-project`
  const project = await rpc.call<{ id: string; devServerId?: string }>('project.create', {
    name,
    description: 'orca live UI flow',
    devServerId: server.id,
    repoPath,
    defaultBranch: 'main',
    visibility: 'private'
  })
  let disposed = false
  return {
    id: project.id,
    name,
    devServerId: server.id,
    dispose: async () => {
      if (disposed) {
        return []
      }
      disposed = true
      const problems: string[] = []
      // Why: a task the agent was still running (or that was just killed) blocks cancel and delete until task-service hears back.
      // Why: a task whose dispatch went nowhere (no agent run behind it) stays in_progress forever; after 30 s it is cancelled so cleanup can finish.
      const releaseStuckTasks = async (): Promise<void> => {
        const tasks = await rpc.call<{ tasks?: RpcRow[] }>('task.list', {
          projectId: project.id,
          taskTypes: ['plan', 'phase', 'task', 'bug', 'feature', 'epic'],
          pageSize: 200
        })
        for (const t of tasks.tasks ?? []) {
          if (t.status === 'in_progress') {
            await rpc.call('task.update', { id: t.id, status: 'cancelled' }).catch(() => {})
            problems.push(
              `released stuck task ${t.id} (${String(t.type)} "${String(t.title).slice(0, 40)}") by cancelling it`
            )
          }
        }
      }
      const retrying = async (label: string, fn: () => Promise<unknown>): Promise<void> => {
        for (let attempt = 0; ; attempt++) {
          try {
            await fn()
            return
          } catch (e) {
            const msg = (e as Error).message
            if (
              attempt === 10 &&
              /ACTIVE_EXECUTION|ACTIVE_WORKFLOWS|still has running tasks/.test(msg)
            ) {
              await releaseStuckTasks().catch(() => {})
            }
            if (
              attempt < 30 &&
              /ACTIVE_EXECUTION|ACTIVE_WORKFLOWS|still has running tasks/.test(msg)
            ) {
              await new Promise((r) => setTimeout(r, 3_000))
              continue
            }
            problems.push(`${label}: ${msg.slice(0, 120)}`)
            return
          }
        }
      }
      try {
        const listed = await rpc.call<{ requests?: RpcRow[] }>('request.list', {
          projectId: project.id
        })
        for (const r of listed.requests ?? []) {
          if (r.status !== 'completed' && r.status !== 'cancelled') {
            await retrying(`cancel ${r.id}`, async () => {
              const fresh = (await rpc.call<{ request: RpcRow }>('request.get', { id: r.id }))
                .request
              if (fresh.status !== 'completed' && fresh.status !== 'cancelled') {
                await rpc.call('request.cancel', {
                  id: r.id,
                  reason: 'e2e-ui cleanup',
                  expectedVersion: fresh.version
                })
              }
            })
          }
        }
      } catch (e) {
        problems.push(`list requests: ${(e as Error).message.slice(0, 120)}`)
      }
      await retrying('delete project', () => rpc.call('project.delete', { projectId: project.id }))
      if (ownsRepo) {
        rmSync(repoPath, { recursive: true, force: true })
      }
      return problems
    }
  }
}
