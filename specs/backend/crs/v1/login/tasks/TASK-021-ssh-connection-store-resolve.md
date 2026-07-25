# TASK-021: Sửa `src/main/ssh/ssh-connection-store.ts` — Thêm `resolveSshTargetForUser()`

> **Status:** ✅ DONE (2026-07-24)


**Phase:** 3 — SSH Isolation
**Solution:** [SOL-LG-003](../solutions/SOL-LG-003-ssh-isolation.md) §4.4
**Depends on:** TASK-019
**Blocks:** (Phase 3 complete)

---

## Mục tiêu

Thêm method `resolveSshTargetForUser()` vào `SshConnectionStore` để lấy SshTarget với username override per user.

---

## File cần sửa

**Path:** `src/main/ssh/ssh-connection-store.ts`

---

## Thay đổi cần thực hiện

### 1. Thêm import

```typescript
import { resolveUserSshTarget } from './ssh-user-resolver'
import type { OrcaSession } from '../auth/auth-types'
```

### 2. Thêm method vào class `SshConnectionStore`

```typescript
/**
 * Resolve SshTarget với username override cho user cụ thể.
 *
 * Trong ORCA_MULTI_USER=1 mode, mỗi user SSH vào dev server
 * với unix account riêng (orca-alice) thay vì shared 'ubuntu'.
 *
 * @param targetId  - SshTarget ID
 * @param session   - OrcaSession của user hiện tại
 * @returns SshTarget với username = orca-{user}, hoặc undefined nếu target không tồn tại
 */
resolveSshTargetForUser(targetId: string, session: OrcaSession): SshTarget | undefined {
  const base = this.getTarget(targetId)
  if (!base) return undefined
  return resolveUserSshTarget(base, session.userId, session.userEmail)
}
```

---

## Lưu ý khi apply

- Chỉ thêm method, không sửa existing methods
- `getTarget(id)` là method đã có trong `SshConnectionStore`
- `SshTarget` type đã có từ `src/shared/ssh-types.ts`
- Nếu `OrcaSession` chưa có trong shared types, import từ `../auth/auth-types`

---

## Acceptance Criteria

- [x] Method `resolveSshTargetForUser(targetId, session)` tồn tại trong class
- [x] Trả về SshTarget với `username = orca-{userEmail_local}`
- [x] Trả về `undefined` khi target không tồn tại
- [x] Không sửa bất kỳ existing method nào
- [x] TypeScript compile sạch
