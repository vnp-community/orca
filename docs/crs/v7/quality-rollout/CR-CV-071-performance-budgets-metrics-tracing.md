# CR-CV-071 — Ngân sách hiệu năng, metrics Prometheus, tracing và cảnh báo

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-071 |
| **Tên** | Ngân sách độ trễ p95 / kích thước payload / bộ nhớ agent cho từng view, benchmark trên repo cỡ Orca, metrics `orca_codeintel_*`, span OTel xuyên gateway → code-intel → infra-fleet → agent, quy tắc cảnh báo |
| **Loại** | Chất lượng / Vận hành |
| **Priority** | 🟠 P1 |
| **Effort** | Medium (5 đến 7 ngày: bộ benchmark, 3 gói metrics, span, khối `perf` của agent, rule cảnh báo) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-010 (service, cổng health), CR-CV-021 (collector), CR-CV-022 (cache), CR-CV-023 (infra-fleet), CR-CV-040 (kênh), CR-CV-001 (khung agent); CR-CV-070 (script chụp chế độ `--bench`) |
| **Mở khoá** | CR-CV-073 (điều kiện chuyển giai đoạn rollout) |
| **Tác động** | `backend-go/services/code-intel-service` (`internal/adapter/metrics`, span trong collector, `/metrics`), `backend-go/services/api-gateway` (`internal/adapter/wscompat`, `cmd/server/mcp_metrics_wiring.go` hoặc gói metrics mới), `backend-go/services/infra-fleet-service` (span + vài bộ đếm), `agent/` (khối `perf`, hàng đợi lệnh CLI), `backend-go/deploy/alerts/codeintel.rules.yaml` (mới), `docs/guides/` (một trang vận hành) |

---

## 1. Bối cảnh và vấn đề

1. README v7 mục 7 ghi rủi ro: "CLI ~1,8 s mỗi lần, DB CodeGraph ~1,2 GB, đồ thị lớn trong UI" và "chi phí truy vấn trên 247k node/644k cạnh chưa đo". Chưa có con số nào để quyết định timeout (CR-CV-040 đề xuất 20 s), đồng thời hạn mức đồng thời trên dev server.
2. **Đo thật ngày 2026-10-05** (chỉ-đọc, repo Orca, máy khảo sát 32 nhân, 32 GiB RAM, mỗi lệnh chạy một lần, tức là mẫu một điểm, không phải phân phối):

| Lệnh | Thời gian | RSS đỉnh | Đầu ra |
|---|---|---|---|
| `gitnexus --version` | 0,07 s | 61 MB | 6 B |
| `gitnexus cypher -r orca` top 500 cụm (`MATCH (c:Community) ... ORDER BY c.symbolCount DESC LIMIT 500`) | 1,62 s | 668 MB | 25 KB |
| `gitnexus cypher` đếm 180 460 cạnh `CALLS` | 1,68 s | 692 MB | rất nhỏ |
| `gitnexus cypher` **gộp cạnh `CALLS` giữa các cụm**, `LIMIT 5000` (mẫu ở nghiên cứu 04 §2, cú pháp chạy được trên LadybugDB) | 4,50 s | 742 MB | 154 KB markdown (5 000 hàng) |
| `gitnexus context -r orca handleInvoke` | 1,75 s | 728 MB | 5 KB |
| `gitnexus impact -r orca handleInvoke` | 1,82 s | 830 MB | 1 KB |
| `codegraph status --json` | 0,51 s | 287 MB | 891 B |
| `codegraph query handleInvoke --json` | 0,40 s | 373 MB | 13,7 KB |

   Kết luận sơ bộ có bằng chứng: (a) chi phí chính của GitNexus là khởi động tiến trình (~1,6 s) chứ không phải truy vấn; (b) **mỗi tiến trình `gitnexus` chiếm 0,6 đến 0,85 GB RSS**, nên 10 truy vấn đồng thời trên một dev server là ~8 GB: hạn mức đồng thời là yêu cầu an toàn, không phải tối ưu; (c) gộp cạnh cụm trên Orca khả thi (4,5 s) nhưng đã sát ngân sách "view lạnh ≤ 12 s" nếu cần nhiều truy vấn kế nhau; (d) CodeGraph nhanh hơn ~4 lần và nhẹ hơn một nửa. Cần đo lại có phân phối (p50/p95) ở benchmark của CR này.
3. Mẫu metrics trong repo đã đọc:
   - `api-gateway/internal/adapter/mcpmetrics/metrics.go`: registry **riêng** (`prometheus.NewRegistry()`), tiền tố `orca_mcp_*`, quy tắc cardinality ở đầu file ("every label value comes from a closed set ... Tenant, user, session, client and trace ids are never labels"), mount bằng `healthAndMetricsMux` ở `/metrics` trên cổng health (`cmd/server/mcp_metrics_wiring.go:39-44`).
   - `mcp-service` và `notification-service` mount `/metrics` cùng cách (`cmd/server/main.go`); `infra-fleet-service` mount `/health/metrics` với registry riêng (`cmd/server/main.go:784`, `adapter/metrics/fleet_collector.go`, tiền tố `orca_fleet_*`). `issue-status-sync` **không có** metrics (kiểm lại 2026-10-05).
   - Rule cảnh báo mẫu: `backend-go/deploy/alerts/mcp.rules.yaml`, ghi chú đầu file: "Not loaded by anything in this repository ... an operator has to mount it". Cảnh báo của CR này sẽ cùng tình trạng cho tới khi có hạ tầng nạp (chưa xác minh nơi nạp, như v5).
