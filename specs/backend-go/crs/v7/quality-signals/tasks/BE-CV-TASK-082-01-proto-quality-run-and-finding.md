# BE-CV-TASK-082-01: Proto `QualityFinding`, `QualityStepResult`, `QualityRunSummary`, `QualityRun`

**From Solution:** BE-CV-SOL-082
**Priority:** P0
**Service:** `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_quality.proto` (mới)
**Depends on:** BE-CV-TASK-080-01 (`IndexBasis`), BE-CV-SOL-010
**Status:** [ ] TODO

## Context
C-DM §2.1 #18: số field theo CR-082 §2.1 cộng `error_code=19, tree_fingerprint=20, provider=21, external_ref=22, external_url=23, fetched_at=24, stale_after=25, dirty=26`. Chỉ message (RPC ở SOL-085). `dirty_fingerprint=15` và `tree_fingerprint=20` trùng nghĩa: điền cả hai (SOL-082 mục 7).

## Việc cần làm
1. Viết các message; `severity/category/status/source/scope` là `string`.
2. `external_ref` là `string` JSON (≤ 4 KiB) hoặc `google.protobuf.Struct`: chọn `string` để khớp cột JSON.
3. Không trường `line_text`/`source`/`snippet`.
4. `buf generate`.

## Kiểm thử
- `buf lint`, `buf breaking`; test phản chiếu cấm trường dòng nguồn; test đủ trường README 3.10.

## Tiêu chí hoàn thành
- [ ] Không trùng tên trong package; stub biên dịch.

## Rủi ro
Chốt số field trước khi SOL-085/086 dùng.
