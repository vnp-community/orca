# BE-CV-TASK-040-24: Kênh `codeIntel.quality.findings` và `codeIntel.quality.waive`

**From Solution:** BE-CV-SOL-040-codeintel-quality-channels
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_codeintel_quality_findings.go` (mới), `channels_codeintel_quality_findings_test.go` (mới)
**Depends on:** TASK-040-23; stub `ListQualityFindings` (BE-CV-SOL-082), `WaiveFinding` (BE-CV-SOL-085-waivers-and-trend)
**Status:** [ ] TODO

---

## Context

UI-API 3.2: `findings` (sel, `runId?`, `severities?`, `categories?`, `file?`, `inScope?`, `limit ≤ 500`, `pageToken`) => `{findings, totalCount, truncated, outsideScopeCount, nextPageToken}` (20 s, `quality_read`); `waive` (sel, `subjectKind` finding|structure_finding|check, `subjectKey`, `action?` waive|revoke, `reason` 1..1000 (`check` >= 20), `expiresAt` RFC 3339 <= 30 ngày, `scope?`) => `{waiver}` (8 s, `quality_waive`). `QualityFinding` không có dòng mã nguồn (4.7). PQ-05: waiver khác dismiss; `WAIVER_EXPIRY_INVALID | {maxDays}` do service.

## Việc cần làm

1. `qualityFindingsArgs{sel, RunID, Severities, Categories, File, InScope *bool, Limit, PageToken}`: `severities` ⊂ error|warning|info; `categories` ⊂ lint|typecheck|test|coverage|complexity|security|dependency|convention|architecture|ai; `file` qua `checkRelPath` (`PATH_NOT_ALLOWED`); `limit` 1..500; mỗi mảng ≤ 32.
2. `qualityWaiveArgs{sel, SubjectKind, SubjectKey, Action, Reason, ExpiresAt, Scope}`: `subjectKey` 1..1024; nếu `action` rỗng/waive: `reason` 1..1000 (và >= 20 khi `subjectKind=check`), `expiresAt` bắt buộc RFC 3339 (gateway **không** kiểm 30 ngày; service quyết theo vai trò); nếu `revoke`: `reason`/`expiresAt` tuỳ chọn (Q1 của solution); `scope` repo|binding.
3. Dịch kết quả; `waiver` có `revokedAt?`; `findings[].waiver?` vắng khi không.
4. Không log `reason` (văn bản người dùng).

## Kiểm thử

- Validate: `file` `../x`; `reason` 1001; `check` + `reason` 19 ký tự; `expiresAt` `tomorrow`; `subjectKind` `rule`; `limit` 501.
- Fake: `WAIVER_EXPIRY_INVALID` đi qua; `findings` rỗng => `[]`, `truncated` giữ; `outsideScopeCount`.
- `DeviceID`, deadline 20 s/8 s.
- Lệnh: `go test ./internal/adapter/wscompat/ -run 'CodeIntelQualityFindings'`.

## Tiêu chí hoàn thành

- [x] Hai kênh đúng shape; ràng buộc `reason` theo `subjectKind`.

## Rủi ro và lưu ý

- Gateway không áp quy tắc "member không miễn `error`" (service).
- `QualityFinding.message` có thể chứa văn bản công cụ không tin cậy (U9): không xử lý ở gateway.
