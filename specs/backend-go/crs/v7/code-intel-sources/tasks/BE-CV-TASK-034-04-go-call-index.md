# BE-CV-TASK-034-04: `CallIndex` (trường→kiểu, lời gọi `recv.field.Method`, interface→phương thức)

**From Solution:** BE-CV-SOL-034-data-flow-model
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/gocallindex/type_table.go`, `method_calls.go`, `interface_table.go` (mới) và `_test.go`
**Depends on:** BE-CV-TASK-034-01, BE-CV-TASK-030-05
**Status:** [x] DONE

---

## Context

Solution 2.B; CR 2.4. Chỉ mục nhỏ (tên, không nội dung), cache theo `(service, blob oids)`.

## Việc cần làm

1. `type_table.go`: mỗi `struct` → trường → kiểu (`*usecase.T`, interface, `<pkg>v1.<Svc>Client`); tham số hàm tạo.
2. `method_calls.go`: mỗi `Receiver.Name` → danh sách `recv.field.Method(...)`/`recv.Method(...)` (tên trường, phương thức, dòng).
3. `interface_table.go`: interface ở `usecase` → tập phương thức.
4. Phạm vi: chỉ service nằm trên đường đi (thường 2–4); dựng lười.
5. Chịu lỗi parse từng file; không panic.

## Kiểm thử

`go test ./services/code-intel-service/internal/adapter/gocallindex/...` (chưa chạy) trên `mini-flow` và đoạn `relay_by_dev_server.go`: tìm được `uc.devServers.Get`, `uc.agent.Exec`; trường con trỏ/interface/tham số.

## Tiêu chí hoàn thành

- [x] Cả ba bảng có test; kích thước chỉ mục nhỏ (đo, ghi PR).

## Rủi ro và lưu ý

- `go/types` không dùng: gán kiểu theo cú pháp; trường nhúng (embedded) có thể sót.
