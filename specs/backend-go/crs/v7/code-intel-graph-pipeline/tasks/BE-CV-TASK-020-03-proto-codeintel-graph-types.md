# BE-CV-TASK-020-03: Proto `codeintel_graph.proto` (kiểu đồ thị; chưa có request/response)

**From Solution:** BE-CV-SOL-020-canonical-graph-model
**Priority:** P0
**Service:** `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_graph.proto` (mới); stub sinh (mới)
**Depends on:** TASK-020-02
**Status:** [ ] TODO

---

## Context

Hợp đồng §2.1 hàng 3: CR-020 sở hữu **kiểu**; request/response của `GetStructure, GetClusterOverview, GetSubgraph, GetImpact, GetSymbol, GetRouteMap` do CR-021 thêm vào cùng file (BE-CV-TASK-021-07). Quy tắc "RPC chưa có message thì không khai báo" nên file này chưa có `rpc`/`service`.

## Việc cần làm

1. Tạo file import `codeintel_common.proto`; cùng `package`/`go_package` như task 02.
2. Khai báo mọi message ở solution 2.B (cụm, module, symbol, flow, impact, route) với số field như bảng; `SymbolEdge.sources` là `repeated ToolName`; `AffectedCluster.id` là `optional string` (null hợp lệ, PQ-19); `AffectedFlow.changed_step` là `optional int32`.
3. Thêm `SymbolDetail`, `RelatedSymbol`, `RelatedSymbolList`, `SymbolFlowRef`, `SymbolSource` (C8). `SymbolSource.text` ghi chú: chỉ xuất hiện trong response `GetSymbol`, không bao giờ vào cache (H8).
4. **Không** thêm `ChangeOverlay` hay `reserved` dải 10–39 (PQ-09). Không comment "reserved for CR-CV-0xx".
5. `buf generate`; cập nhật stub.

## Kiểm thử

- `cd backend-go/proto && buf lint && buf breaking --against '.git#branch=main,subdir=backend-go/proto'`.
- `go build ./...` trong `backend-go/proto`.
- Test reflection (mở rộng `proto_surface_test.go`): `ImpactGraph` **không** có field tên chứa `edge`; `ChangeOverlay` không tồn tại trong package; `FlowGraph.edges` kiểu `SymbolEdge`.

## Tiêu chí hoàn thành

- [ ] Mọi message của solution 2.B có mặt, số field khớp.
- [ ] `ImpactGraph` đủ `impacted_count`, `affected_clusters` kiểu `AffectedCluster`.
- [ ] `buf lint`/`buf breaking` xanh; stub build.

## Rủi ro và lưu ý

- `float confidence` vs `double cohesion` theo CR (Q5), chưa kiểm độ chính xác `cohesion`.
- Kích thước thực của `SymbolGraph` 1 500 nút/4 000 cạnh chưa đo; ngân sách 2 MiB là giả định (SOL-020 mục 6).
