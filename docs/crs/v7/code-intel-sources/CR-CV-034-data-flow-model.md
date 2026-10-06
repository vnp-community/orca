# CR-CV-034 — Luồng dữ liệu: dựng `DataFlow` qua ranh giới service, xuất mô hình sequence/DFD

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-CV-034 |
| **Tên** | Dựng `DataFlow` từ kênh `wscompat` và RPC bằng bộ duyệt lời gọi theo tầng hexagonal (handler → use case → cổng → adapter → kho/RPC kế tiếp), nối qua proto client↔server (CR-CV-032), gắn `StoreAccess` (CR-CV-031), làm giàu bằng GitNexus `Process`; xuất mô hình `SequenceModel` và `DfdModel` |
| **Loại** | Feature (dựng mô hình + RPC `ListDataFlows`, `GetDataFlow`; không render) |
| **Priority** | 🟠 P1 |
| **Effort** | Large |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-05 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-CV-032 (catalog RPC, kênh, cạnh client), CR-CV-033 (`ComponentRef` ổn định), CR-CV-031 (`StoreAccess`), CR-CV-030, CR-CV-020; tuỳ chọn CR-CV-002 (`codeintel.processes`, `codeintel.process`) |
| **Mở khoá** | CR-CV-056 (lens Luồng dữ liệu, Mermaid), CR-CV-036 (tuỳ chọn: luồng bị ảnh hưởng), CR-CV-035 (topic) |
| **Tác động** | `backend-go/proto/orca/codeintel/v1/dataflow.proto` (mới), `code-intel-service/internal/domain/dataflow` (mới), `internal/usecase/build_data_flow.go`, `export_flow_models.go` (mới), `internal/adapter/gocallindex` (mới, `go/parser`) |
| **Phụ thuộc dữ liệu ngoài** | **E8** (kênh `wscompat`), **E2** (proto), **E10** (topic sự kiện, một phần, thuộc CR-CV-035), **E9** (workflow do `workflow-service` định nghĩa: **chưa xác nhận nằm đâu**, ngoài phạm vi), E6 (qua `ComponentRef` của CR-CV-033) |

---

## 1. Bối cảnh và vấn đề

08 §4: dùng `Process` của GitNexus, nối client↔server qua proto, nối thêm `wscompat`, và xuất sequence/DFD. Yêu cầu của CR này là kiểm chứng giả định "Process dừng ở stub gRPC sinh tự động, nối được qua proto". Đã chạy `cypher` trên chỉ mục GitNexus của repo Orca (2026-10-05), kết quả:

