# AG-CV-TASK-005-07: Method `codeintel.detectChanges` và ngân sách thời gian

**From Solution:** [AG-CV-SOL-005-detect-changes](../solutions/AG-CV-SOL-005-detect-changes.md) mục 2.2
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-detect-changes.ts`, `codeintel-method-table.ts` (sửa), `codeintel-detect-changes.test.ts` (mới)
**Depends on:** [004](./AG-CV-TASK-005-04-hunk-to-symbol-mapping.md), [005](./AG-CV-TASK-005-05-affected-flows-and-clusters.md), [006](./AG-CV-TASK-005-06-gitnexus-detect-changes-header-crosscheck.md)
**Status:** [ ] TODO

## Context
Contract §4.8, timeout agent 55 s (`ORCA_CODEINTEL_DETECT_TIMEOUT_MS` hạ về 25 s nếu Go chưa nâng); `warnings` ở phong bì (PQ-19); `data.index {commit,stale,driftedFileCount,mappingConfidence}`.

## Việc cần làm
1. `validate` (`base`,`head` qua `assertGitRef`; `includeUntracked`, `withClusters`, `crossCheck`); ghép các bước; `deadline` chia ngân sách: git (~1 s), ánh xạ, luồng, cụm; hết giờ -> phần đã có + `deadline_partial`.
2. Dựng `data` đúng ví dụ §4.8; `totalCount` = symbol trước cắt; đăng ký bảng (55 s).
3. Windows/`unsupported_platform` theo lõi.

## Kiểm thử
Toàn luồng repo tạm + `gitnexus` giả: bẩn, untracked, hết ngân sách (đồng hồ giả), unborn, `crossCheck`; khoá lạ; `base` `-x`.
Lệnh: `pnpm exec vitest run src/relay/codeintel-detect-changes.test.ts`

## Tiêu chí hoàn thành
- [ ] Các tiêu chí solution mục 5.

## Rủi ro
- Go cắt ở 30 s nếu BE-023 chưa nâng timeout.
