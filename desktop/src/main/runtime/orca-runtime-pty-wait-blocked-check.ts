// frontend/src/main/runtime/orca-runtime-pty-wait-blocked-check.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-059): PTY wait-blocked-reason
// polling commands extracted from OrcaRuntimeService via the composition
// pattern. Field-span analysis (TASK-BIGFILE-054) confirmed
// `waitBlockedCheckStateByPtyId` and these 3 methods are fully
// self-contained — only `this.graph` is a real cross-domain dependency.
import {
  MAX_TAIL_CHARS,
  type TerminalTailWaitState,
  computeTerminalTailWaitState,
  tailGainedNewerBlockedReason
} from './orca-runtime-tail-buffer'
import type { RuntimeGraphStore } from './orca-runtime-graph-store'

const WAIT_BLOCKED_CHECK_MIN_INTERVAL_MS = 50
// Why: chunks that can complete an actionable prompt bypass the throttle so
// blocked stamps stay per-chunk-immediate; the pattern heads mirror
// findTerminalWaitBlockedSignal. Scanned over the new chunk plus a short
// carry only — never the accumulated window.
const WAIT_BLOCKED_KEYWORD_PATTERN =
  /press enter|press t to trust|do you trust|trust this|trusted workspace|update available|choose working directory|codex just got an upgrade|hooks need review/
const WAIT_BLOCKED_KEYWORD_CARRY_CHARS = 31

type WaitBlockedCheckState = {
  lastAt: number
  lastWaitState: TerminalTailWaitState | null
  appended: string
  keywordCarry: string
  timer: ReturnType<typeof setTimeout> | null
}

export type RuntimePtyWaitBlockedCheckCommandHost = {
  getGraph(): RuntimeGraphStore
}

export class RuntimePtyWaitBlockedCheckCommands {
  private readonly waitBlockedCheckStateByPtyId = new Map<string, WaitBlockedCheckState>()

  constructor(private readonly host: RuntimePtyWaitBlockedCheckCommandHost) {}

  // Why: also called from OrcaRuntimeService outside this domain (onPtyData) — public, not private.
  scheduleWaitBlockedCheck(ptyId: string, appendedText: string, at: number): void {
    let state = this.waitBlockedCheckStateByPtyId.get(ptyId)
    if (!state) {
      state = { lastAt: 0, lastWaitState: null, appended: '', keywordCarry: '', timer: null }
      this.waitBlockedCheckStateByPtyId.set(ptyId, state)
    }
    const appendedLower = appendedText.toLowerCase()
    const keywordHit = WAIT_BLOCKED_KEYWORD_PATTERN.test(`${state.keywordCarry}${appendedLower}`)
    state.keywordCarry = appendedLower.slice(-WAIT_BLOCKED_KEYWORD_CARRY_CHARS)
    // Why the cap keeps the tail: the accumulated text only anchors boundary-
    // spanning prompt detection; anything past the tail cap has scrolled out
    // of the retained tail the check reads anyway.
    state.appended =
      state.appended.length + appendedText.length > MAX_TAIL_CHARS
        ? `${state.appended}${appendedText}`.slice(-MAX_TAIL_CHARS)
        : `${state.appended}${appendedText}`
    const elapsed = at - state.lastAt
    if (keywordHit || elapsed >= WAIT_BLOCKED_CHECK_MIN_INTERVAL_MS || elapsed < 0) {
      this.runWaitBlockedCheck(ptyId, state, at)
      return
    }
    if (!state.timer) {
      // Why trailing edge: the final chunks of a burst must still be
      // evaluated or a prompt arriving right after a flood would go
      // unstamped until the next output.
      state.timer = setTimeout(() => {
        state.timer = null
        this.runWaitBlockedCheck(ptyId, state, Date.now())
      }, WAIT_BLOCKED_CHECK_MIN_INTERVAL_MS - elapsed)
    }
  }

  private runWaitBlockedCheck(ptyId: string, state: WaitBlockedCheckState, at: number): void {
    const pty = this.host.getGraph().ptysById.get(ptyId)
    if (!pty) {
      state.appended = ''
      return
    }
    const nextWaitState = computeTerminalTailWaitState(
      pty.tailBuffer,
      pty.tailPartialLine,
      pty.preview
    )
    const previousWaitState = state.lastWaitState ?? {
      waitText: '',
      signal: null,
      fromTail: false
    }
    if (tailGainedNewerBlockedReason(previousWaitState, nextWaitState, state.appended)) {
      pty.waitBlockedAt = at
    }
    state.lastAt = at
    state.lastWaitState = nextWaitState
    state.appended = ''
  }

  // Why: also called from OrcaRuntimeService outside this domain (onPtyExit, pruneDisconnectedPtyRecords) — public, not private.
  clearWaitBlockedCheckState(ptyId: string): void {
    const state = this.waitBlockedCheckStateByPtyId.get(ptyId)
    if (state?.timer) {
      clearTimeout(state.timer)
    }
    this.waitBlockedCheckStateByPtyId.delete(ptyId)
  }
}
