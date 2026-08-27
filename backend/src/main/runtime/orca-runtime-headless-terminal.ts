/* eslint-disable max-lines -- Why: straight extraction of orca-runtime.ts's
headless (server-owned) PTY emulator command block (16 methods), already
covered by orca-runtime.ts's own grandfathered max-lines disable before
this move. Registered in config/max-lines-baseline.txt per AGENTS.md —
NEEDS PR REVIEW. One of the largest/riskiest domains moved off
orca-runtime.ts (14 host deps, zero test coverage) — extracted only after
the user explicitly accepted that risk. */
// frontend/src/main/runtime/orca-runtime-headless-terminal.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-064): headless (server-owned) PTY
// emulator commands extracted from OrcaRuntimeService via the composition
// pattern. Field-span + method-body dependency analysis found 14 host
// dependencies — mostly other PTY-core private methods that stay in
// orca-runtime.ts (title-tracker, path-provenance, OSC-7 metadata) — but
// unlike pty-title-tracker (TASK-BIGFILE-057, cancelled) the dependency
// count and shape here matched the risk level of the largest domains
// already extracted (e.g. mobile-session-tabs cluster 1, TASK-BIGFILE-051),
// so the user explicitly accepted the risk to proceed.
import { HeadlessEmulator } from '../daemon/headless-emulator'
import {
  isNativeWindowsConptyPty,
  shouldModelAnswerHiddenPtyQueries
} from './terminal-model-query-authority'
import { getTerminalViewAttributes } from './terminal-view-attribute-store'
import type { TerminalViewAttributes } from '../../shared/terminal-view-attributes'
import { MOBILE_SUBSCRIBE_SCROLLBACK_ROWS } from './scrollback-limits'
import type { TerminalOscLinkRange } from '../../shared/terminal-osc-link-ranges'
import {
  buildVisibleSnapshotReadFallback,
  shouldFallbackToVisibleTerminalSnapshot
} from './orca-runtime-tail-buffer'
import type { RuntimeTerminalRead } from '../../shared/runtime-types'
import type { RuntimePtyController, RuntimePtyWorktreeRecord, RuntimeStore } from './orca-runtime'
import type { RuntimeGraphStore } from './orca-runtime-graph-store'
import type { RuntimePtyTranscriptStore } from './orca-runtime-pty-transcript-store'

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

export type RuntimeHeadlessTerminalCommandHost = {
  getGraph(): RuntimeGraphStore
  getStore(): RuntimeStore | null
  getPtyController(): RuntimePtyController | null
  getPtyTranscripts(): RuntimePtyTranscriptStore
  getHeadlessHydrationState(): Map<string, 'pending' | 'done'>
  getTerminalSize(ptyId: string): { cols: number; rows: number } | null
  hasRemoteTerminalViewSubscriber(ptyId: string): boolean
  recordOsc7MetadataForPty(ptyId: string, data: string): { cwd: string | null; cwdChanged: boolean }
  recordRecentPtyOutputForPathProvenance(ptyId: string, data: string): void
  getTrackedRawTitleForPty(ptyId: string): string | null
  applySeededAgentStatus(ptyId: string, title: string): void
  pathFlavorForPty(pty?: RuntimePtyWorktreeRecord | null): 'posix' | 'win32'
  preferTrackedLastTitle<T extends { lastTitle?: string }>(ptyId: string, snapshot: T): T
}

export class RuntimeHeadlessTerminalCommands {
  private readonly headlessTerminals = new Map<string, RuntimeHeadlessTerminal>()

  constructor(private readonly host: RuntimeHeadlessTerminalCommandHost) {}

  // Why: also called from OrcaRuntimeService outside this domain (constructor's registerTerminalViewAttributesApplier hook) — public, not private.
  applyPushedViewAttributesToAll(attributes: TerminalViewAttributes): void {
    for (const state of this.headlessTerminals.values()) {
      state.emulator.applyPushedViewAttributes(attributes)
    }
  }

