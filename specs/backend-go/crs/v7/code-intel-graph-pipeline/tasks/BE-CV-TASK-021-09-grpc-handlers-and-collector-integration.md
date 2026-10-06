# BE-CV-TASK-021-09: Handler gRPC cho 8 RPC, metrics, test tích hợp với infra-fleet thật

**From Solution:** BE-CV-SOL-021-agent-collector
**Priority:** P1
**Service:** `code-intel-service`
**File:** `.../internal/adapter/grpc/{server_graph_views.go,server_reindex.go,reindex_proto_mapping.go}` (mới), `.../adapter/infrafleetclient/collector_integration_test.go` (`//go:build integration`)
**Depends on:** TASK-021-06, 021-07, 021-08
**Status:** [ ] TODO

---

## Context

Handler đi qua pipeline SOL-013; trả `ViewReader` (SOL-022 sẽ bọc).

## Việc cần làm

1. Handler 6 RPC đồ thị + 2 reindex; mỗi lỗi qua `apperrors.ToGRPCStatus`.
2. Metrics: `codeintel_collector_calls_total{method,outcome}`, `..._duration_seconds{method}`, `..._reconnect_wait_seconds`, `..._truncated_total{view}` (tên cuối CR-071).
3. Test tích hợp: infra-fleet thật in-process (SOL-023) + agent giả `CODEINTEL_INDEX_MISSING`; trước SOL-023 phải **đỏ**.
4. Kiểm cô lập tenant ở handler (selector tenant khác → `NOT_AUTHORIZED`).

## Kiểm thử

- `go test ./internal/adapter/grpc/ -race`; `go test -tags=integration ./internal/adapter/infrafleetclient/ -run Collector -race`.
- Ghi đỏ/xanh vào PR.

## Tiêu chí hoàn thành

- [ ] 8 RPC hoạt động với `ViewReader`. - [ ] Test tích hợp xanh sau SOL-023. - [ ] Không `max-lines` disable.

## Rủi ro và lưu ý

- Quyền/cờ thuộc pipeline SOL-013; nếu chưa có, handler chỉ chạy trong test.
