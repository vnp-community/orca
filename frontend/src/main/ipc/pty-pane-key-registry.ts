// PaneKey-scoped PTY registry state, extracted from ipc/pty.ts (TASK-BIGFILE-252).
// Owns the paneKey<->ptyId mappings, the pre-signal serializer handshake, and
// spawn-race reservations. `teardownForPty` bundles the pane-key portion of
// clearProviderPtyState's cleanup; the caller (pty.ts) still owns unrelated
// teardown steps (unregisterPty, agentHookServer.*) that read this registry's
// state but are not part of it.
import type { WebContents } from 'electron'
import type { SleepingAgentLaunchConfig } from '../../shared/agent-session-resume'
import type { PtySpawnResult } from '../providers/types'
import { parsePaneKey } from '../../shared/stable-pane-id'

export type PaneKeyTeardownListener = (paneKey: string) => void

export type PaneSpawnReservation = {
  promise: Promise<PaneSpawnReservationResult>
  resolve: (result: PaneSpawnReservationResult) => void
  reject: (error: unknown) => void
}

export type PaneSpawnReservationResult = {
  id: string
  launchConfig?: SleepingAgentLaunchConfig
} & Partial<PtySpawnResult>

export function parseValidPaneKey(paneKey: unknown): ReturnType<typeof parsePaneKey> {
  if (typeof paneKey !== 'string' || paneKey.length > 256) {
    return null
  }
  return parsePaneKey(paneKey)
}

export function isValidPaneKey(paneKey: unknown): paneKey is string {
  return parseValidPaneKey(paneKey) !== null
}

class PaneKeySerializerRegistry {
  // Why: the agent-hooks server caches per-paneKey state (last prompt, last
  // tool) that otherwise grows unbounded as panes come and go. Track the
  // spawn-time paneKey so clearProviderPtyState can clear that cache on PTY
  // teardown — the renderer knows the paneKey but the PTY lifecycle does not
  // without this mapping.
  private ptyPaneKey = new Map<string, string>()
  // Why: reverse of ptyPaneKey — callers that receive a paneKey from outside the
  // PTY lifecycle (e.g. the agent-hook server routing a cursor-agent status event
  // back into the pane's data stream) need to find the ptyId for that paneKey.
  // Kept in lock-step with ptyPaneKey via the same spawn and teardown sites.
  private paneKeyPtyId = new Map<string, string>()
  // Why: consumers (currently the cursor-agent synthesized-spinner loop in
  // main/index.ts) need to tear down paneKey-scoped state when a PTY exits so
  // intervals / timers cannot leak for the process lifetime. A callback
  // registry keeps the cross-module dependency narrow — clearProviderPtyState
  // only has to know about "things to notify", not about every consumer's
  // internals.
  private paneKeyTeardownListeners = new Set<PaneKeyTeardownListener>()
  // Why: pre-signal handshake — the renderer declares it will own the serializer
  // for a paneKey BEFORE issuing pty:spawn. The cooperation gate at provider.spawn
  // return consults this map to suppress the daemon-snapshot seed when a renderer
  // is taking over. Generation tokens prevent paneKey-reuse races during teardown:
  // a paneKeyTeardownListener cleanup only fires settle when the captured gen
  // still matches, so a remount that pre-signals before the old PTY's teardown
  // runs is preserved. See docs/mobile-prefer-renderer-scrollback.md.
  private pendingSerializerGenSeq = 0
  private pendingByPaneKey = new Map<string, { gen: number; ownerWebContentsId: number | null }>()
  private pendingPaneSerializerCleanupRegistered = new Set<number>()
  // Why: mobile runtime materialization and a newly-focused renderer pane can
  // race to spawn the same tab/leaf. Key by stable paneKey so the loser adopts
  // the winner's PTY instead of creating a duplicate shell.
  private paneSpawnReservationsByPaneKey = new Map<string, PaneSpawnReservation>()
  // Why: at PTY spawn time we capture the gen that was pending for the spawn's
  // paneKey, so teardown can settle ONLY that gen. Without this, a paneKey
  // remount that replaces the pending entry with a new gen would still get
  // stomped by the old PTY's teardown firing settle on the wrong gen.
  private ptyPendingGenByPtyId = new Map<string, number>()
  // Why: the runtime's hasRendererSerializer probe needs a ptyId-keyed signal.
  // Populated on settlePaneSerializer (renderer has registered for this ptyId)
  // and cleared on PTY teardown.
  private rendererSerializerByPtyId = new Set<string>()

  getPtyIdForPaneKey(paneKey: string): string | undefined {
    return this.paneKeyPtyId.get(paneKey)
  }

  registerTeardownListener(listener: PaneKeyTeardownListener): () => void {
    this.paneKeyTeardownListeners.add(listener)
    return () => this.paneKeyTeardownListeners.delete(listener)
  }

  hasPendingRendererSerializer(paneKey: string): boolean {
    return isValidPaneKey(paneKey) && this.pendingByPaneKey.has(paneKey)
  }

  rememberPaneKeyForPty(ptyId: string, paneKey: unknown): string | null {
    const normalizedPaneKey = typeof paneKey === 'string' ? paneKey.trim() : ''
    if (!isValidPaneKey(normalizedPaneKey)) {
      return null
    }
    this.ptyPaneKey.set(ptyId, normalizedPaneKey)
    this.paneKeyPtyId.set(normalizedPaneKey, ptyId)
    return normalizedPaneKey
  }

