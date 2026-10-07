# AG-CV-TASK-005-01: Phân giải `base`, `head`, merge-base (không mạng)

**From Solution:** [AG-CV-SOL-005-detect-changes](../solutions/AG-CV-SOL-005-detect-changes.md) mục 2.2
**Priority:** P0
**Area:** `agent/`
**File:** `agent/src/relay/codeintel-merge-base-resolution.ts`, `codeintel-merge-base-resolution.test.ts` (mới)
**Depends on:** [AG-CV-TASK-001-06](./AG-CV-TASK-001-06-gitnexus-registry-and-repo-resolution.md) (`runGit`), [AG-CV-TASK-001-01](./AG-CV-TASK-001-01-codeintel-errors-and-strict-params.md)
**Status:** [x] DONE

## Context
CR-005 2.2; contract §4.8: base mặc định không gọi mạng; so với merge-base (O7). `branchCompare` (`git-handler-ops.ts:124`) bị bỏ qua có chủ ý (không sửa).

## Việc cần làm
1. `resolveCompareRange({base?, head?}, workspaceRoot)` -> `{baseRef, baseOid, headOid|null, mergeBase, headRef|null, unborn}`; ref qua `assertGitRef`, `rev-parse --verify --quiet --end-of-options <ref>^{commit}`.
2. Mặc định base theo chuỗi `symbolic-ref` -> `origin/main|origin/master|main|master` -> `base_required`.
3. HEAD unborn -> `unborn:true` (handler trả `changedFiles:[]` + `unborn_head`); `merge-base` lỗi -> `no_merge_base`.
4. Chỉ lệnh Git ≤ 2.24.

## Kiểm thử
Repo tạm: `origin/HEAD` có, chỉ `main`, chỉ `master`, không có; `no_merge_base` (hai nhánh mồ côi); unborn; `base:'-x'`, `'a b'`, không tồn tại; kiểm argv không có `ls-remote`/mạng.
Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/codeintel-merge-base-resolution.test.ts`

## Tiêu chí hoàn thành
- [x] Mọi lỗi đúng `data.reason`; không spawn công cụ thứ hai.

## Rủi ro
- `--end-of-options` từ Git 2.24; chạy test với Git 2.25 baseline.
