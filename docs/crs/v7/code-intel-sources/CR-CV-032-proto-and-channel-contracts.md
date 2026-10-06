# CR-CV-032 — Trích xuất proto, nối client↔server gRPC và danh mục kênh `wscompat`

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-032 |
| **Tên** | Dựng `ContractCatalog`: định nghĩa service/RPC từ `.proto`, cạnh `client → server` giữa các service Go, trạng thái cài đặt từng RPC, và ánh xạ kênh `wscompat` → RPC/method agent |
| **Loại** | Feature (trích xuất tĩnh + mô hình miền, không render) |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-030 (đọc file), CR-CV-020 (`SymbolRef`), CR-CV-012 (binding). Tuỳ chọn: CR-CV-002 (`codeintel.subgraph` của GitNexus để đối chứng) |
| **Mở khoá** | CR-CV-033 (quan hệ `calls-rpc`), CR-CV-034 (nối qua ranh giới service), CR-CV-038 (so sánh hợp đồng), CR-CV-035 (tuỳ chọn) |
| **Tác động** | `backend-go/proto/orca/codeintel/v1/contract.proto` (message mới), `code-intel-service/internal/domain/contract` (mới), `internal/usecase/build_contract_catalog.go` (mới), `internal/adapter/protoschema` (mới), `internal/adapter/gocallgraph` (mới, dùng `go/parser`) |
| **Phụ thuộc dữ liệu ngoài** | **E2** (proto), **E8** (kênh `wscompat`); đối chiếu với `specs/frontend/api/rpc-catalog.md` (chưa đọc nội dung); E15 chưa đọc |

---

## 1. Bối cảnh và vấn đề

08 §4 cần nối luồng qua ranh giới service: "client method ↔ server handler theo tên service + method trong proto" và "kênh `wscompat` (`channel` → handler → gRPC client)". Khảo sát code (2026-10-05):

### 1.1 Proto

- `backend-go/proto/orca/**/*.proto`: 18 file, 18 `service` (`mcp` có hai file `mcp.proto`, `external_server.proto`, hai service `McpService`, `McpRegistryService`), 548 dòng `rpc` (`gitgateway` 76, `infrafleet` 94, `project` 58, `auth` 52, `scmintegration` 46, `tenant` 39, `mcp` 37+7, …), ~8 957 dòng, 1 128 dòng `message` (kể cả lồng), 23 `enum`, 5 `oneof`, **0 `map<>`**, không có `extend`/`group`/`reserved`.
- `import` thật: `google/protobuf/{empty,timestamp,struct,wrappers}.proto` và một import nội bộ `orca/workflow/v1/workflow.proto`. Không dùng `google.api.http`. `option go_package` có dạng `github.com/stablyai/orca-go/proto/gen/go/orca/<pkg>/v1;<pkg>v1`. Cấu hình `buf.yaml`: lint `STANDARD`, breaking `FILE`.
- Streaming: 15 RPC server-streaming (`returns (stream …)`), trong đó 2 RPC hai chiều (`rpc X(stream …)`). Comment dẫn đầu RPC mang nhiều ngữ cảnh (đã thấy ở `infrafleet.proto`: 497 dòng comment trên 1 639 dòng) và là nguồn mô tả tự nhiên.
- Code Go sinh sẵn nằm trong repo: `backend-go/proto/gen/go/orca/<pkg>/v1/*_grpc.pb.go`.

### 1.2 Phía server

Mỗi service Go đăng ký bằng `<pkg>v1.Register<Svc>Server(grpcServer, …)` trong `services/<svc>/cmd/server/main.go` (18/18 service proto có đúng một chỗ đăng ký; `mcp-service` đăng ký hai; `infra-fleet-service` đăng ký qua wrapper `withAgentSessionList(infraServer, …)`). Handler là phương thức `func (s *Server) <Rpc>(ctx, req *<pkg>v1.<Rpc>Request)` ở `internal/adapter/grpc/*.go`; struct nhúng `<pkg>v1.Unimplemented<Svc>Server`, nên **RPC khai báo trong proto mà không có phương thức tương ứng sẽ trả `Unimplemented`**.

