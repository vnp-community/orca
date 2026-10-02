import { useAppStore } from '@/store'
import { selectMcpOriginForTab } from '@/store/slices/mcp-terminal-origin'
import { useMcpTerminalOrigins } from '@/hooks/useMcpTerminalOrigins'
import { McpOriginBadge } from './McpOriginBadge'

/**
 * Renders nothing unless this tab's PTY was started by an MCP client, so tabs created from
 * the UI keep their exact DOM. Also keeps the shared origin poller alive while tabs exist.
 */
export function McpTabOriginBadge({ tabId }: { tabId: string }): React.JSX.Element | null {
  useMcpTerminalOrigins()
  const origin = useAppStore((s) => selectMcpOriginForTab(s, tabId))
  return origin ? <McpOriginBadge origin={origin} compact /> : null
}
