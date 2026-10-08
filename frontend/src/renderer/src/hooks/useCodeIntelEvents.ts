/**
 * useCodeIntelEvents — FE-CV-TASK-050-11
 *
 * App-level hook (mounted once in App.tsx): opens the single code-intel push stream only when
 * the feature is enabled (never when off/unknown), and releases it on disable/unmount.
 *
 * @module hooks/useCodeIntelEvents
 */

import { useEffect } from 'react'
import { useAppStore } from '../store'
import { getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'
import { retainCodeIntelStream } from '../store/slices/code-intel-stream-reconnect'
import type { CodeIntelStreamDeps } from '../store/slices/code-intel-stream-reconnect'

export function createStoreStreamDeps(): CodeIntelStreamDeps {
  return {
    subscribe: (environmentId, callbacks) => window.api.codeIntel.subscribeEvents(environmentId, callbacks),
    invalidateWorktree: (worktreeId) => useAppStore.getState().invalidateCodeIntelWorktree(worktreeId),
    triggerResync: () => useAppStore.getState().triggerCodeIntelResync(),
    setEventsState: (state) => useAppStore.getState().setCodeIntelEventsState(state)
  }
}

export function useCodeIntelEvents(): void {
  const enabled = useAppStore(
    (s) => s.codeIntelSupportState.state === 'enabled' && s.codeIntelSupportState.effective?.codeIntelEnabled === true
  )
  // Select the primitive: a derived target object would change identity on every store update.
  const activeEnvironmentId = useAppStore((s) => s.settings?.activeRuntimeEnvironmentId)
  const target = getActiveRuntimeTarget({ activeRuntimeEnvironmentId: activeEnvironmentId ?? null })
  const environmentId = target.kind === 'environment' ? target.environmentId : null

  useEffect(() => {
    if (!enabled) {return}
    return retainCodeIntelStream(environmentId, createStoreStreamDeps())
  }, [enabled, environmentId])
}
