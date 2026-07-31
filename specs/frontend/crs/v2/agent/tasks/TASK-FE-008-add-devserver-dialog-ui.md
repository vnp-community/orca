# TASK-FE-008 — `AddDevServerDialog.tsx`: Tách UI cho từng connection type

**Solution:** [SOL-FE-AG-001](../solutions/SOL-FE-AG-001-relay-websocket-ui.md), [SOL-FE-AG-002](../solutions/SOL-FE-AG-002-direct-websocket-token-ui.md)  
**File:** `src/renderer/src/components/dev-server/AddDevServerDialog.tsx` [MODIFY]  
**Depends on:** TASK-FE-006 (agentToken from hook), TASK-FE-007 (AgentTokenPanel)  
**Status:** ✅ DONE (2026-07-26)  

---

## Mục tiêu

Sửa `AddDevServerDialog` để:
1. Tách UI riêng cho `relay-websocket` (URL input + setup guide) và `direct-websocket` (info box, không cần URL)
2. Hiển thị `AgentTokenPanel` sau khi backend generate token
3. Fix `canTest` logic: `direct-websocket` luôn có thể test

---

## Thay đổi 1: Import thêm và destructure agentToken

```tsx
// Thêm import:
import { AgentTokenPanel } from './AgentTokenPanel'

// Sửa destructure từ useAddDevServer():
const {
  state,
  testResult,
  agentToken,    // NEW
  agentOrcaUrl,  // NEW
  testConnection,
  addAndConnect,
  reset
} = useAddDevServer()
```

## Thay đổi 2: Fix `canTest` logic (line ~143-144)

```typescript
// TRƯỚC:
const canTest =
  connectionType === 'relay-ssh' ? Boolean(sshTargetId) : Boolean(wsUrl)

// SAU:
const canTest =
  connectionType === 'relay-ssh'
    ? Boolean(sshTargetId)
    : connectionType === 'relay-websocket'
      ? Boolean(wsUrl)
      : true  // direct-websocket: không cần wsUrl — agent connect vào Orca
```

## Thay đổi 3: Thay thế WebSocket section (line ~299-312)

```tsx
// TRƯỚC (cùng UI cho cả relay-websocket và direct-websocket):
{connectionType !== 'relay-ssh' && (
  <div className="space-y-1">
    <label htmlFor="add-ds-ws" className="text-sm font-medium">
      WebSocket URL
    </label>
    <Input
      id="add-ds-ws"
      placeholder="ws://localhost:6799"
      value={wsUrl}
      onChange={(e) => setWsUrl(e.target.value)}
    />
  </div>
)}

// SAU — 3 sections riêng biệt:

{/* relay-websocket: URL input + setup guide */}
{connectionType === 'relay-websocket' && (
  <div className="space-y-3">
    <div className="space-y-1">
      <label htmlFor="add-ds-ws" className="text-sm font-medium">
        Agent WebSocket URL
      </label>
      <Input
        id="add-ds-ws"
        placeholder="ws://172.20.2.31:6799/orca-relay?token=my-secret"
        value={wsUrl}
        onChange={(e) => setWsUrl(e.target.value)}
      />
      <p className="text-xs text-muted-foreground">
        Format:{' '}
        <code className="font-mono">
          ws://host:port/orca-relay?token=&lt;secret&gt;
        </code>
      </p>
    </div>
    <div className="rounded-md border bg-muted/40 p-3 space-y-1">
      <p className="text-xs font-medium text-muted-foreground">
        Start the agent on your dev server:
      </p>
      <code className="block text-xs font-mono select-all">
        AGENT_PORT=6799 AGENT_TOKEN=my-secret node agent.js
      </code>
      <p className="text-xs text-muted-foreground">
        The agent must expose a WebSocket server at{' '}
        <code>/orca-relay</code> before testing.
      </p>
    </div>
  </div>
)}

{/* direct-websocket: Agent connects to Orca — no URL needed */}
{connectionType === 'direct-websocket' && !agentToken && (
  <div className="rounded-md border bg-blue-50 dark:bg-blue-950/30 p-3 space-y-2">
    <p className="text-xs font-medium text-blue-800 dark:text-blue-200">
      Agent connects to Orca — no URL needed
    </p>
    <p className="text-xs text-blue-700 dark:text-blue-300">
      Click <strong>Test Connection</strong> to generate a one-time token.
      Then start your agent with that token.
    </p>
  </div>
)}

{/* direct-websocket: Token panel after test */}
{connectionType === 'direct-websocket' && agentToken && (
  <AgentTokenPanel
    agentToken={agentToken}
    orcaUrl={agentOrcaUrl ?? 'ws://<orca-host>:6768/agent'}
    waiting={state === 'testing'}
  />
)}
```

---

## Acceptance Criteria

- [x] `relay-websocket`: URL input hiển thị với placeholder đúng format `/orca-relay?token=...`
- [x] `relay-websocket`: Setup guide block hiển thị lệnh khởi động agent
- [x] `direct-websocket`: Không có URL input — chỉ có info box
- [x] `direct-websocket` + `agentToken !== null`: `AgentTokenPanel` hiển thị
- [x] `direct-websocket` + `state === 'testing'`: `AgentTokenPanel` với `waiting=true`
- [x] `canTest = true` cho `direct-websocket` (không cần wsUrl)
- [x] `relay-ssh`: Không thay đổi behavior
- [x] TypeScript compile không lỗi
