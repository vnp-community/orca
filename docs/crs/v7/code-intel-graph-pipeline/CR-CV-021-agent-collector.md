# CR-CV-021 — Collector: gọi agent qua `RelayByDevServer`, điều phối truy vấn, ánh xạ lỗi

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-021 |
| **Tên** | Client trong `code-intel-service` gọi nhóm RPC `codeintel.*` của agent qua infra-fleet `RelayByDevServer`; điều phối theo view, ánh xạ lỗi, chờ dev server kết nối lại, retry, cắt/phân trang, gộp song song |
| **Loại** | Feature (adapter + use case) |
| **Priority** | 🔴 P0 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-010 (khung service), CR-CV-012 (ánh xạ binding), CR-CV-020 (mô hình chuẩn), CR-CV-023 (infra-fleet chuyển được mã lỗi `CODEINTEL_*` và có timeout riêng), CR-CV-001/002 (hợp đồng agent) |
| **Mở khoá** | CR-CV-022 (cache bọc collector), CR-CV-036, CR-CV-040 |
| **Tác động** | `backend-go/services/code-intel-service/internal/usecase/ports.go`, `.../usecase/collect_view.go` (mới), `.../adapter/infrafleetclient/` (mới), `.../config/config.go` |

---

## 1. Bối cảnh và vấn đề

1. `InfraFleetService.RelayByDevServer` đã chuyển `method + params_json` nguyên văn xuống agent và trả `result_json` (`backend-go/proto/orca/infrafleet/v1/infrafleet.proto:866-871`, handler `adapter/grpc/server.go:660-680`, use case `usecase/relay_by_dev_server.go:38-64`). Mẫu client gRPC có sẵn là `git-gateway-service/internal/adapter/grpcclient/relay_executor.go` (hàm `relay`, dòng 66 trở đi: dựng `params_json`, gọi `RelayByDevServer` khi ctx mang `devServerID`, giải mã `result_json`) cùng `resolver.go` (`Dial`, dòng 56-62: `grpc.NewClient` + `insecure` + `otelgrpc`) và `tenant_forwarding.go` (`withTenantMetadata` đặt `grpcmw.MetadataTenantID`).
2. Các điểm lệch giữa kỳ vọng của series và code thật, đã đọc trực tiếp:
   - **Mã lỗi agent bị mất.** `RelayByDevServer.Execute` gói mọi lỗi từ `agent.Exec` thành `apperrors.New(KindInternal, "INFRA_AGENT_EXEC_FAILED", "failed to relay to dev server agent", err)` (`relay_by_dev_server.go:59-62`), và `apperrors.ToGRPCStatus` chỉ gửi `code: message` (`common/apperrors/apperrors.go:97-130`; nguyên nhân gốc chỉ được log, "not sent to client"). Vì vậy `error.data.code` kiểu `CODEINTEL_INDEX_MISSING` của agent (`JSONRPCError.Data`, `devserveragent/jsonrpc.go:27-31`) **không tới được** collector nếu không sửa infra-fleet. CR-CV-023 mục 2.1 là nơi sửa; CR này đặt yêu cầu và cách đọc phía client.
   - **Không có hàng đợi chờ kết nối lại ở Go.** `RelayByDevServer` trả ngay `FailedPrecondition INFRA_DEV_SERVER_NOT_CONNECTED` khi `!IsConnected` (`relay_by_dev_server.go:55-57`). `RECONNECT_WAIT_MS = 20_000` chỉ tồn tại ở cầu nối TypeScript cũ (`backend/src/main/dev-server/dev-server-relay-bridge.ts:77`, hàng đợi ở dòng 629-643); README v7 mục 7 coi "xếp hàng tối đa ~20 s" là hành vi sẵn có, thực ra phải làm ở collector.
   - **Trần gRPC 4 MiB.** Không file Go nào trong `backend-go` đặt `MaxCallRecvMsgSize`/`MaxRecvMsgSize` (grep không có kết quả ngoài code sinh), nên client nhận tối đa 4 MiB mặc định của gRPC dù khung agent cho 16 MiB. Kết quả vượt sẽ ra `ResourceExhausted`.
   - **Kết quả bị giải mã rồi mã hoá lại.** `Exec` trả `map[string]any` (`client.go:424-452`), handler marshal lại; số nguyên lớn qua `float64`, thứ tự khoá mất. Chấp nhận được vì dữ liệu là đồ thị, nhưng collector phải giải mã `result_json` có chủ đích (`json.Decoder.UseNumber` nếu cần chính xác).
   - **Timeout mặc định 30 s** (`session.go:1160-1170`, `config.go:57`); chỉ `agent.execPrompt` được ngoại lệ (`client.go:412-418`). Ngoại lệ `codeintel.*` thuộc CR-CV-023.
