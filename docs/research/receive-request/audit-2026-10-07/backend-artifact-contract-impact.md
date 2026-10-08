# Kiểm toán thực thi: request-artifact-model, execution-contract, impact-risk (backend-go)

Ngày: 2026-10-07. Phạm vi: 32 task (TASK-REQ-027-01..08, 028-01..08, 029-01..08, 030-01..08) đọc từ `specs/backend-go/crs/v6/<feature>/{solutions,tasks}/`. Code: `backend-go/services/request-service`, `backend-go/services/task-service`. Chỉ đọc, không sửa code hay tài liệu.

## 1. Lệnh đã chạy và kết quả thật

| Service | Lệnh | Kết quả |
|---|---|---|
| request-service | `go build ./... && go vet ./... && go test ./... -count=1` | exit 0. 8 package `ok`, các package còn lại `no test files`. Không có test nào cho code của 4 feature này. |
| task-service | cùng lệnh | **exit 1, build lỗi**. `internal/usecase/list_execution_states.go:6` import `github.com/stablyai/orca-go/common/errx` không tồn tại (`backend-go/common` chỉ có `apperrors`, không có `errx`). `internal/domain/task_type.go:14` khai báo lại `ErrInvalidTaskType` đã có ở `internal/domain/task.go:67`. `go test ./...`: mọi package usecase, domain, grpc, mysql, postgres, eventbus, opaclient, cmd/server đều `[setup failed]` hoặc `[build failed]`. Số test chạy được: 0. |

Test tích hợp Postgres/MySQL/NATS không chạy ở đây (không có DB), và cũng không liên quan vì các bảng mới chưa có migration.

## 2. Tóm tắt theo solution

| Solution / nhóm task | Đủ | Một phần | Chưa làm | Không kiểm chứng được | Tỉ lệ Đủ |
|---|---|---|---|---|---|
| 027 artifact schema, ontology, task specs (8) | 0 | 1 | 7 | 0 | 0% |
| 028 clarification, decision, awaiting_information (8) | 0 | 1 | 7 | 0 | 0% |
| 029 execution-contract (8) | 0 | 0 | 8 | 0 | 0% |
| 030 impact-risk (8) | 0 | 0 | 8 | 0 | 0% |
| **Tổng (32)** | **0** | **2** | **30** | **0** | **0%** |

Cả 32 task đang ghi `[x] DONE`. Trạng thái sai: 32/32.

## 3. Bảng từng task

Ghi chú chung: tất cả 32 task có `Status: [x] DONE` (hoặc mọi checkbox `[x]`); 028-01 và 028-02 không có dòng Status nhưng mọi checkbox là `[x]`. Đường dẫn bên dưới tính từ `backend-go/services/`.

