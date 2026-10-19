---
task_id: T-P5-001
phase: 5
cr_id: CR-ORCA-003
solution_ref: solutions/CR-ORCA-003-solution.md
service: pikb-catalog-service
target_file: pikb-catalog-service/internal/interfaces/http/devkg_handler.go
priority: P2
status: done
depends_on: T-P4-003
estimated_effort: 2 giờ
blocks: T-P5-002
---

# T-P5-001 — Thêm PATCH /api/v1/kg/crs/{id}/status endpoint vào pikb-catalog-service

## Context

Để vòng lặp "implement → verify → update KG" hoạt động, `pikb-catalog-service` cần expose write endpoint cho CRS status update. Đây là endpoint mà `products-agent` sẽ gọi khi nhận `pis_update_crs_status` tool call.

## Target file

```
pikb-catalog-service/internal/interfaces/http/devkg_handler.go
```

## Instructions

1. **Đọc file hiện tại** `devkg_handler.go` để hiểu:
   - Struct `DevKGHandler` có gì
   - Các method hiện có
   - Router setup ở đâu (để biết chỗ thêm route mới)

2. **Thêm `UpdateCRSStatus()` handler method**:
   ```go
   // UpdateCRSStatus xử lý PATCH /api/v1/kg/crs/{id}/status
   // CR-ORCA-003: write endpoint cho orca-agent cập nhật CRS status sau implementation.
   func (h *DevKGHandler) UpdateCRSStatus(w http.ResponseWriter, r *http.Request) {
       id := chi.URLParam(r, "id") // hoặc mux.Vars(r)["id"] tùy router

       var body struct {
           Status    string `json:"status"`               // "pending"|"in-progress"|"done"|"cancelled"
           UpdatedBy string `json:"updated_by,omitempty"` // agent ID hoặc user ID
           Reason    string `json:"reason,omitempty"`
       }
       if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
           http.Error(w, "invalid request body", http.StatusBadRequest)
           return
       }

       // Validate status
       validStatuses := map[string]bool{
           "pending":     true,
           "in-progress": true,
           "done":        true,
           "cancelled":   true,
       }
       if !validStatuses[body.Status] {
           http.Error(w, "invalid status: "+body.Status, http.StatusBadRequest)
           return
       }
       if id == "" {
           http.Error(w, "crs id required", http.StatusBadRequest)
           return
       }

       if err := h.kgService.UpdateCRSStatus(r.Context(), id, body.Status, body.UpdatedBy); err != nil {
           http.Error(w, err.Error(), http.StatusInternalServerError)
           return
       }

       w.WriteHeader(http.StatusNoContent)
   }
   ```

3. **Đăng ký route** trong router setup (tìm chỗ đăng ký các routes KG khác):
   ```go
   r.Patch("/api/v1/kg/crs/{id}/status", h.UpdateCRSStatus)
   ```

4. **Thêm `UpdateCRSStatus()` vào service interface** (nếu `kgService` dùng interface):
   ```go
   // Trong interface definition của kgService:
   UpdateCRSStatus(ctx context.Context, crsID, status, updatedBy string) error
   ```

5. **Implement `UpdateCRSStatus()` trong service layer** (nếu chưa có):
   - Update status field của CRS node trong DB/graph store
   - Validate status transition (optional: chỉ cho phép forward transition)

## Acceptance criteria

- [ ] `PATCH /api/v1/kg/crs/{id}/status` endpoint tồn tại
- [ ] Status validation: chỉ accept `pending|in-progress|done|cancelled`
- [ ] Invalid status → `400 Bad Request`
- [ ] Success → `204 No Content`
- [ ] Route đã được đăng ký trong router

## Test command

```bash
# Compile check
go build ./...

# Manual test (nếu có server):
curl -X PATCH http://localhost:PORT/api/v1/kg/crs/CRS-045/status \
  -H "Content-Type: application/json" \
  -d '{"status":"done","updated_by":"orca-agent"}'
# Expected: 204 No Content
```
