# TASK-001: Tạo file `src/shared/dev-server-types.ts`

**Phase:** 1 — Foundation  
**Solution:** [SOL-002](../solutions/SOL-002-dev-server-manager.md) §2  
**Depends on:** (không có)  
**Blocks:** TASK-002, TASK-003, TASK-004, TASK-005

---

## Mục tiêu

Tạo mới file `src/shared/dev-server-types.ts` chứa toàn bộ TypeScript types cho DevServer subsystem.

---

## File cần tạo

**Path:** `src/shared/dev-server-types.ts`

---

## Nội dung cần implement

```typescript
export type DevServerConnectionType =
  | 'relay-ssh'         // Orca SSH → deploy relay → stdin/stdout
  | 'relay-websocket'   // Dev server connects WS → Orca (reverse)
  | 'direct-websocket'  // Orca connects WS → dev server relay

export type DevServerStatus =
  | 'connected'
  | 'disconnected'
  | 'connecting'
  | 'error'

export type DevServer = {
  id: string                          // 'ds-<uuid>'
  name: string                        // Human label: "MacBook Pro M3"
  connectionType: DevServerConnectionType
  // relay-ssh specific:
  sshTargetId?: string                // Links to existing SshTarget
  // relay-websocket / direct-websocket specific:
  wsUrl?: string                      // ws://devserver.local:6799
  // Runtime (không persist):
  status: DevServerStatus
  platform: NodeJS.Platform | null    // Populated after handshake
  arch: string | null                 // 'arm64' | 'x64'
  nodeVersion: string | null
  lastConnectedAt: number | null
  lastError: string | null
  workspaceDir: string | null         // Remote default workspace directory
  addedAt: number
}

export type DevServerInput = {
  name: string
  connectionType: DevServerConnectionType
  sshTargetId?: string
  wsUrl?: string
}

export type ConnectionTestResult =
  | { ok: true; platform: NodeJS.Platform; nodeVersion: string }
  | { ok: false; error: string; hint?: string }

// Persisted subset của DevServer (không persist runtime fields):
export type PersistedDevServer = {
  id: string
  name: string
  connectionType: DevServerConnectionType
  sshTargetId?: string
  wsUrl?: string
  workspaceDir: string | null
  addedAt: number
}
```

> **Lưu ý:** `PersistedDevServer` được export từ file này (không khai báo inline trong types.ts) để tránh circular import.

---

## Acceptance Criteria

- [x] File tồn tại tại `src/shared/dev-server-types.ts`
- [x] Export đầy đủ: `DevServerConnectionType`, `DevServerStatus`, `DevServer`, `DevServerInput`, `ConnectionTestResult`, `PersistedDevServer`
- [x] Không có import từ các module khác trong project (pure type file)
- [x] TypeScript compile thành công: `tsc --noEmit` không báo lỗi liên quan file này