3. Series chưa có thành phần nào gọi `codeintel.*`; research 06 G1 xác nhận backend Go chưa gọi `tools/call`.

## 2. Giải pháp đề xuất

### 2.1 Cổng và adapter (mới)

`internal/usecase/ports.go` thêm cổng hẹp (tên theo khái niệm, không `Client` chung chung):

```go
// AgentCodeIntelGateway: một phương thức cho mỗi RPC codeintel.* (CR-CV-001), trả RawCodeIntelResult.
type AgentCodeIntelGateway interface {
    Status(ctx, target AgentTarget) (RawCodeIntelResult, error)
    Overview(ctx, target AgentTarget, p OverviewParams) (RawCodeIntelResult, error)
    Processes(ctx, target AgentTarget, p ProcessesParams) (RawCodeIntelResult, error)
    Process(ctx, target AgentTarget, p ProcessParams) (RawCodeIntelResult, error)
    Subgraph(ctx, target AgentTarget, p SubgraphParams) (RawCodeIntelResult, error)
    Impact(ctx, target AgentTarget, p ImpactParams) (RawCodeIntelResult, error)
    Symbol(ctx, target AgentTarget, p SymbolParams) (RawCodeIntelResult, error)
    Routes(ctx, target AgentTarget, p RoutesParams) (RawCodeIntelResult, error)
    DetectChanges(ctx, target AgentTarget, p DetectChangesParams) (RawCodeIntelResult, error)
    Reindex(ctx, target AgentTarget, mode ReindexMode) (RawCodeIntelResult, error)
}
// AgentTarget{TenantID, DevServerID, WorkspaceRoot, HostPlatform}   (từ repo_bindings, CR-CV-012)
// RawCodeIntelResult{Sources []SourceInfo; HeadCommit string; Stale, Truncated bool; TotalCount int64; Data json.RawMessage}
```

`internal/adapter/infrafleetclient/` (tên theo `workflow-service/internal/adapter/infrafleetclient`, đã có; không dùng `grpcclient`/`common`):

| File (mới) | Nội dung |
|---|---|
| `agent_codeintel_gateway.go` | cài `AgentCodeIntelGateway`: dựng `params_json` = tham số + `workspaceRoot` (**không bao giờ** `args`, tên lệnh, tên repo; D5 README v7), gọi `RelayByDevServer` |
| `agent_error_mapping.go` | ánh xạ lỗi (2.3) |
| `dev_server_reconnect_wait.go` | chờ kết nối lại (2.4) |
| `tenant_forwarding.go` | `withTenantMetadata` (viết lại trong service này, **không import chéo service**, như CR-REQ-001 mục 2.3 với `toMySQLDriverDSN`) |
| `agent_result_decoding.go` | giải mã `result_json` thành `RawCodeIntelResult`; không chuẩn hoá (việc của CR-CV-020) |

`Dial` theo mẫu `resolver.go:56-62` nhưng thêm `grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(<= 16 MiB>))` cho kết nối này. Mặc định `CODEINTEL_AGENT_MAX_RESULT_BYTES = 12 MiB`; vẫn ép ngân sách mô hình ≤ 3 MiB ở response gửi UI (CR-CV-020 mục 2.7). Phía infra-fleet, server gửi không bị giới hạn mặc định (chỉ client nhận bị giới hạn 4 MiB). Địa chỉ: `INFRA_FLEET_SERVICE_ADDR` (cùng tên biến và mặc định với git-gateway-service, `internal/config/config.go:60`). Truyền tải `insecure` giống mọi service hiện nay (khoảng trống mTLS đã ghi ở `api-gateway/internal/adapter/grpc/dial.go:23-28`, không giải quyết ở đây).

