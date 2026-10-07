# BE-CV-SOL-034: Dựng `DataFlow` qua ranh giới service (kênh/RPC → handler → use case → cổng → adapter → kho/RPC kế tiếp), xuất `SequenceModel` và `DfdModel`

> **📋 Proposed.** Chưa triển khai, chưa chạy test nào. Cần catalog (032), C4 `ComponentRef` (033), ERD `accessedBy` (031), đọc file (030).

**CR:** [CR-CV-034](../../../../../../docs/crs/v7/code-intel-sources/CR-CV-034-data-flow-model.md)
**Service:** `code-intel-service` — `proto/orca/codeintel/v1/codeintel_dataflow.proto` (mới) + `rpc ListDataFlows`, `rpc GetDataFlow`; `internal/domain/dataflow`, `internal/usecase/{build_data_flow,export_flow_models,list_data_flows}.go`, `internal/adapter/gocallindex`, handler gRPC
**TDD tham chiếu:** [`arch/02`](../../../../tdd/architecture/02-microservices-decomposition.md), [`arch/03`](../../../../tdd/architecture/03-clean-architecture-guidelines.md), [`arch/08`](../../../../tdd/architecture/08-inter-service-communication.md) ("Talking to the Dev Server Agent (execution plane)"), [`arch/09`](../../../../tdd/architecture/09-observability-reliability.md)

---

## 0. Hợp đồng áp dụng
| Mã | Áp dụng | Mục |
|---|---|---|
| PQ-04/07/12/14/15 | `selector`; file `codeintel_dataflow.proto`; `ResultMeta`; ≤ 2 MiB; views `dataflows`, `dataflow` | §1, §4 T3 |
| PQ-29 | `ComponentRef`, `StoreAccess`, `StoreRef`, `DataFlowGap` định nghĩa ở file này; `TableAccess` của 031 giữ riêng | §1 |
| §2.1 hàng 12 | Danh sách message: `DataFlow, DataFlowTrigger, DataFlowStep, ComponentRef, StoreAccess, StoreRef, DataFlowGap, RelatedProcess, DataFlowSummary, SequenceModel, SeqParticipant, SeqMessage, DfdModel, DfdNode, DfdEdge`; số field theo CR §2.1/2.2/2.6 | §2.1 |
| §3.1 | `ListDataFlows(trigger_kind, query, service, page_size ≤ 100, page_token) → {flows[], next_page_token, total, meta}`; `GetDataFlow(flow_id, dialect, max_service_hops ≤ 8, max_steps ≤ 200, include_sequence, include_dfd, detail)`; OPA `read`; kênh `codeIntel.dataFlows`, `codeIntel.dataFlow` | §3.1 |
| UI §4.4 | Hình JSON `DataFlow`, `DataFlowStep`, `SequenceModel`, `DfdModel` | `CONTRACT-ui-api` §4.4 |
| PQ-13 | Singleflight/`inProgress` khi > 20 s | §1 |
| README v7 mục 8 điểm 9 | `Process` GitNexus Go không tới use case/adapter/gRPC; chỉ làm giàu | README v7 |
| §8.3-3/4 | Test hai dialect cho chọn adapter Postgres/MySQL; cache cô lập tenant | §8.3 |

## 1. Trạng thái hiện tại (re-verify)
Đã đọc: `backend-go/services/api-gateway/internal/adapter/wscompat/channels_accounts.go` dòng 136–160 (closure gọi `client.RelayByDevServer`, `agentMethod` truyền vào); `backend-go/services/infra-fleet-service/internal/usecase/relay_by_dev_server.go` (`uc.devServers.Get`, `uc.agent.IsConnected`, `uc.agent.Exec`; trường `devServers DevServerRepository`, `agent DevServerAgentClient`); `…/devserveragent/client.go` `Exec` (dòng 424–449); `CONTRACT-agent-rpc` §4.3 (`codeintel.processes|process` tồn tại, kết quả ở phong bì).

### Correction relative to CR-CV-034
| # | CR nói | Hợp đồng/mã | Xử lý |
|---|---|---|---|
| C1 | `dataflow.proto`; `repo_binding_id`; `ref` | PQ-07 `codeintel_dataflow.proto`; PQ-04 `selector`; `ref` ngầm bỏ | `ListDataFlowsRequest{selector=1, trigger_kind, query, service, page_size, page_token, if_none_match}`; luồng dựng ở `HEAD` hoặc cây làm việc (`dirty` trong meta) |
| C2 | `DataFlowSummary`, `total` | §3.1 khớp | Giữ |
| C3 | `DataFlowStep.symbol` chỉ `SymbolRef` | UI §4.4 thêm `evidence`, `requestType`, `responseType`, `unimplemented` | Đã có trong CR (số 10–13) |
| C4 | `ComponentRef.kind` `component|external|ui` | UI §4.4 khớp | Giữ |
| C5 | Nối `Process` GitNexus cho phần TS (Q3) | Không có đường nối kênh↔Process | `related_processes` chỉ theo symbol Go/TS có trong bước; không đoán |
| C6 | Cạnh `event` tới consumer (Q2) | Topic thuộc 035 (`topics[]`) | Bước `event` tới `ext-queue`, `method` = subject nếu 035 có, nếu không rỗng; không nối consumer |
| C7 | Cổng nhiều cài đặt (Q1) | Chưa có phán quyết | Chọn theo `dialect` (mặc định `postgres`), `MULTIPLE_IMPLEMENTATIONS` khi rỗng mà có cả hai (CR) |

