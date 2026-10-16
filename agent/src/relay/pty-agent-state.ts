/**
 * pty-agent-state.ts — the PTY table and trace helper shared by the pty-agent-bridge handlers.
 * Split out of pty-agent-bridge.ts so each file stays under the max-lines budget.
 */

// ─── Trace propagation helper ───────────────────────────────────────────────
// Agent WS JSON-RPC 2.0: traceId nested at params._trace.id (CR-TRACE-000 §3.3).
// No caller today sends _trace for pty.* — this helper is forward-compatible:
// it resumes correctly the moment an Orca-side caller starts sending it.
export function resumeFrom(params: Record<string, unknown>): { id: string } | undefined {
  const t = params['_trace']
  if (t && typeof t === 'object' && typeof (t as { id?: unknown }).id === 'string') {
    return { id: (t as { id: string }).id }
  }
  return undefined
}

// ─── Types ────────────────────────────────────────────────────────────────────

export type NotifyFn = (method: string, params: Record<string, unknown>) => void

export type AgentPtyEntry = {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any -- node-pty IPty
  pty: any
  cwd: string
  cols: number
  rows: number
  shell: string
  buf: string // scrollback buffer (last SCROLLBACK_LINES lines), doubles as the replay buffer
  /** Attach-only identity metadata, mirrors pty-handler.ts's PtyIdentity check —
   *  rejects a reattach whose caller-supplied identity disagrees with the PTY's
   *  own, so a stale lease can't wire a tab to the wrong shell after this
   *  agent process restarts and its pty-id counter resets. */
  paneKey?: string
  tabId?: string
  /** Set while a grace period is running (WS disconnected, not yet reattached
   *  or expired). Cleared on reattach or on real cleanup. */
  graceTimer?: ReturnType<typeof setTimeout> | null
  /** Why this is mutable: term.onData/onExit are registered ONCE at spawn
   *  time and must keep pushing live output for the PTY's entire lifetime,
   *  but the WebSocket they push over does not — every reconnect hands
   *  handlePtyAttach a brand-new notify bound to the new connection. Without
   *  rebinding this field on each successful attach, onData/onExit keep
   *  calling the notify from the connection that existed at create time;
   *  once that socket closes, every push after a reconnect silently no-ops
   *  (the notifier's own readyState guard drops it) even though pty.create/
   *  pty.attach themselves keep reporting success — the terminal looks
   *  merely frozen, with no error surfaced anywhere.
   */
  notify: NotifyFn
}

export const AGENT_PTY_MAP = new Map<string, AgentPtyEntry>()
