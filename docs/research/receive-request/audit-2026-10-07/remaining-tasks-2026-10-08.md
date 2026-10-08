# Task còn lại (backend), 2026-10-08

Tổng hợp từ các worktree đang làm việc (kể cả chưa hợp nhất). Backend: **139/199 DONE, còn 60**.

| Feature | DONE/tổng |
|---|---|
| agent-capabilities | 3/6 |
| ai-governance | 0/8 |
| approval | 13/13 |
| backlog-views | 6/6 |
| context-sources | 1/8 |
| execution-contract | 3/8 |
| gateway-and-mcp | 12/13 |
| impact-risk | 0/8 |
| plan-phase-task | 19/28 |
| request-artifact-model | 11/16 |
| request-lifecycle | 28/28 |
| request-quality-rollout | 12/16 |
| request-service-foundation | 12/13 |
| security-compliance | 7/9 |
| solution-analysis | 11/11 |
| solution-engines | 1/8 |

## agent-capabilities

| Task | Nội dung | Tiến độ / lý do còn lại |
|---|---|---|
| 033-04 | `request-service` đọc hồ sơ năng lực (`DevServerCapabilityReader`) và bảng chọn đường theo | chưa làm |
| 033-05 | Client Go của `agent.execPrompt` mới: `accessMode`, `workspaceKind`, `reportChanges`, `res | chưa làm |
| 033-06 | Test hợp đồng với agent (golden JSON) và kịch bản suy giảm agent cũ/mới | (2026-10-07) Phần `infra-fleet-service` đã làm và kiểm chứng; phần `request-service` để đợt R2 (cần 033-04, 033-05). - Xong: golden `internal/adapter/devserveragent/testdata/agent_capabilities_v1.golden.json` (bản sao nguyên văn c |

## ai-governance

| Task | Nội dung | Tiến độ / lý do còn lại |
|---|---|---|
| 034-01 | Migration `ai_governance` và repository (ledger, ngân sách, bộ đếm, chính sách bước, quyết | chưa làm |
| 034-02 | Domain AI (bước, ngân sách, ledger, chính sách bước) và `BudgetGuard` | chưa làm |
| 034-03 | `PromptRegistry`, phiên bản prompt bất biến và `provenance` | chưa làm |
| 034-04 | `EgressGuard`, `ModelRouter` và `AIGateway` (trình tự, ledger, relay mở rộng) | chưa làm |
| 034-05 | `AiBudgetAdminService`, cài đặt AI theo tenant và thông báo ngân sách | chưa làm |
| 034-06 | `GroundingChecker` kiểm đường dẫn và lệnh do AI đề xuất | chưa làm |
| 034-07 | `HumanGatePolicy` và chế độ chạy bóng (`ai_gate_decisions`) | chưa làm |
| 034-08 | Eval harness (xác định + `eval_live`), baseline và cổng CI | chưa làm |

## context-sources

| Task | Nội dung | Tiến độ / lý do còn lại |
|---|---|---|
| 031-01 | Migration `context_sources`, `context_packs`, `evidence` và repository hai dialect | chưa làm |
| 031-02 | Domain thuần (nguồn, hợp đồng adapter, xếp hạng, cắt, Evidence) và bộ che PII | chưa làm |
| 031-03 | Adapter nguồn nội bộ đọc repo qua Relay (quy ước, ADR, hợp đồng, schema, CI, OPA, git, Cod | chưa làm |
| 031-04 | Adapter nguồn dữ liệu Orca (`request_origin`, `history`, `ownership`, `dev_server_profile` | chưa làm |
| 031-05 | Use case `BuildContextPack`, quản trị nguồn và RPC `Preview`, sự kiện | chưa làm |
| 031-07 | `request-service`: client `mcp-service` và adapter nguồn `transport=mcp` | chưa làm |
| 031-08 | `api-gateway`: tài nguyên `orca://request|solution|plan|evidence|impact|context` và tool ` | chưa làm |

## execution-contract

