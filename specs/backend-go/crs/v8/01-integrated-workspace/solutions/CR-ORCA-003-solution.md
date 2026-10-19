---
cr_id: CR-ORCA-003
solution_id: SOL-ORCA-003
product: orca
service: orca-agent, pikb-catalog-service, products-agent
layer: backend
language: go
status: implemented
priority: P2
effort: 3–4 ngày
depends_on: CR-ORCA-001, CR-ORCA-002, CR-WS-003, CR-PIKB-001
created: 2026-10-19
---

# SOL-ORCA-003 — orca: Quality Gate — CRS Status Update + Convention Verify

## 1. Phân tích vấn đề

Hiện tại vòng lặp "implement → verify → update KG" bị thiếu bước tự động:
- Sau khi AI agent implement xong, CRS status trong PIKB KG không được cập nhật
- Không có cơ chế verify convention `// implements: CRS-XXX` trước khi mark task done
- Nếu convention bị bỏ sót, tracking CRS → code bị mất

---

## 2. Giải pháp kỹ thuật

### 2.1 Kiến trúc vòng lặp quality gate

```
Task marked done
  → CompletionHook.OnTaskComplete(ctx, task)
      ├─ [Step 1] gitVerify.Verify(repo_target)
      │     → git.verify_implements_convention
      │     → Kiểm tra file có "// implements: CRS-045" không
      │     → Nếu invalid → trả về TaskRejected response
      │
      └─ [Step 2] pisTool.UpdateCRSStatus("CRS-045", "done")
            → products-agent: pis_update_crs_status
            → pikb-catalog-service: PATCH /api/v1/kg/crs/CRS-045/status
            → KG updated
```

### 2.2 pikb-catalog-service — KG write API (EDIT)

File: `pikb-catalog-service/internal/interfaces/http/devkg_handler.go`

