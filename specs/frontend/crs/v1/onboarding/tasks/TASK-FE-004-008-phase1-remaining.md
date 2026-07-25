# TASK-FE-004 đến TASK-FE-008: Phase 1 Remaining Tasks

> **Status: ✅ COMPLETED** — 2026-07-23
> **Files created/modified:**
> - `src/renderer/src/hooks/useAddDevServer.ts` [NEW] — TASK-FE-004
> - `src/renderer/src/components/dev-server/DevServerStatusBadge.tsx` [NEW] — TASK-FE-005
> - `src/renderer/src/components/onboarding/DevServerStep.tsx` [NEW] — TASK-FE-006
> - `src/preload/api-types.ts` [MODIFY] — `devServer` namespace types — TASK-FE-007
> - `src/renderer/src/components/onboarding/use-onboarding-flow-types.ts` [MODIFY] — TASK-FE-008
> - `src/renderer/src/components/onboarding/OnboardingFlow.tsx` [MODIFY] — TASK-FE-008

> **Ghi chú:** Các task này được gộp vào 1 file tham khảo. Thực thi theo thứ tự.

---

# TASK-FE-004: Tạo useAddDevServer hook

**Phase:** 1 | **Solution:** [FE-SOL-A](../solutions/FE-SOL-A-dev-server-ui.md)  
**Depends on:** TASK-FE-001, TASK-FE-002, TASK-FE-007

## Goal
Tạo `useAddDevServer` hook quản lý flow: test connection → add → connect → set active.

## Steps

**Tạo** `src/renderer/src/hooks/useAddDevServer.ts`:

```typescript
import { useState, useCallback } from 'react'
import { useAppStore } from '../store'
import type { DevServerInput, ConnectionTestResult, DevServer } from '../../../shared/dev-server-types'

type AddState = 'idle' | 'testing' | 'connecting' | 'error'

export function useAddDevServer(): {
  state: AddState
  testResult: ConnectionTestResult | null
  testConnection: (input: DevServerInput) => Promise<ConnectionTestResult>
  addAndConnect: (input: DevServerInput) => Promise<DevServer>
  reset: () => void
} {
  const [state, setState] = useState<AddState>('idle')
  const [testResult, setTestResult] = useState<ConnectionTestResult | null>(null)
  const upsertDevServer = useAppStore((s) => s.upsertDevServer)
  const setActiveDevServerId = useAppStore((s) => s.setActiveDevServerId)

  const testConnection = useCallback(async (input: DevServerInput): Promise<ConnectionTestResult> => {
    setState('testing')
    setTestResult(null)
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
  }, [])

  const addAndConnect = useCallback(async (input: DevServerInput): Promise<DevServer> => {
    setState('connecting')
    try {
      const server = await window.api.devServer.add(input)
      upsertDevServer(server)
      const connected = await window.api.devServer.connect(server.id)
      upsertDevServer(connected)
      // Set active if first server:
      const current = useAppStore.getState()
      if (!current.activeDevServerId) {
        setActiveDevServerId(server.id)
        await window.api.settings.update({ activeDevServerId: server.id })
      }
      setState('idle')
      return connected
    } catch (err) {
      setState('error')
      throw err
    }
  }, [upsertDevServer, setActiveDevServerId])

  const reset = useCallback(() => {
    setState('idle')
    setTestResult(null)
  }, [])

  return { state, testResult, testConnection, addAndConnect, reset }
}
```

**Tests** (8 cases): state transitions, API calls, activeDevServerId set logic.

## Output Files
- **[NEW]** `src/renderer/src/hooks/useAddDevServer.ts`
- **[NEW]** `src/renderer/src/hooks/__tests__/useAddDevServer.test.ts`

---

# TASK-FE-005: Tạo DevServerStatusBadge component

**Phase:** 1 | **Solution:** [FE-SOL-A](../solutions/FE-SOL-A-dev-server-ui.md)  
**Depends on:** TASK-FE-001

## Goal
Tạo `DevServerStatusBadge` component hiển thị trạng thái kết nối + platform label.

## Steps

**Tạo** `src/renderer/src/components/dev-server/DevServerStatusBadge.tsx`:

```typescript
import type { DevServer } from '../../../../shared/dev-server-types'

const PLATFORM_LABEL: Record<string, string> = {
  darwin: 'macOS',
  win32: 'Windows',
  linux: 'Linux',
  freebsd: 'FreeBSD',
}

type Props = {
  status: DevServer['status']
  platform?: NodeJS.Platform | null
  showLabel?: boolean
}

export function DevServerStatusBadge({ status, platform, showLabel = true }: Props) {
  const label = status === 'connected' && platform
    ? (PLATFORM_LABEL[platform] ?? platform)
    : status

  return (
    <span
      className={`dev-server-badge dev-server-badge--${status}`}
      aria-label={`Dev server: ${label}`}
    >
      <span className="dev-server-badge__dot" aria-hidden="true" />
      {showLabel && <span className="dev-server-badge__label">{label}</span>}
    </span>
  )
}
```

