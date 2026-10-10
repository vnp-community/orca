# IMPACT-CODEGRAPH: phạm vi ảnh hưởng của nhánh `feat/e2e-acceptance-fixes` so với `main`

Thay cho quy trình `gitnexus_impact` (trước khi sửa) và `detect_changes` (trước khi commit) của `AGENTS.md` / `CLAUDE.md`, vì gitnexus không dùng được trong phiên này. Công cụ thay thế: `codegraph` 0.x (chỉ mục SQLite cục bộ, `codegraph sync` báo "Already up to date" trước khi chạy). Tài liệu này CHỈ ĐỌC mã; không sửa mã nguồn.

Phạm vi: `git diff main` trừ các đường dẫn của luồng khác (`docs/crs/v7/workflow-engine/`, `specs/backend-go/crs/v7/workflow-engine/`, `tests/backend|client|e2e/`, `tests/playwright.workflow-web.config.ts`, `package.json`, `tests/shared/orca_route_catalog.py`, `agent/tsconfig.tsbuildinfo`): 794 file (664 file mã/cấu hình trong `backend-go`, `frontend`, `agent`, `deploy`, `.github`, còn lại là tài liệu và test e2e). 7 commit từ merge-base `98e451354`.

## 1. Tóm tắt kết luận

- Không có mức CRITICAL. Có 3 điểm HIGH cần người duyệt: (H1) một test của agent hồi quy thật so với `main` và còn gọi `claude` CLI thật; (H3) hai test migration infra-fleet hỏng do migration `0040` mới; (H2) đổi ngữ nghĩa "token rỗng = tắt guard" thành "token rỗng = từ chối" ở mcp-service và `ResolveMcpPrincipal` của auth-service, chỉ `deploy/dev` được cấp token.
- 11 điểm MEDIUM (M1-M11) và 4 điểm LOW ở mục 4, kèm bằng chứng `file:dòng`.
- Test bị ảnh hưởng đã chạy: xem mục 6. Ngoài H1 và H3 (cả hai là test hỏng, không phải lỗi hành vi sản xuất), mọi lỗi test còn lại đều có sẵn trên `main` (đã đối chiếu bằng worktree).

## 2. Phương pháp và giới hạn của codegraph so với gitnexus

1. `codegraph impact <TênChínhXác> -d 2` cho từng symbol (struct/hàm), đọc danh sách ký hiệu bị ảnh hưởng theo file. Với tên chung (`Probe`, `Detach`, `Execute`, `McpPrincipalResolver`) codegraph gộp theo tên nên kết quả lẫn nhiễu (ví dụ `McpPrincipalResolver` ra 391 ký hiệu, `Probe` ra 155): với các tên đó phải đọc tay bằng `callers` và grep có mục tiêu (`grpcmw.NewClient`, `tenant.Detach(`, `internalcaller.`).
2. `codegraph affected` cho Go chỉ trả test ở package KHÁC import file đổi (ví dụ chỉ trả `request-service/e2e/*`), không trả test cùng package; kèm hàng nghìn test TS vô liên quan vì trùng tên. Vì vậy danh sách test để chạy là hợp của: (a) mọi package có file đổi (kể cả file test đổi), (b) test Go codegraph trả, (c) test frontend ở độ sâu 1 + toàn bộ test thư mục `components/request`, các hook `*equest*|*lan*|*larification*|*olution*|useTask*`, `runtime/request*`, `shared/request*|task*`.
3. Không có `detect_changes`/luồng thực thi (process) như gitnexus: "luồng bị ảnh hưởng" ở bảng dưới suy ra từ đồ thị gọi (caller trực tiếp) cộng đọc diff.
4. Không dò được lời gọi qua interface/gRPC giữa service (đồ thị dừng ở `*_grpc.pb.go`); cặp client/server nối bằng đọc tên RPC.
5. `codegraph sync` không sinh file mới ngoài `.codegraph/` (gitignored).

## 3. Bảng symbol → caller/luồng → rủi ro → test → kết quả

Cột "Caller trực tiếp (codegraph)" chỉ liệt kê caller sản xuất, không kể test. Rủi ro: theo số caller, độ lan và việc đổi ngữ nghĩa, không theo số dòng.

### 3.1 `backend-go/common` (lan sang mọi service)

| Symbol | Caller / luồng | Rủi ro | Test bao phủ | Kết quả |
|---|---|---|---|---|
| `grpcmw.NewClient`, `ClientIdentityInterceptor/StreamInterceptor/DialOptions`, `withForwardedIdentity` (`common/grpcmw/client_identity.go`) | 18 service dùng `grpcmw.NewClient` (grep: ai-provider 2, annotation 1, auth 2, automation 2, code-intel 1, git-gateway 1, infra-fleet 3, issue-status-sync 4, issue-tracking 1, mcp 3, notification 2, orchestration 1, project 8, request 1, scm 1, task 2, tenant 1, workflow 2). api-gateway cố ý dùng dial trần (gắn danh tính thủ công) | MEDIUM: đổi hành vi toàn hệ thống, từ "adapter nhớ gắn" sang "luôn chuyển tiếp tenant/user/role/actor/IP" (xem M1) | `client_identity_test.go`, `project-service/.../infra_fleet_identity_test.go`, CI `check-grpc-dials.sh` | PASS (unit + integration), `check-grpc-dials` ok |
| `eventbus.consumeUntilDone` (`eventbus.go:221-249`), `Subscribe`, `SubscribeEphemeral` | 20 điểm gọi: notification-service, api-gateway (mcp_tools, mcp_resources_prompts, mcp_tools_changed, wscompat workspace_events/task_activity/request_stream/agent, session_host), task-service, tenant-service, automation-service, request-service (5 consumer), issue-status-sync, auth-service (2 audit ingest), mcp-service hub | MEDIUM: nay dừng vòng kéo khi ctx hết và NAK tin đã nhận lúc ctx hủy; handler đang chạy vẫn có thể chạm kênh đã đóng (xem M7) | `consume_stop_test.go`, `push_fanin_sink_test.go` | PASS |
| `tenant.Detach` (`tenant/detach.go`) | `task-service/.../execute_task.go:439` (dispatchDirectAgentAsync), `workflow-service/.../execute.go:160` | MEDIUM: giữ role/actor type/user của RPC gốc cho tác vụ nền (xem M1) | `detach_test.go`, `execution_identity_test.go` (workflow). Task-service CHƯA có test khẳng định danh tính vào goroutine | PASS |
| `health.Probe` (`health/health.go`) | `request-service/cmd/server/healthcheck_command.go:18` (compose healthcheck `orca healthcheck`) | LOW | `probe_test.go`, `healthcheck_command_test.go` | PASS |
| `internalcaller.Guard/StreamGuard` | KHÔNG đổi trong nhánh (đã fail-closed từ trước); đổi ở NƠI DÙNG: auth, mcp, project | xem H2, M4 | `internalcaller_test.go` | PASS |