4. Tracing đã có (đọc code): `common/tracing.Init(ctx, serviceName, otlpEndpoint, opts...)` đặt `TracerProvider` và `otel.SetTextMapPropagator(propagation.TraceContext{})` (`tracing.go:55-89`); thêm bộ xử lý đẩy mỗi span lên JetStream `TRACE` (`orca.trace.<service>.span`, `trace_stream.go`) mà `api-gateway` fan-out cho UI (`cmd/server/main.go:124-137`). `gatewaygrpc.Dial` gắn `otelgrpc.NewClientHandler()` (`dial.go:32`) và các service gắn `grpcmw.StatsHandler()` ở máy chủ (`common/grpcmw/grpcmw.go:140-142`). Khoảng trống đã kiểm chứng:
   - Kênh WS của gateway **không tạo span**: không có `tracer.Start`/`otelhttp` ở `wscompat` hay `httpgateway` ngoài test (grep). Span gốc hiện là span client gRPC đầu tiên.
   - **Agent không có OTel và không có truyền `traceparent`** qua JSON-RPC: `agent/package.json` không có `@opentelemetry/*`; grep `traceparent` chỉ thấy `common/tracing` và một test (`task-service/.../dial_test.go`). Chuỗi trace đứt ở ranh giới infra-fleet → agent.
   - Vì span được đẩy lên `TRACE` và fan-out cho UI, **thuộc tính span không được chứa mã nguồn, đường dẫn, tên symbol hay id tenant dạng chuỗi tự do** (mục 2.6).
5. Dữ liệu từ `codeintel.*` được cắt ở agent trước khi đi (README v7 mục 6), nhưng chưa có số đo nào ghi lại kích thước thật qua các hop, hay tần suất cắt, trong khi khung RPC trần 16 MiB (`devserveragent/frame.go:24`), gRPC nhận 4 MiB mặc định ở gateway (`dial.go`, xem CR-CV-040) và UI không nhận quá ~1 500 nút một lần (nghiên cứu 04 §2).

## 2. Giải pháp đề xuất

### 2.1 Ngân sách (đề xuất ban đầu, **chưa có số đo phân phối**; hiệu chỉnh sau benchmark 2.2)

Các ngân sách là hợp đồng kiểm thử: benchmark đỏ khi vượt, dù chưa thành SLO sản phẩm.

**Độ trễ p95 theo view** (đo từ lúc service nhận RPC tới lúc trả; "ấm" = trúng cache snapshot của service; "lạnh" = phải gọi agent; trên dev server cùng mạng với backend, **chưa tính** 50 đến 200 ms của SSH, theo README v7 mục 6):

| View / RPC | Ấm p95 | Lạnh p95 | Ghi chú |
|---|---|---|---|
| `GetIndexStatus` | ≤ 0,3 s | ≤ 3 s | 1 gọi `codeintel.status` (≈ `codegraph status` 0,5 s + đọc `meta.json`) |
| `GetChangeOverlay` | ≤ 0,5 s | ≤ 10 s | `detect-changes` + `impact` vài symbol + diff `git.*`; đường tới hạn của MVP |
| `GetReadingOrder` | ≤ 0,3 s | ≤ 2 s | tính trên overlay đã có |
| `GetImpact` | ≤ 0,3 s | ≤ 5 s | 1 lệnh `impact` (~1,8 s) |
| `GetSymbol` | ≤ 0,3 s | ≤ 4 s | `context --content` (~1,8 s) |
| `GetSubgraph` | ≤ 0,5 s | ≤ 8 s | |
| `GetArchitecture` (cụm, gộp cạnh) | ≤ 0,5 s | ≤ 12 s | truy vấn gộp 4,5 s đo được, cộng chuẩn hoá |
| `GetStructure` | ≤ 0,5 s | ≤ 8 s | |
| `ListDataFlows` / `GetDataFlow` | ≤ 0,3 s | ≤ 6 s | |
| `GetErd` | ≤ 0,5 s | ≤ 6 s | đọc file SQL qua `fs.*` + parse tại service |
| `GetRouteMap` | ≤ 0,3 s | ≤ 6 s | |
| `ListFindings` | ≤ 0,3 s | ≤ 8 s | |
| `GetReviewState`, `GetC4Overrides` | ≤ 0,1 s | không có (chỉ DB) | |
| `Save*` / `Dismiss*` / `Bind*` | ≤ 0,2 s | không có | chỉ DB |
| `RequestReindex` (nhận job) | ≤ 0,5 s | không có | thời gian chạy `analyze` ngoài ngân sách này (mục 2.5) |