### 2.2 Điều phối truy vấn theo view

Use case `CollectView` (`internal/usecase/collect_view.go`, mới) nhận `(tenant, repoBinding, ViewKind, params)` và trả mô hình chuẩn (đã qua CR-CV-020). Nó **không** biết cache; CR-CV-022 bọc nó.

| `ViewKind` | Cuộc gọi agent | Gộp | Ghi chú |
|---|---|---|---|
| `STATUS` | `codeintel.status` | — | Dùng cả để tính `stale` cho các view khác khi `status` rẻ hơn |
| `ARCHITECTURE` | `codeintel.overview` | song song với `status` | `topN` mặc định 200, tối đa 500 |
| `FLOWS` | `codeintel.processes` | song song với `status` | phân trang 2.6 |
| `FLOW` | `codeintel.process` | — | ≤ 200 bước |
| `SUBGRAPH` | `codeintel.subgraph` | — | `depth ≤ 3`; hợp nhất hai nguồn trong `sources[]` ở CR-CV-020 |
| `IMPACT` | `codeintel.impact` | — | `ambiguous` → lỗi `CODEINTEL_AMBIGUOUS_SYMBOL` kèm `candidates` |
| `SYMBOL` | `codeintel.symbol` | — | mã nguồn ≤ 200 KiB; **không cache dài** (CR-CV-022 mục 2.7) |
| `ROUTES` | `codeintel.routes` | — | |
| `CHANGE_OVERLAY` | `codeintel.detectChanges` | song song với `status`; phần diff `git.*`, SQL do CR-CV-030/036 | CR-CV-036 sở hữu logic hợp nhất overlay; collector chỉ cung cấp cuộc gọi |
| `STRUCTURE` | `codeintel.subgraph` (tâm là thư mục/tệp) | | |

Giả định cần xác nhận với CR-CV-001/002 (mục 7, Q1): các method trả `sources[]` của mọi công cụ khả dụng nên collector không chọn công cụ và không có tham số `tool` (README v7 mục 3.2 không liệt kê `tool`).

Song song: dùng `errgroup` với giới hạn đồng thời **mỗi dev server** (`CODEINTEL_MAX_INFLIGHT_PER_DEVSERVER`, mặc định 4) bằng semaphore, vì mọi lệnh chạy chung một agent và mỗi lệnh CLI tốn ~1,8 s (README v7 mục 1). Hạn mức theo tenant/người dùng là việc của CR-CV-013; collector chỉ chặn quá tải một agent. Nếu một nhánh song song lỗi: nhánh `status` lỗi **không** làm hỏng view (đánh `stale` không biết → `stale=false`, `sources` rỗng, ghi log); nhánh dữ liệu lỗi thì cả view lỗi.

### 2.3 Ánh xạ lỗi

Đọc ở `agent_error_mapping.go`. Lỗi đến collector là gRPC `status` (từ `apperrors.ToGRPCStatus`: `code: message`, `server.go` dòng 673-675). Sau CR-CV-023, infra-fleet với method `codeintel.*` phải trả `status.Message = "<CODEINTEL_X>: <message>"` và đặt dữ liệu có cấu trúc (`error.data` của agent, ví dụ `candidates`) vào **trailer metadata** `x-orca-agent-error-data-bin` (JSON). Collector đọc mã bằng tiền tố của message (`CODEINTEL_[A-Z_]+`) và `data` bằng `grpc.Trailer(&md)` của lời gọi.

