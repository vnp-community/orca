# TASK-039: Sửa `src/main/ipc/onboarding-ipc.ts` — Thêm `markChecklistItem`

**Phase:** 3 — Multi Dev-Server Checklist  
**Solution:** [SOL-007-008-009](../solutions/SOL-007-008-009-windows-notifications-checklist.md) §C.3  
**Depends on:** TASK-022, TASK-037  
**Blocks:** TASK-041

---

## Mục tiêu

Thêm IPC handler `onboarding.markChecklistItem` hỗ trợ đánh dấu cả global items và per-server items.

---

## File cần sửa

**Path:** `src/main/ipc/onboarding-ipc.ts`

---

## Thay đổi cần thực hiện

```typescript
import type { OnboardingChecklistState, PerServerChecklistState } from '../../shared/types'

// Trong registerOnboardingIpcHandlers():
ipc.handle('onboarding.markChecklistItem', async (_, params: {
  item: keyof OnboardingChecklistState | keyof PerServerChecklistState
  devServerId?: string   // undefined = global item
  value?: boolean        // default: true
}): Promise<void> => {
  const { item, devServerId, value = true } = params
  store.mutate(state => {
    const cl = state.onboarding?.checklist ?? {}
    if (devServerId) {
      // Per-server item
      cl.perServer = cl.perServer ?? {}
      cl.perServer[devServerId] = cl.perServer[devServerId] ?? {}
      ;(cl.perServer[devServerId] as Record<string, unknown>)[item] = value
    } else {
      // Global item
      ;(cl as Record<string, unknown>)[item] = value
    }
    if (!state.onboarding) state.onboarding = {}
    state.onboarding.checklist = cl
  })
})
```

---

## Acceptance Criteria

- [x] Handler `onboarding.markChecklistItem` được đăng ký
- [x] Global item (`choseAgent`, `triedCmdJ`, `shapedSidebar`) set đúng khi không có `devServerId`
- [x] Per-server item (`addedRepo`, v.v.) set đúng vào `perServer[devServerId]`
- [x] `value` mặc định là `true`
- [x] `value: false` có thể unmark item
- [x] `perServer` được khởi tạo `{}` nếu chưa có
- [x] TypeScript compile thành công

---

## Lưu ý cho AI

Cần inject `store` vào `registerOnboardingIpcHandlers()` nếu chưa có — cập nhật function signature.
