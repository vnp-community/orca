# BE-CV-TASK-086-08: Proto `CiComparison`, `CiRunRef`, `RefreshCiRun*`

**From Solution:** BE-CV-SOL-086-ci-run-merge-and-comparison
**Priority:** P1
**Service:** `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_ci.proto` (mới); dòng `rpc RefreshCiRun` trong `codeintel_quality_gate.proto` (SOL-085)
**Depends on:** BE-CV-TASK-082-01
**Status:** [x] DONE

## Context
C-DM §2.1 #25, §3.2; hình dạng C-UI §4.7. `relation` là `string` (9 giá trị PQ-25).

## Việc cần làm
1. Message theo SOL mục 2.B; `import codeintel_quality.proto` cho `QualityRun`.
2. Thêm `rpc` khi file service của SOL-085 có; cập nhật bảng C-DM §3.2 trong PR hợp đồng.

## Kiểm thử
- `buf lint`, `buf breaking` gọi trực tiếp.

## Tiêu chí hoàn thành
- [x] Không file nào import `codeintel.proto`; không trùng tên.

## Rủi ro
Thứ tự với SOL-085.
