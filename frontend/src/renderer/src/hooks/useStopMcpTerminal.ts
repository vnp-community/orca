import { useCallback } from 'react'
import { useAppStore } from '@/store'
import { fetchMcpTerminalOrigins, stopMcpOriginTerminal } from '@/lib/mcp-terminal-origin'
import { getActiveRuntimeTarget } from '@/runtime/runtime-rpc-client'

export const STOP_GRACE_MS = 3000

/** Stops by handle, then refreshes the origin map so the entry disappears once the PTY exits. */
export function useStopMcpTerminal(): (
  handle: string,
  opts?: { force?: boolean }
) => Promise<void> {
  return useCallback(async (handle, opts) => {
    const target = getActiveRuntimeTarget(useAppStore.getState().settings)
    await stopMcpOriginTerminal(target, handle, opts?.force === true)
    try {
      useAppStore.getState().setMcpTerminalOrigins(await fetchMcpTerminalOrigins(target))
    } catch {
      // Refresh failure is non-fatal; the next poll reconciles.
    }
  }, [])
}