### 3.2 api-gateway

| Symbol | Caller / luồng | Rủi ro | Test | Kết quả |
|---|---|---|---|---|
| `registerRequestChannels` + kênh `clarification.*`, `clarification.request` (`registerClarificationAsk`) | `RegisterRealChannels`; impact 25 ký hiệu; gọi `RequestClarification` (chỉ caller là gateway) | MEDIUM: kênh mới, danh mục RPC đổi (xem M6) | `channels_request_clarification*_test.go`, `TestRequestRPCChannelParity`, `TestChannelInventoryMatchesGolden` (golden 2134 dòng) | PASS |
| `pushFanInSink` (`push_fanin_sink.go`) | 3 kênh: `request.subscribe`, `task.activity.subscribe`, `agent.statusChanged/rateLimited` (impact 19 ký hiệu) | LOW: các kênh `defer close(out)` còn lại đều chỉ có MỘT producer goroutine (`channels_push.go:58,97`, `channels_mcp_events.go:50`, ...), nên không dính lỗi gửi vào kênh đóng | `push_fanin_sink_test.go` | PASS |
| `task.list` bộ lọc `taskTypes/requestId(s)/parentId` + `toTaskView.RequestID` + `CreatorId` ở `task.create` | `registerTaskCRUDChannels` (16 ký hiệu), `toTaskView` (26) | LOW: thêm trường tùy chọn | `channels_*_test.go` | PASS |
| `requestErrorWith` + `retryAfterSuffix` | mọi kênh request | LOW: thêm đuôi ` {"retryAfterMs":N}` vào thông điệp lỗi; frontend đã đọc | `channels_request_validation_test.go` | PASS |
| `McpPrincipalResolver` (`authclient/mcp_principal_resolver.go`) | `validate_mcp_principal` (gateway) | HIGH (vận hành, H2): token rỗng giờ bị auth-service từ chối | `mcp_dial_token_test.go` | PASS |

### 3.3 request-service

