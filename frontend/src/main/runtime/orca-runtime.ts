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
import {
  detectAgentStatusFromTitle,
  isClaudeManagementTitle,
  isCursorAgentTitle,
  isCursorNativeAgentTitle,
  isShellProcess,
  normalizeTerminalTitle
} from '../../shared/agent-detection'
import { extractOscTitleScanTail } from '../../shared/osc-title-scan-tail'
import { extractLastOsc7Uri, extractOscScanTail } from '../daemon/osc7-uri-extraction'
import { parseFileUriPathParts } from '../daemon/osc7-file-uri'
import type { AgentStatus } from '../../shared/agent-detection'
import type { TerminalOscLinkRange } from '../../shared/terminal-osc-link-ranges'
import {
  createTerminalTitleTracker,
  stripBrailleSpinnerGlyphs,
  type TerminalTitleTracker
} from '../../shared/terminal-output-side-effects'
import { createCommandCodeOutputStatusDetector } from '../../shared/command-code-output-status'
import type {
  TerminalSideEffectBatch,
  TerminalSideEffectFact
} from '../../shared/terminal-side-effect-facts'
import type { TerminalGitHubPRLink } from '../../shared/terminal-github-pr-link-detector'
import {
  AGENT_STATUS_STALE_AFTER_MS,
  type AgentStatusIpcPayload,
  type ParsedAgentStatusPayload,
  type AgentStatusOrchestrationContext,
  type AgentStatusEntry
} from '../../shared/agent-status-types'
import {
  hasCompatibleAgentTitleIdentity,
  normalizeCompatibleAgentStatusEntryForOwner,
  normalizeCompatibleAgentTitleForOwner
} from '../../shared/agent-title-owner'
import {
  createAgentStatusOscProcessor,
  type ProcessedAgentStatusChunk
} from '../../shared/agent-status-osc'
import { buildOrchestrationTaskDisplayMetadata } from '../../shared/orchestration-task-display'
import { iterateTerminalInputChunks } from '../../shared/terminal-input'
import {
  AGENT_PROMPT_BRACKETED_PASTE_END,
  AGENT_PROMPT_SUBMIT,
  AGENT_PROMPT_SUBMIT_DELAY_MS,
  buildAgentPromptPasteBytes
} from '../../shared/agent-prompt-injection'
import { createHash, randomUUID } from 'node:crypto'
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
  WorkspaceLineage,
  WorkspaceKey,
  WorktreeLineageWarning,
  WorktreeMeta,
  WorktreeBaseStatusEvent,
  WorktreeRemoteBranchConflictEvent,
  WorktreeStartupLaunch,
  FolderWorkspace,
  MemorySnapshot,
  Tab,
  TabGroupLayoutNode,
  TerminalLayoutSnapshot,
  TerminalPaneLayoutNode,
  TerminalTab,
  TuiAgent,
  WorkspaceSessionState
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
import { FIRST_PANE_ID } from '../../shared/pane-key'
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
import {
  folderWorkspaceKey,
  parseWorkspaceKey,
  worktreeWorkspaceKey
} from '../../shared/workspace-scope'
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
  RuntimeTerminalListResult,
  RuntimeTerminalResolvePane,
  RuntimeStatus,
  RuntimeSyncWindowGraphResult,
  RuntimeTerminalWait,
  RuntimeTerminalWaitCondition,
  RuntimeWorktreePsSummary,
  RuntimeWorktreeAgentRow,
  RuntimeSpeechModelSummary,
  RuntimeSpeechSetupState,
  RuntimeTerminalShow,
  RuntimeTerminalSummary,
  RuntimeTerminalVisualGroupNode,
  RuntimeTerminalVisualLayout,
  RuntimeTerminalVisualLayoutNode,
  RuntimeTerminalVisualPaneNode,
  RuntimeTerminalVisualTab,
  RuntimeSyncedLeaf,
  RuntimeMarkdownReadTabResult,
  RuntimeMarkdownSaveTabResult,
  RuntimeMobileSessionCreateTerminalResult,
  RuntimeMobileSessionClientTab,
  RuntimeMobileSessionMarkdownTab,
  RuntimeMobileSessionTabMove,
  RuntimeMobileSessionTabMoveResult,
  RuntimeMobileSessionTabGroup,
  RuntimeMobileSessionSnapshotTab,
  RuntimeMobileSessionTerminalTab,
  RuntimeMobileSessionBrowserTab,
  RuntimeMobileSessionTabsRemovedResult,
  RuntimeMobileSessionTabsResult,
  RuntimeMobileSessionTabsSnapshot,
  RuntimeBrowserDriverState,
  RuntimeSyncWindowGraph,
  RuntimeWorktreeListResult,
  BrowserTabInfo,
  BrowserScreencastResult
} from '../../shared/runtime-types'
import type { AutomationService } from '../automations/service'
import { RuntimeBrowserCommands } from './orca-runtime-browser'
import { buildHeadlessTerminalSplitLayout } from './headless-terminal-split-layout'
import {
  buildHeadlessTabGroupMove,
  buildHeadlessTabGroupSplit
} from './headless-tab-group-split-layout'
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
import { BrowserError } from '../browser/cdp-bridge'
import { getRepoUpstream } from '../github/client'
import {
  getLocalProjectWorktreeGitOptions,
  resolveLocalProjectRuntimeForRepo
} from '../project-runtime-git-options'
import type { ProjectExecutionRuntimeResolution } from '../../shared/project-execution-runtime'
import { FLOATING_TERMINAL_WORKTREE_ID, getDefaultVoiceSettings } from '../../shared/constants'
import type { Store } from '../persistence'
import type { StatsCollector } from '../stats/collector'
import { AgentDetector } from '../stats/agent-detector'
import { mergeWorktree, areWorktreePathsEqual } from '../ipc/worktree-logic'
import { HeadlessEmulator } from '../daemon/headless-emulator'
import {
  isNativeWindowsConptyPty,
  registerConptyDa1OverrideInstaller,
  shouldModelAnswerHiddenPtyQueries
} from './terminal-model-query-authority'
import {
  getTerminalViewAttributes,
  registerTerminalViewAttributesApplier
} from './terminal-view-attribute-store'
import { MOBILE_SUBSCRIBE_SCROLLBACK_ROWS } from './scrollback-limits'
import {
  createMobileSessionTabsNotifyCoalescer,
  type MobileSessionTabsNotifyCoalescer
} from './mobile-session-tabs-notify-coalescer'
import type { IPtyProvider, PtyTransientFact } from '../providers/types'
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
import type { ClaudeRateLimitAccountsState, CodexRateLimitAccountsState } from '../../shared/types'
import { applyPRBotAuthorOverride } from '../../shared/pr-bot-author-overrides'
import type { VoiceSettings } from '../../shared/speech-types'
import { getSpeechModelManager, getSpeechSttService } from '../speech/speech-runtime-service'
import { getCatalogModel, isLocalSpeechModel, SPEECH_MODEL_CATALOG } from '../speech/model-catalog'
import {
  deleteLocalSpeechModel,
  getSpeechModelDeletionErrorCode
} from '../speech/speech-model-deletion'
import type { CommitMessageAgentEnvironmentResolvers } from '../text-generation/commit-message-agent-environment'
import {
  MAX_TAIL_CHARS,
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
  buildVisibleSnapshotReadFallback,
  classifyAgentTitle,
  classifyLatestAgentTitle,
  compareWorktreePs,
  detectExplicitIdleStatusFromTitle,
  detectTerminalWaitBlockedReason,
  findResolvedWorktreeIdForPath,
  getLatestAgentCandidateTitle,
  getLatestAgentCandidateTitleInfo,
  getLatestLeafTitle,
  getLatestPtyTitle,
  getLeafWorktreeStatus,
  getSavedTabWorktreeStatus,
  getTerminalState,
  includeTargetResolvedWorktree,
  inferWorktreeIdFromPtyId,
  isKnownReadyPromptPreview,
  mapExplicitAgentStateToRuntimeTerminalStatus,
  maxTimestamp,
  mergeWorktreeStatus,
  MESSAGE_WAIT_DEFAULT_TIMEOUT_MS,
  normalizeTerminalChunk,
  parseRuntimeWorktreeId,
  readTerminalTail,
  type RetainedTailRedrawCursor,
  runtimePathsEqual,
  setsEqual,
  shouldFallbackToVisibleTerminalSnapshot,
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
  AccountsSnapshot,
  DriverState,
  MobileNotificationEvent,
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

type RuntimeAccountServices = {
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

type PtyForegroundAgentRefresh = {
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

type RuntimePtyTitleTrackerEntry = {
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
type RuntimeAgentRowSnapshot = {
  paneKey: string
  ptyId: string
  worktreeId?: string
  tabId?: string
  payload: ParsedAgentStatusPayload
  // When the current payload.state was first observed for this pane (ms).
  stateStartedAt: number
  updatedAt: number
}

type RuntimeHeadlessTerminal = {
  emulator: HeadlessEmulator
  // Why: serialize can race with newer writes appended to writeChain; return
  // the seq actually painted into this emulator, not the latest PTY seq.
  outputSequence: number
  writeChain: Promise<void>
}

type HeadlessSeedMetadata = {
  cwd?: string | null
  oscLinks?: TerminalOscLinkRange[]
  /** Persisted kitty flags from the daemon snapshot, re-applied to the fresh
   *  emulator so hidden `CSI ? u` answers the real flags instead of ?0u
   *  (terminal-query-authority.md §kitty). */
  kittyKeyboardFlags?: number
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

// Why: long enough for a phone to reconnect and retry a create whose response
// was lost, short enough that an intentional later re-resume forks fresh.
const MOBILE_TERMINAL_CREATE_RESULT_TTL_MS = 60_000
const FOREGROUND_AGENT_WRAPPER_RETRY_INTERVAL_MS = 150
const FOREGROUND_AGENT_WRAPPER_RETRY_TIMEOUT_MS = 6_500
const MOBILE_TERMINAL_SURFACE_TIMEOUT_MS = 10_000
const MOBILE_TERMINAL_READY_FALLBACK_MS = 1000

function isClientDisconnectedError(error: unknown): boolean {
  return error instanceof Error && error.message === 'client_disconnected'
}

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

type MessageWaiter = {
  handle: string
  typeFilter: string[] | undefined
  resolve: (result: void) => void
  timeout: NodeJS.Timeout | null
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

type TerminalWorkspaceLaunchScope = {
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

type ResolvedWorkspaceParent =
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

type WorktreeLineageCandidate = {
  source: 'env-workspace' | 'cwd-context' | 'terminal-context' | 'orchestration-context'
  parent: ResolvedWorkspaceParent
  orchestrationRunId?: string
  taskId?: string
  coordinatorHandle?: string
}

function extractOrchestrationTaskId(text?: string): string | undefined {
  return text?.match(/\btask_[A-Za-z0-9]+\b/)?.[0]
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

class WorktreeIdRequiresFullPathError extends Error {
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
  // Why: idempotency map for mobile terminal creation — a retried create with the
  // same clientMutationId returns the in-flight operation instead of duplicating.
  private mobileTerminalCreateByMutationId = new Map<
    string,
    Promise<RuntimeMobileSessionCreateTerminalResult>
  >()
  // Why: a mobile create waits for the renderer to publish the new tab's surface
  // via graph-sync, but a throttled/hidden renderer can park that past the surface
  // timeout and the create would then destroy the live PTY (#7587). This lets the
  // renderer's own PTY spawn publish the surface main-side, scoped to in-flight
  // creates so ordinary renderer spawns never publish here.
  private pendingMobileTerminalCreatesByKey = new Map<
    string,
    { activate: boolean; selectIfNoActiveTab: boolean }
  >()
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
  private ptyForegroundAgentRefreshes = new Map<string, PtyForegroundAgentRefresh>()
  private ptyDelayedForegroundSnapshotTitleObservations = new Map<string, number>()
  private _orchestrationDb: OrchestrationDb | null = null
  private messageWaitersByHandle = new Map<string, Set<MessageWaiter>>()
  // Why: mobile clients subscribe to terminal output via terminal.subscribe.
  // These listeners fire on every onPtyData call, enabling real-time streaming
  // without polling. Keyed by ptyId for O(1) lookup per data event.
  private dataListeners = new Map<
    string,
    Set<(data: string, meta?: { seq?: number; rawLength?: number; cwd?: string }) => void>
  >()
  // Why: startup draft paste can subscribe after the agent already emitted its
  // ready marker. Keep a bounded raw buffer so fast startup output is replayed.
  private recentPtyOutputById = new Map<string, string>()
  // Why: mobile clients need to know when the desktop restores a terminal
  // from mobile-fit so they can update their UI. These listeners are
  // invoked from resizeForClient and onClientDisconnected/onPtyExit.
  private fitOverrideListeners = new Map<
    string,
    Set<
      (event: {
        mode: 'mobile-fit' | 'remote-desktop-fit' | 'desktop-fit'
        cols: number
        rows: number
      }) => void
    >
  >()
  private subscriptionCleanups = new Map<string, () => void>()
  // Why: index of subscriptionIds by per-WebSocket connectionId so the
  // server can sweep all subscriptions for a closing socket without
  // touching subscriptions on other live sockets that share the same
  // deviceToken (multi-screen mobile).
  private subscriptionsByConnection = new Map<string, Set<string>>()
  private subscriptionConnectionByEntry = new Map<string, string>()
  private activeBrowserScreencastsByConnection = new Map<
    string,
    { cancel: (emitEnd?: boolean) => void; done: Promise<void>; connectionKey: string }
  >()
  private activeBrowserScreencastsByPage = new Map<
    string,
    { cancel: (emitEnd?: boolean) => void; done: Promise<void>; connectionKey: string }
  >()
  // Why: mobile clients subscribe to desktop notifications via
  // notifications.subscribe. This set enables fan-out — each connected
  // mobile client gets its own listener, and dispatchMobileNotification
  // iterates them all. Listeners are cleaned up via subscriptionCleanups.
  private notificationListeners = new Set<(event: MobileNotificationEvent) => void>()
  private titleObservationSequence = 0
  private headlessTerminals = new Map<string, RuntimeHeadlessTerminal>()
  private ptyOutputSequenceById = new Map<string, number>()
  private recentPtyPathCandidatesById = new Map<string, string[]>()
  // Why: OSC 9999 status can span PTY chunks. Keeping parser state in the
  // runtime lets hidden/model-owned terminals observe agent state without a
  // mounted xterm view.
  // Why a throttle: the blocked-reason check builds and scans two full wait
  // texts (<=256KB each, lowercased) — measured at ~85% of onPtyData's cost
  // under a TUI flood (findings log 2026-07-03). PTY chunk boundaries are
  // arbitrary, so running the identical computation over coalesced chunks at
  // a bounded cadence (plus a trailing-edge timer so burst-final state is
  // always evaluated) preserves semantics while removing it from the hot path.
  private waitBlockedCheckStateByPtyId = new Map<
    string,
    {
      lastAt: number
      lastWaitState: TerminalTailWaitState | null
      appended: string
      keywordCarry: string
      timer: ReturnType<typeof setTimeout> | null
    }
  >()

  private agentStatusOscProcessorsByPtyId = new Map<
    string,
    ReturnType<typeof createAgentStatusOscProcessor>
  >()
  // Why: per-PTY shared title trackers (all-titles ordering + stale-working
  // timer) replace last-title-per-chunk scanning so main observes the same
  // intra-chunk working→idle transitions the renderer does (issue #1083).
  // Lazily created like agentStatusOscProcessorsByPtyId; disposed on PTY exit.
  private ptyTitleTrackersByPtyId = new Map<string, RuntimePtyTitleTrackerEntry>()
  // Why: the Command Code output detector arms early from the launch command
  // when known (banner detection covers user-typed launches), mirroring the
  // renderer detector's startupCommand seed.
  private terminalSpawnCommandsByPtyId = new Map<string, string>()
  // Why: ordinary OSC 0/1/2 titles can split across PTY chunks, especially over
  // SSH/relay buffering. Keep a small raw scan tail and feed reconstructed
  // chunks into the title tracker instead of falling back to last-title scans.
  private oscTitleScanTailByPtyId = new Map<string, string>()
  // Why: mobile file taps resolve relative paths on the host. OSC 7 is the
  // terminal-owned cwd signal, and it can arrive in live output between snapshots.
  private osc7ScanTailByPtyId = new Map<string, string>()
  private terminalCwdByPtyId = new Map<string, string>()
  private terminalFileUriHostnameByPtyId = new Map<string, string>()
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

  // Why: Phase-5 query-responder suppression — a terminal-RPC subscribe
  // stream feeds a remote xterm view (mobile/web/remote desktop) that answers
  // queries with view authority, so main must yield while one is attached
  // (terminal-query-authority.md). Ref-counted per PTY because multiple
  // streams can attach concurrently; mobileSubscribers is consulted too so
  // grace-window mobile records keep suppressing.
  private remoteTerminalViewSubscriberCounts = new Map<string, number>()

  private stats: StatsCollector | null = null
  private readonly getLocalProviderFn: (() => IPtyProvider) | null
  private readonly onPtyStopped: ((ptyId: string) => void) | null
  private readonly onTerminalAgentStatus: ((event: RuntimeTerminalAgentStatusEvent) => void) | null
  private readonly onTerminalSideEffects: ((batch: TerminalSideEffectBatch) => void) | null
  private readonly getAgentStatusSnapshotFn: (() => AgentStatusIpcPayload[]) | null
  private readonly buildAgentHookPtyEnv: (() => Record<string, string>) | null
  private accountServices: RuntimeAccountServices | null = null
  private commitMessageAgentEnv: CommitMessageAgentEnvironmentResolvers | null = null
  private automationService: AutomationService | null = null
  private readonly claudeAgentTeams = new ClaudeAgentTeamsService()
  private mobileDictation: {
    id: string
    owner: string
    clientId?: string
    connectionId?: string
    state: 'starting' | 'active' | 'closing'
    partialText: string
    finalTexts: string[]
    errors: string[]
  } | null = null

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
      for (const state of this.headlessTerminals.values()) {
        state.emulator.applyPushedViewAttributes(attributes)
      }
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

  async listAllMobileSessionTabs(): Promise<RuntimeMobileSessionTabsResult[]> {
    this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession()
    await this.refreshMobileSessionPtyRecords()
    return [...this.mobileSessionTabsByWorktree.values()].map((snapshot) =>
      this.toMobileSessionTabsResult(snapshot)
    )
  }

  private hydrateHeadlessMobileSessionTabsFromWorkspaceSession(
    worktreeId?: string,
    options: {
      force?: boolean
      allowAttachedWindow?: boolean
      onlyServeOwnedTerminals?: boolean
    } = {}
  ): void {
    if (this.getAvailableAuthoritativeWindow() && options.allowAttachedWindow !== true) {
      return
    }
    const session = this.store?.getWorkspaceSession?.()
    if (!session) {
      return
    }
    const entries =
      worktreeId !== undefined
        ? ([[worktreeId, session.tabsByWorktree[worktreeId] ?? []]] as const)
        : Object.entries(session.tabsByWorktree ?? {})
    for (const [entryWorktreeId, persistedTabs] of entries) {
      const existing = this.mobileSessionTabsByWorktree.get(entryWorktreeId)
      if (
        existing &&
        existing.tabs.length > 0 &&
        options.force !== true &&
        options.onlyServeOwnedTerminals !== true
      ) {
        // Why: terminals are stable/persisted so we normally skip a rebuild, but
        // offscreen browser tabs are live and may have been created/closed since.
        // Reconcile just the browser tabs against the live bridge instead of
        // leaving a stale snapshot that omits a freshly-opened browser tab.
        this.reconcileHeadlessMobileSessionBrowserTabs(entryWorktreeId, existing)
        continue
      }
      const terminalTabs = this.buildHeadlessMobileSessionTerminalTabs(
        entryWorktreeId,
        persistedTabs
      ).filter(
        (tab) => options.onlyServeOwnedTerminals !== true || this.hasServeOwnedPtyBinding(tab)
      )
      // Why: offscreen browser panes are live-only (no persisted session entry),
      // so include them on every hydrate regardless of the onlyServeOwnedTerminals
      // filter, which is about terminal PTY ownership and never applies to browsers.
      const browserTabs = this.buildHeadlessMobileSessionBrowserTabs(entryWorktreeId)
      const tabs: RuntimeMobileSessionSnapshotTab[] = [...terminalTabs, ...browserTabs]
      if (tabs.length === 0) {
        continue
      }
      const activeTab = this.pickHeadlessActiveTerminalTab(terminalTabs)
      const tabOrder = [
        ...this.collectHeadlessParentTabOrder(terminalTabs),
        ...browserTabs.map((tab) => tab.id)
      ]
      const groupId = this.getHeadlessMobileSessionGroupId(entryWorktreeId)
      const mergedTabs =
        options.onlyServeOwnedTerminals === true && existing
          ? this.mergeMobileSessionSnapshotTabs(existing.tabs, tabs)
          : tabs
      const mergedActiveTab =
        existing?.tabs.find((tab) => tab.id === existing.activeTabId) ??
        activeTab ??
        mergedTabs[0] ??
        null
      const mergedTerminalTabs = mergedTabs.filter(
        (tab): tab is RuntimeMobileSessionTerminalTab => tab.type === 'terminal'
      )
      const mergedBrowserOrder = mergedTabs
        .filter((tab): tab is RuntimeMobileSessionBrowserTab => tab.type === 'browser')
        .map((tab) => tab.id)
      // Why: a persisted multi-group split must be restored on cold rebuild, or
      // the headless serve coalesces the user's group layout back into one group
      // (the persisted tabGroups/tabGroupLayouts would otherwise be write-only).
      const persistedGroups = session.tabGroups?.[entryWorktreeId]
      const persistedLayout = session.tabGroupLayouts?.[entryWorktreeId]
      const hasPersistedSplit =
        options.onlyServeOwnedTerminals !== true &&
        persistedGroups !== undefined &&
        persistedGroups.length > 1
      const activeTopLevelId = mergedActiveTab
        ? mergedActiveTab.type === 'terminal'
          ? mergedActiveTab.parentTabId
          : mergedActiveTab.id
        : null
      this.mobileSessionTabsByWorktree.set(entryWorktreeId, {
        worktree: existing?.worktree ?? entryWorktreeId,
        publicationEpoch: `headless-hydrated:${Date.now().toString(36)}`,
        snapshotVersion: (existing?.snapshotVersion ?? 0) + 1,
        activeGroupId: existing?.activeGroupId ?? groupId,
        activeTabId: mergedActiveTab?.id ?? null,
        activeTabType: mergedActiveTab?.type ?? null,
        tabGroups: hasPersistedSplit
          ? this.appendBrowserTabOrder(
              this.distributeHeadlessTabsAcrossGroups(
                persistedGroups.map((group) => ({
                  id: group.id,
                  activeTabId: group.activeTabId,
                  tabOrder: [...group.tabOrder],
                  ...(group.recentTabIds ? { recentTabIds: [...group.recentTabIds] } : {})
                })),
                this.collectHeadlessParentTabOrder(mergedTerminalTabs),
                activeTopLevelId
              ),
              mergedBrowserOrder,
              undefined,
              // Why: distribute drops browser ids (terminal-only), so carry each
              // browser's persisted group forward instead of coalescing left.
              this.collectBrowserGroupAssignment(persistedGroups, mergedBrowserOrder)
            )
          : options.onlyServeOwnedTerminals === true && existing?.tabGroups
            ? this.appendBrowserTabOrder(
                this.mergeMobileSessionTabGroups(
                  entryWorktreeId,
                  existing.tabGroups,
                  mergedTerminalTabs,
                  mergedActiveTab?.type === 'terminal' ? mergedActiveTab : null
                ),
                mergedBrowserOrder
              )
            : [
                {
                  id: groupId,
                  activeTabId: mergedActiveTab?.id
                    ? (activeTab?.parentTabId ?? mergedActiveTab.id)
                    : (tabOrder[0] ?? null),
                  tabOrder
                }
              ],
        ...(hasPersistedSplit && persistedLayout ? { tabGroupLayout: persistedLayout } : {}),
        tabs: mergedTabs
      })
    }
  }

  // Why: keep an existing snapshot's browser tabs in sync with the live bridge
  // without rebuilding stable terminal state. Replaces browser entries with the
  // current live set and rewrites the browser portion of the primary group order.
  private reconcileHeadlessMobileSessionBrowserTabs(
    worktreeId: string,
    existing: RuntimeMobileSessionTabsSnapshot
  ): void {
    if (!this.offscreenBrowserBackend) {
      return
    }
    const liveBrowserTabs = this.buildHeadlessMobileSessionBrowserTabs(worktreeId)
    const liveIds = liveBrowserTabs.map((tab) => tab.id)
    const existingBrowserIds = existing.tabs
      .filter((tab): tab is RuntimeMobileSessionBrowserTab => tab.type === 'browser')
      .map((tab) => tab.id)
    const unchanged =
      liveIds.length === existingBrowserIds.length &&
      liveIds.every((id, index) => existingBrowserIds[index] === id)
    if (unchanged) {
      return
    }
    const nonBrowserTabs = existing.tabs.filter((tab) => tab.type !== 'browser')
    const nextTabs: RuntimeMobileSessionSnapshotTab[] = [...nonBrowserTabs, ...liveBrowserTabs]
    const liveIdSet = new Set(liveIds)
    const tabGroups = this.appendBrowserTabOrder(
      (existing.tabGroups ?? []).map((group) => ({
        ...group,
        // Drop closed browser ids; appendBrowserTabOrder re-adds the live ones.
        tabOrder: group.tabOrder.filter(
          (id) => liveIdSet.has(id) || !existingBrowserIds.includes(id)
        )
      })),
      liveIds
    )
    const activeStillPresent = nextTabs.some((tab) => tab.id === existing.activeTabId)
    const active = activeStillPresent
      ? null
      : (nextTabs.find((tab) => tab.isActive) ?? nextTabs[0] ?? null)
    this.mobileSessionTabsByWorktree.set(worktreeId, {
      ...existing,
      publicationEpoch: `headless-hydrated:${Date.now().toString(36)}`,
      snapshotVersion: existing.snapshotVersion + 1,
      ...(activeStillPresent
        ? {}
        : { activeTabId: active?.id ?? null, activeTabType: active?.type ?? null }),
      tabGroups,
      tabs: nextTabs
    })
  }

  // Why: browser session tabs have no parentTabId so the terminal-only group
  // builder drops them from tabOrder; this re-adds their ids to a group.
  // Browser tabs are live-only (no persisted session entry), but their GROUP
  // membership must still survive snapshot rebuilds like terminals'. The
  // passed-in groups already encode each browser's group (carried from the prior
  // snapshot / persisted tabGroups), so keep each existing browser id where it
  // is; only a genuinely-new browser id goes to its create-target group (when
  // that group exists) and otherwise to the first group. Previously every
  // browser was force-pushed into group[0], so opening a browser in the right
  // split group always snapped it back to the left on the next rebuild.
  private appendBrowserTabOrder(
    groups: readonly RuntimeMobileSessionTabGroup[],
    browserTabIds: readonly string[],
    newTabAssignment?: { tabId: string; groupId: string },
    // browserPageId -> groupId from the prior/persisted groups. The terminal
    // distributor rebuilds tabOrder from terminal ids only and drops browser
    // ids, so this carries each browser's group across rebuilds.
    priorGroupByBrowserId?: ReadonlyMap<string, string>
  ): RuntimeMobileSessionTabGroup[] {
    if (browserTabIds.length === 0) {
      return [...groups]
    }
    const next = groups.map((group) => ({ ...group, tabOrder: [...group.tabOrder] }))
    if (next.length === 0) {
      return next
    }
    const groupById = new Map(next.map((group) => [group.id, group]))
    const ownerGroupByTabId = new Map<string, RuntimeMobileSessionTabGroup>()
    for (const group of next) {
      for (const id of group.tabOrder) {
        ownerGroupByTabId.set(id, group)
      }
    }
    for (const id of browserTabIds) {
      if (ownerGroupByTabId.has(id)) {
        continue
      }
      const priorGroupId = priorGroupByBrowserId?.get(id)
      const targetGroup =
        (newTabAssignment?.tabId === id ? groupById.get(newTabAssignment.groupId) : undefined) ??
        (priorGroupId ? groupById.get(priorGroupId) : undefined) ??
        next[0]!
      targetGroup.tabOrder.push(id)
    }
    return next
  }

  // browserPageId -> groupId from a set of groups (the persisted/prior layout),
  // so a browser stays in its group across rebuilds that drop browser ids.
  private collectBrowserGroupAssignment(
    groups: readonly RuntimeMobileSessionTabGroup[] | undefined,
    browserTabIds: readonly string[]
  ): Map<string, string> {
    const browserIdSet = new Set(browserTabIds)
    const assignment = new Map<string, string>()
    for (const group of groups ?? []) {
      for (const id of group.tabOrder) {
        if (browserIdSet.has(id)) {
          assignment.set(id, group.id)
        }
      }
    }
    return assignment
  }

  private isServeOwnedPtyId(ptyId: string | null | undefined): boolean {
    return typeof ptyId === 'string' && ptyId.startsWith('serve-')
  }

  private hasServeOwnedPtyBinding(tab: RuntimeMobileSessionTerminalTab): boolean {
    if (this.isServeOwnedPtyId(tab.ptyId)) {
      return true
    }
    return Object.values(tab.parentLayout?.ptyIdsByLeafId ?? {}).some((ptyId) =>
      this.isServeOwnedPtyId(ptyId)
    )
  }

  // Why: serve-* (local serve) and ssh:<conn>@@<relay> (SSH relay) ids are minted
  // ONLY for runtime-owned terminals and are preserved/re-hydrated, so tear them
  // down even if the renderer adopted a view (else they resurrect). The daemon
  // session form <worktreeId>@@<shortUuid> is deliberately NOT here: the daemon
  // mints it for ordinary renderer-owned local terminals too, so id shape can't
  // classify ownership for that form — renderer-graph membership does (below).
  private isServeOrSshOwnedPtyId(ptyId: string | null | undefined): boolean {
    return (
      this.isServeOwnedPtyId(ptyId) ||
      (typeof ptyId === 'string' && parseAppSshPtyId(ptyId) !== null)
    )
  }

  private hasServeOrSshOwnedBinding(tab: RuntimeMobileSessionTerminalTab): boolean {
    if (this.isServeOrSshOwnedPtyId(tab.ptyId)) {
      return true
    }
    return Object.values(tab.parentLayout?.ptyIdsByLeafId ?? {}).some((ptyId) =>
      this.isServeOrSshOwnedPtyId(ptyId)
    )
  }

  // Why: a tab needs authoritative runtime teardown (kill + de-persist + prune)
  // only when the renderer can't durably tear it down: either it's serve/SSH
  // (preserved + re-hydrated, would resurrect) or the renderer graph never
  // published it (a leaked/unadopted shell — incl. daemon-session `@@` tabs the
  // host materialized but the renderer never showed). A tab the renderer graph
  // DOES list — including an ordinary daemon-backed local terminal or a pending
  // tab whose PTY hasn't bound — is renderer-owned: delegate, do not de-persist.
  private isRuntimeOwnedHeadlessMobileTab(
    worktreeId: string,
    tab: RuntimeMobileSessionTerminalTab
  ): boolean {
    if (this.hasServeOrSshOwnedBinding(tab)) {
      return true
    }
    const pty = this.findPtyForMobileTerminalTab(worktreeId, tab)
    if (pty && this.isServeOrSshOwnedPtyId(pty.ptyId)) {
      return true
    }
    return !this.graph.tabs.has(tab.parentTabId)
  }

  private mergeMobileSessionSnapshotTabs(
    baseTabs: readonly RuntimeMobileSessionSnapshotTab[],
    extraTabs: readonly RuntimeMobileSessionSnapshotTab[]
  ): RuntimeMobileSessionSnapshotTab[] {
    const seenIds = new Set<string>()
    const merged: RuntimeMobileSessionSnapshotTab[] = []
    const add = (tab: RuntimeMobileSessionSnapshotTab): void => {
      const ids = this.getMobileSessionSnapshotTabIdentityKeys(tab)
      if (ids.some((id) => seenIds.has(id))) {
        return
      }
      for (const id of ids) {
        seenIds.add(id)
      }
      merged.push(tab)
    }
    for (const tab of baseTabs) {
      add(tab)
    }
    for (const tab of extraTabs) {
      add(tab)
    }
    return merged
  }

  private getMobileSessionSnapshotTabIdentityKeys(tab: RuntimeMobileSessionSnapshotTab): string[] {
    if (tab.type === 'terminal') {
      // Why: split terminal leaves share one parent tab; merge dedup must stay
      // leaf-scoped or preserved siblings collapse into a single surface.
      const keys = [tab.id, `${tab.parentTabId}::${tab.leafId}`]
      if (typeof tab.ptyId === 'string' && tab.ptyId.length > 0) {
        // Why: renderer and headless sources can derive different leafIds for the same
        // terminal; real PTYs collapse those duplicates without merging pending splits.
        keys.push(`${tab.parentTabId}::pty:${tab.ptyId}`)
      }
      return keys
    }
    if (tab.type === 'browser') {
      return [tab.id, tab.browserWorkspaceId]
    }
    return [tab.id]
  }

  private mergeMobileSessionTabGroups(
    worktreeId: string,
    groups: readonly RuntimeMobileSessionTabGroup[],
    terminalTabs: readonly RuntimeMobileSessionTerminalTab[],
    activeTab: RuntimeMobileSessionTerminalTab | null
  ): RuntimeMobileSessionTabGroup[] {
    const parentTabOrder = this.collectHeadlessParentTabOrder(terminalTabs)
    if (parentTabOrder.length === 0) {
      return [...groups]
    }
    const targetGroupId = groups[0]?.id ?? this.getHeadlessMobileSessionGroupId(worktreeId)
    const nextGroups =
      groups.length > 0
        ? groups.map((group) => ({ ...group, tabOrder: [...group.tabOrder] }))
        : [
            {
              id: targetGroupId,
              activeTabId: null,
              tabOrder: []
            }
          ]
    // Why: keep each tab in the group that already owns it (a multi-group split
    // must survive the merge), drop tabs no longer present, and route only
    // genuinely-new tabs into the active group — never funnel everything into
    // group[0], which duplicated/coalesced tabs that lived in other groups.
    const ownerGroupId = new Map<string, string>()
    for (const group of nextGroups) {
      for (const tabId of group.tabOrder) {
        ownerGroupId.set(tabId, group.id)
      }
    }
    const liveTabIds = new Set(parentTabOrder)
    const activeParentId = activeTab?.parentTabId ?? null
    const activeGroupId =
      (activeParentId ? ownerGroupId.get(activeParentId) : undefined) ?? nextGroups[0]!.id
    const retainedOrder = new Map<string, string[]>(nextGroups.map((group) => [group.id, []]))
    for (const tabId of parentTabOrder) {
      const groupId = ownerGroupId.get(tabId) ?? activeGroupId
      retainedOrder.get(groupId)?.push(tabId)
    }
    return nextGroups
      .map((group) => {
        const tabOrder = retainedOrder.get(group.id) ?? []
        const keptActive =
          group.activeTabId &&
          tabOrder.includes(group.activeTabId) &&
          liveTabIds.has(group.activeTabId)
            ? group.activeTabId
            : null
        return {
          ...group,
          tabOrder,
          activeTabId:
            activeParentId && tabOrder.includes(activeParentId)
              ? activeParentId
              : (keptActive ?? tabOrder[0] ?? null)
        }
      })
      .filter((group) => group.tabOrder.length > 0)
  }

  /**
   * Publishes a PTY-backed terminal tab snapshot to the synced mobile session,
   * normalizing Pi-compatible titles based on launch or foreground ownership.
   */
  private publishPtyBackedMobileSessionTerminal(
    worktreeId: string,
    pty: RuntimePtyWorktreeRecord,
    args: {
      tabId: string
      leafId: string
      title: string | null
      activate: boolean
      selectIfNoActiveTab?: boolean
      startupCwd?: string
      split?: { splitFromLeafId: string; direction: 'horizontal' | 'vertical' }
    }
  ): void {
    const existing = this.mobileSessionTabsByWorktree.get(worktreeId)
    const ownerAgent = pty.launchAgent ?? pty.foregroundAgent
    const title = normalizeCompatibleAgentTitleForOwner(
      args.title ?? getLatestPtyTitle(pty) ?? 'Terminal',
      ownerAgent
    )
    const existingTab = existing?.tabs.find(
      (candidate): candidate is RuntimeMobileSessionTerminalTab =>
        candidate.type === 'terminal' &&
        candidate.parentTabId === args.tabId &&
        candidate.leafId === args.leafId
    )
    // Why: a split inserts into the parent tab's layout, which lives on the
    // sibling surface, not this new leaf's (empty) existing surface.
    const baseLayout = args.split
      ? (existing?.tabs.find(
          (candidate): candidate is RuntimeMobileSessionTerminalTab =>
            candidate.type === 'terminal' &&
            candidate.parentTabId === args.tabId &&
            candidate.leafId === args.split!.splitFromLeafId
        )?.parentLayout ?? existingTab?.parentLayout)
      : existingTab?.parentLayout
    const parentLayout = this.buildMaterializedHeadlessParentLayout(
      args.leafId,
      pty.ptyId,
      baseLayout,
      args.split
    )
    const tab: RuntimeMobileSessionTerminalTab = {
      type: 'terminal',
      id: `${args.tabId}::${args.leafId}`,
      parentTabId: args.tabId,
      leafId: args.leafId,
      ptyId: pty.ptyId,
      title,
      ...(pty.launchAgent ? { launchAgent: pty.launchAgent } : {}),
      ...(args.startupCwd ? { startupCwd: args.startupCwd } : {}),
      parentLayout,
      isActive:
        args.activate || (args.selectIfNoActiveTab !== false && existing?.activeTabId == null)
    }
    const existingTabs = (existing?.tabs ?? []).filter(
      (candidate) =>
        !(
          candidate.type === 'terminal' &&
          candidate.parentTabId === args.tabId &&
          candidate.leafId === args.leafId
        )
    )
    const tabs = this.mergeMobileSessionSnapshotTabs(
      existingTabs.map((candidate) => ({
        ...candidate,
        // Why: the client picks one sibling's parentLayout to render the whole
        // tab; a split must update every sibling surface to the new tree, or a
        // stale single-leaf sibling makes the client fall back to a default
        // direction ("Split Right" renders as down).
        ...(args.split && candidate.type === 'terminal' && candidate.parentTabId === args.tabId
          ? { parentLayout }
          : {}),
        isActive: tab.isActive ? false : candidate.isActive
      })),
      [tab]
    )
    const activeTab =
      (tab.isActive ? tab : tabs.find((candidate) => candidate.id === existing?.activeTabId)) ??
      tabs.find((candidate) => candidate.isActive) ??
      (args.selectIfNoActiveTab !== false ? tabs[0] : null) ??
      null
    const terminalTabs = tabs.filter(
      (candidate): candidate is RuntimeMobileSessionTerminalTab => candidate.type === 'terminal'
    )
    const next: RuntimeMobileSessionTabsSnapshot = {
      worktree: worktreeId,
      publicationEpoch:
        existing?.publicationEpoch ?? `headless:pty-backed:${Date.now().toString(36)}`,
      snapshotVersion: (existing?.snapshotVersion ?? 0) + 1,
      activeGroupId: existing?.activeGroupId ?? this.getHeadlessMobileSessionGroupId(worktreeId),
      activeTabId: activeTab?.id ?? null,
      activeTabType: activeTab?.type ?? null,
      tabGroups: this.mergeMobileSessionTabGroups(
        worktreeId,
        existing?.tabGroups ?? [],
        terminalTabs,
        activeTab?.type === 'terminal' ? activeTab : null
      ),
      ...(existing?.tabGroupLayout ? { tabGroupLayout: existing.tabGroupLayout } : {}),
      tabs
    }
    this.mobileSessionTabsByWorktree.set(worktreeId, next)
    this.notifyMobileSessionTabsChanged(worktreeId)
  }

  private touchMobileSessionSnapshotsForPty(
    ptyId: string,
    options: { immediate?: boolean } = {}
  ): void {
    for (const [worktreeId, snapshot] of this.mobileSessionTabsByWorktree) {
      const hasPtyBackedTab = snapshot.tabs.some(
        (tab) =>
          tab.type === 'terminal' &&
          (tab.ptyId === ptyId || tab.parentLayout?.ptyIdsByLeafId?.[tab.leafId] === ptyId)
      )
      if (!hasPtyBackedTab) {
        continue
      }
      this.mobileSessionTabsByWorktree.set(worktreeId, {
        ...snapshot,
        snapshotVersion: snapshot.snapshotVersion + 1
      })
      if (options.immediate) {
        // Why: readiness/lifecycle changes are structural and must not wait
        // behind the title/status coalescing window.
        this.notifyMobileSessionTabsChanged(worktreeId)
      } else {
        // Why: title/status flips several times a second under spinner-in-title
        // agents. Coalesce the emit instead of fanning out every version.
        this.mobileSessionTabsNotifyCoalescer.schedule(worktreeId)
      }
    }
  }

  private buildHeadlessMobileSessionTerminalTabs(
    worktreeId: string,
    persistedTabs: readonly TerminalTab[]
  ): RuntimeMobileSessionTerminalTab[] {
    const session = this.store?.getWorkspaceSession?.()
    if (!session) {
      return []
    }
    return [...persistedTabs]
      .sort((a, b) => a.sortOrder - b.sortOrder || a.createdAt - b.createdAt)
      .flatMap((tab, index) => {
        const layout = session.terminalLayoutsByTabId?.[tab.id]
        const leafIds = this.collectPersistedTerminalLeafIds(layout)
        if (leafIds.length === 0) {
          leafIds.push(this.deriveHeadlessLegacyTerminalLeafId(tab.id))
        }
        return leafIds.map((leafId) => {
          const ptyId =
            layout?.ptyIdsByLeafId?.[leafId] ?? (leafIds.length === 1 ? tab.ptyId : null)
          const title =
            tab.customTitle?.trim() ||
            tab.generatedTitle?.trim() ||
            tab.title?.trim() ||
            tab.defaultTitle?.trim() ||
            `Terminal ${index + 1}`
          return {
            type: 'terminal' as const,
            id: `${tab.id}::${leafId}`,
            parentTabId: tab.id,
            leafId,
            title,
            ...(ptyId ? { ptyId } : {}),
            ...(tab.startupCwd ? { startupCwd: tab.startupCwd } : {}),
            ...(tab.launchAgent ? { launchAgent: tab.launchAgent } : {}),
            ...(layout ? { parentLayout: this.cloneTerminalLayoutSnapshot(layout) } : {}),
            ...(tab.color != null ? { color: tab.color } : {}),
            ...(tab.isPinned ? { isPinned: true } : {}),
            ...(tab.viewMode ? { viewMode: tab.viewMode } : {}),
            isActive: this.isPersistedTerminalLeafActive(worktreeId, tab.id, leafId, layout)
          }
        })
      })
  }

  // Why: headless serve backs browser panes with offscreen WebContents that live
  // only in the BrowserManager, never in a renderer graph. Without surfacing them
  // as session tabs, a session.tabs snapshot (e.g. on terminal open) prunes the
  // paired browser tab and closing it fails with tab_not_found. Synthesize browser
  // session tabs from the live bridge so they are first-class alongside terminals.
  private buildHeadlessMobileSessionBrowserTabs(
    worktreeId: string
  ): RuntimeMobileSessionBrowserTab[] {
    if (!this.offscreenBrowserBackend || !this.agentBrowserBridge?.tabList) {
      return []
    }
    return this.agentBrowserBridge.tabList(worktreeId).tabs.map((tab) => {
      const persistedProps = this.getPersistedUnifiedSessionTabProps(worktreeId, tab.browserPageId)
      return {
        type: 'browser' as const,
        // Why: an offscreen page has no separate workspace identity, so the page id
        // is its own workspace id (matches the server's browserWorkspaceId fallback).
        id: tab.browserPageId,
        title: tab.title || tab.url || 'Browser',
        browserWorkspaceId: tab.browserPageId,
        browserPageId: tab.browserPageId,
        url: tab.url || 'about:blank',
        loading: false,
        canGoBack: false,
        canGoForward: false,
        ...(persistedProps ? { color: persistedProps.color } : {}),
        ...(persistedProps ? { isPinned: persistedProps.isPinned === true } : {}),
        isActive: tab.active === true
      }
    })
  }

  private getPersistedUnifiedSessionTabProps(
    worktreeId: string,
    tabId: string
  ): Pick<Tab, 'color' | 'isPinned'> | null {
    const tab =
      this.store
        ?.getWorkspaceSession?.()
        ?.unifiedTabs?.[worktreeId]?.find(
          (candidate) => candidate.id === tabId || candidate.entityId === tabId
        ) ?? null
    return tab ? { color: tab.color, isPinned: tab.isPinned } : null
  }

  private collectPersistedTerminalLeafIds(layout: TerminalLayoutSnapshot | undefined): string[] {
    if (!layout) {
      return []
    }
    const leafIds = new Set<string>()
    const visit = (node: TerminalLayoutSnapshot['root']): void => {
      if (!node) {
        return
      }
      if (node.type === 'leaf') {
        if (isTerminalLeafId(node.leafId)) {
          leafIds.add(node.leafId)
        }
        return
      }
      visit(node.first)
      visit(node.second)
    }
    visit(layout.root)
    if (layout.activeLeafId && isTerminalLeafId(layout.activeLeafId)) {
      leafIds.add(layout.activeLeafId)
    }
    for (const leafId of Object.keys(layout.ptyIdsByLeafId ?? {})) {
      if (isTerminalLeafId(leafId)) {
        leafIds.add(leafId)
      }
    }
    return [...leafIds]
  }

  private deriveHeadlessLegacyTerminalLeafId(tabId: string): string {
    const hash = createHash('sha256').update(`headless-terminal-leaf:${tabId}`).digest('hex')
    const variant = ((Number.parseInt(hash.slice(16, 17), 16) & 0x3) | 0x8).toString(16)
    const leafId = [
      hash.slice(0, 8),
      hash.slice(8, 12),
      `4${hash.slice(13, 16)}`,
      `${variant}${hash.slice(17, 20)}`,
      hash.slice(20, 32)
    ].join('-')
    if (!isTerminalLeafId(leafId)) {
      return randomUUID()
    }
    return leafId
  }

  private cloneTerminalLayoutSnapshot(layout: TerminalLayoutSnapshot): TerminalLayoutSnapshot {
    const cloned: TerminalLayoutSnapshot = {
      root: layout.root,
      activeLeafId: layout.activeLeafId,
      expandedLeafId: layout.expandedLeafId
    }
    if (layout.ptyIdsByLeafId) {
      cloned.ptyIdsByLeafId = { ...layout.ptyIdsByLeafId }
    }
    if (layout.buffersByLeafId) {
      cloned.buffersByLeafId = { ...layout.buffersByLeafId }
    }
    if (layout.scrollbackRefsByLeafId) {
      cloned.scrollbackRefsByLeafId = { ...layout.scrollbackRefsByLeafId }
    }
    if (layout.titlesByLeafId) {
      cloned.titlesByLeafId = { ...layout.titlesByLeafId }
    }
    return cloned
  }

  private isPersistedTerminalLeafActive(
    worktreeId: string,
    tabId: string,
    leafId: string,
    layout: TerminalLayoutSnapshot | undefined
  ): boolean {
    const session = this.store?.getWorkspaceSession?.()
    const activeTabId = session?.activeTabIdByWorktree?.[worktreeId] ?? session?.activeTabId
    return activeTabId === tabId && (!layout?.activeLeafId || layout.activeLeafId === leafId)
  }

  private pickHeadlessActiveTerminalTab(
    tabs: readonly RuntimeMobileSessionTerminalTab[]
  ): RuntimeMobileSessionTerminalTab | null {
    return tabs.find((tab) => tab.isActive) ?? tabs.find((tab) => tab.parentTabId) ?? null
  }

  private collectHeadlessParentTabOrder(
    tabs: readonly RuntimeMobileSessionTerminalTab[]
  ): string[] {
    const order: string[] = []
    const seen = new Set<string>()
    for (const tab of tabs) {
      if (!seen.has(tab.parentTabId)) {
        seen.add(tab.parentTabId)
        order.push(tab.parentTabId)
      }
    }
    return order
  }

  // Why: the group tab order must follow actual creation/insertion order across
  // both terminals and browsers, not list terminals first. A terminal's top-level
  // id is its parentTabId (split leaves share one); a browser's is its own id.
  private collectHeadlessTopLevelTabOrder(
    tabs: readonly RuntimeMobileSessionSnapshotTab[]
  ): string[] {
    const order: string[] = []
    const seen = new Set<string>()
    for (const tab of tabs) {
      const topLevelId = tab.type === 'terminal' ? tab.parentTabId : tab.id
      if (!seen.has(topLevelId)) {
        seen.add(topLevelId)
        order.push(topLevelId)
      }
    }
    return order
  }

  private getHeadlessMobileSessionGroupId(worktreeId: string): string {
    return `headless-terminals:${worktreeId}`
  }

  private buildHeadlessMobileSessionTabGroups(
    worktreeId: string,
    tabs: readonly RuntimeMobileSessionSnapshotTab[],
    activeTab: RuntimeMobileSessionSnapshotTab | null,
    existingGroups?: readonly RuntimeMobileSessionTabGroup[],
    // Why: a new tab created via a specific group's "+" must land in THAT group,
    // not the active one — otherwise every "+" in a split funnels to one group.
    newTabAssignment?: { tabId: string; groupId: string }
  ): RuntimeMobileSessionTabGroup[] {
    // Why: order across terminals and browsers in their actual array order so a
    // tab opened after a browser tab lands to its right, not regrouped before it.
    const tabOrder = this.collectHeadlessTopLevelTabOrder(tabs)
    const topLevelOf = (tab: RuntimeMobileSessionSnapshotTab): string =>
      tab.type === 'terminal' ? tab.parentTabId : tab.id
    const activeTopLevelId =
      (activeTab ? topLevelOf(activeTab) : null) ??
      existingGroups?.[0]?.activeTabId ??
      (() => {
        const active = tabs.find((tab) => tab.isActive)
        return active ? topLevelOf(active) : null
      })() ??
      tabOrder[0] ??
      null

    // Why: when the user has split tabs into multiple groups, preserve that
    // assignment across rebuilds instead of coalescing back to one group.
    if (existingGroups && existingGroups.length > 1) {
      return this.distributeHeadlessTabsAcrossGroups(
        existingGroups,
        tabOrder,
        activeTopLevelId,
        newTabAssignment
      )
    }

    const groupId = existingGroups?.[0]?.id ?? this.getHeadlessMobileSessionGroupId(worktreeId)
    return [
      {
        id: groupId,
        activeTabId:
          activeTopLevelId && tabOrder.includes(activeTopLevelId)
            ? activeTopLevelId
            : (tabOrder[0] ?? null),
        tabOrder
      }
    ]
  }

  // Distribute live top-level tabs into the existing multi-group structure,
  // keeping each tab in its group; tabs new since the last snapshot join the
  // active group. Emptied groups are dropped so a closed split collapses.
  private distributeHeadlessTabsAcrossGroups(
    existingGroups: readonly RuntimeMobileSessionTabGroup[],
    tabOrder: readonly string[],
    activeTopLevelId: string | null,
    newTabAssignment?: { tabId: string; groupId: string }
  ): RuntimeMobileSessionTabGroup[] {
    const groupIdByTabId = new Map<string, string>()
    for (const group of existingGroups) {
      for (const tabId of group.tabOrder) {
        groupIdByTabId.set(tabId, group.id)
      }
    }
    // Why: route a freshly-created tab to the group its "+" was clicked in,
    // when that group still exists; otherwise fall through to the active group.
    const hasTargetGroup =
      newTabAssignment !== undefined &&
      existingGroups.some((group) => group.id === newTabAssignment.groupId)
    if (hasTargetGroup) {
      groupIdByTabId.set(newTabAssignment!.tabId, newTabAssignment!.groupId)
    }
    const activeGroupId =
      (activeTopLevelId ? groupIdByTabId.get(activeTopLevelId) : undefined) ?? existingGroups[0]!.id
    const orderByGroup = new Map<string, string[]>(existingGroups.map((group) => [group.id, []]))
    for (const tabId of tabOrder) {
      const groupId = groupIdByTabId.get(tabId) ?? activeGroupId
      orderByGroup.get(groupId)?.push(tabId)
    }
    return existingGroups
      .map((group) => {
        const nextOrder = orderByGroup.get(group.id) ?? []
        return {
          ...group,
          tabOrder: nextOrder,
          activeTabId:
            activeTopLevelId && nextOrder.includes(activeTopLevelId)
              ? activeTopLevelId
              : group.activeTabId && nextOrder.includes(group.activeTabId)
                ? group.activeTabId
                : (nextOrder[0] ?? null)
        }
      })
      .filter((group) => group.tabOrder.length > 0)
  }

  private buildMaterializedHeadlessParentLayout(
    leafId: string,
    ptyId: string,
    existingLayout: TerminalLayoutSnapshot | undefined,
    split?: { splitFromLeafId: string; direction: 'horizontal' | 'vertical' }
  ): TerminalLayoutSnapshot {
    if (!existingLayout) {
      return {
        root: { type: 'leaf', leafId },
        activeLeafId: leafId,
        expandedLeafId: null,
        ptyIdsByLeafId: { [leafId]: ptyId }
      }
    }
    // Why: a split must insert the new leaf into the live layout tree with the
    // requested direction, or the published snapshot keeps the old single-leaf
    // root and the split renders with a fallback direction ("Split Right" lands
    // as a top/bottom split). Reuse the persisted-split builder for parity.
    if (split) {
      return buildHeadlessTerminalSplitLayout(this.cloneTerminalLayoutSnapshot(existingLayout), {
        leafId,
        ptyId,
        splitFromLeafId: split.splitFromLeafId,
        direction: split.direction
      })
    }
    return {
      ...this.cloneTerminalLayoutSnapshot(existingLayout),
      ptyIdsByLeafId: {
        ...existingLayout.ptyIdsByLeafId,
        [leafId]: ptyId
      }
    }
  }

  private removePersistedHeadlessTerminalTab(worktreeId: string, parentTabId: string): void {
    const session = this.store?.getWorkspaceSession?.()
    if (!session || !this.store?.setWorkspaceSession) {
      return
    }
    const tabs = session.tabsByWorktree[worktreeId] ?? []
    const nextTabs = tabs.filter((tab) => tab.id !== parentTabId)
    const nextTabsByWorktree = {
      ...session.tabsByWorktree,
      [worktreeId]: nextTabs
    }
    const nextLayouts = { ...session.terminalLayoutsByTabId }
    delete nextLayouts[parentTabId]
    const nextActiveTabId =
      session.activeTabIdByWorktree?.[worktreeId] === parentTabId
        ? (nextTabs[0]?.id ?? null)
        : (session.activeTabIdByWorktree?.[worktreeId] ?? null)
    this.store.setWorkspaceSession({
      ...session,
      activeTabId: session.activeTabId === parentTabId ? nextActiveTabId : session.activeTabId,
      tabsByWorktree: nextTabsByWorktree,
      terminalLayoutsByTabId: nextLayouts,
      activeTabIdByWorktree: {
        ...session.activeTabIdByWorktree,
        [worktreeId]: nextActiveTabId
      }
    })
  }

  private persistHeadlessTerminalTabOrder(worktreeId: string, tabOrder: readonly string[]): void {
    const session = this.store?.getWorkspaceSession?.()
    if (!session || !this.store?.setWorkspaceSession) {
      return
    }
    const orderIndexByTabId = new Map(tabOrder.map((tabId, index) => [tabId, index]))
    const tabs = session.tabsByWorktree[worktreeId] ?? []
    const reordered = [...tabs]
      .sort((a, b) => {
        const aIndex = orderIndexByTabId.get(a.id) ?? Number.MAX_SAFE_INTEGER
        const bIndex = orderIndexByTabId.get(b.id) ?? Number.MAX_SAFE_INTEGER
        return aIndex - bIndex || a.sortOrder - b.sortOrder || a.createdAt - b.createdAt
      })
      .map((tab, index) => ({
        ...tab,
        sortOrder: index
      }))
    this.store.setWorkspaceSession({
      ...session,
      tabsByWorktree: {
        ...session.tabsByWorktree,
        [worktreeId]: reordered
      }
    })
  }

  private emitMobileSessionTabsSnapshot(snapshot: RuntimeMobileSessionTabsSnapshot): void {
    if (this.mobileSessionTabListeners.size === 0) {
      return
    }
    const result = this.toMobileSessionTabsResult(snapshot)
    for (const listener of this.mobileSessionTabListeners) {
      listener(result)
    }
  }

  private async refreshMobileSessionPtyRecords(): Promise<void> {
    if (!this.ptyController?.listProcesses) {
      return
    }
    const resolvedWorktrees = await this.listResolvedWorktrees()
    await this.refreshPtyWorktreeRecordsFromController(resolvedWorktrees)
  }

  async activateMobileSessionTab(
    worktreeSelector: string,
    tabId: string,
    leafId?: string,
    opts: { notifyClients?: boolean } = {}
  ): Promise<RuntimeMobileSessionTabsResult> {
    const explicitWorktreeId = this.getValidatedExplicitWorktreeIdSelector(worktreeSelector)
    const worktreeId =
      explicitWorktreeId ?? (await this.resolveWorktreeSelector(worktreeSelector)).id
    this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(worktreeId)
    await this.refreshMobileSessionPtyRecords()
    const snapshot = this.mobileSessionTabsByWorktree.get(worktreeId)
    const directTab = snapshot?.tabs.find((candidate) => candidate.id === tabId)
    const tab = leafId
      ? ((directTab?.type === 'terminal' && directTab.leafId === leafId ? directTab : undefined) ??
        snapshot?.tabs.find(
          (candidate) =>
            candidate.type === 'terminal' &&
            candidate.parentTabId === tabId &&
            candidate.leafId === leafId
        ))
      : (directTab ??
        snapshot?.tabs.find(
          (candidate) => candidate.type === 'terminal' && candidate.parentTabId === tabId
        ) ??
        snapshot?.tabs.find(
          (candidate) => candidate.type === 'browser' && candidate.browserWorkspaceId === tabId
        ))
    if (!tab) {
      throw new Error('tab_not_found')
    }

    if (tab.type === 'terminal') {
      const publicTab = this.toMobileSessionTabsResult(snapshot!).tabs.find(
        (candidate) => candidate.type === 'terminal' && candidate.id === tab.id
      )
      // Why: serve-created tabs can be visible before any renderer has adopted
      // their tab id, so focusing the renderer would silently no-op.
      // Phone-local activation also needs this path for inactive restored tabs:
      // desktop focus is intentionally suppressed, but the PTY still must exist.
      const shouldMaterializePendingTerminal =
        publicTab?.type === 'terminal' &&
        publicTab.status !== 'ready' &&
        (opts.notifyClients === false ||
          !this.notifier?.focusTerminal ||
          this.shouldMaterializeHeadlessMobileSessionTab(snapshot!, tab))
      if (shouldMaterializePendingTerminal) {
        const sessionId = tab.ptyId ?? tab.parentLayout?.ptyIdsByLeafId?.[tab.leafId] ?? undefined
        const targetGroupId = snapshot?.tabGroups?.find((group) =>
          group.tabOrder.includes(tab.parentTabId)
        )?.id
        // Why: a pending agent tab may exist without its startup command ever
        // having been delivered (the create's renderer stalled, #7587), so a
        // bare materialize would put a plain shell under the agent icon.
        // Re-resolve the launch like the create path; providers skip startup
        // commands when attaching to live sessions, so this cannot double-launch.
        let agentStartup: Awaited<
          ReturnType<OrcaRuntimeService['resolveMobileSessionTerminalCommand']>
        > = {}
        if (tab.launchAgent) {
          try {
            const workspace = await this.resolveTerminalWorkspaceLaunchScope(`id:${worktreeId}`)
            agentStartup = await this.resolveMobileSessionTerminalCommand(workspace, {
              agent: tab.launchAgent
            })
          } catch {
            // Why: a disabled or unresolvable agent must not make the tab
            // untappable; fall back to the plain-shell materialize.
          }
        }
        try {
          await this.createHeadlessMobileSessionTerminal(worktreeId, true, undefined, {
            identity: {
              tabId: tab.parentTabId,
              leafId: tab.leafId,
              sessionId
            },
            cwd: tab.startupCwd,
            command: agentStartup.command,
            env: agentStartup.env,
            startupCommandDelivery: agentStartup.startupCommandDelivery,
            launchConfig: agentStartup.launchConfig,
            launchAgent: tab.launchAgent,
            targetGroupId
          })
        } catch (err) {
          if (sessionId && parseAppSshPtyId(sessionId)) {
            // Why: an expired SSH reattach clears durable bindings in the store,
            // but this in-memory headless snapshot can still carry the old id.
            this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(worktreeId, { force: true })
          }
          throw err
        }
        return this.getMobileSessionTabsForWorktree(worktreeId)
      }
      const activeSibling =
        tab.id === tabId || leafId
          ? null
          : snapshot?.tabs.find(
              (candidate): candidate is RuntimeMobileSessionTerminalTab =>
                candidate.type === 'terminal' &&
                candidate.parentTabId === tab.parentTabId &&
                candidate.isActive
            )
      const targetTab = activeSibling ?? tab
      if (opts.notifyClients === false) {
        this.activateMobileSessionTabForRemoteClient(worktreeId, snapshot!, targetTab)
        return this.getMobileSessionTabsForWorktree(worktreeId)
      }
      if (!this.notifier?.focusTerminal) {
        if (
          !targetTab.isActive &&
          this.shouldPersistHeadlessMobileSessionActivation(snapshot!, targetTab)
        ) {
          this.activateHeadlessMobileSessionTerminalTab(worktreeId, snapshot!, targetTab)
        }
        return this.getMobileSessionTabsForWorktree(worktreeId)
      }
      this.notifier?.focusTerminal(targetTab.parentTabId, worktreeId, targetTab.leafId)
    } else if (tab.type === 'browser') {
      if (opts.notifyClients === false) {
        this.activateMobileSessionTabForRemoteClient(worktreeId, snapshot!, tab)
        return this.getMobileSessionTabsForWorktree(worktreeId)
      }
      // Why: browser mobile tabs are renderer-owned unified tabs; focusing the
      // session tab keeps desktop tab order/group state authoritative.
      this.notifier?.focusEditorTab?.(tab.id, worktreeId)
    } else {
      if (opts.notifyClients === false) {
        this.activateMobileSessionTabForRemoteClient(worktreeId, snapshot!, tab)
        return this.getMobileSessionTabsForWorktree(worktreeId)
      }
      this.notifier?.focusEditorTab?.(tab.id, worktreeId)
    }
    return this.getMobileSessionTabsForWorktree(worktreeId)
  }

  private activateMobileSessionTabForRemoteClient(
    worktreeId: string,
    snapshot: RuntimeMobileSessionTabsSnapshot,
    activeTab: RuntimeMobileSessionSnapshotTab
  ): void {
    // Why: phone tab selection should update the mobile snapshot without
    // asking desktop renderers to focus the phone's background worktree.
    const activeTopLevelId = activeTab.type === 'terminal' ? activeTab.parentTabId : activeTab.id
    const tabs = snapshot.tabs.map((tab) => ({
      ...tab,
      isActive: tab.id === activeTab.id
    }))
    const tabGroups = snapshot.tabGroups?.map((group) =>
      group.tabOrder.includes(activeTopLevelId)
        ? { ...group, activeTabId: activeTopLevelId }
        : group
    )
    const activeGroupId =
      tabGroups?.find((group) => group.tabOrder.includes(activeTopLevelId))?.id ??
      snapshot.activeGroupId
    const nextSnapshot: RuntimeMobileSessionTabsSnapshot = {
      ...snapshot,
      publicationEpoch: `mobile-local:${Date.now().toString(36)}`,
      snapshotVersion: snapshot.snapshotVersion + 1,
      activeGroupId,
      activeTabId: activeTab.id,
      activeTabType: activeTab.type,
      ...(tabGroups ? { tabGroups } : {}),
      tabs
    }
    this.mobileSessionTabsByWorktree.set(worktreeId, nextSnapshot)
    this.emitMobileSessionTabsSnapshot(nextSnapshot)
  }

  private shouldMaterializeHeadlessMobileSessionTab(
    snapshot: RuntimeMobileSessionTabsSnapshot,
    tab: RuntimeMobileSessionTerminalTab
  ): boolean {
    return (
      this.isHeadlessMobileSessionPublication(snapshot.publicationEpoch) ||
      this.hasServeOwnedPtyBinding(tab)
    )
  }

  private shouldPersistHeadlessMobileSessionActivation(
    snapshot: RuntimeMobileSessionTabsSnapshot,
    tab: RuntimeMobileSessionTerminalTab
  ): boolean {
    if (snapshot.publicationEpoch.includes(':headless-merge:')) {
      return false
    }
    if (this.graph.authoritativeWindowId !== null && this.graph.graphStatus === 'ready') {
      return false
    }
    return this.shouldMaterializeHeadlessMobileSessionTab(snapshot, tab)
  }

  private activateHeadlessMobileSessionTerminalTab(
    worktreeId: string,
    snapshot: RuntimeMobileSessionTabsSnapshot,
    activeTab: RuntimeMobileSessionTerminalTab
  ): void {
    const tabs = snapshot.tabs.map((candidate) => ({
      ...candidate,
      isActive: candidate.id === activeTab.id
    }))
    const nextSnapshot: RuntimeMobileSessionTabsSnapshot = {
      ...snapshot,
      publicationEpoch: `headless:${Date.now().toString(36)}`,
      snapshotVersion: snapshot.snapshotVersion + 1,
      activeTabId: activeTab.id,
      activeTabType: 'terminal',
      tabGroups: this.buildHeadlessMobileSessionTabGroups(
        worktreeId,
        tabs,
        activeTab,
        snapshot.tabGroups
      ),
      tabs
    }
    this.persistHeadlessTerminalActiveLeaf(worktreeId, activeTab)
    this.mobileSessionTabsByWorktree.set(worktreeId, nextSnapshot)
    this.emitMobileSessionTabsSnapshot(nextSnapshot)
  }

  // Why: a headless split only updated the LIVE session snapshot, never the
  // persisted workspace session layout. So a later snapshot rebuild (e.g. on the
  // next terminal create) re-derived from the stale single-leaf persisted layout
  // and collapsed the split. Persist the new split leaf into the workspace
  // session's terminalLayoutsByTabId so the split survives rebuilds.
  private persistHeadlessTerminalSplit(args: {
    tabId: string
    leafId: string
    ptyId: string
    splitFromLeafId: string
    direction: 'horizontal' | 'vertical'
  }): void {
    const session = this.store?.getWorkspaceSession?.()
    if (!session || !this.store?.setWorkspaceSession) {
      return
    }
    const existing = session.terminalLayoutsByTabId?.[args.tabId]
    const nextLayout = buildHeadlessTerminalSplitLayout(
      existing ? this.cloneTerminalLayoutSnapshot(existing) : undefined,
      args
    )
    this.store.setWorkspaceSession({
      ...session,
      terminalLayoutsByTabId: {
        ...session.terminalLayoutsByTabId,
        [args.tabId]: nextLayout
      }
    })
  }

  private persistHeadlessTerminalActiveLeaf(
    worktreeId: string,
    tab: RuntimeMobileSessionTerminalTab
  ): void {
    const session = this.store?.getWorkspaceSession?.()
    if (!session || !this.store?.setWorkspaceSession) {
      return
    }
    const existingLayout = session.terminalLayoutsByTabId?.[tab.parentTabId]
    const nextLayouts = existingLayout
      ? {
          ...session.terminalLayoutsByTabId,
          [tab.parentTabId]: {
            ...this.cloneTerminalLayoutSnapshot(existingLayout),
            activeLeafId: tab.leafId
          }
        }
      : session.terminalLayoutsByTabId
    this.store.setWorkspaceSession({
      ...session,
      activeTabId: tab.parentTabId,
      activeTabIdByWorktree: {
        ...session.activeTabIdByWorktree,
        [worktreeId]: tab.parentTabId
      },
      terminalLayoutsByTabId: nextLayouts
    })
  }

  async closeMobileSessionTab(worktreeSelector: string, tabId: string): Promise<{ closed: true }> {
    const explicitWorktreeId = this.getValidatedExplicitWorktreeIdSelector(worktreeSelector)
    const worktreeId =
      explicitWorktreeId ?? (await this.resolveWorktreeSelector(worktreeSelector)).id
    this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(worktreeId)
    await this.refreshMobileSessionPtyRecords()
    const snapshot = this.mobileSessionTabsByWorktree.get(worktreeId)
    const tab =
      snapshot?.tabs.find((candidate) => candidate.id === tabId) ??
      snapshot?.tabs.find(
        (candidate) => candidate.type === 'terminal' && candidate.parentTabId === tabId
      ) ??
      snapshot?.tabs.find(
        (candidate) => candidate.type === 'browser' && candidate.browserWorkspaceId === tabId
      )
    if (!tab) {
      throw new Error('tab_not_found')
    }
    if (tab.type === 'terminal') {
      if (!this.notifier?.closeTerminal) {
        this.closeHeadlessMobileTerminalTab(worktreeId, snapshot!, tab)
        return { closed: true }
      }
      // Why: a runtime-owned headless tab whose whole parent is being closed must
      // be torn down authoritatively even with a renderer attached — kill the
      // PTY, drop the persisted binding, and prune+emit — or syncMobileSessionTabs
      // keeps republishing the "closed" tab with a live PTY. Best-effort notify the
      // renderer too so any adopted pane closes (no dead pane). A single split leaf
      // (exact id, multi-leaf parent) keeps the per-leaf path so siblings survive.
      const parentLeafCount = snapshot!.tabs.filter(
        (candidate) => candidate.type === 'terminal' && candidate.parentTabId === tab.parentTabId
      ).length
      const closingWholeParent = tab.id !== tabId || parentLeafCount <= 1
      if (closingWholeParent && this.isRuntimeOwnedHeadlessMobileTab(worktreeId, tab)) {
        this.closeHeadlessMobileTerminalTab(worktreeId, snapshot!, tab)
        this.notifier?.closeTerminal(tab.parentTabId)
        return { closed: true }
      }
      if (tab.id === tabId) {
        const pty = this.findPtyForMobileTerminalTab(worktreeId, tab)
        if (pty) {
          this.ptyController?.kill(pty.ptyId)
        } else {
          this.notifier?.closeTerminal(tab.parentTabId)
        }
      } else {
        // Why: paired web tab bars represent a split terminal with one local
        // parent tab id. Closing that parent should close the desktop tab, not
        // just whichever leaf happened to be first in the session snapshot.
        this.notifier?.closeTerminal(tab.parentTabId)
      }
    } else if (tab.type === 'browser' && this.offscreenBrowserBackend) {
      // Why: headless browser tabs are offscreen WebContents with no renderer to
      // route closeSessionTab to. Close the page directly and drop it from the
      // snapshot so paired clients stop showing it.
      await this.closeHeadlessMobileBrowserTab(worktreeId, snapshot!, tab)
    } else {
      this.notifier?.closeSessionTab?.(tab.id, worktreeId)
    }
    return { closed: true }
  }

  private async closeHeadlessMobileBrowserTab(
    worktreeId: string,
    snapshot: RuntimeMobileSessionTabsSnapshot,
    tab: RuntimeMobileSessionBrowserTab
  ): Promise<void> {
    if (tab.browserPageId) {
      await this.offscreenBrowserBackend?.closeTab(tab.browserPageId).catch(() => {})
    }
    const nextTabs = snapshot.tabs.filter((candidate) => candidate.id !== tab.id)
    const active = nextTabs.find((candidate) => candidate.isActive) ?? nextTabs[0] ?? null
    const nextSnapshot: RuntimeMobileSessionTabsSnapshot = {
      ...snapshot,
      publicationEpoch: `headless:${Date.now().toString(36)}`,
      snapshotVersion: snapshot.snapshotVersion + 1,
      activeTabId: active?.id ?? null,
      activeTabType: active?.type ?? null,
      tabGroups: (snapshot.tabGroups ?? []).map((group) => ({
        ...group,
        tabOrder: group.tabOrder.filter((id) => id !== tab.id),
        activeTabId: group.activeTabId === tab.id ? null : group.activeTabId
      })),
      tabs: nextTabs
    }
    this.mobileSessionTabsByWorktree.set(worktreeId, nextSnapshot)
    this.emitMobileSessionTabsSnapshot(nextSnapshot)
  }

  private markHeadlessBrowserSessionTabActive(
    worktreeId: string | undefined,
    browserPageId: string,
    targetGroupId?: string
  ): void {
    if (!this.offscreenBrowserBackend || !worktreeId) {
      return
    }
    // Hydrate first so the freshly created browser tab is present in the snapshot.
    this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(worktreeId)
    const snapshot = this.mobileSessionTabsByWorktree.get(worktreeId)
    const tab = snapshot?.tabs.find(
      (candidate): candidate is RuntimeMobileSessionBrowserTab =>
        candidate.type === 'browser' && candidate.browserPageId === browserPageId
    )
    if (!snapshot || !tab) {
      return
    }
    const groups = snapshot.tabGroups ?? []
    const hasTargetGroup =
      targetGroupId !== undefined && groups.some((group) => group.id === targetGroupId)
    // Why: move the new browser into the group whose "+" was clicked, removing it
    // from wherever the rebuild placed it. Only the TARGET group's activeTabId
    // (and the global active) change — every other group's active tab is left
    // intact, so creating in the right group never resets the left group's tab.
    const nextGroups = hasTargetGroup
      ? groups.map((group) => {
          const withoutTab = group.tabOrder.filter((id) => id !== tab.id)
          if (group.id === targetGroupId) {
            return { ...group, tabOrder: [...withoutTab, tab.id], activeTabId: tab.id }
          }
          return withoutTab.length === group.tabOrder.length
            ? group
            : { ...group, tabOrder: withoutTab }
        })
      : groups.map((group) =>
          group.tabOrder.includes(tab.id) ? { ...group, activeTabId: tab.id } : group
        )
    const nextSnapshot: RuntimeMobileSessionTabsSnapshot = {
      ...snapshot,
      publicationEpoch: `headless:${Date.now().toString(36)}`,
      snapshotVersion: snapshot.snapshotVersion + 1,
      ...(hasTargetGroup ? { activeGroupId: targetGroupId } : {}),
      activeTabId: tab.id,
      activeTabType: 'browser',
      tabs: snapshot.tabs.map((candidate) => ({
        ...candidate,
        isActive: candidate.id === tab.id
      })),
      tabGroups: nextGroups
    }
    this.mobileSessionTabsByWorktree.set(worktreeId, nextSnapshot)
    // Why: browser group membership is otherwise live-only; persist it so a
    // later rebuild keeps the browser in its group instead of coalescing left.
    if (hasTargetGroup && nextSnapshot.tabGroupLayout) {
      this.persistHeadlessTabGroups(worktreeId, nextGroups, nextSnapshot.tabGroupLayout)
    }
    this.emitMobileSessionTabsSnapshot(nextSnapshot)
  }

  private closeHeadlessMobileTerminalTab(
    worktreeId: string,
    snapshot: RuntimeMobileSessionTabsSnapshot,
    tab: RuntimeMobileSessionTerminalTab
  ): void {
    const closedParentTabId = tab.parentTabId
    const nextTabs = snapshot.tabs.filter((candidate) => {
      if (candidate.type !== 'terminal' || candidate.parentTabId !== closedParentTabId) {
        return true
      }
      const pty = this.findPtyForMobileTerminalTab(worktreeId, candidate)
      if (pty?.connected) {
        this.ptyController?.kill(pty.ptyId)
      } else {
        const persistedSshPtyId = this.getPersistedSshPtyIdForMobileTerminalTab(candidate)
        if (persistedSshPtyId) {
          // Why: close is an explicit deletion. Hydrated SSH PTYs can be known
          // only by durable id before reconnect repopulates pane metadata.
          this.ptyController?.kill(persistedSshPtyId)
        }
      }
      return false
    })
    this.removePersistedHeadlessTerminalTab(worktreeId, closedParentTabId)
    const active = nextTabs.find((candidate) => candidate.isActive) ?? nextTabs[0] ?? null
    const nextSnapshot: RuntimeMobileSessionTabsSnapshot = {
      ...snapshot,
      publicationEpoch: `headless:${Date.now().toString(36)}`,
      snapshotVersion: snapshot.snapshotVersion + 1,
      activeTabId: active?.id ?? null,
      activeTabType: active?.type ?? null,
      tabGroups: this.buildHeadlessMobileSessionTabGroups(
        worktreeId,
        nextTabs,
        active,
        snapshot.tabGroups
      ),
      tabs: nextTabs
    }
    this.mobileSessionTabsByWorktree.set(worktreeId, nextSnapshot)
    this.emitMobileSessionTabsSnapshot(nextSnapshot)
  }

  async moveMobileSessionTab(
    worktreeSelector: string,
    move: RuntimeMobileSessionTabMove
  ): Promise<RuntimeMobileSessionTabMoveResult> {
    const explicitWorktreeId = this.getValidatedExplicitWorktreeIdSelector(worktreeSelector)
    const worktreeId =
      explicitWorktreeId ?? (await this.resolveWorktreeSelector(worktreeSelector)).id
    this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(worktreeId)
    const snapshot = this.mobileSessionTabsByWorktree.get(worktreeId)
    if (!snapshot) {
      throw new Error('tab_not_found')
    }
    if (!this.notifier?.moveSessionTab) {
      return this.moveHeadlessMobileSessionTab(worktreeId, snapshot, move)
    }
    const hostTabId = this.resolveMobileSessionHostTabId(snapshot, move.tabId)
    if (!hostTabId) {
      throw new Error('tab_not_found')
    }
    const publicSnapshot = this.toMobileSessionTabsResult(snapshot)
    const targetGroup = publicSnapshot.tabGroups?.find((group) => group.id === move.targetGroupId)
    if (!targetGroup) {
      throw new Error('target_group_not_found')
    }

    // Why: web clients address terminal surfaces as tab::leaf, while desktop
    // tab grouping is owned by the outer terminal tab id.
    if (move.kind === 'reorder') {
      const tabOrder = this.normalizeMobileSessionTabOrder(snapshot, targetGroup, move.tabOrder)
      if (!tabOrder.includes(hostTabId)) {
        throw new Error('invalid_tab_order')
      }
      this.notifier.moveSessionTab(worktreeId, {
        ...move,
        tabId: hostTabId,
        tabOrder
      })
      return { moved: true }
    }
    this.notifier.moveSessionTab(worktreeId, {
      ...move,
      tabId: hostTabId
    })
    return { moved: true }
  }

  // Why: pane geometry inside a tab (split ratios, expanded pane, pane titles)
  // is host-authoritative for remote-server tabs but had no push path, so a
  // client divider-drag / expand / pane-rename reverted on the next snapshot.
  // Persist the structural fields onto the tab's layout, keeping host-owned
  // pty bindings and active leaf.
  async updateMobileSessionPaneLayout(
    worktreeSelector: string,
    args: {
      tabId: string
      root: TerminalPaneLayoutNode | null
      expandedLeafId: string | null
      titlesByLeafId?: Record<string, string>
    }
  ): Promise<{ updated: true }> {
    const explicitWorktreeId = this.getValidatedExplicitWorktreeIdSelector(worktreeSelector)
    const worktreeId =
      explicitWorktreeId ?? (await this.resolveWorktreeSelector(worktreeSelector)).id
    // Why: when a renderer is authoritative (desktop host reached via shared
    // control), it owns pane geometry and republishes it — a headless write here
    // would be overwritten and could fight the renderer. Persist only headlessly.
    if (this.getAvailableAuthoritativeWindow()) {
      return { updated: true }
    }
    // Why: resolve to the host tab id (older/raw-id clients) so the persisted
    // layout entry matches, matching setMobileSessionTabProps.
    const snapshot = this.mobileSessionTabsByWorktree.get(worktreeId)
    const hostTabId = snapshot
      ? (this.resolveMobileSessionHostTabId(snapshot, args.tabId) ?? args.tabId)
      : args.tabId
    const resolvedArgs = { ...args, tabId: hostTabId }
    this.persistHeadlessTerminalPaneLayout(resolvedArgs)
    this.applyHeadlessTerminalPaneLayoutToSnapshot(worktreeId, resolvedArgs)
    return { updated: true }
  }

  // Why: tab color/pin are host-authoritative for remote-server tabs but had no
  // push path, so pinning or coloring a tab reverted on the next snapshot and
  // was never persisted. Persist to the workspace session + live snapshot.
  async setMobileSessionTabProps(
    worktreeSelector: string,
    args: {
      tabId: string
      color?: string | null
      isPinned?: boolean
      viewMode?: 'terminal' | 'chat'
    }
  ): Promise<{ updated: true }> {
    const explicitWorktreeId = this.getValidatedExplicitWorktreeIdSelector(worktreeSelector)
    const worktreeId =
      explicitWorktreeId ?? (await this.resolveWorktreeSelector(worktreeSelector)).id
    // Why: a renderer-authoritative host owns + republishes tab props, so a
    // headless write would be overwritten. Persist only when headless.
    if (this.getAvailableAuthoritativeWindow()) {
      return { updated: true }
    }
    const snapshot = this.mobileSessionTabsByWorktree.get(worktreeId)
    const hostTabId = snapshot
      ? (this.resolveMobileSessionHostTabId(snapshot, args.tabId) ?? args.tabId)
      : args.tabId
    this.persistHeadlessSessionTabProps(worktreeId, hostTabId, args)
    this.applyHeadlessSessionTabPropsToSnapshot(worktreeId, hostTabId, args)
    return { updated: true }
  }

  private persistHeadlessSessionTabProps(
    worktreeId: string,
    tabId: string,
    props: { color?: string | null; isPinned?: boolean; viewMode?: 'terminal' | 'chat' }
  ): void {
    const session = this.store?.getWorkspaceSession?.()
    if (!session || !this.store?.setWorkspaceSession) {
      return
    }
    const tabs = session.tabsByWorktree[worktreeId]
    const nextSession: WorkspaceSessionState = { ...session }
    let changed = false
    if (tabs?.some((tab) => tab.id === tabId)) {
      changed = true
      nextSession.tabsByWorktree = {
        ...session.tabsByWorktree,
        [worktreeId]: tabs.map((tab) =>
          tab.id === tabId
            ? {
                ...tab,
                ...(props.color !== undefined ? { color: props.color } : {}),
                ...(props.isPinned !== undefined ? { isPinned: props.isPinned } : {}),
                ...(props.viewMode !== undefined ? { viewMode: props.viewMode } : {})
              }
            : tab
        )
      }
    }

    const unifiedTabs = session.unifiedTabs?.[worktreeId]
    if (unifiedTabs?.some((tab) => tab.id === tabId || tab.entityId === tabId)) {
      changed = true
      nextSession.unifiedTabs = {
        ...session.unifiedTabs,
        [worktreeId]: unifiedTabs.map((tab) =>
          tab.id === tabId || tab.entityId === tabId
            ? {
                ...tab,
                ...(props.color !== undefined ? { color: props.color } : {}),
                ...(props.isPinned !== undefined ? { isPinned: props.isPinned } : {})
              }
            : tab
        )
      }
    }

    if (!changed) {
      return
    }
    this.store.setWorkspaceSession(nextSession)
  }

  private applyHeadlessSessionTabPropsToSnapshot(
    worktreeId: string,
    tabId: string,
    props: { color?: string | null; isPinned?: boolean; viewMode?: 'terminal' | 'chat' }
  ): void {
    const snapshot = this.mobileSessionTabsByWorktree.get(worktreeId)
    if (!snapshot) {
      return
    }
    let changed = false
    const tabs = snapshot.tabs.map((tab) => {
      if (this.getMobileSessionTopLevelTabId(tab) !== tabId) {
        return tab
      }
      changed = true
      return {
        ...tab,
        ...(props.color !== undefined ? { color: props.color } : {}),
        ...(props.isPinned !== undefined ? { isPinned: props.isPinned } : {}),
        ...(props.viewMode !== undefined ? { viewMode: props.viewMode } : {})
      }
    })
    if (!changed) {
      return
    }
    const nextSnapshot: RuntimeMobileSessionTabsSnapshot = {
      ...snapshot,
      publicationEpoch: `headless:${Date.now().toString(36)}`,
      snapshotVersion: snapshot.snapshotVersion + 1,
      tabs
    }
    this.mobileSessionTabsByWorktree.set(worktreeId, nextSnapshot)
    this.emitMobileSessionTabsSnapshot(nextSnapshot)
  }

  private getMobileSessionTopLevelTabId(tab: RuntimeMobileSessionSnapshotTab): string {
    return tab.type === 'terminal' ? tab.parentTabId : tab.id
  }

  // Merge the client's pane structure into the persisted tab layout. PTY
  // bindings and active leaf stay host-owned; only ratios/expand/titles change.
  // terminalLayoutsByTabId is keyed by tab id (worktree-independent).
  private persistHeadlessTerminalPaneLayout(args: {
    tabId: string
    root: TerminalPaneLayoutNode | null
    expandedLeafId: string | null
    titlesByLeafId?: Record<string, string>
  }): void {
    const session = this.store?.getWorkspaceSession?.()
    if (!session || !this.store?.setWorkspaceSession) {
      return
    }
    const existing = session.terminalLayoutsByTabId?.[args.tabId]
    if (!existing) {
      return
    }
    this.store.setWorkspaceSession({
      ...session,
      terminalLayoutsByTabId: {
        ...session.terminalLayoutsByTabId,
        [args.tabId]: {
          ...this.cloneTerminalLayoutSnapshot(existing),
          root: args.root ?? existing.root,
          expandedLeafId: args.expandedLeafId,
          ...(args.titlesByLeafId ? { titlesByLeafId: args.titlesByLeafId } : {})
        }
      }
    })
  }

  private applyHeadlessTerminalPaneLayoutToSnapshot(
    worktreeId: string,
    args: {
      tabId: string
      root: TerminalPaneLayoutNode | null
      expandedLeafId: string | null
      titlesByLeafId?: Record<string, string>
    }
  ): void {
    const snapshot = this.mobileSessionTabsByWorktree.get(worktreeId)
    if (!snapshot) {
      return
    }
    let changed = false
    const tabs = snapshot.tabs.map((tab) => {
      if (tab.type !== 'terminal' || tab.parentTabId !== args.tabId || !tab.parentLayout) {
        return tab
      }
      changed = true
      return {
        ...tab,
        parentLayout: {
          ...tab.parentLayout,
          root: args.root ?? tab.parentLayout.root,
          expandedLeafId: args.expandedLeafId,
          ...(args.titlesByLeafId ? { titlesByLeafId: args.titlesByLeafId } : {})
        }
      }
    })
    if (!changed) {
      return
    }
    const nextSnapshot: RuntimeMobileSessionTabsSnapshot = {
      ...snapshot,
      publicationEpoch: `headless:${Date.now().toString(36)}`,
      snapshotVersion: snapshot.snapshotVersion + 1,
      tabs
    }
    this.mobileSessionTabsByWorktree.set(worktreeId, nextSnapshot)
    this.emitMobileSessionTabsSnapshot(nextSnapshot)
  }

  private moveHeadlessMobileSessionTab(
    worktreeId: string,
    snapshot: RuntimeMobileSessionTabsSnapshot,
    move: RuntimeMobileSessionTabMove
  ): RuntimeMobileSessionTabMoveResult {
    if (move.kind === 'split') {
      return this.splitHeadlessMobileSessionTabGroup(worktreeId, snapshot, move)
    }
    if (move.kind === 'move-to-group') {
      return this.moveHeadlessMobileSessionTabToGroup(worktreeId, snapshot, move)
    }
    if (move.kind !== 'reorder') {
      throw new Error('renderer_unavailable')
    }
    const hostTabId = this.resolveMobileSessionHostTabId(snapshot, move.tabId)
    if (!hostTabId) {
      throw new Error('tab_not_found')
    }
    const publicSnapshot = this.toMobileSessionTabsResult(snapshot)
    const targetGroup = publicSnapshot.tabGroups?.find((group) => group.id === move.targetGroupId)
    if (!targetGroup) {
      throw new Error('target_group_not_found')
    }
    const tabOrder = this.normalizeMobileSessionTabOrder(snapshot, targetGroup, move.tabOrder)
    const orderIndexByParentTabId = new Map(tabOrder.map((tabId, index) => [tabId, index]))
    const nextTabs = [...snapshot.tabs].sort((a, b) => {
      const aParent = a.type === 'terminal' ? a.parentTabId : a.id
      const bParent = b.type === 'terminal' ? b.parentTabId : b.id
      const aIndex = orderIndexByParentTabId.get(aParent) ?? Number.MAX_SAFE_INTEGER
      const bIndex = orderIndexByParentTabId.get(bParent) ?? Number.MAX_SAFE_INTEGER
      return aIndex - bIndex
    })
    const active = nextTabs.find((candidate) => candidate.isActive) ?? nextTabs[0] ?? null
    const reorderedTargetActiveTabId =
      active?.type === 'terminal' ? active.parentTabId : active ? active.id : (tabOrder[0] ?? null)
    // Why: reorder only changes ONE group's order. Preserve every other group so
    // a multi-group split isn't deleted by re-sorting tabs in one of its groups.
    const existingGroups = snapshot.tabGroups ?? []
    const nextGroups = existingGroups.some((group) => group.id === targetGroup.id)
      ? existingGroups.map((group) =>
          group.id === targetGroup.id
            ? { ...group, tabOrder, activeTabId: reorderedTargetActiveTabId }
            : group
        )
      : [{ ...targetGroup, tabOrder, activeTabId: reorderedTargetActiveTabId }]
    const nextSnapshot: RuntimeMobileSessionTabsSnapshot = {
      ...snapshot,
      publicationEpoch: `headless:${Date.now().toString(36)}`,
      snapshotVersion: snapshot.snapshotVersion + 1,
      activeTabId: active?.id ?? null,
      activeTabType: active?.type ?? null,
      tabGroups: nextGroups,
      tabs: nextTabs
    }
    this.persistHeadlessTerminalTabOrder(worktreeId, tabOrder)
    if (nextGroups.length > 1 && snapshot.tabGroupLayout) {
      this.persistHeadlessTabGroups(worktreeId, nextGroups, snapshot.tabGroupLayout)
    }
    this.mobileSessionTabsByWorktree.set(worktreeId, nextSnapshot)
    this.emitMobileSessionTabsSnapshot(nextSnapshot)
    return { moved: true }
  }

  // Why: a drag-to-split-group used to be a client-only change the headless host
  // never modeled, so the next snapshot coalesced every tab back into one group.
  // Model + persist the multi-group layout so the split survives rebuilds.
  private splitHeadlessMobileSessionTabGroup(
    worktreeId: string,
    snapshot: RuntimeMobileSessionTabsSnapshot,
    move: Extract<RuntimeMobileSessionTabMove, { kind: 'split' }>
  ): RuntimeMobileSessionTabMoveResult {
    const hostTabId = this.resolveMobileSessionHostTabId(snapshot, move.tabId)
    if (!hostTabId) {
      throw new Error('tab_not_found')
    }
    const split = buildHeadlessTabGroupSplit({
      groups: snapshot.tabGroups ?? [],
      layout: snapshot.tabGroupLayout,
      tabId: hostTabId,
      targetGroupId: move.targetGroupId,
      splitDirection: move.splitDirection,
      newGroupId: randomUUID()
    })
    if (!split) {
      // Renderer treats an unsplittable drop (e.g. last tab onto its own group)
      // as a no-op; mirror that instead of churning the snapshot.
      return { moved: true }
    }
    const nextSnapshot: RuntimeMobileSessionTabsSnapshot = {
      ...snapshot,
      publicationEpoch: `headless:${Date.now().toString(36)}`,
      snapshotVersion: snapshot.snapshotVersion + 1,
      activeGroupId: split.newGroupId,
      tabGroups: split.groups,
      tabGroupLayout: split.layout
    }
    this.persistHeadlessTabGroups(worktreeId, split.groups, split.layout)
    this.mobileSessionTabsByWorktree.set(worktreeId, nextSnapshot)
    this.emitMobileSessionTabsSnapshot(nextSnapshot)
    return { moved: true }
  }

  // Move a tab into an existing group on a headless serve (non-split drop).
  private moveHeadlessMobileSessionTabToGroup(
    worktreeId: string,
    snapshot: RuntimeMobileSessionTabsSnapshot,
    move: Extract<RuntimeMobileSessionTabMove, { kind: 'move-to-group' }>
  ): RuntimeMobileSessionTabMoveResult {
    const hostTabId = this.resolveMobileSessionHostTabId(snapshot, move.tabId)
    if (!hostTabId) {
      throw new Error('tab_not_found')
    }
    const moved = buildHeadlessTabGroupMove({
      groups: snapshot.tabGroups ?? [],
      layout: snapshot.tabGroupLayout,
      tabId: hostTabId,
      targetGroupId: move.targetGroupId,
      index: move.index
    })
    if (!moved) {
      // Same-group / missing-target drop is a renderer no-op; mirror that.
      return { moved: true }
    }
    const layout = moved.layout ?? { type: 'leaf' as const, groupId: move.targetGroupId }
    const nextSnapshot: RuntimeMobileSessionTabsSnapshot = {
      ...snapshot,
      publicationEpoch: `headless:${Date.now().toString(36)}`,
      snapshotVersion: snapshot.snapshotVersion + 1,
      activeGroupId: move.targetGroupId,
      tabGroups: moved.groups,
      tabGroupLayout: layout
    }
    this.persistHeadlessTabGroups(worktreeId, moved.groups, layout)
    this.mobileSessionTabsByWorktree.set(worktreeId, nextSnapshot)
    this.emitMobileSessionTabsSnapshot(nextSnapshot)
    return { moved: true }
  }

  // Persist the headless tab-GROUP layout so snapshot rebuilds keep the split.
  private persistHeadlessTabGroups(
    worktreeId: string,
    groups: readonly RuntimeMobileSessionTabGroup[],
    layout: TabGroupLayoutNode
  ): void {
    const session = this.store?.getWorkspaceSession?.()
    if (!session || !this.store?.setWorkspaceSession) {
      return
    }
    this.store.setWorkspaceSession({
      ...session,
      tabGroups: {
        ...session.tabGroups,
        [worktreeId]: groups.map((group) => ({
          id: group.id,
          worktreeId,
          activeTabId: group.activeTabId,
          tabOrder: [...group.tabOrder],
          ...(group.recentTabIds ? { recentTabIds: [...group.recentTabIds] } : {})
        }))
      },
      tabGroupLayouts: {
        ...session.tabGroupLayouts,
        [worktreeId]: layout
      }
    })
  }

  // Persist a manual terminal rename so a headless rebuild keeps the title
  // instead of reverting to the generated/default one.
  private persistHeadlessTerminalTitle(
    worktreeId: string,
    tabId: string,
    title: string | null
  ): void {
    const session = this.store?.getWorkspaceSession?.()
    if (!session || !this.store?.setWorkspaceSession) {
      return
    }
    const tabs = session.tabsByWorktree[worktreeId]
    if (!tabs?.some((tab) => tab.id === tabId)) {
      return
    }
    this.store.setWorkspaceSession({
      ...session,
      tabsByWorktree: {
        ...session.tabsByWorktree,
        [worktreeId]: tabs.map((tab) => (tab.id === tabId ? { ...tab, customTitle: title } : tab))
      }
    })
  }

  private normalizeMobileSessionTabOrder(
    snapshot: RuntimeMobileSessionTabsSnapshot | undefined,
    targetGroup: RuntimeMobileSessionTabGroup,
    tabOrder: readonly string[]
  ): string[] {
    const normalized: string[] = []
    const seen = new Set<string>()
    for (const tabId of tabOrder) {
      const hostTabId = this.resolveMobileSessionHostTabId(snapshot, tabId)
      if (!hostTabId) {
        throw new Error('invalid_tab_order')
      }
      if (seen.has(hostTabId)) {
        throw new Error('duplicate_tab_order')
      }
      seen.add(hostTabId)
      normalized.push(hostTabId)
    }

    const returnedIds = this.collectPublicMobileSessionTabIds(snapshot)
    const expected = targetGroup.tabOrder
      .map((tabId) => this.resolveMobileSessionHostTabId(snapshot, tabId) ?? tabId)
      // Why: clients reorder the sanitized session.tabs.list model; raw groups
      // can still contain stale browser ids hidden from paired web clients.
      .filter((tabId) => returnedIds.has(tabId))
    // Why: reorder is a pure permutation of one existing group. Missing or
    // extra ids would let a paired web client silently move/lose host tabs.
    if (normalized.length !== expected.length || expected.some((tabId) => !seen.has(tabId))) {
      throw new Error('invalid_tab_order')
    }
    return normalized
  }

  private collectPublicMobileSessionTabIds(
    snapshot: RuntimeMobileSessionTabsSnapshot | undefined
  ): Set<string> {
    const ids = new Set<string>()
    if (!snapshot) {
      return ids
    }
    const liveBrowserTabsByPageId = this.getLiveBrowserTabsByPageId(snapshot.worktree)
    for (const tab of snapshot.tabs) {
      if (tab.type === 'browser') {
        const liveTab = tab.browserPageId
          ? liveBrowserTabsByPageId.get(tab.browserPageId)
          : undefined
        if (!liveTab) {
          continue
        }
        ids.add(tab.id)
        ids.add(tab.browserWorkspaceId)
        continue
      }
      ids.add(tab.id)
      if (tab.type === 'terminal') {
        ids.add(tab.parentTabId)
      }
    }
    return ids
  }

  private resolveMobileSessionHostTabId(
    snapshot: RuntimeMobileSessionTabsSnapshot | undefined,
    tabId: string
  ): string | null {
    const tab =
      snapshot?.tabs.find((candidate) => candidate.id === tabId) ??
      snapshot?.tabs.find(
        (candidate) => candidate.type === 'terminal' && candidate.parentTabId === tabId
      ) ??
      snapshot?.tabs.find(
        (candidate) => candidate.type === 'browser' && candidate.browserWorkspaceId === tabId
      )
    if (!tab) {
      return null
    }
    return tab.type === 'terminal' ? tab.parentTabId : tab.id
  }

  async readMobileMarkdownTab(
    worktreeSelector: string,
    tabId: string
  ): Promise<RuntimeMarkdownReadTabResult> {
    const worktreeId = await this.resolveMobileMarkdownWorktreeId(worktreeSelector, tabId)
    if (!this.notifier?.readMobileMarkdownTab) {
      throw new Error('renderer_unavailable')
    }
    return await this.notifier.readMobileMarkdownTab(worktreeId, tabId)
  }

  async saveMobileMarkdownTab(
    worktreeSelector: string,
    tabId: string,
    baseVersion: string,
    content: string
  ): Promise<RuntimeMarkdownSaveTabResult> {
    const worktreeId = await this.resolveMobileMarkdownWorktreeId(worktreeSelector, tabId)
    if (!this.notifier?.saveMobileMarkdownTab) {
      throw new Error('renderer_unavailable')
    }
    return await this.notifier.saveMobileMarkdownTab(worktreeId, tabId, baseVersion, content)
  }

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

  private async resolveRuntimeGitTarget(worktreeSelector: string): Promise<{
    worktree: ResolvedWorktree
    repo?: Repo
    connectionId?: string
    localGitOptions?: { wslDistro?: string }
  }> {
    const store = this.requireStore()
    const worktree = await this.resolveWorktreeSelector(worktreeSelector)
    const repo = store.getRepo(worktree.repoId)
    // Why: getRepoProviderConnectionKey (not repo.connectionId directly) so a
    // Dev-Server-bound repo (devServerId, no SSH connectionId) also resolves
    // through the provider registries — see dev-server-provider-lifecycle.ts.
    const connectionId = (repo ? getRepoProviderConnectionKey(repo) : null) ?? undefined
    const localGitOptions =
      repo && !connectionId ? getLocalProjectWorktreeGitOptions(store, repo) : {}
    return { worktree, repo, connectionId, localGitOptions }
  }

  private async resolveRuntimeFileTarget(worktreeSelector: string): Promise<{
    worktree: ResolvedWorktree
    connectionId?: string
  }> {
    const folderScope = await this.resolveFolderWorkspaceLaunchScope(worktreeSelector)
    if (folderScope?.folderWorkspace) {
      return {
        worktree: this.folderWorkspaceToResolvedWorktree(folderScope.folderWorkspace),
        connectionId: folderScope.connectionId ?? undefined
      }
    }

    const store = this.requireStore()
    const worktree = await this.resolveWorktreeSelector(worktreeSelector)
    const repo = store.getRepo(worktree.repoId)
    return {
      worktree,
      connectionId: (repo ? getRepoProviderConnectionKey(repo) : null) ?? undefined
    }
  }

  onMobileSessionTabsChanged(
    listener: (snapshot: RuntimeMobileSessionTabsResult) => void
  ): () => void {
    this.mobileSessionTabListeners.add(listener)
    return () => {
      // Why: flush pending coalesced notifies before dropping this listener so a
      // subscriber closing mid-window still receives the latest settled state.
      this.mobileSessionTabsNotifyCoalescer.flushAll()
      this.mobileSessionTabListeners.delete(listener)
    }
  }

  // Why: terminal handles are normally created lazily when first referenced via
  // RPC, but agents need their own handle at spawn time (via ORCA_TERMINAL_HANDLE
  // env var) so they can self-identify in orchestration messages without an
  // extra RPC round-trip. Pre-allocating by ptyId lets issueHandle reuse it.
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
      this.terminalSpawnCommandsByPtyId.set(ptyId, trimmed)
    }
  }

  /**
   * Handles incoming data from a PTY process, running agent detection,
   * updating terminal tail buffers, and triggering foreground agent refreshes.
   */
  onPtyData(ptyId: string, data: string, at: number, sequenceChars = data.length): number {
    const outputSequence = (this.ptyOutputSequenceById.get(ptyId) ?? 0) + sequenceChars
    this.ptyOutputSequenceById.set(ptyId, outputSequence)
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
    const previousTitleScanTail = this.oscTitleScanTailByPtyId.get(ptyId)
    const titleInput = previousTitleScanTail
      ? `${previousTitleScanTail}${agentStatusChunk.cleanData}`
      : agentStatusChunk.cleanData
    const nextTitleScanTail = extractOscTitleScanTail(titleInput)
    if (nextTitleScanTail.length > 0) {
      this.oscTitleScanTailByPtyId.set(ptyId, nextTitleScanTail)
    } else {
      this.oscTitleScanTailByPtyId.delete(ptyId)
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

  private scheduleWaitBlockedCheck(ptyId: string, appendedText: string, at: number): void {
    let state = this.waitBlockedCheckStateByPtyId.get(ptyId)
    if (!state) {
      state = { lastAt: 0, lastWaitState: null, appended: '', keywordCarry: '', timer: null }
      this.waitBlockedCheckStateByPtyId.set(ptyId, state)
    }
    const appendedLower = appendedText.toLowerCase()
    const keywordHit = WAIT_BLOCKED_KEYWORD_PATTERN.test(`${state.keywordCarry}${appendedLower}`)
    state.keywordCarry = appendedLower.slice(-WAIT_BLOCKED_KEYWORD_CARRY_CHARS)
    // Why the cap keeps the tail: the accumulated text only anchors boundary-
    // spanning prompt detection; anything past the tail cap has scrolled out
    // of the retained tail the check reads anyway.
    state.appended =
      state.appended.length + appendedText.length > MAX_TAIL_CHARS
        ? `${state.appended}${appendedText}`.slice(-MAX_TAIL_CHARS)
        : `${state.appended}${appendedText}`
    const elapsed = at - state.lastAt
    if (keywordHit || elapsed >= WAIT_BLOCKED_CHECK_MIN_INTERVAL_MS || elapsed < 0) {
      this.runWaitBlockedCheck(ptyId, state, at)
      return
    }
    if (!state.timer) {
      // Why trailing edge: the final chunks of a burst must still be
      // evaluated or a prompt arriving right after a flood would go
      // unstamped until the next output.
      state.timer = setTimeout(() => {
        state.timer = null
        this.runWaitBlockedCheck(ptyId, state, Date.now())
      }, WAIT_BLOCKED_CHECK_MIN_INTERVAL_MS - elapsed)
    }
  }

  private runWaitBlockedCheck(
    ptyId: string,
    state: {
      lastAt: number
      lastWaitState: TerminalTailWaitState | null
      appended: string
      keywordCarry: string
      timer: ReturnType<typeof setTimeout> | null
    },
    at: number
  ): void {
    const pty = this.graph.ptysById.get(ptyId)
    if (!pty) {
      state.appended = ''
      return
    }
    const nextWaitState = computeTerminalTailWaitState(
      pty.tailBuffer,
      pty.tailPartialLine,
      pty.preview
    )
    const previousWaitState = state.lastWaitState ?? {
      waitText: '',
      signal: null,
      fromTail: false
    }
    if (tailGainedNewerBlockedReason(previousWaitState, nextWaitState, state.appended)) {
      pty.waitBlockedAt = at
    }
    state.lastAt = at
    state.lastWaitState = nextWaitState
    state.appended = ''
  }

  private clearWaitBlockedCheckState(ptyId: string): void {
    const state = this.waitBlockedCheckStateByPtyId.get(ptyId)
    if (state?.timer) {
      clearTimeout(state.timer)
    }
    this.waitBlockedCheckStateByPtyId.delete(ptyId)
  }

  private processAgentStatusOscForPty(ptyId: string, data: string): ProcessedAgentStatusChunk {
    let processor = this.agentStatusOscProcessorsByPtyId.get(ptyId)
    if (!processor) {
      processor = createAgentStatusOscProcessor()
      this.agentStatusOscProcessorsByPtyId.set(ptyId, processor)
    }
    return processor(data)
  }

  /** Emit the facts batched while applying one chunk/frame as a single
   *  pty:sideEffect batch, preserving byte order. */
  private flushPendingTerminalSideEffectFacts(
    ptyId: string,
    entry: RuntimePtyTitleTrackerEntry
  ): void {
    if (entry.pendingFacts.length === 0) {
      return
    }
    const facts = entry.pendingFacts
    entry.pendingFacts = []
    this.emitTerminalSideEffectBatch(ptyId, facts)
  }

  /** Feed a main-fabricated OSC title/BEL frame (agent hook spinners) through
   *  the per-PTY tracker — NOT onPtyData, so emulator state, tails,
   *  transcripts, and stats never see synthetic bytes. Parsed via the
   *  tracker's stateless synthetic path: the shared chunk bell detector must
   *  never observe fabricated bytes, or a tick interleaved with a split real
   *  OSC corrupts its escape state (phantom/swallowed bells). While the
   *  side-effect kill switch is off the legacy pty:data copy still drives
   *  renderer parsers; this ingest keeps main's facts and records
   *  authoritative. */
  ingestSyntheticTitleFrame(ptyId: string, data: string): void {
    const entry = this.getOrCreatePtyTitleTrackerEntry(ptyId)
    entry.applyingChunk = true
    entry.applyingSyntheticFrame = true
    entry.chunkTouchedSessionTabs = false
    try {
      entry.tracker.applySyntheticTitleFrame(data)
    } finally {
      entry.applyingChunk = false
      entry.applyingSyntheticFrame = false
      this.flushPendingTerminalSideEffectFacts(ptyId, entry)
    }
    if (entry.chunkTouchedSessionTabs) {
      this.touchMobileSessionSnapshotsForPty(ptyId)
    }
  }

  /** Scan-authority handoff for a backgrounded PTY (daemon keep-tail
   *  thinning): while delegated, the daemon relays bell/133/pr-link/2031
   *  facts itself and the delivered bytes may be gapped — feeding them to
   *  main's transient scanners would mint phantom or duplicate facts. Title
   *  processing stays main-side either way. */
  setPtyTransientFactDelegation(ptyId: string, delegated: boolean, scanSeedAnsi?: string): void {
    const entry = this.getOrCreatePtyTitleTrackerEntry(ptyId)
    entry.tracker.setTransientFactScanningSuppressed(delegated)
    if (!delegated && scanSeedAnsi) {
      // Prime the freshly reset scanner carry with the emulator's dangling
      // incomplete escape at the handoff position — a sequence split across
      // the un-background toggle must not mint a phantom bell or lose its
      // fact. titleScanData:'' keeps titles out (they were never suppressed).
      entry.tracker.handleChunk(scanSeedAnsi, { titleScanData: '' })
    }
  }

  /** A transient fact the daemon detected while it held scan authority —
   *  emitted through the same fact channel as byte-scanned facts. Arrives
   *  between chunks, so recordTerminalSideEffectFact emits it immediately. */
  emitDaemonPtyTransientFact(ptyId: string, fact: PtyTransientFact): void {
    switch (fact.kind) {
      case 'bell':
        this.recordTerminalSideEffectFact(ptyId, { kind: 'bell' })
        return
      case 'command-finished':
        this.recordTerminalSideEffectFact(ptyId, {
          kind: 'command-finished',
          exitCode: fact.exitCode
        })
        return
      case 'pr-link':
        this.recordTerminalSideEffectFact(ptyId, { kind: 'pr-link', link: fact.link })
        return
      case '2031-subscribe':
        this.recordTerminalSideEffectFact(ptyId, { kind: '2031-subscribe' })
    }
  }

  /** The daemon keep-tail dropped this PTY's oldest undelivered output; the
   *  next delivered chunk is discontinuous. Reset every cross-chunk parse
   *  carry so a half-open escape from before the gap cannot corrupt what
   *  follows, and drop the mobile headless mirror — it rebuilds from the
   *  delivered tail / snapshot seeds instead of parsing a gapped stream. */
  notePtyDataGap(ptyId: string, droppedChars = 0): void {
    if (droppedChars > 0) {
      // Why: the daemon snapshot's seq counts bytes its monitoring stream
      // dropped. Advancing without parsing preserves that absolute domain so
      // post-snapshot live chunks can be reconciled instead of duplicated.
      const outputSequence = (this.ptyOutputSequenceById.get(ptyId) ?? 0) + droppedChars
      this.ptyOutputSequenceById.set(ptyId, outputSequence)
    }
    const pty = this.getOrCreatePtyWorktreeRecord(ptyId)
    if (pty) {
      pty.tailPendingAnsi = ''
    }
    for (const leaf of this.getLeavesForPty(ptyId)) {
      leaf.tailPendingAnsi = ''
    }
    this.oscTitleScanTailByPtyId.delete(ptyId)
    this.osc7ScanTailByPtyId.delete(ptyId)
    this.agentStatusOscProcessorsByPtyId.delete(ptyId)
    this.disposeHeadlessTerminal(ptyId)
  }

  /** Record one derived side-effect fact: batched per chunk while applying
   *  bytes, emitted immediately for between-chunk facts (stale-title timer). */
  private recordTerminalSideEffectFact(ptyId: string, fact: TerminalSideEffectFact): void {
    if (!this.onTerminalSideEffects) {
      return
    }
    const entry = this.ptyTitleTrackersByPtyId.get(ptyId)
    if (entry?.applyingChunk) {
      entry.pendingFacts.push(fact)
      return
    }
    this.emitTerminalSideEffectBatch(ptyId, [fact])
  }

  private emitTerminalSideEffectBatch(
    ptyId: string,
    facts: TerminalSideEffectFact[],
    options: { replay?: boolean } = {}
  ): void {
    if (!this.onTerminalSideEffects || facts.length === 0) {
      return
    }
    const batch: TerminalSideEffectBatch = {
      ptyId,
      seq: this.ptyOutputSequenceById.get(ptyId) ?? 0,
      facts,
      ...(options.replay ? { replay: true } : {}),
      ...this.resolveTerminalSideEffectAttribution(ptyId)
    }
    try {
      this.onTerminalSideEffects(batch)
    } catch (err) {
      console.error('[runtime] terminal side-effect listener threw', { ptyId, err })
    }
  }

  /** Same attribution resolution as emitTerminalAgentStatusEvents: prefer the
   *  first mounted leaf, fall back to the spawn-time PTY record binding. */
  private resolveTerminalSideEffectAttribution(ptyId: string): {
    worktreeId?: string
    tabId?: string
    paneKey?: string
    connectionId?: string | null
  } {
    const pty = this.graph.ptysById.get(ptyId)
    const connectionId = pty?.connectionId ?? null
    for (const leaf of this.getLeavesForPty(ptyId)) {
      return {
        worktreeId: leaf.worktreeId,
        tabId: leaf.tabId,
        paneKey: this.makeRuntimePaneKey(leaf),
        connectionId
      }
    }
    if (pty?.paneKey) {
      return {
        worktreeId: pty.worktreeId,
        ...(pty.tabId ? { tabId: pty.tabId } : {}),
        paneKey: pty.paneKey,
        connectionId
      }
    }
    return {}
  }

  /** Title-only replay batch for renderer (re)attach — the no-attention-replay
   *  rule: snapshots restore title state, never historical bells/completions. */
  getTerminalSideEffectSnapshot(ptyId: string): TerminalSideEffectBatch | null {
    const tracker = this.ptyTitleTrackersByPtyId.get(ptyId)?.tracker
    const recordTitle = this.graph.ptysById.get(ptyId)?.lastOscTitle
    // Why: the cursor-agent literal drop applies to every title surface; a
    // record-fallback snapshot must not replay the bare native title the
    // tracker would have refused to emit live.
    const rawTitle = recordTitle && !isCursorNativeAgentTitle(recordTitle) ? recordTitle : null
    const normalizedTitle = tracker?.getLastNormalizedTitle() ?? null
    if (normalizedTitle === null && !rawTitle) {
      return null
    }
    return {
      ptyId,
      seq: this.ptyOutputSequenceById.get(ptyId) ?? 0,
      replay: true,
      facts: [
        {
          kind: 'title',
          normalizedTitle: normalizedTitle ?? normalizeTerminalTitle(rawTitle!),
          rawTitle: rawTitle ?? normalizedTitle!
        }
      ],
      ...this.resolveTerminalSideEffectAttribution(ptyId)
    }
  }

  /** Raw last title from main's tracked PTY/leaf records — the title surface
   *  the tracker (live bytes + synthetic frames) keeps current. */
  private getTrackedRawTitleForPty(ptyId: string): string | null {
    const recordTitle = this.graph.ptysById.get(ptyId)?.lastOscTitle
    if (recordTitle) {
      return recordTitle
    }
    for (const leaf of this.getLeavesForPty(ptyId)) {
      if (leaf.lastOscTitle) {
        return leaf.lastOscTitle
      }
    }
    return null
  }

  /** Why: synthetic agent title frames no longer ride pty:data, so neither
   *  renderer xterm nor the headless emulator observes them. Mobile-parity
   *  snapshot titles must prefer main's tracker over snapshot lastTitle, or
   *  hook-driven spinner/idle titles vanish from mobile tabs. */
  private preferTrackedLastTitle<T extends { lastTitle?: string }>(ptyId: string, snapshot: T): T {
    const tracked = this.getTrackedRawTitleForPty(ptyId)
    if (!tracked) {
      return snapshot
    }
    return { ...snapshot, lastTitle: tracked }
  }

  /** Decorative comparison key: spinner frame glyphs stripped, derived agent
   *  status kept so a working→idle flip with an otherwise-equal label still
   *  counts as a change. */
  private makeMobileTitleGateKey(rawTitle: string, normalizedTitle: string): string {
    return `${detectAgentStatusFromTitle(rawTitle) ?? ''}\u0000${stripBrailleSpinnerGlyphs(
      normalizedTitle
    )}`
  }

  private getOrCreatePtyTitleTrackerEntry(ptyId: string): RuntimePtyTitleTrackerEntry {
    const existing = this.ptyTitleTrackersByPtyId.get(ptyId)
    if (existing) {
      return existing
    }
    // Why: trackers are created lazily on the first observed chunk. After an
    // app relaunch the PTY/leaf records can already hold a persisted title; a
    // cold tracker would miss the parked working→idle completion and never
    // arm the stale-title timer for a persisted 'working' title.
    let initialTitle = this.graph.ptysById.get(ptyId)?.lastOscTitle ?? null
    if (initialTitle === null) {
      for (const leaf of this.getLeavesForPty(ptyId)) {
        if (leaf.lastOscTitle) {
          initialTitle = leaf.lastOscTitle
          break
        }
      }
    }
    const tracker = createTerminalTitleTracker(
      {
        onTitle: (normalizedTitle, rawTitle, meta) => {
          this.recordTerminalSideEffectFact(ptyId, {
            kind: 'title',
            normalizedTitle,
            rawTitle,
            ...(meta?.staleWorkingTitleClear ? { staleWorkingTitleClear: true } : {})
          })
          const changed = this.applyTrackedPtyTitle(ptyId, rawTitle, normalizedTitle)
          if (!changed) {
            return
          }
          const live = this.ptyTitleTrackersByPtyId.get(ptyId)
          const gateKey = this.makeMobileTitleGateKey(rawTitle, normalizedTitle)
          const decorativeOnly = live?.lastMobileTitleGateKey === gateKey
          if (live) {
            live.lastMobileTitleGateKey = gateKey
          }
          if (live?.applyingChunk) {
            // Why: synthetic spinner ticks change only the braille glyph
            // ~12.5x/sec; fanning out full mobile session snapshots per frame
            // is pure churn. Raw lastOscTitle updates above stay cheap.
            if (!(live.applyingSyntheticFrame && decorativeOnly)) {
              live.chunkTouchedSessionTabs = true
            }
          } else {
            // Stale-working-title timer path — fires between chunks, so the
            // per-chunk batching in onPtyData cannot pick it up.
            this.touchMobileSessionSnapshotsForPty(ptyId)
          }
        },
        // Why: agent transitions and bells become pty:sideEffect facts —
        // main is the single byte parser for local/SSH PTYs; the renderer
        // store handler decides what the facts mean (notification policy).
        onAgentBecameWorking: () => {
          this.recordTerminalSideEffectFact(ptyId, { kind: 'agent-working' })
        },
        onAgentBecameIdle: (title, meta) => {
          this.recordTerminalSideEffectFact(ptyId, {
            kind: 'agent-idle',
            title,
            ...(meta?.staleWorkingTitleClear ? { staleWorkingTitleClear: true } : {})
          })
        },
        onAgentExited: () => {
          this.recordTerminalSideEffectFact(ptyId, { kind: 'agent-exited' })
        },
        // Why: bell/command-finished/pr-link/2031 facts exist only for the
        // pty:sideEffect channel. Headless serve has no consumer, so skip the
        // per-chunk bell walk and 133/URL/2031 scans entirely.
        ...(this.onTerminalSideEffects
          ? {
              onBell: () => {
                this.recordTerminalSideEffectFact(ptyId, { kind: 'bell' })
              },
              onCommandFinished: (exitCode: number | null) => {
                this.recordTerminalSideEffectFact(ptyId, { kind: 'command-finished', exitCode })
              },
              onPrLink: (link: TerminalGitHubPRLink) => {
                this.recordTerminalSideEffectFact(ptyId, { kind: 'pr-link', link })
              },
              // Why: hidden-delivery-gated views never see the bytes, so main
              // surfaces DECSET 2031 subscribes as facts; the theme reply is
              // still sent by the renderer (query authority stays with the view).
              onMode2031Subscribe: () => {
                this.recordTerminalSideEffectFact(ptyId, { kind: '2031-subscribe' })
              }
            }
          : {})
      },
      initialTitle !== null ? { initialTitle } : {}
    )
    const entry: RuntimePtyTitleTrackerEntry = {
      tracker,
      applyingChunk: false,
      applyingSyntheticFrame: false,
      lastMobileTitleGateKey: null,
      chunkTouchedSessionTabs: false,
      pendingFacts: [],
      // Why: command-code facts exist only for the pty:sideEffect channel —
      // headless serve skips the per-chunk scrape entirely. The detector
      // self-arms on the Command Code banner; the spawn command (when main
      // saw one) mirrors the renderer detector's startupCommand fast-arm.
      commandCodeDetector: this.onTerminalSideEffects
        ? createCommandCodeOutputStatusDetector({
            startupCommand: this.terminalSpawnCommandsByPtyId.get(ptyId) ?? null,
            onWorking: (prompt) => {
              this.recordTerminalSideEffectFact(ptyId, { kind: 'command-code-working', prompt })
            },
            onDone: (prompt) => {
              this.recordTerminalSideEffectFact(ptyId, { kind: 'command-code-done', prompt })
            }
          })
        : null
    }
    this.ptyTitleTrackersByPtyId.set(ptyId, entry)
    return entry
  }

  /** Apply one observed OSC title (raw form) to the PTY and leaf records.
   *  Returns true when the PTY record's title or status changed. */
  private applyTrackedPtyTitle(ptyId: string, rawTitle: string, normalizedTitle: string): boolean {
    // Why: status is detected from the RAW title (mirrors the renderer tracker),
    // so working/idle transitions are unaffected by normalization; the records
    // store the NORMALIZED title so rotating Grok/Pi/Gemini frames collapse to
    // one stable stored label (#7880) instead of churning `ps`/mobile tabs.
    const agentStatus = detectAgentStatusFromTitle(rawTitle)
    let ptyRecordChanged = false
    const pty = this.graph.ptysById.get(ptyId)
    if (pty) {
      const prevStatus = pty.lastAgentStatus
      const prevTitle = pty.lastOscTitle
      const observedAt = this.nextTitleObservationSequence()
      pty.lastOscTitle = normalizedTitle
      pty.lastOscTitleAt = observedAt
      pty.lastAgentStatus = agentStatus
      this.setPtyManagementTitleFromObservedTitle(pty, normalizedTitle, observedAt)
      ptyRecordChanged = prevTitle !== normalizedTitle || prevStatus !== agentStatus
      if (agentStatus === 'idle' && prevStatus !== 'idle') {
        this.resolvePtyTuiIdleWaiters(pty, ptyId)
      }
      const shouldDelayMobileSnapshot =
        ptyRecordChanged &&
        this.shouldDelayPtyBackedMobileSnapshotForForegroundAgent(pty, normalizedTitle)
      let foregroundRefresh: Promise<boolean> | undefined
      // Why: gate on an actual status transition — braille spinner frames
      // mutate the title every tick, so probing per-title-change would stream
      // a foreground query per frame during active work.
      if (prevStatus !== agentStatus) {
        foregroundRefresh = this.refreshPtyForegroundAgentFromController(ptyId, {
          afterTitleObservation: observedAt
        })
      } else if (shouldDelayMobileSnapshot) {
        // Why: same-status compatible title changes can arrive before the
        // foreground owner probe settles; publishing them would flicker.
        foregroundRefresh = this.getPendingForegroundAgentRefreshForTitle(ptyId, observedAt)
      }
      if (foregroundRefresh && shouldDelayMobileSnapshot) {
        // Why: report "unchanged" so the per-chunk batch skips the mobile
        // snapshot fan-out; the delayed publish fires when the probe settles.
        ptyRecordChanged = false
        this.delayPtyBackedMobileSnapshotForForegroundAgent(ptyId, observedAt, foregroundRefresh)
      }
    }
    for (const leaf of this.getLeavesForPty(ptyId)) {
      // Why: keep the latest OSC title on the leaf so worktree.ps can
      // recompute status from the live title each call. Without this,
      // daemon-hosted terminals (no renderer pushing pane titles) had no
      // way to clear a stale 'working' status after the agent exited and
      // the shell took over the title — the stuck-spinner bug in #1437.
      leaf.lastOscTitle = normalizedTitle
      leaf.lastOscTitleAt = this.nextTitleObservationSequence()
      const prevStatus = leaf.lastAgentStatus
      // Why: when a new OSC title doesn't classify as an agent state (e.g.
      // bare shell title after the agent exits), clear lastAgentStatus so
      // it is no longer sticky. Tui-idle waiters that needed the previous
      // 'idle' transition were already resolved at the moment of the
      // transition below; only fresh waiters registered after the agent
      // exits would observe the cleared value, and they correctly fall
      // back to title-based detection / polling.
      leaf.lastAgentStatus = agentStatus
      // Why: resolve tui-idle on any transition TO idle (not just working→idle).
      // Claude Code may skip "working" entirely on fast tasks, going null→idle,
      // and the coordinator's tui-idle waiter would hang forever waiting for a
      // working→idle transition that never comes. Permission→idle is excluded:
      // it means the agent was blocked on user approval and the user said no,
      // which isn't a task-completion signal.
      if (agentStatus === 'idle' && prevStatus !== 'idle') {
        this.resolveTuiIdleWaiters(leaf)
        this.deliverPendingMessages(leaf)
      }
    }
    return ptyRecordChanged
  }

  /** Cancel the per-PTY title tracker (stale-title timer included) on PTY
   *  teardown so it cannot fire into pruned records. */
  private disposePtyTitleTracker(ptyId: string): void {
    this.ptyTitleTrackersByPtyId.get(ptyId)?.tracker.dispose()
    this.ptyTitleTrackersByPtyId.delete(ptyId)
  }

  private extractLastOsc7CwdForPty(
    ptyId: string,
    data: string
  ): { path: string; hostname: string } | null {
    const previousTail = this.osc7ScanTailByPtyId.get(ptyId)
    if (!previousTail && !data.includes('\x1b]7;')) {
      return null
    }
    const input = `${previousTail ?? ''}${data}`
    const scanTail = extractOscScanTail(input, 4096)
    if (scanTail.length > 0) {
      this.osc7ScanTailByPtyId.set(ptyId, scanTail)
    } else {
      this.osc7ScanTailByPtyId.delete(ptyId)
    }
    const uri = extractLastOsc7Uri(input)
    const pty = this.graph.ptysById.get(ptyId)
    const pathFlavor = this.pathFlavorForPty(pty)
    return uri
      ? parseFileUriPathParts(uri, {
          pathFlavor,
          remotePosixAuthority: !!pty?.connectionId && pathFlavor !== 'win32'
        })
      : null
  }

  private recordOsc7MetadataForPty(
    ptyId: string,
    data: string
  ): { cwd: string | null; cwdChanged: boolean } {
    const osc7 = this.extractLastOsc7CwdForPty(ptyId, data)
    const cwd = osc7?.path ?? null
    const cwdChanged =
      cwd !== null && cwd.trim().length > 0 && this.terminalCwdByPtyId.get(ptyId) !== cwd
    if (cwdChanged) {
      this.terminalCwdByPtyId.set(ptyId, cwd)
    }
    if (osc7) {
      if (osc7.hostname) {
        this.terminalFileUriHostnameByPtyId.set(ptyId, osc7.hostname)
      } else {
        this.terminalFileUriHostnameByPtyId.delete(ptyId)
      }
    }
    return { cwd, cwdChanged }
  }

  private pathFlavorForPty(pty?: RuntimePtyWorktreeRecord | null): 'posix' | 'win32' {
    if (!pty?.connectionId) {
      return process.platform === 'win32' ? 'win32' : 'posix'
    }
    const worktreePath = splitWorktreeIdForFilesystem(pty.worktreeId)?.worktreePath
    return worktreePath && isWindowsAbsolutePathLike(worktreePath) ? 'win32' : 'posix'
  }

  /** Returns true when any retained agent-row snapshot changed in a
   *  client-visible way, so the caller can republish session snapshots. */
  private emitTerminalAgentStatusEvents(ptyId: string, chunk: ProcessedAgentStatusChunk): boolean {
    // Why: snapshot retention (for mobile worktree.ps) must run even when no
    // renderer listener is attached, so we don't early-return on a missing
    // onTerminalAgentStatus — only the per-target emit below is gated on it.
    if (chunk.payloads.length === 0) {
      return false
    }
    const targets = new Map<
      string,
      {
        source: 'mounted-leaf' | 'pty-record'
        paneKey: string
        tabId?: string
        worktreeId?: string
        connectionId?: string | null
      }
    >()
    const pty = this.graph.ptysById.get(ptyId)
    const connectionId = pty?.connectionId ?? null
    for (const leaf of this.getLeavesForPty(ptyId)) {
      const paneKey = this.makeRuntimePaneKey(leaf)
      targets.set(paneKey, {
        source: 'mounted-leaf',
        paneKey,
        tabId: leaf.tabId,
        worktreeId: leaf.worktreeId,
        connectionId
      })
    }
    if (targets.size === 0 && pty?.paneKey) {
      targets.set(pty.paneKey, {
        source: 'pty-record',
        paneKey: pty.paneKey,
        tabId: pty.tabId ?? undefined,
        worktreeId: pty.worktreeId,
        connectionId
      })
    }
    let retainedChanged = false
    for (const payload of chunk.payloads) {
      for (const target of targets.values()) {
        retainedChanged =
          this.retainAgentRowSnapshot(
            ptyId,
            target.paneKey,
            target.worktreeId,
            target.tabId,
            payload
          ) || retainedChanged
        if (!this.onTerminalAgentStatus) {
          continue
        }
        try {
          this.onTerminalAgentStatus({
            ptyId,
            ...target,
            payload
          })
        } catch (err) {
          console.error('[runtime] terminal agent status listener threw', {
            ptyId,
            paneKey: target.paneKey,
            state: payload.state,
            agentType: payload.agentType,
            err
          })
        }
      }
    }
    return retainedChanged
  }

  private retainAgentRowSnapshot(
    ptyId: string,
    paneKey: string,
    worktreeId: string | undefined,
    tabId: string | undefined,
    payload: ParsedAgentStatusPayload
  ): boolean {
    const now = Date.now()
    const previous = this.latestAgentStatusByPaneKey.get(paneKey)
    // Why: stateStartedAt must mark the transition into the current state, not
    // every within-state ping (tool/prompt updates keep the state but refresh
    // updatedAt) — mirrors AgentStatusEntry.stateStartedAt on the desktop side.
    const stateStartedAt =
      previous && previous.payload.state === payload.state ? previous.stateStartedAt : now
    this.latestAgentStatusByPaneKey.set(paneKey, {
      paneKey,
      ptyId,
      worktreeId,
      tabId,
      payload,
      stateStartedAt,
      updatedAt: now
    })
    // Client-visible change detection: snapshot republish is gated on this so
    // repeated same-state hook pings don't fan a rebuild out to every client.
    return (
      !previous ||
      previous.payload.state !== payload.state ||
      previous.payload.prompt !== payload.prompt ||
      (previous.payload.agentType ?? null) !== (payload.agentType ?? null) ||
      (previous.payload.toolName ?? null) !== (payload.toolName ?? null) ||
      (previous.payload.interactivePrompt ?? null) !== (payload.interactivePrompt ?? null) ||
      (previous.payload.interrupted ?? false) !== (payload.interrupted ?? false)
    )
  }

  private clearAgentRowSnapshotsForPty(ptyId: string): void {
    for (const [paneKey, snapshot] of this.latestAgentStatusByPaneKey) {
      if (snapshot.ptyId === ptyId) {
        this.latestAgentStatusByPaneKey.delete(paneKey)
      }
    }
  }

  getPtyOutputSequence(ptyId: string): number {
    return this.ptyOutputSequenceById.get(ptyId) ?? 0
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

  private notifyRemoteTerminalViewPresenceChanged(ptyId: string): void {
    try {
      this.onRemoteTerminalViewPresenceChanged?.(ptyId)
    } catch (err) {
      console.error('[runtime] remote view presence listener threw', { ptyId, err })
    }
  }

  /** Registered by terminal-RPC subscribe/multiplex streams: while a remote
   *  view subscriber is attached its xterm answers queries with view
   *  authority and the model responder must stay silent. Returns an
   *  idempotent release. */
  registerRemoteTerminalViewSubscriber(ptyId: string): () => void {
    this.remoteTerminalViewSubscriberCounts.set(
      ptyId,
      (this.remoteTerminalViewSubscriberCounts.get(ptyId) ?? 0) + 1
    )
    this.notifyRemoteTerminalViewPresenceChanged(ptyId)
    let released = false
    return () => {
      if (released) {
        return
      }
      released = true
      const next = (this.remoteTerminalViewSubscriberCounts.get(ptyId) ?? 1) - 1
      if (next <= 0) {
        this.remoteTerminalViewSubscriberCounts.delete(ptyId)
      } else {
        this.remoteTerminalViewSubscriberCounts.set(ptyId, next)
      }
      this.notifyRemoteTerminalViewPresenceChanged(ptyId)
    }
  }

  hasRemoteTerminalViewSubscriber(ptyId: string): boolean {
    if ((this.remoteTerminalViewSubscriberCounts.get(ptyId) ?? 0) > 0) {
      return true
    }
    return this.mobileFloorCommands.hasMobileSubscriber(ptyId)
  }

  subscribeToFitOverrideChanges(
    ptyId: string,
    listener: (event: {
      mode: 'mobile-fit' | 'remote-desktop-fit' | 'desktop-fit'
      cols: number
      rows: number
    }) => void
  ): () => void {
    return addListenerToMap(this.fitOverrideListeners, ptyId, listener)
  }

  private notifyFitOverrideListeners(
    ptyId: string,
    mode: 'mobile-fit' | 'remote-desktop-fit' | 'desktop-fit',
    cols: number,
    rows: number
  ): void {
    const listeners = this.fitOverrideListeners.get(ptyId)
    if (!listeners) {
      return
    }
    for (const listener of listeners) {
      listener({ mode, cols, rows })
    }
  }

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
    return this.headlessTerminals.get(ptyId)?.emulator.isAlternateScreen ?? false
  }

  // Why: daemon-backed PTYs that the runtime adopted after an Orca relaunch
  // start with a fresh headless emulator that has zero scrollback, even though
  // the daemon's on-disk checkpoint and the desktop xterm both contain the
  // full prior history. Without this hydration, mobile subscribers see only
  // the bare current prompt because serializeHeadlessTerminalBuffer always
  // wins over the renderer-path fallback. Seeding the emulator with the
  // adapter's snapshot/cold-restore data makes mobile and desktop agree on
  // what scrollback is available.
  seedHeadlessTerminal(
    ptyId: string,
    data: string,
    size?: { cols: number; rows: number },
    metadata: HeadlessSeedMetadata = {}
  ): void {
    if (!data) {
      return
    }
    const existing = this.headlessTerminals.get(ptyId)
    if (existing) {
      // Why: emulator already has live data — re-seeding would duplicate
      // every byte. The seed is only valid when the emulator is fresh.
      return
    }
    const dims = size ?? this.getTerminalSize(ptyId) ?? { cols: 80, rows: 24 }
    const state = this.createPtyHeadlessTerminalState(ptyId, dims)
    this.headlessTerminals.set(ptyId, state)
    this.recordOsc7MetadataForPty(ptyId, data)
    this.recordRecentPtyOutputForPathProvenance(ptyId, data)
    state.writeChain = state.writeChain
      .then(async () => {
        // Why: seed writes never set forwardQueryReplies — the main-side
        // replay guard. A snapshot containing old queries must answer no one.
        await state.emulator.write(data)
        // Why AFTER the seed write: the snapshot payload cannot carry kitty
        // pushes (rehydrateSequences deliberately omits them), but ordering
        // behind it keeps the parse deterministic. Unflagged like the seed —
        // re-applying flags must answer no one.
        if (typeof metadata.kittyKeyboardFlags === 'number') {
          await state.emulator.applyKittyKeyboardFlags(metadata.kittyKeyboardFlags)
        }
        if (metadata.cwd !== undefined) {
          state.emulator.setCwd(metadata.cwd)
        }
        if (metadata.oscLinks !== undefined) {
          state.emulator.setRestoredOscLinks(metadata.oscLinks)
        }
      })
      .catch(() => {
        // Seeding is best-effort; live data will continue to populate the
        // emulator even if the snapshot replay fails.
      })
  }

  // Why: hydrate the runtime headless emulator from the desktop renderer's
  // xterm buffer on the first onPtyData byte after a PTY is taken over by a
  // pane. Eager-state pattern matches seedHeadlessTerminal: headlessTerminals
  // is populated synchronously so concurrent live writes from
  // trackHeadlessTerminalData chain after the seed via the same writeChain.
  // See docs/mobile-prefer-renderer-scrollback.md.
  private maybeHydrateHeadlessFromRenderer(ptyId: string): void {
    if (this.headlessHydrationState.has(ptyId)) {
      return
    }
    if (this.headlessTerminals.has(ptyId)) {
      // Daemon-snapshot seed already populated the emulator — skip hydration.
      this.headlessHydrationState.set(ptyId, 'done')
      return
    }
    const controller = this.ptyController
    if (!controller?.serializeBuffer || !controller.hasRendererSerializer) {
      return
    }
    if (!controller.hasRendererSerializer(ptyId)) {
      // Renderer hasn't registered yet (or never will). Live writes lazy-
      // create the state via trackHeadlessTerminalData on this same tick.
      return
    }

    this.headlessHydrationState.set(ptyId, 'pending')
    const dims = this.getTerminalSize(ptyId) ?? { cols: 80, rows: 24 }
    // Why: hydration writes below never set forwardQueryReplies (main-side
    // replay guard) — renderer-buffer snapshots can embed stale queries.
    const state = this.createPtyHeadlessTerminalState(ptyId, dims)
    this.headlessTerminals.set(ptyId, state)

    // Why: append the seed work to writeChain so live writes queued by
    // trackHeadlessTerminalData (after this method returns synchronously)
    // execute AFTER the seed-write resolves. If we awaited inline before
    // setting headlessTerminals, the live byte would lazy-create a separate
    // state and the seed-resolve would overwrite it, dropping live bytes.
    state.writeChain = state.writeChain.then(async () => {
      try {
        const rendered = await controller.serializeBuffer!(ptyId, {
          scrollbackRows: MOBILE_SUBSCRIBE_SCROLLBACK_ROWS,
          altScreenForcesZeroRows: true
        })
        if (!rendered || rendered.data.length === 0) {
          return
        }
        this.recordOsc7MetadataForPty(ptyId, rendered.data)
        this.recordRecentPtyOutputForPathProvenance(ptyId, rendered.data)
        // Resize to renderer's dims so the seed reflows correctly into the
        // emulator's grid, then resize back to PTY dims (if known) so live
        // writes use the correct cell layout.
        if (rendered.cols !== dims.cols || rendered.rows !== dims.rows) {
          state.emulator.resize(rendered.cols, rendered.rows)
        }
        await state.emulator.write(rendered.data)
        const ptyDims = this.getTerminalSize(ptyId)
        if (ptyDims && (ptyDims.cols !== rendered.cols || ptyDims.rows !== rendered.rows)) {
          state.emulator.resize(ptyDims.cols, ptyDims.rows)
        }
        // Why: the renderer xterm no longer sees synthetic hook title frames
        // (they feed main's tracker only), so its serializer lastTitle can be
        // stale here. Prefer main's tracked title; the renderer's is only the
        // seed when main has observed none (fresh relaunch, cold tracker).
        const seedTitle = this.getTrackedRawTitleForPty(ptyId) ?? rendered.lastTitle
        if (seedTitle) {
          state.emulator.setLastTitle(seedTitle)
          this.applySeededAgentStatus(ptyId, seedTitle)
        }
      } catch {
        // Hydration is best-effort. Live writes continue via the same
        // writeChain that this catch-arm leaves intact.
      } finally {
        this.headlessHydrationState.set(ptyId, 'done')
      }
    })
  }

  // Why: seed-derived agent status reflects historical state. Orchestration
  // waiters (resolveTuiIdleWaiters, deliverPendingMessages) must only react
  // to LIVE transitions, so this helper writes leaf.lastAgentStatus only and
  // never resolves waiters. detectAgentStatusFromTitle wrap mirrors the live
  // path so seeded and live values are the same union member, keeping
  // downstream `=== 'idle'` checks correct.
  private applySeededAgentStatus(ptyId: string, title: string): void {
    if (!title) {
      return
    }
    // Why: a relaunched main starts its per-PTY title tracker cold — without
    // this seed it misses the parked working→idle completion and never arms
    // the stale-title timer for a persisted 'working' title. Seeding no-ops
    // once a live title was observed, so live state always wins.
    this.getOrCreatePtyTitleTrackerEntry(ptyId).tracker.seedInitialTitle(title)
    const status = detectAgentStatusFromTitle(title)
    // Why: live observations store normalized titles, so seeds must match —
    // otherwise the first live frame after hydration compares unequal and
    // touches session tabs once for no visible change.
    const seededTitle = normalizeTerminalTitle(title)
    const pty = this.graph.ptysById.get(ptyId)
    if (pty) {
      const observedAt = this.nextTitleObservationSequence()
      pty.lastOscTitle = seededTitle
      pty.lastOscTitleAt = observedAt
      this.setPtyManagementTitleFromObservedTitle(pty, seededTitle, observedAt)
    }
    for (const leaf of this.getLeavesForPty(ptyId)) {
      // Why: seed lastOscTitle even when the seeded title doesn't classify
      // as an agent state, so worktree.ps recomputes status from the live
      // title rather than treating the leaf as agentless.
      leaf.lastOscTitle = seededTitle
      leaf.lastOscTitleAt = this.nextTitleObservationSequence()
      if (status !== null) {
        leaf.lastAgentStatus = status
      }
    }
  }

  /** Per-chunk reply-ownership capture (Phase 5). Evaluated synchronously at
   *  ingestion only — never re-read at reply time. */
  private shouldAnswerQueriesForLiveChunk(ptyId: string): boolean {
    return shouldModelAnswerHiddenPtyQueries({
      ptyId,
      settings: this.store?.getSettings(),
      hasRemoteViewSubscriber: this.hasRemoteTerminalViewSubscriber(ptyId)
    })
  }

  private trackHeadlessTerminalData(
    ptyId: string,
    data: string,
    outputSequence: number,
    forwardQueryReplies = false
  ): void {
    const state = this.getOrCreateHeadlessTerminal(ptyId)
    state.writeChain = state.writeChain
      .then(async () => {
        // Why: the ingestion-time ownership decision is closed over this
        // chain link; async scheduling cannot retroactively change it.
        await state.emulator.write(data, { forwardQueryReplies })
        state.outputSequence = outputSequence
      })
      .catch(() => {
        // Best-effort state tracking; live streaming must continue even if
        // xterm rejects a malformed or raced write during shutdown.
      })
  }

  /** Shared factory for the per-PTY runtime emulators (seed, hydration, and
   *  lazy live-byte creation): wires the Phase-5 query-reply sink and the
   *  ConPTY DA1 override. The daemon emulator never goes through here. */
  private createPtyHeadlessTerminalState(
    ptyId: string,
    dims: { cols: number; rows: number }
  ): RuntimeHeadlessTerminal {
    let state: RuntimeHeadlessTerminal | null = null
    const pathFlavor = this.pathFlavorForPty(this.graph.ptysById.get(ptyId))
    const emulator = new HeadlessEmulator({
      cols: dims.cols,
      rows: dims.rows,
      pathFlavor,
      remotePosixFileUriAuthority:
        !!this.graph.ptysById.get(ptyId)?.connectionId && pathFlavor !== 'win32',
      // Why: replies take the provider input path (same entry as pty:write —
      // daemon shell-ready gating and the SSH relay write apply unchanged),
      // NOT writePtyInput, so renderer interactive-output metering never
      // counts responder traffic as user-input echo.
      onQueryReply: (reply) => {
        // Why the identity check: queued writeChain links can parse after
        // disposeHeadlessTerminal, and daemon respawns reuse session ids — a
        // stale link's reply must never reach a successor PTY under this id.
        if (state !== null && this.headlessTerminals.get(ptyId) === state) {
          // Why this write is safe pre-shell-ready: daemon Session.write
          // QUEUES (never drops) input while the POSIX shell-ready gate is
          // pending and flushes at the ready marker or the 15s
          // SHELL_READY_TIMEOUT_MS bound (session.ts) — a spawn-time query
          // reply is delayed at most that bound, not lost.
          this.ptyController?.write(ptyId, reply)
        }
      }
    })
    if (isNativeWindowsConptyPty(ptyId)) {
      emulator.installConptyPrimaryDeviceAttributesOverride()
    }
    // Why the lazy getter: replies must use the freshest renderer push at
    // parse time, and stay silent (never default) before the first push.
    emulator.installViewAttributeResponder(() => getTerminalViewAttributes())
    const viewAttributes = getTerminalViewAttributes()
    if (viewAttributes) {
      emulator.applyPushedViewAttributes(viewAttributes)
    }
    state = { emulator, outputSequence: 0, writeChain: Promise.resolve() }
    return state
  }

  /** Phase-5 ConPTY DA1 retrofit (terminal-query-authority.md): invoked via
   *  markNativeWindowsConptyPty when the spawn mark lands after daemon stream
   *  data already created this PTY's emulator. Idempotent emulator-side. */
  private ensureNativeWindowsConptyDa1Override(ptyId: string): void {
    if (isNativeWindowsConptyPty(ptyId)) {
      this.headlessTerminals.get(ptyId)?.emulator.installConptyPrimaryDeviceAttributesOverride()
    }
  }

  private getOrCreateHeadlessTerminal(ptyId: string): RuntimeHeadlessTerminal {
    const existing = this.headlessTerminals.get(ptyId)
    if (existing) {
      return existing
    }
    const size = this.getTerminalSize(ptyId) ?? { cols: 80, rows: 24 }
    const state = this.createPtyHeadlessTerminalState(ptyId, size)
    this.headlessTerminals.set(ptyId, state)
    return state
  }

  private resizeHeadlessTerminal(ptyId: string, cols: number, rows: number): void {
    const state = this.headlessTerminals.get(ptyId)
    if (!state) {
      return
    }
    // Why: terminal reflow is a parser operation. It must sit in the same
    // per-PTY stream as output bytes or restore snapshots can bake in wraps
    // from the wrong terminal width.
    state.writeChain = state.writeChain
      .then(() => {
        state.emulator.resize(cols, rows)
      })
      .catch(() => {
        // Best-effort mirror tracking; live PTY streaming must continue even
        // if xterm rejects a raced resize during teardown.
      })
  }

  // Public: desktop-initiated clears (ipc/pty.ts) must also drop this mobile
  // mirror or a resubscribing mobile client resurrects the cleared scrollback.
  async clearHeadlessTerminalBuffer(ptyId: string): Promise<void> {
    const state = this.headlessTerminals.get(ptyId)
    if (!state) {
      return
    }
    // Why: headless writes are queued to preserve xterm parser order. Clear
    // must join that same chain or an earlier PTY chunk can finish after the
    // clear request and repopulate mobile scrollback.
    state.writeChain = state.writeChain.then(() => state.emulator.clearScrollback())
    await state.writeChain
  }

  private async serializeTerminalBufferFromAvailableState(
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
    pendingEscapeTailAnsi?: string
  } | null> {
    const headlessSnapshot = await this.serializeHeadlessTerminalBuffer(ptyId, opts)
    if (headlessSnapshot) {
      return headlessSnapshot
    }

    return this.serializeRendererTerminalBuffer(ptyId, opts)
  }

  private async serializeRendererTerminalBuffer(
    ptyId: string,
    opts: { scrollbackRows?: number } = {}
  ): Promise<{
    data: string
    cols: number
    rows: number
    cwd?: string | null
    lastTitle?: string
    source?: 'renderer'
    oscLinks?: TerminalOscLinkRange[]
  } | null> {
    let rendererSnapshot: {
      data: string
      cols: number
      rows: number
      cwd?: string | null
      lastTitle?: string
      oscLinks?: TerminalOscLinkRange[]
    } | null = null
    try {
      // Why: recovery/read fallback wants visible alt-screen content (e.g. an
      // active TUI), so altScreenForcesZeroRows is FALSE here. Hydration is
      // the only path that suppresses alt-screen scrollback.
      rendererSnapshot = await (this.ptyController?.serializeBuffer?.(ptyId, {
        scrollbackRows: opts.scrollbackRows,
        altScreenForcesZeroRows: false
      }) ?? Promise.resolve(null))
    } catch {
      // Why: terminal snapshots should not depend on a mounted renderer pane.
      // If renderer serialization races reload/unmount, callers can still use
      // their existing null fallback paths.
    }
    return rendererSnapshot
      ? this.preferTrackedLastTitle(ptyId, {
          ...rendererSnapshot,
          cwd: rendererSnapshot.cwd ?? this.terminalCwdByPtyId.get(ptyId),
          source: 'renderer' as const
        })
      : null
  }

  private async withVisibleSnapshotFallback(
    ptyId: string,
    read: RuntimeTerminalRead,
    opts: { cursor?: number; limit?: number } = {}
  ): Promise<RuntimeTerminalRead> {
    if (!shouldFallbackToVisibleTerminalSnapshot(read, opts)) {
      return read
    }
    const lines = await this.readRendererVisibleSnapshotLines(ptyId)
    if (lines.length === 0) {
      return read
    }
    return buildVisibleSnapshotReadFallback(read, lines, opts.limit)
  }

  private async readRendererVisibleSnapshotLines(ptyId: string): Promise<string[]> {
    const controller = this.ptyController
    if (!controller?.serializeBuffer) {
      return []
    }
    if (controller.hasRendererSerializer && !controller.hasRendererSerializer(ptyId)) {
      return []
    }
    try {
      // Why: raw PTY tails can be whitespace-only while a full-screen TUI is
      // visibly nonblank in renderer xterm. Ask the renderer for the active
      // screen instead of reusing the headless transcript path.
      const snapshot = await controller.serializeBuffer(ptyId, {
        scrollbackRows: 0,
        altScreenForcesZeroRows: false
      })
      if (!snapshot || snapshot.data.length === 0) {
        return []
      }
      const emulator = new HeadlessEmulator({
        cols: snapshot.cols,
        rows: snapshot.rows,
        scrollback: 0
      })
      try {
        await emulator.write(snapshot.data)
        return emulator
          .getVisibleLines()
          .map((line) => line.trimEnd())
          .filter((line) => line.trim().length > 0)
      } finally {
        emulator.dispose()
      }
    } catch {
      return []
    }
  }

  private async serializeHeadlessTerminalBuffer(
    ptyId: string,
    opts: { scrollbackRows?: number; includeEmpty?: boolean } = {}
  ): Promise<{
    data: string
    cols: number
    rows: number
    cwd?: string | null
    lastTitle?: string
    seq?: number
    source?: 'headless'
    oscLinks?: TerminalOscLinkRange[]
    alternateScreen?: boolean
    scrollbackAnsi?: string
    // Why: dangling mid-escape tail the restorer must write LAST, after any
    // reset, so the next live chunk completes it instead of rendering it
    // literally (Bug E / #7329).
    pendingEscapeTailAnsi?: string
  } | null> {
    const state = this.headlessTerminals.get(ptyId)
    if (!state) {
      return null
    }
    await state.writeChain
    // Why: normal history is separated from an active alternate frame, so the
    // caller's scrollback policy can be honored without painting it into alt.
    const isAlternateScreen = state.emulator.isAlternateScreen
    const scrollbackRows = opts.scrollbackRows ?? 0
    const snapshot = state.emulator.getSnapshot({ scrollbackRows })
    const data = snapshot.rehydrateSequences + snapshot.snapshotAnsi
    return data.length > 0 || opts.includeEmpty === true
      ? this.preferTrackedLastTitle(ptyId, {
          data,
          cols: snapshot.cols,
          rows: snapshot.rows,
          cwd: snapshot.cwd ?? this.terminalCwdByPtyId.get(ptyId),
          lastTitle: snapshot.lastTitle,
          seq: state.outputSequence,
          source: 'headless' as const,
          oscLinks: snapshot.oscLinks,
          scrollbackAnsi: snapshot.scrollbackAnsi,
          ...(snapshot.pendingEscapeTailAnsi
            ? { pendingEscapeTailAnsi: snapshot.pendingEscapeTailAnsi }
            : {}),
          // Why: lets the renderer skip the destructive scrollback clear when
          // restoring an alt-screen snapshot — clearing wipes xterm's own
          // history that the TUI relies on for scroll-up after a tab return.
          alternateScreen: isAlternateScreen,
          // Why NOT folded into data: the renderer writes its post-replay
          // reset after data, and any ESC after a dangling partial aborts it.
          // The restorer writes this last (Bug E fix).
          pendingEscapeTailAnsi: snapshot.pendingEscapeTailAnsi
        })
      : null
  }

  private disposeHeadlessTerminal(ptyId: string): void {
    this.headlessHydrationState.delete(ptyId)
    const state = this.headlessTerminals.get(ptyId)
    if (!state) {
      return
    }
    this.headlessTerminals.delete(ptyId)
    // Why: queued chain links still parse below before the emulator disposes;
    // sever the reply sink now so they cannot write to a respawned PTY that
    // reused this id (belt to the sink's state-identity check).
    state.emulator.disableQueryReplyForwarding()
    state.writeChain.finally(() => state.emulator.dispose()).catch(() => state.emulator.dispose())
  }

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
    const tracked = this.terminalCwdByPtyId.get(ptyId)
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
    return ptyId ? (this.terminalFileUriHostnameByPtyId.get(ptyId) ?? null) : null
  }

  private recordRecentPtyOutputForPathProvenance(ptyId: string, data: string): void {
    this.recentPtyOutputById.set(
      ptyId,
      appendRecentPtyOutput(this.recentPtyOutputById.get(ptyId), data)
    )
    this.recentPtyPathCandidatesById.set(
      ptyId,
      appendRecentPtyPathCandidates(this.recentPtyPathCandidatesById.get(ptyId), data)
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
    const recentOutput = ptyId ? this.recentPtyOutputById.get(ptyId) : null
    if (recentOutput && recentTerminalOutputIncludesPath(recentOutput, pathText, absolutePath)) {
      return true
    }
    const candidates = ptyId ? this.recentPtyPathCandidatesById.get(ptyId) : null
    return candidates
      ? recentTerminalPathCandidatesIncludePath(candidates, pathText, absolutePath)
      : false
  }

  registerSubscriptionCleanup(
    subscriptionId: string,
    cleanup: () => void,
    connectionId?: string
  ): void {
    // Why: mobile clients reconnect frequently (phone lock, network switch).
    // The RPC client re-sends terminal.subscribe on reconnect, creating a new
    // handler before the old one is cleaned up. Without this, the old data
    // listener leaks in dataListeners and duplicates every PTY data event.
    const existing = this.subscriptionCleanups.get(subscriptionId)
    if (existing) {
      this.cleanupSubscription(subscriptionId)
    }
    this.subscriptionCleanups.set(subscriptionId, cleanup)
    if (connectionId) {
      let set = this.subscriptionsByConnection.get(connectionId)
      if (!set) {
        set = new Set()
        this.subscriptionsByConnection.set(connectionId, set)
      }
      set.add(subscriptionId)
      this.subscriptionConnectionByEntry.set(subscriptionId, connectionId)
    }
  }

  cleanupSubscription(subscriptionId: string): void {
    const cleanup = this.subscriptionCleanups.get(subscriptionId)
    if (cleanup) {
      this.subscriptionCleanups.delete(subscriptionId)
      const connectionId = this.subscriptionConnectionByEntry.get(subscriptionId)
      if (connectionId) {
        this.subscriptionConnectionByEntry.delete(subscriptionId)
        const set = this.subscriptionsByConnection.get(connectionId)
        if (set) {
          set.delete(subscriptionId)
          if (set.size === 0) {
            this.subscriptionsByConnection.delete(connectionId)
          }
        }
      }
      cleanup()
    }
  }

  cleanupSubscriptionsByPrefix(prefix: string): void {
    const ids = Array.from(this.subscriptionCleanups.keys()).filter((id) => id.startsWith(prefix))
    for (const id of ids) {
      this.cleanupSubscription(id)
    }
  }

  // Why: invoked from the WebSocket transport's on-close hook so streaming
  // listeners registered for this exact socket get torn down even when other
  // sockets sharing the same deviceToken are still alive (multi-screen
  // mobile). Without this sweep, listeners leak across every reconnect.
  cleanupSubscriptionsForConnection(connectionId: string): void {
    const set = this.subscriptionsByConnection.get(connectionId)
    if (!set) {
      return
    }
    // Why: snapshot the ids before iterating because cleanupSubscription
    // mutates both the set and the index map.
    const ids = Array.from(set)
    for (const id of ids) {
      this.cleanupSubscription(id)
    }
  }

  // Why: mobile clients subscribe via notifications.subscribe streaming RPC.
  // Each subscriber gets its own listener. Returns an unsubscribe function
  // that the subscription cleanup mechanism calls on disconnect.
  onNotificationDispatched(listener: (event: MobileNotificationEvent) => void): () => void {
    this.notificationListeners.add(listener)
    return () => {
      this.notificationListeners.delete(listener)
    }
  }

  getMobileNotificationListenerCount(): number {
    return this.notificationListeners.size
  }

  dispatchMobileNotification(event: MobileNotificationEvent): void {
    for (const listener of this.notificationListeners) {
      listener(event)
    }
    // TASK-036: fire web push for agent task completions.
    // Fire-and-forget — push errors must never surface to the caller.
    if (
      event.type === 'notification' &&
      event.source === 'agent-task-complete' &&
      this.pushManager
    ) {
      this.pushManager
        .sendToAll({
          title: event.title,
          body: event.body,
          tag: event.worktreeId ? `worktree-${event.worktreeId}` : 'agent-task-complete',
          url: event.worktreeId ? `/worktree/${event.worktreeId}` : undefined
        })
        .catch((err: unknown) => {
          console.error('[WebPush] sendToAll failed:', err)
        })
    }
  }

  dismissMobileNotification(notificationId: string): void {
    this.dispatchMobileNotification({ type: 'dismiss', notificationId })
  }

  // ─── Account Services (mobile RPC bridge) ─────────────────────

  setAccountServices(services: RuntimeAccountServices): void {
    this.accountServices = services
  }

  setCommitMessageAgentEnvironmentResolvers(
    resolvers: CommitMessageAgentEnvironmentResolvers
  ): void {
    this.commitMessageAgentEnv = resolvers
  }

  getCommitMessageAgentEnvironmentResolvers(): CommitMessageAgentEnvironmentResolvers | undefined {
    return this.commitMessageAgentEnv ?? undefined
  }

  // Lists the speech-model catalog joined with live download/ready state, plus
  // the current enabled flag + selected model, so mobile can present a dictation
  // setup sheet and drive remote enable/download. Always targets this (paired)
  // desktop — speech never routes to a worktree's SSH host.
  async listMobileSpeechModels(): Promise<RuntimeSpeechSetupState> {
    if (!this.store) {
      throw new Error('voice_dictation_unavailable')
    }
    const voice = this.store.getSettings().voice ?? getDefaultVoiceSettings()
    const states = await getSpeechModelManager(this.store).getModelStates()
    const stateById = new Map(states.map((state) => [state.id, state]))
    const models: RuntimeSpeechModelSummary[] = SPEECH_MODEL_CATALOG.map((manifest) => {
      const state = stateById.get(manifest.id)
      return {
        id: manifest.id,
        label: manifest.label,
        provider: manifest.provider === 'openai' ? 'openai' : 'local',
        sizeBytes: manifest.sizeBytes ?? null,
        recommended: manifest.recommended === true,
        status: state?.status ?? 'not-downloaded',
        progress: state?.progress ?? null
      }
    })
    return {
      enabled: voice.enabled === true,
      selectedModelId: voice.sttModel ?? '',
      dictationMode: voice.dictationMode === 'hold' ? 'hold' : 'toggle',
      models
    }
  }

  // Fire-and-forget model download; the ModelManager writes progress into its
  // per-model state, which mobile reads back via listMobileSpeechModels polling.
  async downloadMobileSpeechModel(modelId: string): Promise<{ started: true }> {
    if (!this.store) {
      throw new Error('voice_dictation_unavailable')
    }
    const manifest = getCatalogModel(modelId)
    if (!manifest || !isLocalSpeechModel(manifest)) {
      throw new Error('voice_model_not_downloadable')
    }
    // Why: do not await — downloads run for tens of seconds; the call returns
    // immediately and mobile polls for progress/ready.
    void getSpeechModelManager(this.store)
      .downloadModel(modelId)
      .catch((err) => {
        console.error('[runtime] mobile speech model download failed', { modelId, err })
      })
    return { started: true }
  }

  async deleteMobileSpeechModel(modelId: string): Promise<RuntimeSpeechSetupState> {
    if (!this.store?.getSettings || !this.store.updateSettings) {
      throw new Error('voice_dictation_unavailable')
    }
    const store = this.store
    try {
      // The runtime store is adapted to the minimal speech settings contract used by deletion.
      await deleteLocalSpeechModel({
        store: {
          getSettings: () => store.getSettings(),
          updateSettings: (updates, options) => store.updateSettings?.(updates, options)
        },
        modelManager: getSpeechModelManager(store),
        sttService: getSpeechSttService(store),
        modelId
      })
    } catch (error) {
      throw new Error(getSpeechModelDeletionErrorCode(error) ?? 'voice_model_delete_failed')
    }
    return this.listMobileSpeechModels()
  }

  // Enables/disables dictation and/or selects the model, merging into the
  // existing voice settings so other voice fields are preserved.
  async configureMobileDictation(params: {
    enabled?: boolean
    modelId?: string
    dictationMode?: 'toggle' | 'hold'
  }): Promise<RuntimeSpeechSetupState> {
    if (!this.store?.getSettings || !this.store.updateSettings) {
      throw new Error('voice_dictation_unavailable')
    }
    const current = this.store.getSettings().voice ?? getDefaultVoiceSettings()
    // An explicit '' clears the selected model (the OptionalString RPC schema
    // maps '' → undefined, so this only matters for direct callers); any other
    // non-empty modelId must be a known catalog entry.
    if (params.modelId !== undefined && params.modelId !== '' && !getCatalogModel(params.modelId)) {
      throw new Error('voice_model_unknown')
    }
    const nextVoice: VoiceSettings = {
      ...current,
      ...(params.enabled !== undefined ? { enabled: params.enabled } : {}),
      ...(params.modelId !== undefined ? { sttModel: params.modelId } : {}),
      ...(params.dictationMode !== undefined ? { dictationMode: params.dictationMode } : {})
    }
    this.store.updateSettings({ voice: nextVoice }, { notifyListeners: true })
    return this.listMobileSpeechModels()
  }

  async startMobileDictation(params: {
    dictationId: string
    modelId?: string
    clientId?: string
    connectionId?: string
  }): Promise<{
    dictationId: string
    modelId: string
  }> {
    if (!this.store) {
      throw new Error('voice_dictation_unavailable')
    }

    const voice = this.store.getSettings().voice ?? getDefaultVoiceSettings()
    if (!voice.enabled) {
      throw new Error('voice_dictation_disabled')
    }

    const modelId = params.modelId || voice.sttModel
    if (!modelId) {
      throw new Error('voice_model_not_selected')
    }

    const modelState = await getSpeechModelManager(this.store).getModelState(modelId)
    if (modelState.status !== 'ready') {
      throw new Error(`voice_model_not_ready:${modelState.status}`)
    }

    if (!params.clientId) {
      throw new Error('dictation_requires_mobile_client')
    }

    if (this.mobileDictation) {
      throw new Error('dictation_already_active')
    }

    const owner = `mobile:${params.dictationId}`
    this.mobileDictation = {
      id: params.dictationId,
      owner,
      clientId: params.clientId,
      connectionId: params.connectionId,
      state: 'starting',
      partialText: '',
      finalTexts: [],
      errors: []
    }

    try {
      await getSpeechSttService(this.store).startDictation(
        modelId,
        (event) => {
          const session = this.mobileDictation
          if (!session || session.id !== params.dictationId) {
            return
          }
          if (event.type === 'partial') {
            session.partialText = event.text ?? ''
          } else if (event.type === 'final') {
            const text = event.text?.trim()
            if (text) {
              session.finalTexts.push(text)
              session.partialText = ''
            }
          } else if (event.type === 'error') {
            session.errors.push(event.error ?? 'Speech worker error')
          }
        },
        undefined,
        owner
      )
      if (this.mobileDictation?.id !== params.dictationId) {
        throw new Error('dictation_canceled')
      }
      this.mobileDictation.state = 'active'
    } catch (error) {
      if (this.mobileDictation?.id === params.dictationId) {
        this.mobileDictation = null
      }
      throw error
    }

    return { dictationId: params.dictationId, modelId }
  }

  feedMobileDictation(params: {
    dictationId: string
    audioBase64: string
    sampleRate: number
    clientId?: string
    connectionId?: string
  }): {
    dictationId: string
  } {
    const session = this.mobileDictation
    if (!session || session.id !== params.dictationId) {
      throw new Error('dictation_stream_not_started')
    }
    if (!params.clientId || session.clientId !== params.clientId) {
      throw new Error('dictation_owner_mismatch')
    }
    if (session.connectionId && session.connectionId !== params.connectionId) {
      throw new Error('dictation_owner_mismatch')
    }
    if (session.state !== 'active') {
      throw new Error('dictation_stream_closing')
    }
    if (session.errors.length > 0) {
      throw new Error(session.errors[0])
    }

    const pcm = Buffer.from(params.audioBase64, 'base64')
    const samples = new Float32Array(Math.floor(pcm.length / 2))
    for (let i = 0; i < samples.length; i += 1) {
      samples[i] = pcm.readInt16LE(i * 2) / 32768
    }
    getSpeechSttService(this.store!).feedAudio(samples, params.sampleRate, session.owner)
    return { dictationId: params.dictationId }
  }

  async finishMobileDictation(params: {
    dictationId: string
    clientId?: string
    connectionId?: string
  }): Promise<{
    dictationId: string
    text: string
  }> {
    const session = this.mobileDictation
    if (!session || session.id !== params.dictationId) {
      throw new Error('dictation_stream_not_started')
    }
    if (!params.clientId || session.clientId !== params.clientId) {
      throw new Error('dictation_owner_mismatch')
    }
    if (session.connectionId && session.connectionId !== params.connectionId) {
      throw new Error('dictation_owner_mismatch')
    }
    session.state = 'closing'
    try {
      await getSpeechSttService(this.store!).stopDictation(session.owner)
      if (session.errors.length > 0) {
        throw new Error(session.errors[0])
      }
      const text = [...session.finalTexts, session.partialText].join(' ').trim()
      return { dictationId: params.dictationId, text }
    } finally {
      if (this.mobileDictation?.id === session.id) {
        this.mobileDictation = null
      }
    }
  }

  async cancelMobileDictation(params: {
    dictationId: string
    clientId?: string
    connectionId?: string
  }): Promise<{ dictationId: string }> {
    const session = this.mobileDictation
    if (
      session?.id === params.dictationId &&
      params.clientId &&
      session.clientId === params.clientId &&
      (!session.connectionId || session.connectionId === params.connectionId)
    ) {
      session.state = 'closing'
      try {
        await getSpeechSttService(this.store!).stopDictation(session.owner)
      } finally {
        if (this.mobileDictation?.id === session.id) {
          this.mobileDictation = null
        }
      }
    }
    return { dictationId: params.dictationId }
  }

  private cancelMobileDictationSession(session: NonNullable<typeof this.mobileDictation>): void {
    if (session.state === 'closing') {
      return
    }
    session.state = 'closing'
    void getSpeechSttService(this.store!)
      .stopDictation(session.owner)
      .finally(() => {
        if (this.mobileDictation?.id === session.id) {
          this.mobileDictation = null
        }
      })
  }

  cancelMobileDictationForConnection(connectionId: string): void {
    const session = this.mobileDictation
    if (!session || session.connectionId !== connectionId) {
      return
    }
    this.cancelMobileDictationSession(session)
  }

  private cancelMobileDictationForClient(clientId: string): void {
    const session = this.mobileDictation
    if (!session || session.clientId !== clientId) {
      return
    }
    this.cancelMobileDictationSession(session)
  }

  private requireAccountServices(): RuntimeAccountServices {
    if (!this.accountServices) {
      throw new Error('Account services are not configured on this runtime')
    }
    return this.accountServices
  }

  getAccountsSnapshot(): AccountsSnapshot {
    const { claudeAccounts, codexAccounts, rateLimits } = this.requireAccountServices()
    return {
      claude: claudeAccounts.listAccounts(),
      codex: codexAccounts.listAccounts(),
      rateLimits: rateLimits.getState()
    }
  }

  // Why: RateLimitService polls only when the Electron window is visible AND
  // focused, and the inactive-account caches fill lazily when the user opens
  // the desktop AccountsPane. Mobile has neither trigger, so without this the
  // phone shows 0% / "—" against a backgrounded desktop. Errors swallowed
  // because partial usage is still useful for the rest of the snapshot.
  async refreshAccountsForMobile(): Promise<void> {
    const { rateLimits } = this.requireAccountServices()
    await Promise.allSettled([
      rateLimits.refresh(),
      rateLimits.fetchInactiveClaudeAccountsOnOpen(),
      rateLimits.fetchInactiveCodexAccountsOnOpen()
    ])
  }

  selectClaudeAccount(accountId: string | null): Promise<ClaudeRateLimitAccountsState> {
    return this.requireAccountServices().claudeAccounts.selectAccount(accountId)
  }

  selectCodexAccount(accountId: string | null): Promise<CodexRateLimitAccountsState> {
    return this.requireAccountServices().codexAccounts.selectAccount(accountId)
  }

  removeClaudeAccount(accountId: string): Promise<ClaudeRateLimitAccountsState> {
    return this.requireAccountServices().claudeAccounts.removeAccount(accountId)
  }

  removeCodexAccount(accountId: string): Promise<CodexRateLimitAccountsState> {
    return this.requireAccountServices().codexAccounts.removeAccount(accountId)
  }

  // Why: rate-limit polling fires every 5 minutes and on account switch.
  // Mobile clients subscribe to receive a fresh AccountsSnapshot whenever
  // RateLimitService pushes new usage data, mirroring the existing
  // `rateLimits:update` IPC channel desktop already uses.
  onAccountsChanged(listener: (snapshot: AccountsSnapshot) => void): () => void {
    const services = this.requireAccountServices()
    return services.rateLimits.onStateChange((rateLimits) => {
      listener({
        claude: services.claudeAccounts.listAccounts(),
        codex: services.codexAccounts.listAccounts(),
        rateLimits
      })
    })
  }

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
      this.activeBrowserScreencastsByPage.get(browserPageId)?.cancel(true),
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
    this.remoteTerminalViewSubscriberCounts.delete(ptyId)
    this.recentPtyOutputById.delete(ptyId)
    this.clearWaitBlockedCheckState(ptyId)
    this.recentPtyPathCandidatesById.delete(ptyId)
    this.ptyOutputSequenceById.delete(ptyId)
    this.agentStatusOscProcessorsByPtyId.delete(ptyId)
    this.terminalSpawnCommandsByPtyId.delete(ptyId)
    this.disposePtyTitleTracker(ptyId)
    this.oscTitleScanTailByPtyId.delete(ptyId)
    this.osc7ScanTailByPtyId.delete(ptyId)
    this.terminalCwdByPtyId.delete(ptyId)
    this.terminalFileUriHostnameByPtyId.delete(ptyId)
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

  async listTerminals(
    worktreeSelector?: string,
    limit = DEFAULT_TERMINAL_LIST_LIMIT,
    opts: { requireFreshPtyLiveness?: boolean } = {}
  ): Promise<RuntimeTerminalListResult> {
    if (!Number.isInteger(limit) || limit <= 0) {
      throw new Error('invalid_limit')
    }
    const graphEpoch = this.graph.graphStatus === 'ready' ? this.graph.rendererGraphEpoch : null
    const explicitTargetWorktreeId = worktreeSelector
      ? this.getValidatedExplicitWorktreeIdSelector(worktreeSelector)
      : null
    const initialResolvedWorktreeCache = this.resolvedWorktreeCommands.peekCache()
    const cachedResolvedWorktrees =
      initialResolvedWorktreeCache && initialResolvedWorktreeCache.expiresAt > Date.now()
        ? initialResolvedWorktreeCache.worktrees
        : null
    const cachedExplicitTargetWorktree =
      explicitTargetWorktreeId && cachedResolvedWorktrees
        ? (cachedResolvedWorktrees.find((worktree) => worktree.id === explicitTargetWorktreeId) ??
          null)
        : null
    const parsedExplicitTargetWorktree =
      explicitTargetWorktreeId && !cachedExplicitTargetWorktree
        ? this.buildResolvedWorktreeFromId(explicitTargetWorktreeId)
        : null
    const targetWorktree =
      worktreeSelector && !explicitTargetWorktreeId
        ? await this.resolveWorktreeSelector(worktreeSelector)
        : (cachedExplicitTargetWorktree ?? parsedExplicitTargetWorktree)
    const targetWorktreeId = explicitTargetWorktreeId ?? targetWorktree?.id ?? null
    const classificationResolvedWorktreeCache = this.resolvedWorktreeCommands.peekCache()
    const classificationResolvedWorktrees =
      targetWorktreeId &&
      classificationResolvedWorktreeCache &&
      classificationResolvedWorktreeCache.expiresAt > Date.now()
        ? includeTargetResolvedWorktree(
            classificationResolvedWorktreeCache.worktrees,
            targetWorktree
          )
        : targetWorktreeId && explicitTargetWorktreeId
          ? this.listKnownResolvedWorktreesForExplicitTarget(targetWorktreeId, targetWorktree)
          : null
    const worktreesById =
      targetWorktreeId && targetWorktree
        ? new Map([[targetWorktree.id, targetWorktree]])
        : targetWorktreeId
          ? new Map()
          : await this.getResolvedWorktreeMap()
    if (graphEpoch !== null) {
      this.assertStableReadyGraph(graphEpoch)
    }

    const resolvedWorktrees =
      targetWorktreeId && classificationResolvedWorktrees
        ? classificationResolvedWorktrees
        : targetWorktreeId && targetWorktree
          ? [targetWorktree]
          : targetWorktreeId
            ? []
            : [...worktreesById.values()]
    const refreshedPtyLiveness = await this.refreshPtyWorktreeRecordsFromController(
      resolvedWorktrees,
      targetWorktreeId
    )
    if (opts.requireFreshPtyLiveness && !refreshedPtyLiveness) {
      throw new Error('terminal_liveness_unavailable')
    }

    const livePtyWorktreeIds = new Set<string>()
    for (const pty of this.graph.ptysById.values()) {
      if (pty.connected) {
        livePtyWorktreeIds.add(pty.worktreeId)
      }
    }

    const terminals: RuntimeTerminalSummary[] = []
    const ptyIdsFromLeaves = new Set<string>()
    if (graphEpoch !== null) {
      for (const leaf of this.graph.leaves.values()) {
        if (targetWorktreeId && leaf.worktreeId !== targetWorktreeId) {
          continue
        }
        if (opts.requireFreshPtyLiveness && leaf.ptyId && !refreshedPtyLiveness?.has(leaf.ptyId)) {
          continue
        }
        if (!leaf.ptyId && livePtyWorktreeIds.has(leaf.worktreeId)) {
          continue
        }
        if (leaf.ptyId) {
          ptyIdsFromLeaves.add(leaf.ptyId)
        }
        terminals.push(this.buildTerminalSummary(leaf, worktreesById))
      }
    }

    // Why: worktree.ps can classify active worktrees from PTY records even when
    // the renderer graph is missing a leaf. terminal.list needs the same fallback
    // so mobile does not show a false "No terminals" create flow.
    for (const pty of this.graph.ptysById.values()) {
      if (!pty.connected || ptyIdsFromLeaves.has(pty.ptyId)) {
        continue
      }
      if (opts.requireFreshPtyLiveness && !refreshedPtyLiveness?.has(pty.ptyId)) {
        continue
      }
      if (targetWorktreeId && pty.worktreeId !== targetWorktreeId) {
        continue
      }
      terminals.push(this.buildPtyTerminalSummary(pty, worktreesById))
    }

    const listedTerminals = terminals.slice(0, limit)
    const visualLayouts = this.buildTerminalVisualLayouts(
      listedTerminals,
      worktreesById,
      targetWorktreeId
    )

    return {
      terminals: listedTerminals,
      ...(visualLayouts.length > 0 ? { visualLayouts } : {}),
      totalCount: terminals.length,
      truncated: terminals.length > limit
    }
  }

  private buildTerminalVisualLayouts(
    terminals: RuntimeTerminalSummary[],
    worktreesById: Map<string, ResolvedWorktree>,
    targetWorktreeId: string | null
  ): RuntimeTerminalVisualLayout[] {
    if (terminals.length === 0) {
      return []
    }
    // Why: the mobile/session snapshot supplies topology, but terminal.list
    // must print the same handles in both the flat list and visual tree.
    const summariesByLeafKey = new Map(
      terminals.map((terminal) => [this.getLeafKey(terminal.tabId, terminal.leafId), terminal])
    )
    const summariesByWorktree = new Map<string, RuntimeTerminalSummary[]>()
    for (const terminal of terminals) {
      const existing = summariesByWorktree.get(terminal.worktreeId)
      if (existing) {
        existing.push(terminal)
      } else {
        summariesByWorktree.set(terminal.worktreeId, [terminal])
      }
    }
    const snapshots = targetWorktreeId
      ? [this.mobileSessionTabsByWorktree.get(targetWorktreeId)].filter(
          (snapshot): snapshot is RuntimeMobileSessionTabsSnapshot => snapshot !== undefined
        )
      : [...this.mobileSessionTabsByWorktree.values()]
    const layouts: RuntimeTerminalVisualLayout[] = []
    for (const snapshot of snapshots) {
      const worktreeTerminals = summariesByWorktree.get(snapshot.worktree)
      if (!worktreeTerminals || worktreeTerminals.length === 0) {
        continue
      }
      const groups = this.buildTerminalVisualGroups(snapshot, summariesByLeafKey)
      if (groups.length === 0) {
        continue
      }
      const groupsById = new Map(
        groups
          .filter((group): group is RuntimeTerminalVisualGroupNode & { groupId: string } =>
            Boolean(group.groupId)
          )
          .map((group) => [group.groupId, group])
      )
      const root =
        this.buildTerminalVisualGroupLayout(snapshot.tabGroupLayout, groupsById) ?? groups[0]
      if (!root) {
        continue
      }
      const worktree = worktreesById.get(snapshot.worktree)
      layouts.push({
        worktreeId: snapshot.worktree,
        worktreePath: worktree?.path ?? worktreeTerminals[0]?.worktreePath ?? '',
        root
      })
    }
    return layouts
  }

  private buildTerminalVisualGroups(
    snapshot: RuntimeMobileSessionTabsSnapshot,
    summariesByLeafKey: ReadonlyMap<string, RuntimeTerminalSummary>
  ): RuntimeTerminalVisualGroupNode[] {
    const terminalTabs = snapshot.tabs.filter(
      (tab): tab is RuntimeMobileSessionTerminalTab => tab.type === 'terminal'
    )
    if (terminalTabs.length === 0) {
      return []
    }
    const tabsByParentId = new Map<string, RuntimeMobileSessionTerminalTab[]>()
    const parentOrder: string[] = []
    for (const tab of terminalTabs) {
      const existing = tabsByParentId.get(tab.parentTabId)
      if (existing) {
        existing.push(tab)
      } else {
        parentOrder.push(tab.parentTabId)
        tabsByParentId.set(tab.parentTabId, [tab])
      }
    }
    const groupSources =
      snapshot.tabGroups && snapshot.tabGroups.length > 0
        ? snapshot.tabGroups
        : [{ id: null, activeTabId: snapshot.activeTabId, tabOrder: parentOrder }]
    return groupSources
      .map((group): RuntimeTerminalVisualGroupNode | null => {
        const tabs = group.tabOrder
          .map((tabId) => {
            const surfaces =
              tabsByParentId.get(tabId) ?? terminalTabs.filter((tab) => tab.id === tabId)
            return this.buildTerminalVisualTab(tabId, surfaces, summariesByLeafKey)
          })
          .filter((tab): tab is RuntimeTerminalVisualTab => tab !== null)
        if (tabs.length === 0) {
          return null
        }
        return {
          type: 'group',
          groupId: group.id,
          activeTabId: group.activeTabId,
          tabs
        }
      })
      .filter((group): group is RuntimeTerminalVisualGroupNode => group !== null)
  }

  private buildTerminalVisualTab(
    tabId: string,
    surfaces: RuntimeMobileSessionTerminalTab[],
    summariesByLeafKey: ReadonlyMap<string, RuntimeTerminalSummary>
  ): RuntimeTerminalVisualTab | null {
    const firstSurface = surfaces[0]
    if (!firstSurface) {
      return null
    }
    const parentTabId = firstSurface.parentTabId
    const requestedActiveLeafId =
      firstSurface.parentLayout?.activeLeafId ??
      surfaces.find((surface) => surface.isActive)?.leafId ??
      firstSurface.leafId
    const root = firstSurface.parentLayout?.root ?? {
      type: 'leaf' as const,
      leafId: firstSurface.leafId
    }
    const visibleLeafIds = this.collectVisibleTerminalLeafIds(root, parentTabId, summariesByLeafKey)
    if (visibleLeafIds.length === 0) {
      return null
    }
    const activeLeafId =
      (requestedActiveLeafId && visibleLeafIds.includes(requestedActiveLeafId)
        ? requestedActiveLeafId
        : surfaces.find((surface) => surface.isActive && visibleLeafIds.includes(surface.leafId))
            ?.leafId) ?? visibleLeafIds[0]!
    const panes = this.buildTerminalVisualPane(root, parentTabId, activeLeafId, summariesByLeafKey)
    if (!panes) {
      return null
    }
    return {
      tabId: parentTabId || tabId,
      title: this.graph.tabs.get(parentTabId)?.title ?? firstSurface.title ?? null,
      activeLeafId,
      panes
    }
  }

  private collectVisibleTerminalLeafIds(
    node: TerminalPaneLayoutNode,
    tabId: string,
    summariesByLeafKey: ReadonlyMap<string, RuntimeTerminalSummary>
  ): string[] {
    if (node.type === 'leaf') {
      return summariesByLeafKey.has(this.getLeafKey(tabId, node.leafId)) ? [node.leafId] : []
    }
    return [
      ...this.collectVisibleTerminalLeafIds(node.first, tabId, summariesByLeafKey),
      ...this.collectVisibleTerminalLeafIds(node.second, tabId, summariesByLeafKey)
    ]
  }

  private buildTerminalVisualPane(
    node: TerminalPaneLayoutNode,
    tabId: string,
    activeLeafId: string | null,
    summariesByLeafKey: ReadonlyMap<string, RuntimeTerminalSummary>
  ): RuntimeTerminalVisualPaneNode | null {
    if (node.type === 'leaf') {
      const summary = summariesByLeafKey.get(this.getLeafKey(tabId, node.leafId))
      if (!summary) {
        return null
      }
      return {
        type: 'terminal',
        handle: summary.handle,
        tabId: summary.tabId,
        leafId: summary.leafId,
        title: summary.title,
        connected: summary.connected,
        active: summary.leafId === activeLeafId
      }
    }
    const first = this.buildTerminalVisualPane(node.first, tabId, activeLeafId, summariesByLeafKey)
    const second = this.buildTerminalVisualPane(
      node.second,
      tabId,
      activeLeafId,
      summariesByLeafKey
    )
    if (first && second) {
      return { type: 'pane-split', direction: node.direction, first, second }
    }
    return first ?? second
  }

  private buildTerminalVisualGroupLayout(
    node: TabGroupLayoutNode | null | undefined,
    groupsById: ReadonlyMap<string, RuntimeTerminalVisualGroupNode>
  ): RuntimeTerminalVisualLayoutNode | null {
    if (!node) {
      return null
    }
    if (node.type === 'leaf') {
      return groupsById.get(node.groupId) ?? null
    }
    const first = this.buildTerminalVisualGroupLayout(node.first, groupsById)
    const second = this.buildTerminalVisualGroupLayout(node.second, groupsById)
    if (first && second) {
      return { type: 'split', direction: node.direction, first, second }
    }
    return first ?? second
  }

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
  private refreshPtyForegroundAgent(ptyId: string): void {
    void this.refreshPtyForegroundAgentFromController(ptyId)
  }

  private getPendingForegroundAgentRefreshForTitle(
    ptyId: string,
    titleObservedAt: number
  ): Promise<boolean> | undefined {
    if (!this.ptyForegroundAgentRefreshes.has(ptyId)) {
      return undefined
    }
    return this.refreshPtyForegroundAgentFromController(ptyId, {
      afterTitleObservation: titleObservedAt
    })
  }

  private delayPtyBackedMobileSnapshotForForegroundAgent(
    ptyId: string,
    titleObservedAt: number,
    foregroundRefresh: Promise<boolean>
  ): void {
    this.ptyDelayedForegroundSnapshotTitleObservations.set(ptyId, titleObservedAt)
    void foregroundRefresh.then((foregroundAgentChanged) => {
      if (this.ptyDelayedForegroundSnapshotTitleObservations.get(ptyId) !== titleObservedAt) {
        return
      }
      this.ptyDelayedForegroundSnapshotTitleObservations.delete(ptyId)
      if (!foregroundAgentChanged) {
        this.touchMobileSessionSnapshotsForPty(ptyId)
      }
    })
  }

  /**
   * Deduplicates and manages in-flight foreground agent refresh queries
   * for a specific PTY.
   */
  private refreshPtyForegroundAgentFromController(
    ptyId: string,
    options: { afterTitleObservation?: number } = {}
  ): Promise<boolean> {
    const startedAfterTitleObservation = options.afterTitleObservation ?? 0
    const pendingRefresh = this.ptyForegroundAgentRefreshes.get(ptyId)
    if (pendingRefresh) {
      pendingRefresh.requestedAfterTitleObservation = Math.max(
        pendingRefresh.requestedAfterTitleObservation,
        startedAfterTitleObservation
      )
      return pendingRefresh.promise
    }
    const entry: PtyForegroundAgentRefresh = {
      promise: Promise.resolve(false),
      startedAfterTitleObservation,
      requestedAfterTitleObservation: startedAfterTitleObservation
    }
    const refresh = (async (): Promise<boolean> => {
      while (true) {
        entry.startedAfterTitleObservation = entry.requestedAfterTitleObservation
        const foregroundAgentChanged = await this.loadPtyForegroundAgentFromController(ptyId)
        if (
          foregroundAgentChanged ||
          entry.requestedAfterTitleObservation <= entry.startedAfterTitleObservation
        ) {
          return foregroundAgentChanged
        }
      }
    })().finally(() => {
      if (this.ptyForegroundAgentRefreshes.get(ptyId) === entry) {
        this.ptyForegroundAgentRefreshes.delete(ptyId)
      }
    })
    entry.promise = refresh
    this.ptyForegroundAgentRefreshes.set(ptyId, entry)
    return refresh
  }

  /**
   * Queries the PTY controller for the active foreground process, identifies if it
   * is a recognized agent, and updates the PTY's foreground agent state if changed.
   */
  private async loadPtyForegroundAgentFromController(ptyId: string): Promise<boolean> {
    if (!this.ptyController) {
      return false
    }
    const pty = this.graph.ptysById.get(ptyId)
    if (!pty?.connected) {
      return false
    }
    // Why: foregroundAgent is only consulted as the owner fallback when
    // launchAgent is unknown, so a known launchAgent makes the relay
    // getForegroundProcess round-trip pure waste (covers all launched agents).
    if (pty.launchAgent) {
      return false
    }
    let foregroundProcess: string | null
    try {
      foregroundProcess = await this.ptyController.getForegroundProcess(ptyId)
    } catch {
      return false
    }
    const foregroundAgent = foregroundProcess
      ? (recognizeAgentProcess(foregroundProcess)?.agent ?? null)
      : null
    if (pty.foregroundAgent === foregroundAgent) {
      return false
    }
    pty.foregroundAgent = foregroundAgent
    this.touchMobileSessionSnapshotsForPty(ptyId)
    return true
  }

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

  async getWorktreePs(limit = DEFAULT_WORKTREE_PS_LIMIT): Promise<{
    worktrees: RuntimeWorktreePsSummary[]
    totalCount: number
    truncated: boolean
  }> {
    if (!Number.isInteger(limit) || limit <= 0) {
      throw new Error('invalid_limit')
    }
    const resolvedWorktrees = (await this.listResolvedWorktrees()).filter((worktree) =>
      this.isRuntimeWorktreeVisible(worktree)
    )
    // Why: worktree.ps backs the mobile sidebar, so it must use the same
    // host-owned imported-worktree visibility gate as worktree.list/desktop.
    await this.refreshPtyWorktreeRecordsFromController(resolvedWorktrees)
    const repoById = new Map((this.store?.getRepos() ?? []).map((repo) => [repo.id, repo]))
    const summaries = new Map<string, RuntimeWorktreePsSummary>()

    // Why: the GitHub cache is keyed by `repoPath::branch` (no refs/heads/ prefix),
    // matching how the renderer's fetchPRForBranch stores entries. We look up cached
    // PR info so mobile clients can group worktrees by PR state without making
    // expensive `gh` CLI calls. Falls back to meta.linkedPR if no cache entry exists.
    const ghCache = this.store?.getGitHubCache?.()
    for (const worktree of resolvedWorktrees) {
      const meta =
        this.store?.getWorktreeMeta?.(worktree.id) ?? this.store?.getAllWorktreeMeta()[worktree.id]
      const repo = repoById.get(worktree.repoId)
      let linkedPR: { number: number; state: string } | null = null
      const branch = worktree.branch.replace(/^refs\/heads\//, '')
      if (branch && ghCache) {
        // Why: the renderer keys the PR cache by `repoId::branch` (getGitHubPRCacheKey
        // prefers repo.id over repo.path), so read by id first and fall back to path
        // for legacy/path-keyed entries. Reading only by path missed every cached
        // entry, leaving mobile's linked-PR badge stuck on the 'unknown' fallback.
        const cached =
          (repo?.id ? ghCache.pr[`${repo.id}::${branch}`] : undefined) ??
          (repo?.path ? ghCache.pr[`${repo.path}::${branch}`] : undefined)
        if (cached?.data) {
          linkedPR = { number: cached.data.number, state: cached.data.state }
        }
      }
      if (!linkedPR && meta?.linkedPR != null) {
        linkedPR = { number: meta.linkedPR, state: 'unknown' }
      }
      const terminalPlatform = repo ? this.getAgentLaunchPlatformForRepo(repo) : process.platform
      // Why: use the instance-validated lineage from attachLineageToResolvedWorktrees,
      // not the raw store entry — shipped mobile clients trust parentWorktreeId as-is,
      // so a stale same-path entry would nest replacement checkouts under old parents.
      const lineage = worktree.lineage
      summaries.set(worktree.id, {
        // Why: mobile mirrors desktop workspace grouping/order from persisted
        // metadata, while older runtimes may not have hydrated every field yet.
        workspaceKind: 'git',
        worktreeId: worktree.id,
        repoId: worktree.repoId,
        ...((worktree.hostId ?? meta?.hostId) ? { hostId: worktree.hostId ?? meta?.hostId } : {}),
        terminalPlatform,
        repo: repo?.displayName ?? worktree.repoId,
        path: worktree.path,
        branch: worktree.branch,
        isArchived: worktree.isArchived,
        isMainWorktree: worktree.isMainWorktree,
        hasHostSidebarActivity: false,
        ...(worktree.instanceId !== undefined ? { worktreeInstanceId: worktree.instanceId } : {}),
        ...(lineage?.worktreeInstanceId !== undefined
          ? { lineageWorktreeInstanceId: lineage.worktreeInstanceId }
          : {}),
        ...(lineage?.parentWorktreeInstanceId !== undefined
          ? { parentWorktreeInstanceId: lineage.parentWorktreeInstanceId }
          : {}),
        parentWorktreeId: worktree.parentWorktreeId,
        childWorktreeIds: worktree.childWorktreeIds,
        displayName: worktree.displayName,
        workspaceStatus: meta?.workspaceStatus ?? DEFAULT_WORKSPACE_STATUS_ID,
        sortOrder: meta?.sortOrder ?? 0,
        ...(meta?.manualOrder !== undefined ? { manualOrder: meta.manualOrder } : {}),
        lastActivityAt: worktree.lastActivityAt,
        ...(worktree.createdAt !== undefined ? { createdAt: worktree.createdAt } : {}),
        linkedIssue: worktree.linkedIssue,
        linkedPR,
        linkedLinearIssue: meta?.linkedLinearIssue ?? null,
        linkedGitLabMR: meta?.linkedGitLabMR ?? null,
        linkedGitLabIssue: meta?.linkedGitLabIssue ?? null,
        comment: meta?.comment ?? '',
        isPinned: meta?.isPinned ?? false,
        isActive: false,
        unread: meta?.isUnread ?? false,
        liveTerminalCount: 0,
        hasAttachedPty: false,
        lastOutputAt: null,
        preview: '',
        status: 'inactive',
        agents: []
      })
    }

    const projectGroupById = new Map(
      (this.store?.getProjectGroups?.() ?? []).map((group) => [group.id, group])
    )
    for (const folderWorkspace of this.store?.getFolderWorkspaces?.() ?? []) {
      const projectGroup = projectGroupById.get(folderWorkspace.projectGroupId)
      if (!projectGroup?.parentPath) {
        continue
      }
      const worktree = folderWorkspaceToWorktree(folderWorkspace)
      summaries.set(worktree.id, {
        // Why: folder workspaces use the same mobile grouping/order contract as
        // git worktrees, but legacy records may be missing order metadata.
        workspaceKind: 'folder-workspace',
        worktreeId: worktree.id,
        repoId: worktree.repoId,
        repo: projectGroup.name,
        path: worktree.path,
        branch: worktree.branch,
        isArchived: worktree.isArchived,
        isMainWorktree: worktree.isMainWorktree,
        hasHostSidebarActivity: false,
        ...(worktree.instanceId !== undefined ? { worktreeInstanceId: worktree.instanceId } : {}),
        parentWorktreeId: null,
        childWorktreeIds: [],
        displayName: worktree.displayName,
        workspaceStatus: worktree.workspaceStatus ?? DEFAULT_WORKSPACE_STATUS_ID,
        sortOrder: worktree.sortOrder ?? 0,
        ...(worktree.manualOrder !== undefined ? { manualOrder: worktree.manualOrder } : {}),
        lastActivityAt: worktree.lastActivityAt,
        ...(worktree.createdAt !== undefined ? { createdAt: worktree.createdAt } : {}),
        linkedIssue: worktree.linkedIssue ?? null,
        linkedPR: null,
        linkedLinearIssue: worktree.linkedLinearIssue ?? null,
        linkedGitLabMR: worktree.linkedGitLabMR ?? null,
        linkedGitLabIssue: worktree.linkedGitLabIssue ?? null,
        comment: worktree.comment,
        isPinned: worktree.isPinned,
        isActive: false,
        unread: worktree.isUnread,
        liveTerminalCount: 0,
        hasAttachedPty: false,
        lastOutputAt: null,
        preview: '',
        status: 'inactive',
        agents: []
      })
    }

    const countedPtyIds = new Set<string>()
    for (const leaf of this.graph.leaves.values()) {
      const summary = this.getSummaryForRuntimeWorktreeId(
        summaries,
        resolvedWorktrees,
        leaf.worktreeId
      )
      if (!summary) {
        continue
      }
      if (leaf.ptyId) {
        countedPtyIds.add(leaf.ptyId)
      }
      if (leaf.ptyId && leaf.connected) {
        summary.hasHostSidebarActivity = true
      }
      const previousLastOutputAt = summary.lastOutputAt
      summary.liveTerminalCount += 1
      summary.hasAttachedPty = summary.hasAttachedPty || leaf.connected
      summary.lastOutputAt = maxTimestamp(summary.lastOutputAt, leaf.lastOutputAt)
      summary.status = mergeWorktreeStatus(
        summary.status,
        getLeafWorktreeStatus(leaf, this.graph.tabs.get(leaf.tabId)?.title ?? null)
      )
      if (
        leaf.preview &&
        (summary.preview.length === 0 || (leaf.lastOutputAt ?? -1) >= (previousLastOutputAt ?? -1))
      ) {
        summary.preview = leaf.preview
      }
    }

    for (const pty of this.graph.ptysById.values()) {
      if (!pty.connected || countedPtyIds.has(pty.ptyId)) {
        continue
      }
      const summary = this.getSummaryForRuntimeWorktreeId(
        summaries,
        resolvedWorktrees,
        pty.worktreeId
      )
      if (!summary) {
        continue
      }
      const previousLastOutputAt = summary.lastOutputAt
      summary.liveTerminalCount += 1
      summary.hasAttachedPty = true
      summary.lastOutputAt = maxTimestamp(summary.lastOutputAt, pty.lastOutputAt)
      summary.status = mergeWorktreeStatus(summary.status, 'active')
      if (
        pty.preview &&
        (summary.preview.length === 0 || (pty.lastOutputAt ?? -1) >= (previousLastOutputAt ?? -1))
      ) {
        summary.preview = pty.preview
      }
    }

    const session = this.store?.getWorkspaceSession?.()
    for (const [worktreeId, tabs] of Object.entries(session?.tabsByWorktree ?? {})) {
      if (tabs.length === 0) {
        continue
      }
      const summary = this.getSummaryForRuntimeWorktreeId(summaries, resolvedWorktrees, worktreeId)
      if (!summary) {
        continue
      }
      // Why: desktop can show terminal tabs that are not mounted as renderer
      // leaves and are not currently visible in the PTY provider list. Mobile
      // still needs those worktrees to show as terminal-bearing entries.
      summary.liveTerminalCount = Math.max(summary.liveTerminalCount, tabs.length)
      summary.hasAttachedPty = summary.hasAttachedPty || tabs.some((tab) => tab.ptyId !== null)
      if (tabs.some((tab) => tab.ptyId !== null && this.graph.ptysById.get(tab.ptyId)?.connected)) {
        summary.hasHostSidebarActivity = true
      }
      for (const tab of tabs) {
        summary.status = mergeWorktreeStatus(
          summary.status,
          getSavedTabWorktreeStatus(tab.title, tab.ptyId !== null)
        )
      }
    }

    // Why: surface the desktop's focused worktree so mobile can scroll it into
    // view and highlight it. Resolve through getSummaryForRuntimeWorktreeId so
    // SSH/remote path-projected ids match the same way tabsByWorktree does.
    if (session?.activeWorktreeId) {
      const activeSummary = this.getSummaryForRuntimeWorktreeId(
        summaries,
        resolvedWorktrees,
        session.activeWorktreeId
      )
      if (activeSummary) {
        activeSummary.isActive = true
      }
    }

    this.attachAgentRowsToSummaries(summaries)

    const sorted = [...summaries.values()].sort(compareWorktreePs)
    return {
      worktrees: sorted.slice(0, limit),
      totalCount: sorted.length,
      truncated: sorted.length > limit
    }
  }

  // Why: maps the retained per-pane agent snapshots into each worktree's inline
  // agent list, mirroring the desktop sidebar. Lineage parent is resolved from
  // the orchestration db (paneKey-keyed), not the OSC payload, since spawn
  // hierarchy is pane-level state tracked separately from terminal output.
  private attachAgentRowsToSummaries(summaries: Map<string, RuntimeWorktreePsSummary>): void {
    // Why: most agents report via hooks (agent-hooks/server), not OSC, so the
    // hook snapshot is the primary source — same one the desktop sidebar reads.
    // OSC-only entries (no hook) are merged in as a fallback, keyed by paneKey.
    const rowSources = new Map<
      string,
      {
        paneKey: string
        worktreeId?: string
        state: ParsedAgentStatusPayload['state']
        agentType: string | null
        prompt: string
        lastAssistantMessage: string | null
        toolName: string | null
        toolInput: string | null
        interrupted: boolean
        stateStartedAt: number
        updatedAt: number
      }
    >()
    for (const snapshot of this.latestAgentStatusByPaneKey.values()) {
      const { payload } = snapshot
      rowSources.set(snapshot.paneKey, {
        paneKey: snapshot.paneKey,
        worktreeId: snapshot.worktreeId,
        state: payload.state,
        agentType: payload.agentType ?? null,
        prompt: payload.prompt,
        lastAssistantMessage: payload.lastAssistantMessage ?? null,
        toolName: payload.toolName ?? null,
        toolInput: payload.toolInput ?? null,
        interrupted: payload.interrupted ?? false,
        stateStartedAt: snapshot.stateStartedAt,
        updatedAt: snapshot.updatedAt
      })
    }
    for (const entry of this.getAgentStatusSnapshotFn?.() ?? []) {
      rowSources.set(entry.paneKey, {
        paneKey: entry.paneKey,
        worktreeId: entry.worktreeId,
        state: entry.state,
        agentType: entry.agentType ?? null,
        prompt: entry.prompt,
        lastAssistantMessage: entry.lastAssistantMessage ?? null,
        toolName: entry.toolName ?? null,
        toolInput: entry.toolInput ?? null,
        interrupted: entry.interrupted ?? false,
        stateStartedAt: entry.stateStartedAt,
        updatedAt: entry.receivedAt
      })
    }
    if (rowSources.size === 0) {
      return
    }
    const orchestrationByPaneKey = this.buildAgentOrchestrationByPaneKey()
    const rowsByWorktree = new Map<string, RuntimeWorktreeAgentRow[]>()
    for (const src of rowSources.values()) {
      const worktreeId = src.worktreeId
      if (!worktreeId || !summaries.has(worktreeId)) {
        continue
      }
      const taskTitle = orchestrationByPaneKey?.[src.paneKey]?.taskTitle ?? null
      const displayName = orchestrationByPaneKey?.[src.paneKey]?.displayName ?? null
      const row: RuntimeWorktreeAgentRow = {
        paneKey: src.paneKey,
        parentPaneKey: orchestrationByPaneKey?.[src.paneKey]?.parentPaneKey ?? null,
        state: src.state,
        agentType: src.agentType,
        prompt: src.prompt,
        taskTitle,
        displayName,
        lastAssistantMessage: src.lastAssistantMessage,
        toolName: src.toolName,
        toolInput: src.toolInput,
        interrupted: src.interrupted,
        stateStartedAt: src.stateStartedAt,
        updatedAt: src.updatedAt
      }
      const rows = rowsByWorktree.get(worktreeId)
      if (rows) {
        rows.push(row)
      } else {
        rowsByWorktree.set(worktreeId, [row])
      }
    }
    for (const [worktreeId, rows] of rowsByWorktree) {
      // Oldest-started first, matching the desktop dashboard's start-order sort.
      rows.sort((a, b) => a.stateStartedAt - b.stateStartedAt)
      const summary = summaries.get(worktreeId)
      if (summary) {
        summary.agents = rows
      }
    }
  }

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
    getRecentPtyOutput: (ptyId) => this.recentPtyOutputById.get(ptyId)
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

  async createMobileSessionTerminal(
    worktreeSelector: string,
    opts: {
      afterTabId?: string
      targetGroupId?: string
      command?: string
      cwd?: string
      env?: Record<string, string>
      startupCommandDelivery?: WorktreeStartupLaunch['startupCommandDelivery']
      agent?: TuiAgent
      launchConfig?: SleepingAgentLaunchConfig
      launchAgent?: TuiAgent
      activate?: boolean
      clientMutationId?: string
      signal?: AbortSignal
    } = {}
  ): Promise<RuntimeMobileSessionCreateTerminalResult> {
    const mutationId = opts.clientMutationId
    if (!mutationId) {
      return this.runCreateMobileSessionTerminal(worktreeSelector, opts)
    }
    const mutationKey = `${worktreeSelector}\0${mutationId}`
    // Why: a retried create (double-tap, reconnect replay) with the same
    // idempotency key must return the in-flight operation instead of spawning a
    // duplicate terminal. Successes are kept briefly so a retry whose response
    // was lost in transit reuses the created terminal; failures are dropped
    // immediately so a retry can start a fresh create.
    const inflight = this.mobileTerminalCreateByMutationId.get(mutationKey)
    if (inflight) {
      return inflight
    }
    const run = this.runCreateMobileSessionTerminal(worktreeSelector, opts)
    this.mobileTerminalCreateByMutationId.set(mutationKey, run)
    const drop = (): void => {
      if (this.mobileTerminalCreateByMutationId.get(mutationKey) === run) {
        this.mobileTerminalCreateByMutationId.delete(mutationKey)
      }
    }
    void run.then(() => {
      setTimeout(drop, MOBILE_TERMINAL_CREATE_RESULT_TTL_MS).unref?.()
    }, drop)
    return run
  }

  private async runCreateMobileSessionTerminal(
    worktreeSelector: string,
    opts: {
      afterTabId?: string
      targetGroupId?: string
      command?: string
      cwd?: string
      env?: Record<string, string>
      startupCommandDelivery?: WorktreeStartupLaunch['startupCommandDelivery']
      agent?: TuiAgent
      launchConfig?: SleepingAgentLaunchConfig
      launchAgent?: TuiAgent
      activate?: boolean
      clientMutationId?: string
      signal?: AbortSignal
    } = {}
  ): Promise<RuntimeMobileSessionCreateTerminalResult> {
    this.assertGraphReady()
    const workspace = await this.resolveTerminalWorkspaceLaunchScope(worktreeSelector)
    const worktreeId = workspace.id
    const cwd = this.resolveWorkspaceTerminalStartupCwd(workspace, opts.cwd)
    this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(worktreeId)
    let afterDesktopTabId: string | undefined
    if (opts.afterTabId) {
      const snapshot = this.mobileSessionTabsByWorktree.get(worktreeId)
      const anchor = snapshot?.tabs.find((tab) => tab.id === opts.afterTabId)
      if (!anchor) {
        throw new Error('after_tab_not_found')
      }
      afterDesktopTabId = anchor.type === 'terminal' ? anchor.parentTabId : anchor.id
    }
    const startupCommand = await this.resolveMobileSessionTerminalCommand(workspace, opts)

    const win = this.getAvailableAuthoritativeWindow()
    if (!win) {
      return await this.createHeadlessMobileSessionTerminal(
        worktreeId,
        opts.activate !== false,
        opts.afterTabId,
        {
          command: startupCommand.command,
          cwd,
          env: startupCommand.env,
          startupCommandDelivery: startupCommand.startupCommandDelivery,
          launchAgent: startupCommand.launchAgent,
          targetGroupId: opts.targetGroupId,
          launchConfig: startupCommand.launchConfig
        }
      )
    }
    const requestId = randomUUID()
    const reply = await new Promise<{ tabId: string; title: string }>((resolve, reject) => {
      const timer = setTimeout(() => {
        ipcMain.removeListener('terminal:tabCreateReply', handler)
        opts.signal?.removeEventListener('abort', onAbort)
        reject(new Error('Terminal creation timed out'))
      }, 10_000)
      // Why: a dead client connection cancels the wait; the renderer tab (and
      // its shell) stays alive for the host and mirrors on reconnect (#7718).
      const onAbort = (): void => {
        clearTimeout(timer)
        ipcMain.removeListener('terminal:tabCreateReply', handler)
        reject(new Error('client_disconnected'))
      }

      const handler = (
        event: Electron.IpcMainEvent,
        r: { requestId: string; tabId?: string; title?: string; error?: string }
      ): void => {
        if (event.sender !== win.webContents || r.requestId !== requestId) {
          return
        }
        clearTimeout(timer)
        ipcMain.removeListener('terminal:tabCreateReply', handler)
        opts.signal?.removeEventListener('abort', onAbort)
        if (r.error) {
          reject(new Error(r.error))
        } else {
          resolve({ tabId: r.tabId!, title: r.title ?? '' })
        }
      }
      opts.signal?.addEventListener('abort', onAbort, { once: true })
      ipcMain.on('terminal:tabCreateReply', handler)
      win.webContents.send('terminal:requestTabCreate', {
        requestId,
        worktreeId,
        afterTabId: afterDesktopTabId,
        targetGroupId: opts.targetGroupId,
        command: startupCommand.command,
        cwd,
        ...(startupCommand.env ? { env: startupCommand.env } : {}),
        ...(startupCommand.launchConfig ? { launchConfig: startupCommand.launchConfig } : {}),
        ...(startupCommand.launchAgent ? { launchAgent: startupCommand.launchAgent } : {}),
        startupCommandDelivery: startupCommand.startupCommandDelivery,
        source: 'runtime-session',
        activate: opts.activate
      })
    })

    if (opts.activate !== false) {
      this.notifier?.focusTerminal(reply.tabId, worktreeId, null)
    }
    // Why: register the wait before the renderer's PTY spawn arrives so that
    // spawn (registerPty) can publish the pty-backed surface main-side even if
    // graph-sync is stalled (#7587). Removed in the finally below.
    const pendingCreateKey = `${worktreeId}::${reply.tabId}`
    // Why: a rescue publishes into the active group (opts.targetGroupId is not
    // threaded); the renderer's reconciling publication then moves the tab to the
    // requested group, so any wrong-group placement is cosmetic and stall-window-only.
    this.pendingMobileTerminalCreatesByKey.set(pendingCreateKey, {
      activate: opts.activate !== false,
      selectIfNoActiveTab: true
    })
    try {
      // Why: the PTY spawn and the tabCreate reply race on independent IPC
      // channels; if the spawn already registered, publish immediately so the
      // wait resolves without depending on a graph sync.
      this.ensurePtyBackedMobileSurfaceForRendererTab(worktreeId, reply.tabId)
      const surface = await this.waitForMobileTerminalSurface(worktreeId, reply.tabId, {
        timeoutMs: MOBILE_TERMINAL_SURFACE_TIMEOUT_MS,
        signal: opts.signal
      })
      if (this.isReadyMobileTerminalSurface(surface)) {
        return surface
      }
      const readySurface = await this.waitForMobileTerminalSurface(worktreeId, reply.tabId, {
        timeoutMs: MOBILE_TERMINAL_READY_FALLBACK_MS,
        requireReady: true,
        signal: opts.signal
      }).catch(() => null)
      if (readySurface) {
        return readySurface
      }
      if (opts.signal?.aborted) {
        // Why: nobody is waiting for this create anymore; do not materialize
        // or roll back — the renderer's own publication settles the tab.
        throw new Error('client_disconnected')
      }
      const pendingSurface = this.findMobileTerminalSurface(worktreeId, reply.tabId)
      if (!pendingSurface) {
        throw new Error('Timed out waiting for terminal surface after creation')
      }
      // Why: hidden/occluded renderer windows can publish the tab shell before
      // TerminalPane mounts and spawns the PTY. Materialize into the same
      // identity so later renderer focus adopts instead of creating another tab.
      return await this.createHeadlessMobileSessionTerminal(
        worktreeId,
        opts.activate !== false,
        opts.afterTabId,
        {
          command: startupCommand.command,
          cwd,
          env: startupCommand.env,
          startupCommandDelivery: startupCommand.startupCommandDelivery,
          identity: { tabId: pendingSurface.tab.parentTabId, leafId: pendingSurface.tab.leafId },
          launchAgent: startupCommand.launchAgent,
          targetGroupId: opts.targetGroupId,
          launchConfig: startupCommand.launchConfig
        }
      )
    } catch (error) {
      // Why: publication latency (throttled/hidden renderer), not spawn failure,
      // can trip the surface timeout. Rescue only when a live PTY actually backs
      // the tab — gating on a surface would let a handle-less shell (or a failed
      // materialize) resolve as success and skip the ghost-tab rollback (#7587).
      if (this.findLiveRegisteredPtyForRendererTab(worktreeId, reply.tabId)) {
        const rescued = this.ensurePtyBackedMobileSurfaceForRendererTab(worktreeId, reply.tabId)
        if (rescued) {
          return rescued
        }
      }
      // Why: don't roll back when (a) the client connection died — the wait
      // was cancelled, not the spawn — or (b) a live shell already backs the
      // tab (its pane key may simply not be registered yet). Killing a real
      // terminal the host user can see is the "tab dies after ~10s" bug (#7718).
      if (
        isClientDisconnectedError(error) ||
        this.hasLiveShellForRendererTab(worktreeId, reply.tabId)
      ) {
        throw error
      }
      // Why: the renderer created the tab but no live PTY backs it (true PTY
      // spawn/handle failure). Roll the half-created tab back via the renderer
      // close path so it can't linger as a ghost in mobile snapshots, then
      // surface the failure to the caller.
      this.notifier?.closeTerminal(reply.tabId)
      throw error
    } finally {
      this.pendingMobileTerminalCreatesByKey.delete(pendingCreateKey)
    }
  }

  private async resolveMobileSessionTerminalCommand(
    workspace: TerminalWorkspaceLaunchScope,
    opts: {
      command?: string
      env?: Record<string, string>
      startupCommandDelivery?: WorktreeStartupLaunch['startupCommandDelivery']
      agent?: TuiAgent
      launchConfig?: SleepingAgentLaunchConfig
      launchAgent?: TuiAgent
    }
  ): Promise<{
    command?: string
    env?: Record<string, string>
    startupCommandDelivery?: WorktreeStartupLaunch['startupCommandDelivery']
    launchConfig?: SleepingAgentLaunchConfig
    launchAgent?: TuiAgent
  }> {
    if (opts.command || !opts.agent) {
      return {
        command: opts.command,
        env: opts.env,
        launchConfig: opts.launchConfig,
        launchAgent: opts.launchAgent,
        startupCommandDelivery: opts.startupCommandDelivery
      }
    }
    if (!this.store) {
      throw new Error('runtime_unavailable')
    }
    const settings = this.store.getSettings()
    if (!isTuiAgentEnabled(opts.agent, settings.disabledTuiAgents)) {
      throw new Error('Selected agent is disabled. Choose an enabled agent before creating.')
    }
    // Why: mobile may be running on iOS while the actual terminal shell is
    // Windows/macOS/Linux or an SSH Linux host; quote for the host shell.
    const platform = this.getAgentLaunchPlatformForWorkspace(workspace)
    // Why: an SSH workspace runs the CLI through the relay shim (plain `orca`),
    // so the Linux-only `orca-ide` rename must not be applied.
    const isRemote = workspace.repo ? repoIsRemote(workspace.repo) : repoIsRemote(workspace)
    const queuedShell = resolveLocalWindowsAgentStartupShell({
      platform,
      isRemote,
      terminalWindowsShell: settings.terminalWindowsShell
    })
    const startupPlan = buildAgentStartupPlan({
      agent: opts.agent,
      prompt: '',
      cmdOverrides: settings.agentCmdOverrides ?? {},
      agentArgs: resolveTuiAgentLaunchArgs(opts.agent, settings.agentDefaultArgs),
      agentEnv: resolveTuiAgentLaunchEnv(opts.agent, settings.agentDefaultEnv),
      platform,
      shell: queuedShell,
      isRemote,
      allowEmptyPromptLaunch: true
    })
    if (!startupPlan) {
      throw new Error(`Could not build launch command for ${opts.agent}.`)
    }
    if (workspace.connectionId) {
      await this.markRemoteWorkspaceTrustedForAgent(
        opts.agent,
        workspace.connectionId,
        workspace.path
      )
    } else {
      this.markLocalWorkspaceTrustedForAgent(opts.agent, workspace.path)
    }
    return {
      command: startupPlan.launchCommand,
      env: startupPlan.env,
      launchConfig: startupPlan.launchConfig,
      launchAgent: opts.agent,
      startupCommandDelivery: startupPlan.startupCommandDelivery
    }
  }

  private async createHeadlessMobileSessionTerminal(
    worktreeId: string,
    activate: boolean,
    afterTabId?: string,
    opts: {
      command?: string
      cwd?: string
      env?: Record<string, string>
      startupCommandDelivery?: WorktreeStartupLaunch['startupCommandDelivery']
      identity?: { tabId: string; leafId: string; sessionId?: string }
      launchAgent?: TuiAgent
      targetGroupId?: string
      launchConfig?: SleepingAgentLaunchConfig
    } = {}
  ): Promise<RuntimeMobileSessionCreateTerminalResult> {
    const workspace = await this.resolveTerminalWorkspaceLaunchScope(`id:${worktreeId}`)
    const cwd = this.resolveWorkspaceTerminalStartupCwd(workspace, opts.cwd)
    // Why: SshPtyProvider treats sessionId as a relay reattach request. Only
    // synthesize local serve ids; SSH fresh terminals must call pty.spawn.
    const stableSessionId =
      opts.identity?.sessionId ?? (workspace.connectionId ? undefined : `serve-${randomUUID()}`)
    const terminal = await this.createTerminal(`id:${worktreeId}`, {
      focus: false,
      command: opts.command,
      cwd,
      env: opts.env,
      ...(opts.launchConfig ? { launchConfig: opts.launchConfig } : {}),
      ...(opts.launchAgent ? { launchAgent: opts.launchAgent } : {}),
      startupCommandDelivery: opts.startupCommandDelivery,
      ...(opts.identity
        ? {
            tabId: opts.identity.tabId,
            leafId: opts.identity.leafId,
            ...(stableSessionId ? { sessionId: stableSessionId } : {})
          }
        : stableSessionId
          ? { sessionId: stableSessionId }
          : {}),
      persistHostSessionBinding: true,
      // Why: this method publishes the authoritative snapshot (with the target
      // group) below; skip the intermediate publish to avoid a wrong-group flash.
      deferMobileSessionPublish: true
    })
    const livePty = this.getLivePtyForHandle(terminal.handle)
    if (!livePty) {
      throw new Error('terminal_handle_stale')
    }
    const parentTabId = livePty.pty.tabId ?? `pty:${livePty.pty.ptyId}`
    const leafId = parsePaneKey(livePty.pty.paneKey ?? '')?.leafId ?? randomUUID()
    const existing = this.mobileSessionTabsByWorktree.get(worktreeId)
    const existingSurface =
      existing?.tabs.find(
        (candidate): candidate is RuntimeMobileSessionTerminalTab =>
          candidate.type === 'terminal' &&
          candidate.parentTabId === parentTabId &&
          candidate.leafId === leafId
      ) ?? null
    const parentLayout = this.buildMaterializedHeadlessParentLayout(
      leafId,
      livePty.pty.ptyId,
      existingSurface?.parentLayout
    )
    const tab: RuntimeMobileSessionTerminalTab = {
      type: 'terminal',
      id: `${parentTabId}::${leafId}`,
      parentTabId,
      leafId,
      ptyId: livePty.pty.ptyId,
      title: terminal.title ?? livePty.pty.title ?? 'Terminal',
      ...(cwd ? { startupCwd: cwd } : {}),
      ...(opts.launchAgent ? { launchAgent: opts.launchAgent } : {}),
      parentLayout,
      isActive: activate
    }
    const tabs = (existing?.tabs ?? [])
      .filter((candidate) => candidate.id !== tab.id)
      .map((candidate) => ({
        ...candidate,
        ...(candidate.type === 'terminal' && candidate.parentTabId === parentTabId
          ? { parentLayout }
          : {}),
        isActive: activate ? false : candidate.isActive
      }))
    const insertAfter = afterTabId ? tabs.findIndex((candidate) => candidate.id === afterTabId) : -1
    if (insertAfter >= 0) {
      tabs.splice(insertAfter + 1, 0, tab)
    } else {
      tabs.push(tab)
    }
    const next: RuntimeMobileSessionTabsSnapshot = {
      worktree: worktreeId,
      publicationEpoch: `headless:${Date.now().toString(36)}`,
      snapshotVersion: (existing?.snapshotVersion ?? 0) + 1,
      // Why: activating the new tab also focuses its group, so when "+" targeted
      // a specific split group, make that group active too.
      activeGroupId:
        activate && opts.targetGroupId
          ? opts.targetGroupId
          : (existing?.activeGroupId ?? this.getHeadlessMobileSessionGroupId(worktreeId)),
      activeTabId: activate ? tab.id : (existing?.activeTabId ?? null),
      activeTabType: activate ? 'terminal' : (existing?.activeTabType ?? null),
      tabGroups: this.buildHeadlessMobileSessionTabGroups(
        worktreeId,
        tabs,
        activate ? tab : null,
        existing?.tabGroups,
        opts.targetGroupId ? { tabId: parentTabId, groupId: opts.targetGroupId } : undefined
      ),
      // Why: keep the group split geometry when a new tab is created, otherwise
      // opening a terminal while split loses the groups' arrangement.
      ...(existing?.tabGroupLayout ? { tabGroupLayout: existing.tabGroupLayout } : {}),
      tabs
    }
    this.mobileSessionTabsByWorktree.set(worktreeId, next)
    const result = this.toMobileSessionTabsResult(next)
    for (const listener of this.mobileSessionTabListeners) {
      listener(result)
    }
    const created = result.tabs.find((candidate) => candidate.id === tab.id)
    if (!created || created.type !== 'terminal') {
      throw new Error('terminal_handle_stale')
    }
    return {
      tab: created,
      publicationEpoch: result.publicationEpoch,
      snapshotVersion: result.snapshotVersion
    }
  }

  private waitForMobileTerminalSurface(
    worktreeId: string,
    parentTabId: string,
    options: { timeoutMs?: number; requireReady?: boolean; signal?: AbortSignal } = {}
  ): Promise<RuntimeMobileSessionCreateTerminalResult> {
    const timeoutMs = options.timeoutMs ?? MOBILE_TERMINAL_SURFACE_TIMEOUT_MS
    const existing = this.findMobileTerminalSurface(worktreeId, parentTabId, options)
    if (existing) {
      return Promise.resolve(existing)
    }
    if (options.signal?.aborted) {
      return Promise.reject(new Error('client_disconnected'))
    }

    return new Promise<RuntimeMobileSessionCreateTerminalResult>((resolve, reject) => {
      const cleanup = (): void => {
        clearTimeout(timer)
        options.signal?.removeEventListener('abort', onAbort)
        const idx = this.graph.graphSyncCallbacks.indexOf(check)
        if (idx !== -1) {
          this.graph.graphSyncCallbacks.splice(idx, 1)
        }
      }
      const timer = setTimeout(() => {
        cleanup()
        reject(new Error('Timed out waiting for terminal surface after creation'))
      }, timeoutMs)
      // Why: a dead client connection cancels the wait immediately instead of
      // running down the timeout and triggering rollback (#7718).
      const onAbort = (): void => {
        cleanup()
        reject(new Error('client_disconnected'))
      }
      options.signal?.addEventListener('abort', onAbort, { once: true })

      const check = (): void => {
        const next = this.findMobileTerminalSurface(worktreeId, parentTabId, options)
        if (!next) {
          return
        }
        cleanup()
        resolve(next)
      }
      this.graph.graphSyncCallbacks.push(check)
      check()
    })
  }

  private findMobileTerminalSurface(
    worktreeId: string,
    parentTabId: string,
    options: { requireReady?: boolean } = {}
  ): RuntimeMobileSessionCreateTerminalResult | null {
    const snapshot = this.mobileSessionTabsByWorktree.get(worktreeId)
    if (!snapshot) {
      return null
    }
    const result = this.toMobileSessionTabsResult(snapshot)
    const tab = result.tabs.find(
      (candidate) => candidate.type === 'terminal' && candidate.parentTabId === parentTabId
    )
    if (!tab || tab.type !== 'terminal') {
      return null
    }
    const surface = {
      tab,
      publicationEpoch: result.publicationEpoch,
      snapshotVersion: result.snapshotVersion
    }
    if (options.requireReady === true && !this.isReadyMobileTerminalSurface(surface)) {
      return null
    }
    return surface
  }

  // Why: for an in-flight mobile create whose surface hasn't published yet,
  // publish it main-side from the live renderer PTY so the create doesn't wait
  // on a stalled graph sync and destroy the session (#7587). No-op unless a
  // matching create is pending and a live bound PTY exists; never double-inserts.
  private ensurePtyBackedMobileSurfaceForRendererTab(
    worktreeId: string,
    tabId: string
  ): RuntimeMobileSessionCreateTerminalResult | null {
    const pending = this.pendingMobileTerminalCreatesByKey.get(`${worktreeId}::${tabId}`)
    if (!pending) {
      return null
    }
    const existing = this.findMobileTerminalSurface(worktreeId, tabId)
    if (existing) {
      // Why: the renderer's own publication already landed; stay idempotent.
      return existing
    }
    const pty = this.findLiveRegisteredPtyForRendererTab(worktreeId, tabId)
    const leafId = pty ? parsePaneKey(pty.paneKey ?? '')?.leafId : undefined
    if (!pty || !leafId) {
      return null
    }
    this.publishPtyBackedMobileSessionTerminal(worktreeId, pty, {
      tabId,
      leafId,
      title: null,
      activate: pending.activate,
      selectIfNoActiveTab: pending.selectIfNoActiveTab
    })
    // Why: waitForMobileTerminalSurface's check closures are drained only inside
    // syncWindowGraph; a main-side publish must drain them too or the pending
    // wait won't observe the insertion (mirrors syncWindowGraph's drain).
    for (const cb of [...this.graph.graphSyncCallbacks]) {
      cb()
    }
    return this.findMobileTerminalSurface(worktreeId, tabId)
  }

  private findLiveRegisteredPtyForRendererTab(
    worktreeId: string,
    tabId: string
  ): RuntimePtyWorktreeRecord | null {
    for (const pty of this.graph.ptysById.values()) {
      if (
        pty.worktreeId === worktreeId &&
        pty.tabId === tabId &&
        pty.connected &&
        parsePaneKey(pty.paneKey ?? '')?.leafId
      ) {
        return pty
      }
    }
    return null
  }

  // Why: rollback guard, looser than findLiveRegisteredPtyForRendererTab — a
  // shell whose pane key hasn't registered yet can't be surface-rescued, but
  // it is still a real terminal the create timeout must not kill (#7718).
  private hasLiveShellForRendererTab(worktreeId: string, tabId: string): boolean {
    for (const pty of this.graph.ptysById.values()) {
      if (pty.worktreeId === worktreeId && pty.tabId === tabId && pty.connected) {
        return true
      }
    }
    return false
  }

  private isReadyMobileTerminalSurface(
    surface: RuntimeMobileSessionCreateTerminalResult | null
  ): boolean {
    return (
      surface?.tab.status === 'ready' &&
      typeof surface.tab.terminal === 'string' &&
      surface.tab.terminal.length > 0
    )
  }

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

  private async resolveWorkspaceParentSelector(selector: string): Promise<ResolvedWorkspaceParent> {
    const rawSelector = selector.startsWith('id:') ? selector.slice('id:'.length) : selector
    const parsed = parseWorkspaceKey(rawSelector)
    if (parsed?.type === 'folder') {
      const folderWorkspace = this.store
        ?.getFolderWorkspaces?.()
        .find((workspace) => workspace.id === parsed.folderWorkspaceId)
      if (!folderWorkspace) {
        throw new Error('selector_not_found')
      }
      return {
        type: 'folder',
        workspaceKey: folderWorkspaceKey(folderWorkspace.id),
        folderWorkspace,
        instanceId: null
      }
    }
    const worktreeSelector = parsed?.type === 'worktree' ? `id:${parsed.worktreeId}` : selector
    const worktree = await this.resolveWorktreeSelector(worktreeSelector)
    return {
      type: 'worktree',
      workspaceKey: worktreeWorkspaceKey(worktree.id),
      worktree,
      instanceId: worktree.instanceId ?? null
    }
  }

  private validateLineageParent(child: ResolvedWorktree, parent: ResolvedWorktree): void {
    const childWorktreeId = child.id
    const parentWorktreeId = parent.id
    if (childWorktreeId === parentWorktreeId) {
      throw new RuntimeLineageError('LINEAGE_PARENT_CYCLE', 'A worktree cannot parent itself.')
    }
    const instanceByWorktreeId = new Map(
      this.resolvedWorktreeCommands
        .peekCache()
        ?.worktrees.map((worktree) => [worktree.id, worktree.instanceId]) ?? [
        [child.id, child.instanceId],
        [parent.id, parent.instanceId]
      ]
    )
    let cursor: string | undefined = parentWorktreeId
    const visited = new Set<string>([childWorktreeId])
    while (cursor) {
      if (visited.has(cursor)) {
        throw new RuntimeLineageError(
          'LINEAGE_PARENT_CYCLE',
          'Parent selector would create a lineage cycle.'
        )
      }
      visited.add(cursor)
      const lineage = this.store?.getWorktreeLineage?.(cursor)
      if (!lineage) {
        break
      }
      const cursorInstanceId = instanceByWorktreeId.get(cursor)
      const parentInstanceId = instanceByWorktreeId.get(lineage.parentWorktreeId)
      if (
        cursorInstanceId !== lineage.worktreeInstanceId ||
        parentInstanceId !== lineage.parentWorktreeInstanceId
      ) {
        break
      }
      cursor = lineage.parentWorktreeId
    }
  }

  private async resolveLineageForWorktreeCreate(
    input?: WorktreeLineageInput
  ): Promise<WorktreeLineageResolution> {
    const parentSelectorNextSteps = [
      'Pass a valid --parent-worktree selector such as folder:<id>, worktree:<worktreeId>, id:<repo-id>::<path>, branch:<branch>, issue:<number>, path:<absolute-path>, or active/current.',
      'Retry with --no-parent to create without lineage.'
    ]
    const parentSelectorNotFoundMessage = (err: unknown): string =>
      err instanceof WorktreeIdRequiresFullPathError
        ? err.message
        : 'Parent selector was not found.'

    if (!input) {
      return { kind: 'none', warnings: [] }
    }

    if (input.noParent === true && (input.parentWorkspace || input.parentWorktree)) {
      throw new RuntimeLineageError(
        'LINEAGE_PARENT_CONTEXT_CONFLICT',
        'Choose either one parent selector or --no-parent.'
      )
    }
    if (input.parentWorkspace && input.parentWorktree) {
      throw new RuntimeLineageError(
        'LINEAGE_PARENT_CONTEXT_CONFLICT',
        'Choose either one parent selector or --no-parent.'
      )
    }

    if (input.noParent === true) {
      return { kind: 'none', warnings: [] }
    }

    if (input.parentWorkspace) {
      try {
        return {
          kind: 'lineage',
          parent: await this.resolveWorkspaceParentSelector(input.parentWorkspace),
          origin: 'cli',
          capture: { source: 'explicit-cli-flag', confidence: 'explicit' }
        }
      } catch (err) {
        throw new RuntimeLineageError(
          'LINEAGE_PARENT_NOT_FOUND',
          parentSelectorNotFoundMessage(err),
          {
            nextSteps: parentSelectorNextSteps
          }
        )
      }
    }

    if (input.parentWorktree) {
      try {
        const parent = await this.resolveWorktreeSelector(input.parentWorktree)
        return {
          kind: 'lineage',
          parent: {
            type: 'worktree',
            workspaceKey: worktreeWorkspaceKey(parent.id),
            worktree: parent,
            instanceId: parent.instanceId ?? null
          },
          origin: 'cli',
          capture: { source: 'explicit-cli-flag', confidence: 'explicit' }
        }
      } catch (err) {
        throw new RuntimeLineageError(
          'LINEAGE_PARENT_NOT_FOUND',
          parentSelectorNotFoundMessage(err),
          {
            nextSteps: parentSelectorNextSteps
          }
        )
      }
    }

    const warnings: WorktreeLineageWarning[] = []
    const candidates: WorktreeLineageCandidate[] = []
    let cwdCandidate: WorktreeLineageCandidate | null = null
    let terminalContextResolved = false

    if (input.envParentWorkspace) {
      try {
        candidates.push({
          source: 'env-workspace',
          parent: await this.resolveWorkspaceParentSelector(input.envParentWorkspace)
        })
      } catch {
        warnings.push({
          code: 'LINEAGE_PARENT_CONTEXT_MISSING',
          message: 'Worktree created, but Orca could not validate the environment parent context.',
          details: { envParentWorkspace: input.envParentWorkspace }
        })
      }
    }

    if (input.orchestrationContext?.parentWorktreeId) {
      try {
        const parent = await this.resolveWorktreeSelector(
          `id:${input.orchestrationContext.parentWorktreeId}`
        )
        candidates.push({
          source: 'orchestration-context',
          parent: {
            type: 'worktree',
            workspaceKey: worktreeWorkspaceKey(parent.id),
            worktree: parent,
            instanceId: parent.instanceId ?? null
          }
        })
      } catch {
        // Keep creation recoverable; the warning below covers missing inferred context.
      }
    }

    const commentTaskId = extractOrchestrationTaskId(input.comment)
    if (commentTaskId) {
      const candidate = await this.resolveLineageCandidateForTaskId(commentTaskId)
      if (candidate) {
        candidates.push(candidate)
      }
    }

    if (input.callerTerminalHandle) {
      try {
        const terminal = await this.showTerminal(input.callerTerminalHandle)
        const terminalParent = await this.resolveWorkspaceParentSelector(
          `id:${terminal.worktreeId}`
        )
        const activeDispatch = this._orchestrationDb?.getActiveDispatchForTerminal(
          input.callerTerminalHandle
        )
        const activeRun = this._orchestrationDb?.getActiveCoordinatorRun()
        if (activeDispatch) {
          candidates.push({
            source: 'orchestration-context',
            parent: terminalParent,
            taskId: activeDispatch.task_id,
            ...(activeRun
              ? {
                  orchestrationRunId: activeRun.id,
                  coordinatorHandle: activeRun.coordinator_handle
                }
              : {})
          })
        } else {
          candidates.push({
            source: 'terminal-context',
            parent: terminalParent
          })
        }
        terminalContextResolved = true
      } catch {
        // Why: terminal handles can go stale during reloads or SSH reconnects.
        // A valid orchestration parent is still authoritative, so keep resolving
        // other inferred candidates instead of dropping lineage completely.
        warnings.push({
          code: 'LINEAGE_PARENT_CONTEXT_MISSING',
          message:
            'Worktree created, but Orca could not validate the caller terminal as a parent context.',
          details: { callerTerminalHandle: input.callerTerminalHandle }
        })
      }
    }

    if (input.cwdParentWorktree) {
      try {
        cwdCandidate = {
          source: 'cwd-context',
          parent: await this.resolveWorkspaceParentSelector(input.cwdParentWorktree)
        }
      } catch {
        warnings.push({
          code: 'LINEAGE_PARENT_CONTEXT_MISSING',
          message:
            'Worktree created, but Orca could not validate the current directory as a parent context.',
          details: { cwdParentWorktree: input.cwdParentWorktree }
        })
      }
    }

    if (candidates.length === 0 && cwdCandidate) {
      candidates.push(cwdCandidate)
    }

    if (candidates.length === 0) {
      return { kind: 'none', warnings }
    }

    const [first] = candidates
    const conflict = candidates.find(
      (candidate) => candidate.parent.workspaceKey !== first.parent.workspaceKey
    )
    if (conflict) {
      return {
        kind: 'none',
        warnings: [
          {
            code: 'LINEAGE_PARENT_CONTEXT_CONFLICT',
            message: 'Worktree created, but Orca could not prove which parent context caused it.',
            details: {
              terminalParentWorkspaceKey: candidates.find((c) => c.source === 'terminal-context')
                ?.parent.workspaceKey,
              envParentWorkspaceKey: candidates.find((c) => c.source === 'env-workspace')?.parent
                .workspaceKey,
              orchestrationParentWorkspaceKey: candidates.find(
                (c) => c.source === 'orchestration-context'
              )?.parent.workspaceKey
            }
          }
        ]
      }
    }

    const preferred =
      candidates.find((candidate) => candidate.source === 'env-workspace') ??
      candidates.find((candidate) => candidate.source === 'orchestration-context') ??
      first
    return {
      kind: 'lineage',
      parent: preferred.parent,
      origin: preferred.source === 'orchestration-context' ? 'orchestration' : 'cli',
      capture: { source: preferred.source, confidence: 'inferred' },
      ...((preferred.orchestrationRunId ?? input.orchestrationContext?.orchestrationRunId)
        ? {
            orchestrationRunId:
              preferred.orchestrationRunId ?? input.orchestrationContext?.orchestrationRunId
          }
        : {}),
      ...((preferred.taskId ?? input.orchestrationContext?.taskId)
        ? { taskId: preferred.taskId ?? input.orchestrationContext?.taskId }
        : {}),
      ...((preferred.coordinatorHandle ?? input.orchestrationContext?.coordinatorHandle)
        ? {
            coordinatorHandle:
              preferred.coordinatorHandle ?? input.orchestrationContext?.coordinatorHandle
          }
        : {}),
      ...(terminalContextResolved && input.callerTerminalHandle
        ? { createdByTerminalHandle: input.callerTerminalHandle }
        : {})
    }
  }

  private async resolveLineageCandidateForTaskId(
    taskId: string
  ): Promise<WorktreeLineageCandidate | null> {
    const db = this.getOrchestrationDbIfAvailable()
    const dispatch = db?.getDispatchContext(taskId)
    // Why: agent-created task records may never be dispatched, but the
    // creating terminal still identifies the parent workspace for descendants.
    const parentHandle =
      dispatch?.assignee_handle ?? db?.getTask(taskId)?.created_by_terminal_handle
    if (!parentHandle) {
      return null
    }
    try {
      const terminal = await this.showTerminal(parentHandle)
      const parent = await this.resolveWorktreeSelector(`id:${terminal.worktreeId}`)
      return {
        source: 'orchestration-context',
        parent: {
          type: 'worktree',
          workspaceKey: worktreeWorkspaceKey(parent.id),
          worktree: parent,
          instanceId: parent.instanceId ?? null
        },
        taskId
      }
    } catch {
      return null
    }
  }

  private getOrchestrationDbIfAvailable(): OrchestrationDb | null {
    try {
      return this._orchestrationDb ?? this.getOrchestrationDb()
    } catch {
      return this._orchestrationDb
    }
  }

  async hydrateInferredWorktreeLineage(): Promise<void> {
    const store = this.store
    if (
      !store ||
      typeof store.getWorktreeLineage !== 'function' ||
      typeof store.setWorktreeLineage !== 'function'
    ) {
      return
    }

    const worktrees = await this.listResolvedWorktrees()
    for (const worktree of worktrees) {
      if (store.getWorktreeLineage(worktree.id) || !worktree.instanceId) {
        continue
      }
      const taskId = extractOrchestrationTaskId(worktree.comment)
      if (!taskId) {
        continue
      }
      const candidate = await this.resolveLineageCandidateForTaskId(taskId)
      if (
        !candidate?.parent.instanceId ||
        candidate.parent.type !== 'worktree' ||
        candidate.parent.worktree.id === worktree.id
      ) {
        continue
      }
      try {
        this.validateLineageParent(worktree, candidate.parent.worktree)
      } catch {
        continue
      }
      store.setWorktreeLineage(worktree.id, {
        worktreeId: worktree.id,
        worktreeInstanceId: worktree.instanceId,
        parentWorktreeId: candidate.parent.worktree.id,
        parentWorktreeInstanceId: candidate.parent.instanceId,
        origin: 'orchestration',
        capture: { source: 'orchestration-context', confidence: 'inferred' },
        taskId,
        createdAt: Date.now()
      })
    }
  }

  async listWorktreeLineage(): Promise<Record<string, WorktreeLineage>> {
    await this.hydrateInferredWorktreeLineage()
    return this.store?.getAllWorktreeLineage?.() ?? {}
  }

  async listWorkspaceLineage(): Promise<Record<WorkspaceKey, WorkspaceLineage>> {
    await this.hydrateInferredWorktreeLineage()
    return this.store?.getAllWorkspaceLineage?.() ?? {}
  }

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
    this.recentPtyOutputById.delete(ptyId)
    this.clearWaitBlockedCheckState(ptyId)
    this.recentPtyPathCandidatesById.delete(ptyId)
    this.ptyOutputSequenceById.delete(ptyId)
    this.agentStatusOscProcessorsByPtyId.delete(ptyId)
    this.terminalSpawnCommandsByPtyId.delete(ptyId)
    this.disposePtyTitleTracker(ptyId)
    this.oscTitleScanTailByPtyId.delete(ptyId)
    this.osc7ScanTailByPtyId.delete(ptyId)
    this.terminalCwdByPtyId.delete(ptyId)
    this.terminalFileUriHostnameByPtyId.delete(ptyId)
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

  private syncMobileSessionTabs(snapshots: RuntimeMobileSessionTabsSnapshot[] | undefined): void {
    if (snapshots === undefined) {
      return
    }
    // Why: renderer graphs are authoritative for renderer tabs, but headless
    // serve terminals never enter that graph unless we preserve their bindings.
    this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(undefined, {
      allowAttachedWindow: true,
      onlyServeOwnedTerminals: true
    })
    const nextWorktrees = new Set<string>()
    for (const snapshot of snapshots) {
      nextWorktrees.add(snapshot.worktree)
      const existing = this.mobileSessionTabsByWorktree.get(snapshot.worktree)
      const nextSnapshot = this.mergePreservedHeadlessMobileSessionTabs(snapshot, existing)
      if (
        !existing ||
        nextSnapshot.publicationEpoch !== existing.publicationEpoch ||
        nextSnapshot.snapshotVersion >= existing.snapshotVersion
      ) {
        this.mobileSessionTabsByWorktree.set(snapshot.worktree, nextSnapshot)
      }
    }
    for (const [worktreeId, existing] of [...this.mobileSessionTabsByWorktree.entries()]) {
      if (!nextWorktrees.has(worktreeId)) {
        const preserved = this.buildPreservedHeadlessMobileSessionSnapshot(existing)
        if (preserved) {
          this.mobileSessionTabsByWorktree.set(worktreeId, preserved)
          nextWorktrees.add(worktreeId)
        } else {
          this.mobileSessionTabsByWorktree.delete(worktreeId)
          // Why: drop any pending coalesced notify so a stale snapshot can't
          // land after the removed frame.
          this.mobileSessionTabsNotifyCoalescer.cancel(worktreeId)
          this.notifyMobileSessionTabsRemoved(worktreeId)
        }
      }
    }
  }

  private mergePreservedHeadlessMobileSessionTabs(
    snapshot: RuntimeMobileSessionTabsSnapshot,
    existing: RuntimeMobileSessionTabsSnapshot | undefined
  ): RuntimeMobileSessionTabsSnapshot {
    if (!existing) {
      return snapshot
    }
    const preservedTabs = this.collectPreservedHeadlessMobileSessionTabs(existing, snapshot)
    if (preservedTabs.length === 0) {
      return snapshot
    }
    const hasIncomingActiveTab = snapshot.tabs.some((tab) => tab.isActive)
    const normalizedPreservedTabs = preservedTabs.map((tab) =>
      hasIncomingActiveTab ? { ...tab, isActive: false } : tab
    )
    const tabs = this.mergeMobileSessionSnapshotTabs(snapshot.tabs, normalizedPreservedTabs)
    if (tabs.length === snapshot.tabs.length) {
      return snapshot
    }
    const activeTab =
      snapshot.tabs.find((tab) => tab.id === snapshot.activeTabId) ??
      tabs.find((tab) => tab.id === existing.activeTabId) ??
      tabs.find((tab) => tab.isActive) ??
      tabs[0] ??
      null
    const terminalTabs = tabs.filter(
      (tab): tab is RuntimeMobileSessionTerminalTab => tab.type === 'terminal'
    )
    return {
      ...snapshot,
      publicationEpoch: this.getMergedMobileSessionPublicationEpoch(
        snapshot,
        normalizedPreservedTabs
      ),
      snapshotVersion: Math.max(snapshot.snapshotVersion, existing.snapshotVersion),
      activeGroupId: snapshot.activeGroupId ?? existing.activeGroupId,
      activeTabId: activeTab?.id ?? null,
      activeTabType: activeTab?.type ?? null,
      tabGroups: this.mergeMobileSessionTabGroups(
        snapshot.worktree,
        snapshot.tabGroups ?? existing.tabGroups ?? [],
        terminalTabs,
        activeTab?.type === 'terminal' ? activeTab : null
      ),
      tabs
    }
  }

  private buildPreservedHeadlessMobileSessionSnapshot(
    existing: RuntimeMobileSessionTabsSnapshot
  ): RuntimeMobileSessionTabsSnapshot | null {
    const tabs = this.collectPreservedHeadlessMobileSessionTabs(existing)
    if (tabs.length === 0) {
      return null
    }
    const activeTab =
      tabs.find((tab) => tab.id === existing.activeTabId) ??
      tabs.find((tab) => tab.isActive) ??
      tabs[0] ??
      null
    const terminalTabs = tabs.filter(
      (tab): tab is RuntimeMobileSessionTerminalTab => tab.type === 'terminal'
    )
    return {
      ...existing,
      publicationEpoch: this.getMergedMobileSessionPublicationEpoch(existing, tabs),
      activeGroupId:
        existing.activeGroupId ?? this.getHeadlessMobileSessionGroupId(existing.worktree),
      activeTabId: activeTab?.id ?? null,
      activeTabType: activeTab?.type ?? null,
      tabGroups: this.mergeMobileSessionTabGroups(
        existing.worktree,
        existing.tabGroups ?? [],
        terminalTabs,
        activeTab?.type === 'terminal' ? activeTab : null
      ),
      tabs
    }
  }

  private collectPreservedHeadlessMobileSessionTabs(
    existing: RuntimeMobileSessionTabsSnapshot,
    incoming?: RuntimeMobileSessionTabsSnapshot
  ): RuntimeMobileSessionSnapshotTab[] {
    const incomingIds = new Set(
      incoming?.tabs.flatMap((tab) => this.getMobileSessionSnapshotTabIdentityKeys(tab)) ?? []
    )
    return existing.tabs.filter((tab) => {
      if (this.getMobileSessionSnapshotTabIdentityKeys(tab).some((id) => incomingIds.has(id))) {
        return false
      }
      return this.shouldPreserveHeadlessMobileSessionTab(existing, tab)
    })
  }

  private shouldPreserveHeadlessMobileSessionTab(
    snapshot: RuntimeMobileSessionTabsSnapshot,
    tab: RuntimeMobileSessionSnapshotTab
  ): boolean {
    // Why: headless offscreen browser tabs live only on the server and are
    // re-derived from the live bridge on each hydrate, so a renderer-graph merge
    // must keep them rather than prune them as "not in the renderer graph".
    if (tab.type === 'browser') {
      return (
        Boolean(this.offscreenBrowserBackend) &&
        this.isHeadlessMobileSessionPublication(snapshot.publicationEpoch)
      )
    }
    if (tab.type !== 'terminal') {
      return false
    }
    return (
      this.isHeadlessMobileSessionPublication(snapshot.publicationEpoch) ||
      this.hasServeOwnedPtyBinding(tab)
    )
  }

  private isHeadlessMobileSessionPublication(publicationEpoch: string): boolean {
    return (
      publicationEpoch.startsWith('headless:') ||
      publicationEpoch.startsWith('headless-hydrated:') ||
      publicationEpoch.includes(':headless-merge:')
    )
  }

  private getMergedMobileSessionPublicationEpoch(
    snapshot: RuntimeMobileSessionTabsSnapshot,
    preservedTabs: readonly RuntimeMobileSessionSnapshotTab[]
  ): string {
    // Why: preserved snapshots can be merged repeatedly; normalize the prior
    // merge suffix before recomputing so the publication epoch is idempotent.
    const normalizedPublicationEpoch = snapshot.publicationEpoch.split(':headless-merge:')[0]
    const signature = createHash('sha1')
      .update(
        preservedTabs
          .map((tab) =>
            tab.type === 'terminal'
              ? `${tab.id}:${tab.parentTabId}:${tab.ptyId ?? ''}:${tab.leafId}`
              : tab.id
          )
          .join('|')
      )
      .digest('hex')
      .slice(0, 12)
    return `${normalizedPublicationEpoch}:headless-merge:${signature}`
  }

  private notifyMobileSessionTabsRemoved(worktreeId: string): void {
    const removed: RuntimeMobileSessionTabsRemovedResult = {
      worktree: worktreeId,
      publicationEpoch: `removed:${Date.now().toString(36)}`,
      snapshotVersion: 0,
      removed: true,
      activeGroupId: null,
      activeTabId: null,
      activeTabType: null,
      tabs: []
    }
    for (const listener of this.mobileSessionTabListeners) {
      listener(removed)
    }
  }

  notifyMobileSessionTabsChanged(worktreeId?: string): void {
    if (!worktreeId) {
      this.notifyMobileSessionTabSnapshots()
      return
    }
    // Why: structural changes (tab add/remove/activate) must propagate promptly,
    // so cancel any pending coalesced title/status notify — this immediate emit
    // already reflects the latest snapshot and supersedes it.
    this.mobileSessionTabsNotifyCoalescer.cancel(worktreeId)
    this.notifyMobileSessionTabsChangedNow(worktreeId)
  }

  private notifyMobileSessionTabsChangedNow(worktreeId: string): void {
    if (this.mobileSessionTabListeners.size === 0) {
      return
    }
    const snapshot = this.mobileSessionTabsByWorktree.get(worktreeId)
    if (!snapshot) {
      return
    }
    // Why: browser bridge lifecycle events are already scoped by worktree; avoid
    // fanning out every active workspace snapshot during navigation/tab churn.
    const result = this.toMobileSessionTabsResult(snapshot)
    for (const listener of this.mobileSessionTabListeners) {
      listener(result)
    }
  }

  private notifyMobileSessionTabSnapshots(): void {
    if (this.mobileSessionTabListeners.size === 0) {
      return
    }
    for (const snapshot of this.mobileSessionTabsByWorktree.values()) {
      const result = this.toMobileSessionTabsResult(snapshot)
      for (const listener of this.mobileSessionTabListeners) {
        listener(result)
      }
    }
  }

  private getMobileSessionTabsForWorktree(worktreeId: string): RuntimeMobileSessionTabsResult {
    const snapshot = this.mobileSessionTabsByWorktree.get(worktreeId)
    if (!snapshot) {
      return {
        worktree: worktreeId,
        publicationEpoch: 'none',
        snapshotVersion: 0,
        activeGroupId: null,
        activeTabId: null,
        activeTabType: null,
        tabs: []
      }
    }
    return this.toMobileSessionTabsResult(snapshot)
  }

  private async resolveMobileMarkdownWorktreeId(
    worktreeSelector: string,
    tabId: string
  ): Promise<string> {
    const worktreeId =
      this.getValidatedExplicitWorktreeIdSelector(worktreeSelector) ??
      (await this.resolveWorktreeSelector(worktreeSelector)).id
    const snapshot = this.mobileSessionTabsByWorktree.get(worktreeId)
    const tab = snapshot?.tabs.find(
      (candidate): candidate is RuntimeMobileSessionMarkdownTab =>
        candidate.type === 'markdown' && candidate.id === tabId
    )
    if (!tab) {
      throw new Error('tab_not_found')
    }
    return worktreeId
  }

  private getLiveBrowserTabsByPageId(worktreeId: string): Map<string, BrowserTabInfo> {
    if (!this.agentBrowserBridge?.tabList) {
      return new Map()
    }
    const liveTabs = this.agentBrowserBridge.tabList(worktreeId).tabs
    return new Map(liveTabs.map((tab) => [tab.browserPageId, tab]))
  }

  private collectReturnedSessionTabIds(
    tabs: readonly RuntimeMobileSessionClientTab[]
  ): Set<string> {
    const ids = new Set<string>()
    for (const tab of tabs) {
      ids.add(tab.id)
      if (tab.type === 'terminal') {
        ids.add(tab.parentTabId)
      } else if (tab.type === 'browser') {
        ids.add(tab.browserWorkspaceId)
      }
    }
    return ids
  }

  private sanitizeMobileSessionTabGroups(
    groups: readonly RuntimeMobileSessionTabGroup[] | undefined,
    returnedTabs: readonly RuntimeMobileSessionClientTab[]
  ): RuntimeMobileSessionTabGroup[] | undefined {
    if (!groups || groups.length === 0) {
      return undefined
    }
    const returnedIds = this.collectReturnedSessionTabIds(returnedTabs)
    const sanitized = groups
      .map((group): RuntimeMobileSessionTabGroup | null => {
        const tabOrder = group.tabOrder.filter((tabId) => returnedIds.has(tabId))
        if (tabOrder.length === 0) {
          return null
        }
        const activeTabId =
          group.activeTabId && tabOrder.includes(group.activeTabId)
            ? group.activeTabId
            : (tabOrder[0] ?? null)
        const recentTabIds = group.recentTabIds?.filter((tabId) => tabOrder.includes(tabId))
        return {
          id: group.id,
          activeTabId,
          tabOrder,
          ...(recentTabIds && recentTabIds.length > 0 ? { recentTabIds } : {})
        }
      })
      .filter((group): group is RuntimeMobileSessionTabGroup => group !== null)
    return sanitized.length > 0 ? sanitized : undefined
  }

  private pruneMobileSessionTabGroupLayout(
    layout: TabGroupLayoutNode | null | undefined,
    validGroupIds: ReadonlySet<string>
  ): TabGroupLayoutNode | null {
    if (!layout) {
      return null
    }
    if (layout.type === 'leaf') {
      return validGroupIds.has(layout.groupId) ? layout : null
    }
    const first = this.pruneMobileSessionTabGroupLayout(layout.first, validGroupIds)
    const second = this.pruneMobileSessionTabGroupLayout(layout.second, validGroupIds)
    if (first && second) {
      return { ...layout, first, second }
    }
    return first ?? second
  }

  /**
   * Transforms an internal mobile session tab snapshot into a sanitized client payload,
   * resolving launch agent ownership and normalizing titles.
   */
  private toMobileSessionTabsResult(
    snapshot: RuntimeMobileSessionTabsSnapshot
  ): RuntimeMobileSessionTabsResult {
    const tabs: RuntimeMobileSessionClientTab[] = []
    const liveBrowserTabsByPageId = this.getLiveBrowserTabsByPageId(snapshot.worktree)
    // Why: a live PTY backs exactly one terminal surface, so it must map to a
    // single emitted tab. After agent sleep + mobile wake, a stale
    // headless-hydrated leaf can survive beside the renderer's live leaf and both
    // resolve to the freshly-woken agent PTY (same issuePtyHandle handle) — which
    // renders two panes with the same React key and crashes the client. Claim
    // each live PTY once. Split siblings never collide because distinct leaves own
    // distinct PTYs; renderer tabs precede preserved headless tabs, so the live
    // one wins.
    const claimedLivePtyIds = new Set<string>()
    for (const tab of snapshot.tabs) {
      if (tab.type === 'browser') {
        const liveTab = tab.browserPageId
          ? liveBrowserTabsByPageId.get(tab.browserPageId)
          : undefined
        if (!liveTab) {
          continue
        }
        // Why: renderer session snapshots can lag behind BrowserView teardown or
        // process swaps. Pairing clients should only see browser pages the main
        // browser bridge can still route commands and screencasts to.
        tabs.push({
          ...tab,
          title: liveTab.title || tab.title,
          url: liveTab.url || tab.url,
          // Why: bridge "active" means active BrowserView/webContents, not
          // active Orca tab. Preserve the renderer's app-level session focus.
          isActive: tab.isActive
        })
        continue
      }
      if (tab.type === 'markdown' || tab.type === 'file') {
        tabs.push(tab)
        continue
      }
      const syncedTab = this.graph.tabs.get(tab.parentTabId)
      const leaf = this.graph.leaves.get(this.getLeafKey(tab.parentTabId, tab.leafId)) ?? null
      const liveLeaf = leaf?.ptyId && leaf.connected ? leaf : null
      const liveLeafPtyId = liveLeaf?.ptyId ?? null
      const liveLeafPty = liveLeafPtyId ? (this.graph.ptysById.get(liveLeafPtyId) ?? null) : null
      const pty = liveLeaf
        ? null
        : this.findPtyForMobileTerminalTab(snapshot.worktree, tab, {
            allowWorktreeOnlyMatch: !snapshot.publicationEpoch.startsWith('headless')
          })
      const livePty = pty?.connected ? pty : null
      // Why: enforce the one-live-PTY-per-tab invariant. A later tab resolving to
      // a PTY an earlier tab already claimed is a duplicate surface (e.g. a stale
      // headless-hydrated leaf re-bound to a woken agent PTY) — drop it so the
      // client never sees two tabs sharing a terminal handle. Handles derive purely
      // from the PTY id (issuePtyHandle), so the id is a faithful proxy for the
      // emitted handle. Pending tabs (no live PTY) are left untouched.
      const resolvedLivePtyId = liveLeafPtyId ?? livePty?.ptyId ?? null
      if (resolvedLivePtyId !== null) {
        if (claimedLivePtyIds.has(resolvedLivePtyId)) {
          continue
        }
        claimedLivePtyIds.add(resolvedLivePtyId)
      }
      const legacyPaneId = /^pane:(\d+)$/.exec(tab.leafId)?.[1] ?? null
      const paneKey = isTerminalLeafId(tab.leafId)
        ? makePaneKey(tab.parentTabId, tab.leafId)
        : `${tab.parentTabId}:${legacyPaneId ?? tab.leafId}`
      const leafTitle = leaf
        ? getLatestAgentCandidateTitle(
            { title: leaf.paneTitle, updatedAt: leaf.paneTitleUpdatedAt },
            { title: leaf.lastOscTitle, updatedAt: leaf.lastOscTitleAt }
          )
        : null
      const ptyTitle = pty
        ? getLatestAgentCandidateTitle(
            { title: pty.title, updatedAt: pty.titleUpdatedAt },
            { title: pty.lastOscTitle, updatedAt: pty.lastOscTitleAt }
          )
        : null
      const launchAgent = tab.launchAgent ?? liveLeafPty?.launchAgent ?? pty?.launchAgent ?? null
      const ownerAgent = launchAgent ?? liveLeafPty?.foregroundAgent ?? pty?.foregroundAgent ?? null
      const title = normalizeCompatibleAgentTitleForOwner(
        leafTitle ?? ptyTitle ?? syncedTab?.title ?? tab.title,
        ownerAgent
      )
      const liveTitleEvidence = leafTitle ?? ptyTitle
      const liveTitleEvidenceClassification = classifyAgentTitle(liveTitleEvidence)
      const normalizedTabAgentStatus = tab.agentStatus
        ? normalizeCompatibleAgentStatusEntryForOwner(tab.agentStatus, ownerAgent)
        : null
      // Why: keep the rich hook-driven status when the agent has a live
      // interactive prompt or an active tool — those are authoritative agent
      // activity even if the terminal's title isn't agent-classified (e.g. it
      // shows a task/branch name). Otherwise the mobile/web client falls back to
      // the OSC-title-only status and never sees interactivePrompt (the question
      // card never renders).
      const hasLiveAgentSignal =
        normalizedTabAgentStatus?.interactivePrompt != null ||
        normalizedTabAgentStatus?.toolName != null
      const keepFullAgentStatus =
        normalizedTabAgentStatus &&
        (liveTitleEvidence === null ||
          liveTitleEvidenceClassification === 'agent' ||
          hasLiveAgentSignal)
      const agentStatus = keepFullAgentStatus
        ? { agentStatus: normalizedTabAgentStatus }
        : // Why: when live title evidence says the pane is idle (e.g. the Claude
          // agents picker or a neutral shell title), suppress the stale "working"
          // state so the client shows no spinner — but retain agent identity
          // (agentType + providerSession) so native chat can still address an
          // idle agent's transcript. Reset the transient state to 'done'.
          normalizedTabAgentStatus?.agentType != null
          ? {
              agentStatus: {
                state: 'done' as const,
                prompt: '',
                updatedAt: normalizedTabAgentStatus.updatedAt,
                stateStartedAt: normalizedTabAgentStatus.stateStartedAt,
                paneKey: normalizedTabAgentStatus.paneKey,
                stateHistory: [],
                agentType: normalizedTabAgentStatus.agentType,
                ...(normalizedTabAgentStatus.providerSession
                  ? { providerSession: normalizedTabAgentStatus.providerSession }
                  : {})
              }
            }
          : null
      // Why: web/mobile clients hold these handles across renderer graph syncs;
      // leaf handles are graph-epoch-bound, but PTY handles remain streamable.
      const terminalHandle = liveLeafPtyId
        ? this.issuePtyHandle(
            this.recordPtyWorktree(liveLeafPtyId, snapshot.worktree, {
              tabId: tab.parentTabId,
              paneKey,
              connected: true
            })
          )
        : livePty
          ? this.issuePtyHandle(livePty)
          : null
      tabs.push({
        type: 'terminal',
        id: tab.id,
        parentTabId: tab.parentTabId,
        leafId: tab.leafId,
        title,
        ...(tab.ptyId ? { ptyId: tab.ptyId } : {}),
        ...(tab.terminalTheme ? { terminalTheme: tab.terminalTheme } : {}),
        ...(launchAgent ? { launchAgent } : {}),
        ...(agentStatus ?? this.buildPtyMobileAgentStatus(livePty ?? pty, tab, terminalHandle)),
        ...(tab.parentLayout ? { parentLayout: tab.parentLayout } : {}),
        ...(tab.startupCwd ? { startupCwd: tab.startupCwd } : {}),
        ...(tab.color != null ? { color: tab.color } : {}),
        ...(tab.isPinned ? { isPinned: true } : {}),
        ...(tab.viewMode ? { viewMode: tab.viewMode } : {}),
        isActive: tab.isActive,
        ...(terminalHandle
          ? { status: 'ready' as const, terminal: terminalHandle }
          : { status: 'pending-handle' as const, terminal: null })
      })
    }
    const active =
      tabs.find((tab) => tab.isActive && tab.id === snapshot.activeTabId) ??
      tabs.find((tab) => tab.isActive) ??
      (snapshot.activeTabId ? (tabs[0] ?? null) : null)
    const normalizedTabs =
      active && !tabs.some((tab) => tab.isActive)
        ? tabs.map((tab) => (tab.id === active.id ? { ...tab, isActive: true } : tab))
        : tabs
    const tabGroups = this.sanitizeMobileSessionTabGroups(snapshot.tabGroups, normalizedTabs)
    const validGroupIds = new Set(tabGroups?.map((group) => group.id) ?? [])
    const tabGroupLayout =
      snapshot.tabGroupLayout === undefined
        ? undefined
        : this.pruneMobileSessionTabGroupLayout(snapshot.tabGroupLayout, validGroupIds)
    const activeGroupId =
      snapshot.activeGroupId && validGroupIds.has(snapshot.activeGroupId)
        ? snapshot.activeGroupId
        : (tabGroups?.find((group) =>
            active
              ? group.tabOrder.some((tabId) =>
                  this.collectReturnedSessionTabIds([active]).has(tabId)
                )
              : false
          )?.id ??
          tabGroups?.[0]?.id ??
          null)
    return {
      worktree: snapshot.worktree,
      publicationEpoch: snapshot.publicationEpoch,
      snapshotVersion: snapshot.snapshotVersion,
      activeGroupId,
      activeTabId: active?.id ?? null,
      activeTabType: active?.type ?? null,
      ...(tabGroups ? { tabGroups } : {}),
      ...(snapshot.tabGroupLayout !== undefined ? { tabGroupLayout } : {}),
      tabs: normalizedTabs
    }
  }

  /**
   * Generates a mobile-friendly status entry for a PTY, aligning agentType
   * and titles with the active owner.
   */
  private buildPtyMobileAgentStatus(
    pty: RuntimePtyWorktreeRecord | null,
    tab: RuntimeMobileSessionTerminalTab,
    terminalHandle: string | null
  ): { agentStatus: AgentStatusEntry } | Record<string, never> {
    const paneKey = this.getMobileTerminalPaneKey(tab)
    const retained = this.getFreshRetainedAgentStatusForMobileTab(paneKey, pty, tab)
    if (!pty?.lastAgentStatus && !retained) {
      return {}
    }
    const leaf = this.graph.leaves.get(this.getLeafKey(tab.parentTabId, tab.leafId)) ?? null
    const ptyTitle = pty
      ? getLatestAgentCandidateTitle(
          { title: pty.title, updatedAt: pty.titleUpdatedAt },
          { title: pty.lastOscTitle, updatedAt: pty.lastOscTitleAt }
        )
      : leaf
        ? getLatestAgentCandidateTitle(
            { title: leaf.paneTitle, updatedAt: leaf.paneTitleUpdatedAt },
            { title: leaf.lastOscTitle, updatedAt: leaf.lastOscTitleAt }
          )
        : null
    const ptyTitleClassification = classifyAgentTitle(ptyTitle)
    if (ptyTitle !== null && ptyTitleClassification !== 'agent') {
      // Why: a non-agent title means the shell owns the pane again (the agent
      // exited or was replaced) — suppressing here is what clears stuck
      // spinners (#1437). A live hook signal (question card / active tool) is
      // authoritative agent activity even under a task-named title, so it
      // survives the suppression, mirroring the renderer-synced branch above.
      const hasLiveHookSignal =
        retained?.payload.interactivePrompt != null || retained?.payload.toolName != null
      if (!hasLiveHookSignal) {
        return {}
      }
    }
    const ownerAgent = tab.launchAgent ?? pty?.launchAgent ?? pty?.foregroundAgent ?? null
    const terminalTitle = normalizeCompatibleAgentTitleForOwner(
      (pty ? getLatestPtyTitle(pty) : null) ?? tab.title,
      ownerAgent
    )
    // Why: hook (OSC 9999) payloads carry the real state, prompt, and agent
    // identity; the title heuristic below is a fallback with none of that.
    // Without this, headless-serve clients only ever saw title-derived rows
    // and hook-only transitions (e.g. opencode waiting) never surfaced (#7970).
    if (retained) {
      return {
        agentStatus: normalizeCompatibleAgentStatusEntryForOwner(
          {
            ...retained.payload,
            paneKey,
            updatedAt: retained.updatedAt,
            stateStartedAt: retained.stateStartedAt,
            stateHistory: [],
            ...(terminalHandle ? { terminalHandle } : {}),
            ...((pty?.worktreeId ?? retained.worktreeId)
              ? { worktreeId: pty?.worktreeId ?? retained.worktreeId }
              : {}),
            tabId: tab.parentTabId,
            terminalTitle
          },
          ownerAgent
        )
      }
    }
    const now = pty!.lastOutputAt ?? Date.now()
    const agentType = ownerAgent ?? undefined
    return {
      agentStatus: {
        state:
          pty!.lastAgentStatus === 'working'
            ? 'working'
            : pty!.lastAgentStatus === 'permission'
              ? 'blocked'
              : 'done',
        prompt: '',
        updatedAt: now,
        stateStartedAt: now,
        paneKey,
        ...(terminalHandle ? { terminalHandle } : {}),
        ...(agentType ? { agentType } : {}),
        worktreeId: pty!.worktreeId,
        tabId: tab.parentTabId,
        terminalTitle,
        stateHistory: []
      }
    }
  }

  /** The retained OSC 9999 hook row for this mobile tab, when fresh enough to
   *  trust. Looked up by pane identity first, then by PTY ownership because
   *  legacy `pane:N` leaf ids can drift from the hook-side pane key. */
  private getFreshRetainedAgentStatusForMobileTab(
    paneKey: string,
    pty: RuntimePtyWorktreeRecord | null,
    tab: RuntimeMobileSessionTerminalTab
  ): RuntimeAgentRowSnapshot | null {
    let retained = this.latestAgentStatusByPaneKey.get(paneKey) ?? null
    if (!retained) {
      const ptyId = pty?.ptyId ?? tab.ptyId ?? null
      if (ptyId) {
        for (const snapshot of this.latestAgentStatusByPaneKey.values()) {
          if (snapshot.ptyId !== ptyId) {
            continue
          }
          if (!retained || snapshot.updatedAt > retained.updatedAt) {
            retained = snapshot
          }
        }
      }
    }
    if (!retained || Date.now() - retained.updatedAt > AGENT_STATUS_STALE_AFTER_MS) {
      return null
    }
    return retained
  }

  private findPtyForMobileTerminalTab(
    worktreeId: string,
    tab: RuntimeMobileSessionTerminalTab,
    options: { allowWorktreeOnlyMatch?: boolean } = {}
  ): RuntimePtyWorktreeRecord | null {
    const snapshotPtyId = tab.ptyId ?? tab.parentLayout?.ptyIdsByLeafId?.[tab.leafId] ?? null
    const paneKey = this.getMobileTerminalPaneKey(tab)
    if (snapshotPtyId) {
      const pty = this.graph.ptysById.get(snapshotPtyId)
      if (!pty) {
        return null
      }
      // Why: persisted PTY ids can collide with unrelated provider ids after a
      // restart. Only a matching spawn-time pane identity is safe to expose.
      if (this.mobileTerminalTabMatchesPty(worktreeId, tab, pty, paneKey)) {
        return pty
      }
      if (
        options.allowWorktreeOnlyMatch === true &&
        pty.worktreeId === worktreeId &&
        pty.tabId === null &&
        pty.paneKey === null
      ) {
        return pty
      }
      return null
    }
    const paneKeys = new Set([`${tab.parentTabId}:${tab.leafId}`])
    if (tab.leafId === `pane:${FIRST_PANE_ID}`) {
      paneKeys.add(`${tab.parentTabId}:${FIRST_PANE_ID}`)
    }
    for (const pty of this.graph.ptysById.values()) {
      if (pty.tabId === tab.parentTabId && pty.paneKey && paneKeys.has(pty.paneKey)) {
        return pty
      }
    }
    return null
  }

  private getPersistedSshPtyIdForMobileTerminalTab(
    tab: RuntimeMobileSessionTerminalTab
  ): string | null {
    const ptyId = tab.ptyId ?? tab.parentLayout?.ptyIdsByLeafId?.[tab.leafId] ?? null
    return ptyId && parseAppSshPtyId(ptyId) ? ptyId : null
  }

  private getMobileTerminalPaneKey(tab: RuntimeMobileSessionTerminalTab): string {
    if (isTerminalLeafId(tab.leafId)) {
      return makePaneKey(tab.parentTabId, tab.leafId)
    }
    const legacyPaneId = /^pane:(\d+)$/.exec(tab.leafId)?.[1] ?? null
    return `${tab.parentTabId}:${legacyPaneId ?? tab.leafId}`
  }

  private mobileTerminalTabMatchesPty(
    worktreeId: string,
    tab: RuntimeMobileSessionTerminalTab,
    pty: RuntimePtyWorktreeRecord,
    paneKey = this.getMobileTerminalPaneKey(tab)
  ): boolean {
    return pty.worktreeId === worktreeId && pty.tabId === tab.parentTabId && pty.paneKey === paneKey
  }

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

  deliverPendingMessagesForHandle(handle: string): void {
    try {
      const { leaf } = this.getLiveLeafForHandle(handle)
      if (leaf.lastAgentStatus === 'idle') {
        this.deliverPendingMessages(leaf)
      }
    } catch {
      // Unknown or stale handles cannot be pushed immediately; the persisted
      // message remains available via explicit check or future idle delivery.
    }
  }

  // Why: after a message is inserted for a recipient, any blocking
  // orchestration.check --wait calls watching that handle must be woken
  // so they can return the new message immediately instead of polling.
  notifyMessageArrived(handle: string, messageType?: string): void {
    const waiters = this.messageWaitersByHandle.get(handle)
    if (!waiters || waiters.size === 0) {
      return
    }
    for (const waiter of [...waiters]) {
      // Why: a coordinator waiting for worker_done/escalation should not be
      // woken by worker heartbeat noise and mistake that empty read for idleness.
      if (messageType && waiter.typeFilter && !waiter.typeFilter.includes(messageType)) {
        continue
      }
      this.resolveMessageWaiter(waiter)
    }
  }

  waitForMessage(
    handle: string,
    options?: { typeFilter?: string[]; timeoutMs?: number; signal?: AbortSignal }
  ): Promise<void> {
    return new Promise((resolve) => {
      const timeoutMs = options?.timeoutMs ?? MESSAGE_WAIT_DEFAULT_TIMEOUT_MS

      const waiter: MessageWaiter = {
        handle,
        typeFilter: options?.typeFilter,
        resolve,
        timeout: null,
        abortCleanup: null
      }

      // Why: if the caller aborts (socket closed on the RPC side — see design
      // doc §3.1 counter-lifecycle), resolve immediately so the long-poll slot
      // is released instead of counting down the full timeoutMs with a dead
      // client on the other end.
      const signal = options?.signal
      const onAbort = (): void => {
        this.removeMessageWaiter(waiter)
        resolve()
      }
      if (signal) {
        if (signal.aborted) {
          resolve()
          return
        }
        waiter.abortCleanup = () => signal.removeEventListener('abort', onAbort)
        signal.addEventListener('abort', onAbort, { once: true })
      }

      waiter.timeout = setTimeout(() => {
        this.removeMessageWaiter(waiter)
        resolve()
      }, timeoutMs)

      let waiters = this.messageWaitersByHandle.get(handle)
      if (!waiters) {
        waiters = new Set()
        this.messageWaitersByHandle.set(handle, waiters)
      }
      waiters.add(waiter)
    })
  }

  private resolveMessageWaiter(waiter: MessageWaiter): void {
    this.removeMessageWaiter(waiter)
    waiter.resolve()
  }

  private removeMessageWaiter(waiter: MessageWaiter): void {
    if (waiter.timeout) {
      clearTimeout(waiter.timeout)
      waiter.timeout = null
    }
    if (waiter.abortCleanup) {
      waiter.abortCleanup()
      waiter.abortCleanup = null
    }
    const waiters = this.messageWaitersByHandle.get(waiter.handle)
    if (waiters) {
      waiters.delete(waiter)
      if (waiters.size === 0) {
        this.messageWaitersByHandle.delete(waiter.handle)
      }
    }
  }

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

  async browserScreencast(
    params: Parameters<RuntimeBrowserCommands['browserScreencast']>[0],
    options: {
      connectionId?: string
      sendBinary?: (bytes: Uint8Array<ArrayBufferLike>) => boolean | void
      signal?: AbortSignal
      emit: (result: BrowserScreencastResult) => void
    }
  ): Promise<void> {
    if (!options.sendBinary) {
      throw new BrowserError(
        'browser_error',
        'Browser screencast requires a binary streaming transport.'
      )
    }

    const connectionKey = options.connectionId ?? 'local'
    const requestedPageId = typeof params.page === 'string' ? params.page : null
    let existingPageStream = requestedPageId
      ? this.activeBrowserScreencastsByPage.get(requestedPageId)
      : undefined
    while (existingPageStream) {
      // Why: CDP only supports one screencast per browser page. A stale paired
      // web/mobile stream should not leave the next tab activation stuck on an
      // already-active error or old viewport dimensions.
      existingPageStream.cancel(existingPageStream.connectionKey !== connectionKey)
      await existingPageStream.done
      existingPageStream = requestedPageId
        ? this.activeBrowserScreencastsByPage.get(requestedPageId)
        : undefined
    }
    let existingStream = this.activeBrowserScreencastsByConnection.get(connectionKey)
    while (existingStream) {
      existingStream.cancel()
      await existingStream.done
      existingStream = this.activeBrowserScreencastsByConnection.get(connectionKey)
    }
    if (options.signal?.aborted) {
      throw new BrowserError('browser_error', 'Browser screencast was cancelled.')
    }

    let screencast: Awaited<ReturnType<RuntimeBrowserCommands['browserScreencast']>> | null = null
    let registeredSubscriptionId: string | null = null
    let activeBrowserPageId: string | null = null
    let ended = false
    let cancelledBeforeStart = false
    let readyEmitted = false
    let resolveActiveDone!: () => void
    const activeDone = new Promise<void>((resolve) => {
      resolveActiveDone = resolve
    })
    const end = (emitEnd: boolean): void => {
      if (ended) {
        return
      }
      ended = true
      screencast?.session.stop()
      if (emitEnd && screencast) {
        options.emit({ type: 'end', subscriptionId: screencast.subscriptionId })
      }
    }
    const cancel = (emitEnd = false): void => {
      if (!screencast) {
        cancelledBeforeStart = true
        return
      }
      end(emitEnd)
    }
    const abortScreencast = (): void => cancel()
    const sendBinaryAfterReady = (bytes: Uint8Array<ArrayBufferLike>): boolean | void => {
      if (!readyEmitted) {
        // Why: binary screencast frames are connection-scoped; clients learn the
        // owning subscription from `ready`, so CDP frames must remain unacked
        // until the stream's JSON ready event has been delivered.
        return false
      }
      return options.sendBinary?.(bytes)
    }

    // Why: a phone can rotate before the first stream reaches `ready`, so it
    // has no subscriptionId to unsubscribe. A same-socket replacement cancels
    // and waits here instead of racing the active connection/page gates.
    this.activeBrowserScreencastsByConnection.set(connectionKey, {
      cancel,
      done: activeDone,
      connectionKey
    })
    options.signal?.addEventListener('abort', abortScreencast, { once: true })
    try {
      screencast = await this.browserCommands.browserScreencast(params, {
        sendBinary: sendBinaryAfterReady,
        emit: options.emit
      })
      if (cancelledBeforeStart || options.signal?.aborted) {
        end(false)
        await screencast.session.done
        return
      }
      activeBrowserPageId = screencast.ready.browserPageId
      this.activeBrowserScreencastsByPage.set(activeBrowserPageId, {
        cancel,
        done: activeDone,
        connectionKey
      })
      this.setBrowserDriver(activeBrowserPageId, { kind: 'mobile', clientId: connectionKey })

      // Why: browser screencast frames are connection-scoped media. Registering
      // cleanup ties Page.stopScreencast to the exact remote socket so hidden
      // client panes and dropped connections do not leave Chromium streaming.
      this.registerSubscriptionCleanup(
        screencast.subscriptionId,
        () => end(true),
        options.connectionId
      )
      registeredSubscriptionId = screencast.subscriptionId
      options.emit(screencast.ready)
      readyEmitted = true
      await screencast.session.done
      end(true)
      this.cleanupSubscription(screencast.subscriptionId)
    } finally {
      options.signal?.removeEventListener('abort', abortScreencast)
      if (!ended) {
        end(false)
      }
      if (registeredSubscriptionId) {
        this.cleanupSubscription(registeredSubscriptionId)
      }
      const active = this.activeBrowserScreencastsByConnection.get(connectionKey)
      if (active?.done === activeDone) {
        this.activeBrowserScreencastsByConnection.delete(connectionKey)
      }
      if (activeBrowserPageId) {
        const activePageStream = this.activeBrowserScreencastsByPage.get(activeBrowserPageId)
        if (activePageStream?.done === activeDone) {
          this.activeBrowserScreencastsByPage.delete(activeBrowserPageId)
        }
        const driver = this.getBrowserDriver(activeBrowserPageId)
        if (driver.kind === 'mobile' && driver.clientId === connectionKey) {
          this.setBrowserDriver(activeBrowserPageId, { kind: 'idle' })
        }
      }
      resolveActiveDone()
    }
  }

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

const WAIT_BLOCKED_CHECK_MIN_INTERVAL_MS = 50
// Why: chunks that can complete an actionable prompt bypass the throttle so
// blocked stamps stay per-chunk-immediate; the pattern heads mirror
// findTerminalWaitBlockedSignal. Scanned over the new chunk plus a short
// carry only — never the accumulated window.
const WAIT_BLOCKED_KEYWORD_PATTERN =
  /press enter|press t to trust|do you trust|trust this|trusted workspace|update available|choose working directory|codex just got an upgrade|hooks need review/
const WAIT_BLOCKED_KEYWORD_CARRY_CHARS = 31
const DEFAULT_TERMINAL_LIST_LIMIT = 200
const DEFAULT_WORKTREE_LIST_LIMIT = 200
const DEFAULT_WORKTREE_PS_LIMIT = 200
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
