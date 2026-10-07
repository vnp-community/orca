# BE-CV-SOL-032: `ContractCatalog` (proto, cạnh client→server gRPC, kênh `wscompat`) bằng trích xuất tĩnh

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Catalog là dữ liệu **nội bộ** (PQ-29: không RPC, không kênh WS); tiêu thụ bởi 033, 034, 038.

**CR:** [CR-CV-032](../../../../../../docs/crs/v7/code-intel-sources/CR-CV-032-proto-and-channel-contracts.md)
**Service:** `code-intel-service` — `proto/orca/codeintel/v1/codeintel_contract.proto` (mới, chỉ message), `internal/domain/contract`, `internal/usecase/build_contract_catalog.go`, `internal/adapter/protoschema`, `internal/adapter/gocallgraph` (đều mới)
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) ("gRPC conventions", "API Gateway responsibilities"), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md)

---

## 0. Hợp đồng áp dụng

| Mã | Áp dụng | Mục |
|---|---|---|
| PQ-07, PQ-29 | `codeintel_contract.proto` chứa `ServiceContract, RpcContract, RpcEdge, WsChannel, WsTarget, ContractWarning, ContractCatalog` — **không RPC**; `ServiceRef`, `SourceRef` ở common; `ContractWarning` riêng | §2.1 hàng 10 |
| PQ-30 | `ContractChange` (038) dùng catalog; kind `ws-channel`, `ws-channel-arg`, `route`… | §1 |
| PQ-20 | `SymbolRef.key` Go do backend dựng cùng quy tắc với agent | §1 |
| PQ-14 | Ngân sách ≤ 700 RPC, ≤ 3 000 cạnh, ≤ 800 kênh → `truncated`; view lưu `contractCatalog` | §1, §4 T3 |
| PQ-15 | `graph_snapshots(view="contractCatalog")` | §4 T3 |
| H7 | Cạnh T3 nhãn "suy luận"; `Unimplemented` là phát hiện | §0 |
| §8.3-3/4 | Cache có `tenant_id`, test cô lập; solution không có nhánh dialect riêng | §8.3 |

## 1. Trạng thái hiện tại (re-verify)

Đã đọc/chạy lệnh chỉ-đọc (2026-10-06): `ls backend-go/proto/orca/*/v1/*.proto` → **18** file; `grep -c '^\s*rpc '` tổng **548** (khớp CR); `backend-go/services/infra-fleet-service/internal/adapter/grpc/` ; `api-gateway/internal/adapter/wscompat/{registry.go, registry_channels.go (ChannelKind unary|stream|streamChannel|binaryStream), channels_accounts.go dòng 136–160 (registerAccountsRelay, tham số channel, agentMethod; closure gọi client.RelayByDevServer)}`; `grep PushEvent{Channel:` thấy `accounts.event`, `agent.statusChanged`, `agent:rateLimited` (kênh push có dấu `:`).

### Correction relative to CR-CV-032

| # | CR nói | Đọc được | Xử lý |
|---|---|---|---|
| C1 | `contract.proto` | PQ-07: `codeintel_contract.proto` | Đổi tên |
| C2 | Lộ catalog ra UI qua `GetRouteMap` hoặc `GetContractCatalog` (Q1) | PQ-29: message nội bộ, không RPC; UI chỉ thấy qua `GetContractDiff` (038) | Không thêm RPC |
| C3 | 461 lời gọi `r.Register*` (455 literal) | `grep -c "r.Register"` trên `channels*.go` (kể cả test) ra 464; chưa tách non-test | Tiêu chí dùng "≥ số do chính bộ trích xuất đếm", đối chiếu bằng test chứ không hằng cứng |
| C4 | Kênh push chỉ tên dạng `a.b` | Có `agent:rateLimited` | Tên kênh là chuỗi mở, không chuẩn hoá |
| C5 | `Registry.Channels()` là nguồn chính xác nhưng ở trong gateway (Q2) | Hàm tồn tại (`registry_channels.go`), chỉ trong tiến trình | Giữ trích tĩnh; dùng Channels() làm đối chiếu ở test của gateway (ngoài phạm vi) |

