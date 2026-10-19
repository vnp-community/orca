# TASK-V2-026: Output của agent không tới UI (stream theo `connectionId` rỗng); `platform`/`arch`/`nodeVersion` của dev server luôn null

**From Solution:** (G1 và G2 của [TASK-FE-E2E-011](../../../../../frontend/bugs/e2e-acceptance-v1/tasks/TASK-FE-E2E-011-agent-session-monitoring-in-ui.md), lượt `--real monitor` m1 đến m6) · **Resolves:** `AGENT-OUTPUT-NOT-STREAMED` (panel "Agent activity" ở `running` nhưng 0 dòng), `dev-server-platform` vắng
**Priority:** P1 (G1), P2 (G2) · **Service/Area:** task-service, infra-fleet-service, api-gateway, proto (agent: chỉ thêm test)
**Files:**
- G1 proto: `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (+ `infrafleet.pb.go`): `StreamExecOutputRequest.dev_server_id = 3` (chỉ cộng)
- G1 infra-fleet: `internal/usecase/stream_agent_exec_output.go` (+ test), `internal/adapter/grpc/server.go`, `cmd/server/main.go`
- G1 task-service: `internal/usecase/ports.go` (`AgentExecOutputTarget`, `AgentExecOutputStreamer`), `internal/adapter/grpcclient/{agent_exec_output_relay.go, simple_executor.go, agent_output_partial_frame.go (mới)}`; test: `agent_exec_output_relay_test.go`, `simple_executor_test.go`, `agent_output_partial_nats_integration_test.go (mới, -tags integration)`
- G1 api-gateway: `internal/adapter/wscompat/channels_task_activity.go` (lọc tenant), test: `channels_task_activity_test.go`, `channels_task_activity_nats_integration_test.go (mới, -tags integration)`
- G2 infra-fleet: `internal/adapter/{postgres,mysql}/repository.go` (`Get`, `List`, `scanDevServerRow`/`selectDevServerRow`), test tích hợp `{postgres,mysql}/repository_test.go` (`TestRepository_ReadsReturnHandshakeFacts`)
- G2 agent: `agent/src/relay/__tests__/agent-session.test.ts` (chỉ thêm test; không đổi mã agent)
**Depends on:** TASK-V2-025 (Q3: `RecordDevServerHandshake`, `toDevServerView`), TASK-FE-E2E-012 (dòng hoạt động có ký hiệu)
**Status:** `[x]` XONG và đã kiểm trên stack thật (2026-10-10): khung `agent_output_partial` tới trình duyệt, `dev-local-mac` báo `darwin/arm64/v25.6.0`.

---

## G1. Output của agent không bao giờ tới UI

### Gốc rễ (đã tái hiện bằng test; đúng như N dự đoán, kèm hai điểm phụ N chưa nêu)

Chuỗi giao lượt chạy và chuỗi stream dùng HAI cách định danh dev server khác nhau:

| | Lượt chạy `agent.execPrompt` | Stream `StreamExecOutput` |
|---|---|---|
| Project có hàng `infra.connections` | `Relay(connectionId)` | `StreamExecOutput(connectionId)` |
| Project chỉ gắn dev server qua `devServerId` (`resolveViaDefaultRepo`, trả `connectionID == ""`) | `RelayByDevServer(devServerId)` (đã sửa từ BUG-026) | `StreamExecOutput(connectionId = "")` |

`SimpleExecutor.runAgentPrompt` rẽ nhánh `devServerID != ""` cho lời gọi giao việc nhưng vẫn truyền `connectionID` (rỗng) cho stream. infra-fleet từ chối `INFRA_STREAM_EXEC_OUTPUT_NO_CONNECTION`. Lỗi của RPC server-streaming chỉ hiện ra ở `Recv()` đầu tiên (không phải lúc mở), nơi adapter `return` im lặng. Hệ quả: không có chunk nào, `publishThrottledOutput` không bao giờ flush, `task.outbox_events` không có hàng `orca.task.agent_output_partial` nào.

Test tái hiện (đỏ trước khi sửa vì `StreamExecOutput` nhận chuỗi rỗng, xanh sau): `TestSimpleExecutor_Execute_StreamsOutputByDevServerWhenThereIsNoConnection` (project không có hàng `infra.connections`, resolver trả `devServerID`), cùng `TestStreamAgentExecOutput_DevServerFallback_*` ở infra-fleet.

Hai điểm phụ tìm thấy khi đọc từng mắt xích:
- Lỗi mở stream bị nuốt hoàn toàn (không log): đây là lý do lỗi tồn tại lâu mà không ai thấy.
- `publishAgentOutputPartial` nuốt lỗi insert outbox; payload không bị chặn kích thước (NATS giới hạn 1 MiB, một sự kiện quá cỡ làm kẹt cả outbox vì relay dừng ở lỗi đầu tiên) và `\u0000` trong stdout làm Postgres từ chối cột `jsonb` (đã gặp ở `last_execution_output`, TASK-V2-025).

### Thay đổi

- **Proto (chỉ cộng)**: `StreamExecOutputRequest.dev_server_id = 3`. `buf lint` 307 dòng (không tăng), `buf breaking --against main` xanh.
- **infra-fleet `StreamAgentExecOutput`**: nhận `ConnectionID` hoặc `DevServerID`. Đường `ConnectionID` GIỮ NGUYÊN (kể cả khi gửi cả hai: `connectionId` thắng, có test). Khi chỉ có `DevServerID`: `devServers.Get(ctx, tenantID, id)` (tra theo tenant của ctx, giống `RelayByDevServer`: dev server của tenant khác trả `INFRA_DEV_SERVER_NOT_FOUND`) rồi `IsConnected` (không có phiên sống: `INFRA_DEV_SERVER_NOT_CONNECTED`, không dial). Cả hai đều rỗng: `INFRA_STREAM_EXEC_OUTPUT_NO_CONNECTION`.
- **task-service**:
  - `AgentExecOutputStreamer.StreamExecOutput(ctx, AgentExecOutputTarget{ConnectionID, DevServerID, StepID}, onFailure)`; `SimpleExecutor` truyền cả `connectionID` và `devServerID` như lời gọi giao việc.
  - Adapter báo `onFailure` (một lần) khi mở stream thất bại hoặc `Recv` lỗi KHÔNG phải `io.EOF` và KHÔNG phải do ctx bị huỷ (kết thúc bình thường của lượt chạy không bị coi là lỗi). `SimpleExecutor` ghi `WARN task: agent output stream failed; the run continues without live output` với `task_id`, `request_id`, `dev_server_id`, `grpc_code`, `error` (không có nội dung output; test khẳng định). Lượt chạy vẫn tiếp tục và thành công.
  - Phân biệt thêm: lượt chạy xong mà không có chunk nào và stream không lỗi: `INFO task: agent run produced no streamed output`.
  - Lỗi marshal/insert outbox: `WARN` (task id, không có nội dung).
  - `boundPartialStream`: bỏ NUL và UTF-8 hỏng, giữ phần ĐUÔI tối đa 128 KiB mỗi luồng (khung là toàn bộ output cộng dồn; agent đã giới hạn dòng hoạt động 96 KiB nên chỉ chạm giới hạn ở chế độ text thô).
- **api-gateway `task.activity.subscribe`**: lọc thêm theo tenant của người đăng ký (`ev.TenantID` khác `Identity.TenantID` thì bỏ; bỏ qua khi một bên rỗng). Trước đây chỉ lọc theo `origin_task_id`: người biết task id của tenant khác sẽ thấy output agent của họ.

### Kiểm từng mắt xích tới UI

| # | Mắt xích | Kết luận | Bằng chứng |
|---|---|---|---|
| 1 | agent: `notify('agent.execPrompt.output', {stepId, stream, data})` qua `makeNotifier(ws, state)` cho đường WebSocket; `stepId` = `StepID` mà `SimpleExecutor` gửi (= `requestID`) | khớp | đọc `agent-rpc-dispatch-agent-exec.ts`, `agent-exec-prompt-output-capture.ts` |
| 2 | infra-fleet `session.routeNotification` demux theo `stepId` tới `subscribeExecOutput`; cùng `getOrCreateSession` với `RelayByDevServer`/`Exec` (cùng phiên) | khớp | đọc `session.go`, `client.go`; test `devserveragent` hiện có |
| 3 | gRPC server-streaming infra-fleet → task-service, tenant qua metadata (`withTenantFromStreamMetadata`) | khớp; ĐÂY là chỗ gãy (G1) | sửa ở trên |
| 4 | `publishThrottledOutput`: gom cộng dồn, flush tối đa 1 lần / 2 giây; chỉ nối chuỗi nên các ký hiệu `· » ▸ ↳ ■` (UTF-8 nhiều byte, chunk luôn là chuỗi JSON hợp lệ) đi qua nguyên vẹn | xác nhận | `TestAgentOutputPartial_ReachesNATSThroughTheOutboxRelay` so sánh đúng từng dòng có ký hiệu |
| 5 | outbox → `outbox.Relay` → stream NATS `TASK` (`orca.task.>`, tạo trong `cmd/server/main.go`) subject `orca.task.agent_output_partial` | xác nhận với NATS thật | test tích hợp cùng tệp trên (dựng stream và relay y như `main.go`). Không như stream `INFRA` trước đây: `orca.task.>` đã nằm trong `TASK` |
| 6 | gateway `task.activity.subscribe`: 7 consumer tạm thời (có `orca.task.agent_output_partial` ở stream `TASK`), lọc `origin_task_id` + tenant, đẩy `task.activity.event` `{taskId, engine:"direct_agent", eventType:"agent_output_partial", payload:{origin_task_id, request_id, stdout, stderr}, occurredAt}` | khớp hình dạng mà `task-agent-output-store.ts` đọc (`eventType`, `payload.stdout/stderr/request_id`, cộng dồn, không phải delta); không lệch tên trường | `TestTaskActivity_AgentOutputPartialOverRealNATS` (NATS thật, literal payload giống hệt task-service, kiểm đúng các khoá JSON và frame của tenant khác không tới) |
| 7 | Giới hạn: payload task-service ≤ 128 KiB mỗi luồng; frontend giữ đuôi 512 KiB; ephemeral consumer dùng `DeliverAll` nên mở sheet giữa chừng nhận lại các khung cũ của cùng task rồi tới khung mới nhất (cộng dồn nên khung cuối thắng) | chấp nhận, ghi lại | đọc `common/eventbus.SubscribeEphemeral` |

## G2. `platform`/`arch`/`nodeVersion` luôn null

### Gốc rễ (khác dự đoán: KHÔNG phải do agent)

- Agent ĐÃ gửi đủ trong `agent.handshake`: `platform: process.platform`, `arch: process.arch`, `nodeVersion: process.version` (`agent-session-handshake.ts`); infra-fleet `agentwsserver` đọc vào `HandshakeInfo` (mặc định `linux`/`x64`/`unknown`), `RecordDevServerHandshake` ghi vào cột `platform/arch/node_version/agent_version` bằng `UpdateProvisionResult` (Q3). Q3 chứng minh `pending → healthy` đổi được, tức lệnh UPDATE chạy, và UPDATE đó ghi cả bốn cột cùng lúc.
- Lỗi nằm ở phía ĐỌC: `Repository.Get`, `List` và `scanDevServerRow`/`selectDevServerRow` (cả Postgres và MySQL) chưa bao giờ `SELECT` bốn cột này, nên `domain.DevServer.Platform/Arch/NodeVersion/AgentVersion` luôn rỗng, `toProtoDevServer` gửi chuỗi rỗng, gateway `nonEmptyPtr` thành `null`. Test cũ `TestRepository_UpdateProvisionResult` đọc cột bằng SQL thô nên không thấy lỗ hổng này.

### Thay đổi

- `Get`, `List`, `UpdateApprovalStatus`/`AssignGroup` (RETURNING / đọc lại) ở cả hai dialect đọc thêm `COALESCE(platform,'')`, `arch`, `node_version`, `agent_version`. Cột NULL cho chuỗi rỗng (server chưa bắt tay): hành vi cũ giữ nguyên.
- Test tích hợp `TestRepository_ReadsReturnHandshakeFacts` (Postgres và MySQL): rỗng trước bắt tay, đủ giá trị sau `UpdateProvisionResult` ở `Get`, `List`, `UpdateApprovalStatus`, `AssignGroup`, kèm `healthStatus`.
- Agent: không đổi mã (đã gửi đủ, đa nền tảng vì lấy từ `process.*`; đường `--stdio`/SSH gửi cùng ba trường trong `relay-handshake.ts`, không đụng). Thêm test vitest: handshake WebSocket mang `platform`, `arch`, `nodeVersion` (`agent-session.test.ts`).
- Tác dụng phụ đã xét: `domain.DevServer.AgentVersion` giờ có giá trị khi đọc qua `Get`/`List`. `StartAgentSession` và `ResumeAgentSession` lấy dev server từ `ConnectionResolver.ResolveConnection` (truy vấn khác, không đổi) nên so sánh phiên bản BR-AG-09 không thay đổi.

## Blast radius (codegraph, tên chính xác; gitnexus không có)

- `AgentExecOutputRelay`/`StreamAgentExecOutput`: 1 caller sản xuất (`SimpleExecutor.runAgentPrompt`, qua `Execute` và `ExecuteWithContract`); `NewStreamAgentExecOutput`: 1 nơi dựng (`infra-fleet cmd/server/main.go`) + 5 test. Proto: chỉ cộng một trường; client cũ không gửi `dev_server_id` giữ hành vi cũ.
- `toProtoDevServer`: 8 RPC infra-fleet (`RegisterDevServer`, `ResolveConnection`, `ListDevServers`, `ListDevServersByTag`, `ApproveDevServer`, `RejectDevServer`, `AssignDevServerGroup`, `ListDevServersForUser`); chỉ `ListDevServers`, `Approve/Reject`, `AssignGroup` đi qua `Get`/`List`/`scanDevServerRow` và nay có thêm bốn trường.
- `translateToTaskActivity`: 1 caller (`RegisterTaskActivityStreamChannel`), 4 test; hành vi đổi duy nhất: bỏ frame của tenant khác.
- Hành vi đổi cho người dùng: panel "Agent activity" nhận khung cho project gắn dev server qua `devServerId`; thẻ dev server có platform/arch/Node/agent version (sau lần bắt tay kế tiếp nếu chưa ghi).

## Kết quả kiểm (2026-10-10, máy dev, Colima)

Môi trường: `DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock TESTCONTAINERS_RYUK_DISABLED=true`, `migrate` và `buf` trong PATH.

- task-service: `go build ./... && go vet ./... && go vet -tags integration ./... && go test ./...` PASS; `go test -tags integration ./internal/adapter/grpcclient -run TestAgentOutputPartial_ReachesNATS` PASS (NATS testcontainers; không đụng repo nên không chạy lại bộ MySQL 25 phút).
- api-gateway: `go build/vet ./...`, `go vet -tags integration`, `go test ./...` PASS; `go test -tags integration ./internal/adapter/wscompat -run TestTaskActivity_AgentOutputPartialOverRealNATS` PASS. Tệp vàng `channel_inventory.json` không đổi (không thêm kênh).
- infra-fleet-service: `go build ./... && go vet ./...` sạch; `go test ./...` PASS trừ hai lỗi có sẵn (`backendrelaysshprovisioner`, `ephemeralsshconn`: `bind: invalid argument` của unix socket); tích hợp `-tags integration` (Postgres 86 giây, MySQL 256 giây): `TestRepository_ReadsReturnHandshakeFacts` và `TestRepository_UpdateProvisionResult` PASS ở cả hai dialect; còn đỏ các test KHÔNG liên quan thay đổi này và không đụng bởi nó: `TestMigration0018_DownDropsTable` và `TestMigration0038_DownUpRoundTrip` (Postgres), `TestMigration0038_DownUpRoundTrip` (MySQL) (`down 1` nay lùi migration 0040 thay vì 0038, đã nêu ở IMPACT-CODEGRAPH mục H3) và `TestRepository_ResolveConnection_FoundAndNotFound` (Postgres; test đăng ký dev server không có hàng `infra.connections` nhưng `ResolveConnection` đòi hàng đó, mã không do task này đổi).
- Mọi service Go trong `go.work` biên dịch (`go build ./...`); `quality-service` không nằm trong `go.work` (có sẵn).
- proto: `buf generate` (header về đúng bản đã commit; chỉ `infrafleet.pb.go` đổi thêm so với tay Q), `buf lint` 307 dòng, `buf breaking --against '../../.git#branch=main,subdir=backend-go/proto'` xanh.
- agent: `pnpm exec vitest run src/relay/__tests__/agent-session.test.ts` PASS (24 test, gồm test mới).

## Việc cần làm

- [x] G1: xác nhận gốc rễ bằng test (project không có hàng `infra.connections`), `dev_server_id` ở proto, infra-fleet tra dev server theo tenant, `SimpleExecutor` truyền `devServerID`, giữ nguyên đường connection.
- [x] G1: không nuốt lỗi: log WARN (task id, không nội dung), phân biệt stream hỏng với lượt chạy vẫn tiếp tục, log INFO khi không có output.
- [x] G1: kiểm từng mắt xích tới UI; khoá tenant ở gateway; chặn kích thước và NUL của khung; test NATS thật cho đường publish (task-service) và đường subscribe (gateway).
- [x] G2: sửa đường đọc hai dialect, test tích hợp hai dialect; agent: test handshake.
- [ ] Redeploy `infra-fleet-service`, `task-service`, `api-gateway` (người điều phối); build lại agent KHÔNG cần cho G1/G2 (không đổi mã agent; G1 vẫn cần agent đã có định dạng dòng hoạt động của TASK-FE-E2E-012).
- [ ] Chạy lại `ORCA_E2E_GUARD_IGNORE=specs/:docs/ python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real monitor` (agent N): kỳ vọng panel hiện `▸ Read/Edit/Bash`, `dev-server-platform` có giá trị.
- [ ] Commit.

## Chưa kiểm chứng (nói thẳng)

- Chưa chạy trên stack thật: chưa thấy một khung `agent_output_partial` thật từ Claude thật tới trình duyệt. Đã chứng minh từng đoạn bằng test nhưng không có một test duy nhất đi từ agent thật tới gateway.
- Chưa có log runtime xác nhận `connectionID == ""` trên stack của N (RPC streaming trước đây không ghi log). Gốc rễ được xác nhận bằng tái hiện trong test và đọc mã, khớp với toàn bộ triệu chứng (0 hàng `orca.task.agent_output_partial`, lượt chạy đi `RelayByDevServer`).
- Điều kiện đua còn lại: `StreamExecOutput` được mở đồng thời với `Relay`, không chờ xác nhận đăng ký; chunk đầu (`· Session started`) có thể tới trước khi đăng ký nếu infra-fleet chậm hơn việc khởi động `claude` (thường mất hơn 1 giây). Khung cộng dồn kế tiếp không bù lại phần đã mất. Chưa sửa (cần ack ở đầu stream); nếu e2e thật thấy mất dòng đầu thì xử lý.
- Dev server đã bắt tay trước khi infra-fleet được redeploy chỉ có `platform` sau lần bắt tay kế tiếp nếu `RecordDevServerHandshake` chưa từng ghi cho nó (redeploy infra-fleet làm agent nối lại nên thường tự có).
- `DeliverAll` của consumer tạm thời (gateway) phát lại mọi khung cũ của cùng task khi mở sheet: không sai (khung cuối thắng) nhưng tốn khi stream `TASK` giữ nhiều khung; chưa có giới hạn lưu giữ cho stream này (ngoài phạm vi).
- Chế độ text thô của agent (claude cũ không có stream-json) cắt chunk theo byte có thể chia đôi ký tự UTF-8 trước khi tới Go; Go chỉ bỏ phần hỏng của khung, không sửa lại ký tự.

## Cần redeploy

`infra-fleet-service` (đọc cột, stream theo dev server), `task-service` (truyền `devServerID`, log), `api-gateway` (lọc tenant). Mã sinh proto đổi ở `infrafleet` (cả ba service dùng). Không migration, không biến môi trường mới, không cần build lại agent.
