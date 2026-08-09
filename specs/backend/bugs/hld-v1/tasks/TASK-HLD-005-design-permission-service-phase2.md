# TASK-HLD-005: Thiết kế `PermissionService.hasPermission()` thống nhất (phase 2)

**Priority:** 🟢 LOW — follow-up / optional, KHÔNG chặn release fix bảo mật (TASK-HLD-001..004)
**Effort:** ~2-3 giờ (thiết kế + PR review riêng; không phải mini-fix)
**Status:** ✅ DONE (Bước A only) — 2026-08-09 (tạo file mới `backend/src/main/permissions/PermissionService.ts` đúng nguyên văn Bước A của solution — thuần additive, KHÔNG sửa/wire vào bất kỳ call site nào hiện có (`admin-middleware.ts`, `profile-rpc-handler.ts`, `project-rpc-handler.ts` giữ nguyên 100%, đúng khuyến nghị "chỉ Bước A trong PR đầu tiên"). Xác nhận trước khi viết: `OrcaUser` export tại `rbac-types.ts:20`; `AuthUserStore.getUser()` trả `OrcaSessionUser | null` với `OrcaSessionUser = Pick<OrcaUser, 'id'|'email'|'name'|'role'|'provider'>` (`auth-types.ts:13`) — tương thích hoàn toàn với `Pick<AuthUserStore, 'getUser'>` và `user?.role` như solution dùng. `tsc --noEmit` sạch hoàn toàn, 0 lỗi. Bước B (migrate `admin-middleware.ts`) và Bước C (migrate `profile-rpc-handler.ts`/`project-rpc-handler.ts` sang gọi `PermissionService`) **CHƯA làm** — đúng theo khuyến nghị của chính solution là PR riêng, cần review Bước A trước vì đổi cách `admin-router.ts` đăng ký route (blast radius rộng hơn). ⚠️ Chưa viết `PermissionService.test.ts` — effort budget.)
**Bug refs:** BUG-BE-HLD-003
**Solution ref:** [SOLUTION-rbac-exact.md](../solutions/SOLUTION-rbac-exact.md) — mục "BUG-BE-HLD-003: PermissionService.hasPermission() thống nhất — thiết kế cho phase 2"
**Depends on:** TASK-HLD-001, TASK-HLD-002, TASK-HLD-003 (nên chạy ổn định trong production trước khi bắt đầu task này)

---

## ⚠️ Đây là follow-up, KHÔNG khẩn cấp

Ticket BUG-BE-HLD-003 tự nêu rõ: *"Ưu tiên fix trước 2 lỗ hổng cụ thể BUG-BE-HLD-001 và BUG-BE-HLD-002 trước khi làm refactor lớn."* Task này chỉ nên bắt đầu **sau khi** TASK-HLD-001..004 đã merge và chạy ổn định. KHÔNG gộp vào cùng PR với các fix bảo mật khẩn cấp — đây là refactor có blast radius rộng hơn (đụng route registration ở `admin-router.ts`), rủi ro không tương xứng nếu làm chung.

## Mục tiêu

RBAC hiện bị phân mảnh thành 4 cơ chế khác nhau:

1. `resolveUserPermissions()` (`rbac-types.ts`) — phục vụ fleet/server access, GIỮ NGUYÊN (không có khái niệm resource/action, phạm vi khác — server pairing token).
2. `requireAdmin` (`admin-middleware.ts`, HTTP route).
3. `requireAdmin` (`profile-rpc-handler.ts`, RPC — đã fix ở TASK-HLD-002, dùng `getUserRole` trực tiếp).
4. `requireOwnerOrAdmin` (`project-rpc-handler.ts`, đã fix ở TASK-HLD-003).