| Symbol | Caller / luồng | Rủi ro | Test | Kết quả |
|---|---|---|---|---|
| `ProposeRequestClassification` (`propose_request_classification.go:163-175`) | `wireClassification`, `run`; issue-status-sync `main`. 15 ký hiệu | MEDIUM: nuốt `ErrApprovalNoEligibleApprover` bằng `errors.Is` trong transaction ngoài. An toàn vì lỗi này phát sinh sau các lệnh đọc (`open_approval.go:140`), không sau lệnh SQL lỗi, nên Postgres không hủy transaction | `propose_request_classification_test.go:245`, e2e | PASS |
| `GenerateSolution.start` (`generate_solution.go:138-142`) | `registerSolution`/`registerRequestChannels` (gateway), `server_solution.go`, `SolutionRegenerator`. 34 ký hiệu | LOW: bắt đầu "hệ thống" giờ hành động như người báo cáo | `generate_solution_test.go:324,365` | PASS |
| `AnalysisResultWriter.Persist`, `WithHotfixFixTasks`, `ensureFixTask`, `openFixTaskApproval` | `wirePlan.bindHotfixFixTask`, `wireSolution`, `RunSolutionGeneration.Execute`, `RunAgentReadonlyAnalysis.Execute`. 36 ký hiệu | MEDIUM: gọi task-service TRƯỚC khi mở transaction (có chủ ý); lỗi sau đó khác `REQUEST_SOLUTION_REQUEST_MOVED` để lại task mồ côi, lần chạy sau dọn (xem M5) | `analysis_result_writer_hotfix_test.go`, `hotfix_contract.go`, `run_solution_generation_test.go` | PASS |
| `EnsureHotfixFixTask` (`ensure_hotfix_fix_task.go`) | 1 caller sản xuất: `AnalysisResultWriter`; 13 ký hiệu | LOW: idempotent (tái dùng theo tiêu đề/mô tả, hủy bản cũ). Cần `user` trong ctx, thiếu thì `ErrRequestReporterRequired` | `ensure_hotfix_fix_task_test.go` (6 test) | PASS |
| `CommitPlan`, `GeneratePlan`, `RunPlanGeneration`, `GetPlanProposal`, `server_plan.go` | gateway `generatePlan`; frontend `request-plan-generation`; 16-19 ký hiệu mỗi symbol | MEDIUM (tính năng lớn, +3777 dòng): loại run mới `plan` cần migration `0093` chạy TRƯỚC khi chạy bản mới (xem M8) | `commit_plan_test.go`, `plan_fakes_test.go`, e2e | PASS |
| `StartExecution` + `EvaluateExecution.completePhases` (`evaluate_execution.go:240-262`) | status consumer, task outcome consumer; impact lẫn nhiễu (88/103) | MEDIUM: bỏ điều kiện `finishedAny`, nên MỖI lần `Run` có thể mở Approval Phase kế tiếp. Idempotent nhờ kiểm `GetLatest` (`:268-272`), nhưng hai `Run` đồng thời có thể cùng đi qua kiểm tra (đua; phụ thuộc chỉ mục duy nhất trong `OpenApproval`) | `start_execution_test.go:102,123` | PASS |
| `OpenApproval` | `completePhases`, `RequestTypeApprovalRecorder`, `AnalysisResultWriter`; 40 ký hiệu | LOW | `approval_usecase_test.go` (không có `open_approval_test.go` riêng) | PASS |
| `AIConnectionResolver.ResolveForProject` + `devServerRepoPath` | `RunAgentReadonlyAnalysis`, `GenerateSolution`; 12 ký hiệu | LOW: `RepoPath` lấy từ URL repo mặc định; URL không phải đường dẫn tuyệt đối thì phân tích chỉ-đọc báo `REQUEST_ANALYSIS_NO_REPO_PATH` | `ai_connection_resolver_test.go`, `solution_clients_test.go` | PASS |
| `AuthorizeRequestAction` + `observeOrphan` (`authorize_request_action.go:190-199`) | `RequestAccessInterceptor`, `StreamAccessInterceptor`, `SecurityChain`, `wireSecurity`, `registerEntityResolvers`; 33 ký hiệu, MỌI RPC nhắm vào một Request | MEDIUM (M3): thêm một lần tra project-service đồng bộ, hết hạn 2 giây, vào đường đi của mọi RPC | `request_access_orphan_test.go` | PASS |
| `RPC catalog: RequestClarification` (`rpc_catalog.go:112`) từ `GroupInternal` sang `onRequest(GroupTriage, ..., RateWrite)` | caller duy nhất: gateway `registerClarificationAsk`; test e2e | MEDIUM (M6): trước là RPC nội bộ không gắn Request, nay bị chặn theo vai trò trên Request và giới hạn tốc độ ghi | `security_chain_test.go:313`, `request_access_interceptor_test.go` (duyệt từng RPC theo vai trò) | PASS |
| `CreateRequest` + `ProjectExistence` (`create_request.go:99`) | gateway `request.create`; 79 ký hiệu (nhiễu tên) | MEDIUM (M4): thêm phụ thuộc cứng vào project-service khi có token | `create_request_test.go`, `project_membership_fake_integration_test.go`, contracttest | PASS |
| `ConfirmDecision`/`DecisionQueries.Get`, `ListSolutions` (lọc `RunKindPlan`) | gateway, frontend `useSolutions` | LOW: not-found chỉ khi `isNotFoundKind` | `not_found_contract.go`, `list_solutions_test.go` | PASS |
| `AdminDirectoryResolver`/`ListActiveTenantAdmins` client, `ApprovalNotificationStore`, `TeamMembershipResolver.refusedForGood` | outbox relay thông báo duyệt; 9-13 ký hiệu | MEDIUM (M9): lỗi không thể thử lại giờ ở lại outbox (không chặn) | `approval_notification_store_test.go`, `wire_approval_directory_test.go` | PASS |
| `RateLimitInterceptor` (đổi chữ ký, bỏ biến `RateLimitedTotal`) | 1 caller (`SecurityChain`); grep không còn ai dùng `RateLimitedTotal` | LOW | `security_chain_test.go` | PASS |
| `registerInternalTokenReadiness`, `registerPolicyBundleReadiness`, `runHealthcheck` | `main.go`; compose healthcheck | LOW | `readiness_internal_tokens_test.go`, `readiness_dependencies_test.go`, `healthcheck_command_test.go` | PASS |
| Sentinel not-found (`repository_not_found.go`, `uuid_guard.go`, adapter `notUUID`) | mọi repo của request-service | LOW | `not_found_contract.go` | PASS |

### 3.4 Service khác

