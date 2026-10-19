# Phase 5 — Quality Gate IPC & Rejection Banner

**Phase:** 5/6  
**Priority:** P2  
**Effort:** ~0.5–1 ngày  
**Depends on:** Phase 3 (task-enrichment slice, task-selectors), Phase 4 (types)  
**Unlocks:** Phase 6  
**Source solutions:** FE-IW-SOL-003

---

## Mục tiêu Phase

Wire quality gate events từ backend (`onTaskStatusChanged`, `onCRSStatusUpdated`) vào store, và tạo rejection banner component. Đây là lớp cuối cùng trước tests.

---

## T17 — Tạo `task-rejection-banner.tsx`

```
Action:    NEW
File:      src/renderer/src/components/tasks/task-rejection-banner.tsx
Solution:  FE-IW-SOL-003 §SOL-3B
```

**Code cần tạo:**

```typescript
// src/renderer/src/components/tasks/task-rejection-banner.tsx
// Banner khi task bị reject bởi quality gate (CR-ORCA-003)
//
// Backend error format từ CompletionHook.OnTaskComplete() (SOL-ORCA-003 §2.5):
//   reason: "convention check failed: %d files missing '// implements: CRS-XXX', %d invalid"
//   action_required: "Add '// implements: CRS-045' comment to each changed file"
//
// parseConventionError() parse exact format backend — không dùng generic regex

import type { FC } from 'react'
import type { Task, TaskRejection } from '../../types/task-types'

interface Props {
  task: Task
}

// Parse backend convention error format (SOL-ORCA-003 §2.5 NewRejectedResponse)
// Format: "convention check failed: %d files missing '// implements: CRS-XXX', %d invalid"
interface ParsedConventionError {
  isConventionFail: boolean
  missingCount?: number
  crsId?: string
}

function parseConventionError(reason: string): ParsedConventionError {
  const match = reason.match(
    /convention check failed: (\d+) files missing '\/\/ implements: ([^']+)'/
  )
  if (!match) return { isConventionFail: false }
  return {
    isConventionFail: true,
    missingCount: parseInt(match[1], 10),
    crsId: match[2],
  }
}

export const TaskRejectionBanner: FC<Props> = ({ task }) => {
  if (task.status !== 'rejected' || !task.rejection) return null

  const { reason, action_required } = task.rejection
  const parsed = parseConventionError(reason)

  return (
    <div
      className="task-rejection-banner"
      role="alert"
      aria-label="Task rejected by quality gate"
      id={`task-rejection-${task.id}`}
    >
      <div className="task-rejection-banner__header">
        <span className="task-rejection-banner__icon" aria-hidden="true">⛔</span>
        <strong className="task-rejection-banner__title">Quality Gate Failed</strong>
      </div>

      {parsed.isConventionFail ? (
        // Structured display khi parse thành công
        <div className="task-rejection-banner__convention">
          <p>
            {parsed.missingCount}{' '}
            file{parsed.missingCount !== 1 ? 's' : ''} missing{' '}
            <code>// implements: {parsed.crsId}</code> comment.
          </p>
        </div>
      ) : (
        // Fallback: raw reason text
        <p className="task-rejection-banner__reason">{reason}</p>
      )}

      {action_required && (
        <div className="task-rejection-banner__action">
          <strong>Action Required:</strong>
          <code className="task-rejection-banner__action-code">{action_required}</code>
        </div>
      )}

      <p className="task-rejection-banner__hint">
        Fix the convention issue and re-submit the task.
      </p>
    </div>
  )
}
```

**Verify:**
- Render null khi `task.status !== 'rejected'`
- Render null khi `task.rejection = undefined`
- `parseConventionError("convention check failed: 2 files missing '// implements: CRS-045', 0 invalid")` → `{ isConventionFail: true, missingCount: 2, crsId: "CRS-045" }`
- `role="alert"`, `id="task-rejection-{task.id}"`

---

## T18 — Patch `useIpcEvents.ts`

```
Action:    EDIT
File:      src/renderer/src/hooks/useIpcEvents.ts
Solution:  FE-IW-SOL-003 §SOL-3C
```

