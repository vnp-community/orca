# BE-CV-SOL-021: Collector gọi agent qua `RelayByDevServer`, điều phối view, ánh xạ lỗi, chờ kết nối lại, reindex (`codeintel_reindex.proto`)

> **✅ Implemented.** Đã triển khai, vượt qua toàn bộ test (unit, race, contract, integration). Cần SOL-020 (mô hình), SOL-023 (mã lỗi/timeout, cổng G2), BE-CV-SOL-010 (`apperrors.KindUnavailable`), BE-CV-SOL-012 (`AgentTarget`), BE-CV-SOL-013 (cổng quyền/hạn mức). Số field proto ở 2.E, 2.F là **đã chốt**.

**CR:** [CR-CV-021](../../../../../../docs/crs/v7/code-intel-graph-pipeline/CR-CV-021-agent-collector.md)
**Service:** `code-intel-service` (mới; `internal/usecase`, `internal/adapter/infrafleetclient`, `internal/adapter/grpc`, `internal/config`) · `backend-go/proto/orca/codeintel/v1/`
**TDD tham chiếu:** [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) ("Talking to the Dev Server Agent", gRPC conventions), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md) (timeout mọi lời gọi ra ngoài; retry chỉ lời gọi idempotent; bulkhead), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md) (cổng ở usecase, adapter cài), [`arch/07`](../../../../tdd/architecture/07-security-architecture.md) ("Multi-tenancy isolation"), [`services/infra-fleet-service.md`](../../../../tdd/services/infra-fleet-service.md) §7
**Hợp đồng:** `CONTRACT-codeintel-proto-and-data-map.md` (PQ-02/03/04/12/13/14/16/19/20/21; §2.1 hàng 3, 6; §3.1, §3.3; §4 T7; §5; §6.2), `CONTRACT-codeintel-agent-rpc.md` (§2, §3, §4, §6)

---

## 0. Hợp đồng áp dụng

| Phán quyết / mục | Áp dụng |
|---|---|
| PQ-02 chặng 2→3 | Collector đọc mã bằng tiền tố `^CODEINTEL_[A-Z0-9_]+: ` và `data` từ trailer `x-orca-agent-error-data-bin`; ném `apperrors` giữ mã |
| PQ-03 | Một mã một nghĩa: `INVALID_PARAMS` (không `INVALID_ARGUMENT`); `DEV_SERVER_OFFLINE` = `KindUnavailable`; `SYMBOL_NOT_FOUND`; `OUTPUT_TOO_LARGE` = `FailedPrecondition` |
| PQ-04 | Request gắn worktree có `selector` (trường 1); `repo_binding_id` không có trong request |
| PQ-12/14 | `ResultMeta` phẳng; response ≤ 2 MiB (SOL-020); client `MaxCallRecvMsgSize(16 MiB)`, trần kết quả agent 12 MiB |
| PQ-13 | Collector tự hết hạn 95 s (90 + 5); chờ kết nối lại 20 s; agent luôn hết hạn trước |
| PQ-16 | `reindex` giữ `mode`, `tools`; thêm `trigger`, `ifStale`, `expectHead`; bỏ `tiers`; UI chỉ gửi `mode`; `trigger='manual'` do service đặt; trạng thái DB `queued\|running\|succeeded\|failed\|cancelled` |
| PQ-19/20/21 | Dùng đúng hình `codeintel.status`; agent chuẩn hoá `SymbolRef`, backend kiểm lại; chỉ method của agent contract §4; mọi tham số có `workspaceRoot`, không `args/command/cwd/env/repo/cypher/tool` |
| §3.1 | RPC: `RequestReindex` (`reindex`), `GetReindexJob` (`read`, được phép khi cờ tắt), `GetStructure`, `GetClusterOverview`, `GetSubgraph`, `GetImpact`, `GetSymbol` (`read_source`), `GetRouteMap` |
| §4 T7, §5 | Bảng `reindex_jobs` (`active_key` UNIQUE); phát `orca.codeintel.reindex.started` cùng giao dịch tạo job |
| §8.3 | Test hai dialect + cô lập tenant cho `ReindexJobStore` (hợp đồng cổng; SQL ở SOL-011-repositories); `tenant_id` ở mọi lời gọi tới infra-fleet |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: ba CONTRACT; CR-CV-021; `git-gateway-service/internal/adapter/grpcclient/{resolver.go (Dial, dòng ~40–62), relay_executor.go (hàm `relay`, ~55–140), tenant_forwarding.go}`; `workflow-service/internal/adapter/infrafleetclient/` (danh sách file: tên thư mục mẫu có thật); `infra-fleet-service/internal/usecase/{relay_by_dev_server.go, is_dev_server_connected.go}`, `adapter/grpc/server.go` (dòng 660–720); `common/apperrors/apperrors.go` (8 Kind, **chưa** có `KindUnavailable`/`KindResourceExhausted`); `common/grpcmw/grpcmw.go` (`MetadataTenantID`); danh sách `backend-go/services` (**chưa có** `code-intel-service`) và `proto/orca` (**chưa có** `codeintel/`).