**Tests** (5 cases): platform labels, status variants, showLabel=false.

## Output Files
- **[NEW]** `src/renderer/src/components/dev-server/DevServerStatusBadge.tsx`
- **[NEW]** `src/renderer/src/components/dev-server/__tests__/DevServerStatusBadge.test.tsx`

---

# TASK-FE-006: Tạo DevServerStep wizard component

**Phase:** 1 | **Solution:** [FE-SOL-A](../solutions/FE-SOL-A-dev-server-ui.md)  
**Depends on:** TASK-FE-002, TASK-FE-004, TASK-FE-005

## Goal
Tạo `DevServerStep.tsx` — bước onboarding để thêm dev server. Bao gồm: form thêm mới, test connection, danh sách servers đã kết nối.

## Steps

**Tạo** `src/renderer/src/components/onboarding/DevServerStep.tsx` theo design trong [FE-SOL-A Section 4](../solutions/FE-SOL-A-dev-server-ui.md).

Các elements cần có:
- `<Select>` cho `connectionType` (relay-ssh / relay-websocket)
- `<Input id="dev-server-name">`, `<Input id="dev-server-host">`
- `<Button id="test-connection-btn">` — disabled khi host rỗng hoặc state='testing'
- Test result feedback (success/error message)
- `<Button id="add-server-btn">` — disabled khi test chưa OK hoặc state='connecting'
- List connected servers (nếu có) với Continue button
- Skip link

**Tests** (10 cases): render, button states, API calls, skip, success flow.

## Output Files
- **[NEW]** `src/renderer/src/components/onboarding/DevServerStep.tsx`
- **[NEW]** `src/renderer/src/components/onboarding/__tests__/DevServerStep.test.tsx`

---

# TASK-FE-007: Extend window.api.devServer preload bridge

**Phase:** 1 | **Solution:** [FE-SOL-A](../solutions/FE-SOL-A-dev-server-ui.md)  
**Depends on:** TASK-FE-001

## Goal
Thêm `window.api.devServer` namespace vào preload bridge cho cả Electron và Web mode.

## Steps

1. **Tìm** type declaration của `window.api` trong codebase:
   - `src/preload/index.ts` (Electron)
   - `src/renderer/src/web/web-preload-api.ts` (Web mode)

2. **Thêm** vào cả hai files:

```typescript
// Type declaration:
devServer: {
  list: () => Promise<DevServer[]>
  add: (input: DevServerInput) => Promise<DevServer>
  remove: (id: string) => Promise<void>
  testConnection: (input: DevServerInput) => Promise<ConnectionTestResult>
  connect: (id: string) => Promise<DevServer>
  disconnect: (id: string) => Promise<void>
  onStatusChanged: (handler: (event: {
    id: string
    status: DevServer['status']
    platform?: NodeJS.Platform
    error?: string
  }) => void) => () => void
}
```

3. **Implement** bằng `ipcRenderer.invoke(...)` cho Electron và RPC call cho Web mode.

## Output Files
- **[MODIFY]** `src/preload/index.ts`
- **[MODIFY]** `src/renderer/src/web/web-preload-api.ts`

---

# TASK-FE-008: Sửa use-onboarding-flow.ts — thêm dev_server step

**Phase:** 1 | **Solution:** [FE-SOL-A](../solutions/FE-SOL-A-dev-server-ui.md)  
**Depends on:** TASK-FE-002, TASK-FE-006

## Goal
Sửa `use-onboarding-flow.ts` để:
1. Thêm `'dev_server'` là bước đầu tiên trong wizard sequence
2. Expose `activeDevServer` và `platform` trong return value
3. Render `<DevServerStep>` trong `OnboardingFlow.tsx`

## Steps

1. **Đọc** `src/renderer/src/components/onboarding/use-onboarding-flow.ts` để hiểu step type definition.

2. **Sửa** step type union — thêm `'dev_server'`:
```typescript
type OnboardingStep = 'dev_server' | 'agent' | 'theme' | 'integrations' | 'windows_terminal' | 'notifications'
```

3. **Sửa** step sequence builder — `'dev_server'` luôn là step đầu.

4. **Thêm** `activeDevServer` vào flow state và return object (từ `useActiveDevServer()` selector).

5. **Sửa** `OnboardingFlow.tsx` — thêm case `'dev_server'`:
```typescript
case 'dev_server':
  return <DevServerStep onNext={goNext} onSkip={goNext} />
```

## Output Files
- **[MODIFY]** `src/renderer/src/components/onboarding/use-onboarding-flow.ts`
- **[MODIFY]** `src/renderer/src/components/onboarding/OnboardingFlow.tsx`