| Symbol | Caller / luồng | Rủi ro | Test | Kết quả |
|---|---|---|---|---|
| auth-service `ListActiveTenantAdmins` + `directoryCallerGuard`; proto `AuthService` (+RPC, thêm thuần) | caller duy nhất: request-service `AdminDirectoryResolver` (compose truyền `AUTH_DIRECTORY_CALLER_TOKEN` cho cả hai) | LOW: thêm RPC, tương thích dây; quét tối đa `activeAdminScanMaxPages` trang rồi gắn `truncated` | `list_active_tenant_admins_test.go` | PASS |
| project-service `ResolveProjectAccess` + `internalCallerGuard`; proto `ProjectService` (+RPC, thêm thuần) | caller duy nhất: request-service `ProjectAccessLookup.Lookup` | MEDIUM (M4) | `resolve_project_access_test.go` | PASS |
| task-service `CreateTask` gán `CreatorID` từ ctx (`server_create_task.go`), `taskRepoError`, `ExecuteTask` | gateway `task.create`, request-service `CreateFixTask`, HTTP route; 60 ký hiệu | MEDIUM: client không còn chỉ định được `creator_id` khi có người gọi (đúng về bảo mật); lỗi hạ tầng không còn thành 404 | `server_create_task_creator_test.go` | PASS |
| workflow-service `started_by` (migration `0013`), `RecoverExecutions`, `serverresolver.Resolve` | `Execute`, khởi động lại dịch vụ; 13 ký hiệu | MEDIUM (M1): đổi ngữ cảnh nền; cột mới nullable, hàng cũ khôi phục không có người dùng thì cảnh báo | `recover_executions_test.go`, `execution_identity_test.go` | PASS |
| infra-fleet `PollFleetHealth.PollDevServer` + `OnAttached` | `agentwsserver.handleConnection` (mỗi agent kết nối), `main.go` | MEDIUM (M2) | `poll_fleet_health_test.go`, `server_test.go` | PASS |
| infra-fleet `HealthDegradedSubject` đổi thành `orca.infrafleet.dev_server.health_degraded` + migration `0040` (cập nhật outbox chưa phát) | outbox relay; grep: KHÔNG có consumer nào của subject cũ | LOW | `health_publisher_test.go`, `outbox_subjects_test.go` | PASS |
| infra-fleet `MapAgentExecError` (nhánh `ai.complete`), `aiCompleteTimeout=150s`, `session.shutdown` (close 1001) | request-service `mapAIRelayError`; agent `agent-connection-direct.ts:13` coi mã khác 1000 là kết nối lại | LOW: mã `AI_COMPLETE_FAILED/PROVIDER_UNAVAILABLE/TIMEOUT` của infra-fleet không có trong `aiCompleteCodes` của request-service nên đi tiếp không đổi (có chủ ý) | `agent_rpc_error_mapping_ai_test.go`, `session_close_code_test.go`, `exec_timeouts_test.go` | PASS |
| infra-fleet/task/project/automation/issue-tracking/tenant/mcp/... sentinel not-found (`devServerLookupError`, `taskRepoError`, `credentialResolveError`, `ErrAutomationNotFound`, ...) | 32 file (devServerLookupError), 20 file (taskRepoError), 23 file (credentialResolveError) | MEDIUM (M10) | các `*_not_found*_test.go` | PASS |
| mcp-service `internalCallerServerOptions` | mọi RPC gateway-only, registry nội bộ, `StreamEvents`; client duy nhất: api-gateway | HIGH vận hành (H2) | `mcp_governance_wiring_test.go`, `mcp_dial_token_test.go` | PASS |
| auth-service `internalCallerGuards` (OAuth nội bộ, `ResolveMcpPrincipal`) | api-gateway + mcp-service | HIGH vận hành (H2) | `internal_caller_guards_test.go` | PASS |
| `tenant-service`, `automation-service`, `orchestration-service`, `usage-service`, `credential-broker-service`, `annotation-service`, `notification-service`, `code-intel-service`, `git-gateway-service`, `scm-integration-service`, `issue-status-sync` | chủ yếu đổi dial sang `grpcmw.NewClient` và map lỗi UUID sai dạng (SQLSTATE 22P02) thành not-found | LOW | unit + integration của từng package | xem mục 6.1 |

### 3.5 Frontend và agent

| Symbol | Caller / luồng | Rủi ro | Test | Kết quả |
|---|---|---|---|---|
| `callRequestRpc` (+ `request-rate-limit.ts`: thử lại tự động một lần, cooldown theo lớp) | 120 ký hiệu: RequestDetailPane, CreateRequestDialog, ApprovalInboxTab, GraphPanel, ... | MEDIUM (M11): thử lại tự động các ghi "an toàn lặp" (`SOLUTION_GENERATE`, `CLARIFICATION_ANSWER`, `CLARIFICATION_REQUEST`, `APPROVAL_APPROVE/REJECT`) khi bị `REQUEST_RATE_LIMITED`; phân lớp client (`AI/WRITE/READ`) tách khỏi catalog server, lệch thì chỉ sai phạm vi cooldown | `request-rpc-client.test.ts`, `request-rate-limit.test.ts` | PASS |
| `request-errors.ts` (`classifyRequestRpcError`, bảng `CODE_TO_KIND` mở rộng), `task-errors.ts` | 75 ký hiệu (`requestErrorMessage`) | LOW | `request-errors.test.ts`, `task-errors.test.ts`, `request-rpc-contract.test.ts` | PASS |
| `useSolutions`, `SolutionPanel` | `RequestAnalysisTab`, `SolutionPanel` (5/2 ký hiệu) | LOW | `useSolutions.test.ts`, `SolutionPanel*.test.tsx` | PASS |
| `usePlanTree`, `RequestPlanTab`, `GraphPanel` | 9 ký hiệu | LOW | `usePlanTree.test.ts`, `RequestPlanTab.test.tsx` | PASS |
| `ClarificationPanel`, `useClarifications` | `RequestDetailPane` (3/6 ký hiệu) | LOW | `ClarificationPanel.test.tsx`, `useClarifications.test.ts` | PASS |
| `ai-complete-handler.ts` (agent: thêm nguồn CLI, đổi thông điệp lỗi `No API key found` sang `No AI credential`) | `agent-rpc-dispatch` AI; infra-fleet `ai.complete` | HIGH (H1) | `ai-complete-handler.test.ts` LỖI | FAIL (hồi quy, xem H1) |
| `fs.checkPath` mở rộng (`fs-agent-extensions.ts`), `agent-capability-report.ts` + golden | `check-agent-capability-golden.sh`, gateway | LOW | `fs-check-path.test.ts`, `agent-capability-*.test.ts` | PASS |

Đã chạy 69 truy vấn `codegraph impact` (1 truy vấn, `GenerateAndCommitPlan`, không trả ký hiệu nào); số trong cột caller là "n ký hiệu bị ảnh hưởng" mà codegraph trả.

