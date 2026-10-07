# BE-CV-TASK-073-02: `EffectiveFlags`: hiệu lực cờ, dòng lười, cache 5 s, fail closed, biến môi trường

**From Solution:** BE-CV-SOL-073
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/effective_flags.go` (mới), `.../internal/usecase/effective_flags_test.go` (mới), `.../internal/config/feature_switches.go` (mới), `.../cmd/server/main.go`
**Depends on:** BE-CV-TASK-073-01, BE-CV-SOL-010 (`common/config`), BE-CV-SOL-011-repositories-and-maintenance
**Status:** `[x] DONE`

---

## Context

- PQ-24: cache **5 s**; lỗi đọc = tắt; `code_intel_enabled` **không default** ở cột nên use case luôn chèn giá trị từ `CODEINTEL_TENANT_DEFAULT_ENABLED` (mặc định `false`); công thức hiệu lực §6.1/PQ-24.
- Biến (PQ-23): `CODEINTEL_ENABLED`, `CODEINTEL_QUALITY_GATE_ENABLED`, `CODEINTEL_AI_REVIEW_ENABLED`, `CODEINTEL_TENANT_DEFAULT_ENABLED`, `CODEINTEL_TENANT_DEFAULT_QUALITY_GATE_ENABLED`, tất cả mặc định `false`.
- Mẫu: dòng lười `DefaultTenantSettings` của `mcp-service`; `common/config`.

## Việc cần làm

1. `feature_switches.go`: đọc năm biến bằng `common/config`, log một dòng ở khởi động (giá trị, không secret).
2. `EffectiveFlags.For(ctx, tenantID) (Flags, error)`: cache theo `tenant_id` TTL 5 s (đồng hồ chèn được để test); không lưu lỗi; lỗi đọc ⇒ `Flags{}` tất cả `false` và trả `err` cho nơi gọi biết (interceptor coi là tắt).
3. Thiếu dòng ⇒ chèn lười (`INSERT … ON CONFLICT DO NOTHING` / MySQL `ON DUPLICATE KEY UPDATE`, đã có ở repo CR-011) với `code_intel_enabled` và `quality_gate_enabled` từ biến `…DEFAULT…`; không đổi dòng cũ.
4. Hiệu lực: `codeIntel = env ∧ tenant`; `quality = codeIntel ∧ envQuality ∧ tenant.quality`; `ai = codeIntel ∧ level≠off ∧ envAI`; `security = quality ∧ tenant.security`.
5. `SetSettings` (task 01) xoá mục cache của tenant đó (replica khác chờ tối đa 5 s).

## Kiểm thử

- Bảng 2^5 tổ hợp env × tenant; thiếu dòng; lỗi đọc; cache TTL bằng giả đồng hồ (hết hạn đúng 5 s); lỗi không bị cache; hai tenant độc lập.
- Hai dialect: phần chèn lười thuộc test repo (BE-CV-SOL-011) — ghi chéo; thêm test tích hợp ở đây cho chèn lười đồng thời hai goroutine.
- `go test ./internal/usecase/... -run Flags`.

## Tiêu chí hoàn thành

- [x] Mặc định tất cả tắt; tắt có hiệu lực ≤ 5 s (giả đồng hồ).
- [x] Lỗi đọc không bật nhầm.

## Rủi ro và lưu ý

- Nhiều replica: hiệu lực lệch tối đa 5 s giữa replica (chấp nhận, ghi runbook).
