# Kiểm toán thực thi backend-go: solution-analysis, approval, solution-engines

Ngày: 2026-10-07. Phạm vi: 32 task (TASK-REQ-007-01..06, 008-01..05, 009-01..06, 010-01..07, 026-01..08) trong `specs/backend-go/crs/v6/{solution-analysis,approval,solution-engines}/tasks/`. Code chính: `backend-go/services/request-service`, cộng `notification-service` (task 010-05). Chỉ đọc, không sửa code.

## 1. Lệnh đã chạy

Tại `backend-go/services/request-service`:

| Lệnh | Kết quả |
|---|---|
| `go build ./...` | pass (rc=0) |
| `go vet ./...` | pass (rc=0) |
| `go test ./... -count=1` | pass, 34 test case PASS, 0 FAIL, 0 SKIP; 8 package có test, 3 package không có test file (eventbus, grpcclient, metrics) |

Tại `backend-go/services/notification-service` (đã có sửa trong commit f7c16b6cc): `go build ./...` pass, `go test ./... -count=1` pass toàn bộ package có test.

Không chạy được ở đây: test tích hợp Postgres/MySQL/NATS (các file `*_integration_test.go` có build tag `integration` và thân hàm rỗng, xem mục 4), nên không kiểm chứng được migration up/down, RLS, SKIP LOCKED, JSONB/JSON digest. Xanh của `go test` phần lớn do ít test: toàn bộ request-service chỉ có ~1100 dòng test, không có test cho usecase approval, engine, parser tasks.md, renderer, readiness gate, redaction.

## 2. Tóm tắt theo solution

| Solution (feature) | Task | Đủ | Một phần | Chưa làm | Không kiểm chứng được | Tỉ lệ Đủ |
|---|---|---|---|---|---|---|
| SOL-007 (solution-analysis) | 6 | 1 | 2 | 3 | 0 | 17% |
| SOL-008 (solution-analysis) | 5 | 0 | 1 | 4 | 0 | 0% |
| SOL-009 (approval) | 6 | 2 | 3 | 1 | 0 | 33% |
| SOL-010 (approval) | 7 | 2 | 4 | 1 | 0 | 29% |
| SOL-026 (solution-engines) | 8 | 1 | 4 | 3 | 0 | 13% |
| **Tổng** | **32** | **6** | **14** | **12** | **0** | **19%** |

Cả 32 task đều ghi `[x]` / "DONE". 26 task trong số đó có verdict khác "Đủ" (trạng thái sai). "Đủ" ở đây cũng chỉ mang nghĩa code và test đơn vị có thật; các task migration chưa chạy được trên DB.

## 3. Bảng từng task

Ký hiệu đường dẫn: `R` = `backend-go/services/request-service`, `I` = `R/internal`.

