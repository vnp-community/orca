// frontend/src/main/runtime/orca-runtime-pty-data-ingest.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-074): onPtyData — the single
// hottest method in OrcaRuntimeService, invoked on every PTY output chunk —
// extracted via the composition pattern, same technique as
// TASK-BIGFILE-072's waitForTerminal: the dispatcher itself moves out,
// its ~40 already-extracted collaborator domains stay reachable as host
// deps. Genuinely the highest call-frequency hot path in the file (every
// byte of every terminal's output flows through this), with zero test
// coverage per gitnexus impact scan — same risk tier as
// TASK-BIGFILE-067/068/069, now applied to the dispatcher rather than a
// callee.
import { advertisedUrlWatcher } from '../ports/advertised-url-watcher'
import { serveSimStateWatcher } from '../emulator/serve-sim-state-watcher'
import { extractOscTitleScanTail } from '../../shared/osc-title-scan-tail'
import type { AgentDetector } from '../stats/agent-detector'
import {
  appendNormalizedToTailBuffer,
  buildPreview,
  computeTerminalTailWaitState,
  normalizeTerminalChunk,
  tailGainedNewerBlockedReason,
  tailStateMatches
} from './orca-runtime-tail-buffer'
import type { RuntimeSyncedLeaf } from '../../shared/runtime-types'
import type { RuntimeLeafRecord, RuntimePtyWorktreeRecord } from './orca-runtime'
import type { RuntimeGraphStore } from './orca-runtime-graph-store'
import type { RuntimePtyTranscriptStore } from './orca-runtime-pty-transcript-store'
import type { RuntimeTerminalSideEffectsCommands } from './orca-runtime-terminal-side-effects'
import type { RuntimeHeadlessTerminalCommands } from './orca-runtime-headless-terminal'
import type { RuntimePtyWaitBlockedCheckCommands } from './orca-runtime-pty-wait-blocked-check'
import type { RuntimePtyTitleTrackerCommands } from './orca-runtime-pty-title-tracker'
import type { RuntimeAgentRowSnapshotCommands } from './orca-runtime-agent-row-snapshot'
import type { RuntimeMobileSessionTabsCommands } from './orca-runtime-mobile-session-tabs'

export type RuntimePtyDataIngestCommandHost = {
  getGraph(): RuntimeGraphStore
  getPtyTranscripts(): RuntimePtyTranscriptStore
  getAgentDetector(): AgentDetector | null
  getDataListeners(): Map<
    string,
    Set<(data: string, meta?: { seq?: number; rawLength?: number; cwd?: string }) => void>
  >
  recordOsc7MetadataForPty: RuntimeTerminalSideEffectsCommands['recordOsc7MetadataForPty']
  processAgentStatusOscForPty: RuntimeTerminalSideEffectsCommands['processAgentStatusOscForPty']
  flushPendingTerminalSideEffectFacts: RuntimeTerminalSideEffectsCommands['flushPendingTerminalSideEffectFacts']
  shouldAnswerQueriesForLiveChunk: RuntimeHeadlessTerminalCommands['shouldAnswerQueriesForLiveChunk']
  maybeHydrateHeadlessFromRenderer: RuntimeHeadlessTerminalCommands['maybeHydrateHeadlessFromRenderer']
  trackHeadlessTerminalData: RuntimeHeadlessTerminalCommands['trackHeadlessTerminalData']
  scheduleWaitBlockedCheck: RuntimePtyWaitBlockedCheckCommands['scheduleWaitBlockedCheck']
  getOrCreatePtyTitleTrackerEntry: RuntimePtyTitleTrackerCommands['getOrCreatePtyTitleTrackerEntry']
  emitTerminalAgentStatusEvents: RuntimeAgentRowSnapshotCommands['emitTerminalAgentStatusEvents']
  touchMobileSessionSnapshotsForPty: RuntimeMobileSessionTabsCommands['touchMobileSessionSnapshotsForPty']
  recordRecentPtyOutputForPathProvenance(ptyId: string, data: string): void
  getOrCreatePtyWorktreeRecord(ptyId: string): RuntimePtyWorktreeRecord | null
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
  getLeavesForPty(ptyId: string): RuntimeLeafRecord[]
  makeRuntimePaneKey(leaf: Pick<RuntimeSyncedLeaf, 'tabId' | 'leafId' | 'paneRuntimeId'>): string
}

// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-074): the onPtyData ingestion
// pipeline — sequence tracking, OSC7/agent-status extraction, tail-buffer
// updates for both the PTY record and its leaves, OSC title-tracker feed,
// and subscriber fanout. All in strict byte order per chunk; do not reorder
// steps relative to each other without re-reading the inline "Why"/"DO NOT
// REORDER" comments preserved from the original.
export class RuntimePtyDataIngestCommands {
  constructor(private readonly host: RuntimePtyDataIngestCommandHost) {}

  onPtyData(ptyId: string, data: string, at: number, sequenceChars = data.length): number {
    const outputSequence =
      (this.host.getPtyTranscripts().ptyOutputSequenceById.get(ptyId) ?? 0) + sequenceChars
    this.host.getPtyTranscripts().ptyOutputSequenceById.set(ptyId, outputSequence)
    const osc7Metadata = this.host.recordOsc7MetadataForPty(ptyId, data)
    const cwd = osc7Metadata.cwd
    const cwdChanged = osc7Metadata.cwdChanged
    const agentStatusChunk = this.host.processAgentStatusOscForPty(ptyId, data)
    this.host.recordRecentPtyOutputForPathProvenance(ptyId, data)
    // Agent detection runs on raw data before leaf processing, since the
    // tail buffer logic normalizes away the OSC sequences we need.
    this.host.getAgentDetector()?.onData(ptyId, data, at)
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
    const forwardQueryReplies = this.host.shouldAnswerQueriesForLiveChunk(ptyId)
    // Ordering invariant (DO NOT REORDER): maybeHydrateHeadlessFromRenderer
    // MUST run before trackHeadlessTerminalData so the eager-state pattern
    // (set headlessTerminals + writeChain head = seedPromise) is in place
    // before the live byte's chain link is queued. Without this ordering,
    // trackHeadlessTerminalData would lazy-create a fresh state at PTY dims
    // that the later seed-resolve would overwrite, dropping the live byte.
    // See docs/mobile-prefer-renderer-scrollback.md.
    this.host.maybeHydrateHeadlessFromRenderer(ptyId)
    // Our structure wins: OSC title/agent-status extraction runs through the
    // shared per-PTY title tracker below (getOrCreatePtyTitleTrackerEntry →
    // applyTrackedPtyTitle) in byte order, superseding main's inline
    // extractLastOscTitleForPty block (#7880/#7852 title/status semantics are
    // preserved via the tracker + detectAgentStatusFromTitle path).
    this.host.trackHeadlessTerminalData(ptyId, data, outputSequence, forwardQueryReplies)

    const pty = this.host.getOrCreatePtyWorktreeRecord(ptyId)
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
      this.host.scheduleWaitBlockedCheck(ptyId, normalized.text, at)
    }

    for (const leaf of this.host.getLeavesForPty(ptyId)) {
      this.host.recordPtyWorktree(ptyId, leaf.worktreeId, {
        connected: true,
        lastOutputAt: pty?.lastOutputAt ?? at,
        preview: pty?.preview ?? leaf.preview,
        tabId: leaf.tabId,
        paneKey: this.host.makeRuntimePaneKey(leaf)
      })
      leaf.connected = true
      leaf.writable = this.host.getGraph().graphStatus === 'ready'
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
    const titleTrackerEntry = this.host.getOrCreatePtyTitleTrackerEntry(ptyId)
    const previousTitleScanTail = this.host.getPtyTranscripts().oscTitleScanTailByPtyId.get(ptyId)
    const titleInput = previousTitleScanTail
      ? `${previousTitleScanTail}${agentStatusChunk.cleanData}`
      : agentStatusChunk.cleanData
    const nextTitleScanTail = extractOscTitleScanTail(titleInput)
    if (nextTitleScanTail.length > 0) {
      this.host.getPtyTranscripts().oscTitleScanTailByPtyId.set(ptyId, nextTitleScanTail)
    } else {
      this.host.getPtyTranscripts().oscTitleScanTailByPtyId.delete(ptyId)
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
        retainedAgentStatusChanged = this.host.emitTerminalAgentStatusEvents(
          ptyId,
          agentStatusChunk
        )
      } finally {
        // Why: flushed in the finally so a throwing tracker callback cannot
        // strand this chunk's facts to be emitted under the next chunk's seq.
        this.host.flushPendingTerminalSideEffectFacts(ptyId, titleTrackerEntry)
      }
    }
    // Why: hook (OSC 9999) transitions often arrive without a title change, so
    // headless-serve snapshots would never republish and paired remote clients
    // kept the stale agent state until the next title change (#7970).
    if (titleTrackerEntry.chunkTouchedSessionTabs || retainedAgentStatusChanged) {
      this.host.touchMobileSessionSnapshotsForPty(ptyId)
    }

    const listeners = this.host.getDataListeners().get(ptyId)
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
}
