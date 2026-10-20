/**
 * useTestSpecs — the test-spec lane of a Request (docs/integrate/TDD-REQ-TESTSPEC-001), from `request.testSpecs`.
 *
 * Refreshes on every event of the request and, while a run is active, on a timer that stops when the page is hidden.
 * A gateway without the channel is `unsupported`, not an error. `restart` asks for a Re-run.
 *
 * @module hooks/useTestSpecs
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parseTestSpecs } from '../../../shared/request-testspec-parsers'
import type { TestSpecs } from '../../../shared/request-testspec-types'
import { useRequestSubscription } from './useRequestSubscription'

export const TEST_SPECS_POLL_MS = 5000

export type UseTestSpecsResult = {
  testSpecs: TestSpecs | null
  isLoading: boolean
  error: string | null
  unsupported: boolean
  restarting: boolean
  restartError: string | null
  refetch: () => void
  restart: () => void
}

function pageVisible(): boolean {
  return typeof document === 'undefined' || document.visibilityState === 'visible'
}

export function useTestSpecs(requestId: string | null, enabled = true): UseTestSpecsResult {
  const [testSpecs, setTestSpecs] = useState<TestSpecs | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [unsupported, setUnsupported] = useState(false)
  const [restarting, setRestarting] = useState(false)
  const [restartError, setRestartError] = useState<string | null>(null)
  const generation = useRef(0)

  const load = useCallback(() => {
    if (!requestId || !enabled) {
      return
    }
    const mine = ++generation.current
    setIsLoading(true)
    void callRequestRpc<unknown>(REQUEST_RPC_METHODS.TEST_SPECS, {
      id: requestId
    }).then((res) => {
      if (mine !== generation.current) {
        return
      }
      setIsLoading(false)
      if (!res.ok) {
        if (res.error.kind === 'unsupported') {
          setUnsupported(true)
          setError(null)
        } else {
          setError(res.error.kind)
        }
        return
      }
      setUnsupported(false)
      setError(null)
      setTestSpecs(parseTestSpecs(res.value))
    })
  }, [requestId, enabled])

  useEffect(() => {
    setTestSpecs(null)
    setError(null)
    setUnsupported(false)
    setRestartError(null)
    load()
    return () => {
      generation.current++
    }
  }, [load])

  useRequestSubscription({ requestId, onEvent: () => load() })

  const active = testSpecs?.current
    ? ['pending', 'starting', 'running'].includes(testSpecs.current.status)
    : false
  useEffect(() => {
    if (!requestId || !enabled || !active || unsupported) {
      return undefined
    }
    const timer = setInterval(() => {
      if (pageVisible()) {
        load()
      }
    }, TEST_SPECS_POLL_MS)
    return () => clearInterval(timer)
  }, [requestId, enabled, active, unsupported, load])

  const restart = useCallback(() => {
    if (!requestId) {
      return
    }
    setRestarting(true)
    setRestartError(null)
    void callRequestRpc<unknown>(REQUEST_RPC_METHODS.TEST_SPECS_START, {
      id: requestId
    }).then((res) => {
      setRestarting(false)
      if (!res.ok) {
        setRestartError(res.error.kind)
        return
      }
      load()
    })
  }, [requestId, load])

  return {
    testSpecs,
    isLoading,
    error,
    unsupported,
    restarting,
    restartError,
    refetch: load,
    restart
  }
}
