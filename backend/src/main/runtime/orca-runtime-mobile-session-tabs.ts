/* eslint-disable max-lines -- Why: straight extraction of orca-runtime.ts's
headless mobile-session-tabs materialization/sync cluster (listAllMobileSessionTabs
and ~48 dedicated private helpers), already covered by orca-runtime.ts's own
grandfathered max-lines disable before this move. Registered in
config/max-lines-baseline.txt per AGENTS.md — NEEDS PR REVIEW. This is the
first of 3 mobile-session-tabs clusters (createMobileSessionTerminal and
notifyMobileSessionTabsChanged remain in orca-runtime.ts, deferred to
follow-on tasks) — the largest and riskiest, zero test coverage, extracted
only after the user explicitly accepted that risk. */
// frontend/src/main/runtime/orca-runtime-mobile-session-tabs.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-051): headless mobile-session-tabs
// materialization/sync commands extracted from OrcaRuntimeService via the
// composition pattern. The pre-existing fileCommands/gitCommands composition
// blocks (unrelated domains from TASK-BIGFILE-008/009) sit textually in the
// middle of the original range — deliberately excluded, stay in
// orca-runtime.ts untouched.
import { randomUUID, createHash } from 'node:crypto'
import type {
  Repo,
  Tab,
  TabGroupLayoutNode,
  TerminalLayoutSnapshot,
  TerminalPaneLayoutNode,
  TerminalTab,
  TuiAgent,
  WorkspaceSessionState,
  FolderWorkspace,
  WorktreeStartupLaunch
} from '../../shared/types'
import type { SleepingAgentLaunchConfig } from '../../shared/agent-session-resume'
import type {
  BrowserTabInfo,
  RuntimeMarkdownReadTabResult,
  RuntimeMarkdownSaveTabResult,
  RuntimeMobileSessionBrowserTab,
  RuntimeMobileSessionCreateTerminalResult,
  RuntimeMobileSessionSnapshotTab,
  RuntimeMobileSessionTabGroup,
  RuntimeMobileSessionTabMove,
  RuntimeMobileSessionTabMoveResult,
  RuntimeMobileSessionTabsResult,
  RuntimeMobileSessionTabsSnapshot,
  RuntimeMobileSessionTerminalTab
} from '../../shared/runtime-types'
import { normalizeCompatibleAgentTitleForOwner } from '../../shared/agent-title-owner'
import { getRepoProviderConnectionKey } from '../../shared/execution-host'
import { parseAppSshPtyId } from '../../shared/ssh-pty-id'
import { isTerminalLeafId } from '../../shared/stable-pane-id'
import { getLocalProjectWorktreeGitOptions } from '../project-runtime-git-options'
import {
  buildHeadlessTabGroupMove,
  buildHeadlessTabGroupSplit
} from './headless-tab-group-split-layout'
import { buildHeadlessTerminalSplitLayout } from './headless-terminal-split-layout'
import { getLatestPtyTitle } from './orca-runtime-tail-buffer'
import type { MobileSessionTabsNotifyCoalescer } from './mobile-session-tabs-notify-coalescer'
import type {
  ResolvedWorktree,
  RuntimePtyWorktreeRecord,
  RuntimeStore,
  TerminalWorkspaceLaunchScope
} from './orca-runtime'
import type { AgentBrowserBridge } from '../browser/agent-browser-bridge'
import type { BrowserBackend } from '../browser/browser-backend'
import type { RuntimeGraphStore } from './orca-runtime-graph-store'
import type { RuntimePtyController } from './orca-runtime-types'
import type { BrowserWindow } from 'electron'
import type { Store } from '../persistence'

type RuntimeMobileSessionTabsNotifier = {
  readMobileMarkdownTab?(worktreeId: string, tabId: string): Promise<RuntimeMarkdownReadTabResult>
  saveMobileMarkdownTab?(
    worktreeId: string,
    tabId: string,
    baseVersion: string,
    content: string
  ): Promise<RuntimeMarkdownSaveTabResult>
  focusTerminal(tabId: string, worktreeId: string, leafId?: string | null): void
  focusEditorTab?(tabId: string, worktreeId: string): void
  closeSessionTab?(tabId: string, worktreeId: string): void
  closeTerminal(tabId: string, paneRuntimeId?: number): void
  moveSessionTab?(worktreeId: string, move: RuntimeMobileSessionTabMove): void
}

