/* eslint-disable max-lines -- Why: straight extraction of orca-runtime.ts's
managed-worktree creation/activation method cluster (activateManagedWorktree,
createManagedWorktree, createManagedRemoteWorktree, and their dedicated
startup/provisioning private helpers) plus the module-scope helper functions
that exclusively serve them, already covered by orca-runtime.ts's own
grandfathered max-lines disable before this move. Registered in
config/max-lines-baseline.txt per AGENTS.md — NEEDS PR REVIEW. This is the
PTY-lifecycle-core domain the user explicitly chose to defer earlier in the
BUG-FE-BIGFILE-002 effort (zero test coverage) — extracted here only after
exhaustive `this.X` dependency inventory + manual read-through of the full
~1,860-line block, per the user's explicit acceptance of that risk. */
// frontend/src/main/runtime/orca-runtime-worktree-creation.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-049): managed-worktree creation/
// activation commands extracted from OrcaRuntimeService via the composition
// pattern. This is the single largest and most PTY-entangled domain moved
// off orca-runtime.ts in this effort — createManagedWorktree alone is
// ~990 lines. Deliberately excludes the PTY-adjacent
// stopTerminalsForWorktree cluster and the small scattered managed-worktree
// list/show/sleep cluster (see TASK-BIGFILE-048's notes) — those stay in
// orca-runtime.ts.
import { randomUUID } from 'node:crypto'
import { stat } from 'node:fs/promises'
import type { BrowserWindow } from 'electron'
import type {
  AutomationWorkspaceProvenance,
  CreateWorktreeResult,
  GitPushTarget,
  GlobalSettings,
  Repo,
  TuiAgent,
  WorkspaceCreateTelemetrySource,
  WorkspaceLineage,
  Worktree,
  WorktreeLineage,
  WorktreeLineageWarning,
  WorktreeStartupLaunch
} from '../../shared/types'
import type { RuntimeTerminalCreate, RuntimeTerminalSplit } from '../../shared/runtime-types'
import type { RemoteFetchResult, RemoteTrackingBase } from './orca-runtime-types'
import type { AddWorktreeOptions, AddWorktreeResult } from '../git/worktree'
import type { Store } from '../persistence'
import type { TerminalPaneSplitSource } from '../../shared/feature-education-telemetry'
import type { ForgeProviderId } from '../source-control/forge-provider'
import { getHostedReviewForBranch as getHostedReviewForBranchFromRepo } from '../source-control/hosted-review'
import { WORKTREE_CREATE_MAX_SUFFIX_ATTEMPTS } from '../worktree-create-candidates'
import { gitExecFileAsync } from '../git/runner'
import { isENOENT, invalidateAuthorizedRootsCache } from '../ipc/filesystem-auth'
import { isShellProcess } from '../../shared/agent-detection'
import { repoIsRemote } from '../../shared/agent-launch-remote'
import { isExpectedAgentProcess } from '../../shared/agent-process-recognition'
import { isWindowsAbsolutePathLike } from '../../shared/cross-platform-path'
import { createDraftPasteReadyScanner } from '../../shared/draft-paste-ready-scanner'
import { getRepoExecutionHostId, getRepoProviderConnectionKey } from '../../shared/execution-host'
import { getProjectHostSetupWorktreeMeta } from '../../shared/project-host-setup-projection'
import { isFolderRepo } from '../../shared/repo-kind'
import { createSequencedSetupAgentCommands } from '../../shared/setup-agent-sequencing'
import {
  buildSetupRunnerCommand,
  getSetupRunnerCommandPlatformForPath
} from '../../shared/setup-runner-command'
import { TUI_AGENT_CONFIG, isTuiAgent } from '../../shared/tui-agent-config'
import {
  resolveTuiAgentLaunchArgs,
  resolveTuiAgentLaunchEnv
} from '../../shared/tui-agent-launch-defaults'
import { isTuiAgentEnabled, pickTuiAgent } from '../../shared/tui-agent-selection'
import { buildAgentDraftLaunchPlan, buildAgentStartupPlan } from '../../shared/tui-agent-startup'
import { resolveLocalWindowsAgentStartupShell } from '../../shared/windows-terminal-shell'
import { worktreeWorkspaceKey } from '../../shared/workspace-scope'
import { resolveWorktreeAddBaseRef } from '../../shared/worktree-base-ref'
import { FOLDER_WORKSPACE_INSTANCE_SEPARATOR } from '../../shared/worktree-id'
import {
  markCodexProjectTrusted,
  markCopilotFolderTrusted,
  markCursorWorkspaceTrusted
} from '../agent-trust-presets'
import { hasCommitObjectViaGitExec } from '../git/commit-object-ref'
import { resolveLocalGitUsername } from '../git/git-username'
import {
  getBaseRefDefault,
  getBranchConflictKind,
  resolveDefaultBaseRefWithLocalGit
} from '../git/repo'
import { addSparseWorktree, addWorktree, listWorktrees } from '../git/worktree'
import { hasWorktreeBaseCommitRef } from '../git/worktree-base-ref-probe'
import { getPRForBranch } from '../github/client'
import {
  createSetupRunnerScript,
  getDefaultTabsLaunch,
  getEffectiveHooks,
  loadHooks,
  runHook,
  shouldRunSetupForCreate
} from '../hooks'
import { detectInstalledAgentsWithShellPathHydration, detectRemoteAgents } from '../ipc/preflight'
import { normalizeSparseDirectories } from '../ipc/sparse-checkout-directories'
import {
  areWorktreePathsEqual,
  computeBranchName,
  computeWorkspaceRoot,
  computeWorktreePath,
  ensurePathWithinWorkspace,
  getWorktreeCreationLayout,
  getWorktreePathSettings,
  mergeWorktree,
  sanitizeWorktreeName,
  shouldSetDisplayName
} from '../ipc/worktree-logic'
import {
  configureCreatedWorktreePushTarget,
  createRemoteWorktree,
  prepareWorktreePushTarget
} from '../ipc/worktree-remote'
import { createWorktreeLinkedPaths } from '../ipc/worktree-symlinks'
import {
  getLocalProjectGitExecOptions,
  getLocalProjectWorktreeGitOptions
} from '../project-runtime-git-options'
import { markRemoteAgentWorkspaceTrusted } from '../remote-agent-trust-presets'
import { resolveWorktreeCreateBase } from '../worktree-create-base'
import { prefetchWorktreeCreateBase } from '../worktree-create-base-prefetch'
import {
  getBranchNameOverrideCandidate,
  getWorktreeCreateCandidate
} from '../worktree-create-candidates'
import { normalizeLocalBranchName } from './orca-runtime-tail-buffer'
import type {
  ResolvedWorktree,
  RuntimePtyWorktreeRecord,
  RuntimeStore,
  TerminalCreateOptions,
  TerminalHandleRecord,
  WorktreeLineageInput,
  WorktreeLineageResolution
} from './orca-runtime'
import { mergeRuntimeFolderWorkspace, getRuntimeFolderWorkspaceRootId } from './orca-runtime'
import type { RuntimePtyController } from './orca-runtime-types'

const BRACKETED_PASTE_BEGIN = '\x1b[200~'
const BRACKETED_PASTE_END = '\x1b[201~'
const BRACKETED_PASTE_QUIET_MS = 1500
const DRAFT_PASTE_READY_TIMEOUT_MS = 8000

type RuntimeWorktreeCreationNotifier = {
  resumeSleepingAgents?(worktreeId: string): void
}

export type RuntimeWorktreeCreationCommandHost = {
  getStore(): RuntimeStore | null
  requireStore(): Store
  getPtyController(): RuntimePtyController | null
  getNotifier(): RuntimeWorktreeCreationNotifier | null
  createTerminal(
    worktreeSelector?: string,
    opts?: TerminalCreateOptions
  ): Promise<RuntimeTerminalCreate>
  notifyActivateWorktree(
    repoId: string,
    worktreeId: string,
    setup?: CreateWorktreeResult['setup'],
    startup?: WorktreeStartupLaunch,
    defaultTabs?: CreateWorktreeResult['defaultTabs']
  ): void
  notifyWorktreesChanged(repoId: string): void
  invalidateResolvedWorktreeCache(): void
  hasRemoteTrackingRef(
    repoPath: string,
    base: RemoteTrackingBase,
    gitOptions?: { wslDistro?: string }
  ): Promise<boolean>
  resolveRepoSelector(selector: string): Promise<Repo>
  resolveRemoteTrackingBase(
    repoPath: string,
    baseBranch: string,
    gitOptions?: { wslDistro?: string }
  ): Promise<RemoteTrackingBase | null>
  getLocalGitExecutionOptionArgs(repo: Repo): [] | [{ wslDistro?: string }]
  getLivePtyForHandle(handle: string): {
    record: TerminalHandleRecord
    pty: RuntimePtyWorktreeRecord
  } | null
  getAgentLaunchPlatformForRepo(repo: Repo): NodeJS.Platform
  resolveWorktreeSelector(selector: string): Promise<ResolvedWorktree>
  subscribeToTerminalData(
    ptyId: string,
    listener: (data: string, meta?: { seq?: number; rawLength?: number; cwd?: string }) => void
  ): () => void
  splitTerminal(
    handle: string,
    opts?: {
      direction?: 'horizontal' | 'vertical'
      command?: string
      env?: Record<string, string>
      envToDelete?: string[]
      activate?: boolean
      telemetrySource?: TerminalPaneSplitSource
    }
  ): Promise<RuntimeTerminalSplit>
  setMobileSessionTabProps(
    worktreeSelector: string,
    args: {
      tabId: string
      color?: string | null
      isPinned?: boolean
      viewMode?: 'terminal' | 'chat'
    }
  ): Promise<{ updated: true }>
  resolveLineageForWorktreeCreate(input?: WorktreeLineageInput): Promise<WorktreeLineageResolution>
  refreshMobileSessionPtyRecords(): Promise<void>
  notifyMobileSessionTabsChanged(worktreeId?: string): void
  hydrateHeadlessMobileSessionTabsFromWorkspaceSession(
    worktreeId?: string,
    options?: {
      force?: boolean
      allowAttachedWindow?: boolean
      onlyServeOwnedTerminals?: boolean
    }
  ): void
  getGraphAuthoritativeWindowId(): number | null
  getOrStartRemoteTrackingBaseRefresh(
    repoPath: string,
    base: RemoteTrackingBase,
    gitOptions?: { wslDistro?: string }
  ): Promise<RemoteFetchResult>
  getHostedReviewExecutionOptions(
    repo: Repo
  ): { localGitExecOptions: { wslDistro?: string } } | undefined
  getAvailableAuthoritativeWindow(): BrowserWindow | null
  fetchRemoteWithCache(
    repoPath: string,
    remote: string,
    gitOptions?: { wslDistro?: string }
  ): Promise<void>
  assertGraphReady(): void
  getRecentPtyOutput(ptyId: string): string | undefined
}

