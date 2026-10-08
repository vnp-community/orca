# IMPLEMENTATION-NOTES: security-compliance (CR-REQ-035)

## Đợt S1: `common/secretscan` và `x-orca-actor-type` (2026-10-07)

Phạm vi: TASK-REQ-035-01 (xong), TASK-REQ-035-02 (một phần). Các task 03 đến 09 chưa làm.

### Quyết định lệch so với task
- `secretscan`: mẫu chung (`dotenv_secret`, `secret_assignment`) chỉ lấp vùng chưa bị mẫu cụ thể chiếm, thay vì "dài nhất thắng". Vì `token: ghp_x@host` phải ra `github_token` và giữ `@host`. Hậu quả: ở trường hợp chồng lấp, kết quả che khác `mcp-service` nhưng không bao giờ lộ nhiều hơn.
- `secretscan.Redact` bỏ qua giá trị bắt đầu bằng `[REDACTED` để idempotent và `changed=false` ở lượt hai (mẫu `mcp-service` tự che lại marker).
- `RedactKinds` trả `Result{Text, Kinds, Truncated}` (theo task); phần sau 1 MiB không quét, giữ nguyên, người gọi phải xét `Truncated` (task 09 cần chặn hoặc từ chối đầu vào quá lớn ở cổng vào).
- `Executor.CallTool` ép `agent` bất kể ctx đã mang actor nào (kể cả `system`), vì MCP luôn là agent.
- Đường `Dispatch` nội bộ của `pty_session_reaper`/`pty_session_registry` không đánh `agent` (không tới `request-service`).

### Chưa kiểm chứng
- 035-02: chưa có đường gateway → `request-service` thật (`dialRequestService` chưa được `main.go` gọi, chờ TASK-REQ-016-01); chưa thử với server `request-service` có `Guard`.
- Độ phủ `secretscan` với dữ liệu thật của khách (CR mục 5) và độ lệch với `mcp-service` (không có test chéo module).
- Kiểm tra toàn workspace: `common/policy` `TestEvaluator_InvalidateIfBundleChanged_PicksUpEditWithinCheckPeriod` và `auth-service` `TestUpdateAccessPolicy_PublishedChange_VisibleToLiveEvaluator_NoRestart` thất bại trong môi trường này; chúng liên quan tới bộ đánh giá policy (mtime/thời gian), không thuộc các gói đã sửa. Chưa chạy lại trên cây sạch để so sánh. `cmd/orca-cli`, `automation-service` không build được vì thiếu mạng tải module.

### Câu hỏi mở
- Q5 của solution (cùng vector cho `mcp-service`): chủ sở hữu `mcp-service` nên thêm test đọc `common/secretscan/testdata/vectors.json`.
- Dòng cấu hình `REQUEST_INTERNAL_CALLER_TOKEN` cho README `api-gateway` (README chưa có bảng biến).

## Đợt S2: `request-service` (035-03 đến 035-09, 2026-10-08, nhánh `rf/sec`)

Kiểm chứng: `opa test policy/orca-authz` 120/120; `go test ./services/request-service/...` xanh; `go test -tags integration ./services/request-service/...` xanh trên Postgres 16 và MySQL 8.0 thật (container testcontainers); `go vet` (có tag integration), `gofmt -l` sạch.

### Thứ tự chuỗi interceptor (điểm cắm cho khi hợp nhất với `rf/roll`)
`grpcmw.ChainUnary` (recovery, tenant/identity, logging) đứng trước, rồi `requestgrpc.SecurityChain(SecurityChainConfig)` nối theo thứ tự:
1. `internalcaller.Guard` (token gateway cho RPC công khai, token dịch vụ cho RPC nội bộ; token rỗng từ chối hết)
2. `ActorTypeInterceptor`
3. `AfterGuard` (danh sách có thứ tự): đặt `FlowGate` của rf/roll ở đây
4. `RequestAccessInterceptor` (authz theo Catalog + Rego)
5. `AfterAuthz` (danh sách có thứ tự): đặt `AuditRPC` của rf/roll ở đây (chỉ chạy cho lời gọi đã được phép; lời gọi bị từ chối đã có audit `request.access.denied`)
6. `RateLimitInterceptor`
Stream: `StreamGuard` x2, `StreamIdentityInterceptor`, `StreamAccessInterceptor`. Nối: `wireSecurity(..., securityHooks{AfterGuard, AfterAuthz})` trong `cmd/server/wire_security.go`; `main.go` truyền `rollout.SecurityHooks()` (đã hợp nhất rf/roll 2026-10-08; FlowGate ở `AfterGuard`, AuditRPC ở `AfterAuthz`). Đã có test thứ tự (`TestChain_HookPointsRunInTheDocumentedOrder`). Guard của `LookupRequestBySource` đã có sẵn: RPC này là nhóm `internal`, nên cần token dịch vụ (`SERVICE_INTERNAL_TOKEN`, phía `issue-status-sync` là `REQUEST_SERVICE_INTERNAL_TOKEN`, cùng giá trị); rf/roll không cần Guard riêng. Cả hai nhánh đều lọc `/orca.request.v1.`; `flow_gate.go` của rf/roll nên dùng `domain.IsGuardedMethod`. Hai nhánh cùng sửa `main.go` và `store_wiring.go`: xung đột cơ học.

