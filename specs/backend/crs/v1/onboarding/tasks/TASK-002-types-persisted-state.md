# TASK-002: Cập nhật `src/shared/types.ts` — PersistedState + GlobalSettings

**Phase:** 1 — Foundation  
**Solution:** [SOL-002](../solutions/SOL-002-dev-server-manager.md) §3  
**Depends on:** TASK-001 (import PersistedDevServer)  
**Blocks:** TASK-003, TASK-009

---

## Mục tiêu

Thêm các fields mới vào `PersistedState` và `GlobalSettings` trong `src/shared/types.ts` để hỗ trợ DevServer persistence.

---

## File cần sửa

**Path:** `src/shared/types.ts`

---

## Thay đổi cần thực hiện

### 1. Import `PersistedDevServer`

Thêm import ở đầu file:

```typescript
import type { PersistedDevServer } from './dev-server-types'
```

### 2. Thêm vào `PersistedState`

```typescript
type PersistedState = {
  // ... existing fields giữ nguyên ...
  devServers: PersistedDevServer[]        // NEW — mảng DevServer đã persist
}
```

### 3. Thêm vào `GlobalSettings`

```typescript
type GlobalSettings = {
  // ... existing fields giữ nguyên ...
  activeDevServerId?: string | null       // NEW — DevServer đang được chọn
}
```

---

## Acceptance Criteria

- [x] `PersistedState` có field `devServers: PersistedDevServer[]`
- [x] `GlobalSettings` có field `activeDevServerId?: string | null`
- [x] Import `PersistedDevServer` từ `./dev-server-types` (không inline lại type)
- [x] Các fields hiện có KHÔNG bị thay đổi
- [x] TypeScript compile thành công: `tsc --noEmit`
