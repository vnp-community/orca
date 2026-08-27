import type { FeatureInteractionState } from '../../../../shared/feature-interactions'
import { hasFeatureInteraction } from '../../../../shared/feature-interactions'
import {
  FEATURE_WALL_SETUP_STEPS,
  isAddDevServerRepoComplete,
  isConnectDevServerComplete,
  type FeatureWallSetupStepId
} from '../../../../shared/feature-wall-setup-steps'
import type { DevServer } from '../../../../shared/dev-server-types'
import type { GlobalSettings, Repo, Worktree } from '../../../../shared/types'

export type FeatureWallSetupProgressInput = {
  ready?: boolean
  settings: GlobalSettings | null
  featureInteractions: FeatureInteractionState
  hasConnectedTaskSource: boolean
  browserUseSkillInstalled: boolean
  computerUseSkillInstalled: boolean
  computerUsePermissionsReady: boolean
  computerUseUnavailable?: boolean
  orchestrationSkillInstalled: boolean
  gitRepoCount: number
  worktreesByRepo: Record<string, Worktree[]>
  hasSetupScript: boolean
  devServers: DevServer[]
  // Why optional: only 'connect-dev-server' is a displayed step today (see
  // FEATURE_WALL_SETUP_STEPS) — 'add-dev-server-repo' isn't wired to any UI
  // yet, so callers that don't have this data on hand can omit it and get a
  // conservative "not done" default instead of threading it through unused.
  repos?: Repo[]
  activeDevServerId?: string | null
}

export type FeatureWallSetupProgress = {
  ready: boolean
  stepDone: Record<FeatureWallSetupStepId, boolean>
  coreDoneCount: number
  coreTotal: number
}

function countAvailableNonMainWorktrees(worktreesByRepo: Record<string, Worktree[]>): number {
  // Why: imported git worktrees count as real parallel-work capacity, but
  // partially hydrated placeholders can appear before a worktree path is known.
  return Object.values(worktreesByRepo).reduce(
    (sum, worktrees) =>
      sum +
      worktrees.filter(
        (worktree) => !worktree.isMainWorktree && typeof worktree.path === 'string' && worktree.path
      ).length,
    0
  )
}

export function getFeatureWallSetupProgress(
  input: FeatureWallSetupProgressInput
): FeatureWallSetupProgress {
  const agentCapabilitiesDone =
    input.browserUseSkillInstalled &&
    input.computerUseSkillInstalled &&
    (input.computerUsePermissionsReady || input.computerUseUnavailable === true) &&
    input.orchestrationSkillInstalled
  const stepDone: Record<FeatureWallSetupStepId, boolean> = {
    'connect-dev-server': isConnectDevServerComplete(input.devServers),
    // Why: not currently a displayed step (see FEATURE_WALL_SETUP_STEPS) — the
    // scaffolding from TASK-040 already had this helper ready, so compute it
    // correctly rather than a placeholder, for whenever it gets surfaced.
    'add-dev-server-repo': isAddDevServerRepoComplete(
      input.repos ?? [],
      input.activeDevServerId ?? null
    ),
    'default-agent':
      Boolean(input.settings?.defaultTuiAgent) && input.settings?.defaultTuiAgent !== 'blank',
    'add-two-repos': input.gitRepoCount >= 2,
    notifications:
      input.settings?.notifications.enabled === true &&
      input.settings.notifications.agentTaskComplete === true,
    'two-worktrees': countAvailableNonMainWorktrees(input.worktreesByRepo) >= 1,
    // Why: the 'browser' interaction fires when a non-blank page is viewed, so
    // opening any real page in Orca's browser durably completes this milestone.
    browser: hasFeatureInteraction(input.featureInteractions, 'browser'),
    'task-sources': input.hasConnectedTaskSource,
    'agent-capabilities': agentCapabilitiesDone,
    'setup-script': input.hasSetupScript
  }
  return {
    ready: input.ready ?? true,
    stepDone,
    coreDoneCount: FEATURE_WALL_SETUP_STEPS.filter((step) => stepDone[step.id]).length,
    coreTotal: FEATURE_WALL_SETUP_STEPS.length
  }
}
