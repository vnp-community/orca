# FE-SOL-A: Dev Server UI — Frontend Solution

**CR:** [CR-OB-002](../../../../../docs/crs/v1/onboarding/CR-OB-002-dev-server-registration.md)  
**TDD refs:** TDD-FE-02 (State), TDD-FE-05 (Components), TDD-FE-07 (Hooks)  
**Status:** ✅ COMPLETED (2026-07-23) | **Phase:** 1

---

## 1. New Files

```
src/renderer/src/
├── components/onboarding/
│   └── DevServerStep.tsx               ← Wizard bước thêm dev server
├── components/dev-server/
│   ├── DevServerCard.tsx               ← Card hiển thị 1 dev server + status
│   ├── DevServerList.tsx               ← Danh sách servers trong Settings
│   ├── AddDevServerDialog.tsx          ← Dialog thêm mới
│   └── DevServerStatusBadge.tsx        ← Badge: connected/disconnected/error
├── hooks/
│   ├── useDevServers.ts                ← Zustand selector + IPC subscription
│   └── useDevServerConnection.ts       ← connect/disconnect actions
└── store/slices/
    └── dev-servers.ts                  ← NEW Zustand slice
```

---

## 2. Zustand Slice — `src/renderer/src/store/slices/dev-servers.ts`

```typescript
import { create } from 'zustand'
import { useShallow } from 'zustand/react/shallow'
import type { DevServer, DevServerInput, ConnectionTestResult } from '../../../../shared/dev-server-types'

type DevServerSlice = {
  devServers: DevServer[]
  activeDevServerId: string | null

  // Actions:
  setDevServers: (servers: DevServer[]) => void
  upsertDevServer: (server: DevServer) => void
  removeDevServer: (id: string) => void
  setActiveDevServerId: (id: string | null) => void
  updateDevServerStatus: (id: string, status: DevServer['status'], extra?: Partial<DevServer>) => void
}

export const createDevServerSlice = (set: SetState<AppState>): DevServerSlice => ({
  devServers: [],
  activeDevServerId: null,

  setDevServers: (servers) => set({ devServers: servers }),

  upsertDevServer: (server) =>
    set(state => ({
      devServers: state.devServers.some(ds => ds.id === server.id)
        ? state.devServers.map(ds => ds.id === server.id ? server : ds)
        : [...state.devServers, server]
    })),

  removeDevServer: (id) =>
    set(state => ({
      devServers: state.devServers.filter(ds => ds.id !== id),
      activeDevServerId: state.activeDevServerId === id ? null : state.activeDevServerId
    })),

  setActiveDevServerId: (id) => set({ activeDevServerId: id }),

  updateDevServerStatus: (id, status, extra = {}) =>
    set(state => ({
      devServers: state.devServers.map(ds =>
        ds.id === id ? { ...ds, status, ...extra } : ds
      )
    }))
})

// ── Selectors ─────────────────────────────────────────────────
export function useDevServers() {
  return useAppStore(useShallow(s => s.devServers))
}

export function useActiveDevServer() {
  return useAppStore(useShallow(s => {
    const id = s.activeDevServerId
    return id ? s.devServers.find(ds => ds.id === id) ?? null : null
  }))
}

export function useConnectedDevServers() {
  return useAppStore(useShallow(s =>
    s.devServers.filter(ds => ds.status === 'connected')
  ))
}
```

---

## 3. Hooks

### `useDevServers.ts`

```typescript
// src/renderer/src/hooks/useDevServers.ts
import { useEffect } from 'react'
import { useAppStore } from '../store'

export function useDevServersSync(): void {
  const setDevServers = useAppStore(s => s.setDevServers)
  const upsertDevServer = useAppStore(s => s.upsertDevServer)
  const updateDevServerStatus = useAppStore(s => s.updateDevServerStatus)

  useEffect(() => {
    // Load initial list
    window.api.devServer.list().then(setDevServers)

    // Subscribe to status changes (push from backend)
    const offStatus = window.api.devServer.onStatusChanged((event) => {
      updateDevServerStatus(event.id, event.status, {
        platform: event.platform ?? undefined,
        lastError: event.error ?? null
      })
    })

    return () => {
      offStatus()
    }
  }, [setDevServers, upsertDevServer, updateDevServerStatus])
}
```

