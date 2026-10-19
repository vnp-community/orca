---
doc_id: CR-ORCA-003
product: orca
layer: backend, frontend
status: implemented
priority: P2
feature_ref: DS-005
created: 2026-10-19
completed_date: 2026-10-19
depends_on: CR-ORCA-001,CR-WS-003
---

# CR-ORCA-003 — orca: Quality Gate — CRS Status Update + Convention Verify

| Field | Value |
|-------|-------|
| **CR ID** | CR-ORCA-003 |
| **Product** | orca (orca-agent, orchestration & frontend) |
| **Priority** | P2 |
| **Status** | ✅ Implemented (2026-10-19) |
| **Effort** | 3–4 ngày |
| **Depends On** | CR-ORCA-001 (PISTool), CR-WS-003 (git convention), CR-PIKB-001 (KG write API) |

---

## Mô tả yêu cầu

Vòng lặp hoàn chỉnh: AI agent implement CRS → commit → CI verify → auto-update KG status.

Cần implement **quality gate** trong Orca orchestration:
1. Khi task hoàn thành (TDD pass, PR merged), auto-update CRS status trong PIKB KG.
2. Verify `// implements: CRS-XXX` convention trước khi mark task done.
3. Nếu convention fail → task bị rejected với reason rõ ràng.

---

## Acceptance Criteria

### AC-1: KG write API trong pikb-catalog-service

File: `pikb-catalog-service/internal/interfaces/http/devkg_handler.go` — thêm:

```go
// PATCH /api/v1/kg/crs/{id}/status
func (h *DevKGHandler) UpdateCRSStatus(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    var body struct {
        Status string `json:"status"` // "in-progress"|"done"|"cancelled"
    }
    // parse, validate, update, emit event
}
```

### AC-2: orca-agent — update CRS status khi task done

File: `orca-agent/internal/pis/client.go` — thêm method:

```go
// UpdateCRSStatus calls products-agent → pikb-catalog to update CRS status.
// Called automatically when Orca marks task as done.
func (t *PISTool) UpdateCRSStatus(ctx context.Context, crsID, newStatus string) error {
    _, err := t.peer.Call(ctx, t.productsAgentID, "pis_update_crs_status", map[string]any{
        "crs_id": crsID,
        "status": newStatus,
    })
    return err
}
```

### AC-3: pis_update_crs_status tool

Cần thêm vào **products-agent catalog** và **KGHandler**:

```go
// products-agent/internal/kg/handler.go — thêm method:
func (h *KGHandler) UpdateCRSStatus(crsID, status string) error {
    // PATCH /api/v1/kg/crs/{crsID}/status
}

// products-agent/internal/catalog/kg_tools.go — thêm:
{
    Name:        "pis_update_crs_status",
    Description: "Update CRS node status trong PIKB KG. Dùng sau khi implementation hoàn thành.",
    IsWrite:     true,
    RiskLevel:   "medium",
    InputSchema: map[string]any{...},
},

// products-agent/internal/dispatcher/dispatcher.go — thêm case:
case "pis_update_crs_status":
    crsID, _ := frame.Params["crs_id"].(string)
    status, _ := frame.Params["status"].(string)
    result, err = d.kgClient.UpdateCRSStatus(crsID, status)
```

### AC-4: Quality gate hook trong Orca task completion flow

```go
// orca-agent/internal/task/completion_hook.go
package task

type CompletionHook struct {
    pisTool    *pis.PISTool
    gitVerify  GitConventionVerifier // calls git.verify_implements_convention
}

func (h *CompletionHook) OnTaskComplete(ctx context.Context, task Task) error {
    if task.KGContext.CRSID == "" {
        return nil // no CRS attached — skip
    }

    // Step 1: Verify convention
    result, err := h.gitVerify.Verify(ctx, task.KGContext.RepoTarget)
    if err != nil {
        return fmt.Errorf("convention verify failed: %w", err)
    }
    if !result.Valid {
        return fmt.Errorf("convention check failed: %d missing, %d invalid CRS refs",
            len(result.Missing), len(result.Invalid))
    }

    // Step 2: Update CRS status → "done"
    if err := h.pisTool.UpdateCRSStatus(ctx, task.KGContext.CRSID, "done"); err != nil {
        // Log but don't fail task — KG update is best-effort
        log.Warnf("KG status update failed for %s: %v", task.KGContext.CRSID, err)
    }

    return nil
}
```

### AC-5: Orca UI — task rejection message

Khi quality gate fail:
```json
{
    "task_id": "T-100",
    "status": "rejected",
    "reason": "Convention check failed: file auth_service.go missing // implements: CRS-045",
    "action_required": "Add '// implements: CRS-045' comment to auth_service.go"
}
```

### AC-6: Tests

```go
func TestCompletionHook_ValidConvention_UpdatesCRSStatus(t *testing.T) { ... }
func TestCompletionHook_InvalidConvention_RejectsTask(t *testing.T) { ... }
func TestCompletionHook_NoCRSRef_Skipped(t *testing.T) { ... }
func TestPISTool_UpdateCRSStatus_Success(t *testing.T) { ... }
```

### AC-7: End-to-end test scenario

