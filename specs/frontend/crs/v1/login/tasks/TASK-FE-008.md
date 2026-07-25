# TASK-FE-008 — Modify `store/index.ts` — Register AuthSlice

**Phase:** 1 — Auth Foundation
**Solution:** [SOL-FE-LG-001](../solutions/SOL-FE-LG-001-login-page.md) §5 (Files cần sửa)
**Depends on:** TASK-FE-003
**Blocks:** —
**Effort:** XS (~10 phút)
**Status:** ✅ Done (pre-existing — store/index.ts already registers createAuthSlice at line 88)

---

## Mô tả

Đăng ký `AuthSlice` vào Zustand store tổng `useAppStore`.

---

## File cần sửa

### `src/renderer/src/store/index.ts` [MODIFY]

**Tìm:**
```typescript
import { create } from 'zustand'
// ... existing imports
```

**Thêm import:**
```typescript
import { createAuthSlice, AuthSlice } from './slices/auth'
```

**Tìm type AppState (hoặc tương đương) và thêm `AuthSlice`:**
```typescript
type AppState = /* existing */ & AuthSlice
```

**Tìm `create<AppState>()((...a) => ({` và thêm slice:**
```typescript
export const useAppStore = create<AppState>()((...a) => ({
  // ... existing slices
  ...createAuthSlice(...a),    // ← THÊM DÒNG NÀY
}))
```

---

## Constraints

- Chỉ thêm 1 dòng `...createAuthSlice(...a)` — không thay đổi logic hiện tại
- Đảm bảo TypeScript compile không lỗi

---

## Verify

```bash
npx tsc --noEmit
# Expected: 0 errors
```