1. **Phân bố 300 `Process`** theo vị trí của `entryPointId`: `frontend` 115, `backend-go` 76, `desktop` 39, `backend` (TypeScript cũ) 34, `agent` 8, khác 28.
2. **Process của Go không đi vào logic nghiệp vụ.** 76 `Process` có đầu vào ở `backend-go`; tên đều chung (`ServeHTTP → Row`, `Run → IntEnv`, `Run → EventPublishingProcessor`, `ServeHTTP → WriteJSON`, …). Truy vấn: số `Process` có bước nằm trong `/internal/usecase/`, `/adapter/postgres/` hoặc `/adapter/mysql/` là **0** (0 bước); số `Process` có bước ở đường dẫn chứa `grpc` (không phân biệt hoa/thường) là **0**; chỉ **1** bước ở `wscompat` trong toàn bộ 300 `Process` (`proc_270_servehttp`, hàm `cancel` ở `git_generate_cancellation.go`, không phải hop gRPC); 76 `Process` chạm `backend-go` (cùng số với `Process` có đầu vào ở đó); chỉ kiểm tay một mẫu, cho thấy luồng nằm trọn trong một adapter. Mẫu: `proc_102_servehttp` ("ServeHTTP → Row", 5 bước, tất cả trong `api-gateway/internal/adapter/mcpserver/session_host.go`, điểm vào `sessionHost.ServeHTTP`, đích `localSession.row`).
3. **Kết luận**: giả định ở 08 §4 ("Process thường dừng ở stub gRPC") đúng theo nghĩa nghiêm ngặt hơn: trên backend Go, `Process` **không bao giờ tới** stub gRPC hay use case; vì vậy "nối Process qua proto" không khả thi. Luồng nghiệp vụ xuyên service phải do chính CR này dựng. `Process` vẫn dùng được cho phần TypeScript (frontend/desktop/agent: 162 luồng) và để làm giàu.
4. **Định dạng id** hữu ích: `Process.entryPointId`/`terminalId` có dạng `Method:<đường dẫn>:<Receiver.Tên>#<n>` (ví dụ `Method:backend-go/services/api-gateway/internal/adapter/mcpserver/session_host.go:sessionHost.ServeHTTP#2`), ánh xạ được sang `SymbolRef.key` (`method:<đường dẫn>:<Receiver.Tên>`) theo README v7 mục 3.4.
5. **CALLS tới stub là khớp theo tên và sai chỗ gắn** (CR-CV-032 mục 1.5): 718 cạnh `CALLS` từ `backend-go/services` tới `*_grpc.pb.go` (`NewInfraFleetServiceClient`, `RelayByDevServer`, `Relay`…), cạnh từ `wscompat` gắn vào hàm đăng ký chứ không phải closure.
6. **Mô hình thực tế rất đều đặn, đủ để duyệt tĩnh.** Đã đọc: `api-gateway/.../wscompat/channels_accounts.go:136-160` (`registerAccountsRelay`: closure gọi `client.RelayByDevServer(ctx, &infrafleetv1.RelayByDevServerRequest{DevServerId, Method: agentMethod, ParamsJson})`, và 4 lời gọi `registerAccountsRelay(r, client, "accounts.selectClaude", "accounts.selectClaude")`…); `infra-fleet-service/internal/adapter/grpc/server.go:660-684` (`RelayByDevServer` gọi `s.relayByDevServer.Execute(...)`, trường `relayByDevServer *usecase.RelayByDevServer`); `usecase/relay_by_dev_server.go` (`uc.devServers.Get(...)`, `uc.agent.IsConnected(...)`, `uc.agent.Exec(...)` với `devServers DevServerRepository`, `agent DevServerAgentClient` là interface cổng); `adapter/postgres/repository.go` (`FROM infra.dev_servers`). Chuỗi **kênh → RPC → handler → use case → cổng → adapter → kho/agent** có thể suy bằng kiểu của trường/tham số, không cần `go/types`.

## 2. Giải pháp đề xuất

### 2.1 Mô hình `DataFlow` (domain và proto)

Theo 08 §4 nhưng đặt tên `DataFlow*` để không va chạm `FlowSummary`/`FlowGraph` (05 §2.4, GitNexus) trong `orca.codeintel.v1` (xem "Điều chỉnh hợp đồng").

```proto
message DataFlow {
  string id = 1;                       // "ws:<channel>" | "grpc:<Service>.<Rpc>"
  string label = 2;
  DataFlowTrigger trigger = 3;
  repeated DataFlowStep steps = 4;
  repeated StoreAccess stores = 5;
  string completeness = 6;             // "complete" | "partial"
  repeated DataFlowGap gaps = 7;       // lý do partial
  repeated string services = 8;        // service chạm tới, theo thứ tự đi qua
  repeated RelatedProcess related_processes = 9;   // làm giàu từ GitNexus
}
message DataFlowTrigger { string kind = 1; /* ws-channel|http|grpc|event|cron */ string name = 2; }
message DataFlowStep {
  int32 n = 1; ComponentRef from = 2; ComponentRef to = 3;
  string kind = 4;                     // call|rpc|event|db-read|db-write|ws-push
  string method = 5;                   // tên RPC, method agent, tên kênh hoặc subject
  SymbolRef symbol = 6;                // hàm thực hiện bước (nếu có)
  bool sync = 7; double confidence = 8; string origin = 9;   // static-fieldtype|static-name|process|declared
  repeated SymbolRef evidence = 10; string request_type = 11; string response_type = 12; // tên message proto
  bool unimplemented = 13;
}
message ComponentRef { string container = 1; string component_id = 2; string name = 3; string kind = 4; /* component|external|ui */ }
message StoreAccess { int32 step = 1; StoreRef store = 2; string table = 3; string op = 4; /* read|write */ double confidence = 5; }
message StoreRef { string id = 1; string kind = 2; /* postgres|mysql|queue|vault|... */ string name = 3; string schema = 4; }
message DataFlowGap { int32 after_step = 1; string code = 2; string message = 3; }
message RelatedProcess { string process_id = 1; string label = 2; int32 step_count = 3; string relation = 4; /* contains-symbol */ }
```

