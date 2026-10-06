# BE-CV-TASK-021-08: `codeintel_reindex.proto`, `RequestReindex`, `GetReindexJob`, `ReindexJobStore`

**From Solution:** BE-CV-SOL-021-agent-collector
**Priority:** P0
**Service:** `code-intel-service` · `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_reindex.proto` (mới), `.../usecase/{request_reindex.go,get_reindex_job.go,reindex_job_refresh.go,reindex_job_store.go}` (mới)
**Depends on:** TASK-021-04, SOL-011 (`reindex_jobs`, outbox), SOL-013 (`ReindexAdmission`)
**Status:** [ ] TODO

---

## Context

PQ-16, §4 T7, §5. Job có `active_key` UNIQUE; sự kiện `reindex.started` cùng giao dịch.

## Việc cần làm

1. Proto 2.F; `buf generate`.
2. Cổng `ReindexJobStore` (`CreateActive`, `Get`, `UpdateProgress`, `Finish`) + `ReindexAdmission`.
3. `RequestReindex` theo 5 bước 2.F (nhận nuôi job agent đang chạy, `outcome` → succeeded, offline → failed ngay, không retry).
4. `GetReindexJob`: tenant/binding đúng, làm tươi qua `reindexStatus` khi quá `CODEINTEL_REINDEX_POLL_AFTER`; cho phép khi cờ tắt.
5. Ánh xạ trạng thái agent → DB (`cancelling`→running, `interrupted`→failed).

## Kiểm thử

- `go test ./internal/usecase/ -run Reindex -race` với store trong bộ nhớ.
- Suite hợp đồng cho `ReindexJobStore` chạy ở cả Postgres và MySQL (testcontainers, `-tags=integration`) và test tenant: job tenant khác → `NOT_FOUND`.
- Hai `RequestReindex` đồng thời → một job, một `CODEINTEL_REINDEX_IN_PROGRESS`.

## Tiêu chí hoàn thành

- [ ] Mọi ca trên xanh hai dialect. - [ ] Outbox `reindex.started` đúng một dòng.

## Rủi ro và lưu ý

- Cooldown/quota thuộc SOL-013; nếu chưa có, `Admit` no-op (ghi PR).