### Quyết định lệch so với task
- Ma trận agent: giữ `ChangeRequestType`, `ReturnToBacklog`, `ReopenRequest` cho agent (tool MCP có thể đảo ngược đã phát hành), thêm các RPC `authenticated` chỉ đọc; `GenerateSolution` và `ClassifyRequest` được phép (sửa `AgentAllowed:false` sai của bản cũ). `GeneratePlan`, `StartPhase`, `CancelRequest`, `ConfirmRequestType`, `Approve`, `Reject`, mọi admin: luôn từ chối.
- Phân nhóm RPC không có trong bảng solution (clarification, decision, impact, readiness...): suy luận, cần chủ sản phẩm xác nhận; `RecordRequestCheck`, `RequestClarification`, `RequestApproval`, `ReportTaskOutcome`, `LookupRequestBySource` là nội bộ.
- Cờ secret/erase ở bảng bên `request_security_flags`; cài đặt ở `tenant_security_settings`; migration `0080`.
- Webhook nonce kiểm trực tiếp trong tiến trình (không RPC `RecordWebhookNonce`, vì không sửa proto).
- Bộ giới hạn tốc độ tự viết; metric chỉ là bộ đếm trong tiến trình.

### Phát hiện cho agent/chủ sở hữu khác
- `mysql/openspec_change_repository.go` (rf-sol): GetByRequest không có `tenant_id`, Upsert ghi tenant `0000...`: rò rỉ chéo tenant trên MySQL; để trong danh sách cho phép của `TestTenantScope` kèm lý do, phải sửa cùng lúc khi viết lại.
- `mysql/project_engine_settings_repository.go Get` thiếu `tenant_id`: đã sửa.
- Payload outbox `orca.request.request.created` mang `title`; `EraseRequest` và lưu giữ không xoá nó khỏi `outbox_events.payload` (và sự kiện đã phát lên NATS/consumer). Cần quyết định bỏ `title` khỏi payload hoặc thêm bước xoá khoá.
- `usecase.DefaultRequestVisibility` (lọc ListBacklog) là bản giả "admin hoặc người báo cáo"; việc lọc theo dự án thành viên cho view request làm ở `ProjectScope`, chưa cho view task.
- Dịch vụ anh em gọi RPC nội bộ phải gắn `internalcaller.ClientInterceptor(SERVICE_INTERNAL_TOKEN)`; gateway dùng `GATEWAY_INTERNAL_TOKEN` (gateway gọi biến này là `REQUEST_INTERNAL_CALLER_TOKEN`). Không đặt thì request-service từ chối hết (cố ý).
- `deploy/dev` chưa có token, `REQUEST_ERASE_HMAC_KEY`, `PROJECT_SERVICE_ADDR`, `AUTH_SERVICE_ADDR` cho request-service.

### Chưa kiểm chứng / chưa làm
- 035-06: hành động `ai.budget.set`, `ai.egress.set`, `approval.approve` pre_deploy chưa gọi `AuditRecorder` (CR-034 và rf-appr).
- 035-08: xoá `ai_trace_blobs`/`ai_usage_ledger` (bảng chưa có); task-service không có RPC xoá nội dung.
- 035-09: nối `PromptRedactor` và `BuildAgentEnv` vào call site (chưa có AIGateway/relay), Trivy, bảng đối chiếu 2.11, `protovalidate`; govulncheck trong CI ở chế độ advisory, chưa chạy CI. Chưa chạy `ci/check-opa-bundle-in-images.sh` (build Docker nặng).
- Chưa đo hạn mức tốc độ, TTL cache 30 giây, 730/400/30 ngày. `FORCE RLS` với PgBouncer chưa thử. Bộ quét SQL chỉ thấy chuỗi hằng.
- Môi trường: `/dev/shm` (GOCACHE) từng đầy do nhiều agent; phiên này dùng GOCACHE riêng.

### Câu hỏi mở
1. Ma trận quyền (đặc biệt agent với lifecycle/triage, `member` không phân loại, nhóm các RPC clarification/decision/impact).
2. Webhook chạy với `actor_type=system` hay `user`?
3. Ai sở hữu RPC xoá nội dung ở task-service; gỡ `title` khỏi payload outbox.
4. Khi nào mcp-service đổi mặc định sang `SecretscanRedactor`.


### Hợp nhất vào feat/request-flow-backend (2026-10-08)
- Xung đột: `main.go`, `store_wiring.go`, `grpc/server.go`, `config.go` (giữ cả hai phía). `solution_prompt.go` bỏ `truncateRunes` trùng với `erase_request.go`; test fake `fakeSolutions` của export đổi tên `exportSolutionsFake`.
- Cột văn bản mới của rf/art/rf/roll phải khai báo trong `ErasableColumns`/`ExemptTextColumns`: thêm `requests.acceptance_criteria/type_fields`, `analysis_runs.feedback`, `request_revisions.snapshot`, `clarifications.cancel_reason`, `clarification_questions.*`, `decisions.*`, `decision_history.rationale`. `ErasableColumn.Parent` mới cho bảng con không có `request_id` (xoá qua bảng cha). `clarification_questions.reason` dùng marker vì có CHECK `<> ''`.
- `AnonymizeExpired` đọc lại cờ xoá sau khi khoá hàng (race hai replica làm đếm 52/40; giữ nguyên dữ liệu nhưng sai số đếm).
- Resolver thực thể `clarification`, `decision`, `artifact_ref` đăng ký ở `cmd/server/wire_entity_resolvers.go`; thực thể chưa đăng ký (impact, plan_task, phase, task...) vẫn bị từ chối với non-admin cho tới khi wave tương ứng đăng ký. Quyết định: người ngoài dự án không còn đọc được Clarification (art cũ cho phép, sec chặn).
- Test vị trí migration không còn đếm từ cuối (`MigrationScriptIndex`).