Ví dụ đã kiểm chứng bằng `grep`: `infra-fleet-service` khai báo 94 RPC; có 86 phương thức `(s *Server)` và `AgentSessionListServer.ListAgentSessions`; **7 RPC không có phương thức nào** trong `internal/` hoặc `cmd/` (`ApplyTerraformPlan`, `CreateFleetDefinition`, `DeployFleetDefinition`, `ExportFleetDefinitionYaml`, `GetFleetDefinition`, `ListFleetDefinitions`, `UpdateFleetDefinition`), trong khi `internal/usecase/` đã có file use case cùng tên và `cmd/server/main.go` chưa thấy wiring cho `FleetDefinition`. Đây là loại phát hiện CR này phải làm nổi (chưa chạy runtime để xác nhận hành vi `Unimplemented`).

### 1.3 Phía client

Client gRPC **không chỉ** ở `adapter/grpcclient`: thăm dò bằng chương trình Go tạm (không thuộc sản phẩm, chạy ở scratchpad 2026-10-05; khớp theo tên phương thức RPC trong file có tham chiếu `<pkg>v1.<Svc>Client`, không phân giải kiểu) tìm thấy lời gọi ở `adapter/scmstarcheck` (tenant-service), `adapter/serviceclients` (workflow-service), `adapter/grpcclient` (nhiều service), `cmd/server/main.go`, và trực tiếp trong handler `api-gateway/internal/adapter/wscompat`. Kết quả sơ bộ: 16 service là bên gọi, 834 vị trí khớp, ví dụ `api-gateway` gọi 16 service; `task-service` → `ai-provider, git-gateway, infra-fleet, orchestration, project, tenant, workflow`; `workflow-service` → 8 service. Có dương tính giả (trùng tên phương thức); cần phân giải kiểu như ở mục 2.4 trước khi tin cạnh.

### 1.4 `wscompat`

`api-gateway/internal/adapter/wscompat`: `Registry` có bốn bản đồ (`Register` unary, `RegisterStream`, `RegisterStreamChannel`, `RegisterBinaryStreamHandler`), khai báo trong `registry.go` và `registry_channels.go` (hàm `Channels()` trả danh sách tên đã sắp, **chỉ dùng được trong tiến trình gateway**). Đăng ký nằm ở 62 file `channels_*.go` không-test, cộng `workspace_events.go` (1 chỗ) và `register_production.go`.

Thăm dò tĩnh (chương trình Go tạm dùng `go/parser`, chạy ở scratchpad): trong `channels*.go` không-test có **461** lời gọi `r.Register*` (`Register` 439, `RegisterStream` 12, `RegisterStreamChannel` 8, `RegisterBinaryStreamHandler` 2); **455** có tên kênh là literal, **6** có tên kênh là biến (các hàm dùng chung `registerAccountsRelay`, `registerBrowserRelay`, `registerBrowserProfileRelay`, `registerCLIAuthStatusRelay`, `registerCLIAuthLoginChannel`, `simpleFileOp`, được gọi ở nơi khác với tên kênh literal, ví dụ `registerBrowserRelay(r, client, "browser."+op, "browser."+op)`). Trong 455 kênh literal, **352** có lời gọi trực tiếp tới một client gRPC có kiểu suy ra được từ tham số hàm bao ngoài (`taskClient taskv1.TaskServiceClient`); **103** không có (uỷ quyền cho hàm khác, xử lý cục bộ như `host.wsl.isAvailable`, hoặc relay xuống agent).

Ba kiểu đích khác nhau cần phân biệt:

1. kênh → **RPC của service khác** (`taskClient.CreateTask(...)`);
2. kênh → **method của agent** qua `InfraFleetService.Relay/RelayByDevServer` (`registerAccountsRelay(r, client, channel, agentMethod)`); đích thật là chuỗi `agentMethod`, không phải tên RPC;
3. kênh **xử lý cục bộ** hoặc **đẩy sự kiện** (`PushEvent{Channel: "..."}`; 13 tên kênh push literal đã thấy, ví dụ `agent.statusChanged`, `files.watch`, `notifications.event`).

Điểm cần cẩn trọng: (a) trong `channels.go` có chú thích rằng hình dạng `args` chỉ là "best-effort", **chưa đối chiếu với call site frontend**; vì vậy CR này chỉ cung cấp ánh xạ tên, không cung cấp kiểu payload; (b) cùng một tên kênh có thể được đăng ký hai lần (đăng ký sau thắng; `channels_cli_auth.go` ghi chú về va chạm tên `github.startAuthLogin`).

