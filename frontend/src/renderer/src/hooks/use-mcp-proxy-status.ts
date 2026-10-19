// Hook query MCP Proxy connectivity qua orca-agent RPC (CR-ORCA-005)
// Frontend KHÔNG POST trực tiếp — luôn qua callRuntimeRpc

import { useCallback, useState } from 'react'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'
import type { MCPProxyStatusInfo } from '../types/mcp-proxy-types'

interface MCPProxyCheckResult {
  configured: boolean
  url?: string
  reachable?: boolean
  authOk?: boolean
  error?: string
}

export function useMCPProxyStatus(environmentId: string) {
  const [statusInfo, setStatusInfo] = useState<MCPProxyStatusInfo>({
    status: 'unconfigured',
  })

  const checkStatus = useCallback(async () => {
    setStatusInfo((s) => ({ ...s, status: 'checking' }))
    const start = Date.now()

    try {
      const result = await callRuntimeRpc<MCPProxyCheckResult>(
        { kind: 'environment', environmentId },
        'agent.check_mcp_proxy',
        {}
      )

      const latencyMs = Date.now() - start

      if (!result.configured) {
        setStatusInfo({ status: 'unconfigured', latencyMs })
        return
      }
      if (!result.reachable) {
        setStatusInfo({
          status: 'unreachable',
          url: result.url,
          error: result.error,
          latencyMs,
          lastChecked: Date.now(),
        })
        return
      }
      if (!result.authOk) {
        setStatusInfo({
          status: 'auth-error',
          url: result.url,
          error: 'Peer token invalid or missing grant',
          latencyMs,
          lastChecked: Date.now(),
        })
        return
      }
      setStatusInfo({
        status: 'connected',
        url: result.url,
        latencyMs,
        lastChecked: Date.now(),
      })
    } catch (e) {
      setStatusInfo({
        status: 'unreachable',
        error: String(e),
        lastChecked: Date.now(),
      })
    }
  }, [environmentId])

  return { statusInfo, checkStatus }
}