| Nguồn | Điều kiện | Kết quả `apperrors` (`Kind`, `Code`) |
|---|---|---|
| Agent `error.data.code` | `CODEINTEL_TOOL_UNAVAILABLE` | `FailedPrecondition`, giữ mã |
| | `CODEINTEL_INDEX_MISSING` | `FailedPrecondition`, giữ mã (UI gợi ý reindex) |
| | `CODEINTEL_REPO_NOT_REGISTERED` | `FailedPrecondition`, giữ mã |
| | `CODEINTEL_PATH_NOT_ALLOWED` | `PermissionDenied`, giữ mã; **ghi audit** (có thể là dấu hiệu tấn công, CR-CV-072) |
| | `CODEINTEL_INVALID_PARAMS` | `InvalidArgument`, giữ mã |
| | `CODEINTEL_AMBIGUOUS_SYMBOL` | `InvalidArgument`, giữ mã, `candidates` lấy từ trailer |
| | `CODEINTEL_TIMEOUT` | `DeadlineExceeded`, giữ mã |
| | `CODEINTEL_REINDEX_IN_PROGRESS` | `FailedPrecondition`, giữ mã |
| | `CODEINTEL_OUTPUT_TOO_LARGE` | `FailedPrecondition`, giữ mã (gợi ý thu hẹp phạm vi) |
| | `CODEINTEL_TOOL_FAILED` | `Internal`, giữ mã |
| infra-fleet | `INFRA_DEV_SERVER_NOT_CONNECTED` (`FailedPrecondition`) | vào chờ kết nối lại (2.4); hết hạn → `Unavailable`, mã **mới** `CODEINTEL_DEV_SERVER_OFFLINE` |
| | `INFRA_DEV_SERVER_NOT_FOUND` (`NotFound`) | `NotFound`, `CODEINTEL_REPO_NOT_REGISTERED` (binding trỏ tới dev server đã xoá) + log cảnh báo cho CR-CV-012 |
| | `INFRA_NO_TENANT` | `Unauthenticated`, giữ mã `INFRA_NO_TENANT` (lỗi lập trình: quên chuyển tenant; không thêm mã `CODEINTEL_*`) |
| | `CODEINTEL_AGENT_UNSUPPORTED` (CR-CV-023 sinh khi agent trả -32601 cho `codeintel.*`) | `FailedPrecondition`, giữ mã **mới** (agent cũ chưa có `codeintel.*`) |
| gRPC | `DeadlineExceeded` do ctx của collector hoặc infra-fleet (`request "…" timed out`) | `DeadlineExceeded`, `CODEINTEL_TIMEOUT` |
| | `Unavailable`, `ResourceExhausted` (> trần nhận) | `Unavailable` (retry 2.5) / `FailedPrecondition` `CODEINTEL_OUTPUT_TOO_LARGE` |
| Giải mã | JSON hỏng, thiếu `data`, kiểu sai | `Internal`, mã **mới** `CODEINTEL_RESULT_INVALID` (không lộ nội dung agent trong message) |

Ba mã mới (`CODEINTEL_DEV_SERVER_OFFLINE`, `CODEINTEL_AGENT_UNSUPPORTED`, `CODEINTEL_RESULT_INVALID`) không có trong README v7 mục 3.3; xem báo cáo "Điều chỉnh hợp đồng". Mã lạ đến từ agent (không thuộc danh sách) → `Internal` + `CODEINTEL_TOOL_FAILED`, giữ message đã cắt ≤ 200 ký tự và loại ký tự điều khiển.

### 2.4 Dev server offline và chờ kết nối lại

`dev_server_reconnect_wait.go`:

- Chỉ kích hoạt khi lỗi là `INFRA_DEV_SERVER_NOT_CONNECTED`. Ngân sách `CODEINTEL_RECONNECT_WAIT` mặc định **20 s** (khớp `RECONNECT_WAIT_MS`), tối thiểu 0 (tắt).
- Thăm dò bằng RPC `IsDevServerConnected` (có sẵn: `infrafleet.proto:900-905`, `server.go:~700`; không dial, không tác dụng phụ) theo lịch 0,5 s → 1 s → 2 s → 2 s…, cộng jitter ±20 %; khi `connected=true` thì gọi lại đúng một lần `RelayByDevServer`.
- **Cổng theo dev server** (`map[devServerID]*gate`): nhiều yêu cầu cùng đợi một dev server thì chỉ **một** goroutine thăm dò và đánh thức tất cả; tránh bão thăm dò khi UI mở nhiều view cùng lúc.
- Huỷ theo `ctx` của yêu cầu; nếu ctx còn ít hơn ngân sách thì dùng phần ít hơn.
- Hết hạn: `CODEINTEL_DEV_SERVER_OFFLINE`. CR-CV-022 sẽ phục vụ snapshot cũ (`stale=true`) thay vì lỗi nếu có, nên lỗi này chỉ đến UI khi chưa có snapshot.
- Mọi lệnh ghi (`Reindex`) **không chờ**: offline → lỗi ngay (người dùng bấm lại), vì tác động không idempotent (2.5).

