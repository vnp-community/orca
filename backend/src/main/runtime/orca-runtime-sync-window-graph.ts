/* eslint-disable unicorn/no-useless-spread -- Why: matches orca-runtime.ts's
own grandfathered disable — graphSyncCallbacks is cloned before iterating so
a callback that unsubscribes itself mid-drain can safely mutate the
underlying array. */
// frontend/src/main/runtime/orca-runtime-sync-window-graph.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-080): syncWindowGraph — the
// renderer→main graph-sync ingestion method, reconciling incoming tabs/
// leaves against the live PTY graph — extracted from OrcaRuntimeService via
// the composition pattern. Same "move the dispatcher itself, keep
// already-extracted domains reachable as host deps" technique as
// TASK-BIGFILE-072/074/077. Mutates `this.graph` directly and runs on every
// renderer graph publish (window focus, terminal create/close, pane splits,
// ...) — a core hot path with no direct test coverage.
import type {
  RuntimeStatus,
  RuntimeSyncedLeaf,
  RuntimeSyncWindowGraph,
  RuntimeSyncWindowGraphResult
} from '../../shared/runtime-types'
import type { AgentStatusOrchestrationContext } from '../../shared/agent-status-types'
import type { RuntimeLeafRecord, RuntimePtyWorktreeRecord } from './orca-runtime'
import type { RuntimeGraphStore } from './orca-runtime-graph-store'
import type { RuntimeMobileSessionNotifyCommands } from './orca-runtime-mobile-session-notify'

export type RuntimeSyncWindowGraphCommandHost = {
  getGraph(): RuntimeGraphStore
  syncMobileSessionTabs: RuntimeMobileSessionNotifyCommands['syncMobileSessionTabs']
  notifyMobileSessionTabSnapshots: RuntimeMobileSessionNotifyCommands['notifyMobileSessionTabSnapshots']
  nextTitleObservationSequence(): number
  getLeafKey(tabId: string, leafId: string): string
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
  makeRuntimePaneKey(leaf: Pick<RuntimeSyncedLeaf, 'tabId' | 'leafId' | 'paneRuntimeId'>): string
  invalidateLeafHandle(leafKey: string): void
  rebuildLeafPtyIndex(): void
  refreshWritableFlags(): void
  adoptPreAllocatedHandle(leaf: RuntimeLeafRecord): string | null
  buildAgentOrchestrationByPaneKey(): Record<string, AgentStatusOrchestrationContext> | undefined
  getStatus(): RuntimeStatus
}

// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-080): reconcile the renderer's
// published tab/leaf graph against the live PTY state - preserving CLI
// handles across renderer reloads, deduping leaves that raced onto the same
// PTY, and draining graph-sync waiters once the leaves settle.
export class RuntimeSyncWindowGraphCommands {
  constructor(private readonly host: RuntimeSyncWindowGraphCommandHost) {}

