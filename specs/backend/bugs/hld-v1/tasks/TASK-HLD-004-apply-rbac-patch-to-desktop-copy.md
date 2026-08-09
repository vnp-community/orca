# TASK-HLD-004: Áp lại patch RBAC (task 001+002+003) cho bản sao byte-for-byte ở `desktop/src/main/...`

**Priority:** 🔴 CRITICAL — nếu bỏ qua, `desktop/` vẫn còn nguyên lỗ hổng gốc dù `backend/` đã vá
**Effort:** ~40 phút
**Status:** ✅ DONE — 2026-08-09 (áp patch y hệt backend/ cho `desktop/{profile,project}-rpc-handler.ts` + `server-bootstrap.ts`; sửa 12 call site trong `project-rpc.test.ts` — nhiều hơn 1 call site so với ước lượng ban đầu của solution, đã xử lý đủ cả 12; xác nhận `frontend/src` không có bản sao cần vá qua `rg`. `tsc --noEmit` không phát sinh lỗi mới. Lưu ý: `desktop/src/main/server-bootstrap.ts:462` cũng lộ ra cùng lỗi `StepExecutors` type-mismatch như backend/ (xem TASK-HLD-013) — TASK-HLD-013 hiện chỉ scope cho backend/, cần task follow-up riêng cho desktop/ nếu áp dụng.)
**Bug refs:** BUG-BE-HLD-001, BUG-BE-HLD-002
**Solution ref:** [SOLUTION-rbac-exact.md](../solutions/SOLUTION-rbac-exact.md) — Bước 4
**Depends on:** TASK-HLD-002, TASK-HLD-003

---

## Mục tiêu

Repo có 3 cây mã gần như song song `backend/src/main/...`, `desktop/src/main/...`, `frontend/src/main/...`. Đã xác nhận qua codegraph: `profile-rpc-handler.ts`, `project-rpc-handler.ts`, `server-bootstrap.ts` (và `admin-middleware.ts`, `rbac-types.ts`, `auth-types.ts`, `dispatcher.ts`) tồn tại **byte-for-byte giống hệt nhau** ở `backend/` và `desktop/`.

Task này lặp lại chính xác các thay đổi của TASK-HLD-001 + TASK-HLD-002 + TASK-HLD-003 cho bản sao ở `desktop/`, và sửa test đi kèm (`desktop/src/main/project/__tests__/project-rpc.test.ts`) vì test này gọi `createProjectMethods(...)` sẽ lỗi biên dịch sau khi thêm tham số bắt buộc.

`frontend/src/main/...` **không có** bản sao của 2 file handler này (chỉ có `rbac-types.ts`/`auth-types.ts` bị lặp ở đó) — không cần vá `frontend/`, nhưng phải grep xác nhận trước khi đóng task (xem Verification).

## File cần sửa/tạo

```
desktop/src/main/profile/profile-rpc-handler.ts
desktop/src/main/project/project-rpc-handler.ts
desktop/src/main/server-bootstrap.ts
desktop/src/main/project/__tests__/project-rpc.test.ts
```

## Thay đổi cụ thể

### 1. `desktop/src/main/profile/profile-rpc-handler.ts`

Áp dụng **y hệt** nội dung của TASK-HLD-001 (phần 1, thêm import + type `UserRoleLookup`) và TASK-HLD-002 (toàn bộ 3 thay đổi: chữ ký `createProfileMethods`, 7 call site `requireAdmin(ctx)` → `await requireAdmin(ctx, getUserRole)`, và implementation `requireAdmin` mới). Xem 2 file task đó để lấy nguyên văn diff — nội dung file nguồn giống hệt `backend/`, dòng số cũng giống hệt.

### 2. `desktop/src/main/project/project-rpc-handler.ts`

Áp dụng **y hệt** nội dung của TASK-HLD-001 (phần 2, thêm import + type `UserRoleLookup`) và TASK-HLD-003 (toàn bộ 3 thay đổi: chữ ký `createProjectMethods`, 5 call site `requireOwnerOrAdmin(member.role, userId)` → `await requireOwnerOrAdmin(member.role, userId, getUserRole)`, và implementation `requireOwnerOrAdmin` mới). Dòng số giống hệt `backend/`.

