# BE-CV-TASK-092-02: Proto `codeintel_requirement_trace.proto` và ba RPC

**From Solution:** BE-CV-SOL-092-requirement-trace
**Priority:** P2
**Service:** `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_requirement_trace.proto` (mới); `codeintel_quality_gate.proto` (thêm rpc)
**Depends on:** BE-CV-TASK-085-02
**Status:** [x] DONE

## Việc cần làm
1. `RequirementTrace`, `Requirement`, `RequirementEvidence` theo ui-api §4.7; `GetRequirementTrace`, `ConfirmRequirementEvidence` (`link_kind` enum `CONFIRM|REJECT` + `_UNSPECIFIED`), `LinkWorktreeTask` (`task_id` rỗng = gỡ).
2. `selector` field 1; `buf lint/breaking`.

## Kiểm thử / Tiêu chí hoàn thành
- [x] lint+breaking xanh; [ ] enum chữ thường ở dạng chuỗi dây (PQ-32).

## Rủi ro
- Hình dạng `unlinkedChanges` còn mở cho CR-087.