Xác nhận: `RelayByDevServer` trả `FailedPrecondition INFRA_DEV_SERVER_NOT_CONNECTED` ngay khi offline (không xếp hàng); `git-gateway` dùng `grpc.NewClient(addr, insecure, otelgrpc.NewClientHandler())` và `metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID)`; `relay_executor` chú thích một lỗi đã gặp: lỗi JSON-RPC của agent trả về như gRPC "ok" trong một số đường (cần kiểm `result_json` có phong bì lỗi, xem 2.B).

### Correction relative to CR-CV-021 (Lệch giữa CR và hợp đồng)

| # | CR nói | Hợp đồng / mã thật | Xử lý |
|---|--------|--------------------|-------|
| C1 | 10 method gateway | Hợp đồng agent §4: thêm `StructuralFacts`, `ReindexStatus`, `ReindexCancel`, `Watch`, `CodegraphSearch`, `Files`; `Reindex` có `mode,tools,trigger,ifStale,expectHead` | Cổng đủ 16 method; `CodegraphSearch`/`Files` (đợt 4, P1) phải chịu `-32601` |
| C2 | View `ARCHITECTURE` ← `overview` | PQ-10: `ARCHITECTURE` = C4 (CR-033); đồ thị cụm là `GetClusterOverview` | View `CLUSTERS` ← `codeintel.overview` |
| C3 | View `STRUCTURE` ← `subgraph` tâm thư mục/tệp | Agent §4.4: tâm chỉ `symbol\|file\|cluster`, trả `SymbolGraph`; RPC cần `ModuleGraph` | Chuyển `SymbolGraph` (nút `file`/`folder`, cạnh `IMPORTS/CONTAINS`) thành `ModuleGraph`; `symbol_count`/`loc` = 0 (không biết). **Chưa rõ agent chấp nhận thư mục ở `center.file`** (Q1) |
| C4 | `FLOWS`/`FLOW` là view | Hợp đồng §3.1 không có RPC; README v7 mục 8 điểm 9: `Process` không vào RPC Go | Chỉ cài ở `CollectView` cho MCP/P2, **không** RPC |
| C5 | Nhánh `status` lỗi → `stale=false` | H7: không suy kết luận từ thiếu dữ liệu | `stale` lấy từ phong bì agent, không hạ; thêm cảnh báo `status_unavailable` |
| C6 | Mã lỗi thiếu `PROFILE_UNKNOWN, ENV_NOT_READY, RUN_*, SYMBOL_NOT_FOUND` | Hợp đồng agent §3.2/3.4 | Bảng 2.C đủ; `SYMBOL_NOT_FOUND` = `NotFound` |
| C7 | `RequestReindex` ngoài phạm vi | Hợp đồng §2.1 hàng 6 gán `codeintel_reindex.proto` cho CR-021 | Thêm 2.F (proto, use case, `ReindexJobStore`) |
| C8 | Ngân sách 3 MiB | PQ-14: 2 MiB | Theo SOL-020 |
| C9 | Che secret trong `symbol` không nêu | Agent §4.6: backend che secret, `contentWithheld:"sensitive_path"` | Cổng `SymbolSourceRedactor`; **không có redactor thì giấu mã nguồn** (fail-closed, H8) |
| C10 | `CODEINTEL_INVALID_ARGUMENT` (CR-011/012/085) | PQ-03(2): bỏ | Chỉ `INVALID_PARAMS` |

## 2. Giải pháp

### A. Cây file (mới)

