# AG-CV-TASK-083-04: Tính diff coverage, `uncoveredRanges`, tổng, cắt payload

**From Solution:** [AG-CV-SOL-083-coverage-collection](../solutions/AG-CV-SOL-083-coverage-collection.md) mục 5.3
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-diff-coverage.ts` (mới), `.test.ts` (mới)
**Depends on:** AG-CV-TASK-083-02, 083-03
**Status:** [x] DONE

## Context

Quy tắc K3/K4 của CR-083 2.5 và hợp đồng §5.6.

## Việc cần làm

1. `computeDiffCoverage(blocksByFile, added, opts) → { diff: {...}; files: FileCoverage[] }` theo 5.3: `changedExecutable`, `covered`, `uncovered`, `diffCoverage|null`, `reason`, `uncoveredRanges` (gộp dòng liền kề), `excludedFiles[{path,reason}]` (`test`, `generated`, `doc`, `no_coverage_blocks`, `not_go`).
2. `totals {stmts, covered, pct}` theo module đã đo, `noTests[]` loại khỏi tổng.
3. `shapeReportPayload(report) → { report; truncated; totalCount }`: ≤ 2 000 tệp và ≤ 1 MiB JSON, cắt theo `pct` thấp trước; `uncoveredRanges` chỉ cho tệp đã đổi hoặc `pct < 0.5`.
4. `source:"measured"` cố định; `mode` từ profile; `toolVersions {go}`.

## Kiểm thử

Bảng ca: thêm hàm có test/không test/sửa chú thích/chỉ xoá; khối chồng nhau (một phủ, một không → chưa phủ); mẫu số 0 → null; tệp không khối; > 2 000 tệp; payload > 1 MiB. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-diff-coverage.test.ts`.

## Tiêu chí hoàn thành

- [x] Mọi ca khớp bảng K3/K4; không `100%` giả.

## Rủi ro

Định nghĩa K4 khác dịch vụ ngoài: ghi trong tài liệu hàm.