## 4. Điểm rủi ro cần người duyệt chú ý

### HIGH

**H1. Hồi quy test agent: `ai-complete-handler.test.ts` hỏng trên nhánh, qua trên `main`; test còn gọi `claude` CLI thật.**
- Bằng chứng: `agent/src/relay/__tests__/ai-complete-handler.test.ts:14-28` kỳ vọng `rejects.toThrow('No API key found')` và `fail.fields.err === 'no API key for model'`. Mã mới (`agent/src/relay/ai-complete-handler.ts`) thử nguồn "CLI session" trước khi báo thiếu khóa; thông điệp thành `No AI credential for model ...` và `span.fail('no AI source for model')`. Trên máy có `claude` trong PATH, test chạy CLI thật (log `[TRACE] agent:aiComplete ... claude CLI failed: There's an issue with the selected model (claude-haiku)`, 3,6 giây, có thể tốn hạn mức). Chạy cùng file trên worktree `main`: PASS.
- Đề xuất: test phải đặt `ORCA_AI_CLI_SESSION=off` (kill switch có sẵn ở `ai-complete-cli-session.ts:160`) và cập nhật hai câu kỳ vọng; mọi test của agent không được phép chạm CLI thật.

**H2. "Token rỗng = tắt guard" đã thành "token rỗng = từ chối" (fail closed) ở mcp-service và `ResolveMcpPrincipal`.**
- Bằng chứng: trước: `services/mcp-service/cmd/server/main.go` chỉ bật `internalcaller.Guard` khi `govCfg.InternalCallerToken != ""` (nhánh else chỉ cảnh báo); `auth-service/cmd/server/main.go` chỉ bật guard `ResolveMcpPrincipal` khi `McpPrincipalCallerToken != ""` ("rollout-safe"). Nay: `mcp-service/cmd/server/internal_caller_guards.go:15-22` và `auth-service/cmd/server/internal_caller_guards.go:25-31` luôn gắn guard; `internalcaller.go:61` (`hasToken`) trả false khi token rỗng. Tương tự `project-service` (`ResolveProjectAccess`) và `auth` (`ListActiveTenantAdmins`) là guard mới.
- Ảnh hưởng: mọi môi trường thiếu `MCP_INTERNAL_CALLER_TOKEN`, `OAUTH_INTERNAL_CALLER_TOKEN`, `AUTH_MCP_PRINCIPAL_CALLER_TOKEN`, `PROJECT_INTERNAL_CALLER_TOKEN`, `AUTH_DIRECTORY_CALLER_TOKEN` sẽ mất MCP (gateway-only governance, đăng nhập OAuth MCP, `StreamEvents`) và phân quyền theo dự án. Trong repo chỉ `deploy/dev/docker-compose.yml:253-258,284,435-436,462-469,647-648` truyền đủ; `deploy/prod/docker-compose.yml` không có service backend-go nào, không có manifest khác. `deploy/dev/.env` hiện để trống các token MCP (README mục 5.7). Các khai báo `ensure-mcp-env.sh` / `ensure-request-env.sh` tạo token.
- Chỉ request-service có kiểm tra sẵn sàng (`readiness_internal_tokens.go`, `/readyz` không sẵn sàng khi thiếu token); mcp-service, auth-service, project-service chỉ ghi log ERROR/WARN lúc khởi động, không có `readyz` chặn.
- Đề xuất: nêu rõ trong ghi chú phát hành; thêm kiểm tra sẵn sàng tương tự cho 3 service còn lại hoặc kiểm tra `ensure-*-env.sh` trong CI.

**H3. Migration infra-fleet `0040` làm hỏng `TestMigration0039_UpDownUp` (Postgres và MySQL).**
- Bằng chứng: test chạy `migrate ... down 1` và kỳ vọng bảng của `0039` bị xóa (`migration_0039_capability_profiles_test.go:48-52`, log `expected table to be dropped by down`); từ khi có `0040_health_degraded_outbox_subject` (down là no-op `SELECT 1`), `down 1` chỉ lùi `0040`. Trên `main` (0039 là migration cuối) test qua.
- Đề xuất: sửa test để lùi về đúng phiên bản (`goto`/`migrate down` theo số) thay vì `down 1`; cùng lỗi thiết kế đang làm hỏng `0018`, `0038` trên `main`.

### MEDIUM

