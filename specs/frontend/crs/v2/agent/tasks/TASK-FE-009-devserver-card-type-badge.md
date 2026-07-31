# TASK-FE-009 — `DevServerCard.tsx`: Connection Type Badge

**Solution:** [SOL-FE-AG-004](../solutions/SOL-FE-AG-004-devserver-card-mode-badge.md)  
**File:** `src/renderer/src/components/dev-server/DevServerCard.tsx` [MODIFY]  
**Depends on:** Không có dependency (standalone)  
**Status:** ✅ DONE (2026-07-26)  

---

## Mục tiêu

Thêm `ConnectionTypeBadge` inline trong `DevServerCard` để user thấy rõ từng server đang dùng connection mode nào: `SSH 🔐`, `WS → 🌐` (relay-websocket), hoặc `← WS 🔗` (direct-websocket).

---

## Code hiện tại

```tsx
// src/renderer/src/components/dev-server/DevServerCard.tsx
// Hiện tại hiển thị: name, DevServerStatusBadge, connect/disconnect/remove buttons
// CHƯA có: connection type indicator
```

---

## Thay đổi cần thực hiện

### File: `src/renderer/src/components/dev-server/DevServerCard.tsx`

**Thêm `ConnectionTypeBadge` component và config trước `DevServerCard`:**

```tsx
// ─── Connection type config ────────────────────────────────────────────────────

const CONNECTION_TYPE_CONFIG = {
  'relay-ssh': {
    label: 'SSH',
    emoji: '🔐',
    className:
      'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300',
    title: 'SSH Relay — Orca connects via SSH',
  },
  'relay-websocket': {
    label: 'WS →',
    emoji: '🌐',
    className:
      'bg-orange-100 text-orange-700 dark:bg-orange-900/60 dark:text-orange-300',
    title: 'WebSocket Relay — Orca connects to agent WS server',
  },
  'direct-websocket': {
    label: '← WS',
    emoji: '🔗',
    className:
      'bg-blue-100 text-blue-700 dark:bg-blue-900/60 dark:text-blue-300',
    title: 'Direct WebSocket — Agent connects into Orca',
  },
} as const satisfies Record<
  DevServerConnectionType,
  { label: string; emoji: string; className: string; title: string }
>

function ConnectionTypeBadge({
  connectionType,
}: {
  connectionType: DevServerConnectionType
}) {
  const cfg = CONNECTION_TYPE_CONFIG[connectionType]
  return (
    <span
      className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${cfg.className}`}
      title={cfg.title}
    >
      {cfg.emoji} {cfg.label}
    </span>
  )
}
```

**Sửa `DevServerCard` render — thêm badge vào header row:**

```tsx
export function DevServerCard({ server, isActive, onConnect, onDisconnect, onRemove, onSelect }: DevServerCardProps) {
  return (
    <div className={`...existing classes...`}>
      {/* Header: name + type badge + status */}
      <div className="flex items-center gap-2 min-w-0">
        <span className="font-medium text-sm truncate">{server.name}</span>

        {/* Connection type badge — NEW */}
        <ConnectionTypeBadge connectionType={server.connectionType} />

        <DevServerStatusBadge status={server.status} platform={server.platform} />
      </div>

      {/* URL info for relay-websocket — NEW */}
      {server.connectionType === 'relay-websocket' && server.wsUrl && (
        <p
          className="text-xs text-muted-foreground mt-0.5 truncate"
          title={server.wsUrl}
        >
          {server.wsUrl}
        </p>
      )}

      {/* Guidance for direct-websocket when disconnected — NEW */}
      {server.connectionType === 'direct-websocket' &&
        server.status === 'disconnected' && (
          <p className="text-xs text-muted-foreground mt-0.5">
            Agent connects to Orca — click Connect to generate a token
          </p>
        )}

      {/* ... existing buttons unchanged ... */}
    </div>
  )
}
```

**Thêm import** (nếu chưa có):
```typescript
import type { DevServerConnectionType } from '../../../../shared/dev-server-types'
```

---

## Acceptance Criteria

- [x] `relay-ssh` server card: badge `🔐 SSH` (slate colors)
- [x] `relay-websocket` server card: badge `🌐 WS →` (orange colors)
- [x] `direct-websocket` server card: badge `🔗 ← WS` (blue colors)
- [x] `relay-websocket` card: `wsUrl` hiển thị dưới tên server (truncated)
- [x] `direct-websocket` card khi `status === 'disconnected'`: guidance text
- [x] Badge `title` tooltip giải thích connection type
- [x] Dark mode: màu sắc badges đúng
- [x] Layout không bị vỡ khi tên server dài (dùng `truncate`)
- [x] TypeScript compile không lỗi
