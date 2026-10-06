# BE-CV-TASK-034-03: Miền `dataflow`, mã gap và giới hạn

**From Solution:** BE-CV-SOL-034-data-flow-model
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/dataflow/flow.go`, `gap_codes.go`, `limits.go` (mới) và `_test.go`
**Depends on:** BE-CV-TASK-034-02
**Status:** [ ] TODO

---

## Context

Solution 2.B/2.C.

## Việc cần làm

1. `DataFlow`, `Step`, `Gap`, `StoreAccess`; hàm `AddStep` đánh số `n`, kiểm `max_steps`.
2. Hằng `GapCode` (8 mã) và `Origin` (`static-fieldtype|static-name|process|declared`).
3. `Limits{MaxServiceHops (4, ≤8), MaxSteps (60, ≤200)}` với kẹp **từ chối** ngoài khoảng (không kẹp ngầm, UI §2.4).
4. `Visited` theo `(service,rpc)` cho `CYCLE`.
5. `flow_id`: `ws:<channel>` | `grpc:<Service>.<Rpc>`; parser/formatter + kiểm hợp lệ.

## Kiểm thử

`go test ./services/code-intel-service/internal/domain/dataflow/...` (chưa chạy): giới hạn, chu trình, id xấu.

## Tiêu chí hoàn thành

- [ ] Ngoài khoảng → lỗi `INVALID_PARAMS`.
- [ ] `flow_id` hợp lệ/không đều có test.

## Rủi ro và lưu ý

- Tên kênh có `:` (`agent:rateLimited`): `ws:` + chuỗi nguyên văn, parse theo tiền tố đầu.
