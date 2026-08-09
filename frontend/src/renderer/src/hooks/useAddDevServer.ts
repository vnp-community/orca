import { useState, useCallback } from 'react'
import { useAppStore } from '../store'
import type { DevServer, DevServerInput, ConnectionTestResult } from '../../../shared/dev-server-types'

type AddState = 'idle' | 'testing' | 'connecting' | 'error'

export type UseAddDevServerReturn = {
  state: AddState
  testResult: ConnectionTestResult | null
  testConnection: (input: DevServerInput) => Promise<ConnectionTestResult>
  addAndConnect: (input: DevServerInput) => Promise<DevServer>
  reset: () => void
}

/**
 * Manages the flow of adding a new dev server:
 *   1. testConnection() — validate host reachability without persisting
 *   2. addAndConnect() — persist + open relay + set as active if first server
 */
export function useAddDevServer(): UseAddDevServerReturn {
  const [state, setState] = useState<AddState>('idle')
  const [testResult, setTestResult] = useState<ConnectionTestResult | null>(null)
  const upsertDevServer = useAppStore((s) => s.upsertDevServer)
  const setActiveDevServerId = useAppStore((s) => s.setActiveDevServerId)

  const testConnection = useCallback(
    async (input: DevServerInput): Promise<ConnectionTestResult> => {
      setState('testing')
      setTestResult(null)
      try {
        const result = await window.api.devServer.testConnection(input)
        setTestResult(result)
        setState('idle')
        return result
      } catch (err) {
        const errResult: ConnectionTestResult = {
          ok: false,
          error: (err as Error).message,
        }
        setTestResult(errResult)
        setState('error')
        return errResult
      }
    },
    []
  )

  const addAndConnect = useCallback(
    async (input: DevServerInput): Promise<DevServer> => {
      setState('connecting')
      try {
        // 1. Persist
        const server = await window.api.devServer.add(input)
        upsertDevServer(server)

        // 2. Connect relay
        const connected = await window.api.devServer.connect(server.id)
        upsertDevServer(connected)

        // 3. Set as active if this is the first server
        const currentState = useAppStore.getState()
        if (!currentState.activeDevServerId) {
          setActiveDevServerId(server.id)
          await window.api.settings.update?.({ activeDevServerId: server.id })
        }

        setState('idle')
        return connected
      } catch (err) {
        setState('error')
        throw err
      }
    },
    [upsertDevServer, setActiveDevServerId]
  )

  const reset = useCallback(() => {
    setState('idle')
    setTestResult(null)
  }, [])

  return { state, testResult, testConnection, addAndConnect, reset }
}
