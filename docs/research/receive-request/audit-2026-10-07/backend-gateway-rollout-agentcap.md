# Kiểm toán thực thi: gateway-and-mcp, request-quality-rollout, agent-capabilities (backend-go)

Ngày: 2026-10-07. Phạm vi: 35 task (TASK-REQ-016-01..08, 017-01..05, 024-01..08, 025-01..08, 033-01..06). Chỉ đọc code, không sửa. Dấu `[x]` và tiêu đề "Đã triển khai" không được coi là bằng chứng.

## 1. Lệnh đã chạy và kết quả thật

| Lệnh (trong thư mục service) | Kết quả |
|---|---|
| api-gateway: `go build ./... && go vet ./... && go test ./... -count=1` | PASS toàn bộ (exit 0). wscompat 25s, mcpserver/tools 3.6s |
| infra-fleet-service: cùng lệnh | PASS toàn bộ (exit 0). usecase 10.8s, devserveragent 9.4s. Thư mục `adapter/postgres`, `adapter/mysql`: "no test files" (test capability nằm sau build tag `integration`) |
| issue-status-sync: cùng lệnh | PASS (exit 0). Chỉ `usecase` và `adapter/eventbus` có test |
| mcp-service: cùng lệnh | **FAIL ở build**: `internal/usecase/tools/request_flow_tools.go:3:8: "context" imported and not used`. `go vet` cùng lỗi, test không chạy được |
| request-service (thêm, vì task 024/025/033 trỏ vào): build, vet, test | PASS (exit 0), nhưng chủ yếu test của code cũ |
| common/auditclient: `go test ./auditclient/ -v` | PASS 6/6 (gồm 3 test AppendDetailed) |
| infra-fleet-service `go test -tags=integration ./internal/adapter/postgres/ -count=1` | **FAIL** sau 313.9s (testcontainers Postgres khởi động được, rồi container bị huỷ). Log chỉ còn dòng cuối nên chưa rõ test nào đỏ hoặc timeout; chưa xác định lỗi nằm ở migration 0039 hay ở test khác trong package (có cả test approval/outbox của package này). Chưa chạy lại riêng test capability. Bản MySQL chưa chạy |

Test parity MCP và golden (api-gateway/internal/adapter/mcpserver/tools, chạy `-v`): TestToolsListGolden, TestToolParity, TestChannelInventory, TestInventoryCountsByPack, TestArgsMappingGolden, TestRedactionGolden đều PASS. Nhưng chúng xanh vì **không có kênh Request nào trong Registry** và golden `testdata/tools_list.golden.json` (240 tool) không có tool Request nào (đếm theo tiền tố channel: git, github, task, linear, jira, worktree, repo, project, workflow, files, annotation, gitlab, automation, admin, team, terminal, fleet, connectivity; không có request/solution/approval/backlog). Test xanh không chứng minh gì cho 016/017.

## 2. Kiểm tra theo yêu cầu điều phối

| Câu hỏi | Kết quả | Bằng chứng |
|---|---|---|
| Số kênh WS request/solution/approval/backlog thật so với 28 trong CONTRACT | **0 / 28** | `grep '"(request|solution|approval|backlog)\.' services/api-gateway/internal` chỉ ra `approval.requested/resolved` của `channels_mcp_events.go` (sự kiện MCP, không phải kênh Request). Không có file `channels_request*.go` (`ls wscompat | grep -i request` chỉ ra `request_observability.go`, không liên quan) |
| Có đăng ký vào Registry không | Không | `RegisterProductionChannels` (cmd/server/main.go:~378) không có tham số, dòng nào cho Request; `parity_test.go:productionRegistry()` không có kênh Request |
| `request_flow_enabled` có interceptor thật | Không | `request-service/internal/usecase/flow_gate_interceptor.go` chỉ `return nil`; `cmd/server/main.go:169` `grpc.NewServer(grpcmw.ChainUnary(log), grpcmw.StatsHandler())` không có interceptor Request; `RequestFlowEnabled` (config.go:17) chỉ được đọc trong config và config_test. `feature_flag.go` `IsFeatureEnabled` luôn `true` |
| Có workflow `backend-go-request-service.yml` | Không | `ls .github/workflows \| grep -i -E "backend\|request"`: không có file; cũng không có `request-e2e.yml`. Có `backend-go-infra-fleet-service.yml` (sửa +15 dòng) |
| `tests/request` | Không tồn tại | `ls /opt/repos/orca/tests` không có `request`; `docs/guides/request` cũng không có |
| `ci/check-request-service-wiring.sh` | Không có | `ls backend-go/ci` chỉ 4 script cũ + mcp-conformance |
| `deploy/alerts/request.rules.yaml` | Không có | `ls backend-go/deploy/alerts`: chỉ `mcp.rules.yaml` |

