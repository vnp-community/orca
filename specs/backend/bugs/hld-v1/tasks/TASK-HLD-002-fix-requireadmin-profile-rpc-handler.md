# TASK-HLD-002: Sửa `requireAdmin(ctx)` trong `profile-rpc-handler.ts` dùng role check thật

**Priority:** 🔴 CRITICAL — permission bypass đang mở, mọi user (kể cả `'developer'`) sửa được company/dept policy
**Effort:** ~30 phút
**Status:** ✅ DONE — 2026-08-09 (áp dụng đúng như solution; `tsc --noEmit` xác nhận không còn lỗi arity cho `createProfileMethods`. Chưa chạy test suite `profile-rpc.test.ts` — file test chưa tồn tại, xem ghi chú Verification.)
**Bug refs:** BUG-BE-HLD-001
**Solution ref:** [SOLUTION-rbac-exact.md](../solutions/SOLUTION-rbac-exact.md) — Bước 1
**Depends on:** TASK-HLD-001 (cần type `UserRoleLookup` + `getUserRole` đã wire ở `server-bootstrap.ts`)

---

## Mục tiêu

`requireAdmin(ctx)` hiện tại **chỉ throw nếu chưa đăng nhập** — không hề check role. Bất kỳ user nào (kể cả role `'developer'`) gọi `profile.updateCompany`/`updateDept`/`createCompany`/`createDept`/`setUserDept`/`getCompany`/`invalidate` đều pass, vi phạm KPI F32 "0 permission bypass".

Sửa để `requireAdmin` tra role thật qua `getUserRole(ctx.userId)` (đã wire ở TASK-HLD-001) và throw `FORBIDDEN` nếu role khác `'admin'`.

## File cần sửa/tạo

```
backend/src/main/profile/profile-rpc-handler.ts
```

## Thay đổi cụ thể

### Code sai hiện tại (dòng 282-293)

```typescript
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

### 1. Sửa chữ ký `createProfileMethods` (dòng 82-85)

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

### 2. Sửa 7 call site — thay `requireAdmin(ctx)` bằng `await requireAdmin(ctx, getUserRole)`

Text giống hệt nhau ở cả 7 vị trí (dòng đầu tiên trong mỗi `handler: async (params, ctx) => { ... }`) — có thể replace-all an toàn:

```typescript
// TRƯỚC (xuất hiện ở dòng 152, 163, 193, 223, 241, 255, 268):
        requireAdmin(ctx)

// SAU (áp dụng cho cả 7 vị trí):
        await requireAdmin(ctx, getUserRole)
```

7 vị trí cụ thể (RPC method → dòng gốc):

| RPC method | Dòng gốc `requireAdmin(ctx)` |
|---|---|
| `profile.getCompany` | 152 |
| `profile.updateCompany` | 163 |
| `profile.updateDept` | 193 |
| `profile.invalidate` | 223 |
| `profile.setUserDept` | 241 |
| `profile.createCompany` | 255 |
| `profile.createDept` | 268 |

### 3. Sửa helper `requireAdmin` (dòng 280-293) — thay toàn bộ

```typescript
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

### Tại sao đúng

`requireAdmin` giờ tra `getUserRole(ctx.userId)` — gọi thẳng `AuthUserStore.getUser()`, đọc `role` thật từ bảng `orca_users`. Semantics khớp 100% với `admin-middleware.ts:32` (`session.role !== 'admin'` → 403) — cùng một định nghĩa "admin" trên cả HTTP route lẫn RPC layer.

## Test bắt buộc thêm

Thêm vào (hoặc tạo mới) `backend/src/main/profile/__tests__/profile-rpc.test.ts`:

```typescript
// role 'developer'/'lead' gọi các method admin-only phải bị từ chối
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

> Lưu ý: `minimalValidParamsFor(method)` là helper giả định trong test — nếu chưa tồn tại, viết params hợp lệ tối thiểu riêng cho từng method dựa theo Zod schema đã khai báo trong file (`CreateCompanyParam`, `UpdateDeptParam`, v.v.).

## Verification

```bash
# 1. Type-check — phải PASS (khác với TASK-HLD-001, task này khiến signature khớp lại)
pnpm --filter backend tsc --noEmit

# 2. Xác nhận không còn lời gọi requireAdmin(ctx) không await/không truyền getUserRole
grep -n "requireAdmin(ctx)$" backend/src/main/profile/profile-rpc-handler.ts
# Expected: KHÔNG có kết quả (đã thay hết bằng "await requireAdmin(ctx, getUserRole)")

grep -n "await requireAdmin(ctx, getUserRole)" backend/src/main/profile/profile-rpc-handler.ts
# Expected: đúng 7 dòng khớp

# 3. Chạy test
pnpm --filter backend test -- profile-rpc

# 4. GitNexus regression check trước khi commit
# (theo AGENTS.md — bắt buộc trước khi commit)
```