### `useDevServerConnection.ts`

```typescript
// src/renderer/src/hooks/useDevServerConnection.ts
import { useState, useCallback } from 'react'
import { useAppStore } from '../store'

type ConnectionState = 'idle' | 'testing' | 'connecting' | 'error'

export function useAddDevServer() {
  const [state, setState] = useState<ConnectionState>('idle')
  const [testResult, setTestResult] = useState<ConnectionTestResult | null>(null)
  const upsertDevServer = useAppStore(s => s.upsertDevServer)
  const setActiveDevServerId = useAppStore(s => s.setActiveDevServerId)

  const testConnection = useCallback(async (input: DevServerInput) => {
    setState('testing')
    setTestResult(null)
    try {
      const result = await window.api.devServer.testConnection(input)
      setTestResult(result)
      setState('idle')
      return result
    } catch (err) {
      const errResult = { ok: false, error: (err as Error).message } as ConnectionTestResult
      setTestResult(errResult)
      setState('error')
      return errResult
    }
  }, [])

  const addAndConnect = useCallback(async (input: DevServerInput) => {
    setState('connecting')
    try {
      const server = await window.api.devServer.add(input)
      upsertDevServer(server)
      await window.api.devServer.connect(server.id)
      // Active server nếu là đầu tiên
      const current = useAppStore.getState()
      if (!current.activeDevServerId) {
        setActiveDevServerId(server.id)
        await window.api.settings.update({ activeDevServerId: server.id })
      }
      setState('idle')
      return server
    } catch (err) {
      setState('error')
      throw err
    }
  }, [upsertDevServer, setActiveDevServerId])

  return { state, testResult, testConnection, addAndConnect }
}
```

---

## 4. Component — `DevServerStep.tsx`

