# BE-CV-TASK-086-01: Proto `ListCommitChecks` trong `scmintegration.proto`

**From Solution:** BE-CV-SOL-086-scm-commit-checks
**Priority:** P1
**Service:** `proto`
**File:** `backend-go/proto/orca/scmintegration/v1/scmintegration.proto` (sửa); stub sinh lại
**Depends on:** không
**Status:** [x] DONE

## Context
C-DM §2.4; SOL mục 2.B. Request thêm `tenant_id=1`, `include_steps`; `RateLimitInfo.limited` (additive, lệch hợp đồng, ghi PR).

## Việc cần làm
1. Thêm rpc và message cuối file; enum `CommitCheckRefKind`, `CommitCheckOverall` có `_UNSPECIFIED=0`; số field theo thứ tự khai báo.
2. `buf generate`.

## Kiểm thử
- `buf breaking --against '.git#branch=main,subdir=backend-go/proto'` gọi trực tiếp; `buf lint` cho phần mới.

## Tiêu chí hoàn thành
- [x] Chỉ thêm, không đổi số field cũ; stub biên dịch cho mọi service dùng module `proto`.

## Rủi ro
`make proto-lint` có `|| true`: không tin vào nó.
