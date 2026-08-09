// Why: web-only connection state — Electron mode is always implicitly connected
// so this context lives entirely inside the web entry tree.
import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState
} from 'react'
import type { IRpcClient } from '../../../platform/rpc-client-interface'

export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'error'

type ConnectionContextValue = {
  status: ConnectionStatus
  client: IRpcClient | null
  retry: () => void
}

const ConnectionContext = createContext<ConnectionContextValue>({
  status: 'connecting',
  client: null,
  retry: () => {}
})

export type ConnectionStatusProviderProps = {
  children: React.ReactNode
  client: IRpcClient
  pollIntervalMs?: number
}

export function ConnectionStatusProvider({
  children,
  client,
  pollIntervalMs = 2000
}: ConnectionStatusProviderProps): React.JSX.Element {
  const [status, setStatus] = useState<ConnectionStatus>(() =>
    client.isConnected() ? 'connected' : 'connecting'
  )
  const prevStatusRef = useRef(status)

  const updateStatus = useCallback(() => {
    setStatus(client.isConnected() ? 'connected' : 'disconnected')
  }, [client])

  const retry = useCallback(async () => {
    setStatus('connecting')
    try {
      await client.connect()
      setStatus('connected')
    } catch {
      setStatus('disconnected')
    }
  }, [client])

  // Why: show reconnect toast only on status recovery, not on initial connect
  useEffect(() => {
    if (prevStatusRef.current === 'disconnected' && status === 'connected') {
      // Dynamic import keeps sonner out of the initial bundle chunk
      void import('sonner').then(({ toast }) => {
        toast.success('Reconnected to Orca backend', { duration: 3000 })
      })
    }
    prevStatusRef.current = status
  }, [status])

  useEffect(() => {
    const timer = setInterval(updateStatus, pollIntervalMs)
    updateStatus()
    return () => clearInterval(timer)
  }, [updateStatus, pollIntervalMs])

  return (
    <ConnectionContext.Provider value={{ status, client, retry }}>
      {children}
    </ConnectionContext.Provider>
  )
}

export function useConnectionStatus(): ConnectionStatus {
  return useContext(ConnectionContext).status
}

export function useConnectionClient(): IRpcClient | null {
  return useContext(ConnectionContext).client
}

export function useConnectionRetry(): () => void {
  return useContext(ConnectionContext).retry
}