## 3. Tóm tắt theo solution

| Solution | Đủ | Một phần | Chưa làm | Không kiểm chứng | Tổng | Tỉ lệ Đủ |
|---|---|---|---|---|---|---|
| BE-REQ-SOL-016 (016-01..08) | 0 | 1 | 7 | 0 | 8 | 0% |
| BE-REQ-SOL-017 (017-01..05) | 0 | 0 | 5 | 0 | 5 | 0% |
| BE-REQ-SOL-024 (024-01..08) | 1 | 0 | 7 | 0 | 8 | 12.5% |
| BE-REQ-SOL-025 (025-01..08) | 0 | 0 | 8 | 0 | 8 | 0% |
| BE-REQ-SOL-033 (033-01..06) | 1 | 3 | 2 | 0 | 6 | 17% |
| **Tổng** | **2** | **4** | **29** | **0** | **35** | **5.7%** |

Mọi task đều có `[x]` (hầu hết các mục tiêu chí). 33 task đánh dấu `[x]` nhưng verdict không phải "Đủ" (trạng thái sai); riêng 024-01 và 033-02 là hợp lệ.

## 4. Bảng từng task

| Task | Trạng thái ghi | Verdict | Bằng chứng | Thiếu hoặc sai |
|---|---|---|---|---|
| 016-01 client wiring | [x] | Một phần | `api-gateway/cmd/server/request_wiring.go:16` `dialRequestService`; `request_wiring_test.go` 2 test PASS | Hàm không được gọi ở đâu (grep `dialRequestService` chỉ có định nghĩa). Không có `REQUEST_SERVICE_ADDR` trong config.go, không health check `request-service`, không tạo client |
| 016-02 errors/views/source | [x] | Chưa làm | Không có `requestChannelError`, view, source guard (grep) | Cả ba file |
| 016-03 lifecycle channels | [x] | Chưa làm | Không có `request.*` trong Registry | Toàn bộ |
| 016-04 solution/approval | [x] | Chưa làm | Như trên | 9 kênh |
| 016-05 backlog | [x] | Chưa làm | Như trên | 3 kênh |
| 016-06 request.subscribe | [x] | Chưa làm | Không có `request.subscribe`, `RegisterStream` cho Request | Stream, 14 sự kiện |
| 016-07 HTTP routes + docs | [x] | Chưa làm | `adapter/http/request_routes.go` (thân rỗng `// http routes`), `request_webhook.go` (chỉ `WriteHeader(200)`), không được gọi; httpgateway không có route Request | 5 route; README gateway không có mục Request (mục "code-intel" thêm, không phải Request) |
| 016-08 kênh bổ sung | [x] | Chưa làm | Không tìm thấy | 3 kênh |
| 017-01 ToolSpec pack | [x] | Chưa làm | `mcp-service/internal/usecase/tools/request_flow_tools.go`: `RequestFlowToolSpec() []byte { return []byte("{}") }`, **làm vỡ build mcp-service** (import `context` thừa). Golden gateway không có tool Request | Toàn bộ; build đỏ |
| 017-02 tool origin | [x] | Chưa làm | `origin_executor.go`: `ExecuteWithOrigin` `return nil`; không ai tham chiếu | Logic `source_provider=mcp`, `source_site` |
| 017-03 rate limiter | [x] | Chưa làm | `create_rate_limiter.go`: `Allow` luôn `true`, không đếm, không env `MCP_REQUEST_CREATE_PER_HOUR` | Giới hạn 20/giờ, test |
| 017-04 untrusted + golden | [x] | Chưa làm | Golden không có tool Request; mcp-service không có file `_test.go` nào trong `usecase/tools` | Cờ UntrustedOutput, golden 17 tool |
| 017-05 e2e + guides | [x] | Chưa làm | `tests/mcp/` không có script Request, `docs/guides/mcp` không có mục Request (grep `request_create`) | Script, tài liệu |
| 024-01 AppendDetailed | [x] | Đủ | `common/auditclient/client.go` `Entry`, `AppendDetailed`, `Append` gọi lại; test 6/6 PASS (ActorType lạ, metadata >4096, forward 10 trường) | Tiêu chí "mọi module go.work biên dịch" sai ở thời điểm này vì mcp-service lỗi build (không do task này) |
| 024-02 audit recorder | [x] | Chưa làm | `request-service/usecase/request_audit_recorder.go`: `RecordRequestAudit` `return nil` | Ghi audit thật, redaction |
| 024-03 sync state migration | [x] | Chưa làm | `issue-status-sync/migrations/*` chỉ `0001_processed_events`; không có `request_sync_state.go` | Migration 2 dialect, repo, test |
| 024-04 handle status usecase | [x] | Chưa làm | issue-status-sync không có `sync_request_status.go`, `request_events.go`; handler ở request-service là stub `HandleRequestStatusChange` `return nil` | Toàn bộ |
| 024-05 lookup by source RPC | [x] | Chưa làm | `request.proto` chỉ có GetRequest/ListRequests/ListBacklog (dòng 60-62); `LookupRequestBySource` usecase `return "", nil` | RPC, truy vấn, guard |
| 024-06 ownership check | [x] | Chưa làm | Không có `grpcclient/request_client.go`, `request_lookup_retry.go` | Toàn bộ |
| 024-07 metrics | [x] | Chưa làm | `request-service/adapter/metrics/request_metrics.go`: hai chuỗi hằng, không đăng ký collector; không có `issuesync_metrics.go` | Counter/histogram thật, wiring |
| 024-08 alert/trace/Jira comment | [x] | Chưa làm | Không có `deploy/alerts/request.rules.yaml`; không thư mục `adapter/outbox` ở request-service; không `IssueCommenter` | Toàn bộ |
| 025-01 tenant settings + flow RPC | [x] | Chưa làm | `usecase/tenant_settings.go` trả `TenantSettings{}`; migrations request-service dừng ở 0007, không có tenant_settings; proto không có RPC flow | Migration, repo, RPC `request.flowStatus` |
| 025-02 flow gate interceptor | [x] | Chưa làm | `usecase/flow_gate_interceptor.go` `return nil`; không có `adapter/grpc/flow_gate.go`; không nối vào `grpc.NewServer` (main.go:169) | Interceptor, `REQUEST_FLOW_DISABLED` |
| 025-03 e2e harness + E01 | [x] | Chưa làm | Không có thư mục `request-service/e2e` | Toàn bộ |
| 025-04 kịch bản E02..E19 | [x] | Chưa làm | Như trên | Toàn bộ |
| 025-05 feature flag e2e + CI | [x] | Chưa làm | Không có `backend-go-request-service.yml` | Toàn bộ |
| 025-06 wiring check script | [x] | Chưa làm | Không có `ci/check-request-service-wiring.sh` | Toàn bộ |
| 025-07 T2 stack e2e | [x] | Chưa làm | Không có `tests/request`, `ci/request-e2e`, `request-e2e.yml` | Toàn bộ |
| 025-08 docs + runbook | [x] | Chưa làm | Không có `docs/guides/request` | 7 file docs |
| 033-01 profile migration + repo | [x] | Một phần | `infra-fleet-service/migrations/{postgres,mysql}/0039_dev_server_capability_profiles.{up,down}.sql`; `domain/capability_profile.go`, `agent_features.go`; `adapter/{postgres,mysql}/capability_profile_repository.go`; test domain PASS | Test repo và migration có build tag `integration`, không chạy được ở đây. Store không được khởi tạo trong `cmd/server/main.go` (grep `CapabilityProfileStore`: chỉ interface ở `usecase/capability_ports.go`) |
| 033-02 handshake info | [x] | Đủ | `usecase/ports.go:85-96` `HandshakeInfo` có Features, ProtocolVersion, BuildVersion, `EffectiveProtocolVersion`; `SanitizeAgentFeatures` gọi ở `devserveragent/session.go:325`, `agentwsserver/server.go:225`, `sshrelay/provisioner.go:259`; test `last_handshake_info_test.go`, `agentwsserver/server_test.go`, `agent_features_test.go` PASS | Không |
| 033-03 refresh/get usecase + RPC | [x] | Một phần | Proto có `rpc GetDevServerCapabilities` (infrafleet.proto:368); `usecase/refresh_dev_server_capabilities.go`, `get_dev_server_capabilities.go` có logic thật (singleflight theo khoá, min interval, fallback handshake_only, outbox event) | Không có `adapter/grpc/server_capability.go`, server chỉ nhúng `UnimplementedInfraFleetServiceServer` nên RPC trả Unimplemented. Hai usecase không được dựng trong main.go. Không có test cho hai usecase, không `WithOnSessionAttached`, không config. Nhánh probe `json.Marshal(res)` không parse kết quả `agent.capabilities` thật |
| 033-04 capability reader (request-service) | [x] | Một phần | `grpcclient/infra_capability_client.go`, `domain/dev_server_capability.go` (`ParseCapabilityProfile`, `HasFeature`), `domain/agent_route_selection.go`, port ở `usecase/capability_ports.go` | `NewInfraCapabilityClient` không được gọi ở main.go; không có `INFRA_FLEET_SERVICE_ADDR`; không có test nào (không `_test.go` cho client, domain capability, route selection); comment tự thừa nhận chưa gắn tenant metadata. Gọi vào RPC chưa có server ở 033-03 |
| 033-05 agent exec prompt client | [x] | Chưa làm | `grpcclient/dev_server_executor.go`: `Exec`, `ExecPrompt` là stub trả `"OK"`, `"openspec version 1.2.3"`; không có `AgentPromptInput`, `agent_prompt_params.go`, `agent_prompt_result.go`, `domain/agent_prompt_errors.go` | Toàn bộ; stub hằng cứng trả thành công giả |
| 033-06 golden + degradation tests | [x] | Chưa làm | Không có `agent_capabilities_v1.golden.json`, `capability_degradation_test.go`, `dev_server_capability_contract_test.go` (find `*golden*`, `*capab*test*` ở request-service: rỗng) | Toàn bộ |

