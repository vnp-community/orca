# SOLUTION: RBAC (hld-v1) — Code-Level Exact Fixes

**Source-verified:** ✅ Dựa trên source code thực tế (đọc qua `codegraph_explore`, ngày 2026-08-09)
**Files nguồn đã đọc:**
- `backend/src/main/profile/profile-rpc-handler.ts` (293 dòng, đọc toàn bộ)
- `backend/src/main/project/project-rpc-handler.ts` (253 dòng, đọc toàn bộ)
- `backend/src/shared/rbac-types.ts`
- `backend/src/main/admin/admin-middleware.ts`
- `backend/src/main/auth/auth-types.ts`, `backend/src/main/auth/auth-manager.ts`, `backend/src/main/auth/auth-user-store.ts`
- `backend/src/main/runtime/rpc/core.ts` (`RpcContext`, `defineMethod`)
- `backend/src/main/runtime/rpc/dispatcher.ts` (`RpcDispatcher.dispatch` / `dispatchStreaming`)
- `backend/src/main/session/ws-session-router.ts` (`WsSessionRouter.handleConnection`)
- `backend/src/main/server-bootstrap.ts` (`initializeOrcaServices`, wiring `createProfileMethods`/`createProjectMethods`)
- Đối chiếu: `specs/backend/tdd/v4/05-auth-layer.md`, `specs/backend/tdd/v5/14-profile-hierarchy.md`, `specs/backend/tdd/v5/15-project-binding.md`

**Quan trọng — đọc trước khi áp dụng:** repo có 3 cây mã gần như song song `backend/src/main/...`, `desktop/src/main/...`, `frontend/src/main/...` (đã xác nhận qua codegraph: `admin-middleware.ts`, `profile-rpc-handler.ts`, `project-rpc-handler.ts`, `rbac-types.ts`, `auth-types.ts`, `dispatcher.ts` tồn tại **byte-for-byte giống hệt nhau** ở `backend/` và `desktop/`). Toàn bộ diff dưới đây viết cho `backend/`, nhưng **phải áp dụng lại y hệt cho `desktop/src/main/...`** (Bước 4) — nếu không sẽ vá xong `backend/` mà `desktop/` vẫn còn lỗ hổng gốc. `project-rpc-handler.ts` phía `desktop/` còn có test đi kèm: `desktop/src/main/project/__tests__/project-rpc.test.ts` — test này gọi `createProjectMethods(...)` nên sẽ cần sửa cùng lúc (xem Bước 4).

---

## Quyết định kiến trúc trước khi đọc code fix

Cả 3 bug (001, 002, 003) có chung root cause: **không có nơi nào lấy được org-level role (`'developer'|'lead'|'admin'`) của user đang gọi RPC**. Có 2 cách khả dĩ để đưa role vào:

1. **Plumbing qua transport** — thêm `userRole` vào `RpcContext` (`backend/src/main/runtime/rpc/core.ts`), rồi bơm qua `RpcDispatcher.dispatch()`/`dispatchStreaming()` (`backend/src/main/runtime/rpc/dispatcher.ts`), rồi tới `WsSessionRouter`/`SessionManager` (per-user child process, role phải được truyền lúc `spawn()` qua env var kiểu `ORCA_USER_ID` hiện tại).
2. **Tra cứu trực tiếp theo `userId`** ngay trong handler, dùng `AuthUserStore.getUser(userId).role` (đã có sẵn, `backend/src/main/auth/auth-user-store.ts:112`).

**Chọn phương án 2 cho bản vá khẩn cấp này**, vì:

