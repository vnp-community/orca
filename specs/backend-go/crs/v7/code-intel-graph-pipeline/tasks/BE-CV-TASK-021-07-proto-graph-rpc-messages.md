# BE-CV-TASK-021-07: Proto request/response cho 6 RPC đồ thị và dòng `rpc` trong `codeintel.proto`

**From Solution:** BE-CV-SOL-021-agent-collector
**Priority:** P0
**Service:** `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_graph.proto` (sửa), `codeintel.proto` (sửa)
**Depends on:** BE-CV-TASK-020-03, SOL-010
**Status:** [ ] TODO

---

## Context

Hợp đồng §2.1/§3.1; `selector` là trường 1; chỉ khai báo RPC có message.

## Việc cần làm

1. Thêm message ở 2.E của solution; `GetSubgraphRequest.center` và `GetImpactRequest.target` dùng `oneof`.
2. Thêm `rpc GetStructure|GetClusterOverview|GetSubgraph|GetImpact|GetSymbol|GetRouteMap` vào `service CodeIntelService`.
3. `buf generate`.

## Kiểm thử

- `buf lint && buf breaking --against '.git#branch=main,subdir=backend-go/proto'` trong `backend-go/proto`.
- Cập nhật bảng hợp đồng §3.1 trong cùng PR hợp đồng nếu số field khác.

## Tiêu chí hoàn thành

- [ ] buf xanh. - [ ] Mọi request có `selector=1`.

## Rủi ro và lưu ý

- Tên `GetSymbol` trả `SymbolDetail` (SOL-020 C8).
