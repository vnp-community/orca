/* eslint-disable max-lines -- Why: straight extraction of orca-runtime.ts's
headless mobile-session-tabs notify/materialize-result cluster
(notifyMobileSessionTabsChanged and its ~23 dedicated private helpers),
already covered by orca-runtime.ts's own grandfathered max-lines disable
before this move. Registered in config/max-lines-baseline.txt per
AGENTS.md — NEEDS PR REVIEW. This is the last of 3 mobile-session-tabs
clusters (cluster 1: orca-runtime-mobile-session-tabs.ts, cluster 2:
orca-runtime-mobile-session-terminal.ts). */
/* eslint-disable unicorn/no-useless-spread -- Why: matches orca-runtime.ts's
own grandfathered disable — the worktree snapshot map is cloned before
iterating because the loop body may delete entries from it mid-iteration. */
// frontend/src/main/runtime/orca-runtime-mobile-session-notify.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-053): mobile-session-tabs
// notify/snapshot-merge/client-payload-shaping commands extracted from
// OrcaRuntimeService via the composition pattern. getSummaryForRuntimeWorktreeId/
// buildTerminalSummary (unrelated listTerminals/getWorktreePs infra) and
// getAgentStatusForHandle-onward (unrelated general agent-status-for-pane-key
// infra) sit textually adjacent — deliberately excluded, stay in orca-runtime.ts.
import { createHash } from 'node:crypto'
import type { TabGroupLayoutNode } from '../../shared/types'
import { AGENT_STATUS_STALE_AFTER_MS, type AgentStatusEntry } from '../../shared/agent-status-types'
import {
  normalizeCompatibleAgentStatusEntryForOwner,
  normalizeCompatibleAgentTitleForOwner
} from '../../shared/agent-title-owner'
import { FIRST_PANE_ID } from '../../shared/pane-key'
import { isTerminalLeafId, makePaneKey } from '../../shared/stable-pane-id'
import { parseAppSshPtyId } from '../../shared/ssh-pty-id'
import type {
  BrowserTabInfo,
  RuntimeMobileSessionClientTab,
  RuntimeMobileSessionMarkdownTab,
  RuntimeMobileSessionSnapshotTab,
  RuntimeMobileSessionTabGroup,
  RuntimeMobileSessionTabsRemovedResult,
  RuntimeMobileSessionTabsResult,
  RuntimeMobileSessionTabsSnapshot,
  RuntimeMobileSessionTerminalTab
} from '../../shared/runtime-types'
import {
  classifyAgentTitle,
  getLatestAgentCandidateTitle,
  getLatestPtyTitle
} from './orca-runtime-tail-buffer'
import type { MobileSessionTabsNotifyCoalescer } from './mobile-session-tabs-notify-coalescer'
import type {
  RuntimeAgentRowSnapshot,
  RuntimePtyWorktreeRecord,
  RuntimeStore,
  ResolvedWorktree
} from './orca-runtime'
import type { AgentBrowserBridge } from '../browser/agent-browser-bridge'
import type { BrowserBackend } from '../browser/browser-backend'
import type { RuntimeGraphStore } from './orca-runtime-graph-store'

export type RuntimeMobileSessionNotifyCommandHost = {
  getStore(): RuntimeStore | null
  getAgentBrowserBridge(): AgentBrowserBridge | null
  getOffscreenBrowserBackend(): BrowserBackend | null
  getGraph(): RuntimeGraphStore
  getMobileSessionTabsByWorktree(): Map<string, RuntimeMobileSessionTabsSnapshot>
  getMobileSessionTabListeners(): Set<(snapshot: RuntimeMobileSessionTabsResult) => void>
  getMobileSessionTabsNotifyCoalescer(): MobileSessionTabsNotifyCoalescer
  getLatestAgentStatusByPaneKey(): Map<string, RuntimeAgentRowSnapshot>
  resolveWorktreeSelector(selector: string): Promise<ResolvedWorktree>
  getValidatedExplicitWorktreeIdSelector(selector: string | undefined): string | null
  hasServeOwnedPtyBinding(tab: RuntimeMobileSessionTerminalTab): boolean
  getMobileSessionSnapshotTabIdentityKeys(tab: RuntimeMobileSessionSnapshotTab): string[]
  mergeMobileSessionSnapshotTabs(
    baseTabs: readonly RuntimeMobileSessionSnapshotTab[],
    extraTabs: readonly RuntimeMobileSessionSnapshotTab[]
  ): RuntimeMobileSessionSnapshotTab[]
  mergeMobileSessionTabGroups(
    worktreeId: string,
    groups: readonly RuntimeMobileSessionTabGroup[],
    terminalTabs: readonly RuntimeMobileSessionTerminalTab[],
    activeTab: RuntimeMobileSessionTerminalTab | null
  ): RuntimeMobileSessionTabGroup[]
  getHeadlessMobileSessionGroupId(worktreeId: string): string
  hydrateHeadlessMobileSessionTabsFromWorkspaceSession(
    worktreeId?: string,
    options?: {
      force?: boolean
      allowAttachedWindow?: boolean
      onlyServeOwnedTerminals?: boolean
    }
  ): void
  getLeafKey(tabId: string, leafId: string): string
  issuePtyHandle(pty: RuntimePtyWorktreeRecord): string
  recordPtyWorktree(
    ptyId: string,
    worktreeId: string,
    state?: Partial<
      Pick<
        RuntimePtyWorktreeRecord,
        'connected' | 'lastOutputAt' | 'preview' | 'tabId' | 'paneKey' | 'title' | 'connectionId'
      >
    >
  ): RuntimePtyWorktreeRecord
}