### 2.5 Retry

Chỉ lệnh đọc (idempotent). `Reindex` không bao giờ tự retry.

| Lỗi | Retry |
|---|---|
| `INFRA_DEV_SERVER_NOT_CONNECTED` | theo 2.4 |
| gRPC `Unavailable` (infra-fleet khởi động lại) | tối đa 2 lần, trễ 200 ms rồi 600 ms |
| `CODEINTEL_TOOL_FAILED` | tối đa 1 lần sau 500 ms **chỉ** khi trailer có `retryable=true` (agent biết lỗi khoá WAL/DB tạm); nếu agent không gửi cờ, không retry |
| `CODEINTEL_TIMEOUT`, `CODEINTEL_OUTPUT_TOO_LARGE`, `CODEINTEL_INVALID_PARAMS`, `CODEINTEL_AMBIGUOUS_SYMBOL`, `CODEINTEL_PATH_NOT_ALLOWED`, mọi `*_MISSING/*_UNAVAILABLE/*_NOT_REGISTERED` | không (retry không đổi kết quả hoặc tốn thêm ~giây) |

Hạn mức thời gian tổng của một lần thu thập: `ctx` của yêu cầu UI; thêm `context.WithTimeout` mỗi lời gọi gRPC bằng timeout phía infra-fleet cho `codeintel.*` (CR-CV-023, mặc định 90 s) cộng 5 s dự phòng, để lỗi đến từ agent (`CODEINTEL_TIMEOUT`, rõ nghĩa) chứ không từ ctx (`DeadlineExceeded` chung).

### 2.6 Cắt và phân trang

Agent cắt trước theo ngân sách ở README v7 mục 3.2; collector **phòng thủ** vì bản agent cũ/lỗi có thể trả quá:

1. Giải mã → áp `Limit*` của CR-CV-020 mục 2.7 (cùng thứ tự cắt xác định) → nếu bị cắt thêm, `truncated=true` (OR với cờ từ agent).
2. `total_count`: ưu tiên của agent; nếu bị collector cắt thêm thì lấy số đếm trước khi cắt.
3. Phân trang chỉ có ở danh sách (`FLOWS`: `limit ≤ 100`, `offset`). Collector chuyển `limit/offset` nguyên văn và trả `total_count`; **không** tự lặp để lấy hết các trang (tránh 300 luồng × ~1,8 s). UI/gateway gọi trang tiếp theo.
4. View đồ thị (`SUBGRAPH`, `IMPACT`) không phân trang: thu hẹp bằng `depth`, `kinds`, `limit`; `truncated=true` là tín hiệu để UI gợi ý.
5. Kết quả > `CODEINTEL_AGENT_MAX_RESULT_BYTES` (2.1) → `CODEINTEL_OUTPUT_TOO_LARGE`.

### 2.7 Định dạng kết quả chung

Bộ giải mã đọc đúng `sources`, `headCommit`, `stale`, `truncated`, `totalCount`, `data` (README v7 mục 3.2) vào `RawCodeIntelResult`; trường thiếu hoặc sai kiểu → `CODEINTEL_RESULT_INVALID`. Chuyển sang `ResultMeta` của CR-CV-020 ở use case, thêm `repo`, `worktree_id`, `dev_server_id`, `view`, `generated_at` từ binding; `stale` phía service có thể bị **nâng** thành `true` nếu `headCommit` khác commit chỉ mục của nguồn (xem CR-CV-022 mục 2.3), nhưng không bao giờ hạ từ `true` xuống `false`.

### 2.8 Cấu hình (`internal/config/config.go`, thêm vào struct nhúng `commonconfig.Base`)

| Biến | Mặc định | Ý nghĩa |
|---|---|---|
| `INFRA_FLEET_SERVICE_ADDR` | `infra-fleet-service:9090` | Địa chỉ gRPC infra-fleet; cùng tên và mặc định với `git-gateway-service/internal/config/config.go:60` và `workflow-service/internal/config/config.go:81` |
| `CODEINTEL_RECONNECT_WAIT` | `20s` | 2.4 |
| `CODEINTEL_MAX_INFLIGHT_PER_DEVSERVER` | `4` | 2.2 |
| `CODEINTEL_AGENT_MAX_RESULT_BYTES` | `12582912` | 2.1, 2.6 |