`ComponentRef.component_id` là `id` của `C4Component` (CR-CV-033) hoặc `C4External`; actor người dùng là `{kind:"ui", component_id:"ui"}`. Mã `DataFlowGap`: `RPC_UNIMPLEMENTED`, `DYNAMIC_DISPATCH`, `MULTIPLE_IMPLEMENTATIONS`, `DEPTH_LIMIT`, `CYCLE`, `UNRESOLVED_CLIENT`, `AGENT_METHOD_DYNAMIC`, `SERVICE_NOT_FOUND`.

### 2.2 RPC (tên theo README mục 3.6; message thuộc CR này)

```proto
message ListDataFlowsRequest { string repo_binding_id = 1; string ref = 2; string trigger_kind = 3; string query = 4; string service = 5; int32 page_size = 6; string page_token = 7; }
message ListDataFlowsResponse { repeated DataFlowSummary flows = 1; string next_page_token = 2; int32 total = 3; CodeIntelResultMeta meta = 4; }
message DataFlowSummary { string id = 1; string label = 2; DataFlowTrigger trigger = 3; string entry_service = 4; string entry_rpc = 5; int32 service_hops = 6; string completeness = 7; }

message GetDataFlowRequest {
  string repo_binding_id = 1; string flow_id = 2; string ref = 3;
  string dialect = 4;                  // postgres (mặc định) | mysql: chọn adapter cho cổng nhiều cài đặt
  int32 max_service_hops = 5;          // mặc định 4, tối đa 8
  int32 max_steps = 6;                 // mặc định 60, tối đa 200
  bool include_sequence = 7; bool include_dfd = 8;
  string detail = 9;                   // "service" | "component" (mặc định component)
}
message GetDataFlowResponse { DataFlow flow = 1; SequenceModel sequence = 2; DfdModel dfd = 3; CodeIntelResultMeta meta = 4; }
```

`ListDataFlows` liệt kê **cả hai loại seed** không dựng đầy đủ: mỗi kênh `wscompat` (~455 theo CR-CV-032) và mỗi RPC (548) là một ứng viên; `DataFlowSummary` chỉ cần đầu vào và số hop đã biết (rẻ, lấy từ catalog). Dựng chi tiết khi `GetDataFlow`. `ListDataFlows` hỗ trợ `trigger_kind` và lọc theo từ khoá.

### 2.3 Thuật toán dựng (`build_data_flow.go`)

Đầu vào: `ContractCatalog` (CR-CV-032), view C4 của các container liên quan (CR-CV-033, chỉ để gán `ComponentRef`), ERD (`accessedBy`, CR-CV-031), `CallIndex` của từng service (mục 2.4).

1. **Bước 0, UI → gateway.** Với seed kênh `c`: bước `kind=call`, `from=ui`, `to=api-gateway/adapter-wscompat`, `method=c.Name`, `sync` theo `Kind` (stream → `ws-push` ở bước đảo chiều, mục 8).
2. **Mỗi `WsTarget`** của `c`:
   - `Kind=rpc` → bước `rpc` từ `adapter-wscompat` sang `adapter-grpc` của `ImplementedBy(service)`, `method=Rpc`, `request_type/response_type` từ proto; rồi **mở rộng phía server** (bước 3).
   - `Kind=agent-method` (qua `Relay/RelayByDevServer`) → bước `rpc` tới `infra-fleet-service` (RPC `RelayByDevServer`), mở rộng server, rồi bước `rpc` tới external `ext-agent` với `method=<AgentMethod>`; kết thúc ở agent (CR này không đi vào TypeScript của agent; nhánh agent là điểm cuối).
   - `Kind=local` → không có bước; luồng `complete` với ghi chú "xử lý cục bộ ở gateway".
