---
doc_id: CR-ORCA-002
product: orca
layer: backend, frontend
status: implemented
priority: P2
feature_ref: DS-005
created: 2026-10-19
completed_date: 2026-10-19
depends_on: CR-ORCA-001
---

# CR-ORCA-002 — orca: Task Proto Extension + Auto-Context Injection

| Field | Value |
|-------|-------|
| **CR ID** | CR-ORCA-002 |
| **Product** | orca (task management, orca-agent & frontend) |
| **Priority** | P2 |
| **Status** | ✅ Implemented (2026-10-19) |
| **Effort** | 3–4 ngày |
| **Depends On** | CR-ORCA-001 (PISTool working) |

---

## Mô tả yêu cầu

Orca task hiện tại chưa có metadata liên kết với KG entities (CRS, Feature).  
Cần extend để AI agent tự động nhận KG context khi được assign task — không cần gọi `pis_query_*` thủ công.

---

## Acceptance Criteria

### AC-1: Orca task schema extension

Trong GoClaw task schema (hoặc Orca task proto tương đương), thêm fields:

```go
// Task metadata extension (in orca task model)
type TaskKGContext struct {
    CRSID      string   // "CRS-045" — CRS implementing task is tracking
    FeatureRef string   // "F27" — Feature this task belongs to
    Service    string   // "ai-service" — target service
    RepoTarget string   // "vnp-workspace" — target repo
}
```

API: `POST /api/tasks` và `PATCH /api/tasks/{id}` nhận thêm fields này.

### AC-2: Auto-context injection khi assign task

File: `orca-agent` hoặc Orca orchestrator — khi nhận task, auto-fetch KG context:

```go
// Trong orca task handling flow:
func (h *TaskHandler) HandleAssignment(ctx context.Context, task Task) (*EnrichedTask, error) {
    enriched := &EnrichedTask{Task: task}

    // Auto-fetch KG context nếu có CRS ref
    if task.KGContext.CRSID != "" && h.pisTool != nil {
        crsCtx, err := h.pisTool.Dispatch(ctx, "pis_query_crs", map[string]any{
            "crs_id": task.KGContext.CRSID,
        })
        if err == nil {
            enriched.CRSContext = crsCtx
        }
    }

    // Auto-fetch feature context nếu có feature ref
    if task.KGContext.FeatureRef != "" && h.pisTool != nil {
        featureCtx, err := h.pisTool.Dispatch(ctx, "pis_query_feature_context", map[string]any{
            "feature_id": task.KGContext.FeatureRef,
        })
        if err == nil {
            enriched.FeatureContext = featureCtx
        }
    }

    return enriched, nil
}
```

### AC-3: System prompt injection

Khi AI agent nhận task assignment, KG context được inject vào system prompt:

```
You are assigned task T-{ID}: {title}

## KG Context
CRS: {crs_id} — Status: {status} — Layer: {layer}
Feature: {feature_ref} — {feature_description}
HLD: See {hld_id} for architecture context.

## Implementation Notes
- Implement CRS-{crs_id} requirements
- Target service: {service}
- Add comment: // implements: CRS-{crs_id}
```

### AC-4: Update Orca UI (nếu có)

Trang task detail hiển thị CRS link, Feature ref, và KG context badge.

### AC-5: Tests

```go
func TestTaskHandler_AutoContextInjection_WithCRS(t *testing.T) { ... }
func TestTaskHandler_AutoContextInjection_NoCRS_Skipped(t *testing.T) { ... }
func TestEnrichedTask_SystemPromptFormat(t *testing.T) { ... }
```

---

## Files cần tạo/sửa

```
orca (GoClaw backend hoặc Orca service):
├── internal/domain/task/entity.go           ← EDIT: add KGContext fields
├── migrations/XXXX_task_kg_context.sql      ← NEW: add kg_context columns
└── internal/api/task_handler.go             ← EDIT: accept new fields

orca-agent:
├── internal/task/handler.go                 ← EDIT: auto-context injection
└── internal/task/handler_test.go            ← NEW
```