- **M1. Danh tính được chuyển tiếp ở mọi cuộc gọi gRPC giữa service.** `ClientIdentityInterceptor` (`client_identity.go:43-69`) thêm tenant/user/role/IP/actor (chỉ khi khóa chưa có, không bịa tenant). Hệ quả đã thấy: (a) `workflow-service/internal/adapter/opachecker/checker.go:40-52` `IsAdmin` gọi `auth.ListUsers`, vốn đòi người gọi là admin (`list_users.go:34`): trước đây ngữ cảnh chỉ có tenant nên luôn từ chối (luôn `false`), nay có người dùng nên admin thật được nhận ra, người không phải admin vẫn `false` (fail-closed). Hành vi được sửa nhưng cần biết. (b) `tenant.Detach` ở `execute_task.go:439` và `execute.go:160` giữ role và actor type của RPC gốc cho công việc nền: một lần chạy do agent khởi tạo vẫn là agent ở hạ nguồn (đúng thiết kế), nhưng RPC hạ nguồn nào từ chối actor `agent` giờ sẽ từ chối cả luồng nền (trước đây luồng nền mang tư cách người dùng). Task-service chưa có test khẳng định danh tính vào goroutine.
- **M2. Mỗi lần agent kết nối làm một lượt `ListAllDevServers` toàn bảng.** `poll_fleet_health.go:147-153` (`PollDevServer`) gọi `ListAllDevServers` (mọi tenant) rồi lọc theo id, kích hoạt từ `agentwsserver/server.go:236-238` cho từng kết nối. Khi infra-fleet khởi động lại và N agent nối lại cùng lúc, thành N lượt quét toàn bảng (có khóa phân tán nhưng truy vấn vẫn chạy). Đề xuất: tra theo id.
- **M3. `observeOrphan` thêm tối đa 2 giây đồng bộ vào mọi RPC nhắm một Request** (`authorize_request_action.go:50,190-199`; lỗi không được cache, `project_access_lookup.go` chỉ cache kết quả thành công). Khi project-service chậm hoặc từ chối token, mọi RPC chậm thêm đến 2 giây. Chỉ ảnh hưởng chỉ số, không đổi quyết định.
- **M4. Phụ thuộc cứng mới vào project-service.** `create_request.go:99-105`: khi có `PROJECT_INTERNAL_CALLER_TOKEN`, tra `ResolveProjectAccess`; lỗi (kể cả `PermissionDenied INTERNAL_CALLER_REQUIRED` khi hai bên lệch token, không phải `Unimplemented`) trả `ErrProjectRoleUnavailable` và chặn tạo Request; `RoleOf` cũng lỗi (`project_role_resolver.go:67-70`) nên mọi kiểm tra quyền theo dự án hỏng. Không có kiểm tra sẵn sàng cho việc token lệch. Khi token trống thì tắt tra cứu (giữ hành vi cũ, ghi cảnh báo `wire_project_access.go:29-34`).
- **M5. Task sửa hotfix tạo ngoài transaction.** `analysis_result_writer.go:90` tạo/tái dùng task rồi mới mở transaction; chỉ lỗi `REQUEST_SOLUTION_REQUEST_MOVED` mới gọi `Discard` (`:180`). Lỗi khác sau đó để task `open` mồ côi cho đến lần `Ensure` kế (hủy bản cũ theo `stale`). `Ensure` idempotent, nên rủi ro là nhiễu danh sách task chứ không nhân đôi việc.
- **M6. `RequestClarification` đổi từ RPC nội bộ sang theo-Request.** `rpc_catalog.go:112`. Gateway là caller duy nhất; nay áp vai trò (người báo cáo/admin, `security_chain_test.go:313`) và giới hạn ghi. Mọi client nội bộ khác (không có trong repo) sẽ bị từ chối theo vai trò.
- **M7. Handler vẫn có thể chạy sau khi vòng kéo dừng.** `eventbus.go:221-249`: `cc.Stop()` không đợi callback đang chạy (chính comment ghi). Ba kênh gateway đã dùng `pushFanInSink`; handler của mcp-service hub, notification, task-service, tenant-service không đóng kênh nên an toàn theo đọc mã. Ngoài ra README mục 5.1d: lỗi vĩnh viễn vẫn NAK không trễ (hot-loop), chưa sửa.
- **M8. Thứ tự triển khai migration.** `request-service/migrations/*/0093_analysis_runs_plan_kind`: bản mới ghi `kind='plan'`, nên migration phải chạy TRƯỚC khi chạy bản mới; `down` XÓA mọi run `plan` (mất dữ liệu). `infra-fleet 0040`: không đảo ngược được (ghi trong file). `workflow 0013`: thêm cột nullable (an toàn).
- **M9. Hàng outbox thông báo duyệt bị giữ.** `approval_notification_store.go`: hàng lỗi không thể thử lại (cấu hình thư mục/token sai) ở lại outbox và không chặn các sự kiện khác; một lô toàn hàng như vậy che các hàng vượt giới hạn lô (chính comment ghi), chỉ có log ERROR, chưa có chỉ số (README mục 5.9).
- **M10. Chuyển lỗi "không có" sang sentinel, thất bại hạ tầng thành `Internal`.** Mọi repo phải bọc đúng sentinel bằng `%w`. Đã kiểm adapter automation (`postgres/repository.go:148,215-221,232-238,588-593`; `mysql/repository.go:154,240,262,659`), task-service (`postgres/repository.go:229-249,335-392`, `mysql/repository.go:239-254,344-387`), infra-fleet (`domain/repository_not_found_errors.go`), project-service. Không còn so sánh con trỏ/`==` với sentinel của miền (`grep` `== domain.Err…` trong mã sản xuất: 0 kết quả; chỉ còn `== pgx.ErrNoRows`/`== sql.ErrNoRows` trên lỗi chưa bọc ở `infra-fleet/.../agent_token_repository.go:91,107`, `ephemeral_vm_ssh_target_repository.go:71`, `auth-service/.../service_token_repository.go:30`, `request-service/.../mysql/retention.go:47`, không đổi hành vi). Còn hở đã ghi: scm-integration và git-gateway (README mục 5.5). `credentialResolveError` (`issue-tracking-service/.../credential_resolve_error.go`) gộp `PermissionDenied/Unauthenticated` của broker thành `ISSUETRACKING_NOT_CONNECTED`, che sai cấu hình token.
- **M11. Thử lại tự động ở frontend** (`request-rpc-client.ts`, `request-rate-limit.ts`): chỉ khi lỗi là `REQUEST_RATE_LIMITED` (cuộc gọi chưa chạy), chờ tối đa 5 giây; tập "an toàn lặp" do client tự khai (`IDEMPOTENT_WRITES`), lệch với server nếu server đổi khóa idempotency.

### LOW

