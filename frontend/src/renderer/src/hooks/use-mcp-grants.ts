import { useCallback } from 'react'
import type { McpGrant } from '../../../shared/mcp-types'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { useMcpEvent } from './useMcpEvent'
import { useMcpQuery, type McpQuery } from './useMcpQuery'

const EMPTY: McpGrant[] = []

/** Active grants first, then newest first. */
export function sortMcpGrants(grants: readonly McpGrant[]): McpGrant[] {
  return [...grants].sort((a, b) => {
    if ((a.status === 'active') !== (b.status === 'active')) {
      return a.status === 'active' ? -1 : 1
    }
    return Date.parse(b.createdAt) - Date.parse(a.createdAt)
  })
}

type GrantsApi = Omit<McpQuery<McpGrant[]>, 'data' | 'setData'> & {
  grants: McpGrant[]
  revoke: (grantId: string) => Promise<void>
}

function useRevoke(
  q: McpQuery<McpGrant[]>,
  method: 'mcp.grant.revoke' | 'mcp.admin.grant.revoke'
): (grantId: string) => Promise<void> {
  const { setData, reload } = q
  useMcpEvent('grant.revoked', (e) =>
    setData((gs) => gs.map((g) => (g.id === e.grantId ? { ...g, status: 'revoked' } : g)))
  )
  return useCallback(
    async (grantId: string) => {
      try {
        await mcpClient.call(method, { grantId })
        setData((gs) => gs.map((g) => (g.id === grantId ? { ...g, status: 'revoked' } : g)))
      } catch (e) {
        // Already gone: resync instead of surfacing an error.
        if (e instanceof McpRpcError && e.code === 'MCP_NOT_FOUND') {
          reload()
          return
        }
        throw e
      }
    },
    [method, setData, reload]
  )
}

export function useMcpGrants(): GrantsApi {
  const q = useMcpQuery('mcp.grant.list', undefined, EMPTY, {
    refetchOnFocus: true,
    quietReload: true
  })
  const revoke = useRevoke(q, 'mcp.grant.revoke')
  return {
    status: q.status,
    error: q.error,
    reload: q.reload,
    grants: q.data,
    revoke
  }
}

export function useMcpAllGrants(userId?: string): GrantsApi {
  const q = useMcpQuery('mcp.admin.grant.list', userId ? { userId } : {}, EMPTY, {
    quietReload: true
  })
  const revoke = useRevoke(q, 'mcp.admin.grant.revoke')
  return {
    status: q.status,
    error: q.error,
    reload: q.reload,
    grants: q.data,
    revoke
  }
}