3. **Mở rộng phía server cho `(service S, rpc R)`:**
   1. Nếu `R.Unimplemented` → dừng nhánh, `DataFlowGap{RPC_UNIMPLEMENTED}`, bước đánh `unimplemented:true`, `completeness=partial`. (Đây là chỗ "dừng ở stub": handler không tồn tại. Ví dụ đã thấy 7 RPC ở `infra-fleet-service` theo CR-CV-032.)
   2. Handler `H` = phương thức server của `R`. Trong thân `H` tìm lời gọi tới trường/tham số có kiểu use case (`s.<field>.<Method>(…)`, trường kiểu `*usecase.<T>`): bước `call` `adapter-grpc → usecase`, `symbol=H`, `evidence` là vị trí lời gọi. Có thể có nhiều use case; giữ thứ tự xuất hiện.
   3. Trong thân phương thức use case `U.Method`, thu lời gọi tới trường có kiểu **interface cổng** (`uc.<field>.<Method>(…)` với `field` thuộc `U` và kiểu là interface khai báo ở `internal/usecase`): với mỗi cổng `P`, tra danh sách adapter cài đặt `P` từ CR-CV-033 (`implements`):
      - một adapter → đi tiếp;
      - nhiều adapter (Postgres và MySQL): chọn theo `dialect`, ghi `MULTIPLE_IMPLEMENTATIONS` nếu tham số rỗng mà có cả hai (mặc định `postgres`);
      - không có adapter nào tìm được → `DataFlowGap{UNRESOLVED_CLIENT/…}` và dừng nhánh.
      Bước `call` `usecase → adapter-<n>`, `method=<cổng>.<phương thức>`, `symbol` = phương thức adapter.
   4. Tại adapter: (a) **kho**: nếu phương thức adapter có trong `accessedBy` của ERD (CR-CV-031) → `StoreAccess{table, op}` và bước `db-read`/`db-write` tới external `database`; (b) **RPC kế tiếp**: nếu phương thức adapter là `From` của `RpcEdge` (CR-CV-032) tới `(S2, R2)` → bước `rpc` tới `S2`, đệ quy bước 3 với `(S2, R2)`; (c) **agent**: lời gọi `Exec`/`ExecStream` của client agent → bước `rpc` tới `ext-agent` (`method` là literal nếu có, nếu không `AGENT_METHOD_DYNAMIC`); (d) **sự kiện**: ghi vào cổng `OutboxWriter`/gọi `Publish` → bước `event` tới external `queue` (subject nếu có từ CR-CV-035, nếu không bỏ trống); (e) còn lại là hạ tầng nội bộ, không thêm bước.
   5. Giới hạn: `max_service_hops` (số lần đổi service, mặc định 4), `max_steps` (60), chống vòng bằng tập `(service,rpc)` đã đi (`CYCLE`). Vượt giới hạn → `DEPTH_LIMIT`, `partial`.
4. **Gán `ComponentRef`.** `symbol.filePath` → package → component của view C4 (CR-CV-033). Cần C4 đã dựng cho container tương ứng; không dựng C4 đầy đủ chỉ để lấy id: dùng riêng hàm gán theo đường dẫn của CR-CV-033 (không phụ thuộc `c4.yaml`; nếu có `c4.yaml` gộp component thì ánh xạ qua bảng `merge`).
5. **Làm giàu `Process`** (tuỳ chọn, nếu CR-CV-002 có `codeintel.processes/process`): với mỗi `symbol` trong bước, tra các `Process` chứa symbol đó (`STEP_IN_PROCESS`); đưa vào `related_processes` (không thêm bước, vì Process của Go không bao giờ tới use case/gRPC). Với kênh có đầu vào TypeScript, `RelatedProcess` cho phần frontend chỉ có khi tìm được symbol đầu vào; **chưa có cách nối kênh với Process frontend** (xem Q3).
6. Mỗi bước có `origin`: `static-fieldtype` (kiểu cú pháp, confidence 0,9), `static-name` (khớp theo tên, 0,6, nhãn "suy luận"), `declared` (từ catalog, 1,0).

### 2.4 `CallIndex` (mới, `internal/adapter/gocallindex`)

Chỉ mục theo `(service, commit)`, dựng một lần từ mọi file Go không-test dưới `internal/**` (đọc qua CR-CV-030; cache theo blob oid):