Mọi giá trị "lạnh" nhỏ hơn timeout kênh 20 s của CR-CV-040 và 25 s `invokeTimeout` của WS (`handler.go:247`). Hệ quả nếu vượt: UI nhận `CODEINTEL_TIMEOUT`, service vẫn hoàn tất việc nền để lần sau trúng cache.

**Kích thước payload** (đo tại điểm nêu; bám ngân sách README v7 3.2 và nghiên cứu 04 §2):

| Điểm đo | Ngân sách |
|---|---|
| Kết quả `codeintel.overview` (agent → infra-fleet) | ≤ 512 KiB (≤ 500 cụm, ≤ 5 000 cạnh; mẫu đo: 5 000 hàng markdown = 154 KB, JSON dự kiến cùng bậc, **chưa đo**) |
| `codeintel.subgraph` | ≤ 1 MiB (≤ 1 500 nút, ≤ 4 000 cạnh) |
| `codeintel.symbol` | ≤ 200 KiB mã nguồn (README 3.2) |
| `codeintel.impact`, `detectChanges` | ≤ 512 KiB (≤ 300 nút; ≤ 2 000 symbol) |
| Mọi `codeintel.*` | ≤ 8 MiB stdout đọc vào bộ nhớ agent, thấp hơn trần khung 16 MiB (nghiên cứu 02 §4) |
| Phản hồi gateway → UI | ≤ 2 MiB (cap của CR-CV-040), `symbol` ≤ 320 KiB |
| Một dòng `graph_snapshots.payload` | ≤ 2 MiB (`payload_bytes`, README 3.5) |
| Khung push `codeIntel.changed`/`reindexProgress` | ≤ 1 KiB |

**Bộ nhớ và đồng thời trên dev server (agent):**

| Hạng mục | Ngân sách | Cơ sở |
|---|---|---|
| Số tiến trình CLI GitNexus chạy đồng thời / dev server | ≤ 2 (hàng đợi, phần còn lại chờ) | 0,6 đến 0,85 GB mỗi tiến trình (bảng mục 1.2) → ≤ ~1,7 GB đỉnh |
| Số tiến trình CodeGraph đồng thời | ≤ 3 | 0,29 đến 0,37 GB mỗi tiến trình |
| Tổng RSS đỉnh của cây tiến trình `codeintel` / dev server | ≤ 2 GiB | giữ chỗ cho dự án chính của người dùng trên cùng máy |
| Bộ nhớ heap thêm của tiến trình agent do `codeintel.*` | ≤ 128 MiB | stdout ≤ 8 MiB × 2 đồng thời + parse |
| `codeintel.reindex` | một job một lúc (README 3.2), không chạy song song truy vấn nặng của cùng repo | `analyze` mất nhiều phút; có thể khoá DB (`lbug.wal.missing-shadow.*` đã thấy, nghiên cứu 06 §2) |
| Thời gian chờ hàng đợi tối đa | ≤ 5 s rồi `CODEINTEL_TIMEOUT` hoặc `CODEINTEL_RATE_LIMITED` | không để truy vấn UI treo vô hạn |

Các con số là đề xuất; hạn mức đồng thời đặt ở cấu hình agent (CR-CV-001) và hạn mức đồng thời của service (CR-CV-013). Chưa kiểm chứng máy dev thật có đủ RAM.

**Bộ nhớ và dung lượng ở service:**

| Hạng mục | Ngân sách |
|---|---|
| Heap `code-intel-service` | ≤ 512 MiB ở tải mục tiêu (10 người xem đồng thời, đề xuất) |
| Bộ nhớ cache snapshot theo tenant (dòng `graph_snapshots`) | ≤ 256 MiB/tenant, thu hồi theo `expires_at`/LRU (CR-CV-022) |
| Kênh push | ≤ 500 stream/replica gateway (CR-CV-040) |

### 2.2 Benchmark trên repo cỡ Orca

