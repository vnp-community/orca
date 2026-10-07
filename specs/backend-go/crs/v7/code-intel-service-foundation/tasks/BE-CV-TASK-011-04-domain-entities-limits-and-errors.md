# BE-CV-TASK-011-04: Domain: entity, giới hạn kích thước, `CanTransition`, `CanonicalParamsHash`, mã lỗi

**From Solution:** BE-CV-SOL-011-data-model-and-migrations
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/{tenant_settings,repo_binding,graph_snapshot,review_state,finding_dismissal,c4_override,reindex_job,payload_limits,code_intel_errors}.go` và `*_test.go` (mới)
**Depends on:** BE-CV-TASK-010-03
**Status:** [x] DONE

---

## Context

Domain không import adapter (arch/03). Lỗi dùng `common/apperrors` (`New(kind, code, message, cause)`), mã theo PQ-03 (`CODEINTEL_INVALID_PARAMS`, không `INVALID_ARGUMENT`). Giới hạn/giá trị theo SOL-011 mục 2.D. Mọi constructor kiểm tenant, `json.Valid`, `utf8.Valid`, kích thước, tập giá trị **trước khi chạm DB**.

## Việc cần làm

1. `payload_limits.go`: hằng giới hạn (SOL-011 2.D). Trần snapshot nhận từ cấu hình qua tham số constructor, không hằng.
2. `code_intel_errors.go`: hàm dựng cho bảy mã (SOL-011 2.E) với `Kind` đúng; `ErrNoTenant()`, `ErrInvalidParams(field, reason)`, `ErrPayloadTooLarge(field, limit)`, `ErrNotFound(kind)`, `ErrVersionConflict(current int64)`, `ErrReindexInProgress()`, `ErrAlreadyExists(what)`. Thông điệp không chứa giá trị người dùng/mã nguồn.
3. `tenant_settings.go`: `TenantSettings` đủ cột T1; `NewDefaultTenantSettings(tenantID, defaultEnabled, defaultQualityGate bool, now)`; validate `IndexPolicy`, `AIReviewLevel`, `HotspotWindowDays` 30..365, `AIReviewModel` ≤ 64.
4. `repo_binding.go`: `RepoBinding`, `IndexScope` (`exact|repo_root|unresolved`), `ScopeKeyForWorktree(id)`/`ScopeKeyForPath(repoID, pathHash)` chỉ ghép chuỗi (việc chuẩn hoá đường dẫn ở SOL-012); validate độ dài `scope_key ≤ 128`, `path_hash` 64 hex, `last_status` ≤ 64 KiB (hàm `TrimLastStatus` bỏ `languages`, `pendingChanges` trước khi cắt, không lỗi).
5. `graph_snapshot.go`: `GraphSnapshot`, `SnapshotKey{BindingID, View, HeadCommit, ParamsHash}`, `CanonicalParamsHash(params any) (string, error)` (JSON khoá sắp xếp, SHA-256 hex), `PrepareSnapshotPayload(raw []byte, max int64) (payload []byte, truncated bool, err error)` bỏ NUL và kiểm `utf8.Valid`, `ErrPayloadTooLarge` khi vượt `max`.
6. `review_state.go`: `ReviewState` với `ReadingProgress`, `Notes`, `TurnMarkers` (JSON thô đã kiểm), kiểm `notes` ≤ 256 KiB và ≤ 500 mục, `turn_markers` ≤ 5 phần tử và ≤ 256 KiB; `IsWorktreeRow()` khi hai commit rỗng; `Status ∈ open|reviewed`.
7. `finding_dismissal.go`: `Disposition` (`ignored|resolved`), `FindingKey` ≤ 128 ký tự, không NUL/ký tự điều khiển, `reason`/`note` ≤ 500.
8. `c4_override.go`: `document` ≤ 64 KiB, UTF-8 hợp lệ (parse/schema YAML do SOL-033).
9. `reindex_job.go`: `ReindexStatus` (5 giá trị DB), `ReindexMode`, `ReindexTrigger` (`manual|agent_done|head_change|schedule`), `CanTransition(from, to)`, `ActiveKeyFor(status, bindingID)` (= `bindingID` khi `queued|running`, rỗng khi kết thúc), `MapAgentStatus(agentStatus string) (ReindexStatus, errorCode string)` (`cancelling`→`running`, `canceled`→`cancelled`, `interrupted`→`failed` + `CODEINTEL_REINDEX_INTERRUPTED`), cắt `message` 500 ký tự.

## Kiểm thử

- `cd backend-go && go test ./services/code-intel-service/internal/domain/...`
- Bảng test: mỗi constructor × (hợp lệ, JSON hỏng, UTF-8 hỏng, vượt kích thước, ngoài tập); `CanTransition` đủ ma trận 5×5; `MapAgentStatus` đủ giá trị agent (`queued|running|cancelling|succeeded|failed|cancelled|canceled|interrupted`); `CanonicalParamsHash` (đổi thứ tự khoá, số `1` và `1.0`, khoá lồng); NUL bị bỏ và `truncated=true`; `TrimLastStatus` không lỗi khi vượt.

## Tiêu chí hoàn thành

- [x] Toàn bộ mã lỗi theo PQ-03, không `INVALID_ARGUMENT`.
- [x] `CanTransition` và `MapAgentStatus` khớp PQ-16.
- [x] Domain không import `pgx`/`database/sql`/gRPC.

## Rủi ro và lưu ý

- `canceled` (agent) và `cancelled` (DB) khác chính tả: chuẩn hoá một nơi duy nhất (`MapAgentStatus`).
- Số thực trong `CanonicalParamsHash`: chốt quy tắc và ghi trong chú thích (không dùng `%v`).