```
backend-go/proto/orca/codeintel/v1/codeintel_graph.proto      (sửa: request/response graph RPC)
backend-go/proto/orca/codeintel/v1/codeintel_reindex.proto    (mới)
backend-go/proto/orca/codeintel/v1/codeintel.proto            (sửa: thêm rpc; do SOL-010 tạo)
services/code-intel-service/internal/
  domain/agent_status.go                       AgentStatusData (hình agent §4.1) — dùng chung SOL-012/022
  usecase/agent_code_intel_gateway.go          cổng AgentCodeIntelGateway + AgentRPCCaller + AgentTarget/RawCodeIntelResult/params
  usecase/collect_view.go                      CollectView (điều phối) + ViewReader
  usecase/collect_view_decoding.go             data agent -> domain (dùng SOL-020)
  usecase/symbol_source_redaction.go           cổng SymbolSourceRedactor + fail-closed mặc định
  usecase/request_reindex.go  get_reindex_job.go  reindex_job_refresh.go
  usecase/reindex_job_store.go                 cổng ReindexJobStore, ReindexAdmission
  adapter/infrafleetclient/{agent_codeintel_gateway.go, agent_rpc_caller.go, agent_error_mapping.go,
                            dev_server_reconnect_wait.go, tenant_forwarding.go, agent_result_decoding.go}
  adapter/grpc/{server_graph_views.go, server_reindex.go, reindex_proto_mapping.go}
  config/config.go                             (sửa)
```

`ports.go` của CR-010/011 do nhiều solution sửa nên cổng của solution này đặt **file riêng** (không xung đột merge).

### B. Cổng và adapter

`AgentRPCCaller.Call(ctx, target AgentTarget, method string, params map[string]any) (RawCodeIntelResult, error)` là lõi dùng chung (BE-CV-SOL-082 dùng lại cho `quality.*`). `AgentCodeIntelGateway` bọc 16 method, mỗi method **dựng `params` từ struct có kiểu hẹp** + `workspaceRoot` (không nhận `map` tự do) và có test phản chiếu: `params_json` không bao giờ chứa `args|argv|command|cmd|cwd|env|repo|cypher|shell|timeout|tool`.

Adapter (`infrafleetclient`): `Dial(addr)` theo mẫu `resolver.go` + `grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(16<<20))`; `withTenantMetadata` **viết lại tại service này** (không import chéo, `arch/03`); gọi `RelayByDevServer{dev_server_id, method, params_json}`; đọc `grpc.Trailer(&md)`; giải mã `result_json` bằng `json.Decoder` (`UseNumber`) vào `RawCodeIntelResult{Sources, HeadCommit, Stale, Truncated, TotalCount *int64, Warnings, Perf, Data json.RawMessage}`; thiếu/sai kiểu → `CODEINTEL_RESULT_INVALID` (`Internal`, không lộ nội dung); phát hiện **phong bì lỗi JSON-RPC trong `result_json`** (khuôn bug đã ghi ở `relay_executor.go`) và coi là `CODEINTEL_TOOL_FAILED`. Mỗi lời gọi có `context.WithTimeout(ctx, 95 s)` (`CODEINTEL_AGENT_CALL_TIMEOUT`). Kích thước `result_json` > 12 MiB (`CODEINTEL_AGENT_MAX_RESULT_BYTES`) hoặc gRPC `ResourceExhausted` → `CODEINTEL_OUTPUT_TOO_LARGE`. Log: `tenant_id, dev_server_id, method, outcome, duration`; **không** log `params_json`, `data`, đường dẫn tuyệt đối.

### C. Ánh xạ lỗi (`agent_error_mapping.go`)

| Nguồn | Điều kiện | `apperrors` |
|---|---|---|
| tiền tố message | `TOOL_UNAVAILABLE, INDEX_MISSING, REPO_NOT_REGISTERED, REINDEX_IN_PROGRESS, OUTPUT_TOO_LARGE, AGENT_UNSUPPORTED, ENV_NOT_READY, RUN_IN_PROGRESS, RUN_CANCELLED` | `FailedPrecondition`, giữ mã |
| | `PATH_NOT_ALLOWED` | `PermissionDenied`, giữ mã, **ghi audit** (dấu hiệu tấn công, CR-072) |
| | `INVALID_PARAMS, AMBIGUOUS_SYMBOL, PROFILE_UNKNOWN` | `InvalidArgument`, giữ mã |
| | `SYMBOL_NOT_FOUND, RUN_NOT_FOUND` | `NotFound`, giữ mã |
| | `TIMEOUT` | `DeadlineExceeded`, giữ mã |
| | `TOOL_FAILED` | `Internal`, giữ mã |
| infra-fleet | `INFRA_DEV_SERVER_NOT_CONNECTED` | vào chờ (2.D); hết hạn → `KindUnavailable` `CODEINTEL_DEV_SERVER_OFFLINE` |
| | `INFRA_DEV_SERVER_NOT_FOUND` | `NotFound` `CODEINTEL_REPO_NOT_REGISTERED` + log cảnh báo cho SOL-012 (Q3) |
| | `INFRA_NO_TENANT` | `Unauthenticated`, giữ mã (lỗi lập trình) |
| gRPC | `DeadlineExceeded` | `CODEINTEL_TIMEOUT` |
| | `Unavailable` | retry (2.D) rồi `KindUnavailable` |
| | `ResourceExhausted` | `CODEINTEL_OUTPUT_TOO_LARGE` |
| giải mã | JSON hỏng/thiếu `data` | `Internal` `CODEINTEL_RESULT_INVALID` |
| mã lạ | không thuộc danh sách | `Internal` `CODEINTEL_TOOL_FAILED`, message cắt ≤ 200, bỏ ký tự điều khiển |

