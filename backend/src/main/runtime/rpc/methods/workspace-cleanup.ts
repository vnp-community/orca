// Why: split by method, not a single relay-vs-direct choice — see each
// method's comment below.
//
// dismiss / clearDismissals / hasKillableLocalProcesses are DIRECT ports
// (same shape as onboarding.ts's Store-backed methods and cli.ts's relay
// methods combined): the UI-state persistence and PTY-liveness lookups they
// need already exist server-side (getActiveOnboardingStore(), runtime.
// hasTerminalsForWorktree, getRemotePtyProvider — the last of which is
// already keyed generically by connectionId and covers both SSH targets AND
// Dev Server Agent connections, per its own doc comment in ipc/pty.ts). No
// new agent-side handler or devServerId relay call is needed:
// DevServerPtyProvider.listProcesses() already relays to the agent's
// existing `pty.listProcesses` handler (agent/src/relay/pty-handler.ts).
//
// `scan` is INTENTIONALLY NOT ported here. Desktop's scanWorkspaceCleanup
// (desktop/src/main/ipc/workspace-cleanup-scan.ts) is not a self-contained
// operation — it walks `store.getRepos()` and pulls in
// workspace-cleanup-scan-primitives.ts, workspace-cleanup-candidate.ts,
// workspace-cleanup-activity.ts, and workspace-cleanup-disconnected-ssh.ts
// (worktree activity heuristics, git-dirty/ahead-behind checks, candidate
// fingerprinting/tiering). None of those exist in backend/src/main yet, even
// though the lower-level building blocks they'd need (repo-worktrees.ts,
// worktree-metadata-merge.ts, ssh-git-dispatch.ts, store.getRepos()) do.
// Porting `scan` means porting that whole candidate-building subsystem, not
// writing one relay/direct RPC method — treated as a separate, larger
// follow-up rather than forced into this pass.
import { z } from 'zod'
import { defineMethod, type RpcAnyMethod } from '../core'
import { OptionalString, requiredNumber, requiredString } from '../schemas'
import type { WorkspaceCleanupLocalProcessResult } from '../../../../shared/workspace-cleanup'
import { getActiveOnboardingStore } from '../../../ipc/onboarding-ipc'
import { getRemotePtyProvider } from '../../../ipc/pty'
import { listRegisteredPtys } from '../../../memory/pty-registry'
import type { RpcContext } from '../core'

const WorkspaceCleanupDismissalSchema = z.object({
  worktreeId: requiredString('Missing worktreeId'),
  dismissedAt: requiredNumber('Missing dismissedAt'),
  fingerprint: requiredString('Missing fingerprint'),
  classifierVersion: requiredNumber('Missing classifierVersion')
})

const WorkspaceCleanupDismissParams = z.object({
  dismissals: z.array(WorkspaceCleanupDismissalSchema).optional()
})

const WorkspaceCleanupLocalProcessParams = z.object({
  worktreeId: requiredString('Missing worktreeId'),
  connectionId: OptionalString.nullable().optional(),
  worktreePath: OptionalString
})

// Why: shared by this method so accept/persist shape matches desktop's
// mergeWorkspaceCleanupDismissals (ipc/workspace-cleanup.ts) exactly.
function mergeWorkspaceCleanupDismissals(
  current: Record<string, z.infer<typeof WorkspaceCleanupDismissalSchema>>,
  dismissals: z.infer<typeof WorkspaceCleanupDismissalSchema>[] | undefined
): Record<string, z.infer<typeof WorkspaceCleanupDismissalSchema>> {
  const next = { ...current }
  for (const dismissal of dismissals ?? []) {
    if (dismissal) {
      next[dismissal.worktreeId] = dismissal
    }
  }
  return next
}

// Why: direct port of desktop's hasKillableProcesses + hasKillableSshProcesses
// (ipc/workspace-cleanup.ts), collapsed into one function since backend has
// no local-vs-SSH-provider split at the call site — getRemotePtyProvider
// already covers both SSH targets and Dev Server Agent connections.
async function hasKillableProcesses(
  params: { worktreeId: string; connectionId?: string | null; worktreePath?: string },
  runtime: RpcContext['runtime']
): Promise<boolean | null> {
  const { worktreeId } = params
  if (worktreeId.length === 0) {
    return false
  }

  let livenessUnknown = false
  try {
    if (await runtime.hasTerminalsForWorktree(worktreeId)) {
      return true
    }
  } catch {
    livenessUnknown = true
  }

  if (params.connectionId) {
    const provider = getRemotePtyProvider(params.connectionId)
    if (!provider) {
      return null
    }
    try {
      const worktreePath = (params.worktreePath ?? '').replace(/\\/g, '/').replace(/\/+$/, '')
      const sessions = await provider.listProcesses()
      const matches = sessions.some((session) => {
        if (session.id.startsWith(`${params.worktreePath ?? ''}@@`)) {
          return true
        }
        const sessionCwd = session.cwd.replace(/\\/g, '/').replace(/\/+$/, '')
        return (
          worktreePath.length > 0 &&
          (sessionCwd === worktreePath || sessionCwd.startsWith(`${worktreePath}/`))
        )
      })
      if (matches) {
        return true
      }
      return livenessUnknown ? null : false
    } catch {
      return null
    }
  }

  const registryPtyIds = new Set(
    listRegisteredPtys()
      .filter((entry) => entry.worktreeId === worktreeId)
      .map((entry) => entry.ptyId)
  )
  const provider = runtime.getLocalProvider()
  if (!provider) {
    return registryPtyIds.size > 0 ? true : null
  }
  try {
    const prefix = `${worktreeId}@@`
    const sessions = await provider.listProcesses()
    if (sessions.some((session) => session.id.startsWith(prefix) || registryPtyIds.has(session.id))) {
      return true
    }
    return livenessUnknown ? null : false
  } catch {
    return registryPtyIds.size > 0 ? true : null
  }
}

export const WORKSPACE_CLEANUP_METHODS: readonly RpcAnyMethod[] = [
  defineMethod({
    name: 'workspaceCleanup.dismiss',
    params: WorkspaceCleanupDismissParams,
    handler: async (params): Promise<void> => {
      const store = getActiveOnboardingStore()
      if (!store) {throw new Error('runtime_unavailable')}
      const current = store.getUI().workspaceCleanup?.dismissals ?? {}
      store.updateUI({
        workspaceCleanup: { dismissals: mergeWorkspaceCleanupDismissals(current, params.dismissals) }
      })
    }
  }),
  defineMethod({
    name: 'workspaceCleanup.clearDismissals',
    params: null,
    handler: async (): Promise<void> => {
      const store = getActiveOnboardingStore()
      if (!store) {throw new Error('runtime_unavailable')}
      store.updateUI({ workspaceCleanup: { dismissals: {} } })
    }
  }),
  defineMethod({
    name: 'workspaceCleanup.hasKillableLocalProcesses',
    params: WorkspaceCleanupLocalProcessParams,
    handler: async (params, { runtime }): Promise<WorkspaceCleanupLocalProcessResult> => ({
      hasKillableProcesses: await hasKillableProcesses(params, runtime)
    })
  })
]
