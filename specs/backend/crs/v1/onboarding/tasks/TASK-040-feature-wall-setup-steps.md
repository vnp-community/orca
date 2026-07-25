# TASK-040: Sửa `src/shared/feature-wall-setup-steps.ts` — Thêm Dev Server Steps + Priority Logic

**Phase:** 3 — Multi Dev-Server Checklist  
**Solution:** [SOL-007-008-009](../solutions/SOL-007-008-009-windows-notifications-checklist.md) §C.4  
**Depends on:** TASK-037  
**Blocks:** TASK-041

---

## Mục tiêu

Thêm 2 setup steps mới (`connect-dev-server`, `add-dev-server-repo`) và cập nhật priority logic để dev server steps luôn ưu tiên đầu tiên.

---

## File cần sửa

**Path:** `src/shared/feature-wall-setup-steps.ts` (hoặc tên tương đương trong codebase)

---

## Context cần tra cứu

Grep `FeatureWallSetupStepId` trong `src/shared/` để tìm đúng file.

---

## Thay đổi cần thực hiện

### 1. Mở rộng `FeatureWallSetupStepId` type

```typescript
export type FeatureWallSetupStepId =
  | 'default-agent'
  | 'add-two-repos'
  | 'notifications'
  | 'two-worktrees'
  | 'browser'
  | 'task-sources'
  | 'agent-capabilities'
  | 'setup-script'
  | 'connect-dev-server'     // NEW
  | 'add-dev-server-repo'    // NEW
```

### 2. Thêm completion check functions

```typescript
import type { DevServer } from './dev-server-types'
import type { Repo } from './types'

export function isConnectDevServerComplete(devServers: DevServer[]): boolean {
  return devServers.some(ds => ds.status === 'connected')
}

export function isAddDevServerRepoComplete(
  repos: Repo[],
  activeDevServerId: string | null
): boolean {
  if (!activeDevServerId) return false
  return repos.some(r => r.devServerId === activeDevServerId)
}
```

### 3. Cập nhật `getFirstIncompleteFeatureWallSetupStepId()` (hoặc tương đương)

```typescript
export function getFirstIncompleteFeatureWallSetupStepId(
  steps: Record<FeatureWallSetupStepId, boolean>,
  devServers: DevServer[],
  repos: Repo[]
): FeatureWallSetupStepId | null {
  // Override: nếu chưa có dev server connected → ưu tiên tuyệt đối
  if (!isConnectDevServerComplete(devServers)) {
    return 'connect-dev-server'
  }

  const ORDER: FeatureWallSetupStepId[] = [
    'connect-dev-server',
    'add-dev-server-repo',
    'default-agent',
    'agent-capabilities',
    'task-sources',
    'add-two-repos',
    'setup-script',
    'notifications',
    'two-worktrees',
    'browser'
  ]
  return ORDER.find(id => !steps[id]) ?? null
}
```

---

## Acceptance Criteria

- [x] `FeatureWallSetupStepId` có `'connect-dev-server'` và `'add-dev-server-repo'`
- [x] `isConnectDevServerComplete()` trả về `true` khi có ít nhất 1 server `status === 'connected'`
- [x] `isAddDevServerRepoComplete()` trả về `true` khi có repo với đúng `devServerId`
- [x] `getFirstIncompleteFeatureWallSetupStepId()`: không có server → `'connect-dev-server'`
- [x] `getFirstIncompleteFeatureWallSetupStepId()`: có server, chưa có repo → `'add-dev-server-repo'`
- [x] `getFirstIncompleteFeatureWallSetupStepId()`: tất cả done → `null`
- [x] TypeScript compile thành công
- [x] Không breaking: các step hiện có vẫn hoạt động đúng

---

## Lưu ý cho AI

1. Nếu `getFirstIncompleteFeatureWallSetupStepId` chưa tồn tại, tìm hàm tương đương
2. Signature hiện tại của hàm có thể khác — cập nhật signature cũng cần update tất cả callers
3. Cẩn thận với ORDER array — 'connect-dev-server' phải trước 'add-dev-server-repo'