## 2. Giải pháp

### A. Cây file (mới)

```
proto/orca/codeintel/v1/codeintel_contract.proto
internal/domain/contract/catalog.go          # ContractCatalog, ServiceContract, RpcContract, RpcEdge, WsChannel, WsTarget, ContractWarning
internal/adapter/protoschema/{tokenizer.go,parser.go,message_shape.go}   # .proto → AST nhỏ (A: tự viết)
internal/adapter/gocallgraph/{server_registration.go,handler_detection.go,client_type_table.go,client_call_edges.go,ws_channel_extraction.go}
internal/usecase/build_contract_catalog.go
```

### B. Mô hình miền: theo CR 2.2 (`WsTarget.Kind ∈ rpc|agent-method|local`, `WsChannel.Kind ∈ unary|stream|streamChannel|binaryStream|push`, `Dynamic`, `Duplicate`; `RpcContract.Unimplemented`, `Handler *SymbolRef`, `Callers`). `ServiceRef{name, proto_service}` dùng kiểu của common. `MessageShape` chỉ dựng khi `includeShapes` (038).

### C. Ba tầng nguồn (T1 khai báo 1.0, T2 kiểu cú pháp 0.9, T3 theo tên 0.6 nhãn suy luận, T4 GitNexus tuỳ chọn tắt)
1. **protoschema**: tokenizer + đệ quy xuống; tập cú pháp `syntax/package/import/option/service/rpc(stream)/message lồng/enum/oneof/repeated/optional/comment` (không `map`, `extend`, `reserved` — theo CR, chưa chạy lại). Kiểm chéo ở CI với descriptor sinh sẵn (phương án D chỉ để đối chiếu, không làm nguồn).
2. **Server**: parse `cmd/server/main.go` tìm `<alias>.Register<Svc>Server`, ánh xạ alias qua import `…/proto/gen/go/orca/<pkg>/v1`; handler = `FuncDecl` có receiver trùng tên RPC trong `internal/adapter/grpc/*.go` (gồm receiver thứ hai như `AgentSessionListServer`); thiếu → `Unimplemented=true` + `RPC_UNIMPLEMENTED`; thân chỉ trả `codes.Unimplemented` → `Unimplemented` (suy luận).
3. **Client**: quét `internal/**` và `cmd/**` mọi service (không phải chỉ `adapter/grpcclient`): bảng kiểu (tham số/trường/biến kiểu `<alias>.<Svc>Client`), lời gọi `<recv>.<Rpc>(…)` → T2; còn lại khớp tên → T3. `Relay/RelayByDevServer` với `Method` literal sinh thêm đích `agent-method`.
4. **wscompat**: parse `wscompat/*.go` không-test; `r.Register|RegisterStream|RegisterStreamChannel|RegisterBinaryStreamHandler`; tên kênh literal, nối `"browser."+op` với `op` thuộc `range []string{…}` literal, hoặc truy ngược lời gọi hàm dùng chung (`registerAccountsRelay(r, client, "accounts.selectClaude", "accounts.selectClaude")`); đi vào thân hàm một cấp; kênh push từ `PushEvent{Channel:"…"}`; `Duplicate`→`CHANNEL_DUPLICATE`.
5. Quét hai bước để nằm trong `MAX_FILES_PER_CALL` của SOL-030: liệt kê file Go chứa `v1.` và `Client`, chỉ parse file đó; cache theo blob oid.

### D. Cache và quyền
Use case nội bộ (không handler gRPC): chỉ được gọi từ `BuildC4`, `BuildDataFlow`, `BuildContractDiff`; kiểm quyền đã làm ở RPC gọi. `graph_snapshots(view="contractCatalog")`, `params_hash` gồm tập `(path,ContentHash)` bẩn + `include_shapes`; mọi truy vấn snapshot có `tenant_id`.

