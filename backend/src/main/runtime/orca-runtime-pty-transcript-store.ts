// frontend/src/main/runtime/orca-runtime-pty-transcript-store.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-063): pure state container for the
// per-PTY OSC-parsing/transcript-scan caches that TASK-BIGFILE-054 found
// touched by the shared disconnect-cleanup sweep (dropDisconnectedPtyRecord)
// AND deeply entangled processing methods (onPtyData, applyTrackedPtyTitle,
// processAgentStatusOscForPty) — same "state lõi, không tách được bằng Move"
// classification RuntimeGraphStore (TASK-BIGFILE-041) resolved for the
// leaf/tab/handle/waiter fields. This is NOT a domain Move — no methods
// move, only field storage. OrcaRuntimeService keeps a single
// `private readonly ptyTranscripts = new RuntimePtyTranscriptStore()` and
// accesses these 10 fields via `this.ptyTranscripts.<field>` instead of
// `this.<field>`, so future domain-Move tasks (title-tracker, OSC-status
// processing, ...) can receive RuntimePtyTranscriptStore through a
// constructor instead of reading OrcaRuntimeService's private fields
// directly.
import type { createAgentStatusOscProcessor } from '../../shared/agent-status-osc'
import type { RuntimePtyTitleTrackerEntry } from './orca-runtime'

export class RuntimePtyTranscriptStore {
  // Why: startup draft paste can subscribe after the agent already emitted its
  // ready marker. Keep a bounded raw buffer so fast startup output is replayed.
  readonly recentPtyOutputById = new Map<string, string>()
  readonly ptyOutputSequenceById = new Map<string, number>()
  readonly recentPtyPathCandidatesById = new Map<string, string[]>()
  // Why: OSC 9999 status can span PTY chunks. Keeping parser state in the
  // runtime lets hidden/model-owned terminals observe agent state without a
  // mounted xterm view.
  readonly agentStatusOscProcessorsByPtyId = new Map<
    string,
    ReturnType<typeof createAgentStatusOscProcessor>
  >()
  // Why: per-PTY shared title trackers (all-titles ordering + stale-working
  // timer) replace last-title-per-chunk scanning so main observes the same
  // intra-chunk working→idle transitions the renderer does (issue #1083).
  // Lazily created like agentStatusOscProcessorsByPtyId; disposed on PTY exit.
  readonly ptyTitleTrackersByPtyId = new Map<string, RuntimePtyTitleTrackerEntry>()
  // Why: the Command Code output detector arms early from the launch command
  // when known (banner detection covers user-typed launches), mirroring the
  // renderer detector's startupCommand seed.
  readonly terminalSpawnCommandsByPtyId = new Map<string, string>()
  // Why: ordinary OSC 0/1/2 titles can split across PTY chunks, especially over
  // SSH/relay buffering. Keep a small raw scan tail and feed reconstructed
  // chunks into the title tracker instead of falling back to last-title scans.
  readonly oscTitleScanTailByPtyId = new Map<string, string>()
  // Why: mobile file taps resolve relative paths on the host. OSC 7 is the
  // terminal-owned cwd signal, and it can arrive in live output between snapshots.
  readonly osc7ScanTailByPtyId = new Map<string, string>()
  readonly terminalCwdByPtyId = new Map<string, string>()
  readonly terminalFileUriHostnameByPtyId = new Map<string, string>()
}