| Task | Trạng thái file | Verdict | Bằng chứng | Thiếu hoặc sai |
|---|---|---|---|---|
| 027-01 migration artifact | [x] | Chưa làm | `request-service/migrations/{postgres,mysql}` dừng ở `0007_solution_engines`; `grep -rn "artifact"` không có bảng mới | Không có migration nào cho artifact, revision, schema |
| 027-02 task_specs ở task-service | [x] | Chưa làm | `task-service/internal/domain/task_spec.go` (9 dòng, struct), `internal/usecase/manage_task_specs.go` (3 hàm `return nil`). Migration dừng ở `0016_task_request_id`; `grep task_specs` trong migrations: không có | Không có bảng, repository, RPC; usecase là stub; không có test |
| 027-03 schema registry, canonical JSON, provenance | [x] | Một phần | `request-service/internal/domain/canonical_json_digest.go` (sort key, depth 32, sha256) có test `canonical_json_digest_test.go`; `domain/provenance.go` (7 dòng, struct rỗng) | Canonical hóa chỉ cho `DigestOptions` của solution options, không phải registry. Không có JSON Schema thật (xem mục 4). Không phát hiện khoá trùng (comment tự nhận). Provenance chỉ là struct |
| 027-04 markdown/YAML projection | [x] | Chưa làm | `usecase/markdown_projection.go` (`RenderArtifact` trả `""`, `ParseProjection` trả `nil, nil`) | Stub. Có `domain/tasks_md_parser.go`/`tasks_md_render.go` nhưng là phần tasks.md cũ, không phải projection YAML |
| 027-05 validate nội dung, revisions | [x] | Chưa làm | `usecase/validate_request_content.go` (`return nil`); `domain/request_revision.go` (struct) | Không validate gì, không lưu revision |
| 027-06 artifact ID, relation, semantic validation | [x] | Chưa làm | `grep -rn "relation\|semantic" request-service/internal` không thấy code liên quan artifact | Không có |
| 027-07 nối solution, plan, phase, approval | [x] | Chưa làm | `usecase/start_phase.go` (`return nil`), `generate_and_commit_plan.go` (11 dòng stub) | Không nối gì |
| 027-08 proto, gRPC, tích hợp | [x] | Chưa làm | `proto/orca/request/v1/` chỉ có `approval.proto`, `request.proto`, `request_backlog.proto`; grep `TaskSpec\|clarif\|nonce` trong `proto/orca/{request,task}`: không có | Không có message hay RPC artifact |
| 028-01 migration clarifications, decisions | [x] | Chưa làm | Migration dừng ở 0007; không có bảng clarifications/decisions | Không có |
| 028-02 domain và máy trạng thái awaiting_information | [x] | Một phần | `domain/clarification.go` (struct + 3 hằng trạng thái), `domain/decision.go` (struct 3 trường); `domain/request_status.go` có 11 trạng thái, không có `awaiting_information` | Không có máy trạng thái, flow registry, ma trận chuyển, test ma trận. `grep -rIl awaiting_information backend-go`: không file nào. Không có logic readiness |
| 028-03 repository clarification, decision | [x] | Chưa làm | `adapter/postgres`, `adapter/mysql` không có file clarification/decision | Không có |
| 028-04 readiness, waive | [x] | Chưa làm | `usecase/manage_clarifications.go` stub | Không có |
| 028-05 answer, cancel, hooks | [x] | Chưa làm | `ManageClarifications.AnswerClarification/CancelClarification` là `return nil`, struct rỗng | Không đổi dữ liệu, không hook |
| 028-06 resume consumer, expiry, reminder | [x] | Chưa làm | `remind_pending_approvals.go`, `expire_approvals.go` là của approval, không dính clarification | Không có |
| 028-07 decision record, confirm, approval gate | [x] | Chưa làm | `usecase/manage_decisions.go` (2 hàm `return nil`) | Không có |
| 028-08 proto, gRPC clarification/decision | [x] | Chưa làm | Không có trong `proto/orca/request/v1` | Không có |
| 029-01 task_execution_records migration và repo | [x] | Chưa làm | `task-service/migrations/*` dừng ở 0016; `domain/execution_record.go` (struct 4 trường); grep `task_execution_records`: không có | Không có bảng, repo |
| 029-02 result parser, proto, list records | [x] | Chưa làm | `usecase/execution_result_parser.go` (`return nil, nil`); grep `result_nonce` toàn `backend-go`: không có | Không có parser, không có nonce trong proto, không có RPC list |
| 029-03 contract executor Engine 1, failure events | [x] | Chưa làm | `usecase/contract_executor.go` (`ExecuteContract` `return nil`), `usecase/emit_execution_events.go` (`return nil`). grep `ExecuteWithContract`: không có. `execute_task.go` không nhắc contract | Không có `ExecuteWithContract`; không nối vào Engine 1 |
| 029-04 migration execution contract (request-service) | [x] | Chưa làm | Migration request-service dừng ở 0007 | Không có |
| 029-05 TaskSpec v2, ExecutionPacket | [x] | Chưa làm | `request-service/internal/domain/task_spec_v2.go` (struct 3 trường). grep `ExecutionPacket`: không có | Không có packet, không có render xác định |
| 029-06 ReadinessGate, agent relay, RPC | [x] | Chưa làm | `usecase/engine_readiness_gate.go` chạy thật `openspec --version` qua `DevServerExecutor.Exec` (dòng 52-58), nhưng đó là preflight OpenSpec engine, `NewOpenSpecEngine` (`engine_openspec.go:14`) dùng nó, không phải ReadinessGate của task execution | Không có `agent.exec` của readiness gate cho task, không có RPC, không nối vào AdvanceExecution |
| 029-07 verify execution, phân loại lỗi | [x] | Chưa làm | `usecase/verify_execution.go`: `VerifyExecution` luôn `return true, nil` | Hằng cứng "luôn pass"; không phân loại lỗi |
| 029-08 nối AdvanceExecution, flag, e2e | [x] | Chưa làm | grep `AdvanceExecution` trong `backend-go`: không có. `usecase/feature_flag.go`: `IsFeatureEnabled` luôn `true`. `usecase/transition_request.go`, `flow_gate_interceptor.go` là `return nil` | Không có AdvanceExecution, không có flag thật, không có e2e |
| 030-01 migration impact, risk | [x] | Chưa làm | Migration dừng ở 0007; grep `impact_`/`risk_` bảng: không có | Không có |
| 030-02 risk scoring domain RP1 | [x] | Chưa làm | `domain/risk_score.go:9` `CalculateRiskScore()` không tham số, luôn `Score: 50` | Không có bảng ngưỡng, không có luật cứng của CR-030 (xem mục 4). Không có test |
| 030-03 repository impact, risk | [x] | Chưa làm | Không có adapter nào | Không có |
| 030-04 scanner và GitNexus parser | [x] | Chưa làm | `usecase/gitnexus_parser.go` (`ParseGraph` `return nil`); `domain/graph_types.go` (`NormalizeSymbolRefKey`, `NormalizeRepoPath` trả nguyên input). grep `buf breaking`/scanner buf trong request-service: không có | Không có scanner GitNexus, không có scanner buf, parser là stub |
| 030-05 impact collector, trigger, actual | [x] | Chưa làm | `usecase/impact_collector.go` (`CollectImpacts` `return nil`), `collector_orchestration.go` (`return nil`) | Không thu thập gì |
| 030-06 risk gate, acceptance, drift approval | [x] | Chưa làm | `usecase/risk_gate.go:9` `EvaluateRiskGate` = `score.Score < 80` | Một ngưỡng cứng duy nhất, không acceptance, không drift approval |
| 030-07 impact read API, graph compare, narrator | [x] | Chưa làm | Không có RPC, không có file | Không có |
| 030-08 risk policy shadow/enforce, calibration, wiring | [x] | Chưa làm | `cmd/server/main.go` không tham chiếu các usecase trên (grep) | Không có policy, shadow, calibration |

