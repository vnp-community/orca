import { useEffect } from 'react'
import { useAppStore } from '@/store'
import { selectMcpEnabled } from '@/store/slices/mcp-slice'
import { useMcpEvent } from '@/hooks/useMcpEvent'
import { syncMcpPendingApprovals } from '@/lib/mcp-approval-sync'
import { McpApprovalPrompt } from './McpApprovalPrompt'

const RESYNC_MS = 30_000

/** Single global mount point: feeds the approval queue from the MCP event bus and the server. */
export default function McpGlobalLayer(): React.JSX.Element | null {
  const enabled = useAppStore(selectMcpEnabled)
  const authed = useAppStore((s) => s.currentUser !== null)
  const resync = useAppStore((s) => s.mcpResyncCounter)
  const active = enabled && authed

  useEffect(() => {
    if (!active) {
      useAppStore.getState().clearMcpApprovals()
      return
    }
    const run = (): void => void syncMcpPendingApprovals().catch(() => {})
    run()
    const onVisible = (): void => {
      if (document.visibilityState === 'visible') {
        run()
      }
    }
    document.addEventListener('visibilitychange', onVisible)
    const timer = window.setInterval(onVisible, RESYNC_MS)
    return () => {
      document.removeEventListener('visibilitychange', onVisible)
      window.clearInterval(timer)
    }
    // resync: re-pull after the event stream reconnects (events may have been missed).
  }, [active, resync])

  useMcpEvent('approval.requested', (e) => {
    if (active) {
      useAppStore.getState().enqueueMcpApproval(e.approval, Date.now())
    }
  })
  useMcpEvent('approval.resolved', (e) => useAppStore.getState().resolveMcpApproval(e.id))

  // Deep link: McpPane clears mcpNavigation immediately, so capture the focus id here.
  useEffect(
    () =>
      useAppStore.subscribe((s, prev) => {
        const nav = s.mcpNavigation
        if (nav && nav !== prev.mcpNavigation && nav.tab === 'approvals' && nav.focusId) {
          s.setMcpApprovalFocusId(nav.focusId)
          s.focusMcpApproval(nav.focusId)
        }
      }),
    []
  )

  return active ? <McpApprovalPrompt /> : null
}
