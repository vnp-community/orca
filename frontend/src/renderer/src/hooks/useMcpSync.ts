import { useEffect, useState } from 'react'
import { useAppStore } from '@/store'
import { selectMcpEnabled, selectMcpSectionVisible } from '@/store/slices/mcp-slice'
import { mcpClient } from '@/runtime/runtime-mcp-client'
import {
  parseMcpDeepLink,
  parseMcpDeepLinkParams,
  type McpDeepLinkTarget
} from '@/lib/mcp-deep-link'
import { registerPushDeepLinkHandler } from '../web/web-push-deep-link'

const REFRESH_INTERVAL_MS = 5 * 60_000

/** Loads MCP availability, holds the single event stream, and routes MCP deep links. */
export function useMcpSync(): void {
  const authed = useAppStore((s) => s.currentUser !== null)
  const enabled = useAppStore(selectMcpEnabled)
  const visible = useAppStore(selectMcpSectionVisible)
  const infoStatus = useAppStore((s) => s.mcpServerInfoStatus)
  const uiReady = useAppStore((s) => s.persistedUIReady)
  // Why: seeding the cold-start link at init lets the first consume effect see it without a re-render tick.
  const [pendingLink] = useState<{ current: McpDeepLinkTarget | null }>(() => ({
    current: parseMcpDeepLink(window.location)
  }))
  const [linkTick, setLinkTick] = useState(0)

  useEffect(() => {
    if (!authed || !mcpClient.isBridgeAvailable()) {
      useAppStore.getState().resetMcp()
      return
    }
    void useAppStore.getState().refreshMcpServerInfo()
    const onVisible = (): void => {
      if (document.visibilityState === 'visible') {
        void useAppStore.getState().refreshMcpServerInfo()
      }
    }
    document.addEventListener('visibilitychange', onVisible)
    const timer = window.setInterval(onVisible, REFRESH_INTERVAL_MS)
    return () => {
      document.removeEventListener('visibilitychange', onVisible)
      window.clearInterval(timer)
    }
  }, [authed])

  useEffect(() => {
    if (!authed || !enabled) {
      return
    }
    return useAppStore.getState().startMcpEvents()
  }, [authed, enabled])

  // Deep links (cold start, push click, orca:navigate) are parked until we know
  // whether MCP is visible, so a disabled tenant never learns the UI exists.
  useEffect(() => {
    return registerPushDeepLinkHandler('mcp', (link) => {
      const target = parseMcpDeepLinkParams(link.params)
      if (target) {
        pendingLink.current = target
        setLinkTick((n) => n + 1)
      }
    })
  }, [pendingLink])

  useEffect(() => {
    const target = pendingLink.current
    // Why: startup UI hydration restores the persisted view and would undo a cold-start link.
    if (!target || !authed || !uiReady) {
      return
    }
    if (visible) {
      pendingLink.current = null
      useAppStore.getState().openMcpTab(target.tab, target.focusId)
    } else if (infoStatus === 'ready') {
      pendingLink.current = null
    } else {
      return
    }
    if (parseMcpDeepLink(window.location)) {
      window.history.replaceState(
        null,
        '',
        window.location.pathname === '/settings' ? '/' : window.location.pathname
      )
    }
  }, [authed, visible, infoStatus, linkTick, uiReady, pendingLink])
}