| Việc | Cách làm |
|---|---|
| Đối tượng | Một **commit Orca cố định** (ghi trong báo cáo), chỉ mục dựng sẵn bằng `gitnexus analyze` và `codegraph index` trên **máy benchmark chuyên dụng** (không phải máy người dùng); ghi `meta.json.stats` và `codegraph status --json` vào báo cáo. Đây là nơi duy nhất `analyze` chạy trên repo lớn, do người vận hành chủ động |
| Agent (TS) | `agent/scripts/bench-codeintel.mjs` (mới): gọi từng method `codeintel.*` qua handler thật (không qua mạng), 30 lần lạnh (xoá cache OS không bắt buộc, ghi nhận) và 100 lần ấm, đo thời gian, kích thước kết quả, RSS đỉnh của cây tiến trình (đọc `/proc` hoặc `ps`), số lần cắt `truncated` |
| Service (Go) | `backend-go/services/code-intel-service/bench/` (mới, build tag `bench`): chạy collector với agent giả phát lại kết quả đã đo (độ trễ ghi nhận) để đo chi phí chuẩn hoá, cache, JSON; benchmark `testing.B` cho chuẩn hoá `SymbolRef`, hợp nhất hai nguồn (CR-CV-020), parse SQL → ERD (CR-CV-031) trên toàn bộ 588 tệp `.sql` hiện có (README v7 mục 1) |
| Đầu cuối | Kịch bản `tests/` (Python, theo `tests/mcp/mcp_check_framework.py`) mở WS thật, gọi kênh, đo p50/p95/p99 mỗi view lạnh và ấm, kích thước khung nhận |
| Báo cáo | JSON `codeintel-bench-<commit>.json`: `{commit, tools:{gitnexus,codegraph versions}, host:{cores, ramGiB}, views:[{rpc, cold:{p50,p95,p99,n}, warm:{...}, bytes:{p50,p95,max}, truncatedRate, agentRssPeakMiB}]}`. Lưu làm artifact CI, **không** vào git |
| So sánh | Script `check-codeintel-bench-budgets.mjs` (mẫu `config/scripts/check-terminal-perf-report-budgets.mjs` có trong `package.json`) so báo cáo với ngân sách 2.1 (đặt thành tệp dữ liệu `codeintel-budgets.json`); vượt ngân sách thì thoát khác 0 |
| Khi nào chạy | **Hằng đêm** và `workflow_dispatch` trên runner chuyên dụng (không chặn PR, vì phụ thuộc phần cứng); trên PR chỉ chạy micro-benchmark Go và đo trên repo mẫu (CR-CV-070) với ngưỡng rộng để bắt hồi quy khổng lồ |
| Bài thử tải | 10 phiên đồng thời mở `changeOverlay` + `impact` cùng một worktree (singleflight phải gộp), rồi 10 worktree khác nhau (hàng đợi agent phải giữ ≤ 2 `gitnexus`); ghi RSS dev server, số `CODEINTEL_TIMEOUT`, độ trễ |

Kết quả đầu tiên điền vào bảng 2.1 (cột "đo được") trong PR đầu; ngân sách được hiệu chỉnh một lần theo số đo, sau đó chỉ đổi qua PR có lý do.

### 2.3 Metrics `code-intel-service` (mới)

Gói `internal/adapter/metrics` (mẫu `mcp-service/internal/adapter/metrics/session_gauge.go`, `api-gateway/.../mcpmetrics/metrics.go`): registry riêng, mount `/metrics` trên cổng health như `mcp-service` (`healthAndMetricsMux(healthSrv.Handler(), metrics.Handler())`), **không** theo `/health/metrics` của infra-fleet.

Nhãn chỉ lấy từ tập đóng; `rpc` là 25 tên RPC ở README 3.6; `result` ∈ `ok`, các mã `CODEINTEL_*` viết thường (`tool_unavailable`, `index_missing`, `repo_not_registered`, `path_not_allowed`, `invalid_params`, `ambiguous_symbol`, `timeout`, `reindex_in_progress`, `output_too_large`, `tool_failed`, `disabled`, `forbidden`, `rate_limited`, `internal`), tên không có trong tập thì gộp `other`. **Không bao giờ** nhãn: tenant, user, worktree, project, đường dẫn, tên symbol, `jobId`, `commit`.

| Metric | Loại | Nhãn | Ý nghĩa |
|---|---|---|---|
| `orca_codeintel_requests_total` | counter | `rpc`, `result` | RPC theo kết quả |
| `orca_codeintel_request_duration_seconds` | histogram (0,01 đến 30 s) | `rpc`, `cache` ∈ `hit`,`miss`,`stale_served`,`shared` | độ trễ; `shared` = nhờ singleflight |
| `orca_codeintel_cache_events_total` | counter | `event` ∈ `hit`,`miss`,`evict`,`invalidate`,`expire` | hiệu quả cache (CR-CV-022) |
| `orca_codeintel_cache_bytes` | gauge | không | tổng `payload_bytes` (lấy mẫu định kỳ như `sessions_active`) |
| `orca_codeintel_agent_calls_total` | counter | `method` (10 method `codeintel.*`), `result` | gọi agent qua `RelayByDevServer` |
| `orca_codeintel_agent_call_duration_seconds` | histogram | `method` | độ trễ phía service, gồm mạng |
| `orca_codeintel_agent_payload_bytes` | histogram (1 KiB đến 16 MiB) | `method` | kích thước kết quả agent |
| `orca_codeintel_response_truncated_total` | counter | `method` | số lần agent trả `truncated:true` |
| `orca_codeintel_agent_queue_wait_seconds` | histogram | `tool` ∈ `gitnexus`,`codegraph` | từ khối `perf` của agent (2.5) |
| `orca_codeintel_agent_cli_seconds` | histogram | `tool`, `command` (hằng số whitelist) | thời gian CLI thật, từ `perf` |
| `orca_codeintel_agent_rss_peak_bytes` | histogram | `tool` | RSS đỉnh quan sát, từ `perf` |
| `orca_codeintel_tool_format_drift_total` | counter | `tool`, `command` | trôi định dạng (CR-CV-070 2.6) |
| `orca_codeintel_tool_compat` | gauge | `tool`, `state` ∈ `verified`,`untested`,`incompatible` | số dev server theo trạng thái |
| `orca_codeintel_tool_info` | gauge (=1) | `tool`, `version` (tối đa 8 giá trị, còn lại `other`) | phiên bản đang thấy |
| `orca_codeintel_reindex_jobs` | gauge | `status` ∈ `queued`,`running`,`failed` | job đang có (lấy mẫu từ `reindex_jobs`) |
| `orca_codeintel_reindex_duration_seconds` | histogram (10 s đến 3 600 s) | `mode`, `result` | thời gian chạy `analyze`/`sync` |
| `orca_codeintel_stale_responses_total` | counter | `view` | số kết quả trả có `stale:true` |
| `orca_codeintel_events_total` | counter | `event` ∈ `index_changed`,`reindex_progress`, `result` ∈ `delivered`,`dropped` | luồng sự kiện (CR-CV-024) |
| `orca_codeintel_outbox_pending` | gauge | không | độ trễ outbox (theo `common/outbox`) |
| `orca_codeintel_disabled_rejections_total` | counter | `rpc` | RPC bị từ chối vì cờ tắt (CR-CV-073) |
| `orca_codeintel_concurrency_rejections_total` | counter | `scope` ∈ `tenant`,`dev_server` | từ chối do hạn mức đồng thời (CR-CV-013) |