```tsx
// src/renderer/src/components/onboarding/DevServerStep.tsx
import React, { useState } from 'react'
import { useAddDevServer } from '../../hooks/useDevServerConnection'
import { useConnectedDevServers } from '../../store/slices/dev-servers'
import { DevServerStatusBadge } from '../dev-server/DevServerStatusBadge'
import { Button } from '../ui/button'
import { Input } from '../ui/input'
import { Select, SelectItem } from '../ui/select'

type DevServerStepProps = {
  onNext: () => void
  onSkip: () => void
}

export function DevServerStep({ onNext, onSkip }: DevServerStepProps) {
  const [name, setName] = useState('')
  const [host, setHost] = useState('')
  const [connectionType, setConnectionType] = useState<'relay-ssh'>('relay-ssh')
  const { state, testResult, testConnection, addAndConnect } = useAddDevServer()
  const connectedServers = useConnectedDevServers()

  const handleTestConnection = async () => {
    await testConnection({ name, connectionType, wsUrl: host })
  }

  const handleAdd = async () => {
    await addAndConnect({ name, connectionType, wsUrl: host })
  }

  return (
    <div className="onboarding-step dev-server-step">
      <h2>Connect your dev environment</h2>
      <p className="step-description">
        Orca runs in the cloud. Connect the machine where your code and tools live.
      </p>

      {/* Already connected servers */}
      {connectedServers.length > 0 && (
        <div className="connected-servers">
          {connectedServers.map(ds => (
            <DevServerCard key={ds.id} server={ds} />
          ))}
          <Button variant="primary" onClick={onNext}>
            Continue with {connectedServers.length} server{connectedServers.length > 1 ? 's' : ''}
          </Button>
        </div>
      )}

      {/* Add new server form */}
      <div className="add-server-form">
        <div className="form-row">
          <label>Name</label>
          <Input
            id="dev-server-name"
            placeholder="MacBook Pro M3"
            value={name}
            onChange={e => setName(e.target.value)}
          />
        </div>
        <div className="form-row">
          <label>Host</label>
          <Input
            id="dev-server-host"
            placeholder="user@dev.example.com"
            value={host}
            onChange={e => setHost(e.target.value)}
          />
        </div>
        <div className="form-row">
          <label>Connection type</label>
          <Select value={connectionType} onValueChange={v => setConnectionType(v as 'relay-ssh')}>
            <SelectItem value="relay-ssh">SSH Relay</SelectItem>
            <SelectItem value="relay-websocket">WebSocket (dev server connects to Orca)</SelectItem>
          </Select>
        </div>

        {/* Test result */}
        {testResult && (
          <div className={`test-result ${testResult.ok ? 'success' : 'error'}`}>
            {testResult.ok
              ? `✓ Connected — ${testResult.platform} · Node ${testResult.nodeVersion}`
              : `✗ ${testResult.error}`}
          </div>
        )}

        <div className="form-actions">
          <Button
            id="test-connection-btn"
            variant="secondary"
            onClick={handleTestConnection}
            disabled={!host || state === 'testing'}
            loading={state === 'testing'}
          >
            Test Connection
          </Button>
          <Button
            id="add-server-btn"
            variant="primary"
            onClick={handleAdd}
            disabled={!testResult?.ok || state === 'connecting'}
            loading={state === 'connecting'}
          >
            Add Server
          </Button>
        </div>
      </div>

      {/* Skip */}
      <button className="skip-link" onClick={onSkip}>
        Skip — I'll add a dev server later
      </button>
    </div>
  )
}
```

---

## 5. Component — `DevServerStatusBadge.tsx`

```tsx
// src/renderer/src/components/dev-server/DevServerStatusBadge.tsx
type Props = { status: DevServer['status']; platform?: NodeJS.Platform | null }

const PLATFORM_LABEL: Record<string, string> = {
  darwin: 'macOS',
  win32: 'Windows',
  linux: 'Linux'
}

export function DevServerStatusBadge({ status, platform }: Props) {
  return (
    <span className={`dev-server-badge dev-server-badge--${status}`}>
      <span className="status-dot" />
      <span className="status-label">
        {status === 'connected' && platform
          ? PLATFORM_LABEL[platform] ?? platform
          : status}
      </span>
    </span>
  )
}
```

---

## 6. `useOnboardingFlow` — Thêm bước `dev_server` (MODIFY)

```typescript
// src/renderer/src/components/onboarding/use-onboarding-flow.ts (MODIFY)

// Thêm 'dev_server' vào OnboardingStep union:
type OnboardingStep =
  | 'dev_server'          // NEW — đầu tiên
  | 'agent'
  | 'theme'
  | 'integrations'
  | 'windows_terminal'
  | 'notifications'

// Thêm logic skip:
function buildStepSequence(state: OnboardingFlowState): OnboardingStep[] {
  const steps: OnboardingStep[] = ['dev_server', 'agent', 'theme']

  // Integrations: skip nếu gh đã có VÀ git đã có trên active dev server
  if (!state.preflightStatus?.gh.installed) steps.push('integrations')

  // Windows terminal: chỉ khi active dev server = win32
  if (state.activeDevServer?.platform === 'win32') steps.push('windows_terminal')

  steps.push('notifications')
  return steps
}

// Thêm activeDevServer vào state:
type OnboardingFlowState = {
  // ... existing
  activeDevServer: DevServer | null          // NEW
  activeDevServerPlatform: NodeJS.Platform | null  // NEW
}
```

---

## 7. `useIpcEvents` — Subscribe dev server events (MODIFY)

