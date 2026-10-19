---
cr_id: CR-ORCA-002
solution_id: SOL-ORCA-002
product: orca
service: orca-agent, orca-backend
layer: backend
language: go
status: implemented
priority: P2
effort: 3–4 ngày
depends_on: CR-ORCA-001
created: 2026-10-19
---

# SOL-ORCA-002 — orca: Task Proto Extension + Auto-Context Injection

## 1. Phân tích vấn đề

Orca task hiện tại thiếu metadata liên kết với PIKB KG entities (CRS, Feature). Hệ quả:
- AI agent phải gọi `pis_query_*` thủ công mỗi khi cần context → lãng phí token, dễ bỏ sót
- Không có cơ chế tự động bơm KG context vào system prompt khi agent nhận task
- Vòng lặp CR tracking bị gián đoạn: không biết task đang implement CRS nào

---

## 2. Giải pháp kỹ thuật

### 2.1 Kiến trúc tổng thể

```
POST /api/tasks  {crs_id: "CRS-045", feature_ref: "F27", ...}
  → task-service: lưu KGContext vào DB
  → orca-agent nhận task assignment event
  → TaskHandler.HandleAssignment()
      ├─ pisTool.Dispatch("pis_query_crs", {crs_id: "CRS-045"})
      ├─ pisTool.Dispatch("pis_query_feature_context", {feature_id: "F27"})
      └─ inject KG context vào system prompt → AI agent bắt đầu implement
```

### 2.2 Domain model — `internal/domain/task/entity.go` (EDIT)

```go
package task

// TaskKGContext chứa metadata liên kết task với PIKB KG entities.
// Lưu dưới dạng JSONB column trong DB. CR-ORCA-002.
type TaskKGContext struct {
    CRSID      string `json:"crs_id,omitempty"`      // "CRS-045"
    FeatureRef string `json:"feature_ref,omitempty"` // "F27"
    Service    string `json:"service,omitempty"`     // "ai-service"
    RepoTarget string `json:"repo_target,omitempty"` // "vnp-workspace"
}

// Task entity — thêm KGContext field
type Task struct {
    ID          string
    Title       string
    Description string
    Status      TaskStatus
    KGContext   TaskKGContext `json:"kg_context,omitempty"`
    // ... other existing fields ...
}
```

### 2.3 DB migration — `migrations/XXXX_task_kg_context.sql` (NEW)

```sql
-- CR-ORCA-002: add kg_context JSONB column to tasks table
ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS kg_context JSONB NOT NULL DEFAULT '{}';

-- Index để query theo crs_id
CREATE INDEX IF NOT EXISTS idx_tasks_kg_context_crs_id
    ON tasks ((kg_context->>'crs_id'))
    WHERE kg_context->>'crs_id' IS NOT NULL;

COMMENT ON COLUMN tasks.kg_context IS
    'PIKB KG context: crs_id, feature_ref, service, repo_target (CR-ORCA-002)';
```

### 2.4 API layer — `internal/api/task_handler.go` (EDIT)

```go
// CreateTaskRequest — thêm KGContext fields
type CreateTaskRequest struct {
    Title       string          `json:"title"`
    Description string          `json:"description"`
    KGContext   task.TaskKGContext `json:"kg_context,omitempty"`
    // ... existing fields ...
}

// PatchTaskRequest — thêm KGContext
type PatchTaskRequest struct {
    KGContext *task.TaskKGContext `json:"kg_context,omitempty"`
    // ... existing fields ...
}
```

### 2.5 Auto-context injection — `internal/task/handler.go` (EDIT trong orca-agent)