## 2. Giải pháp
### A. Proto (`codeintel_dataflow.proto`, mới)
Message theo CR §2.1/§2.2/§2.6 (số field của CR là chuẩn); request/response điều chỉnh C1; thêm `rpc ListDataFlows`, `rpc GetDataFlow` vào `CodeIntelService`. `DataFlowGap.code ∈ RPC_UNIMPLEMENTED, DYNAMIC_DISPATCH, MULTIPLE_IMPLEMENTATIONS, DEPTH_LIMIT, CYCLE, UNRESOLVED_CLIENT, AGENT_METHOD_DYNAMIC, SERVICE_NOT_FOUND` (chuỗi mở).

### B. Cây file (mới)
```
internal/domain/dataflow/{flow.go,gap_codes.go,limits.go}
internal/adapter/gocallindex/{type_table.go,method_calls.go,interface_table.go}   # chỉ mục (service, commit): trường→kiểu, lời gọi recv.field.Method, interface→phương thức
internal/usecase/list_data_flows.go          # ứng viên từ catalog, không dựng chi tiết
internal/usecase/build_data_flow.go          # duyệt tầng hexagonal, giới hạn, chống vòng
internal/usecase/component_ref_assignment.go # symbol.filePath → component C4
internal/usecase/export_flow_models.go       # SequenceModel, DfdModel (thuần)
internal/adapter/grpc/data_flow_handlers.go
```
### C. Thuật toán (CR 2.3; điều chỉnh)
0. UI→gateway: bước `call` `ui → api-gateway/adapter-wscompat` (`method`=tên kênh).
1. Mỗi `WsTarget`: `rpc` → bước `rpc` tới `adapter-grpc` của `ImplementedBy`; `agent-method` → bước tới `infra-fleet-service` (`RelayByDevServer`) rồi bước `rpc` tới `ext-agent` (`method`=literal; kết thúc ở agent, không đi vào TS); `local` → luồng `complete` ghi chú xử lý cục bộ.
2. Phía server `(S,R)`: `Unimplemented` → dừng, `RPC_UNIMPLEMENTED`, `partial`; handler → trường kiểu `*usecase.<T>` → phương thức use case → trường kiểu interface cổng → adapter cài đặt cổng (từ `implements` của 033; chọn theo `dialect`; không tìm được → `UNRESOLVED_CLIENT`); tại adapter: (a) `accessedBy` (031) → `StoreAccess` + bước `db-read|db-write`; (b) `RpcEdge` (032) → đệ quy `(S2,R2)`; (c) `Exec/ExecStream` → `ext-agent` (`AGENT_METHOD_DYNAMIC` nếu không literal); (d) `OutboxWriter`/`Publish` → bước `event`; (e) còn lại hạ tầng, không bước.
3. Giới hạn `max_service_hops` (mặc định 4, ≤ 8), `max_steps` (60, ≤ 200), `CYCLE` bằng tập `(service,rpc)`, `DEPTH_LIMIT`.
4. `ComponentRef` theo đường dẫn → component (hàm của 033, không phụ thuộc `c4.yaml`; có `c4.yaml` gộp thì ánh xạ qua bảng `merge`).
5. Làm giàu `Process` tuỳ chọn qua collector (`codeintel.processes/process`); thiếu → rỗng, không lỗi; luôn mang `stale`.
6. `origin`: `static-fieldtype` (0,9), `static-name` (0,6, nhãn suy luận), `declared` (1,0), `process`.
### D. Xuất mô hình (hàm thuần)
`detail=component|service`; sequence: một `SeqMessage` mỗi bước (gộp khi `service`), `dashed_return` cho `rpc` đồng bộ, `event` bất đồng bộ, `unimplemented` → `note=RPC_UNIMPLEMENTED`, `confidence<0,8` → ghi chú suy luận; DFD: node ui/gateway/service/store/queue/external, cạnh gộp `(from,to,kind)` với `data` = tên message proto/bảng. Ngân sách 60 bước/40 participant → `truncated`.
### E. Cache
`graph_snapshots(view="dataflow")`/`dataflows`: khoá `(tenant, binding, view, head_commit, params_hash)`; `params_hash` gồm `flow_id, dialect, max_service_hops, max_steps, detail, include_*, overrides_version, tập file bẩn`. Chỉ mục `gocallindex` cache theo `(service, blob oids)`; `ListDataFlows` chỉ cần catalog (rẻ).

