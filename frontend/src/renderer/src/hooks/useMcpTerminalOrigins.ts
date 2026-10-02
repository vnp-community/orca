import { useEffect } from 'react'
import { useAppStore } from '@/store'
import { selectMcpEnabled } from '@/store/slices/mcp-slice'
import { fetchMcpTerminalOrigins } from '@/lib/mcp-terminal-origin'
import { startMcpTerminalOriginPolling } from '@/lib/mcp-terminal-origin-poller'
import { getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'

let consumers = 0
let stopShared: (() => void) | null = null

/**
 * Keeps `mcpOriginByHandle` fresh. Why a shared poller: every tab (and the settings list)
 * mounts this, but only one terminal.list loop may run.
 */
export function useMcpTerminalOrigins(): void {
  const enabled = useAppStore(selectMcpEnabled)
  const environmentId = useAppStore((s) => s.settings?.activeRuntimeEnvironmentId ?? null)

  useEffect(() => {
    if (!enabled) {
      useAppStore.getState().clearMcpTerminalOrigins?.()
      return
    }
    consumers++
    if (consumers === 1) {
      stopShared = startMcpTerminalOriginPolling({
        fetchOrigins: () =>
          fetchMcpTerminalOrigins(getActiveRuntimeTarget(useAppStore.getState().settings)),
        apply: (next) => useAppStore.getState().setMcpTerminalOrigins(next)
      })
    }
    return () => {
      consumers--
      if (consumers === 0) {
        stopShared?.()
        stopShared = null
      }
    }
  }, [enabled, environmentId])
}
