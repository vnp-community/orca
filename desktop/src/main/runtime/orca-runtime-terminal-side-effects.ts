// frontend/src/main/runtime/orca-runtime-terminal-side-effects.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-068): PTY side-effect fact
// (pty:sideEffect) scanning/emission and OSC 7 cwd-tracking commands
// extracted from OrcaRuntimeService via the composition pattern. This is
// the OSC-processing counterpart to pty-title-tracker
// (TASK-BIGFILE-067) — same host-dependency shape, same
// onPtyData-adjacent hot-path risk, extracted immediately after with the
// user's standing high-risk acceptance.
import { createAgentStatusOscProcessor } from '../../shared/agent-status-osc'
import { isCursorNativeAgentTitle, normalizeTerminalTitle } from '../../shared/agent-detection'
import { isWindowsAbsolutePathLike } from '../../shared/cross-platform-path'
import { splitWorktreeIdForFilesystem } from '../../shared/worktree-id'
import { extractLastOsc7Uri, extractOscScanTail } from '../daemon/osc7-uri-extraction'
import { parseFileUriPathParts } from '../daemon/osc7-file-uri'
import type { PtyTransientFact } from '../providers/types'
import type {
  TerminalSideEffectBatch,
  TerminalSideEffectFact
} from '../../shared/terminal-side-effect-facts'
import type { RuntimeSyncedLeaf } from '../../shared/runtime-types'
import type {
  RuntimeLeafRecord,
  RuntimePtyTitleTrackerEntry,
  RuntimePtyWorktreeRecord
} from './orca-runtime'
import type { RuntimeGraphStore } from './orca-runtime-graph-store'
import type { RuntimePtyTranscriptStore } from './orca-runtime-pty-transcript-store'

export type RuntimeTerminalSideEffectsCommandHost = {
  getGraph(): RuntimeGraphStore
  getPtyTranscripts(): RuntimePtyTranscriptStore
  getOnTerminalSideEffects(): ((batch: TerminalSideEffectBatch) => void) | null
  getLeavesForPty(ptyId: string): RuntimeLeafRecord[]
  getOrCreatePtyTitleTrackerEntry(ptyId: string): RuntimePtyTitleTrackerEntry
  getOrCreatePtyWorktreeRecord(ptyId: string): RuntimePtyWorktreeRecord | null
  makeRuntimePaneKey(leaf: Pick<RuntimeSyncedLeaf, 'tabId' | 'leafId' | 'paneRuntimeId'>): string
  touchMobileSessionSnapshotsForPty(ptyId: string, options?: { immediate?: boolean }): void
  disposeHeadlessTerminal(ptyId: string): void
}

export class RuntimeTerminalSideEffectsCommands {
  constructor(private readonly host: RuntimeTerminalSideEffectsCommandHost) {}

  // Why: also called from OrcaRuntimeService outside this domain (onPtyData) — public, not private.
  processAgentStatusOscForPty(ptyId: string, data: string) {
    const ptyTranscripts = this.host.getPtyTranscripts()
    let processor = ptyTranscripts.agentStatusOscProcessorsByPtyId.get(ptyId)
    if (!processor) {
      processor = createAgentStatusOscProcessor()
      ptyTranscripts.agentStatusOscProcessorsByPtyId.set(ptyId, processor)
    }
    return processor(data)
  }

