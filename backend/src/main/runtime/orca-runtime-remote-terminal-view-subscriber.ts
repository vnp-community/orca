// frontend/src/main/runtime/orca-runtime-remote-terminal-view-subscriber.ts
// Why (BUG-FE-BIGFILE-002 / TASK-BIGFILE-062): remote terminal-view
// subscriber ref-counting commands extracted from OrcaRuntimeService via
// the composition pattern. Method-body dependency analysis (TASK-BIGFILE-054,
// corrected per its "Bài học phương pháp" note) confirmed 2 real host
// dependencies: hasMobileSubscriber and the
// onRemoteTerminalViewPresenceChanged callback hook, which stays a
// directly-assignable public field on OrcaRuntimeService (external code in
// ipc/pty.ts sets it via `runtime.onRemoteTerminalViewPresenceChanged = ...`)
// — read here through a host getter instead of moving the field itself.
export type RuntimeRemoteTerminalViewSubscriberCommandHost = {
  hasMobileSubscriber(ptyId: string): boolean
  getOnRemoteTerminalViewPresenceChanged(): ((ptyId: string) => void) | null
}

export class RuntimeRemoteTerminalViewSubscriberCommands {
  // Why: Phase-5 query-responder suppression — a terminal-RPC subscribe
  // stream feeds a remote xterm view (mobile/web/remote desktop) that answers
  // queries with view authority, so main must yield while one is attached
  // (terminal-query-authority.md). Ref-counted per PTY because multiple
  // streams can attach concurrently; mobileSubscribers is consulted too so
  // grace-window mobile records keep suppressing.
  private readonly remoteTerminalViewSubscriberCounts = new Map<string, number>()

  constructor(private readonly host: RuntimeRemoteTerminalViewSubscriberCommandHost) {}

  // Why: also called from OrcaRuntimeService outside this domain (mobile-floor host wiring, TASK-BIGFILE-037) — public, not private.
  notifyRemoteTerminalViewPresenceChanged(ptyId: string): void {
    try {
      this.host.getOnRemoteTerminalViewPresenceChanged()?.(ptyId)
    } catch (err) {
      console.error('[runtime] remote view presence listener threw', { ptyId, err })
    }
  }

  /** Registered by terminal-RPC subscribe/multiplex streams: while a remote
   *  view subscriber is attached its xterm answers queries with view
   *  authority and the model responder must stay silent. Returns an
   *  idempotent release. */
  registerRemoteTerminalViewSubscriber(ptyId: string): () => void {
    this.remoteTerminalViewSubscriberCounts.set(
      ptyId,
      (this.remoteTerminalViewSubscriberCounts.get(ptyId) ?? 0) + 1
    )
    this.notifyRemoteTerminalViewPresenceChanged(ptyId)
    let released = false
    return () => {
      if (released) {
        return
      }
      released = true
      const next = (this.remoteTerminalViewSubscriberCounts.get(ptyId) ?? 1) - 1
      if (next <= 0) {
        this.remoteTerminalViewSubscriberCounts.delete(ptyId)
      } else {
        this.remoteTerminalViewSubscriberCounts.set(ptyId, next)
      }
      this.notifyRemoteTerminalViewPresenceChanged(ptyId)
    }
  }

  hasRemoteTerminalViewSubscriber(ptyId: string): boolean {
    if ((this.remoteTerminalViewSubscriberCounts.get(ptyId) ?? 0) > 0) {
      return true
    }
    return this.host.hasMobileSubscriber(ptyId)
  }

  // Why: also called from OrcaRuntimeService outside this domain (onPtyExit teardown) — the sub-command instance is reachable directly, no forwarding field needed.
  clearRemoteTerminalViewSubscriberCountForPty(ptyId: string): void {
    this.remoteTerminalViewSubscriberCounts.delete(ptyId)
  }
}
