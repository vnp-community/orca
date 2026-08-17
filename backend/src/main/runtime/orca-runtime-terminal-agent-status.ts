/* eslint-disable max-lines -- Why: straight extraction of orca-runtime.ts's
terminal agent-status domain (getTerminalAgentStatus, isTerminalRunningAgent,
pane-key resolution, and orchestration-context lookup — 22 methods scattered
across ~2,700 lines of the source file, unified by a shared concern: "what is
this terminal's agent doing, and what pane/handle identifies it"), already
covered by orca-runtime.ts's own grandfathered max-lines disable. Registered
in config/max-lines-baseline.txt per AGENTS.md - NEEDS PR REVIEW. */
// frontend/src/main/runtime/orca-runtime-terminal-agent-status.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-076): terminal agent-status
// resolution (getTerminalAgentStatus, isTerminalRunningAgent) and pane-key/
// orchestration-context lookup (getPaneKeyForTerminalHandle,
// getTerminalHandleForPaneKey, getAgentStatusOrchestrationContextForHandle,
// ...) extracted from OrcaRuntimeService via the composition pattern. Was
// organically scattered across 5 non-contiguous segments (2780-2799,
// 2862-3030, 3071-3110, 5060-5095, 5136-5245, 5279-5439 in the pre-move
// source) - excludes shouldDelayPtyBackedMobileSnapshotForForegroundAgent,
// getOrchestrationDbIfAvailable, buildAgentOrchestrationByPaneKey,
// setPtyManagementTitleFromObservedTitle, and nextTitleObservationSequence,
// which sit textually adjacent but are used by other already-extracted
// domains (confirmed via usage audit, not proximity).
import type { AgentStatus } from '../../shared/agent-detection'
import { detectAgentStatusFromTitle, isShellProcess } from '../../shared/agent-detection'
import {
  isAgentForegroundWrapperProcess,
  isExpectedAgentProcess,
  recognizeAgentProcess
} from '../../shared/agent-process-recognition'
import {
  AGENT_STATUS_STALE_AFTER_MS,
  type AgentStatusEntry,
  type AgentStatusIpcPayload,
  type AgentStatusOrchestrationContext
} from '../../shared/agent-status-types'
import { isTerminalLeafId, makePaneKey, parsePaneKey } from '../../shared/stable-pane-id'
import type { SleepingAgentLaunchConfig } from '../../shared/agent-session-resume'
import type {
  RuntimeTerminalAgentStatus,
  RuntimeTerminalResolvePane
} from '../../shared/runtime-types'
import {
  buildTerminalWaitText,
  classifyAgentTitle,
  classifyLatestAgentTitle,
  detectTerminalWaitBlockedReason,
  getLatestAgentCandidateTitle,
  getLatestAgentCandidateTitleInfo,
  getLatestLeafTitle,
  getTerminalState,
  isKnownReadyPromptPreview,
  mapExplicitAgentStateToRuntimeTerminalStatus,
  terminalTitleBlocksExplicitAgentStatus
} from './orca-runtime-tail-buffer'
import { copySleepingAgentLaunchConfig } from './orca-runtime'
import type {
  RuntimeAgentRowSnapshot,
  RuntimeLeafRecord,
  RuntimePtyWorktreeRecord,
  TerminalHandleRecord
} from './orca-runtime'
import type { RuntimeGraphStore } from './orca-runtime-graph-store'
import type { RuntimePtyController } from './orca-runtime-types'
// ADR-021 — "chỉ dùng 1 database": OrchestrationDb (SQLite, sync) →
// PgOrchestrationDb (Postgres, async). Unlike every other file this migration
// touched, the two methods below (getAgentStatusOrchestrationContextForHandle,
// getRecentCompletedDispatchForTerminal) are called from a SYNCHRONOUS chain
// (buildAgentOrchestrationByPaneKey → orca-runtime-sync-window-graph.ts's
// syncWindowGraph(), itself synchronous, and its own caller chain wasn't
// traced) — cascading `async` through that safely needs its own reviewed
// change, not a mechanical add-await inside an already very large ADR-021
// pass. Deliberately degraded (not attempted) here: see the two methods'
// doc comments.
import type { PgOrchestrationDb } from './orchestration/pg-db'