```typescript
// src/renderer/src/hooks/useIpcEvents.ts (MODIFY — thêm vào useEffect):

// Dev server status changes:
const offDevServerStatus = window.api.devServer.onStatusChanged((event) => {
  useAppStore.getState().updateDevServerStatus(event.id, event.status, {
    platform: event.platform,
    lastConnectedAt: event.status === 'connected' ? Date.now() : undefined
  })
})

// cleanup:
return () => {
  // ... existing cleanups
  offDevServerStatus()
}
```

---

## 8. Preload Bridge — `window.api.devServer` (MODIFY)

```typescript
// src/renderer/src/web/web-preload-api.ts (MODIFY — thêm namespace):
// Hoặc: src/preload/index.ts cho Electron mode

window.api.devServer = {
  list: () => ipcRenderer.invoke('devServer.list'),
  add: (input) => ipcRenderer.invoke('devServer.add', input),
  remove: (id) => ipcRenderer.invoke('devServer.remove', id),
  testConnection: (input) => ipcRenderer.invoke('devServer.testConnection', input),
  connect: (id) => ipcRenderer.invoke('devServer.connect', id),
  disconnect: (id) => ipcRenderer.invoke('devServer.disconnect', id),
  onStatusChanged: (handler) => {
    window.api.on('devServer:statusChanged', handler)
    return () => window.api.off('devServer:statusChanged', handler)
  }
}
```

---

## 9. Tests

```tsx
// @vitest-environment happy-dom
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { DevServerStep } from './DevServerStep'

describe('DevServerStep', () => {
  beforeEach(() => {
    vi.stubGlobal('window', {
      api: {
        devServer: {
          testConnection: vi.fn(),
          add: vi.fn(),
          connect: vi.fn(),
          list: vi.fn().mockResolvedValue([]),
          onStatusChanged: vi.fn(() => () => {})
        }
      }
    })
  })

  it('render form với name, host, connection type inputs')
  it('Test Connection button disabled khi host rỗng')
  it('Test Connection gọi api.devServer.testConnection với đúng params')
  it('Hiển thị success message khi testConnection ok:true')
  it('Hiển thị error message khi testConnection ok:false')
  it('Add Server button disabled khi chưa test connection thành công')
  it('Add Server gọi add() rồi connect()')
  it('Sau khi add thành công, set activeDevServerId nếu là server đầu tiên')
  it('Skip link gọi onSkip()')
  it('DevServerStatusBadge hiển thị "macOS" khi platform = darwin')
  it('DevServerStatusBadge hiển thị "Windows" khi platform = win32')
})

describe('dev-servers Zustand slice', () => {
  it('setDevServers() thay thế toàn bộ list')
  it('upsertDevServer() thêm mới nếu không có')
  it('upsertDevServer() cập nhật nếu đã có id')
  it('removeDevServer() xóa và reset activeDevServerId nếu là active')
  it('updateDevServerStatus() chỉ update đúng server')
})
```

---

## 10. Checklist triển khai

- [x] Tạo `src/renderer/src/store/slices/dev-servers.ts`
- [x] Thêm `createDevServerSlice` vào `store/index.ts`
- [x] Tạo `useDevServersSync` hook + đăng ký trong `useIpcEvents`
- [x] Tạo `useAddDevServer` hook
- [x] Tạo `DevServerStatusBadge` component (`components/dev-server/DevServerStatusBadge.tsx`)
- [x] Tạo `DevServerCard` component (`components/dev-server/DevServerCard.tsx`)
- [x] Tạo `DevServerList` component (`components/dev-server/DevServerList.tsx`)
- [x] Tạo `AddDevServerDialog` component (`components/dev-server/AddDevServerDialog.tsx`)
- [x] Tạo `DevServerStep.tsx` component
- [x] Extend `window.api.devServer` trong preload bridge
- [ ] Unit tests (DevServerStep + slice — deferred to test pass)