- L1. `deploy/dev/scripts/build-local.sh` nay cần `rsync` (trước đây `cp -r`); build một service không còn build frontend trừ khi `BUILD_FRONTEND=1`.
- L2. `ci/agent-methods-called-known-gaps.txt` ghi 24 phương thức backend gọi agent mà agent chưa có (`fs.scanNestedRepos`, `git.merge`, `git.fetchRef`, ...): lỗi có thật nhưng đã có sẵn, kiểm tra CI chỉ chặn lỗ hổng MỚI.
- L3. `.github/workflows/*` dùng `go-version: "1.25"`, `go.work` khai báo `go 1.26.0` (module vẫn `go 1.25.0`): chưa kiểm trên runner GitHub.
- L4. Gói mã sinh `backend-go/proto/gen`: proto chỉ THÊM (RPC `ListActiveTenantAdmins`, `ResolveProjectAccess`, trường `value_type = 12`), tương thích dây; README mục 6 ghi `buf breaking` xanh.

## 5. Chưa có test bao phủ (hoặc bao phủ gián tiếp)

- `tenant.Detach` ở `task-service` (`execute_task.go:439`): không có test khẳng định tenant/user/role/actor tới goroutine nền (workflow có `execution_identity_test.go`).
- Đua hai `EvaluateExecution.Run` đồng thời mở Approval Phase (không có test đồng thời; phụ thuộc chỉ mục duy nhất của repo).
- Lệch token giữa request-service và project-service/auth-service (đường `PermissionDenied INTERNAL_CALLER_REQUIRED` trong `Lookup`): chỉ có nhánh `Unimplemented` được test.
- `readyz` không báo thiếu token ở mcp-service, auth-service, project-service (không có đoạn mã để test).
- Hiệu năng `PollDevServer` khi nhiều agent nối lại (không có test tải).
- Chỉ số `orca_request_orphan_total` và đếm hàng outbox bị giữ (README mục 5.9).
- Hồi quy H1 có test nhưng test sai; chưa có test nào thể hiện nguồn CLI bị tắt trong môi trường kiểm thử mặc định.

## 6. Test đã chạy và kết quả

Môi trường: `DOCKER_HOST=unix:///Users/datbeohbbh/.colima/default/docker.sock`, `TESTCONTAINERS_RYUK_DISABLED=true`, `PATH` thêm `scratchpad/bin` (buf, migrate). Không chạy docker compose, redeploy, Playwright, `run_acceptance`.

### 6.1 Go (backend-go), 99 package có file đổi hoặc codegraph trả về

| Lượt | Lệnh | Kết quả |
|---|---|---|
| `go build ./...` | 22 module trong `go.work` (quality-service không thuộc `go.work`, có sẵn, không đổi) | 22/22 build được |
| Unit (không tag) | `go test -count=1` trên 99 package | 62 package `ok`, 36 không có test, 0 test lỗi. Dòng `FAIL .../request-service/e2e [setup failed]` chỉ vì cả thư mục đó có tag `e2e` (không có file nào biên dịch khi không tag): chạy riêng ở dưới |
| Integration (`-tags=integration`, testcontainers Postgres/MySQL thật) | cùng 99 package | 76 package `ok`, 16 không có test, 6 package có test lỗi (23 lần lỗi, 21 tên test): xem bảng dưới |
| e2e (`-tags=e2e`) | `request-service ./e2e/...` | `ok` (46 giây), `e2e/stubs` ok |
| CI tĩnh | `make check-agent-methods-called check-grpc-dials check-agent-capability-golden` | cả 3 ok |

Test Go lỗi trong lượt integration và đối chiếu với `main` (worktree tạm `git worktree add --detach <scratchpad>/basewt main`, đã gỡ sau khi xong):

| Test lỗi | Trên `main` | Kết luận |
|---|---|---|
| `credential-broker TestMigration0004_CategoryCheck_UpDownUp` | lỗi giống hệt | có sẵn |
| `notification-service TestRepository_GetPublicKey_NoActiveKeyReturnsDomainError` | lỗi | có sẵn |
| `orchestration-service TestCoordinatorRun_AutonomouslyAdvancesToCompletion` (`migrate: first .: file does not exist`) | lỗi | có sẵn |
| `infra-fleet` postgres: `TestMigration0018_DownDropsTable`, `TestMigration0038_DownUpRoundTrip`, `TestRepository_ResolveConnection_FoundAndNotFound`; mysql: `TestMigration0038_DownUpRoundTrip` | lỗi | có sẵn |
| `project-service` Postgres: 14 test worktree/ImportNested | cùng 14 tên | có sẵn (khớp ghi nhận ở README mục 6) |
| **`infra-fleet` postgres + mysql `TestMigration0039_UpDownUp`** | **qua** | **MỚI do nhánh (H3)** |

Lỗi có sẵn đã biết khác (`common/tracing`, ssh/socket infra-fleet, git-gateway localgit, `-race` BrowserScreencast) nằm ngoài các package đã chạy, nên không xuất hiện ở đây.

### 6.2 Frontend (vitest, `frontend/config/vitest.config.ts`)

- Lượt 1 (độ sâu 1 + toàn bộ test request/plan/clarification/solution/task): 144 file qua, 1299 test qua. Một "unhandled error": `TaskSourceBadge.test.tsx` cần gói `jsdom` chưa cài (môi trường, không liên quan nhánh).
- Lượt 2 (độ sâu 2 của codegraph, 742 file khác): 732 file qua, 10 file lỗi, 35 test lỗi. Chạy đúng 10 file đó trên `main`: cùng 35 test lỗi (cùng tên): `feature-interactions`, `useIpcEvents`, `Sidebar`, `selectors`, `pty-transport`, `pty-connection`, `task-page-default-repo-selection`, `ProjectSettings`, `landing-preflight-dismissal`, `automation-project-groups`. Tất cả có sẵn, 0 lỗi mới.