### 3. `desktop/src/main/server-bootstrap.ts`

Kiểm tra `authManager` cũng được khởi tạo trước 2 lời gọi `createProfileMethods`/`createProjectMethods` (cấu trúc file đã xác nhận song song với `backend/`), rồi áp dụng y hệt Bước 3 của solution (== nội dung mục 3 trong TASK-HLD-001):

```typescript
// Register profile RPC methods [T01]
const { createProfileMethods } = await import('./profile/profile-rpc-handler')
rpcServer.addMethods(
  createProfileMethods(
    profileService,
    profileResolver,
    async (userId) => (await authManager.userStore.getUser(userId))?.role ?? null
  )
)
console.log('[ServerBootstrap] ✅ Profile RPC methods registered (v5.0)')

// Register project RPC methods [T01]
const { createProjectMethods } = await import('./project/project-rpc-handler')
rpcServer.addMethods(
  createProjectMethods(
    projectService,
    async (userId) => (await authManager.userStore.getUser(userId))?.role ?? null
  )
)
console.log('[ServerBootstrap] ✅ Project RPC methods registered (v5.0)')
```

### 4. `desktop/src/main/project/__tests__/project-rpc.test.ts` — sửa test hiện có

Test này gọi `createProjectMethods(...)` và sẽ lỗi biên dịch sau khi thêm tham số `getUserRole` bắt buộc. Sửa mọi lời gọi để truyền thêm 1 stub:

```typescript
const stubGetUserRole = async (_userId: string) => 'admin' as const // hoặc 'developer' tuỳ kịch bản test cần pass/fail
createProjectMethods(projectService, stubGetUserRole, agentSpawner)
```

Rà tất cả các lời gọi `createProjectMethods(` hiện có trong file test này (không chỉ 1 chỗ) và thêm tham số `getUserRole` đúng vị trí thứ 2 (trước `agentSpawner` nếu có).

## Verification

```bash
# 1. Xác nhận không còn call site cũ ở desktop/ (tương tự check của task 002/003)
grep -n "requireAdmin(ctx)$" desktop/src/main/profile/profile-rpc-handler.ts
# Expected: KHÔNG có kết quả

grep -n "requireOwnerOrAdmin(member.role, userId)$" desktop/src/main/project/project-rpc-handler.ts
# Expected: KHÔNG có kết quả

grep -n "await requireAdmin(ctx, getUserRole)" desktop/src/main/profile/profile-rpc-handler.ts
# Expected: đúng 7 dòng khớp

grep -n "await requireOwnerOrAdmin(member.role, userId, getUserRole)" desktop/src/main/project/project-rpc-handler.ts
# Expected: đúng 5 dòng khớp

# 2. Xác nhận wiring trong server-bootstrap.ts
grep -n "authManager.userStore.getUser" desktop/src/main/server-bootstrap.ts
# Expected: 2 dòng khớp

# 3. Xác nhận frontend/ KHÔNG có bản sao cần vá (đúng như solution đã khảo sát)
rg "requireOwnerOrAdmin|function requireAdmin" frontend/src
# Expected: KHÔNG có kết quả — nếu CÓ, dừng lại và báo cáo (solution đã sai giả định, cần vá thêm frontend/)

# 4. Type-check + test
pnpm --filter desktop tsc --noEmit
pnpm --filter desktop test -- project-rpc
pnpm --filter desktop test -- profile-rpc

# 5. GitNexus regression check trước khi commit — xác nhận CHỈ profile-rpc-handler.ts,
#    project-rpc-handler.ts, server-bootstrap.ts (backend/ + desktop/) và test liên quan bị ảnh hưởng
# (theo AGENTS.md — bắt buộc trước khi commit, dùng detect_changes({ scope: "compare", base_ref: "main" }))
```