### 2.9 Quan sát

Metrics (tên cuối do CR-CV-071): `codeintel_collector_calls_total{method,outcome}`, `codeintel_collector_duration_seconds{method}`, `codeintel_collector_reconnect_wait_seconds`, `codeintel_collector_truncated_total{view}`. Log theo cấu trúc: `tenant_id`, `dev_server_id`, `method`, `outcome`, `duration`; **không** log `params_json` có `content`, **không** log `data` (có thể chứa mã nguồn), che đường dẫn tuyệt đối nếu cần (README v7 mục 6, secret).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Collector tự chờ kết nối lại thay vì sửa `RelayByDevServer` | `RelayByDevServer` dùng chung cho mọi service (git-gateway, workflow, task…); đổi sang chờ 20 s làm treo mọi người gọi |
| Thăm dò bằng `IsDevServerConnected`, không gọi lặp `RelayByDevServer` | RPC có sẵn, không dial, không phát sinh lỗi `INFRA_AGENT_EXEC_FAILED` giả |
| Cổng chờ theo dev server | Tránh N yêu cầu cùng thăm dò |
| Mã lỗi đi bằng tiền tố message + trailer | `ToGRPCStatus` chỉ mang `code: message`; trailer không cần thêm phụ thuộc `errdetails` (chưa dùng ở đâu trong `backend-go`, grep rỗng) |
| Không chọn công cụ ở collector | Hợp đồng agent (README v7 3.2) không có tham số `tool`; việc chọn nằm ở agent |
| Không tự lặp phân trang | 300 luồng × ~1,8 s mỗi lần CLI là quá tải; UI quyết định |
| Chỉ lệnh đọc được retry | `Reindex` có tác dụng phụ, retry tự động có thể chạy `analyze` hai lần |

## 4. Tiêu chí chấp nhận

- [ ] `AgentCodeIntelGateway` được cài trong `internal/adapter/infrafleetclient`; mọi lời gọi chỉ truyền `workspaceRoot` + tham số hợp đồng, không có `args`/tên lệnh/tên repo (kiểm bằng test bắt `params_json`).
- [ ] Với infra-fleet giả trả `INFRA_DEV_SERVER_NOT_CONNECTED` rồi `connected=true` sau 3 s: lời gọi thành công, đúng một lần `RelayByDevServer` sau khi kết nối lại; nếu không bao giờ kết nối: sau 20 s ± 1 s trả `CODEINTEL_DEV_SERVER_OFFLINE` (`Unavailable`).
- [ ] 50 yêu cầu đồng thời cùng một dev server offline chỉ tạo **một** chuỗi thăm dò `IsDevServerConnected` (kiểm bằng bộ đếm trên giả).
- [ ] Mỗi mã trong bảng 2.3 ra đúng `Kind` và `Code`; `CODEINTEL_AMBIGUOUS_SYMBOL` mang `candidates`; mã lạ ra `CODEINTEL_TOOL_FAILED` với message đã cắt.
- [ ] `ResourceExhausted` do kết quả > trần ra `CODEINTEL_OUTPUT_TOO_LARGE`; với `MaxCallRecvMsgSize` đã đặt, kết quả 8 MiB giả đi qua được.
- [ ] Mỗi view trong bảng 2.2 gọi đúng tập method, song song nhánh `status`, và nhánh `status` lỗi không làm view lỗi.
- [ ] Agent trả 5 000 nút cho `SUBGRAPH`: ra 1 500 nút, `truncated=true`, `total_count=5000`, thứ tự xác định.
- [ ] `Reindex` không retry và không chờ khi offline.
- [ ] Giải mã `result_json` thiếu `data` ra `CODEINTEL_RESULT_INVALID`; log không chứa `data`.
- [ ] Không có tên file `helpers/utils/common/misc`; không `max-lines` disable mới.

## 5. Kiểm thử