## 4. Stub và vấn đề chất lượng

Trả lời trực tiếp các câu hỏi kiểm tra:

- **JSON Schema thật hay chỉ khai báo**: không có JSON Schema ở request-service hay task-service. `grep -rIl "jsonschema\|JSONSchema"` chỉ ra `services/api-gateway` (schema của MCP tool, không liên quan). Không có thư viện validate schema trong `go.mod` của request-service.
- **Clarification, Decision, `awaiting_information` trong máy trạng thái**: không. `domain/request_status.go:5-15` không có trạng thái này; không có flow registry hay ma trận chuyển nào (`grep -rIl "FlowRegistry\|TransitionMatrix"`: rỗng); `usecase/transition_request.go` là `return nil`. Clarification/Decision chỉ là struct.
- **ReadinessGate chạy `agent.exec` thật, nối AdvanceExecution**: không. Chỉ có `EngineReadinessGate` của OpenSpec (chạy `openspec --version` qua `DevServerExecutor`, `engine_readiness_gate.go:52`), thuộc luồng khác. `AdvanceExecution` không tồn tại.
- **ExecutionPacket render xác định**: không tồn tại.
- **Scanner GitNexus/buf thật**: không. `gitnexus_parser.go` stub. (code-intel-service có xử lý GitNexus riêng, ngoài phạm vi, không được request-service dùng cho impact.)
- **Điểm rủi ro có bảng ngưỡng và luật cứng CR-030**: không. `risk_score.go:9` trả hằng 50; `risk_gate.go:9` so với 80.