`data` từ trailer (≤ 4 KiB) được giữ trong `AgentErrorData` đính kèm lỗi để lớp trên nối hậu tố `" | {json}"` (PQ-02 chặng 3, việc của SOL-040); trailer mất/hỏng → bỏ `data`, vẫn giữ mã.

### D. Chờ kết nối lại và retry

`dev_server_reconnect_wait.go`: chỉ kích hoạt với `INFRA_DEV_SERVER_NOT_CONNECTED`; ngân sách `CODEINTEL_RECONNECT_WAIT` = 20 s (0 = tắt); thăm dò RPC `IsDevServerConnected` (có sẵn, `infrafleet.proto:900–905`) theo lịch 0,5 → 1 → 2 → 2 s ± 20 %; **cổng theo `tenantID|devServerID`** (một goroutine thăm dò, đánh thức mọi người chờ; khoá có tenant để không dùng metadata tenant này cho tenant khác); khi `connected=true` gọi lại đúng **một** lần; theo `ctx`, lấy phần ngắn hơn. `Reindex` và `Watch` **không chờ**, không retry (tác dụng phụ). Retry (chỉ đọc): gRPC `Unavailable` ≤ 2 lần (200 ms, 600 ms); `TOOL_FAILED` 1 lần sau 500 ms chỉ khi trailer `retryable=true`; mọi mã khác không retry. Đồng thời mỗi dev server: semaphore `CODEINTEL_MAX_INFLIGHT_PER_DEVSERVER` (4, phỏng đoán).

### E. `CollectView` và proto request/response

| `ViewKind` | Agent | Song song | Kết quả |
|---|---|---|---|
| `STATUS` | `status` | — | `AgentStatusData` |
| `CLUSTERS` | `overview` | `status` | `ArchitectureGraph` |
| `STRUCTURE` | `subgraph{center:{file}}` (C3) | `status` | `ModuleGraph` |
| `SUBGRAPH` | `subgraph` | `status` | `SymbolGraph` (hợp nhất hai nguồn) |
| `IMPACT` | `impact` | `status` | `ImpactGraph` (`AMBIGUOUS_SYMBOL` là lỗi kèm `candidates`) |
| `SYMBOL` | `symbol` | — | `SymbolDetail` qua `SymbolSourceRedactor` |
| `ROUTES` | `routes` | `status` | `RouteMap` |
| `FLOWS`/`FLOW` | `processes`/`process` | — | chỉ MCP/P2 (C4) |
| `CHANGE_OVERLAY` | `detectChanges` | `status` | trả `RawCodeIntelResult` nguyên (SOL-036 chuyển đổi) |

Quy tắc: `SymbolRef` đi `ValidateAgentSymbolRef`; hai nguồn → `MergeSymbolSets`/`MergeEdges`; áp `Limit*` (SOL-020); `ResultMeta.stale` = phong bì OR `headCommit` khác `commit` nguồn, **không bao giờ hạ**; `truncated` OR; `total_count` agent (`null`→0); phân trang chỉ truyền `limit/offset`, **không** tự lặp trang. Nhánh `status` lỗi không làm hỏng view (C5); nhánh dữ liệu lỗi thì view lỗi. Interface `ViewReader.Get(ctx, target, view, params) (ViewResult, error)`; `CollectView` là bản cài, SOL-022 bọc nó.

