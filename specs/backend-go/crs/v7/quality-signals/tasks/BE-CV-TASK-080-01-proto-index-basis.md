# BE-CV-TASK-080-01: Proto `IndexBasis` (`codeintel_index_basis.proto`)

**From Solution:** BE-CV-SOL-080
**Priority:** P0
**Service:** `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_index_basis.proto` (mới); stub sinh bằng `buf generate`
**Depends on:** BE-CV-SOL-010 (cổng G0: thư mục `proto/orca/codeintel/v1`, `codeintel_common.proto`)
**Status:** [ ] TODO

## Context
C-DM §2.1 #5: file do CR-080 sở hữu, 13 trường theo CR-080 §2.3 (`tool, index_scope, freshness, indexed_commit, head_commit, merge_base, indexed_at, dirty_since_index, changed_files_not_in_index, refresh_state, trigger, tool_version, index_policy`). PQ-08: `index_scope ∈ exact|repo_root|stale|none`, `freshness ∈ fresh|fresh_base|stale|unknown`. Đã kiểm: `proto/orca/codeintel/` chưa tồn tại; chạy `ls` trước khi viết.

## Việc cần làm
1. Tạo message `IndexBasis` đúng 13 trường, số field 1..13 theo thứ tự CR-080 §2.3 (không đổi về sau); chuỗi (không enum) cho scope/freshness/refresh_state để thêm giá trị không phá tương thích.
2. `import "google/protobuf/timestamp.proto"`; `go_package` theo mẫu các proto khác.
3. Không khai báo RPC. Không import `codeintel.proto`.
4. `buf generate`; commit stub theo quy ước repo.

## Kiểm thử
- `buf lint` (STANDARD) và `buf breaking --against '.git#branch=main,subdir=backend-go/proto'` gọi trực tiếp (không `make proto-lint` có `|| true`).
- Test phản chiếu: đủ 13 trường, tên và số đúng.

## Tiêu chí hoàn thành
- [ ] lint/breaking xanh; stub biên dịch.
- [ ] `QualityRun.index_basis` (SOL-082) import được.

## Rủi ro
Số field phải chốt trước khi SOL-012/082 dùng; đổi sau là phá `buf breaking`.