const FOREGROUND_AGENT_WRAPPER_RETRY_INTERVAL_MS = 150
const FOREGROUND_AGENT_WRAPPER_RETRY_TIMEOUT_MS = 6_500

export type RuntimeTerminalAgentStatusCommandHost = {
  getGraph(): RuntimeGraphStore
  getPtyController(): RuntimePtyController | null
  getLivePtyForHandle(
    handle: string
  ): { record: TerminalHandleRecord; pty: RuntimePtyWorktreeRecord } | null
  getLiveLeafForHandle(handle: string): { record: TerminalHandleRecord; leaf: RuntimeLeafRecord }
  getOrchestrationDbIfAvailable(): PgOrchestrationDb | null
  getLeafKey(tabId: string, leafId: string): string
  getRuntimeId(): string
  getLatestAgentStatusByPaneKey(): Map<string, RuntimeAgentRowSnapshot>
  issuePtyHandle(pty: RuntimePtyWorktreeRecord): string
  issueHandle(leaf: RuntimeLeafRecord): string
  getLeavesForPty(ptyId: string): RuntimeLeafRecord[]
  getAgentStatusSnapshot(): AgentStatusIpcPayload[]
}

// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-076): "what is this terminal's
// agent doing, and what pane/handle identifies it" - the two concerns are
// entangled in the source (agent-status lookups resolve through pane keys,
// pane-key resolution feeds orchestration-context lookups) so they moved
// together rather than being split further.
export class RuntimeTerminalAgentStatusCommands {
  constructor(private readonly host: RuntimeTerminalAgentStatusCommandHost) {}

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
    const record = this.host.getGraph().handles.get(handle)
    const parsed = parsePaneKey(paneKey)
    return {
      handle,
      tabId: record?.tabId ?? parsed?.tabId ?? '',
      leafId: record?.leafId ?? parsed?.leafId ?? '',
      ptyId: record?.ptyId ?? null
    }
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
    const pty = this.host.getLivePtyForHandle(handle)
    if (pty) {
      if (!pty.pty.connected) {
        throw new Error('terminal_gone')
      }
      return pty.pty.ptyId
    }
    const { leaf } = this.host.getLiveLeafForHandle(handle)
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
    const pty = this.host.getLivePtyForHandle(handle)
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