function getRuntimeFolderWorkspaceInstanceId(repo: Repo, instanceId: string): string {
  return `${getRuntimeFolderWorkspaceRootId(repo)}${FOLDER_WORKSPACE_INSTANCE_SEPARATOR}${instanceId}`
}
async function resolveCreateBranchName(
  repoPath: string,
  branchNameOverride: string | undefined,
  sanitizedName: string,
  settings: { branchPrefix: string; branchPrefixCustom?: string },
  username: string | null,
  gitOptions: { wslDistro?: string } = {}
): Promise<string> {
  if (!branchNameOverride) {
    return computeBranchName(sanitizedName, settings, username)
  }
  if (branchNameOverride.startsWith('-')) {
    throw new Error('Branch name must not start with "-"')
  }
  await gitExecFileAsync(['check-ref-format', '--branch', branchNameOverride], {
    cwd: repoPath,
    ...gitOptions
  })
  return branchNameOverride
}
async function canCheckoutExistingLocalBranch(
  repoPath: string,
  branchName: string,
  baseBranch: string,
  gitOptions: { wslDistro?: string } = {}
): Promise<boolean> {
  let localHead = ''
  try {
    const { stdout } = await gitExecFileAsync(
      ['rev-parse', '--verify', '--quiet', `refs/heads/${branchName}^{commit}`],
      {
        cwd: repoPath,
        ...gitOptions
      }
    )
    localHead = stdout.trim()
  } catch {
    return false
  }
  if (normalizeLocalBranchName(baseBranch) !== branchName) {
    if (!localHead) {
      return false
    }
    try {
      const { stdout } = await gitExecFileAsync(
        ['rev-parse', '--verify', '--quiet', `${baseBranch}^{commit}`],
        { cwd: repoPath, ...gitOptions }
      )
      if (stdout.trim() !== localHead) {
        return false
      }
    } catch {
      return false
    }
  }
  const worktrees = await listWorktrees(repoPath, gitOptions)
  return !worktrees.some((worktree) => normalizeLocalBranchName(worktree.branch) === branchName)
}
function hasLocalGitOptions(gitOptions: { wslDistro?: string }): boolean {
  return Object.keys(gitOptions).length > 0
}

function getLocalGitHubPrForBranch(
  repoPath: string,
  branchName: string,
  gitOptions: { wslDistro?: string }
): ReturnType<typeof getPRForBranch> {
  return hasLocalGitOptions(gitOptions)
    ? getPRForBranch(repoPath, branchName, null, null, null, {
        localGitExecOptions: gitOptions
      })
    : getPRForBranch(repoPath, branchName)
}
type SelectedReviewBranchInput = {
  branchNameOverride?: string
  linkedPR?: number | null
  linkedGitLabMR?: number | null
  linkedBitbucketPR?: number | null
  linkedAzureDevOpsPR?: number | null
  linkedGiteaPR?: number | null
  pushTarget?: GitPushTarget
}

type SelectedReviewBranch = {
  provider: ForgeProviderId
  number: number
}

function getSelectedReviewBranch(args: SelectedReviewBranchInput): SelectedReviewBranch | null {
  if (typeof args.linkedPR === 'number') {
    return { provider: 'github', number: args.linkedPR }
  }
  if (typeof args.linkedGitLabMR === 'number') {
    return { provider: 'gitlab', number: args.linkedGitLabMR }
  }
  if (typeof args.linkedBitbucketPR === 'number') {
    return { provider: 'bitbucket', number: args.linkedBitbucketPR }
  }
  if (typeof args.linkedAzureDevOpsPR === 'number') {
    return { provider: 'azure-devops', number: args.linkedAzureDevOpsPR }
  }
  if (typeof args.linkedGiteaPR === 'number') {
    return { provider: 'gitea', number: args.linkedGiteaPR }
  }
  return null
}

function isSelectedGitHubPrBranchOverride(
  args: SelectedReviewBranchInput,
  branchName: string
): boolean {
  return typeof args.linkedPR === 'number' && args.branchNameOverride === branchName
}

function isSelectedReviewBranchOverride(
  args: SelectedReviewBranchInput,
  branchName: string
): boolean {
  return getSelectedReviewBranch(args) !== null && args.branchNameOverride === branchName
}

function isMatchingSelectedGitHubPr(
  existingPR: Awaited<ReturnType<typeof getPRForBranch>>,
  args: SelectedReviewBranchInput,
  branchName: string
): boolean {
  return Boolean(
    existingPR &&
    isSelectedGitHubPrBranchOverride(args, branchName) &&
    existingPR.number === args.linkedPR
  )
}

function isAllowedPushTargetRemoteConflict(
  conflictKind: 'local' | 'remote' | null,
  branchName: string,
  args: SelectedReviewBranchInput
): boolean {
  return (
    conflictKind === 'remote' &&
    isSelectedReviewBranchOverride(args, branchName) &&
    args.pushTarget?.branchName === branchName
  )
}

function getSelectedReviewLookupHints(args: SelectedReviewBranchInput): {
  linkedGitHubPR?: number | null
  linkedGitLabMR?: number | null
  linkedBitbucketPR?: number | null
  linkedAzureDevOpsPR?: number | null
  linkedGiteaPR?: number | null
} {
  return {
    linkedGitHubPR: args.linkedPR ?? null,
    linkedGitLabMR: args.linkedGitLabMR ?? null,
    linkedBitbucketPR: args.linkedBitbucketPR ?? null,
    linkedAzureDevOpsPR: args.linkedAzureDevOpsPR ?? null,
    linkedGiteaPR: args.linkedGiteaPR ?? null
  }
}

async function getSelectedHostedReviewForBranch(
  repo: Pick<Repo, 'path' | 'connectionId'>,
  branchName: string,
  args: SelectedReviewBranchInput,
  executionOptions: { localGitExecOptions?: { wslDistro?: string } } = {}
): Promise<{ matchesSelected: boolean; number: number } | null> {
  const selectedReview = getSelectedReviewBranch(args)
  if (!selectedReview) {
    return null
  }
  const review = await getHostedReviewForBranchFromRepo({
    repoPath: repo.path,
    connectionId: repo.connectionId ?? null,
    branch: branchName,
    ...executionOptions,
    ...getSelectedReviewLookupHints(args)
  })
  if (!review) {
    return null
  }
  return {
    matchesSelected:
      review.provider === selectedReview.provider && review.number === selectedReview.number,
    number: review.number
  }
}
async function pathExists(pathValue: string): Promise<boolean> {
  try {
    await stat(pathValue)
    return true
  } catch (error) {
    if (isENOENT(error)) {
      return false
    }
    throw error
  }
}
async function hasLocalWorktreeBaseRef(
  repoPath: string,
  baseRef: string,
  options: { wslDistro?: string } = {}
): Promise<boolean> {
  const refExists = (qualifiedRef: string) =>
    hasWorktreeBaseCommitRef(repoPath, qualifiedRef, options)
  const resolvedBaseRef = await resolveWorktreeAddBaseRef(baseRef, refExists)
  if (resolvedBaseRef !== baseRef) {
    return true
  }
  if (baseRef.startsWith('refs/')) {
    return refExists(baseRef)
  }
  return hasCommitObjectViaGitExec(
    (gitArgs) => gitExecFileAsync(gitArgs, { cwd: repoPath, ...options }),
    baseRef
  )
}

type WorktreeStartupDraftPaste = {
  agent: TuiAgent
  content: string
}

type WorktreeStartupFollowup = {
  expectedProcess: string
  prompt: string
}

export class RuntimeWorktreeCreationCommands {
  constructor(private readonly host: RuntimeWorktreeCreationCommandHost) {}

  async activateManagedWorktree(
    worktreeSelector: string,
    opts: { notifyClients?: boolean; clientKind?: 'mobile' | 'runtime' } = {}
  ): Promise<{
    repoId: string
    worktreeId: string
    activated: boolean
    /** Mobile-scoped slept-agent wake outcome. `unsupported-headless` means no
     *  renderer holds the sleeping records (headless `orca serve`), so nothing
     *  woke — clients must not present the worktree's agents as resumed. */
    sleepingAgentWake: 'requested' | 'unsupported-headless' | 'not-applicable'
  }> {
    this.host.assertGraphReady()
    const worktree = await this.host.resolveWorktreeSelector(worktreeSelector)
    const repo = this.host.getStore()?.getRepo(worktree.repoId)
    if (!repo) {
      throw new Error('repo_not_found')
    }

    if (
      opts.notifyClients === false &&
      this.host.getStore()?.getWorktreeMeta(worktree.id)?.isUnread
    ) {
      // Why: mobile/web session activation intentionally bypasses renderer
      // selection, so the runtime must acknowledge the unread state itself.
      this.host.getStore()?.setWorktreeMeta(worktree.id, { isUnread: false })
      this.host.notifyWorktreesChanged(repo.id)
    }

    let sleepingAgentWake: 'requested' | 'unsupported-headless' | 'not-applicable' =
      'not-applicable'
    if (opts.notifyClients !== false) {
      // Why: inactive worktree terminal panes are renderer-owned and may not have
      // live PTYs until the desktop activates the worktree and mounts them.
      this.host.notifyActivateWorktree(repo.id, worktree.id)
    } else {
      // Why: mobile/web selection needs fresh session surfaces without forcing
      // every attached desktop renderer to navigate to the phone's workspace.
      this.host.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(worktree.id, {
        allowAttachedWindow: true
      })
      await this.host.refreshMobileSessionPtyRecords()
      this.host.notifyMobileSessionTabsChanged(worktree.id)
      // Why: a phone open must also wake the worktree's slept agents (experimental
      // agent sleep). Only the host renderer holds the sleeping records + wake
      // authority, so fire-and-forget ask it — mobile-scoped so web/desktop are
      // unaffected. Headless serve has no renderer to wake anything, so report
      // that explicitly instead of letting mobile assume the agents resumed.
      if (opts.clientKind === 'mobile') {
        if (this.host.getAvailableAuthoritativeWindow()) {
          this.host.getNotifier()?.resumeSleepingAgents?.(worktree.id)
          sleepingAgentWake = 'requested'
        } else if (
          // Why: sleeping records are partitioned by execution host; reading
          // only the local partition would miss slept agents on SSH-host
          // worktrees and skip the headless warning for them.
          Object.values(
            this.host.getStore()?.getWorkspaceSession?.(getRepoExecutionHostId(repo))
              .sleepingAgentSessionsByPaneKey ?? {}
          ).some((record) => record.worktreeId === worktree.id)
        ) {
          // Why: headless is only degraded when this worktree actually has a
          // persisted resume record. Ordinary mobile activation must not show
          // an unsupported warning merely because no desktop window is open.
          sleepingAgentWake = 'unsupported-headless'
        }
      }
    }
    return { repoId: repo.id, worktreeId: worktree.id, activated: true, sleepingAgentWake }
  }

