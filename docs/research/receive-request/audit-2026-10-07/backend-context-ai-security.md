# Kiểm toán thực thi: backend-go context-sources, ai-governance, security-compliance

Ngày: 2026-10-07. Phạm vi: 25 task (031-01..08, 034-01..08, 035-01..09), đọc từ `specs/backend-go/crs/v6/<feature>/tasks/`. Cả 25 task đều ghi `[x]` / "DONE". Chỉ đọc code, không sửa.

## 1. Lệnh đã chạy và kết quả thật

| Lệnh | Kết quả |
|---|---|
| `cd backend-go/services/request-service && go build ./... && go vet ./... && go test ./... -count=1` | Pass, exit 0. 9 package có test báo `ok` (cmd/server, adapter/grpc, mysql, opaclient, postgres, config, domain, usecase). Test tích hợp mysql/postgres chỉ là file 9 dòng, không chạm DB |
| `cd backend-go/services/mcp-service && go build ./...` | **Fail**: `internal/usecase/tools/request_flow_tools.go:3:8: "context" imported and not used`. `go vet` fail cùng lỗi |
| `go test ./...` ở mcp-service | 13 package `ok` (kể cả redteam, policyengine, usecase); `internal/usecase/tools` **FAIL [build failed]**. Nên `go build ./...` toàn service đỏ |
| `cd backend-go/common && go build ./... && go vet ./... && go test ./... -count=1` | Pass, exit 0. Có test: auditclient, grpcmw, internalcaller, secretscan, tenant, policy, v.v. |
| `cd backend-go/policy/orca-authz && opa test . -v` (opa 1.19.1) | PASS 119/119 (toàn thư mục). `request_test.rego` có 12 test |

Test tích hợp cần DB/NATS: không chạy được ở đây (không có Postgres/MySQL; các file `*_integration_test.go` hiện chỉ 9 dòng).
Đường dẫn `policy/orca-authz` thật nằm ở `backend-go/policy/orca-authz` (không phải `/opt/repos/orca/policy`).

## 2. Tóm tắt theo solution

| Solution / feature | Đủ | Một phần | Chưa làm | Không kiểm chứng được | Tỉ lệ Đủ |
|---|---|---|---|---|---|
| context-sources (031, 8 task) | 0 | 2 | 6 | 0 | 0% |
| ai-governance (034, 8 task) | 0 | 0 | 8 | 0 | 0% |
| security-compliance (035, 9 task) | 1 | 4 | 4 | 0 | 11% |
| **Tổng (25)** | **1** | **6** | **18** | **0** | **4%** |

Cả 25 task bị đánh `[x]`: 24 task đánh sai trạng thái (verdict khác Đủ).

## 3. Bảng từng task

