// Surface peer-invoke errors trong task context (CR-ORCA-005)
//
// Classify theo exact backend error strings từ ws.Client.Call() (SOL-ORCA-005 §2.2):
//   "peer-invoke not configured: set ORCA_MCP_PROXY_URL"  → not-configured
//   "peer-invoke: no grant for..."                         → auth-error
//   "peer-invoke ... HTTP 503: ..."                        → network-error

import type { FC } from 'react'

interface Props {
  toolName: string
  errorMessage: string
}

type PeerInvokeErrorType = 'not-configured' | 'auth-error' | 'network-error' | 'unknown'

// Classify error message từ SOL-ORCA-005 §2.2 Call() error cases
function classifyError(msg: string): PeerInvokeErrorType {
  if (msg.includes('not configured') || msg.includes('ORCA_MCP_PROXY_URL')) {
    return 'not-configured'
  }
  if (msg.includes('no grant') || msg.includes('HTTP 403') || msg.includes('HTTP 401')) {
    return 'auth-error'
  }
  if (msg.includes('peer-invoke') && /HTTP \d/.test(msg)) {
    return 'network-error'
  }
  return 'unknown'
}

interface ErrorConfig {
  icon: string
  title: string
  fix: React.ReactNode
}

export const TaskPeerInvokeError: FC<Props> = ({ toolName, errorMessage }) => {
  const errorType = classifyError(errorMessage)

  const errorConfig: Record<PeerInvokeErrorType, ErrorConfig> = {
    'not-configured': {
      icon: '⚙️',
      title: 'KG Integration Not Configured',
      fix: (
        <p>
          Set <code>ORCA_MCP_PROXY_URL</code> và{' '}
          <code>ORCA_BRIDGES_PRODUCTS_AGENT_ID</code> trong orca-agent deploy config, sau đó
          restart orca-agent.
        </p>
      ),
    },
    'auth-error': {
      icon: '🔑',
      title: 'Peer-Invoke Auth Failed',
      fix: (
        <p>
          Kiểm tra <code>ORCA_MCP_PROXY_PEER_TOKEN</code> — có thể expired hoặc thiếu grant cho{' '}
          <code>{toolName}</code>.
        </p>
      ),
    },
    'network-error': {
      icon: '🔌',
      title: 'MCP Proxy Unreachable',
      fix: (
        <p>
          Verify mcp-proxy đang chạy tại <code>ORCA_MCP_PROXY_URL</code> và reachable từ
          orca-agent.
        </p>
      ),
    },
    unknown: {
      icon: '❌',
      title: 'KG Query Failed',
      fix: null,
    },
  }

  const cfg = errorConfig[errorType]

  return (
    <div
      className={`task-peer-invoke-error task-peer-invoke-error--${errorType}`}
      role="alert"
      id={`peer-invoke-error-${toolName}`}
    >
      <span className="task-peer-invoke-error__icon" aria-hidden="true">
        {cfg.icon}
      </span>
      <div className="task-peer-invoke-error__content">
        <strong>{cfg.title}</strong>
        <p>
          Tool <code>{toolName}</code>: {errorMessage}
        </p>
        {cfg.fix}
      </div>
    </div>
  )
}
