import { z } from 'zod'
import { defineMethod, type RpcMethod } from '../core'
import { getSshConnectionManager } from '../../../ipc/ssh'
import { fleetHealthStore } from '../../../ssh/fleet-health-store.js'

const FleetHealthCheckAllParams = z.object({
  serverIds: z.array(z.string().min(1))
})

// Matches frontend/src/renderer/src/store/slices/ssh.ts's ServerHealthMetrics —
// use-fleet-health-polling.ts destructures exactly these fields off each result.
type ServerHealthMetrics = {
  serverId: string
  lastCheckedAt: number
  isReachable: boolean
  uptimeSeconds: number | null
  relayVersion: string | null
  nodeVersion: string | null
  diskUsagePercent: number | null
  cpuUsagePercent: number | null
  memUsagePercent: number | null
}

// Why: thin per-id wrapper over the same primitives fleet-status-service.ts's
// getFleetStatus() maps per target (fleetHealthStore + SshConnectionManager) —
// queried directly by the caller-supplied serverId instead of going through
// getFleetStatus()'s store.listTargets() + fleetId-vs-targetId report shape,
// since the frontend sends the client-side SshTarget.id (the same id
// ssh.getState/ssh.getUptimeHistory already key off), not a report fleetId.
function buildServerHealthMetrics(serverId: string): ServerHealthMetrics {
  const manager = getSshConnectionManager()
  const connState = manager?.getState(serverId)
  const healthRecord = fleetHealthStore.getLastRecord(serverId)
  const connectedSince = fleetHealthStore.getConnectedSince(serverId)
  const uptimeSeconds = connectedSince ? Math.round((Date.now() - connectedSince) / 1000) : null

  return {
    serverId,
    lastCheckedAt: Date.now(),
    isReachable: connState?.status === 'connected',
    uptimeSeconds,
    relayVersion: healthRecord?.relayVersion ?? null,
    // Why: no node-version collection wired into fleet health tracking yet
    // (HealthRecord has no nodeVersion field) — surfaced as null rather than
    // fabricated, same "don't invent data" choice as fleet-status-service.ts's
    // null fallbacks for cpuPercent/ramPercent/diskPercent.
    nodeVersion: null,
    diskUsagePercent: healthRecord?.diskPercent ?? null,
    cpuUsagePercent: healthRecord?.cpuPercent ?? null,
    memUsagePercent: healthRecord?.ramPercent ?? null
  }
}

export const FLEET_METHODS: RpcMethod[] = [
  defineMethod({
    name: 'fleet.health.checkAll',
    params: FleetHealthCheckAllParams,
    handler: async (params) => {
      return params.serverIds.map(buildServerHealthMetrics)
    }
  })
]
