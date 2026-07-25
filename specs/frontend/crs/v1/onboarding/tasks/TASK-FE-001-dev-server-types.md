# TASK-FE-001: Tạo shared dev-server-types.ts

> **Status: ✅ COMPLETED** — 2026-07-23
> **Files modified:**
> - `src/shared/dev-server-types.ts` — thêm `RemotePreflightStatus`, `PerServerChecklistState`, `WindowsTerminalCapabilities`

**Phase:** 1 | **Solution:** [FE-SOL-A](../solutions/FE-SOL-A-dev-server-ui.md) | **CR:** CR-OB-002  
**Depends on:** _(none — base task)_  
**Estimated effort:** ~30 phút

---

## Context

Đọc trước:
- [`src/shared/types.ts`](../../../../../src/shared/types.ts) — xem cấu trúc hiện tại của shared types
- [`../solutions/FE-SOL-A-dev-server-ui.md`](../solutions/FE-SOL-A-dev-server-ui.md) — Section 2 (Zustand slice) để xem type definitions

---

## Goal

Tạo file `src/shared/dev-server-types.ts` chứa tất cả TypeScript types liên quan đến DevServer, để cả frontend lẫn backend có thể import.

---

## Steps

1. **Tạo file** `src/shared/dev-server-types.ts` với nội dung:

```typescript
export type DevServerConnectionType =
  | 'relay-ssh'
  | 'relay-websocket'
  | 'direct-websocket'

export type DevServerStatus =
  | 'connected'
  | 'disconnected'
  | 'connecting'
  | 'error'

export type DevServer = {
  id: string
  name: string
  connectionType: DevServerConnectionType
  sshTargetId?: string
  wsUrl?: string
  status: DevServerStatus
  platform: NodeJS.Platform | null
  arch: string | null
  nodeVersion: string | null
  lastConnectedAt: number | null
  lastError: string | null
  workspaceDir: string | null
  addedAt: number
}

export type DevServerInput = {
  name: string
  connectionType: DevServerConnectionType
  sshTargetId?: string
  wsUrl?: string
}

export type ConnectionTestResult =
  | { ok: true; platform: NodeJS.Platform; nodeVersion: string; arch?: string }
  | { ok: false; error: string; hint?: string }

export type RemotePreflightStatus = {
  devServerId: string
  platform: NodeJS.Platform
  checkedAt: number
  gh: {
    installed: boolean
    authenticated: boolean
    version?: string
  }
  git: {
    installed: boolean
    version?: string
    hasUserName: boolean
    hasUserEmail: boolean
  }
}

export type PerServerChecklistState = {
  addedRepo?: boolean
  ranFirstAgent?: boolean
  ranSecondAgentOnSameTask?: boolean
  reviewedDiff?: boolean
  openedPr?: boolean
  addedFolder?: boolean
  openedFile?: boolean
  ranAgentOnFile?: boolean
}
```

2. **Sửa** `src/shared/types.ts` — thêm import và extend `OnboardingChecklistState`:

```typescript
// Thêm import:
import type { PerServerChecklistState } from './dev-server-types'

// Tìm OnboardingChecklistState và thêm perServer field:
type OnboardingChecklistState = {
  // ...existing fields...
  perServer?: Record<string, PerServerChecklistState>   // NEW
}
```

3. **Verify**: Chạy TypeScript check:
```bash
pnpm tsc --noEmit
```

---

## Acceptance Criteria

- [ ] `src/shared/dev-server-types.ts` tồn tại và export đủ types
- [ ] `DevServer.status` có đủ 4 giá trị
- [ ] `ConnectionTestResult` là discriminated union (`ok: true/false`)
- [ ] `OnboardingChecklistState.perServer` được thêm vào
- [ ] `pnpm tsc --noEmit` không có lỗi mới

## Output Files

- **[NEW]** `src/shared/dev-server-types.ts`
- **[MODIFY]** `src/shared/types.ts`
