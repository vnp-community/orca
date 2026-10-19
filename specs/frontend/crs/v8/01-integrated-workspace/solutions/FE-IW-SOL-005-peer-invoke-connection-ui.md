# FE-IW-SOL-005 — Frontend: Peer-Invoke Connection Status UI

**Solution ID:** FE-IW-SOL-005  
**CR Reference:** CR-ORCA-005 (orca-ws-client-call-peer-invoke)  
**Backend Solution:** SOL-ORCA-005  
**TDD Reference:** TDD-FE-03 Runtime Client | TDD-FE-10 Fleet Management  
**Status:** ✅ Done — implemented 2026-10-19  
**Priority:** P1  
**Effort:** 0.5–1 ngày  
**Depends On:** SOL-ORCA-005 backend (ws.Client.Call() implemented)

---

## Alignment với Backend Solution

| Backend detail (SOL-ORCA-005) | Frontend implication |
|-------------------------------|----------------------|
| `ws.Client.Call()` — HTTP POST đến `mcp-proxy`, KHÔNG qua WS | Frontend không cần biết transport detail — chỉ hiển thị status |
| Request body: `{"agent_id": targetAgentID, "tool": toolName, "input": {...}}` | Frontend gọi qua `callRuntimeRpc` → orca-agent → proxy, không POST trực tiếp |
| Endpoint: `/internal/v1/agent-peer/mcp/proxy` | Frontend dùng đây để construct test URL hint |
| Error khi `mcpProxyURL == ""`: `"peer-invoke not configured: set ORCA_MCP_PROXY_URL"` | Frontend detect string này để hiển thị config hint |
| Error `403/401`: `"peer-invoke: no grant for %s on %s"` | Frontend map → "auth-error" status |
| HTTP timeout: 30s (`http.Client{Timeout: 30 * time.Second}`) | Frontend show loading spinner nếu > 5s |
| `SetPeerConfig(mcpProxyURL, peerToken)` — setter sau `NewClient` | Frontend không expose này — backend init detail |
| Compile-time: `var _ pis.PeerCaller = (*Client)(nil)` | Frontend không có equivalent — type safety ở Go layer |
| `authResult.WebSocketURL` dùng cho WS, `MCPProxyURL` dùng cho HTTP peer-invoke — **tách biệt** | Frontend phân biệt WS connection status vs. Proxy connection status |

---

## Giải pháp

### SOL-5A: MCP Proxy Connection Status Types

```typescript
// src/renderer/src/types/mcp-proxy-types.ts

// Mapping từ error messages của SOL-ORCA-005
export type MCPProxyStatus =
  | 'connected'     // HTTP 200 from proxy
  | 'unreachable'   // network error hoặc HTTP !200 non-auth
  | 'auth-error'    // HTTP 403/401: "no grant for..."
  | 'unconfigured'  // "peer-invoke not configured: set ORCA_MCP_PROXY_URL"
  | 'checking'

export interface MCPProxyStatusInfo {
  status: MCPProxyStatus
  url?: string                  // ORCA_MCP_PROXY_URL value khi biết
  lastChecked?: number          // epoch ms
  error?: string                // error message từ backend
  latencyMs?: number            // HTTP round-trip time
}
```

---

### SOL-5B: `useMCPProxyStatus` Hook

```typescript
// src/renderer/src/hooks/use-mcp-proxy-status.ts

import { useCallback, useState } from 'react'
import { callRuntimeRpc } from '../runtime/runtime-rpc-client'
import type { MCPProxyStatus, MCPProxyStatusInfo } from '../types/mcp-proxy-types'

export function useMCPProxyStatus(environmentId: string) {
  const [statusInfo, setStatusInfo] = useState<MCPProxyStatusInfo>({
    status: 'unconfigured',
  })

  const checkStatus = useCallback(async () => {
    setStatusInfo((s) => ({ ...s, status: 'checking' }))
    const start = Date.now()

    try {
      // Backend check: orca-agent kiểm tra mcpProxyURL config và ping proxy
      // Trả về structured result thay vì raw error
      const result = await callRuntimeRpc<{
        configured: boolean
        url?: string
        reachable?: boolean
        authOk?: boolean
        error?: string
      }>(
        { kind: 'environment', environmentId },
        'agent.check_mcp_proxy',
        {}
      )

      const latencyMs = Date.now() - start

      if (!result.configured) {
        // Backend ORCA_MCP_PROXY_URL chưa set — SOL-ORCA-005 §2.2
        setStatusInfo({ status: 'unconfigured', latencyMs })
        return
      }

      if (!result.reachable) {
        setStatusInfo({
          status: 'unreachable',
          url: result.url,
          error: result.error,
          latencyMs,
          lastChecked: Date.now(),
        })
        return
      }

      if (!result.authOk) {
        // HTTP 403/401 từ mcp-proxy — SOL-ORCA-005 §2.2 error case
        setStatusInfo({
          status: 'auth-error',
          url: result.url,
          error: 'Peer token invalid or missing grant',
          latencyMs,
          lastChecked: Date.now(),
        })
        return
      }

      setStatusInfo({
        status: 'connected',
        url: result.url,
        latencyMs,
        lastChecked: Date.now(),
      })
    } catch (e) {
      setStatusInfo({
        status: 'unreachable',
        error: String(e),
        lastChecked: Date.now(),
      })
    }
  }, [environmentId])

  return { statusInfo, checkStatus }
}
```