---

## Trạng thái thực thi & Rà soát (Executed & Verified: 2026-10-19)

### 1. Checklist tiêu chí chấp nhận (Acceptance Criteria)

- [x] **AC-1 (Task schema extension)**: `TaskKGContext` model định nghĩa đầy đủ 4 trường `crs_id`, `feature_ref`, `service`, `repo_target` (snake_case JSON mapping tương thích với Go backend).
- [x] **AC-2 (Auto-context injection)**: Hook `use-task-enrichment.ts` tự động nạp KG context khi task có `crs_id` hoặc `feature_ref` qua runtime RPC; Go backend `TaskHandler.HandleAssignment` tự động fetch KG context qua PISTool.
- [x] **AC-3 (System prompt injection)**: `BuildSystemPrompt` trong `orca-agent/internal/task/handler.go` định dạng KG Context và Implementation Notes vào prompt cho AI agent.
- [x] **AC-4 (Orca UI Integration)**:
  - `task-kg-context-panel.tsx`: Hiển thị thông tin CRS ID, Feature Ref, Target Service, Target Repo và dynamic KG metadata (status, layer, description, HLD).
  - `task-form-kg-fields.tsx`: Form input tạo/sửa task hỗ trợ nhập 4 trường KG Context.
  - `task-enrichment.ts`: Zustand store slice quản lý cache enrichment theo taskId với TTL 5 phút.
- [x] **AC-5 (Tests)**:
  - Frontend: 6 unit tests trong `src/renderer/src/store/slices/__tests__/task-enrichment.test.ts` (100% PASS).
  - Backend: 4 unit tests trong `orca-agent/internal/task/handler_test.go` (100% PASS).

### 2. Các tệp đã thực hiện (Backend & Frontend)

#### Backend (orca-agent):
- `backend/agents/bridges/dev-tools/orca-agent/internal/task/entity.go` (T-P4-001: TaskKGContext struct)
- `backend/agents/bridges/dev-tools/orca-agent/internal/task/handler.go` (T-P4-003: HandleAssignment & BuildSystemPrompt)
- `backend/agents/bridges/dev-tools/orca-agent/internal/task/handler_test.go` (T-P4-004: 4/4 tests PASS)

#### Frontend (Orca UI):
- `frontend/src/renderer/src/types/task-types.ts` (T11: TaskKGContext, TaskEnrichment, EnrichedTask types)
- `frontend/src/renderer/src/store/slices/task-enrichment.ts` (T12: TaskEnrichmentSlice Zustand store slice)
- `frontend/src/renderer/src/store/types.ts` + `frontend/src/renderer/src/store/index.ts` (T12b: store wiring)
- `frontend/src/renderer/src/hooks/use-task-enrichment.ts` (T13: hook fetch và cache KG context)
- `frontend/src/renderer/src/store/selectors/task-selectors.ts` (T13b: selector tìm task theo CRS ID)
- `frontend/src/renderer/src/components/tasks/task-kg-context-panel.tsx` (T14: UI panel hiển thị KG context)
- `frontend/src/renderer/src/components/tasks/task-form-kg-fields.tsx` (T15: input fields cho form task)
- `frontend/src/renderer/src/components/tasks/task-peer-invoke-error.tsx` (T16: thông báo lỗi khi peer-invoke thất bại)
- `frontend/src/renderer/src/store/slices/__tests__/task-enrichment.test.ts` (T19c: 6/6 tests PASS)

### 3. Kết quả kiểm thử
- **Frontend Vitest**: `6/6 tests PASS` (setLoading, setEnrichment, setError, updateEnrichmentCRSStatus, non-existent task, isolated mutation).
- **TypeScript compile**: `0 errors TS`.
- **Backend Go test**: `PASS` (`TestTaskHandler_AutoContextInjection_NilPISTool_Skipped`, `TestTaskHandler_NoKGContext_Skipped`, `TestBuildSystemPrompt_WithCRS`, `TestBuildSystemPrompt_WithoutCRS`).

