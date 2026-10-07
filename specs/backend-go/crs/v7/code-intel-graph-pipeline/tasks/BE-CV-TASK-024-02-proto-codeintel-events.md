# BE-CV-TASK-024-02: Proto `codeintel_events.proto` và `rpc StreamCodeIntelEvents`

**From Solution:** BE-CV-SOL-024-event-distribution
**Priority:** P0
**Service:** `code-intel-service` · `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_events.proto` (mới), `codeintel.proto` (sửa)
**Depends on:** BE-CV-TASK-020-02
**Status:** [x] DONE

---

## Context

Hợp đồng §2.3: `StreamCodeIntelEventsRequest{selectors}`, `CodeIntelPush` 20 field; `percent` là `optional int32`.

## Việc cần làm

1. Khai báo message nguyên văn hợp đồng §2.3; import `codeintel_common.proto`.
2. Thêm `rpc StreamCodeIntelEvents(...) returns (stream CodeIntelPush)` vào `CodeIntelService`.
3. `buf generate`.

## Kiểm thử

- `buf lint && buf breaking --against '.git#branch=main,subdir=backend-go/proto'`.
- Test phản chiếu: `CodeIntelPush` đúng 20 field số 1–20; `percent` optional.

## Tiêu chí hoàn thành

- [x] buf xanh. - [x] Khớp §2.3.

## Rủi ro và lưu ý

- Tên trùng `orca.infrafleet.v1.StreamCodeIntelEventsRequest`: dùng alias import `fleetv1`/`codeintelv1`.