## 5. Stub và vấn đề chất lượng

- `mcp-service/internal/usecase/tools/request_flow_tools.go:3` import `context` thừa, **mcp-service không build được, vet đỏ**. Hàm trả `{}`.
- `mcp-service/.../tools/origin_executor.go`, `create_rate_limiter.go`: thân hàm no-op, `Allow` luôn `true` (rate limit giả).
- `request-service/internal/usecase/`: `flow_gate_interceptor.go` (`return nil`), `feature_flag.go` (luôn `true`), `tenant_settings.go` (rỗng), `request_audit_recorder.go`, `handle_request_status.go`, `lookup_request_by_source.go` (`return "", nil`), `agent_capabilities.go` (`return nil, nil`), `infrafleet_client.go` (`ReconnectWaitAndRetry` rỗng). Không ai gọi các hàm này.
- `request-service/internal/adapter/grpcclient/dev_server_executor.go`: stub trả thành công giả (`Stdout: "OK"`).
- `request-service/internal/adapter/metrics/request_metrics.go`: chỉ hằng chuỗi.
- `api-gateway/internal/adapter/http/request_routes.go`, `request_webhook.go`, `websocket/request_client.go`: vỏ rỗng, không được import.
- `api-gateway/cmd/server/request_wiring.go`: helper dial mồ côi, test chỉ khẳng định "không panic".
- `infra-fleet-service`: RPC `GetDevServerCapabilities` khai báo trong proto nhưng không có handler (Unimplemented); usecase chưa dựng trong main.go; không test cho Refresh/Get usecase.
- `api-gateway/internal/adapter/wscompat/registry_test.go:157` có `t.Skip("TODO: wire every channels_*_test.go ...")` (cũ, không thuộc phạm vi nhưng làm giảm độ tin cậy kiểm tra identity theo kênh).
- Test parity và golden xanh một cách rỗng với Request (xem mục 1).
- Test tích hợp `-tags=integration` của package postgres (infra-fleet) FAIL sau 314s, chưa rõ nguyên nhân; bản MySQL chưa chạy.

