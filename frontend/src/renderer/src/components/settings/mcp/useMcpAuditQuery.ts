import { useCallback, useEffect, useRef, useState } from 'react'
import type { McpAuditEntry } from '../../../../../shared/mcp-types'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import { classifyMcpError, type McpQueryStatus } from '@/hooks/useMcpQuery'
import { auditFilterIssue, toAuditParams, type McpAuditFilterState } from './mcp-audit-filters'

const PAGE_SIZE = 50

type State = {
  entries: McpAuditEntry[]
  nextCursor?: string
  status: McpQueryStatus | 'idle'
  loadingMore: boolean
  error: string | null
  moreError: string | null
}
const INITIAL: State = {
  entries: [],
  status: 'loading',
  loadingMore: false,
  error: null,
  moreError: null
}

export function mergeAuditById(
  a: readonly McpAuditEntry[],
  b: readonly McpAuditEntry[]
): McpAuditEntry[] {
  const seen = new Set(a.map((e) => e.id))
  return [...a, ...b.filter((e) => !seen.has(e.id))]
}

/** Server order is kept (no client sort: it would break cursor semantics). Rows stay in memory only. */
export function useMcpAuditQuery(filters: McpAuditFilterState) {
  const [state, setState] = useState<State>(INITIAL)
  const seq = useRef(0)
  const filtersKey = JSON.stringify(filters)
  const nextRef = useRef<string | undefined>(undefined)

  const run = useCallback(
    async (cursor?: string): Promise<void> => {
      const id = cursor ? seq.current : ++seq.current
      if (!cursor && auditFilterIssue(filters)) {
        setState({ ...INITIAL, status: 'idle' })
        return
      }
      setState((s) =>
        cursor ? { ...s, loadingMore: true, moreError: null } : { ...INITIAL, status: 'loading' }
      )
      try {
        const page = await mcpClient.call('mcp.admin.audit.query', {
          ...toAuditParams(filters),
          ...(cursor ? { cursor } : {}),
          limit: PAGE_SIZE
        })
        if (id !== seq.current) {
          return
        }
        nextRef.current = page.nextCursor
        setState((s) => ({
          ...INITIAL,
          status: 'ready',
          entries: mergeAuditById(cursor ? s.entries : [], page.entries),
          nextCursor: page.nextCursor
        }))
      } catch (e) {
        if (id !== seq.current) {
          return
        }
        const c = classifyMcpError(e)
        setState((s) =>
          cursor
            ? { ...s, loadingMore: false, moreError: c.message }
            : { ...INITIAL, status: c.status, error: c.message }
        )
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [filtersKey]
  )

  useEffect(() => {
    void run()
    const counter = seq
    return () => {
      counter.current++
    }
  }, [run])

  return {
    ...state,
    reload: () => void run(),
    loadMore: () => void run(nextRef.current)
  }
}
