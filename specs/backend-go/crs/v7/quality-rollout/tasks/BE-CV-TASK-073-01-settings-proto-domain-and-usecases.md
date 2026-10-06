# BE-CV-TASK-073-01: Proto `codeintel_settings.proto`, domain `TenantSettings` và use case `GetSettings`/`SetSettings`

**From Solution:** BE-CV-SOL-073
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_settings.proto` (mới), `backend-go/proto/orca/codeintel/v1/codeintel.proto` (thêm 2 RPC), `.../internal/domain/tenant_settings.go` (mới), `.../internal/usecase/settings.go` (mới), `.../internal/usecase/settings_test.go` (mới), `.../internal/adapter/grpc/settings_server.go` (mới)
**Depends on:** BE-CV-SOL-010 (G0: `codeintel.proto`), BE-CV-SOL-011-data-model-and-migrations (bảng T1, repo), BE-CV-SOL-013-authorization-flags-and-audit (cổng audit)
**Status:** `[ ] TODO`

---

## Context

- Hợp đồng §2.1 mục 8: `codeintel_settings.proto` = `TenantSettings`, `GetSettings*`, `SetSettings*`; §3.1: `GetSettings` → `{effective, tenant}`, `SetSettings` (`role=admin`); ui-api §4.1 `Settings`. Cột T1 do CR-011 tạo; **không** có migration ở task này.
- Mẫu domain: `mcp-service/internal/domain/tenant_settings.go` (`Validate` phản chiếu CHECK).
- Quy tắc proto: enum có `_UNSPECIFIED = 0`, `buf lint`/`buf breaking`.

## Việc cần làm

1. Proto: `TenantSettings` (các cột T1 trừ `tenant_id`), `EffectiveSettings{code_intel_enabled, quality_gate_enabled, quality_security_scan_enabled, ai_review_enabled}`, `GetSettingsResponse{effective, tenant, updated_by, updated_at}`, `SetSettingsRequest` với các trường tuỳ chọn (`optional`) và yêu cầu ≥ 1 trường; enum `IndexPolicy`, `AiReviewLevel`.
2. Domain `TenantSettings` + `Validate`: `hotspot_window_days` 30..365; `index_policy ∈ auto_in_place|per_worktree|off`; `ai_review_level ∈ off|metadata|diff`; `ai_review_model` theo allowlist tiền tố `claude|gpt|o*|gemini` (sai ⇒ `CODEINTEL_INVALID_PARAMS`).
3. Use case `GetSettings` (đọc + tính `effective` bằng `EffectiveFlags`, task 02) và `SetSettings`: chỉ `role=admin` (lỗi `CODEINTEL_NOT_AUTHORIZED`), cập nhật có `updated_by`, ghi audit `codeintel.settings.set` (không nuốt lỗi validate; lỗi audit best-effort); trả cấu hình mới.
4. gRPC server đăng ký hai RPC; không sinh mã ngoài `buf generate`.

## Kiểm thử

- Unit: bảng `Validate`; `SetSettings` từ chối non-admin; ≥ 1 trường; `GetSettings` không có dòng ⇒ mặc định (task 02 chèn lười); audit được gọi.
- `buf lint`, `buf breaking`; `go test ./internal/usecase/... ./internal/adapter/grpc/...`.
- Hai dialect: thuộc test repo của BE-CV-SOL-011 (ghi chéo).

## Tiêu chí hoàn thành

- [ ] Hai RPC hoạt động với dữ liệu hợp đồng.
- [ ] Non-admin bị từ chối; giá trị ngoài phạm vi bị từ chối.

## Rủi ro và lưu ý

- Sửa `codeintel.proto` chung: PR hợp đồng trước nếu đổi bảng §3.1 (§8.3 mục 2).
- Xác nhận hai bước khi `ai_review_level=diff` là việc UI, server chỉ validate.