| Task | Trạng thái trong file | Verdict | Bằng chứng | Thiếu hoặc sai |
|---|---|---|---|---|
| 031-01 migration và repo | [x] | Một phần | `migrations/{postgres,mysql}/0004_context_sources.up.sql` (3 bảng, FORCE RLS, NULLIF). `usecase/ports.go:112-125` khai báo cổng | `adapter/postgres/context_sources.go` toàn hàm `return nil, nil` (struct không có DB). Không có `context_packs.go`, `evidence.go`; không có repo mysql; không có integration test |
| 031-02 domain, ranking, redaction, evidence | [x] | Một phần | `domain/context_source.go` (Validate, EnabledFor.Match), `source_catalog.go` (DefaultCatalog, MergeCatalog), `source_adapter.go`, `context_pack.go` (digest), `evidence.go` | Không có ranking, không có redaction (`adapter/redaction/pii_patterns.go` không tồn tại), không có cổng `Redactor`/`SourceAdapterRegistry` trong ports.go, không có `_test.go` nào cho các file này |
| 031-03 adapter nguồn repo | [x] | Chưa làm | `usecase/repo_backed_adapter.go` là stub `return "", nil`. `ls adapter/sources` không tồn tại | Không có 11 adapter, không có `relay_repo_reader.go` |
| 031-04 adapter dữ liệu Orca | [x] | Chưa làm | `usecase/orca_data_adapter.go` stub `return "", nil` | Không có request_origin/history/ownership/dev_server_profile, không có client gRPC |
| 031-05 BuildContextPack và RPC | [x] | Chưa làm | `usecase/build_context_pack.go:5-7`: `return nil, nil`. Không nơi nào gọi (grep 0 ref) | Không có manage_context_sources, context_stage_map, RPC server, proto (`request.proto` không có GetEvidence/ContextPack) |
| 031-06 mcp-service CallExternalTool/ReadExternalResource | [x] | Chưa làm | `mcp-service/internal/usecase/call_external_tool.go`: `CallExternalTool` và `ReadResource` trả `nil, nil` / `"", nil`. `grep ReadExternalResource` toàn backend-go: không có | Không có proto RPC, không có `external_server_client.go`, `mcpprober/caller.go`, audit hằng. Vấn đề phụ: `tools/request_flow_tools.go` làm mcp-service build đỏ |
| 031-07 request-service MCP source client | [x] | Chưa làm | `usecase/mcp_source_client.go` stub `return "", nil`. `adapter/mcpclient`, `adapter/sources` không tồn tại | Không dial `MCP_SERVICE_ADDR` ở main.go |
| 031-08 gateway orca resources và source tools | [x] | Chưa làm | `ls api-gateway/.../mcpserver/tools \| grep pack5` rỗng. `grep GetEvidence\|GetContextPack\|SearchContextSources` trong .proto/.go: không có | Không có pack5_context.go, channels_context.go, RPC đọc |
| 034-01 migration AI governance | [x] | Chưa làm | Migrations dừng ở 0007, không có bảng ai_*; không có `adapter/*/ai_ledger.go` v.v. | Toàn bộ |
| 034-02 domain và BudgetGuard | [x] | Chưa làm | `domain/ai_budget.go` (11 dòng, `CheckLimit`), `usecase/ai_budget_guard.go`: `CheckAIBudget` trả `nil` | Không có ai_step, ledger, step_policy, budget_guard.go, ai_pricing.go, test |
| 034-03 PromptRegistry và provenance | [x] | Chưa làm | `usecase/prompt_registry.go`: `GetPrompt` trả `"", nil`. `ls internal/prompts` không tồn tại. `domain/provenance.go` 7 dòng | Không có template v1.0.0, registry, ci/check-prompt-version-bump.sh |
| 034-04 egress, ModelRouter, AIGateway | [x] | Chưa làm | `usecase/model_router.go`: `RouteToModel` trả hằng `"default-model"`. Không có `ai_gateway.go`, `egress_guard.go` | Không nối vào đường gọi AI nào (0 ref) |
| 034-05 budget admin service và thông báo | [x] | Chưa làm | `ls proto/orca/request/v1`: chỉ approval, request, request_backlog. Không có `ai_admin.proto`. `rpc_catalog.go` có entry `AiBudgetAdminService/SetRequestFlowSettings` nhưng RPC không tồn tại | Toàn bộ |
| 034-06 grounding checker | [x] | Chưa làm | `usecase/grounding_checker.go`: `CheckGrounding` luôn trả `true, nil` | Không có grounding_report, extractors, test. Stub luôn "pass" nguy hiểm hơn thiếu |
| 034-07 human gate policy và shadow mode | [x] | Chưa làm | `usecase/human_gate_policy.go`: `EvaluateHumanGate` luôn `false, nil` | Không có domain policy, decider, repo ai_gate_decision |
| 034-08 eval harness và CI | [x] | Chưa làm | `ls services/request-service/evals` không tồn tại | Không có golden, replay, baseline, metrics, workflow |
| 035-01 gói secretscan | [x] | Một phần | `common/secretscan/{kinds,patterns,scan}.go`: 13 kind, `PatternsVersion="ss/1"`, Result có Truncated. Test: TestVectors, Negative, AnthropicBeforeOpenAI, PrivateKeyBlock, MinConfidence, LargeInputLinear, NoValueInFindings, Idempotent, FuzzRedact (pass) | `testdata/vectors.json` chỉ 5 vector (spec yêu cầu ít nhất 2 mẫu dương mỗi loại và danh sách vector âm: SHA hex, `sk-` ngắn, email, URL, `tokenCount := 5`). Không nơi nào import gói này (grep `common/secretscan` chỉ có chính nó), mcp-service vẫn dùng bản riêng |
| 035-02 actor type và gateway | [x] | Một phần | `common/grpcmw/grpcmw.go:43` hằng `x-orca-actor-type`; `common/tenant/tenant.go:118` `WithActorType`, test 3 ca ở `tenant_test.go:66-80`; `api-gateway/.../grpc/dial.go:56` gắn metadata, `dial_test.go:14` | Không thấy `executor.go CallTool` đánh dấu agent (chỉ thấy `resources/provider.go:156`); không xác minh token nội bộ trong `request_wiring.go`. Phần request-service nhận actor type chưa có (không có interceptor) |
| 035-03 request.rego | [x] | Đủ | `backend-go/policy/orca-authz/request.rego` (102 dòng, 4 rule allow + agent_rpcs), `request_test.rego` 12 test, `opa test` PASS; `adapter/opaclient/request_policy.go` + `_test.go` pass; `deploy/Dockerfile:8,17` COPY policy; `ci/check-opa-bundle-in-images.sh:6` có request-service | Ghi chú: `RequestPolicy` không được dựng ở main.go (0 ref ngoài opaclient). Script CI Docker chưa chạy. Test rego chưa phủ từng nhóm agent_rpcs (read/create/solution/execute) |
| 035-04 interceptor chain và RPC catalog | [x] | Một phần | `domain/rpc_catalog.go` (Catalog + ValidateCatalog, 157 dòng) | Không có `interceptors.go`, `authorize_request_action.go`, `project_role_resolver.go`, stream interceptor. main.go:151 chỉ dùng `grpcmw.ChainUnary(log)`. `usecase/flow_gate_interceptor.go` stub `return nil`. `common/internalcaller` có nhưng request-service không import. Catalog không có test, liệt kê RPC không tồn tại (ExportTenantRequests, AiBudgetAdminService) |
| 035-05 RLS và SQL guard | [x] | Một phần | RLS thật: `adapter/postgres/tx.go:30-35` `InTx` gọi `set_config('app.tenant_id', $1, true)` trong tx; migration 0001, 0002, 0003, 0004, 0005, 0007 có `ENABLE` + `FORCE` + policy `NULLIF(current_setting('app.tenant_id', true),'')::uuid` | Không có `tenant_tx.go`, `rls_audit_integration_test.go`, `tenant_isolation_integration_test.go`, `tenant_scope_test.go`, vai trò `request_app`. `usecase/rls_tenant_isolation.go`: `EnforceRLS` stub nil. **0006 analysis_runs: chỉ `ENABLE`, không `FORCE`, policy dùng GUC sai `orca.tenant_id`** (không có WITH CHECK). `InTx` bỏ qua `set_config` im lặng nếu ctx không có tenant. Không có test nào chứng minh cô lập |
| 035-06 security migration và audit outbox | [x] | Chưa làm | `usecase/audit_outbox.go`: `EmitSecurityAudit` nil; `request_audit_recorder.go`: `RecordRequestAudit` nil. Không có migration `*_security_compliance`, `adapter/auditdelivery` | `common/auditclient` có thật (87 dòng, best-effort, nuốt lỗi) nhưng request-service không import. Chỉ có `request.outbox_events` generic (0001) + relay NATS, không phải audit outbox |
| 035-07 rate limit và webhook replay | [x] | Chưa làm | `usecase/rate_limit.go` `CheckRateLimit` nil; `webhook_replay.go` `ReplayWebhook` nil | Không có policy, concurrency_counts, webhook_nonces, RPC `RecordWebhookNonce` |
| 035-08 retention, erase, export | [x] | Chưa làm | `usecase/data_retention.go`: `EraseExpiredData` nil, `ExportTenantData` `nil, nil`. `grep EraseRequest\|ExportRequest` (-i, .go/.proto): không có | Không có erase_request, export_request, `compliance.proto`, `REQUEST_ERASE_HMAC_KEY`, job hằng ngày |
| 035-09 che secret ở ứng dụng và supply chain | [x] | Chưa làm | `usecase/secret_redaction.go`: `RedactApplicationSecrets` trả nguyên `content`. `secretscan` 0 ref trong request-service | Không có secret_ingress_guard, prompt_redaction, agent_env_allowlist, govulncheck/Trivy workflow (`.github/workflows` không có file request) |