Cardinality tối đa: `rpc` (25) × `result` (15) ≈ 375 chuỗi cho counter chính; có test cardinality (`TestMetricsLabelsAreClosedSets`) khẳng định không nhãn nào nhận giá trị ngoài tập (kể cả khi lỗi mới xuất hiện).

### 2.4 Metrics ở gateway và infra-fleet

**Gateway** (gói mới `internal/adapter/wscompat` hoặc `codeintelmetrics`; cần chốt cách ghép vì `healthAndMetricsMux` hiện chỉ nhận một `*mcpmetrics.Metrics`, `mcp_metrics_wiring.go:39`): ghép hai registry bằng `prometheus.Gatherers` hoặc truyền registry thứ hai vào mux; **chưa kiểm chứng** `mcpmetrics.Metrics` có lộ `Gatherer` hay không.

| Metric | Nhãn |
|---|---|
| `orca_gateway_codeintel_channel_total` | `channel` (26 kênh), `result` (cùng tập đóng) |
| `orca_gateway_codeintel_channel_duration_seconds` | `channel` |
| `orca_gateway_codeintel_response_bytes` | `channel` (histogram, theo `proto.Size`, CR-CV-040) |
| `orca_gateway_codeintel_streams_active` | không (gauge) |
| `orca_gateway_codeintel_events_dropped_total` | `reason` ∈ `closed`,`resync` |

**infra-fleet** (registry riêng `fleet`, cùng `/health/metrics` hiện có, tiền tố `orca_fleet_*`): `orca_fleet_codeintel_notifications_total{type}` (`indexChanged`, `reindexProgress`), `orca_fleet_codeintel_notifications_dropped_total{reason}`, `orca_fleet_codeintel_relay_seconds` (histogram, chỉ cho các method `codeintel.*`), `orca_fleet_codeintel_streams_active`. Sửa `execTimeoutForMethod` cho `codeintel.*` là việc của CR-CV-023, không lặp ở đây.

**Agent** không tự phơi `/metrics` (không có hạ tầng Prometheus phía agent; quyết định D3); số đo đi theo khối `perf` trong kết quả (2.5).

### 2.5 Khối `perf` trong kết quả agent (mới)

Mỗi kết quả `codeintel.*` có thêm trường tuỳ chọn ngoài `data` (không phá hợp đồng README 3.2 vì là trường bổ sung, theo "Điều chỉnh hợp đồng"):

```jsonc
"perf": { "totalMs": 1840, "queueWaitMs": 12, "cliCalls": 1,
          "cli": [{ "tool": "gitnexus", "command": "impact", "ms": 1790, "stdoutBytes": 1019, "rssPeakKb": 830232 }],
          "parseMs": 6, "truncated": false }
```

- `command` chỉ nhận hằng số whitelist; không chứa `args`, tên symbol, đường dẫn.
- `rssPeakKb` lấy từ `wait4`/`/proc/<pid>/status` khi tiến trình thoát (nền tảng không hỗ trợ thì bỏ trường; Windows/WSL chưa kiểm chứng).
- Service đọc khối này, ghi vào histogram 2.3 và tạo span con (2.6), rồi **không** lưu vào cache snapshot, **không** trả ra gateway.

### 2.6 Tracing xuyên các hop

Chuỗi mong muốn (mỗi mục là một span):

```
[gateway] codeintel.channel <kênh>          (mới: bao quanh handler, span gốc của yêu cầu WS)
  └ [gateway→service] gRPC client (đã có: otelgrpc ở Dial)
      └ [service] gRPC server (đã có: grpcmw.StatsHandler)
          └ [service] codeintel.collect {view, cache, agent_calls}      (mới)
              ├ [service] codeintel.cache.lookup                          (mới)
              └ [service→fleet] gRPC client RelayByDevServer (cần otelgrpc ở dial của service, theo mẫu project-service/grpcclient/dev_server_relay.go)
                  └ [fleet] gRPC server (đã có)
                      └ [fleet] devserveragent.call {method}               (mới)
                          └ [agent] (không có span thật) ──▶ span con tổng hợp từ khối `perf`:
                                codeintel.agent.queue, codeintel.agent.cli {tool, command}, codeintel.agent.parse
```

