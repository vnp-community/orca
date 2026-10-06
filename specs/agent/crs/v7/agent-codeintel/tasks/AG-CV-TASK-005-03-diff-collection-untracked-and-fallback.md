# AG-CV-TASK-005-03: Thu thập diff, tệp bẩn, untracked, fallback `maxBuffer`

**From Solution:** [AG-CV-SOL-005-detect-changes](../solutions/AG-CV-SOL-005-detect-changes.md) mục 2.2
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-diff-collection.ts`, `codeintel-diff-collection.test.ts` (mới)
**Depends on:** [001](./AG-CV-TASK-005-01-merge-base-resolution.md), [002](./AG-CV-TASK-005-02-diff-hunk-parser.md)
**Status:** [ ] TODO

## Context
Không `head`: so `<mergeBase>` với cây làm việc (bỏ `<headOid>`); untracked ≤ 200, ≤ 2 MiB/tệp, NUL trong 8 KiB đầu -> `binary`; diff lớn -> chỉ `--raw`+`--numstat`.

## Việc cần làm
1. `collectDiff(range, opts)` -> `changedFiles[]` (path, oldPath, status, additions, deletions, hunks, untracked, binary), `dirtyFiles`, cờ cảnh báo; 5 000 tệp tối đa (đếm phần còn lại).
2. Untracked: `ls-files --others --exclude-standard -z`, hunk giả `+1,N`.
3. Tràn `maxBuffer` (tham khảo `git-buffer-overflow.ts`) -> chế độ không hunk + `hunks_unavailable_diff_too_large`; stderr `rename detection was skipped` -> cảnh báo (chuỗi chưa kiểm chứng).

## Kiểm thử
Repo tạm: sửa, thêm, xoá, đổi tên, untracked, nhị phân, tên xuống dòng, diff giả lớn (`maxBuffer` nhỏ trong test), `head` đặt vs không.
Lệnh: `pnpm exec vitest run src/relay/codeintel-diff-collection.test.ts`

## Tiêu chí hoàn thành
- [ ] Tệp untracked `status:'A'`; `head` đặt thì không tính thay đổi chưa commit.

## Rủi ro
- `ls-files --others` chậm trên repo lớn (chưa đo).
