# FE-IW-SOL-003 — Frontend: Quality Gate Rejection UI

**Solution ID:** FE-IW-SOL-003  
**CR Reference:** CR-ORCA-003 (quality-gate-crs-status)  
**Backend Solution:** SOL-ORCA-003  
**TDD Reference:** TDD-FE-02 State Management | TDD-FE-05 UI Components | TDD-FE-07 Hooks & IPC  
**Status:** ✅ Done — implemented 2026-10-19  
**Priority:** P2  
**Effort:** 1–2 ngày  
**Depends On:** FE-IW-SOL-002 (TaskKGContext + TaskEnrichment slice), SOL-ORCA-003 backend

---

## Alignment với Backend Solution

| Backend detail (SOL-ORCA-003) | Frontend implication |
|-------------------------------|----------------------|
| `TaskRejectedResponse` struct: `task_id`, `status: "rejected"`, `reason`, `action_required` | Frontend type mirror JSON field names chính xác |
| `NewRejectedResponse()` format: `"Add '// implements: CRS-XXX' comment to each changed file"` | UI extract CRS ID từ `action_required` text để highlight |
| `ConventionVerifyResult`: `Valid bool`, `Missing []string`, `Invalid []string` | Frontend hiển thị missing files list nếu backend include trong reason |
| Error message format: `"convention check failed: %d files missing '// implements: %s', %d invalid"` | Frontend parse count từ error string để hiển thị structured |
| `CompletionHook.OnTaskComplete()`: best-effort KG update — KG fail không block task | Frontend nhận task.status="done" dù KG update fail → không hiện KG error |
| `PISTool.UpdateCRSStatus()` gọi products-agent với `updated_by: "orca-agent"` | Frontend nhận `onCRSStatusUpdated` event khi KG cập nhật thành công |
| Quality gate reject → task.status = "rejected" | Frontend subscribe task status change event |
| `pis_update_crs_status` tool thêm vào catalog với `IsWrite: true`, `RiskLevel: "medium"` | Frontend **không** gọi tool này trực tiếp — đây là backend-triggered |

---

## Giải pháp

### SOL-3A: Task Rejection Types

```typescript
// src/renderer/src/types/task-types.ts (extend)

// Mirror JSON của TaskRejectedResponse struct (SOL-ORCA-003 §2.5)
export interface TaskRejection {
  task_id: string           // backend: TaskID
  status: 'rejected'        // luôn là "rejected"
  reason: string            // "convention check failed: 1 files missing '// implements: CRS-045', 0 invalid"
  action_required: string   // "Add '// implements: CRS-045' comment to each changed file"
}

export type TaskStatus =
  | 'pending'
  | 'in-progress'
  | 'done'
  | 'rejected'     // ← quality gate fail (SOL-ORCA-003)
  | 'cancelled'

// Extend Task type
export interface Task {
  // ... existing ...
  status: TaskStatus
  rejection?: TaskRejection  // present khi status="rejected"
}
```

---

### SOL-3B: Task Rejection Banner

```typescript
// src/renderer/src/components/tasks/task-rejection-banner.tsx

import type { FC } from 'react'
import type { Task, TaskRejection } from '../../types/task-types'

interface Props {
  task: Task
}

// Parse convention check error để extract structured info
// Backend format: "convention check failed: %d files missing '// implements: CRS-XXX', %d invalid"
function parseConventionError(reason: string): {
  isConventionFail: boolean
  missingCount?: number
  crsId?: string
} {
  const match = reason.match(/convention check failed: (\d+) files missing '\/\/ implements: ([^']+)'/)
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
        <div className="task-rejection-banner__convention">
          <p>
            {parsed.missingCount} file{parsed.missingCount !== 1 ? 's' : ''} missing{' '}
            <code>// implements: {parsed.crsId}</code> comment.
          </p>
        </div>
      ) : (
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

---

### SOL-3C: IPC Event Handler — Task Status Changed

Thêm vào `useIpcEvents` (TDD-FE-07) để handle backend events:

```typescript
// src/renderer/src/hooks/useIpcEvents.ts — EDIT

// CR-ORCA-003: Task status change event từ CompletionHook
window.api.onTaskStatusChanged?.((event: {
  taskId: string
  status: TaskStatus
  rejection?: TaskRejection
}) => {
  // Update task status trong store
  store.updateTaskStatus(event.taskId, event.status, event.rejection)

  // Chỉ toast khi quality gate fail (rejected by backend CompletionHook)
  if (event.status === 'rejected' && event.rejection) {
    const parsed = parseConventionError(event.rejection.reason)
    showToast({
      type: 'error',
      title: 'Quality Gate Failed',
      // Structured message — dùng parsed info nếu có
      message: parsed.isConventionFail
        ? `${parsed.missingCount} file(s) missing // implements: ${parsed.crsId}`
        : event.rejection.reason,
      action: {
        label: 'View Task',
        onClick: () => store.openTask(event.taskId),
      },
      // Duration dài vì user cần đọc reason và action (SOL-ORCA-003 §2.5)
      duration: 10_000,
    })
  }
})