## 6. Lệch giữa tài liệu và code

- Task ghi "file do CR-REQ-001 tạo" cho `backend-go-request-service.yml` nhưng workflow này chưa bao giờ được tạo; task 025-05 sửa một file không tồn tại.
- Task 016-01 mô tả cờ rỗng `REQUEST_SERVICE_ADDR`; code dùng `REQUEST_INTERNAL_CALLER_TOKEN` và không có biến địa chỉ.
- Task 033-03 mô tả `internal/adapter/grpc/server_capability.go`; code thực tế có `server_code_intel.go` và `usecase/get_agent_capabilities.go` (RPC khác của luồng code-intel, thuộc solution khác) trong cùng commit, không phải sản phẩm của 033.
- CONTRACT nói "28 kênh"; bảng mục 2 có 42 dòng bắt đầu bằng kênh Request (gồm kênh bổ sung mục 2.5). Con số 28 chỉ đúng với CR-REQ-016 gốc.

## 7. Việc còn lại theo ưu tiên

1. Sửa build mcp-service (xoá import thừa hoặc làm thật `RequestFlowToolSpec`); đưa test mcp-service vào CI vì hiện build đỏ.
2. Bỏ `[x]` sai ở 33 task (hoặc đánh lại), đổi tiêu đề "Đã triển khai" của SOL-016/017/024/025.
3. Nối infra-fleet 033-03: viết `server_capability.go`, dựng hai usecase trong main.go, thêm test usecase, chạy test tích hợp 0039 trên Postgres/MySQL thật.
4. Làm thật nền Request ở request-service: tenant settings + interceptor `request_flow_enabled` (025-01/02), audit recorder (024-02), lookup RPC (024-05).
5. Làm kênh WS Request và nối vào `RegisterProductionChannels` (016-01..06), rồi thêm tool MCP và cập nhật golden, parity (017).
6. issue-status-sync: migration, usecase sync request status, ownership check (024-03/04/06).
7. Metrics, alert, trace (024-07/08), workflow CI request-service, script wiring check, e2e (025-03..07), docs (025-08).
8. 033-05 client exec prompt thật thay stub, 033-06 golden và degradation test, 033-04 thêm test và nối main.go.