  /** Emit the facts batched while applying one chunk/frame as a single
   *  pty:sideEffect batch, preserving byte order. */
  // Why: also called from OrcaRuntimeService outside this domain (onPtyData) — public, not private.
  flushPendingTerminalSideEffectFacts(ptyId: string, entry: RuntimePtyTitleTrackerEntry): void {
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
    const entry = this.host.getOrCreatePtyTitleTrackerEntry(ptyId)
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
      this.host.touchMobileSessionSnapshotsForPty(ptyId)
    }
  }

  /** Scan-authority handoff for a backgrounded PTY (daemon keep-tail
   *  thinning): while delegated, the daemon relays bell/133/pr-link/2031
   *  facts itself and the delivered bytes may be gapped — feeding them to
   *  main's transient scanners would mint phantom or duplicate facts. Title
   *  processing stays main-side either way. */
  setPtyTransientFactDelegation(ptyId: string, delegated: boolean, scanSeedAnsi?: string): void {
    const entry = this.host.getOrCreatePtyTitleTrackerEntry(ptyId)
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
    const ptyTranscripts = this.host.getPtyTranscripts()
    if (droppedChars > 0) {
      // Why: the daemon snapshot's seq counts bytes its monitoring stream
      // dropped. Advancing without parsing preserves that absolute domain so
      // post-snapshot live chunks can be reconciled instead of duplicated.
      const outputSequence = (ptyTranscripts.ptyOutputSequenceById.get(ptyId) ?? 0) + droppedChars
      ptyTranscripts.ptyOutputSequenceById.set(ptyId, outputSequence)
    }
    const pty = this.host.getOrCreatePtyWorktreeRecord(ptyId)
    if (pty) {
      pty.tailPendingAnsi = ''
    }
    for (const leaf of this.host.getLeavesForPty(ptyId)) {
      leaf.tailPendingAnsi = ''
    }
    ptyTranscripts.oscTitleScanTailByPtyId.delete(ptyId)
    ptyTranscripts.osc7ScanTailByPtyId.delete(ptyId)
    ptyTranscripts.agentStatusOscProcessorsByPtyId.delete(ptyId)
    this.host.disposeHeadlessTerminal(ptyId)
  }

  /** Record one derived side-effect fact: batched per chunk while applying
   *  bytes, emitted immediately for between-chunk facts (stale-title timer). */
  // Why: also called from OrcaRuntimeService outside this domain (pty-title-tracker host wiring, TASK-BIGFILE-067) — public, not private.
  recordTerminalSideEffectFact(ptyId: string, fact: TerminalSideEffectFact): void {
    const onTerminalSideEffects = this.host.getOnTerminalSideEffects()
    if (!onTerminalSideEffects) {
      return
    }
    const entry = this.host.getPtyTranscripts().ptyTitleTrackersByPtyId.get(ptyId)
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
    const onTerminalSideEffects = this.host.getOnTerminalSideEffects()
    if (!onTerminalSideEffects || facts.length === 0) {
      return
    }
    const batch: TerminalSideEffectBatch = {
      ptyId,
      seq: this.host.getPtyTranscripts().ptyOutputSequenceById.get(ptyId) ?? 0,
      facts,
      ...(options.replay ? { replay: true } : {}),
      ...this.resolveTerminalSideEffectAttribution(ptyId)
    }
    try {
      onTerminalSideEffects(batch)
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
    const graph = this.host.getGraph()
    const pty = graph.ptysById.get(ptyId)
    const connectionId = pty?.connectionId ?? null
    for (const leaf of this.host.getLeavesForPty(ptyId)) {
      return {
        worktreeId: leaf.worktreeId,
        tabId: leaf.tabId,
        paneKey: this.host.makeRuntimePaneKey(leaf),
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
    const ptyTranscripts = this.host.getPtyTranscripts()
    const tracker = ptyTranscripts.ptyTitleTrackersByPtyId.get(ptyId)?.tracker
    const recordTitle = this.host.getGraph().ptysById.get(ptyId)?.lastOscTitle
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
      seq: ptyTranscripts.ptyOutputSequenceById.get(ptyId) ?? 0,
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

  private extractLastOsc7CwdForPty(
    ptyId: string,
    data: string
  ): { path: string; hostname: string } | null {
    const ptyTranscripts = this.host.getPtyTranscripts()
    const previousTail = ptyTranscripts.osc7ScanTailByPtyId.get(ptyId)
    if (!previousTail && !data.includes('\x1b]7;')) {
      return null
    }
    const input = `${previousTail ?? ''}${data}`
    const scanTail = extractOscScanTail(input, 4096)
    if (scanTail.length > 0) {
      ptyTranscripts.osc7ScanTailByPtyId.set(ptyId, scanTail)
    } else {
      ptyTranscripts.osc7ScanTailByPtyId.delete(ptyId)
    }
    const uri = extractLastOsc7Uri(input)
    const pty = this.host.getGraph().ptysById.get(ptyId)
    const pathFlavor = this.pathFlavorForPty(pty)
    return uri
      ? parseFileUriPathParts(uri, {
          pathFlavor,
          remotePosixAuthority: !!pty?.connectionId && pathFlavor !== 'win32'
        })
      : null
  }

  // Why: also called from OrcaRuntimeService outside this domain (headless-terminal host wiring, TASK-BIGFILE-064) — public, not private.
  recordOsc7MetadataForPty(
    ptyId: string,
    data: string
  ): { cwd: string | null; cwdChanged: boolean } {
    const ptyTranscripts = this.host.getPtyTranscripts()
    const osc7 = this.extractLastOsc7CwdForPty(ptyId, data)
    const cwd = osc7?.path ?? null
    const cwdChanged =
      cwd !== null && cwd.trim().length > 0 && ptyTranscripts.terminalCwdByPtyId.get(ptyId) !== cwd
    if (cwdChanged) {
      ptyTranscripts.terminalCwdByPtyId.set(ptyId, cwd)
    }
    if (osc7) {
      if (osc7.hostname) {
        ptyTranscripts.terminalFileUriHostnameByPtyId.set(ptyId, osc7.hostname)
      } else {
        ptyTranscripts.terminalFileUriHostnameByPtyId.delete(ptyId)
      }
    }
    return { cwd, cwdChanged }
  }

  // Why: also called from OrcaRuntimeService outside this domain (headless-terminal host wiring, TASK-BIGFILE-064) — public, not private.
  pathFlavorForPty(pty?: RuntimePtyWorktreeRecord | null): 'posix' | 'win32' {
    if (!pty?.connectionId) {
      return process.platform === 'win32' ? 'win32' : 'posix'
    }
    const worktreePath = splitWorktreeIdForFilesystem(pty.worktreeId)?.worktreePath
    return worktreePath && isWindowsAbsolutePathLike(worktreePath) ? 'win32' : 'posix'
  }
}
