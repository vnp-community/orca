# BE-CV-TASK-033-02: `codeintel_c4.proto` và RPC `GetArchitecture`

**From Solution:** BE-CV-SOL-033-c4-component-view
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_c4.proto` (mới); `.../codeintel.proto` (thêm `rpc GetArchitecture`)
**Depends on:** `codeintel_common.proto`, `codeintel.proto` (G0)
**Status:** [x] DONE

---

## Context

Solution mục 2.A; số field theo CR §2.6; request/response điều chỉnh theo PQ-04/§3.1.

## Việc cần làm

1. Message `ContainerRef`, `C4ComponentView`, `C4Component`, `C4Relation`, `C4External`, `C4Warning` (số CR).
2. `GetArchitectureRequest{selector=1, container=2, include_hidden=3, if_none_match=4}`; `GetArchitectureResponse{meta=1, containers=2, view=3}` (đề xuất; ghi G2 để chủ hợp đồng chấp nhận **trước** merge).
3. Thêm `rpc GetArchitecture` vào `CodeIntelService`.
4. **Chưa** thêm `GetC4Overrides*/SaveC4Overrides*` (task 08 thêm vào cùng file; tránh va chạm PR).
5. `buf generate`.

## Kiểm thử

`cd backend-go/proto && buf lint && buf breaking --against '.git#branch=main,subdir=backend-go/proto'` (chưa chạy); `go build ./proto/...`.

## Tiêu chí hoàn thành

- [x] `buf` xanh; không `repo_binding_id`.
- [x] Tiền tố `C4*` (trừ `ContainerRef`).

## Rủi ro và lưu ý

- Số field đề xuất; sau merge bị `buf breaking` khoá.
