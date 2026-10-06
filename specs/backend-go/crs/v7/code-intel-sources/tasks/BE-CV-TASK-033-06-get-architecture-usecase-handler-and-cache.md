# BE-CV-TASK-033-06: `GetArchitecture` (use case, handler, cache khoá `overrides_version`)

**From Solution:** BE-CV-SOL-033-c4-component-view
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/usecase/get_architecture.go`, `backend-go/services/code-intel-service/internal/adapter/grpc/get_architecture_handler.go` (mới) và `_test.go`
**Depends on:** BE-CV-TASK-033-02, -05; BE-CV-TASK-033-08 (merge); BE-CV-SOL-022; BE-CV-SOL-013
**Status:** [ ] TODO

---

## Context

Solution mục 2.D. Chuỗi xử lý chuẩn §3 đầu mục.

## Việc cần làm

1. Validate `container` (`^[a-z0-9][a-z0-9-]{0,63}$`), `if_none_match ≤ 80`.
2. `containers[]`: liệt kê `backend-go/services/*` có `internal/`; `container` rỗng → `view` rỗng.
3. Dựng view → `MergeC4Overrides` → lọc `hidden` trừ `include_hidden`.
4. `graph_snapshots(view="architecture")`, `params_hash` gồm `overrides_version` + tập file bẩn; `etag`; `not_modified`; ≤ 2 MiB; timeout PQ-13 (`inProgress`).
5. Lỗi `CODEINTEL_*`; không đường dẫn tuyệt đối.

## Kiểm thử

`go test ./services/code-intel-service/... -run Architecture` (chưa chạy): hit/miss; đổi override → miss; hai tenant không chéo cache; ma trận hai dialect cho ghi snapshot (SOL-011).

## Tiêu chí hoàn thành

- [ ] Tiêu chí solution mục 6 về `has_overrides`, cache, quyền.
- [ ] `TestChannelInventory` thuộc 040 (không làm ở đây).

## Rủi ro và lưu ý

- Phụ thuộc task 08 (merge); nếu chậm, dùng merge rỗng + TODO có kiểm tra.