---

### SOL-5C: MCPProxyStatusIndicator Component

```typescript
// src/renderer/src/components/fleet/mcp-proxy-status-indicator.tsx

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
      <span className="mcp-proxy-status__icon" aria-hidden="true">{cfg.icon}</span>
      <span className="mcp-proxy-status__label">{cfg.label}</span>

      {statusInfo.url && (
        <code className="mcp-proxy-status__url">{statusInfo.url}</code>
      )}

      {statusInfo.latencyMs !== undefined && statusInfo.status === 'connected' && (
        <span className="mcp-proxy-status__latency">{statusInfo.latencyMs}ms</span>
      )}

      {/* Error messages từ SOL-ORCA-005 §2.2 */}
      {statusInfo.error && (
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

      {/* Endpoint hint — từ SOL-ORCA-005 §2.1 */}
      {statusInfo.url && (
        <div className="mcp-proxy-status__endpoint">
          Peer-invoke endpoint:{' '}
          <code>{statusInfo.url}/internal/v1/agent-peer/mcp/proxy</code>
        </div>
      )}

      {statusInfo.status !== 'unconfigured' && (
        <button
          className="mcp-proxy-status__recheck-btn"
          onClick={onRecheck}
          id={`mcp-proxy-recheck-${environmentId}`}
          disabled={statusInfo.status === 'checking'}
          aria-label="Re-check MCP Proxy connectivity"
          title="Re-check"
        >
          {statusInfo.status === 'checking' ? '…' : '↺'}
        </button>
      )}

      {statusInfo.lastChecked && (
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
```

---

### SOL-5D: Peer-Invoke Error Surface — Task Context

Khi AI agent call pis_* tool và peer-invoke fail, error message từ backend có format:
- `"peer-invoke not configured: set ORCA_MCP_PROXY_URL"` — config issue
- `"peer-invoke: no grant for pis_query_crs on products-agent: ..."` — auth issue
- `"peer-invoke pis_query_crs HTTP 503: ..."` — network issue

```typescript
// src/renderer/src/components/tasks/task-peer-invoke-error.tsx

interface Props {
  toolName: string
  errorMessage: string
}

type PeerInvokeErrorType = 'not-configured' | 'auth-error' | 'network-error' | 'unknown'

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

export const TaskPeerInvokeError: FC<Props> = ({ toolName, errorMessage }) => {
  const errorType = classifyError(errorMessage)

  const errorConfig = {
    'not-configured': {
      icon: '⚙️',
      title: 'KG Integration Not Configured',
      fix: (
        <p>
          Set <code>ORCA_MCP_PROXY_URL</code> and <code>ORCA_BRIDGES_PRODUCTS_AGENT_ID</code>{' '}
          in the orca-agent deploy config, then restart orca-agent.
        </p>
      ),
    },
    'auth-error': {
      icon: '🔑',
      title: 'Peer-Invoke Auth Failed',
      fix: (
        <p>
          Check <code>ORCA_MCP_PROXY_PEER_TOKEN</code> — it may be expired or missing grants
          for <code>{toolName}</code>.
        </p>
      ),
    },
    'network-error': {
      icon: '🔌',
      title: 'MCP Proxy Unreachable',
      fix: (
        <p>
          Verify mcp-proxy is running at <code>ORCA_MCP_PROXY_URL</code> and reachable from
          orca-agent.
        </p>
      ),
    },
    unknown: {
      icon: '❌',
      title: 'KG Query Failed',
      fix: null,
    },
  }[errorType]

  return (
    <div
      className={`task-peer-invoke-error task-peer-invoke-error--${errorType}`}
      role="alert"
      id={`peer-invoke-error-${toolName}`}
    >
      <span className="task-peer-invoke-error__icon" aria-hidden="true">
        {errorConfig.icon}
      </span>
      <div className="task-peer-invoke-error__content">
        <strong>{errorConfig.title}</strong>
        <p>
          Tool <code>{toolName}</code>: {errorMessage}
        </p>
        {errorConfig.fix}
      </div>
    </div>
  )
}
```

---