| Task | Nội dung | Tiến độ / lý do còn lại |
|---|---|---|
| 029-04 | Migration `execution_contract` ở `request-service` và repository `execution_packets`, `tas | chưa làm |
| 029-05 | `TaskSpecV2` (kiểm hợp lệ, digest chuẩn tắc), `ScopeMatcher` và `RenderExecutionPacket` | chưa làm |
| 029-06 | `ReadinessGate` ba tầng, cổng `AgentRelay` và RPC `CheckReadiness`, `GetReadinessReport`,  | chưa làm |
| 029-07 | `VerifyExecution` (chạy lại Check, kiểm phạm vi, quét bí mật) và `ClassifyFailure` | chưa làm |
| 029-08 | Nối `ReadinessGate`, packet, `VerifyExecution` vào `AdvanceExecution` và `ReportTaskOutcom | chưa làm |

## gateway-and-mcp

| Task | Nội dung | Tiến độ / lý do còn lại |
|---|---|---|
| 017-05 | Kịch bản e2e MCP cho Request và tài liệu hướng dẫn | Chưa làm: script `tests/mcp/check_mcp_request_flow.py` và `docs/guides/mcp` nằm ngoài phạm vi lần triển khai này (chỉ api-gateway, mcp-service, specs) và cần stack dev thật để chạy. Phần tool, `ToolOrigin`, hạn mức đã xong và có t |

## impact-risk

