# SOL-FE-AG-001 — relay-websocket: UX cải thiện trong AddDevServerDialog

**CR:** [CR-AG-003](../../../../../docs/crs/v2/agent/CR-AG-003-relay-websocket-mode.md)  
**TDD Refs:** TDD-FE-09 §7 (Dev Server UI Components)  
**Depends on:** SOL-AG-003 (Backend relay-websocket đã implement)  
**Approach:** Additive UI — chỉ thêm hint text và URL format guidance  
**Status:** ✅ IMPLEMENTED (2026-07-26)  

---

## 1. Phân tích hiện trạng

### 1.1 Vấn đề hiện tại trong `AddDevServerDialog.tsx`

```typescript
// Line 300-312: relay-websocket và direct-websocket dùng chung UI input
{connectionType !== 'relay-ssh' && (
  <div className="space-y-1">
    <label htmlFor="add-ds-ws" className="text-sm font-medium">
      WebSocket URL
    </label>
    <Input
      id="add-ds-ws"
      placeholder="ws://localhost:6799"   // ← Placeholder quá generic
      value={wsUrl}
      onChange={(e) => setWsUrl(e.target.value)}
    />
  </div>
)}
```

**Vấn đề:**
1. `relay-websocket` và `direct-websocket` đang dùng **cùng một UI section** — nhưng chúng có flow khác nhau hoàn toàn
2. Placeholder `ws://localhost:6799` không đúng format (cần path `/orca-relay` + `?token=...`)
3. Không có hướng dẫn gì để user biết cách khởi động agent
4. `direct-websocket` không cần nhập URL — agent sẽ connect vào Orca, không phải Orca connect ra

### 1.2 Backend đã implement (SOL-AG-003)

- `connectRelayWebSocket()` nhận URL format: `ws://host:port/orca-relay?token=secret`
- Token được strip khỏi URL, dùng `Authorization: Bearer` header

---

## 2. Giải pháp

### 2.1 Tách UI theo connection type

**Nguyên tắc:**
- `relay-ssh` → SSH host selector (giữ nguyên)
- `relay-websocket` → URL input với hướng dẫn khởi động agent WS server
- `direct-websocket` → Không cần URL input, chỉ cần button Connect (xử lý ở SOL-FE-AG-002)

### 2.2 Sửa `AddDevServerDialog.tsx`

#### Thay đổi 1: Tách section relay-websocket

```tsx
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
        Format: <code>ws://host:port/orca-relay?token=&lt;secret&gt;</code>
      </p>
    </div>

    {/* Setup guide */}
    <div className="rounded-md border bg-muted/40 p-3 space-y-1">
      <p className="text-xs font-medium text-muted-foreground">
        Start the agent on your dev server:
      </p>
      <pre className="text-xs font-mono select-all">
        AGENT_PORT=6799 AGENT_TOKEN=my-secret node agent.js
      </pre>
      <p className="text-xs text-muted-foreground">
        The agent must run a WebSocket server at{' '}
        <code>/orca-relay</code> before you test connection.
      </p>
    </div>
  </div>
)}
```

#### Thay đổi 2: Tách section direct-websocket

```tsx
{/* direct-websocket: Không cần URL — agent sẽ connect vào Orca */}
{connectionType === 'direct-websocket' && (
  <div className="rounded-md border bg-blue-50 dark:bg-blue-950/30 p-3 space-y-2">
    <p className="text-xs font-medium text-blue-800 dark:text-blue-200">
      Agent connects to Orca — no URL needed
    </p>
    <p className="text-xs text-blue-700 dark:text-blue-300">
      Click <strong>Test Connection</strong> to generate a token.
      Your agent will connect to:
    </p>
    <pre className="text-xs font-mono text-blue-800 dark:text-blue-200 select-all">
      {/* Orca server URL — user-facing info */}
      ORCA_URL=ws://&lt;orca-host&gt;:6768/agent
    </pre>
    <p className="text-xs text-blue-600 dark:text-blue-400">
      A unique <code>AGENT_TOKEN</code> will be generated after clicking Test.
    </p>
  </div>
)}
```

#### Thay đổi 3: canTest logic

```typescript
// direct-websocket không cần wsUrl — agent sẽ connect vào Orca
const canTest =
  connectionType === 'relay-ssh'
    ? Boolean(sshTargetId)
    : connectionType === 'relay-websocket'
      ? Boolean(wsUrl)
      : true  // direct-websocket: luôn có thể Test (generates token)
```

---

## 3. Files thay đổi

### [MODIFY] `src/renderer/src/components/dev-server/AddDevServerDialog.tsx`

**Thay đổi:**
- Line ~300-312: Replace single WebSocket section với 2 separate sections
- Line ~143-144: Update `canTest` logic cho direct-websocket

---

## 4. Acceptance Criteria

- [x] `relay-websocket`: Dialog hiển thị URL input với placeholder `ws://172.20.2.31:6799/orca-relay?token=...`
- [x] `relay-websocket`: Có setup guide block: lệnh khởi động agent
- [x] `direct-websocket`: Không có URL input — chỉ hiển thị info box
- [x] `direct-websocket`: `canTest = true` (không phụ thuộc vào wsUrl)
- [x] `relay-ssh`: Không thay đổi behavior
- [x] TypeScript compile không lỗi
