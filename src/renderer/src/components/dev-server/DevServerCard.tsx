import { Loader2, Trash2, Plug, PlugZap, ChevronDown, ChevronRight } from 'lucide-react'
import { useState } from 'react'
import type { DevServer } from '../../../../shared/dev-server-types'
import { DevServerStatusBadge } from './DevServerStatusBadge'
import { Button } from '@/components/ui/button'
import { useAppStore } from '../../store'
import { latestAgentWsStatusForDevServer } from '@/lib/agent-ws-trace-status'

// ─── Types ────────────────────────────────────────────────────────────────────

type Props = {
  server: DevServer
  /** Show connect/disconnect controls. Default: true */
  showActions?: boolean
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function formatTimestamp(ts: number | null): string {
  if (!ts) return '—'
  return new Date(ts).toLocaleString()
}

const CONNECTION_TYPE_LABEL: Record<string, string> = {
  'direct-websocket': 'Direct WebSocket (daemon)',
  'relay-websocket': 'Relay WebSocket',
  'relay-ssh': 'Relay SSH',
}

// ─── Component ────────────────────────────────────────────────────────────────

/**
 * Card displaying a single dev server with status, connection controls, and
 * expandable connection details.
 */
export function DevServerCard({ server, showActions = true }: Props) {
  const [connecting, setConnecting] = useState(false)
  const [expanded, setExpanded] = useState(false)
  const removeDevServer = useAppStore((s) => s.removeDevServer)
  // Read-only: consumes agentWs:*/agentToken:* trace events already fed by
  // the SSE bridge (initBrowserTrace) — does not subscribe or emit spans.
  const agentWsStatus = useAppStore((s) => latestAgentWsStatusForDevServer(s.traceEvents, server.id))

  const handleConnect = async () => {
    setConnecting(true)
    try {
      await window.api.devServer.connect(server.id)
    } finally {
      setConnecting(false)
    }
  }

  const handleDisconnect = async () => {
    setConnecting(true)
    try {
      await window.api.devServer.disconnect(server.id)
    } finally {
      setConnecting(false)
    }
  }

  const handleRemove = async (e: React.MouseEvent) => {
    e.stopPropagation()
    await window.api.devServer.remove(server.id)
    removeDevServer(server.id)
  }

  // direct-websocket: agent self-manages connection via systemd daemon.
  // Showing Connect/Disconnect buttons is misleading — connect() would
  // generate a new token and timeout waiting for agent to pick it up.
  const isDaemonManaged = server.connectionType === 'direct-websocket'

  return (
    <div className="dev-server-card" data-server-id={server.id}>
      {/* ── Clickable header row ── */}
      <div
        className="dev-server-card__header"
        role="button"
        tabIndex={0}
        aria-expanded={expanded}
        onClick={() => setExpanded((v) => !v)}
        onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && setExpanded((v) => !v)}
        style={{ cursor: 'pointer' }}
      >
        {/* Expand chevron */}
        <span className="dev-server-card__chevron" aria-hidden="true">
          {expanded ? (
            <ChevronDown className="size-4 text-muted-foreground" />
          ) : (
            <ChevronRight className="size-4 text-muted-foreground" />
          )}
        </span>

        <div className="dev-server-card__info">
          <span className="dev-server-card__name">{server.name}</span>
          {/* Always show status badge — shows 'connected', 'connecting', etc. */}
          <DevServerStatusBadge status={server.status} platform={server.platform} />
          {/* Read-only Agent WS trace badge — only when a matching trace event exists */}
          {agentWsStatus && (
            <span
              className={
                agentWsStatus.level === 'fail'
                  ? 'text-xs text-destructive'
                  : 'text-xs text-muted-foreground'
              }
              title={`${agentWsStatus.flow} · ${new Date(agentWsStatus.ts).toLocaleTimeString()}`}
            >
              {agentWsStatus.level === 'ok' ? 'Agent WS: handshake ok' : null}
              {agentWsStatus.level === 'fail'
                ? `Agent WS: ${agentWsStatus.reason ?? 'handshake failed'}`
                : null}
            </span>
          )}
        </div>

        {showActions && (
          <div
            className="dev-server-card__actions"
            onClick={(e) => e.stopPropagation()}
          >
            {!isDaemonManaged && (
              server.status === 'connected' ? (
                <Button
                  variant="ghost"
                  size="sm"
                  id={`disconnect-btn-${server.id}`}
                  onClick={() => void handleDisconnect()}
                  disabled={connecting}
                  title="Disconnect"
                >
                  {connecting ? <Loader2 className="size-4 animate-spin" /> : <PlugZap className="size-4" />}
                </Button>
              ) : (
                <Button
                  variant="ghost"
                  size="sm"
                  id={`connect-btn-${server.id}`}
                  onClick={() => void handleConnect()}
                  disabled={connecting || server.status === 'connecting'}
                  title="Connect"
                >
                  {connecting ? <Loader2 className="size-4 animate-spin" /> : <Plug className="size-4" />}
                </Button>
              )
            )}
            {isDaemonManaged && (
              <span
                className="dev-server-card__daemon-badge text-xs text-muted-foreground px-2"
                title="Agent self-connects via systemd daemon"
              >
                daemon
              </span>
            )}
            <Button
              variant="ghost"
              size="sm"
              id={`remove-btn-${server.id}`}
              onClick={handleRemove}
              title="Remove server"
            >
              <Trash2 className="size-4 text-destructive" />
            </Button>
          </div>
        )}
      </div>

      {/* ── Inline error (always visible) ── */}
      {server.lastError && (
        <div className="dev-server-card__error-bar">
          <span className="dev-server-card__error">{server.lastError}</span>
        </div>
      )}

      {/* ── Expanded detail panel ── */}
      {expanded && (
        <div className="dev-server-card__details">
          <table className="dev-server-card__details-table">
            <tbody>
              <tr>
                <td className="dev-server-card__detail-key">ID</td>
                <td className="dev-server-card__detail-val">{server.id}</td>
              </tr>
              <tr>
                <td className="dev-server-card__detail-key">Connection</td>
                <td className="dev-server-card__detail-val">
                  {CONNECTION_TYPE_LABEL[server.connectionType] ?? server.connectionType}
                </td>
              </tr>
              <tr>
                <td className="dev-server-card__detail-key">Status</td>
                <td className="dev-server-card__detail-val">
                  <DevServerStatusBadge status={server.status} platform={server.platform} />
                </td>
              </tr>
              {server.platform && (
                <tr>
                  <td className="dev-server-card__detail-key">Platform</td>
                  <td className="dev-server-card__detail-val">{server.platform}</td>
                </tr>
              )}
              {server.arch && (
                <tr>
                  <td className="dev-server-card__detail-key">Arch</td>
                  <td className="dev-server-card__detail-val">{server.arch}</td>
                </tr>
              )}
              {server.nodeVersion && (
                <tr>
                  <td className="dev-server-card__detail-key">Node</td>
                  <td className="dev-server-card__detail-val">{server.nodeVersion}</td>
                </tr>
              )}
              {server.wsUrl && (
                <tr>
                  <td className="dev-server-card__detail-key">URL</td>
                  <td className="dev-server-card__detail-val">{server.wsUrl}</td>
                </tr>
              )}
              {server.workspaceDir && (
                <tr>
                  <td className="dev-server-card__detail-key">Workspace</td>
                  <td className="dev-server-card__detail-val">{server.workspaceDir}</td>
                </tr>
              )}
              <tr>
                <td className="dev-server-card__detail-key">Last connected</td>
                <td className="dev-server-card__detail-val">
                  {formatTimestamp(server.lastConnectedAt)}
                </td>
              </tr>
              <tr>
                <td className="dev-server-card__detail-key">Added</td>
                <td className="dev-server-card__detail-val">
                  {formatTimestamp(server.addedAt)}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