Task này tạo một `PermissionService` trung tâm để 2 và 3+4 hội tụ về cùng 1 điểm quyết định RBAC, thay vì mỗi nơi tự implement lại logic role check. `TaskGrantService.resolvePermission()` GIỮ NGUYÊN (đặc thù cho BFS ancestor + apply_tree của task graph), nhưng có thể expose thêm 1 adapter mỏng qua cùng interface `hasPermission` sau này (không bắt buộc trong task này).

## File cần sửa/tạo

```
backend/src/main/permissions/PermissionService.ts   (MỚI)
backend/src/main/admin/admin-middleware.ts            (follow-up Bước B — optional trong task này)
backend/src/main/profile/profile-rpc-handler.ts        (follow-up Bước C — optional trong task này)
backend/src/main/project/project-rpc-handler.ts        (follow-up Bước C — optional trong task này)
```

> Khuyến nghị: chỉ tạo `PermissionService.ts` (Bước A) trong PR đầu tiên của task này. Bước B (migrate `admin-middleware.ts`) và Bước C (migrate 2 RPC handler) nên là PR riêng kế tiếp, sau khi Bước A đã review xong — vì Bước B đổi cách `admin-router.ts` đăng ký route (`createRequireAdmin(permissions)` thay vì import trực tiếp `requireAdmin`).

## Thay đổi cụ thể

### Bước A (bắt buộc trong task này) — file mới `backend/src/main/permissions/PermissionService.ts`

```typescript
/**
 * PermissionService — Role → resource → action policy table trung tâm (BUG-BE-HLD-003).
 *
 * Thay thế dần 4 cơ chế RBAC phân mảnh:
 *   1. resolveUserPermissions() (rbac-types.ts) — giữ nguyên, phục vụ fleet/server access,
 *      không có khái niệm resource/action nên KHÔNG migrate (đúng theo phạm vi khác — server
 *      pairing token, không phải RPC permission check).
 *   2. requireAdmin (admin-middleware.ts, HTTP route)      → migrate ở Bước B.
 *   3. requireAdmin (profile-rpc-handler.ts, RPC — đã fix ở TASK-HLD-002 dùng getUserRole trực tiếp)
 *      → migrate sang PermissionService.hasPermission() ở Bước C.
 *   4. requireOwnerOrAdmin (project-rpc-handler.ts, đã fix ở TASK-HLD-003) → migrate ở Bước C.
 *   TaskGrantService.resolvePermission() — GIỮ NGUYÊN, đặc thù cho BFS ancestor + apply_tree
 *   của task graph (đúng như ticket 003 đề xuất), nhưng expose thêm 1 adapter mỏng qua cùng
 *   interface `hasPermission` để nơi gọi không cần biết đây là task graph hay resource thường.
 */
import type { OrcaUser } from '../../shared/rbac-types'
import type { AuthUserStore } from '../auth/auth-user-store'

export type PermissionResource = 'company' | 'department' | 'project' | 'task'
export type PermissionAction = 'read' | 'write' | 'create' | 'delete' | 'admin'

export interface PermissionContext {
  /** Role trong phạm vi resource (vd ProjectMemberRole 'owner'|'member'|'viewer'). */
  resourceRole?: string
}

export class PermissionService {
  constructor(private readonly userStore: Pick<AuthUserStore, 'getUser'>) {}

  async getGlobalRole(userId: string): Promise<OrcaUser['role'] | null> {
    const user = await this.userStore.getUser(userId)
    return user?.role ?? null
  }

  /** Điểm quyết định RBAC duy nhất — mọi domain (Admin/Profile/Project/...) gọi qua đây. */
  async hasPermission(
    userId: string,
    resource: PermissionResource,
    action: PermissionAction,
    context?: PermissionContext
  ): Promise<boolean> {
    const globalRole = await this.getGlobalRole(userId)
    if (!globalRole) return false
    if (globalRole === 'admin') return true // global admin override mọi resource (F32)

    switch (resource) {
      case 'company':
      case 'department':
        // Company/dept mutation là admin-only bất kể resourceRole (F32/F33 security lock).
        return action === 'read'
      case 'project':
        if (action === 'read') return true // membership đã được assertAccess() check trước
        return context?.resourceRole === 'owner'
      default:
        return false
    }
  }
}
```

