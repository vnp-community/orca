# BE-CV-TASK-085-08: Miễn trừ: domain, repository hai dialect, use case `WaiveFinding`

**From Solution:** BE-CV-SOL-085-waivers-and-trend
**Priority:** P0
**Service:** `code-intel-service`
**File:** `internal/domain/quality_waiver.go`, `internal/usecase/waive_quality_finding.go`, `internal/adapter/{postgres,mysql}/quality_waiver_repository.go`, `internal/adapter/grpc/quality_gate_waiver_trend_server.go` (mới)
**Depends on:** BE-CV-TASK-085-01, 085-02, 085-07
**Status:** [x] DONE

## Context
Quy tắc: SOL-085-waivers §2.2. `active_key = sha256(tenant|repo|kind|key|scope)` hex; hiệu lực tính trong SQL bằng đồng hồ DB.

## Việc cần làm
1. Domain: kiểm hạn (`CODEINTEL_WAIVER_MAX_DAYS` mặc định 30; `member` 7), lý do 1–1000 (`check` ≥ 20), vai trò × kind × severity, tính `active_key`.
2. Repo: `Upsert` (PG `ON CONFLICT (active_key)`; MySQL `ON DUPLICATE KEY UPDATE`; thử lại một lần khi 23505/1062), `Revoke` (`active_key=NULL`, idempotent), `ListActive(tenant, repo, binding, limit 50)`.
3. Use case: kiểm subject tồn tại (`quality_findings` theo `fingerprint`; `check.id` trong profile hiệu lực; `structure_finding` qua port, thiếu ⇒ chỉ owner/admin), `selector → repo_id`, audit `codeintel.quality.waive[.revoke]` (reason chỉ vào `slog`).
4. Handler `WaiveFinding`; mã `CODEINTEL_WAIVER_EXPIRY_INVALID` kèm `maxDays`.

## Kiểm thử
- Bảng ca vai trò/hạn/kind; integration hai dialect: hai goroutine WAIVE cùng khoá ⇒ một hàng hiệu lực; REVOKE rồi WAIVE ⇒ hàng mới; hết hạn theo đồng hồ DB (`expires_at=now()+1s`); cách ly tenant; test AST.

## Tiêu chí hoàn thành
- [x] `WaiveFinding` idempotent; [ ] `member` không miễn `error`/`check`, ≤ 7 ngày; [ ] không job nền để hết hạn.

## Rủi ro
- `finding_key` đổi khi refactor (CR-037) ⇒ waiver `structure_finding` có thể mất.