### 6.3 Agent (vitest, `agent/`)

- Chạy 16 file (`ai-complete-*`, `fs-check-path`, `agent-capability-*`, `agent-rpc-dispatch*`, `fs-agent-extensions`): 186 test qua, 3 bỏ qua, 2 lỗi:
  - `agent-rpc-dispatch.test.ts > notification.send ... delivered:false`: lỗi cả trên `main` (máy macOS có trình thông báo), có sẵn.
  - `ai-complete-handler.test.ts > ... no API key for model`: qua trên `main`, lỗi trên nhánh: **MỚI (H1)**.
- Chạy cả bộ `agent` bị kết thúc giữa chừng (mã 144), nên không có số liệu toàn bộ.


## 7. Checklist `detect_changes` tương đương

| Symbol đổi | Luồng bị ảnh hưởng | Test đã chạy | Kết quả |
|---|---|---|---|
| `grpcmw.NewClient` + `ClientIdentityInterceptor` | mọi cuộc gọi giữa 18 service, danh tính xuyên service | `common/grpcmw`, `project-service/.../infra_fleet_identity_test.go`, `check-grpc-dials.sh`, `go build ./...` 22 module | PASS |
| `eventbus.consumeUntilDone` | 20 điểm subscribe, 3 kênh push gateway | `common/eventbus` (unit + integration), `push_fanin_sink_test.go`, wscompat | PASS |
| `tenant.Detach` | execute task/workflow nền | `common/tenant`, workflow `execution_identity_test.go` | PASS (task-service chưa có test chuyên biệt) |
| `internalcaller` guard nơi dùng (auth/mcp/project) | MCP, OAuth, phân quyền dự án, thông báo duyệt | gateway `mcp_*_test.go`, auth `internal_caller_guards_test.go`/`directory_caller_guard_test.go`, request `wire_*_test.go` | PASS; rủi ro vận hành H2 |
| `completePhases`/`StartExecution` | thực thi change_request có cổng Phase | `start_execution_test.go`, e2e | PASS |
| `AnalysisResultWriter.Persist` + hotfix | phân tích hotfix → task + Approval pre_deploy | `analysis_result_writer_hotfix_test.go`, `hotfix_contract.go` | PASS |
| `RequestClarification` catalog + kênh `clarification.*` | hỏi lại từ Solution | `security_chain_test.go`, `channels_request_clarification*_test.go`, golden kênh | PASS |
| `CreateRequest`/`ProjectAccessLookup` | tạo Request, phân quyền | `create_request_test.go`, `project_membership_fake_integration_test.go`, contracttest | PASS |
| Sentinel not-found (automation, task, infra-fleet, project, request, ...) | RPC tra theo id, 404 so với 500 | `*_not_found*_test.go`, contracttest, integration Postgres/MySQL | PASS (xem mục 6.1) |
| Agent `ai.complete` CLI session | `ai.complete` qua infra-fleet | `ai-complete-*.test.ts` | FAIL 1 (H1) |
| infra-fleet migration `0040` | outbox `dev_server.health_degraded` | `TestMigration0039_UpDownUp` | FAIL 2 (H3), test hỏng, không phải hành vi |
| Frontend request/plan/clarification/solution | màn Request | vitest 886 file | PASS; 35 test lỗi có sẵn trên `main` |

## 8. Hạn chế của báo cáo

- Không có `detect_changes`/luồng thực thi của gitnexus; "luồng" suy từ đồ thị gọi và diff.
- `codegraph affected` yếu với Go (bỏ test cùng package) nên phải bù bằng danh sách package; nhiều kết quả TS trùng tên vô nghĩa đã bị loại.
- Không chạy toàn bộ bộ test `agent` (chạy cả bộ bị kết thúc với mã 144 giữa chừng, nghi một test giết tiến trình); chỉ chạy các file liên quan.
- Không kiểm trên runner GitHub, không dựng stack, không chạy Playwright.

## Xử lý các điểm rủi ro CAO (người điều phối, 2026-10-10)

- **H1 (đã sửa):** `agent/src/relay/__tests__/ai-complete-handler.test.ts` đặt `ORCA_AI_CLI_SESSION=off` (để test không spawn `claude` thật trên máy dev có `claude`) và cập nhật kỳ vọng sang thông điệp/lý do mới (`No AI credential`, `no AI source for model`). 4/4 test xanh.
- **H3 (đã sửa):** `TestMigration0039_UpDownUp` (Postgres và MySQL của infra-fleet) lùi/tiến 2 bước vì 0040 là migration mới nhất và down của nó là no-op. Cả hai dialect xanh.
- **H2 (chấp nhận, đã ghi):** "token rỗng = từ chối" ở auth/mcp/project là thay đổi có chủ đích (fail-closed, TASK-E2E-020). Chỉ `deploy/dev/docker-compose.yml` truyền token; `ensure-request-env.sh` và `ensure-mcp-env.sh` sinh token thiếu; request-service có `/readyz` báo thiếu token, các service còn lại ghi log ERROR lúc khởi động. Việc thêm `/readyz` cho auth/mcp/project ghi vào danh sách còn lại của đợt.
- Workflow CI dùng `go-version: "1.25"` còn `go.work` khai `go 1.26.0`: cùng giá trị với các workflow backend-go có sẵn (không do đợt này); `actions/setup-go` tải toolchain theo `go.work`. Chưa kiểm trên runner.
