# TASK-037: Sửa `src/shared/types.ts` — Thêm `OnboardingChecklistState.perServer`

**Phase:** 3 — Multi Dev-Server Checklist  
**Solution:** [SOL-007-008-009](../solutions/SOL-007-008-009-windows-notifications-checklist.md) §C.1  
**Depends on:** TASK-002  
**Blocks:** TASK-038, TASK-039

---

## Mục tiêu

Mở rộng `OnboardingChecklistState` để hỗ trợ per-server checklist items, keyed by `devServerId`.

---

## File cần sửa

**Path:** `src/shared/types.ts`

---

## Thay đổi cần thực hiện

Tìm `OnboardingChecklistState` trong file và cập nhật:

```typescript
export type PerServerChecklistState = {
  addedRepo?: boolean
  ranFirstAgent?: boolean
  ranSecondAgentOnSameTask?: boolean
  reviewedDiff?: boolean
  openedPr?: boolean
  addedFolder?: boolean
  openedFile?: boolean
  ranAgentOnFile?: boolean
}

type OnboardingChecklistState = {
  // Global items (1 lần cho toàn account — giữ nguyên):
  choseAgent?: boolean
  triedCmdJ?: boolean
  shapedSidebar?: boolean

  // Per-server items — keyed by devServerId:
  perServer?: Record<string, PerServerChecklistState>  // NEW
}
```

---

## Acceptance Criteria

- [x] `PerServerChecklistState` type được export
- [x] `OnboardingChecklistState` có field `perServer?: Record<string, PerServerChecklistState>`
- [x] Global fields (`choseAgent`, `triedCmdJ`, `shapedSidebar`) giữ nguyên
- [x] `perServer` là optional (backward compatible với state cũ không có field này)
- [x] TypeScript compile thành công