export class RuntimeMobileSessionNotifyCommands {
  constructor(private readonly host: RuntimeMobileSessionNotifyCommandHost) {}

  // Why: also called from OrcaRuntimeService outside this domain (renderer graph-sync completion) — public, not private.
  syncMobileSessionTabs(snapshots: RuntimeMobileSessionTabsSnapshot[] | undefined): void {
    if (snapshots === undefined) {
      return
    }
    // Why: renderer graphs are authoritative for renderer tabs, but headless
    // serve terminals never enter that graph unless we preserve their bindings.
    this.host.hydrateHeadlessMobileSessionTabsFromWorkspaceSession(undefined, {
      allowAttachedWindow: true,
      onlyServeOwnedTerminals: true
    })
    const mobileSessionTabsByWorktree = this.host.getMobileSessionTabsByWorktree()
    const nextWorktrees = new Set<string>()
    for (const snapshot of snapshots) {
      nextWorktrees.add(snapshot.worktree)
      const existing = mobileSessionTabsByWorktree.get(snapshot.worktree)
      const nextSnapshot = this.mergePreservedHeadlessMobileSessionTabs(snapshot, existing)
      if (
        !existing ||
        nextSnapshot.publicationEpoch !== existing.publicationEpoch ||
        nextSnapshot.snapshotVersion >= existing.snapshotVersion
      ) {
        mobileSessionTabsByWorktree.set(snapshot.worktree, nextSnapshot)
      }
    }
    for (const [worktreeId, existing] of [...mobileSessionTabsByWorktree.entries()]) {
      if (!nextWorktrees.has(worktreeId)) {
        const preserved = this.buildPreservedHeadlessMobileSessionSnapshot(existing)
        if (preserved) {
          mobileSessionTabsByWorktree.set(worktreeId, preserved)
          nextWorktrees.add(worktreeId)
        } else {
          mobileSessionTabsByWorktree.delete(worktreeId)
          // Why: drop any pending coalesced notify so a stale snapshot can't
          // land after the removed frame.
          this.host.getMobileSessionTabsNotifyCoalescer().cancel(worktreeId)
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
    const tabs = this.host.mergeMobileSessionSnapshotTabs(snapshot.tabs, normalizedPreservedTabs)
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
      tabGroups: this.host.mergeMobileSessionTabGroups(
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
        existing.activeGroupId ?? this.host.getHeadlessMobileSessionGroupId(existing.worktree),
      activeTabId: activeTab?.id ?? null,
      activeTabType: activeTab?.type ?? null,
      tabGroups: this.host.mergeMobileSessionTabGroups(
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
      incoming?.tabs.flatMap((tab) => this.host.getMobileSessionSnapshotTabIdentityKeys(tab)) ?? []
    )
    return existing.tabs.filter((tab) => {
      if (
        this.host.getMobileSessionSnapshotTabIdentityKeys(tab).some((id) => incomingIds.has(id))
      ) {
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
        Boolean(this.host.getOffscreenBrowserBackend()) &&
        this.isHeadlessMobileSessionPublication(snapshot.publicationEpoch)
      )
    }
    if (tab.type !== 'terminal') {
      return false
    }
    return (
      this.isHeadlessMobileSessionPublication(snapshot.publicationEpoch) ||
      this.host.hasServeOwnedPtyBinding(tab)
    )
  }

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs cluster 1's host wiring) — public, not private.
  isHeadlessMobileSessionPublication(publicationEpoch: string): boolean {
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
    for (const listener of this.host.getMobileSessionTabListeners()) {
      listener(removed)
    }
  }

  // Why: also called from OrcaRuntimeService outside this domain (field-initializer coalescer + external notify call sites) — public, not private.
  notifyMobileSessionTabsChanged(worktreeId?: string): void {
    if (!worktreeId) {
      this.notifyMobileSessionTabSnapshots()
      return
    }
    // Why: structural changes (tab add/remove/activate) must propagate promptly,
    // so cancel any pending coalesced title/status notify — this immediate emit
    // already reflects the latest snapshot and supersedes it.
    this.host.getMobileSessionTabsNotifyCoalescer().cancel(worktreeId)
    this.notifyMobileSessionTabsChangedNow(worktreeId)
  }

  // Why: also called from OrcaRuntimeService outside this domain (coalescer field-initializer callback) — public, not private.
  notifyMobileSessionTabsChangedNow(worktreeId: string): void {
    const mobileSessionTabListeners = this.host.getMobileSessionTabListeners()
    if (mobileSessionTabListeners.size === 0) {
      return
    }
    const snapshot = this.host.getMobileSessionTabsByWorktree().get(worktreeId)
    if (!snapshot) {
      return
    }
    // Why: browser bridge lifecycle events are already scoped by worktree; avoid
    // fanning out every active workspace snapshot during navigation/tab churn.
    const result = this.toMobileSessionTabsResult(snapshot)
    for (const listener of mobileSessionTabListeners) {
      listener(result)
    }
  }

  // Why: also called from OrcaRuntimeService outside this domain (createTerminal graph-sync completion) — public, not private.
  notifyMobileSessionTabSnapshots(): void {
    const mobileSessionTabListeners = this.host.getMobileSessionTabListeners()
    if (mobileSessionTabListeners.size === 0) {
      return
    }
    for (const snapshot of this.host.getMobileSessionTabsByWorktree().values()) {
      const result = this.toMobileSessionTabsResult(snapshot)
      for (const listener of mobileSessionTabListeners) {
        listener(result)
      }
    }
  }

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs cluster 1's host wiring) — public, not private.
  getMobileSessionTabsForWorktree(worktreeId: string): RuntimeMobileSessionTabsResult {
    const snapshot = this.host.getMobileSessionTabsByWorktree().get(worktreeId)
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

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs cluster 1's host wiring) — public, not private.
  async resolveMobileMarkdownWorktreeId(worktreeSelector: string, tabId: string): Promise<string> {
    const worktreeId =
      this.host.getValidatedExplicitWorktreeIdSelector(worktreeSelector) ??
      (await this.host.resolveWorktreeSelector(worktreeSelector)).id
    const snapshot = this.host.getMobileSessionTabsByWorktree().get(worktreeId)
    const tab = snapshot?.tabs.find(
      (candidate): candidate is RuntimeMobileSessionMarkdownTab =>
        candidate.type === 'markdown' && candidate.id === tabId
    )
    if (!tab) {
      throw new Error('tab_not_found')
    }
    return worktreeId
  }

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs cluster 1's host wiring) — public, not private.
  getLiveBrowserTabsByPageId(worktreeId: string): Map<string, BrowserTabInfo> {
    const agentBrowserBridge = this.host.getAgentBrowserBridge()
    if (!agentBrowserBridge?.tabList) {
      return new Map()
    }
    const liveTabs = agentBrowserBridge.tabList(worktreeId).tabs
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
  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs/-terminal cluster host wiring) — public, not private.
  toMobileSessionTabsResult(
    snapshot: RuntimeMobileSessionTabsSnapshot
  ): RuntimeMobileSessionTabsResult {
    const graph = this.host.getGraph()
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
      const syncedTab = graph.tabs.get(tab.parentTabId)
      const leaf = graph.leaves.get(this.host.getLeafKey(tab.parentTabId, tab.leafId)) ?? null
      const liveLeaf = leaf?.ptyId && leaf.connected ? leaf : null
      const liveLeafPtyId = liveLeaf?.ptyId ?? null
      const liveLeafPty = liveLeafPtyId ? (graph.ptysById.get(liveLeafPtyId) ?? null) : null
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
        ? this.host.issuePtyHandle(
            this.host.recordPtyWorktree(liveLeafPtyId, snapshot.worktree, {
              tabId: tab.parentTabId,
              paneKey,
              connected: true
            })
          )
        : livePty
          ? this.host.issuePtyHandle(livePty)
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
    const graph = this.host.getGraph()
    const leaf = graph.leaves.get(this.host.getLeafKey(tab.parentTabId, tab.leafId)) ?? null
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
    const latestAgentStatusByPaneKey = this.host.getLatestAgentStatusByPaneKey()
    let retained = latestAgentStatusByPaneKey.get(paneKey) ?? null
    if (!retained) {
      const ptyId = pty?.ptyId ?? tab.ptyId ?? null
      if (ptyId) {
        for (const snapshot of latestAgentStatusByPaneKey.values()) {
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

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs cluster 1's host wiring) — public, not private.
  findPtyForMobileTerminalTab(
    worktreeId: string,
    tab: RuntimeMobileSessionTerminalTab,
    options: { allowWorktreeOnlyMatch?: boolean } = {}
  ): RuntimePtyWorktreeRecord | null {
    const graph = this.host.getGraph()
    const snapshotPtyId = tab.ptyId ?? tab.parentLayout?.ptyIdsByLeafId?.[tab.leafId] ?? null
    const paneKey = this.getMobileTerminalPaneKey(tab)
    if (snapshotPtyId) {
      const pty = graph.ptysById.get(snapshotPtyId)
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
    for (const pty of graph.ptysById.values()) {
      if (pty.tabId === tab.parentTabId && pty.paneKey && paneKeys.has(pty.paneKey)) {
        return pty
      }
    }
    return null
  }

  // Why: also called from OrcaRuntimeService outside this domain (mobile-session-tabs cluster 1's host wiring) — public, not private.
  getPersistedSshPtyIdForMobileTerminalTab(tab: RuntimeMobileSessionTerminalTab): string | null {
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
}
