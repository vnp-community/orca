/* eslint-disable max-lines -- Why: straight extraction of orca-runtime.ts's
terminal-create/split domain (createTerminal, launchAgentTerminal,
splitTerminal, splitPtyBackedTerminal, and their exclusive handle-waiting
helpers), already covered by orca-runtime.ts's own grandfathered max-lines
disable. Registered in config/max-lines-baseline.txt per AGENTS.md - NEEDS
PR REVIEW. */
// frontend/src/main/runtime/orca-runtime-terminal-create.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-073): terminal creation and split
// commands (createTerminal, launchAgentTerminal, splitTerminal,
// splitPtyBackedTerminal) extracted from OrcaRuntimeService via the
// composition pattern, along with the handle-waiting helpers exclusive to
// this cluster (waitForTerminalHandle, waitForLeafPtyId, countLeavesInTab,
// resolveHandleForTab, waitForNewLeafInTab — confirmed via TASK-BIGFILE-052's
// own comment that these are "core createTerminal plumbing", not shared with
// the mobile-session-terminal cluster it extracted). Not in the onPtyData
// hot path, but createTerminal is the single densest method in the file —
// ~30 collaborators across nearly every already-extracted domain — so risk
// comes from breadth of host surface, not call frequency.
import { randomUUID } from 'node:crypto'
import { ipcMain } from 'electron'
import type { BrowserWindow } from 'electron'
import {
  addClaudeTeammateModeAuto,
  addClaudeTeammateModeInProcess,
  type ClaudeAgentTeamsMode
} from '../../shared/claude-agent-teams-tmux-compat'
import { buildClaudeAgentTeamsLaunchPlan } from './claude-agent-teams-shim-env'
import type { ClaudeAgentTeamsService } from './claude-agent-teams-service'
import { isValidHostTerminalTabId } from '../../shared/terminal-tab-id'
import { isTerminalLeafId, makePaneKey, parsePaneKey } from '../../shared/stable-pane-id'
import { SETUP_AGENT_SEQUENCE_STARTUP_COMMAND_ENV } from '../../shared/setup-agent-sequencing'
import { copySleepingAgentLaunchConfig } from './orca-runtime'
import type { SleepingAgentLaunchConfig } from '../../shared/agent-session-resume'
import type { TerminalPaneSplitSource } from '../../shared/feature-education-telemetry'
import type { Repo, TuiAgent } from '../../shared/types'
import type {
  RuntimeTerminalCreate,
  RuntimeTerminalPresentation,
  RuntimeTerminalSplit
} from '../../shared/runtime-types'
import type {
  RuntimeLeafRecord,
  RuntimePtyWorktreeRecord,
  TerminalCreateOptions,
  TerminalHandleRecord,
  TerminalWorkspaceLaunchScope,
  ResolvedWorktree,
  RuntimeStore
} from './orca-runtime'
import type { RuntimeGraphStore } from './orca-runtime-graph-store'
import type { RuntimePtyController } from './orca-runtime-types'
import type { RuntimeMobileSessionTabsCommands } from './orca-runtime-mobile-session-tabs'
import type { RuntimeWorktreeCreationCommands } from './orca-runtime-worktree-creation'

// Why: sibling notifier interfaces (see e.g.
// orca-runtime-mobile-session-terminal.ts's RuntimeMobileSessionTerminalNotifier)
// each declare only the RuntimeNotifier methods they call, since RuntimeNotifier
// itself is not exported from orca-runtime.ts.
type RuntimeTerminalCreateNotifier = {
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
}

