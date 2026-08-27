// frontend/src/main/runtime/orca-runtime-pty-exit.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-077): onPtyExit — the PTY-exit
// counterpart to onPtyData (TASK-BIGFILE-074) — extracted from
// OrcaRuntimeService via the composition pattern. Same "move the dispatcher
// itself, keep ~15 already-extracted domains reachable as host deps"
// technique. Lower call frequency than onPtyData (once per PTY lifetime,
// not once per output chunk) but still directly in the exit hot path.
import { advertisedUrlWatcher } from '../ports/advertised-url-watcher'
import { serveSimStateWatcher } from '../emulator/serve-sim-state-watcher'
import type { AgentDetector } from '../stats/agent-detector'
import type { RuntimeLeafRecord, RuntimePtyWorktreeRecord } from './orca-runtime'
import type { RuntimeGraphStore } from './orca-runtime-graph-store'
import type { RuntimePtyTranscriptStore } from './orca-runtime-pty-transcript-store'
// ADR-021 — "chỉ dùng 1 database": OrchestrationDb (SQLite) → PgOrchestrationDb
// (async). onPtyExit() stays a synchronous void method (it's a raw PTY-exit
// event handler) — failActiveDispatchOnExit() is fired-and-forgotten through
// a KeyedAsyncQueue keyed by terminal handle instead, so a rapid exit/respawn
// of the same handle can never interleave two dispatch-failure sequences
// against the orchestration DB. See keyed-async-queue.ts's module doc comment.
import type { PgOrchestrationDb } from './orchestration/pg-db'
import type { KeyedAsyncQueue } from './orchestration/keyed-async-queue'

export type RuntimePtyExitCommandHost = {
  getGraph(): RuntimeGraphStore
  getPtyTranscripts(): RuntimePtyTranscriptStore
  // Why: the raw field, not getOrchestrationDbIfAvailable() - dispatch-failure
  // handling on exit must not force-create an orchestration DB that doesn't
  // already exist (matches the original's direct this._orchestrationDb read).
  getRawOrchestrationDb(): PgOrchestrationDb | null
  // ADR-021 — shared across every orchestration-DB event-driven call site
  // (pty-exit, message-delivery, terminal-agent-status), all keyed by
  // terminal handle, so no two touch the same handle's dispatch state
  // concurrently regardless of which code path triggered them.
  getOrchestrationDeliveryQueue(): KeyedAsyncQueue
  getAgentDetector(): AgentDetector | null
  getLeafKey(tabId: string, leafId: string): string
  getLeavesForPty(ptyId: string): RuntimeLeafRecord[]
  clearRemoteTerminalViewSubscriberCountForPty(ptyId: string): void
  clearWaitBlockedCheckState(ptyId: string): void
  disposePtyTitleTracker(ptyId: string): void
  clearAgentRowSnapshotsForPty(ptyId: string): void
  removeTeamForLeaderHandle(handle: string): void
  clearStateForExitedPty(ptyId: string): void
  disposeHeadlessTerminal(ptyId: string): void
  resolvePtyExitWaiters(pty: RuntimePtyWorktreeRecord, ptyId: string): void
  resolveExitWaiters(leaf: RuntimeLeafRecord): void
  pruneDisconnectedPtyTranscript(pty: RuntimePtyWorktreeRecord): void
  touchMobileSessionSnapshotsForPty(ptyId: string, options?: { immediate?: boolean }): void
  pruneDisconnectedPtyRecords(): void
}

// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-077): PTY-exit teardown — clears
// per-PTY transcript/tracker state across ~15 already-extracted domains,
// resolves exit waiters, fails any active dispatch, then prunes stale
// records. failActiveDispatchOnExit moved with it as its sole caller.
export class RuntimePtyExitCommands {
  constructor(private readonly host: RuntimePtyExitCommandHost) {}