- `RpcContext`/`RpcDispatcher` được dùng chung bởi **56+ RPC method** (`ai-providers`, `accounts`, `workflow`, `task`, `terminal`, …) — sửa 2 file lõi này để thêm 1 field cho đúng 2 handler là rủi ro không tương xứng, và không có test nào che phủ `dispatcher.ts` hiện tại (`⚠️ no covering tests found`).
- Đường dẫn user thật (`WsSessionRouter.handleConnection` → `SessionManager.getOrSpawnUserProcess(userId)` → fork **per-user child process**, chạy `dispatchStreaming`) chỉ truyền `userId` một lần lúc spawn qua env var — muốn mang thêm `role` phải sửa cả logic spawn/env của `SessionManager` (ngoài phạm vi 2 bug này) và **role sẽ bị đóng băng (stale)** cho tới khi WS reconnect, y hệt vấn đề mà `OrcaSession.role` (snapshot lúc login) đã có sẵn.
- Tra cứu DB theo `userId` hoạt động **đúng trên cả 3 transport** (in-process/CLI dispatch, Unix-socket per-user process, WS proxy) mà không cần biết transport nào đang gọi, và **luôn phản ánh role hiện tại** (không stale) — đúng tinh thần "role check thật" mà `BUG-BE-HLD-001` yêu cầu, chỉ khác cơ chế mang dữ liệu.
- Đây cũng là hạt giống tự nhiên cho `PermissionService.hasPermission()` ở `BUG-BE-HLD-003` (xem cuối file) — `getUserRole` chính là `PermissionService.getGlobalRole()` thu nhỏ.

Đánh đổi: thêm 1 query DB mỗi lần gọi RPC admin/owner-check. Các RPC này (`profile.updateCompany`, `project.update`, …) là **thao tác quản trị tần suất thấp**, không nằm trên hot path streaming (terminal, agent exec) — chấp nhận được. Nếu cần tối ưu sau này có thể thêm cache TTL ngắn kiểu `ProfileResolver` (60s), không bắt buộc ngay.

---

## Bước 1 — `backend/src/main/profile/profile-rpc-handler.ts` (fix BUG-BE-HLD-001)