### 1.5 GitNexus: đối chứng được, nhưng không đủ làm nguồn chính (đã chạy `cypher` trên repo Orca)

- Cạnh `IMPLEMENTS` của Go chỉ do nhúng struct ("scope-resolution: inherits", confidence 0,85); ở `infra-fleet-service/internal/adapter` chỉ có 3 cạnh (ví dụ `AgentSessionListServer → InfraFleetServiceServer`), **không** thể hiện việc thoả mãn interface cấu trúc.
- Cạnh `CALLS` từ `wscompat` tới code sinh được gắn vào hàm đăng ký (`registerGitDeepChannels`, `registerFilesChannels`), không vào từng closure, và phần lớn tới **hàm tạo message** (`GetDiffRequest`, `GetEntries`) chứ không tới phương thức RPC của client (`import-resolved`, 0,85).
- Có 718 cạnh `CALLS` từ `backend-go/services` tới `*_grpc.pb.go`, nhiều nhất là `NewInfraFleetServiceClient` (10), `RelayByDevServer` (9), `Relay` (7), `ListIssues` (6): khớp theo tên nên dễ trùng tên.
- Trong 300 `Process` của repo, 76 `Process` chạm `backend-go` nhưng đều là luồng chung (`Run → …`, `ServeHTTP → …`); chỉ 1 `Process` có bước trong `wscompat` (`proc_270_servehttp`, bước `cancel` ở `git_generate_cancellation.go`) và **không có** bước nào ở `*_grpc.pb.go` hay `adapter/grpcclient` (chi tiết ở CR-CV-034).

Kết luận: GitNexus không thay thế được việc trích xuất riêng; dùng làm tầng đối chứng tuỳ chọn.

## 2. Giải pháp đề xuất

### 2.1 Kiến trúc: ba tầng nguồn, một mô hình

| Tầng | Nguồn | Độ tin cậy | Bật |
|---|---|---|---|
| T1 | `.proto` + `Register<Svc>Server` + phương thức server (`go/parser`) | 1,0 (khai báo) | luôn |
| T2 | Lời gọi client phân giải theo kiểu cú pháp (tham số/trường kiểu `<pkg>v1.<Svc>Client`) | 0,9 | luôn |
| T3 | Khớp theo tên (file tham chiếu client + tên phương thức trùng RPC) | 0,6 | luôn, nhãn "suy luận" |
| T4 | GitNexus `CALLS` tới `*_grpc.pb.go` (qua `codeintel.subgraph`, CR-CV-002) | 0,4 | tuỳ chọn, mặc định tắt |

Cạnh được gộp theo `(callerSymbol, serviceName, rpc)`, giữ `confidence` lớn nhất và liệt kê `evidence` (danh sách tầng và vị trí).

### 2.2 Mô hình miền (`internal/domain/contract`, mới)

```go
type ContractCatalog struct {
    Services   []ServiceContract
    Edges      []RpcEdge
    Channels   []WsChannel
    Warnings   []ContractWarning
}
type ServiceContract struct { // một proto service
    Name, ProtoPackage, ProtoFile string
    ImplementedBy string          // thư mục service Go; rỗng nếu không tìm thấy Register<Svc>Server
    Rpcs []RpcContract
}
type RpcContract struct {
    Name, InputType, OutputType string
    ClientStreaming, ServerStreaming bool
    Doc string                    // comment dẫn đầu, cắt 300 ký tự
    Handler *SymbolRef            // phương thức server; nil nếu chưa cài đặt
    Unimplemented bool            // không có phương thức cài đặt
    Callers []RpcEdgeRef
    Fields  *MessageShape         // tuỳ chọn, cho CR-CV-038
}
type RpcEdge struct { From SymbolRef; FromService string; To ServiceRef; Rpc string; Confidence float64; Evidence []Evidence }
type WsChannel struct {
    Name string; Kind string      // unary|stream|streamChannel|binaryStream|push
    Targets []WsTarget            // một kênh có thể gọi nhiều đích
    Registered SymbolRef; Dynamic bool; Duplicate bool
}
type WsTarget struct { Kind string /* rpc|agent-method|local */; Service, Rpc, AgentMethod string; Confidence float64 }
```