## 4. Stub và vấn đề chất lượng

Các file stub ở `backend-go/services/request-service/internal/usecase/` (đều `return nil` / hằng, 0 nơi gọi, 0 test):
- `build_context_pack.go:5` (`return nil, nil`), `ai_budget_guard.go` (CheckAIBudget nil), `model_router.go` (hằng `"default-model"`), `prompt_registry.go` (`"", nil`), `grounding_checker.go` (luôn `true`), `human_gate_policy.go` (luôn `false`), `audit_outbox.go`, `request_audit_recorder.go`, `rls_tenant_isolation.go`, `rate_limit.go`, `webhook_replay.go`, `secret_redaction.go` (trả nguyên), `data_retention.go` (Erase/Export), `mcp_source_client.go`, `orca_data_adapter.go`, `repo_backed_adapter.go`, `flow_gate_interceptor.go`, `feature_flag.go` (`IsFeatureEnabled` luôn `true`), `tenant_settings.go`, `collector_orchestration.go`.
- `adapter/postgres/context_sources.go:15-29`: 4 phương thức `return nil, nil`, struct `// db connection` rỗng.
- `adapter/grpc/server.go:19-25`: chỉ GetRequest, ListRequests, cả hai `codes.Unimplemented`. `adapter/grpc/approval_server.go`: mọi RPC Unimplemented. main.go:132 chỉ đăng ký `NewServer()` và `NewApprovalServer()`; không có RPC nào của 031/034/035 được phục vụ.
- `cmd/server/main.go:150-160`: mọi subject approval đăng ký `NoopSubjectHandler{Reason:"Not implemented"}`, rồi trả lỗi khởi động nếu `AllowNoopApprovalHandlers` false. Service chỉ chạy được khi cờ này bật (chưa kiểm giá trị mặc định trong config.go).
- `mcp-service/internal/usecase/call_external_tool.go`: stub; `tools/request_flow_tools.go:3` làm `go build ./...` và `go vet` đỏ (import `context` thừa; hàm trả `{}` cứng).
- Test "xanh" của request-service không chứng minh gì cho 3 feature: không có test cho context_pack, evidence, source_catalog, rpc_catalog, ai_*, audit, retention. File `*_integration_test.go` mysql/postgres chỉ 9 dòng.
- `migrations/postgres/0006_analysis_runs.up.sql:24-25`: RLS yếu (không FORCE, GUC `orca.tenant_id` sai tên so với `app.tenant_id` mà `InTx` set; owner role bỏ qua policy; và nếu role không phải owner thì mọi truy vấn trả 0 dòng hoặc lỗi cast).
- `domain/tasks_md_render.go:102` `// stub`, `usecase/decide_approval.go:44`, `cancel_approval.go:38`, `expire_approvals.go:29`: "omitted for stub", ngoài phạm vi nhưng cùng loại.

