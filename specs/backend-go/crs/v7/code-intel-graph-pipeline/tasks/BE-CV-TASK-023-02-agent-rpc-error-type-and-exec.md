# BE-CV-TASK-023-02: `domain.AgentRPCError` và `Client.Exec` giữ lỗi có cấu trúc cho `codeintel.*`/`quality.*`

**From Solution:** BE-CV-SOL-023-infra-fleet-codeintel-transport
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/domain/agent_rpc_error.go` (mới), `.../adapter/devserveragent/client.go` (sửa `Exec`, dòng ~423–452), `.../adapter/devserveragent/client_test.go` hoặc `agent_rpc_error_test.go` (mới)
**Depends on:** TASK-023-01
**Status:** [ ] TODO

---

## Context

Hiện `Exec` chỉ nhận diện `-32601` (`client.go` ~431–439) và trả `*JSONRPCError` thô cho các lỗi khác; `error.data.code` của agent không tới use case. Hợp đồng agent §3.4: **chỉ khi** `method` bắt đầu `codeintel.` hoặc `quality.`. `domain` không được import `adapter` (`arch/03`).

## Việc cần làm

1. Tạo `domain/agent_rpc_error.go`: `AgentRPCError{Code int; Message string; Data json.RawMessage}`, `Error()`, `Unwrap()` trả `ErrAgentMethodNotFound` khi `Code == -32601` (SOL-023 mục 2.B).
2. `Client.Exec`: trước nhánh `-32601` hiện có, thêm nhánh: nếu method có tiền tố `codeintel.` hoặc `quality.` và `errors.As(err, &rpcErr *JSONRPCError)` thì `return nil, &domain.AgentRPCError{Code: rpcErr.Code, Message: rpcErr.Message, Data: rpcErr.Data}`. Mọi method khác giữ nguyên (kể cả `fmt.Errorf("%w: %v", domain.ErrAgentMethodNotFound, err)`).
3. Hàm nhỏ `isCodeIntelMethod(method string) bool` đặt trong `exec_timeouts.go` hoặc file riêng `codeintel_methods.go` (tên theo khái niệm) để TASK-023-03/04 dùng chung; **không** đặt tên `helpers`.
4. Không đổi kiểu trả về của `Exec` (`(map[string]any, error)`).

## Kiểm thử

- `cd backend-go/services/infra-fleet-service && go test ./internal/adapter/devserveragent/ -run 'AgentRPCError|ClientExec' -race`.
- Ca (dùng `startFakeAgent` ở `client_test.go:310`): agent trả `{code:-32000, message:"x", data:{code:"CODEINTEL_INDEX_MISSING"}}` cho `codeintel.overview` → `errors.As` ra `*AgentRPCError` có `Data` nguyên vẹn; `-32601` cho `codeintel.status` → `errors.Is(err, ErrAgentMethodNotFound)` đúng **và** `errors.As` ra `AgentRPCError`; lỗi cho `ports.scan` vẫn là kiểu cũ (`*JSONRPCError` hoặc wrap `ErrAgentMethodNotFound`) — test hồi quy kiểu lỗi.
- Test `TestClientExec_VmProvisionMethodNotFound` đang có (`client_stream_vm_provision_test.go:310`) vẫn xanh.

## Tiêu chí hoàn thành

- [ ] `AgentRPCError` giữ nguyên `Code`, `Message`, `Data`.
- [ ] Hành vi mọi method ngoài hai nhóm không đổi (test hồi quy).
- [ ] `go vet` và `go test ./internal/adapter/devserveragent/... -race` xanh.

## Rủi ro và lưu ý

- `Data` có thể chứa đường dẫn/ứng viên symbol: **không log** `Data` ở đây.
- Nếu TASK-023-01 phát hiện người tiêu thụ khác dựa vào `*JSONRPCError` cho `codeintel.*` thì cập nhật trước.
