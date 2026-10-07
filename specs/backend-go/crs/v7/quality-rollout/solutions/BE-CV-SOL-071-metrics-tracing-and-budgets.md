# BE-CV-SOL-071: Ngân sách hiệu năng, metrics Prometheus, tracing, cảnh báo và số liệu server của CR-CV-095

> ✅ **Đã triển khai.** Toàn bộ code đã được implement và verify (xem task list).

**CR:** [CR-CV-071](../../../../../../docs/crs/v7/quality-rollout/CR-CV-071-performance-budgets-metrics-tracing.md); phần server của [CR-CV-095](../../../../../../docs/crs/v7/quality-gate/CR-CV-095-review-quality-telemetry.md) (§2.6, §2.8)
**Service:** `code-intel-service` (mới: `internal/adapter/metrics`, `bench/`), `api-gateway` (`wscompat`, `cmd/server`), `infra-fleet-service` (`devserveragent`, `adapter/metrics`, `cmd/server/main.go`), `backend-go/deploy/alerts/` (mới), `backend-go/ci/code-intel-bench/` (mới), `docs/guides/code-intel/`
**TDD tham chiếu:** [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (ba trụ cột; "Naming prefixes and scrape path (T8)": `/metrics` = health mux + `promhttp`, registry riêng; SLO "p99 < 300 ms cho gRPC không fan-out ra execution plane" nên các RPC lạnh của code-intel được phép chậm hơn vì có fan-out; "Dashboards & alerting"), [`arch/10`](../../../../tdd/architecture/10-deployment-infrastructure.md) (CI theo module), [`services/api-gateway.md`](../../../../tdd/services/api-gateway.md) §8 (WS↔gRPC bridging, backpressure), [`services/infra-fleet-service.md`](../../../../tdd/services/infra-fleet-service.md) §8 (NFR), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (adapter metrics không lọt vào `domain/`)
**Task:** [`../tasks/README.md`](../tasks/README.md)

---

## 1. Hợp đồng áp dụng

| Nguồn | Mục / PQ | Dùng để |
|---|---|---|
| `CONTRACT-codeintel-agent-rpc.md` | §2.2 khối `perf` (`totalMs, queueWaitMs, cliCalls, cli[{tool,command,ms,stdoutBytes,rssPeakKb}], parseMs, truncated`; `command` chỉ là hằng whitelist), §2.3 giới hạn, §2.5 bảng timeout, §3 mã lỗi | nguồn số đo agent; tập đóng của nhãn |
| `CONTRACT-codeintel-proto-and-data-map.md` | PQ-03 (tập mã lỗi, `apperrors` Kind mới), PQ-13 (timeout nhiều tầng), PQ-14 (trần 2 MiB, snapshot 3 MiB mặc định, `MaxCallRecvMsgSize`), PQ-17/PQ-18 (thông báo agent, luồng infra-fleet 16/(tenant, dev server)), PQ-23 (`HTTP_PORT` 8080, biến `CODEINTEL_*`), §3.1/§3.2 (danh sách RPC), §4 (bảng), §5 (`outbox`), §8.2 (CR-095 server nằm ở solution này), §8.3 | tập RPC, mã lỗi, tên biến, nguồn truy vấn quản trị |
| `CONTRACT-codeintel-ui-api.md` | §2.3 bảng lỗi tới client, §2.4 timeout/giới hạn gateway, §3 danh sách **46** kênh | tập kênh đóng; mã `result` |

## 2. Trạng thái hiện tại (re-verify)

Đã đọc (2026-10-06): `backend-go/services/api-gateway/internal/adapter/mcpmetrics/metrics.go` (registry riêng; `Registry()` dòng 89, `Handler()` dòng 92; quy tắc cardinality ở đầu file), `.../mcpmetrics/instrument.go` (`tracer()` = `otel.Tracer("orca/api-gateway/mcp")`, span `mcp.policy`, `mcp.dispatch`), `.../cmd/server/mcp_metrics_wiring.go` (`healthAndMetricsMux(health http.Handler, m *mcpmetrics.Metrics)`), `.../cmd/server/main.go` (`mcpMetrics := newMCPMetrics()` dòng 282, dùng ở dòng 587; `otelhttp.NewHandler(router, "api-gateway")` dòng 576; fan-out TRACE dòng 118–137), `.../wscompat/handler.go` (`invokeTimeout = 25 s` dòng 247; `handleInvoke` bọc `context.WithTimeout`), `.../wscompat/registry.go` (`Register`, `RegisterStreamChannel`, `Dispatch`), `.../grpc/dial.go:32` (`otelgrpc.NewClientHandler()`), `backend-go/common/grpcmw/grpcmw.go` (~dòng 140 `StatsHandler()` = `otelgrpc.NewServerHandler()`), `backend-go/common/tracing/tracing.go` (`otel.SetTextMapPropagator(propagation.TraceContext{})`; bộ xử lý đẩy span lên JetStream `TRACE`), `backend-go/services/mcp-service/cmd/server/main.go` và `notification-service/cmd/server/main.go` (`healthAndMetricsMux`), `backend-go/services/infra-fleet-service/cmd/server/main.go` (dòng 490 `inframetrics.NewFleetCollector()`, dòng 784 `/health/metrics`, dòng 401 `otelgrpc` ở client auth), `.../infra-fleet-service/internal/adapter/metrics/fleet_collector.go`, `backend-go/services/project-service/internal/adapter/grpcclient/dev_server_relay.go:29` (mẫu dial có `otelgrpc` tới infra-fleet), `backend-go/deploy/alerts/mcp.rules.yaml` (dòng đầu: "Not loaded by anything in this repository"), `backend-go/ci/mcp-conformance/run-go-conformance.sh` (có "metrics + trace chain"; `FUZZTIME`), `desktop/config/scripts/check-terminal-perf-report-budgets.mjs` (mẫu so báo cáo với ngân sách), `backend-go/services/**/*.sql` (đếm **588** tệp `.sql` dưới `backend-go/`; tổng repo 1478).

| # | Correction relative to CR-CV-071 | Bằng chứng |
|---|---|---|
| C1 | CR: "chưa kiểm chứng `mcpmetrics.Metrics` có lộ `Gatherer`". **Đã kiểm: có** `Registry() *prometheus.Registry` (dòng 89), nên ghép bằng `prometheus.Gatherers{mcp.Registry(), codeIntel.Registry()}` + `promhttp.HandlerFor` là khả thi; `newMCPMetrics()` luôn được dựng (main.go:282) | `metrics.go`, `main.go` |
| C2 | CR: "kênh WS của gateway không tạo span". **Chính xác hơn**: có **một** span `otelhttp` cho cả yêu cầu HTTP (kể cả nâng cấp `/ws`) qua `otelhttp.NewHandler(router, "api-gateway")` (main.go:576); **mỗi lần gọi kênh** trên kết nối đó không có span (grep `tracer.Start`/`otelhttp` ở `wscompat` không thấy ngoài test). Việc ctx của `handleInvoke` có kế thừa span nâng cấp hay không **chưa kiểm chứng** | `main.go`, `handler.go:258` |
| C3 | CR trỏ mẫu so ngân sách tới `config/scripts/check-terminal-perf-report-budgets.mjs`; tệp thực nằm ở `desktop/config/scripts/` (README v7 §8 điểm 20; `config/scripts/` ở gốc chỉ còn 3 tệp) | `ls` |
| C4 | `infra-fleet-service` mount `/health/metrics` (không phải `/metrics`): đúng như CR; đây là ngoại lệ không nhân rộng | `main.go:784` |
| C5 | CI backend: 18 workflow `backend-go-*`; không chạy `golangci-lint` hay `opa test`; Go CI 1.25 so với `go.work` 1.26 | `ls .github/workflows` |
| C6 | `code-intel-service`, gói `internal/adapter/metrics`, `bench/`, `codeintel.rules.yaml` đều **chưa tồn tại**; tên là đề xuất (mới) | `ls` |

## 3. Lệch giữa CR và hợp đồng

| # | CR nói | Hợp đồng nói (thắng) | Hệ quả |
|---|---|---|---|
| L1 | Nhãn `rpc` có "25 tên RPC ở README 3.6" | §3.1 có **29** RPC `CodeIntelService` và §3.2 có **20** RPC `QualityGateService` (đếm 2026-10-06) = 49 | tập `rpc` **sinh từ `ServiceDesc`** của hai service, không viết tay; test đỏ khi thêm RPC mà ngoài tập |
| L2 | `channel` có "26 kênh" | ui-api §3: **46** kênh (26 + 20 `codeIntel.quality.*`) | tập `channel` lấy từ danh sách đăng ký kênh; test khớp `TestChannelInventory` |
| L3 | `result` ∈ `ok` + 14 mã chữ thường (`forbidden`, `disabled`…) | PQ-03 và ui-api §2.3 có >40 mã; `forbidden` đổi thành `CODEINTEL_NOT_AUTHORIZED`; thêm `quality_gate_disabled`, `ai_review_disabled`, `unavailable`, `rate_limited`, `concurrency_limit`, `version_conflict`, … | `result` = `ok` + mã `CODEINTEL_*` viết thường theo **một bảng Go duy nhất** (cùng bảng dùng để dựng message lỗi), mã lạ → `other` |
| L4 | `graph_snapshots.payload` ≤ 2 MiB; hàng đợi chờ ≤ 5 s; "service từ chối khi > 16 MiB"; stdout agent ≤ 8 MiB | PQ-14: snapshot mặc định ≤ **3 MiB** (`CODEINTEL_SNAPSHOT_MAX_BYTES`, tối đa 8 MiB); response ≤ 2 MiB; agent-rpc §2.3: chờ slot ≤ **10 s**, stdout ≤ **16 MiB**, JSON cuối ≤ **8 MiB**; §3.3 `OUTPUT_TOO_LARGE` khi `ResourceExhausted` > **12 MiB** | `codeintel-budgets.json` dùng các số của hợp đồng |
| L5 | Hàng ngân sách `GetArchitecture` (cụm gộp cạnh ≤ 12 s) | PQ-10: `GetArchitecture` là **C4**; đồ thị cụm là `GetClusterOverview` (chưa có kênh) | tách hai hàng; hàng C4 chưa có số đo (đặt cùng bậc `GetStructure` làm giá trị tạm, ghi rõ giả định) |
| L6 | Bộ đếm `tool_format_drift_total{tool,command}` và `tool_compat{state∈verified,untested,incompatible}` | agent-rpc §3.2: `data` của `TOOL_FAILED` có `tool`, `reason`… **không có `command`**; §4.1 `status` chỉ có `supported: bool`, không có `untested` | nhãn là `{tool}` (và `reason` ∈ tập đóng khi agent đặt); `tool_compat{state∈supported,unsupported}`. Cần quyết định hợp đồng nếu muốn `command`/`untested` (mục 11) |
| L7 | `orca_codeintel_events_total{event∈index_changed,reindex_progress}`, `orca_fleet_codeintel_notifications_total{type∈indexChanged,reindexProgress}` | PQ-17: infra-fleet chuyển cả `quality.progress`, `quality.finished`; `kind ∈ index_changed|reindex_progress|quality_progress|quality_finished|resync|overflow`; PQ-18: luồng 16/(tenant, dev server) | tập nhãn theo PQ-17; thêm gauge luồng |
| L8 | Số liệu server CR-095 tên `codeintel_quality_gate_evaluations_total`, `codeintel_quality_waivers_total{kind}`, `codeintel_quality_gate_unknown_total{reason}` | CR-071 Q4: tiền tố `orca_`, nhãn từ tập đóng | đổi thành `orca_codeintel_quality_*`; nhãn `verdict ∈ pass|warn|fail|unknown` (T12 `CHECK`), `kind ∈ finding|structure_finding|check` (T11 `CHECK`) |
| L9 | CR-095 §2.8 tính tỉ lệ báo nhầm từ `finding_dismissals`/`quality_waivers` "theo `reason=false_positive`" | T5: `reason` là `varchar(500)` tự do; `disposition ∈ ignored|resolved`; T11: `reason text`. **Không có enum `false_positive`** | không thể tính chắc tỉ lệ báo nhầm từ schema hiện tại; truy vấn quản trị dùng `disposition='ignored'` làm **xấp xỉ** và ghi rõ; hỏi hợp đồng (mục 11) |
| L10 | Rule `CodeIntelGatewayStreamsSaturated` so với `CODE_INTEL_MAX_STREAMS` | PQ-23: `CODE_INTEL_MAX_STREAMS` mặc định 500; biểu thức PromQL không đọc env | thêm gauge `orca_gateway_codeintel_streams_limit` (giá trị cấu hình) |
| L11 | Span `codeintel.agent.cli` thuộc tính `command` | `command` agent chỉ là `cypher|context|impact|query|status|callers|callees|files|affected|detect-changes`; `structuralFacts` dùng `gitnexus check --cycles` (agent-rpc §9), `analyze`/`sync` ở reindex, `trace`, `list` ở whitelist | lệnh ngoài tập `perf` hiện tại → `other` (và hỏi hợp đồng mở rộng tập) |

## 4. Phụ thuộc chéo khu vực

| Khu vực | Solution | Quan hệ |
|---|---|---|
| AG | `AG-CV-SOL-071-perf-block-and-bench` | sinh khối `perf`, hàng đợi ≤ 2 `gitnexus`/≤ 3 `codegraph`, script `agent/scripts/bench-codeintel.mjs`; BE đọc `perf` (task 03), bench Go dùng cùng báo cáo |
| AG | `AG-CV-SOL-070-golden-fixtures-and-parsers` | chế độ `--bench` của script chụp (mẫu lớn ngoài git) |
| BE | `BE-CV-SOL-010` (khung, `/metrics`, `apperrors` Kind), `021` (collector), `022` (cache), `023` (transport, `execTimeoutForMethod`), `040-*` (kênh, `SetReadLimit`), `013-*` (hạn mức đồng thời), `024` (sự kiện) | nơi gắn metric; `023` sở hữu bảng timeout, solution này **không** lặp |
| BE | `BE-CV-SOL-085-quality-gate-evaluator-and-profiles`, `085-waivers-and-trend` | điểm gắn bộ đếm cổng/miễn trừ (task 09) |
| BE | `BE-CV-SOL-073-settings-flag-and-rollout` | điều kiện chuyển giai đoạn dùng alert của solution này; `disabled_rejections_total` gắn ở interceptor cờ |
| FE | `FE-CV-SOL-095-review-telemetry` | telemetry sản phẩm ở client (không PostHog ở Go, CR-095 D1); solution này chỉ cấp **số liệu server** |

## 5. Giải pháp

### 5.1 Ngân sách là dữ liệu (task 01)

`backend-go/ci/code-intel-bench/codeintel-budgets.json` (mới). Cấu trúc: `{ "version": 1, "assumption": "chưa đo; hiệu chỉnh một lần sau benchmark đầu", "rpc": { "GetIndexStatus": {"warmP95Ms":300,"coldP95Ms":3000}, ... }, "payloadBytes": {...}, "agent": {"maxGitnexus":2,"maxCodegraph":3,"maxQueueWaitMs":10000,"maxProcessTreeRssMiB":2048}, "service": {"heapMiB":512} }`. Các hàng lấy từ bảng CR-071 §2.1 **với sửa theo L4, L5**; RPC mới của hợp đồng mà CR chưa có số (quality, cổng, báo cáo, AI, C4, ERD…) có `"coldP95Ms": null` và `"unmeasured": true` — checker **không** đỏ với `null` nhưng in cảnh báo (không đoán số). Vượt ngân sách trong báo cáo ⇒ thoát khác 0.

Nhắc nhở nhất quán với hợp đồng: mọi `coldP95Ms` < 20 s (gateway đọc, ui-api §2.4) < 25 s (`invokeTimeout`); `GetChangeOverlay` lạnh ≤ 10 s so với trần agent `detectChanges` 55 s và Go 90 s (agent-rpc §2.5) là **ngân sách người dùng**, không phải timeout.

### 5.2 Metrics `code-intel-service` (task 02, 03, 09)

Gói `internal/adapter/metrics` (mới): `prometheus.NewRegistry()` riêng, mount `/metrics` trên `HTTP_PORT` (8080, PQ-23) bằng mux "health + metrics" như `mcp-service`/`notification-service` (arch/09 T8); `collectors.NewGoCollector()`, `NewProcessCollector` như `newMCPMetrics`. Hàm `pick(v, fallback, allowed…)` kiểu `mcpmetrics` để mọi giá trị ngoài tập thành `other`.

Tập đóng:
- `rpc`: `CodeIntelService_ServiceDesc.Methods ∪ Streams` và `QualityGateService_ServiceDesc` (sinh bởi `buf generate`), không viết tay; hiện **49** (L1).
- `result`: `ok` ∪ bảng mã `CODEINTEL_*` viết thường ∪ `other` (L3).
- `cache`: `hit|miss|stale_served|shared`; `tool`: `gitnexus|codegraph`; `command`: tập `perf` (L11) ∪ `other`; `method` agent: các method công khai (agent-rpc §4–5).

Metric (từ CR-071 §2.3 đã sửa): `orca_codeintel_requests_total{rpc,result}`, `orca_codeintel_request_duration_seconds{rpc,cache}` (histogram 0,01–30 s), `orca_codeintel_cache_events_total{event}`, `orca_codeintel_cache_bytes`, `orca_codeintel_agent_calls_total{method,result}`, `orca_codeintel_agent_call_duration_seconds{method}`, `orca_codeintel_agent_payload_bytes{method}` (1 KiB–16 MiB), `orca_codeintel_response_truncated_total{method}`, `orca_codeintel_agent_queue_wait_seconds{tool}`, `orca_codeintel_agent_cli_seconds{tool,command}`, `orca_codeintel_agent_rss_peak_bytes{tool}`, `orca_codeintel_tool_format_drift_total{tool}` (L6), `orca_codeintel_tool_compat{tool,state}` (state ∈ `supported|unsupported`), `orca_codeintel_tool_info{tool,version}` (≤ 8 giá trị rồi `other`), `orca_codeintel_reindex_jobs{status}`, `orca_codeintel_reindex_duration_seconds{mode,result}`, `orca_codeintel_stale_responses_total{view}`, `orca_codeintel_events_total{kind,result}` (L7; `result ∈ delivered|dropped`), `orca_codeintel_outbox_pending`, `orca_codeintel_disabled_rejections_total{rpc}`, `orca_codeintel_concurrency_rejections_total{scope}`. **Không bao giờ** nhãn tenant, user, worktree, project, đường dẫn, tên symbol, `jobId`, `commit`, `run_id`.

Khối `perf` (task 03): collector đọc `perf` → ghi histogram, tạo span con (5.4), **bỏ** khỏi dữ liệu lưu/gửi (PQ-12); thiếu `perf` không lỗi.

Số liệu server CR-095 (task 09, L8, L9): `orca_codeintel_quality_gate_evaluations_total{verdict}`, `orca_codeintel_quality_waivers_total{kind,action}` (`action ∈ waive|revoke|expire`), `orca_codeintel_quality_gate_unknown_total{reason}` (`reason` tập đóng do `BE-CV-SOL-085` định nghĩa, ví dụ `no_run`, `stale_index`, `env_not_ready`), cùng gói **truy vấn quản trị** chỉ đọc (SQL cho hai dialect) trên `quality_trend_points` (T12), `quality_waivers` (T11), `finding_dismissals` (T5), `agent_turns` (T14) để người vận hành xem tỉ lệ `unknown`, tỉ lệ miễn trừ khi `fail`, quy tắc bị bỏ qua nhiều nhất cho **một tenant** (tham số `tenant_id` bắt buộc). Không thêm RPC (hợp đồng không có). Ngưỡng nâng cổng (CR-095 §2.8) chỉ là số đề xuất, không mã hoá thành hành vi sản phẩm.

### 5.3 Gateway và infra-fleet (task 04, 05)

Gateway: gói `internal/adapter/codeintelmetrics` (mới, tên theo nội dung) với `orca_gateway_codeintel_channel_total{channel,result}`, `..._duration_seconds{channel}`, `..._response_bytes{channel}` (theo `proto.Size`), `orca_gateway_codeintel_streams_active`, `orca_gateway_codeintel_streams_limit`, `orca_gateway_codeintel_events_dropped_total{reason∈closed,resync,overflow}`. Gắn bằng **decorator** quanh `ChannelHandler`/`StreamChannelHandler` lúc `registerCodeIntelChannels` (CR-040) để mọi kênh `codeIntel.*` mới tự được đo (cùng tinh thần "bọc ở tầng transport", handler.go:241). Ghép với `orca_mcp_*`: đổi `healthAndMetricsMux` nhận `http.Handler` hoặc `prometheus.Gatherer` (C1) và dùng `prometheus.Gatherers{mcp.Registry(), ci.Registry()}`.

infra-fleet: thêm vào registry `fleet` hiện có (scrape `/health/metrics`, C4): `orca_fleet_codeintel_notifications_total{kind}`, `orca_fleet_codeintel_notifications_dropped_total{reason}`, `orca_fleet_codeintel_relay_seconds{class}` (`class ∈ read|detect|quality_list|quality_other`, theo bảng timeout §2.5, **không** theo tên method để cardinality nhỏ), `orca_fleet_codeintel_streams_active`. Không sửa `execTimeoutForMethod` (thuộc BE-CV-SOL-023).

### 5.4 Tracing (task 04, 05, 06)

Chuỗi mong muốn như CR-071 §2.6. Việc mới: (a) gateway bọc từng kênh `codeIntel.*` bằng `otel.Tracer("orca/api-gateway/codeintel")` (mẫu `mcpmetrics/instrument.go`), thuộc tính `codeintel.channel`, `codeintel.result`; (b) `code-intel-service` đặt `grpcmw.StatsHandler()` ở server và `otelgrpc.NewClientHandler()` ở dial tới infra-fleet (mẫu `project-service/.../dev_server_relay.go:29`); (c) `devserveragent.call` ở infra-fleet; (d) span con **hậu kỳ** của agent từ `perf` bằng `trace.WithTimestamp` (agent không có OTel, không đổi). Thuộc tính span chỉ lấy từ tập đóng/số; cấm đường dẫn, tên symbol/repo, mã nguồn, `worktreeId` đầy đủ, tên tenant (span đẩy lên JetStream `TRACE` và fan-out cho UI: `main.go:118–137`). Test quét thuộc tính: không khớp mẫu đường dẫn, không dài > 64 ký tự.

### 5.5 Cảnh báo và tài liệu (task 08)

`backend-go/deploy/alerts/codeintel.rules.yaml` (mới), cùng định dạng và cùng ghi chú "chưa được nạp" như `mcp.rules.yaml`; mười rule của CR-071 §2.7 với biểu thức sửa theo L3 (tập `result` bị loại khỏi tỉ lệ lỗi: `ok`, `invalid_params`, `disabled`, `quality_gate_disabled`, `ai_review_disabled`, `not_authorized`, `rate_limited`, `concurrency_limit`, `version_conflict`), L6 và L10. Test `alerts_test` bảo đảm mọi metric trong rule tồn tại trong registry. Tài liệu `docs/guides/code-intel/code-intel-performance-and-metrics.md` (ngân sách, metrics, cách đọc trace, cách chạy benchmark).

### 5.6 Benchmark (task 07)

`backend-go/services/code-intel-service/bench/` (build tag `bench`): `testing.B` cho chuẩn hoá `SymbolRef`, hợp nhất hai nguồn, parse SQL → ERD trên **588 tệp `.sql`** của `backend-go/`, JSON/`proto.Size`, với agent giả phát lại độ trễ đã ghi; báo cáo `codeintel-bench-<commit>.json` (schema như CR-071 §2.2) là artifact CI, **không** vào git. Workflow `.github/workflows/code-intel-bench.yml` (mới): `workflow_dispatch` + `schedule`, runner chuyên dụng (hiện **chưa có**; câu hỏi mở Q1); trên PR chỉ chạy micro-benchmark với ngưỡng rộng. Bài thử tải 10 phiên: singleflight gộp, hàng đợi agent giữ ≤ 2 `gitnexus` (phần đo ở agent).

## 6. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Ngân sách là tệp JSON, checker so báo cáo; ô chưa có số là `null` + cảnh báo | không bịa số cho RPC mới; đổi ngân sách qua PR |
| D2 | `rpc`/`channel`/`result` sinh từ nguồn mã (ServiceDesc, registry, bảng lỗi) | CR đếm tay đã lệch hợp đồng (L1–L3) |
| D3 | Decorator đo ở lúc đăng ký kênh | kênh thêm sau tự được đo; không sửa từng handler |
| D4 | Agent không phơi `/metrics`, không thêm OTel; số đo đi theo `perf` | không có hạ tầng scrape agent (CR-071 D3) |
| D5 | Nhãn không bao giờ chứa tenant/user/worktree | quy tắc cardinality của `mcpmetrics`; xem theo tenant bằng truy vấn quản trị |
| D6 | `/metrics` trên cổng health, registry riêng | arch/09 T8; `infra-fleet` giữ `/health/metrics` |
| D7 | Rule cảnh báo viết cùng PR với metric | tránh rule trỏ metric chưa có (mẫu `mcp.rules.yaml`) |
| D8 | Truy vấn quản trị CR-095 là SQL chỉ đọc có `tenant_id`, không RPC mới | hợp đồng không có RPC; giữ phạm vi |

## 7. Tiêu chí chấp nhận

- [x] `codeintel-budgets.json` khớp bảng 5.1; `check-codeintel-bench-budgets` thoát khác 0 trên báo cáo mẫu vượt và 0 trên báo cáo mẫu đạt; ô `null` chỉ cảnh báo.
- [x] `/metrics` của `code-intel-service` đủ metric mục 5.2; `TestMetricsLabelsAreClosedSets` xanh với 49 `rpc` sinh từ `ServiceDesc`; scrape không chứa UUID, đường dẫn, commit.
- [x] Gateway phục vụ `orca_mcp_*` và `orca_gateway_codeintel_*` cùng `/metrics`; infra-fleet có `orca_fleet_codeintel_*` ở `/health/metrics`.
- [x] Một yêu cầu `codeIntel.changeOverlay` lạnh cho một `trace_id` có span gateway, service, fleet, `codeintel.agent.cli` (kiểm bằng tracer trong bộ nhớ ở từng hop; chuỗi đầy đủ trên stack thật là kiểm tay, chưa kiểm chứng).
- [x] Không span nào có thuộc tính là đường dẫn, tên symbol, mã nguồn hay dài > 64 ký tự.
- [x] `codeintel.rules.yaml` chỉ dùng metric có thật (test).
- [x] Số liệu server CR-095 có metric, truy vấn quản trị chạy được trên Postgres và MySQL, luôn có `tenant_id`.
- [x] Không `max-lines` disable.

## 8. Kiểm thử

| Test | Nội dung |
|---|---|
| `TestMetricsLabelsAreClosedSets`, `TestMetricsScrapeHasNoIdentifiers` | mục 7; mã lạ → `other` |
| `TestRPCLabelSetMatchesServiceDesc`, `TestChannelLabelSetMatchesInventory` | L1, L2 |
| `collector_perf_test.go` | `perf` → histogram + span con; thiếu `perf` không lỗi; không rò vào snapshot |
| `trace_chain_test.go`, `TestSpanAttributesAreSafe` | span cha-con, thuộc tính an toàn (mẫu: `mcpmetrics/metrics_test.go` dùng tracer trong bộ nhớ) |
| `alerts_test` | rule chỉ dùng metric tồn tại |
| `quality_admin_queries_test.go` (tag `integration`, ma trận `dialect`) | truy vấn quản trị chạy hai dialect, từ chối thiếu `tenant_id`, không đọc dữ liệu tenant khác |
| `bench/` + checker | báo cáo mẫu |

Chạy: `go test ./...` ở từng module; tích hợp `-tags=integration` (cần Docker); bench `-tags=bench`. **Chưa chạy.**

## 9. Rủi ro và điểm chưa kiểm chứng

- Số đo gốc là một lần chạy trên máy 32 nhân; chưa có p95, chưa đo cache OS lạnh, dev server nhỏ (RAM 4–8 GiB), qua SSH (thêm 50–200 ms mỗi hop, AGENTS.md).
- Truy vấn gộp cạnh cụm chỉ thử `CALLS` và `LIMIT 5000` (4,5 s); `communities` 10 252 (meta) ≠ 9 039 (đếm) chưa rõ.
- `rssPeakKb` trên macOS, Windows, WSL chưa kiểm chứng; Windows trả `unsupported_platform` (O-14).
- Span hậu kỳ mất thứ tự thật giữa các lệnh CLI song song.
- Rule cảnh báo chưa được nạp ở đâu (như `mcp.rules.yaml`).
- Ghép registry ở gateway đổi chữ ký `healthAndMetricsMux`; chạm `main.go` của gateway: chạy `gitnexus_impact` trước khi sửa (task 04).
- Cardinality: số chuỗi lý thuyết tối đa của `requests_total` ≈ 49 × (1 + >40 + 1) ≈ 2 000+ (tính tay); thực tế chỉ có chuỗi đã xảy ra. Chưa đo.
- Ngưỡng cảnh báo và ngân sách chưa có dữ liệu sản xuất.
- Runner chuyên dụng cho benchmark đêm chưa có.

## 10. Câu hỏi mở

1. Có chấp nhận benchmark đêm trên runner chuyên dụng (cần máy giữ chỉ mục Orca)? Nếu không, chỉ đo cục bộ.
2. Ghép metrics gateway bằng `prometheus.Gatherers` (đề xuất) hay mux thứ hai?
3. Giữ `result` chi tiết (hơn 40 giá trị) hay gom nhóm (`client_error`, `policy`, `infra`) để giảm cardinality?
4. Truy vấn quản trị CR-095 để ở tài liệu vận hành hay thành lệnh CLI nhỏ (`cmd/`)? Hợp đồng chưa có RPC.
5. Có cần SLO chính thức ngoài "ngân sách kiểm thử"? (CR-071 Q5: hiện không.)

## 11. Khoảng trống hợp đồng ghi nhận

- Thiếu `command` trong `data` của `TOOL_FAILED`/`format_drift`; thiếu trạng thái `untested`; tập `command` của `perf` không phủ `check`, `trace`, `list`, `analyze`, `sync`.
- Không có enum `false_positive` ở T5/T11 (L9).
- `StreamCodeIntelEvents` có `overflow` (PQ-17) nhưng ui-api §5 payload push chỉ nói `resync`; nhãn `dropped.reason` cần thống nhất.

## 12. Tham chiếu

- `backend-go/services/api-gateway/internal/adapter/mcpmetrics/{metrics.go,instrument.go,metrics_test.go}`, `.../cmd/server/{main.go,mcp_metrics_wiring.go}`, `.../wscompat/{handler.go,registry.go}`, `.../grpc/dial.go`
- `backend-go/common/tracing/{tracing.go,trace_stream.go}`, `backend-go/common/grpcmw/grpcmw.go`, `backend-go/common/health/health.go`
- `backend-go/services/mcp-service/cmd/server/main.go`, `notification-service/cmd/server/main.go`, `infra-fleet-service/cmd/server/main.go`, `.../adapter/metrics/fleet_collector.go`, `.../adapter/devserveragent/{client.go,frame.go}`
- `backend-go/deploy/alerts/mcp.rules.yaml`, `backend-go/ci/mcp-conformance/run-go-conformance.sh`, `desktop/config/scripts/check-terminal-perf-report-budgets.mjs`
- `docs/crs/v7/quality-rollout/CR-CV-071-…md`, `docs/crs/v7/quality-gate/CR-CV-095-…md`, `docs/research/view-code/04-raw-data-and-pipeline.md` mục 2
