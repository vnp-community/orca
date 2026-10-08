# FE-CV-TASK-087-09: Mô hình marker thuần và quy tắc đủ điều kiện theo `DiffSource`

**From Solution:** [FE-CV-SOL-087-quality-diff-annotations](../solutions/FE-CV-SOL-087-quality-diff-annotations.md) mục 2.2, 2.3
**Priority:** P0
**Area:** frontend / pure functions
**File:** `frontend/src/renderer/src/components/editor/quality-annotations/quality-marker-model.ts`, `quality-annotation-eligibility.ts` (mới) và `*.test.ts`
**Depends on:** 087-01 (kiểu `QualityFinding`)
**Status:** [x] DONE (verified 2026-10-07: quality-marker-model.test.ts + quality-annotation-eligibility.test.ts, 16/16 pass; oxlint + tsc sạch)

## Context

- Hợp đồng §4.7: `line,endLine,column,endColumn` (quy ước cột chưa nêu), `severity`, `tool`, `ruleId`, `fixHint?`.
- `DiffSource` ở `store/slices/editor.ts:100` (8 giá trị). Phía modified của từng loại **chưa kiểm chứng**: task này phải đọc nơi dựng `modifiedContent` (`EditorContent.tsx`, `ChangesModeView.tsx`, `CombinedDiffViewer.tsx`/`DiffSectionItem.tsx`) và ghi kết luận vào bảng.
- Dùng enum `monaco.MarkerSeverity` (import từ `@/lib/monaco-setup` như `DiffViewer.tsx`); `buildQualityMarkers` nhận `MarkerSeverity` qua tham số để test không cần Monaco.

## Việc cần làm

1. `buildQualityMarkers(findings, {lineCount, lineMaxColumn, severityMap})` → `{markers: IMarkerData-like[], glyphs: {line, severity, fingerprint}[]}` theo 2.2: kẹp dòng, `endLine<line`, cột hợp lệ hay cả dòng, mức tệp → dòng 1, `unknown`→Hint, `message` + `fixHint`, `source`, `code`; gộp nhiều phát hiện cùng dòng cho glyph (mức cao nhất, đếm).
2. `isAnnotationEligible({diffSource, runHeadCommit, currentHead, compareHeadOid?, hasWorktreeId})` trả `{eligible, softWarning?: 'dirty'|'changed-during-run'}` + lý do; bảng khởi đầu: `unstaged` đủ điều kiện; `branch`/`combined-branch` chỉ khi `compareHeadOid` = `runHeadCommit`; `staged`, `commit`, `combined-commit` không; `combined-all|combined-uncommitted` theo từng section (chốt sau khi đọc code). Không `worktreeId` → không đủ.
3. Ghi kết luận đọc code vào mô tả PR.

## Kiểm thử

Marker: mức, kẹp, `endLine<line`, `column` ngoài khoảng, mức tệp, 1 000 phát hiện (thời gian ngưỡng rộng), không ném với dữ liệu lạ; eligibility: ma trận 8 `DiffSource` × head khớp/không × dirty. Chạy `pnpm --filter orca-frontend test -- src/renderer/src/components/editor/quality-annotations/quality-`.

## Tiêu chí hoàn thành

- [ ] Hai hàm thuần có test; bảng `DiffSource` có kết luận từ đọc code.
- [ ] Không import Monaco ở `quality-marker-model.ts`.

## Rủi ro

- Nếu `staged`/`commit` cần ánh xạ dòng thì chưa hỗ trợ (SOL câu hỏi mở 4).

## Ghi chú triển khai (2026-10-07)

- `quality-marker-model.ts` không import Monaco (nhận `MarkerSeverity` qua tham số). Cột theo D5 (1-based, 0 = cả dòng). Bảng đủ điều kiện nằm trong `quality-annotation-eligibility.ts` (suy ra từ cách dựng phía modified của từng `DiffSource`).