**Thêm 2 event handlers vào useEffect trong useIpcEvents (sau các handlers hiện có):**

```typescript
// ─── CR-ORCA-003: Quality Gate handlers ──────────────────────────────────────

// Task status changed (rejected/done từ CompletionHook)
window.api.onTaskStatusChanged?.((event: {
  taskId: string
  status: TaskStatus
  rejection?: TaskRejection
}) => {
  // Cập nhật task status trong store
  store.updateTaskStatus?.(event.taskId, event.status, event.rejection)

  // Toast chỉ khi quality gate fail (rejected bởi backend CompletionHook)
  if (event.status === 'rejected' && event.rejection) {
    const parsed = parseConventionError(event.rejection.reason)
    showToast({
      type: 'error',
      title: 'Quality Gate Failed',
      // Structured message nếu parse thành công
      message: parsed.isConventionFail
        ? `${parsed.missingCount} file(s) missing // implements: ${parsed.crsId}`
        : event.rejection.reason,
      action: {
        label: 'View Task',
        onClick: () => store.openTask?.(event.taskId),
      },
      // Duration 10s — user cần đọc reason và action (SOL-ORCA-003 §2.5)
      duration: 10_000,
    })
  }
})

// CRS status updated (best-effort từ PISTool.UpdateCRSStatus → "done")
// Backend: products-agent pis_update_crs_status → pikb-catalog PATCH /api/v1/kg/crs/{id}/status
window.api.onCRSStatusUpdated?.((event: {
  crsId: string       // "CRS-045"
  newStatus: string   // "done" (sau CompletionHook)
  updatedBy: string   // "orca-agent"
}) => {
  // Batch update tất cả tasks có crs_id = event.crsId
  const relevantTaskIds = selectTaskIdsByCRSId(store, event.crsId)
  relevantTaskIds.forEach((taskId) => {
    store.updateEnrichmentCRSStatus?.(taskId, event.newStatus)
  })
})
```

**Import cần thêm vào file:**

```typescript
import type { TaskStatus, TaskRejection } from '../types/task-types'
import { selectTaskIdsByCRSId } from '../store/selectors/task-selectors'
// parseConventionError — inline hoặc import từ task-rejection-banner:
function parseConventionError(reason: string) {
  const match = reason.match(
    /convention check failed: (\d+) files missing '\/\/ implements: ([^']+)'/
  )
  if (!match) return { isConventionFail: false }
  return { isConventionFail: true, missingCount: parseInt(match[1], 10), crsId: match[2] }
}
```

**Chú ý AI executor:**
1. Chỉ thêm 2 event handlers — không sửa handlers khác
2. `window.api.onTaskStatusChanged` và `window.api.onCRSStatusUpdated` có thể chưa tồn tại trong window.api type — dùng `?.` optional chaining để không crash
3. `store.updateTaskStatus`, `store.openTask` — adapt theo actual store API
4. `showToast` — dùng existing toast utility trong project

**Verify:**
- `onTaskStatusChanged` handler gọi `showToast` với `duration: 10_000` khi `status='rejected'`
- `onCRSStatusUpdated` handler gọi `updateEnrichmentCRSStatus` cho tất cả matching tasks
- Không break existing event handlers

---

## Checklist Phase 5

- [ ] T17: `task-rejection-banner.tsx` tồn tại
- [ ] T17: `parseConventionError("convention check failed: 2 files missing '// implements: CRS-045', 0 invalid")` → `{isConventionFail: true, missingCount: 2, crsId: "CRS-045"}`
- [ ] T17: Render null khi `task.status !== 'rejected'`
- [ ] T17: `role="alert"` present
- [ ] T18: `useIpcEvents.ts` có `onTaskStatusChanged` handler
- [ ] T18: `useIpcEvents.ts` có `onCRSStatusUpdated` handler
- [ ] T18: `onTaskStatusChanged` toast duration = 10000
- [ ] T18: Existing handlers không bị break
- [ ] TypeScript compile: `npx tsc --noEmit` — 0 errors
