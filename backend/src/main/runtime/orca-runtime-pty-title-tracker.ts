/* eslint-disable max-lines -- Why: straight extraction of orca-runtime.ts's
per-PTY title-tracking/seeded-agent-status command block (7 methods),
already covered by orca-runtime.ts's own grandfathered max-lines disable
before this move. Registered in config/max-lines-baseline.txt per
AGENTS.md — NEEDS PR REVIEW. One of the highest-risk domains moved off
orca-runtime.ts (15 host deps, onPtyData-adjacent hot path, zero test
coverage) — TASK-BIGFILE-057 cancelled this same extraction earlier;
re-attempted and completed after TASK-BIGFILE-060/064 narrowed the host
surface, with the user explicitly accepting the risk. */
// frontend/src/main/runtime/orca-runtime-pty-title-tracker.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-067): per-PTY title-tracking and
// seeded-agent-status commands extracted from OrcaRuntimeService via the
// composition pattern. TASK-BIGFILE-057 cancelled this same extraction —
// re-attempted after TASK-BIGFILE-060/064 already made
// refreshPtyForegroundAgentFromController/getPendingForegroundAgentRefreshForTitle/
// delayPtyBackedMobileSnapshotForForegroundAgent public+forwarded (no longer
// need new forwarding for those 3), significantly narrowing the remaining
// host-dependency surface to the same shape as other successfully extracted
// PTY-core domains (headless-terminal, TASK-BIGFILE-064). User explicitly
// accepted the risk to proceed (zero test coverage, onPtyData-adjacent hot
// path) — see TASK-BIGFILE-067's task doc for the full host-dependency audit.
import {
  createTerminalTitleTracker,
  stripBrailleSpinnerGlyphs
} from '../../shared/terminal-output-side-effects'
import { detectAgentStatusFromTitle, normalizeTerminalTitle } from '../../shared/agent-detection'
import { createCommandCodeOutputStatusDetector } from '../../shared/command-code-output-status'
import type { TerminalGitHubPRLink } from '../../shared/terminal-github-pr-link-detector'
import type { TerminalSideEffectFact } from '../../shared/terminal-side-effect-facts'
import type {
  RuntimeLeafRecord,
  RuntimePtyTitleTrackerEntry,
  RuntimePtyWorktreeRecord
} from './orca-runtime'
import type { RuntimeGraphStore } from './orca-runtime-graph-store'
import type { RuntimePtyTranscriptStore } from './orca-runtime-pty-transcript-store'

export type RuntimePtyTitleTrackerCommandHost = {
  getGraph(): RuntimeGraphStore
  getPtyTranscripts(): RuntimePtyTranscriptStore
  getOnTerminalSideEffects(): boolean
  getLeavesForPty(ptyId: string): RuntimeLeafRecord[]
  nextTitleObservationSequence(): number
  setPtyManagementTitleFromObservedTitle(
    pty: RuntimePtyWorktreeRecord,
    title: string | null | undefined,
    observedAt: number
  ): void
  recordTerminalSideEffectFact(ptyId: string, fact: TerminalSideEffectFact): void
  touchMobileSessionSnapshotsForPty(ptyId: string, options?: { immediate?: boolean }): void
  resolveTuiIdleWaiters(leaf: RuntimeLeafRecord): void
  resolvePtyTuiIdleWaiters(pty: RuntimePtyWorktreeRecord, ptyId: string): void
  deliverPendingMessages(leaf: RuntimeLeafRecord): void
  shouldDelayPtyBackedMobileSnapshotForForegroundAgent(
    pty: RuntimePtyWorktreeRecord,
    title: string
  ): boolean
  refreshPtyForegroundAgentFromController(
    ptyId: string,
    options?: { afterTitleObservation?: number }
  ): Promise<boolean>
  getPendingForegroundAgentRefreshForTitle(
    ptyId: string,
    titleObservedAt: number
  ): Promise<boolean> | undefined
  delayPtyBackedMobileSnapshotForForegroundAgent(
    ptyId: string,
    titleObservedAt: number,
    foregroundRefresh: Promise<boolean>
  ): void
}

export class RuntimePtyTitleTrackerCommands {
  constructor(private readonly host: RuntimePtyTitleTrackerCommandHost) {}

  /** Raw last title from main's tracked PTY/leaf records — the title surface
   *  the tracker (live bytes + synthetic frames) keeps current. */
  // Why: also called from OrcaRuntimeService outside this domain (headless-terminal host wiring, TASK-BIGFILE-064) — public, not private.
  getTrackedRawTitleForPty(ptyId: string): string | null {
    const recordTitle = this.host.getGraph().ptysById.get(ptyId)?.lastOscTitle
    if (recordTitle) {
      return recordTitle
    }
    for (const leaf of this.host.getLeavesForPty(ptyId)) {
      if (leaf.lastOscTitle) {
        return leaf.lastOscTitle
      }
    }
    return null
  }

