// src/renderer/src/hooks/use-fleet-health-polling.ts
// CR-005: Fleet health polling — 30s interval with auto-alerting
// Polls backend for server health metrics and creates alerts for issues

import { useState, useEffect, useCallback, useRef } from 'react'
import { useAppStore } from '../store'
import { useShallow } from 'zustand/react/shallow'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'
import { getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import type { ServerHealthMetrics, FleetAlert } from '../store/slices/ssh'

const DEFAULT_POLL_INTERVAL_MS = 30_000
const MIN_RELAY_VERSION = '1.5.0'

interface UseFleetHealthPollingOptions {
  intervalMs?: number
  autoStart?: boolean
}

// Semver comparison: returns negative if a < b, 0 if equal, positive if a > b
function compareVersions(a: string, b: string): number {
  const aParts = a.split('.').map(Number)
  const bParts = b.split('.').map(Number)
  for (let i = 0; i < 3; i++) {
    const diff = (aParts[i] ?? 0) - (bParts[i] ?? 0)
    if (diff !== 0) return diff
  }
  return 0
}

function isRelayOutdated(version: string | null): boolean {
  if (!version) return false
  return compareVersions(version, MIN_RELAY_VERSION) < 0
}

export function useFleetHealthPolling({
  intervalMs = DEFAULT_POLL_INTERVAL_MS,
  autoStart = true,
}: UseFleetHealthPollingOptions = {}) {
  const [isPolling, setIsPolling] = useState(false)
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const {
    sshTargets,
    updateServerHealth,
    setLastFleetHealthCheck,
    addFleetAlert,
    serverHealthMetrics,
  } = useAppStore(
    useShallow(s => ({
      sshTargets:          s.sshTargets,
      updateServerHealth:  s.updateServerHealth,
      setLastFleetHealthCheck: s.setLastFleetHealthCheck,
      addFleetAlert:       s.addFleetAlert,
      serverHealthMetrics: s.serverHealthMetrics,
    }))
  )

  const checkNow = useCallback(async () => {
    if (isPolling || sshTargets.length === 0) return
    setIsPolling(true)
    try {
      const target = getActiveRuntimeTarget(useAppStore.getState().settings)
      const results = await callRuntimeRpc<ServerHealthMetrics[]>(
        target,
        'fleet.health.checkAll',
        { serverIds: sshTargets.map(t => t.id) }
      )

      for (const metrics of results) {
        updateServerHealth(metrics.serverId, metrics)

        const sshTarget = sshTargets.find(t => t.id === metrics.serverId)
        const label = sshTarget?.label ?? metrics.serverId

        // Alert: server became unreachable
        if (!metrics.isReachable) {
          const prev = serverHealthMetrics[metrics.serverId]
          // Only alert if previously reachable (avoid repeated alerts)
          if (!prev || prev.isReachable) {
            const alert: FleetAlert = {
              id: `${metrics.serverId}-disconnected-${Date.now()}`,
              serverId: metrics.serverId,
              serverLabel: label,
              type: 'disconnected',
              message: `${label} is unreachable`,
              timestamp: Date.now(),
              dismissed: false,
            }
            addFleetAlert(alert)
          }
        }

        // Alert: relay version outdated
        if (metrics.isReachable && isRelayOutdated(metrics.relayVersion)) {
          const alert: FleetAlert = {
            id: `${metrics.serverId}-relay-${Date.now()}`,
            serverId: metrics.serverId,
            serverLabel: label,
            type: 'relay-outdated',
            message: `${label}: relay v${metrics.relayVersion} is outdated (min ${MIN_RELAY_VERSION})`,
            timestamp: Date.now(),
            dismissed: false,
          }
          addFleetAlert(alert)
        }
      }

      setLastFleetHealthCheck(Date.now())
    } catch {
      // Network or RPC error — non-fatal, will retry on next interval
    } finally {
      setIsPolling(false)
    }
  }, [sshTargets, updateServerHealth, setLastFleetHealthCheck, addFleetAlert, serverHealthMetrics, isPolling])

  // Auto-polling
  useEffect(() => {
    if (!autoStart) return
    checkNow()
    intervalRef.current = setInterval(checkNow, intervalMs)
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current)
    }
  }, [autoStart, intervalMs]) // eslint-disable-line react-hooks/exhaustive-deps

  return { isPolling, checkNow }
}
