# Phase 1 — Types & MCP Proxy Status

**Phase:** 1/6  
**Priority:** P1 (blocking)  
**Effort:** ~0.5 ngày  
**Depends on:** nothing (entry point)  
**Unlocks:** Phase 2  
**Source solutions:** FE-IW-SOL-005

---

## Mục tiêu Phase

Tạo type definitions và hook/component cho MCP Proxy connectivity — đây là **blocking dependency** cho toàn bộ pis_* chain (khớp backend: CR-ORCA-005 phải làm đầu tiên).

---

## T01 — Tạo `mcp-proxy-types.ts`

```
Action:    NEW
File:      src/renderer/src/types/mcp-proxy-types.ts
Solution:  FE-IW-SOL-005 §SOL-5A
```

**Code cần tạo:**

```typescript
// src/renderer/src/types/mcp-proxy-types.ts
// Types cho MCP Proxy peer-invoke connectivity (CR-ORCA-005)
// Mapping từ ws.Client.Call() error cases trong SOL-ORCA-005

export type MCPProxyStatus =
  | 'connected'     // HTTP 200 từ mcp-proxy
  | 'unreachable'   // network error hoặc HTTP non-200 non-auth
  | 'auth-error'    // HTTP 403/401: "no grant for..."
  | 'unconfigured'  // "peer-invoke not configured: set ORCA_MCP_PROXY_URL"
  | 'checking'

export interface MCPProxyStatusInfo {
  status: MCPProxyStatus
  url?: string          // ORCA_MCP_PROXY_URL value khi biết
  lastChecked?: number  // epoch ms
  error?: string        // error message từ backend Call()
  latencyMs?: number    // HTTP round-trip time (backend timeout: 30s)
}
```

**Verify:** File tồn tại, TypeScript compile clean.

---

## T02 — Tạo `use-mcp-proxy-status.ts`

```
Action:    NEW
File:      src/renderer/src/hooks/use-mcp-proxy-status.ts
Solution:  FE-IW-SOL-005 §SOL-5B
```

**Code cần tạo:**

```typescript
// src/renderer/src/hooks/use-mcp-proxy-status.ts
// Hook query MCP Proxy connectivity qua orca-agent RPC (CR-ORCA-005)
// Frontend KHÔNG POST trực tiếp — luôn qua callRuntimeRpc

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

**Verify:** Import `callRuntimeRpc` resolve. TypeScript clean.

---

## T03 — Tạo `mcp-proxy-status-indicator.tsx`

```
Action:    NEW
File:      src/renderer/src/components/fleet/mcp-proxy-status-indicator.tsx
Solution:  FE-IW-SOL-005 §SOL-5C
```

**Code cần tạo:**

```typescript
// src/renderer/src/components/fleet/mcp-proxy-status-indicator.tsx
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
  connected:    { label: 'MCP Proxy Connected',       icon: '🟢', className: 'status--connected' },
  unreachable:  { label: 'MCP Proxy Unreachable',     icon: '🔴', className: 'status--error' },
  'auth-error': { label: 'MCP Proxy Auth Error',      icon: '🟡', className: 'status--warning' },
  unconfigured: { label: 'MCP Proxy Not Configured',  icon: '⚪', className: 'status--inactive' },
  checking:     { label: 'Checking connectivity…',    icon: '🔄', className: 'status--checking' },
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

      {/* latencyMs visible khi connected — backend timeout 30s (SOL-ORCA-005 §2.2) */}
      {statusInfo.latencyMs !== undefined && statusInfo.status === 'connected' && (
        <span className="mcp-proxy-status__latency">{statusInfo.latencyMs}ms</span>
      )}

      {statusInfo.error && (
        <div className="mcp-proxy-status__error" role="alert">
          {statusInfo.status === 'auth-error' ? (
            <span>Auth failed. Check <code>ORCA_MCP_PROXY_PEER_TOKEN</code>.</span>
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

      {/* Peer-invoke endpoint từ SOL-ORCA-005 §2.1 */}
      {statusInfo.url && (
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

**Verify:** Component render không lỗi, 5 status variants hiển thị đúng.

---

## T04 — Tạo `mcp-proxy-connection-test.tsx`

```
Action:    NEW
File:      src/renderer/src/components/settings/mcp-proxy-connection-test.tsx
Solution:  FE-IW-SOL-005 §SOL-5E
```

**Code cần tạo:**

```typescript
// src/renderer/src/components/settings/mcp-proxy-connection-test.tsx
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
```

**Verify:** Import MCPProxyStatusIndicator và useMCPProxyStatus resolve. TypeScript clean.

---

## Checklist Phase 1

- [ ] T01: `mcp-proxy-types.ts` tồn tại
- [ ] T02: `use-mcp-proxy-status.ts` tồn tại, import resolve
- [ ] T03: `mcp-proxy-status-indicator.tsx` tồn tại, render 5 states
- [ ] T04: `mcp-proxy-connection-test.tsx` tồn tại
- [ ] TypeScript compile: `npx tsc --noEmit` — 0 errors từ Phase 1 files
