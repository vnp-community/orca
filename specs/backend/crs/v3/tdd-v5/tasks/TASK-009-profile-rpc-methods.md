# TASK-009: Profile RPC Methods

**Phase:** 2 — Profile Hierarchy  
**Solution ref:** [SOL-V5-001](../solutions/SOL-V5-001-profile-hierarchy.md) §4  
**Prerequisite:** TASK-007, TASK-008  
**Status:** ✅ DONE — 2026-07-28

---

## Mô tả

Tạo file RPC method registration cho profile domain. Tìm pattern đăng ký RPC hiện tại trong codebase để follow đúng interface.

---

## Bước 1: Tìm RPC dispatcher pattern

```bash
# Tìm cách RPC methods được register hiện tại
grep -r "register\|dispatcher\|addMethod" src/main/runtime/ --include="*.ts" -l
ls src/main/runtime/
ls src/main/ipc/
```

---

## File cần tạo: `src/main/profile/profile-rpc-handler.ts`

Sau khi xác định pattern RPC, implement các handlers sau:

```typescript
// Methods to expose:
// profile.getResolved     → profileResolver.resolve(session.userId)
// profile.getUserProfile  → profileService.getUserProfile(userId)
// profile.updateUser      → profileService.setUserProfile (reject security section)
// profile.getCompany      → profileService.getCompanyProfile (admin only)
// profile.updateCompany   → profileService.setCompanyProfile (admin only)
// profile.invalidate      → profileResolver.invalidate(userId?) (admin only)
// profile.setUserDept     → profileService.setUserDepartment (admin only)
// profile.createCompany   → profileService.createCompany (admin only)
// profile.createDept      → profileService.createDepartment (admin only)
```

**Access control:**
- `profile.getResolved`, `profile.getUserProfile`, `profile.updateUser` → any authenticated user
- `profile.updateUser` → reject nếu `params.profile.security !== undefined` → throw `PROFILE_FIELD_LOCKED`
- All `company.*` and `dept.*` operations → `role === 'admin'` only
- `profile.invalidate` → admin only

---

## Verification

```bash
pnpm tsc --noEmit
```

## Acceptance Criteria

- [x] 9 RPC methods được register
- [x] `profile.updateUser` reject nếu có `security` field
- [x] Company/dept methods check `role === 'admin'`
- [x] `profile.getResolved` gọi `profileResolver.resolve`
- [x] Không TypeScript errors
