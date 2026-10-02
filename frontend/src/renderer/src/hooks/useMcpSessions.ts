import { useCallback, useEffect, useRef } from 'react'
import type { McpSessionView } from '../../../shared/mcp-types'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import { McpRpcError } from '@/runtime/runtime-mcp-error'
import { installWindowVisibilityInterval } from '@/lib/window-visibility-interval'
import { useAppStore } from '@/store'
import { useMcpEvent } from './useMcpEvent'
import { useMcpQuery, type McpQueryStatus } from './useMcpQuery'

export type McpSessionsScope = 'mine' | 'all'
const REFRESH_MS = 15_000
const EMPTY: McpSessionView[] = []

export function useMcpSessions(scope: McpSessionsScope): {
  sessions: McpSessionView[]
  status: McpQueryStatus
  error: string | null
  reload: () => void
  closeSession: (sessionId: string) => Promise<void>
} {
  const method = scope === 'all' ? 'mcp.admin.session.list' : 'mcp.session.list'
  const q = useMcpQuery(method, undefined, EMPTY, { quietReload: true })
  const resync = useAppStore((s) => s.mcpResyncCounter)

  const reloadRef = useRef(q.reload)
  reloadRef.current = q.reload

  useEffect(() => {
    // Why: the interval fires once on install; useMcpQuery already did the initial load.
    let skipInitial = true
    return installWindowVisibilityInterval({
      run: () => reloadRef.current(),
      runOnVisible: () => {
        if (skipInitial) {
          skipInitial = false
          return
        }
        reloadRef.current()
      },
      intervalMs: REFRESH_MS
    })
  }, [scope])
  // Why: events may have been missed while the stream was down.
  const firstResync = useRef(resync)
  useEffect(() => {
    if (resync !== firstResync.current) {
      reloadRef.current()
    }
  }, [resync])

  const { setData } = q
  useMcpEvent('session.closed', (e) => setData((xs) => xs.filter((s) => s.id !== e.sessionId)))

  const closeSession = useCallback(async (sessionId: string): Promise<void> => {
    try {
      await mcpClient.call('mcp.session.close', { sessionId })
    } catch (e) {
      // Already gone (or not ours): treat as closed and resync.
      if (!(e instanceof McpRpcError && e.code === 'MCP_NOT_FOUND')) {
        throw e
      }
    }
    reloadRef.current()
  }, [])

  return {
    sessions: q.data,
    status: q.status,
    error: q.error,
    reload: q.reload,
    closeSession
  }
}