// CR-ORCA-003: CRS status updated (best-effort từ PISTool.UpdateCRSStatus)
// Backend: UpdateCRSStatus gọi products-agent → pikb-catalog PATCH /api/v1/kg/crs/{id}/status
window.api.onCRSStatusUpdated?.((event: {
  crsId: string       // CRS-045
  newStatus: string   // "done" (sau CompletionHook)
  updatedBy: string   // "orca-agent"
}) => {
  // Tìm tất cả tasks có crs_id = event.crsId và update badge
  // Dùng updateEnrichmentCRSStatus từ task-enrichment slice (SOL-002)
  const relevantTaskIds = store.getTaskIdsByCRSId(event.crsId)
  relevantTaskIds.forEach((taskId) => {
    store.updateEnrichmentCRSStatus(taskId, event.newStatus)
  })
})
```

**Tại sao duration 10s**: SOL-ORCA-003 §2.5 `action_required` text dài, user cần đọc và act. 3s quá ngắn.

---

### SOL-3D: CRS Status Badge — Live Update

Sau khi `updateEnrichmentCRSStatus` chạy (từ `onCRSStatusUpdated` event), `TaskKGContextPanel` tự động re-render vì store reactive:

```typescript
// Trong TaskKGContextPanel (SOL-002) — đã đủ, không cần thêm code.
// Khi store.updateEnrichmentCRSStatus(taskId, "done") chạy:
// → taskEnrichments[taskId].crsContext.status = "done"
// → KGStatusBadge render "done" badge ngay lập tức

// Test verify: sau onCRSStatusUpdated → badge thay đổi
```

---

### SOL-3E: Selector — getTaskIdsByCRSId

```typescript
// src/renderer/src/store/selectors/task-selectors.ts

// Tìm taskIds theo crs_id — cần cho onCRSStatusUpdated batch update
export const selectTaskIdsByCRSId = (state: AppState, crsId: string): string[] =>
  Object.values(state.tasks ?? {})
    .filter((task) => task.kg_context?.crs_id === crsId)
    .map((task) => task.id)
```

---

## Files cần tạo/sửa

```
src/renderer/src/
├── types/
│   └── task-types.ts                             ← EDIT: TaskRejection, TaskStatus (+ rejected)
├── hooks/
│   └── useIpcEvents.ts                           ← EDIT: +onTaskStatusChanged, +onCRSStatusUpdated
├── store/
│   ├── slices/
│   │   └── task-enrichment.ts                    ← EDIT: updateEnrichmentCRSStatus đã có (SOL-002)
│   └── selectors/
│       └── task-selectors.ts                     ← NEW/EDIT: selectTaskIdsByCRSId
└── components/
    └── tasks/
        └── task-rejection-banner.tsx              ← NEW
```

---

## Tests

```typescript
// components/tasks/__tests__/task-rejection-banner.test.tsx
describe('TaskRejectionBanner', () => {
  it('render null khi task.status !== "rejected"')
  it('render null khi task.rejection absent')
  it('hiển thị structured convention error (missingCount + crsId) khi reason match format')
  it('fallback render reason text khi không match convention format')
  it('hiển thị action_required với code formatting')
  it('role="alert" và id=task-rejection-{taskId}')
})

// hooks/__tests__/useIpcEvents-quality-gate.test.ts
describe('useIpcEvents — quality gate', () => {
  it('gọi showToast duration=10000 khi status=rejected')
  it('toast message structured khi convention error (missingCount + crsId)')
  it('toast message raw reason khi không phải convention error')
  it('gọi store.updateTaskStatus với rejection payload')
  it('không toast khi status=done')
})

// hooks/__tests__/useIpcEvents-crs-updated.test.ts
describe('useIpcEvents — onCRSStatusUpdated', () => {
  it('gọi updateEnrichmentCRSStatus cho tất cả tasks có crs_id match')
  it('không gọi khi không có task nào match crsId')
})

// store/selectors/__tests__/task-selectors.test.ts
describe('selectTaskIdsByCRSId', () => {
  it('trả về đúng taskIds khi multiple tasks cùng crs_id')
  it('trả về [] khi không có task nào match')
})
```

---

## Acceptance Criteria (Frontend)

| AC | Mô tả | Khớp với Backend AC |
|----|-------|---------------------|
| AC-F1 | `TaskRejection` type mirror JSON: `task_id`, `status`, `reason`, `action_required` | SOL-ORCA-003 §2.5 `TaskRejectedResponse` |
| AC-F2 | Banner parse convention error format backend: "N files missing '// implements: CRS-XXX'" | SOL-ORCA-003 §2.5 `NewRejectedResponse` format |
| AC-F3 | Toast duration=10s khi quality gate fail | CR-ORCA-003 AC-5 requirement |
| AC-F4 | `onCRSStatusUpdated` → `updateEnrichmentCRSStatus` → badge re-render | SOL-ORCA-003 §2.4 best-effort KG update |
| AC-F5 | Banner không render khi status="done" (KG update success path) | SOL-ORCA-003 §2.5 OnTaskComplete success |
| AC-F6 | `selectTaskIdsByCRSId` tìm đúng tasks để batch update | SOL-ORCA-003 — nhiều tasks có thể cùng CRS ref |