  private async buildStartupForDraft(
    repo: Repo,
    draft: string,
    requestedAgent?: TuiAgent
  ): Promise<{
    agent: TuiAgent
    startup: WorktreeStartupLaunch
    draftPaste?: WorktreeStartupDraftPaste
  } | null> {
    const store = this.host.getStore()
    if (!store) {
      return null
    }
    const content = draft.trim()
    if (!content) {
      return null
    }
    const settings = store.getSettings()
    const preferredAgent = requestedAgent ?? settings.defaultTuiAgent
    if (preferredAgent === 'blank') {
      // Why: `blank` is an explicit user preference to create a shell-only
      // workspace, so linked task drafts must not auto-pick a detected agent.
      return null
    }
    let agent =
      isTuiAgent(preferredAgent) && isTuiAgentEnabled(preferredAgent, settings.disabledTuiAgents)
        ? preferredAgent
        : null
    if (!agent) {
      let detected: string[] = []
      try {
        // Why: startup-draft fallback can run from sparse runtime launch envs too.
        detected = repo.connectionId
          ? await detectRemoteAgents({ connectionId: repo.connectionId })
          : await detectInstalledAgentsWithShellPathHydration()
      } catch {
        detected = []
      }
      const typedDetected = detected.filter(isTuiAgent)
      agent = pickTuiAgent(null, typedDetected, settings.disabledTuiAgents)
    }
    if (!agent) {
      return null
    }

    // Why: a mobile client can run on Windows while the workspace shell is
    // Linux over SSH. Startup command quoting must target the shell that runs it.
    const agentLaunchPlatform = this.host.getAgentLaunchPlatformForRepo(repo)
    const isRemote = repoIsRemote(repo)
    const queuedShell = resolveLocalWindowsAgentStartupShell({
      platform: agentLaunchPlatform,
      isRemote,
      terminalWindowsShell: settings.terminalWindowsShell
    })
    const draftLaunchPlan = buildAgentDraftLaunchPlan({
      agent,
      draft: content,
      cmdOverrides: settings.agentCmdOverrides ?? {},
      agentArgs: resolveTuiAgentLaunchArgs(agent, settings.agentDefaultArgs),
      agentEnv: resolveTuiAgentLaunchEnv(agent, settings.agentDefaultEnv),
      platform: agentLaunchPlatform,
      shell: queuedShell,
      isRemote
    })
    if (draftLaunchPlan) {
      return {
        agent,
        startup: {
          command: draftLaunchPlan.launchCommand,
          launchConfig: draftLaunchPlan.launchConfig,
          ...(draftLaunchPlan.startupCommandDelivery
            ? { startupCommandDelivery: draftLaunchPlan.startupCommandDelivery }
            : {}),
          ...(draftLaunchPlan.env ? { env: draftLaunchPlan.env } : {})
        }
      }
    }

    const startupPlan = buildAgentStartupPlan({
      agent,
      prompt: '',
      cmdOverrides: settings.agentCmdOverrides ?? {},
      agentArgs: resolveTuiAgentLaunchArgs(agent, settings.agentDefaultArgs),
      agentEnv: resolveTuiAgentLaunchEnv(agent, settings.agentDefaultEnv),
      platform: agentLaunchPlatform,
      shell: queuedShell,
      isRemote,
      allowEmptyPromptLaunch: true
    })
    if (!startupPlan) {
      return null
    }
    return {
      agent,
      startup: {
        command: startupPlan.launchCommand,
        launchConfig: startupPlan.launchConfig,
        ...(startupPlan.startupCommandDelivery
          ? { startupCommandDelivery: startupPlan.startupCommandDelivery }
          : {}),
        ...(startupPlan.env ? { env: startupPlan.env } : {})
      },
      draftPaste: { agent, content }
    }
  }

