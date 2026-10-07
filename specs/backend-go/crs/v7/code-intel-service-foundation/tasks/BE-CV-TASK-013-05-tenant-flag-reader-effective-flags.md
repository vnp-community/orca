# BE-CV-TASK-013-05: `FlagReader`: cờ hiệu lực (env ∧ tenant), cache 5 s, fail closed, ngoại lệ khi tắt

**From Solution:** BE-CV-SOL-013-authorization-flags-and-audit
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/flag_reader.go`, `flag_reader_test.go` (mới)
**Depends on:** BE-CV-TASK-011-06, 011-07, 011-09, 010-03
**Status:** [x] DONE

---

## Context

PQ-01, PQ-24 và §6.1. Công thức: `CodeIntel = env ∧ tenant`; `QualityGate = CodeIntel ∧ env.QualityGate ∧ tenant.quality_gate_enabled`; `SecurityScan = QualityGate ∧ tenant.quality_security_scan_enabled`; `AIReview = CodeIntel ∧ env.AIReview ∧ ai_review_level ≠ off`. RPC `SetSettings` thuộc SOL-073; task này chỉ đọc.

## Việc cần làm

1. `FlagReader{settings TenantSettingsRepository, cfg, clock, cache}`; `Effective(ctx)` lấy tenant từ ctx; cache TTL `CODEINTEL_FLAG_CACHE_TTL` (5 s) khoá tenant.
2. Dòng `tenant_settings` thiếu → `GetOrCreate` với `TenantDefaultEnabled` và `TenantDefaultQualityGateEnabled`; các cờ khác mặc định cột.
3. Lỗi đọc DB → `EffectiveFlags{}` all-false (đóng) và trả `(flags, err)`; use case trên hiển thị `CODEINTEL_DISABLED` (không lộ lỗi DB).
4. `Invalidate(tenant)` cho SOL-073 gọi sau `SetSettings`.
5. Hằng danh sách RPC cho phép khi tắt (`AllowWhenDisabled`): `GetSettings`, `SetSettings`, `GetReindexJob`; đặt trong bảng chính sách (013-07), test đảm bảo chỉ ba cái này.
6. `EffectiveFlags` có phương thức `ForScope(scope FlagScope) (ok bool, errCode string)` trả `CODEINTEL_DISABLED`/`CODEINTEL_QUALITY_GATE_DISABLED`/`CODEINTEL_AI_REVIEW_DISABLED`.

## Kiểm thử

- Unit (đồng hồ giả, repo giả): ma trận env × tenant × cờ; thiếu dòng → tạo lười với default cấu hình; lỗi đọc → đóng; cache 5 s (đổi DB không thấy trước TTL, thấy sau); `Invalidate`.
- Integration hai dialect: `GetOrCreate` đồng thời 10 goroutine không tạo hai dòng.
- `go test ./services/code-intel-service/internal/usecase/... -run FlagReader`

## Tiêu chí hoàn thành

- [x] Công thức PQ-24 đúng cho mọi tổ hợp.
- [x] Lỗi đọc = tắt; hiệu lực ≤ 5 s.

## Rủi ro và lưu ý

- Mặc định `false` cho mọi tenant tới GA: dev phải bật qua `SetSettings` hoặc `CODEINTEL_TENANT_DEFAULT_ENABLED=true` cục bộ.
