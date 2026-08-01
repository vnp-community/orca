// src/renderer/src/hooks/use-agent-orchestration-events.ts
// BUG-FE-ORCH-001-G: Subscribe to agentOrchestration IPC push events and sync to store
// Analogous to existing ipc-tab-switch.ts and runtime-client-events-sync.ts patterns

import { useEffect } from 'react'
import { useAppStore } from '../store'

/**
 * Subscribes to `agentOrchestration:statusChanged` IPC events and syncs them
 * into the `remoteAgentSessions` Zustand slice.
 *
 * Mount this hook once at the app root level (e.g., in App.tsx or a global
 * effects provider) so that status updates from main always reach the store.
 */
export function useAgentOrchestrationEvents(): void {
  const updateAgentStatus = useAppStore(s => s.updateAgentStatus)

  useEffect(() => {
    const unsubscribe = window.api.agentOrchestration.onStatusChanged(event => {
      updateAgentStatus({
        worktreeId:    event.worktreeId,
        sessionId:     event.sessionId,
        status:        event.status,
        errorMessage:  event.errorMessage,
      })
    })
    return unsubscribe
  }, [updateAgentStatus])
}
