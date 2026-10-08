# TASK-REQ-016-03: Kênh `request.*` (vòng đời, phân loại, plan, phase, cờ)

**From Solution:** BE-REQ-SOL-016
**Priority:** P0
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_request.go`, `.../channels_request_flow.go` (mới), `.../channels_request_test.go` (mới), `backend-go/services/api-gateway/internal/adapter/mcpserver/tools/excluded_channels.yaml`
**Depends on:** TASK-REQ-016-02; RPC của CR-REQ-004, 005, 006, 012, 013, 025 (thêm kênh nào khi RPC của nó đã có trong proto)
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: cd backend-go/services/api-gateway && go build ./... && go vet ./... && go test ./... -count=1)

---

## Context

- Mẫu handler: `channels_task_source.go:22-60` (`r.Register(name, func(ctx, id Identity, args []json.RawMessage) (any, error)`, `decodeArg[T](args, 0)`, request gRPC dùng `id.TenantID` khi proto có trường; `request-service` lấy tenant từ metadata do `Dispatch` gắn bằng `AttachIdentity`).
- Hằng `groupRPCTimeout = 8 * time.Second` ở `channels_issuetracking_orchestration.go:27`.
- `handler.go:247` `invokeTimeout = 25s` chặn mọi dispatch; hai kênh AI đặt 24s (SOL-016 mục 1 điểm 2).
- Bảng kênh, tham số, kết quả: CONTRACT mục 2.1. Ánh xạ tên: SOL-016 mục 2.4.

## Việc cần làm

1. Trong `registerRequestChannels`, đăng ký: `request.create`, `request.get`, `request.list`, `request.typeHistory`, `request.classify`, `request.confirmType`, `request.changeType`, `request.returnToBacklog`, `request.reopen`, `request.cancel`, `request.spawnChild`, `request.generatePlan`, `request.startPhase`. Mỗi kênh: `requestClientMissing` thì `errRequestUnavailable`; `decodeArg`; kiểm enum ở gateway; `context.WithTimeout` đúng bảng (8s, 15s cho `startPhase` và `generatePlan` commit, 24s cho `classify` và `generatePlan` propose); gọi RPC; lỗi qua `requestChannelError`; kết quả qua view.
2. `request.create`: dùng `resolveRequestSource`; `hints` ánh xạ `SourceHints`; trả `{request, created}`.
3. `request.generatePlan`: `mode` rỗng hoặc `propose` thành `PlanMode.PROPOSE`; `commit` bắt buộc `proposal` (kiểm không nil, nếu không `INVALID_ARGUMENT: proposal required for commit`); chuyển `proposal` và `rawAiResponse` nguyên văn (kiểu `PlanProposal` ánh xạ từ JSON bằng `protojson.Unmarshal`, giới hạn 256 KB trước khi giải mã).
4. `request.list`: `pageSize` mặc định 20, kẹp tối đa 100 ở gateway; trả `{requests, nextPageToken}`, không kèm `body`.
5. `channels_request_flow.go`: `request.flowStatus` (`GetRequestFlowSettings`), `request.flowSet` (`SetRequestFlowSettings`; kiểm `id.Role == "admin"` ở gateway để lỗi rõ, `request-service` vẫn kiểm lại). Chỉ đăng ký khi CR-REQ-025 task cờ đã thêm RPC; trước đó bỏ qua hai kênh này.
6. `excluded_channels.yaml`: thêm từng kênh đã đăng ký, `category: request_flow`, `reason: "Chờ BE-REQ-SOL-017"`.

## Kiểm thử

- `channels_request_test.go` với `fakeRequestClient` (mẫu `fakeOrchestrationClient` trong `channels_orchestration_test.go`): mỗi kênh một ca thành công và một ca lỗi; assert ánh xạ tham số (ví dụ `toType` đến `NewType`), deadline (`ctx.Deadline()` trong fake nằm trong khoảng kỳ vọng), `tenantId` giả trong args không đi vào RPC.
- Ca `request.create` với `source.provider=mcp` từ WS thì `REQUEST_SOURCE_FORBIDDEN`; với `ToolOrigin` thì `mcp`.
- `registry_channels_test.go`: kênh có trong `Registry.Channels()`.
- `go test ./internal/adapter/wscompat/... ./internal/adapter/mcpserver/tools/...` (parity xanh).

## Tiêu chí hoàn thành

- [x] Từng kênh trong CONTRACT 2.1 (trừ `subscribe`) có test.
- [x] Không kênh nào nhận `tenantId`, `userId`, `reporterId`.
- [x] `request.list` kẹp `pageSize`, trả `[]` khi rỗng.
- [x] Parity test xanh; không kênh nào thiếu dòng loại trừ.

## Rủi ro và lưu ý

- `ClassifyRequest` và `GeneratePlan` propose có thể vượt 24s cho tới khi CR-REQ-005, 012 làm RPC trả sớm (CONTRACT Q2); test chỉ khẳng định deadline, không mô phỏng AI.
- Thêm kênh theo từng PR khi RPC tương ứng đã có trong proto để không có kênh gọi RPC `Unimplemented`.

## Ghi chú triển khai (2026-10-08)

Thêm kênh `request.planProposal` (đọc kết quả `GeneratePlan` bất đồng bộ, RPC `GetPlanProposal`) vì propose chỉ trả `runId`. `request.classify` trả thêm `runId`.