```go
package task

import (
    "context"
    "log"
    "github.com/vnp-blc/orca/orca-agent/internal/pis"
)

// EnrichedTask là Task + KG context đã fetch từ PIKB.
type EnrichedTask struct {
    Task
    CRSContext     any // raw JSON từ pis_query_crs
    FeatureContext any // raw JSON từ pis_query_feature_context
}

// TaskHandler xử lý task assignment với auto-context injection.
type TaskHandler struct {
    pisTool *pis.PISTool // CR-ORCA-002: optional
}

// HandleAssignment enriches task với KG context trước khi dispatch cho AI agent.
// Nếu pisTool nil hoặc KGContext rỗng → skip enrichment, không fail.
func (h *TaskHandler) HandleAssignment(ctx context.Context, t Task) (*EnrichedTask, error) {
    enriched := &EnrichedTask{Task: t}

    if h.pisTool == nil {
        return enriched, nil // degraded mode — no KG context
    }

    // Fetch CRS context nếu có crs_id
    if t.KGContext.CRSID != "" {
        crsCtx, err := h.pisTool.Dispatch(ctx, "pis_query_crs", map[string]any{
            "crs_id": t.KGContext.CRSID,
        })
        if err != nil {
            // Best-effort: log warning nhưng không fail task assignment
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

### 2.6 System prompt injection template

```go
// BuildSystemPrompt tạo system prompt cho AI agent với KG context.
// CR-ORCA-002: KG context inject tự động khi task có crs_id/feature_ref.
func BuildSystemPrompt(t *EnrichedTask) string {
    var b strings.Builder
    fmt.Fprintf(&b, "You are assigned task T-%s: %s\n\n", t.ID, t.Title)

    if t.KGContext.CRSID != "" {
        b.WriteString("## KG Context\n")
        if t.CRSContext != nil {
            fmt.Fprintf(&b, "CRS: %s (see attached context)\n", t.KGContext.CRSID)
        } else {
            fmt.Fprintf(&b, "CRS: %s\n", t.KGContext.CRSID)
        }
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

---

## 3. File map

| File | Action | Ghi chú |
|------|--------|---------|
| `internal/domain/task/entity.go` | **EDIT** | +`TaskKGContext` struct, +`KGContext` field trong `Task` |
| `migrations/XXXX_task_kg_context.sql` | **NEW** | ADD COLUMN kg_context JSONB + index |
| `internal/api/task_handler.go` | **EDIT** | Accept `kg_context` trong Create/Patch |
| `orca-agent/internal/task/handler.go` | **EDIT** | `HandleAssignment` + auto-context injection |
| `orca-agent/internal/task/prompt.go` | **NEW** | `BuildSystemPrompt()` |
| `orca-agent/internal/task/handler_test.go` | **NEW** | 3 test cases |

---

## 4. Test strategy

```go
// orca-agent/internal/task/handler_test.go

func TestTaskHandler_AutoContextInjection_WithCRS(t *testing.T) {
    mockPIS := &mockPISTool{returnData: map[string]any{"id": "CRS-045", "status": "pending"}}
    handler := &TaskHandler{pisTool: mockPIS}

    task := Task{ID: "T-100", KGContext: TaskKGContext{CRSID: "CRS-045"}}
    enriched, err := handler.HandleAssignment(ctx, task)

    assert.NoError(t, err)
    assert.NotNil(t, enriched.CRSContext)
    assert.Equal(t, "pis_query_crs", mockPIS.lastTool)
}

func TestTaskHandler_AutoContextInjection_NoCRS_Skipped(t *testing.T) {
    mockPIS := &mockPISTool{}
    handler := &TaskHandler{pisTool: mockPIS}

    task := Task{ID: "T-101"} // no KGContext
    enriched, err := handler.HandleAssignment(ctx, task)

    assert.NoError(t, err)
    assert.Nil(t, enriched.CRSContext)
    assert.Empty(t, mockPIS.lastTool) // pisTool.Dispatch never called
}

func TestEnrichedTask_SystemPromptFormat(t *testing.T) {
    enriched := &EnrichedTask{
        Task:       Task{ID: "T-100", Title: "Implement auth"},
        CRSContext: map[string]any{"id": "CRS-045"},
    }
    enriched.KGContext = TaskKGContext{CRSID: "CRS-045", Service: "auth-service"}

    prompt := BuildSystemPrompt(enriched)
    assert.Contains(t, prompt, "T-100")
    assert.Contains(t, prompt, "CRS-045")
    assert.Contains(t, prompt, "// implements: CRS-045")
    assert.Contains(t, prompt, "auth-service")
}
```

---

## 5. Risks & mitigations

| Risk | Mitigation |
|------|-----------|
| KG fetch thêm latency vào task assignment | Fetch concurrently (CRS + feature parallel), timeout 3s |
| KG unavailable → task bị block | Best-effort: log warning, tiếp tục với `EnrichedTask` không có context |
| JSONB schema thay đổi | Migration idempotent với `IF NOT EXISTS` |

---

## 6. UI consideration (ngoài scope backend)

Frontend task detail nên hiển thị:
- CRS badge: `[CRS-045]` → link đến PIKB
- Feature ref: `[F27]` → link đến feature page
- KG context panel (collapsible)

---

## 7. Execution order

```
CR-ORCA-001 (PISTool working)
  → CR-ORCA-002 (Task KGContext + auto-inject)  ← file này
```

---

## Execution Log — 2026-10-19

**Status**: 🔶 partially_implemented

### Files changed (orca-agent side — ✅ done)
- [`task/entity.go`](../../../../vnp-workspace/backend/agents/bridges/dev-tools/orca-agent/internal/task/entity.go): `TaskKGContext`, `Task`, `TaskStatus`
- [`task/handler.go`](../../../../vnp-workspace/backend/agents/bridges/dev-tools/orca-agent/internal/task/handler.go): `HandleAssignment()`, `BuildSystemPrompt()`, `SetCompletionHook()`
- [`task/handler_test.go`](../../../../vnp-workspace/backend/agents/bridges/dev-tools/orca-agent/internal/task/handler_test.go): 4 tests (PASS)

### Files changed (platform services — ✅ done)
- `mcp-service/httpclient/ai_service_client.go`: +`OrcaTaskKGContext`, +`KGContext` field in `OrcaDispatchInput`
- `mcp-service/tools/orca_tools.go`: +`kg_context` in `dispatch_orca_task` schema
- `mcp-service/tools/orca_tool_handler.go`: +`kg_context` extraction
- `workspace-context-service/registries/orca.go`: +`CreateTaskWithKGContext()`

### Deferred (orca-backend API)
- `POST /api/tasks` needs to accept + persist `kg_context` field in DB — requires orca-backend repo not in current workspace scope

### Verification
```
GOWORK=off go test ./internal/task/... → 10 PASS
GOWORK=off go test ./internal/application/tools/... → ALL PASS (mcp-service)
```