Danh sách stub (đều `return nil`/`return true`/struct rỗng), trong `request-service/internal`:
- `usecase/manage_clarifications.go:7-17`, `usecase/manage_decisions.go:7-13`, `usecase/markdown_projection.go:5-11`, `usecase/validate_request_content.go:5`, `usecase/impact_collector.go:5`, `usecase/collector_orchestration.go:5`, `usecase/gitnexus_parser.go:7`, `usecase/verify_execution.go:5` (luôn true), `usecase/feature_flag.go:5` (luôn true), `usecase/risk_gate.go:9` (ngưỡng cứng), `usecase/transition_request.go`, `usecase/flow_gate_interceptor.go`, `usecase/start_phase.go`, `usecase/report_task_outcome.go`.
- `domain/risk_score.go:9` (hằng 50), `domain/graph_types.go:9,13` (hàm identity), `domain/provenance.go`, `domain/task_spec_v2.go`, `domain/request_revision.go`, `domain/request_check.go`, `domain/snapshot.go`, `domain/decision.go`: chỉ struct.
- Cùng kiểu, ngoài phạm vi nhưng cùng commit: nhiều file usecase 7-11 dòng khác (`run_analysis.go`, `model_router.go`, `rate_limit.go`, `rls_tenant_isolation.go`, `webhook_replay.go`, `human_gate_policy.go`, ...). Nên kiểm toán riêng.

Trong `task-service/internal`:
- `usecase/manage_task_specs.go:7-17`, `usecase/contract_executor.go:7`, `usecase/execution_result_parser.go:5`, `usecase/emit_execution_events.go:5`, `usecase/agent_exec.go:5`: stub. `domain/task_spec.go`, `domain/execution_record.go`: struct.
- Build hỏng: `usecase/list_execution_states.go:6` (và test cùng thư mục) import `common/errx` không có; `domain/task_type.go:14` trùng khai báo với `domain/task.go:67`. Làm hỏng toàn bộ test của task-service, kể cả code cũ trước đây chạy được.
- Không có migration, không có thay đổi proto cho bất kỳ bảng hoặc RPC nào của 4 feature (migration request-service dừng 0007, task-service dừng 0016, nhưng 0016 là `task_request_id`, thuộc việc khác).
- Test: request-service không có test nào cho Clarification, Decision, risk, impact, readiness. Test pass ở request-service là test cũ của approval/solution options.

## 5. Lệch giữa tài liệu và code

- Task ghi `[x] DONE` và solution đổi tiêu đề "Đã triển khai" nhưng code không có tương ứng; trạng thái trong file không phản ánh thực tế.
- Tên có trong code nhưng khác vai trò: `EngineReadinessGate` (OpenSpec preflight) bị dễ nhầm với `ReadinessGate` của 029-06.
- `canonical_json_digest.go` là đóng góp thật nhưng thuộc solution options (task cũ), chỉ gần với 027-03.
- Comment code tự thừa nhận không cấm khoá JSON trùng (`canonical_json_digest.go` phần đầu file), trái với yêu cầu canonical hóa chặt.

## 6. Việc còn lại (ưu tiên)

1. Sửa build task-service: thay `common/errx` bằng `common/apperrors` (hoặc thêm gói), bỏ khai báo trùng `ErrInvalidTaskType`; chạy lại test để có mốc.
2. Đặt lại trạng thái 32 task và các solution về chưa làm (trừ phần 027-03, 028-02 làm dở).
3. Migration cho 4 feature (request-service 0008+, task-service 0017+) kèm proto (`result_nonce`, artifact, clarification, decision, impact).
4. Máy trạng thái: thêm `awaiting_information`, flow registry, ma trận chuyển và test ma trận (nền cho 028, 029-08).
5. Thay stub bằng logic: JSON Schema thật, repository, ContractExecutor/ExecuteWithContract, ReadinessGate dùng `agent.exec`, ExecutionPacket xác định.
6. Impact-risk: bảng ngưỡng và luật cứng CR-030, scanner GitNexus/buf, collector; xóa hằng 50 và 80.
7. Loại bỏ stub `return true`/`IsFeatureEnabled` luôn true trước khi bật bất kỳ flag nào.
