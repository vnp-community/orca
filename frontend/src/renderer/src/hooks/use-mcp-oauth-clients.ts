import { useCallback, useState } from 'react'
import type { McpOAuthClient } from '../../../shared/mcp-types'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import { useMcpQuery, type McpQuery } from './useMcpQuery'

const EMPTY: McpOAuthClient[] = []

/** Pending first (they need a decision), then by name. */
export function sortMcpClients(clients: readonly McpOAuthClient[]): McpOAuthClient[] {
  const rank = (c: McpOAuthClient): number => (c.status === 'pending' ? 0 : 1)
  return [...clients].sort((a, b) => rank(a) - rank(b) || a.name.localeCompare(b.name))
}

export function useMcpOAuthClients(): Omit<McpQuery<McpOAuthClient[]>, 'data' | 'setData'> & {
  clients: McpOAuthClient[]
  busy: ReadonlySet<string>
  setStatus: (clientId: string, status: 'allowed' | 'blocked') => Promise<void>
} {
  const q = useMcpQuery('mcp.admin.client.list', undefined, EMPTY, {
    quietReload: true
  })
  const [busy, setBusy] = useState<ReadonlySet<string>>(new Set())
  const { setData } = q
  const setStatus = useCallback(
    async (clientId: string, status: 'allowed' | 'blocked') => {
      setBusy((s) => new Set(s).add(clientId))
      try {
        const updated = await mcpClient.call('mcp.admin.client.setStatus', {
          clientId,
          status
        })
        setData((cs) => cs.map((c) => (c.clientId === clientId ? updated : c)))
      } finally {
        setBusy((s) => {
          const next = new Set(s)
          next.delete(clientId)
          return next
        })
      }
    },
    [setData]
  )
  return {
    status: q.status,
    error: q.error,
    reload: q.reload,
    clients: q.data,
    busy,
    setStatus
  }
}