`MessageShape` (tên trường, số, kiểu, nhãn `repeated/optional/oneof`) chỉ dựng khi `includeShapes` (CR-CV-038), để mô hình mặc định nhẹ (18 service, 548 RPC).

### 2.3 Parse proto (T1) — CHƯA CHỐT thư viện

Tập cú pháp thực tế nhỏ (mục 1.1): `syntax`, `package`, `import`, `option`, `service`, `rpc` (kể cả `stream`), `message` lồng, `enum`, `oneof`, `repeated`/`optional`, comment. Phương án:

| Phương án | Ưu | Nhược |
|---|---|---|
| **A. Bộ phân tích tự viết (tokenizer + đệ quy xuống), không dependency mới** (đề xuất) | Đủ cho tập cú pháp hiện có (không `map`, `extend`, `group`, `reserved`); không đổi `go.mod`; giữ comment dễ dàng | Phải bổ sung khi proto dùng cú pháp mới; không kiểm tra ngữ nghĩa (import, kiểu) |
| B. `bufbuild/protocompile` (Go thuần) | Chuẩn, có thông tin comment/nguồn, kiểm tra ngữ nghĩa | Thêm dependency; chưa đánh giá kích thước, giấy phép, tương thích Go 1.25/1.26; cần cung cấp các file import (`google/protobuf/*.proto` có sẵn nội bộ theo hiểu biết hiện tại, chưa kiểm chứng) |
| C. `emicklei/proto` (AST nhẹ) | Nhẹ, giữ comment | Thêm dependency; chưa đánh giá |
| D. Dùng descriptor đã biên dịch (`protoregistry` từ `proto/gen/go`) | Chính xác, không parse | Phản ánh **phiên bản đã build vào `code-intel-service`**, không phải commit của repo đang review; không có comment; loại cho mục đích review |

Đề xuất A; B là phương án thay thế nếu proto bắt đầu dùng cú pháp phức tạp. **Chưa chốt** vì chưa chạy thử A trên cả 18 file và chưa đánh giá B/C. Kiểm chứng chéo ở CI: so tập `(service, rpc, input, output, streaming)` với descriptor sinh sẵn ở D.

Đầu ra T1 còn gồm `Register<Svc>Server`: parse `cmd/server/main.go` bằng `go/parser`, dò `*ast.CallExpr` có `Fun` dạng `<alias>.Register<Svc>Server`; ánh xạ `alias` → thư mục proto bằng khai báo import (`.../proto/gen/go/orca/<pkg>/v1`). `ImplementedBy` = tên thư mục service chứa `main.go`.

### 2.4 Phía server, phía client (T1–T3)

**Server (trạng thái cài đặt).** Với service Go `S` và proto service `P` có `ImplementedBy = S`: liệt kê `S/internal/adapter/grpc/*.go` (không-test), thu mọi `FuncDecl` có receiver và tên trùng một RPC của `P`, tham số thứ hai có kiểu `*<alias>.<Rpc>Request` (hoặc bất kỳ nếu streaming). RPC không có phương thức → `Unimplemented=true` + cảnh báo `RPC_UNIMPLEMENTED`. Gom cả receiver thứ hai (như `AgentSessionListServer`) vì wrapper tồn tại thật. Phát hiện thân phương thức chỉ trả `status.Error(codes.Unimplemented, …)` → `Unimplemented=true` (heuristic, nhãn "suy luận").

**Client (cạnh).**
1. Quét `internal/**` và `cmd/**` của mọi service (không-test) bằng `go/parser`; tìm khai báo có kiểu `<alias>.<Svc>Client` (tham số hàm, trường struct, biến cục bộ `x := <alias>.New<Svc>Client(...)`).
2. Với mỗi `*ast.CallExpr` dạng `<recv>.<Rpc>(…)` mà `<recv>` là tham số/trường/biến đã gắn kiểu ở bước 1 (T2), hoặc có dạng `<x>.<field>.<Rpc>` với `field` là trường kiểu client của struct nhận (`r.client.RelayByDevServer(...)`), tạo `RpcEdge`.
3. Lời gọi còn lại mà file có tham chiếu client và tên phương thức trùng RPC (T3) → cạnh `confidence 0.6`, nhãn "suy luận".
4. Bên gọi (`From`) là hàm/phương thức bao ngoài (`method:<path>:<Recv>.<Name>` theo README 3.4); với closure đăng ký kênh thì bên gọi là hàm đăng ký, và cạnh được gắn vào `WsChannel` (mục 2.5).
5. `RelayByDevServer`/`Relay` là RPC đặc biệt: tham số `Method` là literal → sinh thêm cạnh `to: agent`, `AgentMethod = <literal>` để luồng có thể đi tiếp tới agent (CR-CV-034).

