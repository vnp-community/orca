# BE-CV-TASK-040-19: Kênh `codeIntel.settings.get` và `codeIntel.settings.set`

**From Solution:** BE-CV-SOL-040-codeintel-write-and-stream-channels
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_state_settings.go` (thêm), `channels_codeintel_state_settings_test.go` (mới)
**Depends on:** TASK-040-07; stub `GetSettings`/`SetSettings` (BE-CV-SOL-073-settings-flag-and-rollout)
**Status:** [ ] TODO

---

## Context

UI-API 3.1, 4.1, §6: `settings.get` `{}` => `Settings{effective{codeIntelEnabled, qualityGateEnabled, qualitySecurityScanEnabled, aiReviewEnabled}, tenant{...}, updatedBy?, updatedAt?}`, luôn trả được kể cả cờ tắt, **cho phép phiên thiết bị** (U8) và là kênh duy nhất FE gọi khi tắt tính năng; `settings.set` quyền `admin` (service kiểm `Identity.Role`) với 9 trường tuỳ chọn, ≥ 1 trường. PQ-24: lỗi đọc cờ = tắt. CR-040: không có `CODE_INTEL_ENABLED` ở gateway.

## Việc cần làm

1. `settingsGetArgs struct{}` (chấp nhận `{}`; mọi khoá khác bị từ chối); `AllowDevice: true`; `Core.GetSettings`; trả `Settings` (encoder).
2. `settingsSetArgs{CodeIntelEnabled, QualityGateEnabled, QualitySecurityScanEnabled *bool; IndexPolicy *string; AIReviewLevel *string; AIReviewModel *string; AgentTurnStorePromptExcerpt, AgentClaimTextEnabled *bool; HotspotWindowDays *int}`: ≥ 1 trường khác nil (`invalidParam("args","empty_update")`); `indexPolicy` auto_in_place|per_worktree|off; `aiReviewLevel` off|metadata|diff; `aiReviewModel ≤ 128` (allowlist tiền tố do service); `hotspotWindowDays` 30..365 (từ chối ngoài khoảng); `Core.SetSettings` chỉ gửi trường có mặt (dùng `optional`/`FieldMask` theo proto của CR-073).
3. `Identity.Role` đã nằm trong metadata (`Dispatch`); gateway **không** tự kiểm admin (service quyết, `NOT_AUTHORIZED`).
4. Không bao giờ che `CODEINTEL_DISABLED` thành lỗi khác cho hai kênh này; chúng không bị service chặn khi tắt.

## Kiểm thử

- Validate: `{}` cho `set`; `indexPolicy` `daily`; `aiReviewLevel` `full`; `hotspotWindowDays` 29/366; `aiReviewModel` 129 ký tự; khoá `enabled` cũ (CR) => `INVALID_PARAMS`.
- Fake: trường vắng không được gửi (so sánh request); `Identity.Role=admin` ra metadata; phiên thiết bị gọi `settings.get` thành công, `settings.set` => `NOT_AUTHORIZED`.
- Gateway chưa cấu hình: `settings.get` => `CODEINTEL_UNAVAILABLE` (FE coi là tính năng tắt).
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelState(Settings)'`.

## Tiêu chí hoàn thành

- [x] `settings.get` cho phép phiên thiết bị; `settings.set` thì không.
- [x] Cập nhật từng phần đúng ngữ nghĩa trường vắng.

## Rủi ro và lưu ý

- Cách biểu diễn "trường vắng" trong proto (`optional`/`FieldMask`) do CR-073 chốt; nếu dùng bool thường, không phân biệt được `false` với vắng: yêu cầu CR-073 dùng `optional`.
- Đổi cờ tắt hiệu lực ≤ 5 s (PQ-24); FE tải lại `settings.get` mỗi 60 s.