  onPtyExit(ptyId: string, exitCode: number): void {
    advertisedUrlWatcher.unbindPty(ptyId)
    serveSimStateWatcher.unbindPty(ptyId)
    // Clean up new mobile state for this PTY
    this.host.clearRemoteTerminalViewSubscriberCountForPty(ptyId)
    this.host.getPtyTranscripts().recentPtyOutputById.delete(ptyId)
    this.host.clearWaitBlockedCheckState(ptyId)
    this.host.getPtyTranscripts().recentPtyPathCandidatesById.delete(ptyId)
    this.host.getPtyTranscripts().ptyOutputSequenceById.delete(ptyId)
    this.host.getPtyTranscripts().agentStatusOscProcessorsByPtyId.delete(ptyId)
    this.host.getPtyTranscripts().terminalSpawnCommandsByPtyId.delete(ptyId)
    this.host.disposePtyTitleTracker(ptyId)
    this.host.getPtyTranscripts().oscTitleScanTailByPtyId.delete(ptyId)
    this.host.getPtyTranscripts().osc7ScanTailByPtyId.delete(ptyId)
    this.host.getPtyTranscripts().terminalCwdByPtyId.delete(ptyId)
    this.host.getPtyTranscripts().terminalFileUriHostnameByPtyId.delete(ptyId)
    this.host.clearAgentRowSnapshotsForPty(ptyId)
    // Why: a Claude agent-team leader whose PTY exits naturally (agent finished,
    // process died, renderer reload) must release its team + nested panes map.
    // Previously only explicit closeTerminal evicted it, so natural exits leaked
    // one team per never-reused teamId for the runtime's lifetime.
    const exitedTeamLeaderHandle = this.host.getGraph().handleByPtyId.get(ptyId)
    if (exitedTeamLeaderHandle) {
      this.host.removeTeamForLeaderHandle(exitedTeamLeaderHandle)
    }
    // Why: mobile floor/layout/remote-desktop state for this PTY moved to
    // RuntimeMobileFloorCommands (TASK-BIGFILE-037) — delegate the cleanup.
    this.host.clearStateForExitedPty(ptyId)
    this.host.disposeHeadlessTerminal(ptyId)
    this.host.getAgentDetector()?.onExit(ptyId)
    const pty = this.host.getGraph().ptysById.get(ptyId)
    if (pty) {
      pty.connected = false
      pty.disconnectedAt = Date.now()
      pty.lastExitCode = exitCode
      this.host.resolvePtyExitWaiters(pty, ptyId)
      this.host.pruneDisconnectedPtyTranscript(pty)
      this.host.touchMobileSessionSnapshotsForPty(ptyId, { immediate: true })
    }

    for (const leaf of this.host.getLeavesForPty(ptyId)) {
      this.host.getGraph().detachedPreAllocatedLeaves.delete(ptyId)
      leaf.connected = false
      leaf.writable = false
      leaf.lastExitCode = exitCode
      this.host.resolveExitWaiters(leaf)
      const handle = this.host
        .getGraph()
        .handleByLeafKey.get(this.host.getLeafKey(leaf.tabId, leaf.leafId))
      if (handle) {
        // Why fire-and-forget through the keyed queue, not `await` here:
        // onPtyExit() is itself a synchronous void event handler (see class
        // doc comment) — it cannot become async without cascading up through
        // whatever raw PTY-exit source calls it. The queue guarantees this
        // call and any other orchestration-DB write for the same `handle`
        // (message delivery, another exit) still run strictly in order.
        void this.host
          .getOrchestrationDeliveryQueue()
          .run(handle, () => this.failActiveDispatchOnExit(handle, exitCode))
      }
    }
    this.host.pruneDisconnectedPtyRecords()
  }

  // Why: Section 7.2 — the runtime detects agent exit directly and updates
  // dispatch contexts immediately, rather than waiting for the coordinator's
  // next poll cycle. This catches agent crashes and unexpected exits within
  // milliseconds. The task is set back to 'pending' so it can be re-dispatched.
  // Why async (ADR-021) + `handle: string` (not `leaf`, unlike before): the
  // caller resolves `handle` synchronously (graph lookups only) and hands
  // this off to the KeyedAsyncQueue keyed by that handle — see onPtyExit()'s
  // call site.
  private async failActiveDispatchOnExit(handle: string, exitCode: number): Promise<void> {
    const db = this.host.getRawOrchestrationDb()
    if (!db) {
      return
    }

    const dispatch = await db.getActiveDispatchForTerminal(handle)
    if (!dispatch) {
      return
    }

    const errorContext = `Agent exited with code ${exitCode}`
    await db.failDispatch(dispatch.id, errorContext)

    // Why: create an escalation message so the coordinator is notified about
    // the unexpected exit on its next check cycle, even if the circuit breaker
    // hasn't tripped yet.
    const run = await db.getActiveCoordinatorRun()
    if (run) {
      await db.insertMessage({
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
}