| Task | Nội dung | Tiến độ / lý do còn lại |
|---|---|---|
| 030-01 | Migration `impact_risk` hai dialect (`impact_assessments`, `impact_tool_runs`, `risk_accep | chưa làm |
| 030-02 | Domain điểm rủi ro `rp/1`: chín chiều, `Score`, luật cứng, digest, so lệch kế hoạch | chưa làm |
| 030-03 | Repository hai dialect cho `impact_assessments`, `impact_tool_runs`, `risk_acceptances`, ` | chưa làm |
| 030-04 | Bộ quét `migrationscan`, `contractscan`, `gitnexusparse`, `AreaResolver` và fixture từ mẫu | chưa làm |
| 030-05 | `ImpactCollector` (công cụ qua `AgentRelay`), worker lease, consumer kích hoạt và `AssessA | chưa làm |
| 030-06 | `RiskGate`, `AcceptRisk`, `OverrideRiskGate`, drift Approval và `PlanPreconditions` theo m | chưa làm |
| 030-07 | RPC đọc, `GetImpactGraph` (4 lens), `CompareImpact`, `GetPlanRiskHeatmap`, `ImpactNarrator | chưa làm |
| 030-08 | `RiskPolicy` admin, chế độ `shadow` và `enforce`, công cụ hiệu chỉnh ngoại tuyến, wiring,  | chưa làm |

## plan-phase-task

| Task | Nội dung | Tiến độ / lý do còn lại |
|---|---|---|
| 012-03 | Domain `request-service`: `plan_shape`, nhãn, `PlanProposal`, `ValidateProposal` | chưa làm |
| 012-04 | Proto `request_plan.proto`, adapter `PlanGenerator` (relay `ai.complete`) và `TaskPlanWrit | chưa làm |
| 012-05 | Use case `GeneratePlan` (PROPOSE) và `CommitPlan` (COMMIT) kèm RPC | chưa làm |
| 012-06 | `SubjectHandler` cho `plan` và `task_list`, đường `single_task` (hotfix) | chưa làm |
| 012-07 | Test tích hợp và e2e luồng sinh Plan (hai dialect, hai service) | chưa làm |
| 014-03 | Interface `TypePolicy`, registry, `noopPolicy` và nối hook vào `GeneratePlan`, `AdvanceExe | Đã làm: interface `TypePolicy`, `PolicyRegistry` (`NewPolicyRegistry`, `PolicyFor`, noop cho 6 loại), hook `PreExecutionGate`, `CompletionChecks`, `OnCompleted` trong `AdvanceExecution`/`EvaluateExecution` (có test, kể cả hồi quy  |
| 014-04 | `SubjectHandler` `pre_deploy`, chính sách `hotfix` (cổng) và `security` | Đã làm: `PreDeployArtifacts` (hotfix/security/ops_request) gắn vào `TransitionSubjectHandler` qua `approvalSubjectArtifacts` (service từ chối khởi động nếu thiếu handler `pre_deploy`), `hotfixPolicy`, `securityPolicy` (có test, gồ |
| 014-05 | Chính sách `performance` (baseline, đo lại) và `refactor` (test cũ vẫn xanh) | Đã làm: `performancePolicy`, `refactorPolicy`, `ImprovementPercent`, kiểm hoàn tất trong `EvaluateExecution` (có test hàm thuần và luồng qua DB). Còn thiếu: lời gọi `PlanPreconditions` từ `GeneratePlan` (CR-REQ-012 chưa có). Lệch  |
| 014-06 | Chính sách `ops_request`: runbook, rollback, cổng `pre_deploy` từng bước, ghi kết quả | Đã làm: `CheckRunbook`, `opsRequestPolicy` (`PreExecutionGate` mở Approval một lần, `CompletionChecks`, `FailureHint` nêu task rollback), test `AdvanceExecution` cổng, lý do backlog. Còn thiếu: lời gọi `PlanPreconditions` từ `Gene |

## request-artifact-model

| Task | Nội dung | Tiến độ / lý do còn lại |
|---|---|---|
| 027-07 | Nối vào Solution, Plan, Phase và Approval: schema `options`, provenance, bảng phủ, khoá sp | Phần `task-service` đã làm (2026-10-08, trong 027-02): `RunInTxWithSpecs` ở cả hai adapter (có test rollback chung task và spec trên Postgres và MySQL thật), RPC `LockTaskSpecs` idempotent sẵn cho consumer `approval.decided`. Còn  |
| 027-08 | Proto `artifact.proto`, gRPC server, quyền đọc và kiểm thử tích hợp hai dialect | (rf/art, 2026-10-08) Đã làm và kiểm chứng (`go test ./internal/adapter/grpc`; `go test -tags integration ./cmd/server -run ArtifactAndClarificationFlow`): - 7 RPC thật (`EditRequestContent`, `ListRequestRevisions`, `GetRequestRevi |
| 028-06 | Consumer kích hoạt lại, vòng hết hạn, nhắc và thông báo (`notification-service`) | Một phần (đã kiểm chứng 2026-10-07: `cd backend-go/services/notification-service && go test ./... ; go test -tags integration ./internal/adapter/eventbus/...`): - [x] `notification-service`: hai binding `REQUEST` (`orca.request.cl |
| 028-07 | `RecordDecision`, `ConfirmDecision` và chặn duyệt Solution, Plan khi Decision chưa `effect | (rf/art, 2026-10-08) Đã làm và kiểm chứng (`go test ./internal/usecase -run 'RecordDecision|ConfirmDecision|ApprovalGates|DecisionQueries'`, `go test -tags integration ./internal/adapter/postgres ./internal/adapter/mysql -run Clar |
| 028-08 | Proto `clarification.proto`, `decision.proto`, gRPC server, `ListPendingClarificationsForU | (rf/art, 2026-10-08) Đã làm và kiểm chứng (`go test ./internal/adapter/grpc`, `go test -tags integration ./cmd/server -run ArtifactAndClarificationFlow`): - 11 RPC thật ở `adapter/grpc/server_clarification.go` (+ `clarification_ma |

## request-quality-rollout

| Task | Nội dung | Tiến độ / lý do còn lại |
|---|---|---|
| 025-03 | Khung e2e T1 (testcontainers, fake cổng AI, `task-service` giả) và kịch bản E01 | (2026-10-08) Đã làm: khung e2e chạy BINARY THẬT (`go build ../cmd/server`) trên DB thật (Postgres với role NOSUPERUSER NOBYPASSRLS, hoặc MySQL 8.0), NATS thật, outbox relay và consumer phân loại thật; chỉ biên gRPC phía sau được s |
| 025-04 | Kịch bản e2e E02 đến E19 (các luồng, chéo, quyền) và test ma trận loại | (2026-10-08) Đã làm và xanh trên cả hai dialect: E02..E12 (mỗi loại trong 11 loại tạo, phân loại bằng stub agent, xác nhận loại, trạng thái kế tiếp lấy từ `domain.NextStatus`), E13 (trả về backlog giai đoạn analysis, mở lại, phân  |
| 025-07 | T2 e2e trên stack dev (Python), stub dev server agent và workflow `request-e2e.yml` | (2026-10-08) Đã làm: stub agent là biên relay của infra-fleet (`e2e/stubs` + binary `e2e/cmd/agent-stub`, quyết định ghi ở IMPLEMENTATION-NOTES; có unit test, và đã được e2e T1 dùng chung code); `backend-go/ci/request-e2e/docker-c |
| 025-08 | Tài liệu `docs/guides/request/`, runbook và diễn tập rollback | (2026-10-08) Đã làm: 7 tài liệu trong `docs/guides/request/` (chỉ nói điều đã có, chỗ chưa có ghi rõ), cập nhật `backend-go/README.md`, `backend-go/services/request-service/README.md`, `docs/guides/jira/jira-orca-mapping.md`; liên |

## request-service-foundation

| Task | Nội dung | Tiến độ / lý do còn lại |
|---|---|---|
| 001-06 | Deploy dev (compose, migrate, build-local), Dockerfile, workflow CI hai dialect | Đã làm: `.github/workflows/backend-go-request-service.yml` (ma trận dialect, build, vet kể cả tag integration, gofmt, test đơn vị, test tích hợp, buf lint chỉ trên hai file request vì `approval.proto` còn nợ lint, buf breaking khi |

## security-compliance

| Task | Nội dung | Tiến độ / lý do còn lại |
|---|---|---|
| 035-02 | `x-orca-actor-type` (grpcmw, tenant, gateway) và token nội bộ gateway → `request-service` | (2026-10-07) Đã làm và kiểm chứng (`cd backend-go/common && go test ./grpcmw/ ./tenant/`; `cd backend-go/services/api-gateway && go test ./internal/adapter/grpc/ ./internal/adapter/mcpserver/... ./cmd/server/`): - [x] `grpcmw.Meta |
| 035-09 | Áp dụng `secretscan` (cổng vào, prompt, đầu ra), danh sách `env` cho agent, supply chain v | Đã làm: `SecretIngressGuard` (nối vào `CreateRequest.CreateWithinTx`, nên manual/webhook/MCP/child đều qua; text quá cửa sổ quét bị từ chối `REQUEST_PAYLOAD_TOO_LARGE` vì `Truncated`), cờ `contains_secret_suspected` ghi cùng giao  |

## solution-engines

| Task | Nội dung | Tiến độ / lý do còn lại |
|---|---|---|
| 026-02 | Migration `NNNN_solution_engines` và repository `project_engine_settings`, `openspec_chang | chưa làm |
| 026-03 | Parser và renderer vùng `plan` của `tasks.md` | chưa làm |
| 026-04 | Cổng điều kiện `EngineReadinessGate` và adapter `DevServerExecutor` | chưa làm |
| 026-05 | Giao diện `SolutionEngine`, `nativeEngine`, ghim engine và `engine_override` | chưa làm |
| 026-06 | `openspecEngine.GenerateAnalysis` (Solution hồ sơ `full`), `ProposalWorkspace` và kiểm phạ | chưa làm |
| 026-07 | `openspecEngine.GeneratePlan`, `TasksMdSyncer` và archive sau `request.completed` | chưa làm |
| 026-08 | RPC `Get/SetProjectEngineSettings`, metric, audit và kiểm thử tích hợp hai dialect | chưa làm |
