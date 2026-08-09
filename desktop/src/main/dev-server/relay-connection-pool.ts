/**
 * RelayConnectionPool — Managed pool of DevServerRelayBridge connections.
 *
 * Provides ref-counted relay connection reuse across multiple callers.
 * When all refs are released, a 5-minute idle timer starts before disconnect.
 * Re-acquiring before the timer fires cancels cleanup (no unnecessary reconnects).
 *
 * Usage:
 *   const relay = await pool.getOrConnect(devServerId, server)
 *   try {
 *     await relay.call('some.method', params)
 *   } finally {
 *     pool.release(devServerId)
 *   }
 *
 * @module main/dev-server/relay-connection-pool
 */

import type { DevServerRelayBridge } from './dev-server-relay-bridge'
import type { PersistedDevServer } from '../../shared/dev-server-types'

/** Idle connection lifetime before automatic disconnect (ms) */
const IDLE_CLEANUP_MS = 5 * 60 * 1000

export class RelayConnectionPool {
  private readonly connections = new Map<string, DevServerRelayBridge>()
  private readonly refCounts = new Map<string, number>()
  private readonly idleTimers = new Map<string, ReturnType<typeof setTimeout>>()

  constructor(
    /** Factory function that creates and connects a new relay bridge */
    private readonly connectFn: (server: PersistedDevServer) => Promise<DevServerRelayBridge>
  ) {}

  /**
   * Get an existing alive relay for devServerId, or establish a new connection.
   * Increments the ref-count and cancels any pending idle cleanup.
   */
  async getOrConnect(devServerId: string, server: PersistedDevServer): Promise<DevServerRelayBridge> {
    // Cancel any pending idle cleanup timer
    const pendingTimer = this.idleTimers.get(devServerId)
    if (pendingTimer !== undefined) {
      clearTimeout(pendingTimer)
      this.idleTimers.delete(devServerId)
    }

    const existing = this.connections.get(devServerId)
    if (existing?.isAlive()) {
      this.refCounts.set(devServerId, (this.refCounts.get(devServerId) ?? 0) + 1)
      return existing
    }

    // Remove stale / dead connection before reconnecting
    if (existing !== undefined) {
      this.connections.delete(devServerId)
      this.refCounts.delete(devServerId)
    }

    const relay = await this.connectFn(server)
    this.connections.set(devServerId, relay)
    this.refCounts.set(devServerId, 1)
    return relay
  }

  /**
   * Decrement the ref-count for devServerId.
   * When count reaches 0, a 5-minute idle timer starts.
   * The connection stays alive until the timer fires (or a new caller arrives).
   */
  release(devServerId: string): void {
    const current = this.refCounts.get(devServerId) ?? 0
    const next = Math.max(0, current - 1)
    this.refCounts.set(devServerId, next)

    if (next === 0) {
      const timer = setTimeout(() => {
        const relay = this.connections.get(devServerId)
        if (relay) {
          relay.disconnect().catch(() => { /* ignore disconnect errors */ })
        }
        this.connections.delete(devServerId)
        this.refCounts.delete(devServerId)
        this.idleTimers.delete(devServerId)
      }, IDLE_CLEANUP_MS)
      this.idleTimers.set(devServerId, timer)
    }
  }

  /**
   * Immediately disconnect all connections and cancel all idle timers.
   * Call this during server shutdown.
   */
  async disconnectAll(): Promise<void> {
    // Cancel all pending idle timers
    for (const timer of this.idleTimers.values()) {
      clearTimeout(timer)
    }
    this.idleTimers.clear()

    // Disconnect all active relays
    const disconnectPromises: Promise<void>[] = []
    for (const relay of this.connections.values()) {
      disconnectPromises.push(relay.disconnect().catch(() => { /* ignore */ }))
    }
    await Promise.allSettled(disconnectPromises)

    this.connections.clear()
    this.refCounts.clear()
  }

  /**
   * Returns a snapshot of current pool status for monitoring / diagnostics.
   */
  getStatus(): Record<string, { refCount: number; alive: boolean }> {
    return Object.fromEntries(
      [...this.connections.entries()].map(([id, relay]) => [
        id,
        {
          refCount: this.refCounts.get(id) ?? 0,
          alive: relay.isAlive(),
        },
      ])
    )
  }
}