  syncWindowGraph(windowId: number, graph: RuntimeSyncWindowGraph): RuntimeSyncWindowGraphResult {
    if (this.host.getGraph().authoritativeWindowId === null) {
      this.host.getGraph().authoritativeWindowId = windowId
    }
    if (windowId !== this.host.getGraph().authoritativeWindowId) {
      throw new Error('Runtime graph publisher does not match the authoritative window')
    }

    this.host.getGraph().tabs = new Map(graph.tabs.map((tab) => [tab.tabId, tab]))
    this.host.syncMobileSessionTabs(graph.mobileSessionTabs)
    const nextLeaves = new Map<string, RuntimeLeafRecord>()
    const graphSyncedAt = this.host.nextTitleObservationSequence()

    // Why: renderer reloads can briefly republish the same leaf with no ptyId;
    // keep live CLI handles usable while the UI graph rebuilds.
    const preserveLivePtysDuringReload = this.host.getGraph().graphStatus === 'reloading'
    for (const leaf of graph.leaves) {
      const leafKey = this.host.getLeafKey(leaf.tabId, leaf.leafId)
      const existing = this.host.getGraph().leaves.get(leafKey)
      const ptyId =
        preserveLivePtysDuringReload && leaf.ptyId === null && existing?.ptyId
          ? existing.ptyId
          : leaf.ptyId
      const ptyGeneration =
        existing && existing.ptyId !== ptyId
          ? existing.ptyGeneration + 1
          : (existing?.ptyGeneration ?? 0)
      const existingPty = ptyId ? this.host.getGraph().ptysById.get(ptyId) : undefined
      const tailSource = existing?.ptyId === ptyId ? existing : existingPty

      nextLeaves.set(leafKey, {
        ...leaf,
        ptyId,
        ptyGeneration,
        connected: ptyId !== null,
        writable: this.host.getGraph().graphStatus === 'ready' && ptyId !== null,
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
        this.host.recordPtyWorktree(leaf.ptyId, leaf.worktreeId, {
          connected: true,
          lastOutputAt: existing?.ptyId === leaf.ptyId ? existing.lastOutputAt : null,
          preview: existing?.ptyId === leaf.ptyId ? existing.preview : '',
          tabId: leaf.tabId,
          paneKey: this.host.makeRuntimePaneKey(leaf)
        })
      }

      if (existing && (existing.ptyId !== ptyId || existing.ptyGeneration !== ptyGeneration)) {
        this.host.invalidateLeafHandle(leafKey)
      }
    }

    // Why: computed BEFORE preserving stale leaves so preservation can refuse a
    // leaf whose PTY the incoming graph already rebound to a live leaf. Two
    // leaves on one PTY resolve to the same handle (handles are ptyId-keyed) and
    // crash paired clients with a duplicate React key.
    const nextPtyIds = new Set(
      [...nextLeaves.values()].map((leaf) => leaf.ptyId).filter((ptyId): ptyId is string => !!ptyId)
    )
    for (const oldLeafKey of this.host.getGraph().leaves.keys()) {
      if (!nextLeaves.has(oldLeafKey)) {
        const oldLeaf = this.host.getGraph().leaves.get(oldLeafKey)
        if (
          preserveLivePtysDuringReload &&
          oldLeaf?.ptyId &&
          this.host.getGraph().handleByPtyId.has(oldLeaf.ptyId) &&
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
          const oldHandle = this.host.getGraph().handleByLeafKey.get(oldLeafKey)
          if (
            oldHandle !== undefined &&
            oldHandle === this.host.getGraph().handleByPtyId.get(oldLeaf.ptyId)
          ) {
            this.host.getGraph().handleByLeafKey.delete(oldLeafKey)
          } else {
            this.host.invalidateLeafHandle(oldLeafKey)
          }
        } else {
          this.host.invalidateLeafHandle(oldLeafKey)
        }
      }
    }

    for (const [ptyId, leaf] of this.host.getGraph().detachedPreAllocatedLeaves) {
      if (nextPtyIds.has(ptyId) || !this.host.getGraph().handleByPtyId.has(ptyId)) {
        this.host.getGraph().detachedPreAllocatedLeaves.delete(ptyId)
        continue
      }
      nextLeaves.set(this.host.getLeafKey(leaf.tabId, leaf.leafId), leaf)
      nextPtyIds.add(ptyId)
    }

    this.host.getGraph().leaves = nextLeaves
    this.host.rebuildLeafPtyIndex()
    this.host.notifyMobileSessionTabSnapshots()
    this.host.getGraph().graphStatus = 'ready'
    this.host.refreshWritableFlags()
    for (const leaf of this.host.getGraph().leaves.values()) {
      this.host.adoptPreAllocatedHandle(leaf)
    }

    // Why: createTerminal waits for the renderer's graph sync to populate the
    // new leaf so it can return a handle. Drain callbacks after leaves update.
    for (const cb of [...this.host.getGraph().graphSyncCallbacks]) {
      cb()
    }

    const agentOrchestrationByPaneKey = this.host.buildAgentOrchestrationByPaneKey()
    return {
      ...this.host.getStatus(),
      ...(agentOrchestrationByPaneKey ? { agentOrchestrationByPaneKey } : {})
    }
  }
}