  private cleanupPendingPaneSerializersForSender(ownerWebContentsId: number): void {
    this.pendingPaneSerializerCleanupRegistered.delete(ownerWebContentsId)
    for (const [paneKey, pending] of this.pendingByPaneKey) {
      if (pending.ownerWebContentsId === ownerWebContentsId) {
        this.pendingByPaneKey.delete(paneKey)
      }
    }
  }

  private registerPendingPaneSerializerCleanup(sender: WebContents | undefined): void {
    if (!sender || this.pendingPaneSerializerCleanupRegistered.has(sender.id)) {
      return
    }
    this.pendingPaneSerializerCleanupRegistered.add(sender.id)
    sender.once('destroyed', () => this.cleanupPendingPaneSerializersForSender(sender.id))
  }

  declarePendingSerializer(paneKey: string, sender: WebContents | undefined): number {
    const gen = ++this.pendingSerializerGenSeq
    this.registerPendingPaneSerializerCleanup(sender)
    this.pendingByPaneKey.set(paneKey, { gen, ownerWebContentsId: sender?.id ?? null })
    return gen
  }

  settlePendingSerializer(paneKey: string, gen: number): void {
    if (this.pendingByPaneKey.get(paneKey)?.gen === gen) {
      this.pendingByPaneKey.delete(paneKey)
    }
  }

  hasPendingSerializerEntry(paneKey: string): boolean {
    return this.pendingByPaneKey.has(paneKey)
  }

  getPendingSerializerEntry(
    paneKey: string
  ): { gen: number; ownerWebContentsId: number | null } | undefined {
    return this.pendingByPaneKey.get(paneKey)
  }

  hasRendererSerializer(ptyId: string): boolean {
    return this.rendererSerializerByPtyId.has(ptyId)
  }

  markRendererSerializerRegistered(ptyId: string): void {
    this.rendererSerializerByPtyId.add(ptyId)
  }

  recordPendingGenForPty(ptyId: string, gen: number): void {
    this.ptyPendingGenByPtyId.set(ptyId, gen)
  }

  reservePaneSpawn(paneKey: string): PaneSpawnReservation {
    let resolve!: (result: PaneSpawnReservationResult) => void
    let reject!: (error: unknown) => void
    const promise = new Promise<PaneSpawnReservationResult>((promiseResolve, promiseReject) => {
      resolve = promiseResolve
      reject = promiseReject
    })
    promise.catch(() => {})
    const reservation = { promise, resolve, reject }
    this.paneSpawnReservationsByPaneKey.set(paneKey, reservation)
    return reservation
  }

  getPaneSpawnReservation(paneKey: string): PaneSpawnReservation | undefined {
    return this.paneSpawnReservationsByPaneKey.get(paneKey)
  }

  private clearPaneSpawnReservation(paneKey: string, reservation: PaneSpawnReservation): void {
    if (this.paneSpawnReservationsByPaneKey.get(paneKey) === reservation) {
      this.paneSpawnReservationsByPaneKey.delete(paneKey)
    }
  }

  rejectPaneSpawnReservation(
    paneKey: string | null | undefined,
    reservation: PaneSpawnReservation | null | undefined,
    error: unknown
  ): void {
    if (!reservation) {
      return
    }
    reservation.reject(error)
    if (paneKey) {
      this.clearPaneSpawnReservation(paneKey, reservation)
    }
  }

  resolvePaneSpawnReservation<T extends PaneSpawnReservationResult>(
    paneKey: string | null | undefined,
    reservation: PaneSpawnReservation | null | undefined,
    response: T
  ): T {
    if (!reservation) {
      return response
    }
    reservation.resolve(response)
    if (paneKey) {
      this.clearPaneSpawnReservation(paneKey, reservation)
    }
    return response
  }

  /** Bundles the pane-key portion of clearProviderPtyState's teardown. The
   *  caller still owns unregisterPty/agentHookServer.* — those read `paneKey`/
   *  `stillOwnsPaneKey` from this return value to decide whether to run. */
  teardownForPty(id: string): { paneKey: string | undefined; stillOwnsPaneKey: boolean } {
    const paneKey = this.ptyPaneKey.get(id)
    const stillOwnsPaneKey = paneKey ? this.paneKeyPtyId.get(paneKey) === id : false
    this.rendererSerializerByPtyId.delete(id)
    if (paneKey) {
      if (stillOwnsPaneKey) {
        this.paneKeyPtyId.delete(paneKey)
      }
      this.ptyPaneKey.delete(id)
      // Why: drop the pre-signal pending entry only if it still belongs to THIS
      // PTY's spawn generation. If a remount for the same paneKey has already
      // pre-signaled a new gen, this teardown must NOT touch it — otherwise
      // the second mount's hydration loses to the daemon-snapshot seed. See
      // the generation-token rationale in
      // docs/mobile-prefer-renderer-scrollback.md.
      const ownedGen = this.ptyPendingGenByPtyId.get(id)
      if (ownedGen !== undefined) {
        this.settlePendingSerializer(paneKey, ownedGen)
      }
      this.ptyPendingGenByPtyId.delete(id)
      if (stillOwnsPaneKey) {
        // Why: notify registered consumers AFTER we've dropped the paneKey↔ptyId
        // entries so a listener that re-reads the map sees the post-teardown
        // state. Wrap each call so one throwing listener cannot block the rest.
        for (const listener of this.paneKeyTeardownListeners) {
          try {
            listener(paneKey)
          } catch (err) {
            console.error('[pty] paneKey teardown listener threw', err)
          }
        }
      }
    }
    return { paneKey, stillOwnsPaneKey }
  }
}

export const paneKeySerializerRegistry = new PaneKeySerializerRegistry()