## 3. Quyết định thiết kế

| Quyết định | Lý do |
|---|---|
| Trích riêng bằng `go/parser`, GitNexus chỉ đối chứng | `IMPLEMENTS` Go chỉ nhúng struct; `CALLS` khớp tên (CR 1.5) |
| Phân giải kiểu theo cú pháp, không `go/types` | Không có toàn module khi chỉ có file đọc qua RPC |
| Quét toàn `internal/**`+`cmd/**` | Client ở `scmstarcheck`, `serviceclients`, `main.go`, `wscompat` |
| Không lộ payload kênh | `args` chưa đối chiếu frontend (CR 1.4) |

## 4. Lệch giữa CR và hợp đồng
C1, C2 (mục 1); `ContractWarning` giữ riêng (PQ-29); không RPC mới (S8). Q3 parser: tự viết (O-7).

## 5. Phụ thuộc chéo khu vực
BE: `030` (đọc file), `020` (`SymbolRef`, `ServiceRef`), `012`, `022`; tiêu thụ bởi `033-c4-component-view` (`calls-rpc`), `034-data-flow-model`, `038-contract-diff` (`MessageShape`), `035` (tuỳ chọn), `070`. FE: không (không có kênh; `FE-CV-SOL-059` thấy qua 038). AG: không.

## 6. Tiêu chí chấp nhận
- [x] 18 service, 548 RPC (đếm độc lập bằng `grep` trong test), cờ streaming khớp (CR: 15 server-stream, 2 hai chiều — chưa kiểm).
- [x] `ImplementedBy` đúng 18/18 (`McpService`, `McpRegistryService` cùng `mcp-service`).
- [x] `infra-fleet-service`: 7 RPC `Unimplemented` như CR (hoặc số mới có giải thích); `ListAgentSessions` có handler.
- [x] Cạnh `workflow-service→automation-service` và `tenant-service→scm-integration-service` có `evidence` file:dòng; T3 có nhãn suy luận.
- [x] Kênh `accounts.*` → `agent-method` đúng; `host.wsl.isAvailable` → `local`; `CHANNEL_DUPLICATE` khi trùng; kênh `browser.*` khai triển.
- [x] Kết quả có `headCommit`, `stale`, `truncated`; không thân hàm; cache cô lập tenant; kiểm chéo descriptor không lệch; không `max-lines` disable.

## 7. Kiểm thử, rủi ro, câu hỏi mở
**Kiểm thử.** Unit parser proto, alias `Register`, `Unimplemented`, phân giải client, khai triển tên kênh; golden cho `usage-service`, `annotation-service`, `notification-service`, `infra-fleet-service` (CR-070); cô lập tenant ở cache; lệnh `go test ./services/code-intel-service/internal/adapter/{protoschema,gocallgraph}/... ./services/code-intel-service/internal/usecase/... -run Contract` (chưa chạy).
**Rủi ro.** Dương tính giả T3; 103 kênh chưa phân loại; kênh đăng ký bằng bảng cấu hình thành `Dynamic`; số 834/352/455 chưa kiểm tay; chi phí quét qua RTT SSH chưa đo.
**Hợp đồng thiếu (báo chủ).** G1: không nơi nào lộ catalog kênh ra UI (PQ-29 chấp nhận); G2: ngưỡng `confidence` hiển thị mặc định (Q5) chưa chốt.
**Mở.** Q4 CR: đối chiếu kênh "chết" với frontend.

## 8. Tham chiếu
`docs/crs/v7/code-intel-sources/CR-CV-032-*.md`; hợp đồng PQ-07/14/15/20/29/30; `backend-go/proto/orca/**`; `backend-go/services/*/cmd/server/main.go`; `backend-go/services/api-gateway/internal/adapter/wscompat/{registry_channels.go,channels_accounts.go}`.