export type RuntimeTerminalCreateCommandHost = {
  getStore(): RuntimeStore | null
  getGraph(): RuntimeGraphStore
  getPtyController(): RuntimePtyController | null
  getNotifier(): RuntimeTerminalCreateNotifier | null
  getClaudeAgentTeams(): ClaudeAgentTeamsService
  getAuthoritativeWindow(): BrowserWindow
  getAvailableAuthoritativeWindow(): BrowserWindow | null
  assertGraphReady(): void
  resolveWorktreeSelector(selector: string): Promise<ResolvedWorktree>
  resolveTerminalWorkspaceLaunchScope(selector: string): Promise<TerminalWorkspaceLaunchScope>
  resolveAgentTerminalCreateOptions(
    workspace: TerminalWorkspaceLaunchScope,
    opts: TerminalCreateOptions
  ): Promise<TerminalCreateOptions>
  resolveWorkspaceTerminalStartupCwd(
    workspace: Pick<TerminalWorkspaceLaunchScope, 'path'>,
    requestedCwd?: string | null
  ): string | undefined
  buildTerminalWorkspaceEnv(
    scope: TerminalWorkspaceLaunchScope,
    baseEnv: Record<string, string>,
    paneKey: string,
    tabId: string,
    agentTeamsEnv?: Record<string, string>
  ): Record<string, string>
  createPreAllocatedTerminalHandle(): string
  registerPreAllocatedHandleForPty(ptyId: string, handle: string): void
  registerPty(
    ptyId: string,
    worktreeId: string,
    connectionId?: string | null,
    binding?: { tabId: string; leafId: string }
  ): void
  getOrCreatePtyWorktreeRecord(ptyId: string): RuntimePtyWorktreeRecord | null
  nextTitleObservationSequence(): number
  setPtyManagementTitleFromObservedTitle(
    pty: RuntimePtyWorktreeRecord,
    title: string | null | undefined,
    observedAt: number
  ): void
  issuePtyHandle(pty: RuntimePtyWorktreeRecord): string
  issueHandle(leaf: RuntimeLeafRecord): string
  publishPtyBackedMobileSessionTerminal: RuntimeMobileSessionTabsCommands['publishPtyBackedMobileSessionTerminal']
  persistHeadlessTerminalSplit: RuntimeMobileSessionTabsCommands['persistHeadlessTerminalSplit']
  buildStartupForAgent: RuntimeWorktreeCreationCommands['buildStartupForAgent']
  markLocalWorkspaceTrustedForAgent: RuntimeWorktreeCreationCommands['markLocalWorkspaceTrustedForAgent']
  markRemoteWorkspaceTrustedForAgent: RuntimeWorktreeCreationCommands['markRemoteWorkspaceTrustedForAgent']
  getLivePtyForHandle(
    handle: string
  ): { record: TerminalHandleRecord; pty: RuntimePtyWorktreeRecord } | null
  getLiveLeafForHandle(handle: string): { record: TerminalHandleRecord; leaf: RuntimeLeafRecord }
  resolveLeafForHandle(handle: string): { ptyId: string | null } | null
  getLeafKey(tabId: string, leafId: string): string
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

// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-073): terminal creation, agent
// launch, and split commands — the densest single cluster in
// OrcaRuntimeService, touching nearly every already-extracted domain as a
// host dependency.
export class RuntimeTerminalCreateCommands {
  constructor(private readonly host: RuntimeTerminalCreateCommandHost) {}

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
      opts.rendererBacked === true ? this.host.getAvailableAuthoritativeWindow() : null
    const shouldCreateInBackground =
      worktreeSelector !== undefined &&
      ((!requiresRendererFocus && opts.rendererBacked !== true) ||
        // Why: `orca serve` exposes the local runtime without a renderer
        // window. Renderer-backed Codex terminals are preferred for the app,
        // but headless CLI users still need a usable terminal handle.
        (opts.rendererBacked === true && rendererWindow === null))

    if (shouldCreateInBackground) {
      const ptyController = this.host.getPtyController()
      if (!ptyController?.spawn) {
        throw new Error('runtime_unavailable')
      }
      const workspace = await this.host.resolveTerminalWorkspaceLaunchScope(worktreeSelector)
      const launchOpts = await this.host.resolveAgentTerminalCreateOptions(workspace, opts)
      const cwd =
        this.host.resolveWorkspaceTerminalStartupCwd(workspace, launchOpts.cwd) ?? workspace.path
      const preAllocatedHandle = this.host.createPreAllocatedTerminalHandle()
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
      const claudeAgentTeamsMode = this.host.getStore()?.getSettings?.().claudeAgentTeamsMode
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
          this.host.getClaudeAgentTeams().createLaunchEnv({
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
      const env = this.host.buildTerminalWorkspaceEnv(
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
      const result = await ptyController.spawn({
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
      this.host.registerPreAllocatedHandleForPty(result.id, preAllocatedHandle)
      this.host.registerPty(result.id, workspace.id, workspace.connectionId)
      const pty = this.host.getOrCreatePtyWorktreeRecord(result.id)
      if (pty) {
        if (launchOpts.title) {
          const observedAt = this.host.nextTitleObservationSequence()
          pty.title = launchOpts.title
          pty.titleUpdatedAt = observedAt
          this.host.setPtyManagementTitleFromObservedTitle(pty, launchOpts.title, observedAt)
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
      const handle = pty ? this.host.issuePtyHandle(pty) : preAllocatedHandle
      if (pty && launchOpts.deferMobileSessionPublish !== true) {
        this.host.publishPtyBackedMobileSessionTerminal(workspace.id, pty, {
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
      if (presentation !== 'background' && this.host.getNotifier()?.revealTerminalSession) {
        try {
          // Why: after the PTY is spawned, renderer tab adoption is best-effort;
          // failing here must not strand a live process without returning a handle.
          // Pass the pre-minted tabId so the renderer adopts under the same id
          // already baked into the PTY env — keeps paneKey hook attribution intact.
          await this.host.getNotifier()!.revealTerminalSession!(workspace.id, {
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

    this.host.assertGraphReady()
    const win = rendererWindow ?? this.host.getAuthoritativeWindow()
    // Why: mirrors browserTabCreate — when no worktree is specified, pass
    // undefined so the renderer uses its current active worktree.
    const workspace = worktreeSelector
      ? await this.host.resolveTerminalWorkspaceLaunchScope(worktreeSelector)
      : null
    const launchOpts = workspace
      ? await this.host.resolveAgentTerminalCreateOptions(workspace, opts)
      : opts
    const worktreeId = workspace?.id
    const cwd = workspace
      ? this.host.resolveWorkspaceTerminalStartupCwd(workspace, launchOpts.cwd)
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
    const worktree = await this.host.resolveWorktreeSelector(worktreeSelector)
    const repo: Repo | undefined = this.host.getStore()?.getRepo(worktree.repoId)
    if (!repo) {
      throw new Error('Repository for the selected workspace is no longer available.')
    }
    const startup = this.host.buildStartupForAgent(repo, opts.agent, opts.prompt)
    if (repo.connectionId) {
      await this.host.markRemoteWorkspaceTrustedForAgent(
        opts.agent,
        repo.connectionId,
        worktree.path
      )
    } else {
      this.host.markLocalWorkspaceTrustedForAgent(opts.agent, worktree.path)
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

  private waitForTerminalHandle(tabId: string, timeoutMs = 10_000): Promise<string> {
    const existing = this.resolveHandleForTab(tabId)
    if (existing) {
      return Promise.resolve(existing)
    }

    return new Promise<string>((resolve, reject) => {
      const timer = setTimeout(() => {
        const idx = this.host.getGraph().graphSyncCallbacks.indexOf(check)
        if (idx !== -1) {
          this.host.getGraph().graphSyncCallbacks.splice(idx, 1)
        }
        reject(new Error('Timed out waiting for terminal handle after creation'))
      }, timeoutMs)

      const check = (): void => {
        const handle = this.resolveHandleForTab(tabId)
        if (handle) {
          clearTimeout(timer)
          const idx = this.host.getGraph().graphSyncCallbacks.indexOf(check)
          if (idx !== -1) {
            this.host.getGraph().graphSyncCallbacks.splice(idx, 1)
          }
          resolve(handle)
        }
      }
      this.host.getGraph().graphSyncCallbacks.push(check)
      // Why: the graph sync may have fired between the initial check and
      // callback registration. Re-check immediately to avoid a missed wake-up.
      check()
    })
  }

  // Why: mobile clients may subscribe before the PTY spawns (the left pane
  // of a new workspace). Instead of bailing with a bare scrollback+end,
  // wait for the PTY to appear so the subscribe can proceed with phone-fit.
  waitForLeafPtyId(handle: string, timeoutMs = 10_000, signal?: AbortSignal): Promise<string> {
    const leaf = this.host.resolveLeafForHandle(handle)
    if (leaf?.ptyId) {
      return Promise.resolve(leaf.ptyId)
    }

    // Why: when the ptyId changes from null to a real value, the old handle
    // is invalidated (deleted from this.graph.handles). Capture the tabId+leafId
    // now so we can look up the leaf directly even after handle invalidation.
    const record = this.host.getGraph().handles.get(handle)
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
        const idx = this.host.getGraph().graphSyncCallbacks.indexOf(check)
        if (idx !== -1) {
          this.host.getGraph().graphSyncCallbacks.splice(idx, 1)
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
        let ptyId = this.host.resolveLeafForHandle(handle)?.ptyId
        // Why: when ptyId transitions null→real, issueHandle invalidates the
        // old handle. Fall back to direct leaf lookup by the saved coordinates.
        if (!ptyId && savedTabId && savedLeafId) {
          const directLeaf = this.host
            .getGraph()
            .leaves.get(this.host.getLeafKey(savedTabId, savedLeafId))
          ptyId = directLeaf?.ptyId ?? null
        }
        if (ptyId) {
          finish(ptyId)
        }
      }
      this.host.getGraph().graphSyncCallbacks.push(check)
      check()
    })
  }

  // Why: a leaf appears in the graph before its PTY spawns. If we issue a
  // handle while ptyId is null, the next graph sync after PTY spawn will
  // change ptyId and invalidate the handle. Wait for a connected PTY so
  // the handle is stable and immediately usable for send/read/wait.
  private resolveHandleForTab(tabId: string): string | null {
    for (const leaf of this.host.getGraph().leaves.values()) {
      if (leaf.tabId === tabId && leaf.ptyId !== null) {
        return this.host.issueHandle(leaf)
      }
    }
    return null
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
    const livePty = this.host.getLivePtyForHandle(handle)
    if (livePty) {
      return await this.splitPtyBackedTerminal(livePty.pty, opts)
    }
    this.host.assertGraphReady()
    const { leaf } = this.host.getLiveLeafForHandle(handle)
    const direction = opts.direction ?? 'horizontal'

    // Why: snapshot current leaf keys for this tab so we can detect the new
    // pane that appears after the split via graph sync delta.
    const leafKeysBefore = new Set<string>()
    for (const [key, l] of this.host.getGraph().leaves) {
      if (l.tabId === leaf.tabId) {
        leafKeysBefore.add(key)
      }
    }

    this.host.getNotifier()?.splitTerminal(leaf.tabId, leaf.paneRuntimeId, {
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
    const ptyController = this.host.getPtyController()
    if (!ptyController?.spawn) {
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
    const workspace = await this.host.resolveTerminalWorkspaceLaunchScope(`id:${pty.worktreeId}`)
    const leafId = randomUUID()
    const preAllocatedHandle = this.host.createPreAllocatedTerminalHandle()
    const paneKey = makePaneKey(parentTabId, leafId)
    const result = await ptyController.spawn({
      cols: 120,
      rows: 40,
      cwd: workspace.path,
      command: opts.command,
      commandDelivery: 'provider',
      env: this.host.buildTerminalWorkspaceEnv(workspace, opts.env ?? {}, paneKey, parentTabId),
      envToDelete: opts.envToDelete,
      connectionId: workspace.connectionId,
      worktreeId: workspace.id,
      preAllocatedHandle
    })
    this.host.registerPreAllocatedHandleForPty(result.id, preAllocatedHandle)
    this.host.registerPty(result.id, workspace.id, workspace.connectionId)
    const createdPty = this.host.getOrCreatePtyWorktreeRecord(result.id)
    if (createdPty) {
      createdPty.tabId = parentTabId
      createdPty.paneKey = paneKey
    }

    try {
      await this.host.getNotifier()?.revealTerminalSession?.(workspace.id, {
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
      ptyController.kill?.(result.id)
      throw error
    }
    if (createdPty) {
      this.host.publishPtyBackedMobileSessionTerminal(workspace.id, createdPty, {
        tabId: parentTabId,
        leafId,
        title: null,
        activate: opts.activate !== false,
        split: { splitFromLeafId: parsedPaneKey.leafId, direction }
      })
      // Why: persist the split into the workspace session so a later snapshot
      // rebuild keeps it instead of collapsing back to a single pane.
      this.host.persistHeadlessTerminalSplit({
        tabId: parentTabId,
        leafId,
        ptyId: createdPty.ptyId,
        splitFromLeafId: parsedPaneKey.leafId,
        direction
      })
    }

    return {
      handle: this.host.issuePtyHandle(createdPty ?? pty),
      tabId: parentTabId,
      paneRuntimeId: -1
    }
  }

  private waitForNewLeafInTab(
    tabId: string,
    existingLeafKeys: Set<string>,
    timeoutMs = 10_000
  ): Promise<string> {
    const tryResolve = (): string | null => {
      for (const [key, leaf] of this.host.getGraph().leaves) {
        if (leaf.tabId === tabId && !existingLeafKeys.has(key) && leaf.ptyId !== null) {
          return this.host.issueHandle(leaf)
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
        const idx = this.host.getGraph().graphSyncCallbacks.indexOf(check)
        if (idx !== -1) {
          this.host.getGraph().graphSyncCallbacks.splice(idx, 1)
        }
        reject(new Error('Timed out waiting for split pane handle'))
      }, timeoutMs)

      const check = (): void => {
        const handle = tryResolve()
        if (handle) {
          clearTimeout(timer)
          const idx = this.host.getGraph().graphSyncCallbacks.indexOf(check)
          if (idx !== -1) {
            this.host.getGraph().graphSyncCallbacks.splice(idx, 1)
          }
          resolve(handle)
        }
      }
      this.host.getGraph().graphSyncCallbacks.push(check)
      check()
    })
  }
}