- bảng kiểu: với mỗi `struct`, trường → kiểu (`*usecase.RelayByDevServer`, `DevServerRepository`, `<pkg>v1.<Svc>Client`);
- bảng phương thức: `Receiver.Name` → danh sách lời gọi `recv.field.Method(...)`/`recv.Method(...)` (tên trường + tên phương thức + dòng);
- bảng interface: tên interface → tập phương thức (từ `usecase`);
- bảng kết quả khởi tạo: không dùng trong MVP.

Kết hợp CR-CV-032 (`RpcEdge` đã gắn client-call vào hàm bao) để không phân tích lại. Dung lượng: `infra-fleet-service` 207 file/~1,05 MiB (CR-CV-033); chỉ mục là cấu trúc nhỏ (tên, không nội dung). Phạm vi một lần `GetDataFlow` chỉ yêu cầu chỉ mục của các service nằm trên đường đi (thường 2–4 service).

### 2.5 Ví dụ có kiểm chứng một phần: kênh `accounts.selectClaude`

1. UI → `api-gateway/adapter-wscompat` (`accounts.selectClaude`, đăng ký bằng `registerAccountsRelay(r, client, "accounts.selectClaude", "accounts.selectClaude")`).
2. `adapter-wscompat → infra-fleet-service/adapter-grpc`: RPC `InfraFleetService.RelayByDevServer` (`request_type=RelayByDevServerRequest`, `response_type=RelayResponse`), tham số `Method` = chuỗi `agentMethod` (literal `"accounts.selectClaude"` lấy từ đối số của lời gọi hàm đăng ký).
3. `adapter-grpc → usecase`: `Server.RelayByDevServer` → `RelayByDevServer.Execute`.
4. `usecase → adapter-postgres` (hoặc `-mysql` theo `dialect`): cổng `DevServerRepository.Get` (đọc `infra.dev_servers`: có các câu `SELECT … FROM infra.dev_servers` ở `repository.go:65,96,137`, chưa xác định hàm); `StoreAccess{table:"dev_servers", op:"read"}`.
5. `usecase → adapter-devserveragent`: `DevServerAgentClient.IsConnected`, `Exec`; bước `rpc` → `ext-agent`, `method="accounts.selectClaude"`.
Luồng `complete` (không có `RPC_UNIMPLEMENTED`); điểm kết thúc là agent. Các bước 3–5 đã đọc code (ở mục 1.6); toàn bộ chuỗi chưa được chạy bằng công cụ.

### 2.6 Xuất mô hình sequence và DFD (không render)

Hàm thuần trong `export_flow_models.go`, đầu vào `DataFlow` + `detail`:

```proto
message SequenceModel {
  repeated SeqParticipant participants = 1;   // thứ tự xuất hiện đầu tiên
  repeated SeqMessage messages = 2;
}
message SeqParticipant { string id = 1; string label = 2; string kind = 3; /* ui|component|external */ string group = 4; /* service */ }
message SeqMessage { int32 n = 1; string from = 2; string to = 3; string label = 4; string kind = 5; bool sync = 6; bool dashed_return = 7; string note = 8; double confidence = 9; }

message DfdModel { repeated DfdNode nodes = 1; repeated DfdEdge edges = 2; }
message DfdNode { string id = 1; string label = 2; string kind = 3; /* ui|gateway|service|store|queue|external */ string group = 4; }
message DfdEdge { string from = 1; string to = 2; string label = 3; string kind = 4; repeated string data = 5; /* tên message proto / bảng */ int32 count = 6; }
```

Quy tắc:
- `detail="component"`: participant/node = `ComponentRef` (kèm nhóm = service); `detail="service"`: gộp component cùng container thành một participant/node, ẩn bước nội bộ cùng service (dùng cho DFD cấp cao).
- Sequence: một `SeqMessage` mỗi `DataFlowStep` (trừ bước nội bộ khi gộp); bước `rpc` đồng bộ có thêm `dashed_return=true` (phản hồi); `event` là bất đồng bộ; `unimplemented` có `note="RPC_UNIMPLEMENTED"`; bước `confidence < 0,8` có ghi chú "suy luận".
- DFD: node UI → gateway → service → store/queue/external; cạnh gộp theo `(from,to,kind)` với `label` là danh sách RPC/ kênh, `data` là tên message proto (request/response) và tên bảng cho cạnh kho.
- Kết quả là **mô hình**; CR-CV-056 sinh Mermaid và hiển thị. CR này không sinh chuỗi Mermaid.
- Ngân sách: ≤ 60 bước (mặc định), ≤ 40 participant; vượt thì `truncated` và `DEPTH_LIMIT`.

