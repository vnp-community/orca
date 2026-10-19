// Settings panel cho MCP Proxy peer-invoke test (CR-ORCA-005)

import type { FC } from 'react'
import { useMCPProxyStatus } from '../../hooks/use-mcp-proxy-status'
import { MCPProxyStatusIndicator } from '../fleet/mcp-proxy-status-indicator'

interface Props {
  environmentId: string
}

export const MCPProxyConnectionTest: FC<Props> = ({ environmentId }) => {
  const { statusInfo, checkStatus } = useMCPProxyStatus(environmentId)

  return (
    <div className="mcp-proxy-connection-test" id="mcp-proxy-connection-test">
      <h4>MCP Proxy (Peer-Invoke)</h4>
      <p className="mcp-proxy-connection-test__desc">
        orca-agent sử dụng HTTP POST đến{' '}
        <code>/internal/v1/agent-peer/mcp/proxy</code> cho cross-agent calls — không qua
        WebSocket. Cấu hình qua <code>ORCA_MCP_PROXY_URL</code> +{' '}
        <code>ORCA_MCP_PROXY_PEER_TOKEN</code>.
      </p>

      <MCPProxyStatusIndicator
        environmentId={environmentId}
        statusInfo={statusInfo}
        onRecheck={checkStatus}
      />

      {statusInfo.status !== 'unconfigured' && (
        <button
          className="mcp-proxy-connection-test__check-btn"
          onClick={checkStatus}
          id="mcp-proxy-initial-check"
          disabled={statusInfo.status === 'checking'}
        >
          Check Connectivity
        </button>
      )}
    </div>
  )
}
