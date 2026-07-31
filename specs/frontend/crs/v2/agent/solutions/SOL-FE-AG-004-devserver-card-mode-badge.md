# SOL-FE-AG-004 — DevServerCard: Connection Type Badge & Mode Info

**CR:** [CR-AG-003](../../../../../docs/crs/v2/agent/CR-AG-003-relay-websocket-mode.md), [CR-AG-004](../../../../../docs/crs/v2/agent/CR-AG-004-direct-websocket-mode.md)  
**TDD Refs:** TDD-FE-09 §7 (Dev Server UI Components)  
**Depends on:** Backend solutions implemented (SOL-AG-003, SOL-AG-004)  
**Approach:** Additive UI enhancement  
**Status:** ✅ IMPLEMENTED (2026-07-26)  

---

## 1. Phân tích hiện trạng

### 1.1 `DevServerCard.tsx` hiện tại

```tsx
// src/renderer/src/components/dev-server/DevServerCard.tsx
// DevServerCard hiển thị: name, status badge, connect/disconnect/remove buttons
// CHƯA có: connection type indicator
```

User không biết `DevServer` nào đang dùng SSH, relay-websocket, hay direct-websocket.

### 1.2 `DevServer` type đã có `connectionType`

```typescript
// src/shared/dev-server-types.ts
export type DevServer = {
  id: string
  name: string
  connectionType: DevServerConnectionType  // ← đã có
  // ...
}
```

---

## 2. Giải pháp

### 2.1 Connection Type Badge Component

```tsx
// Thêm inline trong DevServerCard.tsx hoặc tách ra file riêng

type ConnectionTypeBadgeProps = {
  connectionType: DevServerConnectionType
}

const CONNECTION_TYPE_CONFIG = {
  'relay-ssh': {
    label: 'SSH',
    icon: '🔐',
    className: 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300',
  },
  'relay-websocket': {
    label: 'WS →',
    icon: '🌐',
    className: 'bg-orange-100 text-orange-700 dark:bg-orange-900 dark:text-orange-300',
    title: 'Orca connects to agent WebSocket server',
  },
  'direct-websocket': {
    label: '← WS',
    icon: '🔗',
    className: 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300',
    title: 'Agent connects to Orca WebSocket server',
  },
} as const

function ConnectionTypeBadge({ connectionType }: ConnectionTypeBadgeProps) {
  const config = CONNECTION_TYPE_CONFIG[connectionType]
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${config.className}`}
      title={'title' in config ? config.title : undefined}
    >
      {config.icon} {config.label}
    </span>
  )
}
```

### 2.2 Integrate vào `DevServerCard.tsx`

```tsx
// src/renderer/src/components/dev-server/DevServerCard.tsx

export function DevServerCard({ server, isActive, onConnect, onDisconnect, onRemove, onSelect }: DevServerCardProps) {
  return (
    <div className={`...`}>
      <div className="flex items-center gap-2">
        {/* Server name */}
        <span className="font-medium text-sm">{server.name}</span>

        {/* Connection type badge — NEW */}
        <ConnectionTypeBadge connectionType={server.connectionType} />

        {/* Status badge */}
        <DevServerStatusBadge status={server.status} platform={server.platform} />
      </div>

      {/* Connection info — hiển thị URL cho relay-websocket */}
      {server.connectionType === 'relay-websocket' && server.wsUrl && (
        <p className="text-xs text-muted-foreground mt-1 truncate" title={server.wsUrl}>
          {server.wsUrl}
        </p>
      )}

      {/* direct-websocket: hiển thị note khi disconnected */}
      {server.connectionType === 'direct-websocket' && server.status === 'disconnected' && (
        <p className="text-xs text-muted-foreground mt-1">
          Agent connects to Orca — use Connect to generate a new token
        </p>
      )}

      {/* ... existing buttons ... */}
    </div>
  )
}
```

---

## 3. Design decisions

### 3.1 Badge labels

| Mode | Label | Mũi tên | Lý do |
|------|-------|---------|-------|
| relay-ssh | `SSH 🔐` | — | SSH relay, familiar |
| relay-websocket | `WS → 🌐` | → Orca connects OUT | Orca là client |
| direct-websocket | `← WS 🔗` | ← Agent connects IN | Agent là client |

### 3.2 Màu sắc

- `relay-ssh`: Slate — neutral, stable, SSH is default
- `relay-websocket`: Orange — "active outbound" feeling
- `direct-websocket`: Blue — "inbound waiting" feeling

---

## 4. Files thay đổi

### [MODIFY] `src/renderer/src/components/dev-server/DevServerCard.tsx`
- Thêm `ConnectionTypeBadge` component inline
- Render badge sau server name
- Hiển thị wsUrl cho relay-websocket
- Hiển thị note cho direct-websocket khi disconnected

---

## 5. Acceptance Criteria

- [x] `DevServerCard` hiển thị badge `SSH 🔐` cho relay-ssh servers
- [x] `DevServerCard` hiển thị badge `WS → 🌐` cho relay-websocket servers
- [x] `DevServerCard` hiển thị badge `← WS 🔗` cho direct-websocket servers
- [x] relay-websocket card hiển thị wsUrl (truncated nếu dài)
- [x] direct-websocket card hiển thị note khi disconnected
- [x] Badge không làm vỡ layout khi server name dài
- [x] TypeScript compile không lỗi
- [x] Dark mode: badge màu sắc đúng