**File:** [`backend/src/main/profile/profile-rpc-handler.ts`](file:///opt/repos/orca/backend/src/main/profile/profile-rpc-handler.ts)
**Lines:** 17–23 (imports), 82–85 (chữ ký factory), 152/163/193/223/241/255/268 (7 call site), 280–293 (`requireAdmin`)

### Code sai thực tế:

```typescript
// dòng 282-293
function requireAdmin(ctx: { userId?: string; runtime?: unknown }): void {
  // In server mode, admin role is checked via the authenticated session.
  // ctx.runtime carries the OrcaRuntimeService; for now we throw FORBIDDEN
  // and rely on the auth layer (AuthManager) to gate admin routes.
  // TODO: when AuthManager exposes ctx.userRole, replace with:
  //   if (ctx.userRole !== 'admin') throw new Error('FORBIDDEN')
  // For now: any authenticated user with admin role passes; non-authenticated always fails.
  if (!ctx.userId) throw new Error('UNAUTHENTICATED')
  // Admin enforcement deferred to AuthManager middleware in http-server.ts
  // for routes decorated with requireAdmin flag. In-process RPC callers
  // must pass role validation upstream.
}
```

`requireAdmin(ctx)` chỉ throw nếu **chưa đăng nhập** — bất kỳ user role nào (kể cả `'developer'`) gọi `profile.updateCompany`/`updateDept`/`createCompany`/`createDept`/`setUserDept`/`getCompany`/`invalidate` đều pass.

### Fix:

**1a. Thêm import + type ở đầu file** (sau dòng 23, trước dòng 24 `// ── Shared param schemas`):

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

**1b. Sửa chữ ký `createProfileMethods`** (dòng 82-85):

```typescript
// TRƯỚC:
export function createProfileMethods(
  profileService: ProfileService,
  profileResolver: ProfileResolver
): RpcMethod[] {

// SAU:
export function createProfileMethods(
  profileService: ProfileService,
  profileResolver: ProfileResolver,
  getUserRole: UserRoleLookup
): RpcMethod[] {
```

**1c. Sửa 7 call site** — thay `requireAdmin(ctx)` bằng `await requireAdmin(ctx, getUserRole)`. Cả 7 vị trí có text giống hệt nhau (`requireAdmin(ctx)` đứng đầu dòng đầu tiên trong mỗi `handler: async (params, ctx) => { ... }`), nên có thể replace-all an toàn:

```typescript
// TRƯỚC (xuất hiện ở dòng 152, 163, 193, 223, 241, 255, 268):
        requireAdmin(ctx)

// SAU (áp dụng cho cả 7 vị trí):
        await requireAdmin(ctx, getUserRole)
```

7 vị trí cụ thể (method name → dòng gốc):
| RPC method | Dòng gốc `requireAdmin(ctx)` |
|---|---|
| `profile.getCompany` | 152 |
| `profile.updateCompany` | 163 |
| `profile.updateDept` | 193 |
| `profile.invalidate` | 223 |
| `profile.setUserDept` | 241 |
| `profile.createCompany` | 255 |
| `profile.createDept` | 268 |

**1d. Sửa helper `requireAdmin`** (dòng 280-293) — thay toàn bộ:

```typescript
// TRƯỚC:
function requireAdmin(ctx: { userId?: string; runtime?: unknown }): void {
  // In server mode, admin role is checked via the authenticated session.
  // ctx.runtime carries the OrcaRuntimeService; for now we throw FORBIDDEN
  // and rely on the auth layer (AuthManager) to gate admin routes.
  // TODO: when AuthManager exposes ctx.userRole, replace with:
  //   if (ctx.userRole !== 'admin') throw new Error('FORBIDDEN')
  // For now: any authenticated user with admin role passes; non-authenticated always fails.
  if (!ctx.userId) throw new Error('UNAUTHENTICATED')
  // Admin enforcement deferred to AuthManager middleware in http-server.ts
  // for routes decorated with requireAdmin flag. In-process RPC callers
  // must pass role validation upstream.
}

// SAU:
// FIX BUG-BE-HLD-001: role check thật — trước đây chỉ check đã login, không check
// role, nên bất kỳ user nào cũng sửa được company/dept policy (F32 KPI "0 permission bypass").
async function requireAdmin(
  ctx: { userId?: string },
  getUserRole: UserRoleLookup
): Promise<void> {
  if (!ctx.userId) throw new Error('UNAUTHENTICATED')
  const role = await getUserRole(ctx.userId)
  if (role !== 'admin') throw new Error('FORBIDDEN: admin role required')
}
```

### Tại sao đúng:

`requireAdmin` giờ tra `getUserRole(ctx.userId)` — hàm này (được wire ở Bước 3) gọi thẳng `AuthUserStore.getUser()`, đọc `role` thật từ bảng `orca_users`. Semantics khớp 100% với `admin-middleware.ts:32` (`session.role !== 'admin'` → 403) — cùng một định nghĩa "admin" trên cả HTTP route lẫn RPC layer, đúng như audit yêu cầu ("pattern đúng đã tồn tại ở nơi khác nhưng không được áp dụng cho RPC layer").

---

## Bước 2 — `backend/src/main/project/project-rpc-handler.ts` (fix BUG-BE-HLD-002)

**File:** [`backend/src/main/project/project-rpc-handler.ts`](file:///opt/repos/orca/backend/src/main/project/project-rpc-handler.ts)
**Lines:** 18–23 (imports), 81–84 (chữ ký factory), 132/147/162/177/192 (5 call site), 247–253 (`requireOwnerOrAdmin`)

### Code sai thực tế:

```typescript
// dòng 247-253
type ProjectRole = 'owner' | 'member' | 'viewer'

function requireOwnerOrAdmin(role: ProjectRole, _userId: string): void {
  if (role !== 'owner') {
    throw new Error('FORBIDDEN: only project owners can perform this action')
  }
}
```

Tên hàm hứa hẹn "OrAdmin" nhưng **không có nhánh nào check global admin** — `_userId` bị bỏ qua (prefix `_` xác nhận cố ý không dùng). `role` ở đây là `ProjectRole` (project-level: `owner|member|viewer`), khác hoàn toàn org-level role (`developer|lead|admin`) — global admin không override được project mà mình không phải `owner`.

### Fix:

**2a. Thêm import + type** (sau dòng 23, trước `// ── Param schemas`):

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

**2b. Sửa chữ ký `createProjectMethods`** (dòng 81-84) — thêm `getUserRole` bắt buộc, đặt **trước** `agentSpawner` (optional) để giữ đúng thứ tự required-trước-optional của TypeScript:

```typescript
// TRƯỚC:
export function createProjectMethods(
  projectService: ProjectService,
  agentSpawner?: ProfileAwareAgentSpawner
): RpcMethod[] {

// SAU:
export function createProjectMethods(
  projectService: ProjectService,
  getUserRole: UserRoleLookup,
  agentSpawner?: ProfileAwareAgentSpawner
): RpcMethod[] {
```

**2c. Sửa 5 call site** — thay `requireOwnerOrAdmin(member.role, userId)` bằng `await requireOwnerOrAdmin(member.role, userId, getUserRole)`. Text giống hệt nhau ở cả 5 vị trí, replace-all an toàn:

```typescript
// TRƯỚC (dòng 132, 147, 162, 177, 192):
        requireOwnerOrAdmin(member.role, userId)

// SAU (áp dụng cho cả 5 vị trí):
        await requireOwnerOrAdmin(member.role, userId, getUserRole)
```

5 vị trí cụ thể:
| RPC method | Dòng gốc |
|---|---|
| `project.update` | 132 |
| `project.delete` | 147 |
| `project.addMember` | 162 |
| `project.removeMember` | 177 |
| `project.updateMemberRole` | 192 |

**2d. Sửa helper `requireOwnerOrAdmin`** (dòng 249-253):

```typescript
// TRƯỚC:
function requireOwnerOrAdmin(role: ProjectRole, _userId: string): void {
  if (role !== 'owner') {
    throw new Error('FORBIDDEN: only project owners can perform this action')
  }
}

// SAU:
// FIX BUG-BE-HLD-002: thêm nhánh global-admin override — trước đây tên hàm hứa
// "OrAdmin" nhưng userId chưa từng được dùng để check admin thật.
async function requireOwnerOrAdmin(
  role: ProjectRole,
  userId: string,
  getUserRole: UserRoleLookup
): Promise<void> {
  if (role === 'owner') return
  const globalRole = await getUserRole(userId)
  if (globalRole === 'admin') return
  throw new Error('FORBIDDEN: only project owners or global admins can perform this action')
}
```

### Về `project.create` (mục 2 trong "Đề xuất fix" của ticket) — CHƯA sửa, cần PO quyết định trước:

Ticket tự nêu rõ đây là câu hỏi sản phẩm, không phải bug thuần kỹ thuật: *"Giới hạn `project.create` theo role Lead/Admin nếu đúng ý định thiết kế F34, hoặc cập nhật lại F34 để phản ánh đúng 'mọi user đều tạo được project' nếu đó là chủ đích sản phẩm (làm rõ với PO trước khi sửa code)."* Code hiện tại (`project-rpc-handler.ts:113-121`) chỉ yêu cầu `UNAUTHENTICATED`:

```typescript
defineMethod({
  name: 'project.create',
  params: CreateProjectParam,
  handler: async (params, ctx) => {
    const userId = ctx.userId
    if (!userId) throw new Error('UNAUTHENTICATED')
    return projectService.create({ ...params, createdBy: userId })
  }
}),
```

Nếu PO xác nhận đúng ý định F34 là Lead/Admin-only, áp dụng thêm (dùng lại `getUserRole` đã wire ở bước 2b):

```typescript
defineMethod({
  name: 'project.create',
  params: CreateProjectParam,
  handler: async (params, ctx) => {
    const userId = ctx.userId
    if (!userId) throw new Error('UNAUTHENTICATED')
    const role = await getUserRole(userId)
    if (role !== 'lead' && role !== 'admin') {
      throw new Error('FORBIDDEN: only lead or admin can create projects')
    }
    return projectService.create({ ...params, createdBy: userId })
  }
}),
```

Không đưa nhánh này vào bản vá bắt buộc vì thay đổi hành vi sản phẩm (có thể chặn user hợp lệ đang dùng tính năng) — khác bản chất với 001/002 (đóng lỗ hổng cho phép làm việc KHÔNG được phép).

### Về đổi tên `ProjectRole` → `ProjectMemberRole` (mục 3 của ticket) — khuyến nghị follow-up riêng, KHÔNG gộp vào bản vá khẩn cấp:

Đúng theo AGENTS.md ("Never use vague names" + "NEVER rename symbols with find-and-replace — use `rename`"), nhưng đây là refactor phạm vi rộng (đụng cả `desktop/src/main/project/project-rpc-handler.ts` và mọi nơi import `ProjectRole`), không phải sửa lỗ hổng bảo mật — làm ở PR riêng bằng:

```
gitnexus rename --target ProjectRole --to ProjectMemberRole
```

để tool hiểu call-graph và không rename nhầm `ProjectRole` ở nơi khác (nếu có) hoặc để sót call site.

---

## Bước 3 — `backend/src/main/server-bootstrap.ts` (wiring `getUserRole`)

**File:** [`backend/src/main/server-bootstrap.ts`](file:///opt/repos/orca/backend/src/main/server-bootstrap.ts)
**Lines:** 411–426

Đây là bước bắt buộc để Bước 1 + 2 compile và chạy đúng — `createProfileMethods`/`createProjectMethods` giờ có tham số bắt buộc mới. `authManager` đã được khởi tạo từ dòng 291 (`const authManager = new AuthManager(authDb)`), **trước** 2 lời gọi này (dòng 412-425) — không cần đổi thứ tự khởi tạo.

### Code sai thực tế (thiếu tham số):

```typescript
// dòng 411-414
// Register profile RPC methods [T01]
const { createProfileMethods } = await import('./profile/profile-rpc-handler')
rpcServer.addMethods(createProfileMethods(profileService, profileResolver))
console.log('[ServerBootstrap] ✅ Profile RPC methods registered (v5.0)')

// dòng 423-426
// Register project RPC methods [T01]
const { createProjectMethods } = await import('./project/project-rpc-handler')
rpcServer.addMethods(createProjectMethods(projectService))
console.log('[ServerBootstrap] ✅ Project RPC methods registered (v5.0)')
```

### Fix:

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

Ghi chú: `AuthUserStore.getUser(id: string): Promise<OrcaSessionUser | null>` đã tồn tại sẵn ở `backend/src/main/auth/auth-user-store.ts:112-120`, trả về `{ id, email, name, role, provider }` — không cần thêm method mới trong `AuthUserStore`.

Nếu muốn tránh lặp lambda 2 lần, có thể khai báo 1 biến dùng chung ngay trước 2 khối trên:

```typescript
const getUserRole = async (userId: string) => (await authManager.userStore.getUser(userId))?.role ?? null
// ... rồi truyền `getUserRole` vào cả createProfileMethods(...) và createProjectMethods(...)
```

---

## Bước 4 — Áp dụng lại y hệt cho `desktop/src/main/...` (mirror tree)

Xác nhận qua codegraph: `desktop/src/main/profile/profile-rpc-handler.ts`, `desktop/src/main/project/project-rpc-handler.ts` **giống hệt từng dòng** với bản `backend/`. Lặp lại chính xác Bước 1, 2, 3 cho:

- `desktop/src/main/profile/profile-rpc-handler.ts`
- `desktop/src/main/project/project-rpc-handler.ts`
- `desktop/src/main/server-bootstrap.ts` (kiểm tra `authManager` cũng được khởi tạo trước 2 lời gọi `createProfileMethods`/`createProjectMethods` tương tự — cấu trúc file đã xác nhận song song với `backend/`)

**Test cần cập nhật cùng lúc:** `desktop/src/main/project/__tests__/project-rpc.test.ts` (đã tồn tại, che phủ `createProjectMethods`) — mọi lời gọi `createProjectMethods(projectService, ...)` trong test này sẽ lỗi biên dịch sau khi thêm tham số `getUserRole` bắt buộc; sửa test truyền thêm 1 stub, ví dụ:

```typescript
const stubGetUserRole = async (_userId: string) => 'admin' as const // hoặc 'developer' tuỳ kịch bản test cần pass/fail
createProjectMethods(projectService, stubGetUserRole, agentSpawner)
```

`frontend/src/main/...` không có bản sao của `profile-rpc-handler.ts`/`project-rpc-handler.ts` trong kết quả khảo sát (chỉ `rbac-types.ts`/`auth-types.ts` bị lặp ở đó) — không cần vá thêm ở `frontend/`, nhưng nên grep nhanh (`rg "requireOwnerOrAdmin|function requireAdmin" frontend/src`) trước khi đóng ticket để chắc chắn.

---

## Test bắt buộc thêm (theo đề xuất §2 của BUG-BE-HLD-001 và test coverage TDD-14/15 §8)

Thêm vào (hoặc tạo mới) `backend/src/main/profile/__tests__/profile-rpc.test.ts` và `backend/src/main/project/__tests__/project-rpc.test.ts`:

```typescript
// profile-rpc.test.ts — role 'developer'/'lead' gọi các method admin-only phải bị từ chối
for (const method of [
  'profile.getCompany', 'profile.updateCompany', 'profile.updateDept',
  'profile.invalidate', 'profile.setUserDept', 'profile.createCompany', 'profile.createDept'
]) {
  it(`${method}: role 'developer' → FORBIDDEN`, async () => {
    const getUserRole = async () => 'developer' as const
    const methods = createProfileMethods(profileService, profileResolver, getUserRole)
    const handler = methods.find((m) => m.name === method)!
    await expect(
      handler.handler(minimalValidParamsFor(method), { userId: 'u-dev' })
    ).rejects.toThrow(/FORBIDDEN/)
  })
}

it("profile.updateCompany: role 'admin' → OK", async () => {
  const getUserRole = async () => 'admin' as const
  const methods = createProfileMethods(profileService, profileResolver, getUserRole)
  const handler = methods.find((m) => m.name === 'profile.updateCompany')!
  await expect(
    handler.handler({ companyId: 'c1', profile: {} }, { userId: 'u-admin' })
  ).resolves.toEqual({ success: true })
})
```

```typescript
// project-rpc.test.ts — global admin override project mà mình không phải owner
it("project.update: role 'admin' nhưng member.role='viewer' → vẫn OK (global override)", async () => {
  const getUserRole = async () => 'admin' as const
  projectServiceStub.assertAccess.mockResolvedValue({ projectId: 'p1', userId: 'u-admin', role: 'viewer' })
  const methods = createProjectMethods(projectServiceStub, getUserRole)
  const handler = methods.find((m) => m.name === 'project.update')!
  await expect(
    handler.handler({ projectId: 'p1', patch: { name: 'x' } }, { userId: 'u-admin' })
  ).resolves.toEqual({ success: true })
})

it("project.update: role 'developer' + member.role='member' → FORBIDDEN", async () => {
  const getUserRole = async () => 'developer' as const
  projectServiceStub.assertAccess.mockResolvedValue({ projectId: 'p1', userId: 'u-dev', role: 'member' })
  const methods = createProjectMethods(projectServiceStub, getUserRole)
  const handler = methods.find((m) => m.name === 'project.update')!
  await expect(
    handler.handler({ projectId: 'p1', patch: { name: 'x' } }, { userId: 'u-dev' })
  ).rejects.toThrow(/FORBIDDEN/)
})
```

---

## BUG-BE-HLD-003: `PermissionService.hasPermission()` thống nhất — thiết kế cho phase 2 (KHÔNG bắt buộc để merge 001/002)

Đúng như "Đề xuất fix" của ticket 003 (mục 3): *"Ưu tiên fix trước 2 lỗ hổng cụ thể BUG-BE-HLD-001 và BUG-BE-HLD-002 trước khi làm refactor lớn."* Phần dưới đây là thiết kế follow-up, để `getUserRole` ở Bước 1-3 trở thành nền cho một `PermissionService` trung tâm thay vì bị bỏ quên như một lambda rời rạc thứ 5.

### File mới: `backend/src/main/permissions/PermissionService.ts`

```typescript
/**
 * PermissionService — Role → resource → action policy table trung tâm (BUG-BE-HLD-003).
 *
 * Thay thế dần 4 cơ chế RBAC phân mảnh:
 *   1. resolveUserPermissions() (rbac-types.ts) — giữ nguyên, phục vụ fleet/server access,
 *      không có khái niệm resource/action nên KHÔNG migrate (đúng theo phạm vi khác — server
 *      pairing token, không phải RPC permission check).
 *   2. requireAdmin (admin-middleware.ts, HTTP route)      → migrate ở Bước B.
 *   3. requireAdmin (profile-rpc-handler.ts, RPC — đã fix ở Bước 1 dùng getUserRole trực tiếp)
 *      → migrate sang PermissionService.hasPermission() ở Bước C.
 *   4. requireOwnerOrAdmin (project-rpc-handler.ts, đã fix ở Bước 2) → migrate ở Bước C.
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

### Bước B (follow-up) — migrate `admin-middleware.ts` sang `PermissionService`

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

Đây là thay đổi có blast radius rộng hơn (route registration ở `admin-router.ts` phải đổi từ import trực tiếp `requireAdmin` sang gọi `createRequireAdmin(permissions)`) — **không** làm trong cùng PR với 001/002.

### Bước C (follow-up) — migrate `profile-rpc-handler.ts`/`project-rpc-handler.ts` sang `PermissionService`

Sau khi Bước 1-3 đã chạy ổn định, thay `getUserRole: UserRoleLookup` bằng `permissions: PermissionService` và:

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

`UserRoleLookup` (Bước 1/2 ở trên) và `PermissionService.getGlobalRole` có cùng chữ ký `(userId) => Promise<Role | null>` — migrate này chỉ đổi loại tham số truyền vào factory, không đổi behavior đã fix ở 001/002. Đây chính là lý do bản vá khẩn cấp ở Bước 1-3 an toàn để làm trước: nó không phải "lối tắt phải vứt bỏ" mà là **tập con đúng** của thiết kế `PermissionService` cuối cùng.

---

## Tóm tắt thay đổi

| Bug | File | Vị trí | Thay đổi |
|-----|------|--------|---------|
| BUG-BE-HLD-001 | `backend/src/main/profile/profile-rpc-handler.ts` | import (23), factory sig (82-85), 7 call site (152/163/193/223/241/255/268), `requireAdmin` (282-293) | Thêm `getUserRole` param; `requireAdmin` tra role thật, throw `FORBIDDEN` nếu ≠ `'admin'` |
| BUG-BE-HLD-002 | `backend/src/main/project/project-rpc-handler.ts` | import (23), factory sig (81-84), 5 call site (132/147/162/177/192), `requireOwnerOrAdmin` (249-253) | Thêm `getUserRole` param; `requireOwnerOrAdmin` bypass nếu `role==='owner'` HOẶC `globalRole==='admin'` |
| Wiring (bắt buộc cho 001+002) | `backend/src/main/server-bootstrap.ts` | 411-426 | Truyền `authManager.userStore.getUser` làm `getUserRole` vào cả 2 factory |
| Mirror (bắt buộc cho 001+002) | `desktop/src/main/{profile,project}/*.ts`, `desktop/src/main/server-bootstrap.ts`, `desktop/src/main/project/__tests__/project-rpc.test.ts` | như trên | Lặp lại y hệt Bước 1-3 + sửa test hiện có |
| Optional (PO quyết định) | `backend/src/main/project/project-rpc-handler.ts` | `project.create` (113-121) | Giới hạn Lead/Admin nếu đúng ý F34 — CHƯA áp dụng, cần PO xác nhận |
| Optional (follow-up riêng) | rename toàn repo | `ProjectRole` → `ProjectMemberRole` | Dùng `gitnexus rename`, không find-and-replace |
| BUG-BE-HLD-003 (phase 2) | `backend/src/main/permissions/PermissionService.ts` (mới) + `admin-middleware.ts` + 2 rpc-handler ở trên | — | Thay `UserRoleLookup` bằng `PermissionService.hasPermission()` thống nhất; `TaskGrantService` giữ nguyên logic riêng nhưng có thể expose qua cùng interface sau |

**Thứ tự áp dụng bắt buộc:** Bước 1 → Bước 2 → Bước 3 (không có thứ tự phụ thuộc giữa 1 và 2, nhưng cả hai phải xong trước Bước 3 vì Bước 3 gọi cả hai factory với chữ ký mới) → Bước 4 (mirror desktop/) → chạy `detect_changes({scope: "compare", base_ref: "main"})` để xác nhận chỉ `profile-rpc-handler.ts`, `project-rpc-handler.ts`, `server-bootstrap.ts` (và bản mirror) bị ảnh hưởng trước khi commit. BUG-BE-HLD-003 (PermissionService) làm ở PR riêng, sau khi 001/002 đã merge và chạy ổn định.
