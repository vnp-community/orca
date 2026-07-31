# T01 — Register Profile + Project RPC Methods in server-bootstrap.ts

**Phase:** 1 (Quick Win)  
**Effort:** ~10 min  
**Depends on:** —  
**Solution ref:** [08-server-bootstrap-wiring.md](../solutions/08-server-bootstrap-wiring.md)  
**TDD ref:** TDD-14, TDD-15

---

## Mục tiêu

Đăng ký Profile và Project RPC methods vào `OrcaRuntimeRpcServer` trong `server-bootstrap.ts`.  
Các methods đã tồn tại trong handler files — chỉ cần gọi `rpcServer.addMethods()`.

---

## Context — Tại sao cần làm

`server-bootstrap.ts` hiện tại đã wire TDD-16 (AI providers), TDD-17 (workflow), TDD-18 (task), TDD-19 (workspace) vào `rpcServer.addMethods()`.  
Nhưng TDD-14 (profile) và TDD-15 (project) **chưa được register**.

---

## Files Cần Đọc

1. `src/main/server-bootstrap.ts` — xem step 9 (line ~379-384) và step 10 (line ~386-391)
2. `src/main/profile/profile-rpc-handler.ts` — xem export `createProfileMethods()`
3. `src/main/project/project-rpc-handler.ts` — xem export `createProjectMethods()`

---

## Files Cần Sửa

### `src/main/server-bootstrap.ts`

**Tìm step 9** (sau khi profileService + profileResolver được khởi tạo):
```typescript
// Hiện tại (line ~382-384):
const profileService = new ProfileService(pool)
const profileResolver = new ProfileResolver(profileService)
console.log('[ServerBootstrap] ✅ ProfileService + ProfileResolver initialized (v5.0)')
```

**Thêm ngay sau:**
```typescript
// Register profile RPC methods
const { createProfileMethods } = await import('./profile/profile-rpc-handler')
rpcServer.addMethods(createProfileMethods(profileService, profileResolver, authManager))
console.log('[ServerBootstrap] ✅ Profile RPC methods registered (v5.0)')
```

**Tìm step 10** (sau khi projectService + _projectRouter được khởi tạo):
```typescript
// Hiện tại (line ~389-391):
const projectService = new ProjectService(pool, devServerManager)
const _projectRouter = new ProjectServerRouter(projectService, devServerManager, relayConnectionPool)
console.log('[ServerBootstrap] ✅ ProjectService + ProjectServerRouter initialized (v5.0)')
```

**Thêm ngay sau:**
```typescript
// Register project RPC methods
const { createProjectMethods } = await import('./project/project-rpc-handler')
rpcServer.addMethods(createProjectMethods(projectService, _projectRouter, profileResolver))
console.log('[ServerBootstrap] ✅ Project RPC methods registered (v5.0)')
```

---

## Kiểm tra createProfileMethods signature

Trước khi sửa, đọc `profile-rpc-handler.ts` để xác nhận đúng function signature:

```bash
grep -n "createProfileMethods\|export function\|export const" src/main/profile/profile-rpc-handler.ts
```

Tương tự cho project:
```bash
grep -n "createProjectMethods\|export function\|export const" src/main/project/project-rpc-handler.ts
```

Nếu signature khác → điều chỉnh arguments cho phù hợp.

---

## Acceptance Criteria

- [x] `grep "createProfileMethods" src/main/server-bootstrap.ts` → tìm thấy ✅ (line 387)
- [x] `grep "createProjectMethods" src/main/server-bootstrap.ts` → tìm thấy ✅ (line 399)
- [x] `pnpm tsc --noEmit` → 0 TypeScript errors ✅
- [x] Server khởi động không crash: các log `✅ Profile RPC methods registered` và `✅ Project RPC methods registered` xuất hiện ✅
