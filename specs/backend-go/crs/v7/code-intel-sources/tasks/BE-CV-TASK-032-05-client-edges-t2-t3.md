# BE-CV-TASK-032-05: Cạnh client→server (T2 kiểu cú pháp, T3 khớp tên) và đích `agent-method`

**From Solution:** BE-CV-SOL-032-proto-and-wscompat-contract-catalog
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/gocallgraph/client_type_table.go`, `client_call_edges.go` (mới) và `_test.go`
**Depends on:** BE-CV-TASK-032-04
**Status:** [ ] TODO

---

## Context

SOL-032 mục 2.C.3. Client gRPC nằm ở `adapter/grpcclient`, `scmstarcheck`, `serviceclients`, `cmd/server/main.go`, `wscompat`.

## Việc cần làm

1. Quét `internal/**`, `cmd/**` mọi service theo hai bước (liệt kê file chứa `v1.`+`Client` rồi parse), cache theo blob oid.
2. Bảng kiểu: tham số/trường/biến kiểu `<alias>.<Svc>Client`, `x := <alias>.New<Svc>Client(...)`.
3. Lời gọi `<recv>.<Rpc>(…)`/`<x>.<field>.<Rpc>` → T2 (0,9); file có tham chiếu client + tên phương thức trùng RPC → T3 (0,6, nhãn suy luận); `From` = hàm bao.
4. `RelayByDevServer`/`Relay`: `Method` literal → thêm `to: agent`, `AgentMethod`; biến → `CHANNEL_DYNAMIC`-tương đương `AGENT_METHOD_DYNAMIC` (mã dùng ở 034).
5. Rút gọn `service→service` (số RPC/vị trí) cho 033/037.

## Kiểm thử

`go test ./services/code-intel-service/internal/adapter/gocallgraph/... -run Client` (chưa chạy): test phân biệt T2/T3 với tên phương thức trùng ở hai service; cạnh `workflow→automation`, `tenant→scm-integration` có evidence; ngân sách 3 000 cạnh → `truncated`.

## Tiêu chí hoàn thành

- [ ] Mọi T3 có nhãn suy luận.
- [ ] Hai cạnh mẫu có file:dòng.

## Rủi ro và lưu ý

- 834 vị trí khớp của CR có dương tính giả (tên chung `Get/List`); đo tỷ lệ T2/T3 và ghi vào PR.