    const { leaf } = this.host.getLiveLeafForHandle(handle)
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
      { title: this.host.getGraph().tabs.get(leaf.tabId)?.title, updatedAt: 0 }
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
    const ptyController = this.host.getPtyController()
    if (!ptyController) {
      return false
    }
    let foregroundProcess: string | null
    try {
      foregroundProcess = await ptyController.getForegroundProcess(ptyId)
    } catch {
      this.assertTerminalAgentStatusPtyBinding(handle, ptyId)
      return false
    }
    this.assertTerminalAgentStatusPtyBinding(handle, ptyId)
    if (!foregroundProcess || !isShellProcess(foregroundProcess)) {
      return false
    }
    const confirmationController = this.host.getPtyController()
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
      const retained = this.host.getLatestAgentStatusByPaneKey().get(paneKey)
      consider(retained?.payload.state, retained?.updatedAt)
    }

    for (const entry of this.host.getAgentStatusSnapshot()) {
      if (entry.terminalHandle !== handle && (!paneKey || entry.paneKey !== paneKey)) {
        continue
      }
      consider(entry.state, entry.receivedAt)
    }

    return bestStatus ? { status: bestStatus, updatedAt: bestUpdatedAt } : null
  }

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

  // ADR-021 — "chỉ dùng 1 database": deliberately degraded, not converted.
  // This method is synchronous and reached from a synchronous caller chain
  // (buildAgentOrchestrationByPaneKey → syncWindowGraph()) whose own callers
  // weren't traced as part of this pass — see this file's PgOrchestrationDb
  // import comment. PgOrchestrationDb's methods are all async (Postgres I/O),
  // so a sync function structurally cannot call them and use the result;
  // the only correct-by-construction choice here is to return `undefined`
  // (same as "no orchestration DB available", which every caller already
  // handles — see e.g. orca-runtime.ts's buildAgentOrchestrationByPaneKey
  // `if (context) {...}` checks) rather than cascading `async` through a
  // chain that hasn't been reviewed for it.
  getAgentStatusOrchestrationContextForHandle(
    _handle: string,
    _db = this.host.getOrchestrationDbIfAvailable()
  ): AgentStatusOrchestrationContext | undefined {
    return undefined
  }

  getTerminalHandleForPaneKey(paneKey: string): string | null {
    const parsed = parsePaneKey(paneKey)
    if (parsed) {
      const leaf = this.host
        .getGraph()
        .leaves.get(this.host.getLeafKey(parsed.tabId, parsed.leafId))
      if (leaf?.ptyId) {
        return this.host.issueHandle(leaf)
      }
    }
    for (const pty of this.host.getGraph().ptysById.values()) {
      if (pty.paneKey === paneKey) {
        return this.host.issuePtyHandle(pty)
      }
    }
    return null
  }

  private getPtyRecordForPaneKey(paneKey: string): RuntimePtyWorktreeRecord | null {
    const parsed = parsePaneKey(paneKey)
    if (parsed) {
      const leaf = this.host
        .getGraph()
        .leaves.get(this.host.getLeafKey(parsed.tabId, parsed.leafId))
      const pty = leaf?.ptyId ? this.host.getGraph().ptysById.get(leaf.ptyId) : undefined
      if (pty) {
        return pty
      }
    }
    for (const pty of this.host.getGraph().ptysById.values()) {
      if (pty.paneKey === paneKey) {
        return pty
      }
    }
    return null
  }

  private getPaneKeyForTerminalHandle(handle: string): string | null {
    const livePty = this.host.getLivePtyForHandle(handle)
    if (livePty?.pty.paneKey) {
      return livePty.pty.paneKey
    }
    const record = this.host.getGraph().handles.get(handle)
    if (!record || record.runtimeId !== this.host.getRuntimeId()) {
      return null
    }
    if (!isTerminalLeafId(record.leafId)) {
      return null
    }
    return makePaneKey(record.tabId, record.leafId)
  }

  async isTerminalRunningAgent(handle: string): Promise<boolean> {
    try {
      const pty = this.host.getLivePtyForHandle(handle)
      if (pty) {
        const leaf = this.getPrimaryLeafForPty(pty.pty.ptyId)
        return await this.isPtyRunningAgent(pty.pty, leaf)
      }
      const { leaf } = this.host.getLiveLeafForHandle(handle)
      // Why: check both the leaf-level pane title (synced from the renderer's
      // runtimePaneTitlesByTabId) and the tab-level title. The tab title already
      // includes OSC-enriched agent indicators (e.g. ✳ prefix) synced from the
      // renderer's xterm instance.
      const paneTitle = getLatestLeafTitle(leaf, null)
      const paneTitleClassification = classifyAgentTitle(paneTitle)
      if (paneTitleClassification === 'agent') {
        return true
      }
      const tabTitle = this.host.getGraph().tabs.get(leaf.tabId)?.title?.trim() || null
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
      const ptyController = this.host.getPtyController()
      if (!leaf.ptyId || !ptyController) {
        return false
      }
      const fg = await ptyController.getForegroundProcess(leaf.ptyId)
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
    const ptyController = this.host.getPtyController()
    if (!ptyController) {
      return false
    }
    const fg = await ptyController.getForegroundProcess(pty.ptyId)
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
    const ptyController = this.host.getPtyController()
    if (!this.isAgentWrapperForegroundProcess(foregroundProcess) || !ptyController) {
      return false
    }
    const startedAt = Date.now()
    while (Date.now() - startedAt < FOREGROUND_AGENT_WRAPPER_RETRY_TIMEOUT_MS) {
      await new Promise((resolve) =>
        setTimeout(resolve, FOREGROUND_AGENT_WRAPPER_RETRY_INTERVAL_MS)
      )
      const refreshedProcess = await ptyController.getForegroundProcess(ptyId)
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
    return this.host.getLeavesForPty(ptyId)[0] ?? null
  }
}