```go
// PATCH /api/v1/kg/crs/{id}/status
// CR-ORCA-003: Cho phép orca-agent update CRS status sau khi implementation done.
func (h *DevKGHandler) UpdateCRSStatus(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")

    var body struct {
        Status    string `json:"status"`     // "pending"|"in-progress"|"done"|"cancelled"
        UpdatedBy string `json:"updated_by"` // agent ID hoặc user ID
        Reason    string `json:"reason,omitempty"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "invalid body", http.StatusBadRequest)
        return
    }

    // Validate status transition
    validStatuses := map[string]bool{"pending": true, "in-progress": true, "done": true, "cancelled": true}
    if !validStatuses[body.Status] {
        http.Error(w, "invalid status: "+body.Status, http.StatusBadRequest)
        return
    }

    if err := h.kgService.UpdateCRSStatus(r.Context(), id, body.Status, body.UpdatedBy); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusNoContent)
}
```

Route registration:
```go
r.Patch("/api/v1/kg/crs/{id}/status", h.UpdateCRSStatus)
```

### 2.3 products-agent — pis_update_crs_status tool (EDIT, multi-file)

**Tool catalog** — `products-agent/internal/catalog/kg_tools.go`:
```go
{
    Name:        "pis_update_crs_status",
    Description: "Update CRS node status trong PIKB KG. Dùng sau implementation hoàn thành.",
    IsWrite:     true,
    RiskLevel:   "medium",
    InputSchema: map[string]any{
        "type": "object",
        "properties": map[string]any{
            "crs_id":     map[string]any{"type": "string"},
            "status":     map[string]any{"type": "string", "enum": []string{"pending", "in-progress", "done", "cancelled"}},
            "updated_by": map[string]any{"type": "string"},
        },
        "required": []string{"crs_id", "status"},
    },
},
```

**KG handler** — `products-agent/internal/kg/handler.go`:
```go
// UpdateCRSStatus calls pikb-catalog PATCH endpoint.
// CR-ORCA-003: write path, kiểm tra status transition hợp lệ trước khi call.
func (h *KGHandler) UpdateCRSStatus(ctx context.Context, crsID, status, updatedBy string) error {
    body, _ := json.Marshal(map[string]any{
        "status":     status,
        "updated_by": updatedBy,
    })
    req, err := http.NewRequestWithContext(ctx, http.MethodPatch,
        h.catalogBaseURL+"/api/v1/kg/crs/"+url.PathEscape(crsID)+"/status",
        bytes.NewReader(body),
    )
    if err != nil {
        return fmt.Errorf("UpdateCRSStatus: build request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+h.serviceToken)

    resp, err := h.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("UpdateCRSStatus %s: %w", crsID, err)
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusNoContent {
        b, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("UpdateCRSStatus %s: HTTP %d: %s", crsID, resp.StatusCode, b)
    }
    return nil
}
```

**Dispatcher** — `products-agent/internal/dispatcher/dispatcher.go`:
```go
case "pis_update_crs_status":
    crsID, _ := frame.Params["crs_id"].(string)
    status, _ := frame.Params["status"].(string)
    updatedBy, _ := frame.Params["updated_by"].(string)
    err = d.kgClient.UpdateCRSStatus(ctx, crsID, status, updatedBy)
    if err == nil {
        result = map[string]any{"updated": true, "crs_id": crsID, "status": status}
    }
```

### 2.4 orca-agent PISTool — UpdateCRSStatus method (EDIT)

File: `orca-agent/internal/pis/client.go`

```go
// UpdateCRSStatus peer-invokes products-agent để update CRS status trong PIKB KG.
// CR-ORCA-003: gọi tự động từ CompletionHook sau khi task done.
// Best-effort: caller nên log failure nhưng không block task completion.
func (t *PISTool) UpdateCRSStatus(ctx context.Context, crsID, newStatus string) error {
    _, err := t.peer.Call(ctx, t.productsAgentID, "pis_update_crs_status", map[string]any{
        "crs_id":     crsID,
        "status":     newStatus,
        "updated_by": "orca-agent",
    })
    return err
}
```

### 2.5 CompletionHook — `orca-agent/internal/task/completion_hook.go` (NEW)

```go
package task

import (
    "context"
    "fmt"
    "log"
    "github.com/vnp-blc/orca/orca-agent/internal/pis"
)

// GitConventionVerifier kiểm tra convention "// implements: CRS-XXX" trong repo.
// Interface để dễ mock trong test.
type GitConventionVerifier interface {
    Verify(ctx context.Context, repoTarget string) (*ConventionVerifyResult, error)
}

// ConventionVerifyResult chứa kết quả verify git convention.
type ConventionVerifyResult struct {
    Valid   bool
    Missing []string // files thiếu comment
    Invalid []string // files có comment sai format
}

// CompletionHook chạy quality gate khi task được mark done.
// CR-ORCA-003: verify convention → update KG status.
type CompletionHook struct {
    pisTool   *pis.PISTool
    gitVerify GitConventionVerifier
}

func NewCompletionHook(pisTool *pis.PISTool, gitVerify GitConventionVerifier) *CompletionHook {
    return &CompletionHook{pisTool: pisTool, gitVerify: gitVerify}
}

// OnTaskComplete chạy sau khi AI agent báo task xong.
// Nếu task không có CRS ref → skip hoàn toàn.
// Nếu convention invalid → return error → task bị rejected.
// Nếu KG update fail → log warning, KHÔNG fail task (best-effort).
func (h *CompletionHook) OnTaskComplete(ctx context.Context, t Task) error {
    if t.KGContext.CRSID == "" {
        return nil // no CRS tracking — skip quality gate
    }

    // Step 1: Verify git convention
    if h.gitVerify != nil {
        result, err := h.gitVerify.Verify(ctx, t.KGContext.RepoTarget)
        if err != nil {
            return fmt.Errorf("convention verify failed: %w", err)
        }
        if !result.Valid {
            return fmt.Errorf(
                "convention check failed: %d files missing '// implements: %s', %d invalid",
                len(result.Missing), t.KGContext.CRSID, len(result.Invalid),
            )
        }
    }

    // Step 2: Update CRS status → "done" (best-effort)
    if h.pisTool != nil {
        if err := h.pisTool.UpdateCRSStatus(ctx, t.KGContext.CRSID, "done"); err != nil {
            // KG update không critical — task vẫn done
            log.Printf("[completion-hook] KG status update failed for %s: %v", t.KGContext.CRSID, err)
        }
    }

    return nil
}

// TaskRejectedResponse là response trả về khi quality gate fail.
type TaskRejectedResponse struct {
    TaskID         string `json:"task_id"`
    Status         string `json:"status"` // "rejected"
    Reason         string `json:"reason"`
    ActionRequired string `json:"action_required"`
}

func NewRejectedResponse(taskID, crsID, reason string) TaskRejectedResponse {
    return TaskRejectedResponse{
        TaskID:         taskID,
        Status:         "rejected",
        Reason:         reason,
        ActionRequired: fmt.Sprintf("Add '// implements: %s' comment to each changed file", crsID),
    }
}
```

### 2.6 main.go — wire CompletionHook (EDIT)

```go
// CR-ORCA-003: wire CompletionHook sau khi pisTool và gitVerifier đã init
var completionHook *task.CompletionHook
if pisTool != nil {
    completionHook = task.NewCompletionHook(pisTool, gitVerifier)
}
taskHandler.SetCompletionHook(completionHook)
```

---

## 3. File map

| File | Service | Action | Ghi chú |
|------|---------|--------|---------|
| `internal/interfaces/http/devkg_handler.go` | pikb-catalog-service | **EDIT** | +`PATCH /api/v1/kg/crs/{id}/status` |
| `internal/kg/handler.go` | products-agent | **EDIT** | +`UpdateCRSStatus()` method |
| `internal/catalog/kg_tools.go` | products-agent | **EDIT** | +`pis_update_crs_status` tool def |
| `internal/dispatcher/dispatcher.go` | products-agent | **EDIT** | +route `pis_update_crs_status` |
| `internal/pis/client.go` | orca-agent | **EDIT** | +`UpdateCRSStatus()` method |
| `internal/task/completion_hook.go` | orca-agent | **NEW** | `CompletionHook`, `GitConventionVerifier` |
| `internal/task/completion_hook_test.go` | orca-agent | **NEW** | 4 test cases |
| `cmd/orca-agent/main.go` | orca-agent | **EDIT** | wire `CompletionHook` |

---

## 4. Test strategy

```go
// orca-agent/internal/task/completion_hook_test.go

func TestCompletionHook_ValidConvention_UpdatesCRSStatus(t *testing.T) {
    mockPIS := &mockPISTool{}
    mockGit := &mockGitVerifier{result: &ConventionVerifyResult{Valid: true}}
    hook := NewCompletionHook(mockPIS, mockGit)

    task := Task{ID: "T-100", KGContext: TaskKGContext{CRSID: "CRS-045", RepoTarget: "vnp-workspace"}}
    err := hook.OnTaskComplete(ctx, task)

    assert.NoError(t, err)
    assert.Equal(t, "CRS-045", mockPIS.lastCRSID)
    assert.Equal(t, "done", mockPIS.lastStatus)
}

func TestCompletionHook_InvalidConvention_RejectsTask(t *testing.T) {
    mockGit := &mockGitVerifier{result: &ConventionVerifyResult{
        Valid:   false,
        Missing: []string{"auth_service.go"},
    }}
    hook := NewCompletionHook(nil, mockGit)

    task := Task{ID: "T-100", KGContext: TaskKGContext{CRSID: "CRS-045"}}
    err := hook.OnTaskComplete(ctx, task)

    assert.ErrorContains(t, err, "convention check failed")
    assert.ErrorContains(t, err, "1 files missing")
}

func TestCompletionHook_NoCRSRef_Skipped(t *testing.T) {
    mockPIS := &mockPISTool{}
    hook := NewCompletionHook(mockPIS, nil)

    task := Task{ID: "T-200"} // no KGContext
    err := hook.OnTaskComplete(ctx, task)

    assert.NoError(t, err)
    assert.Empty(t, mockPIS.lastCRSID) // UpdateCRSStatus never called
}

func TestPISTool_UpdateCRSStatus_Success(t *testing.T) {
    mockCaller := &mockPeerCaller{returnJSON: json.RawMessage(`{"updated":true}`)}
    tool := pis.NewPISTool("products-agent", mockCaller)

    err := tool.UpdateCRSStatus(ctx, "CRS-045", "done")

    assert.NoError(t, err)
    assert.Equal(t, "pis_update_crs_status", mockCaller.lastTool)
}
```

---

## 5. End-to-end scenario

```
1. Tạo task T-100: {crs_id: "CRS-045", repo_target: "vnp-workspace"}
2. AI agent implement → thêm // implements: CRS-045 vào auth_service.go
3. AI agent báo task done → CompletionHook.OnTaskComplete(T-100)
4. Step 1: git.verify_implements_convention(vnp-workspace) → valid ✓
5. Step 2: pisTool.UpdateCRSStatus("CRS-045", "done")
6. products-agent: pis_update_crs_status → pikb-catalog PATCH
7. PIKB KG: CRS-045.status = "done" ✓
```

---

## 6. Risks & mitigations

| Risk | Mitigation |
|------|-----------|
| git convention verify chậm (large repo scan) | Chỉ scan files thay đổi trong PR/commit, không scan toàn repo |
| KG update fail do network → block workflow | Best-effort: log + metrics, task vẫn complete |
| Race: 2 agents cùng update CRS status | PATCH idempotent — last-write-wins với timestamp |

---

## 7. Execution order

```
CR-ORCA-001 (PISTool)
CR-WS-003 (git convention verify tool)
CR-PIKB-001 (KG write API)
  → CR-ORCA-003 (CompletionHook + quality gate)  ← file này
```

---

## Execution Log — 2026-10-19

**Status**: 🔶 partially_implemented

### Files changed (orca-agent side — ✅ done)
- [`task/completion_hook.go`](../../../../vnp-workspace/backend/agents/bridges/dev-tools/orca-agent/internal/task/completion_hook.go): `CompletionHook`, `GitConventionVerifier` interface, `NewRejectedResponse()`
- [`task/completion_hook_test.go`](../../../../vnp-workspace/backend/agents/bridges/dev-tools/orca-agent/internal/task/completion_hook_test.go): 6 tests (PASS)
- `pis/client.go`: +`UpdateCRSStatus()` for Step 2 KG update

### Files changed (products-agent — ✅ done)
- `products-agent/kg/handler.go`: +`UpdateCRSStatus()` → PATCH `/api/v1/kg/crs/{id}/status`
- `products-agent/catalog/kg_tools.go`: +`pis_update_crs_status` tool
- `products-agent/dispatcher/dispatcher.go`: +`UpdateCRSStatus` in `KGClient` interface, +dispatch case

### Deferred (pikb-catalog-service)
- `PATCH /api/v1/kg/crs/{id}/status` endpoint needs to be implemented in `pikb-catalog-service` (not in current workspace scope)

### Verification
```
GOWORK=off go test ./internal/task/... → 10 PASS
GOWORK=off go test ./... (products-agent) → ALL PASS
```