### SOL-5E: Connection Test in Agent Settings

```typescript
// src/renderer/src/components/settings/mcp-proxy-connection-test.tsx

export const MCPProxyConnectionTest: FC<{ environmentId: string }> = ({ environmentId }) => {
  const { statusInfo, checkStatus } = useMCPProxyStatus(environmentId)

  return (
    <div className="mcp-proxy-connection-test" id="mcp-proxy-connection-test">
      <h4>MCP Proxy (Peer-Invoke)</h4>
      <p className="mcp-proxy-connection-test__desc">
        orca-agent uses HTTP POST to{' '}
        <code>/internal/v1/agent-peer/mcp/proxy</code> for cross-agent calls (not WebSocket).
        Configured via <code>ORCA_MCP_PROXY_URL</code> + <code>ORCA_MCP_PROXY_PEER_TOKEN</code>.
      </p>

      <MCPProxyStatusIndicator
        environmentId={environmentId}
        statusInfo={statusInfo}
        onRecheck={checkStatus}
      />

      {/* Trigger initial check on mount */}
      <button
        className="mcp-proxy-connection-test__initial-check"
        onClick={checkStatus}
        id="mcp-proxy-initial-check"
        style={{ display: statusInfo.status === 'unconfigured' ? 'none' : undefined }}
      >
        Check Connectivity
      </button>
    </div>
  )
}
```

---

## Files cần tạo/sửa

```
src/renderer/src/
├── types/
│   └── mcp-proxy-types.ts                       ← NEW (MCPProxyStatus enum, MCPProxyStatusInfo)
├── hooks/
│   └── use-mcp-proxy-status.ts                  ← NEW
└── components/
    ├── fleet/
    │   └── mcp-proxy-status-indicator.tsx        ← NEW
    ├── tasks/
    │   └── task-peer-invoke-error.tsx             ← NEW
    └── settings/
        └── mcp-proxy-connection-test.tsx          ← NEW
```

---

## Tests

```typescript
// hooks/__tests__/use-mcp-proxy-status.test.ts
describe('useMCPProxyStatus', () => {
  it('status=unconfigured khi configured=false')
  it('status=unreachable khi reachable=false')
  it('status=auth-error khi authOk=false')
  it('status=connected khi reachable=true && authOk=true')
  it('latencyMs calculated từ start đến response')
  it('status=checking trong khi call đang chạy')
  it('status=unreachable nếu callRuntimeRpc throw')
})

// components/fleet/__tests__/mcp-proxy-status-indicator.test.tsx
describe('MCPProxyStatusIndicator', () => {
  it('render 🟢 và label "Connected" khi connected')
  it('render 🔴 khi unreachable')
  it('render 🟡 khi auth-error')
  it('render ⚪ khi unconfigured — không có recheck button')
  it('recheck button disabled khi checking')
  it('hiển thị endpoint URL: {url}/internal/v1/agent-peer/mcp/proxy')
  it('hiển thị config hint "ORCA_MCP_PROXY_URL" khi unconfigured')
  it('aria-label = status label text')
})

// components/tasks/__tests__/task-peer-invoke-error.test.tsx
describe('TaskPeerInvokeError', () => {
  it('not-configured: detect "ORCA_MCP_PROXY_URL" trong error message')
  it('auth-error: detect "no grant" trong error message')
  it('network-error: detect "HTTP 503" trong error message')
  it('fix hint show đúng env var name per error type')
  it('role=alert trên tất cả variants')
  it('id unique: peer-invoke-error-{toolName}')
})
```

---

## Acceptance Criteria (Frontend)

| AC | Mô tả | Khớp với Backend AC |
|----|-------|---------------------|
| AC-F1 | 5 status types map đúng backend error strings | SOL-ORCA-005 §2.2 `Call()` error cases |
| AC-F2 | Endpoint hint: `/internal/v1/agent-peer/mcp/proxy` | SOL-ORCA-005 §2.1 — URL path cố định |
| AC-F3 | Config hint dùng đúng env var: `ORCA_MCP_PROXY_URL` | SOL-ORCA-005 §2.2 error message |
| AC-F4 | Auth error hint dùng: `ORCA_MCP_PROXY_PEER_TOKEN` | SOL-ORCA-005 §2.2 `peerToken` field |
| AC-F5 | WS status và Proxy status **hiển thị riêng** (không ghép) | SOL-ORCA-005 §2.1 — WS vs HTTP tách biệt |
| AC-F6 | `latencyMs` hiển thị khi connected (30s HTTP timeout backend) | SOL-ORCA-005 §2.2 `http.Client{Timeout: 30s}` |
| AC-F7 | Frontend không POST trực tiếp đến mcp-proxy | SOL-ORCA-005 — luôn qua `callRuntimeRpc` → orca-agent |
