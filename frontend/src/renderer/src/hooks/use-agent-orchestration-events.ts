// src/renderer/src/hooks/use-agent-orchestration-events.ts
// BUG-FE-ORCH-001-G: Subscribe to agentOrchestration IPC push events and sync to store
// Analogous to existing ipc-tab-switch.ts and runtime-client-events-sync.ts patterns

import { useEffect } from 'react'
import { useAppStore } from '../store'
import { peekOpenAgentOrchSpan, takeOpenAgentOrchSpan } from '@/lib/agent-orchestration-active-spans'

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

      // BL-AG-05: no dedicated span per statusChanged event — only step()/close
      // the BL-AG-01/03 span already open in the registry for this worktree, if any.
      if (event.status === 'running') {
        const span = takeOpenAgentOrchSpan(event.worktreeId)
        span?.ok({ worktreeId: event.worktreeId, sessionId: event.sessionId ?? '', status: event.status })
      } else if (event.status === 'error') {
        const span = takeOpenAgentOrchSpan(event.worktreeId)
        span?.fail(new Error(event.errorMessage ?? 'agent error'), { worktreeId: event.worktreeId })
      } else if (event.status === 'starting') {
        // Intermediate — ui:agentOrch.spawn/resume is still running, just log a step().
        peekOpenAgentOrchSpan(event.worktreeId)?.step('statusChanged', { status: event.status })
      }
      // 'stopped' doesn't touch the registry: stopAgent() already closed its own
      // ui:agentOrch.stop span (TASK-FE-002.2) — statusChanged 'stopped' here only syncs the store.
    })
    return unsubscribe
  }, [updateAgentStatus])
}