Bên gọi → bên nhận ở mức service: `From.service` = thư mục service chứa file; `To.service` = `ImplementedBy`. Cạnh `service → service` được rút gọn (đếm số RPC/số vị trí) cho CR-CV-033 (container diagram) và CR-CV-037.

### 2.5 Danh mục kênh `wscompat`

1. Liệt kê `wscompat/*.go` không-test qua CR-CV-030, parse bằng `go/parser`.
2. Với mỗi `*ast.CallExpr` có `Fun` là `r.<Register|RegisterStream|RegisterStreamChannel|RegisterBinaryStreamHandler>` (tên biến receiver `r` lấy từ khai báo tham số kiểu `*Registry`, không hằng):
   - đối số 1 là literal chuỗi → tên kênh; là biểu thức nối (`"browser."+op`, `op` thuộc vòng `for … range []string{…}` literal) → khai triển nếu thành phần đều là literal cục bộ; còn lại → `Dynamic=true` và truy ngược các lời gọi hàm đăng ký dùng chung (`registerBrowserRelay(r, client, "browser."+op, …)`) để lấy tên literal;
   - đối số 2 (closure hoặc tên hàm): thu các đích bên trong theo mục 2.4; nếu là tên hàm trong cùng package thì đi vào thân hàm **một cấp** (đủ cho `simpleFileOp`/các hàm xử lý tách file);
   - phân loại `Kind` theo tên hàm `Register*`.
3. Đích: lời gọi client → `WsTarget{Kind:"rpc"}`; lời gọi `Relay/RelayByDevServer` với `Method` literal hoặc tham số `agentMethod` đã biết → `WsTarget{Kind:"agent-method"}`; còn lại không có đích → `Kind:"local"`.
4. Kênh push: các `PushEvent{Channel: "<literal>"}` → `WsChannel{Kind:"push"}` kèm hàm phát (`From`).
5. Phát hiện `Duplicate` (cùng tên kênh đăng ký ≥ 2 lần) và cảnh báo `CHANNEL_DUPLICATE`.
6. Kết quả là `[]WsChannel` sắp theo tên. Số lượng dự kiến theo thăm dò: ~455 kênh literal (352 có đích RPC trực tiếp), cộng các kênh khai triển từ hàm dùng chung.

Không trả cờ "đã được frontend gọi" ở CR này (đối chiếu với `specs/frontend/api/rpc-catalog.md` hoặc mã frontend là phần mở rộng; chưa làm).

### 2.6 Tính toán, cache, giới hạn

- Số file phải đọc (ước tính, chưa đo): proto 18 file; `cmd/server/main.go` 18; `adapter/grpc` mỗi service 1–20 file; quét client toàn bộ `internal/**` khoảng vài trăm đến hơn một nghìn file Go. Phải đọc qua CR-CV-030 bị chặn 2 000 file/lần; vì vậy quét client thực hiện **theo hai bước**: (1) liệt kê file Go chứa chuỗi `v1.` và `Client` bằng danh sách file từ `ListDir` + cache theo `ContentHash`, (2) chỉ parse file chứa tham chiếu. Dùng cache kết quả theo blob oid (CR-CV-030 2.6): file không đổi thì không đọc lại, nên chỉ lần đầu đắt.
- Ngân sách kết quả: ≤ 700 RPC, ≤ 3 000 cạnh, ≤ 800 kênh; vượt thì `truncated`.
- Cache snapshot: `graph_snapshots(view="contract", params_hash)` (CR-CV-022).

### 2.7 Proto (mới; chia sẻ với CR-CV-034, 038)

