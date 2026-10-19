// Visual indicator cho MCP Proxy connectivity (CR-ORCA-005)
// Endpoint hint: /internal/v1/agent-peer/mcp/proxy (SOL-ORCA-005 §2.1)

import type { FC } from 'react'
import type { MCPProxyStatusInfo } from '../../types/mcp-proxy-types'

interface Props {
  environmentId: string
  statusInfo: MCPProxyStatusInfo
  onRecheck: () => void
}

const STATUS_CONFIG = {
  connected: { label: 'MCP Proxy Connected', icon: '🟢', className: 'status--connected' },
  unreachable: { label: 'MCP Proxy Unreachable', icon: '🔴', className: 'status--error' },
  'auth-error': { label: 'MCP Proxy Auth Error', icon: '🟡', className: 'status--warning' },
  unconfigured: { label: 'MCP Proxy Not Configured', icon: '⚪', className: 'status--inactive' },
  checking: { label: 'Checking connectivity…', icon: '🔄', className: 'status--checking' },
} as const

export const MCPProxyStatusIndicator: FC<Props> = ({ environmentId, statusInfo, onRecheck }) => {
  const cfg = STATUS_CONFIG[statusInfo.status]

  return (
    <div
      className={`mcp-proxy-status ${cfg.className}`}
      id={`mcp-proxy-status-${environmentId}`}
      aria-label={cfg.label}
    >
      <span className="mcp-proxy-status__icon" aria-hidden="true">
        {cfg.icon}
      </span>
      <span className="mcp-proxy-status__label">{cfg.label}</span>

      {statusInfo.url != null && (
        <code className="mcp-proxy-status__url">{statusInfo.url}</code>
      )}

      {/* latencyMs visible khi connected — backend timeout 30s (SOL-ORCA-005 §2.2) */}
      {statusInfo.latencyMs !== undefined && statusInfo.status === 'connected' && (
        <span className="mcp-proxy-status__latency">{statusInfo.latencyMs}ms</span>
      )}

      {statusInfo.error != null && (
        <div className="mcp-proxy-status__error" role="alert">
          {statusInfo.status === 'auth-error' ? (
            <span>
              Auth failed. Check <code>ORCA_MCP_PROXY_PEER_TOKEN</code>.
            </span>
          ) : (
            <span>{statusInfo.error}</span>
          )}
        </div>
      )}

      {statusInfo.status === 'unconfigured' && (
        <div className="mcp-proxy-status__config-hint">
          Set <code>ORCA_MCP_PROXY_URL=http://mcp-proxy:3012</code> in{' '}
          <code>deploy/orca-agent/.env</code>.
        </div>
      )}

      {/* Peer-invoke endpoint path từ SOL-ORCA-005 §2.1 */}
      {statusInfo.url != null && (
        <div className="mcp-proxy-status__endpoint">
          Peer-invoke: <code>{statusInfo.url}/internal/v1/agent-peer/mcp/proxy</code>
        </div>
      )}

      {statusInfo.status !== 'unconfigured' && (
        <button
          className="mcp-proxy-status__recheck-btn"
          onClick={onRecheck}
          id={`mcp-proxy-recheck-${environmentId}`}
          disabled={statusInfo.status === 'checking'}
          aria-label="Re-check MCP Proxy connectivity"
        >
          {statusInfo.status === 'checking' ? '…' : '↺'}
        </button>
      )}

      {statusInfo.lastChecked != null && (
        <time
          className="mcp-proxy-status__last-checked"
          dateTime={new Date(statusInfo.lastChecked).toISOString()}
        >
          Last checked: {new Date(statusInfo.lastChecked).toLocaleTimeString()}
        </time>
      )}
    </div>
  )
}