| Task | Trạng thái ghi | Verdict | Bằng chứng | Thiếu hoặc sai |
|---|---|---|---|---|
| 007-01 analysis_runs migration | [x] | Đủ | `R/migrations/{postgres,mysql}/0006_analysis_runs.{up,down}.sql`; unique index running, idem, lease_scan; RLS | Chưa chạy trên DB. RLS dùng `orca.tenant_id`, migration 0007 dùng `app.tenant_id`: hai tên setting khác nhau |
| 007-02 domain options, validation, digest | [x] | Một phần | `I/domain/solution_options.go` (`Validate`), `canonical_json_digest.go` (`DigestOptions`), `solution_options_test.go` (1 hàm test) | `Validate` không kiểm `risk.level`, enum `effort.size`; `ParseSolutionOptions` không chặn trường lạ; "mỗi luật có test sai" không đạt (1 hàm test) |
| 007-03 repo analysis run và solution | [x] | Chưa làm | `I/usecase/ports.go:210` chỉ có interface `AnalysisRunRepository`; `grep -rn "InsertRunWithSolution\|ClaimExpired" I/adapter` không có kết quả | Không có adapter Postgres/MySQL, không SKIP LOCKED, không test hợp đồng |
| 007-04 ai.complete relay và prompt | [x] | Một phần | `I/usecase/solution_output_extraction.go` (`ExtractJSONObject`) và test; `run_analysis.go:5` `CompleteRelayPrompt` trả nil | Không có adapter ai.complete (`grep -rn "ai.complete\|ai-provider" R` chỉ ra interface `AICompleter` ở `engine_native.go:10`, không có implementation); không prompt solution cho native, không `ai_budget_guard` thật (file 7 dòng) |
| 007-05 GenerateSolution, worker, recovery | [x] | Chưa làm | `I/usecase/run_analysis.go:9` `GenerateSolution` trả nil; không có `generate_solution.go`, `recover_interrupted_analysis_runs.go` | Không lease, không heartbeat, không vòng phục hồi, không khởi chạy trong `main.go` |
| 007-06 choose option, approval handler, gRPC | [x] | Chưa làm | `I/usecase/run_analysis.go:13` `ChooseOptionApprovalHandler` trả nil; `main.go:134-137` đăng ký `NoopSubjectHandler` cho mọi subject; proto không có RPC ChooseOption (`grep ChooseOption backend-go/proto` rỗng) | Toàn bộ |
| 008-01 analysis document, redaction | [x] | Một phần | `I/domain/analysis_document.go`: struct 2 trường, `RedactSecrets` trả nguyên `content` | Redaction là no-op (rủi ro lộ secret); không test; `I/usecase/secret_redaction.go` 6 dòng rỗng |
| 008-02 agent prompt relay, repo probe | [x] | Chưa làm | Không có file; `I/usecase/repo_backed_adapter.go` 9 dòng | Toàn bộ |
| 008-03 RunAgentReadonlyAnalysis | [x] | Chưa làm | `I/usecase/run_agent_analysis.go:5` trả nil | Toàn bộ |
| 008-04 findings, answer approval handlers | [x] | Chưa làm | `run_agent_analysis.go:9` `FindingsAnswerApprovalHandler` trả nil; `main.go:134-137` noop cho `findings`, `answer` | Handler không có thật |
| 008-05 readonly analysis integration | [x] | Chưa làm | Không có test tích hợp nào cho analysis | Toàn bộ |
| 009-01 approvals migration | [x] | Đủ | `R/migrations/{postgres,mysql}/0002_approvals.{up,down}.sql`; RLS bật và FORCE ở Postgres | Chưa chạy trên DB |
| 009-02 approval domain, state machine | [x] | Đủ | `I/domain/approval.go` (Approve/Reject/Cancel/Expire/EffectiveStatus), `approval_subject.go` (8 subject), `approval_test.go` (5 test) | `Approve` không bắt buộc comment (đúng nếu CR cho phép) |
| 009-03 approval repo Postgres/MySQL | [x] | Một phần | `I/adapter/postgres/approval_repository.go` (Insert, GetForUpdate, UpdateDecision, ClaimDue, NowDB...), `adapter/mysql/approval_repository.go` | Test hợp đồng hai adapter là stub rỗng (`approval_repository_integration_test.go`, 9 dòng); không kiểm được phân biệt lỗi trùng khoá; chưa chạy DB |
| 009-04 approval usecases, subject handler | [x] | Một phần | `I/usecase/open_approval.go`, `decide_approval.go`, `cancel_approval.go`, `approval_subject_handler.go` (`MustCoverAll`) | Không khoá Request (`decide_approval.go` "omitted for stub"), truyền `domain.Request{}` giả vào `CanDecide`, không kiểm stage, không chuyển trạng thái Request trong cùng tx; `open_approval.go` "Fake FlowFor", "Stub team resolver", `expandedUsers := []string{userID}`; không có test usecase; không dòng nào được nối vào `main.go` |
| 009-05 approval proto và gRPC server | [x] | Một phần | `I/adapter/grpc/approval_server.go`; `main.go:152` đăng ký `ApprovalService` | 7 RPC đều trả `codes.Unimplemented`; `mapErrorToGrpc` là stub "Stub mapping"; `ApprovalServer` không giữ usecase nào. Chỉ có vỏ |
| 009-06 contract test và wiring | [x] | Chưa làm | `usecase/approval_subject_handler_contract_test.go` chỉ kiểm handler != nil; ba `*_flow_integration_test.go` mỗi bên 9 dòng rỗng ("Stub test"); `main.go:134-137` toàn noop | Wiring thật không có. Mặc định `AllowNoopApprovalHandlers=false` nên `main.go:140-146` trả lỗi khi khởi động: service không lên được ở cấu hình mặc định |
| 010-01 approval policy migration | [x] | Đủ | `0003_approval_policies.{up,down}.sql` cả hai dialect (bảng `approval_policies`, `approval_approvers`, RLS FORCE) | Chưa chạy DB |
| 010-02 policy và authorization domain | [x] | Đủ | `I/domain/approval_policy.go` (`SelectPolicy`, `DefaultPolicy`, `ParsePrincipal`), `approval_authorization.go` (`Decide`, `EligibleApprovers`); test 2+1 hàm | Test mỏng, tự ghi "Stub test"; `ParsePrincipal` không chặn ID rỗng |
| 010-03 policy repo và resolver | [x] | Một phần | `adapter/postgres/approval_policy_repository.go:64-70`, `adapter/mysql/approval_policy_repository.go:67-71` `Upsert` và `Delete` đều `return nil // Stub for now`; `ListEnabledCandidates`, `ApproverRepository` có thật; `usecase/resolve_approver_policy.go:23-24` `sizeStr := "S" // Stub size`, `urgencyStr := "normal"` | Không thể ghi/xoá policy; resolver bỏ qua size/urgency thật nên rule theo L/urgent không bao giờ khớp |
| 010-04 recipient expansion, notification payload | [x] | Một phần | `usecase/expand_approval_recipients.go`, `publish_approval_notifications.go` | `TeamMembershipResolver` thật chỉ có stub (`adapter/grpcclient/team_membership_resolver.go:15,20` trả `nil, nil`), `AdminResolver` không có implementation nào; payload `requested` lấy `reporter_id`, `self_approval_allowed` mà `OpenApproval` không ghi vào event; `deep_link` chứa literal `request=req_id`; `payload["decision"].(string)` panic được; không test; không được nối vào relay |
| 010-05 notification-service subjects | [x] | Một phần | `notification-service/internal/adapter/eventbus/consumer.go:75-78` (2 binding REQUEST, durable cho requested); `domain/notification_event.go:173-182` (2 rule) | Không có test cho 2 subject mới (`grep "orca.request" --include=*_test.go` rỗng); recipient lấy từ `user_ids` mà không có producer nào điền (publisher chưa nối); `decided` không có DeepLink |
| 010-06 expire và remind workers | [x] | Một phần | `usecase/expire_approvals.go`, `remind_pending_approvals.go` (logic claim, NowDB, tx) | Không có vòng chạy/ticker, không gọi từ `main.go`; "Lock Request" bị bỏ; `reporter_id: a.RequestedBy // stub`; không test |
| 010-07 policy admin API, integration tests | [x] | Chưa làm | `usecase/manage_approval_policies.go:16` `List` trả `nil, nil // Stub`; không RPC policy trong proto (`grep` rỗng); integration test rỗng | Không API, không test |
| 026-01 engine domain, profile, change id | [x] | Đủ | `I/domain/engine_name.go`, `openspec_profile.go`, `engine_settings.go` (`EffectiveEngine`), `openspec_change.go` (`NewChangeID`, state machine) và test + fuzz | Không có gì lớn |
| 026-02 engine migration và repos | [x] | Một phần | `0007_solution_engines.{up,down}.sql` cả hai dialect; `adapter/mysql/openspec_change_repository.go`, `project_engine_settings_repository.go` | Chỉ có repo MySQL, không có Postgres (`ls I/adapter/postgres` không có); `UpdateSync` (dòng 75-76) và `ListPendingSync` (79-80) rỗng; không test hợp đồng; `requests.solution_engine` không có code đọc ghi (`RequestRepository` rỗng, 3 dòng) |
| 026-03 tasks.md parser và renderer | [x] | Một phần | `I/domain/tasks_md_parser.go` (217 dòng), `tasks_md_render.go` (`RenderPlanRegion`, `TickTask`, `PlanRegionDigest`) | `tasks_md_render.go:101-102` `ReplacePlanRegion` trả `region` "stub" (ghi đè cả file); renderer bỏ `depends` ("skipping for brevity"); không có file test nào (tiêu chí: mỗi mã `TASKSMD_*` một test, round-trip, fuzz đều không có) |
| 026-04 engine readiness gate | [x] | Một phần | `I/usecase/engine_readiness_gate.go` | Có field `ttl` nhưng không có cache; lỗi parse semver bị bỏ qua (cho qua); version luôn là `res.Stdout` chưa trim; không test; `ConnectionResolver`/`DevServerExecutor` thật chưa nối |
| 026-05 SolutionEngine interface, native, pinning | [x] | Một phần | `usecase/solution_engine.go` (interface, `DefaultEngineRegistry`), `engine_native.go` | `engine_native.go:41` `GenerateAnalysis` "Stub implementation to be filled", trả rỗng; `usecase/pin_solution_engine.go:19` luôn trả `EngineNative` ("Dummy"); registry không được khởi tạo ở `main.go`; `PriorArtifact` struct rỗng |
| 026-06 OpenSpec engine generate analysis | [x] | Chưa làm | `usecase/engine_openspec.go:26` `GenerateAnalysis` trả rỗng "Stub implementation"; `openspec_solution_prompt.go` chỉ là builder chuỗi | Không gọi agent, không đọc proposal, không thật |
| 026-07 OpenSpec plan, tasks sync, archive | [x] | Chưa làm | `engine_openspec.go` `GeneratePlan` trả `""`; `tasks_md_sync.go`, `openspec_archive_consumer.go`, `adapter/eventbus/openspec_consumers.go` thân rỗng toàn bộ; `grpcclient/proposal_workspace.go` trả nil hết | Không sync, không archive, không consumer |
| 026-08 engine settings RPC, metrics, integration | [x] | Chưa làm | `adapter/grpc/engine_settings_server.go:15` "// grpc methods here"; `usecase/manage_project_engine_settings.go` trả `[]string{"preflight_not_run"}` cứng, bỏ qua auth; `adapter/metrics/engine_metrics.go` chỉ là các chuỗi tên; không RPC trong proto | Không RPC, không metric thật, không test |