Proto (`codeintel_graph.proto`, `selector=1` mọi request; số theo thứ tự hợp đồng §3.1): `GetStructureRequest{selector, path, depth, limit, page_token, if_none_match}` → `GetStructureResponse{meta, data: ModuleGraph, next_page_token}`; `GetClusterOverviewRequest{selector, top_n, if_none_match}` → `{meta, data: ArchitectureGraph}`; `GetSubgraphRequest{selector, center (oneof symbol|file|cluster), depth, repeated string kinds, limit, if_none_match}` → `{meta, data: SymbolGraph}`; `GetImpactRequest{selector, target (oneof key | {name,file,kind}), direction, depth, include_tests, if_none_match}` → `{meta, data: ImpactGraph}`; `GetSymbolRequest{selector, oneof {key | {name,file}}, include_source}` → `{meta, data: SymbolDetail}`; `GetRouteMapRequest{selector, limit, page_token, if_none_match}` → `{meta, data: RouteMap, next_page_token}`. Biên (`depth 1..3`, `limit ≤ 1500`, `top_n ≤ 500`) bị từ chối `INVALID_PARAMS`, không kẹp ngầm (ui-api §2.4).

### F. Reindex (`codeintel_reindex.proto`)

```proto
message ReindexJob {                      // proto: số đề xuất theo thứ tự ui-api §4.1 ReindexJob
  string job_id = 1; string repo_binding_id = 2; string mode = 3;      // incremental|full
  string status = 4;  string trigger = 5; string stage = 6;           // status: queued|running|succeeded|failed|cancelled
  optional int32 percent = 7; string message = 8; string outcome = 9; string error_code = 10;
  google.protobuf.Timestamp started_at = 11; google.protobuf.Timestamp finished_at = 12; google.protobuf.Timestamp created_at = 13;
}
message RequestReindexRequest { WorktreeSelector selector = 1; string mode = 2; }
message RequestReindexResponse { ReindexJob job = 1; }
message GetReindexJobRequest { WorktreeSelector selector = 1; string job_id = 2; }
message GetReindexJobResponse { ReindexJob job = 1; }
```

`RequestReindex.Execute`: (1) cổng SOL-013 (`reindex`) + `ResolveTarget` SOL-012; (2) `ReindexAdmission.Admit` (cooldown 5 phút, hạn mức: SOL-013; mã `CODEINTEL_REINDEX_COOLDOWN|RATE_LIMITED|CONCURRENCY_LIMIT`); (3) `ReindexJobStore.CreateActive` (cùng giao dịch ghi outbox `reindex.started`; xung đột `active_key` → trả `CODEINTEL_REINDEX_IN_PROGRESS | {"jobId","stage"}`); (4) gọi agent `codeintel.reindex{workspaceRoot, mode, trigger:"manual"}` — offline → `Finish(failed, DEV_SERVER_OFFLINE)` ngay; agent trả `REINDEX_IN_PROGRESS` kèm `jobId` → nhận nuôi (lưu `agent_job_id`, `running`); `outcome != ""` (`already_up_to_date|skipped_scope_repo_root|superseded`) → `succeeded` ngay; (5) lưu `agent_job_id`. `interrupted` ánh xạ `failed` `CODEINTEL_REINDEX_INTERRUPTED`, `cancelling` hiển thị `running` (PQ-16). `GetReindexJob`: đọc DB (job không thuộc tenant/binding của selector → `CODEINTEL_NOT_FOUND`); nếu `queued|running` mà `updated_at` cũ hơn `CODEINTEL_REINDEX_POLL_AFTER` (10 s, đề xuất) → gọi `codeintel.reindexStatus{jobId}` làm tươi hàng (phòng mất sự kiện). Không có RPC huỷ (O-17).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Collector tự chờ kết nối lại | `RelayByDevServer` dùng chung nhiều service (F3 README feature) |
| Cổng chờ khoá theo tenant | Metadata tenant của lời thăm dò thuộc đúng một tenant |
| `AgentRPCCaller` lõi dùng chung cho `quality.*` | Một nơi xử lý lỗi/timeout/trailer |
| Params dựng từ struct hẹp | D5/H3: không có đường truyền lệnh tuỳ ý |
| `stale` không hạ, redactor fail-closed | H7/H8 |
| Reindex không retry/không chờ | Tác dụng phụ không idempotent |
| Cổng đặt file riêng thay vì `ports.go` | Giảm xung đột với SOL-010/011/012 |

## 4. Phụ thuộc chéo khu vực

