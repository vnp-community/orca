# SOL-EMU-005: `AgentKind` trong `infra-fleet-service`

**Resolves:** CR-DS-009 §3.1, Phase 3
**Service:** `backend-go/services/infra-fleet-service`
**Status:** ✅ DONE
**Task(s):** [TASK-EMU-007](../tasks/TASK-EMU-007-proto-agent-kind.md)

## Giải pháp (đã triển khai)

Thêm `enum AgentKind {AGENT_KIND_UNSPECIFIED, AGENT_KIND_DEV_SERVER,
AGENT_KIND_MOBILE_EMULATOR}` + field `kind` trên `DevServer` và
`RegisterDevServerRequest` trong `infrafleet.proto`. Migration
(`0011_dev_server_kind`) backfill `kind = 'dev_server'` (DEFAULT) cho hàng
hiện có. `ListDevServers`/`ListDevServersForUser` nhận filter `kind`
(rỗng = không filter) để UI tách "Dev Servers" khỏi "Mobile Emulator
Agents" — chưa có UI dùng filter này (Phase 5, chưa bắt đầu), nhưng RPC
contract đã sẵn sàng.

## Vì sao pass trước "chưa làm"

Pass trước dừng lại vì chưa xác nhận môi trường sandbox có sẵn
`protoc`/`buf` tương thích. Pass này xác nhận CÓ (`buf` 1.72.0, `protoc`
3.21.12, `protoc-gen-go`/`protoc-gen-go-grpc` đều có trong `$PATH`), chạy
`buf generate` idempotent trên trạng thái `.proto` hiện tại trước khi sửa gì
(xác nhận toolchain hoạt động đúng), rồi mới sửa `.proto` + regenerate +
implement domain/usecase/postgres/grpc + test. `go build`/`go vet`/`go test`
xanh cho `infra-fleet-service` và cả 6 service khác import `infrafleetv1`.
Xem TASK-EMU-007 để có lệnh + output thật.
