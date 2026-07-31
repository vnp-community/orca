# TASK-FE-006 — `useAddDevServer.ts`: Thêm `agentToken` state & subscription

**Solution:** [SOL-FE-AG-002](../solutions/SOL-FE-AG-002-direct-websocket-token-ui.md)  
**File:** `src/renderer/src/hooks/useAddDevServer.ts` [MODIFY]  
**Depends on:** TASK-FE-005 (web-preload-api exposes onAgentToken)  
**Status:** ✅ DONE (2026-07-26)  

---

## Mục tiêu

Sửa `useAddDevServer` hook để:
1. Thêm `agentToken: string | null` và `agentOrcaUrl: string | null` state
2. Subscribe `window.api.devServer.onAgentToken` trong `useEffect` (cleanup on unmount)
3. Reset token khi `reset()` được gọi (dialog close)
4. Update `canTest` logic: `direct-websocket` không cần `wsUrl`

---

## Code hiện tại

```typescript
// src/renderer/src/hooks/useAddDevServer.ts

import { useState, useCallback } from 'react'
import { useAppStore } from '../store'
import type { DevServer, DevServerInput, ConnectionTestResult } from '../../../shared/dev-server-types'

type AddState = 'idle' | 'testing' | 'connecting' | 'error'

export type UseAddDevServerReturn = {
  state: AddState
  testResult: ConnectionTestResult | null
  testConnection: (input: DevServerInput) => Promise<ConnectionTestResult>
  addAndConnect: (input: DevServerInput) => Promise<DevServer>
  reset: () => void
}

export function useAddDevServer(): UseAddDevServerReturn {
  const [state, setState] = useState<AddState>('idle')
  const [testResult, setTestResult] = useState<ConnectionTestResult | null>(null)
  // ...
  const reset = useCallback(() => {
    setState('idle')
    setTestResult(null)
  }, [])
}
```

---

## Thay đổi cần thực hiện

### File: `src/renderer/src/hooks/useAddDevServer.ts`

**Thêm imports:**
```typescript
import { useState, useCallback, useEffect } from 'react'
import type { AgentTokenInfo } from '../../../shared/dev-server-types'
```

**Sửa `UseAddDevServerReturn` type:**
```typescript
export type UseAddDevServerReturn = {
  state: AddState
  testResult: ConnectionTestResult | null
  agentToken: string | null       // NEW: token cho direct-websocket mode
  agentOrcaUrl: string | null     // NEW: Orca URL kèm theo token
  testConnection: (input: DevServerInput) => Promise<ConnectionTestResult>
  addAndConnect: (input: DevServerInput) => Promise<DevServer>
  reset: () => void
}
```

**Sửa function body:**
```typescript
export function useAddDevServer(): UseAddDevServerReturn {
  const [state, setState] = useState<AddState>('idle')
  const [testResult, setTestResult] = useState<ConnectionTestResult | null>(null)
  const [agentToken, setAgentToken] = useState<string | null>(null)     // NEW
  const [agentOrcaUrl, setAgentOrcaUrl] = useState<string | null>(null) // NEW
  // ... existing state ...

  // Subscribe to agentTokenGenerated event from backend (NEW)
  useEffect(() => {
    const handler = (info: AgentTokenInfo) => {
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
      // Reset token khi test connection mới (không phải direct-websocket thì xóa luôn)
      setAgentToken(null)        // NEW
      setAgentOrcaUrl(null)      // NEW
      try {
        const result = await window.api.devServer.testConnection(input)
        setTestResult(result)
        setState('idle')
        return result
      } catch (err) {
        const errResult: ConnectionTestResult = { ok: false, error: (err as Error).message }
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
    setAgentToken(null)    // NEW: xóa token khi dialog đóng
    setAgentOrcaUrl(null)  // NEW
  }, [])

  return {
    state,
    testResult,
    agentToken,      // NEW
    agentOrcaUrl,    // NEW
    testConnection,
    addAndConnect,
    reset,
  }
}
```

---

## Acceptance Criteria

- [x] `agentToken: string | null` được thêm vào return type
- [x] `agentOrcaUrl: string | null` được thêm vào return type
- [x] `useEffect` subscribe `onAgentToken` với cleanup `offAgentToken`
- [x] `reset()` xóa `agentToken` và `agentOrcaUrl`
- [x] `testConnection()` reset token ngay khi bắt đầu test mới
- [x] TypeScript compile không lỗi
