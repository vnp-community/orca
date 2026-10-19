---
task_id: T-P4-001
phase: 4
cr_id: CR-ORCA-002
solution_ref: solutions/CR-ORCA-002-solution.md
service: orca-agent / orca-backend
target_files:
  - internal/domain/task/entity.go
  - migrations/XXXX_task_kg_context.sql
priority: P2
status: done
depends_on: T-P3-003
estimated_effort: 2 giờ
blocks: T-P4-002, T-P4-003
---

# T-P4-001 — Thêm TaskKGContext vào domain model + DB migration

## Context

Task hiện tại không có metadata liên kết với PIKB KG entities. Cần extend domain model để lưu `crs_id`, `feature_ref`, `service`, `repo_target` — phục vụ auto-context injection khi AI agent nhận task.

## Target files

```
internal/domain/task/entity.go
migrations/XXXX_task_kg_context.sql   ← tên thật: timestamp prefix, vd: 20261019_task_kg_context.sql
```

## Instructions

### 1. `internal/domain/task/entity.go`

1. Đọc file hiện tại để xem struct `Task` đang có gì

2. **Thêm `TaskKGContext` struct** (tạo file mới hoặc thêm vào entity.go):
   ```go
   // TaskKGContext chứa metadata liên kết task với PIKB KG entities.
   // Lưu dưới dạng JSONB column trong DB (CR-ORCA-002).
   type TaskKGContext struct {
       CRSID      string `json:"crs_id,omitempty"`      // VD: "CRS-045"
       FeatureRef string `json:"feature_ref,omitempty"` // VD: "F27"
       Service    string `json:"service,omitempty"`     // VD: "ai-service"
       RepoTarget string `json:"repo_target,omitempty"` // VD: "vnp-workspace"
   }
   ```

3. **Thêm `KGContext` field** vào `Task` struct:
   ```go
   KGContext TaskKGContext `json:"kg_context,omitempty"`
   ```

### 2. Migration file

1. Tìm thư mục migration (thường là `migrations/` hoặc `db/migrations/`)

2. Tạo file migration mới với naming convention phù hợp (xem các file hiện có để biết format):
   ```sql
   -- CR-ORCA-002: add kg_context JSONB column to tasks table
   -- Stores CRS tracking metadata for AI agent context injection.
   ALTER TABLE tasks
       ADD COLUMN IF NOT EXISTS kg_context JSONB NOT NULL DEFAULT '{}';

   -- Index để query tasks theo crs_id
   CREATE INDEX IF NOT EXISTS idx_tasks_kg_context_crs_id
       ON tasks ((kg_context->>'crs_id'))
       WHERE kg_context->>'crs_id' IS NOT NULL;

   COMMENT ON COLUMN tasks.kg_context IS
       'PIKB KG context: crs_id, feature_ref, service, repo_target (CR-ORCA-002)';
   ```

## Acceptance criteria

- [ ] `TaskKGContext` struct tồn tại với 4 fields: `CRSID`, `FeatureRef`, `Service`, `RepoTarget`
- [ ] `Task` struct có field `KGContext TaskKGContext`
- [ ] Migration file tồn tại và hợp lệ SQL
- [ ] Migration dùng `IF NOT EXISTS` để idempotent
- [ ] `GOWORK=off go build ./internal/domain/task/...` thành công

## Test command

```bash
cd backend/agents/bridges/dev-tools/orca-agent  # hoặc thư mục service phù hợp
GOWORK=off go build ./internal/domain/...
```
