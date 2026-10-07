# BE-CV-TASK-023-03: Ánh xạ lỗi agent sang `apperrors` + trailer `x-orca-agent-error-data-bin` (Relay, RelayByDevServer)

**From Solution:** BE-CV-SOL-023-infra-fleet-codeintel-transport
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/agent_rpc_error_mapping.go` (mới), `.../usecase/relay_by_dev_server.go` (sửa dòng 59–62), `.../usecase/relay.go` (sửa nhánh `INFRA_AGENT_EXEC_FAILED`), `.../adapter/grpc/server.go` (sửa handler `RelayByDevServer` dòng ~660–680 và `Relay`), test cùng thư mục
**Depends on:** TASK-023-02
**Status:** [x] DONE

---

## Context

PQ-02 chặng 2 và agent contract §3.4. `apperrors.ToGRPCStatus` chỉ gửi `Code: Message` (`apperrors.go`), nên `candidates`, `jobId`, `runId`, `missing`, `retryable` phải đi bằng trailer. Danh sách mã cho phép và `Kind` ở SOL-023 mục 2.B.

## Việc cần làm

1. `usecase/agent_rpc_error_mapping.go`: `MapAgentExecError(method string, err error) error` theo thứ tự 1–4 của SOL-023 mục 2.B; hằng `allowedAgentErrorCodes` (map mã → `apperrors.Kind`); `cắtRune(message, 300)` và loại ký tự điều khiển; `AgentErrorDataForTrailer(data json.RawMessage, limit int) []byte`.
2. Thuật toán `AgentErrorDataForTrailer`: ≤ limit → nguyên văn; ngược lại giải mã thành `map[string]any`, lặp: bỏ phần tử cuối của mảng dài nhất (ví dụ `candidates`), nếu vẫn vượt thì bỏ khoá không phải `code` theo thứ tự khoá, mã hoá lại; luôn giữ `code`; kết quả xác định (sắp khoá).
3. `RelayByDevServer.Execute` và `Relay.Execute`: thay `apperrors.New(KindInternal, "INFRA_AGENT_EXEC_FAILED", …)` bằng `return nil, MapAgentExecError(in.Method, err)`.
4. Handler `RelayByDevServer` và `Relay` trong `server.go`: trước `return nil, apperrors.ToGRPCStatus(err)`, nếu `errors.As(err, &rpcErr *domain.AgentRPCError)` và `len(rpcErr.Data) > 0` thì `grpc.SetTrailer(ctx, metadata.Pairs("x-orca-agent-error-data-bin", string(AgentErrorDataForTrailer(rpcErr.Data, 4096))))` (import `google.golang.org/grpc/metadata`; kiểm đã có trong `server.go`: có `metadata.FromIncomingContext` ở `withTenantFromStreamMetadata`). `AppError.Unwrap` có (`apperrors.go:59`) nên `errors.As` thấy cause.
5. Không đổi `RelayStream`.

## Kiểm thử

- `cd backend-go/services/infra-fleet-service && go test ./internal/usecase/ -run 'MapAgentExecError|AgentErrorData' -race` và `go test ./internal/adapter/grpc/ -run 'RelayByDevServer|Relay_' -race`.
- Bảng: mỗi dòng bảng 2.B một ca (đúng `Kind` và tiền tố `CODEINTEL_X: `); mã lạ (`CODEINTEL_FOO`) và mã không tiền tố → `INFRA_AGENT_EXEC_FAILED`; `-32601` cho `quality.run` → `CODEINTEL_AGENT_UNSUPPORTED`; `context.DeadlineExceeded` → `CODEINTEL_TIMEOUT`; method `ports.scan` lỗi → `INFRA_AGENT_EXEC_FAILED` như cũ.
- `AgentErrorDataForTrailer`: 12 `candidates` lớn → ≤ 4 KiB, JSON hợp lệ, có `code`; không cắt giữa chuỗi (test unmarshal thành công).
- gRPC in-process: client đọc `grpc.Trailer(&md)` thấy `x-orca-agent-error-data-bin`; message `status` bắt đầu `CODEINTEL_AMBIGUOUS_SYMBOL: `.

## Tiêu chí hoàn thành

- [x] Hợp đồng agent §3.4 bảng ánh xạ đúng từng dòng.
- [x] Trailer ≤ 4 KiB và luôn JSON hợp lệ; thiếu `Data` thì không đặt trailer.
- [x] Hành vi method ngoài hai nhóm không đổi.
- [x] Không log `Data`/`Message` thô ở mức info.

## Rủi ro và lưu ý

- Trailer đi qua `otelgrpc`/interceptor client chưa kiểm (SOL-021 TASK-021-01 kiểm phía nhận).
- `Message` của agent có thể chứa đường dẫn: agent đã hứa che (§3.1) nhưng infra-fleet vẫn cắt 300 rune; không cố che thêm (không biết `workspaceRoot`).
