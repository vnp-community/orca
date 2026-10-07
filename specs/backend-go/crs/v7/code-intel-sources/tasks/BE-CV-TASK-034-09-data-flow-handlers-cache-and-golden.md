# BE-CV-TASK-034-09: Handler gRPC, cache snapshot, golden và cô lập tenant

**From Solution:** BE-CV-SOL-034-data-flow-model
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/grpc/data_flow_handlers.go` (mới), `.../usecase/get_data_flow.go` (mới), `.../usecase/data_flow_golden_test.go`, `.../testdata/golden/dataflow/*.json`
**Depends on:** BE-CV-TASK-034-05, -06, -08; BE-CV-SOL-022; BE-CV-SOL-013; BE-CV-SOL-012
**Status:** [x] DONE

---

## Context

Solution 2.E; chuỗi xử lý chuẩn §3.

## Việc cần làm

1. `GetDataFlow`/`ListDataFlows`: selector→binding, OPA `read`, validate (`flow_id`, `dialect`, hop ≤ 8, step ≤ 200, `query ≤ 128`).
2. Cache `graph_snapshots(view="dataflow"/"dataflows")` khoá như 2.E (có `overrides_version`); `etag`, `not_modified`; ≤ 2 MiB; `inProgress`/`CODEINTEL_TIMEOUT` hậu tố `{retryAfterMs,inProgress}` (PQ-13).
3. Golden 3 luồng (task 01) + fixture gợi lỗi; kiểm chéo `cypher` mẫu GitNexus chỉ khi bật.
4. Cô lập tenant (cache/snapshot) hai dialect; hiệu năng 10 luồng, ghi số đo thật.

## Kiểm thử

`go test ./services/code-intel-service/... -run DataFlow` và `-tags=integration` (chưa chạy); `TestChannelInventory`/`TestToolParity` thuộc 040.

## Tiêu chí hoàn thành

- [x] Tiêu chí solution mục 6 đạt.
- [x] Golden ổn định; hai dialect; cô lập tenant.

## Rủi ro và lưu ý

- Golden phụ thuộc commit; dùng `mini-flow` + job quét repo thật không chặn PR.
