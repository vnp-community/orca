# SOL-FE-AG-002 — direct-websocket: Token Display & Agent Command Panel

**CR:** [CR-AG-004](../../../../../docs/crs/v2/agent/CR-AG-004-direct-websocket-mode.md)  
**TDD Refs:** TDD-FE-09 §7 (Dev Server UI Components)  
**Depends on:** SOL-FE-AG-003 (IPC bridge cho agentTokenGenerated), SOL-AG-004 (Backend direct-websocket)  
**Approach:** New UI component + state extension  
**Status:** ✅ IMPLEMENTED (2026-07-26)  

---

## 1. Phân tích yêu cầu

### 1.1 Backend flow (SOL-AG-004 đã implement)

```
User click "Test Connection" (direct-websocket)
  → window.api.devServer.testConnection({ connectionType: 'direct-websocket' })
  → DevServerManager.testConnection() → DevServerRelayBridge.connectDirectWebSocket()
  → generateAgentToken(devServerId)  // "agt-ds-xxx-1722033600"
  → AgentWebSocketServer.registerSlot(token, onConnected, onExpired)
  → DevServerRelayBridge.emit('agentTokenGenerated', { devServerId, agentToken, orcaUrl })
  → [IPC bridge] → window.api.devServer.onAgentToken(handler) fires
  → Frontend receives { devServerId, agentToken, orcaUrl }
  → [60s timeout] → agent không connect → testConnection rejects với error
  → [Agent connects] → handshake → testConnection resolves với ok: true
```

### 1.2 Vấn đề hiện tại

- `useAddDevServer` không có state cho `agentToken`
- `AddDevServerDialog` không có UI để hiển thị token + command
- Khi `direct-websocket` → click Test Connection → UI không làm gì ngoài spinner

---

## 2. Giải pháp

### 2.1 New component: `AgentTokenPanel.tsx`

```tsx
// src/renderer/src/components/dev-server/AgentTokenPanel.tsx
//
// Panel hiển thị agentToken + lệnh khởi động agent khi direct-websocket.
// Hiển thị sau khi backend emit agentTokenGenerated.

import { Copy, CheckCircle2, Loader2 } from 'lucide-react'
import { useState } from 'react'
import { Button } from '@/components/ui/button'

type Props = {
  agentToken: string
  orcaUrl: string
  waiting?: boolean   // true = đang chờ agent connect
}

export function AgentTokenPanel({ agentToken, orcaUrl, waiting }: Props) {
  const [copied, setCopied] = useState(false)

  const command = [
    `ORCA_URL=${orcaUrl}`,
    `AGENT_TOKEN=${agentToken}`,
    `node agent.js`,
  ].join(' \\\n  ')

  const handleCopy = async () => {
    await navigator.clipboard.writeText(command)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="rounded-md border bg-muted/50 p-3 space-y-3">
      <div className="flex items-center gap-2">
        {waiting ? (
          <>
            <Loader2 className="size-4 animate-spin text-blue-500" />
            <span className="text-sm font-medium">Waiting for agent to connect…</span>
          </>
        ) : (
          <>
            <CheckCircle2 className="size-4 text-green-500" />
            <span className="text-sm font-medium">Agent token generated</span>
          </>
        )}
      </div>

      <div className="space-y-1">
        <p className="text-xs text-muted-foreground">
          Run this command on your dev server:
        </p>
        <div className="relative rounded bg-background border p-2">
          <pre className="text-xs font-mono whitespace-pre-wrap break-all pr-8">
            {command}
          </pre>
          <Button
            variant="ghost"
            size="icon"
            className="absolute right-1 top-1 h-6 w-6"
            onClick={() => void handleCopy()}
            title="Copy command"
          >
            {copied ? (
              <CheckCircle2 className="size-3 text-green-500" />
            ) : (
              <Copy className="size-3" />
            )}
          </Button>
        </div>
      </div>

      {waiting && (
        <p className="text-xs text-muted-foreground">
          Token expires in 60s. If agent doesn&apos;t connect, the test will fail
          and you can try again.
        </p>
      )}
    </div>
  )
}
```

### 2.2 Sửa `useAddDevServer.ts` — thêm agentToken state