`contract.proto` trong `orca/codeintel/v1` khai báo `ServiceContract`, `RpcContract`, `RpcEdge`, `WsChannel`, `WsTarget`, `ContractWarning` tương ứng 2.2. **Không thêm RPC mới ở CR này** (README 3.6 không có RPC cho catalog; xem "Điều chỉnh hợp đồng"): các message được tiêu thụ nội bộ bởi CR-CV-033/034/038; việc lộ ra UI thực hiện qua `GetRouteMap` (mở rộng bằng `repeated WsChannel channels`, cần CR sở hữu `GetRouteMap` đồng ý) hoặc qua `GetContractDiff` (CR-CV-038). Quy ước CR-REQ-001: không khai báo RPC chưa có message.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Trích xuất riêng bằng `go/parser`, GitNexus chỉ đối chứng | `IMPLEMENTS` của Go chỉ nhúng struct; `CALLS` tới stub khớp theo tên và gắn vào hàm đăng ký (mục 1.5) |
| Phân giải kiểu theo cú pháp (tham số/trường/biến) thay vì `go/types` | `go/types` cần toàn bộ module và dependency để type-check; không khả thi khi chỉ có file đọc qua RPC. Chấp nhận độ chính xác thấp hơn và gắn `confidence` |
| Quét client toàn bộ `internal/**`+`cmd/**`, không chỉ `adapter/grpcclient` | Có client ở `scmstarcheck`, `serviceclients`, `cmd/server/main.go`, `wscompat` (mục 1.3) |
| `Unimplemented` là phát hiện hạng nhất | Đã thấy 7 RPC của `infra-fleet-service` không có handler (mục 1.2) |
| Một catalog, ba loại đích kênh | Kênh không phải lúc nào cũng gọi RPC; có relay xuống agent và kênh cục bộ |
| Không lộ payload kênh | Hình dạng `args` chưa kiểm chứng với frontend (mục 1.4) |
| Không đưa RPC mới vào README ở CR này | Tránh phá hợp đồng; xem Câu hỏi mở Q1 |

## 4. Tiêu chí chấp nhận

- [ ] Từ commit hiện tại, catalog liệt kê đủ 18 service và 548 RPC; số RPC từng service khớp `grep -c '^\s*rpc '` trên file proto; cờ streaming khớp (15 server-streaming, 2 hai chiều).
- [ ] Mỗi proto service có `ImplementedBy` đúng (18/18), gồm `McpService` và `McpRegistryService` cùng `mcp-service`.
- [ ] `infra-fleet-service`: đúng 7 RPC `Unimplemented` đã nêu ở 1.2 (hoặc số khác kèm giải thích nếu code đã đổi); `ListAgentSessions` có handler ở `AgentSessionListServer`.
- [ ] Cạnh client: có `workflow-service → automation-service` (qua `adapter/serviceclients`) và `tenant-service → scm-integration-service` (qua `adapter/scmstarcheck`); mỗi cạnh có `evidence` vị trí tệp:dòng và `confidence`.
- [ ] Mọi cạnh T3 có nhãn "suy luận"; có test phân biệt T2/T3 trên tên phương thức trùng nhau ở hai service.
- [ ] `wscompat`: tìm ≥ 455 kênh literal và khai triển 6 điểm đăng ký dùng chung (ví dụ `browser.*`, `accounts.*`) thành tên kênh thật; `Dynamic` chỉ còn khi không khai triển được.
- [ ] Kênh `accounts.*` relay được ghi `Kind:"agent-method"` với `AgentMethod` đúng; kênh `host.wsl.isAvailable` ghi `local`.
- [ ] Có cảnh báo `CHANNEL_DUPLICATE` khi trùng tên.
- [ ] Kết quả có `headCommit`, `stale`, `truncated`; không chứa mã nguồn thân hàm (chỉ `SymbolRef` và vị trí).
- [ ] Kiểm tra chéo tập `(service, rpc)` với descriptor sinh sẵn (phương án D) ở CI không lệch.
- [ ] Tên file/thư mục theo khái niệm cụ thể; không `max-lines` disable.

## 5. Kiểm thử

- **Unit:** parser proto trên bảng cú pháp (service nhiều RPC, `stream`, comment, message lồng, `oneof`, `import` nội bộ); phân giải `Register<Svc>Server` với alias import; phát hiện `Unimplemented`; phân giải kiểu client (tham số, trường struct, biến); khai triển tên kênh ghép; đi vào thân hàm một cấp.
- **Fixture vàng (CR-CV-070):** snapshot catalog của ba service nhỏ (`usage-service`, `annotation-service`, `notification-service`) và `infra-fleet-service`; kiểm tra thay đổi theo phiên bản công cụ.
- **Đối chứng:** so tập RPC với descriptor trong `proto/gen/go`; so cạnh T2 với cạnh T4 (GitNexus) trên mẫu để đo tỷ lệ trùng, ghi vào báo cáo (chưa có số liệu).
- **Hiệu năng:** quét đầy đủ repo Orca với cache lạnh/nóng (mục tiêu chưa đo).
- Chưa chạy bất kỳ test nào ở thời điểm viết CR.

