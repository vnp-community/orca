# TASK-HLD-001: Thêm `getUserRole` lookup + type `UserRoleLookup`, wire vào `server-bootstrap.ts`

**Priority:** 🔴 CRITICAL — prerequisite bảo mật, chặn merge fix BUG-BE-HLD-001/002
**Effort:** ~20 phút
**Status:** ✅ DONE — 2026-08-09 (code áp dụng đúng như solution; đã xác nhận `authManager.userStore` là `readonly` public field, truy cập được từ `server-bootstrap.ts`. `tsc` sẽ đỏ tạm thời cho tới khi TASK-HLD-002/003 xong — đúng như dự kiến.)
**Bug refs:** BUG-BE-HLD-001, BUG-BE-HLD-002
**Solution ref:** [SOLUTION-rbac-exact.md](../solutions/SOLUTION-rbac-exact.md) — Bước 1a, 2a, Bước 3
**Depends on:** None

---

## Mục tiêu

Chưa có nơi nào trong RPC layer tra được org-level role (`'developer'|'lead'|'admin'`) của user đang gọi. Task này:

1. Khai báo type `UserRoleLookup` + import cần thiết ở đầu `profile-rpc-handler.ts` và `project-rpc-handler.ts` (chỉ khai báo type, CHƯA đổi chữ ký factory — việc đó thuộc TASK-HLD-002/003).
2. Wire một implementation thật của `getUserRole` trong `server-bootstrap.ts`, tra `AuthUserStore.getUser()` (đã tồn tại sẵn, không cần thêm method mới trong `AuthUserStore`), và truyền nó vào 2 lời gọi `createProfileMethods(...)` / `createProjectMethods(...)`.

Đây là **prerequisite bắt buộc** cho TASK-HLD-002 và TASK-HLD-003 — 2 task đó sẽ sửa chữ ký factory để nhận tham số `getUserRole` mà bước này truyền vào.

⚠️ **Lưu ý về trạng thái compile tạm thời:** sau khi hoàn thành task này, `pnpm tsc --noEmit` ở `backend/` SẼ BÁO LỖI vì `createProfileMethods`/`createProjectMethods` ở bước này đã được gọi với thêm 1 tham số nhưng chữ ký factory (`profile-rpc-handler.ts`, `project-rpc-handler.ts`) chưa được cập nhật — việc đó thuộc TASK-HLD-002 và TASK-HLD-003. Compile chỉ xanh trở lại sau khi cả 3 task 001+002+003 hoàn tất. Đây là hành vi mong đợi, không phải lỗi.

## File cần sửa/tạo

```
backend/src/main/profile/profile-rpc-handler.ts   (chỉ thêm import + type, không đổi factory signature)
backend/src/main/project/project-rpc-handler.ts   (chỉ thêm import + type, không đổi factory signature)
backend/src/main/server-bootstrap.ts               (wiring thật, dòng 411-426)
```

## Thay đổi cụ thể

### 1. `backend/src/main/profile/profile-rpc-handler.ts` — thêm sau dòng 23 (trước `// ── Shared param schemas`)

```typescript
import { z } from 'zod'
import { defineMethod } from '../runtime/rpc/core'
import type { RpcMethod } from '../runtime/rpc/core'
import type { ProfileService } from './ProfileService'
import type { ProfileResolver } from './ProfileResolver'
import type { OrcaProfile } from './OrcaProfile'
import { Tracers } from '../../shared/trace/tracers'
// FIX BUG-BE-HLD-001: cần tra org-level role thật để requireAdmin không còn là no-op.
import type { OrcaUser } from '../../shared/rbac-types'

/** Tra org-level role của userId. Trong bootstrap thật, trỏ tới AuthUserStore.getUser(). */
export type UserRoleLookup = (userId: string) => Promise<OrcaUser['role'] | null>
```

### 2. `backend/src/main/project/project-rpc-handler.ts` — thêm sau dòng 23 (trước `// ── Param schemas`)