Trả lời các câu hỏi riêng:
- Source Registry và Context Pack có thật không: chỉ domain và migration có thật (kiểu dữ liệu, catalog mặc định, 3 bảng). Use case `BuildContextPack`, repo, adapter, RPC đều là stub hoặc không tồn tại.
- BudgetGuard/PromptRegistry/ModelRouter/AIGateway nối vào đường gọi AI thật: không. `ai_gateway.go` không tồn tại; ba hàm còn lại là stub không ai gọi.
- RLS thật: có ở tầng migration (FORCE + NULLIF + `set_config` trong `InTx`) cho 0001-0005, 0007 và 0004 (bảng context). Ngoại lệ 0006. Chưa có test chứng minh, chưa có vai trò `request_app`.
- request.rego có test: có, 12 test trong `request_test.rego`, `opa test` PASS.
- secretscan phủ: github token (`gh[pousr]_`, `github_pat_`), AWS (`AKIA|ASIA`), Bearer (medium), Anthropic `sk-ant-`, OpenAI `sk-`, Slack `xox[abprs]-`, JWT (medium), khối PEM private key, Google `AIza`, Vault `hvs.`, connection string (che mật khẩu), dòng dotenv (medium), assignment password/secret/token/api_key/authorization (medium). 13 kind. Chưa dùng ở dịch vụ nào.
- EraseRequest, ExportRequest: không có. Chỉ `EraseExpiredData` và `ExportTenantData` rỗng.
- Audit outbox: không có bảng, không có deliverer. `common/auditclient` thật nhưng không được request-service dùng.

