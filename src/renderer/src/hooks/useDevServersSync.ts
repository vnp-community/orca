import { useEffect } from 'react'
import { useAppStore } from '../store'

/**
 * Syncs dev-server list and status into Zustand store.
 *
 * - Loads the initial list from the backend on mount.
 * - Restores the persisted activeDevServerId from global settings.
 * - Subscribes to status-change push events for the lifetime of the component.
 *
 * Should be called once near the root of the component tree, e.g. inside useIpcEvents.
 */
export function useDevServersSync(): void {
  const setDevServers = useAppStore((s) => s.setDevServers)
  const updateDevServerStatus = useAppStore((s) => s.updateDevServerStatus)
  const setActiveDevServerId = useAppStore((s) => s.setActiveDevServerId)

  useEffect(() => {
    // ── Initial load ──────────────────────────────────────────────────────────
    void window.api.devServer.list().then((servers) => {
      setDevServers(servers)
    })

    // Restore active dev server from persisted settings
    void window.api.settings.getGlobalSettings?.().then((settings) => {
      const id = settings?.activeDevServerId
      if (id) setActiveDevServerId(id)
    })

    // ── Status change subscription ────────────────────────────────────────────
    const offStatus = window.api.devServer.onStatusChanged(
      (event: {
        id: string
        status: 'connected' | 'disconnected' | 'connecting' | 'error'
        platform?: NodeJS.Platform
        error?: string
      }) => {
        updateDevServerStatus(event.id, event.status, {
          platform: event.platform ?? undefined,
          lastError: event.error ?? null,
          lastConnectedAt: event.status === 'connected' ? Date.now() : undefined,
        })
      }
    )

    return () => {
      offStatus()
    }
  }, [setDevServers, updateDevServerStatus, setActiveDevServerId])
}
