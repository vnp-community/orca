# BE-CV-TASK-034-02: `codeintel_dataflow.proto` và RPC `ListDataFlows`/`GetDataFlow`

**From Solution:** BE-CV-SOL-034-data-flow-model
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_dataflow.proto` (mới); `.../codeintel.proto` (thêm 2 rpc)
**Depends on:** `codeintel_common.proto` (G0)
**Status:** [x] DONE

---

## Context

Solution 2.A; PQ-29 giữ `StoreAccess` riêng khỏi `TableAccess`; số field CR §2.1/2.2/2.6.

## Việc cần làm

1. Message theo CR (`DataFlow`(1–9), `DataFlowTrigger`, `DataFlowStep`(1–13), `ComponentRef`, `StoreAccess`, `StoreRef`, `DataFlowGap`, `RelatedProcess`, `DataFlowSummary`, `SequenceModel`, `SeqParticipant`, `SeqMessage`, `DfdModel`, `DfdNode`, `DfdEdge`).
2. `ListDataFlowsRequest{selector=1, trigger_kind=2, query=3, service=4, page_size=5, page_token=6, if_none_match=7}`; `ListDataFlowsResponse{flows, next_page_token, total, meta}`; `GetDataFlowRequest{selector=1, flow_id=2, dialect=3, max_service_hops=4, max_steps=5, include_sequence=6, include_dfd=7, detail=8, if_none_match=9}`; `GetDataFlowResponse{flow, sequence, dfd, meta}` (số đề xuất, G1).
3. `StreamRef` không cần; `ComponentRef.component_id` = id của `C4Component`/`C4External`.
4. `buf generate`.

## Kiểm thử

`cd backend-go/proto && buf lint && buf breaking --against '.git#branch=main,subdir=backend-go/proto'` (chưa chạy); `go build ./proto/...`.

## Tiêu chí hoàn thành

- [x] `buf` xanh; không `repo_binding_id`; tên không trùng package.

## Rủi ro và lưu ý

- Số field request/response đề xuất; chủ CR-034 xác nhận trước merge.
