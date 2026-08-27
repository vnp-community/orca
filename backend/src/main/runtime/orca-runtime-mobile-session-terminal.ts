/* eslint-disable max-lines -- Why: straight extraction of orca-runtime.ts's
mobile-session-terminal creation command block, already covered by
orca-runtime.ts's own grandfathered max-lines disable before this move.
Registered in config/max-lines-baseline.txt per AGENTS.md — NEEDS PR
REVIEW. */
/* eslint-disable unicorn/no-useless-spread -- Why: graphSyncCallbacks is
cloned intentionally before iterating so a callback that unsubscribes
itself mid-drain can safely mutate the underlying array — same pattern as
orca-runtime.ts's own grandfathered disable for waiter sets. */
// frontend/src/main/runtime/orca-runtime-mobile-session-terminal.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-052): mobile-session-terminal
// creation commands extracted from OrcaRuntimeService via the composition
// pattern. This is cluster 2 of 3 mobile-session-tabs clusters —
// listAllMobileSessionTabs (cluster 1, TASK-BIGFILE-051) and
// notifyMobileSessionTabsChanged (cluster 3, not yet extracted) are
// siblings. waitForTerminalHandle/resolveHandleForTab/waitForLeafPtyId/
// countLeavesInTab sit textually adjacent but are core createTerminal
// plumbing (used by the desktop create path too) — deliberately excluded,
// stay in orca-runtime.ts.
import { randomUUID } from 'node:crypto'
import { ipcMain } from 'electron'
import type { TuiAgent, WorktreeStartupLaunch } from '../../shared/types'
import type { SleepingAgentLaunchConfig } from '../../shared/agent-session-resume'
import type {
  RuntimeMobileSessionCreateTerminalResult,
  RuntimeMobileSessionTabsResult,
  RuntimeMobileSessionTabsSnapshot,
  RuntimeMobileSessionTerminalTab,
  RuntimeTerminalCreate
} from '../../shared/runtime-types'
import { repoIsRemote } from '../../shared/agent-launch-remote'
import { parsePaneKey } from '../../shared/stable-pane-id'
import {
  resolveTuiAgentLaunchArgs,
  resolveTuiAgentLaunchEnv
} from '../../shared/tui-agent-launch-defaults'
import { isTuiAgentEnabled } from '../../shared/tui-agent-selection'
import { buildAgentStartupPlan } from '../../shared/tui-agent-startup'
import { resolveLocalWindowsAgentStartupShell } from '../../shared/windows-terminal-shell'
import type {
  RuntimePtyWorktreeRecord,
  RuntimeStore,
  TerminalCreateOptions,
  TerminalWorkspaceLaunchScope
} from './orca-runtime'
import type { RuntimeGraphStore } from './orca-runtime-graph-store'
import type { BrowserWindow } from 'electron'

// Why: long enough for a phone to reconnect and retry a create whose response
// was lost, short enough that an intentional later re-resume forks fresh.
const MOBILE_TERMINAL_CREATE_RESULT_TTL_MS = 60_000
const MOBILE_TERMINAL_SURFACE_TIMEOUT_MS = 10_000
const MOBILE_TERMINAL_READY_FALLBACK_MS = 1000

function isClientDisconnectedError(error: unknown): boolean {
  return error instanceof Error && error.message === 'client_disconnected'
}

type RuntimeMobileSessionTerminalNotifier = {
  focusTerminal(tabId: string, worktreeId: string, leafId?: string | null): void
  closeTerminal(tabId: string, paneRuntimeId?: number): void
}