  // Why: also called from OrcaRuntimeService's resolveAgentTerminalCreateOptions
  // (createTerminal's agent-launch path) — public, not private.
  buildStartupForAgent(
    repo: Repo,
    agent: TuiAgent,
    prompt: string | undefined
  ): { agent: TuiAgent; startup: WorktreeStartupLaunch; followup?: WorktreeStartupFollowup } {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('runtime_unavailable')
    }
    const settings = store.getSettings()
    if (!isTuiAgentEnabled(agent, settings.disabledTuiAgents)) {
      throw new Error('Selected agent is disabled. Choose an enabled agent before creating.')
    }
    // Why: CLI clients may target SSH runtimes from macOS/Windows, so quote for
    // the workspace shell rather than the client shell.
    const agentLaunchPlatform = this.host.getAgentLaunchPlatformForRepo(repo)
    const isRemote = repoIsRemote(repo)
    const queuedShell = resolveLocalWindowsAgentStartupShell({
      platform: agentLaunchPlatform,
      isRemote,
      terminalWindowsShell: settings.terminalWindowsShell
    })
    const startupPlan = buildAgentStartupPlan({
      agent,
      prompt: prompt ?? '',
      cmdOverrides: settings.agentCmdOverrides ?? {},
      agentArgs: resolveTuiAgentLaunchArgs(agent, settings.agentDefaultArgs),
      agentEnv: resolveTuiAgentLaunchEnv(agent, settings.agentDefaultEnv),
      platform: agentLaunchPlatform,
      shell: queuedShell,
      isRemote,
      allowEmptyPromptLaunch: true
    })
    if (!startupPlan) {
      throw new Error(`Could not build launch command for ${agent}.`)
    }
    return {
      agent,
      startup: {
        command: startupPlan.launchCommand,
        launchConfig: startupPlan.launchConfig,
        ...(startupPlan.startupCommandDelivery
          ? { startupCommandDelivery: startupPlan.startupCommandDelivery }
          : {}),
        ...(startupPlan.env ? { env: startupPlan.env } : {})
      },
      ...(startupPlan.followupPrompt
        ? {
            followup: {
              expectedProcess: startupPlan.expectedProcess,
              prompt: startupPlan.followupPrompt
            }
          }
        : {})
    }
  }

  // Why: also called from OrcaRuntimeService's resolveAgentTerminalCreateOptions
  // — public, not private.
  markLocalWorkspaceTrustedForAgent(agent: TuiAgent, workspacePath: string): void {
    const preset = TUI_AGENT_CONFIG[agent].preflightTrust
    if (!preset) {
      return
    }
    try {
      if (preset === 'cursor') {
        markCursorWorkspaceTrusted(workspacePath)
      } else if (preset === 'copilot') {
        markCopilotFolderTrusted(workspacePath)
      } else if (preset === 'codex') {
        markCodexProjectTrusted(workspacePath)
      }
    } catch {
      // Best-effort: the user can still accept the agent trust prompt manually.
    }
  }

  // Why: also called from OrcaRuntimeService's resolveAgentTerminalCreateOptions
  // — public, not private.
  async markRemoteWorkspaceTrustedForAgent(
    agent: TuiAgent,
    connectionId: string,
    workspacePath: string
  ): Promise<void> {
    const preset = TUI_AGENT_CONFIG[agent].preflightTrust
    if (!preset) {
      return
    }
    try {
      await markRemoteAgentWorkspaceTrusted({ preset, connectionId, workspacePath })
    } catch {
      // Best-effort: the user can still accept the remote agent trust prompt manually.
    }
  }

  private recordCreatedWorktreeLineage(
    worktree: Pick<Worktree, 'id' | 'instanceId'>,
    lineageResolution: WorktreeLineageResolution
  ): {
    lineage: WorktreeLineage | null
    workspaceLineage: WorkspaceLineage | null
    warnings: WorktreeLineageWarning[]
  } {
    const store = this.host.getStore()
    const warnings = lineageResolution.kind === 'none' ? [...lineageResolution.warnings] : []
    let lineage: WorktreeLineage | null = null
    let workspaceLineage: WorkspaceLineage | null = null
    if (lineageResolution.kind !== 'lineage') {
      return { lineage, workspaceLineage, warnings }
    }

    const childInstanceId = worktree.instanceId
    const parentInstanceId = lineageResolution.parent.instanceId
    const createdAt = Date.now()
    if (
      lineageResolution.parent.type === 'worktree' &&
      childInstanceId &&
      parentInstanceId &&
      store?.setWorktreeLineage
    ) {
      lineage = store.setWorktreeLineage(worktree.id, {
        worktreeId: worktree.id,
        worktreeInstanceId: childInstanceId,
        parentWorktreeId: lineageResolution.parent.worktree.id,
        parentWorktreeInstanceId: parentInstanceId,
        origin: lineageResolution.origin,
        capture: lineageResolution.capture,
        ...(lineageResolution.orchestrationRunId
          ? { orchestrationRunId: lineageResolution.orchestrationRunId }
          : {}),
        ...(lineageResolution.taskId ? { taskId: lineageResolution.taskId } : {}),
        ...(lineageResolution.coordinatorHandle
          ? { coordinatorHandle: lineageResolution.coordinatorHandle }
          : {}),
        ...(lineageResolution.createdByTerminalHandle
          ? { createdByTerminalHandle: lineageResolution.createdByTerminalHandle }
          : {}),
        createdAt
      })
    } else if (lineageResolution.parent.type === 'worktree') {
      warnings.push({
        code: 'LINEAGE_PARENT_CONTEXT_MISSING',
        message:
          'Worktree created, but Orca could not record lineage because instance identity was unavailable.',
        details: {
          childHasInstanceId: Boolean(childInstanceId),
          parentHasInstanceId: Boolean(parentInstanceId),
          storeSupportsLineage: Boolean(store?.setWorktreeLineage)
        }
      })
    }
    if (childInstanceId && store?.setWorkspaceLineage) {
      workspaceLineage = store.setWorkspaceLineage({
        childWorkspaceKey: worktreeWorkspaceKey(worktree.id),
        childInstanceId,
        parentWorkspaceKey: lineageResolution.parent.workspaceKey,
        parentInstanceId,
        origin: lineageResolution.origin,
        capture: lineageResolution.capture,
        ...(lineageResolution.taskId ? { taskId: lineageResolution.taskId } : {}),
        ...(lineageResolution.orchestrationRunId
          ? { orchestrationRunId: lineageResolution.orchestrationRunId }
          : {}),
        ...(lineageResolution.coordinatorHandle
          ? { coordinatorHandle: lineageResolution.coordinatorHandle }
          : {}),
        ...(lineageResolution.createdByTerminalHandle
          ? { createdByTerminalHandle: lineageResolution.createdByTerminalHandle }
          : {}),
        createdAt
      })
    }
    return { lineage, workspaceLineage, warnings }
  }

  private pasteStartupDraftWhenReady(handle: string, draft: WorktreeStartupDraftPaste): void {
    void this.waitForStartupDraftReady(handle, draft.agent)
      .then((ptyId) => {
        if (!ptyId) {
          console.warn('[worktree-create] agent did not become ready for draft paste')
          return
        }
        this.host
          .getPtyController()
          ?.write(ptyId, `${BRACKETED_PASTE_BEGIN}${draft.content}${BRACKETED_PASTE_END}`)
      })
      .catch((error) => {
        console.warn('[worktree-create] failed to paste startup draft:', error)
      })
  }

  private sendStartupFollowupWhenReady(handle: string, followup: WorktreeStartupFollowup): void {
    void this.waitForStartupFollowupReady(handle, followup.expectedProcess)
      .then((ptyId) => {
        if (!ptyId) {
          console.warn('[worktree-create] agent did not become ready for follow-up prompt')
          return
        }
        this.host.getPtyController()?.write(ptyId, `${followup.prompt}\r`)
      })
      .catch((error) => {
        console.warn('[worktree-create] failed to send startup follow-up prompt:', error)
      })
  }

  private async createDefaultTabTerminals(
    worktreeSelector: string,
    worktreeId: string,
    defaultTabs: CreateWorktreeResult['defaultTabs'] | undefined
  ): Promise<string[]> {
    if (!defaultTabs || defaultTabs.tabs.length === 0 || !this.host.getPtyController()?.spawn) {
      return []
    }
    const handles: string[] = []
    for (const template of defaultTabs.tabs) {
      try {
        const command = template.command?.trim()
        const terminal = await this.host.createTerminal(worktreeSelector, {
          ...(template.title ? { title: template.title } : {}),
          ...(command && defaultTabs.runCommands ? { command } : {})
        })
        handles.push(terminal.handle)
        if (template.color && terminal.tabId) {
          await this.host.setMobileSessionTabProps(`id:${worktreeId}`, {
            tabId: terminal.tabId,
            color: template.color
          })
        }
      } catch (error) {
        console.warn(`[worktree-create] Failed to create default tab for ${worktreeId}:`, error)
      }
    }
    return handles
  }

  private async provisionManagedWorktreeTerminals(args: {
    worktreeSelector: string
    worktreeId: string
    worktreePath: string
    setup?: CreateWorktreeResult['setup']
    defaultTabs?: CreateWorktreeResult['defaultTabs']
    primaryTerminalHandle?: string | null
    hasStartupTerminal: boolean
    setupCommandPlatform: 'windows' | 'posix'
    // Why: when the agent startup is sequenced to wait for setup
    // (waitForAgentStartup), the startup PTY runs a wrapper that already embeds
    // the setup command. Pass that wrapped command through so the Setup tab runs
    // the same script the agent is waiting on instead of a bare runner.
    wrappedSetupCommand?: string
  }): Promise<{ setupSpawned: boolean }> {
    if (!this.host.getPtyController()?.spawn) {
      return { setupSpawned: false }
    }
    let setupSpawned = false
    try {
      const defaultTabHandles = await this.createDefaultTabTerminals(
        args.worktreeSelector,
        args.worktreeId,
        args.defaultTabs
      )
      let primaryTerminalHandle = args.primaryTerminalHandle ?? defaultTabHandles[0] ?? null
      const setupLaunchMode =
        (
          this.host.requireStore().getSettings() as Partial<
            Pick<GlobalSettings, 'setupScriptLaunchMode'>
          >
        ).setupScriptLaunchMode ?? 'new-tab'
      if (!args.hasStartupTerminal && !primaryTerminalHandle) {
        const terminal = await this.host.createTerminal(args.worktreeSelector)
        primaryTerminalHandle = terminal.handle
      }
      if (args.setup) {
        const setupCommand =
          args.wrappedSetupCommand ??
          buildSetupRunnerCommand(args.setup.runnerScriptPath, args.setupCommandPlatform)
        const shouldSplitSetup =
          primaryTerminalHandle &&
          (setupLaunchMode === 'split-vertical' || setupLaunchMode === 'split-horizontal')
        await (shouldSplitSetup
          ? this.host.splitTerminal(primaryTerminalHandle!, {
              direction: setupLaunchMode === 'split-horizontal' ? 'horizontal' : 'vertical',
              command: setupCommand,
              env: args.setup.envVars,
              activate: false
            })
          : this.host.createTerminal(args.worktreeSelector, {
              title: 'Setup',
              command: setupCommand,
              env: args.setup.envVars
            }))
        setupSpawned = true
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      console.warn(
        `[worktree-create] Failed to create setup/default terminals for ${args.worktreePath}: ${message}`
      )
    }
    return { setupSpawned }
  }

  private async waitForStartupFollowupReady(
    handle: string,
    expectedProcess: string
  ): Promise<string | null> {
    const livePty = this.host.getLivePtyForHandle(handle)
    const ptyId = livePty?.pty.ptyId
    const ptyController = this.host.getPtyController()
    if (!ptyId || !ptyController) {
      return null
    }
    for (let attempt = 0; attempt < 30; attempt += 1) {
      if (attempt > 0) {
        await new Promise((resolve) => setTimeout(resolve, 150))
      }
      try {
        const foregroundProcess = await ptyController.getForegroundProcess(ptyId)
        if (isExpectedAgentProcess(foregroundProcess, expectedProcess)) {
          return ptyId
        }
        if (attempt >= 4 && !isShellProcess(foregroundProcess ?? '')) {
          const hasChildProcesses =
            (await ptyController.hasChildProcesses?.(ptyId).catch(() => false)) ?? false
          if (hasChildProcesses) {
            return ptyId
          }
        }
      } catch {
        // Ignore transient PTY inspection failures and keep polling.
      }
    }
    return null
  }

  private waitForStartupDraftReady(handle: string, agent: TuiAgent): Promise<string | null> {
    const livePty = this.host.getLivePtyForHandle(handle)
    const ptyId = livePty?.pty.ptyId
    if (!ptyId) {
      return Promise.resolve(null)
    }
    const readySignal =
      TUI_AGENT_CONFIG[agent].draftPasteReadySignal ?? 'render-quiet-after-bracketed-paste'
    return new Promise<string | null>((resolve) => {
      let settled = false
      const scanner = createDraftPasteReadyScanner(readySignal)
      let quietTimer: NodeJS.Timeout | null = null
      let hardTimer: NodeJS.Timeout | null = null
      let unsubscribe: (() => void) | null = null

      const finish = (value: string | null): void => {
        if (settled) {
          return
        }
        settled = true
        if (quietTimer) {
          clearTimeout(quietTimer)
        }
        if (hardTimer) {
          clearTimeout(hardTimer)
        }
        unsubscribe?.()
        resolve(value)
      }

      const armQuietTimer = (): void => {
        if (quietTimer) {
          clearTimeout(quietTimer)
        }
        quietTimer = setTimeout(() => finish(ptyId), BRACKETED_PASTE_QUIET_MS)
      }

      const observeData = (data: string): void => {
        const { ready, armQuietTimer: shouldArm } = scanner.observe(data)
        if (ready) {
          finish(ptyId)
          return
        }
        if (shouldArm) {
          armQuietTimer()
        }
      }

      unsubscribe = this.host.subscribeToTerminalData(ptyId, observeData)
      const replay = this.host.getRecentPtyOutput(ptyId)
      if (replay) {
        observeData(replay)
      }
      hardTimer = setTimeout(() => finish(null), DRAFT_PASTE_READY_TIMEOUT_MS)
    })
  }

  async prefetchManagedWorktreeCreateBase(args: {
    repoSelector: string
    baseBranch?: string
  }): Promise<void> {
    if (!this.host.getStore()) {
      throw new Error('runtime_unavailable')
    }

    const repo = await this.host.resolveRepoSelector(args.repoSelector)
    // Why: prefetchWorktreeCreateBase only needs the narrow
    // WorktreeCreateBasePrefetchRuntime shape (resolveRemoteTrackingBase,
    // hasRemoteTrackingRef, getOrStartRemoteTrackingBaseRefresh,
    // fetchRemoteWithCache) — this.host already satisfies it structurally.
    await prefetchWorktreeCreateBase({
      repo,
      baseBranch: args.baseBranch,
      runtime: this.host
    })
  }

  async createManagedWorktree(args: {
    repoSelector: string
    name: string
    baseBranch?: string
    compareBaseRef?: string
    branchNameOverride?: string
    linkedIssue?: number | null
    linkedPR?: number | null
    linkedLinearIssue?: string
    linkedLinearIssueWorkspaceId?: string | null
    linkedLinearIssueOrganizationUrlKey?: string | null
    linkedGitLabMR?: number | null
    linkedGitLabIssue?: number | null
    linkedBitbucketPR?: number | null
    linkedAzureDevOpsPR?: number | null
    linkedGiteaPR?: number | null
    comment?: string
    displayName?: string
    telemetrySource?: WorkspaceCreateTelemetrySource
    workspaceStatus?: string
    manualOrder?: number
    sparseCheckout?: { directories: string[]; presetId?: string }
    pushTarget?: GitPushTarget
    runHooks?: boolean
    activate?: boolean
    setupDecision?: 'run' | 'skip' | 'inherit'
    createdWithAgent?: TuiAgent
    startupAgent?: TuiAgent
    startupPrompt?: string
    pendingFirstAgentMessageRename?: boolean
    automationProvenance?: AutomationWorkspaceProvenance
    startup?: WorktreeStartupLaunch
    startupDraft?: string
    startupDraftPaste?: WorktreeStartupDraftPaste
    lineage?: WorktreeLineageInput
  }): Promise<CreateWorktreeResult> {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('runtime_unavailable')
    }

    const repo = await this.host.resolveRepoSelector(args.repoSelector)
    const createSettings = store.getSettings()
    const requestedAgent = args.startupAgent ?? args.createdWithAgent
    const requestedAgentEnabled =
      requestedAgent !== undefined
        ? isTuiAgentEnabled(requestedAgent, createSettings.disabledTuiAgents)
        : false
    if ((args.startup || args.startupAgent) && requestedAgent && !requestedAgentEnabled) {
      throw new Error('Selected agent is disabled. Choose an enabled agent before creating.')
    }
    if (
      args.startup &&
      args.startupDraftPaste &&
      !isTuiAgentEnabled(args.startupDraftPaste.agent, createSettings.disabledTuiAgents)
    ) {
      throw new Error('Selected agent is disabled. Choose an enabled agent before creating.')
    }
    const agentStartup =
      !args.startup && args.startupAgent
        ? this.buildStartupForAgent(repo, args.startupAgent, args.startupPrompt)
        : null
    const draftStartup =
      !args.startup && !agentStartup && args.startupDraft
        ? await this.buildStartupForDraft(repo, args.startupDraft, requestedAgent)
        : null
    const effectiveStartup = args.startup ?? agentStartup?.startup ?? draftStartup?.startup
    const effectiveStartupFollowup = agentStartup?.followup
    const effectiveCreatedWithAgent = args.startup
      ? args.createdWithAgent
      : (agentStartup?.agent ??
        draftStartup?.agent ??
        (requestedAgentEnabled ? requestedAgent : undefined))
    const effectiveDraftPaste = args.startupDraftPaste ?? draftStartup?.draftPaste
    if (isFolderRepo(repo)) {
      const now = Date.now()
      const settings = createSettings
      const instanceId = randomUUID()
      const worktreeId = getRuntimeFolderWorkspaceInstanceId(repo, instanceId)
      const meta = store.setWorktreeMeta(worktreeId, {
        instanceId,
        ...getProjectHostSetupWorktreeMeta(store.getProjectHostSetups?.() ?? [], repo),
        displayName: args.displayName?.trim() || args.name,
        lastActivityAt: now,
        createdAt: now,
        orcaCreatedAt: now,
        orcaCreationSource: 'runtime',
        orcaCreationWorkspaceLayout: {
          path: settings.workspaceDir,
          nestWorkspaces: settings.nestWorkspaces
        },
        ...(args.automationProvenance ? { automationProvenance: args.automationProvenance } : {}),
        ...(args.linkedIssue !== undefined ? { linkedIssue: args.linkedIssue } : {}),
        ...(args.linkedPR !== undefined ? { linkedPR: args.linkedPR } : {}),
        ...(args.linkedLinearIssue !== undefined
          ? { linkedLinearIssue: args.linkedLinearIssue }
          : {}),
        ...(args.linkedLinearIssueWorkspaceId !== undefined
          ? { linkedLinearIssueWorkspaceId: args.linkedLinearIssueWorkspaceId }
          : {}),
        ...(args.linkedLinearIssueOrganizationUrlKey !== undefined
          ? { linkedLinearIssueOrganizationUrlKey: args.linkedLinearIssueOrganizationUrlKey }
          : {}),
        ...(args.linkedGitLabIssue !== undefined
          ? { linkedGitLabIssue: args.linkedGitLabIssue }
          : {}),
        ...(args.linkedGitLabMR !== undefined ? { linkedGitLabMR: args.linkedGitLabMR } : {}),
        ...(args.linkedBitbucketPR !== undefined
          ? { linkedBitbucketPR: args.linkedBitbucketPR }
          : {}),
        ...(args.linkedAzureDevOpsPR !== undefined
          ? { linkedAzureDevOpsPR: args.linkedAzureDevOpsPR }
          : {}),
        ...(args.linkedGiteaPR !== undefined ? { linkedGiteaPR: args.linkedGiteaPR } : {}),
        ...(effectiveCreatedWithAgent ? { createdWithAgent: effectiveCreatedWithAgent } : {}),
        ...(args.comment !== undefined ? { comment: args.comment } : {}),
        ...(args.manualOrder !== undefined ? { manualOrder: args.manualOrder } : {}),
        ...(args.workspaceStatus !== undefined ? { workspaceStatus: args.workspaceStatus } : {})
      })
      const worktree = mergeRuntimeFolderWorkspace(repo, worktreeId, meta)
      this.host.invalidateResolvedWorktreeCache()
      this.host.notifyWorktreesChanged(repo.id)
      const shouldActivate = args.activate === true || args.runHooks === true
      let warning: string | undefined
      let didSpawnStartup = false
      let startupTerminal: CreateWorktreeResult['startupTerminal']
      if (effectiveStartup && this.host.getPtyController()?.spawn) {
        try {
          const startupTrustAgent = effectiveDraftPaste?.agent ?? effectiveCreatedWithAgent
          if (startupTrustAgent) {
            this.markLocalWorkspaceTrustedForAgent(startupTrustAgent, worktree.path)
          }
          const terminal = await this.host.createTerminal(`id:${worktree.id}`, {
            command: effectiveStartup.command,
            env: effectiveStartup.env,
            ...(effectiveStartup.launchConfig
              ? { launchConfig: effectiveStartup.launchConfig }
              : {}),
            ...(effectiveCreatedWithAgent ? { launchAgent: effectiveCreatedWithAgent } : {}),
            startupCommandDelivery: effectiveStartup.startupCommandDelivery,
            telemetry: effectiveStartup.telemetry
          })
          if (effectiveDraftPaste) {
            this.pasteStartupDraftWhenReady(terminal.handle, effectiveDraftPaste)
          }
          if (effectiveStartupFollowup) {
            this.sendStartupFollowupWhenReady(terminal.handle, effectiveStartupFollowup)
          }
          didSpawnStartup = true
          startupTerminal = {
            spawned: true,
            handle: terminal.handle,
            ...(terminal.tabId ? { tabId: terminal.tabId } : {}),
            ...(terminal.paneKey ? { paneKey: terminal.paneKey } : {}),
            ...(terminal.ptyId ? { ptyId: terminal.ptyId } : {}),
            surface: 'background'
          }
        } catch (err) {
          const message = err instanceof Error ? err.message : String(err)
          warning = `Failed to create the startup terminal for ${worktree.path}: ${message}`
          console.warn(`[worktree-create] ${warning}`)
        }
      }
      if (shouldActivate) {
        if (effectiveStartup && !didSpawnStartup) {
          this.host.notifyActivateWorktree(repo.id, worktree.id, undefined, effectiveStartup)
        } else {
          this.host.notifyActivateWorktree(repo.id, worktree.id)
        }
      } else if (this.host.getPtyController()?.spawn && !didSpawnStartup) {
        try {
          await this.host.createTerminal(`id:${worktree.id}`)
        } catch (err) {
          const message = err instanceof Error ? err.message : String(err)
          warning = warning
            ? `${warning} Also failed to create the initial terminal for ${worktree.path}: ${message}`
            : `Failed to create the initial terminal for ${worktree.path}: ${message}`
          console.warn(`[worktree-create] ${warning}`)
        }
      }
      return {
        worktree: {
          ...worktree,
          parentWorktreeId: null,
          childWorktreeIds: [],
          lineage: null,
          git: {
            path: worktree.path,
            head: worktree.head,
            branch: worktree.branch,
            isBare: worktree.isBare,
            isMainWorktree: worktree.isMainWorktree
          }
        },
        ...(startupTerminal ? { startupTerminal } : {}),
        ...(warning ? { warning } : {})
      }
    }
    const lineageInput =
      args.lineage || args.comment ? { ...args.lineage, comment: args.comment } : undefined
    const lineageResolution = await this.host.resolveLineageForWorktreeCreate(lineageInput)
    if (getRepoProviderConnectionKey(repo)) {
      const result = await this.createManagedRemoteWorktree(repo, {
        ...args,
        activate: args.activate,
        ...(effectiveStartup ? { startup: effectiveStartup } : {}),
        ...(effectiveStartupFollowup ? { startupFollowup: effectiveStartupFollowup } : {}),
        ...(effectiveCreatedWithAgent ? { createdWithAgent: effectiveCreatedWithAgent } : {}),
        ...(effectiveDraftPaste ? { startupDraftPaste: effectiveDraftPaste } : {})
      })
      const recordedLineage = this.recordCreatedWorktreeLineage(result.worktree, lineageResolution)
      return {
        ...result,
        worktree: {
          ...result.worktree,
          parentWorktreeId: recordedLineage.lineage?.parentWorktreeId ?? null,
          childWorktreeIds: result.worktree.childWorktreeIds ?? [],
          lineage: recordedLineage.lineage,
          workspaceLineage: recordedLineage.workspaceLineage
        },
        ...(lineageInput
          ? {
              lineage: recordedLineage.lineage,
              workspaceLineage: recordedLineage.workspaceLineage,
              warnings: recordedLineage.warnings
            }
          : {})
      }
    }
    const settings = createSettings
    const worktreePathSettings = getWorktreePathSettings(repo, settings)
    const localGitExecOptions = getLocalProjectGitExecOptions(this.host.requireStore(), repo)
    const localWorktreeGitOptions = getLocalProjectWorktreeGitOptions(
      this.host.requireStore(),
      repo
    )
    const hasLocalWorktreeGitOptions = hasLocalGitOptions(localWorktreeGitOptions)
    const localWorktreeGitOptionArgs: [] | [{ wslDistro?: string }] = hasLocalWorktreeGitOptions
      ? [localWorktreeGitOptions]
      : []
    const addProjectGitOptions = (options?: AddWorktreeOptions): AddWorktreeOptions | undefined => {
      if (!hasLocalWorktreeGitOptions) {
        return options
      }
      return { ...options, ...localWorktreeGitOptions }
    }
    const hostedReviewExecutionContext = this.host.getHostedReviewExecutionOptions(repo)
    let effectiveRequestedName = args.name
    const requestedDisplayName = args.displayName?.trim() || undefined
    const sanitizedName = sanitizeWorktreeName(args.name)
    let effectiveSanitizedName = sanitizedName
    // Why: explicit branches and non-username prefix modes never consume this
    // value; skipping the probes preserves the exact generated branch name.
    const username =
      !args.branchNameOverride && settings.branchPrefix === 'git-username'
        ? await resolveLocalGitUsername(repo.path)
        : ''

    const baseBranch = await resolveWorktreeCreateBase({
      requestedBaseBranch: args.baseBranch,
      repoWorktreeBaseRef: repo.worktreeBaseRef,
      resolveDefaultBaseRef: () =>
        hasLocalWorktreeGitOptions
          ? resolveDefaultBaseRefWithLocalGit(localGitExecOptions)
          : getBaseRefDefault(repo.path),
      isBaseUsable: async (baseBranchCandidate) => {
        const remoteTrackingBase = await this.host.resolveRemoteTrackingBase(
          repo.path,
          baseBranchCandidate,
          ...localWorktreeGitOptionArgs
        )
        if (remoteTrackingBase) {
          if (
            await this.host.hasRemoteTrackingRef(
              repo.path,
              remoteTrackingBase,
              ...localWorktreeGitOptionArgs
            )
          ) {
            return true
          }
          return hasLocalWorktreeBaseRef(
            repo.path,
            baseBranchCandidate,
            hasLocalWorktreeGitOptions ? localWorktreeGitOptions : {}
          )
        }
        return hasLocalWorktreeBaseRef(
          repo.path,
          baseBranchCandidate,
          hasLocalWorktreeGitOptions ? localWorktreeGitOptions : {}
        )
      }
    })
    if (!baseBranch) {
      // Why: a null default means no suitable ref exists; fail clearly instead
      // of handing Git a fabricated origin/main ref.
      throw new Error(
        'Could not resolve a default base ref for this repo. Pass an explicit --base and try again.'
      )
    }

    const workspaceRoot = computeWorkspaceRoot(repo.path, worktreePathSettings)
    // Why: CLI-managed WSL worktrees live under ~/orca/workspaces inside the
    // distro filesystem through computeWorkspaceRoot. If home lookup fails,
    // still validate against the effective workspace dir.
    let branchName = ''
    let checkoutExistingBranch = false
    let selectedExistingLocalBranchName: string | null = null
    let branchConflictKind: 'local' | 'remote' | null = null
    let worktreePath = ''
    let worktreePathResolved = false
    // Why: runtime/mobile create-from-review callers should get a new workspace
    // even when the PR branch or review branch name is already in use.
    for (let suffix = 1; suffix <= WORKTREE_CREATE_MAX_SUFFIX_ATTEMPTS; suffix += 1) {
      effectiveSanitizedName = getWorktreeCreateCandidate(sanitizedName, suffix)
      effectiveRequestedName = args.name.trim()
        ? getWorktreeCreateCandidate(args.name, suffix)
        : effectiveSanitizedName
      branchName = await resolveCreateBranchName(
        repo.path,
        selectedExistingLocalBranchName ??
          getBranchNameOverrideCandidate(args.branchNameOverride, suffix),
        effectiveSanitizedName,
        settings,
        username,
        localWorktreeGitOptions
      )
      checkoutExistingBranch = await canCheckoutExistingLocalBranch(
        repo.path,
        branchName,
        baseBranch,
        ...localWorktreeGitOptionArgs
      )
      if (checkoutExistingBranch && !selectedExistingLocalBranchName) {
        // Why: once a user-selected branch is safe to reuse, path retries should
        // keep that branch exact instead of creating a sibling branch.
        selectedExistingLocalBranchName = branchName
      }
      branchConflictKind = checkoutExistingBranch
        ? null
        : await getBranchConflictKind(
            repo.path,
            branchName,
            baseBranch,
            ...localWorktreeGitOptionArgs
          )
      const allowedPushTargetRemoteConflict =
        branchConflictKind &&
        isAllowedPushTargetRemoteConflict(branchConflictKind, branchName, args)
      let selectedReviewConflictMatched = false
      if (branchConflictKind) {
        if (allowedPushTargetRemoteConflict) {
          let existingPR: Awaited<ReturnType<typeof getPRForBranch>> | null = null
          const selectedReview = getSelectedReviewBranch(args)
          if (selectedReview?.provider === 'github') {
            try {
              existingPR = await getLocalGitHubPrForBranch(
                repo.path,
                branchName,
                localWorktreeGitOptions
              )
            } catch {
              // Retry with a suffixed branch when selected review verification is unavailable.
            }
            if (isMatchingSelectedGitHubPr(existingPR, args, branchName)) {
              branchConflictKind = null
              selectedReviewConflictMatched = true
            }
          } else if (selectedReview) {
            const hostedReview = await getSelectedHostedReviewForBranch(
              repo,
              branchName,
              args,
              hostedReviewExecutionContext
            ).catch(() => null)
            if (hostedReview?.matchesSelected) {
              branchConflictKind = null
              selectedReviewConflictMatched = true
            }
          }
        }
        if (branchConflictKind) {
          continue
        }
      }

      if (!checkoutExistingBranch && !selectedReviewConflictMatched) {
        let existingPR: Awaited<ReturnType<typeof getPRForBranch>> | null = null
        try {
          existingPR = await getLocalGitHubPrForBranch(
            repo.path,
            branchName,
            localWorktreeGitOptions
          )
        } catch {
          // Why: GitHub reachability should not block creating a suffixed
          // workspace; git conflicts still decide whether this candidate works.
        }
        if (existingPR && !isMatchingSelectedGitHubPr(existingPR, args, branchName)) {
          continue
        }
      }
      worktreePath = ensurePathWithinWorkspace(
        computeWorktreePath(effectiveSanitizedName, repo.path, worktreePathSettings),
        workspaceRoot
      )
      if (!(await pathExists(worktreePath))) {
        worktreePathResolved = true
        break
      }
    }
    if (!worktreePathResolved) {
      if (branchConflictKind) {
        throw new Error(
          `Branch "${branchName}" already exists ${branchConflictKind === 'local' ? 'locally' : 'on a remote'}.`
        )
      }
      throw new Error(
        `Could not find an available worktree path for "${sanitizedName}". Pick a different worktree name.`
      )
    }
    let remoteTrackingBase = await this.host.resolveRemoteTrackingBase(
      repo.path,
      baseBranch,
      ...localWorktreeGitOptionArgs
    )
    if (remoteTrackingBase) {
      const hadRemoteTrackingBaseRef = await this.host.hasRemoteTrackingRef(
        repo.path,
        remoteTrackingBase,
        ...localWorktreeGitOptionArgs
      )
      const hasLocalBaseRef =
        hadRemoteTrackingBaseRef ||
        (await hasLocalWorktreeBaseRef(
          repo.path,
          baseBranch,
          hasLocalWorktreeGitOptions ? localWorktreeGitOptions : {}
        ))
      if (!hadRemoteTrackingBaseRef && hasLocalBaseRef) {
        remoteTrackingBase = null
      } else {
        const refreshResult = await this.host.getOrStartRemoteTrackingBaseRefresh(
          repo.path,
          remoteTrackingBase,
          ...localWorktreeGitOptionArgs
        )
        if (!refreshResult.ok && !hadRemoteTrackingBaseRef) {
          // Why: only block creation when the refresh failed AND there is no
          // usable local base ref to fall back on. If a local remote-tracking ref
          // already exists, `git worktree add` can create from it — a possibly
          // stale but valid base — so a transient offline/auth failure must not
          // make the workspace uncreatable. The compare-to-base view reflects any
          // drift once the remote is reachable again.
          throw new Error(
            `Could not refresh base ref "${baseBranch}" from "${remoteTrackingBase.remote}". Check your network and try again.`
          )
        }
        if (
          !hadRemoteTrackingBaseRef &&
          !(await this.host.hasRemoteTrackingRef(
            repo.path,
            remoteTrackingBase,
            ...localWorktreeGitOptionArgs
          ))
        ) {
          throw new Error(`Base ref "${baseBranch}" was not found after fetching.`)
        }
      }
    } else if (
      !(await hasLocalWorktreeBaseRef(
        repo.path,
        baseBranch,
        hasLocalWorktreeGitOptions ? localWorktreeGitOptions : {}
      ))
    ) {
      // Why: local bases keep legacy best-effort fetch behavior. Verified PR
      // SHA bases already have the commit object needed by `git worktree add`.
      try {
        await this.host.fetchRemoteWithCache(repo.path, 'origin', ...localWorktreeGitOptionArgs)
      } catch {
        // Why: belt-and-suspenders. fetchRemoteWithCache already logs and does
        // not throw; the outer try/catch guarantees create-path tolerance even
        // if future refactors change that contract.
      }
    }

    const sparseDirectories = args.sparseCheckout
      ? normalizeSparseDirectories(args.sparseCheckout.directories)
      : []
    if (args.sparseCheckout && sparseDirectories.length === 0) {
      throw new Error('Sparse checkout requires at least one repo-relative directory.')
    }

    let preparedPushTarget: GitPushTarget | undefined
    if (args.pushTarget) {
      // Why: fork-PR worktrees created through a remote runtime need the same
      // upstream target setup as local desktop creates, or Push would publish
      // to the wrong remote after the client/server split.
      preparedPushTarget = await prepareWorktreePushTarget(
        repo.path,
        args.pushTarget,
        store,
        repo.id,
        localWorktreeGitOptions
      )
    }

    const suggestLocalBaseRefUpdate =
      !settings.refreshLocalBaseRefOnWorktreeCreate &&
      !settings.localBaseRefSuggestionDismissed &&
      Boolean(remoteTrackingBase)
    const remoteTrackingBaseOption = remoteTrackingBase ? { remoteTrackingBase } : undefined
    const existingBranchOption = {
      checkoutExistingBranch,
      ...remoteTrackingBaseOption,
      ...(suggestLocalBaseRefUpdate ? { suggestLocalBaseRefUpdate } : {})
    }
    const defaultAddWorktreeOption = addProjectGitOptions()
    const addResult: AddWorktreeResult =
      (await (sparseDirectories.length > 0
        ? checkoutExistingBranch
          ? addSparseWorktree(
              repo.path,
              worktreePath,
              branchName,
              sparseDirectories,
              baseBranch,
              settings.refreshLocalBaseRefOnWorktreeCreate,
              addProjectGitOptions(existingBranchOption)
            )
          : suggestLocalBaseRefUpdate
            ? addSparseWorktree(
                repo.path,
                worktreePath,
                branchName,
                sparseDirectories,
                baseBranch,
                settings.refreshLocalBaseRefOnWorktreeCreate,
                addProjectGitOptions({ ...remoteTrackingBaseOption, suggestLocalBaseRefUpdate })
              )
            : remoteTrackingBaseOption
              ? addSparseWorktree(
                  repo.path,
                  worktreePath,
                  branchName,
                  sparseDirectories,
                  baseBranch,
                  settings.refreshLocalBaseRefOnWorktreeCreate,
                  addProjectGitOptions(remoteTrackingBaseOption)
                )
              : defaultAddWorktreeOption
                ? addSparseWorktree(
                    repo.path,
                    worktreePath,
                    branchName,
                    sparseDirectories,
                    baseBranch,
                    settings.refreshLocalBaseRefOnWorktreeCreate,
                    defaultAddWorktreeOption
                  )
                : addSparseWorktree(
                    repo.path,
                    worktreePath,
                    branchName,
                    sparseDirectories,
                    baseBranch,
                    settings.refreshLocalBaseRefOnWorktreeCreate
                  )
        : checkoutExistingBranch
          ? addWorktree(
              repo.path,
              worktreePath,
              branchName,
              baseBranch,
              settings.refreshLocalBaseRefOnWorktreeCreate,
              false,
              addProjectGitOptions(existingBranchOption)
            )
          : suggestLocalBaseRefUpdate
            ? addWorktree(
                repo.path,
                worktreePath,
                branchName,
                baseBranch,
                settings.refreshLocalBaseRefOnWorktreeCreate,
                false,
                addProjectGitOptions({ ...remoteTrackingBaseOption, suggestLocalBaseRefUpdate })
              )
            : remoteTrackingBaseOption
              ? addWorktree(
                  repo.path,
                  worktreePath,
                  branchName,
                  baseBranch,
                  settings.refreshLocalBaseRefOnWorktreeCreate,
                  false,
                  addProjectGitOptions(remoteTrackingBaseOption)
                )
              : defaultAddWorktreeOption
                ? addWorktree(
                    repo.path,
                    worktreePath,
                    branchName,
                    baseBranch,
                    settings.refreshLocalBaseRefOnWorktreeCreate,
                    false,
                    defaultAddWorktreeOption
                  )
                : addWorktree(
                    repo.path,
                    worktreePath,
                    branchName,
                    baseBranch,
                    settings.refreshLocalBaseRefOnWorktreeCreate
                  ))) ?? {}

    let configuredPushTarget: GitPushTarget | undefined
    if (preparedPushTarget) {
      configuredPushTarget = await configureCreatedWorktreePushTarget(
        worktreePath,
        branchName,
        preparedPushTarget,
        localWorktreeGitOptions
      )
    }

    const gitWorktrees = hasLocalWorktreeGitOptions
      ? await listWorktrees(repo.path, localWorktreeGitOptions)
      : await listWorktrees(repo.path)
    const created = gitWorktrees.find((gw) => areWorktreePathsEqual(gw.path, worktreePath))
    if (!created) {
      throw new Error('Worktree created but not found in listing')
    }

    const worktreeId = `${repo.id}::${created.path}`
    const now = Date.now()
    // Why: PR/MR-created worktrees can start from a head ref/SHA while Source
    // Control must compare against the review target branch.
    const metadataBaseRef = args.compareBaseRef ?? remoteTrackingBase?.ref ?? baseBranch
    const displayNameMeta = requestedDisplayName
      ? { displayName: requestedDisplayName }
      : shouldSetDisplayName(effectiveRequestedName, branchName, effectiveSanitizedName)
        ? { displayName: effectiveRequestedName }
        : {}
    const meta = store.setWorktreeMeta(worktreeId, {
      // Why: worktree IDs are path-derived. If a path is deleted outside Orca
      // and later recreated, creation must mint a fresh instance identity so
      // stale lineage records tied to the old occupant fail validation.
      instanceId: randomUUID(),
      ...getProjectHostSetupWorktreeMeta(store.getProjectHostSetups?.() ?? [], repo),
      lastActivityAt: now,
      // See createRemoteWorktree: createdAt grants the new worktree a grace
      // window in Recent sort so ambient PTY bumps in OTHER worktrees can't
      // push it down before the user has had a chance to notice it. Smart-sort
      // uses max(lastActivityAt, createdAt + CREATE_GRACE_MS).
      createdAt: now,
      orcaCreatedAt: now,
      orcaCreationSource: 'runtime',
      orcaCreationWorkspaceLayout: getWorktreeCreationLayout(repo, settings),
      ...displayNameMeta,
      baseRef: metadataBaseRef,
      ...(checkoutExistingBranch ? { preserveBranchOnDelete: true } : {}),
      ...(configuredPushTarget ? { pushTarget: configuredPushTarget } : {}),
      ...(sparseDirectories.length > 0
        ? {
            sparseDirectories,
            sparseBaseRef: metadataBaseRef,
            sparsePresetId: args.sparseCheckout?.presetId
          }
        : {}),
      ...(args.linkedIssue !== undefined ? { linkedIssue: args.linkedIssue } : {}),
      ...(args.linkedPR !== undefined ? { linkedPR: args.linkedPR } : {}),
      ...(args.linkedLinearIssue !== undefined
        ? { linkedLinearIssue: args.linkedLinearIssue }
        : {}),
      ...(args.linkedLinearIssueWorkspaceId !== undefined
        ? { linkedLinearIssueWorkspaceId: args.linkedLinearIssueWorkspaceId }
        : {}),
      ...(args.linkedLinearIssueOrganizationUrlKey !== undefined
        ? { linkedLinearIssueOrganizationUrlKey: args.linkedLinearIssueOrganizationUrlKey }
        : {}),
      ...(args.linkedGitLabIssue !== undefined
        ? { linkedGitLabIssue: args.linkedGitLabIssue }
        : {}),
      ...(args.linkedGitLabMR !== undefined ? { linkedGitLabMR: args.linkedGitLabMR } : {}),
      ...(args.linkedBitbucketPR !== undefined
        ? { linkedBitbucketPR: args.linkedBitbucketPR }
        : {}),
      ...(args.linkedAzureDevOpsPR !== undefined
        ? { linkedAzureDevOpsPR: args.linkedAzureDevOpsPR }
        : {}),
      ...(args.linkedGiteaPR !== undefined ? { linkedGiteaPR: args.linkedGiteaPR } : {}),
      ...(effectiveCreatedWithAgent ? { createdWithAgent: effectiveCreatedWithAgent } : {}),
      ...(args.pendingFirstAgentMessageRename === true && effectiveCreatedWithAgent
        ? { pendingFirstAgentMessageRename: true }
        : {}),
      ...(args.automationProvenance ? { automationProvenance: args.automationProvenance } : {}),
      ...(args.comment !== undefined ? { comment: args.comment } : {}),
      ...(args.manualOrder !== undefined ? { manualOrder: args.manualOrder } : {}),
      ...(args.workspaceStatus !== undefined ? { workspaceStatus: args.workspaceStatus } : {})
    })
    const worktree = mergeWorktree(repo.id, created, meta)
    const {
      lineage,
      workspaceLineage,
      warnings: lineageWarnings
    } = this.recordCreatedWorktreeLineage(worktree, lineageResolution)

    if (repo.symlinkPaths && repo.symlinkPaths.length > 0) {
      await createWorktreeLinkedPaths(repo.path, created.path, repo.symlinkPaths)
    }

    let setup: CreateWorktreeResult['setup']
    let warning: string | undefined
    // Why: CLI-created worktrees do not have a renderer preview to mismatch
    // against. Trust is granted by the direct CLI invocation (`--run-hooks`),
    // so loading the setup hook from the created worktree is intentional here.
    const yamlHooks = loadHooks(worktreePath)
    const hooks = getEffectiveHooks(repo, worktreePath)
    // Why: setupDecision lets mobile/CLI callers control whether the setup
    // script runs. 'skip' suppresses it, 'run' forces it, 'inherit' (default)
    // defers to the repo's orca.yaml setupRunPolicy. runHooks === true maps
    // to 'run' for backwards compatibility with the desktop create flow.
    const effectiveDecision = args.runHooks ? 'run' : (args.setupDecision ?? 'inherit')
    let defaultTabs: CreateWorktreeResult['defaultTabs']
    try {
      defaultTabs = getDefaultTabsLaunch(yamlHooks, repo, effectiveDecision)
    } catch (error) {
      console.warn(`[hooks] default tab commands skipped for ${worktreePath}:`, error)
      defaultTabs = yamlHooks?.defaultTabs
        ? { tabs: yamlHooks.defaultTabs, runCommands: false }
        : undefined
    }
    const shouldRunSetup = hooks?.scripts.setup && shouldRunSetupForCreate(repo, effectiveDecision)
    if (shouldRunSetup && hooks?.scripts.setup) {
      const shouldUseSetupRunner =
        this.host.getGraphAuthoritativeWindowId() !== null || Boolean(effectiveStartup)
      if (shouldUseSetupRunner) {
        try {
          // Why: setup+startup must share the terminal runner path even without
          // a renderer window, so the startup shell can wait on setup completion.
          setup = createSetupRunnerScript(
            repo,
            worktreePath,
            hooks.scripts.setup,
            this.host.getLocalGitExecutionOptionArgs(repo)[0]
          )
        } catch (error) {
          // Why: the git worktree is already real at this point. If runner
          // generation fails, keep creation successful and surface the problem in
          // logs rather than pretending the worktree was never created.
          console.error(`[hooks] Failed to prepare setup runner for ${worktreePath}:`, error)
        }
      } else {
        void runHook(
          'setup',
          worktreePath,
          repo,
          worktreePath,
          this.host.getLocalGitExecutionOptionArgs(repo)[0]
        ).then((result) => {
          if (!result.success) {
            console.error(`[hooks] setup hook failed for ${worktreePath}:`, result.output)
          }
        })
      }
    } else if (hooks?.scripts.setup && effectiveDecision !== 'skip') {
      // Runtime RPC calls have no renderer trust prompt, so hooks require explicit CLI opt-in.
      warning = `orca.yaml setup hook skipped for ${worktreePath}; pass --setup run to run it.`
      console.warn(`[hooks] ${warning}`)
    }

    this.host.invalidateResolvedWorktreeCache()
    // Why: the filesystem-auth layer maintains a separate cache of registered
    // worktree roots used by git IPC handlers (branchCompare, diff, status, etc.)
    // to authorize paths. Without invalidating it here, CLI-created worktrees
    // are not recognized and all git operations fail with "Access denied:
    // unknown repository or worktree path".
    invalidateAuthorizedRootsCache()

    this.host.notifyWorktreesChanged(repo.id)
    const shouldActivate = args.activate === true || args.runHooks === true
    let didSpawnStartup = false
    // Why: tracks whether runtime itself launched the setup script (via
    // provisionManagedWorktreeTerminals). When true, renderer activation and the
    // RPC return value must omit setup so the client does not spawn it a second
    // time. Mirrors the wait-for-agent setup contract from #6298.
    let didSpawnSetup = false
    let startupTerminalHandle: string | null = null
    let startupTerminalTabId: string | null = null
    let startupTerminalPaneKey: string | null = null
    let startupTerminalPtyId: string | null = null

    let sequencedStartup = effectiveStartup
    let wrappedSetupCommandStr: string | undefined
    if (effectiveStartup && setup?.waitForAgentStartup === true) {
      const platform = getSetupRunnerCommandPlatformForPath(
        setup.runnerScriptPath,
        process.platform === 'win32' ? 'windows' : 'posix'
      )
      const sequenced = createSequencedSetupAgentCommands({
        runnerScriptPath: setup.runnerScriptPath,
        startupCommand: effectiveStartup.command,
        platform
      })
      sequencedStartup = {
        ...effectiveStartup,
        command: sequenced.startupCommand,
        ...(sequenced.startupEnv
          ? { env: { ...effectiveStartup.env, ...sequenced.startupEnv } }
          : {})
      }
      wrappedSetupCommandStr = sequenced.setupCommand
    }

    if (sequencedStartup && this.host.getPtyController()?.spawn) {
      try {
        // Why: automation startup must not depend on a renderer TerminalPane
        // mounting. Runtime-spawned PTYs run immediately and the UI adopts the
        // session later, matching `orca terminal create` background semantics.
        const startupTrustAgent = effectiveDraftPaste?.agent ?? effectiveCreatedWithAgent
        if (startupTrustAgent) {
          this.markLocalWorkspaceTrustedForAgent(startupTrustAgent, worktreePath)
        }
        const terminal = await this.host.createTerminal(`id:${worktree.id}`, {
          command: sequencedStartup.command,
          ...(setup && effectiveStartup
            ? { claudeAgentTeamsSourceCommand: effectiveStartup.command }
            : {}),
          env: sequencedStartup.env,
          ...(sequencedStartup.launchConfig ? { launchConfig: sequencedStartup.launchConfig } : {}),
          ...(effectiveCreatedWithAgent ? { launchAgent: effectiveCreatedWithAgent } : {}),
          startupCommandDelivery: sequencedStartup.startupCommandDelivery,
          telemetry: sequencedStartup.telemetry
        })
        if (effectiveDraftPaste) {
          this.pasteStartupDraftWhenReady(terminal.handle, effectiveDraftPaste)
        }
        if (effectiveStartupFollowup) {
          this.sendStartupFollowupWhenReady(terminal.handle, effectiveStartupFollowup)
        }
        didSpawnStartup = true
        startupTerminalHandle = terminal.handle
        startupTerminalTabId = terminal.tabId ?? null
        startupTerminalPaneKey = terminal.paneKey ?? null
        startupTerminalPtyId = terminal.ptyId ?? null
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err)
        warning = warning
          ? `${warning} Also failed to create the startup terminal for ${worktreePath}: ${message}`
          : `Failed to create the startup terminal for ${worktreePath}: ${message}`
        console.warn(`[worktree-create] ${warning}`)
      }
    }
    if (shouldActivate) {
      // Why: plain CLI creates should not steal the user's current workspace.
      // Explicit activation and hook-running still use renderer activation so
      // the user can watch prompts/output in a visible pane.
      const runtimeWillProvisionTerminals = didSpawnStartup && Boolean(setup || defaultTabs)
      if (runtimeWillProvisionTerminals) {
        // Why: once runtime spawned the startup PTY, renderer activation may see
        // an existing terminal and skip setup/default tabs. Await provisioning so
        // a failed setup spawn falls back to renderer activation (which still
        // carries the wrapped command for retry); #6298's wait-for-setup
        // guarantee is enforced by the shell marker, not by spawn timing.
        const provisioned = await this.provisionManagedWorktreeTerminals({
          worktreeSelector: `id:${worktree.id}`,
          worktreeId: worktree.id,
          worktreePath,
          ...(setup ? { setup } : {}),
          ...(defaultTabs ? { defaultTabs } : {}),
          primaryTerminalHandle: startupTerminalHandle,
          hasStartupTerminal: didSpawnStartup,
          setupCommandPlatform: setup
            ? isWindowsAbsolutePathLike(setup.runnerScriptPath)
              ? 'windows'
              : 'posix'
            : 'posix',
          // Why: carry the wait-for-agent wrapped setup command (#6298) so the
          // Setup tab runs the same script the sequenced agent waits on.
          ...(wrappedSetupCommandStr ? { wrappedSetupCommand: wrappedSetupCommandStr } : {})
        })
        didSpawnSetup = provisioned.setupSpawned
      }
      // Why: when runtime spawned setup, omit it from activation. When setup
      // spawn failed, fall through with the wrapped command so renderer
      // activation retries it.
      const activationSetup = didSpawnSetup
        ? undefined
        : setup
          ? {
              ...setup,
              ...(didSpawnStartup && wrappedSetupCommandStr
                ? { command: wrappedSetupCommandStr }
                : {})
            }
          : undefined
      const activationDefaultTabs = runtimeWillProvisionTerminals ? undefined : defaultTabs
      if (effectiveStartup && !didSpawnStartup) {
        this.host.notifyActivateWorktree(
          repo.id,
          worktree.id,
          activationSetup,
          effectiveStartup,
          activationDefaultTabs
        )
      } else {
        this.host.notifyActivateWorktree(
          repo.id,
          worktree.id,
          activationSetup,
          undefined,
          activationDefaultTabs
        )
      }
    } else if (this.host.getPtyController()?.spawn && (setup || defaultTabs || didSpawnStartup)) {
      // Why: inactive terminal materialization matches normal worktree creation,
      // but setup/default tab failures must not gate automation dispatch.
      void this.provisionManagedWorktreeTerminals({
        worktreeSelector: `id:${worktree.id}`,
        worktreeId: worktree.id,
        worktreePath,
        ...(setup ? { setup } : {}),
        ...(defaultTabs ? { defaultTabs } : {}),
        primaryTerminalHandle: startupTerminalHandle,
        hasStartupTerminal: didSpawnStartup,
        setupCommandPlatform: setup
          ? isWindowsAbsolutePathLike(setup.runnerScriptPath)
            ? 'windows'
            : 'posix'
          : 'posix',
        ...(wrappedSetupCommandStr ? { wrappedSetupCommand: wrappedSetupCommandStr } : {})
      })
      // Why: runtime owns setup spawning here, so the RPC result must omit setup
      // to keep the headless/mobile caller from launching it a second time.
      if (setup) {
        didSpawnSetup = true
      }
    } else if (this.host.getPtyController()?.spawn) {
      try {
        await this.host.createTerminal(`id:${worktree.id}`)
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err)
        warning = warning
          ? `${warning} Also failed to create the initial terminal for ${worktreePath}: ${message}`
          : `Failed to create the initial terminal for ${worktreePath}: ${message}`
        console.warn(`[worktree-create] ${warning}`)
      }
    }
    const returnedSetup = didSpawnSetup
      ? undefined
      : setup
        ? {
            ...setup,
            ...(didSpawnStartup && wrappedSetupCommandStr
              ? { command: wrappedSetupCommandStr }
              : {})
          }
        : undefined
    return {
      worktree: {
        ...worktree,
        parentWorktreeId: lineage?.parentWorktreeId ?? null,
        childWorktreeIds: [],
        lineage,
        workspaceLineage,
        git: created
      },
      ...(lineageInput ? { lineage, workspaceLineage, warnings: lineageWarnings } : {}),
      ...(returnedSetup ? { setup: returnedSetup } : {}),
      ...(defaultTabs ? { defaultTabs } : {}),
      ...(warning ? { warning } : {}),
      ...(addResult.localBaseRefRefresh
        ? { localBaseRefRefresh: addResult.localBaseRefRefresh }
        : {}),
      ...(addResult.localBaseRefUpdateSuggestion
        ? { localBaseRefUpdateSuggestion: addResult.localBaseRefUpdateSuggestion }
        : {}),
      ...(didSpawnStartup && startupTerminalHandle
        ? {
            startupTerminal: {
              spawned: true,
              handle: startupTerminalHandle,
              ...(startupTerminalTabId ? { tabId: startupTerminalTabId } : {}),
              ...(startupTerminalPaneKey ? { paneKey: startupTerminalPaneKey } : {}),
              ...(startupTerminalPtyId ? { ptyId: startupTerminalPtyId } : {}),
              surface: 'background' as const
            }
          }
        : {})
    }
  }

  private async createManagedRemoteWorktree(
    repo: Repo,
    args: {
      name: string
      baseBranch?: string
      compareBaseRef?: string
      branchNameOverride?: string
      linkedIssue?: number | null
      linkedPR?: number | null
      linkedLinearIssue?: string
      linkedLinearIssueWorkspaceId?: string | null
      linkedLinearIssueOrganizationUrlKey?: string | null
      linkedGitLabMR?: number | null
      linkedGitLabIssue?: number | null
      linkedBitbucketPR?: number | null
      linkedAzureDevOpsPR?: number | null
      linkedGiteaPR?: number | null
      comment?: string
      displayName?: string
      workspaceStatus?: string
      manualOrder?: number
      sparseCheckout?: { directories: string[]; presetId?: string }
      pushTarget?: GitPushTarget
      runHooks?: boolean
      activate?: boolean
      setupDecision?: 'run' | 'skip' | 'inherit'
      createdWithAgent?: TuiAgent
      pendingFirstAgentMessageRename?: boolean
      automationProvenance?: AutomationWorkspaceProvenance
      startup?: WorktreeStartupLaunch
      startupFollowup?: WorktreeStartupFollowup
      startupDraftPaste?: WorktreeStartupDraftPaste
    }
  ): Promise<CreateWorktreeResult> {
    const store = this.host.getStore()
    if (!store) {
      throw new Error('runtime_unavailable')
    }

    // Why: runtime/mobile callers do not own a renderer BrowserWindow, but the
    // SSH create helper only uses it for progress and change notifications.
    // Runtime emits those through RuntimeNotifier after the create succeeds.
    const headlessWindow = {
      isDestroyed: () => false,
      webContents: { send: () => undefined }
    } as unknown as BrowserWindow

    const result = await createRemoteWorktree(
      {
        repoId: repo.id,
        name: args.name,
        ...(args.displayName ? { displayName: args.displayName } : {}),
        ...(args.baseBranch ? { baseBranch: args.baseBranch } : {}),
        ...(args.compareBaseRef ? { compareBaseRef: args.compareBaseRef } : {}),
        ...(args.branchNameOverride ? { branchNameOverride: args.branchNameOverride } : {}),
        ...(args.runHooks ? { setupDecision: 'run' as const } : {}),
        ...(!args.runHooks && args.setupDecision ? { setupDecision: args.setupDecision } : {}),
        ...(args.sparseCheckout ? { sparseCheckout: args.sparseCheckout } : {}),
        ...(args.linkedIssue != null ? { linkedIssue: args.linkedIssue } : {}),
        ...(args.linkedPR != null ? { linkedPR: args.linkedPR } : {}),
        ...(args.linkedLinearIssue ? { linkedLinearIssue: args.linkedLinearIssue } : {}),
        ...(args.linkedLinearIssueWorkspaceId !== undefined
          ? { linkedLinearIssueWorkspaceId: args.linkedLinearIssueWorkspaceId }
          : {}),
        ...(args.linkedLinearIssueOrganizationUrlKey !== undefined
          ? { linkedLinearIssueOrganizationUrlKey: args.linkedLinearIssueOrganizationUrlKey }
          : {}),
        ...(args.linkedGitLabMR != null ? { linkedGitLabMR: args.linkedGitLabMR } : {}),
        ...(args.linkedGitLabIssue != null ? { linkedGitLabIssue: args.linkedGitLabIssue } : {}),
        ...(args.linkedBitbucketPR != null ? { linkedBitbucketPR: args.linkedBitbucketPR } : {}),
        ...(args.linkedAzureDevOpsPR != null
          ? { linkedAzureDevOpsPR: args.linkedAzureDevOpsPR }
          : {}),
        ...(args.linkedGiteaPR != null ? { linkedGiteaPR: args.linkedGiteaPR } : {}),
        ...(args.pushTarget ? { pushTarget: args.pushTarget } : {}),
        ...(args.workspaceStatus ? { workspaceStatus: args.workspaceStatus as never } : {}),
        ...(args.manualOrder !== undefined ? { manualOrder: args.manualOrder } : {}),
        ...(args.createdWithAgent ? { createdWithAgent: args.createdWithAgent } : {}),
        ...(args.pendingFirstAgentMessageRename === true
          ? { pendingFirstAgentMessageRename: true }
          : {}),
        ...(args.automationProvenance ? { automationProvenance: args.automationProvenance } : {})
      },
      repo,
      store as unknown as Store,
      headlessWindow
    )

    if (args.comment !== undefined) {
      store.setWorktreeMeta(result.worktree.id, { comment: args.comment })
      result.worktree.comment = args.comment
    }

    this.host.invalidateResolvedWorktreeCache()
    this.host.notifyWorktreesChanged(repo.id)

    let warning = result.warning
    let didSpawnStartup = false
    // Why: same no-double-spawn contract as the local path — once runtime
    // provisions setup, omit it from activation and the RPC result.
    let didSpawnSetup = false
    let startupTerminalHandle: string | null = null
    let startupTerminalTabId: string | null = null
    let startupTerminalPaneKey: string | null = null
    let startupTerminalPtyId: string | null = null

    let sequencedStartup = args.startup
    let wrappedSetupCommandStr: string | undefined
    if (args.startup && result.setup?.waitForAgentStartup === true) {
      const platform = getSetupRunnerCommandPlatformForPath(result.setup.runnerScriptPath, 'posix')
      const sequenced = createSequencedSetupAgentCommands({
        runnerScriptPath: result.setup.runnerScriptPath,
        startupCommand: args.startup.command,
        platform
      })
      sequencedStartup = {
        ...args.startup,
        command: sequenced.startupCommand,
        ...(sequenced.startupEnv ? { env: { ...args.startup.env, ...sequenced.startupEnv } } : {})
      }
      wrappedSetupCommandStr = sequenced.setupCommand
    }

    if (sequencedStartup && this.host.getPtyController()?.spawn) {
      try {
        const startupTrustAgent = args.startupDraftPaste?.agent ?? args.createdWithAgent
        if (startupTrustAgent) {
          await this.markRemoteWorkspaceTrustedForAgent(
            startupTrustAgent,
            repo.connectionId!,
            result.worktree.path
          )
        }
        const terminal = await this.host.createTerminal(`path:${result.worktree.path}`, {
          command: sequencedStartup.command,
          ...(result.setup && args.startup
            ? { claudeAgentTeamsSourceCommand: args.startup.command }
            : {}),
          env: sequencedStartup.env,
          ...(sequencedStartup.launchConfig ? { launchConfig: sequencedStartup.launchConfig } : {}),
          ...(args.createdWithAgent ? { launchAgent: args.createdWithAgent } : {}),
          startupCommandDelivery: sequencedStartup.startupCommandDelivery,
          telemetry: sequencedStartup.telemetry
        })
        if (args.startupDraftPaste) {
          this.pasteStartupDraftWhenReady(terminal.handle, args.startupDraftPaste)
        }
        if (args.startupFollowup) {
          this.sendStartupFollowupWhenReady(terminal.handle, args.startupFollowup)
        }
        didSpawnStartup = true
        startupTerminalHandle = terminal.handle
        startupTerminalTabId = terminal.tabId ?? null
        startupTerminalPaneKey = terminal.paneKey ?? null
        startupTerminalPtyId = terminal.ptyId ?? null
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err)
        warning = warning
          ? `${warning} Also failed to create the startup terminal for ${result.worktree.path}: ${message}`
          : `Failed to create the startup terminal for ${result.worktree.path}: ${message}`
      }
    }

    const shouldActivate = args.activate === true || args.runHooks === true
    if (shouldActivate) {
      const runtimeWillProvisionTerminals =
        didSpawnStartup && Boolean(result.setup || result.defaultTabs)
      if (runtimeWillProvisionTerminals) {
        // Why: remote/mobile task creates spawn the agent terminal in runtime,
        // so renderer activation may not materialize setup/default tabs. Await so
        // a failed setup spawn falls back to renderer activation for retry.
        const provisioned = await this.provisionManagedWorktreeTerminals({
          worktreeSelector: `path:${result.worktree.path}`,
          worktreeId: result.worktree.id,
          worktreePath: result.worktree.path,
          ...(result.setup ? { setup: result.setup } : {}),
          ...(result.defaultTabs ? { defaultTabs: result.defaultTabs } : {}),
          primaryTerminalHandle: startupTerminalHandle,
          hasStartupTerminal: didSpawnStartup,
          setupCommandPlatform: result.setup
            ? isWindowsAbsolutePathLike(result.setup.runnerScriptPath)
              ? 'windows'
              : 'posix'
            : 'posix',
          // Why: carry the wait-for-agent wrapped setup command (#6298) so the
          // remote Setup tab runs the same script the sequenced agent waits on.
          ...(wrappedSetupCommandStr ? { wrappedSetupCommand: wrappedSetupCommandStr } : {})
        })
        didSpawnSetup = provisioned.setupSpawned
      }
      // Why: omit setup from activation when runtime spawned it; on spawn
      // failure fall through with the wrapped command so renderer retries.
      const activationSetup = didSpawnSetup
        ? undefined
        : result.setup
          ? {
              ...result.setup,
              ...(didSpawnStartup && wrappedSetupCommandStr
                ? { command: wrappedSetupCommandStr }
                : {})
            }
          : undefined
      const activationDefaultTabs = runtimeWillProvisionTerminals ? undefined : result.defaultTabs
      if (args.startup && !didSpawnStartup) {
        this.host.notifyActivateWorktree(
          repo.id,
          result.worktree.id,
          activationSetup,
          args.startup,
          activationDefaultTabs
        )
      } else {
        this.host.notifyActivateWorktree(
          repo.id,
          result.worktree.id,
          activationSetup,
          undefined,
          activationDefaultTabs
        )
      }
    }

    if (
      !shouldActivate &&
      this.host.getPtyController()?.spawn &&
      (result.setup || result.defaultTabs || didSpawnStartup)
    ) {
      // Why: inactive terminal materialization matches normal worktree creation,
      // but setup/default tab failures must not gate automation dispatch.
      void this.provisionManagedWorktreeTerminals({
        worktreeSelector: `path:${result.worktree.path}`,
        worktreeId: result.worktree.id,
        worktreePath: result.worktree.path,
        ...(result.setup ? { setup: result.setup } : {}),
        ...(result.defaultTabs ? { defaultTabs: result.defaultTabs } : {}),
        primaryTerminalHandle: startupTerminalHandle,
        hasStartupTerminal: didSpawnStartup,
        setupCommandPlatform: result.setup
          ? isWindowsAbsolutePathLike(result.setup.runnerScriptPath)
            ? 'windows'
            : 'posix'
          : 'posix',
        ...(wrappedSetupCommandStr ? { wrappedSetupCommand: wrappedSetupCommandStr } : {})
      })
      // Why: runtime owns setup spawning here, so omit setup from the RPC result
      // to keep the headless/mobile caller from launching it a second time.
      if (result.setup) {
        didSpawnSetup = true
      }
    } else if (!shouldActivate && this.host.getPtyController()?.spawn) {
      try {
        await this.host.createTerminal(`path:${result.worktree.path}`)
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err)
        warning = warning
          ? `${warning} Also failed to create the initial terminal for ${result.worktree.path}: ${message}`
          : `Failed to create the initial terminal for ${result.worktree.path}: ${message}`
      }
    }

    const returnedSetup = didSpawnSetup
      ? undefined
      : result.setup
        ? {
            ...result.setup,
            ...(didSpawnStartup && wrappedSetupCommandStr
              ? { command: wrappedSetupCommandStr }
              : {})
          }
        : undefined
    const resultForRenderer = returnedSetup
      ? { ...result, setup: returnedSetup }
      : (() => {
          const { setup: _setup, ...resultWithoutSetup } = result
          return resultWithoutSetup
        })()

    const resultWithStartupTerminal =
      didSpawnStartup && startupTerminalHandle
        ? {
            ...resultForRenderer,
            startupTerminal: {
              spawned: true,
              handle: startupTerminalHandle,
              ...(startupTerminalTabId ? { tabId: startupTerminalTabId } : {}),
              ...(startupTerminalPaneKey ? { paneKey: startupTerminalPaneKey } : {}),
              ...(startupTerminalPtyId ? { ptyId: startupTerminalPtyId } : {}),
              surface: 'background' as const
            }
          }
        : resultForRenderer

    return warning ? { ...resultWithStartupTerminal, warning } : resultWithStartupTerminal
  }
}
