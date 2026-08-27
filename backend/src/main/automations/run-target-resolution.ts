// ADR-021 Phase 1: narrowed from `Store` to the methods actually used
// (getRepo, getProjectHostSetups) — see automation-store-dependency.ts's
// module doc comment. Both are async on the interface (Postgres I/O), so this
// whole module is async now too — `Store`'s synchronous versions still work
// fine through `wrapStoreAsAutomationStoreDependency()`'s Promise.resolve()
// wrapping (automation-store-store-adapter.ts).
import type { AutomationStoreDependency } from './automation-store-dependency'
import type { Automation } from '../../shared/automations-types'
import { getAutomationLegacyRepoId } from '../../shared/automation-run-identity'
import { getRepoExecutionHostId, parseExecutionHostId } from '../../shared/execution-host'
import type { ProjectHostSetup, Repo } from '../../shared/types'
import { splitWorktreeIdForFilesystem } from '../../shared/worktree-id'

export type AutomationRunTargetResult =
  | { ok: true; cwd: string; repo: Repo; setup?: ProjectHostSetup }
  | { ok: false; error: string }

type AutomationRunTargetOptions = {
  allowRemoteHostScheduling?: boolean
}

async function getLegacyPrecheckCwd(
  store: AutomationStoreDependency,
  automation: Automation
): Promise<string | null> {
  if (automation.workspaceMode === 'existing') {
    const parsed = automation.workspaceId
      ? splitWorktreeIdForFilesystem(automation.workspaceId)
      : null
    return parsed?.worktreePath ?? null
  }
  const repo = await store.getRepo(getAutomationLegacyRepoId(automation))
  return repo?.path ?? null
}

export async function resolveAutomationRunTarget(
  store: AutomationStoreDependency,
  automation: Automation,
  options: AutomationRunTargetOptions = {}
): Promise<AutomationRunTargetResult> {
  const context = automation.runContext ?? null
  if (!context) {
    const repo = await store.getRepo(getAutomationLegacyRepoId(automation))
    const cwd = await getLegacyPrecheckCwd(store, automation)
    if (!repo || !cwd) {
      return { ok: false, error: 'Automation run target is no longer available.' }
    }
    return { ok: true, cwd, repo }
  }
  const parsedHost = parseExecutionHostId(context.hostId)
  if (
    parsedHost?.kind === 'runtime' &&
    (!options.allowRemoteHostScheduling || automation.schedulerOwner !== 'remote_host_service')
  ) {
    return {
      ok: false,
      error:
        'Remote-server automation scheduling is not available from this Orca client yet. Run this automation on the remote server or update Orca when durable remote scheduling is available.'
    }
  }

  const projectHostSetups = await store.getProjectHostSetups()
  const setup = projectHostSetups.find((candidate) => candidate.id === context.projectHostSetupId)
  if (!setup) {
    return {
      ok: false,
      error: 'Project is not set up on the selected automation host anymore.'
    }
  }
  if (setup.setupState !== 'ready') {
    return {
      ok: false,
      error: `Project setup on the selected automation host is ${setup.setupState}.`
    }
  }
  // Why: projectId is a derived identity that upgrades over time (repo:→git:→github:);
  // matching on it strands automations created before their repo's identity resolved.
  // Anchor on repoId/hostId/path instead — the durable, stable target identity.
  if (setup.hostId !== context.hostId || setup.repoId !== context.repoId) {
    return {
      ok: false,
      error: 'Automation run target no longer matches the selected project host setup.'
    }
  }

  const repo = await store.getRepo(context.repoId)
  if (!repo) {
    return {
      ok: false,
      error: 'Repository for the selected automation host is no longer available.'
    }
  }
  if (getRepoExecutionHostId(repo) !== context.hostId) {
    return {
      ok: false,
      error: 'Repository is no longer attached to the selected automation host.'
    }
  }
  if (repo.path !== setup.path || context.path !== setup.path) {
    return {
      ok: false,
      error: 'Project path for the selected automation host has changed.'
    }
  }

  return { ok: true, cwd: setup.path, repo, setup }
}
