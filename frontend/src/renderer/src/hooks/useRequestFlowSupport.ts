/**
 * useRequestFlowSupport — CR-REQ-018-03
 *
 * Checks whether the connected runtime supports the request-service.
 * Result is cached per target (keyed by JSON.stringify(target)) to avoid
 * redundant RPC calls when the hook re-mounts.
 * Written to store via setRequestFlowSupport for sidebar/nav to read, together with the `features` list of
 * `request.flowStatus` (CR-REQ-040): a gateway that sends none has none, so feature UI stays hidden.
 *
 * @module hooks/useRequestFlowSupport
 */

import { useEffect, useRef } from 'react'
import { useAppStore } from '../store'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { getActiveRuntimeTarget } from '../runtime/runtime-rpc-client'

type ProbeResult = { support: 'supported' | 'unsupported'; features: readonly string[] }

// Module-level cache: targetKey → result. Avoids repeated probe on same target.
const supportCache = new Map<string, ProbeResult>()

// Target the store's feature flags were last written for: flags of one gateway must never leak onto another.
let featuresTargetKey: string | null = null

// `features` is absent on a gateway older than CR-REQ-039; anything but a list of strings counts as no features.
function parseFeatures(raw: unknown): string[] {
  return Array.isArray(raw) ? raw.filter((item): item is string => typeof item === 'string') : []
}

export function useRequestFlowSupport(): void {
  const settings = useAppStore((s) => s.settings)
  const setRequestFlowSupport = useAppStore((s) => s.setRequestFlowSupport)
  const setRequestFlowFeatures = useAppStore((s) => s.setRequestFlowFeatures)

  const targetKey = JSON.stringify(getActiveRuntimeTarget(settings))
  const lastCheckedKeyRef = useRef<string | null>(null)

  useEffect(() => {
    function publish(key: string, result: ProbeResult): void {
      featuresTargetKey = key
      setRequestFlowSupport(result.support)
      setRequestFlowFeatures(result.features)
    }

    // Why: switching gateway drops the previous gateway's flags until the new probe answers, so a picker is never shown for
    // a gateway that would silently ignore the branch.
    if (featuresTargetKey !== targetKey) {
      featuresTargetKey = targetKey
      setRequestFlowFeatures([])
    }

    // Already checked this target in this session
    if (lastCheckedKeyRef.current === targetKey && supportCache.has(targetKey)) {
      publish(targetKey, supportCache.get(targetKey)!)
      return
    }

    let cancelled = false

    async function check() {
      const result = await callRequestRpc<{ enabled: boolean; features?: unknown }>(
        REQUEST_RPC_METHODS.FLOW_STATUS
      )

      if (cancelled) {
        return
      }

      if (!result.ok) {
        if (result.error.kind === 'unsupported') {
          const unsupported: ProbeResult = { support: 'unsupported', features: [] }
          supportCache.set(targetKey, unsupported)
          publish(targetKey, unsupported)
        }
        // Network/unknown errors: leave as 'unknown' so it retries on focus
        return
      }

      const probed: ProbeResult = result.value.enabled
        ? { support: 'supported', features: parseFeatures(result.value.features) }
        : { support: 'unsupported', features: [] }
      supportCache.set(targetKey, probed)
      lastCheckedKeyRef.current = targetKey
      publish(targetKey, probed)
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
  }, [targetKey, setRequestFlowSupport, setRequestFlowFeatures])
}

/** Exported for tests: clear the module-level cache */
export function _clearRequestFlowSupportCache(): void {
  supportCache.clear()
  featuresTargetKey = null
}