### 2.7 Cache và chi phí

Khoá `(repo, headCommit, flow_id, dialect, max_service_hops, detail, params_hash(file bẩn))` ở `graph_snapshots(view="dataflow")`. Chỉ mục `CallIndex` cache theo `(service, blob oids)`; hai bản `ContractCatalog`/ERD/C4 dùng lại từ cache của CR 031–033. `ListDataFlows` rẻ, chỉ cần catalog.

### 2.8 Phạm vi ngoài

Luồng sự kiện hai đầu (consumer theo topic; E10 thuộc CR-CV-035); workflow do `workflow-service` định nghĩa (E9 chưa xác nhận); luồng bên trong agent TypeScript; HTTP/cron ngoài MVP; luồng dùng `goroutine`/channel/hàm giá trị (ghi `DYNAMIC_DISPATCH`).

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Tự dựng luồng, `Process` chỉ để làm giàu | `Process` của Go có 0 bước ở usecase/adapter/gRPC (mục 1.2) |
| Duyệt theo kiểu trường/tham số qua tầng hexagonal | Đều đặn trong code đã đọc; không cần `go/types` |
| Chọn adapter theo `dialect` | Cổng thường có hai cài đặt (Postgres/MySQL); hiển thị cả hai sẽ làm luồng nhân đôi |
| Dừng ở `Unimplemented` và báo `partial` | Có RPC thật sự không có handler (CR-CV-032) |
| Xuất mô hình, không chuỗi Mermaid | Tách trách nhiệm với CR-CV-056 |
| `completeness` + `gaps` hiển thị rõ | Luồng suy luận không được trông như đầy đủ |
| Tên `DataFlow*` thay `FlowStep` | Tránh va chạm với `FlowSummary/FlowGraph` ở 05 §2.4 |
| Agent là điểm cuối | Đi vào TS của agent ngoài phạm vi |

## 4. Tiêu chí chấp nhận

- [ ] `ListDataFlows` trả ≥ 455 ứng viên kênh và 548 RPC (theo catalog), phân trang, lọc theo `trigger_kind`, không dựng chi tiết.
- [ ] `GetDataFlow("ws:accounts.selectClaude")` ra chuỗi 5 bước như 2.5, `completeness=complete`, bước cuối tới `ext-agent` với `method="accounts.selectClaude"`; `StoreAccess` có bảng `dev_servers` (`read`); chuyển `dialect=mysql` đổi adapter sang `adapter-mysql`.
- [ ] Kênh có đích RPC mà RPC `Unimplemented` (ví dụ gọi `ListFleetDefinitions` nếu có kênh) cho `completeness=partial`, mã `RPC_UNIMPLEMENTED`, `unimplemented:true`.
- [ ] Luồng đi qua ≥ 2 service (một handler gọi client sang service khác) nối đúng qua `RpcEdge` và dừng đúng ở `max_service_hops`; vòng gọi dừng với `CYCLE`.
- [ ] Mỗi bước có `confidence`, `origin`, `evidence`; bước `static-name` mang nhãn "suy luận".
- [ ] `SequenceModel`/`DfdModel` sinh đúng từ ví dụ ở cả hai mức `detail`; số participant/node khớp; DFD có `data` là tên message proto thật.
- [ ] Làm giàu `Process`: khi CR-CV-002 sẵn sàng, `related_processes` chỉ chứa `Process` thật chứa symbol của bước; khi không sẵn sàng, trường rỗng và luồng không lỗi.
- [ ] Kết quả có `headCommit`, `stale`, `truncated`; không chứa mã nguồn thân hàm.
- [ ] Hiệu năng: dựng 10 luồng mẫu với cache nóng < 1 s mỗi luồng (mục tiêu, chưa đo).
- [ ] Không tên `helpers`/`utils`/`common`/`misc`; không `max-lines` disable.

## 5. Kiểm thử