### Bước B (optional, PR riêng) — migrate `admin-middleware.ts` sang `PermissionService`

```typescript
// backend/src/main/admin/admin-middleware.ts
export function createRequireAdmin(permissions: PermissionService) {
  return async function requireAdmin(req: Request, res: Response, next: NextFunction): Promise<void> {
    const session = req.orcaSession
    if (!session) {
      res.status(401).json({ error: 'unauthenticated', message: 'Login required' })
      return
    }
    const ok = await permissions.hasPermission(session.userId, 'company', 'admin')
    if (!ok) {
      res.status(403).json({
        error: 'forbidden', message: 'Admin role required',
        required_role: 'admin', your_role: session.role
      })
      return
    }
    next()
  }
}
```

Lưu ý: route registration ở `admin-router.ts` phải đổi từ import trực tiếp `requireAdmin` sang gọi `createRequireAdmin(permissions)` — kiểm tra tất cả nơi đăng ký route admin trước khi áp dụng.

### Bước C (optional, PR riêng, sau Bước B) — migrate `profile-rpc-handler.ts`/`project-rpc-handler.ts` sang `PermissionService`

Thay `getUserRole: UserRoleLookup` bằng `permissions: PermissionService`:

```typescript
// profile-rpc-handler.ts
async function requireAdmin(ctx: { userId?: string }, permissions: PermissionService): Promise<void> {
  if (!ctx.userId) throw new Error('UNAUTHENTICATED')
  const ok = await permissions.hasPermission(ctx.userId, 'company', 'admin')
  if (!ok) throw new Error('FORBIDDEN: admin role required')
}

// project-rpc-handler.ts
async function requireOwnerOrAdmin(
  role: ProjectRole,
  userId: string,
  permissions: PermissionService
): Promise<void> {
  const ok = await permissions.hasPermission(userId, 'project', 'write', { resourceRole: role })
  if (!ok) throw new Error('FORBIDDEN: only project owners or global admins can perform this action')
}
```

`UserRoleLookup` (TASK-HLD-001/002/003) và `PermissionService.getGlobalRole` có cùng chữ ký `(userId) => Promise<Role | null>` — migrate này chỉ đổi loại tham số truyền vào factory, KHÔNG đổi behavior đã fix ở TASK-HLD-002/003. Vì vậy bản vá khẩn cấp ở 001-003 là **tập con đúng** của thiết kế `PermissionService` cuối cùng, an toàn để làm trước.

## Verification

```bash
# Bước A — chỉ file mới, không đụng code hiện có
pnpm --filter backend tsc --noEmit

# Unit test gợi ý cho PermissionService.ts (viết mới, chưa tồn tại — tạo tại
# backend/src/main/permissions/__tests__/PermissionService.test.ts):
#   - getGlobalRole: userId không tồn tại → null
#   - hasPermission: globalRole 'admin' → true cho MỌI resource/action (override)
#   - hasPermission: resource 'company'/'department', action != 'read', globalRole != 'admin' → false
#   - hasPermission: resource 'project', action 'write', resourceRole 'owner' → true
#   - hasPermission: resource 'project', action 'write', resourceRole 'member'/'viewer' → false
pnpm --filter backend test -- PermissionService

# Nếu làm Bước B/C trong task này (không bắt buộc):
grep -rn "requireAdmin\b" backend/src/main/admin/admin-router.ts
# Xác nhận route registration đã đổi sang createRequireAdmin(permissions)

pnpm --filter backend test -- admin-middleware profile-rpc project-rpc

# GitNexus regression check trước khi commit (theo AGENTS.md)
# — đặc biệt quan trọng ở Bước B vì blast radius rộng hơn (route registration)
```
