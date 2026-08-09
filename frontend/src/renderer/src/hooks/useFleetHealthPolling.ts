// useFleetHealthPolling — poll fleet health metrics every 60s, detect disconnects (CR-005)
import { useEffect, useRef } from 'react'
import { useAppStore } from '@/store'
import { translate } from '@/i18n/i18n'
import type { SshConnectionStatus } from '../../../shared/ssh-types'

const POLL_INTERVAL_MS = 60_000 // 60 seconds

export function useFleetHealthPolling(enabled: boolean): void {
  const sshTargets = useAppStore((s) => s.sshTargets ?? [])
  // Why: connectionStates is a Map<string, SshConnectionState> — use .get(), not bracket access.
  const connectionStates = useAppStore((s) => s.sshConnectionStates)

  const prevStatesRef = useRef<Record<string, SshConnectionStatus>>({})

  // ── Health polling ────────────────────────────────────────────────────────────
  useEffect(() => {
    if (!enabled) {return}

    const doPoll = async (): Promise<void> => {
      try {
        const healthData = await window.api.ssh.getFleetHealth?.()
        if (!healthData) {return}

        const now = Date.now()
        const store = useAppStore.getState()
        store.setLastFleetHealthCheck(now)

        for (const entry of healthData.servers ?? []) {
          store.updateServerHealth(entry.serverId, {
            lastCheckedAt: now,
            isReachable: entry.isReachable,
            uptimeSeconds: entry.uptimeSeconds ?? null,
            relayVersion: entry.relayVersion ?? null,
            nodeVersion: entry.nodeVersion ?? null,
            diskUsagePercent: entry.diskUsagePercent ?? null
          })
        }
      } catch (err) {
        console.warn('[FleetHealthPolling] Poll failed:', err)
      }
    }

    void doPoll()
    const interval = setInterval(() => void doPoll(), POLL_INTERVAL_MS)
    return () => clearInterval(interval)
  }, [enabled])

  // ── Disconnect alert detection ────────────────────────────────────────────────
  useEffect(() => {
    const store = useAppStore.getState()

    for (const target of sshTargets) {
      const prevStatus = prevStatesRef.current[target.id]
      // Why: Map.get() — connectionStates is a Map not a plain Record
      const currStatus = connectionStates.get(target.id)?.status

      const wasConnected = prevStatus === 'connected'
      const isDisconnectedNow =
        currStatus === 'disconnected' ||
        currStatus === 'error' ||
        currStatus === 'reconnection-failed'

      if (wasConnected && isDisconnectedNow) {
        store.addFleetAlert({
          id: `disconnect-${target.id}-${Date.now()}`,
          serverId: target.id,
          serverLabel: target.label,
          type: 'disconnected',
          message: translate('fleet.alert.disconnected', `${target.label} disconnected`),
          timestamp: Date.now(),
          dismissed: false
        })
      }
    }

    // Update the prev-state snapshot for the next render
    const newSnapshot: Record<string, SshConnectionStatus> = {}
    for (const target of sshTargets) {
      const status = connectionStates.get(target.id)?.status
      if (status) {newSnapshot[target.id] = status}
    }
    prevStatesRef.current = newSnapshot
  }, [connectionStates, sshTargets])
}