  /** Why: synthetic agent title frames no longer ride pty:data, so neither
   *  renderer xterm nor the headless emulator observes them. Mobile-parity
   *  snapshot titles must prefer main's tracker over snapshot lastTitle, or
   *  hook-driven spinner/idle titles vanish from mobile tabs. */
  // Why: also called from OrcaRuntimeService outside this domain (headless-terminal host wiring, TASK-BIGFILE-064) — public, not private.
  preferTrackedLastTitle<T extends { lastTitle?: string }>(ptyId: string, snapshot: T): T {
    const tracked = this.getTrackedRawTitleForPty(ptyId)
    if (!tracked) {
      return snapshot
    }
    return { ...snapshot, lastTitle: tracked }
  }

  /** Decorative comparison key: spinner frame glyphs stripped, derived agent
   *  status kept so a working→idle flip with an otherwise-equal label still
   *  counts as a change. */
  private makeMobileTitleGateKey(rawTitle: string, normalizedTitle: string): string {
    return `${detectAgentStatusFromTitle(rawTitle) ?? ''}\u0000${stripBrailleSpinnerGlyphs(
      normalizedTitle
    )}`
  }

  // Why: seed-derived agent status reflects historical state. Orchestration
  // waiters (resolveTuiIdleWaiters, deliverPendingMessages) must only react
  // to LIVE transitions, so this helper writes leaf.lastAgentStatus only and
  // never resolves waiters. detectAgentStatusFromTitle wrap mirrors the live
  // path so seeded and live values are the same union member, keeping
  // downstream `=== 'idle'` checks correct.
  // Why: also called from OrcaRuntimeService outside this domain (headless-terminal host wiring, TASK-BIGFILE-064) — public, not private.
  applySeededAgentStatus(ptyId: string, title: string): void {
    if (!title) {
      return
    }
    // Why: a relaunched main starts its per-PTY title tracker cold — without
    // this seed it misses the parked working→idle completion and never arms
    // the stale-title timer for a persisted 'working' title. Seeding no-ops
    // once a live title was observed, so live state always wins.
    this.getOrCreatePtyTitleTrackerEntry(ptyId).tracker.seedInitialTitle(title)
    const status = detectAgentStatusFromTitle(title)
    // Why: live observations store normalized titles, so seeds must match —
    // otherwise the first live frame after hydration compares unequal and
    // touches session tabs once for no visible change.
    const seededTitle = normalizeTerminalTitle(title)
    const graph = this.host.getGraph()
    const pty = graph.ptysById.get(ptyId)
    if (pty) {
      const observedAt = this.host.nextTitleObservationSequence()
      pty.lastOscTitle = seededTitle
      pty.lastOscTitleAt = observedAt
      this.host.setPtyManagementTitleFromObservedTitle(pty, seededTitle, observedAt)
    }
    for (const leaf of this.host.getLeavesForPty(ptyId)) {
      // Why: seed lastOscTitle even when the seeded title doesn't classify
      // as an agent state, so worktree.ps recomputes status from the live
      // title rather than treating the leaf as agentless.
      leaf.lastOscTitle = seededTitle
      leaf.lastOscTitleAt = this.host.nextTitleObservationSequence()
      if (status !== null) {
        leaf.lastAgentStatus = status
      }
    }
  }

