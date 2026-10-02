import { useCallback } from 'react'
import type { McpToken } from '../../../../../shared/mcp-types'
import { useMcpQuery, type McpQuery } from '@/hooks/useMcpQuery'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import { McpRpcError } from '@/runtime/runtime-mcp-error'

const EMPTY: McpToken[] = []

// Why: list state is local, never in the store. addCreated takes metadata only, so a secret has
// no path into this hook by construction.
export function useMcpTokens(): Omit<McpQuery<McpToken[]>, 'data' | 'setData'> & {
  tokens: McpToken[]
  addCreated: (token: McpToken) => void
  revoke: (tokenId: string) => Promise<void>
} {
  const q = useMcpQuery('mcp.token.list', undefined, EMPTY, {
    refetchOnFocus: true,
    quietReload: true
  })
  const { setData, reload } = q
  const addCreated = useCallback(
    (token: McpToken) => setData((ts) => [token, ...ts.filter((t) => t.id !== token.id)]),
    [setData]
  )
  const revoke = useCallback(
    async (tokenId: string) => {
      try {
        await mcpClient.call('mcp.token.revoke', { tokenId })
        setData((ts) => ts.map((t) => (t.id === tokenId ? { ...t, status: 'revoked' } : t)))
      } catch (e) {
        if (e instanceof McpRpcError && e.code === 'MCP_NOT_FOUND') {
          reload()
          return
        }
        throw e
      }
    },
    [setData, reload]
  )
  return {
    status: q.status,
    error: q.error,
    reload,
    tokens: q.data,
    addCreated,
    revoke
  }
}
