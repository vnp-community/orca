# AG-CV-TASK-084-04: Cài các luật diff ORCA-007, 010-015 với fixture vi phạm/không vi phạm

**From Solution:** [AG-CV-SOL-084-convention-rule-pack](../solutions/AG-CV-SOL-084-convention-rule-pack.md) mục 5.3
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-rule-diff-runner.ts` (mới), `.test.ts`, bổ sung dữ liệu `quality-rule-pack-orca.ts`
**Depends on:** AG-CV-TASK-084-03
**Status:** [x] DONE

## Context

Bảng luật mục 5.3; mức mặc định đề xuất (chưa đo báo nhầm).

## Việc cần làm

1. `runDiffRules(pack, addedFiles, enabledIds) → ruleResults + findings`.
2. ORCA-011: parse JSON của `mobile/.oxlintrc.json` ở base (từ `git show <mb>:path`) và HEAD/worktree, so giá trị `max-lines` từng override; tăng → finding (đọc nội dung base bằng `execFile git show`, giới hạn 1 MiB, không dùng `jsonc` mới nếu tệp là JSON thường; nếu dùng `jsonc-parser` thì đã có sẵn).
3. ORCA-012: tên chứa đoạn `helpers|utils|common|misc|shared-stuff` hoặc đuôi `-helpers.ts|-utils.ts`, chỉ tệp **mới** (A/đổi tên đến).
4. ORCA-014: bỏ qua khi cùng tệp có `navigator.userAgent.includes(Mac)`, `isMac`, hoặc `CmdOrCtrl`.
5. ORCA-015: bảng "cần Git mới hơn" là hằng trong pack (từ `guides/reference/git-compatibility.md`: `worktree list -z`, `rev-parse --path-format`, `for-each-ref --exclude`, `merge-tree --write-tree`), chỉ khi tệp có `execFile(git`/`git(` và không tham chiếu `GitCapabilityCache`.
6. ORCA-013: loại `main.css`, `*.test.*`, `href="#..."`, `url(#...)`.

## Kiểm thử

Mỗi luật 2 fixture (vi phạm / không) + ca biên đã nêu; diff nhiều tệp; `ruleResults` có `ran`. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-rule-diff-runner.test.ts`.

## Tiêu chí hoàn thành

- [x] Mỗi luật có test hai chiều; ORCA-008/009/016/017 không xuất hiện.

## Rủi ro

ORCA-013/014/015 báo nhầm cao: giữ mức `warning`/`info`.
