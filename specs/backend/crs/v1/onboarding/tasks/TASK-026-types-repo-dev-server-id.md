# TASK-026: Sửa `src/shared/types.ts` — Thêm `Repo.devServerId`

**Phase:** 2 — Remote Repo  
**Solution:** [SOL-004-005-006](../solutions/SOL-004-005-006-platform-preflight-repo.md) §C.4  
**Depends on:** TASK-002  
**Blocks:** TASK-025

---

## Mục tiêu

Thêm field `devServerId` vào `Repo` type để phân biệt local repos và remote repos trên dev server.

---

## File cần sửa

**Path:** `src/shared/types.ts`

---

## Thay đổi cần thực hiện

Tìm `type Repo` hoặc `interface Repo` và thêm field:

```typescript
type Repo = {
  // ... existing fields giữ nguyên ...
  devServerId?: string | null    // NEW — ID của DevServer chứa repo (null hoặc undefined = local)
}
```

---

## Acceptance Criteria

- [x] `Repo` type có field `devServerId?: string | null`
- [x] Field là optional (backward compatible với repos hiện có)
- [x] TypeScript compile thành công
- [x] Không có breaking changes cho existing code dùng `Repo` type
