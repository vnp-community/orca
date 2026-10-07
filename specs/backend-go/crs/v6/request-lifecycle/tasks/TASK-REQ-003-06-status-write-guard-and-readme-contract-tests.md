# TASK-REQ-003-06: Test kiến trúc cấm ghi `status` ngoài `TransitionRequest` và test hợp đồng README

**From Solution:** BE-REQ-SOL-003
**Priority:** P1
**Service:** `request-service`
**File:** `internal/usecase/status_write_guard_test.go`, `internal/domain/readme_contract_test.go` (mới)
**Depends on:** TASK-REQ-003-03
**Status:** [x] DONE

---

## Context

CR-REQ-003 mục 2.3 và Q3 đề nghị kiểm tra tự động cấm ghi `status` ngoài use case; đề xuất gốc là grep CI trên `SET status`, nhưng `RequestRepository.Update` ghi mọi cột nên grep SQL vô dụng (Correction C2 của SOL-003). Mục 5 của CR cũng yêu cầu test đọc README v6 để lệch tên trạng thái/loại bị bắt ở CI.

## Việc cần làm

1. `status_write_guard_test.go`: dùng `go/parser` và `go/ast` đọc mọi `*.go` (trừ `*_test.go`) trong thư mục `internal/usecase`; duyệt `*ast.AssignStmt` và `*ast.IncDecStmt`; nếu vế trái là `*ast.SelectorExpr` có `Sel.Name` thuộc {`Status`, `ReturnedFromStage`, `ReturnReason`} và file không phải `transition_request.go` thì `t.Errorf` kèm vị trí. Thêm danh sách tên có thể mở rộng (`ReturnedCategory` do TASK-REQ-006-02 thêm).
2. Phủ cả composite literal: `domain.Request{Status: ...}` trong `usecase` cũng vi phạm (kiểm `*ast.KeyValueExpr` có khoá `Status` trong literal kiểu `domain.Request`).
3. `readme_contract_test.go`: đọc `../../../../../docs/crs/v6/README.md` (đường dẫn tương đối từ thư mục test tới repo; nếu file không có, `t.Skip` với lý do), trích 11 giá trị loại ở mục 3.2 và 11 trạng thái ở mục 3.3 bằng regex trên đoạn mã (`` `...` ``), và so với `AllRequestTypes()`, `AllRequestStatuses()`. Lệch thì fail.
4. Tên test: `TestOnlyTransitionRequestWritesStatus`, `TestREADMEListsSameTypesAndStatuses`.

## Kiểm thử

- Đối chứng: tạo tạm một file `usecase/zz_bad.go` có `r.Status = domain.StatusNew`, xác nhận test đỏ, rồi xoá (không commit).
- Lệnh: `go test ./services/request-service/internal/usecase/... ./services/request-service/internal/domain/... -run "OnlyTransition|README"`.

## Tiêu chí hoàn thành

- [x] Test xanh trên cây hiện tại, đỏ khi thêm file gán `Status` trong `usecase`.
- [x] Test README xanh, đỏ nếu đổi tên một trạng thái trong `domain`.
- [x] Không phụ thuộc mạng.

## Rủi ro và lưu ý

- README v6 có thể bị đổi định dạng; regex phải chịu được dấu phẩy và xuống dòng; nếu quá mong manh, dùng danh sách sao chép trong test kèm ghi chú nguồn (CR-REQ-003 mục 5 chấp nhận "hằng sao chép từ README").
- Gán qua hàm trung gian (`setStatus(r, ...)`) trong `usecase` vẫn lọt; bổ sung bằng review.
