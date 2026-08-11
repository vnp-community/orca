/* eslint-disable max-lines -- Why: OrcaRuntimeService still owns the mutable live graph, PTY handles, waiters, mobile floor/layout state, and managed-worktree reconciliation. Stateless browser and file command adapters live beside it; the remaining split points need state-owner extraction before enforcing max-lines. */
/* eslint-disable unicorn/no-useless-spread -- Why: waiter sets and handle keys are cloned intentionally before mutation so resolution and rejection can safely remove entries while iterating. */
/* eslint-disable no-control-regex -- Why: terminal normalization must strip ANSI and OSC control sequences from PTY output before returning bounded text to agents. */
import type { WebPushManager } from '../notifications/web-push-manager'
import { RuntimeGraphStore } from './orca-runtime-graph-store'
import { RuntimeAutomationCommands } from './orca-runtime-automation'
import { RuntimeRemoteFetchCache } from './orca-runtime-remote-fetch-cache'
import { RuntimeResolvedWorktreeCommands } from './orca-runtime-resolved-worktree-cache'
import { RuntimeBranchCleanupCommands } from './orca-runtime-branch-cleanup'
import { RuntimeMobileFloorCommands } from './orca-runtime-mobile-floor'
import { RuntimeIssueTrackingCommands } from './orca-runtime-issue-tracking'
import { RuntimeRepoHooksCommands } from './orca-runtime-repo-hooks'
import { RuntimeLinearCommands } from './orca-runtime-linear'
import { RuntimeJiraCommands } from './orca-runtime-jira'
import { RuntimeProjectGroupsCommands } from './orca-runtime-project-groups'
import { RuntimeWorktreeBaseStatusCommands } from './orca-runtime-worktree-base-status'
import { RuntimeWorktreeCreationCommands } from './orca-runtime-worktree-creation'
import { RuntimeRepoLifecycleCommands } from './orca-runtime-repo-lifecycle'
import { RuntimeMobileSessionTabsCommands } from './orca-runtime-mobile-session-tabs'
import { RuntimeMobileSessionTerminalCommands } from './orca-runtime-mobile-session-terminal'
import { RuntimeMobileSessionNotifyCommands } from './orca-runtime-mobile-session-notify'
import { RuntimeMobileDictationCommands } from './orca-runtime-mobile-dictation'
import { RuntimeAccountServicesCommands } from './orca-runtime-account-services'
import { RuntimePtyWaitBlockedCheckCommands } from './orca-runtime-pty-wait-blocked-check'
import { RuntimeTerminalMessageWaiterCommands } from './orca-runtime-terminal-message-waiter'
import { RuntimeConnectionSubscriptionNotifyCommands } from './orca-runtime-connection-subscription-notify'
import { RuntimePtyForegroundAgentRefreshCommands } from './orca-runtime-pty-foreground-agent-refresh'
import { RuntimeRemoteTerminalViewSubscriberCommands } from './orca-runtime-remote-terminal-view-subscriber'
import { RuntimePtyTranscriptStore } from './orca-runtime-pty-transcript-store'
import { RuntimeHeadlessTerminalCommands } from './orca-runtime-headless-terminal'
import { RuntimeWorktreeLineageCommands } from './orca-runtime-worktree-lineage'
import { RuntimeBrowserScreencastCommands } from './orca-runtime-browser-screencast'
import { RuntimePtyTitleTrackerCommands } from './orca-runtime-pty-title-tracker'
import { RuntimeTerminalSideEffectsCommands } from './orca-runtime-terminal-side-effects'
import { RuntimeAgentRowSnapshotCommands } from './orca-runtime-agent-row-snapshot'
import { RuntimeTerminalListingCommands } from './orca-runtime-terminal-listing'
import { RuntimeWorktreePsCommands } from './orca-runtime-worktree-ps'
import {
  detectAgentStatusFromTitle,
  isClaudeManagementTitle,
  isCursorAgentTitle,
  isShellProcess
} from '../../shared/agent-detection'
import { extractOscTitleScanTail } from '../../shared/osc-title-scan-tail'
import type { AgentStatus } from '../../shared/agent-detection'
import type { TerminalOscLinkRange } from '../../shared/terminal-osc-link-ranges'
import type { TerminalTitleTracker } from '../../shared/terminal-output-side-effects'
import type {
  TerminalSideEffectBatch,
  TerminalSideEffectFact
} from '../../shared/terminal-side-effect-facts'
import {
  AGENT_STATUS_STALE_AFTER_MS,
  type AgentStatusIpcPayload,
  type ParsedAgentStatusPayload,
  type AgentStatusOrchestrationContext,
  type AgentStatusEntry
} from '../../shared/agent-status-types'
import { hasCompatibleAgentTitleIdentity } from '../../shared/agent-title-owner'
import { buildOrchestrationTaskDisplayMetadata } from '../../shared/orchestration-task-display'
import { iterateTerminalInputChunks } from '../../shared/terminal-input'
import {
  AGENT_PROMPT_BRACKETED_PASTE_END,
  AGENT_PROMPT_SUBMIT,
  AGENT_PROMPT_SUBMIT_DELAY_MS,
  buildAgentPromptPasteBytes
} from '../../shared/agent-prompt-injection'
import { randomUUID } from 'node:crypto'
import { homedir } from 'node:os'
import { join } from 'node:path'
import { OrchestrationDb } from './orchestration/db'
import { formatMessagesForInjection } from './orchestration/formatter'
import type {
  CreateWorktreeResult,
  DetectedWorktree,
  DetectedWorktreeListResult,
  GitWorktreeInfo,
  GlobalSettings,
  PersistedUIState,
  Repo,
  StatsSummary,
  Worktree,
  WorktreeLineage,
  WorkspaceKey,
  WorktreeLineageWarning,
  WorktreeMeta,
  WorktreeBaseStatusEvent,
  WorktreeRemoteBranchConflictEvent,
  WorktreeStartupLaunch,
  FolderWorkspace,
  MemorySnapshot,
  TuiAgent
} from '../../shared/types'
import type { SleepingAgentLaunchConfig } from '../../shared/agent-session-resume'
import type { RuntimeClientEvent } from '../../shared/runtime-client-events'
import { toRuntimeActivateWorktreeEvent } from '../../shared/runtime-client-events'
import type { SshConnectionState } from '../../shared/ssh-types'
import type { FeatureInteractionId } from '../../shared/feature-interactions'
import type { TerminalPaneSplitSource } from '../../shared/feature-education-telemetry'
import {
  FOLDER_WORKSPACE_INSTANCE_SEPARATOR,
  WORKTREE_ID_SEPARATOR,
  splitWorktreeIdForFilesystem
} from '../../shared/worktree-id'
import { isFolderRepo } from '../../shared/repo-kind'
import { getRepoProviderConnectionKey } from '../../shared/execution-host'
import { DEFAULT_WORKSPACE_STATUS_ID } from '../../shared/workspace-statuses'
import { SETUP_AGENT_SEQUENCE_STARTUP_COMMAND_ENV } from '../../shared/setup-agent-sequencing'
import { TASK_PROVIDERS } from '../../shared/task-providers'
import { isTerminalLeafId, makePaneKey, parsePaneKey } from '../../shared/stable-pane-id'
import { parseAppSshPtyId } from '../../shared/ssh-pty-id'
import { isValidHostTerminalTabId, isValidTerminalTabId } from '../../shared/terminal-tab-id'
import { buildAgentStartupPlan } from '../../shared/tui-agent-startup'
import { repoIsRemote } from '../../shared/agent-launch-remote'
import {
  isAgentForegroundWrapperProcess,
  isExpectedAgentProcess,
  recognizeAgentProcess
} from '../../shared/agent-process-recognition'
import { isTuiAgentEnabled } from '../../shared/tui-agent-selection'
import {
  resolveTuiAgentLaunchArgs,
  resolveTuiAgentLaunchEnv
} from '../../shared/tui-agent-launch-defaults'
import { resolveLocalWindowsAgentStartupShell } from '../../shared/windows-terminal-shell'
import { getTuiAgentLaunchCommand, TUI_AGENT_CONFIG } from '../../shared/tui-agent-config'
import { applyAgentStatusHooksEnabled } from '../agent-hooks/managed-agent-hook-controls'
import { isWindowsAbsolutePathLike, isPathInsideOrEqual } from '../../shared/cross-platform-path'
import { resolveTerminalStartupCwd } from '../../shared/terminal-startup-cwd'
import { isWslUncPath } from '../../shared/wsl-paths'
import { folderWorkspaceKey, parseWorkspaceKey } from '../../shared/workspace-scope'
import { folderWorkspaceToWorktree } from '../../shared/folder-workspace-worktree'
import {
  buildKnownOrcaWorkspaceLayouts,
  isLegacyRepoForExternalWorktreeVisibility,
  toDetectedWorktree
} from '../../shared/worktree-ownership'
import {
  BROWSER_HEADLESS_RUNTIME_CAPABILITY,
  MIN_COMPATIBLE_RUNTIME_CLIENT_VERSION,
  RUNTIME_CAPABILITIES,
  RUNTIME_PROTOCOL_VERSION,
  type RuntimeCapability
} from '../../shared/protocol-version'
import {
  configureAiVaultSessionSources,
  listAiVaultSessions
} from '../ai-vault/cached-session-list'
import type { AiVaultListArgs, AiVaultListResult } from '../../shared/ai-vault-types'
import type {
  WorkspacePortKillRequest,
  WorkspacePortKillResult,
  WorkspacePortProbe,
  WorkspacePortScanResult
} from '../../shared/workspace-ports'
import {
  filterWorkspacePortProbes,
  killWorkspacePort,
  scanWorkspacePortProbes
} from '../ports/workspace-port-ownership'
import { advertisedUrlWatcher } from '../ports/advertised-url-watcher'
import type {
  RuntimeTerminalRead,
  RuntimeTerminalRename,
  RuntimeTerminalAgentStatus,
  RuntimeTerminalSend,
  RuntimeTerminalCreate,
  RuntimeTerminalPresentation,
  RuntimeTerminalSplit,
  RuntimeTerminalFocus,
  RuntimeTerminalClose,
  RuntimeTerminalResolvePane,
  RuntimeStatus,
  RuntimeSyncWindowGraphResult,
  RuntimeTerminalWait,
  RuntimeTerminalWaitCondition,
  RuntimeWorktreePsSummary,
  RuntimeTerminalShow,
  RuntimeTerminalSummary,
  RuntimeSyncedLeaf,
  RuntimeMarkdownReadTabResult,
  RuntimeMarkdownSaveTabResult,
  RuntimeMobileSessionTabMove,
  RuntimeMobileSessionTabsResult,
  RuntimeMobileSessionTabsSnapshot,
  RuntimeBrowserDriverState,
  RuntimeSyncWindowGraph,
  RuntimeWorktreeListResult
} from '../../shared/runtime-types'
import type { AutomationService } from '../automations/service'
import { RuntimeBrowserCommands } from './orca-runtime-browser'
import { RuntimeEmulatorCommands, setEmulatorBridge } from './orca-runtime-emulator'
import { serveSimStateWatcher } from '../emulator/serve-sim-state-watcher'
import type { EmulatorBridge } from '../emulator/emulator-bridge'
import { RuntimeFileCommands } from './orca-runtime-files'
import { RuntimeGitCommands } from './orca-runtime-git'
import { ClaudeAgentTeamsService } from './claude-agent-teams-service'
import type {
  AgentTeamsTmuxCompatRequest,
  AgentTeamsTmuxCompatResponse
} from './claude-agent-teams-service'
import {
  buildClaudeAgentTeamsLaunchPlan,
  ensureClaudeAgentTeamsShimDir,
  resolveClaudeAgentTeamsShimBin
} from './claude-agent-teams-shim-env'
import {
  addClaudeTeammateModeAuto,
  addClaudeTeammateModeInProcess,
  type ClaudeAgentTeamsMode
} from '../../shared/claude-agent-teams-tmux-compat'
import { collectMemorySnapshot } from '../memory/collector'
import { BrowserWindow, ipcMain } from 'electron'
import type { AgentBrowserBridge } from '../browser/agent-browser-bridge'
import type { BrowserBackend } from '../browser/browser-backend'
import { getRepoUpstream } from '../github/client'
import {
  getLocalProjectWorktreeGitOptions,
  resolveLocalProjectRuntimeForRepo
} from '../project-runtime-git-options'
import type { ProjectExecutionRuntimeResolution } from '../../shared/project-execution-runtime'
import { FLOATING_TERMINAL_WORKTREE_ID } from '../../shared/constants'
import type { Store } from '../persistence'
import type { StatsCollector } from '../stats/collector'
import { AgentDetector } from '../stats/agent-detector'
import { mergeWorktree, areWorktreePathsEqual } from '../ipc/worktree-logic'
import { registerConptyDa1OverrideInstaller } from './terminal-model-query-authority'
import { registerTerminalViewAttributesApplier } from './terminal-view-attribute-store'
import {
  createMobileSessionTabsNotifyCoalescer,
  type MobileSessionTabsNotifyCoalescer
} from './mobile-session-tabs-notify-coalescer'
import type { IPtyProvider } from '../providers/types'
import { getRemoteFilesystemProvider } from '../providers/ssh-filesystem-dispatch'
import {
  assertFolderWorkspacePathUsable,
  getFolderWorkspacePathStatus,
  inferFolderWorkspacePathConnection
} from '../project-groups/folder-workspace-path-status'
import { githubAvatarIcon } from '../../shared/repo-icon'
import type { ClaudeAccountService } from '../claude-accounts/service'
import type { CodexAccountService } from '../codex-accounts/service'
import type { RateLimitService } from '../rate-limits/service'
import { applyPRBotAuthorOverride } from '../../shared/pr-bot-author-overrides'
import type { VoiceSettings } from '../../shared/speech-types'
import type { CommitMessageAgentEnvironmentResolvers } from '../text-generation/commit-message-agent-environment'
import {
  appendRecentPtyOutput,
  appendRecentPtyPathCandidates,
  recentTerminalPathCandidatesIncludePath,
  recentTerminalOutputIncludesPath,
  buildPreview,
  type TerminalTailWaitState,
  computeTerminalTailWaitState,
  tailGainedNewerBlockedReason,
  appendNormalizedToTailBuffer,
  assertTerminalInputWithinLimitWithYield,
  branchSelectorMatches,
  buildPtyTerminalWaitBlockedResult,
  buildPtyTerminalWaitResult,
  buildSendPayload,
  buildTerminalWaitBlockedResult,
  buildTerminalWaitResult,
  buildTerminalWaitText,
  classifyAgentTitle,
  classifyLatestAgentTitle,
  detectExplicitIdleStatusFromTitle,
  detectTerminalWaitBlockedReason,
  findResolvedWorktreeIdForPath,
  getLatestAgentCandidateTitle,
  getLatestAgentCandidateTitleInfo,
  getLatestLeafTitle,
  getLatestPtyTitle,
  getTerminalState,
  inferWorktreeIdFromPtyId,
  isKnownReadyPromptPreview,
  mapExplicitAgentStateToRuntimeTerminalStatus,
  maxTimestamp,
  normalizeTerminalChunk,
  parseRuntimeWorktreeId,
  readTerminalTail,
  type RetainedTailRedrawCursor,
  runtimePathsEqual,
  setsEqual,
  tailStateMatches,
  terminalTitleBlocksExplicitAgentStatus,
  TUI_IDLE_DEFAULT_TIMEOUT_MS,
  TUI_IDLE_POLL_INTERVAL_MS,
  TUI_IDLE_QUIESCENCE_MS
} from './orca-runtime-tail-buffer'
// Why: OrcaRuntimeService calls the vast majority of tail-buffer.ts's helpers
// directly throughout its body (terminal-wait detection, agent-title
// classification, worktree-status merging, ...), not just the 10 originally
// public exports re-exported at the bottom of this file for external API
// compatibility. This bulk import is the honest reflection of how tightly
// coupled this "tail region" actually is to OrcaRuntimeService — confirmed
// via tsc after the plain barrel-move first attempt surfaced 50 missing
// names (BUG-FE-BIGFILE-002 / TASK-BIGFILE-008).
import type {
  DriverState,
  RuntimePtyController,
  RuntimeTerminalAgentStatusEvent
} from './orca-runtime-types'
// Why: 12 of the original 14 types (BUG-FE-BIGFILE-002 / TASK-BIGFILE-009)
// are used as parameter/return types throughout OrcaRuntimeService's body —
// imported back here for internal use; re-exported below for external
// importers (e.g. main/runtime/rpc/methods/terminal.ts's `DriverState`).
// RuntimeAutomationCreateInput/UpdateInput moved on to
// orca-runtime-automation.ts (TASK-BIGFILE-036) and are no longer imported
// back here, only re-exported below for external API compatibility.

export type RuntimeAccountServices = {
  claudeAccounts: ClaudeAccountService
  codexAccounts: CodexAccountService
  rateLimits: RateLimitService
}

export type RuntimeStore = {
  getRepos: Store['getRepos']
  getRepo: Store['getRepo']
  addRepo: Store['addRepo']
  updateRepo: Store['updateRepo']
  getProjects?: Store['getProjects']
  updateProject?: Store['updateProject']
  getProjectHostSetups?: Store['getProjectHostSetups']
  createProjectHostSetup?: Store['createProjectHostSetup']
  updateProjectHostSetup?: Store['updateProjectHostSetup']
  deleteProjectHostSetup?: Store['deleteProjectHostSetup']
  getProjectGroups?: Store['getProjectGroups']
  createProjectGroup?: Store['createProjectGroup']
  updateProjectGroup?: Store['updateProjectGroup']
  deleteProjectGroup?: Store['deleteProjectGroup']
  moveProjectToGroup?: Store['moveProjectToGroup']
  getFolderWorkspaces?: Store['getFolderWorkspaces']
  createFolderWorkspace?: Store['createFolderWorkspace']
  updateFolderWorkspace?: Store['updateFolderWorkspace']
  removeFolderWorkspace?: Store['removeFolderWorkspace']
  removeProject?: Store['removeProject']
  reorderRepos?: Store['reorderRepos']
  getAllWorktreeMeta: Store['getAllWorktreeMeta']
  getWorktreeMeta: Store['getWorktreeMeta']
  setWorktreeMeta: Store['setWorktreeMeta']
  removeWorktreeMeta: Store['removeWorktreeMeta']
  getWorktreeLineage?: Store['getWorktreeLineage']
  getAllWorktreeLineage?: Store['getAllWorktreeLineage']
  setWorktreeLineage?: Store['setWorktreeLineage']
  removeWorktreeLineage?: Store['removeWorktreeLineage']
  getAllWorkspaceLineage?: Store['getAllWorkspaceLineage']
  setWorkspaceLineage?: Store['setWorkspaceLineage']
  removeWorkspaceLineage?: Store['removeWorkspaceLineage']
  getGitHubCache: Store['getGitHubCache']
  getWorkspaceSession?: Store['getWorkspaceSession']
  setWorkspaceSession?: Store['setWorkspaceSession']
  persistPtyBinding?: Store['persistPtyBinding']
  getUI?: Store['getUI']
  updateUI?: Store['updateUI']
  recordFeatureInteraction?: Store['recordFeatureInteraction']
  listAutomations?: Store['listAutomations']
  listAutomationRuns?: Store['listAutomationRuns']
  createAutomation?: Store['createAutomation']
  updateAutomation?: Store['updateAutomation']
  deleteAutomation?: Store['deleteAutomation']
  getSparsePresets?: Store['getSparsePresets']
  saveSparsePreset?: Store['saveSparsePreset']
  getSettings(): {
    workspaceDir: string
    nestWorkspaces: boolean
    refreshLocalBaseRefOnWorktreeCreate: boolean
    localBaseRefSuggestionDismissed?: boolean
    branchPrefix: string
    branchPrefixCustom: string
    defaultTuiAgent?: GlobalSettings['defaultTuiAgent']
    disabledTuiAgents?: GlobalSettings['disabledTuiAgents']
    agentCmdOverrides?: GlobalSettings['agentCmdOverrides']
    agentDefaultArgs?: GlobalSettings['agentDefaultArgs']
    agentDefaultEnv?: GlobalSettings['agentDefaultEnv']
    terminalWindowsShell?: GlobalSettings['terminalWindowsShell']
    agentStatusHooksEnabled?: GlobalSettings['agentStatusHooksEnabled']
    defaultTaskSource?: GlobalSettings['defaultTaskSource']
    defaultTaskViewPreset?: GlobalSettings['defaultTaskViewPreset']
    visibleTaskProviders?: GlobalSettings['visibleTaskProviders']
    defaultRepoSelection?: GlobalSettings['defaultRepoSelection']
    defaultLinearTeamSelection?: GlobalSettings['defaultLinearTeamSelection']
    githubProjects?: GlobalSettings['githubProjects']
    experimentalNewWorktreeCardStyle?: GlobalSettings['experimentalNewWorktreeCardStyle']
    compactWorktreeCards?: GlobalSettings['compactWorktreeCards']
    minimaxGroupId?: GlobalSettings['minimaxGroupId']
    minimaxUsageModels?: GlobalSettings['minimaxUsageModels']
    prBotAuthorOverrides?: GlobalSettings['prBotAuthorOverrides']
    gitlabProjects?: GlobalSettings['gitlabProjects']
    mobileAutoRestoreFitMs?: number | null
    mobileEmulatorEnabled?: boolean
    mobileEmulatorDefaultDeviceUdid?: string | null
    voice?: VoiceSettings
    claudeAgentTeamsMode?: GlobalSettings['claudeAgentTeamsMode']
    // Why: Phase-5 query responder kill switches — read per chunk in
    // onPtyData to capture reply ownership at ingestion.
    terminalMainSideEffectAuthority?: GlobalSettings['terminalMainSideEffectAuthority']
    terminalHiddenDeliveryGate?: GlobalSettings['terminalHiddenDeliveryGate']
    terminalModelQueryAuthority?: GlobalSettings['terminalModelQueryAuthority']
  }
  // Why: narrow to `unknown` return so test mocks can return void without
  // a cast. The runtime never reads the return value — the persisted value
  // is read back via getSettings() on the next access.
  updateSettings?: (
    updates: Partial<GlobalSettings>,
    options?: { notifyListeners?: boolean; originWebContentsId?: number }
  ) => unknown
}

export type RuntimeLeafRecord = RuntimeSyncedLeaf & {
  ptyGeneration: number
  connected: boolean
  writable: boolean
  lastOutputAt: number | null
  lastExitCode: number | null
  tailBuffer: string[]
  tailPartialLine: string
  tailPendingAnsi: string
  tailRedrawCursor: RetainedTailRedrawCursor | null
  tailTruncated: boolean
  tailLinesTotal: number
  preview: string
  waitBlockedAt: number | null
  // Why: memoized wait scan of the current retained tail so the next PTY chunk
  // reuses it as its "previous" state instead of rebuilding + rescanning the
  // full tail. See computeTerminalTailWaitState.
  tailWaitState?: TerminalTailWaitState
  lastAgentStatus: AgentStatus | null
  // Why: the most recent OSC title observed on this leaf's PTY data. Used by
  // worktree.ps so daemon-hosted terminals (no renderer pushing pane titles)
  // still recompute working/idle from the live title each call instead of
  // serving a stale `lastAgentStatus` after the agent process exits and the
  // shell takes over the title — the bug behind issue #1437.
  lastOscTitle: string | null
  lastOscTitleAt: number | null
  paneTitleUpdatedAt: number | null
}

function isCursorAgentOrchestrationTarget(
  leaf: RuntimeLeafRecord,
  tabTitle: string | null | undefined
): boolean {
  return [leaf.lastOscTitle, leaf.paneTitle, tabTitle].some(isCursorAgentTitle)
}

export type RuntimePtyWorktreeRecord = {
  ptyId: string
  worktreeId: string
  connectionId: string | null
  // Why: background CLI PTYs can outlive a failed renderer reveal. Preserve the
  // spawn-time tab/pane identity so later reveals can adopt under the env key.
  tabId: string | null
  paneKey: string | null
  launchConfig: SleepingAgentLaunchConfig | null
  launchToken: string | null
  launchAgent: TuiAgent | null
  foregroundAgent: TuiAgent | null
  connected: boolean
  disconnectedAt: number | null
  lastExitCode: number | null
  lastAgentStatus: AgentStatus | null
  lastOscTitle: string | null
  lastOscTitleAt: number | null
  managementTitle: string | null
  managementTitleAt: number | null
  title: string | null
  titleUpdatedAt: number | null
  lastOutputAt: number | null
  tailBuffer: string[]
  tailPartialLine: string
  tailPendingAnsi: string
  tailRedrawCursor: RetainedTailRedrawCursor | null
  tailTruncated: boolean
  tailLinesTotal: number
  preview: string
  waitBlockedAt: number | null
  // Why: memoized wait scan of the current retained tail (see RuntimeLeafRecord).
  tailWaitState?: TerminalTailWaitState
}

export type TerminalCreateOptions = {
  command?: string
  claudeAgentTeamsSourceCommand?: string
  cwd?: string
  env?: Record<string, string>
  launchConfig?: WorktreeStartupLaunch['launchConfig']
  launchToken?: string
  launchAgent?: TuiAgent
  startupCommandDelivery?: WorktreeStartupLaunch['startupCommandDelivery']
  telemetry?: WorktreeStartupLaunch['telemetry']
  title?: string
  focus?: boolean
  rendererBacked?: boolean
  activate?: boolean
  presentation?: RuntimeTerminalPresentation
  tabId?: string
  leafId?: string
  sessionId?: string
  persistHostSessionBinding?: boolean
  // Why: the headless mobile-session create publishes its own authoritative
  // snapshot (with the correct target group) right after spawn. Skip the
  // intermediate pty-backed publish so the new tab doesn't briefly flash in
  // the wrong (active) group before the corrected snapshot lands.
  deferMobileSessionPublish?: boolean
}

export type PtyForegroundAgentRefresh = {
  promise: Promise<boolean>
  startedAfterTitleObservation: number
  requestedAfterTitleObservation: number
}

function copySleepingAgentLaunchConfig(
  config: SleepingAgentLaunchConfig
): SleepingAgentLaunchConfig {
  return {
    ...(config.agentCommand ? { agentCommand: config.agentCommand } : {}),
    agentArgs: config.agentArgs,
    agentEnv: { ...config.agentEnv }
  }
}

function normalizeAgentLaunchCommandForMatch(command: string): string {
  return command.trim().replace(/\s+/g, ' ')
}

function resolveBareAgentLaunchCommand(args: {
  command: string | undefined
  settings: {
    agentCmdOverrides?: Partial<Record<TuiAgent, string>> | null
    disabledTuiAgents?: Iterable<unknown> | null
  }
  platform: NodeJS.Platform
  isRemote: boolean
}): TuiAgent | null {
  const command = args.command ? normalizeAgentLaunchCommandForMatch(args.command) : ''
  if (!command) {
    return null
  }

  const cmdOverrides = args.settings.agentCmdOverrides ?? {}
  for (const agent of Object.keys(TUI_AGENT_CONFIG) as TuiAgent[]) {
    if (!isTuiAgentEnabled(agent, args.settings.disabledTuiAgents)) {
      continue
    }
    const override = cmdOverrides[agent]?.trim()
    const defaultLaunchCommand = getTuiAgentLaunchCommand(TUI_AGENT_CONFIG[agent], args.platform, {
      isRemote: args.isRemote
    })
    const launchCommands = override ? [defaultLaunchCommand, override] : [defaultLaunchCommand]
    if (
      launchCommands.some((candidate) => command === normalizeAgentLaunchCommandForMatch(candidate))
    ) {
      return agent
    }
  }

  return null
}

function inferCapturedClaudeAgentTeamsMode(
  launchConfig: SleepingAgentLaunchConfig | undefined,
  command: string | undefined,
  currentMode: ClaudeAgentTeamsMode | undefined
): ClaudeAgentTeamsMode | undefined {
  const capturedCommand = launchConfig?.agentCommand?.trim() || command?.trim() || ''
  const capturedArgs = launchConfig?.agentArgs?.trim() ?? ''
  const capturedLaunch = `${capturedCommand} ${capturedArgs}`.trim()
  if (/(^|\s)--teammate-mode(?:=|\s+)auto(?:\s|$)/.test(capturedLaunch)) {
    return 'native-panes-shim'
  }
  if (/(^|\s)--teammate-mode(?:=|\s+)in-process(?:\s|$)/.test(capturedLaunch)) {
    return 'in-process'
  }
  if (launchConfig && /(^|\s)--resume(?:\s|=|$)/.test(command?.trim() ?? '')) {
    return 'off'
  }
  return currentMode
}

export type RuntimePtyTitleTrackerEntry = {
  tracker: TerminalTitleTracker
  // Why: onPtyData batches the mobile session-tab touch to once per chunk;
  // the stale-working-title timer fires between chunks and must touch
  // immediately. These flags route the tracker callback to the right mode.
  applyingChunk: boolean
  // Why: synthetic spinner ticks arrive ~12.5x/sec per working pane; the
  // synthetic path gates mobile snapshot fan-out on a non-decorative title
  // change (spinner glyph + status comparison key kept below).
  applyingSyntheticFrame: boolean
  lastMobileTitleGateKey: string | null
  chunkTouchedSessionTabs: boolean
  // Why: facts observed while applying a chunk are batched into one
  // pty:sideEffect emission per chunk, preserving byte order (titles in
  // sequence, then bell). Timer-fired facts emit immediately between chunks.
  pendingFacts: TerminalSideEffectFact[]
  // Why: Command Code lacks hooks, so its working/done state is scraped from
  // TUI output. Null when no side-effect consumer exists (headless serve) —
  // the scrape produces facts only.
  commandCodeDetector: { observe: (data: string) => boolean } | null
}

// Why: the full OSC 9999 payload flows through emitTerminalAgentStatusEvents and
// is then forwarded to the renderer and dropped. Mobile is served by the main
// process and has no renderer store, so we retain the latest payload per pane
// here to feed worktree.ps's inline agent rows (1:1 with the desktop sidebar).
export type RuntimeAgentRowSnapshot = {
  paneKey: string
  ptyId: string
  worktreeId?: string
  tabId?: string
  payload: ParsedAgentStatusPayload
  // When the current payload.state was first observed for this pane (ms).
  stateStartedAt: number
  updatedAt: number
}

function getAgentLaunchPlatformForRepo(
  repo: Pick<Repo, 'connectionId' | 'path'>,
  projectRuntime?: ProjectExecutionRuntimeResolution
): NodeJS.Platform {
  if (!repo.connectionId) {
    if (projectRuntime?.status === 'repair-required') {
      return projectRuntime.repair.preferredRuntime.kind === 'wsl' ? 'linux' : process.platform
    }
    if (projectRuntime?.status === 'resolved' && projectRuntime.runtime.kind === 'wsl') {
      return 'linux'
    }
    return process.platform
  }
  return isWindowsAbsolutePathLike(repo.path) ? 'win32' : 'linux'
}

const FOREGROUND_AGENT_WRAPPER_RETRY_INTERVAL_MS = 150
const FOREGROUND_AGENT_WRAPPER_RETRY_TIMEOUT_MS = 6_500

function createTerminalRevealWarning(handle: string, error?: unknown): string {
  const reason =
    error instanceof Error && error.message.trim().length > 0
      ? ` Reason: ${error.message.trim()}.`
      : ''
  return [
    `Terminal ${handle} is running, but Orca could not make it discoverable.${reason}`,
    `Run \`orca terminal focus --terminal ${handle}\` to reveal and focus it.`
  ].join(' ')
}

function resolveTerminalPresentation(opts: {
  presentation?: RuntimeTerminalPresentation
  focus?: boolean
  activate?: boolean
}): RuntimeTerminalPresentation | undefined {
  if (opts.presentation) {
    return opts.presentation
  }
  if (opts.focus === true || opts.activate === true) {
    return 'focused'
  }
  return undefined
}

type RuntimeNotifier = {
  worktreesChanged(repoId: string, renamed?: { oldWorktreeId: string; newWorktreeId: string }): void
  worktreeBaseStatus?(event: WorktreeBaseStatusEvent): void
  worktreeRemoteBranchConflict?(event: WorktreeRemoteBranchConflictEvent): void
  reposChanged(): void
  activateWorktree(
    repoId: string,
    worktreeId: string,
    setup?: CreateWorktreeResult['setup'],
    startup?: WorktreeStartupLaunch,
    defaultTabs?: CreateWorktreeResult['defaultTabs']
  ): void
  createTerminal(
    worktreeId: string,
    opts: {
      command?: string
      cwd?: string
      env?: Record<string, string>
      title?: string
      presentation?: RuntimeTerminalPresentation
    }
  ): void
  revealTerminalSession?(
    worktreeId: string,
    opts: {
      ptyId: string
      title?: string | null
      cwd?: string
      launchConfig?: SleepingAgentLaunchConfig
      launchToken?: string
      launchAgent?: TuiAgent
      activate?: boolean
      presentation?: RuntimeTerminalPresentation
      tabId?: string
      leafId?: string
      splitFromLeafId?: string
      splitDirection?: 'horizontal' | 'vertical'
      splitTelemetrySource?: TerminalPaneSplitSource
    }
  ):
    | Promise<{ tabId: string; title?: string | null }>
    | { tabId: string; title?: string | null }
    | void
  splitTerminal(
    tabId: string,
    paneRuntimeId: number,
    opts: {
      direction: 'horizontal' | 'vertical'
      command?: string
      telemetrySource?: TerminalPaneSplitSource
    }
  ): void
  renameTerminal(tabId: string, title: string | null): void
  focusTerminal(tabId: string, worktreeId: string, leafId?: string | null): void
  focusEditorTab?(tabId: string, worktreeId: string): void
  closeSessionTab?(tabId: string, worktreeId: string): void
  moveSessionTab?(worktreeId: string, move: RuntimeMobileSessionTabMove): void
  openFile?(
    worktreeId: string,
    filePath: string,
    relativePath: string,
    runtimeEnvironmentId?: string | null
  ): void
  openDiff?(
    worktreeId: string,
    filePath: string,
    relativePath: string,
    staged: boolean,
    runtimeEnvironmentId?: string | null
  ): void
  readMobileMarkdownTab?(worktreeId: string, tabId: string): Promise<RuntimeMarkdownReadTabResult>
  saveMobileMarkdownTab?(
    worktreeId: string,
    tabId: string,
    baseVersion: string,
    content: string
  ): Promise<RuntimeMarkdownSaveTabResult>
  closeTerminal(tabId: string, paneRuntimeId?: number): void
  sleepWorktree(worktreeId: string): void
  // Why: a phone opening a worktree wakes its slept agents by asking the host
  // renderer to run its own navigation-free wake (experimental agent sleep);
  // the runtime has no in-memory sleeping records or wake authority. Optional to
  // match the many renderer-backed notifier methods only the real bridge wires.
  resumeSleepingAgents?(worktreeId: string): void
  terminalFitOverrideChanged(
    ptyId: string,
    mode: 'mobile-fit' | 'remote-desktop-fit' | 'desktop-fit',
    cols: number,
    rows: number
  ): void
  // Why: presence-based lock signal — desktop renderer mounts the lock
  // banner when `driver.kind === 'mobile'` and unmounts otherwise. The
  // structured payload (vs a `locked: boolean`) carries the active mobile
  // actor's clientId so the renderer can disambiguate multi-phone scenarios
  // and so a future write coordinator can use the same signal as scheduling
  // input. See docs/mobile-presence-lock.md.
  terminalDriverChanged(ptyId: string, driver: DriverState): void
  browserDriverChanged?(browserPageId: string, driver: RuntimeBrowserDriverState): void
}

export type TerminalHandleRecord = {
  handle: string
  runtimeId: string
  rendererGraphEpoch: number
  worktreeId: string
  tabId: string
  leafId: string
  ptyId: string | null
  ptyGeneration: number
}

export type TerminalWaiter = {
  handle: string
  condition: RuntimeTerminalWaitCondition
  resolve: (result: RuntimeTerminalWait) => void
  reject: (error: Error) => void
  timeout: NodeJS.Timeout | null
  pollInterval: NodeJS.Timeout | null
  abortCleanup: (() => void) | null
}

export function omitUndefinedProperties<T extends Record<string, unknown>>(value: T): Partial<T> {
  return Object.fromEntries(
    Object.entries(value).filter(([, entry]) => entry !== undefined)
  ) as Partial<T>
}

export function getRuntimeFolderWorkspaceRootId(repo: Repo): string {
  return `${repo.id}::${repo.path}`
}

function getRuntimeFolderWorkspaceInstanceIdentity(repo: Repo, worktreeId: string): string {
  const prefix = `${getRuntimeFolderWorkspaceRootId(repo)}${FOLDER_WORKSPACE_INSTANCE_SEPARATOR}`
  return worktreeId.startsWith(prefix) ? worktreeId.slice(prefix.length) : randomUUID()
}

function isRuntimeFolderWorkspaceIdForRepo(repo: Repo, worktreeId: string): boolean {
  const rootId = getRuntimeFolderWorkspaceRootId(repo)
  return (
    worktreeId === rootId ||
    worktreeId.startsWith(`${rootId}${FOLDER_WORKSPACE_INSTANCE_SEPARATOR}`)
  )
}

export function mergeRuntimeFolderWorkspace(
  repo: Repo,
  worktreeId: string,
  meta: WorktreeMeta
): Worktree {
  return {
    id: worktreeId,
    ...(meta.instanceId !== undefined ? { instanceId: meta.instanceId } : {}),
    repoId: repo.id,
    ...(meta.projectId !== undefined ? { projectId: meta.projectId } : {}),
    ...(meta.hostId !== undefined ? { hostId: meta.hostId } : {}),
    ...(meta.projectHostSetupId !== undefined
      ? { projectHostSetupId: meta.projectHostSetupId }
      : {}),
    path: repo.path,
    head: '',
    branch: '',
    isBare: false,
    isMainWorktree: worktreeId === getRuntimeFolderWorkspaceRootId(repo),
    displayName: meta.displayName || repo.displayName,
    comment: meta.comment || '',
    linkedIssue: meta.linkedIssue ?? null,
    linkedPR: meta.linkedPR ?? null,
    linkedLinearIssue: meta.linkedLinearIssue ?? null,
    linkedLinearIssueWorkspaceId: meta.linkedLinearIssueWorkspaceId ?? null,
    linkedLinearIssueOrganizationUrlKey: meta.linkedLinearIssueOrganizationUrlKey ?? null,
    linkedGitLabMR: meta.linkedGitLabMR ?? null,
    linkedGitLabIssue: meta.linkedGitLabIssue ?? null,
    linkedBitbucketPR: meta.linkedBitbucketPR ?? null,
    linkedAzureDevOpsPR: meta.linkedAzureDevOpsPR ?? null,
    linkedGiteaPR: meta.linkedGiteaPR ?? null,
    isArchived: meta.isArchived ?? false,
    isUnread: meta.isUnread ?? false,
    isPinned: meta.isPinned ?? false,
    sortOrder: meta.sortOrder ?? 0,
    ...(meta.manualOrder !== undefined ? { manualOrder: meta.manualOrder } : {}),
    lastActivityAt: meta.lastActivityAt ?? 0,
    ...(meta.createdAt !== undefined ? { createdAt: meta.createdAt } : {}),
    ...(meta.createdWithAgent !== undefined ? { createdWithAgent: meta.createdWithAgent } : {}),
    ...(meta.automationProvenance !== undefined
      ? { automationProvenance: meta.automationProvenance }
      : {}),
    ...(meta.priorWorktreeIds !== undefined ? { priorWorktreeIds: meta.priorWorktreeIds } : {}),
    workspaceStatus: meta.workspaceStatus ?? DEFAULT_WORKSPACE_STATUS_ID,
    diffComments: meta.diffComments,
    mobileDiffReview: meta.mobileDiffReview
  }
}

export function listRuntimeFolderWorkspaces(
  store: Pick<RuntimeStore, 'getAllWorktreeMeta' | 'setWorktreeMeta'>,
  repo: Repo
): Worktree[] {
  const rootId = getRuntimeFolderWorkspaceRootId(repo)
  const allMeta = store.getAllWorktreeMeta()
  const ids = Object.keys(allMeta).filter((worktreeId) =>
    isRuntimeFolderWorkspaceIdForRepo(repo, worktreeId)
  )
  if (!ids.includes(rootId)) {
    ids.unshift(rootId)
  } else {
    ids.sort((left, right) => {
      if (left === rootId) {
        return -1
      }
      if (right === rootId) {
        return 1
      }
      return 0
    })
  }

  return ids.map((worktreeId) => {
    const existing = allMeta[worktreeId]
    const meta = existing?.instanceId
      ? existing
      : store.setWorktreeMeta(worktreeId, {
          instanceId: getRuntimeFolderWorkspaceInstanceIdentity(repo, worktreeId),
          ...(existing ? {} : { displayName: repo.displayName, lastActivityAt: Date.now() })
        })
    return mergeRuntimeFolderWorkspace(repo, worktreeId, meta)
  })
}

// Clamp terminal dimensions to the PTY's supported range (cols 20–240, rows 8–120).
// Subscribe a listener to a per-key Set, pruning the key's entry once its last
// listener unsubscribes. Returns the unsubscribe callback.
export function addListenerToMap<T>(
  map: Map<string, Set<T>>,
  key: string,
  listener: T
): () => void {
  let listeners = map.get(key)
  if (!listeners) {
    listeners = new Set<T>()
    map.set(key, listeners)
  }
  const set = listeners
  set.add(listener)
  return () => {
    set.delete(listener)
    if (set.size === 0) {
      map.delete(key)
    }
  }
}

export type ResolvedWorktree = Worktree & {
  parentWorktreeId: string | null
  childWorktreeIds: string[]
  lineage: WorktreeLineage | null
  git: GitWorktreeInfo
}

const AGENT_HOOK_RUNTIME_ENV_KEYS = [
  'ORCA_AGENT_HOOK_PORT',
  'ORCA_AGENT_HOOK_TOKEN',
  'ORCA_AGENT_HOOK_ENV',
  'ORCA_AGENT_HOOK_VERSION',
  'ORCA_AGENT_HOOK_ENDPOINT'
] as const

export type TerminalWorkspaceLaunchScope = {
  id: string
  path: string
  connectionId: string | null
  repo: Repo | null
  folderWorkspace: FolderWorkspace | null
}

export type WorktreeLineageInput = {
  parentWorkspace?: string
  envParentWorkspace?: string
  parentWorktree?: string
  cwdParentWorktree?: string
  noParent?: boolean
  callerTerminalHandle?: string
  comment?: string
  orchestrationContext?: {
    parentWorktreeId?: string
    orchestrationRunId?: string
    taskId?: string
    coordinatorHandle?: string
  }
}

export type ResolvedWorkspaceParent =
  | {
      type: 'worktree'
      workspaceKey: WorkspaceKey
      worktree: ResolvedWorktree
      instanceId: string | null
    }
  | {
      type: 'folder'
      workspaceKey: WorkspaceKey
      folderWorkspace: FolderWorkspace
      instanceId: string | null
    }

export type WorktreeLineageResolution =
  | {
      kind: 'lineage'
      parent: ResolvedWorkspaceParent
      origin: WorktreeLineage['origin']
      capture: WorktreeLineage['capture']
      orchestrationRunId?: string
      taskId?: string
      coordinatorHandle?: string
      createdByTerminalHandle?: string
    }
  | {
      kind: 'none'
      warnings: WorktreeLineageWarning[]
    }

export type RuntimeWorktreeScanResult =
  | { ok: true; worktrees: GitWorktreeInfo[] }
  | { ok: false; worktrees: GitWorktreeInfo[] }

export type WorktreeLineageCandidate = {
  source: 'env-workspace' | 'cwd-context' | 'terminal-context' | 'orchestration-context'
  parent: ResolvedWorkspaceParent
  orchestrationRunId?: string
  taskId?: string
  coordinatorHandle?: string
}

export class RuntimeLineageError extends Error {
  code: string
  data?: unknown

  constructor(code: string, message: string, data?: unknown) {
    super(message)
    this.code = code
    this.data = data
  }
}

export class WorktreeIdRequiresFullPathError extends Error {
  readonly code = 'worktree_id_requires_full_path'

  constructor() {
    super(
      'Worktree id selectors must use the full <repo-id>::<path> value. Use the id from `orca worktree list --json`, or target by path:<path>, branch:<branch>, or issue:<number>.'
    )
  }
}

export class OrcaRuntimeService {
  private readonly runtimeId = randomUUID()
  private readonly startedAt = Date.now()
  private readonly store: RuntimeStore | null
  // Why: the mutable live graph (leaves/tabs/PTY handles/waiters) is read
  // from nearly every method in this class (TASK-BIGFILE-035 field-span
  // analysis: 15,000-21,000+ line reference spread) — too pervasive to move
  // alongside any single domain, so it gets its own logic-free holder
  // (TASK-BIGFILE-041) instead of living as scattered fields here.
  private readonly graph = new RuntimeGraphStore()
  private mobileSessionTabsByWorktree = new Map<string, RuntimeMobileSessionTabsSnapshot>()
  private mobileSessionTabListeners = new Set<(snapshot: RuntimeMobileSessionTabsResult) => void>()
  // Why: coalesces title/status-driven session.tabs emits so spinner churn
  // doesn't fan out (and per-client JSON.stringify) a snapshot several times a
  // second. Emit reads the latest snapshot, so only the freshest version ships.
  private readonly mobileSessionTabsNotifyCoalescer: MobileSessionTabsNotifyCoalescer =
    createMobileSessionTabsNotifyCoalescer((worktreeId) =>
      this.notifyMobileSessionTabsChangedNow(worktreeId)
    )
  private ptyController: RuntimePtyController | null = null
  private notifier: RuntimeNotifier | null = null
  private clientEventListeners = new Set<(event: RuntimeClientEvent) => void>()
  private forkBackfillStarted = false
  private agentBrowserBridge: AgentBrowserBridge | null = null
  private offscreenBrowserBackend: BrowserBackend | null = null
  private emulatorBridge: EmulatorBridge | null = null
  /** Web Push manager — optional; null until setPushManager() is called. TASK-036. */
  private pushManager: WebPushManager | null = null
  private agentDetector: AgentDetector | null = null
  private _orchestrationDb: OrchestrationDb | null = null
  // Why: mobile clients subscribe to terminal output via terminal.subscribe.
  // These listeners fire on every onPtyData call, enabling real-time streaming
  // without polling. Keyed by ptyId for O(1) lookup per data event.
  private dataListeners = new Map<
    string,
    Set<(data: string, meta?: { seq?: number; rawLength?: number; cwd?: string }) => void>
  >()
  private readonly ptyTranscripts = new RuntimePtyTranscriptStore()
  private titleObservationSequence = 0
  private readonly headlessTerminalCommands = new RuntimeHeadlessTerminalCommands({
    getGraph: () => this.graph,
    getStore: () => this.store,
    getPtyController: () => this.ptyController,
    getPtyTranscripts: () => this.ptyTranscripts,
    getHeadlessHydrationState: () => this.headlessHydrationState,
    getTerminalSize: (ptyId) => this.getTerminalSize(ptyId),
    hasRemoteTerminalViewSubscriber: (ptyId) => this.hasRemoteTerminalViewSubscriber(ptyId),
    recordOsc7MetadataForPty: (ptyId, data) => this.recordOsc7MetadataForPty(ptyId, data),
    recordRecentPtyOutputForPathProvenance: (ptyId, data) =>
      this.recordRecentPtyOutputForPathProvenance(ptyId, data),
    getTrackedRawTitleForPty: (ptyId) => this.getTrackedRawTitleForPty(ptyId),
    applySeededAgentStatus: (ptyId, title) => this.applySeededAgentStatus(ptyId, title),
    pathFlavorForPty: (pty) => this.pathFlavorForPty(pty),
    preferTrackedLastTitle: (ptyId, snapshot) => this.preferTrackedLastTitle(ptyId, snapshot)
  })
  // Why: latest agent-status payload per pane, retained so worktree.ps can serve
  // mobile the same inline agent rows the desktop sidebar renders. Cleared on pty
  // teardown so dead agents don't linger. See RuntimeAgentRowSnapshot.
  private latestAgentStatusByPaneKey = new Map<string, RuntimeAgentRowSnapshot>()
  // Why: per-PTY hydration state guards against double-hydration. Keys:
  //   'pending'  → maybeHydrateHeadlessFromRenderer is in flight
  //   'done'     → hydration completed (success or skip); never run again
  // Absent  → hydration has not been considered yet for this PTY.
  // See docs/mobile-prefer-renderer-scrollback.md.
  private headlessHydrationState = new Map<string, 'pending' | 'done'>()

  private stats: StatsCollector | null = null
  private readonly getLocalProviderFn: (() => IPtyProvider) | null
  private readonly onPtyStopped: ((ptyId: string) => void) | null
  private readonly onTerminalAgentStatus: ((event: RuntimeTerminalAgentStatusEvent) => void) | null
  private readonly onTerminalSideEffects: ((batch: TerminalSideEffectBatch) => void) | null
  private readonly getAgentStatusSnapshotFn: (() => AgentStatusIpcPayload[]) | null
  private readonly buildAgentHookPtyEnv: (() => Record<string, string>) | null
  private commitMessageAgentEnv: CommitMessageAgentEnvironmentResolvers | null = null
  private automationService: AutomationService | null = null
  private readonly claudeAgentTeams = new ClaudeAgentTeamsService()

  constructor(
    store: RuntimeStore | null = null,
    stats?: StatsCollector,
    deps?: {
      getLocalProvider?: () => IPtyProvider
      onPtyStopped?: (ptyId: string) => void
      onTerminalAgentStatus?: (event: RuntimeTerminalAgentStatusEvent) => void
      onTerminalSideEffects?: (batch: TerminalSideEffectBatch) => void
      // Why: agent status mostly arrives via hooks (agent-hooks/server), not OSC
      // terminal output. worktree.ps reads this at query time so mobile shows the
      // same inline agent rows the desktop sidebar does — same source, 1:1.
      getAgentStatusSnapshot?: () => AgentStatusIpcPayload[]
      // Why: codex-home paths for the Agent Session History scan must be sourced
      // here, not via the window-only registerCoreHandlers path — that path never
      // runs under `orca serve`, so remote/SSH hosts would silently drop
      // managed-Codex sessions. The runtime ctor runs in BOTH window and serve.
      getAdditionalAiVaultCodexHomePaths?: () => readonly string[]
      buildAgentHookPtyEnv?: () => Record<string, string>
    }
  ) {
    this.store = store
    if (stats) {
      this.stats = stats
      this.agentDetector = new AgentDetector(stats)
    }
    this.getAgentStatusSnapshotFn = deps?.getAgentStatusSnapshot ?? null
    // Why: configure the shared AiVault scan cache from a serve-mode-reachable
    // seam so the aiVault.listSessions RPC includes managed-Codex + WSL sessions
    // even on headless `orca serve` hosts where registerCoreHandlers never runs.
    if (deps?.getAdditionalAiVaultCodexHomePaths) {
      configureAiVaultSessionSources({
        getAdditionalCodexHomePaths: deps.getAdditionalAiVaultCodexHomePaths
      })
    }
    // Why: the daemon adapter is installed via `setLocalPtyProvider()` during
    // attachMainWindowServices, AFTER this service is constructed. Capturing
    // `getLocalPtyProvider()` at construction time would freeze a reference to
    // the pre-daemon `LocalPtyProvider` and miss the routed adapter. Resolve
    // lazily via thunk so teardown always sees the currently-installed
    // provider (design §4.3 wire-up).
    this.getLocalProviderFn = deps?.getLocalProvider ?? null
    this.onPtyStopped = deps?.onPtyStopped ?? null
    this.onTerminalAgentStatus = deps?.onTerminalAgentStatus ?? null
    this.buildAgentHookPtyEnv = deps?.buildAgentHookPtyEnv ?? null
    this.onTerminalSideEffects = deps?.onTerminalSideEffects ?? null
    // Why: the ConPTY spawn mark can land after daemon stream data already
    // created this PTY's emulator; the mark retrofits the DA1 override here
    // (terminal-query-authority.md §ConPTY DA1).
    registerConptyDa1OverrideInstaller((ptyId) => this.ensureNativeWindowsConptyDa1Override(ptyId))
    // Why: a renderer attribute push must reach already-live emulators too —
    // cursor options for DECRQSS/DECRQM parity plus the per-PTY OSC color
    // override reset a theme apply implies (terminal-query-authority.md
    // §View-attribute bridge).
    registerTerminalViewAttributesApplier((attributes) => {
      this.headlessTerminalCommands.applyPushedViewAttributesToAll(attributes)
    })
  }

  getLocalProvider(): IPtyProvider | null {
    return this.getLocalProviderFn ? this.getLocalProviderFn() : null
  }

  getStatsSummary(): StatsSummary | null {
    return this.stats?.getSummary() ?? null
  }

  getMemorySnapshot(): Promise<MemorySnapshot> {
    if (!this.store) {
      throw new Error('runtime_unavailable')
    }
    return collectMemorySnapshot(this.store)
  }

  getUIState(): PersistedUIState {
    if (!this.store?.getUI) {
      throw new Error('runtime_unavailable')
    }
    return this.store.getUI()
  }

  updateUIState(updates: Partial<PersistedUIState>): PersistedUIState {
    if (!this.store?.getUI || !this.store.updateUI) {
      throw new Error('runtime_unavailable')
    }
    this.store.updateUI(updates)
    return this.store.getUI()
  }

  recordFeatureInteraction(id: FeatureInteractionId): PersistedUIState {
    if (!this.store?.recordFeatureInteraction) {
      throw new Error('runtime_unavailable')
    }
    return this.store.recordFeatureInteraction(id)
  }

  getClientSettings(): Pick<
    GlobalSettings,
    | 'defaultTuiAgent'
    | 'disabledTuiAgents'
    | 'agentCmdOverrides'
    | 'agentDefaultArgs'
    | 'agentDefaultEnv'
    | 'agentStatusHooksEnabled'
    | 'defaultTaskSource'
    | 'defaultTaskViewPreset'
    | 'visibleTaskProviders'
    | 'defaultRepoSelection'
    | 'defaultLinearTeamSelection'
    | 'githubProjects'
    | 'experimentalNewWorktreeCardStyle'
    | 'compactWorktreeCards'
    | 'minimaxGroupId'
    | 'minimaxUsageModels'
    | 'prBotAuthorOverrides'
  > {
    if (!this.store?.getSettings) {
      throw new Error('runtime_unavailable')
    }
    const settings = this.store.getSettings()
    return {
      defaultTuiAgent: settings.defaultTuiAgent ?? null,
      disabledTuiAgents: settings.disabledTuiAgents ?? [],
      agentCmdOverrides: settings.agentCmdOverrides ?? {},
      agentDefaultArgs: settings.agentDefaultArgs ?? {},
      agentDefaultEnv: settings.agentDefaultEnv ?? {},
      agentStatusHooksEnabled: settings.agentStatusHooksEnabled !== false,
      defaultTaskSource: settings.defaultTaskSource ?? 'github',
      defaultTaskViewPreset: settings.defaultTaskViewPreset ?? 'issues',
      visibleTaskProviders: settings.visibleTaskProviders ?? [...TASK_PROVIDERS],
      defaultRepoSelection: settings.defaultRepoSelection ?? null,
      defaultLinearTeamSelection: settings.defaultLinearTeamSelection ?? null,
      githubProjects: settings.githubProjects,
      experimentalNewWorktreeCardStyle: settings.experimentalNewWorktreeCardStyle === true,
      compactWorktreeCards: settings.compactWorktreeCards === true,
      minimaxGroupId: settings.minimaxGroupId ?? '',
      minimaxUsageModels: settings.minimaxUsageModels ?? 'general',
      prBotAuthorOverrides: settings.prBotAuthorOverrides ?? []
    }
  }

  updateClientSettings(
    updates: Pick<
      Partial<GlobalSettings>,
      | 'agentStatusHooksEnabled'
      | 'defaultTuiAgent'
      | 'disabledTuiAgents'
      | 'agentDefaultArgs'
      | 'agentDefaultEnv'
      | 'defaultTaskSource'
      | 'defaultTaskViewPreset'
      | 'visibleTaskProviders'
      | 'defaultRepoSelection'
      | 'defaultLinearTeamSelection'
      | 'githubProjects'
      | 'experimentalNewWorktreeCardStyle'
      | 'compactWorktreeCards'
      | 'minimaxGroupId'
      | 'minimaxUsageModels'
      | 'prBotAuthorOverrides'
    >
  ): Pick<
    GlobalSettings,
    | 'defaultTuiAgent'
    | 'disabledTuiAgents'
    | 'agentCmdOverrides'
    | 'agentDefaultArgs'
    | 'agentDefaultEnv'
    | 'agentStatusHooksEnabled'
    | 'defaultTaskSource'
    | 'defaultTaskViewPreset'
    | 'visibleTaskProviders'
    | 'defaultRepoSelection'
    | 'defaultLinearTeamSelection'
    | 'githubProjects'
    | 'experimentalNewWorktreeCardStyle'
    | 'compactWorktreeCards'
    | 'minimaxGroupId'
    | 'minimaxUsageModels'
    | 'prBotAuthorOverrides'
  > {
    if (!this.store?.getSettings || !this.store.updateSettings) {
      throw new Error('runtime_unavailable')
    }
    const before = this.store.getSettings().agentStatusHooksEnabled !== false
    this.store.updateSettings(updates, { notifyListeners: true })
    if (
      typeof updates.agentStatusHooksEnabled === 'boolean' &&
      before !== updates.agentStatusHooksEnabled
    ) {
      applyAgentStatusHooksEnabled(updates.agentStatusHooksEnabled)
    }
    return this.getClientSettings()
  }

  updateClientPRBotAuthorOverride(args: { author: string; isBot: boolean }) {
    if (!this.store?.getSettings || !this.store.updateSettings) {
      throw new Error('runtime_unavailable')
    }
    const current = this.store.getSettings().prBotAuthorOverrides
    this.store.updateSettings(
      { prBotAuthorOverrides: applyPRBotAuthorOverride(current, args.author, args.isBot) },
      { notifyListeners: true }
    )
    return this.getClientSettings()
  }

  private readonly automationCommands = new RuntimeAutomationCommands({
    getStore: () => this.store,
    getAutomationService: () => this.automationService,
    showManagedWorktree: (selector) => this.showManagedWorktree(selector),
    showRepo: (selector) => this.showRepo(selector)
  })

  listAutomations: RuntimeAutomationCommands['listAutomations'] =
    this.automationCommands.listAutomations.bind(this.automationCommands)
  listAutomationRuns: RuntimeAutomationCommands['listAutomationRuns'] =
    this.automationCommands.listAutomationRuns.bind(this.automationCommands)
  showAutomation: RuntimeAutomationCommands['showAutomation'] =
    this.automationCommands.showAutomation.bind(this.automationCommands)
  createAutomation: RuntimeAutomationCommands['createAutomation'] =
    this.automationCommands.createAutomation.bind(this.automationCommands)
  updateAutomation: RuntimeAutomationCommands['updateAutomation'] =
    this.automationCommands.updateAutomation.bind(this.automationCommands)
  deleteAutomation: RuntimeAutomationCommands['deleteAutomation'] =
    this.automationCommands.deleteAutomation.bind(this.automationCommands)
  runAutomationNow: RuntimeAutomationCommands['runAutomationNow'] =
    this.automationCommands.runAutomationNow.bind(this.automationCommands)

  // Why: lazy initialization — the DB path depends on Electron's userData
  // which may not be finalized until after app.ready. Also allows unit tests
  // to inject an in-memory DB without touching the filesystem.
  getOrchestrationDb(): OrchestrationDb {
    if (!this._orchestrationDb) {
      const { app } = require('electron')
      const dbPath = join(app.getPath('userData'), 'orchestration.db')
      this._orchestrationDb = new OrchestrationDb(dbPath)
    }
    return this._orchestrationDb
  }

  setOrchestrationDb(db: OrchestrationDb): void {
    this._orchestrationDb = db
  }

  setAutomationService(service: AutomationService): void {
    this.automationService = service
  }

  /** TASK-036: Inject a WebPushManager so agent-task-complete triggers web push. */
  setPushManager(manager: WebPushManager): void {
    this.pushManager = manager
  }

  getRuntimeId(): string {
    return this.runtimeId
  }

  getStartedAt(): number {
    return this.startedAt
  }

  getStatus(): RuntimeStatus {
    // Why: browser panes need a backend that can create and stream a page. A
    // desktop renderer provides one via <webview>; a headless serve provides one
    // via the offscreen backend. Either way the same browser.screencast.v1 path
    // works, so advertise it when either is present. browser.headless.v1
    // additionally tells clients this host owns browser pages with no renderer,
    // so they must not fall back to a local desktop browser tab.
    const hasRenderer = Boolean(this.getAvailableAuthoritativeWindow())
    const hasOffscreen = !hasRenderer && Boolean(this.offscreenBrowserBackend)
    const canBrowse = hasRenderer || hasOffscreen
    const capabilities: RuntimeCapability[] = RUNTIME_CAPABILITIES.filter(
      (capability) => capability !== 'browser.screencast.v1' || canBrowse
    )
    if (hasOffscreen) {
      capabilities.push(BROWSER_HEADLESS_RUNTIME_CAPABILITY)
    }
    return {
      runtimeId: this.runtimeId,
      rendererGraphEpoch: this.graph.rendererGraphEpoch,
      graphStatus: this.graph.graphStatus,
      authoritativeWindowId: this.graph.authoritativeWindowId,
      liveTabCount: this.graph.tabs.size,
      liveLeafCount: this.graph.leaves.size,
      runtimeProtocolVersion: RUNTIME_PROTOCOL_VERSION,
      minCompatibleRuntimeClientVersion: MIN_COMPATIBLE_RUNTIME_CLIENT_VERSION,
      // Why: headless orca serve cannot create/stream BrowserViews, so clients
      // must not treat browser panes as supported just because runtime RPC is up.
      capabilities,
      hostPlatform: process.platform,
      terminalWindowsShell: this.store?.getSettings?.().terminalWindowsShell ?? null,
      protocolVersion: RUNTIME_PROTOCOL_VERSION,
      minCompatibleMobileVersion: MIN_COMPATIBLE_RUNTIME_CLIENT_VERSION
    }
  }

  // Why: scans the transcript-owning host's disk (correct by construction over
  // RPC — a remote/SSH host scans its own disk). Delegates to the one shared
  // cache so the desktop panel and the mobile screen never double-scan.
  listAiVaultSessions(args?: AiVaultListArgs): Promise<AiVaultListResult> {
    return listAiVaultSessions(args)
  }

  setPtyController(controller: RuntimePtyController | null): void {
    // Why: CLI terminal writes must go through the main-owned PTY registry
    // instead of tunneling back through renderer IPC, or live handles could
    // drift from the process they are supposed to control during reloads.
    this.ptyController = controller
  }

  setNotifier(notifier: RuntimeNotifier | null): void {
    this.notifier = notifier
    // Why: run the one-shot fork-upstream backfill once a renderer is attached,
    // so existing forks self-correct on launch and the result can be broadcast.
    if (notifier && !this.forkBackfillStarted) {
      this.forkBackfillStarted = true
      void this.backfillForkUpstreams()
    }
  }

  onClientEvent(listener: (event: RuntimeClientEvent) => void): () => void {
    this.clientEventListeners.add(listener)
    return () => {
      this.clientEventListeners.delete(listener)
    }
  }

  private emitClientEvent(event: RuntimeClientEvent): void {
    for (const listener of this.clientEventListeners) {
      listener(event)
    }
  }

  private notifyWorktreesChanged(repoId: string): void {
    this.notifier?.worktreesChanged(repoId)
    this.emitClientEvent({ type: 'worktreesChanged', repoId })
  }

  private notifyReposChanged(): void {
    this.notifier?.reposChanged()
    this.emitClientEvent({ type: 'reposChanged' })
  }

  // Why: SSH state changes originate in main's ssh handlers, not in runtime
  // methods, so they need a public entry point onto the client-event stream.
  notifySshStateChanged(targetId: string, state: SshConnectionState): void {
    this.emitClientEvent({ type: 'sshStateChanged', targetId, state })
  }

  // Why: renderer-initiated meta updates intentionally skip the renderer
  // notifier (the renderer already applied them optimistically), but remote
  // clients hold no optimistic copy and need the invalidation event.
  notifyWorktreesChangedForRemoteClients(repoId: string): void {
    this.invalidateResolvedWorktreeCache()
    this.emitClientEvent({ type: 'worktreesChanged', repoId })
  }

  private notifyActivateWorktree(
    repoId: string,
    worktreeId: string,
    setup?: CreateWorktreeResult['setup'],
    startup?: WorktreeStartupLaunch,
    defaultTabs?: CreateWorktreeResult['defaultTabs']
  ): void {
    this.notifier?.activateWorktree(repoId, worktreeId, setup, startup, defaultTabs)
    this.emitClientEvent(
      toRuntimeActivateWorktreeEvent(repoId, worktreeId, setup, startup, defaultTabs)
    )
  }

  setAgentBrowserBridge(bridge: AgentBrowserBridge | null): void {
    this.agentBrowserBridge = bridge
  }

  getAgentBrowserBridge(): AgentBrowserBridge | null {
    return this.agentBrowserBridge
  }

  setOffscreenBrowserBackend(backend: BrowserBackend | null): void {
    this.offscreenBrowserBackend = backend
  }

  getOffscreenBrowserBackend(): BrowserBackend | null {
    return this.offscreenBrowserBackend
  }

  setEmulatorBridge(bridge: EmulatorBridge | null): void {
    this.emulatorBridge = bridge
    setEmulatorBridge(bridge)
  }

  getEmulatorBridge(): EmulatorBridge | null {
    return this.emulatorBridge
  }

  attachWindow(windowId: number): void {
    if (this.graph.authoritativeWindowId === null) {
      this.graph.authoritativeWindowId = windowId
    }
  }

  syncWindowGraph(windowId: number, graph: RuntimeSyncWindowGraph): RuntimeSyncWindowGraphResult {
    if (this.graph.authoritativeWindowId === null) {
      this.graph.authoritativeWindowId = windowId
    }
    if (windowId !== this.graph.authoritativeWindowId) {
      throw new Error('Runtime graph publisher does not match the authoritative window')
    }

    this.graph.tabs = new Map(graph.tabs.map((tab) => [tab.tabId, tab]))
    this.syncMobileSessionTabs(graph.mobileSessionTabs)
    const nextLeaves = new Map<string, RuntimeLeafRecord>()
    const graphSyncedAt = this.nextTitleObservationSequence()

    // Why: renderer reloads can briefly republish the same leaf with no ptyId;
    // keep live CLI handles usable while the UI graph rebuilds.
    const preserveLivePtysDuringReload = this.graph.graphStatus === 'reloading'
    for (const leaf of graph.leaves) {
      const leafKey = this.getLeafKey(leaf.tabId, leaf.leafId)
      const existing = this.graph.leaves.get(leafKey)
      const ptyId =
        preserveLivePtysDuringReload && leaf.ptyId === null && existing?.ptyId
          ? existing.ptyId
          : leaf.ptyId
      const ptyGeneration =
        existing && existing.ptyId !== ptyId
          ? existing.ptyGeneration + 1
          : (existing?.ptyGeneration ?? 0)
      const existingPty = ptyId ? this.graph.ptysById.get(ptyId) : undefined
      const tailSource = existing?.ptyId === ptyId ? existing : existingPty

      nextLeaves.set(leafKey, {
        ...leaf,
        ptyId,
        ptyGeneration,
        connected: ptyId !== null,
        writable: this.graph.graphStatus === 'ready' && ptyId !== null,
        lastOutputAt: tailSource?.lastOutputAt ?? null,
        lastExitCode: tailSource?.lastExitCode ?? null,
        tailBuffer: tailSource?.tailBuffer ?? [],
        tailPartialLine: tailSource?.tailPartialLine ?? '',
        tailPendingAnsi: tailSource?.tailPendingAnsi ?? '',
        tailRedrawCursor: tailSource?.tailRedrawCursor ?? null,
        tailTruncated: tailSource?.tailTruncated ?? false,
        tailLinesTotal: tailSource?.tailLinesTotal ?? 0,
        preview: tailSource?.preview ?? '',
        waitBlockedAt: tailSource?.waitBlockedAt ?? null,
        lastAgentStatus: tailSource?.lastAgentStatus ?? null,
        lastOscTitle: tailSource?.lastOscTitle ?? null,
        lastOscTitleAt: tailSource?.lastOscTitleAt ?? null,
        paneTitleUpdatedAt:
          existing?.ptyId === ptyId && existing.paneTitle === leaf.paneTitle
            ? existing.paneTitleUpdatedAt
            : graphSyncedAt
      })

      if (leaf.ptyId) {
        this.recordPtyWorktree(leaf.ptyId, leaf.worktreeId, {
          connected: true,
          lastOutputAt: existing?.ptyId === leaf.ptyId ? existing.lastOutputAt : null,
          preview: existing?.ptyId === leaf.ptyId ? existing.preview : '',
          tabId: leaf.tabId,
          paneKey: this.makeRuntimePaneKey(leaf)
        })
      }

      if (existing && (existing.ptyId !== ptyId || existing.ptyGeneration !== ptyGeneration)) {
        this.invalidateLeafHandle(leafKey)
      }
    }

    // Why: computed BEFORE preserving stale leaves so preservation can refuse a
    // leaf whose PTY the incoming graph already rebound to a live leaf. Two
    // leaves on one PTY resolve to the same handle (handles are ptyId-keyed) and
    // crash paired clients with a duplicate React key.
    const nextPtyIds = new Set(
      [...nextLeaves.values()].map((leaf) => leaf.ptyId).filter((ptyId): ptyId is string => !!ptyId)
    )
    for (const oldLeafKey of this.graph.leaves.keys()) {
      if (!nextLeaves.has(oldLeafKey)) {
        const oldLeaf = this.graph.leaves.get(oldLeafKey)
        if (
          preserveLivePtysDuringReload &&
          oldLeaf?.ptyId &&
          this.graph.handleByPtyId.has(oldLeaf.ptyId) &&
          !nextPtyIds.has(oldLeaf.ptyId)
        ) {
          // Why: a CLI-created agent keeps using its exported handle even if
          // the reloaded renderer has not rebound the pane yet.
          nextLeaves.set(oldLeafKey, oldLeaf)
          nextPtyIds.add(oldLeaf.ptyId)
        } else if (oldLeaf?.ptyId && nextPtyIds.has(oldLeaf.ptyId)) {
          // Why: the incoming graph already rebound this PTY to a live leaf (e.g.
          // a woken agent re-keyed to a new leaf during renderer reload). Keeping
          // the old leaf too would put two leaves on ONE PTY, which emit the same
          // terminal handle and crash paired clients. Drop the stale leaf; if its
          // handle is the shared ptyId-keyed one it belongs to the live leaf now,
          // so release only this dead leaf key's alias. A leaf-unique handle has
          // no next owner — invalidate it so in-flight CLI waiters fail fast
          // instead of hanging on a dead leaf.
          const oldHandle = this.graph.handleByLeafKey.get(oldLeafKey)
          if (
            oldHandle !== undefined &&
            oldHandle === this.graph.handleByPtyId.get(oldLeaf.ptyId)
          ) {
            this.graph.handleByLeafKey.delete(oldLeafKey)
          } else {
            this.invalidateLeafHandle(oldLeafKey)
          }
        } else {
          this.invalidateLeafHandle(oldLeafKey)
        }
      }
    }

    for (const [ptyId, leaf] of this.graph.detachedPreAllocatedLeaves) {
      if (nextPtyIds.has(ptyId) || !this.graph.handleByPtyId.has(ptyId)) {
        this.graph.detachedPreAllocatedLeaves.delete(ptyId)
        continue
      }
      nextLeaves.set(this.getLeafKey(leaf.tabId, leaf.leafId), leaf)
      nextPtyIds.add(ptyId)
    }

    this.graph.leaves = nextLeaves
    this.rebuildLeafPtyIndex()
    this.notifyMobileSessionTabSnapshots()
    this.graph.graphStatus = 'ready'
    this.refreshWritableFlags()
    for (const leaf of this.graph.leaves.values()) {
      this.adoptPreAllocatedHandle(leaf)
    }

    // Why: createTerminal waits for the renderer's graph sync to populate the
    // new leaf so it can return a handle. Drain callbacks after leaves update.
    for (const cb of [...this.graph.graphSyncCallbacks]) {
      cb()
    }

    const agentOrchestrationByPaneKey = this.buildAgentOrchestrationByPaneKey()
    return {
      ...this.getStatus(),
      ...(agentOrchestrationByPaneKey ? { agentOrchestrationByPaneKey } : {})
    }
  }

  async listMobileSessionTabs(worktreeSelector: string): Promise<RuntimeMobileSessionTabsResult> {
    const explicitWorktreeId = this.getValidatedExplicitWorktreeIdSelector(worktreeSelector)
    if (explicitWorktreeId) {
      this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(explicitWorktreeId)
      await this.refreshMobileSessionPtyRecords()
      return this.getMobileSessionTabsForWorktree(explicitWorktreeId)
    }
    const worktree = await this.resolveWorktreeSelector(worktreeSelector)
    this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(worktree.id)
    await this.refreshMobileSessionPtyRecords()
    return this.getMobileSessionTabsForWorktree(worktree.id)
  }

  private readonly mobileSessionTabsCommands = new RuntimeMobileSessionTabsCommands({
    getStore: () => this.store,
    requireStore: () => this.requireStore(),
    resolveWorktreeSelector: (selector) => this.resolveWorktreeSelector(selector),
    getNotifier: () => this.notifier,
    getPtyController: () => this.ptyController,
    getGraph: () => this.graph,
    getAgentBrowserBridge: () => this.agentBrowserBridge,
    getOffscreenBrowserBackend: () => this.offscreenBrowserBackend,
    getMobileSessionTabsByWorktree: () => this.mobileSessionTabsByWorktree,
    getMobileSessionTabListeners: () => this.mobileSessionTabListeners,
    getMobileSessionTabsNotifyCoalescer: () => this.mobileSessionTabsNotifyCoalescer,
    listResolvedWorktrees: () => this.listResolvedWorktrees(),
    notifyMobileSessionTabsChanged: (worktreeId) => this.notifyMobileSessionTabsChanged(worktreeId),
    createHeadlessMobileSessionTerminal: (worktreeId, activate, afterTabId, opts) =>
      this.createHeadlessMobileSessionTerminal(worktreeId, activate, afterTabId, opts),
    findPtyForMobileTerminalTab: (worktreeId, tab, options) =>
      this.findPtyForMobileTerminalTab(worktreeId, tab, options),
    resolveMobileSessionTerminalCommand: (workspace, opts) =>
      this.resolveMobileSessionTerminalCommand(workspace, opts),
    getMobileSessionTabsForWorktree: (worktreeId) =>
      this.getMobileSessionTabsForWorktree(worktreeId),
    getLiveBrowserTabsByPageId: (worktreeId) => this.getLiveBrowserTabsByPageId(worktreeId),
    toMobileSessionTabsResult: (snapshot) => this.toMobileSessionTabsResult(snapshot),
    isHeadlessMobileSessionPublication: (publicationEpoch) =>
      this.isHeadlessMobileSessionPublication(publicationEpoch),
    getPersistedSshPtyIdForMobileTerminalTab: (tab) =>
      this.getPersistedSshPtyIdForMobileTerminalTab(tab),
    refreshPtyWorktreeRecordsFromController: (resolvedWorktrees, targetWorktreeId) =>
      this.refreshPtyWorktreeRecordsFromController(resolvedWorktrees, targetWorktreeId),
    resolveMobileMarkdownWorktreeId: (worktreeSelector, tabId) =>
      this.resolveMobileMarkdownWorktreeId(worktreeSelector, tabId),
    getValidatedExplicitWorktreeIdSelector: (selector) =>
      this.getValidatedExplicitWorktreeIdSelector(selector),
    resolveFolderWorkspaceLaunchScope: (selector) =>
      this.resolveFolderWorkspaceLaunchScope(selector),
    resolveTerminalWorkspaceLaunchScope: (selector) =>
      this.resolveTerminalWorkspaceLaunchScope(selector),
    folderWorkspaceToResolvedWorktree: (folderWorkspace) =>
      this.folderWorkspaceToResolvedWorktree(folderWorkspace),
    getAvailableAuthoritativeWindow: () => this.getAvailableAuthoritativeWindow()
  })

  listAllMobileSessionTabs: RuntimeMobileSessionTabsCommands['listAllMobileSessionTabs'] =
    this.mobileSessionTabsCommands.listAllMobileSessionTabs.bind(this.mobileSessionTabsCommands)
  activateMobileSessionTab: RuntimeMobileSessionTabsCommands['activateMobileSessionTab'] =
    this.mobileSessionTabsCommands.activateMobileSessionTab.bind(this.mobileSessionTabsCommands)
  closeMobileSessionTab: RuntimeMobileSessionTabsCommands['closeMobileSessionTab'] =
    this.mobileSessionTabsCommands.closeMobileSessionTab.bind(this.mobileSessionTabsCommands)
  moveMobileSessionTab: RuntimeMobileSessionTabsCommands['moveMobileSessionTab'] =
    this.mobileSessionTabsCommands.moveMobileSessionTab.bind(this.mobileSessionTabsCommands)
  updateMobileSessionPaneLayout: RuntimeMobileSessionTabsCommands['updateMobileSessionPaneLayout'] =
    this.mobileSessionTabsCommands.updateMobileSessionPaneLayout.bind(
      this.mobileSessionTabsCommands
    )
  setMobileSessionTabProps: RuntimeMobileSessionTabsCommands['setMobileSessionTabProps'] =
    this.mobileSessionTabsCommands.setMobileSessionTabProps.bind(this.mobileSessionTabsCommands)
  readMobileMarkdownTab: RuntimeMobileSessionTabsCommands['readMobileMarkdownTab'] =
    this.mobileSessionTabsCommands.readMobileMarkdownTab.bind(this.mobileSessionTabsCommands)
  saveMobileMarkdownTab: RuntimeMobileSessionTabsCommands['saveMobileMarkdownTab'] =
    this.mobileSessionTabsCommands.saveMobileMarkdownTab.bind(this.mobileSessionTabsCommands)
  hasServeOwnedPtyBinding: RuntimeMobileSessionTabsCommands['hasServeOwnedPtyBinding'] =
    this.mobileSessionTabsCommands.hasServeOwnedPtyBinding.bind(this.mobileSessionTabsCommands)
  getMobileSessionSnapshotTabIdentityKeys: RuntimeMobileSessionTabsCommands['getMobileSessionSnapshotTabIdentityKeys'] =
    this.mobileSessionTabsCommands.getMobileSessionSnapshotTabIdentityKeys.bind(
      this.mobileSessionTabsCommands
    )
  publishPtyBackedMobileSessionTerminal: RuntimeMobileSessionTabsCommands['publishPtyBackedMobileSessionTerminal'] =
    this.mobileSessionTabsCommands.publishPtyBackedMobileSessionTerminal.bind(
      this.mobileSessionTabsCommands
    )
  touchMobileSessionSnapshotsForPty: RuntimeMobileSessionTabsCommands['touchMobileSessionSnapshotsForPty'] =
    this.mobileSessionTabsCommands.touchMobileSessionSnapshotsForPty.bind(
      this.mobileSessionTabsCommands
    )
  persistHeadlessTerminalSplit: RuntimeMobileSessionTabsCommands['persistHeadlessTerminalSplit'] =
    this.mobileSessionTabsCommands.persistHeadlessTerminalSplit.bind(this.mobileSessionTabsCommands)
  markHeadlessBrowserSessionTabActive: RuntimeMobileSessionTabsCommands['markHeadlessBrowserSessionTabActive'] =
    this.mobileSessionTabsCommands.markHeadlessBrowserSessionTabActive.bind(
      this.mobileSessionTabsCommands
    )
  persistHeadlessTerminalTitle: RuntimeMobileSessionTabsCommands['persistHeadlessTerminalTitle'] =
    this.mobileSessionTabsCommands.persistHeadlessTerminalTitle.bind(this.mobileSessionTabsCommands)
  resolveRuntimeGitTarget: RuntimeMobileSessionTabsCommands['resolveRuntimeGitTarget'] =
    this.mobileSessionTabsCommands.resolveRuntimeGitTarget.bind(this.mobileSessionTabsCommands)
  resolveRuntimeFileTarget: RuntimeMobileSessionTabsCommands['resolveRuntimeFileTarget'] =
    this.mobileSessionTabsCommands.resolveRuntimeFileTarget.bind(this.mobileSessionTabsCommands)
  hydrateHeadlessMobileSessionTabsFromWorkspaceSession: RuntimeMobileSessionTabsCommands['hydrateHeadlessMobileSessionTabsFromWorkspaceSession'] =
    this.mobileSessionTabsCommands.hydrateHeadlessMobileSessionTabsFromWorkspaceSession.bind(
      this.mobileSessionTabsCommands
    )
  refreshMobileSessionPtyRecords: RuntimeMobileSessionTabsCommands['refreshMobileSessionPtyRecords'] =
    this.mobileSessionTabsCommands.refreshMobileSessionPtyRecords.bind(
      this.mobileSessionTabsCommands
    )
  buildMaterializedHeadlessParentLayout: RuntimeMobileSessionTabsCommands['buildMaterializedHeadlessParentLayout'] =
    this.mobileSessionTabsCommands.buildMaterializedHeadlessParentLayout.bind(
      this.mobileSessionTabsCommands
    )
  getHeadlessMobileSessionGroupId: RuntimeMobileSessionTabsCommands['getHeadlessMobileSessionGroupId'] =
    this.mobileSessionTabsCommands.getHeadlessMobileSessionGroupId.bind(
      this.mobileSessionTabsCommands
    )
  buildHeadlessMobileSessionTabGroups: RuntimeMobileSessionTabsCommands['buildHeadlessMobileSessionTabGroups'] =
    this.mobileSessionTabsCommands.buildHeadlessMobileSessionTabGroups.bind(
      this.mobileSessionTabsCommands
    )
  mergeMobileSessionSnapshotTabs: RuntimeMobileSessionTabsCommands['mergeMobileSessionSnapshotTabs'] =
    this.mobileSessionTabsCommands.mergeMobileSessionSnapshotTabs.bind(
      this.mobileSessionTabsCommands
    )
  mergeMobileSessionTabGroups: RuntimeMobileSessionTabsCommands['mergeMobileSessionTabGroups'] =
    this.mobileSessionTabsCommands.mergeMobileSessionTabGroups.bind(this.mobileSessionTabsCommands)

  private readonly fileCommands = new RuntimeFileCommands({
    getRuntimeId: () => this.runtimeId,
    requireStore: () => this.requireStore(),
    resolveWorktreeSelector: (selector) => this.resolveWorktreeSelector(selector),
    resolveRuntimeFileTarget: (selector) => this.resolveRuntimeFileTarget(selector),
    resolveTerminalCwd: (terminalHandle) => this.resolveTerminalCwd(terminalHandle),
    resolveTerminalContext: (terminalHandle) => this.resolveTerminalContext(terminalHandle),
    resolveTerminalFileUriHostname: (terminalHandle) =>
      this.resolveTerminalFileUriHostname(terminalHandle),
    hasRecentTerminalOutputPath: (terminalHandle, pathText, absolutePath) =>
      this.hasRecentTerminalOutputPath(terminalHandle, pathText, absolutePath),
    resolveRuntimeGitTarget: (selector) => this.resolveRuntimeGitTarget(selector),
    openFile: (worktreeId, filePath, relativePath, runtimeEnvironmentId) => {
      if (!this.notifier?.openFile) {
        throw new Error('renderer_unavailable')
      }
      this.notifier.openFile(worktreeId, filePath, relativePath, runtimeEnvironmentId)
    },
    openDiff: (worktreeId, filePath, relativePath, staged, runtimeEnvironmentId) => {
      if (!this.notifier?.openDiff) {
        throw new Error('renderer_unavailable')
      }
      this.notifier.openDiff(worktreeId, filePath, relativePath, staged, runtimeEnvironmentId)
    }
  })

  listMobileFiles: RuntimeFileCommands['listMobileFiles'] = this.fileCommands.listMobileFiles.bind(
    this.fileCommands
  )
  openMobileFile: RuntimeFileCommands['openMobileFile'] = this.fileCommands.openMobileFile.bind(
    this.fileCommands
  )
  openMobileDiff: RuntimeFileCommands['openMobileDiff'] = this.fileCommands.openMobileDiff.bind(
    this.fileCommands
  )
  readMobileFile: RuntimeFileCommands['readMobileFile'] = this.fileCommands.readMobileFile.bind(
    this.fileCommands
  )
  resolveTerminalPath: RuntimeFileCommands['resolveTerminalPath'] =
    this.fileCommands.resolveTerminalPath.bind(this.fileCommands)
  readTerminalArtifactFile: RuntimeFileCommands['readTerminalArtifactFile'] =
    this.fileCommands.readTerminalArtifactFile.bind(this.fileCommands)
  readTerminalArtifactPreview: RuntimeFileCommands['readTerminalArtifactPreview'] =
    this.fileCommands.readTerminalArtifactPreview.bind(this.fileCommands)
  writeTerminalArtifactFile: RuntimeFileCommands['writeTerminalArtifactFile'] =
    this.fileCommands.writeTerminalArtifactFile.bind(this.fileCommands)
  revokeTerminalFileGrantsForClient: RuntimeFileCommands['revokeTerminalFileGrantsForClient'] =
    this.fileCommands.revokeTerminalFileGrantsForClient.bind(this.fileCommands)
  readFileExplorerDir: RuntimeFileCommands['readFileExplorerDir'] =
    this.fileCommands.readFileExplorerDir.bind(this.fileCommands)
  watchFileExplorer: RuntimeFileCommands['watchFileExplorer'] =
    this.fileCommands.watchFileExplorer.bind(this.fileCommands)
  readFileExplorerPreview: RuntimeFileCommands['readFileExplorerPreview'] =
    this.fileCommands.readFileExplorerPreview.bind(this.fileCommands)
  readFileExplorerChunk: RuntimeFileCommands['readFileExplorerChunk'] =
    this.fileCommands.readFileExplorerChunk.bind(this.fileCommands)
  writeFileExplorerFile: RuntimeFileCommands['writeFileExplorerFile'] =
    this.fileCommands.writeFileExplorerFile.bind(this.fileCommands)
  writeFileExplorerFileBase64: RuntimeFileCommands['writeFileExplorerFileBase64'] =
    this.fileCommands.writeFileExplorerFileBase64.bind(this.fileCommands)
  writeFileExplorerFileBase64Chunk: RuntimeFileCommands['writeFileExplorerFileBase64Chunk'] =
    this.fileCommands.writeFileExplorerFileBase64Chunk.bind(this.fileCommands)
  createFileExplorerFile: RuntimeFileCommands['createFileExplorerFile'] =
    this.fileCommands.createFileExplorerFile.bind(this.fileCommands)
  createFileExplorerDir: RuntimeFileCommands['createFileExplorerDir'] =
    this.fileCommands.createFileExplorerDir.bind(this.fileCommands)
  createFileExplorerDirNoClobber: RuntimeFileCommands['createFileExplorerDirNoClobber'] =
    this.fileCommands.createFileExplorerDirNoClobber.bind(this.fileCommands)
  commitFileExplorerUpload: RuntimeFileCommands['commitFileExplorerUpload'] =
    this.fileCommands.commitFileExplorerUpload.bind(this.fileCommands)
  renameFileExplorerPath: RuntimeFileCommands['renameFileExplorerPath'] =
    this.fileCommands.renameFileExplorerPath.bind(this.fileCommands)
  copyFileExplorerPath: RuntimeFileCommands['copyFileExplorerPath'] =
    this.fileCommands.copyFileExplorerPath.bind(this.fileCommands)
  deleteFileExplorerPath: RuntimeFileCommands['deleteFileExplorerPath'] =
    this.fileCommands.deleteFileExplorerPath.bind(this.fileCommands)
  searchRuntimeFiles: RuntimeFileCommands['searchRuntimeFiles'] =
    this.fileCommands.searchRuntimeFiles.bind(this.fileCommands)
  listRuntimeFiles: RuntimeFileCommands['listRuntimeFiles'] =
    this.fileCommands.listRuntimeFiles.bind(this.fileCommands)
  listRuntimeMarkdownDocuments: RuntimeFileCommands['listRuntimeMarkdownDocuments'] =
    this.fileCommands.listRuntimeMarkdownDocuments.bind(this.fileCommands)
  statRuntimeFile: RuntimeFileCommands['statRuntimeFile'] = this.fileCommands.statRuntimeFile.bind(
    this.fileCommands
  )

  private readonly gitCommands = new RuntimeGitCommands({
    resolveRuntimeGitTarget: (selector) => this.resolveRuntimeGitTarget(selector),
    getRuntimeSettings: () => this.requireStore().getSettings() as GlobalSettings,
    getCommitMessageAgentEnvironment: () => this.commitMessageAgentEnv ?? undefined
  })

  getRuntimeGitStatus: RuntimeGitCommands['getRuntimeGitStatus'] =
    this.gitCommands.getRuntimeGitStatus.bind(this.gitCommands)
  getRuntimeGitSubmoduleStatus: RuntimeGitCommands['getRuntimeGitSubmoduleStatus'] =
    this.gitCommands.getRuntimeGitSubmoduleStatus.bind(this.gitCommands)
  checkRuntimeGitIgnoredPaths: RuntimeGitCommands['checkRuntimeGitIgnoredPaths'] =
    this.gitCommands.checkRuntimeGitIgnoredPaths.bind(this.gitCommands)
  getRuntimeGitHistory: RuntimeGitCommands['getRuntimeGitHistory'] =
    this.gitCommands.getRuntimeGitHistory.bind(this.gitCommands)
  getRuntimeGitConflictOperation: RuntimeGitCommands['getRuntimeGitConflictOperation'] =
    this.gitCommands.getRuntimeGitConflictOperation.bind(this.gitCommands)
  abortRuntimeGitMerge: RuntimeGitCommands['abortRuntimeGitMerge'] =
    this.gitCommands.abortRuntimeGitMerge.bind(this.gitCommands)
  abortRuntimeGitRebase: RuntimeGitCommands['abortRuntimeGitRebase'] =
    this.gitCommands.abortRuntimeGitRebase.bind(this.gitCommands)
  checkoutRuntimeGitBranch: RuntimeGitCommands['checkoutRuntimeGitBranch'] =
    this.gitCommands.checkoutRuntimeGitBranch.bind(this.gitCommands)
  listRuntimeGitLocalBranches: RuntimeGitCommands['listRuntimeGitLocalBranches'] =
    this.gitCommands.listRuntimeGitLocalBranches.bind(this.gitCommands)
  getRuntimeGitDiff: RuntimeGitCommands['getRuntimeGitDiff'] =
    this.gitCommands.getRuntimeGitDiff.bind(this.gitCommands)
  getRuntimeGitBranchCompare: RuntimeGitCommands['getRuntimeGitBranchCompare'] =
    this.gitCommands.getRuntimeGitBranchCompare.bind(this.gitCommands)
  getRuntimeGitCommitCompare: RuntimeGitCommands['getRuntimeGitCommitCompare'] =
    this.gitCommands.getRuntimeGitCommitCompare.bind(this.gitCommands)
  getRuntimeGitUpstreamStatus: RuntimeGitCommands['getRuntimeGitUpstreamStatus'] =
    this.gitCommands.getRuntimeGitUpstreamStatus.bind(this.gitCommands)
  fetchRuntimeGit: RuntimeGitCommands['fetchRuntimeGit'] = this.gitCommands.fetchRuntimeGit.bind(
    this.gitCommands
  )
  syncRuntimeGitForkDefaultBranch: RuntimeGitCommands['syncRuntimeGitForkDefaultBranch'] =
    this.gitCommands.syncRuntimeGitForkDefaultBranch.bind(this.gitCommands)
  pullRuntimeGit: RuntimeGitCommands['pullRuntimeGit'] = this.gitCommands.pullRuntimeGit.bind(
    this.gitCommands
  )
  fastForwardRuntimeGit: RuntimeGitCommands['fastForwardRuntimeGit'] =
    this.gitCommands.fastForwardRuntimeGit.bind(this.gitCommands)
  rebaseRuntimeGitFromBase: RuntimeGitCommands['rebaseRuntimeGitFromBase'] =
    this.gitCommands.rebaseRuntimeGitFromBase.bind(this.gitCommands)
  pushRuntimeGit: RuntimeGitCommands['pushRuntimeGit'] = this.gitCommands.pushRuntimeGit.bind(
    this.gitCommands
  )
  getRuntimeGitBranchDiff: RuntimeGitCommands['getRuntimeGitBranchDiff'] =
    this.gitCommands.getRuntimeGitBranchDiff.bind(this.gitCommands)
  getRuntimeGitCommitDiff: RuntimeGitCommands['getRuntimeGitCommitDiff'] =
    this.gitCommands.getRuntimeGitCommitDiff.bind(this.gitCommands)
  commitRuntimeGit: RuntimeGitCommands['commitRuntimeGit'] = this.gitCommands.commitRuntimeGit.bind(
    this.gitCommands
  )
  generateRuntimeCommitMessage: RuntimeGitCommands['generateRuntimeCommitMessage'] =
    this.gitCommands.generateRuntimeCommitMessage.bind(this.gitCommands)
  discoverRuntimeCommitMessageModels: RuntimeGitCommands['discoverRuntimeCommitMessageModels'] =
    this.gitCommands.discoverRuntimeCommitMessageModels.bind(this.gitCommands)
  cancelRuntimeGenerateCommitMessage: RuntimeGitCommands['cancelRuntimeGenerateCommitMessage'] =
    this.gitCommands.cancelRuntimeGenerateCommitMessage.bind(this.gitCommands)
  generateRuntimePullRequestFields: RuntimeGitCommands['generateRuntimePullRequestFields'] =
    this.gitCommands.generateRuntimePullRequestFields.bind(this.gitCommands)
  cancelRuntimeGeneratePullRequestFields: RuntimeGitCommands['cancelRuntimeGeneratePullRequestFields'] =
    this.gitCommands.cancelRuntimeGeneratePullRequestFields.bind(this.gitCommands)
  stageRuntimeGitPath: RuntimeGitCommands['stageRuntimeGitPath'] =
    this.gitCommands.stageRuntimeGitPath.bind(this.gitCommands)
  unstageRuntimeGitPath: RuntimeGitCommands['unstageRuntimeGitPath'] =
    this.gitCommands.unstageRuntimeGitPath.bind(this.gitCommands)
  bulkStageRuntimeGitPaths: RuntimeGitCommands['bulkStageRuntimeGitPaths'] =
    this.gitCommands.bulkStageRuntimeGitPaths.bind(this.gitCommands)
  bulkUnstageRuntimeGitPaths: RuntimeGitCommands['bulkUnstageRuntimeGitPaths'] =
    this.gitCommands.bulkUnstageRuntimeGitPaths.bind(this.gitCommands)
  bulkDiscardRuntimeGitPaths: RuntimeGitCommands['bulkDiscardRuntimeGitPaths'] =
    this.gitCommands.bulkDiscardRuntimeGitPaths.bind(this.gitCommands)
  discardRuntimeGitPath: RuntimeGitCommands['discardRuntimeGitPath'] =
    this.gitCommands.discardRuntimeGitPath.bind(this.gitCommands)
  getRuntimeGitRemoteFileUrl: RuntimeGitCommands['getRuntimeGitRemoteFileUrl'] =
    this.gitCommands.getRuntimeGitRemoteFileUrl.bind(this.gitCommands)
  getRuntimeGitRemoteCommitUrl: RuntimeGitCommands['getRuntimeGitRemoteCommitUrl'] =
    this.gitCommands.getRuntimeGitRemoteCommitUrl.bind(this.gitCommands)

  onMobileSessionTabsChanged: RuntimeMobileSessionTabsCommands['onMobileSessionTabsChanged'] =
    this.mobileSessionTabsCommands.onMobileSessionTabsChanged.bind(this.mobileSessionTabsCommands)

  preAllocateHandleForPty(ptyId: string): string {
    const existing = this.graph.handleByPtyId.get(ptyId)
    if (existing) {
      return existing
    }
    const handle = this.createPreAllocatedTerminalHandle()
    this.graph.handleByPtyId.set(ptyId, handle)
    return handle
  }

  createPreAllocatedTerminalHandle(): string {
    return `term_${randomUUID()}`
  }

  registerPreAllocatedHandleForPty(ptyId: string, handle: string): void {
    this.graph.handleByPtyId.set(ptyId, handle)
    for (const leaf of this.getLeavesForPty(ptyId)) {
      this.adoptPreAllocatedHandle(leaf)
    }
  }

  private adoptControllerTerminalHandle(ptyId: string, handle: string | undefined): void {
    const trimmed = handle?.trim()
    if (!trimmed || !trimmed.startsWith('term_')) {
      return
    }
    if (this.isTerminalHandleAdoptionBlocked(ptyId, trimmed)) {
      return
    }
    // Why: after an app/runtime restart, the live PTY child still has its
    // original ORCA_TERMINAL_HANDLE, but the runtime's in-memory map is gone.
    this.registerPreAllocatedHandleForPty(ptyId, trimmed)
  }

  // Why: adoption is best-effort restart recovery and must be first-wins.
  // Re-keying a pty that already has a handle this session would strand
  // waiters registered under the old handle, and provider-reported values
  // are not trusted to be collision-free — a handle bound to a different
  // pty must never be stolen by a later report.
  private isTerminalHandleAdoptionBlocked(ptyId: string, handle: string): boolean {
    if (this.graph.handleByPtyId.get(ptyId) ?? this.findHandleForPtyRecord(ptyId)) {
      return true
    }
    for (const leaf of this.getLeavesForPty(ptyId)) {
      const issued = this.graph.handleByLeafKey.get(this.getLeafKey(leaf.tabId, leaf.leafId))
      if (issued && issued !== handle) {
        return true
      }
    }
    const existingRecord = this.graph.handles.get(handle)
    if (existingRecord && existingRecord.ptyId !== ptyId) {
      return true
    }
    for (const [otherPtyId, otherHandle] of this.graph.handleByPtyId) {
      if (otherHandle === handle && otherPtyId !== ptyId) {
        return true
      }
    }
    return false
  }

  onPtySpawned(ptyId: string): void {
    const pty = this.getOrCreatePtyWorktreeRecord(ptyId)
    if (pty) {
      pty.connected = true
      pty.disconnectedAt = null
    }
    for (const leaf of this.getLeavesForPty(ptyId)) {
      leaf.connected = true
      leaf.writable = this.graph.graphStatus === 'ready'
      this.adoptPreAllocatedHandle(leaf)
    }
  }

  registerPty(
    ptyId: string,
    worktreeId: string,
    connectionId: string | null = null,
    binding?: { tabId: string; leafId: string }
  ): void {
    // Why: record the renderer pane identity at spawn time so a stalled graph
    // sync can't hide that a live PTY already backs a pending mobile create.
    const paneKey =
      binding && isValidTerminalTabId(binding.tabId) && isTerminalLeafId(binding.leafId)
        ? makePaneKey(binding.tabId, binding.leafId)
        : null
    this.recordPtyWorktree(ptyId, worktreeId, {
      connected: true,
      connectionId,
      ...(binding && paneKey ? { tabId: binding.tabId, paneKey } : {})
    })
    // Why: the renderer's own PTY spawn is the reliable signal that the pending
    // mobile create's tab is live; publish its surface main-side (#7587).
    if (binding && paneKey) {
      this.ensurePtyBackedMobileSurfaceForRendererTab(worktreeId, binding.tabId)
    }
  }

  /** Record the spawn launch command so the per-PTY Command Code detector can
   *  arm from it (renderer startupCommand parity). Best-effort: a chunk that
   *  beats this call falls back to the detector's banner arming. */
  noteTerminalSpawnCommand(ptyId: string, command: string | null | undefined): void {
    const trimmed = typeof command === 'string' ? command.trim() : ''
    if (trimmed.length > 0) {
      this.ptyTranscripts.terminalSpawnCommandsByPtyId.set(ptyId, trimmed)
    }
  }

  /**
   * Handles incoming data from a PTY process, running agent detection,
   * updating terminal tail buffers, and triggering foreground agent refreshes.
   */
  onPtyData(ptyId: string, data: string, at: number, sequenceChars = data.length): number {
    const outputSequence =
      (this.ptyTranscripts.ptyOutputSequenceById.get(ptyId) ?? 0) + sequenceChars
    this.ptyTranscripts.ptyOutputSequenceById.set(ptyId, outputSequence)
    const osc7Metadata = this.recordOsc7MetadataForPty(ptyId, data)
    const cwd = osc7Metadata.cwd
    const cwdChanged = osc7Metadata.cwdChanged
    const agentStatusChunk = this.processAgentStatusOscForPty(ptyId, data)
    this.recordRecentPtyOutputForPathProvenance(ptyId, data)
    // Agent detection runs on raw data before leaf processing, since the
    // tail buffer logic normalizes away the OSC sequences we need.
    this.agentDetector?.onData(ptyId, data, at)
    // Why: watch terminal output for advertised dev-server URLs (e.g. Vite's
    // `Network: https://local.example.com:3001/`) so the workspace ports
    // panel can surface them in place of the kernel bind address.
    advertisedUrlWatcher.ingest(ptyId, data, at)
    serveSimStateWatcher.ingestPtyOutput(ptyId, data)
    // Why: reply ownership is captured per chunk, here at ingestion — the
    // same module state and tick as the hidden-gate drop sites — and rides
    // the writeChain link. A mark/setting/subscriber flip before the queued
    // emulator write runs must not change who answers (terminal-query-
    // authority.md invariant 1).
    const forwardQueryReplies = this.shouldAnswerQueriesForLiveChunk(ptyId)
    // Ordering invariant (DO NOT REORDER): maybeHydrateHeadlessFromRenderer
    // MUST run before trackHeadlessTerminalData so the eager-state pattern
    // (set headlessTerminals + writeChain head = seedPromise) is in place
    // before the live byte's chain link is queued. Without this ordering,
    // trackHeadlessTerminalData would lazy-create a fresh state at PTY dims
    // that the later seed-resolve would overwrite, dropping the live byte.
    // See docs/mobile-prefer-renderer-scrollback.md.
    this.maybeHydrateHeadlessFromRenderer(ptyId)
    // Our structure wins: OSC title/agent-status extraction runs through the
    // shared per-PTY title tracker below (getOrCreatePtyTitleTrackerEntry →
    // applyTrackedPtyTitle) in byte order, superseding main's inline
    // extractLastOscTitleForPty block (#7880/#7852 title/status semantics are
    // preserved via the tracker + detectAgentStatusFromTitle path).
    this.trackHeadlessTerminalData(ptyId, data, outputSequence, forwardQueryReplies)

    const pty = this.getOrCreatePtyWorktreeRecord(ptyId)
    const ptyTailBefore = pty
      ? {
          lines: pty.tailBuffer,
          partialLine: pty.tailPartialLine,
          pendingAnsi: pty.tailPendingAnsi,
          redrawCursor: pty.tailRedrawCursor,
          truncated: pty.tailTruncated,
          linesTotal: pty.tailLinesTotal
        }
      : null
    let ptyTailAfter: ReturnType<typeof appendNormalizedToTailBuffer> | null = null
    if (pty) {
      pty.connected = true
      pty.disconnectedAt = null
      pty.lastOutputAt = at
      const normalized = normalizeTerminalChunk(data, pty.tailPendingAnsi)
      pty.tailPendingAnsi = normalized.pendingAnsi
      const nextTail = appendNormalizedToTailBuffer(
        pty.tailBuffer,
        pty.tailPartialLine,
        normalized.text,
        pty.tailRedrawCursor
      )
      ptyTailAfter = nextTail
      pty.tailBuffer = nextTail.lines
      pty.tailPartialLine = nextTail.partialLine
      pty.tailRedrawCursor = nextTail.redrawCursor
      pty.tailTruncated = pty.tailTruncated || nextTail.truncated
      pty.tailLinesTotal += nextTail.newCompleteLines
      pty.preview = buildPreview(pty.tailBuffer, pty.tailPartialLine)
      this.scheduleWaitBlockedCheck(ptyId, normalized.text, at)
    }

    for (const leaf of this.getLeavesForPty(ptyId)) {
      this.recordPtyWorktree(ptyId, leaf.worktreeId, {
        connected: true,
        lastOutputAt: pty?.lastOutputAt ?? at,
        preview: pty?.preview ?? leaf.preview,
        tabId: leaf.tabId,
        paneKey: this.makeRuntimePaneKey(leaf)
      })
      leaf.connected = true
      leaf.writable = this.graph.graphStatus === 'ready'
      leaf.lastOutputAt = at
      if (
        pty &&
        ptyTailBefore &&
        ptyTailAfter &&
        tailStateMatches(
          leaf.tailBuffer,
          leaf.tailPartialLine,
          leaf.tailPendingAnsi,
          leaf.tailRedrawCursor,
          leaf.tailTruncated,
          leaf.tailLinesTotal,
          ptyTailBefore
        )
      ) {
        // Why: the leaf and PTY record usually mirror the same terminal. Reuse
        // the PTY tail update instead of splitting large output twice.
        leaf.tailBuffer = pty.tailBuffer
        leaf.tailPartialLine = pty.tailPartialLine
        leaf.tailPendingAnsi = pty.tailPendingAnsi
        leaf.tailRedrawCursor = pty.tailRedrawCursor
        leaf.tailTruncated = pty.tailTruncated
        leaf.tailLinesTotal = pty.tailLinesTotal
        leaf.preview = pty.preview
        leaf.waitBlockedAt = pty.waitBlockedAt
        // Why undefined on this branch: the PTY record's wait scan is throttled
        // (scheduleWaitBlockedCheck), so pty.tailWaitState is never populated;
        // copying it here intentionally invalidates the leaf cache and the
        // mismatch branch below recomputes an exact state on its next chunk.
        leaf.tailWaitState = pty.tailWaitState
      } else {
        const normalized = normalizeTerminalChunk(data, leaf.tailPendingAnsi)
        leaf.tailPendingAnsi = normalized.pendingAnsi
        const previousWaitState =
          leaf.tailWaitState?.fromTail === true
            ? leaf.tailWaitState
            : computeTerminalTailWaitState(leaf.tailBuffer, leaf.tailPartialLine, leaf.preview)
        const nextTail = appendNormalizedToTailBuffer(
          leaf.tailBuffer,
          leaf.tailPartialLine,
          normalized.text,
          leaf.tailRedrawCursor
        )
        const nextWaitState = computeTerminalTailWaitState(
          nextTail.lines,
          nextTail.partialLine,
          leaf.preview
        )
        if (tailGainedNewerBlockedReason(previousWaitState, nextWaitState, normalized.text)) {
          leaf.waitBlockedAt = at
        }
        leaf.tailWaitState = nextWaitState
        leaf.tailBuffer = nextTail.lines
        leaf.tailPartialLine = nextTail.partialLine
        leaf.tailRedrawCursor = nextTail.redrawCursor
        leaf.tailTruncated = leaf.tailTruncated || nextTail.truncated
        leaf.tailLinesTotal += nextTail.newCompleteLines
        leaf.preview = buildPreview(leaf.tailBuffer, leaf.tailPartialLine)
      }
    }

    // Why: feed the chunk's OSC titles through the shared per-PTY tracker in
    // byte order — the same ordering the renderer transport uses — so
    // coalesced working→idle transitions reach tui-idle waiters and
    // pending-message delivery instead of being masked by the chunk's last
    // title (issue #1083). Uses the OSC 9999-stripped cleanData like the
    // renderer, so pure status chunks don't perturb the stale-title probe.
    const titleTrackerEntry = this.getOrCreatePtyTitleTrackerEntry(ptyId)
    const previousTitleScanTail = this.ptyTranscripts.oscTitleScanTailByPtyId.get(ptyId)
    const titleInput = previousTitleScanTail
      ? `${previousTitleScanTail}${agentStatusChunk.cleanData}`
      : agentStatusChunk.cleanData
    const nextTitleScanTail = extractOscTitleScanTail(titleInput)
    if (nextTitleScanTail.length > 0) {
      this.ptyTranscripts.oscTitleScanTailByPtyId.set(ptyId, nextTitleScanTail)
    } else {
      this.ptyTranscripts.oscTitleScanTailByPtyId.delete(ptyId)
    }
    titleTrackerEntry.applyingChunk = true
    titleTrackerEntry.chunkTouchedSessionTabs = false
    let retainedAgentStatusChanged = false
    try {
      titleTrackerEntry.tracker.handleChunk(agentStatusChunk.cleanData, {
        titleScanData: titleInput
      })
      // Why: the Command Code scrape rides the same per-chunk batch (its facts
      // trail the tracker's). cleanData keeps OSC 9999 payloads out of the
      // detector's bounded recent-text window; the detector strips remaining
      // control sequences itself, exactly like the renderer byte path.
      titleTrackerEntry.commandCodeDetector?.observe(agentStatusChunk.cleanData)
    } finally {
      titleTrackerEntry.applyingChunk = false
      try {
        // Why: per-chunk cross-channel contract order is status → titles →
        // bell — the chunk's agentStatus:set events must reach the renderer
        // before its pty:sideEffect batch.
        retainedAgentStatusChanged = this.emitTerminalAgentStatusEvents(ptyId, agentStatusChunk)
      } finally {
        // Why: flushed in the finally so a throwing tracker callback cannot
        // strand this chunk's facts to be emitted under the next chunk's seq.
        this.flushPendingTerminalSideEffectFacts(ptyId, titleTrackerEntry)
      }
    }
    // Why: hook (OSC 9999) transitions often arrive without a title change, so
    // headless-serve snapshots would never republish and paired remote clients
    // kept the stale agent state until the next title change (#7970).
    if (titleTrackerEntry.chunkTouchedSessionTabs || retainedAgentStatusChanged) {
      this.touchMobileSessionSnapshotsForPty(ptyId)
    }

    const listeners = this.dataListeners.get(ptyId)
    if (listeners) {
      const meta = {
        seq: outputSequence,
        rawLength: data.length,
        ...(cwdChanged && cwd !== null ? { cwd } : {})
      }
      for (const listener of listeners) {
        listener(data, meta)
      }
    }
    return outputSequence
  }

  private readonly ptyWaitBlockedCheckCommands = new RuntimePtyWaitBlockedCheckCommands({
    getGraph: () => this.graph
  })

  scheduleWaitBlockedCheck: RuntimePtyWaitBlockedCheckCommands['scheduleWaitBlockedCheck'] =
    this.ptyWaitBlockedCheckCommands.scheduleWaitBlockedCheck.bind(this.ptyWaitBlockedCheckCommands)
  clearWaitBlockedCheckState: RuntimePtyWaitBlockedCheckCommands['clearWaitBlockedCheckState'] =
    this.ptyWaitBlockedCheckCommands.clearWaitBlockedCheckState.bind(
      this.ptyWaitBlockedCheckCommands
    )

  private readonly ptyTitleTrackerCommands = new RuntimePtyTitleTrackerCommands({
    getGraph: () => this.graph,
    getPtyTranscripts: () => this.ptyTranscripts,
    getOnTerminalSideEffects: () => this.onTerminalSideEffects !== null,
    getLeavesForPty: (ptyId) => this.getLeavesForPty(ptyId),
    nextTitleObservationSequence: () => this.nextTitleObservationSequence(),
    setPtyManagementTitleFromObservedTitle: (pty, title, observedAt) =>
      this.setPtyManagementTitleFromObservedTitle(pty, title, observedAt),
    recordTerminalSideEffectFact: (ptyId, fact) => this.recordTerminalSideEffectFact(ptyId, fact),
    touchMobileSessionSnapshotsForPty: (ptyId, options) =>
      this.touchMobileSessionSnapshotsForPty(ptyId, options),
    resolveTuiIdleWaiters: (leaf) => this.resolveTuiIdleWaiters(leaf),
    resolvePtyTuiIdleWaiters: (pty, ptyId) => this.resolvePtyTuiIdleWaiters(pty, ptyId),
    deliverPendingMessages: (leaf) => this.deliverPendingMessages(leaf),
    shouldDelayPtyBackedMobileSnapshotForForegroundAgent: (pty, title) =>
      this.shouldDelayPtyBackedMobileSnapshotForForegroundAgent(pty, title),
    refreshPtyForegroundAgentFromController: (ptyId, options) =>
      this.refreshPtyForegroundAgentFromController(ptyId, options),
    getPendingForegroundAgentRefreshForTitle: (ptyId, titleObservedAt) =>
      this.getPendingForegroundAgentRefreshForTitle(ptyId, titleObservedAt),
    delayPtyBackedMobileSnapshotForForegroundAgent: (ptyId, titleObservedAt, foregroundRefresh) =>
      this.delayPtyBackedMobileSnapshotForForegroundAgent(ptyId, titleObservedAt, foregroundRefresh)
  })

  getTrackedRawTitleForPty: RuntimePtyTitleTrackerCommands['getTrackedRawTitleForPty'] =
    this.ptyTitleTrackerCommands.getTrackedRawTitleForPty.bind(this.ptyTitleTrackerCommands)
  preferTrackedLastTitle: RuntimePtyTitleTrackerCommands['preferTrackedLastTitle'] =
    this.ptyTitleTrackerCommands.preferTrackedLastTitle.bind(this.ptyTitleTrackerCommands)
  applySeededAgentStatus: RuntimePtyTitleTrackerCommands['applySeededAgentStatus'] =
    this.ptyTitleTrackerCommands.applySeededAgentStatus.bind(this.ptyTitleTrackerCommands)
  getOrCreatePtyTitleTrackerEntry: RuntimePtyTitleTrackerCommands['getOrCreatePtyTitleTrackerEntry'] =
    this.ptyTitleTrackerCommands.getOrCreatePtyTitleTrackerEntry.bind(this.ptyTitleTrackerCommands)
  applyTrackedPtyTitle: RuntimePtyTitleTrackerCommands['applyTrackedPtyTitle'] =
    this.ptyTitleTrackerCommands.applyTrackedPtyTitle.bind(this.ptyTitleTrackerCommands)
  disposePtyTitleTracker: RuntimePtyTitleTrackerCommands['disposePtyTitleTracker'] =
    this.ptyTitleTrackerCommands.disposePtyTitleTracker.bind(this.ptyTitleTrackerCommands)

  private readonly terminalSideEffectsCommands = new RuntimeTerminalSideEffectsCommands({
    getGraph: () => this.graph,
    getPtyTranscripts: () => this.ptyTranscripts,
    getOnTerminalSideEffects: () => this.onTerminalSideEffects,
    getLeavesForPty: (ptyId) => this.getLeavesForPty(ptyId),
    getOrCreatePtyTitleTrackerEntry: (ptyId) => this.getOrCreatePtyTitleTrackerEntry(ptyId),
    getOrCreatePtyWorktreeRecord: (ptyId) => this.getOrCreatePtyWorktreeRecord(ptyId),
    makeRuntimePaneKey: (leaf) => this.makeRuntimePaneKey(leaf),
    touchMobileSessionSnapshotsForPty: (ptyId, options) =>
      this.touchMobileSessionSnapshotsForPty(ptyId, options),
    disposeHeadlessTerminal: (ptyId) => this.disposeHeadlessTerminal(ptyId)
  })

  processAgentStatusOscForPty: RuntimeTerminalSideEffectsCommands['processAgentStatusOscForPty'] =
    this.terminalSideEffectsCommands.processAgentStatusOscForPty.bind(
      this.terminalSideEffectsCommands
    )
  flushPendingTerminalSideEffectFacts: RuntimeTerminalSideEffectsCommands['flushPendingTerminalSideEffectFacts'] =
    this.terminalSideEffectsCommands.flushPendingTerminalSideEffectFacts.bind(
      this.terminalSideEffectsCommands
    )
  ingestSyntheticTitleFrame: RuntimeTerminalSideEffectsCommands['ingestSyntheticTitleFrame'] =
    this.terminalSideEffectsCommands.ingestSyntheticTitleFrame.bind(
      this.terminalSideEffectsCommands
    )
  setPtyTransientFactDelegation: RuntimeTerminalSideEffectsCommands['setPtyTransientFactDelegation'] =
    this.terminalSideEffectsCommands.setPtyTransientFactDelegation.bind(
      this.terminalSideEffectsCommands
    )
  emitDaemonPtyTransientFact: RuntimeTerminalSideEffectsCommands['emitDaemonPtyTransientFact'] =
    this.terminalSideEffectsCommands.emitDaemonPtyTransientFact.bind(
      this.terminalSideEffectsCommands
    )
  notePtyDataGap: RuntimeTerminalSideEffectsCommands['notePtyDataGap'] =
    this.terminalSideEffectsCommands.notePtyDataGap.bind(this.terminalSideEffectsCommands)
  recordTerminalSideEffectFact: RuntimeTerminalSideEffectsCommands['recordTerminalSideEffectFact'] =
    this.terminalSideEffectsCommands.recordTerminalSideEffectFact.bind(
      this.terminalSideEffectsCommands
    )
  getTerminalSideEffectSnapshot: RuntimeTerminalSideEffectsCommands['getTerminalSideEffectSnapshot'] =
    this.terminalSideEffectsCommands.getTerminalSideEffectSnapshot.bind(
      this.terminalSideEffectsCommands
    )
  recordOsc7MetadataForPty: RuntimeTerminalSideEffectsCommands['recordOsc7MetadataForPty'] =
    this.terminalSideEffectsCommands.recordOsc7MetadataForPty.bind(this.terminalSideEffectsCommands)
  pathFlavorForPty: RuntimeTerminalSideEffectsCommands['pathFlavorForPty'] =
    this.terminalSideEffectsCommands.pathFlavorForPty.bind(this.terminalSideEffectsCommands)

  private readonly agentRowSnapshotCommands = new RuntimeAgentRowSnapshotCommands({
    getGraph: () => this.graph,
    getLeavesForPty: (ptyId) => this.getLeavesForPty(ptyId),
    makeRuntimePaneKey: (leaf) => this.makeRuntimePaneKey(leaf),
    getOnTerminalAgentStatus: () => this.onTerminalAgentStatus,
    getLatestAgentStatusByPaneKey: () => this.latestAgentStatusByPaneKey
  })

  emitTerminalAgentStatusEvents: RuntimeAgentRowSnapshotCommands['emitTerminalAgentStatusEvents'] =
    this.agentRowSnapshotCommands.emitTerminalAgentStatusEvents.bind(this.agentRowSnapshotCommands)
  clearAgentRowSnapshotsForPty: RuntimeAgentRowSnapshotCommands['clearAgentRowSnapshotsForPty'] =
    this.agentRowSnapshotCommands.clearAgentRowSnapshotsForPty.bind(this.agentRowSnapshotCommands)

  getPtyOutputSequence(ptyId: string): number {
    return this.ptyTranscripts.ptyOutputSequenceById.get(ptyId) ?? 0
  }

  subscribeToTerminalData(
    ptyId: string,
    listener: (data: string, meta?: { seq?: number; rawLength?: number; cwd?: string }) => void
  ): () => void {
    return addListenerToMap(this.dataListeners, ptyId, listener)
  }

  /** Set by pty IPC: fires when a PTY gains/loses remote view subscribers so
   *  the daemon background mark (keep-tail stream thinning) can resync — a
   *  live mobile/web view consumes raw bytes and must never be thinned, even
   *  while the desktop pane is hidden. */
  onRemoteTerminalViewPresenceChanged: ((ptyId: string) => void) | null = null

  private readonly remoteTerminalViewSubscriberCommands =
    new RuntimeRemoteTerminalViewSubscriberCommands({
      hasMobileSubscriber: (ptyId) => this.mobileFloorCommands.hasMobileSubscriber(ptyId),
      getOnRemoteTerminalViewPresenceChanged: () => this.onRemoteTerminalViewPresenceChanged
    })

  notifyRemoteTerminalViewPresenceChanged: RuntimeRemoteTerminalViewSubscriberCommands['notifyRemoteTerminalViewPresenceChanged'] =
    this.remoteTerminalViewSubscriberCommands.notifyRemoteTerminalViewPresenceChanged.bind(
      this.remoteTerminalViewSubscriberCommands
    )
  registerRemoteTerminalViewSubscriber: RuntimeRemoteTerminalViewSubscriberCommands['registerRemoteTerminalViewSubscriber'] =
    this.remoteTerminalViewSubscriberCommands.registerRemoteTerminalViewSubscriber.bind(
      this.remoteTerminalViewSubscriberCommands
    )
  hasRemoteTerminalViewSubscriber: RuntimeRemoteTerminalViewSubscriberCommands['hasRemoteTerminalViewSubscriber'] =
    this.remoteTerminalViewSubscriberCommands.hasRemoteTerminalViewSubscriber.bind(
      this.remoteTerminalViewSubscriberCommands
    )

  serializeTerminalBuffer(
    ptyId: string,
    opts: { scrollbackRows?: number } = {}
  ): Promise<{
    data: string
    cols: number
    rows: number
    cwd?: string | null
    lastTitle?: string
    seq?: number
    source?: 'headless' | 'renderer'
    oscLinks?: TerminalOscLinkRange[]
    alternateScreen?: boolean
    scrollbackAnsi?: string
    pendingEscapeTailAnsi?: string
  } | null> {
    return this.serializeTerminalBufferFromAvailableState(ptyId, opts)
  }

  serializeMainTerminalBuffer(
    ptyId: string,
    opts: { scrollbackRows?: number } = {}
  ): Promise<{
    data: string
    cols: number
    rows: number
    cwd?: string | null
    lastTitle?: string
    seq?: number
    source?: 'headless' | 'renderer'
    oscLinks?: TerminalOscLinkRange[]
    alternateScreen?: boolean
    scrollbackAnsi?: string
  } | null> {
    return this.serializeHeadlessTerminalBuffer(ptyId, { ...opts, includeEmpty: true })
  }

  async serializeHiddenOutputRecoveryBuffer(
    ptyId: string,
    opts: { scrollbackRows?: number } = {}
  ): Promise<{
    data: string
    cols: number
    rows: number
    cwd?: string | null
    lastTitle?: string
    seq?: number
    source?: 'headless' | 'renderer'
    oscLinks?: TerminalOscLinkRange[]
    alternateScreen?: boolean
    scrollbackAnsi?: string
    pendingEscapeTailAnsi?: string
  } | null> {
    const headlessSnapshot = await this.serializeHeadlessTerminalBuffer(ptyId, {
      ...opts,
      includeEmpty: true
    })
    if (headlessSnapshot) {
      return headlessSnapshot
    }
    // Why: hidden-output recovery is initiated by the desktop renderer. If the
    // runtime has not built headless state yet, the mounted xterm is still the
    // best available state and avoids a false "snapshot unavailable" result.
    return this.serializeRendererTerminalBuffer(ptyId, opts)
  }

  async clearTerminalBuffer(handle: string): Promise<{ handle: string; cleared: boolean }> {
    const leaf = this.resolveLeafForHandle(handle)
    if (!leaf?.ptyId) {
      throw new Error('terminal_not_found')
    }
    // Why: clear is a terminal UI action (Cmd+K on desktop), not shell input.
    // Route through the controller so renderer-owned xterm buffers, daemon
    // sessions, and SSH relay sessions all drop scrollback before the next
    // mobile snapshot.
    await this.ptyController?.clearBuffer?.(leaf.ptyId)
    await this.clearHeadlessTerminalBuffer(leaf.ptyId)
    return { handle, cleared: true }
  }

  getTerminalSize(ptyId: string): { cols: number; rows: number } | null {
    return this.ptyController?.getSize?.(ptyId) ?? null
  }

  // Why: a width reflow on a normal-buffer PTY must re-stream the full
  // scrollback to mobile so it rewraps at the new cols, but alternate-screen
  // TUIs (vim, Claude Code) own their repaint and have no scrollback — for
  // those the mobile client just resizes xterm geometry and consumes the
  // TUI's own redraw, so the resize re-stream must be skipped. Returns false
  // when there is no headless emulator (resize falls back to geometry-only).
  isTerminalAlternateScreen(ptyId: string): boolean {
    return this.headlessTerminalCommands.isAlternateScreen(ptyId)
  }

  seedHeadlessTerminal: RuntimeHeadlessTerminalCommands['seedHeadlessTerminal'] =
    this.headlessTerminalCommands.seedHeadlessTerminal.bind(this.headlessTerminalCommands)
  maybeHydrateHeadlessFromRenderer: RuntimeHeadlessTerminalCommands['maybeHydrateHeadlessFromRenderer'] =
    this.headlessTerminalCommands.maybeHydrateHeadlessFromRenderer.bind(
      this.headlessTerminalCommands
    )
  shouldAnswerQueriesForLiveChunk: RuntimeHeadlessTerminalCommands['shouldAnswerQueriesForLiveChunk'] =
    this.headlessTerminalCommands.shouldAnswerQueriesForLiveChunk.bind(
      this.headlessTerminalCommands
    )
  trackHeadlessTerminalData: RuntimeHeadlessTerminalCommands['trackHeadlessTerminalData'] =
    this.headlessTerminalCommands.trackHeadlessTerminalData.bind(this.headlessTerminalCommands)
  ensureNativeWindowsConptyDa1Override: RuntimeHeadlessTerminalCommands['ensureNativeWindowsConptyDa1Override'] =
    this.headlessTerminalCommands.ensureNativeWindowsConptyDa1Override.bind(
      this.headlessTerminalCommands
    )
  resizeHeadlessTerminal: RuntimeHeadlessTerminalCommands['resizeHeadlessTerminal'] =
    this.headlessTerminalCommands.resizeHeadlessTerminal.bind(this.headlessTerminalCommands)
  clearHeadlessTerminalBuffer: RuntimeHeadlessTerminalCommands['clearHeadlessTerminalBuffer'] =
    this.headlessTerminalCommands.clearHeadlessTerminalBuffer.bind(this.headlessTerminalCommands)
  serializeTerminalBufferFromAvailableState: RuntimeHeadlessTerminalCommands['serializeTerminalBufferFromAvailableState'] =
    this.headlessTerminalCommands.serializeTerminalBufferFromAvailableState.bind(
      this.headlessTerminalCommands
    )
  serializeRendererTerminalBuffer: RuntimeHeadlessTerminalCommands['serializeRendererTerminalBuffer'] =
    this.headlessTerminalCommands.serializeRendererTerminalBuffer.bind(
      this.headlessTerminalCommands
    )
  withVisibleSnapshotFallback: RuntimeHeadlessTerminalCommands['withVisibleSnapshotFallback'] =
    this.headlessTerminalCommands.withVisibleSnapshotFallback.bind(this.headlessTerminalCommands)
  serializeHeadlessTerminalBuffer: RuntimeHeadlessTerminalCommands['serializeHeadlessTerminalBuffer'] =
    this.headlessTerminalCommands.serializeHeadlessTerminalBuffer.bind(
      this.headlessTerminalCommands
    )
  disposeHeadlessTerminal: RuntimeHeadlessTerminalCommands['disposeHeadlessTerminal'] =
    this.headlessTerminalCommands.disposeHeadlessTerminal.bind(this.headlessTerminalCommands)

  resolveLeafForHandle(handle: string): { ptyId: string | null } | null {
    const record = this.graph.handles.get(handle)
    if (!record) {
      return null
    }
    if (record.tabId.startsWith('pty:')) {
      return { ptyId: record.ptyId }
    }
    const leaf = this.graph.leaves.get(this.getLeafKey(record.tabId, record.leafId))
    if (!leaf) {
      return null
    }
    return { ptyId: leaf.ptyId }
  }

  // Why: remote clients hold handles across transport reconnects. A handle
  // minted for a concrete PTY must never silently adopt a different PTY that
  // later occupies the same pane — that misroutes keystrokes (#7718). Handles
  // still awaiting their first PTY (ptyId null) may adopt it, which preserves
  // the mobile pre-spawn subscribe flow.
  resolveLiveLeafForHandle(handle: string): { ptyId: string | null } | null {
    const record = this.graph.handles.get(handle)
    if (!record) {
      return null
    }
    if (record.tabId.startsWith('pty:')) {
      return { ptyId: record.ptyId }
    }
    const leaf = this.graph.leaves.get(this.getLeafKey(record.tabId, record.leafId))
    if (!leaf) {
      return null
    }
    if (
      record.ptyId !== null &&
      (leaf.ptyId !== record.ptyId || leaf.ptyGeneration !== record.ptyGeneration)
    ) {
      throw new Error('terminal_handle_stale')
    }
    return { ptyId: leaf.ptyId }
  }

  async resolveTerminalCwd(handle: string): Promise<string | null> {
    const ptyId = this.resolveLeafForHandle(handle)?.ptyId
    if (!ptyId) {
      return null
    }
    const tracked = this.ptyTranscripts.terminalCwdByPtyId.get(ptyId)
    if (tracked) {
      return tracked
    }
    try {
      const cwd = await this.ptyController?.getCwd?.(ptyId)
      return cwd && cwd.trim().length > 0 ? cwd : null
    } catch {
      return null
    }
  }

  resolveTerminalFileUriHostname(handle: string): string | null {
    const ptyId = this.resolveLeafForHandle(handle)?.ptyId
    return ptyId ? (this.ptyTranscripts.terminalFileUriHostnameByPtyId.get(ptyId) ?? null) : null
  }

  private recordRecentPtyOutputForPathProvenance(ptyId: string, data: string): void {
    this.ptyTranscripts.recentPtyOutputById.set(
      ptyId,
      appendRecentPtyOutput(this.ptyTranscripts.recentPtyOutputById.get(ptyId), data)
    )
    this.ptyTranscripts.recentPtyPathCandidatesById.set(
      ptyId,
      appendRecentPtyPathCandidates(
        this.ptyTranscripts.recentPtyPathCandidatesById.get(ptyId),
        data
      )
    )
  }

  resolveTerminalContext(
    handle: string
  ): { worktreeId: string; connectionId: string | null } | null {
    const ptyId = this.resolveLeafForHandle(handle)?.ptyId
    const pty = ptyId ? this.graph.ptysById.get(ptyId) : null
    return pty ? { worktreeId: pty.worktreeId, connectionId: pty.connectionId } : null
  }

  hasRecentTerminalOutputPath(handle: string, pathText: string, absolutePath: string): boolean {
    const ptyId = this.resolveLeafForHandle(handle)?.ptyId
    const recentOutput = ptyId ? this.ptyTranscripts.recentPtyOutputById.get(ptyId) : null
    if (recentOutput && recentTerminalOutputIncludesPath(recentOutput, pathText, absolutePath)) {
      return true
    }
    const candidates = ptyId ? this.ptyTranscripts.recentPtyPathCandidatesById.get(ptyId) : null
    return candidates
      ? recentTerminalPathCandidatesIncludePath(candidates, pathText, absolutePath)
      : false
  }

  private readonly connectionSubscriptionNotifyCommands =
    new RuntimeConnectionSubscriptionNotifyCommands({
      getPushManager: () => this.pushManager
    })

  subscribeToFitOverrideChanges: RuntimeConnectionSubscriptionNotifyCommands['subscribeToFitOverrideChanges'] =
    this.connectionSubscriptionNotifyCommands.subscribeToFitOverrideChanges.bind(
      this.connectionSubscriptionNotifyCommands
    )
  notifyFitOverrideListeners: RuntimeConnectionSubscriptionNotifyCommands['notifyFitOverrideListeners'] =
    this.connectionSubscriptionNotifyCommands.notifyFitOverrideListeners.bind(
      this.connectionSubscriptionNotifyCommands
    )
  registerSubscriptionCleanup: RuntimeConnectionSubscriptionNotifyCommands['registerSubscriptionCleanup'] =
    this.connectionSubscriptionNotifyCommands.registerSubscriptionCleanup.bind(
      this.connectionSubscriptionNotifyCommands
    )
  cleanupSubscription: RuntimeConnectionSubscriptionNotifyCommands['cleanupSubscription'] =
    this.connectionSubscriptionNotifyCommands.cleanupSubscription.bind(
      this.connectionSubscriptionNotifyCommands
    )
  cleanupSubscriptionsByPrefix: RuntimeConnectionSubscriptionNotifyCommands['cleanupSubscriptionsByPrefix'] =
    this.connectionSubscriptionNotifyCommands.cleanupSubscriptionsByPrefix.bind(
      this.connectionSubscriptionNotifyCommands
    )
  cleanupSubscriptionsForConnection: RuntimeConnectionSubscriptionNotifyCommands['cleanupSubscriptionsForConnection'] =
    this.connectionSubscriptionNotifyCommands.cleanupSubscriptionsForConnection.bind(
      this.connectionSubscriptionNotifyCommands
    )
  onNotificationDispatched: RuntimeConnectionSubscriptionNotifyCommands['onNotificationDispatched'] =
    this.connectionSubscriptionNotifyCommands.onNotificationDispatched.bind(
      this.connectionSubscriptionNotifyCommands
    )
  getMobileNotificationListenerCount: RuntimeConnectionSubscriptionNotifyCommands['getMobileNotificationListenerCount'] =
    this.connectionSubscriptionNotifyCommands.getMobileNotificationListenerCount.bind(
      this.connectionSubscriptionNotifyCommands
    )
  dispatchMobileNotification: RuntimeConnectionSubscriptionNotifyCommands['dispatchMobileNotification'] =
    this.connectionSubscriptionNotifyCommands.dispatchMobileNotification.bind(
      this.connectionSubscriptionNotifyCommands
    )
  dismissMobileNotification: RuntimeConnectionSubscriptionNotifyCommands['dismissMobileNotification'] =
    this.connectionSubscriptionNotifyCommands.dismissMobileNotification.bind(
      this.connectionSubscriptionNotifyCommands
    )

  setCommitMessageAgentEnvironmentResolvers(
    resolvers: CommitMessageAgentEnvironmentResolvers
  ): void {
    this.commitMessageAgentEnv = resolvers
  }

  getCommitMessageAgentEnvironmentResolvers(): CommitMessageAgentEnvironmentResolvers | undefined {
    return this.commitMessageAgentEnv ?? undefined
  }

  private readonly mobileDictationCommands = new RuntimeMobileDictationCommands({
    getStore: () => this.store
  })

  listMobileSpeechModels: RuntimeMobileDictationCommands['listMobileSpeechModels'] =
    this.mobileDictationCommands.listMobileSpeechModels.bind(this.mobileDictationCommands)
  downloadMobileSpeechModel: RuntimeMobileDictationCommands['downloadMobileSpeechModel'] =
    this.mobileDictationCommands.downloadMobileSpeechModel.bind(this.mobileDictationCommands)
  deleteMobileSpeechModel: RuntimeMobileDictationCommands['deleteMobileSpeechModel'] =
    this.mobileDictationCommands.deleteMobileSpeechModel.bind(this.mobileDictationCommands)
  configureMobileDictation: RuntimeMobileDictationCommands['configureMobileDictation'] =
    this.mobileDictationCommands.configureMobileDictation.bind(this.mobileDictationCommands)
  startMobileDictation: RuntimeMobileDictationCommands['startMobileDictation'] =
    this.mobileDictationCommands.startMobileDictation.bind(this.mobileDictationCommands)
  feedMobileDictation: RuntimeMobileDictationCommands['feedMobileDictation'] =
    this.mobileDictationCommands.feedMobileDictation.bind(this.mobileDictationCommands)
  finishMobileDictation: RuntimeMobileDictationCommands['finishMobileDictation'] =
    this.mobileDictationCommands.finishMobileDictation.bind(this.mobileDictationCommands)
  cancelMobileDictation: RuntimeMobileDictationCommands['cancelMobileDictation'] =
    this.mobileDictationCommands.cancelMobileDictation.bind(this.mobileDictationCommands)
  cancelMobileDictationForConnection: RuntimeMobileDictationCommands['cancelMobileDictationForConnection'] =
    this.mobileDictationCommands.cancelMobileDictationForConnection.bind(
      this.mobileDictationCommands
    )
  cancelMobileDictationForClient: RuntimeMobileDictationCommands['cancelMobileDictationForClient'] =
    this.mobileDictationCommands.cancelMobileDictationForClient.bind(this.mobileDictationCommands)

  // ─── Account Services (mobile RPC bridge) ─────────────────────

  private readonly accountServicesCommands = new RuntimeAccountServicesCommands()

  setAccountServices: RuntimeAccountServicesCommands['setAccountServices'] =
    this.accountServicesCommands.setAccountServices.bind(this.accountServicesCommands)
  getAccountsSnapshot: RuntimeAccountServicesCommands['getAccountsSnapshot'] =
    this.accountServicesCommands.getAccountsSnapshot.bind(this.accountServicesCommands)
  refreshAccountsForMobile: RuntimeAccountServicesCommands['refreshAccountsForMobile'] =
    this.accountServicesCommands.refreshAccountsForMobile.bind(this.accountServicesCommands)
  selectClaudeAccount: RuntimeAccountServicesCommands['selectClaudeAccount'] =
    this.accountServicesCommands.selectClaudeAccount.bind(this.accountServicesCommands)
  selectCodexAccount: RuntimeAccountServicesCommands['selectCodexAccount'] =
    this.accountServicesCommands.selectCodexAccount.bind(this.accountServicesCommands)
  removeClaudeAccount: RuntimeAccountServicesCommands['removeClaudeAccount'] =
    this.accountServicesCommands.removeClaudeAccount.bind(this.accountServicesCommands)
  removeCodexAccount: RuntimeAccountServicesCommands['removeCodexAccount'] =
    this.accountServicesCommands.removeCodexAccount.bind(this.accountServicesCommands)
  onAccountsChanged: RuntimeAccountServicesCommands['onAccountsChanged'] =
    this.accountServicesCommands.onAccountsChanged.bind(this.accountServicesCommands)

  // ─── Mobile Fit Override Management ─────────────────────────

  private readonly mobileFloorCommands = new RuntimeMobileFloorCommands({
    getStore: () => this.store,
    getNotifier: () => this.notifier,
    getPtyController: () => this.ptyController,
    getTerminalSize: (ptyId) => this.getTerminalSize(ptyId),
    resizeHeadlessTerminal: (ptyId, cols, rows) => this.resizeHeadlessTerminal(ptyId, cols, rows),
    notifyRemoteTerminalViewPresenceChanged: (ptyId) =>
      this.notifyRemoteTerminalViewPresenceChanged(ptyId),
    notifyFitOverrideListeners: (ptyId, reason, cols, rows) =>
      this.notifyFitOverrideListeners(ptyId, reason, cols, rows),
    revokeTerminalFileGrantsForClient: (clientId) =>
      this.revokeTerminalFileGrantsForClient(clientId),
    cancelMobileDictationForClient: (clientId) => this.cancelMobileDictationForClient(clientId),
    cancelBrowserScreencastForPage: (browserPageId) =>
      this.cancelBrowserScreencastForPage(browserPageId),
    getAgentBrowserBridge: () => this.agentBrowserBridge
  })

  getDriver: RuntimeMobileFloorCommands['getDriver'] = this.mobileFloorCommands.getDriver.bind(
    this.mobileFloorCommands
  )
  isPtyResizeDrivenRemotely: RuntimeMobileFloorCommands['isPtyResizeDrivenRemotely'] =
    this.mobileFloorCommands.isPtyResizeDrivenRemotely.bind(this.mobileFloorCommands)
  isRemoteDesktopResizeDriven: RuntimeMobileFloorCommands['isRemoteDesktopResizeDriven'] =
    this.mobileFloorCommands.isRemoteDesktopResizeDriven.bind(this.mobileFloorCommands)
  isRemoteDesktopViewerOwner: RuntimeMobileFloorCommands['isRemoteDesktopViewerOwner'] =
    this.mobileFloorCommands.isRemoteDesktopViewerOwner.bind(this.mobileFloorCommands)
  getRemoteDesktopFitHold: RuntimeMobileFloorCommands['getRemoteDesktopFitHold'] =
    this.mobileFloorCommands.getRemoteDesktopFitHold.bind(this.mobileFloorCommands)
  recordRemoteDesktopHostReclaimTarget: RuntimeMobileFloorCommands['recordRemoteDesktopHostReclaimTarget'] =
    this.mobileFloorCommands.recordRemoteDesktopHostReclaimTarget.bind(this.mobileFloorCommands)
  applyRemoteDesktopLayout: RuntimeMobileFloorCommands['applyRemoteDesktopLayout'] =
    this.mobileFloorCommands.applyRemoteDesktopLayout.bind(this.mobileFloorCommands)
  updateRemoteDesktopViewer: RuntimeMobileFloorCommands['updateRemoteDesktopViewer'] =
    this.mobileFloorCommands.updateRemoteDesktopViewer.bind(this.mobileFloorCommands)
  claimRemoteDesktopViewer: RuntimeMobileFloorCommands['claimRemoteDesktopViewer'] =
    this.mobileFloorCommands.claimRemoteDesktopViewer.bind(this.mobileFloorCommands)
  claimRemoteDesktopHost: RuntimeMobileFloorCommands['claimRemoteDesktopHost'] =
    this.mobileFloorCommands.claimRemoteDesktopHost.bind(this.mobileFloorCommands)
  unregisterRemoteDesktopViewer: RuntimeMobileFloorCommands['unregisterRemoteDesktopViewer'] =
    this.mobileFloorCommands.unregisterRemoteDesktopViewer.bind(this.mobileFloorCommands)
  unregisterRemoteDesktopViewers: RuntimeMobileFloorCommands['unregisterRemoteDesktopViewers'] =
    this.mobileFloorCommands.unregisterRemoteDesktopViewers.bind(this.mobileFloorCommands)
  refreshRemoteDesktopViewer: RuntimeMobileFloorCommands['refreshRemoteDesktopViewer'] =
    this.mobileFloorCommands.refreshRemoteDesktopViewer.bind(this.mobileFloorCommands)
  updateDesktopViewport: RuntimeMobileFloorCommands['updateDesktopViewport'] =
    this.mobileFloorCommands.updateDesktopViewport.bind(this.mobileFloorCommands)
  markMobileActor: RuntimeMobileFloorCommands['markMobileActor'] =
    this.mobileFloorCommands.markMobileActor.bind(this.mobileFloorCommands)
  mobileTookFloor: RuntimeMobileFloorCommands['mobileTookFloor'] =
    this.mobileFloorCommands.mobileTookFloor.bind(this.mobileFloorCommands)
  updateMobileViewport: RuntimeMobileFloorCommands['updateMobileViewport'] =
    this.mobileFloorCommands.updateMobileViewport.bind(this.mobileFloorCommands)
  reclaimTerminalForDesktop: RuntimeMobileFloorCommands['reclaimTerminalForDesktop'] =
    this.mobileFloorCommands.reclaimTerminalForDesktop.bind(this.mobileFloorCommands)
  cancelAllPendingFitRestoreTimers: RuntimeMobileFloorCommands['cancelAllPendingFitRestoreTimers'] =
    this.mobileFloorCommands.cancelAllPendingFitRestoreTimers.bind(this.mobileFloorCommands)
  getMobileAutoRestoreFitMs: RuntimeMobileFloorCommands['getMobileAutoRestoreFitMs'] =
    this.mobileFloorCommands.getMobileAutoRestoreFitMs.bind(this.mobileFloorCommands)
  setMobileAutoRestoreFitMs: RuntimeMobileFloorCommands['setMobileAutoRestoreFitMs'] =
    this.mobileFloorCommands.setMobileAutoRestoreFitMs.bind(this.mobileFloorCommands)
  getLayout: RuntimeMobileFloorCommands['getLayout'] = this.mobileFloorCommands.getLayout.bind(
    this.mobileFloorCommands
  )
  setMobileDisplayMode: RuntimeMobileFloorCommands['setMobileDisplayMode'] =
    this.mobileFloorCommands.setMobileDisplayMode.bind(this.mobileFloorCommands)
  getMobileDisplayMode: RuntimeMobileFloorCommands['getMobileDisplayMode'] =
    this.mobileFloorCommands.getMobileDisplayMode.bind(this.mobileFloorCommands)
  isMobileSubscriberActive: RuntimeMobileFloorCommands['isMobileSubscriberActive'] =
    this.mobileFloorCommands.isMobileSubscriberActive.bind(this.mobileFloorCommands)
  updateMobileSubscriberViewport: RuntimeMobileFloorCommands['updateMobileSubscriberViewport'] =
    this.mobileFloorCommands.updateMobileSubscriberViewport.bind(this.mobileFloorCommands)
  handleMobileSubscribe: RuntimeMobileFloorCommands['handleMobileSubscribe'] =
    this.mobileFloorCommands.handleMobileSubscribe.bind(this.mobileFloorCommands)
  handleMobileUnsubscribe: RuntimeMobileFloorCommands['handleMobileUnsubscribe'] =
    this.mobileFloorCommands.handleMobileUnsubscribe.bind(this.mobileFloorCommands)
  applyMobileDisplayMode: RuntimeMobileFloorCommands['applyMobileDisplayMode'] =
    this.mobileFloorCommands.applyMobileDisplayMode.bind(this.mobileFloorCommands)
  onExternalPtyResize: RuntimeMobileFloorCommands['onExternalPtyResize'] =
    this.mobileFloorCommands.onExternalPtyResize.bind(this.mobileFloorCommands)
  recordRendererGeometry: RuntimeMobileFloorCommands['recordRendererGeometry'] =
    this.mobileFloorCommands.recordRendererGeometry.bind(this.mobileFloorCommands)
  getLastRendererSize: RuntimeMobileFloorCommands['getLastRendererSize'] =
    this.mobileFloorCommands.getLastRendererSize.bind(this.mobileFloorCommands)
  isResizeSuppressed: RuntimeMobileFloorCommands['isResizeSuppressed'] =
    this.mobileFloorCommands.isResizeSuppressed.bind(this.mobileFloorCommands)
  subscribeToTerminalResize: RuntimeMobileFloorCommands['subscribeToTerminalResize'] =
    this.mobileFloorCommands.subscribeToTerminalResize.bind(this.mobileFloorCommands)
  subscribeToDriverChanges: RuntimeMobileFloorCommands['subscribeToDriverChanges'] =
    this.mobileFloorCommands.subscribeToDriverChanges.bind(this.mobileFloorCommands)
  resizeForClient: RuntimeMobileFloorCommands['resizeForClient'] =
    this.mobileFloorCommands.resizeForClient.bind(this.mobileFloorCommands)
  isMobileTerminalQueryReplyAuthority: RuntimeMobileFloorCommands['isMobileTerminalQueryReplyAuthority'] =
    this.mobileFloorCommands.isMobileTerminalQueryReplyAuthority.bind(this.mobileFloorCommands)
  enqueueLayout: RuntimeMobileFloorCommands['enqueueLayout'] =
    this.mobileFloorCommands.enqueueLayout.bind(this.mobileFloorCommands)
  resolveDesktopRestoreTarget: RuntimeMobileFloorCommands['resolveDesktopRestoreTarget'] =
    this.mobileFloorCommands.resolveDesktopRestoreTarget.bind(this.mobileFloorCommands)
  getBrowserDriver: RuntimeMobileFloorCommands['getBrowserDriver'] =
    this.mobileFloorCommands.getBrowserDriver.bind(this.mobileFloorCommands)
  setBrowserDriver: RuntimeMobileFloorCommands['setBrowserDriver'] =
    this.mobileFloorCommands.setBrowserDriver.bind(this.mobileFloorCommands)
  reclaimBrowserForDesktop: RuntimeMobileFloorCommands['reclaimBrowserForDesktop'] =
    this.mobileFloorCommands.reclaimBrowserForDesktop.bind(this.mobileFloorCommands)
  getTerminalFitOverride: RuntimeMobileFloorCommands['getTerminalFitOverride'] =
    this.mobileFloorCommands.getTerminalFitOverride.bind(this.mobileFloorCommands)
  getAllTerminalFitOverrides: RuntimeMobileFloorCommands['getAllTerminalFitOverrides'] =
    this.mobileFloorCommands.getAllTerminalFitOverrides.bind(this.mobileFloorCommands)
  getAllTerminalDrivers: RuntimeMobileFloorCommands['getAllTerminalDrivers'] =
    this.mobileFloorCommands.getAllTerminalDrivers.bind(this.mobileFloorCommands)
  getAllBrowserDrivers: RuntimeMobileFloorCommands['getAllBrowserDrivers'] =
    this.mobileFloorCommands.getAllBrowserDrivers.bind(this.mobileFloorCommands)
  onClientDisconnected: RuntimeMobileFloorCommands['onClientDisconnected'] =
    this.mobileFloorCommands.onClientDisconnected.bind(this.mobileFloorCommands)

  onPtyExit(ptyId: string, exitCode: number): void {
    advertisedUrlWatcher.unbindPty(ptyId)
    serveSimStateWatcher.unbindPty(ptyId)
    // Clean up new mobile state for this PTY
    this.remoteTerminalViewSubscriberCommands.clearRemoteTerminalViewSubscriberCountForPty(ptyId)
    this.ptyTranscripts.recentPtyOutputById.delete(ptyId)
    this.clearWaitBlockedCheckState(ptyId)
    this.ptyTranscripts.recentPtyPathCandidatesById.delete(ptyId)
    this.ptyTranscripts.ptyOutputSequenceById.delete(ptyId)
    this.ptyTranscripts.agentStatusOscProcessorsByPtyId.delete(ptyId)
    this.ptyTranscripts.terminalSpawnCommandsByPtyId.delete(ptyId)
    this.disposePtyTitleTracker(ptyId)
    this.ptyTranscripts.oscTitleScanTailByPtyId.delete(ptyId)
    this.ptyTranscripts.osc7ScanTailByPtyId.delete(ptyId)
    this.ptyTranscripts.terminalCwdByPtyId.delete(ptyId)
    this.ptyTranscripts.terminalFileUriHostnameByPtyId.delete(ptyId)
    this.clearAgentRowSnapshotsForPty(ptyId)
    // Why: a Claude agent-team leader whose PTY exits naturally (agent finished,
    // process died, renderer reload) must release its team + nested panes map.
    // Previously only explicit closeTerminal evicted it, so natural exits leaked
    // one team per never-reused teamId for the runtime's lifetime.
    const exitedTeamLeaderHandle = this.graph.handleByPtyId.get(ptyId)
    if (exitedTeamLeaderHandle) {
      this.claudeAgentTeams.removeTeamForLeaderHandle(exitedTeamLeaderHandle)
    }
    // Why: mobile floor/layout/remote-desktop state for this PTY moved to
    // RuntimeMobileFloorCommands (TASK-BIGFILE-037) — delegate the cleanup.
    this.mobileFloorCommands.clearStateForExitedPty(ptyId)
    this.disposeHeadlessTerminal(ptyId)
    this.agentDetector?.onExit(ptyId)
    const pty = this.graph.ptysById.get(ptyId)
    if (pty) {
      pty.connected = false
      pty.disconnectedAt = Date.now()
      pty.lastExitCode = exitCode
      this.resolvePtyExitWaiters(pty, ptyId)
      this.pruneDisconnectedPtyTranscript(pty)
      this.touchMobileSessionSnapshotsForPty(ptyId, { immediate: true })
    }

    for (const leaf of this.getLeavesForPty(ptyId)) {
      this.graph.detachedPreAllocatedLeaves.delete(ptyId)
      leaf.connected = false
      leaf.writable = false
      leaf.lastExitCode = exitCode
      this.resolveExitWaiters(leaf)
      this.failActiveDispatchOnExit(leaf, exitCode)
    }
    this.pruneDisconnectedPtyRecords()
  }

  // Why: Section 7.2 — the runtime detects agent exit directly and updates
  // dispatch contexts immediately, rather than waiting for the coordinator's
  // next poll cycle. This catches agent crashes and unexpected exits within
  // milliseconds. The task is set back to 'pending' so it can be re-dispatched.
  private failActiveDispatchOnExit(leaf: RuntimeLeafRecord, exitCode: number): void {
    if (!this._orchestrationDb) {
      return
    }

    const handle = this.graph.handleByLeafKey.get(this.getLeafKey(leaf.tabId, leaf.leafId))
    if (!handle) {
      return
    }

    const dispatch = this._orchestrationDb.getActiveDispatchForTerminal(handle)
    if (!dispatch) {
      return
    }

    const errorContext = `Agent exited with code ${exitCode}`
    this._orchestrationDb.failDispatch(dispatch.id, errorContext)

    // Why: create an escalation message so the coordinator is notified about
    // the unexpected exit on its next check cycle, even if the circuit breaker
    // hasn't tripped yet.
    const run = this._orchestrationDb.getActiveCoordinatorRun()
    if (run) {
      this._orchestrationDb.insertMessage({
        from: handle,
        to: run.coordinator_handle,
        subject: `Agent exited unexpectedly (code ${exitCode})`,
        type: 'escalation',
        priority: 'high',
        payload: JSON.stringify({
          taskId: dispatch.task_id,
          exitCode,
          handle
        })
      })
    }
  }

  private readonly terminalListingCommands = new RuntimeTerminalListingCommands({
    getGraph: () => this.graph,
    getLeafKey: (tabId, leafId) => this.getLeafKey(tabId, leafId),
    peekResolvedWorktreeCache: () => this.resolvedWorktreeCommands.peekCache(),
    getMobileSessionTabsByWorktree: () => this.mobileSessionTabsByWorktree,
    resolveWorktreeSelector: (selector) => this.resolveWorktreeSelector(selector),
    refreshPtyWorktreeRecordsFromController: (resolvedWorktrees, targetWorktreeId) =>
      this.refreshPtyWorktreeRecordsFromController(resolvedWorktrees, targetWorktreeId),
    listKnownResolvedWorktreesForExplicitTarget: (targetWorktreeId, targetWorktree) =>
      this.listKnownResolvedWorktreesForExplicitTarget(targetWorktreeId, targetWorktree),
    getValidatedExplicitWorktreeIdSelector: (selector) =>
      this.getValidatedExplicitWorktreeIdSelector(selector),
    getResolvedWorktreeMap: () => this.getResolvedWorktreeMap(),
    buildTerminalSummary: (leaf, worktreesById) => this.buildTerminalSummary(leaf, worktreesById),
    buildResolvedWorktreeFromId: (worktreeId) => this.buildResolvedWorktreeFromId(worktreeId),
    buildPtyTerminalSummary: (pty, worktreesById) =>
      this.buildPtyTerminalSummary(pty, worktreesById),
    assertStableReadyGraph: (expectedGraphEpoch) => this.assertStableReadyGraph(expectedGraphEpoch)
  })

  listTerminals: RuntimeTerminalListingCommands['listTerminals'] =
    this.terminalListingCommands.listTerminals.bind(this.terminalListingCommands)

  // Why: when --terminal is omitted, the CLI auto-resolves to the active
  // terminal in the current worktree — matching browser's implicit active tab.
  async resolveActiveTerminal(worktreeSelector?: string): Promise<string> {
    if (this.graph.graphStatus !== 'ready') {
      const targetWorktreeId = worktreeSelector
        ? (await this.resolveWorktreeSelector(worktreeSelector)).id
        : null
      const snapshots = targetWorktreeId
        ? [this.getMobileSessionTabsForWorktree(targetWorktreeId)]
        : await this.listAllMobileSessionTabs()
      for (const snapshot of snapshots) {
        const activeTerminal = snapshot.tabs.find(
          (tab) =>
            tab.type === 'terminal' &&
            tab.isActive &&
            tab.status === 'ready' &&
            typeof tab.terminal === 'string'
        )
        if (activeTerminal?.type === 'terminal' && activeTerminal.terminal) {
          return activeTerminal.terminal
        }
      }
      const listed = await this.listTerminals(worktreeSelector)
      const first = listed.terminals[0]?.handle
      if (first) {
        return first
      }
      throw new Error('no_active_terminal')
    }
    this.assertGraphReady()

    const targetWorktreeId = worktreeSelector
      ? (await this.resolveWorktreeSelector(worktreeSelector)).id
      : null

    // Prefer the tab's activeLeafId — this is the pane the user last focused
    for (const tab of this.graph.tabs.values()) {
      if (targetWorktreeId && tab.worktreeId !== targetWorktreeId) {
        continue
      }
      if (!tab.activeLeafId) {
        continue
      }
      const leafKey = this.getLeafKey(tab.tabId, tab.activeLeafId)
      const leaf = this.graph.leaves.get(leafKey)
      if (leaf) {
        return this.issueHandle(leaf)
      }
    }

    // Fallback: any leaf in the target worktree
    for (const leaf of this.graph.leaves.values()) {
      if (targetWorktreeId && leaf.worktreeId !== targetWorktreeId) {
        continue
      }
      return this.issueHandle(leaf)
    }

    throw new Error('no_active_terminal')
  }

  // Why: orchestration records the pane key as the remint-stable assignee
  // identity at dispatch time; null (best-effort) rather than throwing so
  // dispatch still works for handles without a resolvable pane.
  getTerminalPaneKey(handle: string): string | null {
    return this.getPaneKeyForTerminalHandle(handle)
  }

  resolveTerminalPane(paneKey: string): RuntimeTerminalResolvePane {
    // Why: the renderer context menu only knows the stable pane key; main owns
    // the runtime terminal handle that agents and CLI commands can address.
    const handle = this.getTerminalHandleForPaneKey(paneKey)
    if (!handle) {
      throw new Error('terminal_not_found')
    }
    const record = this.graph.handles.get(handle)
    const parsed = parsePaneKey(paneKey)
    return {
      handle,
      tabId: record?.tabId ?? parsed?.tabId ?? '',
      leafId: record?.leafId ?? parsed?.leafId ?? '',
      ptyId: record?.ptyId ?? null
    }
  }

  async showTerminal(handle: string): Promise<RuntimeTerminalShow> {
    const pty = this.getLivePtyForHandle(handle)
    if (pty) {
      const worktreesById = await this.getResolvedWorktreeMap()
      return {
        ...this.buildPtyTerminalSummary(pty.pty, worktreesById),
        tabId: pty.pty.tabId ?? pty.record.tabId,
        leafId: parsePaneKey(pty.pty.paneKey ?? '')?.leafId ?? pty.record.leafId,
        paneRuntimeId: -1,
        ptyId: pty.pty.ptyId,
        rendererGraphEpoch: this.graph.rendererGraphEpoch
      }
    }
    const graphEpoch = this.captureReadyGraphEpoch()
    const worktreesById = await this.getResolvedWorktreeMap()
    this.assertStableReadyGraph(graphEpoch)
    const { leaf } = this.getLiveLeafForHandle(handle)
    const summary = this.buildTerminalSummary(leaf, worktreesById)
    return {
      ...summary,
      paneRuntimeId: leaf.paneRuntimeId,
      ptyId: leaf.ptyId,
      rendererGraphEpoch: this.graph.rendererGraphEpoch
    }
  }

  async readTerminal(
    handle: string,
    opts: { cursor?: number; limit?: number } = {}
  ): Promise<RuntimeTerminalRead> {
    const pty = this.getLivePtyForHandle(handle)
    if (pty) {
      const read = this.readPtyTerminal(handle, pty.pty, opts)
      return this.withVisibleSnapshotFallback(pty.pty.ptyId, read, opts)
    }

    const { leaf } = this.getLiveLeafForHandle(handle)
    const read = readTerminalTail({
      handle,
      status: getTerminalState(leaf),
      completedLines: leaf.tailBuffer,
      partialLine: leaf.tailPartialLine,
      completedLineCount: leaf.tailLinesTotal,
      bufferTruncated: leaf.tailTruncated,
      cursor: opts.cursor,
      limit: opts.limit
    })
    return leaf.ptyId ? this.withVisibleSnapshotFallback(leaf.ptyId, read, opts) : read
  }

  async sendTerminal(
    handle: string,
    action: {
      text?: string
      enter?: boolean
      interrupt?: boolean
    },
    options: {
      beforeWrite?: (ptyId: string) => void | Promise<void>
      suffixFailureError?: string
    } = {}
  ): Promise<RuntimeTerminalSend> {
    const pty = this.getLivePtyForHandle(handle)
    if (pty) {
      if (!pty.pty.connected) {
        throw new Error('terminal_not_writable')
      }
      const payload = buildSendPayload(action)
      if (payload === null) {
        throw new Error('invalid_terminal_send')
      }
      await assertTerminalInputWithinLimitWithYield(action.text)
      await this.writeTerminalAction(pty.pty.ptyId, action, payload, options)
      return {
        handle,
        accepted: true,
        bytesWritten: Buffer.byteLength(payload, 'utf8')
      }
    }

    const { leaf } = this.getLiveLeafForHandle(handle)
    if (!leaf.writable || !leaf.ptyId) {
      throw new Error('terminal_not_writable')
    }
    const payload = buildSendPayload(action)
    if (payload === null) {
      throw new Error('invalid_terminal_send')
    }
    await assertTerminalInputWithinLimitWithYield(action.text)

    await this.writeTerminalAction(leaf.ptyId, action, payload, options)

    return {
      handle,
      accepted: true,
      bytesWritten: Buffer.byteLength(payload, 'utf8')
    }
  }

  async sendTerminalAgentPrompt(
    handle: string,
    prompt: string,
    options: {
      beforeWrite?: (ptyId: string) => void | Promise<void>
      suffixFailureError?: string
    } = {}
  ): Promise<RuntimeTerminalSend> {
    const payload = buildAgentPromptPasteBytes(prompt)
    const bytesWritten = Buffer.byteLength(`${payload}${AGENT_PROMPT_SUBMIT}`, 'utf8')
    const pty = this.getLivePtyForHandle(handle)
    if (pty) {
      if (!pty.pty.connected) {
        throw new Error('terminal_not_writable')
      }
      await assertTerminalInputWithinLimitWithYield(payload)
      await this.writeTerminalAgentPrompt(pty.pty.ptyId, payload, options)
      return { handle, accepted: true, bytesWritten }
    }

    const { leaf } = this.getLiveLeafForHandle(handle)
    if (!leaf.writable || !leaf.ptyId) {
      throw new Error('terminal_not_writable')
    }
    await assertTerminalInputWithinLimitWithYield(payload)
    await this.writeTerminalAgentPrompt(leaf.ptyId, payload, options)
    return { handle, accepted: true, bytesWritten }
  }

  async getTerminalAgentStatus(handle: string): Promise<RuntimeTerminalAgentStatus> {
    const ptyId = this.getTerminalAgentStatusPtyId(handle)
    const terminal = this.getTerminalAgentStatusSnapshot(handle, ptyId)
    const explicitStatus = this.getFreshExplicitAgentStatusForHandle(handle)
    const blockedByWaitText = detectTerminalWaitBlockedReason(terminal.waitText)
    const liveTitleClearsBlockedText =
      terminal.titleStatusIsLive &&
      terminal.titleStatus !== null &&
      terminal.titleStatus !== 'permission'
    if (terminal.titleStatus === 'permission' && terminal.titleStatusIsLive) {
      return { handle, isRunningAgent: true, status: 'permission' }
    }
    if (
      blockedByWaitText &&
      !liveTitleClearsBlockedText &&
      (!explicitStatus ||
        explicitStatus.status === 'permission' ||
        (terminal.waitBlockedAt !== null && terminal.waitBlockedAt >= explicitStatus.updatedAt))
    ) {
      return { handle, isRunningAgent: true, status: 'permission' }
    }
    if (explicitStatus) {
      // Why: permission titles can linger after hooks report the agent resumed.
      // Fresh hook state is tighter, but current shell/management evidence wins.
      const isRunningAgent =
        !terminalTitleBlocksExplicitAgentStatus(terminal.title) &&
        !(await this.terminalHasShellForegroundProcess(handle, ptyId))
      this.assertTerminalAgentStatusPtyBinding(handle, ptyId)
      return {
        handle,
        isRunningAgent,
        status: isRunningAgent ? explicitStatus.status : null
      }
    }
    if (terminal.titleStatus) {
      return { handle, isRunningAgent: true, status: terminal.titleStatus }
    }

    const isRunningAgent = await this.isTerminalRunningAgent(handle)
    this.assertTerminalAgentStatusPtyBinding(handle, ptyId)
    return { handle, isRunningAgent, status: null }
  }

  private getTerminalAgentStatusPtyId(handle: string): string {
    const pty = this.getLivePtyForHandle(handle)
    if (pty) {
      if (!pty.pty.connected) {
        throw new Error('terminal_gone')
      }
      return pty.pty.ptyId
    }
    const { leaf } = this.getLiveLeafForHandle(handle)
    if (getTerminalState(leaf) !== 'running') {
      throw new Error('terminal_exited')
    }
    if (!leaf.ptyId) {
      throw new Error('terminal_gone')
    }
    return leaf.ptyId
  }

  private assertTerminalAgentStatusPtyBinding(handle: string, expectedPtyId: string): void {
    if (this.getTerminalAgentStatusPtyId(handle) === expectedPtyId) {
      return
    }
    // Why: delayed process evidence belongs only to the PTY that started the
    // read, while callers still rely on the established stale-handle contract.
    throw new Error('terminal_handle_stale')
  }

  private getTerminalAgentStatusSnapshot(
    handle: string,
    expectedPtyId: string
  ): {
    waitText: string
    waitBlockedAt: number | null
    title: string | null
    titleStatus: AgentStatus | null
    titleStatusIsLive: boolean
  } {
    const pty = this.getLivePtyForHandle(handle)
    if (pty) {
      if (!pty.pty.connected || pty.pty.ptyId !== expectedPtyId) {
        throw new Error('terminal_not_writable')
      }
      const leaf = this.getPrimaryLeafForPty(pty.pty.ptyId)
      const leafTitle = leaf
        ? getLatestAgentCandidateTitleInfo(
            { title: leaf.paneTitle, updatedAt: leaf.paneTitleUpdatedAt },
            { title: leaf.lastOscTitle, updatedAt: leaf.lastOscTitleAt }
          )
        : null
      const ptyTitle =
        leafTitle ??
        getLatestAgentCandidateTitleInfo(
          { title: pty.pty.title, updatedAt: pty.pty.titleUpdatedAt },
          { title: pty.pty.lastOscTitle, updatedAt: pty.pty.lastOscTitleAt }
        )
      const waitText = buildTerminalWaitText(
        pty.pty.tailBuffer,
        pty.pty.tailPartialLine,
        pty.pty.preview
      )
      return {
        waitText,
        waitBlockedAt: pty.pty.waitBlockedAt,
        title: ptyTitle?.title ?? null,
        titleStatus: ptyTitle
          ? detectAgentStatusFromTitle(ptyTitle.title)
          : pty.pty.lastAgentStatus,
        titleStatusIsLive: ptyTitle !== null
      }
    }

    const { leaf } = this.getLiveLeafForHandle(handle)
    if (getTerminalState(leaf) !== 'running') {
      throw new Error('terminal_exited')
    }
    if (!leaf.ptyId) {
      throw new Error('terminal_gone')
    }
    if (leaf.ptyId !== expectedPtyId) {
      throw new Error('terminal_not_writable')
    }
    const title = getLatestAgentCandidateTitleInfo(
      { title: leaf.paneTitle, updatedAt: leaf.paneTitleUpdatedAt },
      { title: leaf.lastOscTitle, updatedAt: leaf.lastOscTitleAt },
      { title: this.graph.tabs.get(leaf.tabId)?.title, updatedAt: 0 }
    )
    return {
      waitText: buildTerminalWaitText(leaf.tailBuffer, leaf.tailPartialLine, leaf.preview),
      waitBlockedAt: leaf.waitBlockedAt,
      title: title?.title ?? null,
      titleStatus: title ? detectAgentStatusFromTitle(title.title) : leaf.lastAgentStatus,
      titleStatusIsLive: (title?.updatedAt ?? 0) > 0
    }
  }

  private async terminalHasShellForegroundProcess(handle: string, ptyId: string): Promise<boolean> {
    if (!this.ptyController) {
      return false
    }
    let foregroundProcess: string | null
    try {
      foregroundProcess = await this.ptyController.getForegroundProcess(ptyId)
    } catch {
      this.assertTerminalAgentStatusPtyBinding(handle, ptyId)
      return false
    }
    this.assertTerminalAgentStatusPtyBinding(handle, ptyId)
    if (!foregroundProcess || !isShellProcess(foregroundProcess)) {
      return false
    }
    const confirmationController = this.ptyController
    if (!confirmationController?.confirmForegroundProcess) {
      return true
    }
    let confirmedProcess: string | null
    try {
      confirmedProcess = await confirmationController.confirmForegroundProcess(ptyId)
    } catch {
      this.assertTerminalAgentStatusPtyBinding(handle, ptyId)
      return true
    }
    this.assertTerminalAgentStatusPtyBinding(handle, ptyId)
    // Why: hook identity is generic; strong provider evidence only needs to
    // prove that some recognized agent still owns this exact PTY.
    return recognizeAgentProcess(confirmedProcess) === null
  }

  private shouldDelayPtyBackedMobileSnapshotForForegroundAgent(
    pty: RuntimePtyWorktreeRecord,
    title: string
  ): boolean {
    return (
      !pty.launchAgent && pty.foregroundAgent === null && hasCompatibleAgentTitleIdentity(title)
    )
  }

  /**
   * Schedules an asynchronous query to check which agent process is currently
   * running in the foreground of a PTY.
   */
  private readonly ptyForegroundAgentRefreshCommands = new RuntimePtyForegroundAgentRefreshCommands(
    {
      getGraph: () => this.graph,
      getPtyController: () => this.ptyController,
      touchMobileSessionSnapshotsForPty: (ptyId, options) =>
        this.touchMobileSessionSnapshotsForPty(ptyId, options)
    }
  )

  refreshPtyForegroundAgent: RuntimePtyForegroundAgentRefreshCommands['refreshPtyForegroundAgent'] =
    this.ptyForegroundAgentRefreshCommands.refreshPtyForegroundAgent.bind(
      this.ptyForegroundAgentRefreshCommands
    )
  getPendingForegroundAgentRefreshForTitle: RuntimePtyForegroundAgentRefreshCommands['getPendingForegroundAgentRefreshForTitle'] =
    this.ptyForegroundAgentRefreshCommands.getPendingForegroundAgentRefreshForTitle.bind(
      this.ptyForegroundAgentRefreshCommands
    )
  delayPtyBackedMobileSnapshotForForegroundAgent: RuntimePtyForegroundAgentRefreshCommands['delayPtyBackedMobileSnapshotForForegroundAgent'] =
    this.ptyForegroundAgentRefreshCommands.delayPtyBackedMobileSnapshotForForegroundAgent.bind(
      this.ptyForegroundAgentRefreshCommands
    )
  refreshPtyForegroundAgentFromController: RuntimePtyForegroundAgentRefreshCommands['refreshPtyForegroundAgentFromController'] =
    this.ptyForegroundAgentRefreshCommands.refreshPtyForegroundAgentFromController.bind(
      this.ptyForegroundAgentRefreshCommands
    )

  private getFreshExplicitAgentStatusForHandle(handle: string): {
    status: NonNullable<RuntimeTerminalAgentStatus['status']>
    updatedAt: number
  } | null {
    const paneKey = this.getPaneKeyForTerminalHandle(handle)
    const now = Date.now()
    let bestStatus: NonNullable<RuntimeTerminalAgentStatus['status']> | null = null
    let bestUpdatedAt = -1

    const consider = (
      state: AgentStatusEntry['state'] | undefined,
      updatedAt: number | null | undefined
    ): void => {
      if (!state) {
        return
      }
      if (typeof updatedAt !== 'number' || now - updatedAt > AGENT_STATUS_STALE_AFTER_MS) {
        return
      }
      const status = mapExplicitAgentStateToRuntimeTerminalStatus(state)
      // Why: older retained permission rows can remain visible after the agent
      // resumes. Prefer the newest explicit state; only let permission win ties.
      if (updatedAt > bestUpdatedAt || (updatedAt === bestUpdatedAt && status === 'permission')) {
        bestStatus = status
        bestUpdatedAt = updatedAt
      }
    }

    if (paneKey) {
      const retained = this.latestAgentStatusByPaneKey.get(paneKey)
      consider(retained?.payload.state, retained?.updatedAt)
    }

    for (const entry of this.getAgentStatusSnapshotFn?.() ?? []) {
      if (entry.terminalHandle !== handle && (!paneKey || entry.paneKey !== paneKey)) {
        continue
      }
      consider(entry.state, entry.receivedAt)
    }

    return bestStatus ? { status: bestStatus, updatedAt: bestUpdatedAt } : null
  }

  private async writeTerminalAction(
    ptyId: string,
    action: { text?: string; enter?: boolean; interrupt?: boolean },
    payload: string,
    options: {
      beforeWrite?: (ptyId: string) => void | Promise<void>
      suffixFailureError?: string
    } = {}
  ): Promise<void> {
    // Why: direct terminal.send can carry paste-sized text from RPC/mobile
    // clients; chunk text before PTY/ConPTY while preserving suffix separation.
    const hasText = typeof action.text === 'string' && action.text.length > 0
    const hasSuffix = action.enter || action.interrupt
    if (hasText) {
      await this.writeTerminalInputChunks(ptyId, action.text!, options)
    }
    if (hasSuffix) {
      const suffix = (action.enter ? '\r' : '') + (action.interrupt ? '\x03' : '')
      if (hasText) {
        await new Promise((resolve) => setTimeout(resolve, 500))
      }
      try {
        await options.beforeWrite?.(ptyId)
      } catch (error) {
        if (options.suffixFailureError) {
          throw new Error(options.suffixFailureError)
        }
        throw error
      }
      const suffixWrote = this.ptyController?.write(ptyId, suffix) ?? false
      if (!suffixWrote) {
        throw new Error(options.suffixFailureError ?? 'terminal_not_writable')
      }
      return
    }
    if (hasText) {
      return
    }

    await options.beforeWrite?.(ptyId)
    const wrote = this.ptyController?.write(ptyId, payload) ?? false
    if (!wrote) {
      throw new Error('terminal_not_writable')
    }
  }

  private async writeTerminalInputChunks(
    ptyId: string,
    text: string,
    options: {
      beforeWrite?: (ptyId: string) => void | Promise<void>
    } = {}
  ): Promise<void> {
    const chunks = iterateTerminalInputChunks(text)
    let chunk = chunks.next()
    while (!chunk.done) {
      await options.beforeWrite?.(ptyId)
      const wrote = this.ptyController?.write(ptyId, chunk.value) ?? false
      if (!wrote) {
        throw new Error('terminal_not_writable')
      }
      chunk = chunks.next()
      if (!chunk.done) {
        await new Promise((resolve) => setTimeout(resolve, 0))
      }
    }
  }

  private async writeTerminalAgentPrompt(
    ptyId: string,
    pastePayload: string,
    options: {
      beforeWrite?: (ptyId: string) => void | Promise<void>
      suffixFailureError?: string
    } = {}
  ): Promise<void> {
    let wrotePasteBytes = false
    let completedPaste = false
    try {
      const chunks = iterateTerminalInputChunks(pastePayload)
      let chunk = chunks.next()
      while (!chunk.done) {
        await options.beforeWrite?.(ptyId)
        const wrote = this.ptyController?.write(ptyId, chunk.value) ?? false
        if (!wrote) {
          throw new Error('terminal_not_writable')
        }
        wrotePasteBytes = true
        chunk = chunks.next()
        if (!chunk.done) {
          await new Promise((resolve) => setTimeout(resolve, 0))
        }
      }
      completedPaste = true
    } catch (error) {
      if (wrotePasteBytes && !completedPaste) {
        this.ptyController?.write(ptyId, AGENT_PROMPT_BRACKETED_PASTE_END)
      }
      throw error
    }

    await new Promise((resolve) => setTimeout(resolve, AGENT_PROMPT_SUBMIT_DELAY_MS))
    try {
      await options.beforeWrite?.(ptyId)
    } catch (error) {
      if (options.suffixFailureError) {
        throw new Error(options.suffixFailureError)
      }
      throw error
    }
    const suffixWrote = this.ptyController?.write(ptyId, AGENT_PROMPT_SUBMIT) ?? false
    if (!suffixWrote) {
      throw new Error(options.suffixFailureError ?? 'terminal_not_writable')
    }
  }

  async waitForTerminal(
    handle: string,
    options?: {
      condition?: RuntimeTerminalWaitCondition
      timeoutMs?: number
      signal?: AbortSignal
    }
  ): Promise<RuntimeTerminalWait> {
    const condition = options?.condition ?? 'exit'
    const pty = this.getLivePtyForHandle(handle)
    if (pty) {
      if (condition === 'exit' && !pty.pty.connected) {
        return buildPtyTerminalWaitResult(handle, condition, pty.pty)
      }
      const ptyWaitText = buildTerminalWaitText(
        pty.pty.tailBuffer,
        pty.pty.tailPartialLine,
        pty.pty.preview
      )
      const ptyBlockedReason = detectTerminalWaitBlockedReason(ptyWaitText)
      if (condition === 'tui-idle' && ptyBlockedReason) {
        return buildPtyTerminalWaitBlockedResult(handle, condition, pty.pty, ptyBlockedReason)
      }
      if (condition === 'tui-idle' && pty.pty.lastAgentStatus === 'idle') {
        return buildPtyTerminalWaitResult(handle, condition, pty.pty)
      }
      if (
        condition === 'tui-idle' &&
        (this.getAdoptedPtyExplicitIdleStatus(pty.pty) === 'idle' ||
          isKnownReadyPromptPreview(ptyWaitText))
      ) {
        return buildPtyTerminalWaitResult(handle, condition, pty.pty)
      }
      return await new Promise<RuntimeTerminalWait>((resolve, reject) => {
        const effectiveTimeoutMs =
          typeof options?.timeoutMs === 'number' && options.timeoutMs > 0
            ? options.timeoutMs
            : condition === 'tui-idle'
              ? TUI_IDLE_DEFAULT_TIMEOUT_MS
              : 0
        const waiter: TerminalWaiter = {
          handle,
          condition,
          resolve,
          reject,
          timeout: null,
          pollInterval: null,
          abortCleanup: null
        }
        if (!this.bindTerminalWaiterAbort(waiter, options?.signal)) {
          reject(new Error('request_aborted'))
          return
        }
        if (effectiveTimeoutMs > 0) {
          waiter.timeout = setTimeout(() => {
            this.removeWaiter(waiter)
            reject(new Error('timeout'))
          }, effectiveTimeoutMs)
        }
        let waiters = this.graph.waitersByHandle.get(handle)
        if (!waiters) {
          waiters = new Set()
          this.graph.waitersByHandle.set(handle, waiters)
        }
        waiters.add(waiter)
        const live = this.getLivePtyForHandle(handle)
        if (!live) {
          this.removeWaiter(waiter)
          reject(new Error('terminal_handle_stale'))
        } else if (condition === 'exit' && !live.pty.connected) {
          this.resolveWaiter(waiter, buildPtyTerminalWaitResult(handle, condition, live.pty))
        } else if (condition === 'tui-idle') {
          const livePtyWaitText = buildTerminalWaitText(
            live.pty.tailBuffer,
            live.pty.tailPartialLine,
            live.pty.preview
          )
          const blockedReason = detectTerminalWaitBlockedReason(livePtyWaitText)
          if (blockedReason) {
            this.resolveWaiter(
              waiter,
              buildPtyTerminalWaitBlockedResult(handle, condition, live.pty, blockedReason)
            )
          } else if (live.pty.lastAgentStatus === 'idle') {
            this.resolveWaiter(waiter, buildPtyTerminalWaitResult(handle, condition, live.pty))
          } else if (
            this.getAdoptedPtyExplicitIdleStatus(live.pty) === 'idle' ||
            isKnownReadyPromptPreview(livePtyWaitText)
          ) {
            this.resolveWaiter(waiter, buildPtyTerminalWaitResult(handle, condition, live.pty))
          } else {
            this.startPtyTuiIdleFallbackPoll(waiter, live.pty)
          }
        }
      })
    }
    const { leaf } = this.getLiveLeafForHandle(handle)

    if (condition === 'exit' && getTerminalState(leaf) === 'exited') {
      return buildTerminalWaitResult(handle, condition, leaf)
    }

    const leafWaitText = buildTerminalWaitText(leaf.tailBuffer, leaf.tailPartialLine, leaf.preview)
    const leafBlockedReason = detectTerminalWaitBlockedReason(leafWaitText)
    if (condition === 'tui-idle' && leafBlockedReason) {
      return buildTerminalWaitBlockedResult(handle, condition, leaf, leafBlockedReason)
    }

    // Why: if the agent already transitioned to idle (or permission) before the
    // waiter was registered, resolve immediately. This uses the same OSC title
    // detection that powers the renderer's "Task complete" notifications.
    // Why: only 'idle' satisfies tui-idle, not 'permission'. Permission means the
    // agent is blocked on user approval, not finished with its task.
    if (condition === 'tui-idle' && leaf.lastAgentStatus === 'idle') {
      return buildTerminalWaitResult(handle, condition, leaf)
    }
    if (condition === 'tui-idle') {
      const fastPathTitle = leaf.paneTitle ?? this.graph.tabs.get(leaf.tabId)?.title
      if (
        (fastPathTitle && detectExplicitIdleStatusFromTitle(fastPathTitle) === 'idle') ||
        isKnownReadyPromptPreview(leafWaitText)
      ) {
        return buildTerminalWaitResult(handle, condition, leaf)
      }
    }

    return await new Promise<RuntimeTerminalWait>((resolve, reject) => {
      // Why: tui-idle depends on OSC title transitions from a recognized agent.
      // If no agent is detected, the waiter would hang forever. Enforce a default
      // timeout so unsupported CLIs fail predictably instead of silently blocking.
      const effectiveTimeoutMs =
        typeof options?.timeoutMs === 'number' && options.timeoutMs > 0
          ? options.timeoutMs
          : condition === 'tui-idle'
            ? TUI_IDLE_DEFAULT_TIMEOUT_MS
            : 0

      const waiter: TerminalWaiter = {
        handle,
        condition,
        resolve,
        reject,
        timeout: null,
        pollInterval: null,
        abortCleanup: null
      }

      if (!this.bindTerminalWaiterAbort(waiter, options?.signal)) {
        reject(new Error('request_aborted'))
        return
      }

      if (effectiveTimeoutMs > 0) {
        waiter.timeout = setTimeout(() => {
          this.removeWaiter(waiter)
          reject(new Error('timeout'))
        }, effectiveTimeoutMs)
      }

      let waiters = this.graph.waitersByHandle.get(handle)
      if (!waiters) {
        waiters = new Set()
        this.graph.waitersByHandle.set(handle, waiters)
      }
      waiters.add(waiter)

      // Why: the handle may go stale or exit in the small gap between the first
      // validation and waiter registration. Re-checking here keeps wait --for
      // exit honest instead of hanging on a terminal that already changed.
      try {
        const live = this.getLiveLeafForHandle(handle)
        if (getTerminalState(live.leaf) === 'exited') {
          this.resolveWaiter(waiter, buildTerminalWaitResult(handle, condition, live.leaf))
        } else if (condition === 'tui-idle') {
          const liveLeafWaitText = buildTerminalWaitText(
            live.leaf.tailBuffer,
            live.leaf.tailPartialLine,
            live.leaf.preview
          )
          const blockedReason = detectTerminalWaitBlockedReason(liveLeafWaitText)
          if (blockedReason) {
            this.resolveWaiter(
              waiter,
              buildTerminalWaitBlockedResult(handle, condition, live.leaf, blockedReason)
            )
          } else if (live.leaf.lastAgentStatus === 'idle') {
            // Why: don't clear lastAgentStatus here. It's a factual record of the
            // last detected OSC state, not a one-shot signal. Clearing it causes
            // subsequent tui-idle waiters to hang even though the agent is idle —
            // the first waiter consumes the status and all later ones see null.
            this.resolveWaiter(waiter, buildTerminalWaitResult(handle, condition, live.leaf))
          } else {
            // Why: renderer-synced previews can show a known ready prompt even
            // while the last OSC title is still "working"; keep polling the
            // preview/title until the waiter resolves or hits its timeout.
            const fastPathTitle = live.leaf.paneTitle ?? this.graph.tabs.get(live.leaf.tabId)?.title
            if (
              (fastPathTitle && detectExplicitIdleStatusFromTitle(fastPathTitle) === 'idle') ||
              isKnownReadyPromptPreview(liveLeafWaitText)
            ) {
              this.resolveWaiter(waiter, buildTerminalWaitResult(handle, condition, live.leaf))
            } else {
              this.startTuiIdleFallbackPoll(waiter, live.leaf)
            }
          }
        }
      } catch (error) {
        this.removeWaiter(waiter)
        reject(error instanceof Error ? error : new Error(String(error)))
      }
    })
  }

  private readonly worktreePsCommands = new RuntimeWorktreePsCommands({
    getStore: () => this.store,
    getGraph: () => this.graph,
    listResolvedWorktrees: () => this.listResolvedWorktrees(),
    isRuntimeWorktreeVisible: (worktree) => this.isRuntimeWorktreeVisible(worktree),
    refreshPtyWorktreeRecordsFromController: (resolvedWorktrees, targetWorktreeId) =>
      this.refreshPtyWorktreeRecordsFromController(resolvedWorktrees, targetWorktreeId),
    getAgentLaunchPlatformForRepo: (repo) => this.getAgentLaunchPlatformForRepo(repo),
    getSummaryForRuntimeWorktreeId: (summaries, resolvedWorktrees, runtimeWorktreeId) =>
      this.getSummaryForRuntimeWorktreeId(summaries, resolvedWorktrees, runtimeWorktreeId),
    getLatestAgentStatusByPaneKey: () => this.latestAgentStatusByPaneKey,
    getAgentStatusSnapshot: () => this.getAgentStatusSnapshotFn?.() ?? [],
    buildAgentOrchestrationByPaneKey: () => this.buildAgentOrchestrationByPaneKey()
  })

  getWorktreePs: RuntimeWorktreePsCommands['getWorktreePs'] =
    this.worktreePsCommands.getWorktreePs.bind(this.worktreePsCommands)

  private readonly projectGroupsCommands = new RuntimeProjectGroupsCommands({
    getStore: () => this.store,
    resolveRepoSelector: (selector) => this.resolveRepoSelector(selector),
    notifyReposChanged: () => this.notifyReposChanged(),
    invalidateResolvedWorktreeCache: () => this.invalidateResolvedWorktreeCache(),
    addRepo: (path, kind, executionHostId) => this.addRepo(path, kind, executionHostId),
    cloneRepo: (url, destination, executionHostId) =>
      this.cloneRepo(url, destination, executionHostId)
  })

  listRepos: RuntimeProjectGroupsCommands['listRepos'] = this.projectGroupsCommands.listRepos.bind(
    this.projectGroupsCommands
  )
  enrichMissingRepoGitRemoteIdentities: RuntimeProjectGroupsCommands['enrichMissingRepoGitRemoteIdentities'] =
    this.projectGroupsCommands.enrichMissingRepoGitRemoteIdentities.bind(this.projectGroupsCommands)
  listProjects: RuntimeProjectGroupsCommands['listProjects'] =
    this.projectGroupsCommands.listProjects.bind(this.projectGroupsCommands)
  updateProject: RuntimeProjectGroupsCommands['updateProject'] =
    this.projectGroupsCommands.updateProject.bind(this.projectGroupsCommands)
  listProjectHostSetups: RuntimeProjectGroupsCommands['listProjectHostSetups'] =
    this.projectGroupsCommands.listProjectHostSetups.bind(this.projectGroupsCommands)
  createProjectHostSetup: RuntimeProjectGroupsCommands['createProjectHostSetup'] =
    this.projectGroupsCommands.createProjectHostSetup.bind(this.projectGroupsCommands)
  setupProjectExistingFolder: RuntimeProjectGroupsCommands['setupProjectExistingFolder'] =
    this.projectGroupsCommands.setupProjectExistingFolder.bind(this.projectGroupsCommands)
  setupProjectClone: RuntimeProjectGroupsCommands['setupProjectClone'] =
    this.projectGroupsCommands.setupProjectClone.bind(this.projectGroupsCommands)
  updateProjectHostSetup: RuntimeProjectGroupsCommands['updateProjectHostSetup'] =
    this.projectGroupsCommands.updateProjectHostSetup.bind(this.projectGroupsCommands)
  deleteProjectHostSetup: RuntimeProjectGroupsCommands['deleteProjectHostSetup'] =
    this.projectGroupsCommands.deleteProjectHostSetup.bind(this.projectGroupsCommands)
  listProjectGroups: RuntimeProjectGroupsCommands['listProjectGroups'] =
    this.projectGroupsCommands.listProjectGroups.bind(this.projectGroupsCommands)
  listFolderWorkspaces: RuntimeProjectGroupsCommands['listFolderWorkspaces'] =
    this.projectGroupsCommands.listFolderWorkspaces.bind(this.projectGroupsCommands)
  createProjectGroup: RuntimeProjectGroupsCommands['createProjectGroup'] =
    this.projectGroupsCommands.createProjectGroup.bind(this.projectGroupsCommands)
  updateProjectGroup: RuntimeProjectGroupsCommands['updateProjectGroup'] =
    this.projectGroupsCommands.updateProjectGroup.bind(this.projectGroupsCommands)
  deleteProjectGroup: RuntimeProjectGroupsCommands['deleteProjectGroup'] =
    this.projectGroupsCommands.deleteProjectGroup.bind(this.projectGroupsCommands)
  moveProjectToGroup: RuntimeProjectGroupsCommands['moveProjectToGroup'] =
    this.projectGroupsCommands.moveProjectToGroup.bind(this.projectGroupsCommands)
  createFolderWorkspace: RuntimeProjectGroupsCommands['createFolderWorkspace'] =
    this.projectGroupsCommands.createFolderWorkspace.bind(this.projectGroupsCommands)
  getFolderWorkspacePathStatus: RuntimeProjectGroupsCommands['getFolderWorkspacePathStatus'] =
    this.projectGroupsCommands.getFolderWorkspacePathStatus.bind(this.projectGroupsCommands)
  updateFolderWorkspace: RuntimeProjectGroupsCommands['updateFolderWorkspace'] =
    this.projectGroupsCommands.updateFolderWorkspace.bind(this.projectGroupsCommands)
  deleteFolderWorkspace: RuntimeProjectGroupsCommands['deleteFolderWorkspace'] =
    this.projectGroupsCommands.deleteFolderWorkspace.bind(this.projectGroupsCommands)
  scanNestedRepos: RuntimeProjectGroupsCommands['scanNestedRepos'] =
    this.projectGroupsCommands.scanNestedRepos.bind(this.projectGroupsCommands)
  browseServerDir: RuntimeProjectGroupsCommands['browseServerDir'] =
    this.projectGroupsCommands.browseServerDir.bind(this.projectGroupsCommands)
  isGitAvailable: RuntimeProjectGroupsCommands['isGitAvailable'] =
    this.projectGroupsCommands.isGitAvailable.bind(this.projectGroupsCommands)
  importNestedRepos: RuntimeProjectGroupsCommands['importNestedRepos'] =
    this.projectGroupsCommands.importNestedRepos.bind(this.projectGroupsCommands)
  listSparsePresets: RuntimeProjectGroupsCommands['listSparsePresets'] =
    this.projectGroupsCommands.listSparsePresets.bind(this.projectGroupsCommands)
  saveSparsePreset: RuntimeProjectGroupsCommands['saveSparsePreset'] =
    this.projectGroupsCommands.saveSparsePreset.bind(this.projectGroupsCommands)

  private readonly repoLifecycleCommands = new RuntimeRepoLifecycleCommands({
    getStore: () => this.store,
    notifyReposChanged: () => this.notifyReposChanged(),
    invalidateResolvedWorktreeCache: () => this.invalidateResolvedWorktreeCache(),
    resolveRepoSelector: (selector) => this.resolveRepoSelector(selector)
  })

  addRepo: RuntimeRepoLifecycleCommands['addRepo'] = this.repoLifecycleCommands.addRepo.bind(
    this.repoLifecycleCommands
  )
  createRepo: RuntimeRepoLifecycleCommands['createRepo'] =
    this.repoLifecycleCommands.createRepo.bind(this.repoLifecycleCommands)
  cloneRepo: RuntimeRepoLifecycleCommands['cloneRepo'] = this.repoLifecycleCommands.cloneRepo.bind(
    this.repoLifecycleCommands
  )
  showRepo: RuntimeRepoLifecycleCommands['showRepo'] = this.repoLifecycleCommands.showRepo.bind(
    this.repoLifecycleCommands
  )
  setRepoBaseRef: RuntimeRepoLifecycleCommands['setRepoBaseRef'] =
    this.repoLifecycleCommands.setRepoBaseRef.bind(this.repoLifecycleCommands)
  updateRepo: RuntimeRepoLifecycleCommands['updateRepo'] =
    this.repoLifecycleCommands.updateRepo.bind(this.repoLifecycleCommands)
  removeProject: RuntimeRepoLifecycleCommands['removeProject'] =
    this.repoLifecycleCommands.removeProject.bind(this.repoLifecycleCommands)

  async inspectTerminalProcess(
    terminalSelector: string
  ): Promise<{ foregroundProcess: string | null; hasChildProcesses: boolean }> {
    const leaf = this.resolveLeafForHandle(terminalSelector)
    if (!leaf?.ptyId || !this.ptyController) {
      return { foregroundProcess: null, hasChildProcesses: false }
    }
    const foregroundProcess = await this.ptyController.getForegroundProcess(leaf.ptyId)
    const hasChildProcesses =
      (await this.ptyController.hasChildProcesses?.(leaf.ptyId).catch(() => false)) ?? false
    return { foregroundProcess, hasChildProcesses }
  }

  reorderRepos: RuntimeRepoLifecycleCommands['reorderRepos'] =
    this.repoLifecycleCommands.reorderRepos.bind(this.repoLifecycleCommands)
  searchRepoRefs: RuntimeRepoLifecycleCommands['searchRepoRefs'] =
    this.repoLifecycleCommands.searchRepoRefs.bind(this.repoLifecycleCommands)
  getRepoBaseRefDefault: RuntimeRepoLifecycleCommands['getRepoBaseRefDefault'] =
    this.repoLifecycleCommands.getRepoBaseRefDefault.bind(this.repoLifecycleCommands)

  private getHostedReviewExecutionOptions(
    repo: Repo
  ): { localGitExecOptions: { wslDistro?: string } } | undefined {
    const localGitOptions = this.getLocalGitExecutionOptionArgs(repo)[0] ?? {}
    return Object.keys(localGitOptions).length > 0
      ? { localGitExecOptions: localGitOptions }
      : undefined
  }

  private getLocalGitExecutionOptionArgs(repo: Repo): [] | [{ wslDistro?: string }] {
    const localGitOptions = getLocalProjectWorktreeGitOptions(this.requireStore(), repo)
    return Object.keys(localGitOptions).length > 0 ? [localGitOptions] : []
  }

  private getAgentLaunchPlatformForRepo(repo: Repo): NodeJS.Platform {
    const projectRuntime = repo.connectionId
      ? undefined
      : resolveLocalProjectRuntimeForRepo(this.requireStore(), repo)
    return getAgentLaunchPlatformForRepo(repo, projectRuntime)
  }

  private getAgentLaunchPlatformForWorkspace(scope: TerminalWorkspaceLaunchScope): NodeJS.Platform {
    if (scope.repo) {
      return this.getAgentLaunchPlatformForRepo(scope.repo)
    }
    if (scope.connectionId) {
      return isWindowsAbsolutePathLike(scope.path) ? 'win32' : 'linux'
    }
    return isWslUncPath(scope.path) ? 'linux' : process.platform
  }

  // Why: repos added before fork detection existed have no stored `upstream`, so
  // their avatar/badge would never self-correct. Resolve it once at startup for
  // local git repos; SSH repos resolve lazily when their settings open (their
  // connection may not be up yet). Sequential to respect the gh rate limit;
  // failures leave `upstream` unset so the next launch retries.
  private async backfillForkUpstreams(): Promise<void> {
    try {
      const store = this.requireStore()
      let changed = false
      for (const repo of store.getRepos()) {
        if (repo.upstream !== undefined || repo.kind === 'folder' || repo.connectionId) {
          continue
        }
        let upstream: { owner: string; repo: string } | null
        try {
          upstream = await getRepoUpstream(repo.path, null)
        } catch {
          continue
        }
        const updates: Partial<Repo> = { upstream: upstream ?? null }
        // Only migrate the auto-detected origin avatar; never touch a chosen icon.
        if (upstream && repo.repoIcon?.type === 'image' && repo.repoIcon.source === 'github') {
          updates.repoIcon = githubAvatarIcon(upstream)
        }
        store.updateRepo(repo.id, updates)
        changed = true
      }
      if (changed) {
        this.notifyReposChanged()
      }
    } catch {
      // Best-effort startup backfill; never disrupt launch.
    }
  }

  private readonly issueTrackingCommands = new RuntimeIssueTrackingCommands({
    getStore: () => this.store,
    getStats: () => this.stats,
    resolveRepoSelector: (selector) => this.resolveRepoSelector(selector),
    resolveWorktreeSelector: (selector) => this.resolveWorktreeSelector(selector),
    getLocalGitExecutionOptionArgs: (repo) => this.getLocalGitExecutionOptionArgs(repo),
    getHostedReviewExecutionOptions: (repo) => this.getHostedReviewExecutionOptions(repo)
  })

  addGitHubIssueCommentBySlug: RuntimeIssueTrackingCommands['addGitHubIssueCommentBySlug'] =
    this.issueTrackingCommands.addGitHubIssueCommentBySlug.bind(this.issueTrackingCommands)
  addGitLabRepoIssueComment: RuntimeIssueTrackingCommands['addGitLabRepoIssueComment'] =
    this.issueTrackingCommands.addGitLabRepoIssueComment.bind(this.issueTrackingCommands)
  addGitLabRepoMRComment: RuntimeIssueTrackingCommands['addGitLabRepoMRComment'] =
    this.issueTrackingCommands.addGitLabRepoMRComment.bind(this.issueTrackingCommands)
  addGitLabRepoMRInlineComment: RuntimeIssueTrackingCommands['addGitLabRepoMRInlineComment'] =
    this.issueTrackingCommands.addGitLabRepoMRInlineComment.bind(this.issueTrackingCommands)
  addRepoIssueComment: RuntimeIssueTrackingCommands['addRepoIssueComment'] =
    this.issueTrackingCommands.addRepoIssueComment.bind(this.issueTrackingCommands)
  addRepoPRReviewComment: RuntimeIssueTrackingCommands['addRepoPRReviewComment'] =
    this.issueTrackingCommands.addRepoPRReviewComment.bind(this.issueTrackingCommands)
  addRepoPRReviewCommentReply: RuntimeIssueTrackingCommands['addRepoPRReviewCommentReply'] =
    this.issueTrackingCommands.addRepoPRReviewCommentReply.bind(this.issueTrackingCommands)
  clearGitHubProjectItemField: RuntimeIssueTrackingCommands['clearGitHubProjectItemField'] =
    this.issueTrackingCommands.clearGitHubProjectItemField.bind(this.issueTrackingCommands)
  countRepoWorkItems: RuntimeIssueTrackingCommands['countRepoWorkItems'] =
    this.issueTrackingCommands.countRepoWorkItems.bind(this.issueTrackingCommands)
  createGitLabRepoIssue: RuntimeIssueTrackingCommands['createGitLabRepoIssue'] =
    this.issueTrackingCommands.createGitLabRepoIssue.bind(this.issueTrackingCommands)
  createHostedReview: RuntimeIssueTrackingCommands['createHostedReview'] =
    this.issueTrackingCommands.createHostedReview.bind(this.issueTrackingCommands)
  createRepoIssue: RuntimeIssueTrackingCommands['createRepoIssue'] =
    this.issueTrackingCommands.createRepoIssue.bind(this.issueTrackingCommands)
  deleteGitHubIssueCommentBySlug: RuntimeIssueTrackingCommands['deleteGitHubIssueCommentBySlug'] =
    this.issueTrackingCommands.deleteGitHubIssueCommentBySlug.bind(this.issueTrackingCommands)
  diagnoseGitLabAuth: RuntimeIssueTrackingCommands['diagnoseGitLabAuth'] =
    this.issueTrackingCommands.diagnoseGitLabAuth.bind(this.issueTrackingCommands)
  getGitHubProjectViewTable: RuntimeIssueTrackingCommands['getGitHubProjectViewTable'] =
    this.issueTrackingCommands.getGitHubProjectViewTable.bind(this.issueTrackingCommands)
  getGitHubProjectWorkItemDetailsBySlug: RuntimeIssueTrackingCommands['getGitHubProjectWorkItemDetailsBySlug'] =
    this.issueTrackingCommands.getGitHubProjectWorkItemDetailsBySlug.bind(
      this.issueTrackingCommands
    )
  getGitHubRateLimit: RuntimeIssueTrackingCommands['getGitHubRateLimit'] =
    this.issueTrackingCommands.getGitHubRateLimit.bind(this.issueTrackingCommands)
  getGitLabRateLimit: RuntimeIssueTrackingCommands['getGitLabRateLimit'] =
    this.issueTrackingCommands.getGitLabRateLimit.bind(this.issueTrackingCommands)
  getGitLabRepoJobTrace: RuntimeIssueTrackingCommands['getGitLabRepoJobTrace'] =
    this.issueTrackingCommands.getGitLabRepoJobTrace.bind(this.issueTrackingCommands)
  getGitLabRepoWorkItemByPath: RuntimeIssueTrackingCommands['getGitLabRepoWorkItemByPath'] =
    this.issueTrackingCommands.getGitLabRepoWorkItemByPath.bind(this.issueTrackingCommands)
  getGitLabRepoWorkItemDetails: RuntimeIssueTrackingCommands['getGitLabRepoWorkItemDetails'] =
    this.issueTrackingCommands.getGitLabRepoWorkItemDetails.bind(this.issueTrackingCommands)
  getHostedReviewCreationEligibility: RuntimeIssueTrackingCommands['getHostedReviewCreationEligibility'] =
    this.issueTrackingCommands.getHostedReviewCreationEligibility.bind(this.issueTrackingCommands)
  getHostedReviewForBranch: RuntimeIssueTrackingCommands['getHostedReviewForBranch'] =
    this.issueTrackingCommands.getHostedReviewForBranch.bind(this.issueTrackingCommands)
  getRepoIssue: RuntimeIssueTrackingCommands['getRepoIssue'] =
    this.issueTrackingCommands.getRepoIssue.bind(this.issueTrackingCommands)
  getRepoPRCheckDetails: RuntimeIssueTrackingCommands['getRepoPRCheckDetails'] =
    this.issueTrackingCommands.getRepoPRCheckDetails.bind(this.issueTrackingCommands)
  getRepoPRChecks: RuntimeIssueTrackingCommands['getRepoPRChecks'] =
    this.issueTrackingCommands.getRepoPRChecks.bind(this.issueTrackingCommands)
  getRepoPRComments: RuntimeIssueTrackingCommands['getRepoPRComments'] =
    this.issueTrackingCommands.getRepoPRComments.bind(this.issueTrackingCommands)
  getRepoPRFileContents: RuntimeIssueTrackingCommands['getRepoPRFileContents'] =
    this.issueTrackingCommands.getRepoPRFileContents.bind(this.issueTrackingCommands)
  getRepoPRForBranch: RuntimeIssueTrackingCommands['getRepoPRForBranch'] =
    this.issueTrackingCommands.getRepoPRForBranch.bind(this.issueTrackingCommands)
  getRepoWorkItem: RuntimeIssueTrackingCommands['getRepoWorkItem'] =
    this.issueTrackingCommands.getRepoWorkItem.bind(this.issueTrackingCommands)
  getRepoWorkItemByOwnerRepo: RuntimeIssueTrackingCommands['getRepoWorkItemByOwnerRepo'] =
    this.issueTrackingCommands.getRepoWorkItemByOwnerRepo.bind(this.issueTrackingCommands)
  getRepoWorkItemDetails: RuntimeIssueTrackingCommands['getRepoWorkItemDetails'] =
    this.issueTrackingCommands.getRepoWorkItemDetails.bind(this.issueTrackingCommands)
  listGitHubAssignableUsersBySlug: RuntimeIssueTrackingCommands['listGitHubAssignableUsersBySlug'] =
    this.issueTrackingCommands.listGitHubAssignableUsersBySlug.bind(this.issueTrackingCommands)
  listGitHubIssueTypesBySlug: RuntimeIssueTrackingCommands['listGitHubIssueTypesBySlug'] =
    this.issueTrackingCommands.listGitHubIssueTypesBySlug.bind(this.issueTrackingCommands)
  listGitHubLabelsBySlug: RuntimeIssueTrackingCommands['listGitHubLabelsBySlug'] =
    this.issueTrackingCommands.listGitHubLabelsBySlug.bind(this.issueTrackingCommands)
  listGitHubProjects: RuntimeIssueTrackingCommands['listGitHubProjects'] =
    this.issueTrackingCommands.listGitHubProjects.bind(this.issueTrackingCommands)
  listGitHubProjectViews: RuntimeIssueTrackingCommands['listGitHubProjectViews'] =
    this.issueTrackingCommands.listGitHubProjectViews.bind(this.issueTrackingCommands)
  listGitLabRepoIssues: RuntimeIssueTrackingCommands['listGitLabRepoIssues'] =
    this.issueTrackingCommands.listGitLabRepoIssues.bind(this.issueTrackingCommands)
  listGitLabRepoLabels: RuntimeIssueTrackingCommands['listGitLabRepoLabels'] =
    this.issueTrackingCommands.listGitLabRepoLabels.bind(this.issueTrackingCommands)
  listGitLabRepoMRs: RuntimeIssueTrackingCommands['listGitLabRepoMRs'] =
    this.issueTrackingCommands.listGitLabRepoMRs.bind(this.issueTrackingCommands)
  listGitLabRepoTodos: RuntimeIssueTrackingCommands['listGitLabRepoTodos'] =
    this.issueTrackingCommands.listGitLabRepoTodos.bind(this.issueTrackingCommands)
  listGitLabRepoWorkItems: RuntimeIssueTrackingCommands['listGitLabRepoWorkItems'] =
    this.issueTrackingCommands.listGitLabRepoWorkItems.bind(this.issueTrackingCommands)
  listRepoAssignableUsers: RuntimeIssueTrackingCommands['listRepoAssignableUsers'] =
    this.issueTrackingCommands.listRepoAssignableUsers.bind(this.issueTrackingCommands)
  listRepoIssues: RuntimeIssueTrackingCommands['listRepoIssues'] =
    this.issueTrackingCommands.listRepoIssues.bind(this.issueTrackingCommands)
  listRepoLabels: RuntimeIssueTrackingCommands['listRepoLabels'] =
    this.issueTrackingCommands.listRepoLabels.bind(this.issueTrackingCommands)
  mergeGitLabRepoMR: RuntimeIssueTrackingCommands['mergeGitLabRepoMR'] =
    this.issueTrackingCommands.mergeGitLabRepoMR.bind(this.issueTrackingCommands)
  mergeRepoPR: RuntimeIssueTrackingCommands['mergeRepoPR'] =
    this.issueTrackingCommands.mergeRepoPR.bind(this.issueTrackingCommands)
  removeRepoPRReviewers: RuntimeIssueTrackingCommands['removeRepoPRReviewers'] =
    this.issueTrackingCommands.removeRepoPRReviewers.bind(this.issueTrackingCommands)
  requestRepoPRReviewers: RuntimeIssueTrackingCommands['requestRepoPRReviewers'] =
    this.issueTrackingCommands.requestRepoPRReviewers.bind(this.issueTrackingCommands)
  rerunRepoPRChecks: RuntimeIssueTrackingCommands['rerunRepoPRChecks'] =
    this.issueTrackingCommands.rerunRepoPRChecks.bind(this.issueTrackingCommands)
  resolveGitHubProjectRef: RuntimeIssueTrackingCommands['resolveGitHubProjectRef'] =
    this.issueTrackingCommands.resolveGitHubProjectRef.bind(this.issueTrackingCommands)
  resolveGitLabRepoMRDiscussion: RuntimeIssueTrackingCommands['resolveGitLabRepoMRDiscussion'] =
    this.issueTrackingCommands.resolveGitLabRepoMRDiscussion.bind(this.issueTrackingCommands)
  resolveRepoReviewThread: RuntimeIssueTrackingCommands['resolveRepoReviewThread'] =
    this.issueTrackingCommands.resolveRepoReviewThread.bind(this.issueTrackingCommands)
  retryGitLabRepoJob: RuntimeIssueTrackingCommands['retryGitLabRepoJob'] =
    this.issueTrackingCommands.retryGitLabRepoJob.bind(this.issueTrackingCommands)
  setRepoPRAutoMerge: RuntimeIssueTrackingCommands['setRepoPRAutoMerge'] =
    this.issueTrackingCommands.setRepoPRAutoMerge.bind(this.issueTrackingCommands)
  setRepoPRFileViewed: RuntimeIssueTrackingCommands['setRepoPRFileViewed'] =
    this.issueTrackingCommands.setRepoPRFileViewed.bind(this.issueTrackingCommands)
  updateGitHubIssueBySlug: RuntimeIssueTrackingCommands['updateGitHubIssueBySlug'] =
    this.issueTrackingCommands.updateGitHubIssueBySlug.bind(this.issueTrackingCommands)
  updateGitHubIssueCommentBySlug: RuntimeIssueTrackingCommands['updateGitHubIssueCommentBySlug'] =
    this.issueTrackingCommands.updateGitHubIssueCommentBySlug.bind(this.issueTrackingCommands)
  updateGitHubIssueTypeBySlug: RuntimeIssueTrackingCommands['updateGitHubIssueTypeBySlug'] =
    this.issueTrackingCommands.updateGitHubIssueTypeBySlug.bind(this.issueTrackingCommands)
  updateGitHubProjectItemField: RuntimeIssueTrackingCommands['updateGitHubProjectItemField'] =
    this.issueTrackingCommands.updateGitHubProjectItemField.bind(this.issueTrackingCommands)
  updateGitHubPullRequestBySlug: RuntimeIssueTrackingCommands['updateGitHubPullRequestBySlug'] =
    this.issueTrackingCommands.updateGitHubPullRequestBySlug.bind(this.issueTrackingCommands)
  updateGitLabRepoIssue: RuntimeIssueTrackingCommands['updateGitLabRepoIssue'] =
    this.issueTrackingCommands.updateGitLabRepoIssue.bind(this.issueTrackingCommands)
  updateGitLabRepoMR: RuntimeIssueTrackingCommands['updateGitLabRepoMR'] =
    this.issueTrackingCommands.updateGitLabRepoMR.bind(this.issueTrackingCommands)
  updateGitLabRepoMRReviewers: RuntimeIssueTrackingCommands['updateGitLabRepoMRReviewers'] =
    this.issueTrackingCommands.updateGitLabRepoMRReviewers.bind(this.issueTrackingCommands)
  updateGitLabRepoMRState: RuntimeIssueTrackingCommands['updateGitLabRepoMRState'] =
    this.issueTrackingCommands.updateGitLabRepoMRState.bind(this.issueTrackingCommands)
  updateRepoIssue: RuntimeIssueTrackingCommands['updateRepoIssue'] =
    this.issueTrackingCommands.updateRepoIssue.bind(this.issueTrackingCommands)
  updateRepoPRDetails: RuntimeIssueTrackingCommands['updateRepoPRDetails'] =
    this.issueTrackingCommands.updateRepoPRDetails.bind(this.issueTrackingCommands)
  updateRepoPRState: RuntimeIssueTrackingCommands['updateRepoPRState'] =
    this.issueTrackingCommands.updateRepoPRState.bind(this.issueTrackingCommands)
  updateRepoPRTitle: RuntimeIssueTrackingCommands['updateRepoPRTitle'] =
    this.issueTrackingCommands.updateRepoPRTitle.bind(this.issueTrackingCommands)
  getRepoSlug: RuntimeIssueTrackingCommands['getRepoSlug'] =
    this.issueTrackingCommands.getRepoSlug.bind(this.issueTrackingCommands)
  getRepoUpstream: RuntimeIssueTrackingCommands['getRepoUpstream'] =
    this.issueTrackingCommands.getRepoUpstream.bind(this.issueTrackingCommands)
  listRepoWorkItems: RuntimeIssueTrackingCommands['listRepoWorkItems'] =
    this.issueTrackingCommands.listRepoWorkItems.bind(this.issueTrackingCommands)

  private readonly repoHooksCommands = new RuntimeRepoHooksCommands({
    resolveRepoSelector: (selector) => this.resolveRepoSelector(selector)
  })

  getRepoHooks: RuntimeRepoHooksCommands['getRepoHooks'] = this.repoHooksCommands.getRepoHooks.bind(
    this.repoHooksCommands
  )
  checkRepoHooks: RuntimeRepoHooksCommands['checkRepoHooks'] =
    this.repoHooksCommands.checkRepoHooks.bind(this.repoHooksCommands)
  inspectRepoSetupScriptImports: RuntimeRepoHooksCommands['inspectRepoSetupScriptImports'] =
    this.repoHooksCommands.inspectRepoSetupScriptImports.bind(this.repoHooksCommands)
  readRepoIssueCommand: RuntimeRepoHooksCommands['readRepoIssueCommand'] =
    this.repoHooksCommands.readRepoIssueCommand.bind(this.repoHooksCommands)
  writeRepoIssueCommand: RuntimeRepoHooksCommands['writeRepoIssueCommand'] =
    this.repoHooksCommands.writeRepoIssueCommand.bind(this.repoHooksCommands)

  async listManagedWorktrees(
    repoSelector?: string,
    limit = DEFAULT_WORKTREE_LIST_LIMIT
  ): Promise<RuntimeWorktreeListResult> {
    if (!Number.isInteger(limit) || limit <= 0) {
      throw new Error('invalid_limit')
    }
    const resolved = await this.listResolvedWorktrees()
    const repoId = repoSelector ? (await this.resolveRepoSelector(repoSelector)).id : null
    const worktrees = resolved.filter((worktree) => {
      if (repoId && worktree.repoId !== repoId) {
        return false
      }
      return this.isRuntimeWorktreeVisible(worktree)
    })
    return {
      worktrees: worktrees.slice(0, limit),
      totalCount: worktrees.length,
      truncated: worktrees.length > limit
    }
  }

  async listDetectedManagedWorktrees(repoSelector: string): Promise<DetectedWorktreeListResult> {
    const repo = await this.resolveRepoSelector(repoSelector)
    if (isFolderRepo(repo)) {
      const worktrees = listRuntimeFolderWorkspaces(this.requireStore(), repo)
      return {
        repoId: repo.id,
        authoritative: true,
        source: 'git',
        worktrees: worktrees.map((worktree) => this.toRuntimeDetectedWorktree(repo, worktree))
      }
    }
    let scan: RuntimeWorktreeScanResult
    try {
      scan = await this.resolvedWorktreeCommands.listRepoWorktreesForResolution(repo)
    } catch {
      scan = { ok: false, worktrees: [] }
    }
    if (scan.ok) {
      this.resolvedWorktreeCommands.pruneLineageForMissingRepoWorktrees(repo, scan.worktrees)
    }
    const detected = scan.worktrees.map((gitWorktree) => {
      const worktreeId = `${repo.id}::${gitWorktree.path}`
      const meta = this.store?.getWorktreeMeta(worktreeId)
      const worktree = mergeWorktree(repo.id, gitWorktree, meta, repo.displayName)
      const detectedWorktree = this.toRuntimeDetectedWorktree(repo, worktree)
      if (scan.ok) {
        return detectedWorktree
      }
      return {
        ...detectedWorktree,
        visible: true,
        ownership: detectedWorktree.ownership === 'orca-managed' ? 'orca-managed' : 'unknown-legacy'
      } satisfies DetectedWorktree
    })
    return {
      repoId: repo.id,
      authoritative: scan.ok,
      source: scan.ok ? 'git' : 'metadata-fallback',
      worktrees: detected
    }
  }

  private isRuntimeWorktreeVisible(worktree: Worktree): boolean {
    const repo = this.store?.getRepo(worktree.repoId)
    if (!repo || !this.store) {
      return true
    }
    return this.toRuntimeDetectedWorktree(repo, worktree).visible
  }

  private toRuntimeDetectedWorktree(repo: Repo, worktree: Worktree): DetectedWorktree {
    const settings = this.store?.getSettings()
    if (!settings) {
      return {
        ...worktree,
        ownership: 'unknown-legacy',
        selectedCheckout: false,
        visible: true
      }
    }
    return toDetectedWorktree({
      repo,
      worktree,
      meta: this.store?.getWorktreeMeta(worktree.id),
      settings,
      knownOrcaLayouts: buildKnownOrcaWorkspaceLayouts(settings, repo),
      isLegacyRepoForVisibility: isLegacyRepoForExternalWorktreeVisibility(repo)
    })
  }

  async showManagedWorktree(worktreeSelector: string) {
    return await this.resolveWorktreeSelector(worktreeSelector)
  }

  async scanWorkspacePorts(repoId?: string): Promise<WorkspacePortScanResult> {
    return scanWorkspacePortProbes(await this.getWorkspacePortProbes(repoId))
  }

  async killWorkspacePort(args: WorkspacePortKillRequest): Promise<WorkspacePortKillResult> {
    return killWorkspacePort(await this.getWorkspacePortProbes(args.repoId), args)
  }

  // Why: remote clients may invoke this over RPC, so the runtime derives
  // allowed worktree paths from its own store instead of trusting client paths.
  private async getWorkspacePortProbes(repoId?: string): Promise<WorkspacePortProbe[]> {
    const reposById = new Map(
      this.requireStore()
        .getRepos()
        .map((repo) => [repo.id, repo])
    )
    return filterWorkspacePortProbes(
      (await this.listResolvedWorktrees()).map((worktree) => ({
        id: worktree.id,
        repoId: worktree.repoId,
        displayName: worktree.displayName,
        path: worktree.git.path,
        connectionId: reposById.get(worktree.repoId)?.connectionId ?? null
      })),
      repoId
    )
  }

  async sleepManagedWorktree(worktreeSelector: string): Promise<{ worktreeId: string }> {
    const worktree = await this.resolveWorktreeSelector(worktreeSelector)
    // Why: sleep is renderer-initiated on desktop (it tears down tab state
    // before killing PTYs). The notifier tells the renderer to run its own
    // sleep flow so all cleanup happens in the correct order.
    this.notifier?.sleepWorktree(worktree.id)
    return { worktreeId: worktree.id }
  }

  private readonly worktreeCreationCommands = new RuntimeWorktreeCreationCommands({
    getStore: () => this.store,
    requireStore: () => this.requireStore(),
    getPtyController: () => this.ptyController,
    getNotifier: () => this.notifier,
    createTerminal: (worktreeSelector, opts) => this.createTerminal(worktreeSelector, opts),
    notifyActivateWorktree: (repoId, worktreeId, setup, startup, defaultTabs) =>
      this.notifyActivateWorktree(repoId, worktreeId, setup, startup, defaultTabs),
    notifyWorktreesChanged: (repoId) => this.notifyWorktreesChanged(repoId),
    invalidateResolvedWorktreeCache: () => this.invalidateResolvedWorktreeCache(),
    hasRemoteTrackingRef: (repoPath, base, gitOptions) =>
      this.hasRemoteTrackingRef(repoPath, base, gitOptions),
    resolveRepoSelector: (selector) => this.resolveRepoSelector(selector),
    resolveRemoteTrackingBase: (repoPath, baseBranch, gitOptions) =>
      this.resolveRemoteTrackingBase(repoPath, baseBranch, gitOptions),
    getLocalGitExecutionOptionArgs: (repo) => this.getLocalGitExecutionOptionArgs(repo),
    getLivePtyForHandle: (handle) => this.getLivePtyForHandle(handle),
    getAgentLaunchPlatformForRepo: (repo) => this.getAgentLaunchPlatformForRepo(repo),
    resolveWorktreeSelector: (selector) => this.resolveWorktreeSelector(selector),
    subscribeToTerminalData: (ptyId, listener) => this.subscribeToTerminalData(ptyId, listener),
    splitTerminal: (handle, opts) => this.splitTerminal(handle, opts),
    setMobileSessionTabProps: (worktreeSelector, args) =>
      this.setMobileSessionTabProps(worktreeSelector, args),
    resolveLineageForWorktreeCreate: (input) => this.resolveLineageForWorktreeCreate(input),
    refreshMobileSessionPtyRecords: () => this.refreshMobileSessionPtyRecords(),
    notifyMobileSessionTabsChanged: (worktreeId) => this.notifyMobileSessionTabsChanged(worktreeId),
    hydrateHeadlessMobileSessionTabsFromWorkspaceSession: (worktreeId, options) =>
      this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(worktreeId, options),
    getGraphAuthoritativeWindowId: () => this.graph.authoritativeWindowId,
    getOrStartRemoteTrackingBaseRefresh: (repoPath, base, gitOptions) =>
      this.getOrStartRemoteTrackingBaseRefresh(repoPath, base, gitOptions),
    getHostedReviewExecutionOptions: (repo) => this.getHostedReviewExecutionOptions(repo),
    getAvailableAuthoritativeWindow: () => this.getAvailableAuthoritativeWindow(),
    fetchRemoteWithCache: (repoPath, remote, gitOptions) =>
      this.fetchRemoteWithCache(repoPath, remote, gitOptions),
    assertGraphReady: () => this.assertGraphReady(),
    getRecentPtyOutput: (ptyId) => this.ptyTranscripts.recentPtyOutputById.get(ptyId)
  })

  activateManagedWorktree: RuntimeWorktreeCreationCommands['activateManagedWorktree'] =
    this.worktreeCreationCommands.activateManagedWorktree.bind(this.worktreeCreationCommands)
  prefetchManagedWorktreeCreateBase: RuntimeWorktreeCreationCommands['prefetchManagedWorktreeCreateBase'] =
    this.worktreeCreationCommands.prefetchManagedWorktreeCreateBase.bind(
      this.worktreeCreationCommands
    )
  createManagedWorktree: RuntimeWorktreeCreationCommands['createManagedWorktree'] =
    this.worktreeCreationCommands.createManagedWorktree.bind(this.worktreeCreationCommands)
  buildStartupForAgent: RuntimeWorktreeCreationCommands['buildStartupForAgent'] =
    this.worktreeCreationCommands.buildStartupForAgent.bind(this.worktreeCreationCommands)
  markLocalWorkspaceTrustedForAgent: RuntimeWorktreeCreationCommands['markLocalWorkspaceTrustedForAgent'] =
    this.worktreeCreationCommands.markLocalWorkspaceTrustedForAgent.bind(
      this.worktreeCreationCommands
    )
  markRemoteWorkspaceTrustedForAgent: RuntimeWorktreeCreationCommands['markRemoteWorkspaceTrustedForAgent'] =
    this.worktreeCreationCommands.markRemoteWorkspaceTrustedForAgent.bind(
      this.worktreeCreationCommands
    )

  /**
   * Fetch `remote` in `repoPath`, sharing the 30s freshness window + in-flight
   * serialization with all other callers. Never rejects — callers
   * log-and-proceed on offline failures (§3.3 Lifecycle).
   *
   * Why a shared cache on the runtime instead of module-scoped: §7.1 relies on
   * one cache for BOTH the renderer create path and `probeWorktreeDrift`. A
   * dispatch tick that reuses a just-completed create-path fetch is the
   * primary telemetry target; splitting the cache by call-site would double
   * the fetch load on warm repos.
   */
  private readonly remoteFetchCache = new RuntimeRemoteFetchCache()

  getCanonicalFetchKey: RuntimeRemoteFetchCache['getCanonicalFetchKey'] =
    this.remoteFetchCache.getCanonicalFetchKey.bind(this.remoteFetchCache)
  getOrStartRemoteFetch: RuntimeRemoteFetchCache['getOrStartRemoteFetch'] =
    this.remoteFetchCache.getOrStartRemoteFetch.bind(this.remoteFetchCache)
  getOrStartRemoteTrackingBaseRefresh: RuntimeRemoteFetchCache['getOrStartRemoteTrackingBaseRefresh'] =
    this.remoteFetchCache.getOrStartRemoteTrackingBaseRefresh.bind(this.remoteFetchCache)
  fetchRemoteWithCache: RuntimeRemoteFetchCache['fetchRemoteWithCache'] =
    this.remoteFetchCache.fetchRemoteWithCache.bind(this.remoteFetchCache)
  resolveRemoteTrackingBase: RuntimeRemoteFetchCache['resolveRemoteTrackingBase'] =
    this.remoteFetchCache.resolveRemoteTrackingBase.bind(this.remoteFetchCache)
  hasRemoteTrackingRef: RuntimeRemoteFetchCache['hasRemoteTrackingRef'] =
    this.remoteFetchCache.hasRemoteTrackingRef.bind(this.remoteFetchCache)

  private readonly worktreeBaseStatusCommands = new RuntimeWorktreeBaseStatusCommands({
    getStore: () => this.store,
    requireStore: () => this.requireStore(),
    resolveWorktreeSelector: (selector) => this.resolveWorktreeSelector(selector),
    resolveRepoSelector: (selector) => this.resolveRepoSelector(selector),
    showManagedWorktree: (worktreeSelector) => this.showManagedWorktree(worktreeSelector),
    notifyWorktreesChanged: (repoId) => this.notifyWorktreesChanged(repoId),
    notifyReposChanged: () => this.notifyReposChanged(),
    invalidateResolvedWorktreeCache: () => this.invalidateResolvedWorktreeCache(),
    validateLineageParent: (child, parent) => this.validateLineageParent(child, parent),
    getOrStartRemoteFetch: (repoPath, remote, gitOptions) =>
      this.getOrStartRemoteFetch(repoPath, remote, gitOptions),
    fetchRemoteWithCache: (repoPath, remote, gitOptions) =>
      this.fetchRemoteWithCache(repoPath, remote, gitOptions),
    resolveRemoteTrackingBase: (repoPath, baseBranch, gitOptions) =>
      this.resolveRemoteTrackingBase(repoPath, baseBranch, gitOptions),
    getNotifier: () => this.notifier
  })

  recordOptimisticReconcileToken: RuntimeWorktreeBaseStatusCommands['recordOptimisticReconcileToken'] =
    this.worktreeBaseStatusCommands.recordOptimisticReconcileToken.bind(
      this.worktreeBaseStatusCommands
    )
  clearOptimisticReconcileToken: RuntimeWorktreeBaseStatusCommands['clearOptimisticReconcileToken'] =
    this.worktreeBaseStatusCommands.clearOptimisticReconcileToken.bind(
      this.worktreeBaseStatusCommands
    )
  emitWorktreeBaseStatus: RuntimeWorktreeBaseStatusCommands['emitWorktreeBaseStatus'] =
    this.worktreeBaseStatusCommands.emitWorktreeBaseStatus.bind(this.worktreeBaseStatusCommands)
  reconcileWorktreeBaseStatus: RuntimeWorktreeBaseStatusCommands['reconcileWorktreeBaseStatus'] =
    this.worktreeBaseStatusCommands.reconcileWorktreeBaseStatus.bind(
      this.worktreeBaseStatusCommands
    )
  probeWorktreeDrift: RuntimeWorktreeBaseStatusCommands['probeWorktreeDrift'] =
    this.worktreeBaseStatusCommands.probeWorktreeDrift.bind(this.worktreeBaseStatusCommands)
  updateManagedWorktreeMeta: RuntimeWorktreeBaseStatusCommands['updateManagedWorktreeMeta'] =
    this.worktreeBaseStatusCommands.updateManagedWorktreeMeta.bind(this.worktreeBaseStatusCommands)
  persistManagedWorktreeSortOrder: RuntimeWorktreeBaseStatusCommands['persistManagedWorktreeSortOrder'] =
    this.worktreeBaseStatusCommands.persistManagedWorktreeSortOrder.bind(
      this.worktreeBaseStatusCommands
    )
  resolveManagedPrBase: RuntimeWorktreeBaseStatusCommands['resolveManagedPrBase'] =
    this.worktreeBaseStatusCommands.resolveManagedPrBase.bind(this.worktreeBaseStatusCommands)
  resolveManagedMrBase: RuntimeWorktreeBaseStatusCommands['resolveManagedMrBase'] =
    this.worktreeBaseStatusCommands.resolveManagedMrBase.bind(this.worktreeBaseStatusCommands)

  private readonly branchCleanupCommands = new RuntimeBranchCleanupCommands({
    getStore: () => this.store,
    requireStore: () => this.requireStore(),
    resolveWorktreeSelector: (selector) => this.resolveWorktreeSelector(selector),
    getAgentBrowserBridge: () => this.agentBrowserBridge,
    getOffscreenBrowserBackend: () => this.offscreenBrowserBackend,
    getLocalProvider: () => this.getLocalProvider(),
    getOnPtyStopped: () => this.onPtyStopped,
    clearOptimisticReconcileToken: (worktreeId) => this.clearOptimisticReconcileToken(worktreeId),
    invalidateResolvedWorktreeCache: () => this.invalidateResolvedWorktreeCache(),
    notifyWorktreesChanged: (repoId) => this.notifyWorktreesChanged(repoId),
    getRuntimeForTeardown: () => this
  })

  forceDeletePreservedBranch: RuntimeBranchCleanupCommands['forceDeletePreservedBranch'] =
    this.branchCleanupCommands.forceDeletePreservedBranch.bind(this.branchCleanupCommands)
  removeManagedWorktree: RuntimeBranchCleanupCommands['removeManagedWorktree'] =
    this.branchCleanupCommands.removeManagedWorktree.bind(this.branchCleanupCommands)

  async renameTerminal(handle: string, title: string | null): Promise<RuntimeTerminalRename> {
    const pty = this.getLivePtyForHandle(handle)
    if (pty) {
      pty.pty.title = title
      // Why: a manual rename must outrank later agent OSC title updates (which
      // win by timestamp), so stamp it as the freshest title.
      pty.pty.titleUpdatedAt = Date.now()
      this.touchMobileSessionSnapshotsForPty(pty.pty.ptyId)
      // Why: without a renderer the rename only lived on the live pty and was
      // lost on restart. Persist customTitle so a headless rebuild keeps it.
      if (!this.notifier?.renameTerminal && pty.pty.tabId) {
        this.persistHeadlessTerminalTitle(pty.pty.worktreeId, pty.pty.tabId, title)
      }
      for (const leaf of this.graph.leaves.values()) {
        if (leaf.ptyId === pty.pty.ptyId) {
          this.notifier?.renameTerminal(leaf.tabId, title)
          return { handle, tabId: leaf.tabId, title }
        }
      }
      return { handle, tabId: pty.pty.tabId ?? pty.record.tabId, title }
    }
    this.assertGraphReady()
    const { leaf } = this.getLiveLeafForHandle(handle)
    this.notifier?.renameTerminal(leaf.tabId, title)
    return { handle, tabId: leaf.tabId, title }
  }

  private async resolveAgentTerminalCreateOptions(
    workspace: TerminalWorkspaceLaunchScope,
    opts: TerminalCreateOptions
  ): Promise<TerminalCreateOptions> {
    // Why: raw shell commands like `codex exec` must remain user-authored shell.
    // Only unmanaged, repo-backed, bare agent launches get Settings defaults.
    if (
      !opts.command ||
      opts.env ||
      opts.launchConfig ||
      opts.launchAgent ||
      opts.startupCommandDelivery ||
      opts.claudeAgentTeamsSourceCommand ||
      !workspace.repo ||
      !this.store
    ) {
      return opts
    }

    const settings = this.store.getSettings()
    const platform = this.getAgentLaunchPlatformForWorkspace(workspace)
    const isRemote = repoIsRemote(workspace.repo)
    const queuedShell = resolveLocalWindowsAgentStartupShell({
      platform,
      isRemote,
      terminalWindowsShell: settings.terminalWindowsShell
    })
    const agent = resolveBareAgentLaunchCommand({
      command: opts.command,
      settings,
      platform,
      isRemote
    })
    if (!agent) {
      return opts
    }

    const startupPlan = buildAgentStartupPlan({
      agent,
      prompt: '',
      cmdOverrides: settings.agentCmdOverrides ?? {},
      agentArgs: resolveTuiAgentLaunchArgs(agent, settings.agentDefaultArgs),
      agentEnv: resolveTuiAgentLaunchEnv(agent, settings.agentDefaultEnv),
      platform,
      shell: queuedShell,
      isRemote,
      allowEmptyPromptLaunch: true
    })
    if (!startupPlan) {
      return opts
    }

    if (workspace.connectionId) {
      await this.markRemoteWorkspaceTrustedForAgent(agent, workspace.connectionId, workspace.path)
    } else {
      this.markLocalWorkspaceTrustedForAgent(agent, workspace.path)
    }

    return {
      ...opts,
      command: startupPlan.launchCommand,
      ...(startupPlan.env ? { env: startupPlan.env } : {}),
      launchConfig: startupPlan.launchConfig,
      launchAgent: agent,
      startupCommandDelivery: startupPlan.startupCommandDelivery
    }
  }

  async createTerminal(
    worktreeSelector?: string,
    opts: TerminalCreateOptions = {}
  ): Promise<RuntimeTerminalCreate> {
    const presentation = resolveTerminalPresentation(opts)
    const requiresRendererFocus = opts.presentation === 'focused' || opts.focus === true
    // Why: pre-diff createTerminal fell back to the renderer's active worktree
    // when no selector was provided. The new background-spawn branch hard-
    // requires a resolvable selector, so route the no-selector case through
    // the renderer IPC path to preserve that behavior.
    const rendererWindow =
      opts.rendererBacked === true ? this.getAvailableAuthoritativeWindow() : null
    const shouldCreateInBackground =
      worktreeSelector !== undefined &&
      ((!requiresRendererFocus && opts.rendererBacked !== true) ||
        // Why: `orca serve` exposes the local runtime without a renderer
        // window. Renderer-backed Codex terminals are preferred for the app,
        // but headless CLI users still need a usable terminal handle.
        (opts.rendererBacked === true && rendererWindow === null))

    if (shouldCreateInBackground) {
      if (!this.ptyController?.spawn) {
        throw new Error('runtime_unavailable')
      }
      const workspace = await this.resolveTerminalWorkspaceLaunchScope(worktreeSelector)
      const launchOpts = await this.resolveAgentTerminalCreateOptions(workspace, opts)
      const cwd =
        this.resolveWorkspaceTerminalStartupCwd(workspace, launchOpts.cwd) ?? workspace.path
      const preAllocatedHandle = this.createPreAllocatedTerminalHandle()
      // Why: mint tabId in main before spawn so paneKey is known at PTY env
      // build time. Hook-based agent status (Claude/Codex/Cursor/Gemini) keys
      // off `${tabId}:${leafId}` — without these vars set on the PTY, the
      // hook payload arrives with an empty paneKey and the renderer cannot
      // attribute the event. Use a stable UUID leaf because hooks reject the
      // legacy numeric pane keys after the pane-id migration.
      const hintedTabId = launchOpts.tabId?.trim()
      const canAdoptPaneIdentity =
        hintedTabId !== undefined &&
        isValidHostTerminalTabId(hintedTabId) &&
        launchOpts.leafId !== undefined &&
        isTerminalLeafId(launchOpts.leafId)
      const tabId = canAdoptPaneIdentity ? (hintedTabId as string) : randomUUID()
      const leafId = canAdoptPaneIdentity ? (launchOpts.leafId as string) : randomUUID()
      const paneKey = makePaneKey(tabId, leafId)
      const launchToken = launchOpts.launchConfig
        ? (launchOpts.launchToken ?? randomUUID())
        : undefined
      const baseEnv = {
        ...launchOpts.env,
        ...(launchToken ? { ORCA_AGENT_LAUNCH_TOKEN: launchToken } : {})
      }
      const claudeAgentTeamsSourceCommand =
        launchOpts.claudeAgentTeamsSourceCommand?.trim() || launchOpts.command?.trim() || undefined
      const claudeAgentTeamsMode = this.store?.getSettings?.().claudeAgentTeamsMode
      const effectiveClaudeAgentTeamsMode = inferCapturedClaudeAgentTeamsMode(
        launchOpts.launchConfig,
        claudeAgentTeamsSourceCommand,
        claudeAgentTeamsMode
      )
      const agentTeamsPlan = await buildClaudeAgentTeamsLaunchPlan({
        command: claudeAgentTeamsSourceCommand,
        mode: effectiveClaudeAgentTeamsMode,
        baseEnv: {
          ...process.env,
          ...baseEnv
        },
        createTeamEnv: (shimDir, shimBin) =>
          this.claudeAgentTeams.createLaunchEnv({
            leaderHandle: preAllocatedHandle,
            baseEnv: {
              ...process.env,
              ...baseEnv
            },
            shimDir,
            shimBin
          }).env
      })
      const sequencedStartupCommand =
        agentTeamsPlan &&
        claudeAgentTeamsSourceCommand &&
        launchOpts.command &&
        claudeAgentTeamsSourceCommand !== launchOpts.command
          ? agentTeamsPlan.command
          : undefined
      const effectiveLaunchConfig =
        launchOpts.launchConfig && agentTeamsPlan
          ? {
              ...launchOpts.launchConfig,
              agentCommand: launchOpts.launchConfig.agentCommand
                ? effectiveClaudeAgentTeamsMode === 'in-process' || process.platform === 'win32'
                  ? addClaudeTeammateModeInProcess(launchOpts.launchConfig.agentCommand)
                  : addClaudeTeammateModeAuto(launchOpts.launchConfig.agentCommand)
                : agentTeamsPlan.command,
              agentEnv: {
                ...launchOpts.launchConfig.agentEnv,
                ...agentTeamsPlan.env
              }
            }
          : launchOpts.launchConfig
      // Why: setup/agent sequencing wraps the PTY launch in a wait shell before
      // Claude Agent Teams runs. Preserve the direct Claude command separately
      // so the wrapper can exec the teammate-mode variant after setup completes.
      const env = this.buildTerminalWorkspaceEnv(
        workspace,
        {
          ...baseEnv,
          ...(sequencedStartupCommand
            ? { [SETUP_AGENT_SEQUENCE_STARTUP_COMMAND_ENV]: sequencedStartupCommand }
            : {})
        },
        paneKey,
        tabId,
        agentTeamsPlan?.env
      )
      const result = await this.ptyController.spawn({
        cols: 120,
        rows: 40,
        cwd,
        command: sequencedStartupCommand
          ? launchOpts.command
          : (agentTeamsPlan?.command ?? launchOpts.command),
        commandDelivery: 'provider',
        startupCommandDelivery: launchOpts.startupCommandDelivery,
        env,
        envToDelete: agentTeamsPlan?.envToDelete,
        telemetry: launchOpts.telemetry,
        connectionId: workspace.connectionId,
        worktreeId: workspace.id,
        preAllocatedHandle,
        tabId,
        leafId,
        ...(launchOpts.sessionId ? { sessionId: launchOpts.sessionId } : {}),
        ...(launchOpts.persistHostSessionBinding ? { persistHostSessionBinding: true } : {})
      })
      this.registerPreAllocatedHandleForPty(result.id, preAllocatedHandle)
      this.registerPty(result.id, workspace.id, workspace.connectionId)
      const pty = this.getOrCreatePtyWorktreeRecord(result.id)
      if (pty) {
        if (launchOpts.title) {
          const observedAt = this.nextTitleObservationSequence()
          pty.title = launchOpts.title
          pty.titleUpdatedAt = observedAt
          this.setPtyManagementTitleFromObservedTitle(pty, launchOpts.title, observedAt)
        } else {
          pty.title = null
          pty.titleUpdatedAt = null
        }
        pty.tabId = tabId
        pty.paneKey = paneKey
        pty.launchConfig = effectiveLaunchConfig
          ? copySleepingAgentLaunchConfig(effectiveLaunchConfig)
          : null
        pty.launchToken = launchToken ?? null
        pty.launchAgent = launchOpts.launchAgent ?? null
      }
      const handle = pty ? this.issuePtyHandle(pty) : preAllocatedHandle
      if (pty && launchOpts.deferMobileSessionPublish !== true) {
        this.publishPtyBackedMobileSessionTerminal(workspace.id, pty, {
          tabId,
          leafId,
          title: launchOpts.title ?? null,
          activate: presentation === 'focused',
          // Why: explicit background presentation may carry legacy activate
          // metadata from an already-owned renderer pane; don't select it on mobile.
          selectIfNoActiveTab: presentation !== 'background',
          ...(cwd !== workspace.path ? { startupCwd: cwd } : {})
        })
      }
      let surface: RuntimeTerminalCreate['surface'] = 'background'
      let warning: string | undefined
      if (presentation !== 'background' && this.notifier?.revealTerminalSession) {
        try {
          // Why: after the PTY is spawned, renderer tab adoption is best-effort;
          // failing here must not strand a live process without returning a handle.
          // Pass the pre-minted tabId so the renderer adopts under the same id
          // already baked into the PTY env — keeps paneKey hook attribution intact.
          await this.notifier.revealTerminalSession(workspace.id, {
            ptyId: result.id,
            title: launchOpts.title ?? null,
            ...(cwd !== workspace.path ? { cwd } : {}),
            ...(effectiveLaunchConfig ? { launchConfig: effectiveLaunchConfig } : {}),
            ...(launchToken ? { launchToken } : {}),
            ...(launchOpts.launchAgent ? { launchAgent: launchOpts.launchAgent } : {}),
            activate: presentation === 'focused',
            ...(presentation ? { presentation } : {}),
            tabId,
            leafId
          })
          surface = 'visible'
        } catch (err) {
          console.warn(`[terminal-create] failed to create inactive tab for ${result.id}:`, err)
          warning = createTerminalRevealWarning(handle, err)
        }
      } else if (presentation !== 'background') {
        warning = createTerminalRevealWarning(handle)
      }
      return {
        handle,
        tabId,
        paneKey,
        ptyId: result.id,
        worktreeId: workspace.id,
        title: launchOpts.title ?? null,
        surface,
        ...(warning ? { warning } : {})
      }
    }

    this.assertGraphReady()
    const win = rendererWindow ?? this.getAuthoritativeWindow()
    // Why: mirrors browserTabCreate — when no worktree is specified, pass
    // undefined so the renderer uses its current active worktree.
    const workspace = worktreeSelector
      ? await this.resolveTerminalWorkspaceLaunchScope(worktreeSelector)
      : null
    const launchOpts = workspace
      ? await this.resolveAgentTerminalCreateOptions(workspace, opts)
      : opts
    const worktreeId = workspace?.id
    const cwd = workspace
      ? this.resolveWorkspaceTerminalStartupCwd(workspace, launchOpts.cwd)
      : launchOpts.cwd
    const requestId = randomUUID()

    // Why: terminal creation is a renderer-side Zustand store operation (like
    // browser tab creation). The main process sends a request, the renderer
    // creates the tab and replies with the tabId so we can resolve the handle.
    const reply = await new Promise<{ tabId: string; title: string }>((resolve, reject) => {
      const timer = setTimeout(() => {
        ipcMain.removeListener('terminal:tabCreateReply', handler)
        reject(new Error('Terminal creation timed out'))
      }, 10_000)

      const handler = (
        event: Electron.IpcMainEvent,
        r: { requestId: string; tabId?: string; title?: string; error?: string }
      ): void => {
        if (event.sender !== win.webContents || r.requestId !== requestId) {
          return
        }
        clearTimeout(timer)
        ipcMain.removeListener('terminal:tabCreateReply', handler)
        if (r.error) {
          reject(new Error(r.error))
        } else {
          resolve({ tabId: r.tabId!, title: r.title ?? launchOpts.title ?? '' })
        }
      }
      ipcMain.on('terminal:tabCreateReply', handler)
      win.webContents.send('terminal:requestTabCreate', {
        requestId,
        worktreeId,
        command: launchOpts.command,
        cwd,
        ...(launchOpts.env ? { env: launchOpts.env } : {}),
        ...(launchOpts.launchConfig ? { launchConfig: launchOpts.launchConfig } : {}),
        ...(launchOpts.launchToken ? { launchToken: launchOpts.launchToken } : {}),
        ...(launchOpts.launchAgent ? { launchAgent: launchOpts.launchAgent } : {}),
        startupCommandDelivery: launchOpts.startupCommandDelivery,
        title: launchOpts.title,
        activate: presentation === 'focused',
        ...(presentation ? { presentation } : {})
      })
    })

    // Why: the renderer created the tab immediately, but the graph sync that
    // populates this.graph.leaves may not have arrived yet. Wait for the leaf to
    // appear so we can return a valid handle the caller can use right away.
    const handle = await this.waitForTerminalHandle(reply.tabId)
    return {
      handle,
      tabId: reply.tabId,
      worktreeId: worktreeId ?? '',
      title: reply.title,
      surface: 'visible'
    }
  }

  async launchAgentTerminal(
    worktreeSelector: string,
    opts: { agent: TuiAgent; prompt: string; title?: string }
  ): Promise<RuntimeTerminalCreate> {
    const worktree = await this.resolveWorktreeSelector(worktreeSelector)
    const repo = this.store?.getRepo(worktree.repoId)
    if (!repo) {
      throw new Error('Repository for the selected workspace is no longer available.')
    }
    const startup = this.buildStartupForAgent(repo, opts.agent, opts.prompt)
    if (repo.connectionId) {
      await this.markRemoteWorkspaceTrustedForAgent(opts.agent, repo.connectionId, worktree.path)
    } else {
      this.markLocalWorkspaceTrustedForAgent(opts.agent, worktree.path)
    }
    return await this.createTerminal(`id:${worktree.id}`, {
      command: startup.startup.command,
      env: startup.startup.env,
      ...(startup.startup.launchConfig ? { launchConfig: startup.startup.launchConfig } : {}),
      launchAgent: startup.agent,
      startupCommandDelivery: startup.startup.startupCommandDelivery,
      telemetry: startup.startup.telemetry,
      title: opts.title
    })
  }

  private readonly mobileSessionTerminalCommands = new RuntimeMobileSessionTerminalCommands({
    getStore: () => this.store,
    getNotifier: () => this.notifier,
    getGraph: () => this.graph,
    getAvailableAuthoritativeWindow: () => this.getAvailableAuthoritativeWindow(),
    getLivePtyForHandle: (handle) => this.getLivePtyForHandle(handle),
    createTerminal: (worktreeSelector, opts) => this.createTerminal(worktreeSelector, opts),
    assertGraphReady: () => this.assertGraphReady(),
    markLocalWorkspaceTrustedForAgent: (agent, workspacePath) =>
      this.markLocalWorkspaceTrustedForAgent(agent, workspacePath),
    markRemoteWorkspaceTrustedForAgent: (agent, connectionId, workspacePath) =>
      this.markRemoteWorkspaceTrustedForAgent(agent, connectionId, workspacePath),
    getMobileSessionTabsByWorktree: () => this.mobileSessionTabsByWorktree,
    getMobileSessionTabListeners: () => this.mobileSessionTabListeners,
    hydrateHeadlessMobileSessionTabsFromWorkspaceSession: (worktreeId, options) =>
      this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(worktreeId, options),
    resolveTerminalWorkspaceLaunchScope: (selector) =>
      this.resolveTerminalWorkspaceLaunchScope(selector),
    resolveWorkspaceTerminalStartupCwd: (workspace, requestedCwd) =>
      this.resolveWorkspaceTerminalStartupCwd(workspace, requestedCwd),
    getAgentLaunchPlatformForWorkspace: (workspace) =>
      this.getAgentLaunchPlatformForWorkspace(workspace),
    buildMaterializedHeadlessParentLayout: (leafId, ptyId, existingLayout) =>
      this.buildMaterializedHeadlessParentLayout(leafId, ptyId, existingLayout),
    getHeadlessMobileSessionGroupId: (worktreeId) =>
      this.getHeadlessMobileSessionGroupId(worktreeId),
    buildHeadlessMobileSessionTabGroups: (
      worktreeId,
      tabs,
      activeTab,
      existingGroups,
      targetAssignment
    ) =>
      this.buildHeadlessMobileSessionTabGroups(
        worktreeId,
        tabs,
        activeTab,
        existingGroups,
        targetAssignment
      ),
    toMobileSessionTabsResult: (snapshot) => this.toMobileSessionTabsResult(snapshot),
    publishPtyBackedMobileSessionTerminal: (worktreeId, pty, args) =>
      this.publishPtyBackedMobileSessionTerminal(worktreeId, pty, args)
  })

  createMobileSessionTerminal: RuntimeMobileSessionTerminalCommands['createMobileSessionTerminal'] =
    this.mobileSessionTerminalCommands.createMobileSessionTerminal.bind(
      this.mobileSessionTerminalCommands
    )
  createHeadlessMobileSessionTerminal: RuntimeMobileSessionTerminalCommands['createHeadlessMobileSessionTerminal'] =
    this.mobileSessionTerminalCommands.createHeadlessMobileSessionTerminal.bind(
      this.mobileSessionTerminalCommands
    )
  resolveMobileSessionTerminalCommand: RuntimeMobileSessionTerminalCommands['resolveMobileSessionTerminalCommand'] =
    this.mobileSessionTerminalCommands.resolveMobileSessionTerminalCommand.bind(
      this.mobileSessionTerminalCommands
    )
  ensurePtyBackedMobileSurfaceForRendererTab: RuntimeMobileSessionTerminalCommands['ensurePtyBackedMobileSurfaceForRendererTab'] =
    this.mobileSessionTerminalCommands.ensurePtyBackedMobileSurfaceForRendererTab.bind(
      this.mobileSessionTerminalCommands
    )

  private waitForTerminalHandle(tabId: string, timeoutMs = 10_000): Promise<string> {
    const existing = this.resolveHandleForTab(tabId)
    if (existing) {
      return Promise.resolve(existing)
    }

    return new Promise<string>((resolve, reject) => {
      const timer = setTimeout(() => {
        const idx = this.graph.graphSyncCallbacks.indexOf(check)
        if (idx !== -1) {
          this.graph.graphSyncCallbacks.splice(idx, 1)
        }
        reject(new Error('Timed out waiting for terminal handle after creation'))
      }, timeoutMs)

      const check = (): void => {
        const handle = this.resolveHandleForTab(tabId)
        if (handle) {
          clearTimeout(timer)
          const idx = this.graph.graphSyncCallbacks.indexOf(check)
          if (idx !== -1) {
            this.graph.graphSyncCallbacks.splice(idx, 1)
          }
          resolve(handle)
        }
      }
      this.graph.graphSyncCallbacks.push(check)
      // Why: the graph sync may have fired between the initial check and
      // callback registration. Re-check immediately to avoid a missed wake-up.
      check()
    })
  }

  // Why: mobile clients may subscribe before the PTY spawns (the left pane
  // of a new workspace). Instead of bailing with a bare scrollback+end,
  // wait for the PTY to appear so the subscribe can proceed with phone-fit.
  waitForLeafPtyId(handle: string, timeoutMs = 10_000, signal?: AbortSignal): Promise<string> {
    const leaf = this.resolveLeafForHandle(handle)
    if (leaf?.ptyId) {
      return Promise.resolve(leaf.ptyId)
    }

    // Why: when the ptyId changes from null to a real value, the old handle
    // is invalidated (deleted from this.graph.handles). Capture the tabId+leafId
    // now so we can look up the leaf directly even after handle invalidation.
    const record = this.graph.handles.get(handle)
    const savedTabId = record?.tabId ?? null
    const savedLeafId = record?.leafId ?? null

    return new Promise<string>((resolve, reject) => {
      let timer: ReturnType<typeof setTimeout> | null = null
      let check: () => void = () => {}
      const cleanup = (): void => {
        if (timer) {
          clearTimeout(timer)
          timer = null
        }
        const idx = this.graph.graphSyncCallbacks.indexOf(check)
        if (idx !== -1) {
          this.graph.graphSyncCallbacks.splice(idx, 1)
        }
        signal?.removeEventListener('abort', onAbort)
      }
      const finish = (ptyId: string): void => {
        cleanup()
        resolve(ptyId)
      }
      const fail = (error: Error): void => {
        cleanup()
        reject(error)
      }
      const onAbort = (): void => {
        fail(new Error('request_aborted'))
      }
      if (signal?.aborted) {
        reject(new Error('request_aborted'))
        return
      }
      signal?.addEventListener('abort', onAbort, { once: true })
      timer = setTimeout(() => {
        fail(new Error('Timed out waiting for PTY to spawn'))
      }, timeoutMs)

      check = (): void => {
        // Try the handle first (works if handle wasn't invalidated yet)
        let ptyId = this.resolveLeafForHandle(handle)?.ptyId
        // Why: when ptyId transitions null→real, issueHandle invalidates the
        // old handle. Fall back to direct leaf lookup by the saved coordinates.
        if (!ptyId && savedTabId && savedLeafId) {
          const directLeaf = this.graph.leaves.get(this.getLeafKey(savedTabId, savedLeafId))
          ptyId = directLeaf?.ptyId ?? null
        }
        if (ptyId) {
          finish(ptyId)
        }
      }
      this.graph.graphSyncCallbacks.push(check)
      check()
    })
  }

  // Why: a leaf appears in the graph before its PTY spawns. If we issue a
  // handle while ptyId is null, the next graph sync after PTY spawn will
  // change ptyId and invalidate the handle. Wait for a connected PTY so
  // the handle is stable and immediately usable for send/read/wait.
  private countLeavesInTab(tabId: string): number {
    let count = 0
    for (const leaf of this.graph.leaves.values()) {
      if (leaf.tabId === tabId) {
        count++
      }
    }
    return count
  }

  private resolveHandleForTab(tabId: string): string | null {
    for (const leaf of this.graph.leaves.values()) {
      if (leaf.tabId === tabId && leaf.ptyId !== null) {
        return this.issueHandle(leaf)
      }
    }
    return null
  }

  async focusTerminal(handle: string): Promise<RuntimeTerminalFocus> {
    const pty = this.getLivePtyForHandle(handle)
    if (pty) {
      if (!pty.pty.connected) {
        throw new Error('terminal_exited')
      }
      const parsedPaneKey = parsePaneKey(pty.pty.paneKey ?? '')
      const revealed = await this.notifier?.revealTerminalSession?.(pty.pty.worktreeId, {
        ptyId: pty.pty.ptyId,
        title: getLatestPtyTitle(pty.pty),
        ...(pty.pty.launchConfig
          ? { launchConfig: copySleepingAgentLaunchConfig(pty.pty.launchConfig) }
          : {}),
        ...(pty.pty.launchToken ? { launchToken: pty.pty.launchToken } : {}),
        ...(pty.pty.launchAgent ? { launchAgent: pty.pty.launchAgent } : {}),
        ...(pty.pty.tabId !== null ? { tabId: pty.pty.tabId } : {}),
        ...(parsedPaneKey ? { leafId: parsedPaneKey.leafId } : {})
      })
      return {
        handle,
        tabId: revealed?.tabId ?? pty.pty.tabId ?? pty.record.tabId,
        worktreeId: pty.pty.worktreeId
      }
    }
    this.assertGraphReady()
    const { leaf } = this.getLiveLeafForHandle(handle)
    this.notifier?.focusTerminal(leaf.tabId, leaf.worktreeId, leaf.leafId)
    return { handle, tabId: leaf.tabId, worktreeId: leaf.worktreeId }
  }

  async closeTerminal(handle: string): Promise<RuntimeTerminalClose> {
    const pty = this.getLivePtyForHandle(handle)
    this.claudeAgentTeams.removeTeamForLeaderHandle(handle)
    if (pty) {
      const ptyKilled = this.ptyController?.kill(pty.pty.ptyId) ?? false
      return { handle, tabId: pty.pty.tabId ?? pty.record.tabId, ptyKilled }
    }
    this.assertGraphReady()
    const { leaf } = this.getLiveLeafForHandle(handle)
    let ptyKilled = false
    if (leaf.ptyId) {
      ptyKilled = this.ptyController?.kill(leaf.ptyId) ?? false
    }
    // Why: killing the PTY in a multi-pane tab is sufficient — the renderer's
    // PTY exit handler already calls PaneManager.closePane() for split layouts.
    // Sending an additional IPC close would race with the exit handler and
    // incorrectly close the entire tab (the pane count drops to 1 before the
    // IPC arrives, triggering the single-pane fallback path).
    // We only send the notifier close when the PTY wasn't killed (e.g. PTY not
    // yet spawned) or when this is the only pane in the tab.
    const siblingCount = this.countLeavesInTab(leaf.tabId)
    if (!ptyKilled || siblingCount <= 1) {
      this.notifier?.closeTerminal(leaf.tabId, leaf.paneRuntimeId)
    }
    return { handle, tabId: leaf.tabId, ptyKilled }
  }

  async splitTerminal(
    handle: string,
    opts: {
      direction?: 'horizontal' | 'vertical'
      command?: string
      env?: Record<string, string>
      envToDelete?: string[]
      activate?: boolean
      telemetrySource?: TerminalPaneSplitSource
    } = {}
  ): Promise<RuntimeTerminalSplit> {
    const livePty = this.getLivePtyForHandle(handle)
    if (livePty) {
      return await this.splitPtyBackedTerminal(livePty.pty, opts)
    }
    this.assertGraphReady()
    const { leaf } = this.getLiveLeafForHandle(handle)
    const direction = opts.direction ?? 'horizontal'

    // Why: snapshot current leaf keys for this tab so we can detect the new
    // pane that appears after the split via graph sync delta.
    const leafKeysBefore = new Set<string>()
    for (const [key, l] of this.graph.leaves) {
      if (l.tabId === leaf.tabId) {
        leafKeysBefore.add(key)
      }
    }

    this.notifier?.splitTerminal(leaf.tabId, leaf.paneRuntimeId, {
      direction,
      command: opts.command,
      telemetrySource: opts.telemetrySource
    })

    const newHandle = await this.waitForNewLeafInTab(leaf.tabId, leafKeysBefore)
    return { handle: newHandle, tabId: leaf.tabId, paneRuntimeId: leaf.paneRuntimeId }
  }

  private async splitPtyBackedTerminal(
    pty: RuntimePtyWorktreeRecord,
    opts: {
      direction?: 'horizontal' | 'vertical'
      command?: string
      env?: Record<string, string>
      envToDelete?: string[]
      activate?: boolean
      telemetrySource?: TerminalPaneSplitSource
    } = {}
  ): Promise<RuntimeTerminalSplit> {
    if (!this.ptyController?.spawn) {
      throw new Error('runtime_unavailable')
    }
    if (!pty.connected) {
      throw new Error('terminal_exited')
    }
    const parsedPaneKey = parsePaneKey(pty.paneKey ?? '')
    const parentTabId = pty.tabId?.trim()
    if (!parentTabId || !parsedPaneKey) {
      throw new Error('terminal_handle_stale')
    }
    const direction = opts.direction ?? 'horizontal'
    const workspace = await this.resolveTerminalWorkspaceLaunchScope(`id:${pty.worktreeId}`)
    const leafId = randomUUID()
    const preAllocatedHandle = this.createPreAllocatedTerminalHandle()
    const paneKey = makePaneKey(parentTabId, leafId)
    const result = await this.ptyController.spawn({
      cols: 120,
      rows: 40,
      cwd: workspace.path,
      command: opts.command,
      commandDelivery: 'provider',
      env: this.buildTerminalWorkspaceEnv(workspace, opts.env ?? {}, paneKey, parentTabId),
      envToDelete: opts.envToDelete,
      connectionId: workspace.connectionId,
      worktreeId: workspace.id,
      preAllocatedHandle
    })
    this.registerPreAllocatedHandleForPty(result.id, preAllocatedHandle)
    this.registerPty(result.id, workspace.id, workspace.connectionId)
    const createdPty = this.getOrCreatePtyWorktreeRecord(result.id)
    if (createdPty) {
      createdPty.tabId = parentTabId
      createdPty.paneKey = paneKey
    }

    try {
      await this.notifier?.revealTerminalSession?.(workspace.id, {
        ptyId: result.id,
        title: null,
        activate: opts.activate !== false,
        tabId: parentTabId,
        leafId,
        splitFromLeafId: parsedPaneKey.leafId,
        splitDirection: direction,
        splitTelemetrySource: opts.telemetrySource
      })
    } catch (error) {
      this.ptyController.kill?.(result.id)
      throw error
    }
    if (createdPty) {
      this.publishPtyBackedMobileSessionTerminal(workspace.id, createdPty, {
        tabId: parentTabId,
        leafId,
        title: null,
        activate: opts.activate !== false,
        split: { splitFromLeafId: parsedPaneKey.leafId, direction }
      })
      // Why: persist the split into the workspace session so a later snapshot
      // rebuild keeps it instead of collapsing back to a single pane.
      this.persistHeadlessTerminalSplit({
        tabId: parentTabId,
        leafId,
        ptyId: createdPty.ptyId,
        splitFromLeafId: parsedPaneKey.leafId,
        direction
      })
    }

    return { handle: this.issuePtyHandle(createdPty ?? pty), tabId: parentTabId, paneRuntimeId: -1 }
  }

  async handleAgentTeamsTmuxCompat(
    request: AgentTeamsTmuxCompatRequest
  ): Promise<AgentTeamsTmuxCompatResponse> {
    return await this.claudeAgentTeams.handleTmuxCompat(request, {
      splitTerminal: (handle, opts) => this.splitTerminal(handle, opts),
      readTerminal: (handle, opts) => this.readTerminal(handle, opts),
      sendTerminal: (handle, action) => this.sendTerminal(handle, action),
      focusTerminal: (handle) => this.focusTerminal(handle),
      closeTerminal: (handle) => this.closeTerminal(handle),
      showTerminal: (handle) => this.showTerminal(handle)
    })
  }

  async prepareClaudeAgentTeamsLeader(args: {
    paneKey: string
    baseEnv?: Record<string, string>
  }): Promise<{ env: Record<string, string> }> {
    const handle = this.getTerminalHandleForPaneKey(args.paneKey)
    if (!handle) {
      throw new Error('claude_agent_teams_requires_orca_terminal')
    }
    return await this.prepareClaudeAgentTeamsLeaderForHandle({
      handle,
      baseEnv: args.baseEnv
    })
  }

  async prepareClaudeAgentTeamsLeaderForHandle(args: {
    handle: string
    baseEnv?: Record<string, string>
  }): Promise<{ env: Record<string, string> }> {
    const baseEnv = {
      ...process.env,
      ...args.baseEnv
    }
    const shimDir = await ensureClaudeAgentTeamsShimDir()
    const shimBin = resolveClaudeAgentTeamsShimBin(baseEnv)
    return this.claudeAgentTeams.createLaunchEnv({
      leaderHandle: args.handle,
      baseEnv,
      shimDir,
      shimBin
    })
  }

  private waitForNewLeafInTab(
    tabId: string,
    existingLeafKeys: Set<string>,
    timeoutMs = 10_000
  ): Promise<string> {
    const tryResolve = (): string | null => {
      for (const [key, leaf] of this.graph.leaves) {
        if (leaf.tabId === tabId && !existingLeafKeys.has(key) && leaf.ptyId !== null) {
          return this.issueHandle(leaf)
        }
      }
      return null
    }

    const existing = tryResolve()
    if (existing) {
      return Promise.resolve(existing)
    }

    return new Promise<string>((resolve, reject) => {
      const timer = setTimeout(() => {
        const idx = this.graph.graphSyncCallbacks.indexOf(check)
        if (idx !== -1) {
          this.graph.graphSyncCallbacks.splice(idx, 1)
        }
        reject(new Error('Timed out waiting for split pane handle'))
      }, timeoutMs)

      const check = (): void => {
        const handle = tryResolve()
        if (handle) {
          clearTimeout(timer)
          const idx = this.graph.graphSyncCallbacks.indexOf(check)
          if (idx !== -1) {
            this.graph.graphSyncCallbacks.splice(idx, 1)
          }
          resolve(handle)
        }
      }
      this.graph.graphSyncCallbacks.push(check)
      check()
    })
  }

  async stopTerminalsForWorktree(worktreeSelector: string): Promise<{ stopped: number }> {
    // Why: this mutates live PTYs, so the runtime must reject it while the
    // renderer graph is reloading instead of acting on cached leaf ownership.
    const graphEpoch = this.captureReadyGraphEpoch()
    const worktree = await this.resolveWorktreeSelector(worktreeSelector)
    this.assertStableReadyGraph(graphEpoch)
    const ptyIds = new Set<string>()
    for (const leaf of this.graph.leaves.values()) {
      if (leaf.worktreeId === worktree.id && leaf.ptyId) {
        ptyIds.add(leaf.ptyId)
      }
    }
    for (const pty of this.graph.ptysById.values()) {
      if (pty.worktreeId === worktree.id && pty.connected) {
        ptyIds.add(pty.ptyId)
      }
    }

    let stopped = 0
    for (const ptyId of ptyIds) {
      if (this.ptyController?.kill(ptyId)) {
        stopped += 1
      }
    }
    return { stopped }
  }

  async stopExactTerminalsForWorktree(
    worktreeSelector: string,
    expectedPtyIds: readonly string[],
    opts: { keepHistory?: boolean; targetOnly?: boolean } = {}
  ): Promise<{
    stopped: number
    stoppedPtyIds: string[]
    livePtyIds: string[]
    postStopVerified: boolean
    postStopFailure?: string
    remainingLivePtyIds?: string[]
  }> {
    // Why: worktree sleep needs proof of the complete live set; pane hibernation
    // only needs proof that its target PTY was live and is now gone.
    const graphEpoch = this.captureReadyGraphEpoch()
    const worktree = await this.resolveWorktreeSelector(worktreeSelector)
    this.assertStableReadyGraph(graphEpoch)
    const expected = new Set(expectedPtyIds.filter((ptyId) => ptyId.length > 0))
    if (expected.size !== 1) {
      throw new Error('terminal_exact_stop_requires_single_pty')
    }
    const resolvedWorktrees = [...(await this.getResolvedWorktreeMap()).values()]
    const refreshedPtyLiveness =
      await this.refreshPtyWorktreeRecordsFromController(resolvedWorktrees)
    if (!refreshedPtyLiveness) {
      throw new Error('terminal_liveness_unavailable')
    }
    const livePtyIds = this.getLivePtyIdsForWorktree(worktree.id, refreshedPtyLiveness)
    const targetOnly = opts.targetOnly === true
    const expectedIsLive = [...expected].every((ptyId) => livePtyIds.has(ptyId))
    if (targetOnly ? !expectedIsLive : !setsEqual(livePtyIds, expected)) {
      const error = Object.assign(new Error('terminal_stop_pty_set_mismatch'), {
        livePtyIds: [...livePtyIds].sort(),
        expectedPtyIds: [...expected].sort()
      })
      throw error
    }

    if (!this.ptyController?.stopAndWait) {
      throw new Error('terminal_exact_stop_unavailable')
    }

    const stoppedPtyIds: string[] = []
    for (const ptyId of [...expected].sort()) {
      if (!(await this.ptyController.stopAndWait(ptyId, { keepHistory: opts.keepHistory }))) {
        throw Object.assign(new Error('terminal_exact_stop_failed'), { ptyId })
      }
      stoppedPtyIds.push(ptyId)
    }
    const postStopLiveness = await this.refreshPtyWorktreeRecordsFromController(resolvedWorktrees)
    if (!postStopLiveness) {
      return {
        stopped: stoppedPtyIds.length,
        stoppedPtyIds,
        livePtyIds: [...livePtyIds].sort(),
        postStopVerified: false,
        postStopFailure: 'terminal_liveness_unavailable'
      }
    }
    const remainingLivePtyIds = this.getLivePtyIdsForWorktree(worktree.id, postStopLiveness)
    const stoppedTargetsStillLive = [...expected].filter((ptyId) => remainingLivePtyIds.has(ptyId))
    if (targetOnly ? stoppedTargetsStillLive.length > 0 : remainingLivePtyIds.size > 0) {
      return {
        stopped: stoppedPtyIds.length,
        stoppedPtyIds,
        livePtyIds: [...livePtyIds].sort(),
        postStopVerified: false,
        postStopFailure: 'terminal_exact_stop_still_live',
        remainingLivePtyIds: [...remainingLivePtyIds].sort()
      }
    }
    return {
      stopped: stoppedPtyIds.length,
      stoppedPtyIds,
      livePtyIds: [...livePtyIds].sort(),
      postStopVerified: true,
      ...(targetOnly && remainingLivePtyIds.size > 0
        ? { remainingLivePtyIds: [...remainingLivePtyIds].sort() }
        : {})
    }
  }

  private getLivePtyIdsForWorktree(
    worktreeId: string,
    freshPtyIds?: ReadonlySet<string>
  ): Set<string> {
    const ptyIds = new Set<string>()
    for (const leaf of this.graph.leaves.values()) {
      if (
        leaf.worktreeId === worktreeId &&
        leaf.connected &&
        leaf.ptyId &&
        (!freshPtyIds || freshPtyIds.has(leaf.ptyId))
      ) {
        ptyIds.add(leaf.ptyId)
      }
    }
    for (const pty of this.graph.ptysById.values()) {
      if (
        pty.worktreeId === worktreeId &&
        pty.connected &&
        (!freshPtyIds || freshPtyIds.has(pty.ptyId))
      ) {
        ptyIds.add(pty.ptyId)
      }
    }
    return ptyIds
  }

  async hasTerminalsForWorktree(worktreeSelector: string): Promise<boolean> {
    const graphEpoch = this.captureReadyGraphEpoch()
    const worktree = await this.resolveWorktreeSelector(worktreeSelector)
    this.assertStableReadyGraph(graphEpoch)
    for (const leaf of this.graph.leaves.values()) {
      if (leaf.worktreeId === worktree.id && leaf.ptyId) {
        return true
      }
    }
    for (const pty of this.graph.ptysById.values()) {
      if (pty.worktreeId === worktree.id && pty.connected) {
        return true
      }
    }
    return false
  }

  markRendererReloading(windowId: number): void {
    if (windowId !== this.graph.authoritativeWindowId) {
      return
    }
    if (this.graph.graphStatus !== 'ready') {
      return
    }
    // Why: any renderer reload tears down the published live graph, so live
    // terminal handles must become stale immediately instead of being reused
    // against whatever the renderer rebuilds next.
    this.graph.rendererGraphEpoch += 1
    this.graph.graphStatus = 'reloading'
    this.rememberDetachedPreAllocatedLeaves()
    this.graph.handles.clear()
    this.graph.handleByLeafKey.clear()
    // Why: handleByPtyId maps ptyId → pre-allocated CLI handle (ORCA_TERMINAL_HANDLE).
    // These must survive renderer reloads so CLI agents can keep controlling the
    // same terminal across graph rebuilds — adoptPreAllocatedHandle re-links
    // them when the new graph arrives.
    this.rejectAllWaiters('terminal_handle_stale')
    this.refreshWritableFlags()
  }

  markGraphReady(windowId: number): void {
    if (windowId !== this.graph.authoritativeWindowId) {
      return
    }
    this.graph.graphStatus = 'ready'
    this.refreshWritableFlags()
  }

  markGraphUnavailable(windowId: number): void {
    if (windowId !== this.graph.authoritativeWindowId) {
      return
    }
    // Why: once the authoritative renderer graph disappears, Orca must fail
    // closed for live-terminal operations instead of guessing from old state.
    if (this.graph.graphStatus !== 'unavailable') {
      this.graph.rendererGraphEpoch += 1
    }
    this.graph.graphStatus = 'unavailable'
    this.graph.authoritativeWindowId = null
    this.rememberDetachedPreAllocatedLeaves()
    this.graph.tabs.clear()
    this.graph.leaves.clear()
    this.graph.leavesByPtyId.clear()
    this.graph.handles.clear()
    this.graph.handleByLeafKey.clear()
    // Why: same as markRendererReloading — pre-allocated CLI handles must
    // survive graph unavailability so they can be re-adopted on reconnect.
    this.rejectAllWaiters('terminal_handle_stale')
  }

  private assertGraphReady(): void {
    if (this.graph.graphStatus !== 'ready') {
      throw new Error('runtime_unavailable')
    }
  }

  private captureReadyGraphEpoch(): number {
    this.assertGraphReady()
    return this.graph.rendererGraphEpoch
  }

  private assertStableReadyGraph(expectedGraphEpoch: number): void {
    if (
      this.graph.graphStatus !== 'ready' ||
      this.graph.rendererGraphEpoch !== expectedGraphEpoch
    ) {
      throw new Error('runtime_unavailable')
    }
  }

  private resolveFolderWorkspaceConnectionId(workspace: FolderWorkspace): string | null {
    const repos = this.store?.getRepos() ?? []
    const projectGroups = this.store?.getProjectGroups?.() ?? []
    const connection = inferFolderWorkspacePathConnection({
      folderPath: workspace.folderPath,
      projectGroupId: workspace.projectGroupId,
      connectionId: workspace.connectionId ?? null,
      projectGroups,
      repos
    })
    if (connection.kind === 'ambiguous') {
      // Why: a single PTY can only be spawned on one runtime target; mixed
      // child repo connections need an explicit V2 routing decision.
      throw new Error('folder_workspace_connection_ambiguous')
    }
    return connection.kind === 'ssh' ? connection.connectionId : null
  }

  private async resolveFolderWorkspaceLaunchScope(
    selector: string
  ): Promise<TerminalWorkspaceLaunchScope | null> {
    const workspaceSelector = selector.startsWith('id:') ? selector.slice(3) : selector
    const parsed = parseWorkspaceKey(workspaceSelector)
    if (parsed?.type !== 'folder') {
      return null
    }
    const workspace = this.store
      ?.getFolderWorkspaces?.()
      .find((entry) => entry.id === parsed.folderWorkspaceId)
    if (!workspace) {
      throw new Error('selector_not_found')
    }
    if (!this.store) {
      throw new Error('runtime_unavailable')
    }
    const status = await getFolderWorkspacePathStatus(
      this.store,
      { scope: 'folder-workspace', folderWorkspaceId: workspace.id },
      { getRemoteFilesystemProvider }
    )
    assertFolderWorkspacePathUsable(status)
    return {
      id: folderWorkspaceKey(workspace.id),
      path: workspace.folderPath,
      connectionId: this.resolveFolderWorkspaceConnectionId(workspace),
      repo: null,
      folderWorkspace: workspace
    }
  }

  private folderWorkspaceToResolvedWorktree(folderWorkspace: FolderWorkspace): ResolvedWorktree {
    const worktree = folderWorkspaceToWorktree(folderWorkspace)
    return {
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
    }
  }

  private resolveWorkspaceTerminalStartupCwd(
    workspace: Pick<TerminalWorkspaceLaunchScope, 'path'>,
    requestedCwd?: string | null
  ): string | undefined {
    return resolveTerminalStartupCwd(workspace.path, requestedCwd)
  }

  private async resolveTerminalWorkspaceLaunchScope(
    selector: string
  ): Promise<TerminalWorkspaceLaunchScope> {
    const floatingTerminalSelector =
      selector === FLOATING_TERMINAL_WORKTREE_ID ||
      selector === `id:${FLOATING_TERMINAL_WORKTREE_ID}`
    if (floatingTerminalSelector) {
      // Why: the floating sentinel is terminal-only; other workspace APIs must
      // keep rejecting it because there is no backing repo/worktree record.
      return {
        id: FLOATING_TERMINAL_WORKTREE_ID,
        path: homedir(),
        connectionId: null,
        repo: null,
        folderWorkspace: null
      }
    }

    const folderScope = await this.resolveFolderWorkspaceLaunchScope(selector)
    if (folderScope) {
      return folderScope
    }

    const workspaceSelector = selector.startsWith('id:') ? selector.slice(3) : selector
    const parsed = parseWorkspaceKey(workspaceSelector)
    const worktreeSelector = parsed?.type === 'worktree' ? `id:${parsed.worktreeId}` : selector
    const worktree = await this.resolveWorktreeSelector(worktreeSelector)
    const repo = this.store?.getRepo(worktree.repoId) ?? null
    return {
      id: worktree.id,
      path: worktree.path,
      // Why getRepoProviderConnectionKey (not repo.connectionId directly): a
      // repo bound to a Dev Server may have only devServerId set, never
      // connectionId — raw repo.connectionId silently resolved to null for
      // those repos, making every terminal spawn reject with "no local
      // shell" even though a real remote PTY provider was registered.
      connectionId: (repo ? getRepoProviderConnectionKey(repo) : null) ?? null,
      repo,
      folderWorkspace: null
    }
  }

  private buildTerminalWorkspaceEnv(
    scope: TerminalWorkspaceLaunchScope,
    baseEnv: Record<string, string>,
    paneKey: string,
    tabId: string,
    agentTeamsEnv?: Record<string, string>
  ): Record<string, string> {
    const cleanBaseEnv = { ...baseEnv }
    for (const key of AGENT_HOOK_RUNTIME_ENV_KEYS) {
      delete cleanBaseEnv[key]
    }
    const env = {
      ...cleanBaseEnv,
      ...agentTeamsEnv,
      ...this.buildAgentHookPtyEnv?.(),
      ORCA_PANE_KEY: paneKey,
      ORCA_TAB_ID: tabId,
      ORCA_WORKTREE_ID: scope.id
    }
    if (!scope.folderWorkspace) {
      return env
    }
    return {
      ...env,
      ORCA_WORKSPACE_ID: scope.id,
      ORCA_PROJECT_GROUP_ID: scope.folderWorkspace.projectGroupId,
      ORCA_WORKSPACE_ROOT: scope.folderWorkspace.folderPath
    }
  }

  private getValidatedExplicitWorktreeIdSelector(selector: string | undefined): string | null {
    const worktreeId = getExplicitWorktreeIdSelector(selector)
    if (
      worktreeId &&
      !worktreeId.includes(WORKTREE_ID_SEPARATOR) &&
      this.store?.getRepo(worktreeId)
    ) {
      // Why: registered repo ids are known-invalid worktree ids, so reject them
      // before exact-id fast paths or Git/SSH worktree scans can hide the mistake.
      throw new WorktreeIdRequiresFullPathError()
    }
    return worktreeId
  }

  private async resolveWorktreeSelector(selector: string): Promise<ResolvedWorktree> {
    const explicitWorktreeId = this.getValidatedExplicitWorktreeIdSelector(selector)
    const worktrees = await this.listResolvedWorktrees()
    let candidates: ResolvedWorktree[]

    if (selector === 'active') {
      throw new Error('selector_not_found')
    }

    if (selector.startsWith('id:')) {
      const worktreeId = explicitWorktreeId ?? selector.slice(3)
      candidates = worktrees.filter((worktree) => worktree.id === worktreeId)
      if (candidates.length === 0) {
        const parsed = splitWorktreeIdForFilesystem(worktreeId)
        const repo = parsed ? this.store?.getRepo(parsed.repoId) : null
        const fallback =
          repo?.connectionId && this.store?.getWorktreeMeta(worktreeId)
            ? this.buildResolvedWorktreeFromId(worktreeId)
            : null
        if (fallback !== null) {
          candidates = [fallback]
        }
      }
    } else if (selector.startsWith('path:')) {
      candidates = worktrees.filter((worktree) =>
        runtimePathsEqual(worktree.path, selector.slice(5))
      )
      if (candidates.length > 1) {
        // Why: registering another worktree from the same Git repo makes git
        // report the same physical worktree path under multiple repo IDs.
        // A path selector is already exact, so prefer the first resolved row
        // instead of surfacing a duplicate-registration ambiguity.
        candidates = [candidates[0]]
      }
    } else if (selector.startsWith('branch:')) {
      const branchSelector = selector.slice(7)
      candidates = worktrees.filter((worktree) =>
        branchSelectorMatches(worktree.branch, branchSelector)
      )
    } else if (selector.startsWith('name:')) {
      // Keep display-name matching exact so selector behavior stays deterministic
      // and duplicate names use the same ambiguity path as other selectors.
      candidates = worktrees.filter((worktree) => worktree.displayName === selector.slice(5))
    } else if (selector.startsWith('issue:')) {
      candidates = worktrees.filter(
        (worktree) =>
          worktree.linkedIssue !== null && String(worktree.linkedIssue) === selector.slice(6)
      )
    } else {
      candidates = worktrees.filter(
        (worktree) =>
          worktree.id === selector ||
          runtimePathsEqual(worktree.path, selector) ||
          branchSelectorMatches(worktree.branch, selector)
      )
    }

    if (candidates.length === 1) {
      return candidates[0]
    }
    if (candidates.length > 1) {
      throw new Error('selector_ambiguous')
    }
    throw new Error('selector_not_found')
  }

  private readonly worktreeLineageCommands = new RuntimeWorktreeLineageCommands({
    getStore: () => this.store,
    getOrchestrationDbField: () => this._orchestrationDb,
    getOrchestrationDbIfAvailable: () => this.getOrchestrationDbIfAvailable(),
    listResolvedWorktrees: () => this.listResolvedWorktrees(),
    resolveWorktreeSelector: (selector) => this.resolveWorktreeSelector(selector),
    showTerminal: (handle) => this.showTerminal(handle),
    peekResolvedWorktreeCache: () => this.resolvedWorktreeCommands.peekCache()
  })

  validateLineageParent: RuntimeWorktreeLineageCommands['validateLineageParent'] =
    this.worktreeLineageCommands.validateLineageParent.bind(this.worktreeLineageCommands)
  resolveLineageForWorktreeCreate: RuntimeWorktreeLineageCommands['resolveLineageForWorktreeCreate'] =
    this.worktreeLineageCommands.resolveLineageForWorktreeCreate.bind(this.worktreeLineageCommands)
  hydrateInferredWorktreeLineage: RuntimeWorktreeLineageCommands['hydrateInferredWorktreeLineage'] =
    this.worktreeLineageCommands.hydrateInferredWorktreeLineage.bind(this.worktreeLineageCommands)
  listWorktreeLineage: RuntimeWorktreeLineageCommands['listWorktreeLineage'] =
    this.worktreeLineageCommands.listWorktreeLineage.bind(this.worktreeLineageCommands)
  listWorkspaceLineage: RuntimeWorktreeLineageCommands['listWorkspaceLineage'] =
    this.worktreeLineageCommands.listWorkspaceLineage.bind(this.worktreeLineageCommands)

  private async resolveRepoSelector(selector: string): Promise<Repo> {
    if (!this.store) {
      throw new Error('repo_not_found')
    }
    const repos = this.store.getRepos()
    let candidates: Repo[]

    if (selector.startsWith('id:')) {
      candidates = repos.filter((repo) => repo.id === selector.slice(3))
    } else if (selector.startsWith('path:')) {
      candidates = repos.filter((repo) => runtimePathsEqual(repo.path, selector.slice(5)))
    } else if (selector.startsWith('name:')) {
      candidates = repos.filter((repo) => repo.displayName === selector.slice(5))
    } else {
      candidates = repos.filter(
        (repo) =>
          repo.id === selector ||
          runtimePathsEqual(repo.path, selector) ||
          repo.displayName === selector
      )
    }

    if (candidates.length === 1) {
      return candidates[0]
    }
    if (candidates.length > 1) {
      throw new Error('selector_ambiguous')
    }
    throw new Error('repo_not_found')
  }

  private requireStore(): Store {
    if (!this.store) {
      throw new Error('runtime_unavailable')
    }
    return this.store as unknown as Store
  }

  private buildResolvedWorktreeFromId(worktreeId: string): ResolvedWorktree | null {
    const parsed = splitWorktreeIdForFilesystem(worktreeId)
    if (!parsed?.repoId || !parsed.worktreePath) {
      return null
    }
    const repo = this.store?.getRepos().find((entry) => entry.id === parsed.repoId)
    const git = {
      path: parsed.worktreePath,
      head: '',
      branch: '',
      isBare: false,
      isMainWorktree: repo ? areWorktreePathsEqual(parsed.worktreePath, repo.path) : false
    }
    const meta = this.store?.getWorktreeMeta(worktreeId)
    const merged = mergeWorktree(parsed.repoId, git, meta, repo?.displayName)
    return {
      ...merged,
      id: worktreeId,
      parentWorktreeId: null,
      childWorktreeIds: [],
      lineage: null,
      git,
      displayName: merged.displayName,
      comment: merged.comment
    }
  }

  private listKnownResolvedWorktreesForExplicitTarget(
    targetWorktreeId: string,
    targetWorktree: ResolvedWorktree | null
  ): ResolvedWorktree[] {
    if (!this.store || !targetWorktree) {
      return []
    }
    const target = splitWorktreeIdForFilesystem(targetWorktreeId)
    if (!target?.repoId || !target.worktreePath) {
      return []
    }
    const worktreeIds = new Set(
      Object.keys(this.store.getAllWorktreeMeta()).filter((worktreeId) => {
        const parsed = splitWorktreeIdForFilesystem(worktreeId)
        return (
          parsed?.repoId === target.repoId &&
          Boolean(parsed.worktreePath) &&
          (isPathInsideOrEqual(target.worktreePath, parsed.worktreePath) ||
            isPathInsideOrEqual(parsed.worktreePath, target.worktreePath))
        )
      })
    )
    worktreeIds.add(targetWorktreeId)

    const resolved: ResolvedWorktree[] = []
    for (const worktreeId of worktreeIds) {
      const worktree =
        worktreeId === targetWorktreeId
          ? targetWorktree
          : this.buildResolvedWorktreeFromId(worktreeId)
      if (worktree) {
        resolved.push(worktree)
      }
    }
    return resolved
  }

  private readonly resolvedWorktreeCommands = new RuntimeResolvedWorktreeCommands({
    getStore: () => this.store,
    requireStore: () => this.requireStore(),
    notifyWorktreesChanged: (repoId) => this.notifyWorktreesChanged(repoId),
    notifierWorktreesChanged: (repoId, renamed) => this.notifier?.worktreesChanged(repoId, renamed),
    emitWorktreesChangedClientEvent: (repoId) =>
      this.emitClientEvent({ type: 'worktreesChanged', repoId })
  })

  listResolvedWorktrees: RuntimeResolvedWorktreeCommands['listResolvedWorktrees'] =
    this.resolvedWorktreeCommands.listResolvedWorktrees.bind(this.resolvedWorktreeCommands)
  getResolvedWorktreeMap: RuntimeResolvedWorktreeCommands['getResolvedWorktreeMap'] =
    this.resolvedWorktreeCommands.getResolvedWorktreeMap.bind(this.resolvedWorktreeCommands)
  invalidateResolvedWorktreeCache: RuntimeResolvedWorktreeCommands['invalidateResolvedWorktreeCache'] =
    this.resolvedWorktreeCommands.invalidateResolvedWorktreeCache.bind(
      this.resolvedWorktreeCommands
    )
  notifyBranchRenamed: RuntimeResolvedWorktreeCommands['notifyBranchRenamed'] =
    this.resolvedWorktreeCommands.notifyBranchRenamed.bind(this.resolvedWorktreeCommands)
  notifyWorktreeFolderRenamed: RuntimeResolvedWorktreeCommands['notifyWorktreeFolderRenamed'] =
    this.resolvedWorktreeCommands.notifyWorktreeFolderRenamed.bind(this.resolvedWorktreeCommands)

  notifyFolderWorkspaceChanged(): void {
    this.invalidateResolvedWorktreeCache()
    this.notifyReposChanged()
  }

  private recordPtyWorktree(
    ptyId: string,
    worktreeId: string,
    state: Partial<
      Pick<
        RuntimePtyWorktreeRecord,
        'connected' | 'lastOutputAt' | 'preview' | 'tabId' | 'paneKey' | 'title' | 'connectionId'
      >
    > = {}
  ): RuntimePtyWorktreeRecord {
    let pty = this.graph.ptysById.get(ptyId)
    if (!pty) {
      const titleObservedAt = state.title ? this.nextTitleObservationSequence() : null
      pty = {
        ptyId,
        worktreeId,
        connectionId: state.connectionId ?? parseAppSshPtyId(ptyId)?.connectionId ?? null,
        tabId: state.tabId ?? null,
        paneKey: state.paneKey ?? null,
        launchConfig: null,
        launchToken: null,
        launchAgent: null,
        foregroundAgent: null,
        connected: state.connected ?? true,
        disconnectedAt: state.connected === false ? Date.now() : null,
        lastExitCode: null,
        lastAgentStatus: null,
        lastOscTitle: null,
        lastOscTitleAt: null,
        managementTitle: null,
        managementTitleAt: null,
        title: state.title ?? null,
        titleUpdatedAt: titleObservedAt,
        lastOutputAt: state.lastOutputAt ?? null,
        tailBuffer: [],
        tailPartialLine: '',
        tailPendingAnsi: '',
        tailRedrawCursor: null,
        tailTruncated: false,
        tailLinesTotal: 0,
        preview: state.preview ?? '',
        waitBlockedAt: null
      }
      if (state.title) {
        this.setPtyManagementTitleFromObservedTitle(pty, state.title, titleObservedAt ?? 0)
      }
      this.graph.ptysById.set(ptyId, pty)
      // Why: restored/controller-discovered PTYs learn their worktree here
      // without registerPty(), so URL enrichment must bind at this source.
      advertisedUrlWatcher.bindPty(ptyId, worktreeId)
      serveSimStateWatcher.bindPty(ptyId, worktreeId)
      return pty
    }

    pty.worktreeId = worktreeId
    if (state.connectionId !== undefined) {
      pty.connectionId = state.connectionId
    }
    if (state.tabId !== undefined) {
      pty.tabId = state.tabId
    }
    if (state.paneKey !== undefined) {
      pty.paneKey = state.paneKey
    }
    if (state.connected !== undefined) {
      pty.connected = state.connected
      pty.disconnectedAt = state.connected ? null : (pty.disconnectedAt ?? Date.now())
    }
    if (state.lastOutputAt !== undefined) {
      pty.lastOutputAt = maxTimestamp(pty.lastOutputAt, state.lastOutputAt)
    }
    if (state.preview !== undefined && state.preview.length > 0) {
      pty.preview = state.preview
    }
    if (state.title !== undefined && state.title !== null && state.title.length > 0) {
      const observedAt = this.nextTitleObservationSequence()
      pty.title = state.title
      pty.titleUpdatedAt = observedAt
      this.setPtyManagementTitleFromObservedTitle(pty, state.title, observedAt)
    }
    // Why: recordPtyWorktree is the common lifecycle point for every path that
    // resolves a PTY's worktree, including renderer restore and controller list.
    advertisedUrlWatcher.bindPty(ptyId, worktreeId)
    serveSimStateWatcher.bindPty(ptyId, worktreeId)
    return pty
  }

  private makeRuntimePaneKey(
    leaf: Pick<RuntimeSyncedLeaf, 'tabId' | 'leafId' | 'paneRuntimeId'>
  ): string {
    return isTerminalLeafId(leaf.leafId)
      ? makePaneKey(leaf.tabId, leaf.leafId)
      : `${leaf.tabId}:${leaf.paneRuntimeId}`
  }

  private getOrCreatePtyWorktreeRecord(ptyId: string): RuntimePtyWorktreeRecord | null {
    const existing = this.graph.ptysById.get(ptyId)
    if (existing) {
      return existing
    }
    const inferredWorktreeId = inferWorktreeIdFromPtyId(ptyId)
    if (!inferredWorktreeId) {
      return null
    }
    // Why: daemon-backed PTY session IDs are prefixed with the worktree ID so
    // mobile summaries survive renderer graph gaps and Electron reloads.
    return this.recordPtyWorktree(ptyId, inferredWorktreeId)
  }

  /**
   * Synchronizes PTY tracking records with the running daemon sessions,
   * querying their foreground agent states.
   */
  private async refreshPtyWorktreeRecordsFromController(
    resolvedWorktrees: ResolvedWorktree[],
    targetWorktreeId: string | null = null
  ): Promise<Set<string> | null> {
    if (!this.ptyController?.listProcesses) {
      return null
    }
    const sessionsResult = await withTimeoutResult(
      this.ptyController.listProcesses(),
      PTY_CONTROLLER_LIST_TIMEOUT_MS
    )
    if (!sessionsResult.ok) {
      // Why: a transient controller failure is not evidence that retained PTYs exited.
      return null
    }
    const sessions = sessionsResult.value
    const livePtyIds = new Set(sessions.map((session) => session.id))
    for (const session of sessions) {
      this.adoptControllerTerminalHandle(session.id, session.terminalHandle)
      const worktreeId =
        inferWorktreeIdFromPtyId(session.id) ??
        findResolvedWorktreeIdForPath(resolvedWorktrees, session.cwd)
      if (targetWorktreeId && worktreeId !== targetWorktreeId) {
        continue
      }
      if (worktreeId) {
        this.recordPtyWorktree(session.id, worktreeId, {
          connected: true
        })
      }
      // Why: fire-and-forget so this listing hot path (listTerminals/getWorktreePs)
      // does not serialize a relay round-trip per session — and a throwing snapshot
      // listener cannot abort the liveness sweep below.
      this.refreshPtyForegroundAgent(session.id)
    }
    for (const pty of this.graph.ptysById.values()) {
      if (!livePtyIds.has(pty.ptyId) && !this.leafExistsForPty(pty.ptyId)) {
        pty.connected = false
        pty.disconnectedAt ??= Date.now()
      }
    }
    this.pruneDisconnectedPtyRecords()
    return livePtyIds
  }

  private pruneDisconnectedPtyTranscript(pty: RuntimePtyWorktreeRecord): void {
    if (pty.connected) {
      return
    }
    // Why: disconnected PTY records can stay addressable for status/exit reads,
    // but their retained transcripts must not accumulate after the process dies.
    pty.tailBuffer = []
    pty.tailPartialLine = ''
    pty.tailPendingAnsi = ''
    pty.tailRedrawCursor = null
    pty.tailTruncated = false
    pty.tailLinesTotal = 0
    pty.waitBlockedAt = null
    // Why: the tail is now empty, so the memoized wait scan must not be reused as
    // the next chunk's "previous" state — clear it so onPtyData recomputes from
    // the reset tail if this record resumes output (adoption/reattach).
    pty.tailWaitState = undefined
  }

  private pruneDisconnectedPtyRecords(): void {
    const retained = [...this.graph.ptysById.values()]
      .filter((pty) => !pty.connected && !this.leafExistsForPty(pty.ptyId))
      .sort((a, b) => (a.disconnectedAt ?? 0) - (b.disconnectedAt ?? 0))
    const staleCount = Math.max(0, retained.length - DISCONNECTED_PTY_RECORD_MAX)
    for (const stale of retained.slice(0, staleCount)) {
      // Why: exited runtime-owned PTYs stay readable after exit, but long-lived
      // runtimes can churn through many background sessions. Bound the archive.
      this.dropDisconnectedPtyRecord(stale.ptyId)
    }
  }

  private dropDisconnectedPtyRecord(ptyId: string): void {
    // Why: pruning can remove a PTY without the normal exit callback.
    serveSimStateWatcher.unbindPty(ptyId)
    this.graph.ptysById.delete(ptyId)
    this.ptyTranscripts.recentPtyOutputById.delete(ptyId)
    this.clearWaitBlockedCheckState(ptyId)
    this.ptyTranscripts.recentPtyPathCandidatesById.delete(ptyId)
    this.ptyTranscripts.ptyOutputSequenceById.delete(ptyId)
    this.ptyTranscripts.agentStatusOscProcessorsByPtyId.delete(ptyId)
    this.ptyTranscripts.terminalSpawnCommandsByPtyId.delete(ptyId)
    this.disposePtyTitleTracker(ptyId)
    this.ptyTranscripts.oscTitleScanTailByPtyId.delete(ptyId)
    this.ptyTranscripts.osc7ScanTailByPtyId.delete(ptyId)
    this.ptyTranscripts.terminalCwdByPtyId.delete(ptyId)
    this.ptyTranscripts.terminalFileUriHostnameByPtyId.delete(ptyId)
    this.clearAgentRowSnapshotsForPty(ptyId)
    const handle = this.graph.handleByPtyId.get(ptyId)
    if (handle) {
      // Why: pruning can remove a PTY without onPtyExit firing; release any agent
      // team owned by this leader handle so it does not leak.
      this.claudeAgentTeams.removeTeamForLeaderHandle(handle)
      this.graph.handleByPtyId.delete(ptyId)
      const record = this.graph.handles.get(handle)
      if (record?.tabId.startsWith('pty:')) {
        this.graph.handles.delete(handle)
      }
    }
  }

  private leafExistsForPty(ptyId: string): boolean {
    return (this.graph.leavesByPtyId.get(ptyId)?.length ?? 0) > 0
  }

  private rebuildLeafPtyIndex(): void {
    const next = new Map<string, RuntimeLeafRecord[]>()
    for (const leaf of this.graph.leaves.values()) {
      if (!leaf.ptyId) {
        continue
      }
      const leaves = next.get(leaf.ptyId)
      if (leaves) {
        leaves.push(leaf)
      } else {
        next.set(leaf.ptyId, [leaf])
      }
    }
    this.graph.leavesByPtyId = next
  }

  private getLeavesForPty(ptyId: string): RuntimeLeafRecord[] {
    return this.graph.leavesByPtyId.get(ptyId) ?? []
  }

  private getSummaryForRuntimeWorktreeId(
    summaries: Map<string, RuntimeWorktreePsSummary>,
    resolvedWorktrees: ResolvedWorktree[],
    runtimeWorktreeId: string
  ): RuntimeWorktreePsSummary | null {
    const exact = summaries.get(runtimeWorktreeId)
    if (exact) {
      return exact
    }
    const parsed = parseRuntimeWorktreeId(runtimeWorktreeId)
    if (!parsed) {
      return null
    }
    const resolved = resolvedWorktrees.find(
      (worktree) =>
        worktree.repoId === parsed.repoId &&
        areWorktreePathsEqual(worktree.path, parsed.worktreePath)
    )
    return resolved ? (summaries.get(resolved.id) ?? null) : null
  }

  private buildTerminalSummary(
    leaf: RuntimeLeafRecord,
    worktreesById: Map<string, ResolvedWorktree>
  ): RuntimeTerminalSummary {
    const worktree = worktreesById.get(leaf.worktreeId)
    const tab = this.graph.tabs.get(leaf.tabId) ?? null

    return {
      handle: this.issueHandle(leaf),
      ptyId: leaf.ptyId,
      worktreeId: leaf.worktreeId,
      worktreePath: worktree?.path ?? '',
      branch: worktree?.branch ?? '',
      tabId: leaf.tabId,
      leafId: leaf.leafId,
      title: getLatestLeafTitle(leaf, tab?.title ?? null),
      connected: leaf.connected,
      writable: leaf.writable,
      lastOutputAt: leaf.lastOutputAt,
      preview: leaf.preview
    }
  }

  private readonly mobileSessionNotifyCommands = new RuntimeMobileSessionNotifyCommands({
    getStore: () => this.store,
    getAgentBrowserBridge: () => this.agentBrowserBridge,
    getOffscreenBrowserBackend: () => this.offscreenBrowserBackend,
    getGraph: () => this.graph,
    getMobileSessionTabsByWorktree: () => this.mobileSessionTabsByWorktree,
    getMobileSessionTabListeners: () => this.mobileSessionTabListeners,
    getMobileSessionTabsNotifyCoalescer: () => this.mobileSessionTabsNotifyCoalescer,
    getLatestAgentStatusByPaneKey: () => this.latestAgentStatusByPaneKey,
    resolveWorktreeSelector: (selector) => this.resolveWorktreeSelector(selector),
    getValidatedExplicitWorktreeIdSelector: (selector) =>
      this.getValidatedExplicitWorktreeIdSelector(selector),
    hasServeOwnedPtyBinding: (tab) => this.hasServeOwnedPtyBinding(tab),
    getMobileSessionSnapshotTabIdentityKeys: (tab) =>
      this.getMobileSessionSnapshotTabIdentityKeys(tab),
    mergeMobileSessionSnapshotTabs: (baseTabs, extraTabs) =>
      this.mergeMobileSessionSnapshotTabs(baseTabs, extraTabs),
    mergeMobileSessionTabGroups: (worktreeId, groups, terminalTabs, activeTab) =>
      this.mergeMobileSessionTabGroups(worktreeId, groups, terminalTabs, activeTab),
    getHeadlessMobileSessionGroupId: (worktreeId) =>
      this.getHeadlessMobileSessionGroupId(worktreeId),
    hydrateHeadlessMobileSessionTabsFromWorkspaceSession: (worktreeId, options) =>
      this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(worktreeId, options),
    getLeafKey: (tabId, leafId) => this.getLeafKey(tabId, leafId),
    issuePtyHandle: (pty) => this.issuePtyHandle(pty),
    recordPtyWorktree: (ptyId, worktreeId, state) =>
      this.recordPtyWorktree(ptyId, worktreeId, state)
  })

  syncMobileSessionTabs: RuntimeMobileSessionNotifyCommands['syncMobileSessionTabs'] =
    this.mobileSessionNotifyCommands.syncMobileSessionTabs.bind(this.mobileSessionNotifyCommands)
  isHeadlessMobileSessionPublication: RuntimeMobileSessionNotifyCommands['isHeadlessMobileSessionPublication'] =
    this.mobileSessionNotifyCommands.isHeadlessMobileSessionPublication.bind(
      this.mobileSessionNotifyCommands
    )
  notifyMobileSessionTabsChanged: RuntimeMobileSessionNotifyCommands['notifyMobileSessionTabsChanged'] =
    this.mobileSessionNotifyCommands.notifyMobileSessionTabsChanged.bind(
      this.mobileSessionNotifyCommands
    )
  notifyMobileSessionTabsChangedNow: RuntimeMobileSessionNotifyCommands['notifyMobileSessionTabsChangedNow'] =
    this.mobileSessionNotifyCommands.notifyMobileSessionTabsChangedNow.bind(
      this.mobileSessionNotifyCommands
    )
  notifyMobileSessionTabSnapshots: RuntimeMobileSessionNotifyCommands['notifyMobileSessionTabSnapshots'] =
    this.mobileSessionNotifyCommands.notifyMobileSessionTabSnapshots.bind(
      this.mobileSessionNotifyCommands
    )
  getMobileSessionTabsForWorktree: RuntimeMobileSessionNotifyCommands['getMobileSessionTabsForWorktree'] =
    this.mobileSessionNotifyCommands.getMobileSessionTabsForWorktree.bind(
      this.mobileSessionNotifyCommands
    )
  resolveMobileMarkdownWorktreeId: RuntimeMobileSessionNotifyCommands['resolveMobileMarkdownWorktreeId'] =
    this.mobileSessionNotifyCommands.resolveMobileMarkdownWorktreeId.bind(
      this.mobileSessionNotifyCommands
    )
  getLiveBrowserTabsByPageId: RuntimeMobileSessionNotifyCommands['getLiveBrowserTabsByPageId'] =
    this.mobileSessionNotifyCommands.getLiveBrowserTabsByPageId.bind(
      this.mobileSessionNotifyCommands
    )
  toMobileSessionTabsResult: RuntimeMobileSessionNotifyCommands['toMobileSessionTabsResult'] =
    this.mobileSessionNotifyCommands.toMobileSessionTabsResult.bind(
      this.mobileSessionNotifyCommands
    )
  getPersistedSshPtyIdForMobileTerminalTab: RuntimeMobileSessionNotifyCommands['getPersistedSshPtyIdForMobileTerminalTab'] =
    this.mobileSessionNotifyCommands.getPersistedSshPtyIdForMobileTerminalTab.bind(
      this.mobileSessionNotifyCommands
    )
  findPtyForMobileTerminalTab: RuntimeMobileSessionNotifyCommands['findPtyForMobileTerminalTab'] =
    this.mobileSessionNotifyCommands.findPtyForMobileTerminalTab.bind(
      this.mobileSessionNotifyCommands
    )

  // Why: group address resolution (Section 4.5) needs to query per-handle agent
  // status without throwing on stale handles, so this returns null on any error.
  getAgentStatusForHandle(handle: string): string | null {
    try {
      const ptyId = this.getTerminalAgentStatusPtyId(handle)
      return this.getTerminalAgentStatusSnapshot(handle, ptyId).titleStatus
    } catch {
      return null
    }
  }

  getAgentStatusOrchestrationContextForPaneKey(
    paneKey: string
  ): AgentStatusOrchestrationContext | undefined {
    const handle = this.getTerminalHandleForPaneKey(paneKey)
    if (!handle) {
      return undefined
    }
    return this.getAgentStatusOrchestrationContextForHandle(handle)
  }

  getAgentStatusTerminalHandleForPaneKey(paneKey: string): string | undefined {
    return this.getTerminalHandleForPaneKey(paneKey) ?? undefined
  }

  getAgentStatusLaunchConfigForPaneKey(
    paneKey: string,
    args?: { launchToken?: string }
  ): SleepingAgentLaunchConfig | undefined {
    const pty = this.getPtyRecordForPaneKey(paneKey)
    if (!pty?.launchConfig) {
      return undefined
    }
    if (pty.launchToken === null || pty.launchToken !== args?.launchToken) {
      return undefined
    }
    return copySleepingAgentLaunchConfig(pty.launchConfig)
  }

  private getOrchestrationDbIfAvailable(): OrchestrationDb | null {
    try {
      return this._orchestrationDb ?? this.getOrchestrationDb()
    } catch {
      return this._orchestrationDb
    }
  }

  private buildAgentOrchestrationByPaneKey():
    | Record<string, AgentStatusOrchestrationContext>
    | undefined {
    const db = this.getOrchestrationDbIfAvailable()
    if (!db) {
      return undefined
    }
    const contexts: Record<string, AgentStatusOrchestrationContext> = {}
    for (const leaf of this.graph.leaves.values()) {
      if (!leaf.ptyId) {
        continue
      }
      const handle = this.issueHandle(leaf)
      const context = this.getAgentStatusOrchestrationContextForHandle(handle, db)
      if (context) {
        contexts[this.makeRuntimePaneKey(leaf)] = context
      }
    }
    for (const pty of this.graph.ptysById.values()) {
      if (!pty.paneKey || contexts[pty.paneKey]) {
        continue
      }
      const handle = this.issuePtyHandle(pty)
      const context = this.getAgentStatusOrchestrationContextForHandle(handle, db)
      if (context) {
        contexts[pty.paneKey] = context
      }
    }
    return Object.keys(contexts).length > 0 ? contexts : undefined
  }

  private getAgentStatusOrchestrationContextForHandle(
    handle: string,
    db = this.getOrchestrationDbIfAvailable()
  ): AgentStatusOrchestrationContext | undefined {
    // Why: active dispatches are authoritative for reused terminals. Completed
    // context is only useful while the corresponding done/recent row can still
    // be visible; after that it would stale-group unrelated future work.
    const dispatch =
      db?.getActiveDispatchForTerminal?.(handle) ??
      this.getRecentCompletedDispatchForTerminal(handle, db)
    if (!dispatch) {
      return undefined
    }
    const task = db?.getTask?.(dispatch.task_id)
    const display =
      typeof task?.spec === 'string'
        ? buildOrchestrationTaskDisplayMetadata({
            spec: task.spec,
            taskTitle: task.task_title,
            displayName: task.display_name
          })
        : { taskTitle: '', displayName: '' }
    const activeRun = dispatch.status === 'completed' ? undefined : db?.getActiveCoordinatorRun?.()
    const parentTerminalHandle =
      task?.created_by_terminal_handle ??
      (activeRun?.coordinator_handle && activeRun.coordinator_handle !== handle
        ? activeRun.coordinator_handle
        : undefined)
    const parentPaneKey = parentTerminalHandle
      ? this.getPaneKeyForTerminalHandle(parentTerminalHandle)
      : undefined

    return {
      taskId: dispatch.task_id,
      dispatchId: dispatch.id,
      ...(display.taskTitle ? { taskTitle: display.taskTitle } : {}),
      ...(display.displayName ? { displayName: display.displayName } : {}),
      ...(parentTerminalHandle ? { parentTerminalHandle } : {}),
      ...(parentPaneKey ? { parentPaneKey } : {}),
      ...(activeRun?.coordinator_handle ? { coordinatorHandle: activeRun.coordinator_handle } : {}),
      ...(activeRun?.id ? { orchestrationRunId: activeRun.id } : {})
    }
  }

  private getRecentCompletedDispatchForTerminal(
    handle: string,
    db = this.getOrchestrationDbIfAvailable()
  ): ReturnType<OrchestrationDb['getLatestDispatchForTerminal']> {
    const dispatch = db?.getLatestDispatchForTerminal?.(handle)
    if (dispatch?.status !== 'completed' || !dispatch.completed_at) {
      return undefined
    }
    const completedAtMs = Date.parse(
      dispatch.completed_at.includes('T')
        ? dispatch.completed_at
        : `${dispatch.completed_at.replace(' ', 'T')}Z`
    )
    if (!Number.isFinite(completedAtMs)) {
      return undefined
    }
    return Date.now() - completedAtMs <= AGENT_STATUS_STALE_AFTER_MS ? dispatch : undefined
  }

  private getTerminalHandleForPaneKey(paneKey: string): string | null {
    const parsed = parsePaneKey(paneKey)
    if (parsed) {
      const leaf = this.graph.leaves.get(this.getLeafKey(parsed.tabId, parsed.leafId))
      if (leaf?.ptyId) {
        return this.issueHandle(leaf)
      }
    }
    for (const pty of this.graph.ptysById.values()) {
      if (pty.paneKey === paneKey) {
        return this.issuePtyHandle(pty)
      }
    }
    return null
  }

  private getPtyRecordForPaneKey(paneKey: string): RuntimePtyWorktreeRecord | null {
    const parsed = parsePaneKey(paneKey)
    if (parsed) {
      const leaf = this.graph.leaves.get(this.getLeafKey(parsed.tabId, parsed.leafId))
      const pty = leaf?.ptyId ? this.graph.ptysById.get(leaf.ptyId) : undefined
      if (pty) {
        return pty
      }
    }
    for (const pty of this.graph.ptysById.values()) {
      if (pty.paneKey === paneKey) {
        return pty
      }
    }
    return null
  }

  private getPaneKeyForTerminalHandle(handle: string): string | null {
    const livePty = this.getLivePtyForHandle(handle)
    if (livePty?.pty.paneKey) {
      return livePty.pty.paneKey
    }
    const record = this.graph.handles.get(handle)
    if (!record || record.runtimeId !== this.runtimeId) {
      return null
    }
    if (!isTerminalLeafId(record.leafId)) {
      return null
    }
    return makePaneKey(record.tabId, record.leafId)
  }

  private setPtyManagementTitleFromObservedTitle(
    pty: RuntimePtyWorktreeRecord,
    title: string | null | undefined,
    observedAt: number
  ): void {
    const trimmed = title?.trim()
    if (!trimmed) {
      return
    }
    if (isClaudeManagementTitle(trimmed)) {
      pty.managementTitle = trimmed
      pty.managementTitleAt = observedAt
      return
    }
    if (
      detectAgentStatusFromTitle(trimmed) !== null &&
      observedAt >= (pty.managementTitleAt ?? -1)
    ) {
      pty.managementTitle = null
      pty.managementTitleAt = null
    }
  }

  private nextTitleObservationSequence(): number {
    this.titleObservationSequence += 1
    return this.titleObservationSequence
  }

  // Why: title detection is the tightest signal for agent presence, but a
  // Claude management title is negative evidence for task-capable activity.
  // Check pane-scoped titles before tab fallback, then retained ready-tail text,
  // stale title status, and foreground process.
  async isTerminalRunningAgent(handle: string): Promise<boolean> {
    try {
      const pty = this.getLivePtyForHandle(handle)
      if (pty) {
        const leaf = this.getPrimaryLeafForPty(pty.pty.ptyId)
        return await this.isPtyRunningAgent(pty.pty, leaf)
      }
      const { leaf } = this.getLiveLeafForHandle(handle)
      // Why: check both the leaf-level pane title (synced from the renderer's
      // runtimePaneTitlesByTabId) and the tab-level title. The tab title already
      // includes OSC-enriched agent indicators (e.g. ✳ prefix) synced from the
      // renderer's xterm instance.
      const paneTitle = getLatestLeafTitle(leaf, null)
      const paneTitleClassification = classifyAgentTitle(paneTitle)
      if (paneTitleClassification === 'agent') {
        return true
      }
      const tabTitle = this.graph.tabs.get(leaf.tabId)?.title?.trim() || null
      const tabTitleClassification = paneTitle === null ? classifyAgentTitle(tabTitle) : 'neutral'
      if (tabTitleClassification === 'agent') {
        return true
      }
      const waitText = buildTerminalWaitText(leaf.tailBuffer, leaf.tailPartialLine, leaf.preview)
      if (isKnownReadyPromptPreview(waitText)) {
        return true
      }
      const hasCurrentTitleEvidence = paneTitle !== null || tabTitle !== null
      if (leaf.lastAgentStatus !== null && !hasCurrentTitleEvidence) {
        return true
      }
      if (!leaf.ptyId || !this.ptyController) {
        return false
      }
      const fg = await this.ptyController.getForegroundProcess(leaf.ptyId)
      if (!fg) {
        return false
      }
      // Why: Claude's management UI runs under the Claude process but is not a
      // task-capable agent session. Suppress that process only; another foreground
      // agent can take over before titles update.
      const shouldSuppressClaudeForeground =
        paneTitleClassification === 'management' || tabTitleClassification === 'management'
      if (shouldSuppressClaudeForeground && isExpectedAgentProcess(fg, 'claude')) {
        return false
      }
      // Why: review-note delivery auto-submits with Enter. A generic non-shell
      // TUI can be focused in a terminal, but only known agent processes are safe.
      return await this.isRecognizedForegroundAgentProcess(leaf.ptyId, fg, {
        suppressClaude: shouldSuppressClaudeForeground
      })
    } catch {
      return false
    }
  }

  private async isPtyRunningAgent(
    pty: RuntimePtyWorktreeRecord,
    leaf: RuntimeLeafRecord | null = null
  ): Promise<boolean> {
    const leafTitle = leaf
      ? getLatestAgentCandidateTitle(
          { title: leaf.paneTitle, updatedAt: leaf.paneTitleUpdatedAt },
          { title: leaf.lastOscTitle, updatedAt: leaf.lastOscTitleAt }
        )
      : null
    const leafTitleClassification = classifyAgentTitle(leafTitle)
    if (leafTitleClassification === 'agent') {
      return true
    }
    const ptyTitle = getLatestAgentCandidateTitle(
      { title: pty.title, updatedAt: pty.titleUpdatedAt },
      { title: pty.lastOscTitle, updatedAt: pty.lastOscTitleAt }
    )
    const ptyTitleClassification = classifyAgentTitle(ptyTitle)
    if (leafTitle === null && ptyTitleClassification === 'agent') {
      return true
    }
    const managementTitleClassification = classifyLatestAgentTitle({
      title: pty.managementTitle,
      updatedAt: pty.managementTitleAt
    })
    const waitText = buildTerminalWaitText(pty.tailBuffer, pty.tailPartialLine, pty.preview)
    if (isKnownReadyPromptPreview(waitText)) {
      return true
    }
    // Why: stale status is only a fallback when no current title evidence
    // exists; neutral titles such as shells should clear it.
    if (
      pty.lastAgentStatus !== null &&
      leafTitle === null &&
      ptyTitle === null &&
      managementTitleClassification !== 'management'
    ) {
      return true
    }
    if (!this.ptyController) {
      return false
    }
    const fg = await this.ptyController.getForegroundProcess(pty.ptyId)
    if (!fg) {
      return false
    }
    const shouldSuppressClaudeForeground =
      leafTitle !== null
        ? leafTitleClassification === 'management'
        : managementTitleClassification === 'management'
    if (shouldSuppressClaudeForeground && isExpectedAgentProcess(fg, 'claude')) {
      return false
    }
    // Why: review-note delivery auto-submits with Enter. A generic non-shell
    // TUI can be focused in a terminal, but only known agent processes are safe.
    return await this.isRecognizedForegroundAgentProcess(pty.ptyId, fg, {
      suppressClaude: shouldSuppressClaudeForeground
    })
  }

  private async isRecognizedForegroundAgentProcess(
    ptyId: string,
    foregroundProcess: string,
    options: { suppressClaude?: boolean } = {}
  ): Promise<boolean> {
    const initialRecognition = recognizeAgentProcess(foregroundProcess)
    if (initialRecognition !== null) {
      return !(
        options.suppressClaude === true &&
        isExpectedAgentProcess(initialRecognition.processName, 'claude')
      )
    }
    if (!this.isAgentWrapperForegroundProcess(foregroundProcess) || !this.ptyController) {
      return false
    }
    const startedAt = Date.now()
    while (Date.now() - startedAt < FOREGROUND_AGENT_WRAPPER_RETRY_TIMEOUT_MS) {
      await new Promise((resolve) =>
        setTimeout(resolve, FOREGROUND_AGENT_WRAPPER_RETRY_INTERVAL_MS)
      )
      const refreshedProcess = await this.ptyController.getForegroundProcess(ptyId)
      const refreshedRecognition = recognizeAgentProcess(refreshedProcess)
      if (refreshedRecognition !== null) {
        return !(
          options.suppressClaude === true &&
          isExpectedAgentProcess(refreshedRecognition.processName, 'claude')
        )
      }
      if (!refreshedProcess || !this.isAgentWrapperForegroundProcess(refreshedProcess)) {
        return false
      }
    }
    return false
  }

  private isAgentWrapperForegroundProcess(processName: string): boolean {
    // Why: daemon/SSH PTYs can report the interpreter before their async
    // command-line cache resolves to the actual agent binary. Retry only
    // known wrappers, never arbitrary non-shell TUIs.
    return isAgentForegroundWrapperProcess(processName)
  }

  private getPrimaryLeafForPty(ptyId: string): RuntimeLeafRecord | null {
    return this.getLeavesForPty(ptyId)[0] ?? null
  }

  private readonly terminalMessageWaiterCommands = new RuntimeTerminalMessageWaiterCommands({
    getLiveLeafForHandle: (handle) => this.getLiveLeafForHandle(handle),
    deliverPendingMessages: (leaf) => this.deliverPendingMessages(leaf)
  })

  deliverPendingMessagesForHandle: RuntimeTerminalMessageWaiterCommands['deliverPendingMessagesForHandle'] =
    this.terminalMessageWaiterCommands.deliverPendingMessagesForHandle.bind(
      this.terminalMessageWaiterCommands
    )
  notifyMessageArrived: RuntimeTerminalMessageWaiterCommands['notifyMessageArrived'] =
    this.terminalMessageWaiterCommands.notifyMessageArrived.bind(this.terminalMessageWaiterCommands)
  waitForMessage: RuntimeTerminalMessageWaiterCommands['waitForMessage'] =
    this.terminalMessageWaiterCommands.waitForMessage.bind(this.terminalMessageWaiterCommands)

  private buildPtyTerminalSummary(
    pty: RuntimePtyWorktreeRecord,
    worktreesById: Map<string, ResolvedWorktree>
  ): RuntimeTerminalSummary {
    const worktree = worktreesById.get(pty.worktreeId)

    return {
      handle: this.issuePtyHandle(pty),
      ptyId: pty.ptyId,
      worktreeId: pty.worktreeId,
      worktreePath: worktree?.path ?? '',
      branch: worktree?.branch ?? '',
      tabId: `pty:${pty.ptyId}`,
      leafId: `pty:${pty.ptyId}`,
      title: getLatestPtyTitle(pty),
      connected: pty.connected,
      writable: pty.connected,
      lastOutputAt: pty.lastOutputAt,
      preview: pty.preview
    }
  }

  private getLiveLeafForHandle(handle: string): {
    record: TerminalHandleRecord
    leaf: RuntimeLeafRecord
  } {
    this.assertGraphReady()
    const record = this.graph.handles.get(handle)
    if (!record || record.runtimeId !== this.runtimeId) {
      throw new Error('terminal_handle_stale')
    }
    if (record.rendererGraphEpoch !== this.graph.rendererGraphEpoch) {
      throw new Error('terminal_handle_stale')
    }

    const leaf = this.graph.leaves.get(this.getLeafKey(record.tabId, record.leafId))
    if (!leaf || leaf.ptyId !== record.ptyId || leaf.ptyGeneration !== record.ptyGeneration) {
      throw new Error('terminal_handle_stale')
    }
    return { record, leaf }
  }

  private getLivePtyForHandle(handle: string): {
    record: TerminalHandleRecord
    pty: RuntimePtyWorktreeRecord
  } | null {
    let record = this.graph.handles.get(handle)
    if (!record) {
      const ptyId = [...this.graph.handleByPtyId.entries()].find(
        ([, mappedHandle]) => mappedHandle === handle
      )?.[0]
      const pty = ptyId ? this.graph.ptysById.get(ptyId) : null
      if (pty) {
        // Why: graph reload/unavailability clears renderer handle records, but
        // runtime-owned PTY handles remain the caller's control identity.
        this.issuePtyHandle(pty)
        record = this.graph.handles.get(handle)
      }
    }
    if (!record || record.runtimeId !== this.runtimeId || !record.tabId.startsWith('pty:')) {
      return null
    }
    if (!record.ptyId) {
      return null
    }
    const pty = this.graph.ptysById.get(record.ptyId)
    if (!pty || pty.ptyId !== record.ptyId) {
      return null
    }
    // Why: renderer adoption can race with CLI reads. If this synthetic PTY
    // handle is valid, keep ptyId -> handle populated so summaries do not mint
    // a second handle for the same terminal.
    this.graph.handleByPtyId.set(record.ptyId, handle)
    return { record, pty }
  }

  private readPtyTerminal(
    handle: string,
    pty: RuntimePtyWorktreeRecord,
    opts: { cursor?: number; limit?: number } = {}
  ): RuntimeTerminalRead {
    return readTerminalTail({
      handle,
      status: pty.connected ? 'running' : pty.lastExitCode !== null ? 'exited' : 'unknown',
      completedLines: pty.tailBuffer,
      partialLine: pty.tailPartialLine,
      completedLineCount: pty.tailLinesTotal,
      bufferTruncated: pty.tailTruncated,
      cursor: opts.cursor,
      limit: opts.limit
    })
  }

  private issueHandle(leaf: RuntimeLeafRecord): string {
    const leafKey = this.getLeafKey(leaf.tabId, leaf.leafId)
    const existingHandle = this.graph.handleByLeafKey.get(leafKey)
    if (existingHandle) {
      const existingRecord = this.graph.handles.get(existingHandle)
      if (
        existingRecord &&
        existingRecord.rendererGraphEpoch === this.graph.rendererGraphEpoch &&
        existingRecord.ptyId === leaf.ptyId &&
        existingRecord.ptyGeneration === leaf.ptyGeneration
      ) {
        return existingHandle
      }
    }

    const handle = this.adoptPreAllocatedHandle(leaf) ?? `term_${randomUUID()}`
    if (this.graph.handles.has(handle)) {
      return handle
    }
    this.graph.handles.set(handle, {
      handle,
      runtimeId: this.runtimeId,
      rendererGraphEpoch: this.graph.rendererGraphEpoch,
      worktreeId: leaf.worktreeId,
      tabId: leaf.tabId,
      leafId: leaf.leafId,
      ptyId: leaf.ptyId,
      ptyGeneration: leaf.ptyGeneration
    })
    this.graph.handleByLeafKey.set(leafKey, handle)
    return handle
  }

  private adoptPreAllocatedHandle(leaf: RuntimeLeafRecord): string | null {
    if (!leaf.ptyId) {
      return null
    }
    const preAllocated = this.graph.handleByPtyId.get(leaf.ptyId)
    if (!preAllocated) {
      return null
    }
    const leafKey = this.getLeafKey(leaf.tabId, leaf.leafId)
    this.graph.handles.set(preAllocated, {
      handle: preAllocated,
      runtimeId: this.runtimeId,
      rendererGraphEpoch: this.graph.rendererGraphEpoch,
      worktreeId: leaf.worktreeId,
      tabId: leaf.tabId,
      leafId: leaf.leafId,
      ptyId: leaf.ptyId,
      ptyGeneration: leaf.ptyGeneration
    })
    this.graph.handleByLeafKey.set(leafKey, preAllocated)
    return preAllocated
  }

  private issuePtyHandle(pty: RuntimePtyWorktreeRecord): string {
    const existingHandle =
      this.graph.handleByPtyId.get(pty.ptyId) ?? this.findHandleForPtyRecord(pty.ptyId)
    if (existingHandle) {
      const existingRecord = this.graph.handles.get(existingHandle)
      if (
        existingRecord &&
        existingRecord.runtimeId === this.runtimeId &&
        existingRecord.ptyId === pty.ptyId
      ) {
        this.graph.handleByPtyId.set(pty.ptyId, existingHandle)
        return existingHandle
      }
    }

    const handle = existingHandle ?? `term_${randomUUID()}`
    const syntheticId = `pty:${pty.ptyId}`
    this.graph.handles.set(handle, {
      handle,
      runtimeId: this.runtimeId,
      rendererGraphEpoch: this.graph.rendererGraphEpoch,
      worktreeId: pty.worktreeId,
      tabId: syntheticId,
      leafId: syntheticId,
      ptyId: pty.ptyId,
      ptyGeneration: 0
    })
    this.graph.handleByPtyId.set(pty.ptyId, handle)
    return handle
  }

  private findHandleForPtyRecord(ptyId: string): string | null {
    for (const [handle, record] of this.graph.handles) {
      if (
        record.runtimeId === this.runtimeId &&
        record.ptyId === ptyId &&
        record.tabId.startsWith('pty:')
      ) {
        return handle
      }
    }
    return null
  }

  private refreshWritableFlags(): void {
    for (const leaf of this.graph.leaves.values()) {
      leaf.writable = this.graph.graphStatus === 'ready' && leaf.connected && leaf.ptyId !== null
    }
  }

  private invalidateLeafHandle(leafKey: string): void {
    const handle = this.graph.handleByLeafKey.get(leafKey)
    if (!handle) {
      return
    }
    this.graph.handleByLeafKey.delete(leafKey)
    this.graph.handles.delete(handle)
    this.rejectWaitersForHandle(handle, 'terminal_handle_stale')
  }

  private rememberDetachedPreAllocatedLeaves(): void {
    for (const leaf of this.graph.leaves.values()) {
      if (leaf.ptyId && this.graph.handleByPtyId.has(leaf.ptyId)) {
        // Why: ORCA_TERMINAL_HANDLE is an agent identity, so CLI control should
        // survive renderer graph loss as long as the underlying PTY is alive.
        this.graph.detachedPreAllocatedLeaves.set(leaf.ptyId, leaf)
      }
    }
  }

  private resolveExitWaiters(leaf: RuntimeLeafRecord): void {
    const handle = this.issueHandle(leaf)
    if (!handle) {
      return
    }
    const waiters = this.graph.waitersByHandle.get(handle)
    if (!waiters || waiters.size === 0) {
      return
    }
    for (const waiter of [...waiters]) {
      if (waiter.condition === 'exit') {
        this.resolveWaiter(waiter, buildTerminalWaitResult(handle, 'exit', leaf))
      } else {
        // Why: if the terminal exited, conditions like tui-idle can never be
        // satisfied. Reject immediately instead of letting the poll interval
        // spin until timeout on a dead process.
        this.removeWaiter(waiter)
        waiter.reject(new Error('terminal_exited'))
      }
    }
  }

  private resolveTuiIdleWaiters(leaf: RuntimeLeafRecord): void {
    const handle = this.graph.handleByLeafKey.get(this.getLeafKey(leaf.tabId, leaf.leafId))
    if (!handle) {
      return
    }
    const waiters = this.graph.waitersByHandle.get(handle)
    if (!waiters || waiters.size === 0) {
      return
    }
    for (const waiter of [...waiters]) {
      if (waiter.condition === 'tui-idle') {
        this.resolveWaiter(waiter, buildTerminalWaitResult(handle, 'tui-idle', leaf))
      }
    }
  }

  private resolvePtyExitWaiters(pty: RuntimePtyWorktreeRecord, ptyId: string): void {
    const handle = this.graph.handleByPtyId.get(ptyId)
    if (!handle) {
      return
    }
    const waiters = this.graph.waitersByHandle.get(handle)
    if (!waiters || waiters.size === 0) {
      return
    }
    for (const waiter of [...waiters]) {
      if (waiter.condition === 'exit') {
        this.resolveWaiter(waiter, buildPtyTerminalWaitResult(handle, 'exit', pty))
      } else {
        this.removeWaiter(waiter)
        waiter.reject(new Error('terminal_exited'))
      }
    }
  }

  private resolvePtyTuiIdleWaiters(pty: RuntimePtyWorktreeRecord, ptyId: string): void {
    const handle = this.graph.handleByPtyId.get(ptyId)
    if (!handle) {
      return
    }
    const waiters = this.graph.waitersByHandle.get(handle)
    if (!waiters || waiters.size === 0) {
      return
    }
    for (const waiter of [...waiters]) {
      if (waiter.condition === 'tui-idle') {
        this.resolveWaiter(waiter, buildPtyTerminalWaitResult(handle, 'tui-idle', pty))
      }
    }
  }

  // Why: OSC title detection via onPtyData is the primary signal for tui-idle,
  // but daemon-hosted terminals don't flow PTY data through the runtime, and
  // some agents don't emit recognized titles on startup. This fallback polls
  // two signals: (1) the renderer-synced tab title (reflects xterm's OSC title
  // handler, works even for daemon terminals), and (2) the PTY foreground process
  // + output quiescence. The poll self-cancels when the primary OSC path fires.
  private startTuiIdleFallbackPoll(waiter: TerminalWaiter, leaf: RuntimeLeafRecord): void {
    let foregroundPollInFlight = false
    waiter.pollInterval = setInterval(async () => {
      if (!waiter.pollInterval) {
        return
      }
      let startedForegroundPoll = false
      try {
        if (leaf.lastAgentStatus === 'idle') {
          if (waiter.pollInterval) {
            clearInterval(waiter.pollInterval)
            waiter.pollInterval = null
          }
          this.resolveWaiter(waiter, buildTerminalWaitResult(waiter.handle, 'tui-idle', leaf))
          return
        }
        // Why: check the renderer-synced title. For daemon-hosted terminals,
        // this is the only path where OSC titles are visible to the runtime.
        const pollTitle = leaf.paneTitle ?? this.graph.tabs.get(leaf.tabId)?.title
        if (pollTitle) {
          const titleStatus = detectExplicitIdleStatusFromTitle(pollTitle)
          if (titleStatus === 'idle') {
            if (waiter.pollInterval) {
              clearInterval(waiter.pollInterval)
              waiter.pollInterval = null
            }
            this.resolveWaiter(waiter, buildTerminalWaitResult(waiter.handle, 'tui-idle', leaf))
            return
          }
        }
        const leafWaitText = buildTerminalWaitText(
          leaf.tailBuffer,
          leaf.tailPartialLine,
          leaf.preview
        )
        const blockedReason = detectTerminalWaitBlockedReason(leafWaitText)
        if (blockedReason) {
          if (waiter.pollInterval) {
            clearInterval(waiter.pollInterval)
            waiter.pollInterval = null
          }
          this.resolveWaiter(
            waiter,
            buildTerminalWaitBlockedResult(waiter.handle, 'tui-idle', leaf, blockedReason)
          )
          return
        }
        if (isKnownReadyPromptPreview(leafWaitText)) {
          if (waiter.pollInterval) {
            clearInterval(waiter.pollInterval)
            waiter.pollInterval = null
          }
          this.resolveWaiter(waiter, buildTerminalWaitResult(waiter.handle, 'tui-idle', leaf))
          return
        }
        // Foreground process fallback: if the daemon/local provider can report
        // the process and it's a non-shell with quiet output, treat as idle.
        if (
          leaf.lastAgentStatus === null &&
          leaf.ptyId &&
          this.ptyController &&
          !foregroundPollInFlight
        ) {
          foregroundPollInFlight = true
          startedForegroundPoll = true
          const fg = await this.ptyController.getForegroundProcess(leaf.ptyId)
          if (fg && !isShellProcess(fg)) {
            const quietMs = leaf.lastOutputAt ? Date.now() - leaf.lastOutputAt : 0
            if (quietMs >= TUI_IDLE_QUIESCENCE_MS) {
              if (waiter.pollInterval) {
                clearInterval(waiter.pollInterval)
                waiter.pollInterval = null
              }
              this.resolveWaiter(waiter, buildTerminalWaitResult(waiter.handle, 'tui-idle', leaf))
            }
          }
        }
      } catch {
        // Swallow transient PTY inspection errors and keep polling.
      } finally {
        if (startedForegroundPoll) {
          foregroundPollInFlight = false
        }
      }
    }, TUI_IDLE_POLL_INTERVAL_MS)
  }

  private startPtyTuiIdleFallbackPoll(waiter: TerminalWaiter, pty: RuntimePtyWorktreeRecord): void {
    let foregroundPollInFlight = false
    waiter.pollInterval = setInterval(async () => {
      if (!waiter.pollInterval) {
        return
      }
      let startedForegroundPoll = false
      try {
        if (pty.lastAgentStatus === 'idle') {
          if (waiter.pollInterval) {
            clearInterval(waiter.pollInterval)
            waiter.pollInterval = null
          }
          this.resolveWaiter(waiter, buildPtyTerminalWaitResult(waiter.handle, 'tui-idle', pty))
          return
        }
        const ptyWaitText = buildTerminalWaitText(pty.tailBuffer, pty.tailPartialLine, pty.preview)
        const blockedReason = detectTerminalWaitBlockedReason(ptyWaitText)
        if (blockedReason) {
          if (waiter.pollInterval) {
            clearInterval(waiter.pollInterval)
            waiter.pollInterval = null
          }
          this.resolveWaiter(
            waiter,
            buildPtyTerminalWaitBlockedResult(waiter.handle, 'tui-idle', pty, blockedReason)
          )
          return
        }
        // Why: background PTY handles can later be adopted by the renderer.
        // Use that live xterm title as the same readiness signal as leaf handles.
        if (
          this.getAdoptedPtyExplicitIdleStatus(pty) === 'idle' ||
          isKnownReadyPromptPreview(ptyWaitText)
        ) {
          if (waiter.pollInterval) {
            clearInterval(waiter.pollInterval)
            waiter.pollInterval = null
          }
          this.resolveWaiter(waiter, buildPtyTerminalWaitResult(waiter.handle, 'tui-idle', pty))
          return
        }
        if (pty.lastAgentStatus === null && this.ptyController && !foregroundPollInFlight) {
          foregroundPollInFlight = true
          startedForegroundPoll = true
          const fg = await this.ptyController.getForegroundProcess(pty.ptyId)
          if (fg && !isShellProcess(fg)) {
            const quietMs = pty.lastOutputAt ? Date.now() - pty.lastOutputAt : 0
            if (quietMs >= TUI_IDLE_QUIESCENCE_MS) {
              if (waiter.pollInterval) {
                clearInterval(waiter.pollInterval)
                waiter.pollInterval = null
              }
              this.resolveWaiter(waiter, buildPtyTerminalWaitResult(waiter.handle, 'tui-idle', pty))
            }
          }
        }
      } catch {
        // Swallow transient PTY inspection errors and keep polling.
      } finally {
        if (startedForegroundPoll) {
          foregroundPollInFlight = false
        }
      }
    }, TUI_IDLE_POLL_INTERVAL_MS)
  }

  private getAdoptedPtyExplicitIdleStatus(pty: RuntimePtyWorktreeRecord): AgentStatus | null {
    for (const leaf of this.graph.leaves.values()) {
      if (leaf.ptyId !== pty.ptyId) {
        continue
      }
      const title = leaf.paneTitle ?? this.graph.tabs.get(leaf.tabId)?.title
      if (!title) {
        continue
      }
      const status = detectExplicitIdleStatusFromTitle(title)
      if (status !== null) {
        return status
      }
    }
    return null
  }

  // Why: push-on-idle delivery — when an agent transitions working→idle, check
  // for unread orchestration messages addressed to that terminal and inject them
  // into the PTY. This is event-driven (no polling) because the runtime owns
  // both the message store and terminal status detection.
  private deliverPendingMessages(leaf: RuntimeLeafRecord): void {
    if (!this._orchestrationDb) {
      return
    }

    const handle = this.graph.handleByLeafKey.get(this.getLeafKey(leaf.tabId, leaf.leafId))
    if (!handle) {
      return
    }

    const unread = this._orchestrationDb.getUndeliveredUnreadMessages(handle)
    if (unread.length === 0) {
      return
    }

    if (!leaf.writable || !leaf.ptyId) {
      return
    }

    const payload = formatMessagesForInjection(unread)
    const wrote = this.ptyController?.write(leaf.ptyId, payload) ?? false
    if (!wrote) {
      return
    }

    // The active coordinator prompt is user-owned input, so push-on-idle must not synthesize Enter.
    if (this._orchestrationDb.getActiveCoordinatorRun()?.coordinator_handle === handle) {
      this._orchestrationDb.markAsDelivered(unread.map((m) => m.id))
      return
    }

    const tabTitle = this.graph.tabs.get(leaf.tabId)?.title
    if (isCursorAgentOrchestrationTarget(leaf, tabTitle)) {
      // Why: Cursor Agent treats injected PTY text as editable prompt input.
      // Push-on-idle may surface the message, but submitting it must stay
      // under user control.
      this._orchestrationDb.markAsDelivered(unread.map((m) => m.id))
      return
    }

    // Why: Claude Code treats large single PTY writes as paste events and
    // swallows a \r included in the same write. Send Enter separately after
    // a delay so the agent processes the pasted message first. Stamp
    // `delivered_at` only after \r is confirmed, so failed deliveries stay
    // queued.
    //
    // Important (design doc §3.2, feedback #2): we stamp `delivered_at` here
    // instead of flipping `read`. `read` is reserved for "a check-caller
    // consumed this message." Flipping `read` on push-on-idle would hide the
    // message from the coordinator's next `check --unread`, which is the
    // exact bug feedback #2 reported. The two bits must stay independent.
    const ptyId = leaf.ptyId
    setTimeout(() => {
      try {
        if (!leaf.writable) {
          return
        }
        const submitted = this.ptyController?.write(ptyId, '\r') ?? false
        if (submitted) {
          this._orchestrationDb?.markAsDelivered(unread.map((m) => m.id))
        }
      } catch {
        // Terminal may have closed during the delay — messages stay queued
        // (delivered_at still NULL) and will be re-delivered on the next
        // idle transition.
      }
    }, 500)
  }

  private resolveWaiter(waiter: TerminalWaiter, result: RuntimeTerminalWait): void {
    this.removeWaiter(waiter)
    waiter.resolve(result)
  }

  private bindTerminalWaiterAbort(
    waiter: TerminalWaiter,
    signal: AbortSignal | undefined
  ): boolean {
    if (!signal) {
      return true
    }
    if (signal.aborted) {
      return false
    }
    const onAbort = (): void => {
      this.removeWaiter(waiter)
      waiter.reject(new Error('request_aborted'))
    }
    waiter.abortCleanup = () => signal.removeEventListener('abort', onAbort)
    signal.addEventListener('abort', onAbort, { once: true })
    return true
  }

  private rejectWaitersForHandle(handle: string, code: string): void {
    const waiters = this.graph.waitersByHandle.get(handle)
    if (!waiters || waiters.size === 0) {
      return
    }
    for (const waiter of [...waiters]) {
      this.removeWaiter(waiter)
      waiter.reject(new Error(code))
    }
  }

  private rejectAllWaiters(code: string): void {
    for (const handle of [...this.graph.waitersByHandle.keys()]) {
      this.rejectWaitersForHandle(handle, code)
    }
  }

  private removeWaiter(waiter: TerminalWaiter): void {
    if (waiter.timeout) {
      clearTimeout(waiter.timeout)
    }
    if (waiter.pollInterval) {
      clearInterval(waiter.pollInterval)
    }
    if (waiter.abortCleanup) {
      waiter.abortCleanup()
      waiter.abortCleanup = null
    }
    const waiters = this.graph.waitersByHandle.get(waiter.handle)
    if (!waiters) {
      return
    }
    waiters.delete(waiter)
    if (waiters.size === 0) {
      this.graph.waitersByHandle.delete(waiter.handle)
    }
  }

  private getLeafKey(tabId: string, leafId: string): string {
    return `${tabId}::${leafId}`
  }

  private readonly linearCommands = new RuntimeLinearCommands({
    getStore: () => this.store,
    showTerminal: (handle) => this.showTerminal(handle),
    resolveWorktreeSelector: (selector) => this.resolveWorktreeSelector(selector),
    emitClientEvent: (event) => this.emitClientEvent(event),
    listResolvedWorktrees: () => this.listResolvedWorktrees()
  })

  linearAddIssueComment: RuntimeLinearCommands['linearAddIssueComment'] =
    this.linearCommands.linearAddIssueComment.bind(this.linearCommands)
  linearConnect: RuntimeLinearCommands['linearConnect'] = this.linearCommands.linearConnect.bind(
    this.linearCommands
  )
  linearCreateIssue: RuntimeLinearCommands['linearCreateIssue'] =
    this.linearCommands.linearCreateIssue.bind(this.linearCommands)
  linearCreateProject: RuntimeLinearCommands['linearCreateProject'] =
    this.linearCommands.linearCreateProject.bind(this.linearCommands)
  linearDisconnect: RuntimeLinearCommands['linearDisconnect'] =
    this.linearCommands.linearDisconnect.bind(this.linearCommands)
  linearGetCustomView: RuntimeLinearCommands['linearGetCustomView'] =
    this.linearCommands.linearGetCustomView.bind(this.linearCommands)
  linearGetIssue: RuntimeLinearCommands['linearGetIssue'] = this.linearCommands.linearGetIssue.bind(
    this.linearCommands
  )
  linearGetProject: RuntimeLinearCommands['linearGetProject'] =
    this.linearCommands.linearGetProject.bind(this.linearCommands)
  linearIssueAddComment: RuntimeLinearCommands['linearIssueAddComment'] =
    this.linearCommands.linearIssueAddComment.bind(this.linearCommands)
  linearIssueAttachLink: RuntimeLinearCommands['linearIssueAttachLink'] =
    this.linearCommands.linearIssueAttachLink.bind(this.linearCommands)
  linearIssueComments: RuntimeLinearCommands['linearIssueComments'] =
    this.linearCommands.linearIssueComments.bind(this.linearCommands)
  linearIssueContext: RuntimeLinearCommands['linearIssueContext'] =
    this.linearCommands.linearIssueContext.bind(this.linearCommands)
  linearIssueCreate: RuntimeLinearCommands['linearIssueCreate'] =
    this.linearCommands.linearIssueCreate.bind(this.linearCommands)
  linearIssueListForAgents: RuntimeLinearCommands['linearIssueListForAgents'] =
    this.linearCommands.linearIssueListForAgents.bind(this.linearCommands)
  linearIssueSetState: RuntimeLinearCommands['linearIssueSetState'] =
    this.linearCommands.linearIssueSetState.bind(this.linearCommands)
  linearIssueUpdateTask: RuntimeLinearCommands['linearIssueUpdateTask'] =
    this.linearCommands.linearIssueUpdateTask.bind(this.linearCommands)
  linearListCustomViewIssues: RuntimeLinearCommands['linearListCustomViewIssues'] =
    this.linearCommands.linearListCustomViewIssues.bind(this.linearCommands)
  linearListCustomViewProjects: RuntimeLinearCommands['linearListCustomViewProjects'] =
    this.linearCommands.linearListCustomViewProjects.bind(this.linearCommands)
  linearListCustomViews: RuntimeLinearCommands['linearListCustomViews'] =
    this.linearCommands.linearListCustomViews.bind(this.linearCommands)
  linearListIssues: RuntimeLinearCommands['linearListIssues'] =
    this.linearCommands.linearListIssues.bind(this.linearCommands)
  linearListProjectIssues: RuntimeLinearCommands['linearListProjectIssues'] =
    this.linearCommands.linearListProjectIssues.bind(this.linearCommands)
  linearListProjects: RuntimeLinearCommands['linearListProjects'] =
    this.linearCommands.linearListProjects.bind(this.linearCommands)
  linearListTeams: RuntimeLinearCommands['linearListTeams'] =
    this.linearCommands.linearListTeams.bind(this.linearCommands)
  linearProjectListForAgents: RuntimeLinearCommands['linearProjectListForAgents'] =
    this.linearCommands.linearProjectListForAgents.bind(this.linearCommands)
  linearResolveCurrentIssue: RuntimeLinearCommands['linearResolveCurrentIssue'] =
    this.linearCommands.linearResolveCurrentIssue.bind(this.linearCommands)
  linearSearchForAgents: RuntimeLinearCommands['linearSearchForAgents'] =
    this.linearCommands.linearSearchForAgents.bind(this.linearCommands)
  linearSearchIssues: RuntimeLinearCommands['linearSearchIssues'] =
    this.linearCommands.linearSearchIssues.bind(this.linearCommands)
  linearSelectWorkspace: RuntimeLinearCommands['linearSelectWorkspace'] =
    this.linearCommands.linearSelectWorkspace.bind(this.linearCommands)
  linearStatus: RuntimeLinearCommands['linearStatus'] = this.linearCommands.linearStatus.bind(
    this.linearCommands
  )
  linearTeamLabels: RuntimeLinearCommands['linearTeamLabels'] =
    this.linearCommands.linearTeamLabels.bind(this.linearCommands)
  linearTeamLabelsForAgents: RuntimeLinearCommands['linearTeamLabelsForAgents'] =
    this.linearCommands.linearTeamLabelsForAgents.bind(this.linearCommands)
  linearTeamListForAgents: RuntimeLinearCommands['linearTeamListForAgents'] =
    this.linearCommands.linearTeamListForAgents.bind(this.linearCommands)
  linearTeamMembers: RuntimeLinearCommands['linearTeamMembers'] =
    this.linearCommands.linearTeamMembers.bind(this.linearCommands)
  linearTeamMembersForAgents: RuntimeLinearCommands['linearTeamMembersForAgents'] =
    this.linearCommands.linearTeamMembersForAgents.bind(this.linearCommands)
  linearTeamStates: RuntimeLinearCommands['linearTeamStates'] =
    this.linearCommands.linearTeamStates.bind(this.linearCommands)
  linearTeamStatesForAgents: RuntimeLinearCommands['linearTeamStatesForAgents'] =
    this.linearCommands.linearTeamStatesForAgents.bind(this.linearCommands)
  linearTestConnection: RuntimeLinearCommands['linearTestConnection'] =
    this.linearCommands.linearTestConnection.bind(this.linearCommands)
  linearUpdateIssue: RuntimeLinearCommands['linearUpdateIssue'] =
    this.linearCommands.linearUpdateIssue.bind(this.linearCommands)

  private readonly jiraCommands = new RuntimeJiraCommands()

  jiraConnect: RuntimeJiraCommands['jiraConnect'] = this.jiraCommands.jiraConnect.bind(
    this.jiraCommands
  )
  jiraDisconnect: RuntimeJiraCommands['jiraDisconnect'] = this.jiraCommands.jiraDisconnect.bind(
    this.jiraCommands
  )
  jiraSelectSite: RuntimeJiraCommands['jiraSelectSite'] = this.jiraCommands.jiraSelectSite.bind(
    this.jiraCommands
  )
  jiraStatus: RuntimeJiraCommands['jiraStatus'] = this.jiraCommands.jiraStatus.bind(
    this.jiraCommands
  )
  jiraTestConnection: RuntimeJiraCommands['jiraTestConnection'] =
    this.jiraCommands.jiraTestConnection.bind(this.jiraCommands)
  jiraSearchIssues: RuntimeJiraCommands['jiraSearchIssues'] =
    this.jiraCommands.jiraSearchIssues.bind(this.jiraCommands)
  jiraListIssues: RuntimeJiraCommands['jiraListIssues'] = this.jiraCommands.jiraListIssues.bind(
    this.jiraCommands
  )
  jiraCreateIssue: RuntimeJiraCommands['jiraCreateIssue'] = this.jiraCommands.jiraCreateIssue.bind(
    this.jiraCommands
  )
  jiraGetIssue: RuntimeJiraCommands['jiraGetIssue'] = this.jiraCommands.jiraGetIssue.bind(
    this.jiraCommands
  )
  jiraUpdateIssue: RuntimeJiraCommands['jiraUpdateIssue'] = this.jiraCommands.jiraUpdateIssue.bind(
    this.jiraCommands
  )
  jiraAddIssueComment: RuntimeJiraCommands['jiraAddIssueComment'] =
    this.jiraCommands.jiraAddIssueComment.bind(this.jiraCommands)
  jiraIssueComments: RuntimeJiraCommands['jiraIssueComments'] =
    this.jiraCommands.jiraIssueComments.bind(this.jiraCommands)
  jiraListProjects: RuntimeJiraCommands['jiraListProjects'] =
    this.jiraCommands.jiraListProjects.bind(this.jiraCommands)
  jiraListIssueTypes: RuntimeJiraCommands['jiraListIssueTypes'] =
    this.jiraCommands.jiraListIssueTypes.bind(this.jiraCommands)
  jiraListCreateFields: RuntimeJiraCommands['jiraListCreateFields'] =
    this.jiraCommands.jiraListCreateFields.bind(this.jiraCommands)
  jiraListPriorities: RuntimeJiraCommands['jiraListPriorities'] =
    this.jiraCommands.jiraListPriorities.bind(this.jiraCommands)
  jiraListAssignableUsers: RuntimeJiraCommands['jiraListAssignableUsers'] =
    this.jiraCommands.jiraListAssignableUsers.bind(this.jiraCommands)
  jiraListTransitions: RuntimeJiraCommands['jiraListTransitions'] =
    this.jiraCommands.jiraListTransitions.bind(this.jiraCommands)
  jiraGetProjectStatusOrder: RuntimeJiraCommands['jiraGetProjectStatusOrder'] =
    this.jiraCommands.jiraGetProjectStatusOrder.bind(this.jiraCommands)

  // ── Browser automation ──

  private readonly browserCommands = new RuntimeBrowserCommands({
    getAgentBrowserBridge: () => this.agentBrowserBridge,
    resolveWorktreeSelector: (selector) => this.resolveWorktreeSelector(selector),
    getAuthoritativeWindow: () => this.getAuthoritativeWindow(),
    getAvailableAuthoritativeWindow: () => this.getAvailableAuthoritativeWindow(),
    getOffscreenBrowserBackend: () => this.offscreenBrowserBackend,
    // Why: bind the method directly rather than re-listing params in a wrapper
    // arrow — a hand-listed wrapper silently dropped targetGroupId before, so a
    // new browser opened in the right split group landed in the left.
    markHeadlessBrowserSessionTabActive: this.markHeadlessBrowserSessionTabActive.bind(this)
  })

  private readonly emulatorCommands = new RuntimeEmulatorCommands({
    getEmulatorBridge: () => this.emulatorBridge,
    resolveWorktreeSelector: (selector) => this.resolveWorktreeSelector(selector),
    getAuthoritativeWindow: () => this.getAuthoritativeWindow(),
    getSettings: () => this.requireStore().getSettings()
  })

  browserSnapshot: RuntimeBrowserCommands['browserSnapshot'] =
    this.browserCommands.browserSnapshot.bind(this.browserCommands)

  browserClick: RuntimeBrowserCommands['browserClick'] = this.browserCommands.browserClick.bind(
    this.browserCommands
  )

  browserGoto: RuntimeBrowserCommands['browserGoto'] = this.browserCommands.browserGoto.bind(
    this.browserCommands
  )

  browserFill: RuntimeBrowserCommands['browserFill'] = this.browserCommands.browserFill.bind(
    this.browserCommands
  )

  browserType: RuntimeBrowserCommands['browserType'] = this.browserCommands.browserType.bind(
    this.browserCommands
  )

  browserSelect: RuntimeBrowserCommands['browserSelect'] = this.browserCommands.browserSelect.bind(
    this.browserCommands
  )

  browserScroll: RuntimeBrowserCommands['browserScroll'] = this.browserCommands.browserScroll.bind(
    this.browserCommands
  )

  browserBack: RuntimeBrowserCommands['browserBack'] = this.browserCommands.browserBack.bind(
    this.browserCommands
  )

  browserReload: RuntimeBrowserCommands['browserReload'] = this.browserCommands.browserReload.bind(
    this.browserCommands
  )

  browserScreenshot: RuntimeBrowserCommands['browserScreenshot'] =
    this.browserCommands.browserScreenshot.bind(this.browserCommands)

  private readonly browserScreencastCommands = new RuntimeBrowserScreencastCommands({
    browserScreencast: (params, opts) => this.browserCommands.browserScreencast(params, opts),
    getBrowserDriver: (browserPageId) => this.getBrowserDriver(browserPageId),
    setBrowserDriver: (browserPageId, driver) => this.setBrowserDriver(browserPageId, driver),
    registerSubscriptionCleanup: (subscriptionId, cleanup, connectionId) =>
      this.registerSubscriptionCleanup(subscriptionId, cleanup, connectionId),
    cleanupSubscription: (subscriptionId) => this.cleanupSubscription(subscriptionId)
  })

  browserScreencast: RuntimeBrowserScreencastCommands['browserScreencast'] =
    this.browserScreencastCommands.browserScreencast.bind(this.browserScreencastCommands)
  cancelBrowserScreencastForPage: RuntimeBrowserScreencastCommands['cancelBrowserScreencastForPage'] =
    this.browserScreencastCommands.cancelBrowserScreencastForPage.bind(
      this.browserScreencastCommands
    )

  browserEval: RuntimeBrowserCommands['browserEval'] = this.browserCommands.browserEval.bind(
    this.browserCommands
  )

  browserTabList: RuntimeBrowserCommands['browserTabList'] =
    this.browserCommands.browserTabList.bind(this.browserCommands)

  browserTabShow: RuntimeBrowserCommands['browserTabShow'] =
    this.browserCommands.browserTabShow.bind(this.browserCommands)

  browserTabCurrent: RuntimeBrowserCommands['browserTabCurrent'] =
    this.browserCommands.browserTabCurrent.bind(this.browserCommands)

  browserTabSwitch: RuntimeBrowserCommands['browserTabSwitch'] =
    this.browserCommands.browserTabSwitch.bind(this.browserCommands)

  browserHover: RuntimeBrowserCommands['browserHover'] = this.browserCommands.browserHover.bind(
    this.browserCommands
  )

  browserDrag: RuntimeBrowserCommands['browserDrag'] = this.browserCommands.browserDrag.bind(
    this.browserCommands
  )

  browserUpload: RuntimeBrowserCommands['browserUpload'] = this.browserCommands.browserUpload.bind(
    this.browserCommands
  )

  browserWait: RuntimeBrowserCommands['browserWait'] = this.browserCommands.browserWait.bind(
    this.browserCommands
  )

  browserCheck: RuntimeBrowserCommands['browserCheck'] = this.browserCommands.browserCheck.bind(
    this.browserCommands
  )

  browserFocus: RuntimeBrowserCommands['browserFocus'] = this.browserCommands.browserFocus.bind(
    this.browserCommands
  )

  browserClear: RuntimeBrowserCommands['browserClear'] = this.browserCommands.browserClear.bind(
    this.browserCommands
  )

  browserSelectAll: RuntimeBrowserCommands['browserSelectAll'] =
    this.browserCommands.browserSelectAll.bind(this.browserCommands)

  browserKeypress: RuntimeBrowserCommands['browserKeypress'] =
    this.browserCommands.browserKeypress.bind(this.browserCommands)

  browserPdf: RuntimeBrowserCommands['browserPdf'] = this.browserCommands.browserPdf.bind(
    this.browserCommands
  )

  browserFullScreenshot: RuntimeBrowserCommands['browserFullScreenshot'] =
    this.browserCommands.browserFullScreenshot.bind(this.browserCommands)

  browserCookieGet: RuntimeBrowserCommands['browserCookieGet'] =
    this.browserCommands.browserCookieGet.bind(this.browserCommands)

  browserCookieSet: RuntimeBrowserCommands['browserCookieSet'] =
    this.browserCommands.browserCookieSet.bind(this.browserCommands)

  browserCookieDelete: RuntimeBrowserCommands['browserCookieDelete'] =
    this.browserCommands.browserCookieDelete.bind(this.browserCommands)

  browserSetViewport: RuntimeBrowserCommands['browserSetViewport'] =
    this.browserCommands.browserSetViewport.bind(this.browserCommands)

  browserSetGeolocation: RuntimeBrowserCommands['browserSetGeolocation'] =
    this.browserCommands.browserSetGeolocation.bind(this.browserCommands)

  browserInterceptEnable: RuntimeBrowserCommands['browserInterceptEnable'] =
    this.browserCommands.browserInterceptEnable.bind(this.browserCommands)

  browserInterceptDisable: RuntimeBrowserCommands['browserInterceptDisable'] =
    this.browserCommands.browserInterceptDisable.bind(this.browserCommands)

  browserInterceptList: RuntimeBrowserCommands['browserInterceptList'] =
    this.browserCommands.browserInterceptList.bind(this.browserCommands)

  browserCaptureStart: RuntimeBrowserCommands['browserCaptureStart'] =
    this.browserCommands.browserCaptureStart.bind(this.browserCommands)

  browserCaptureStop: RuntimeBrowserCommands['browserCaptureStop'] =
    this.browserCommands.browserCaptureStop.bind(this.browserCommands)

  browserConsoleLog: RuntimeBrowserCommands['browserConsoleLog'] =
    this.browserCommands.browserConsoleLog.bind(this.browserCommands)

  browserNetworkLog: RuntimeBrowserCommands['browserNetworkLog'] =
    this.browserCommands.browserNetworkLog.bind(this.browserCommands)

  browserDblclick: RuntimeBrowserCommands['browserDblclick'] =
    this.browserCommands.browserDblclick.bind(this.browserCommands)

  browserForward: RuntimeBrowserCommands['browserForward'] =
    this.browserCommands.browserForward.bind(this.browserCommands)

  browserScrollIntoView: RuntimeBrowserCommands['browserScrollIntoView'] =
    this.browserCommands.browserScrollIntoView.bind(this.browserCommands)

  browserGet: RuntimeBrowserCommands['browserGet'] = this.browserCommands.browserGet.bind(
    this.browserCommands
  )

  browserIs: RuntimeBrowserCommands['browserIs'] = this.browserCommands.browserIs.bind(
    this.browserCommands
  )

  browserKeyboardInsertText: RuntimeBrowserCommands['browserKeyboardInsertText'] =
    this.browserCommands.browserKeyboardInsertText.bind(this.browserCommands)

  browserMouseMove: RuntimeBrowserCommands['browserMouseMove'] =
    this.browserCommands.browserMouseMove.bind(this.browserCommands)

  browserMouseDown: RuntimeBrowserCommands['browserMouseDown'] =
    this.browserCommands.browserMouseDown.bind(this.browserCommands)

  browserMouseClick: RuntimeBrowserCommands['browserMouseClick'] =
    this.browserCommands.browserMouseClick.bind(this.browserCommands)

  browserMouseUp: RuntimeBrowserCommands['browserMouseUp'] =
    this.browserCommands.browserMouseUp.bind(this.browserCommands)

  browserMouseWheel: RuntimeBrowserCommands['browserMouseWheel'] =
    this.browserCommands.browserMouseWheel.bind(this.browserCommands)

  browserFind: RuntimeBrowserCommands['browserFind'] = this.browserCommands.browserFind.bind(
    this.browserCommands
  )

  browserSetDevice: RuntimeBrowserCommands['browserSetDevice'] =
    this.browserCommands.browserSetDevice.bind(this.browserCommands)

  browserSetOffline: RuntimeBrowserCommands['browserSetOffline'] =
    this.browserCommands.browserSetOffline.bind(this.browserCommands)

  browserSetHeaders: RuntimeBrowserCommands['browserSetHeaders'] =
    this.browserCommands.browserSetHeaders.bind(this.browserCommands)

  browserSetCredentials: RuntimeBrowserCommands['browserSetCredentials'] =
    this.browserCommands.browserSetCredentials.bind(this.browserCommands)

  browserSetMedia: RuntimeBrowserCommands['browserSetMedia'] =
    this.browserCommands.browserSetMedia.bind(this.browserCommands)

  browserClipboardRead: RuntimeBrowserCommands['browserClipboardRead'] =
    this.browserCommands.browserClipboardRead.bind(this.browserCommands)

  browserClipboardWrite: RuntimeBrowserCommands['browserClipboardWrite'] =
    this.browserCommands.browserClipboardWrite.bind(this.browserCommands)

  browserDialogAccept: RuntimeBrowserCommands['browserDialogAccept'] =
    this.browserCommands.browserDialogAccept.bind(this.browserCommands)

  browserDialogDismiss: RuntimeBrowserCommands['browserDialogDismiss'] =
    this.browserCommands.browserDialogDismiss.bind(this.browserCommands)

  browserStorageLocalGet: RuntimeBrowserCommands['browserStorageLocalGet'] =
    this.browserCommands.browserStorageLocalGet.bind(this.browserCommands)

  browserStorageLocalSet: RuntimeBrowserCommands['browserStorageLocalSet'] =
    this.browserCommands.browserStorageLocalSet.bind(this.browserCommands)

  browserStorageLocalClear: RuntimeBrowserCommands['browserStorageLocalClear'] =
    this.browserCommands.browserStorageLocalClear.bind(this.browserCommands)

  browserStorageSessionGet: RuntimeBrowserCommands['browserStorageSessionGet'] =
    this.browserCommands.browserStorageSessionGet.bind(this.browserCommands)

  browserStorageSessionSet: RuntimeBrowserCommands['browserStorageSessionSet'] =
    this.browserCommands.browserStorageSessionSet.bind(this.browserCommands)

  browserStorageSessionClear: RuntimeBrowserCommands['browserStorageSessionClear'] =
    this.browserCommands.browserStorageSessionClear.bind(this.browserCommands)

  browserDownload: RuntimeBrowserCommands['browserDownload'] =
    this.browserCommands.browserDownload.bind(this.browserCommands)

  browserHighlight: RuntimeBrowserCommands['browserHighlight'] =
    this.browserCommands.browserHighlight.bind(this.browserCommands)

  browserExec: RuntimeBrowserCommands['browserExec'] = this.browserCommands.browserExec.bind(
    this.browserCommands
  )

  browserTabCreate: RuntimeBrowserCommands['browserTabCreate'] =
    this.browserCommands.browserTabCreate.bind(this.browserCommands)

  browserTabSetProfile: RuntimeBrowserCommands['browserTabSetProfile'] =
    this.browserCommands.browserTabSetProfile.bind(this.browserCommands)

  browserTabProfileShow: RuntimeBrowserCommands['browserTabProfileShow'] =
    this.browserCommands.browserTabProfileShow.bind(this.browserCommands)

  browserTabProfileClone: RuntimeBrowserCommands['browserTabProfileClone'] =
    this.browserCommands.browserTabProfileClone.bind(this.browserCommands)

  browserProfileList: RuntimeBrowserCommands['browserProfileList'] =
    this.browserCommands.browserProfileList.bind(this.browserCommands)

  browserProfileCreate: RuntimeBrowserCommands['browserProfileCreate'] =
    this.browserCommands.browserProfileCreate.bind(this.browserCommands)

  browserProfileDelete: RuntimeBrowserCommands['browserProfileDelete'] =
    this.browserCommands.browserProfileDelete.bind(this.browserCommands)

  browserProfileDetectBrowsers: RuntimeBrowserCommands['browserProfileDetectBrowsers'] =
    this.browserCommands.browserProfileDetectBrowsers.bind(this.browserCommands)

  browserProfileImportFromBrowser: RuntimeBrowserCommands['browserProfileImportFromBrowser'] =
    this.browserCommands.browserProfileImportFromBrowser.bind(this.browserCommands)

  browserProfileClearDefaultCookies: RuntimeBrowserCommands['browserProfileClearDefaultCookies'] =
    this.browserCommands.browserProfileClearDefaultCookies.bind(this.browserCommands)

  browserTabClose: RuntimeBrowserCommands['browserTabClose'] =
    this.browserCommands.browserTabClose.bind(this.browserCommands)

  // Emulator bindings (delegated to dedicated commands for surface separation).
  emulatorTap: RuntimeEmulatorCommands['emulatorTap'] = this.emulatorCommands.emulatorTap.bind(
    this.emulatorCommands
  )
  emulatorGesture: RuntimeEmulatorCommands['emulatorGesture'] =
    this.emulatorCommands.emulatorGesture.bind(this.emulatorCommands)
  emulatorType: RuntimeEmulatorCommands['emulatorType'] = this.emulatorCommands.emulatorType.bind(
    this.emulatorCommands
  )
  emulatorButton: RuntimeEmulatorCommands['emulatorButton'] =
    this.emulatorCommands.emulatorButton.bind(this.emulatorCommands)
  emulatorRotate: RuntimeEmulatorCommands['emulatorRotate'] =
    this.emulatorCommands.emulatorRotate.bind(this.emulatorCommands)
  emulatorExec: RuntimeEmulatorCommands['emulatorExec'] = this.emulatorCommands.emulatorExec.bind(
    this.emulatorCommands
  )
  emulatorAttach: RuntimeEmulatorCommands['emulatorAttach'] =
    this.emulatorCommands.emulatorAttach.bind(this.emulatorCommands)
  emulatorList: RuntimeEmulatorCommands['emulatorList'] = this.emulatorCommands.emulatorList.bind(
    this.emulatorCommands
  )
  emulatorKill: RuntimeEmulatorCommands['emulatorKill'] = this.emulatorCommands.emulatorKill.bind(
    this.emulatorCommands
  )
  emulatorShutdown: RuntimeEmulatorCommands['emulatorShutdown'] =
    this.emulatorCommands.emulatorShutdown.bind(this.emulatorCommands)
  emulatorListSimulators: RuntimeEmulatorCommands['emulatorListSimulators'] =
    this.emulatorCommands.emulatorListSimulators.bind(this.emulatorCommands)
  emulatorAvailability: RuntimeEmulatorCommands['emulatorAvailability'] =
    this.emulatorCommands.emulatorAvailability.bind(this.emulatorCommands)
  emulatorListDevices: RuntimeEmulatorCommands['emulatorListDevices'] =
    this.emulatorCommands.emulatorListDevices.bind(this.emulatorCommands)
  emulatorInstall: RuntimeEmulatorCommands['emulatorInstall'] =
    this.emulatorCommands.emulatorInstall.bind(this.emulatorCommands)
  emulatorLaunch: RuntimeEmulatorCommands['emulatorLaunch'] =
    this.emulatorCommands.emulatorLaunch.bind(this.emulatorCommands)
  emulatorPermissions: RuntimeEmulatorCommands['emulatorPermissions'] =
    this.emulatorCommands.emulatorPermissions.bind(this.emulatorCommands)
  emulatorAx: RuntimeEmulatorCommands['emulatorAx'] = this.emulatorCommands.emulatorAx.bind(
    this.emulatorCommands
  )
  emulatorLogcat: RuntimeEmulatorCommands['emulatorLogcat'] =
    this.emulatorCommands.emulatorLogcat.bind(this.emulatorCommands)
  emulatorUnregisterActive: RuntimeEmulatorCommands['emulatorUnregisterActive'] =
    this.emulatorCommands.emulatorUnregisterActive.bind(this.emulatorCommands)

  // Why: serve-sim-state-watcher runs from main/index.ts startup; keep window IPC behind runtime (getAuthoritativeWindow is private).
  notifyEmulatorAutoAttachFromWatcher(
    worktreeId: string,
    info: { deviceUdid: string; streamUrl: string; wsUrl: string; axUrl?: string }
  ): void {
    try {
      this.getAuthoritativeWindow().webContents.send('ui:emulatorAutoAttach', { worktreeId, info })
    } catch {
      // Window may not exist during shutdown
    }
  }

  private getAuthoritativeWindow(): BrowserWindow {
    const win = this.getAvailableAuthoritativeWindow()
    if (!win || win.isDestroyed()) {
      throw new Error('No renderer window available')
    }
    return win
  }

  private getAvailableAuthoritativeWindow(): BrowserWindow | null {
    if (this.graph.authoritativeWindowId === null) {
      return null
    }
    if (!BrowserWindow?.fromId) {
      return null
    }
    const win = BrowserWindow.fromId(this.graph.authoritativeWindowId)
    return win && !win.isDestroyed() ? win : null
  }
}

const DEFAULT_WORKTREE_LIST_LIMIT = 200
const DISCONNECTED_PTY_RECORD_MAX = 128
const PTY_CONTROLLER_LIST_TIMEOUT_MS = 3000

function getExplicitWorktreeIdSelector(selector: string | undefined): string | null {
  if (!selector?.startsWith('id:')) {
    return null
  }
  const id = selector.slice(3)
  return id.length > 0 ? id : null
}

export function withTimeout<T>(promise: Promise<T>, timeoutMs: number, fallback: T): Promise<T> {
  let timeout: ReturnType<typeof setTimeout> | null = null
  return new Promise<T>((resolve) => {
    timeout = setTimeout(() => resolve(fallback), timeoutMs)
    promise.then(
      (value) => resolve(value),
      () => resolve(fallback)
    )
  }).finally(() => {
    if (timeout) {
      clearTimeout(timeout)
    }
  })
}

function withTimeoutResult<T>(
  promise: Promise<T>,
  timeoutMs: number
): Promise<{ ok: true; value: T } | { ok: false }> {
  return withTimeout(
    promise.then((value) => ({ ok: true, value }) as const),
    timeoutMs,
    {
      ok: false
    }
  )
}

// FIX BUG-FE-BIGFILE-002 / TASK-BIGFILE-008: pure terminal-tail/output-scanning
// and branch-name helpers extracted to orca-runtime-tail-buffer.ts (no
// OrcaRuntimeService state). Re-exported here so existing external importers
// of orca-runtime.ts are unaffected.
export {
  appendRecentPtyOutput,
  appendRecentPtyPathCandidates,
  recentTerminalPathCandidatesIncludePath,
  recentTerminalOutputIncludesPath,
  buildPreview,
  type TerminalTailWaitState,
  computeTerminalTailWaitState,
  tailGainedNewerBlockedReason,
  appendNormalizedToTailBuffer,
  appendNormalizedToMultilineTailBufferUnwindowed
} from './orca-runtime-tail-buffer'

// FIX BUG-FE-BIGFILE-002 / TASK-BIGFILE-009: pure type declarations extracted
// to orca-runtime-types.ts (no runtime logic). Re-exported here so existing
// external importers of orca-runtime.ts are unaffected.
export type {
  AccountsSnapshot,
  ApplyLayoutResult,
  DriverState,
  MobileNotificationDispatchEvent,
  MobileNotificationDismissEvent,
  MobileNotificationEvent,
  PtyLayoutState,
  PtyLayoutTarget,
  RemoteFetchResult,
  RemoteTrackingBase,
  RuntimeAutomationCreateInput,
  RuntimeAutomationUpdateInput,
  RuntimePtyController,
  RuntimeTerminalAgentStatusEvent
} from './orca-runtime-types'