  // Why: also called from OrcaRuntimeService outside this domain (onPtyData/processAgentStatusOscForPty) — public, not private.
  getOrCreatePtyTitleTrackerEntry(ptyId: string): RuntimePtyTitleTrackerEntry {
    const ptyTranscripts = this.host.getPtyTranscripts()
    const existing = ptyTranscripts.ptyTitleTrackersByPtyId.get(ptyId)
    if (existing) {
      return existing
    }
    const graph = this.host.getGraph()
    // Why: trackers are created lazily on the first observed chunk. After an
    // app relaunch the PTY/leaf records can already hold a persisted title; a
    // cold tracker would miss the parked working→idle completion and never
    // arm the stale-title timer for a persisted 'working' title.
    let initialTitle = graph.ptysById.get(ptyId)?.lastOscTitle ?? null
    if (initialTitle === null) {
      for (const leaf of this.host.getLeavesForPty(ptyId)) {
        if (leaf.lastOscTitle) {
          initialTitle = leaf.lastOscTitle
          break
        }
      }
    }
    const hasSideEffectConsumer = this.host.getOnTerminalSideEffects()
    const tracker = createTerminalTitleTracker(
      {
        onTitle: (normalizedTitle, rawTitle, meta) => {
          this.host.recordTerminalSideEffectFact(ptyId, {
            kind: 'title',
            normalizedTitle,
            rawTitle,
            ...(meta?.staleWorkingTitleClear ? { staleWorkingTitleClear: true } : {})
          })
          const changed = this.applyTrackedPtyTitle(ptyId, rawTitle, normalizedTitle)
          if (!changed) {
            return
          }
          const live = ptyTranscripts.ptyTitleTrackersByPtyId.get(ptyId)
          const gateKey = this.makeMobileTitleGateKey(rawTitle, normalizedTitle)
          const decorativeOnly = live?.lastMobileTitleGateKey === gateKey
          if (live) {
            live.lastMobileTitleGateKey = gateKey
          }
          if (live?.applyingChunk) {
            // Why: synthetic spinner ticks change only the braille glyph
            // ~12.5x/sec; fanning out full mobile session snapshots per frame
            // is pure churn. Raw lastOscTitle updates above stay cheap.
            if (!(live.applyingSyntheticFrame && decorativeOnly)) {
              live.chunkTouchedSessionTabs = true
            }
          } else {
            // Stale-working-title timer path — fires between chunks, so the
            // per-chunk batching in onPtyData cannot pick it up.
            this.host.touchMobileSessionSnapshotsForPty(ptyId)
          }
        },
        // Why: agent transitions and bells become pty:sideEffect facts —
        // main is the single byte parser for local/SSH PTYs; the renderer
        // store handler decides what the facts mean (notification policy).
        onAgentBecameWorking: () => {
          this.host.recordTerminalSideEffectFact(ptyId, { kind: 'agent-working' })
        },
        onAgentBecameIdle: (title, meta) => {
          this.host.recordTerminalSideEffectFact(ptyId, {
            kind: 'agent-idle',
            title,
            ...(meta?.staleWorkingTitleClear ? { staleWorkingTitleClear: true } : {})
          })
        },
        onAgentExited: () => {
          this.host.recordTerminalSideEffectFact(ptyId, { kind: 'agent-exited' })
        },
        // Why: bell/command-finished/pr-link/2031 facts exist only for the
        // pty:sideEffect channel. Headless serve has no consumer, so skip the
        // per-chunk bell walk and 133/URL/2031 scans entirely.
        ...(hasSideEffectConsumer
          ? {
              onBell: () => {
                this.host.recordTerminalSideEffectFact(ptyId, { kind: 'bell' })
              },
              onCommandFinished: (exitCode: number | null) => {
                this.host.recordTerminalSideEffectFact(ptyId, {
                  kind: 'command-finished',
                  exitCode
                })
              },
              onPrLink: (link: TerminalGitHubPRLink) => {
                this.host.recordTerminalSideEffectFact(ptyId, { kind: 'pr-link', link })
              },
              // Why: hidden-delivery-gated views never see the bytes, so main
              // surfaces DECSET 2031 subscribes as facts; the theme reply is
              // still sent by the renderer (query authority stays with the view).
              onMode2031Subscribe: () => {
                this.host.recordTerminalSideEffectFact(ptyId, { kind: '2031-subscribe' })
              }
            }
          : {})
      },
      initialTitle !== null ? { initialTitle } : {}
    )
    const entry: RuntimePtyTitleTrackerEntry = {
      tracker,
      applyingChunk: false,
      applyingSyntheticFrame: false,
      lastMobileTitleGateKey: null,
      chunkTouchedSessionTabs: false,
      pendingFacts: [],
      // Why: command-code facts exist only for the pty:sideEffect channel —
      // headless serve skips the per-chunk scrape entirely. The detector
      // self-arms on the Command Code banner; the spawn command (when main
      // saw one) mirrors the renderer detector's startupCommand fast-arm.
      commandCodeDetector: hasSideEffectConsumer
        ? createCommandCodeOutputStatusDetector({
            startupCommand: ptyTranscripts.terminalSpawnCommandsByPtyId.get(ptyId) ?? null,
            onWorking: (prompt) => {
              this.host.recordTerminalSideEffectFact(ptyId, {
                kind: 'command-code-working',
                prompt
              })
            },
            onDone: (prompt) => {
              this.host.recordTerminalSideEffectFact(ptyId, { kind: 'command-code-done', prompt })
            }
          })
        : null
    }
    ptyTranscripts.ptyTitleTrackersByPtyId.set(ptyId, entry)
    return entry
  }