| Điểm | Quyết định |
|---|---|
| Span gốc ở gateway | bọc mỗi handler `codeIntel.*` bằng `otel.Tracer("orca/api-gateway/codeintel")` (cùng kiểu `mcpmetrics/instrument.go:22`), vì `/ws` không có span. Thuộc tính: `codeintel.channel`, `codeintel.result` |
| Qua gRPC | gRPC client/server đã propagate `traceparent` (`propagation.TraceContext`); CR-CV-010 phải đặt `otelgrpc` ở máy chủ và ở dial tới infra-fleet |
| Qua agent | **Không** thêm OTel vào agent ở v7 (không có phụ thuộc, tăng kích thước và bề mặt). Service dựng span con **hậu kỳ** từ `perf` bằng `trace.WithTimestamp` (thời điểm bắt đầu suy ra từ `end - ms`). Phương án thay thế (P2): truyền `_trace.traceparent` trong `params_json` để agent ghi log kèm trace id; chưa làm vì `codeintel.*` không nhận tham số ngoài danh sách (D5 README) |
| Thuộc tính span | chỉ giá trị từ tập đóng hoặc số: `view`, `rpc`, `cache`, `tool`, `command`, `bytes`, `truncated`, `result`, `stale`. **Cấm**: đường dẫn, tên symbol, tên repo, nội dung mã nguồn, `worktreeId` dạng đầy đủ, tên tenant. Lý do: span được đẩy lên JetStream `TRACE` và fan-out cho UI (`cmd/server/main.go:124-137`); test khẳng định không thuộc tính nào khớp mẫu đường dẫn hoặc dài > 64 ký tự |
| Mẫu | dùng cấu hình chung của `common/tracing`; không tăng tỷ lệ lấy mẫu riêng |

Tiêu chí: một yêu cầu `codeIntel.changeOverlay` lạnh cho **một trace id** chứa đủ span gateway, service, fleet, và ít nhất một span `codeintel.agent.cli`.

### 2.7 Cảnh báo (`backend-go/deploy/alerts/codeintel.rules.yaml`, mới)

Cùng định dạng và cùng ghi chú "chưa được nạp" như `mcp.rules.yaml`. Chỉ dùng metrics ở 2.3 và 2.4 (đều do CR này tạo ra, nên viết rule **cùng PR** với metrics).

| Cảnh báo | Biểu thức (rút gọn) | Mức | Hành động |
|---|---|---|---|
| `CodeIntelLatencyOverBudget` | `histogram_quantile(0.95, sum by (le,rpc)(rate(orca_codeintel_request_duration_seconds_bucket{cache="miss"}[15m]))) > 12` trong 20 phút | warning | xem agent queue, RSS, phiên bản công cụ |
| `CodeIntelErrorRate` | tỉ lệ `result` ∉ {`ok`,`invalid_params`,`disabled`,`forbidden`,`rate_limited`} > 10 % trong 15 phút | warning | runbook CR-CV-073 |
| `CodeIntelToolFormatDrift` | `increase(orca_codeintel_tool_format_drift_total[1h]) > 0` | warning | công cụ trôi định dạng: chụp lại fixture (CR-CV-070) |
| `CodeIntelToolIncompatible` | `orca_codeintel_tool_compat{state="incompatible"} > 0` trong 10 phút | warning | nâng `SUPPORTED_*` hoặc hạ phiên bản công cụ |
| `CodeIntelReindexSlow` | `histogram_quantile(0.95, sum by (le)(rate(orca_codeintel_reindex_duration_seconds_bucket[6h]))) > 1800` | info | `analyze` quá 30 phút |
| `CodeIntelReindexStuck` | `orca_codeintel_reindex_jobs{status="running"} > 0` liên tục quá 90 phút (rule `for`) | warning | job kẹt (khoá DB) |
| `CodeIntelCacheNearCap` | `orca_codeintel_cache_bytes` > 90 % cấu hình | info | tăng dung lượng hoặc giảm TTL |
| `CodeIntelOutboxLag` | `orca_codeintel_outbox_pending > 100` trong 10 phút | warning | NATS hoặc consumer |
| `CodeIntelResponseTruncatedHigh` | tỉ lệ `response_truncated_total / agent_calls_total` > 30 % trong 1 giờ | info | ngân sách kích thước quá chặt hoặc repo bất thường |
| `CodeIntelGatewayStreamsSaturated` | `orca_gateway_codeintel_streams_active` > 80 % `CODE_INTEL_MAX_STREAMS` | warning | tăng giới hạn hoặc replica |

Ngưỡng là đề xuất, chưa có dữ liệu để hiệu chỉnh; chỉnh sau benchmark 2.2 và sau giai đoạn dogfood (CR-CV-073).