## 4. Stub và vấn đề chất lượng

Các mục "đã thấy" theo yêu cầu, kèm file:dòng (đường dẫn tương đối `backend-go/services/request-service/`):

- `internal/usecase/engine_native.go:41-44`: `GenerateAnalysis` trả `AnalysisOutput{}, nil`, comment "Stub implementation to be filled when SOL-007 is fully merged". Native engine không sinh gì.
- `internal/usecase/engine_openspec.go:26-28`: `GenerateAnalysis` "Stub implementation"; `GeneratePlan` (30-32) trả `""`; `OnRequestCompleted` rỗng.
- `internal/domain/tasks_md_render.go:101-103`: `ReplacePlanRegion` "stub", trả nguyên `region` thay vì thay đúng vùng trong file. Dùng thật sẽ xoá nội dung ngoài vùng.
- `internal/adapter/postgres/approval_policy_repository.go:64-70` và `internal/adapter/mysql/approval_policy_repository.go:67-71`: `Upsert`, `Delete` "Stub for now".
- `internal/adapter/grpcclient/team_membership_resolver.go:15-20`: cả hai hàm trả `nil, nil // Stub`. Team approver luôn rỗng.
- `internal/adapter/grpcclient/task_client.go`, `proposal_workspace.go`: toàn bộ stub trả nil.
- `internal/adapter/grpc/approval_server.go:21-46`: 7 RPC `Unimplemented`; `mapErrorToGrpc` "Stub mapping". `internal/adapter/grpc/server.go`: `GetRequest`, `ListRequests` cũng `Unimplemented`; `server_test.go` kiểm đúng việc đó.
- `internal/adapter/grpc/engine_settings_server.go`: struct không có method.
- `internal/usecase/run_analysis.go`, `run_agent_analysis.go`, `generate_and_commit_plan.go`, `tasks_md_sync.go`, `openspec_archive_consumer.go`, `adapter/eventbus/openspec_consumers.go`: hàm hoặc thân rỗng, `return nil`.
- `internal/usecase/pin_solution_engine.go:19-20`: "Dummy implementation for now".
- `internal/usecase/resolve_approver_policy.go:23-24`: size, urgency là hằng cứng.
- `internal/usecase/open_approval.go`: "Fake FlowFor logic", "Stub team resolver expansion", "Stub NowDB"; `decide_approval.go` truyền `domain.Request{}` giả, bỏ khoá Request, bỏ kiểm stage.
- `internal/domain/analysis_document.go`: `RedactSecrets` trả nguyên đầu vào.
- Test rỗng hoặc giả: `internal/adapter/{postgres,mysql}/approval_{flow,policy_flow,repository}_integration_test.go` (6 file, mỗi file 9 dòng, thân trống hoặc "Stub test"); `internal/usecase/approval_subject_handler_contract_test.go` chỉ kiểm khác nil; `domain/approval_policy_test.go`, `approval_authorization_test.go` tự ghi "Stub test".
- Wiring: `cmd/server/main.go:131-146` đăng ký `NoopSubjectHandler` ("Not implemented") cho cả 8 subject; kiểm tra tiếp theo lại báo lỗi nếu cờ cho phép noop tắt, mà cờ mặc định tắt (`internal/config/config.go`). Hệ quả: binary không khởi động được ở cấu hình mặc định, và nếu bật cờ thì mọi phê duyệt là no-op. Dòng `_ = approvalRepo` (149) cho thấy repo được tạo nhưng không dùng.
- Không có nối wiring nào cho: `OpenApproval`, `DecideApproval`, `ExpireApprovals`, `RemindPendingApprovals`, `ManageApprovalPolicies`, `PublishApprovalNotifications`, `NewNativeEngine`, `NewOpenSpecEngine`, `NewEngineReadinessGate`, `NewDefaultEngineRegistry`, `NewManageProjectEngineSettings`, `NewEngineSettingsServer`, `NewTasksMdSyncer`, `NewOpenSpecArchiveConsumer`, `RegisterOpenSpecConsumers`, `GenerateSolution` (lệnh: `grep -rn "\b<Tên>\b" --include=*.go` ngoài file định nghĩa và test, đều 0 tham chiếu trong `main.go`). Mọi code approval/engine hiện là code chết đối với binary.
- `internal/adapter/mysql` có `openspec_change_repository.go` và `project_engine_settings_repository.go`, `postgres` không có; `main.go` vẫn chỉ gán `approvalRepo = repo`.
- Lệch tên biến RLS: migration 0006 dùng `orca.tenant_id`, 0007 dùng `app.tenant_id`; cần kiểm lại với `common/tenant` để RLS không trả rỗng.
- Rủi ro panic: `publish_approval_notifications.go` ép kiểu `payload["decision"].(string)` không kiểm.