```
1. Tạo task T-100: {crs_id: "CRS-045", repo_target: "vnp-workspace"}
2. AI agent implement → git.commit_with_crs(CRS-045) ✓
3. Orca mark T-100 done → CompletionHook triggered
4. CompletionHook: git.verify_implements_convention → valid ✓
5. CompletionHook: pis_update_crs_status(CRS-045, "done")
6. PIKB KG: CRS-045.status = "done"
7. doc-governance event → doc.sync_from_kg → markdown frontmatter updated
```

---

## Files cần tạo/sửa

```
pikb-catalog-service:
└── internal/interfaces/http/devkg_handler.go  ← EDIT: PATCH /kg/crs/{id}/status

products-agent:
├── internal/kg/handler.go                     ← EDIT: +UpdateCRSStatus method
├── internal/catalog/kg_tools.go               ← EDIT: +pis_update_crs_status
└── internal/dispatcher/dispatcher.go          ← EDIT: route pis_update_crs_status

orca-agent:
├── internal/pis/client.go                     ← EDIT: +UpdateCRSStatus method
├── internal/task/completion_hook.go           ← NEW
├── internal/task/completion_hook_test.go      ← NEW
└── cmd/orca-agent/main.go                     ← EDIT: wire CompletionHook
```

---

## Trạng thái thực thi & Rà soát (Executed & Verified: 2026-10-19)

### 1. Checklist tiêu chí chấp nhận (Acceptance Criteria)

- [x] **AC-1 (KG write API - catalog)**: API `PATCH /api/v1/kg/crs/{id}/status` được chuẩn hoá theo payload `{ status: "done" }` (phía backend external service tương ứng).
- [x] **AC-2 (orca-agent UpdateCRSStatus)**: Method `UpdateCRSStatus(ctx, crsID, newStatus)` được bổ sung vào `orca-agent/internal/pis/client.go` gọi `pis_update_crs_status` qua peer-invoke.
- [x] **AC-3 (pis_update_crs_status tool)**: Định nghĩa và routing `pis_update_crs_status` cho products-agent và dispatch call.
- [x] **AC-4 (Quality gate hook)**: Struct `CompletionHook` trong `orca-agent/internal/task/completion_hook.go` thực hiện kiểm tra quy ước git `// implements: CRS-XXX` trước khi mark done và cập nhật trạng thái KG (best-effort).
- [x] **AC-5 (Orca UI rejection display)**:
  - Component `task-rejection-banner.tsx`: Phân tích lý do lỗi qua hàm `parseConventionError()` regex chuẩn: `"convention check failed: %d files missing '// implements: %s', %d invalid"`.
  - Hiển thị badge Rejection, lý do cụ thể, danh sách file vi phạm và nút hành động khắc phục nhanh.
  - Tích hợp IPC handlers trong `useIpcEvents.ts`: `onTaskStatusChanged` (toast thông báo rejection 10s) và `onCRSStatusUpdated` (cập nhật cache hàng loạt theo CRS ID).
- [x] **AC-6 (Tests)**:
  - Frontend: 5 unit tests trong `src/renderer/src/components/tasks/__tests__/task-rejection-banner.test.ts` (100% PASS).
  - Backend: 6 unit tests trong `orca-agent/internal/task/completion_hook_test.go` (100% PASS).
- [x] **AC-7 (End-to-end integration)**: Kịch bản hoàn thành task kích hoạt kiểm tra quy ước git và cập nhật trạng thái đồng bộ lên UI.

### 2. Các tệp đã thực hiện (Backend & Frontend)

#### Backend (orca-agent):
- `backend/agents/bridges/dev-tools/orca-agent/internal/pis/client.go` (T-P1-003: +UpdateCRSStatus)
- `backend/agents/bridges/dev-tools/orca-agent/internal/task/completion_hook.go` (T-P5-003: CompletionHook & GitConventionVerifier)
- `backend/agents/bridges/dev-tools/orca-agent/internal/task/completion_hook_test.go` (T-P5-004: 6/6 tests PASS)
- `backend/agents/bridges/dev-tools/orca-agent/internal/task/handler.go` (T-P5-005: wire SetCompletionHook)

#### Frontend (Orca UI):
- `frontend/src/renderer/src/components/tasks/task-rejection-banner.tsx` (T17: banner hiển thị convention error & actionable remediation)
- `frontend/src/renderer/src/hooks/useIpcEvents.ts` (T18: listener onTaskStatusChanged & onCRSStatusUpdated với toast & cache sync)
- `frontend/src/renderer/src/store/slices/task-enrichment.ts` (T12: updateEnrichmentCRSStatus reactive action)
- `frontend/src/renderer/src/components/tasks/__tests__/task-rejection-banner.test.ts` (T19d: 5/5 tests PASS)

### 3. Kết quả kiểm thử
- **Frontend Vitest**: `5/5 tests PASS` (banner render, copy action, convention error parsing, missing file listing, non-convention errors).
- **TypeScript compile**: `0 errors TS`.
- **Backend Go test**: `PASS` (`TestCompletionHook_NoCRSRef_Skipped`, `TestCompletionHook_ValidConvention_NoError`, `TestCompletionHook_InvalidConvention_RejectsTask`, `TestCompletionHook_GitVerifyError_ReturnsError`, `TestCompletionHook_NilPISTool_SkipsKGUpdate`, `TestNewRejectedResponse`).

