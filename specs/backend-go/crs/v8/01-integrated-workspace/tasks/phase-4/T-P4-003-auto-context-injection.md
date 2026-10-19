---
task_id: T-P4-003
phase: 4
cr_id: CR-ORCA-002
solution_ref: solutions/CR-ORCA-002-solution.md
service: orca-agent
target_files:
  - orca-agent/internal/task/handler.go
  - orca-agent/internal/task/prompt.go
priority: P2
status: done
depends_on: T-P4-001, T-P3-003
estimated_effort: 3 giờ
blocks: T-P4-004
---

# T-P4-003 — Implement auto-context injection trong TaskHandler

## Context

Khi AI agent nhận task assignment có `crs_id` hoặc `feature_ref`, hệ thống nên **tự động fetch** KG context từ PIKB và inject vào system prompt — không cần AI agent tự gọi `pis_query_*`. Điều này giảm token waste và đảm bảo context luôn đầy đủ.

## Target files

```
orca-agent/internal/task/handler.go   ← EDIT hoặc tạo mới
orca-agent/internal/task/prompt.go    ← NEW FILE
```

## Instructions

### 1. `orca-agent/internal/task/handler.go`

1. Tạo hoặc mở file. Nếu đã có `TaskHandler` struct, thêm vào; nếu chưa có, tạo mới.

2. **Định nghĩa `EnrichedTask`**:
   ```go
   // EnrichedTask là Task + KG context đã được fetch từ PIKB KG.
   type EnrichedTask struct {
       Task
       CRSContext     json.RawMessage // raw JSON từ pis_query_crs
       FeatureContext json.RawMessage // raw JSON từ pis_query_feature_context
   }
   ```

3. **Định nghĩa `TaskHandler`** (hoặc thêm field vào struct hiện có):
   ```go
   // TaskHandler xử lý task assignment với auto-context injection.
   type TaskHandler struct {
       pisTool *pis.PISTool // CR-ORCA-002: optional, nil → skip enrichment
   }

   func NewTaskHandler(pisTool *pis.PISTool) *TaskHandler {
       return &TaskHandler{pisTool: pisTool}
   }
   ```

4. **Implement `HandleAssignment()`**:
   ```go
   // HandleAssignment enriches task với KG context trước khi dispatch cho AI agent.
   // Nếu pisTool nil hoặc KGContext rỗng → skip enrichment, không fail.
   // CR-ORCA-002: KG fetch là best-effort — task không bị block nếu KG unavailable.
   func (h *TaskHandler) HandleAssignment(ctx context.Context, t Task) (*EnrichedTask, error) {
       enriched := &EnrichedTask{Task: t}
       if h.pisTool == nil {
           return enriched, nil
       }

       // Fetch CRS context nếu có crs_id
       if t.KGContext.CRSID != "" {
           crsCtx, err := h.pisTool.Dispatch(ctx, "pis_query_crs", map[string]any{
               "crs_id": t.KGContext.CRSID,
           })
           if err != nil {
               log.Printf("[task-handler] KG crs fetch failed for %s: %v", t.KGContext.CRSID, err)
           } else {
               enriched.CRSContext = crsCtx
           }
       }

       // Fetch feature context nếu có feature_ref
       if t.KGContext.FeatureRef != "" {
           featureCtx, err := h.pisTool.Dispatch(ctx, "pis_query_feature_context", map[string]any{
               "feature_id": t.KGContext.FeatureRef,
           })
           if err != nil {
               log.Printf("[task-handler] KG feature fetch failed for %s: %v", t.KGContext.FeatureRef, err)
           } else {
               enriched.FeatureContext = featureCtx
           }
       }

       return enriched, nil
   }
   ```

### 2. `orca-agent/internal/task/prompt.go` (NEW)

```go
package task

import (
    "fmt"
    "strings"
)

// BuildSystemPrompt tạo system prompt cho AI agent với KG context inject.
// CR-ORCA-002: KG context được inject tự động khi task có crs_id/feature_ref.
func BuildSystemPrompt(t *EnrichedTask) string {
    var b strings.Builder
    fmt.Fprintf(&b, "You are assigned task T-%s: %s\n\n", t.ID, t.Title)

    if t.KGContext.CRSID != "" {
        b.WriteString("## KG Context\n")
        fmt.Fprintf(&b, "CRS: %s\n", t.KGContext.CRSID)
        if t.KGContext.FeatureRef != "" {
            fmt.Fprintf(&b, "Feature: %s\n", t.KGContext.FeatureRef)
        }
        b.WriteString("\n## Implementation Notes\n")
        fmt.Fprintf(&b, "- Implement requirements từ %s\n", t.KGContext.CRSID)
        if t.KGContext.Service != "" {
            fmt.Fprintf(&b, "- Target service: %s\n", t.KGContext.Service)
        }
        fmt.Fprintf(&b, "- Add comment to each changed file: // implements: %s\n", t.KGContext.CRSID)
    }

    return b.String()
}
```

## Acceptance criteria

- [ ] `EnrichedTask` struct tồn tại với `CRSContext` và `FeatureContext` fields
- [ ] `HandleAssignment()` gọi `pis_query_crs` khi `KGContext.CRSID != ""`
- [ ] `HandleAssignment()` gọi `pis_query_feature_context` khi `KGContext.FeatureRef != ""`
- [ ] Khi KG fetch fail → log warning, `EnrichedTask` trả về với field nil, không return error
- [ ] `BuildSystemPrompt()` tạo prompt có chứa `// implements: CRS-XXX`
- [ ] `GOWORK=off go build ./internal/task/...` thành công

## Test command

```bash
cd backend/agents/bridges/dev-tools/orca-agent
GOWORK=off go build ./internal/task/...
```