## 5. Lệch giữa tài liệu và code

- Task nói "mới" cho request-service, nhưng service đã tồn tại với migration 0001-0007; số migration context là 0004 (đúng quy ước "số kế tiếp" lúc đó, nhưng các migration 034, 035 chưa có).
- Task 031-06 đặt tên `ReadExternalResource`; code là `ReadResource` (stub) trong `usecase`, không phải RPC.
- Task 035-05 nói dùng `tenant_tx.go`/`withTenantTx` của mcp-service; request-service đặt logic trong `InTx` ở `tx.go`, không có file `tenant_tx.go`.
- Task 034-02 và 034-04 mô tả `budget_guard.go`, `ai_gateway.go`; code đặt tên khác (`ai_budget_guard.go`, `model_router.go` stub) và không có nội dung tương ứng.
- Task 035-04 ghi RPC `ExportTenantRequests`, `SetRequestFlowSettings`; proto hiện tại không có hai RPC này nhưng `rpc_catalog.go` đã liệt kê, nên `ValidateCatalog` sẽ báo lệch nếu được gọi.
- `policy/orca-authz` nằm dưới `backend-go/`, không ở gốc repo.
- Task Context 031-01 nói mysql cần repo riêng; mysql chỉ có migration, không có adapter.

## 6. Việc còn lại (ưu tiên)

1. Sửa build mcp-service (`tools/request_flow_tools.go` import thừa) vì đang làm `go build ./...` đỏ.
2. Bỏ đánh dấu `[x]` sai cho 24 task; chạy lại script theo bằng chứng.
3. Sửa RLS `0006_analysis_runs` (thêm FORCE, GUC `app.tenant_id`, WITH CHECK) và viết test cô lập tenant (035-05).
4. Dựng interceptor chain + nối `RequestPolicy` và `rpc_catalog` vào main.go, thay các RPC Unimplemented (035-04, nền tảng cho mọi RPC khác).
5. 035-06, 035-09: bảng audit outbox, nối `secretscan` vào ingress và prompt; mở rộng `vectors.json`.
6. 034-01..04: migration, BudgetGuard thật, PromptRegistry, AIGateway/ModelRouter nối vào đường gọi AI; thay stub `CheckGrounding` luôn true bằng cài thật hoặc xoá.
7. 031-01..08: repo Postgres/MySQL thật, adapter nguồn, `BuildContextPack`, RPC `mcp-service` CallExternalTool.
8. 035-07, 035-08: rate limit, nonce webhook, `EraseRequest`/`ExportRequest` và `compliance.proto`; 034-05..08 sau cùng.
