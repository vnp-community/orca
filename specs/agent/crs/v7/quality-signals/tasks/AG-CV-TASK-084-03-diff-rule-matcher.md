# AG-CV-TASK-084-03: Bộ so khớp luật diff (added-line-regex, added-file-name, file-content-regex)

**From Solution:** [AG-CV-SOL-084-convention-rule-pack](../solutions/AG-CV-SOL-084-convention-rule-pack.md) mục 5.1
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-rule-diff-matcher.ts` (mới), `.test.ts` (mới)
**Depends on:** AG-CV-TASK-084-02, AG-CV-TASK-083-03
**Status:** [ ] TODO

## Context

CR-084 2.2/2.3: chỉ dòng thêm; tệp binary/ > 1 MiB/sinh bị bỏ; dòng dài hơn `maxLineLength` bị bỏ và đếm `skippedLongLines`; trần 5 000 tệp, 20 MiB diff.

## Việc cần làm

1. `matchRule(rule, addedFiles, deps{ now, budgetMsPerLine: 5 }) → { findings: RawQualityFinding[]; skippedLongLines; truncated }`.
2. `added-line-regex`: so từng dòng thêm; `added-file-name`: tên tệp có `status A` hoặc đổi tên đến (khớp `scope.include/exclude` bằng glob tự viết đơn giản `**`, `*`, `{a,b}`; không thêm thư viện); `file-content-regex`: nội dung tệp mới ≤ 1 MiB.
3. Ngân sách thời gian mỗi dòng (đo `performance.now()` quanh `exec`; vượt → bỏ luật cho phần còn lại của tệp, ghi cảnh báo) — không dùng `worker` ở bước này.
4. Phát hiện: `line` = số dòng mới, `anchorOverride` không đặt (pipeline dùng văn bản dòng thêm đã chuẩn hoá), `message` cố định theo luật, `matchedText` không đưa ra.
5. Bỏ qua `exclude` (vd `*.test.*`, `node_modules`).

## Kiểm thử

Bảng ca regex/glob; chèn dòng cũ vi phạm vs dòng mới; dòng 5 000 ký tự; `(a+)+b` trên chuỗi dài không treo (test timeout < 1 s); CRLF; tên tệp Unicode. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-rule-diff-matcher.test.ts`.

## Tiêu chí hoàn thành

- [ ] Không ca nào treo; chỉ dòng thêm bị báo.

## Rủi ro

Giới hạn thời gian trong luồng đơn không ngắt được một `exec` đang chạy: giảm thiểu bằng tập con regex an toàn + `maxLineLength`.
