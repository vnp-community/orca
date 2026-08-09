# TASK-HLD-003: Sửa `requireOwnerOrAdmin` trong `project-rpc-handler.ts` — thêm nhánh global-admin override

**Priority:** 🔴 CRITICAL — tên hàm hứa "OrAdmin" nhưng chưa từng check admin thật
**Effort:** ~30 phút
**Status:** ✅ DONE — 2026-08-09 (áp dụng đúng như solution; `tsc --noEmit` xác nhận không còn lỗi arity cho `createProjectMethods`. `project.create` và rename `ProjectRole` cố tình KHÔNG đụng tới, đúng phạm vi task. Chưa chạy test suite — xem ghi chú Verification.)
**Bug refs:** BUG-BE-HLD-002
**Solution ref:** [SOLUTION-rbac-exact.md](../solutions/SOLUTION-rbac-exact.md) — Bước 2
**Depends on:** TASK-HLD-001 (cần type `UserRoleLookup` + `getUserRole` đã wire ở `server-bootstrap.ts`)

---

## Mục tiêu

`requireOwnerOrAdmin(role, _userId)` hiện tại **không có nhánh nào check global admin** — tham số `_userId` bị bỏ qua hoàn toàn (prefix `_` xác nhận cố ý không dùng). `role` ở đây là `ProjectRole` (project-level: `owner|member|viewer`), khác hoàn toàn org-level role (`developer|lead|admin`) — global admin không override được project mà mình không phải `owner`, dù tên hàm hứa hẹn điều đó.

Sửa để hàm nhận thêm `getUserRole`, cho phép user có org-level role `'admin'` bypass check `role === 'owner'`.

## File cần sửa/tạo

```
backend/src/main/project/project-rpc-handler.ts
```

## Thay đổi cụ thể

### Code sai hiện tại (dòng 247-253)

```typescript
type ProjectRole = 'owner' | 'member' | 'viewer'

function requireOwnerOrAdmin(role: ProjectRole, _userId: string): void {
  if (role !== 'owner') {
    throw new Error('FORBIDDEN: only project owners can perform this action')
  }
}
```

### 1. Sửa chữ ký `createProjectMethods` (dòng 81-84)

Đặt `getUserRole` **trước** `agentSpawner` (optional) để giữ đúng thứ tự required-trước-optional của TypeScript:

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

### 2. Sửa 5 call site — thay `requireOwnerOrAdmin(member.role, userId)` bằng `await requireOwnerOrAdmin(member.role, userId, getUserRole)`

Text giống hệt nhau ở cả 5 vị trí, replace-all an toàn:

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

### 3. Sửa helper `requireOwnerOrAdmin` (dòng 249-253) — thay toàn bộ

```typescript
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

## Ngoài phạm vi task này — KHÔNG sửa

- **`project.create`** (dòng 113-121) — ticket gốc nêu rõ đây là câu hỏi sản phẩm ("giới hạn theo role Lead/Admin nếu đúng ý định F34, hoặc cập nhật F34" — cần PO xác nhận trước). Giữ nguyên `if (!userId) throw new Error('UNAUTHENTICATED')`, không thêm role check.
- **Đổi tên `ProjectRole` → `ProjectMemberRole`** — đúng theo AGENTS.md nhưng là refactor phạm vi rộng, làm ở PR riêng bằng `gitnexus rename --target ProjectRole --to ProjectMemberRole` (không find-and-replace).

## Test bắt buộc thêm

Thêm vào (hoặc tạo mới) `backend/src/main/project/__tests__/project-rpc.test.ts`:

```typescript
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

## Verification

```bash
# 1. Type-check — phải PASS
pnpm --filter backend tsc --noEmit

# 2. Xác nhận không còn call site cũ (không await / không truyền getUserRole)
grep -n "requireOwnerOrAdmin(member.role, userId)$" backend/src/main/project/project-rpc-handler.ts
# Expected: KHÔNG có kết quả

grep -n "await requireOwnerOrAdmin(member.role, userId, getUserRole)" backend/src/main/project/project-rpc-handler.ts
# Expected: đúng 5 dòng khớp

# 3. Chạy test
pnpm --filter backend test -- project-rpc

# 4. GitNexus regression check trước khi commit
# (theo AGENTS.md — bắt buộc trước khi commit)
```
