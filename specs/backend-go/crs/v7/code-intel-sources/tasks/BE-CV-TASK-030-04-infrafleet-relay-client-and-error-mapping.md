# BE-CV-TASK-030-04: Client `AgentRelay` qua `RelayByDevServer` và ánh xạ lỗi phong bì nhúng

**From Solution:** BE-CV-SOL-030-repo-file-access-gateway
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/infrafleetrelay/relay_client.go` (mới), `.../internal/adapter/agentrepofs/agent_envelope_errors.go` (mới), `.../internal/adapter/agentrepofs/method_whitelist.go` (mới) và các `_test.go` kèm theo
**Depends on:** BE-CV-TASK-030-01, BE-CV-TASK-030-03; `apperrors.KindUnavailable` của BE-CV-SOL-010 (nếu chưa có, chặn task này)
**Status:** [ ] TODO

---

## Context

Mẫu gọi: `api-gateway/.../wscompat/channels_accounts.go:136–160` (`client.RelayByDevServer(ctx, &infrafleetv1.RelayByDevServerRequest{DevServerId, Method, ParamsJson})`) và `git-gateway-service/.../grpcclient/tenant_forwarding.go` (`withTenantMetadata`, `grpcmw.MetadataTenantID`). Không tái dùng `RelayExecutor` của git-gateway (lệch tên trường). Lỗi: SOL-030 mục 2.F (agent chỉ trả mã cho `codeintel.*`/`quality.*`, nên chỉ phân biệt được phong bì nhúng, tiền tố `INFRA_*`, deadline).

## Việc cần làm

1. `relay_client.go`: struct giữ `infrafleetv1.InfraFleetServiceClient`; `Call` gắn metadata tenant bằng `tenant.RequireTenantID(ctx)` + `metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID)` (alias import `fleetv1`, theo PQ-07 quy tắc alias); `json.Marshal(params)`; đặt deadline `CallTimeout`; trả `json.RawMessage(resp.GetResultJson())`.
2. `method_whitelist.go`: hằng `allowedAgentMethods = {fs.readDir, fs.readFile, git.exec, git.status, git.branchCompare, git.branchDiff}` và `allowedGitSubcommands = {log, show, rev-parse, diff, status}`; hàm `assertAllowed(method string, args []string) error` trả `CODEINTEL_INVALID_PARAMS` khi ngoài bảng. Mọi lời gọi của adapter đi qua hàm này trước khi tới `AgentRelay`.
3. `agent_envelope_errors.go`: `detectEnvelopeError(raw json.RawMessage) *AgentEnvelopeError` — giải mã `{error:{code json.Number, message}}` vô điều kiện (bài học `relay_executor.go:128–148`: code là **số**); `mapRelayError(err error) error` theo bảng SOL-030 2.F: tiền tố `INFRA_DEV_SERVER_NOT_CONNECTED` → `CODEINTEL_DEV_SERVER_OFFLINE` (`KindUnavailable`), `INFRA_DEV_SERVER_NOT_FOUND` → `CODEINTEL_NO_DEV_SERVER`, `DeadlineExceeded` → `CODEINTEL_TIMEOUT`, phong bì `-33003` → `domain.ErrNotFound`, `-32601` → `CODEINTEL_TOOL_UNAVAILABLE`, message chứa `FILE_TOO_LARGE` → `CODEINTEL_OUTPUT_TOO_LARGE`, còn lại `CODEINTEL_TOOL_FAILED`.
4. Che đường dẫn tuyệt đối khỏi message (`/…` ≥ 3 phân đoạn hoặc `X:\…` → `<path>`), cắt ≤ 200 ký tự, không chứa nội dung file.
5. Thử lại tối đa 1 lần với `Unavailable` không phải OFFLINE; không retry khi lỗi tham số.

## Kiểm thử

- `go test ./services/code-intel-service/internal/adapter/infrafleetrelay/... ./services/code-intel-service/internal/adapter/agentrepofs/...` (chưa chạy).
- Fake `InfraFleetServiceClient` (server gRPC in-process hoặc interface giả): metadata tenant có mặt ở **mọi** cuộc gọi; hai tenant khác nhau gửi metadata khác nhau (cô lập).
- Bảng lỗi từ fixture của BE-CV-TASK-030-01: phong bì nhúng `code:-33003` (số), `-32601`, `FILE_TOO_LARGE`, `result_json` rỗng, JSON hỏng; status `FailedPrecondition` với message `INFRA_DEV_SERVER_NOT_CONNECTED: …`.
- Test whitelist: gọi `fs.writeFile`, `git.exec ["push"]`, `git.exec ["-c","x=y","log"]` đều bị từ chối trước khi tới relay giả; test liệt kê đúng tập cho phép.

## Tiêu chí hoàn thành

- [ ] Metadata tenant luôn được gắn; thiếu tenant trong ctx → lỗi, không gọi.
- [ ] Phong bì lỗi số được phát hiện; không có đường "lỗi agent thành kết quả thành công".
- [ ] Whitelist có test khẳng định; thêm method lạ làm test đỏ.
- [ ] Message lỗi không chứa đường dẫn tuyệt đối hay nội dung file.

## Rủi ro và lưu ý

- Nếu `BE-CV-SOL-023` sau này mở rộng ánh xạ mã sang `fs.*`/`git.*`, task này đổi nhánh (c) thành đọc mã; ghi chú trong code.
- `MaxCallRecvMsgSize(16 MiB)` đặt ở nơi khởi tạo kết nối gRPC (SOL-021/023); task này chỉ nhận client đã dựng.
