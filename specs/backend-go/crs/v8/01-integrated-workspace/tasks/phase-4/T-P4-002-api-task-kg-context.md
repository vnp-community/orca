---
task_id: T-P4-002
phase: 4
cr_id: CR-ORCA-002
solution_ref: solutions/CR-ORCA-002-solution.md
service: orca-backend
target_file: internal/api/task_handler.go
priority: P2
status: done
depends_on: T-P4-001
estimated_effort: 1 giờ
---

# T-P4-002 — Cập nhật API task handler để nhận kg_context fields

## Context

`POST /api/tasks` và `PATCH /api/tasks/{id}` cần accept `kg_context` trong request body để client có thể set CRS tracking khi tạo/update task.

## Target file

```
internal/api/task_handler.go
```

## Instructions

1. **Đọc file hiện tại** để xem `CreateTaskRequest` và `PatchTaskRequest` structs

2. **Thêm `KGContext` vào `CreateTaskRequest`**:
   ```go
   type CreateTaskRequest struct {
       Title       string               `json:"title"`
       Description string               `json:"description"`
       // ... existing fields ...
       KGContext   task.TaskKGContext   `json:"kg_context,omitempty"` // CR-ORCA-002
   }
   ```

3. **Thêm `KGContext` vào `PatchTaskRequest`** (pointer vì optional):
   ```go
   type PatchTaskRequest struct {
       // ... existing fields ...
       KGContext *task.TaskKGContext `json:"kg_context,omitempty"` // CR-ORCA-002
   }
   ```

4. **Đảm bảo `KGContext` được map** từ request vào domain `Task` entity khi create/update:
   ```go
   // Trong handler logic — khi build Task từ request:
   if req.KGContext.CRSID != "" || req.KGContext.FeatureRef != "" {
       t.KGContext = req.KGContext
   }
   ```

## Acceptance criteria

- [ ] `CreateTaskRequest` có `KGContext task.TaskKGContext` field
- [ ] `PatchTaskRequest` có `KGContext *task.TaskKGContext` field
- [ ] `KGContext` được persist vào DB khi create task với crs_id
- [ ] `GOWORK=off go build ./internal/api/...` thành công

## Test command

```bash
GOWORK=off go build ./internal/api/...
```