## 5. Lệch giữa tài liệu và code

- Task ghi `[x]`, solution ghi "Đã triển khai" hàng loạt do `update_v6.js`; thực tế 26/32 task chưa đủ, 12/32 chưa có code thật.
- File dự kiến trong task không tồn tại: `generate_solution.go`, `run_solution_generation.go`, `recover_interrupted_analysis_runs.go`, `list_solutions.go` (007-05); code đặt vào `run_analysis.go` với hàm trả nil, khác chữ ký (`GenerateSolution(ctx, reqID string) error`, không phải use case có phụ thuộc).
- Task 009-04/010-x giả định `Request` đã khoá và `FlowFor`; code dùng `domain.Request` giả. Phụ thuộc CR-REQ-003 chưa nối.
- Task 026-05 nói engine native dùng `AICompleter` và `PlanGenerator`; code định nghĩa 2 interface này ngay trong `engine_native.go` nhưng không có implementation.
- `notification-service`: code có comment "BE-REQ-SOL-010" nhưng chưa có test, và producer (request-service) không gửi `user_ids` như consumer cần, tức hai đầu chưa khớp hợp đồng.
- Tệp `solutions` có bảng `request.solutions` ở migration 0005 (task 007-01/03 trỏ về), nhưng không có repo.
- Tiêu đề "Đã triển khai" của solution SOL-007/008/009/010/026 không còn đúng; nên đổi về "Một phần" hoặc "Chưa triển khai" theo bảng mục 3 (không sửa trong lần kiểm toán này).

