# AG-CV-TASK-083-03: Thành phần diff dùng chung: dòng thêm/sửa (số và văn bản) so với merge-base

**From Solution:** [AG-CV-SOL-083-coverage-collection](../solutions/AG-CV-SOL-083-coverage-collection.md) mục 5.1
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-diff-changed-lines.ts` (mới), `.test.ts` (mới)
**Depends on:** AG-CV-TASK-081-14
**Status:** [ ] TODO

## Context

Dùng chung 083/084/091 (README quality-signals §3). Lệnh git dưới đều < 2.25; giữ `-c core.quotePath=false` ở đầu (AGENTS.md). Tên tệp lấy từ `--raw -z` (đáng tin), nội dung hunk từ `--unified=0` khớp theo thứ tự tệp.

## Việc cần làm

1. `getAddedLines(root, spec: { mergeBaseOf: string | null; to: "worktree" | "HEAD" }) → { files: { path; status: "A"|"M"|"R"|"C"|"T"; oldPath?; untracked: boolean; binary: boolean; addedLines: { line: number; text: string }[] }[]; warnings: string[]; truncated: boolean }`.
2. `mergeBase`: `git merge-base HEAD <base>`; `spec.to="worktree"` dùng `git diff --raw -z --no-abbrev -M <mb>` và `git diff --unified=0 -M --no-color --no-ext-diff <mb>`; `HEAD` dùng `<mb> HEAD`; untracked: `git ls-files --others --exclude-standard -z` (≤ 200 tệp, ≤ 2 MiB/tệp, bỏ nhị phân NUL).
3. Parse tiêu đề hunk `@@ -a[,b] +c[,d] @@` (thiếu `,d` = 1; `d=0` chỉ xoá → bỏ); dòng `+text` theo số dòng mới tăng dần; `\ No newline at end of file`, CRLF.
4. Giới hạn: diff ≤ 20 MiB, ≤ 5 000 tệp, mỗi dòng cắt 1 000 ký tự; vượt → `truncated:true`. `warnings`: `hunk_file_order_mismatch` (số tệp raw ≠ số section patch) thì bỏ mapping hunk cho tệp đó.
5. `base` đã qua `isSafeBaseRef` (task 081-14); scope `worktree` không có base → hàm không được gọi (người gọi `skipped`, xem task 084/091).

## Kiểm thử

Repo git thật: thêm/sửa/xoá/đổi tên/nhị phân/CRLF/tên Unicode/tên có khoảng trắng/untracked/tệp lớn; hunk `@@ -1 +1 @@` không số lượng; diff rỗng; unborn HEAD. Quét argv: không `--merge-base`, `--path-format`, `merge-tree`. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-diff-changed-lines.test.ts`.

## Tiêu chí hoàn thành

- [ ] Số dòng mới đúng cho mọi ca; không throw khi git lỗi (trả `warnings`).

## Rủi ro

Đây là bản trùng một phần với `detectChanges` của AG-CV-SOL-005 (CR-005); không hợp nhất ở task này (khác nhu cầu: có văn bản dòng). Ghi để cân nhắc hợp nhất sau.
