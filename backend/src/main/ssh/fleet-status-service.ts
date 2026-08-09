// src/main/ssh/fleet-status-service.ts
// Standalone getFleetStatus() service — builds FleetStatusReport from existing
// SSH store + connection manager + health store. Deliberately NOT inside
// OrcaRuntimeService to avoid touching the 26k-line orca-runtime.ts.
import type { FleetServerStatus, FleetStatusReport } from '../../shared/fleet-types'
import { getSshConnectionStore, getSshConnectionManager } from '../ipc/ssh'
import { fleetHealthStore } from './fleet-health-store'

export type FleetStatusFilter = {
  project?: string
  team?: string
}

/**
 * Build a FleetStatusReport for all (or filtered) fleet targets.
 * Pulls data from:
 *  - SshConnectionStore (target metadata)
 *  - SshConnectionManager (live connection state)
 *  - FleetHealthStore (uptime history)
 */
export function getFleetStatus(filter?: FleetStatusFilter): FleetStatusReport {
  const store = getSshConnectionStore()
  const manager = getSshConnectionManager()

  let targets = store?.listTargets() ?? []

  // Apply filters
  if (filter?.project) {targets = targets.filter((t) => t.project === filter.project)}
  if (filter?.team) {targets = targets.filter((t) => t.team === filter.team)}

  const servers: FleetServerStatus[] = targets.map((target) => {
    const connState = manager?.getState(target.id)
    const healthRecord = fleetHealthStore.getLastRecord(target.id)
    const connectedSince = fleetHealthStore.getConnectedSince(target.id)
    const uptime24h = fleetHealthStore.getUptimeForTarget(target.id)
    const uptimeSeconds = connectedSince ? Math.round((Date.now() - connectedSince) / 1000) : 0

    return {
      id: target.fleetId ?? target.id,
      label: target.label,
      host: target.host,
      project: target.project,
      team: target.team,
      environment: target.environment,
      status: connState?.status ?? 'disconnected',
      error: connState?.error ?? null,
      uptimeSeconds,
      uptimePercent24h: uptime24h.uptimePercent,
      relayVersion: healthRecord?.relayVersion ?? null,
      lastSeenAt: healthRecord?.timestamp ?? null,
      reconnectAttempt: connState?.reconnectAttempt ?? 0,
      // FIX BUG-BE-HLD-010
      cpuPercent: healthRecord?.cpuPercent ?? null,
      ramPercent: healthRecord?.ramPercent ?? null,
      diskPercent: healthRecord?.diskPercent ?? null,
      pingLatencyMs: healthRecord?.pingLatencyMs ?? null,
    }
  })

  const connected = servers.filter((s) => s.status === 'connected').length
  const errorCount = servers.filter(
    (s) => s.status === 'error' || s.status === 'reconnection-failed'
  ).length

  return {
    generatedAt: Date.now(),
    servers,
    summary: {
      total: servers.length,
      connected,
      disconnected: servers.length - connected - errorCount,
      error: errorCount,
      healthScore: Math.round((connected / Math.max(servers.length, 1)) * 100),
    },
  }
}