export type RuntimeMobileSessionTabsCommandHost = {
  getStore(): RuntimeStore | null
  requireStore(): Store
  resolveWorktreeSelector(selector: string): Promise<ResolvedWorktree>
  getNotifier(): RuntimeMobileSessionTabsNotifier | null
  getPtyController(): RuntimePtyController | null
  getGraph(): RuntimeGraphStore
  getAgentBrowserBridge(): AgentBrowserBridge | null
  getOffscreenBrowserBackend(): BrowserBackend | null
  getMobileSessionTabsByWorktree(): Map<string, RuntimeMobileSessionTabsSnapshot>
  getMobileSessionTabListeners(): Set<(snapshot: RuntimeMobileSessionTabsResult) => void>
  getMobileSessionTabsNotifyCoalescer(): MobileSessionTabsNotifyCoalescer
  listResolvedWorktrees(): Promise<ResolvedWorktree[]>
  notifyMobileSessionTabsChanged(worktreeId?: string): void
  // Why: shared with createMobileSessionTerminal (a separate, not-yet-
  // extracted mobile-session-tabs cluster) — stays in orca-runtime.ts.
  createHeadlessMobileSessionTerminal(
    worktreeId: string,
    activate: boolean,
    afterTabId?: string,
    opts?: {
      command?: string
      cwd?: string
      env?: Record<string, string>
      startupCommandDelivery?: WorktreeStartupLaunch['startupCommandDelivery']
      identity?: { tabId: string; leafId: string; sessionId?: string }
      launchAgent?: TuiAgent
      targetGroupId?: string
      launchConfig?: SleepingAgentLaunchConfig
    }
  ): Promise<RuntimeMobileSessionCreateTerminalResult>
  findPtyForMobileTerminalTab(
    worktreeId: string,
    tab: RuntimeMobileSessionTerminalTab,
    options?: { allowWorktreeOnlyMatch?: boolean }
  ): RuntimePtyWorktreeRecord | null
  resolveMobileSessionTerminalCommand(
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
  }>
  // Why: shared with notifyMobileSessionTabsChanged (a separate, not-yet-
  // extracted mobile-session-tabs cluster) — stays in orca-runtime.ts.
  getMobileSessionTabsForWorktree(worktreeId: string): RuntimeMobileSessionTabsResult
  getLiveBrowserTabsByPageId(worktreeId: string): Map<string, BrowserTabInfo>
  toMobileSessionTabsResult(
    snapshot: RuntimeMobileSessionTabsSnapshot
  ): RuntimeMobileSessionTabsResult
  isHeadlessMobileSessionPublication(publicationEpoch: string): boolean
  getPersistedSshPtyIdForMobileTerminalTab(tab: RuntimeMobileSessionTerminalTab): string | null
  refreshPtyWorktreeRecordsFromController(
    resolvedWorktrees: ResolvedWorktree[],
    targetWorktreeId?: string | null
  ): Promise<Set<string> | null>
  resolveMobileMarkdownWorktreeId(worktreeSelector: string, tabId: string): Promise<string>
  getValidatedExplicitWorktreeIdSelector(selector: string | undefined): string | null
  resolveFolderWorkspaceLaunchScope(selector: string): Promise<TerminalWorkspaceLaunchScope | null>
  resolveTerminalWorkspaceLaunchScope(selector: string): Promise<TerminalWorkspaceLaunchScope>
  folderWorkspaceToResolvedWorktree(folderWorkspace: FolderWorkspace): ResolvedWorktree
  getAvailableAuthoritativeWindow(): BrowserWindow | null
}

export class RuntimeMobileSessionTabsCommands {
  constructor(private readonly host: RuntimeMobileSessionTabsCommandHost) {}