## 6. Rủi ro và điểm chưa kiểm chứng

- Thăm dò bằng tên phương thức có dương tính giả (tên chung như `Get`, `List`); số "834 vị trí khớp" và danh sách cạnh ở 1.3 chưa được kiểm tay từng cạnh, mới xác nhận hai cạnh (`tenant→scm`, `workflow→automation`) có tệp thật.
- `352/455` là tỉ lệ có lời gọi client trực tiếp trong closure; 103 kênh còn lại chưa phân loại chi tiết (uỷ quyền, cục bộ hay relay). Phải đi sâu thêm một cấp để tăng độ phủ và có thể vẫn sót.
- Parse literal bỏ qua kênh đăng ký bằng dữ liệu cấu hình (vòng lặp trên bảng ánh xạ ở file khác); sẽ thành `Dynamic`.
- Chưa chọn thư viện proto, chưa chạy thử bộ phân tích tự viết (mục 2.3).
- Chưa đọc `specs/frontend/api/rpc-catalog.md` để đối chiếu danh sách kênh; `Registry.Channels()` (runtime) là nguồn chính xác nhất nhưng ở trong tiến trình gateway, chưa có đường đọc (Q2).
- Quét client trên toàn bộ `internal/**` tốn nhiều lần đọc file; chi phí qua `RelayByDevServer` chưa đo.
- Cạnh qua `Relay`/`RelayByDevServer` với `Method` là biến (không literal) sẽ không tới được method agent.

## 7. Câu hỏi mở

- **Q1.** Mở RPC nào để lộ catalog ra UI: mở rộng `GetRouteMap` (cần CR sở hữu đồng ý, có thể là CR-CV-040/lens hợp đồng) hay thêm `GetContractCatalog` vào README mục 3.6?
- **Q2.** Có thêm một endpoint chỉ-đọc ở `api-gateway` trả `Registry.Channels()` để làm nguồn kênh chính xác không (thay đổi ngoài phạm vi series)?
- **Q3.** Chốt thư viện proto (A tự viết hay B/C).
- **Q4.** Có đối chiếu danh sách kênh với mã frontend (`window.api` / `rpc-client`) để biết kênh nào "chết" không?
- **Q5.** Ngưỡng `confidence` UI hiển thị mặc định (đề xuất ≥ 0,6 và nhãn "suy luận" cho T3).

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md`; `/opt/repos/orca/docs/research/view-code/08-views-and-review-models.md` (§4), `09-external-inputs-required.md` (E2, E8)
- `/opt/repos/orca/backend-go/proto/orca/**/*.proto`, `/opt/repos/orca/backend-go/proto/buf.yaml`, `/opt/repos/orca/backend-go/proto/gen/go/orca/*/v1/*_grpc.pb.go`
- `/opt/repos/orca/backend-go/services/*/cmd/server/main.go` (`Register<Svc>Server`), `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/grpc/{server.go,agent_session_list_server.go}`, `.../internal/usecase/*_fleet_definition.go`
- `/opt/repos/orca/backend-go/services/tenant-service/internal/adapter/scmstarcheck/grpc_adapter.go`, `/opt/repos/orca/backend-go/services/workflow-service/internal/adapter/serviceclients/serviceclients.go`
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/wscompat/{registry.go,registry_channels.go,channels.go,channels_accounts.go,channels_browser.go,channels_cli_auth.go,channels_git.go,workspace_events.go,register_production.go,push_bridge.go}`
- `/opt/repos/orca/specs/frontend/api/rpc-catalog.md` (chưa đọc nội dung)
- `/opt/repos/orca/docs/crs/v6/request-service-foundation/CR-REQ-001-scaffold-request-service.md` (quy ước proto)
- CR liên quan: CR-CV-002, 012, 020, 022, 030, 033, 034, 038, 040, 070