## 3. Quyết định thiết kế
| Quyết định | Lý do |
|---|---|
| Tự dựng luồng, `Process` chỉ làm giàu | 0 Process Go có bước ở usecase/adapter/gRPC (README v7 mục 8 điểm 9) |
| Duyệt theo kiểu trường/tham số | Đều đặn; không cần `go/types` |
| Chọn adapter theo `dialect` | Tránh nhân đôi luồng |
| Dừng ở `Unimplemented` | RPC thật sự thiếu handler |
| Xuất mô hình, không Mermaid | 056 (FE) render |
| `completeness` + `gaps` luôn hiển thị | Luồng suy luận không được trông đầy đủ |

## 4. Lệch giữa CR và hợp đồng
C1–C7; `dataflow.proto`→`codeintel_dataflow.proto`; `repo_binding_id`→`selector`; kênh 2 (`dataFlows`, `dataFlow`) theo §3.1.

## 5. Phụ thuộc chéo khu vực
BE: `030`, `031-erd-model` (`accessedBy`), `032` (catalog, `RpcEdge`, `Unimplemented`), `033-c4-component-view` (`implements`, `ComponentRef`), `035` (subject, tuỳ chọn), `021` (collector `codeintel.processes`), `022`, `012/013`; tiêu thụ `036` (luồng bị ảnh hưởng, tuỳ chọn), `040-codeintel-view-channels`. FE: `FE-CV-SOL-056-dataflow-lens`. AG: `AG-CV-SOL-002-gitnexus-extraction` (processes, tuỳ chọn).

## 6. Tiêu chí chấp nhận
- [x] `ListDataFlows` trả ứng viên kênh + RPC theo catalog (số khớp đếm độc lập), phân trang, lọc, không dựng chi tiết.
- [x] `GetDataFlow("ws:accounts.selectClaude")`: chuỗi UI→wscompat→`RelayByDevServer`→usecase→`DevServerRepository.Get` (`StoreAccess dev_servers read`)→`ext-agent method=accounts.selectClaude`; `complete`; `dialect=mysql` đổi adapter sang MySQL.
- [x] RPC `Unimplemented` → `partial`, `RPC_UNIMPLEMENTED`, `unimplemented:true`.
- [x] Luồng ≥ 2 service dừng đúng `max_service_hops`; vòng → `CYCLE`.
- [x] Mọi bước có `confidence`, `origin`, `evidence`; `static-name` nhãn suy luận.
- [x] Sequence/DFD đúng ở hai `detail`; `data` DFD là tên message/bảng thật.
- [x] Không `Process` → trường rỗng không lỗi; kết quả `headCommit`, `stale`, `truncated`; cô lập tenant; không `max-lines` disable.

## 7. Kiểm thử, rủi ro, câu hỏi mở
**Kiểm thử.** Unit duyệt kiểu trường, chọn adapter theo dialect (hai dialect), chống vòng, giới hạn, mã gap, xuất mô hình; golden (CR-070) cho ba luồng cố định (`accounts.selectClaude`, hai service, `Unimplemented`); hiệu năng 10 luồng nóng < 1 s (mục tiêu, chưa đo); `go test ./services/code-intel-service/internal/{domain/dataflow,adapter/gocallindex,usecase}/...` (chưa chạy).
**Rủi ro.** Chuỗi 2.5 mới đọc từng mảnh, chưa chạy; closure/goroutine/hàm trung gian → `partial`; tỉ lệ kênh dựng đầy đủ chưa đo (CR 032: 352/455 có client trực tiếp); `ComponentRef` đổi theo `c4.yaml` → khoá cache có `overrides_version`.
**Hợp đồng thiếu.** G1: `ListDataFlowsRequest`/`GetDataFlowRequest` chưa có số field trong hợp đồng; G2: `ComponentRef.container` — id container (tên thư mục service) chưa định nghĩa; G3: quy tắc ghép `ext-*` id giữa 033 và 034 (hiện theo CR 033 §2.4).
**Mở.** Q1 (hai cài đặt), Q3 (Process frontend↔kênh), Q4 (`max_service_hops` 4), Q5 (E9).

## 8. Tham chiếu
`docs/crs/v7/code-intel-sources/CR-CV-034-*.md`; hợp đồng PQ-04/07/12–15/29, §2.1, §3.1, UI §4.4, agent §4.3; `backend-go/services/api-gateway/internal/adapter/wscompat/channels_accounts.go`; `backend-go/services/infra-fleet-service/internal/usecase/relay_by_dev_server.go`.