- **Unit:** duyệt kiểu trường (trường con trỏ, interface, tham số), chọn adapter theo `dialect`, chống vòng, giới hạn, ánh xạ `symbol → ComponentRef`, các mã `DataFlowGap`; xuất sequence/DFD ở hai `detail`.
- **Fixture vàng (CR-CV-070):** snapshot `DataFlow` cho 3 luồng cố định (`accounts.selectClaude`, một luồng hai service, một luồng `Unimplemented`) trên commit cố định; kèm kiểm tra `cypher` mẫu về `Process` (Go không có bước ở usecase/adapter) để phát hiện khi GitNexus đổi hành vi.
- **Hợp đồng GitNexus (nếu bật):** mẫu Cypher tra `Process` theo symbol: `MATCH (s)-[r:CodeRelation {type:'STEP_IN_PROCESS'}]->(p:Process) WHERE s.id = $id RETURN p.id, p.label, r.step` (cú pháp liệt kê bước theo `p.id` đã chạy thử; dạng tham số `s.id` chưa chạy; phải thử trước khi chốt).
- Chưa chạy bất kỳ test nào ở thời điểm viết.

## 6. Rủi ro và điểm chưa kiểm chứng

- Chuỗi trong 2.5 mới được đọc từng mảnh, chưa chạy bộ duyệt; nhiều phương thức use case có điều kiện/nhánh, gọi qua hàm trung gian, closure, hoặc `goroutine` nên sẽ thành `partial`.
- Tỉ lệ kênh dựng được đầy đủ chưa đo (CR-CV-032 chỉ có 352/455 kênh có lời gọi client trực tiếp).
- Gán `ComponentRef` phụ thuộc quy tắc C4 và `c4.yaml` gộp; thay `c4.yaml` làm đổi `component_id` trong luồng đã cache (khoá cache nên gồm `overrides_version`).
- Chỉ mục GitNexus có thể cũ hơn commit; `related_processes` luôn mang `stale`.
- Số liệu `Process` dựa trên chỉ mục ngày 2026-10-05 (index có thể đã đổi); các con số 0/1 cần chạy lại khi triển khai.
- Chưa xác định đường nối từ `Process` frontend tới kênh `wscompat`.
- `workflow-service` có thể có "workflow" dưới dạng dữ liệu (E9); nằm ngoài phạm vi.

## 7. Câu hỏi mở

- **Q1.** Cổng có hai cài đặt: luôn chọn một theo `dialect` hay hiện cả hai (nét đứt)?
- **Q2.** Luồng `event` có cần nối tới consumer (topic) ở CR này hay chờ E10/CR-CV-035?
- **Q3.** Có làm cầu nối `Process` frontend ↔ kênh (tìm `invoke('<channel>')` trong TypeScript) không? Cần CR agent/GitNexus riêng.
- **Q4.** Ngưỡng `max_service_hops` mặc định 4 có đủ?
- **Q5.** E9: workflow do `workflow-service` chạy lưu ở đâu để đưa vào luồng?

## 8. Tham chiếu

- `/opt/repos/orca/docs/crs/v7/README.md` (mục 3.4, 3.6, 3.9); `/opt/repos/orca/docs/research/view-code/08-views-and-review-models.md` (§4), `04-raw-data-and-pipeline.md` (§1.1 hình dạng `Process`), `09-external-inputs-required.md` (E8, E9, E10)
- `/opt/repos/orca/backend-go/services/api-gateway/internal/adapter/wscompat/channels_accounts.go`, `.../registry.go`, `.../push_bridge.go`
- `/opt/repos/orca/backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go` (hàm `RelayByDevServer`), `.../internal/usecase/relay_by_dev_server.go`, `.../internal/adapter/postgres/repository.go`
- GitNexus `cypher` đã chạy trên repo `orca` (2026-10-05): phân bố `Process` theo `entryPointId`; số `Process` có bước ở `/internal/usecase/`, `/adapter/postgres/`, `/adapter/mysql/` (0); bước ở đường dẫn chứa `grpc` (0); `proc_270_servehttp`, `proc_102_servehttp`; 718 cạnh `CALLS` tới `*_grpc.pb.go`
- CR liên quan: CR-CV-002, 020, 030, 031, 032, 033, 035, 036, 056, 070