  // Why: also called from OrcaRuntimeService outside this domain (isTerminalAlternateScreen, not moved) — public, not private.
  isAlternateScreen(ptyId: string): boolean {
    return this.headlessTerminals.get(ptyId)?.emulator.isAlternateScreen ?? false
  }

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
    const dims = size ?? this.host.getTerminalSize(ptyId) ?? { cols: 80, rows: 24 }
    const state = this.createPtyHeadlessTerminalState(ptyId, dims)
    this.headlessTerminals.set(ptyId, state)
    this.host.recordOsc7MetadataForPty(ptyId, data)
    this.host.recordRecentPtyOutputForPathProvenance(ptyId, data)
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
  // Why: also called from OrcaRuntimeService outside this domain (onPtyData) — public, not private.
  maybeHydrateHeadlessFromRenderer(ptyId: string): void {
    const headlessHydrationState = this.host.getHeadlessHydrationState()
    if (headlessHydrationState.has(ptyId)) {
      return
    }
    if (this.headlessTerminals.has(ptyId)) {
      // Daemon-snapshot seed already populated the emulator — skip hydration.
      headlessHydrationState.set(ptyId, 'done')
      return
    }
    const controller = this.host.getPtyController()
    if (!controller?.serializeBuffer || !controller.hasRendererSerializer) {
      return
    }
    if (!controller.hasRendererSerializer(ptyId)) {
      // Renderer hasn't registered yet (or never will). Live writes lazy-
      // create the state via trackHeadlessTerminalData on this same tick.
      return
    }

    headlessHydrationState.set(ptyId, 'pending')
    const dims = this.host.getTerminalSize(ptyId) ?? { cols: 80, rows: 24 }
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
        this.host.recordOsc7MetadataForPty(ptyId, rendered.data)
        this.host.recordRecentPtyOutputForPathProvenance(ptyId, rendered.data)
        // Resize to renderer's dims so the seed reflows correctly into the
        // emulator's grid, then resize back to PTY dims (if known) so live
        // writes use the correct cell layout.
        if (rendered.cols !== dims.cols || rendered.rows !== dims.rows) {
          state.emulator.resize(rendered.cols, rendered.rows)
        }
        await state.emulator.write(rendered.data)
        const ptyDims = this.host.getTerminalSize(ptyId)
        if (ptyDims && (ptyDims.cols !== rendered.cols || ptyDims.rows !== rendered.rows)) {
          state.emulator.resize(ptyDims.cols, ptyDims.rows)
        }
        // Why: the renderer xterm no longer sees synthetic hook title frames
        // (they feed main's tracker only), so its serializer lastTitle can be
        // stale here. Prefer main's tracked title; the renderer's is only the
        // seed when main has observed none (fresh relaunch, cold tracker).
        const seedTitle = this.host.getTrackedRawTitleForPty(ptyId) ?? rendered.lastTitle
        if (seedTitle) {
          state.emulator.setLastTitle(seedTitle)
          this.host.applySeededAgentStatus(ptyId, seedTitle)
        }
      } catch {
        // Hydration is best-effort. Live writes continue via the same
        // writeChain that this catch-arm leaves intact.
      } finally {
        headlessHydrationState.set(ptyId, 'done')
      }
    })
  }

  /** Per-chunk reply-ownership capture (Phase 5). Evaluated synchronously at
   *  ingestion only — never re-read at reply time. */
  // Why: also called from OrcaRuntimeService outside this domain (onPtyData) — public, not private.
  shouldAnswerQueriesForLiveChunk(ptyId: string): boolean {
    return shouldModelAnswerHiddenPtyQueries({
      ptyId,
      settings: this.host.getStore()?.getSettings(),
      hasRemoteViewSubscriber: this.host.hasRemoteTerminalViewSubscriber(ptyId)
    })
  }

  // Why: also called from OrcaRuntimeService outside this domain (onPtyData) — public, not private.
  trackHeadlessTerminalData(
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
    const graph = this.host.getGraph()
    const pathFlavor = this.host.pathFlavorForPty(graph.ptysById.get(ptyId))
    const emulator = new HeadlessEmulator({
      cols: dims.cols,
      rows: dims.rows,
      pathFlavor,
      remotePosixFileUriAuthority:
        !!graph.ptysById.get(ptyId)?.connectionId && pathFlavor !== 'win32',
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
          this.host.getPtyController()?.write(ptyId, reply)
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
  // Why: also called from OrcaRuntimeService outside this domain (constructor's registerConptyDa1OverrideInstaller hook) — public, not private.
  ensureNativeWindowsConptyDa1Override(ptyId: string): void {
    if (isNativeWindowsConptyPty(ptyId)) {
      this.headlessTerminals.get(ptyId)?.emulator.installConptyPrimaryDeviceAttributesOverride()
    }
  }

  private getOrCreateHeadlessTerminal(ptyId: string): RuntimeHeadlessTerminal {
    const existing = this.headlessTerminals.get(ptyId)
    if (existing) {
      return existing
    }
    const size = this.host.getTerminalSize(ptyId) ?? { cols: 80, rows: 24 }
    const state = this.createPtyHeadlessTerminalState(ptyId, size)
    this.headlessTerminals.set(ptyId, state)
    return state
  }

  // Why: also called from OrcaRuntimeService outside this domain (mobile-floor host wiring, TASK-BIGFILE-037) — public, not private.
  resizeHeadlessTerminal(ptyId: string, cols: number, rows: number): void {
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

  // Why: also called from OrcaRuntimeService outside this domain (serializeTerminalBuffer) — public, not private.
  async serializeTerminalBufferFromAvailableState(
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

  // Why: also called from OrcaRuntimeService outside this domain (serializeHiddenOutputRecoveryBuffer) — public, not private.
  async serializeRendererTerminalBuffer(
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
      rendererSnapshot = await (this.host.getPtyController()?.serializeBuffer?.(ptyId, {
        scrollbackRows: opts.scrollbackRows,
        altScreenForcesZeroRows: false
      }) ?? Promise.resolve(null))
    } catch {
      // Why: terminal snapshots should not depend on a mounted renderer pane.
      // If renderer serialization races reload/unmount, callers can still use
      // their existing null fallback paths.
    }
    return rendererSnapshot
      ? this.host.preferTrackedLastTitle(ptyId, {
          ...rendererSnapshot,
          cwd: rendererSnapshot.cwd ?? this.host.getPtyTranscripts().terminalCwdByPtyId.get(ptyId),
          source: 'renderer' as const
        })
      : null
  }

  // Why: also called from OrcaRuntimeService outside this domain (readTerminal/hidden-delivery read fallback) — public, not private.
  async withVisibleSnapshotFallback(
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
    const controller = this.host.getPtyController()
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

  // Why: also called from OrcaRuntimeService outside this domain (serializeMainTerminalBuffer, serializeHiddenOutputRecoveryBuffer) — public, not private.
  async serializeHeadlessTerminalBuffer(
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
      ? this.host.preferTrackedLastTitle(ptyId, {
          data,
          cols: snapshot.cols,
          rows: snapshot.rows,
          cwd: snapshot.cwd ?? this.host.getPtyTranscripts().terminalCwdByPtyId.get(ptyId),
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

  // Why: also called from OrcaRuntimeService outside this domain (onPtyExit, pruneDisconnectedPtyRecords) — public, not private.
  disposeHeadlessTerminal(ptyId: string): void {
    this.host.getHeadlessHydrationState().delete(ptyId)
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
}