export type RuntimeMobileSessionTerminalCommandHost = {
  getStore(): RuntimeStore | null
  getNotifier(): RuntimeMobileSessionTerminalNotifier | null
  getGraph(): RuntimeGraphStore
  getAvailableAuthoritativeWindow(): BrowserWindow | null
  getLivePtyForHandle(handle: string): {
    pty: RuntimePtyWorktreeRecord
  } | null
  createTerminal(
    worktreeSelector?: string,
    opts?: TerminalCreateOptions
  ): Promise<RuntimeTerminalCreate>
  assertGraphReady(): void
  markLocalWorkspaceTrustedForAgent(agent: TuiAgent, workspacePath: string): void
  markRemoteWorkspaceTrustedForAgent(
    agent: TuiAgent,
    connectionId: string,
    workspacePath: string
  ): Promise<void>
  getMobileSessionTabsByWorktree(): Map<string, RuntimeMobileSessionTabsSnapshot>
  getMobileSessionTabListeners(): Set<(snapshot: RuntimeMobileSessionTabsResult) => void>
  // Why: shared with listAllMobileSessionTabs (cluster 1) and
  // notifyMobileSessionTabsChanged (cluster 3, not yet extracted) — stays
  // in orca-runtime.ts.
  hydrateHeadlessMobileSessionTabsFromWorkspaceSession(
    worktreeId?: string,
    options?: {
      force?: boolean
      allowAttachedWindow?: boolean
      onlyServeOwnedTerminals?: boolean
    }
  ): void
  resolveTerminalWorkspaceLaunchScope(selector: string): Promise<TerminalWorkspaceLaunchScope>
  resolveWorkspaceTerminalStartupCwd(
    workspace: TerminalWorkspaceLaunchScope,
    requestedCwd: string | undefined
  ): string | undefined
  getAgentLaunchPlatformForWorkspace(workspace: TerminalWorkspaceLaunchScope): NodeJS.Platform
  buildMaterializedHeadlessParentLayout(
    leafId: string,
    ptyId: string,
    existingLayout: RuntimeMobileSessionTerminalTab['parentLayout']
  ): RuntimeMobileSessionTerminalTab['parentLayout']
  getHeadlessMobileSessionGroupId(worktreeId: string): string
  buildHeadlessMobileSessionTabGroups(
    worktreeId: string,
    tabs: RuntimeMobileSessionTabsSnapshot['tabs'],
    activeTab: RuntimeMobileSessionTabsSnapshot['tabs'][number] | null,
    existingGroups: RuntimeMobileSessionTabsSnapshot['tabGroups'] | undefined,
    targetAssignment?: { tabId: string; groupId: string }
  ): RuntimeMobileSessionTabsSnapshot['tabGroups']
  toMobileSessionTabsResult(
    snapshot: RuntimeMobileSessionTabsSnapshot
  ): RuntimeMobileSessionTabsResult
  publishPtyBackedMobileSessionTerminal(
    worktreeId: string,
    pty: RuntimePtyWorktreeRecord,
    args: {
      tabId: string
      leafId: string
      title: string | null
      activate: boolean
      selectIfNoActiveTab: boolean
    }
  ): void
}

export class RuntimeMobileSessionTerminalCommands {
  constructor(private readonly host: RuntimeMobileSessionTerminalCommandHost) {}

  // Why: idempotency map for mobile terminal creation — a retried create with the
  // same clientMutationId returns the in-flight operation instead of duplicating.
  private mobileTerminalCreateByMutationId = new Map<
    string,
    Promise<RuntimeMobileSessionCreateTerminalResult>
  >()
  // Why: register the wait before the renderer's PTY spawn arrives so that
  // spawn (registerPty) can publish the pty-backed surface main-side even if
  // graph-sync is stalled (#7587). See ensurePtyBackedMobileSurfaceForRendererTab.
  private pendingMobileTerminalCreatesByKey = new Map<
    string,
    { activate: boolean; selectIfNoActiveTab: boolean }
  >()

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
    this.host.assertGraphReady()
    const workspace = await this.host.resolveTerminalWorkspaceLaunchScope(worktreeSelector)
    const worktreeId = workspace.id
    const cwd = this.host.resolveWorkspaceTerminalStartupCwd(workspace, opts.cwd)
    this.host.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(worktreeId)
    let afterDesktopTabId: string | undefined
    if (opts.afterTabId) {
      const snapshot = this.host.getMobileSessionTabsByWorktree().get(worktreeId)
      const anchor = snapshot?.tabs.find((tab) => tab.id === opts.afterTabId)
      if (!anchor) {
        throw new Error('after_tab_not_found')
      }
      afterDesktopTabId = anchor.type === 'terminal' ? anchor.parentTabId : anchor.id
    }
    const startupCommand = await this.resolveMobileSessionTerminalCommand(workspace, opts)

