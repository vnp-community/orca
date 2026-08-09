// src/main/ssh/fleet-health-store.ts
// Persists connection-state snapshots for health reporting, uptime calculation,
// and historical analysis. In-memory only — no disk I/O; survives app restarts
// only while the process is live.
import type { SshConnectionStatus, SshConnectionState } from '../../shared/ssh-types'

export type HealthRecord = {
  targetId: string
  timestamp: number
  status: SshConnectionStatus
  error?: string
  relayVersion?: string
  remotePlatform?: SshConnectionState['remotePlatform']
  pingLatencyMs?: number
}

export type UptimeWindow = {
  windowMs: number
  uptimeMs: number
  uptimePercent: number
}

// 7 days of history per target
const HEALTH_HISTORY_MAX_AGE_MS = 7 * 24 * 60 * 60 * 1000

export class FleetHealthStore {
  private history: Map<string, HealthRecord[]> = new Map()
  private connectedSince: Map<string, number> = new Map()

  /**
   * Record a new state snapshot for a target.
   * Automatically prunes records older than 7 days.
   */
  recordConnectionState(state: SshConnectionState, relayVersion?: string): void {
    const record: HealthRecord = {
      targetId: state.targetId,
      timestamp: Date.now(),
      status: state.status,
      error: state.error ?? undefined,
      relayVersion,
      remotePlatform: state.remotePlatform,
    }

    const existing = this.history.get(state.targetId) ?? []
    existing.push(record)

    // Prune old records
    const cutoff = Date.now() - HEALTH_HISTORY_MAX_AGE_MS
    this.history.set(state.targetId, existing.filter((r) => r.timestamp > cutoff))

    // Track connected-since timestamp
    if (state.status === 'connected' && !this.connectedSince.has(state.targetId)) {
      this.connectedSince.set(state.targetId, Date.now())
    } else if (state.status !== 'connected') {
      this.connectedSince.delete(state.targetId)
    }
  }

  /**
   * Calculate uptime % for a target over a rolling window.
   * @param windowMs - window size in ms. Defaults to 24h.
   */
  getUptimeForTarget(targetId: string, windowMs = 24 * 60 * 60 * 1000): UptimeWindow {
    const records = this.history.get(targetId) ?? []
    const cutoff = Date.now() - windowMs
    const windowRecords = records.filter((r) => r.timestamp > cutoff)

    if (!windowRecords.length) return { windowMs, uptimeMs: 0, uptimePercent: 0 }

    let uptimeMs = 0
    let lastConnectedAt: number | null = null

    for (const record of windowRecords) {
      if (record.status === 'connected' && !lastConnectedAt) {
        lastConnectedAt = record.timestamp
      } else if (record.status !== 'connected' && lastConnectedAt) {
        uptimeMs += record.timestamp - lastConnectedAt
        lastConnectedAt = null
      }
    }
    // Target still connected at end of window
    if (lastConnectedAt) uptimeMs += Date.now() - lastConnectedAt

    return {
      windowMs,
      uptimeMs,
      uptimePercent: Math.round((uptimeMs / windowMs) * 1000) / 10, // one decimal place
    }
  }

  /** Unix ms timestamp when the target last became connected, or null. */
  getConnectedSince(targetId: string): number | null {
    return this.connectedSince.get(targetId) ?? null
  }

  /** Most recent health record for a target, or null. */
  getLastRecord(targetId: string): HealthRecord | null {
    const records = this.history.get(targetId) ?? []
    return records.at(-1) ?? null
  }

  /** Full history, optionally filtered to records within the past `limitMs` ms. */
  getHistory(targetId: string, limitMs?: number): HealthRecord[] {
    const records = this.history.get(targetId) ?? []
    if (!limitMs) return records
    const cutoff = Date.now() - limitMs
    return records.filter((r) => r.timestamp > cutoff)
  }

  /** Remove all history for a target (e.g. when it's deleted from the store). */
  clearHistory(targetId: string): void {
    this.history.delete(targetId)
    this.connectedSince.delete(targetId)
  }
}

// Process-lifetime singleton
export const fleetHealthStore = new FleetHealthStore()