### 2.8 Tài liệu

`docs/guides/` (một trang vận hành, đặt tên theo nội dung, ví dụ `code-intel-performance-and-metrics.md`): bảng ngân sách, danh sách metrics, cách đọc trace, cách chạy benchmark. Cập nhật `README.md` của `code-intel-service`.

## 3. Quyết định thiết kế

| # | Quyết định | Lý do |
|---|---|---|
| D1 | Ngân sách là tệp dữ liệu (`codeintel-budgets.json`), benchmark so với nó | Đổi ngân sách phải qua PR, có lý do; không rải số trong test |
| D2 | Benchmark nặng chạy hằng đêm trên runner chuyên dụng, không chặn PR | Phụ thuộc phần cứng; PR chỉ bắt hồi quy khổng lồ |
| D3 | Agent không phơi metrics, không thêm OTel; số đo đi qua khối `perf` | Không có hạ tầng scrape phía agent; tránh phụ thuộc mới; một điểm ghi (service) |
| D4 | Span tổng hợp hậu kỳ cho phần agent | Có đủ trace liền mạch mà không đổi hợp đồng `codeintel.*` |
| D5 | Nhãn chỉ từ tập đóng, không nhãn tenant/user/worktree | Quy tắc cardinality của `mcpmetrics`; tenant xem qua audit, không qua metrics |
| D6 | `/metrics` trên cổng health như `mcp-service` | `/health/metrics` của infra-fleet là ngoại lệ không nên nhân rộng |
| D7 | Hạn mức đồng thời CLI là yêu cầu an toàn, đặt từ số đo RSS | 10 `gitnexus` đồng thời ~8 GB |
| D8 | Thuộc tính span cấm đường dẫn, symbol, mã nguồn | Span fan-out cho UI qua TRACE |
| D9 | Rule cảnh báo viết cùng PR với metrics | Tránh rule tham chiếu metric chưa tồn tại (mẫu `mcp.rules.yaml`: "only metrics that exist today") |

## 4. Tiêu chí chấp nhận

- [ ] Bảng ngân sách 2.1 nằm trong `codeintel-budgets.json`; báo cáo benchmark đầu tiên trên một commit Orca cố định có cột p50/p95/p99, kích thước, RSS đỉnh, tỉ lệ cắt, và được ghi vào PR.
- [ ] Benchmark đêm chạy được qua `workflow_dispatch`; vượt ngân sách thì script so sánh thoát khác 0 (kiểm bằng một bản báo cáo cố ý vượt).
- [ ] Agent giữ ≤ 2 tiến trình `gitnexus` và ≤ 3 `codegraph` đồng thời; bài thử tải 10 phiên không đưa RSS cây tiến trình quá 2 GiB; vượt hàng đợi 5 s thì trả `CODEINTEL_TIMEOUT` hoặc `CODEINTEL_RATE_LIMITED`.
- [ ] Kết quả `codeintel.*` có `perf`; service ghi vào histogram và không lưu `perf` vào cache hay trả ra gateway.
- [ ] `/metrics` của `code-intel-service` trả đủ metric ở 2.3; `TestMetricsLabelsAreClosedSets` xanh; scrape không chứa tenant, user, worktree, đường dẫn.
- [ ] Gateway và infra-fleet có metric ở 2.4; `healthAndMetricsMux` của gateway phục vụ cả `orca_mcp_*` lẫn `orca_gateway_codeintel_*`.
- [ ] Một yêu cầu `codeIntel.changeOverlay` lạnh cho một trace id có span gateway, service, fleet và `codeintel.agent.cli`; không span nào mang thuộc tính là đường dẫn, tên symbol hay mã nguồn (test quét thuộc tính).
- [ ] `codeintel.rules.yaml` tồn tại, chỉ dùng metric có thật (test `promtool check rules` hoặc parser YAML + so tên với registry), ghi chú "chưa nạp" nếu hạ tầng chưa có.
- [ ] Trang vận hành ở `docs/guides/` có bảng ngân sách, metrics và cách đọc trace.
- [ ] Không thêm `max-lines` disable (AGENTS.md).

## 5. Kiểm thử

| Test | Nội dung |
|---|---|
| `TestMetricsLabelsAreClosedSets` | giá trị lạ (mã lỗi mới, `rpc` giả) rơi vào `other`, không sinh chuỗi mới |
| `TestMetricsScrapeHasNoIdentifiers` | quét `/metrics` sau tải giả: không khớp UUID, đường dẫn, `commit` |
| `collector_perf_test.go` | khối `perf` → histogram và span con; thiếu `perf` không lỗi |
| `agent bench` (vitest + script) | hàng đợi giữ giới hạn đồng thời; RSS đo được; `perf` đúng định dạng |
| `trace_chain_test.go` | tracer trong bộ nhớ: đủ span, cha con đúng, thuộc tính an toàn |
| `check-codeintel-bench-budgets` | báo cáo mẫu vượt/không vượt |
| `alerts_test` | rule chỉ tham chiếu metric tồn tại |