    const win = this.host.getAvailableAuthoritativeWindow()
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
      this.host.getNotifier()?.focusTerminal(reply.tabId, worktreeId, null)
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
      this.host.getNotifier()?.closeTerminal(reply.tabId)
      throw error
    } finally {
      this.pendingMobileTerminalCreatesByKey.delete(pendingCreateKey)
    }
  }

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs cluster 1/3 host wiring, or core PTY registration) — public, not private.
  async resolveMobileSessionTerminalCommand(
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
    const store = this.host.getStore()
    if (!store) {
      throw new Error('runtime_unavailable')
    }
    const settings = store.getSettings()
    if (!isTuiAgentEnabled(opts.agent, settings.disabledTuiAgents)) {
      throw new Error('Selected agent is disabled. Choose an enabled agent before creating.')
    }
    // Why: mobile may be running on iOS while the actual terminal shell is
    // Windows/macOS/Linux or an SSH Linux host; quote for the host shell.
    const platform = this.host.getAgentLaunchPlatformForWorkspace(workspace)
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
      await this.host.markRemoteWorkspaceTrustedForAgent(
        opts.agent,
        workspace.connectionId,
        workspace.path
      )
    } else {
      this.host.markLocalWorkspaceTrustedForAgent(opts.agent, workspace.path)
    }
    return {
      command: startupPlan.launchCommand,
      env: startupPlan.env,
      launchConfig: startupPlan.launchConfig,
      launchAgent: opts.agent,
      startupCommandDelivery: startupPlan.startupCommandDelivery
    }
  }

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs cluster 1/3 host wiring, or core PTY registration) — public, not private.
  async createHeadlessMobileSessionTerminal(
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
    const workspace = await this.host.resolveTerminalWorkspaceLaunchScope(`id:${worktreeId}`)
    const cwd = this.host.resolveWorkspaceTerminalStartupCwd(workspace, opts.cwd)
    // Why: SshPtyProvider treats sessionId as a relay reattach request. Only
    // synthesize local serve ids; SSH fresh terminals must call pty.spawn.
    const stableSessionId =
      opts.identity?.sessionId ?? (workspace.connectionId ? undefined : `serve-${randomUUID()}`)
    const terminal = await this.host.createTerminal(`id:${worktreeId}`, {
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
    const livePty = this.host.getLivePtyForHandle(terminal.handle)
    if (!livePty) {
      throw new Error('terminal_handle_stale')
    }
    const parentTabId = livePty.pty.tabId ?? `pty:${livePty.pty.ptyId}`
    const leafId = parsePaneKey(livePty.pty.paneKey ?? '')?.leafId ?? randomUUID()
    const existing = this.host.getMobileSessionTabsByWorktree().get(worktreeId)
    const existingSurface =
      existing?.tabs.find(
        (candidate): candidate is RuntimeMobileSessionTerminalTab =>
          candidate.type === 'terminal' &&
          candidate.parentTabId === parentTabId &&
          candidate.leafId === leafId
      ) ?? null
    const parentLayout = this.host.buildMaterializedHeadlessParentLayout(
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
          : (existing?.activeGroupId ?? this.host.getHeadlessMobileSessionGroupId(worktreeId)),
      activeTabId: activate ? tab.id : (existing?.activeTabId ?? null),
      activeTabType: activate ? 'terminal' : (existing?.activeTabType ?? null),
      tabGroups: this.host.buildHeadlessMobileSessionTabGroups(
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
    this.host.getMobileSessionTabsByWorktree().set(worktreeId, next)
    const result = this.host.toMobileSessionTabsResult(next)
    for (const listener of this.host.getMobileSessionTabListeners()) {
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
        const idx = this.host.getGraph().graphSyncCallbacks.indexOf(check)
        if (idx !== -1) {
          this.host.getGraph().graphSyncCallbacks.splice(idx, 1)
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
      this.host.getGraph().graphSyncCallbacks.push(check)
      check()
    })
  }

  private findMobileTerminalSurface(
    worktreeId: string,
    parentTabId: string,
    options: { requireReady?: boolean } = {}
  ): RuntimeMobileSessionCreateTerminalResult | null {
    const snapshot = this.host.getMobileSessionTabsByWorktree().get(worktreeId)
    if (!snapshot) {
      return null
    }
    const result = this.host.toMobileSessionTabsResult(snapshot)
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
  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs cluster 1/3 host wiring, or core PTY registration) — public, not private.
  ensurePtyBackedMobileSurfaceForRendererTab(
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
    this.host.publishPtyBackedMobileSessionTerminal(worktreeId, pty, {
      tabId,
      leafId,
      title: null,
      activate: pending.activate,
      selectIfNoActiveTab: pending.selectIfNoActiveTab
    })
    // Why: waitForMobileTerminalSurface's check closures are drained only inside
    // syncWindowGraph; a main-side publish must drain them too or the pending
    // wait won't observe the insertion (mirrors syncWindowGraph's drain).
    for (const cb of [...this.host.getGraph().graphSyncCallbacks]) {
      cb()
    }
    return this.findMobileTerminalSurface(worktreeId, tabId)
  }

  private findLiveRegisteredPtyForRendererTab(
    worktreeId: string,
    tabId: string
  ): RuntimePtyWorktreeRecord | null {
    for (const pty of this.host.getGraph().ptysById.values()) {
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
    for (const pty of this.host.getGraph().ptysById.values()) {
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
}