| Hướng | Solution | Quan hệ |
|---|---|---|
| Trước | `BE-CV-SOL-020`, `BE-CV-SOL-023` (G2), `BE-CV-SOL-010` (`KindUnavailable`), `BE-CV-SOL-011-*` (`reindex_jobs`, outbox), `BE-CV-SOL-012-target-resolution-and-bindings` (`AgentTarget`), `BE-CV-SOL-013-*` (cổng, `ReindexAdmission`, redaction) | |
| Sau | `BE-CV-SOL-022` (bọc `ViewReader`), `BE-CV-SOL-024` (dùng `Watch`, `ReindexStatus`, `reindex_jobs`), `BE-CV-SOL-036`, `BE-CV-SOL-040-*` (gateway), `BE-CV-SOL-082`, `BE-CV-SOL-070-collector-golden-contract` | |
| Agent | `AG-CV-SOL-001/002/003/004/005`, `AG-CV-SOL-037-structural-facts`, `AG-CV-SOL-070-*` (tệp vàng G1) | hình `data` theo agent contract §4 |
| Frontend | `FE-CV-SOL-050-types-and-runtime-bridge` | mirror kiểu; `ReindexJob` ui-api §4.1 |

## 5. Kiểm thử

- **Unit:** bảng ánh xạ lỗi (mỗi dòng một ca); lịch thăm dò + jitter (đồng hồ giả); cổng chờ (50 yêu cầu → một chuỗi thăm dò; huỷ ctx); bộ giải mã (JSON hỏng, thiếu trường, số lớn, phong bì lỗi trong `result_json`); cắt xác định; test phản chiếu cấm khoá `args|command|…`; mỗi view gọi đúng tập method.
- **Hợp đồng với tệp vàng G1:** đọc `testdata/agent-results/*.json` (AG-CV-SOL-070) qua infra-fleet giả; khi chưa có, dùng mẫu hợp đồng §4.
- **Tích hợp (`-tags=integration`):** infra-fleet **thật** in-process (SOL-023) với agent giả trả `CODEINTEL_INDEX_MISSING`: mã tới collector; trước SOL-023 test này **phải đỏ**. Trailer qua `otelgrpc` thật.
- **Reindex hai dialect + tenant:** `ReindexJobStore` contract (PG + MySQL; không `RETURNING` ở MySQL): hai `CreateActive` đồng thời → một thắng; job tenant khác → `NOT_FOUND`.
- Chưa chạy test nào.

## 6. Rủi ro và điểm chưa kiểm chứng

- Trailer chưa kiểm qua `otelgrpc`; nếu mất thì mã vẫn tới (`Message`), `data` mất.
- 12 MiB × N yêu cầu song song có thể tốn RAM; có thể cần semaphore byte toàn service. Số 4/dev server là phỏng đoán.
- Mẫu Cypher của agent chưa chạy (agent contract §0): hình `data` có thể lệch; chặn bằng tệp vàng.
- `ModuleGraph` thiếu `symbol_count/loc` (C3): có thể làm lens Cấu trúc nghèo (FE-054).
- Lịch chờ 20 s so với backoff agent `1,2,5,15,30 s`: thường đi hết ngân sách.
- Chưa kiểm một lệnh nặng có làm chậm `pty.*` trên cùng phiên agent không.

## 7. Câu hỏi mở

- **Q1.** Agent có nhận thư mục ở `subgraph.center.file`? Nếu không, cần method/`center.folder` ở AG-CV-SOL-002 (ảnh hưởng `GetStructure`).
- **Q2.** `mode`/`status`/`trigger` của `ReindexJob` là chuỗi hay enum? Chọn chuỗi (mở), chờ chủ hợp đồng.
- **Q3.** Binding trỏ dev server đã xoá: `REPO_NOT_REGISTERED` hay `CODEINTEL_NO_DEV_SERVER`?
- **Q4.** `CODEINTEL_REINDEX_POLL_AFTER` chưa có trong hợp đồng §6.2.

## 8. Tham chiếu

- `/opt/repos/orca/specs/backend-go/crs/v7/CONTRACT-codeintel-*.md`; `/opt/repos/orca/docs/crs/v7/code-intel-graph-pipeline/CR-CV-021-agent-collector.md`
- `/opt/repos/orca/backend-go/services/git-gateway-service/internal/adapter/grpcclient/{resolver.go,relay_executor.go,tenant_forwarding.go}`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/usecase/{relay_by_dev_server.go,is_dev_server_connected.go}`, `adapter/grpc/server.go`
- `/opt/repos/orca/backend-go/common/apperrors/apperrors.go`, `common/grpcmw/grpcmw.go`