- **Unit:** bảng ánh xạ lỗi (mỗi dòng một ca); lịch thăm dò và jitter (đồng hồ giả); cổng chờ (đồng thời, huỷ ctx); bộ giải mã (JSON hỏng, thiếu trường, số lớn); cắt xác định.
- **Hợp đồng với infra-fleet giả:** `InfraFleetServiceClient` giả cho `RelayByDevServer`, `IsDevServerConnected`; ca: lỗi kèm trailer, không kèm trailer, status message lạ.
- **Tích hợp (`-tags=integration`):** chạy `infra-fleet-service` thật in-process với `DevServerAgentClient` giả trả JSON-RPC lỗi `CODEINTEL_INDEX_MISSING`, xác nhận mã đến collector sau CR-CV-023 (trước CR-CV-023, test này **phải đỏ**, chứng minh vấn đề ở mục 1).
- **Quan sát:** counter tăng đúng nhãn.
- Chưa chạy test nào ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- Phụ thuộc cứng vào CR-CV-023 để mã lỗi tới được collector; nếu CR-CV-023 chậm, collector chỉ thấy `INFRA_AGENT_EXEC_FAILED` chung và mọi lỗi thành `CODEINTEL_TOOL_FAILED` (chức năng suy giảm, không hỏng).
- Chưa kiểm tra `grpc.Trailer` có qua được lớp `otelgrpc` và các interceptor client hiện hữu không; nếu không, dùng `grpc.Header`/mã trong message.
- Trần 12 MiB nhận chưa đo bộ nhớ; N yêu cầu song song mỗi tới 12 MiB có thể tốn RAM. Cần giới hạn đồng thời toàn service (`semaphore` byte) nếu đo thấy vấn đề.
- Chưa kiểm xem `Client.Exec` có chặn khi agent đang xử lý một lệnh `codeintel.*` dài hay không (các lệnh khác dùng cùng khung WS, chỉ ghép theo `reqID`); khả năng một lệnh nặng làm chậm `pty.*` ở agent chưa được đo.
- Số 4 đồng thời/dev server là phỏng đoán.
- Lịch thăm dò 20 s mặc định theo README; agent reconnect thật có backoff `1,2,5,15,30 s` (research 01 mục 3), nên dev server rớt lâu hơn ~8 s thường đi hết ngân sách.

## 7. Câu hỏi mở

- **Q1.** Method `codeintel.*` trả dữ liệu của một hay hai công cụ trong một lần gọi? Ảnh hưởng đến việc collector có cần tham số `tool` hay không (xem 2.2). Chốt ở CR-CV-001/002.
- **Q2.** Dùng `retryable` trong trailer: agent (CR-CV-001) có gửi cờ này không? Nếu không, bỏ dòng retry `CODEINTEL_TOOL_FAILED`.
- **Q3.** Có cần `Unavailable` riêng cho `CODEINTEL_DEV_SERVER_OFFLINE` ở gateway (CR-CV-040) để UI hiện nút "thử lại"?
- **Q4.** Với thao tác `Reindex` khi dev server offline: tự xếp hàng (đợi lên mạng rồi chạy) hay từ chối? Hiện từ chối.

## 8. Tham chiếu

- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/usecase/relay_by_dev_server.go` (dòng 38-64)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` (dòng 660-680)
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/devserveragent/client.go` (dòng 412-452), `jsonrpc.go` (dòng 27-35), `session.go` (dòng 1160-1170)
- `/opt/repos/orca/backend-go/common/apperrors/apperrors.go` (dòng 97-130)
- `/opt/repos/orca/backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (dòng 866-871, 900-905)
- `/opt/repos/orca/backend-go/services/git-gateway-service/internal/adapter/grpcclient/relay_executor.go`, `resolver.go`, `tenant_forwarding.go`
- `/opt/repos/orca/backend-go/services/workflow-service/internal/adapter/infrafleetclient/` (tên thư mục mẫu)
- `/opt/repos/orca/backend/src/main/dev-server/dev-server-relay-bridge.ts` (dòng 69-77, 625-650)
- `/opt/repos/orca/docs/crs/v7/README.md` (mục 3.2, 3.3, 7), `/opt/repos/orca/docs/research/view-code/03-command-and-data-flow.md`, `04-raw-data-and-pipeline.md`
- CR-CV-020, CR-CV-022, CR-CV-023 cùng folder