```typescript
// src/renderer/src/hooks/useAddDevServer.ts

import { useState, useCallback, useEffect, useRef } from 'react'
// ... existing imports

export type UseAddDevServerReturn = {
  state: AddState
  testResult: ConnectionTestResult | null
  agentToken: string | null       // NEW
  agentOrcaUrl: string | null     // NEW
  testConnection: (input: DevServerInput) => Promise<ConnectionTestResult>
  addAndConnect: (input: DevServerInput) => Promise<DevServer>
  reset: () => void
}

export function useAddDevServer(): UseAddDevServerReturn {
  const [state, setState] = useState<AddState>('idle')
  const [testResult, setTestResult] = useState<ConnectionTestResult | null>(null)
  const [agentToken, setAgentToken] = useState<string | null>(null)   // NEW
  const [agentOrcaUrl, setAgentOrcaUrl] = useState<string | null>(null)  // NEW
  // ... existing state

  // Subscribe to agentTokenGenerated event from backend
  useEffect(() => {
    const handler = (info: { devServerId: string; agentToken: string; orcaUrl: string }) => {
      setAgentToken(info.agentToken)
      setAgentOrcaUrl(info.orcaUrl)
    }
    window.api.devServer.onAgentToken?.(handler)
    return () => {
      window.api.devServer.offAgentToken?.(handler)
    }
  }, [])

  const testConnection = useCallback(
    async (input: DevServerInput): Promise<ConnectionTestResult> => {
      setState('testing')
      setTestResult(null)
      // Reset token on new test
      if (input.connectionType !== 'direct-websocket') {
        setAgentToken(null)
        setAgentOrcaUrl(null)
      }
      try {
        const result = await window.api.devServer.testConnection(input)
        setTestResult(result)
        setState('idle')
        return result
      } catch (err) {
        const errResult: ConnectionTestResult = {
          ok: false,
          error: (err as Error).message,
        }
        setTestResult(errResult)
        setState('error')
        return errResult
      }
    },
    []
  )

  const reset = useCallback(() => {
    setState('idle')
    setTestResult(null)
    setAgentToken(null)       // Reset token on dialog close
    setAgentOrcaUrl(null)
  }, [])

  return {
    state,
    testResult,
    agentToken,               // NEW
    agentOrcaUrl,             // NEW
    testConnection,
    addAndConnect,
    reset,
  }
}
```

### 2.3 Sửa `AddDevServerDialog.tsx` — hiển thị AgentTokenPanel

```tsx
// Thêm vào import:
import { AgentTokenPanel } from './AgentTokenPanel'

// Destructure agentToken từ hook:
const { state, testResult, agentToken, agentOrcaUrl, testConnection, addAndConnect, reset } = useAddDevServer()

// Thêm sau section direct-websocket info box:
{connectionType === 'direct-websocket' && agentToken && (
  <AgentTokenPanel
    agentToken={agentToken}
    orcaUrl={agentOrcaUrl ?? 'ws://<orca-host>:6768/agent'}
    waiting={state === 'testing'}
  />
)}
```

---

## 3. Files thay đổi

### [NEW] `src/renderer/src/components/dev-server/AgentTokenPanel.tsx`
Component hiển thị token + command copy.

### [MODIFY] `src/renderer/src/hooks/useAddDevServer.ts`
- Thêm `agentToken: string | null` state
- Thêm `agentOrcaUrl: string | null` state
- Subscribe `window.api.devServer.onAgentToken`
- Reset token trong `reset()`

### [MODIFY] `src/renderer/src/components/dev-server/AddDevServerDialog.tsx`
- Import và render `<AgentTokenPanel>` khi `connectionType === 'direct-websocket'` và `agentToken` có giá trị

---

## 4. UX Flow

```
1. User chọn "direct-websocket" mode
2. Dialog hiển thị info box (không có URL input)
3. User click "Test Connection"
   → Spinner, state = 'testing'
   → Backend emit agentTokenGenerated
   → AgentTokenPanel xuất hiện với token + command
   → User copy lệnh, chạy agent trên dev server
4. Agent connect → backend handshake
   → testConnection resolves ok: true
   → testResult xuất hiện "✓ Connected"
5. User click "Add Server"
   → server được lưu, relay session active
```

---

## 5. Acceptance Criteria

- [x] `direct-websocket`: Click "Test Connection" → `AgentTokenPanel` xuất hiện trong vòng 1s
- [x] `AgentTokenPanel`: Token hiển thị dạng full command có thể copy
- [x] `AgentTokenPanel`: Copy button hoạt động (clipboard API)
- [x] `direct-websocket`: Timeout 60s → `testResult` hiển thị error message rõ ràng
- [x] Reset: Khi dialog đóng và mở lại, token cũ bị xóa
- [x] `relay-websocket` và `relay-ssh`: Không bị ảnh hưởng
- [x] TypeScript compile không lỗi
