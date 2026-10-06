# BE-CV-TASK-033-05: Quan hệ C4 (uses, implements, calls-rpc, reads/writes, publishes/subscribes) và luật lớp

**From Solution:** BE-CV-SOL-033-c4-component-view
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/c4/relation.go`, `layer_rules.go` (mới), `backend-go/services/code-intel-service/internal/usecase/derive_c4_view.go` (sửa) và `_test.go`
**Depends on:** BE-CV-TASK-033-04, BE-CV-TASK-031-09
**Status:** [ ] TODO

---

## Context

Solution mục 2.C (quan hệ, luật lớp).

## Việc cần làm

1. `uses` từ import; `implements` từ bằng chứng task 03; `calls-rpc` từ `RpcEdge`; `reads/writes` từ `accessedBy` (`readwrite` → hai cạnh); `publishes/subscribes` từ `Publish|Subscribe|outbox.NewRelay` (topic trống).
2. `evidence` (SymbolRef), `count`, `confidence`, `origin:"derived"`.
3. `layer_rules.go`: bảng hợp lệ/vi phạm; `violatesLayering`; ngoại lệ do 037 cung cấp qua tham số (mặc định rỗng).
4. Ngân sách 2 000 quan hệ → `truncated`.

## Kiểm thử

`go test ... -run C4Relations` (chưa chạy): cây mẫu có `domain→usecase` và `usecase→adapter` bị đánh dấu; `calls-rpc` khớp 032; `reads` khớp 031 trên cùng commit.

## Tiêu chí hoàn thành

- [ ] Mọi cạnh có evidence/conf; vi phạm vẫn được trả (không lọc).
- [ ] Khớp hợp đồng 031/032 trên cùng commit.

## Rủi ro và lưu ý

- Khớp tên có thể trùng nhầm khi interface nhỏ chung tên phương thức.
