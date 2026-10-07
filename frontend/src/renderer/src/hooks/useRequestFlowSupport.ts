/**
 * useRequestFlowSupport — CR-REQ-018-03
 *
 * Checks whether the connected runtime supports the request-service.
 * Result is cached per target (keyed by JSON.stringify(target)) to avoid
 * redundant RPC calls when the hook re-mounts.
 * Written to store via setRequestFlowSupport for sidebar/nav to read.
 *
 * @module hooks/useRequestFlowSupport
 */

import { useEffect, useRef } from 'react'
import { useAppStore } from '../store'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'

// Module-level cache: targetKey → result. Avoids repeated probe on same target.
const supportCache = new Map<string, 'supported' | 'unsupported'>()

export function useRequestFlowSupport(): void {
  const settings = useAppStore((s) => s.settings)
  const setRequestFlowSupport = useAppStore((s) => s.setRequestFlowSupport)

  const targetKey = JSON.stringify(getActiveRuntimeTarget(settings))
  const lastCheckedKeyRef = useRef<string | null>(null)

  useEffect(() => {
    // Already checked this target in this session
    if (lastCheckedKeyRef.current === targetKey && supportCache.has(targetKey)) {
      const cached = supportCache.get(targetKey)!
      setRequestFlowSupport(cached)
      return
    }

    let cancelled = false

    async function check() {
      const result = await callRequestRpc<{ enabled: boolean }>(
        REQUEST_RPC_METHODS.FLOW_STATUS
      )

      if (cancelled) return

      if (!result.ok) {
        if (result.error.kind === 'unsupported') {
          supportCache.set(targetKey, 'unsupported')
          setRequestFlowSupport('unsupported')
        }
        // Network/unknown errors: leave as 'unknown' so it retries on focus
        return
      }

      const support = result.value.enabled ? 'supported' : 'unsupported'
      supportCache.set(targetKey, support)
      lastCheckedKeyRef.current = targetKey
      setRequestFlowSupport(support)
    }

    check()

    // Retry on window focus (handles network recovery)
    function onFocus() {
      if (!supportCache.has(targetKey)) {
        check()
      }
    }

    window.addEventListener('focus', onFocus)
    return () => {
      cancelled = true
      window.removeEventListener('focus', onFocus)
    }
  }, [targetKey, setRequestFlowSupport])
}

/** Exported for tests: clear the module-level cache */
export function _clearRequestFlowSupportCache(): void {
  supportCache.clear()
}