  /** Apply one observed OSC title (raw form) to the PTY and leaf records.
   *  Returns true when the PTY record's title or status changed. */
  // Why: also called from OrcaRuntimeService outside this domain (onPtyData) — public, not private.
  applyTrackedPtyTitle(ptyId: string, rawTitle: string, normalizedTitle: string): boolean {
    // Why: status is detected from the RAW title (mirrors the renderer tracker),
    // so working/idle transitions are unaffected by normalization; the records
    // store the NORMALIZED title so rotating Grok/Pi/Gemini frames collapse to
    // one stable stored label (#7880) instead of churning `ps`/mobile tabs.
    const agentStatus = detectAgentStatusFromTitle(rawTitle)
    let ptyRecordChanged = false
    const graph = this.host.getGraph()
    const pty = graph.ptysById.get(ptyId)
    if (pty) {
      const prevStatus = pty.lastAgentStatus
      const prevTitle = pty.lastOscTitle
      const observedAt = this.host.nextTitleObservationSequence()
      pty.lastOscTitle = normalizedTitle
      pty.lastOscTitleAt = observedAt
      pty.lastAgentStatus = agentStatus
      this.host.setPtyManagementTitleFromObservedTitle(pty, normalizedTitle, observedAt)
      ptyRecordChanged = prevTitle !== normalizedTitle || prevStatus !== agentStatus
      if (agentStatus === 'idle' && prevStatus !== 'idle') {
        this.host.resolvePtyTuiIdleWaiters(pty, ptyId)
      }
      const shouldDelayMobileSnapshot =
        ptyRecordChanged &&
        this.host.shouldDelayPtyBackedMobileSnapshotForForegroundAgent(pty, normalizedTitle)
      let foregroundRefresh: Promise<boolean> | undefined
      // Why: gate on an actual status transition — braille spinner frames
      // mutate the title every tick, so probing per-title-change would stream
      // a foreground query per frame during active work.
      if (prevStatus !== agentStatus) {
        foregroundRefresh = this.host.refreshPtyForegroundAgentFromController(ptyId, {
          afterTitleObservation: observedAt
        })
      } else if (shouldDelayMobileSnapshot) {
        // Why: same-status compatible title changes can arrive before the
        // foreground owner probe settles; publishing them would flicker.
        foregroundRefresh = this.host.getPendingForegroundAgentRefreshForTitle(ptyId, observedAt)
      }
      if (foregroundRefresh && shouldDelayMobileSnapshot) {
        // Why: report "unchanged" so the per-chunk batch skips the mobile
        // snapshot fan-out; the delayed publish fires when the probe settles.
        ptyRecordChanged = false
        this.host.delayPtyBackedMobileSnapshotForForegroundAgent(
          ptyId,
          observedAt,
          foregroundRefresh
        )
      }
    }
    for (const leaf of this.host.getLeavesForPty(ptyId)) {
      // Why: keep the latest OSC title on the leaf so worktree.ps can
      // recompute status from the live title each call. Without this,
      // daemon-hosted terminals (no renderer pushing pane titles) had no
      // way to clear a stale 'working' status after the agent exited and
      // the shell took over the title — the stuck-spinner bug in #1437.
      leaf.lastOscTitle = normalizedTitle
      leaf.lastOscTitleAt = this.host.nextTitleObservationSequence()
      const prevStatus = leaf.lastAgentStatus
      // Why: when a new OSC title doesn't classify as an agent state (e.g.
      // bare shell title after the agent exits), clear lastAgentStatus so
      // it is no longer sticky. Tui-idle waiters that needed the previous
      // 'idle' transition were already resolved at the moment of the
      // transition below; only fresh waiters registered after the agent
      // exits would observe the cleared value, and they correctly fall
      // back to title-based detection / polling.
      leaf.lastAgentStatus = agentStatus
      // Why: resolve tui-idle on any transition TO idle (not just working→idle).
      // Claude Code may skip "working" entirely on fast tasks, going null→idle,
      // and the coordinator's tui-idle waiter would hang forever waiting for a
      // working→idle transition that never comes. Permission→idle is excluded:
      // it means the agent was blocked on user approval and the user said no,
      // which isn't a task-completion signal.
      if (agentStatus === 'idle' && prevStatus !== 'idle') {
        this.host.resolveTuiIdleWaiters(leaf)
        this.host.deliverPendingMessages(leaf)
      }
    }
    return ptyRecordChanged
  }

  /** Cancel the per-PTY title tracker (stale-title timer included) on PTY
   *  teardown so it cannot fire into pruned records. */
  // Why: also called from OrcaRuntimeService outside this domain (onPtyExit, pruneDisconnectedPtyRecords) — public, not private.
  disposePtyTitleTracker(ptyId: string): void {
    const ptyTranscripts = this.host.getPtyTranscripts()
    ptyTranscripts.ptyTitleTrackersByPtyId.get(ptyId)?.tracker.dispose()
    ptyTranscripts.ptyTitleTrackersByPtyId.delete(ptyId)
  }
}