```typescript
import { z } from 'zod'
import { defineMethod } from '../runtime/rpc/core'
import type { RpcMethod } from '../runtime/rpc/core'
import type { ProjectService } from './ProjectService'
import type { ProfileAwareAgentSpawner } from './ProfileAwareAgentSpawner'
import { Tracers } from '../../shared/trace/tracers'
// FIX BUG-BE-HLD-002: cần org-level role để global admin override được project owner-only actions.
import type { OrcaUser } from '../../shared/rbac-types'

/** Tra org-level role của userId — cùng type với profile-rpc-handler.ts (xem SOLUTION-rbac-exact.md). */
export type UserRoleLookup = (userId: string) => Promise<OrcaUser['role'] | null>
```

### 3. `backend/src/main/server-bootstrap.ts` — dòng 411-426, thay:

```typescript
// TRƯỚC (dòng 411-414):
// Register profile RPC methods [T01]
const { createProfileMethods } = await import('./profile/profile-rpc-handler')
rpcServer.addMethods(createProfileMethods(profileService, profileResolver))
console.log('[ServerBootstrap] ✅ Profile RPC methods registered (v5.0)')

// TRƯỚC (dòng 423-426):
// Register project RPC methods [T01]
const { createProjectMethods } = await import('./project/project-rpc-handler')
rpcServer.addMethods(createProjectMethods(projectService))
console.log('[ServerBootstrap] ✅ Project RPC methods registered (v5.0)')
```

thành:

```typescript
// Register profile RPC methods [T01]
const { createProfileMethods } = await import('./profile/profile-rpc-handler')
rpcServer.addMethods(
  createProfileMethods(
    profileService,
    profileResolver,
    // FIX BUG-BE-HLD-001: org-level role lookup, dùng chung 1 nguồn với admin-middleware.ts
    // (session.role) — cả hai đều đọc từ orca_users qua AuthUserStore.
    async (userId) => (await authManager.userStore.getUser(userId))?.role ?? null
  )
)
console.log('[ServerBootstrap] ✅ Profile RPC methods registered (v5.0)')

// Register project RPC methods [T01]
const { createProjectMethods } = await import('./project/project-rpc-handler')
rpcServer.addMethods(
  createProjectMethods(
    projectService,
    // FIX BUG-BE-HLD-002: cùng lookup — cho phép global admin override project owner-only actions.
    async (userId) => (await authManager.userStore.getUser(userId))?.role ?? null
  )
)
console.log('[ServerBootstrap] ✅ Project RPC methods registered (v5.0)')
```

> Ghi chú: `authManager` đã được khởi tạo từ dòng 291 (`const authManager = new AuthManager(authDb)`), **trước** 2 lời gọi này — không cần đổi thứ tự khởi tạo trong file.
>
> Tuỳ chọn (không bắt buộc): để tránh lặp lambda 2 lần, có thể khai báo 1 biến dùng chung ngay trước 2 khối trên:
>
> ```typescript
> const getUserRole = async (userId: string) => (await authManager.userStore.getUser(userId))?.role ?? null
> // ... rồi truyền `getUserRole` vào cả createProfileMethods(...) và createProjectMethods(...)
> ```

## Verification

```bash
# 1. Xác nhận wiring đã có trong server-bootstrap.ts
grep -n "authManager.userStore.getUser" backend/src/main/server-bootstrap.ts
# Expected: 2 dòng khớp (1 trong khối Profile, 1 trong khối Project)

# 2. Xác nhận type UserRoleLookup đã khai báo ở cả 2 handler file
grep -n "UserRoleLookup" backend/src/main/profile/profile-rpc-handler.ts
grep -n "UserRoleLookup" backend/src/main/project/project-rpc-handler.ts
# Expected: mỗi file có ít nhất 1 dòng khai báo `export type UserRoleLookup = ...`

# 3. tsc SẼ báo lỗi tạm thời — đây là hành vi mong đợi cho tới khi TASK-HLD-002/003 hoàn tất
pnpm --filter backend tsc --noEmit || true
# Expected lỗi kiểu: "Expected 2 arguments, but got 3" tại createProfileMethods(...)/createProjectMethods(...)
# trong server-bootstrap.ts — biến mất sau khi TASK-HLD-002 và TASK-HLD-003 xong.
```

Không chạy test suite ở task này (chưa có behavior mới để test — helper chỉ được *dùng thật* từ TASK-HLD-002/003).
