import { useCallback, useEffect, useRef, useState } from 'react'
import type { McpApproval } from '../../../../../shared/mcp-types'
import { translate } from '@/i18n/i18n'
import { Button } from '@/components/ui/button'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { useAppStore } from '@/store'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import { classifyMcpError, type McpQueryStatus } from '@/hooks/useMcpQuery'
import { McpInlineAlert, McpListSkeleton, McpMutedNote } from './McpListStates'
import { McpApprovalRow } from './McpApprovalRow'

type Mode = 'pending' | 'history'
const PAGE = 50

type History = {
  items: McpApproval[]
  next?: string
  status: McpQueryStatus
  error: string | null
  more: boolean
}

function useApprovalHistory(enabled: boolean) {
  const [h, setH] = useState<History>({ items: [], status: 'loading', error: null, more: false })
  const seq = useRef(0)
  const load = useCallback(async (cursor?: string): Promise<void> => {
    const id = ++seq.current
    setH((s) =>
      cursor ? { ...s, more: true, error: null } : { ...s, status: 'loading', error: null }
    )
    try {
      const res = await mcpClient.call('mcp.approval.list', {
        status: 'all',
        limit: PAGE,
        ...(cursor ? { cursor } : {})
      })
      if (id !== seq.current) {
        return
      }
      setH((s) => {
        const seen = new Set((cursor ? s.items : []).map((a) => a.id))
        const fresh = res.approvals.filter((a) => a.status !== 'pending' && !seen.has(a.id))
        return {
          items: [...(cursor ? s.items : []), ...fresh],
          next: res.nextCursor,
          status: 'ready',
          error: null,
          more: false
        }
      })
    } catch (e) {
      if (id === seq.current) {
        const c = classifyMcpError(e)
        // Keep loaded rows when only "load more" failed.
        setH((s) => ({ ...s, status: cursor ? 'ready' : c.status, error: c.message, more: false }))
      }
    }
  }, [])
  useEffect(() => {
    if (enabled) {
      void load()
    }
    const current = seq
    return () => {
      current.current++
    }
  }, [enabled, load])
  return { ...h, load }
}

export function McpApprovalsTab(): React.JSX.Element {
  const pending = useAppStore((s) => s.mcpApprovalQueue)
  const focusId = useAppStore((s) => s.mcpApprovalFocusId)
  const [mode, setMode] = useState<Mode>('pending')
  const history = useApprovalHistory(true)
  const [highlight, setHighlight] = useState<string | null>(null)
  const [missing, setMissing] = useState(false)
  const rows = useRef(new Map<string, HTMLLIElement>())

  // Deep link: find the row in either list, scroll to it, then consume the id.
  // State is adjusted during render; the store write (not render-safe) is the effect below.
  const focusReady = Boolean(focusId) && history.status !== 'loading'
  if (focusReady) {
    const inPending = pending.some((a) => a.id === focusId)
    const inHistory = history.items.some((a) => a.id === focusId)
    if (inPending || inHistory) {
      const target: Mode = inPending ? 'pending' : 'history'
      if (mode !== target) {
        setMode(target)
      }
      if (missing) {
        setMissing(false)
      }
      if (highlight !== focusId) {
        setHighlight(focusId)
      }
    } else if (!missing) {
      setMissing(true)
    }
  }
  useEffect(() => {
    if (focusReady) {
      useAppStore.getState().setMcpApprovalFocusId(null)
    }
  }, [focusReady, focusId, history.items, pending])

  useEffect(() => {
    if (highlight) {
      rows.current.get(highlight)?.scrollIntoView?.({ block: 'center' })
    }
  }, [highlight, mode])

  const review = (id: string): void => useAppStore.getState().focusMcpApproval(id)
  const setRef =
    (id: string) =>
    (el: HTMLLIElement | null): void => {
      if (el) {
        rows.current.set(id, el)
      } else {
        rows.current.delete(id)
      }
    }

  return (
    <div className="space-y-3">
      <ToggleGroup
        type="single"
        variant="outline"
        size="sm"
        value={mode}
        aria-label={translate('auto.mcp.approval.mode', 'Approval list')}
        onValueChange={(v) => v && setMode(v as Mode)}
      >
        <ToggleGroupItem value="pending">
          {translate('auto.mcp.approval.pending', 'Pending')} ({pending.length})
        </ToggleGroupItem>
        <ToggleGroupItem value="history">
          {translate('auto.mcp.approval.history', 'History')}
        </ToggleGroupItem>
      </ToggleGroup>
      {missing ? (
        <McpMutedNote>
          {translate('auto.mcp.approval.unavailable', 'This request is no longer available')}
        </McpMutedNote>
      ) : null}
      {mode === 'pending' ? (
        pending.length === 0 ? (
          <McpMutedNote>
            {translate('auto.mcp.approval.emptyPending', 'Nothing is waiting for your approval.')}
          </McpMutedNote>
        ) : (
          <ul className="space-y-2">
            {pending.map((a) => (
              <McpApprovalRow
                key={a.id}
                ref={setRef(a.id)}
                approval={a}
                highlighted={highlight === a.id}
                onReview={review}
              />
            ))}
          </ul>
        )
      ) : history.status === 'loading' ? (
        <McpListSkeleton />
      ) : history.status === 'forbidden' ||
        history.status === 'unavailable' ||
        (history.status === 'error' && history.items.length === 0) ? (
        <McpInlineAlert message={history.error ?? ''} onRetry={() => void history.load()} />
      ) : history.items.length === 0 ? (
        <McpMutedNote>
          {translate(
            'auto.mcp.approval.empty',
            'No approval requests yet. When an AI agent needs your OK, it will appear here.'
          )}
        </McpMutedNote>
      ) : (
        <div className="space-y-2">
          <ul className="space-y-2">
            {history.items.map((a) => (
              <McpApprovalRow
                key={a.id}
                ref={setRef(a.id)}
                approval={a}
                highlighted={highlight === a.id}
              />
            ))}
          </ul>
          {history.error ? (
            <McpInlineAlert
              message={history.error}
              onRetry={() => void history.load(history.next)}
            />
          ) : null}
          {history.next ? (
            <Button
              variant="outline"
              size="sm"
              disabled={history.more}
              aria-busy={history.more}
              onClick={() => void history.load(history.next)}
            >
              {translate('auto.mcp.approval.loadMore', 'Load more')}
            </Button>
          ) : null}
        </div>
      )}
    </div>
  )
}