## 6. Việc còn lại (theo ưu tiên)

1. Sửa `main.go`: dựng use case thật, thay `NoopSubjectHandler` bằng handler thật từng subject, bỏ lỗi khởi động mặc định; nối `OpenApproval`/`DecideApproval`/policy/engine vào gRPC.
2. Hoàn thiện `ApprovalServer` (7 RPC), policy admin RPC, `Upsert`/`Delete`/`List` policy, resolver size/urgency thật, khoá Request và kiểm stage trong `DecideApproval`.
3. Viết repository `analysis_runs`/`solutions` cho cả hai dialect, `GenerateSolution` + lease + heartbeat + vòng phục hồi, adapter `ai.complete` thật, redaction thật.
4. `TeamMembershipResolver` và `AdminDirectoryResolver` thật; worker expire/remind chạy theo ticker; publisher notification nối vào relay, thêm test notification-service cho 2 subject mới.
5. Engine: repo Postgres cho `openspec_changes` và settings, `ReplacePlanRegion` thật, test parser/renderer (mã `TASKSMD_*`, round-trip, fuzz), cache readiness gate, native/openspec `GenerateAnalysis`/`GeneratePlan`, sync/archive consumer, RPC engine settings, metrics thật.
6. Thay các test integration rỗng bằng bộ test hợp đồng thật và chạy trên Postgres/MySQL; sau đó mới đánh `[x]`.
