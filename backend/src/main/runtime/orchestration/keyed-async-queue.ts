/**
 * KeyedAsyncQueue — serializes async work per key (ADR-021)
 *
 * Why this exists: converting `OrchestrationDb` (synchronous, SQLite) call
 * sites to `PgOrchestrationDb` (async, Postgres) is safe by construction
 * inside already-`async` functions (add `await`, done) — but several call
 * sites are inside SYNCHRONOUS event-driven handlers
 * (`orca-runtime.ts`'s `deliverPendingMessages()`, fired on a
 * working→idle status transition) that cannot themselves become `async`
 * without cascading that up through their event source. Making the handler
 * fire-and-forget an async call (`void this.deliverPendingMessagesAsync(leaf)`)
 * is safe ONLY if two overlapping calls for the SAME resource (leaf/handle)
 * can never interleave — otherwise a second status-change event firing
 * before the first delivery's `await`s resolve could re-read
 * "undelivered unread messages," double-inject, or write to the PTY out of
 * order. `better-sqlite3`'s synchronous single-threaded execution gave this
 * ordering for free; this queue is the explicit replacement for it.
 *
 * `run(key, fn)` guarantees every call sharing a key executes strictly in
 * submission order, one at a time — same guarantee synchronous code had,
 * scoped to whatever identity actually needs it (not a single global lock,
 * which would serialize unrelated leaves/handles for no reason).
 *
 * @module main/runtime/orchestration/keyed-async-queue
 */

export class KeyedAsyncQueue {
  private tails = new Map<string, Promise<unknown>>()

  /**
   * Run `fn` after every previously-queued call for the same `key` has
   * settled (resolved OR rejected — a failed call must not permanently wedge
   * the queue for that key). Returns `fn`'s own result/rejection.
   */
  async run<T>(key: string, fn: () => Promise<T>): Promise<T> {
    const previous = this.tails.get(key) ?? Promise.resolve()
    // Why swallow the previous call's rejection here (not propagate it):
    // this chain link only exists to sequence work — a failure earlier in
    // the queue must not fail every later, unrelated call for the same key.
    // The real error still reaches ITS OWN caller via the `await run(...)`
    // below/at that call site.
    const settled = previous.then(
      () => {},
      () => {}
    )
    const current = settled.then(fn)
    // Why store `current` (which still carries fn's rejection) rather than
    // `settled`: the NEXT call in line needs to know this one is done
    // (fulfilled or rejected) — .then(fn) already resolves/rejects at the
    // right time for that purpose, no separate bookkeeping needed.
    const tail = current.then(
      () => {},
      () => {}
    )
    this.tails.set(key, tail)
    // Why delete on settle (not leave the entry forever): terminal
    // handles/leaf ids churn over a long-running process's lifetime — without
    // this, `tails` grows by one entry per distinct key ever seen, never
    // shrinking. Guarded by identity check so a NEWER call queued for this
    // key in the meantime (tails.get(key) !== tail) is never evicted early.
    void tail.then(() => {
      if (this.tails.get(key) === tail) {
        this.tails.delete(key)
      }
    })
    return current
  }
}