  async listAllMobileSessionTabs(): Promise<RuntimeMobileSessionTabsResult[]> {
    this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession()
    await this.refreshMobileSessionPtyRecords()
    return [...this.host.getMobileSessionTabsByWorktree().values()].map((snapshot) =>
      this.host.toMobileSessionTabsResult(snapshot)
    )
  }

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs-adjacent PTY plumbing) — public, not private.
  hydrateHeadlessMobileSessionTabsFromWorkspaceSession(
    worktreeId?: string,
    options: {
      force?: boolean
      allowAttachedWindow?: boolean
      onlyServeOwnedTerminals?: boolean
    } = {}
  ): void {
    if (this.host.getAvailableAuthoritativeWindow() && options.allowAttachedWindow !== true) {
      return
    }
    const session = this.host.getStore()?.getWorkspaceSession?.()
    if (!session) {
      return
    }
    const entries =
      worktreeId !== undefined
        ? ([[worktreeId, session.tabsByWorktree[worktreeId] ?? []]] as const)
        : Object.entries(session.tabsByWorktree ?? {})
    for (const [entryWorktreeId, persistedTabs] of entries) {
      const existing = this.host.getMobileSessionTabsByWorktree().get(entryWorktreeId)
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
      this.host.getMobileSessionTabsByWorktree().set(entryWorktreeId, {
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
    if (!this.host.getOffscreenBrowserBackend()) {
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
    this.host.getMobileSessionTabsByWorktree().set(worktreeId, {
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

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs-adjacent PTY plumbing) — public, not private.
  hasServeOwnedPtyBinding(tab: RuntimeMobileSessionTerminalTab): boolean {
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
    const pty = this.host.findPtyForMobileTerminalTab(worktreeId, tab)
    if (pty && this.isServeOrSshOwnedPtyId(pty.ptyId)) {
      return true
    }
    return !this.host.getGraph().tabs.has(tab.parentTabId)
  }

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs-adjacent PTY plumbing) — public, not private.
  mergeMobileSessionSnapshotTabs(
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

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs-adjacent PTY plumbing) — public, not private.
  getMobileSessionSnapshotTabIdentityKeys(tab: RuntimeMobileSessionSnapshotTab): string[] {
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

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs-adjacent PTY plumbing) — public, not private.
  mergeMobileSessionTabGroups(
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
  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs-adjacent PTY plumbing) — public, not private.
  publishPtyBackedMobileSessionTerminal(
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
    const existing = this.host.getMobileSessionTabsByWorktree().get(worktreeId)
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
    this.host.getMobileSessionTabsByWorktree().set(worktreeId, next)
    this.host.notifyMobileSessionTabsChanged(worktreeId)
  }

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs-adjacent PTY plumbing) — public, not private.
  touchMobileSessionSnapshotsForPty(ptyId: string, options: { immediate?: boolean } = {}): void {
    for (const [worktreeId, snapshot] of this.host.getMobileSessionTabsByWorktree()) {
      const hasPtyBackedTab = snapshot.tabs.some(
        (tab) =>
          tab.type === 'terminal' &&
          (tab.ptyId === ptyId || tab.parentLayout?.ptyIdsByLeafId?.[tab.leafId] === ptyId)
      )
      if (!hasPtyBackedTab) {
        continue
      }
      this.host.getMobileSessionTabsByWorktree().set(worktreeId, {
        ...snapshot,
        snapshotVersion: snapshot.snapshotVersion + 1
      })
      if (options.immediate) {
        // Why: readiness/lifecycle changes are structural and must not wait
        // behind the title/status coalescing window.
        this.host.notifyMobileSessionTabsChanged(worktreeId)
      } else {
        // Why: title/status flips several times a second under spinner-in-title
        // agents. Coalesce the emit instead of fanning out every version.
        this.host.getMobileSessionTabsNotifyCoalescer().schedule(worktreeId)
      }
    }
  }

  private buildHeadlessMobileSessionTerminalTabs(
    worktreeId: string,
    persistedTabs: readonly TerminalTab[]
  ): RuntimeMobileSessionTerminalTab[] {
    const session = this.host.getStore()?.getWorkspaceSession?.()
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
    const bridge = this.host.getAgentBrowserBridge()
    if (!this.host.getOffscreenBrowserBackend() || !bridge?.tabList) {
      return []
    }
    return bridge.tabList(worktreeId).tabs.map((tab) => {
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
      this.host
        .getStore()
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
    const session = this.host.getStore()?.getWorkspaceSession?.()
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

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs-adjacent PTY plumbing) — public, not private.
  getHeadlessMobileSessionGroupId(worktreeId: string): string {
    return `headless-terminals:${worktreeId}`
  }

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs-adjacent PTY plumbing) — public, not private.
  buildHeadlessMobileSessionTabGroups(
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

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs-adjacent PTY plumbing) — public, not private.
  buildMaterializedHeadlessParentLayout(
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
    const store = this.host.getStore()
    const session = store?.getWorkspaceSession?.()
    if (!session || !store?.setWorkspaceSession) {
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
    store.setWorkspaceSession({
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
    const store = this.host.getStore()
    const session = store?.getWorkspaceSession?.()
    if (!session || !store?.setWorkspaceSession) {
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
    store.setWorkspaceSession({
      ...session,
      tabsByWorktree: {
        ...session.tabsByWorktree,
        [worktreeId]: reordered
      }
    })
  }

  private emitMobileSessionTabsSnapshot(snapshot: RuntimeMobileSessionTabsSnapshot): void {
    if (this.host.getMobileSessionTabListeners().size === 0) {
      return
    }
    const result = this.host.toMobileSessionTabsResult(snapshot)
    for (const listener of this.host.getMobileSessionTabListeners()) {
      listener(result)
    }
  }

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs-adjacent PTY plumbing) — public, not private.
  async refreshMobileSessionPtyRecords(): Promise<void> {
    if (!this.host.getPtyController()?.listProcesses) {
      return
    }
    const resolvedWorktrees = await this.host.listResolvedWorktrees()
    await this.host.refreshPtyWorktreeRecordsFromController(resolvedWorktrees)
  }

  async activateMobileSessionTab(
    worktreeSelector: string,
    tabId: string,
    leafId?: string,
    opts: { notifyClients?: boolean } = {}
  ): Promise<RuntimeMobileSessionTabsResult> {
    const explicitWorktreeId = this.host.getValidatedExplicitWorktreeIdSelector(worktreeSelector)
    const worktreeId =
      explicitWorktreeId ?? (await this.host.resolveWorktreeSelector(worktreeSelector)).id
    this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(worktreeId)
    await this.refreshMobileSessionPtyRecords()
    const snapshot = this.host.getMobileSessionTabsByWorktree().get(worktreeId)
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
      const publicTab = this.host
        .toMobileSessionTabsResult(snapshot!)
        .tabs.find((candidate) => candidate.type === 'terminal' && candidate.id === tab.id)
      // Why: serve-created tabs can be visible before any renderer has adopted
      // their tab id, so focusing the renderer would silently no-op.
      // Phone-local activation also needs this path for inactive restored tabs:
      // desktop focus is intentionally suppressed, but the PTY still must exist.
      const shouldMaterializePendingTerminal =
        publicTab?.type === 'terminal' &&
        publicTab.status !== 'ready' &&
        (opts.notifyClients === false ||
          !this.host.getNotifier()?.focusTerminal ||
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
          ReturnType<RuntimeMobileSessionTabsCommandHost['resolveMobileSessionTerminalCommand']>
        > = {}
        if (tab.launchAgent) {
          try {
            const workspace = await this.host.resolveTerminalWorkspaceLaunchScope(
              `id:${worktreeId}`
            )
            agentStartup = await this.host.resolveMobileSessionTerminalCommand(workspace, {
              agent: tab.launchAgent
            })
          } catch {
            // Why: a disabled or unresolvable agent must not make the tab
            // untappable; fall back to the plain-shell materialize.
          }
        }
        try {
          await this.host.createHeadlessMobileSessionTerminal(worktreeId, true, undefined, {
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
        return this.host.getMobileSessionTabsForWorktree(worktreeId)
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
        return this.host.getMobileSessionTabsForWorktree(worktreeId)
      }
      if (!this.host.getNotifier()?.focusTerminal) {
        if (
          !targetTab.isActive &&
          this.shouldPersistHeadlessMobileSessionActivation(snapshot!, targetTab)
        ) {
          this.activateHeadlessMobileSessionTerminalTab(worktreeId, snapshot!, targetTab)
        }
        return this.host.getMobileSessionTabsForWorktree(worktreeId)
      }
      this.host.getNotifier()?.focusTerminal(targetTab.parentTabId, worktreeId, targetTab.leafId)
    } else if (tab.type === 'browser') {
      if (opts.notifyClients === false) {
        this.activateMobileSessionTabForRemoteClient(worktreeId, snapshot!, tab)
        return this.host.getMobileSessionTabsForWorktree(worktreeId)
      }
      // Why: browser mobile tabs are renderer-owned unified tabs; focusing the
      // session tab keeps desktop tab order/group state authoritative.
      this.host.getNotifier()?.focusEditorTab?.(tab.id, worktreeId)
    } else {
      if (opts.notifyClients === false) {
        this.activateMobileSessionTabForRemoteClient(worktreeId, snapshot!, tab)
        return this.host.getMobileSessionTabsForWorktree(worktreeId)
      }
      this.host.getNotifier()?.focusEditorTab?.(tab.id, worktreeId)
    }
    return this.host.getMobileSessionTabsForWorktree(worktreeId)
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
    this.host.getMobileSessionTabsByWorktree().set(worktreeId, nextSnapshot)
    this.emitMobileSessionTabsSnapshot(nextSnapshot)
  }

  private shouldMaterializeHeadlessMobileSessionTab(
    snapshot: RuntimeMobileSessionTabsSnapshot,
    tab: RuntimeMobileSessionTerminalTab
  ): boolean {
    return (
      this.host.isHeadlessMobileSessionPublication(snapshot.publicationEpoch) ||
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
    if (
      this.host.getGraph().authoritativeWindowId !== null &&
      this.host.getGraph().graphStatus === 'ready'
    ) {
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
    this.host.getMobileSessionTabsByWorktree().set(worktreeId, nextSnapshot)
    this.emitMobileSessionTabsSnapshot(nextSnapshot)
  }

  // Why: a headless split only updated the LIVE session snapshot, never the
  // persisted workspace session layout. So a later snapshot rebuild (e.g. on the
  // next terminal create) re-derived from the stale single-leaf persisted layout
  // and collapsed the split. Persist the new split leaf into the workspace
  // session's terminalLayoutsByTabId so the split survives rebuilds.
  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs-adjacent PTY plumbing) — public, not private.
  persistHeadlessTerminalSplit(args: {
    tabId: string
    leafId: string
    ptyId: string
    splitFromLeafId: string
    direction: 'horizontal' | 'vertical'
  }): void {
    const store = this.host.getStore()
    const session = store?.getWorkspaceSession?.()
    if (!session || !store?.setWorkspaceSession) {
      return
    }
    const existing = session.terminalLayoutsByTabId?.[args.tabId]
    const nextLayout = buildHeadlessTerminalSplitLayout(
      existing ? this.cloneTerminalLayoutSnapshot(existing) : undefined,
      args
    )
    store.setWorkspaceSession({
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
    const store = this.host.getStore()
    const session = store?.getWorkspaceSession?.()
    if (!session || !store?.setWorkspaceSession) {
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
    store.setWorkspaceSession({
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
    const explicitWorktreeId = this.host.getValidatedExplicitWorktreeIdSelector(worktreeSelector)
    const worktreeId =
      explicitWorktreeId ?? (await this.host.resolveWorktreeSelector(worktreeSelector)).id
    this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(worktreeId)
    await this.refreshMobileSessionPtyRecords()
    const snapshot = this.host.getMobileSessionTabsByWorktree().get(worktreeId)
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
      if (!this.host.getNotifier()?.closeTerminal) {
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
        this.host.getNotifier()?.closeTerminal(tab.parentTabId)
        return { closed: true }
      }
      if (tab.id === tabId) {
        const pty = this.host.findPtyForMobileTerminalTab(worktreeId, tab)
        if (pty) {
          this.host.getPtyController()?.kill(pty.ptyId)
        } else {
          this.host.getNotifier()?.closeTerminal(tab.parentTabId)
        }
      } else {
        // Why: paired web tab bars represent a split terminal with one local
        // parent tab id. Closing that parent should close the desktop tab, not
        // just whichever leaf happened to be first in the session snapshot.
        this.host.getNotifier()?.closeTerminal(tab.parentTabId)
      }
    } else if (tab.type === 'browser' && this.host.getOffscreenBrowserBackend()) {
      // Why: headless browser tabs are offscreen WebContents with no renderer to
      // route closeSessionTab to. Close the page directly and drop it from the
      // snapshot so paired clients stop showing it.
      await this.closeHeadlessMobileBrowserTab(worktreeId, snapshot!, tab)
    } else {
      this.host.getNotifier()?.closeSessionTab?.(tab.id, worktreeId)
    }
    return { closed: true }
  }

  private async closeHeadlessMobileBrowserTab(
    worktreeId: string,
    snapshot: RuntimeMobileSessionTabsSnapshot,
    tab: RuntimeMobileSessionBrowserTab
  ): Promise<void> {
    if (tab.browserPageId) {
      await this.host
        .getOffscreenBrowserBackend()
        ?.closeTab(tab.browserPageId)
        .catch(() => {})
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
    this.host.getMobileSessionTabsByWorktree().set(worktreeId, nextSnapshot)
    this.emitMobileSessionTabsSnapshot(nextSnapshot)
  }

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs-adjacent PTY plumbing) — public, not private.
  markHeadlessBrowserSessionTabActive(
    worktreeId: string | undefined,
    browserPageId: string,
    targetGroupId?: string
  ): void {
    if (!this.host.getOffscreenBrowserBackend() || !worktreeId) {
      return
    }
    // Hydrate first so the freshly created browser tab is present in the snapshot.
    this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(worktreeId)
    const snapshot = this.host.getMobileSessionTabsByWorktree().get(worktreeId)
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
    this.host.getMobileSessionTabsByWorktree().set(worktreeId, nextSnapshot)
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
      const pty = this.host.findPtyForMobileTerminalTab(worktreeId, candidate)
      if (pty?.connected) {
        this.host.getPtyController()?.kill(pty.ptyId)
      } else {
        const persistedSshPtyId = this.host.getPersistedSshPtyIdForMobileTerminalTab(candidate)
        if (persistedSshPtyId) {
          // Why: close is an explicit deletion. Hydrated SSH PTYs can be known
          // only by durable id before reconnect repopulates pane metadata.
          this.host.getPtyController()?.kill(persistedSshPtyId)
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
    this.host.getMobileSessionTabsByWorktree().set(worktreeId, nextSnapshot)
    this.emitMobileSessionTabsSnapshot(nextSnapshot)
  }

  async moveMobileSessionTab(
    worktreeSelector: string,
    move: RuntimeMobileSessionTabMove
  ): Promise<RuntimeMobileSessionTabMoveResult> {
    const explicitWorktreeId = this.host.getValidatedExplicitWorktreeIdSelector(worktreeSelector)
    const worktreeId =
      explicitWorktreeId ?? (await this.host.resolveWorktreeSelector(worktreeSelector)).id
    this.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(worktreeId)
    const snapshot = this.host.getMobileSessionTabsByWorktree().get(worktreeId)
    if (!snapshot) {
      throw new Error('tab_not_found')
    }
    const notifier = this.host.getNotifier()
    if (!notifier?.moveSessionTab) {
      return this.moveHeadlessMobileSessionTab(worktreeId, snapshot, move)
    }
    const hostTabId = this.resolveMobileSessionHostTabId(snapshot, move.tabId)
    if (!hostTabId) {
      throw new Error('tab_not_found')
    }
    const publicSnapshot = this.host.toMobileSessionTabsResult(snapshot)
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
      notifier.moveSessionTab(worktreeId, {
        ...move,
        tabId: hostTabId,
        tabOrder
      })
      return { moved: true }
    }
    notifier.moveSessionTab(worktreeId, {
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
    const explicitWorktreeId = this.host.getValidatedExplicitWorktreeIdSelector(worktreeSelector)
    const worktreeId =
      explicitWorktreeId ?? (await this.host.resolveWorktreeSelector(worktreeSelector)).id
    // Why: when a renderer is authoritative (desktop host reached via shared
    // control), it owns pane geometry and republishes it — a headless write here
    // would be overwritten and could fight the renderer. Persist only headlessly.
    if (this.host.getAvailableAuthoritativeWindow()) {
      return { updated: true }
    }
    // Why: resolve to the host tab id (older/raw-id clients) so the persisted
    // layout entry matches, matching setMobileSessionTabProps.
    const snapshot = this.host.getMobileSessionTabsByWorktree().get(worktreeId)
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
    const explicitWorktreeId = this.host.getValidatedExplicitWorktreeIdSelector(worktreeSelector)
    const worktreeId =
      explicitWorktreeId ?? (await this.host.resolveWorktreeSelector(worktreeSelector)).id
    // Why: a renderer-authoritative host owns + republishes tab props, so a
    // headless write would be overwritten. Persist only when headless.
    if (this.host.getAvailableAuthoritativeWindow()) {
      return { updated: true }
    }
    const snapshot = this.host.getMobileSessionTabsByWorktree().get(worktreeId)
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
    const store = this.host.getStore()
    const session = store?.getWorkspaceSession?.()
    if (!session || !store?.setWorkspaceSession) {
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
    store.setWorkspaceSession(nextSession)
  }

  private applyHeadlessSessionTabPropsToSnapshot(
    worktreeId: string,
    tabId: string,
    props: { color?: string | null; isPinned?: boolean; viewMode?: 'terminal' | 'chat' }
  ): void {
    const snapshot = this.host.getMobileSessionTabsByWorktree().get(worktreeId)
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
    this.host.getMobileSessionTabsByWorktree().set(worktreeId, nextSnapshot)
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
    const store = this.host.getStore()
    const session = store?.getWorkspaceSession?.()
    if (!session || !store?.setWorkspaceSession) {
      return
    }
    const existing = session.terminalLayoutsByTabId?.[args.tabId]
    if (!existing) {
      return
    }
    store.setWorkspaceSession({
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
    const snapshot = this.host.getMobileSessionTabsByWorktree().get(worktreeId)
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
    this.host.getMobileSessionTabsByWorktree().set(worktreeId, nextSnapshot)
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
    const publicSnapshot = this.host.toMobileSessionTabsResult(snapshot)
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
    this.host.getMobileSessionTabsByWorktree().set(worktreeId, nextSnapshot)
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
    this.host.getMobileSessionTabsByWorktree().set(worktreeId, nextSnapshot)
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
    this.host.getMobileSessionTabsByWorktree().set(worktreeId, nextSnapshot)
    this.emitMobileSessionTabsSnapshot(nextSnapshot)
    return { moved: true }
  }

  // Persist the headless tab-GROUP layout so snapshot rebuilds keep the split.
  private persistHeadlessTabGroups(
    worktreeId: string,
    groups: readonly RuntimeMobileSessionTabGroup[],
    layout: TabGroupLayoutNode
  ): void {
    const store = this.host.getStore()
    const session = store?.getWorkspaceSession?.()
    if (!session || !store?.setWorkspaceSession) {
      return
    }
    store.setWorkspaceSession({
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
  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs-adjacent PTY plumbing) — public, not private.
  persistHeadlessTerminalTitle(worktreeId: string, tabId: string, title: string | null): void {
    const store = this.host.getStore()
    const session = store?.getWorkspaceSession?.()
    if (!session || !store?.setWorkspaceSession) {
      return
    }
    const tabs = session.tabsByWorktree[worktreeId]
    if (!tabs?.some((tab) => tab.id === tabId)) {
      return
    }
    store.setWorkspaceSession({
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
    const liveBrowserTabsByPageId = this.host.getLiveBrowserTabsByPageId(snapshot.worktree)
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
    const worktreeId = await this.host.resolveMobileMarkdownWorktreeId(worktreeSelector, tabId)
    const notifier = this.host.getNotifier()
    if (!notifier?.readMobileMarkdownTab) {
      throw new Error('renderer_unavailable')
    }
    return await notifier.readMobileMarkdownTab(worktreeId, tabId)
  }

  async saveMobileMarkdownTab(
    worktreeSelector: string,
    tabId: string,
    baseVersion: string,
    content: string
  ): Promise<RuntimeMarkdownSaveTabResult> {
    const worktreeId = await this.host.resolveMobileMarkdownWorktreeId(worktreeSelector, tabId)
    const notifier = this.host.getNotifier()
    if (!notifier?.saveMobileMarkdownTab) {
      throw new Error('renderer_unavailable')
    }
    return await notifier.saveMobileMarkdownTab(worktreeId, tabId, baseVersion, content)
  }
  // Why: also called from OrcaRuntimeService's fileCommands/gitCommands
  // composition wiring (unrelated domains that sit textually adjacent to
  // this one) — public, not private.
  async resolveRuntimeGitTarget(worktreeSelector: string): Promise<{
    worktree: ResolvedWorktree
    repo?: Repo
    connectionId?: string
    localGitOptions?: { wslDistro?: string }
  }> {
    const store = this.host.requireStore()
    const worktree = await this.host.resolveWorktreeSelector(worktreeSelector)
    const repo = store.getRepo(worktree.repoId)
    // Why: getRepoProviderConnectionKey (not repo.connectionId directly) so a
    // Dev-Server-bound repo (devServerId, no SSH connectionId) also resolves
    // through the provider registries — see dev-server-provider-lifecycle.ts.
    const connectionId = (repo ? getRepoProviderConnectionKey(repo) : null) ?? undefined
    const localGitOptions =
      repo && !connectionId ? getLocalProjectWorktreeGitOptions(store, repo) : {}
    return { worktree, repo, connectionId, localGitOptions }
  }

  // Why: also called from OrcaRuntimeService's fileCommands/gitCommands
  // composition wiring (unrelated domains that sit textually adjacent to
  // this one) — public, not private.
  async resolveRuntimeFileTarget(worktreeSelector: string): Promise<{
    worktree: ResolvedWorktree
    connectionId?: string
  }> {
    const folderScope = await this.host.resolveFolderWorkspaceLaunchScope(worktreeSelector)
    if (folderScope?.folderWorkspace) {
      return {
        worktree: this.host.folderWorkspaceToResolvedWorktree(folderScope.folderWorkspace),
        connectionId: folderScope.connectionId ?? undefined
      }
    }

    const store = this.host.requireStore()
    const worktree = await this.host.resolveWorktreeSelector(worktreeSelector)
    const repo = store.getRepo(worktree.repoId)
    return {
      worktree,
      connectionId: (repo ? getRepoProviderConnectionKey(repo) : null) ?? undefined
    }
  }

  onMobileSessionTabsChanged(
    listener: (snapshot: RuntimeMobileSessionTabsResult) => void
  ): () => void {
    this.host.getMobileSessionTabListeners().add(listener)
    return () => {
      // Why: flush pending coalesced notifies before dropping this listener so a
      // subscriber closing mid-window still receives the latest settled state.
      this.host.getMobileSessionTabsNotifyCoalescer().flushAll()
      this.host.getMobileSessionTabListeners().delete(listener)
    }
  }

  // Why: terminal handles are normally created lazily when first referenced via
  // RPC, but agents need their own handle at spawn time (via ORCA_TERMINAL_HANDLE
  // env var) so they can self-identify in orchestration messages without an
  // extra RPC round-trip. Pre-allocating by ptyId lets issueHandle reuse it.
}
