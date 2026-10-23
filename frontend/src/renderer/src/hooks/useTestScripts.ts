/**
 * useTestScripts — the test-script lane of a Request (docs/integrate/TDD-REQ-TESTSCRIPT-001), from `request.testScripts`.
 *
 * Refreshes on every event of the request and, while a run is active, on a timer that stops when the page is hidden.
 * A gateway without the channel is `unsupported`, not an error. `restart` asks for a Re-run.
 *
 * @module hooks/useTestScripts
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import { callRequestRpc } from '../runtime/request-rpc-client'
import { REQUEST_RPC_METHODS } from '../../../shared/request-rpc-methods'
import { parseTestScripts } from '../../../shared/request-testscript-parsers'
import type { TestScripts } from '../../../shared/request-testscript-types'
import { useRequestSubscription } from './useRequestSubscription'

export const TEST_SCRIPTS_POLL_MS = 5000

export type UseTestScriptsResult = {
  testScripts: TestScripts | null
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

export function useTestScripts(requestId: string | null, enabled = true): UseTestScriptsResult {
  const [testScripts, setTestScripts] = useState<TestScripts | null>(null)
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
    void callRequestRpc<unknown>(REQUEST_RPC_METHODS.TEST_SCRIPTS, {
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
      setTestScripts(parseTestScripts(res.value))
    })
  }, [requestId, enabled])

  useEffect(() => {
    setTestScripts(null)
    setError(null)
    setUnsupported(false)
    setRestartError(null)
    load()
    return () => {
      generation.current++
    }
  }, [load])

  useRequestSubscription({ requestId, onEvent: () => load() })

  const active = testScripts?.current
    ? ['pending', 'running'].includes(testScripts.current.status)
    : false
  useEffect(() => {
    if (!requestId || !enabled || !active || unsupported) {
      return undefined
    }
    const timer = setInterval(() => {
      if (pageVisible()) {
        load()
      }
    }, TEST_SCRIPTS_POLL_MS)
    return () => clearInterval(timer)
  }, [requestId, enabled, active, unsupported, load])

  const restart = useCallback(() => {
    if (!requestId) {
      return
    }
    setRestarting(true)
    setRestartError(null)
    void callRequestRpc<unknown>(REQUEST_RPC_METHODS.TEST_SCRIPTS_START, {
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
    testScripts,
    isLoading,
    error,
    unsupported,
    restarting,
    restartError,
    refetch: load,
    restart
  }
}