Chưa chạy: toàn bộ danh sách trên là kế hoạch; các con số ở 1.2 là một mẫu đơn lẻ.

## 6. Rủi ro và điểm chưa kiểm chứng

- Số đo ở 1.2 là **một lần chạy**, trên máy 32 nhân; chưa có p95, chưa đo lúc cache OS lạnh, chưa đo trên dev server thật nhỏ hơn (RAM 4 đến 8 GiB), chưa đo qua SSH.
- Truy vấn gộp cạnh chỉ thử với `CALLS` (không `IMPORTS`) và `LIMIT 5000`; kết hợp nhiều loại cạnh hoặc không `LIMIT` có thể chậm hơn nhiều. Cú pháp này nay đã chạy được trên LadybugDB (khác README v7 mục 7 vốn ghi "chưa kiểm chứng"), nhưng chưa kiểm số `communities` 10 252 so với 9 039.
- Chưa biết chi phí ghi `perf` (đo RSS qua `/proc`) trên macOS, Windows, WSL; có thể bỏ trường.
- `healthAndMetricsMux` của gateway chưa cho ghép nhiều registry; cần đọc `mcpmetrics.Metrics.Handler()` trước khi chốt cách (chưa kiểm chứng).
- Span tổng hợp hậu kỳ phụ thuộc đồng hồ của agent chỉ qua độ dài (`ms`), không qua mốc thời gian tuyệt đối, nên tránh lệch đồng hồ; nhưng thứ tự thật giữa các lệnh CLI song song bị mất.
- Rule cảnh báo chưa được nạp ở đâu (như `mcp.rules.yaml`); metric đúng nhưng không ai nhận cảnh báo cho tới khi có hạ tầng.
- Ngưỡng cảnh báo và ngân sách là suy đoán có căn cứ nhưng chưa có dữ liệu sản xuất.
- `issue-status-sync` chưa có `/metrics`; nếu CR-CV-024 dùng nó làm đường sự kiện thì sẽ không có số đo (đã kiểm: không dùng ở v7).
- SSH và remote (AGENTS.md): mọi độ trễ ở 2.1 chưa tính 50 đến 200 ms mỗi hop; UI phải hiển thị trạng thái chờ.

## 7. Câu hỏi mở

1. Có chấp nhận benchmark hằng đêm trên runner chuyên dụng (cần một máy lưu chỉ mục Orca)? Nếu không, chỉ còn đo cục bộ do người vận hành chạy.
2. Giới hạn đồng thời `gitnexus` ≤ 2 có quá chặt khi nhiều người xem cùng một dev server? Cần số đo hàng đợi ở dogfood.
3. Có dùng phiên MCP stdio giữ chung (phương án B, nghiên cứu 02) để bỏ chi phí khởi động 1,6 s và giảm RSS từng lệnh? Sẽ đổi hẳn ngân sách; ngoài phạm vi v7 MVP.
4. Gateway ghép metrics bằng `prometheus.Gatherers` hay mux thứ hai?
5. Có cần SLO chính thức ngoài "ngân sách kiểm thử"? Hiện không.

## 8. Tham chiếu

- `backend-go/services/api-gateway/internal/adapter/mcpmetrics/metrics.go`, `backend-go/services/api-gateway/internal/adapter/mcpmetrics/instrument.go`, `backend-go/services/api-gateway/cmd/server/mcp_metrics_wiring.go`, `backend-go/services/api-gateway/cmd/server/main.go` (dòng 118-137 TRACE), `backend-go/services/api-gateway/internal/adapter/grpc/dial.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/handler.go`
- `backend-go/services/mcp-service/internal/adapter/metrics/session_gauge.go`, `backend-go/services/mcp-service/cmd/server/main.go`, `backend-go/services/notification-service/internal/adapter/metrics/push_metrics.go`, `backend-go/services/infra-fleet-service/internal/adapter/metrics/fleet_collector.go`, `backend-go/services/infra-fleet-service/cmd/server/main.go` (dòng 679, 779-784), `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/frame.go`, `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go`
- `backend-go/common/tracing/tracing.go`, `backend-go/common/tracing/trace_stream.go`, `backend-go/common/grpcmw/grpcmw.go`, `backend-go/common/health/health.go`, `backend-go/common/config/config.go`
- `backend-go/services/project-service/internal/adapter/grpcclient/dev_server_relay.go` (mẫu dial có `otelgrpc` tới infra-fleet)
- `backend-go/deploy/alerts/mcp.rules.yaml`, `package.json` (script `test:e2e:terminal-perf:check-report`, mẫu so báo cáo với ngân sách), `agent/package.json`
- `docs/research/view-code/02-local-mcp-interaction.md` mục 3 và 4, `04-raw-data-and-pipeline.md` mục 2, `06-gaps-risks-roadmap.md` mục 2
- `docs/crs/v7/README.md` mục 3.2, 6, 7; `docs/crs/v5/mcp-quality-rollout/CR-MCP-015-conformance-e2e-observability-rollout.md`; `docs/crs/v6/request-quality-rollout/CR-REQ-024-jira-status-sync-audit-observability.md`
