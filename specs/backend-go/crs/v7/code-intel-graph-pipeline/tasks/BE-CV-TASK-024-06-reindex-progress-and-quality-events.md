# BE-CV-TASK-024-06: Xử lý `reindex_progress`, `quality_progress`, `quality_finished`

**From Solution:** BE-CV-SOL-024-event-distribution
**Priority:** P1
**Service:** `code-intel-service`
**File:** `.../internal/usecase/handle_agent_code_intel_event.go` (sửa), `.../usecase/binding_stream_targets.go` (`QualityRunEventSink`) và test
**Depends on:** TASK-024-05, BE-CV-TASK-021-08
**Status:** [ ] TODO

---

## Context

SOL-024 bảng 2.C: tiến trình push trực tiếp; kết thúc job qua outbox `reindex.finished`; `quality_finished` gọi `QualityRunEventSink`.

## Việc cần làm

1. `reindex_progress`: đọc `state` từ `payload_json`; cập nhật `reindex_jobs` có giới hạn; kết thúc → giao dịch `processed_events` + `Finish` + `InvalidateBinding` + outbox `reindex.finished`; `interrupted` → `failed`/`CODEINTEL_REINDEX_INTERRUPTED`.
2. `quality_progress` push trực tiếp ≤ 1/s/run.
3. `quality_finished`: push + `QualityRunEventSink.OnFinished(ctx, tenant, binding, runID)` (no-op mặc định; SOL-082 cài).
4. Lọc `payload_json` quality theo allowlist ≤ 8 KiB.

## Kiểm thử

- `go test ./internal/usecase/ -run 'ReindexProgress|Quality' -race`; integration PG + MySQL cho `reindex_jobs`; ca: `percent:null` giữ `null`; kết thúc thành công → một `reindex.finished`; không outbox cho tiến trình.

## Tiêu chí hoàn thành

- [ ] Các ca xanh hai dialect.

## Rủi ro và lưu ý

- Phụ thuộc `state` trong `payload_json` (SOL-023 Q1).
