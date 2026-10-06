# AG-CV-TASK-084-05: Định vị script, chạy script rules (ORCA-004..006) với `cwd` đúng

**From Solution:** [AG-CV-SOL-084-convention-rule-pack](../solutions/AG-CV-SOL-084-convention-rule-pack.md) mục 5.3
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-rule-script-locator.ts`, `quality-rule-script-runner.ts` (mới) + test
**Depends on:** AG-CV-TASK-084-01, 084-02, AG-CV-TASK-081-04, 082-04
**Status:** [ ] TODO

## Context

Script chạy mã của repo: dùng executor có giới hạn (081-A). Không chạy ORCA-001..003 (bí danh). Không đối số động.

## Việc cần làm

1. `locateScript(root, rule.script)`: thử `<root>/config/scripts/<name>.mjs`, `<root>/desktop/config/scripts/<name>.mjs`; `realpath` trong worktree; không thấy → `script_not_found`.
2. `cwd`: `repoRoot` hoặc `desktop` theo `script.cwd`; argv `[process.execPath, scriptPath, ...rule.script.args]` (`args` hằng, ví dụ `--check` cho ORCA-005); env từ `buildQualityChildEnv`.
3. Chạy qua `executeStep` (timeout/đầu ra giới hạn, heavy: false); parser: `exit-code` mặc định → một finding mức repo (`file:""`, `anchorOverride:"<scriptName>"`) kèm 2 KiB cuối stdout/stderr **đã che**; thiếu module → `env_not_ready` (không finding).
4. Chạy chỉ khi `scope` của luật khớp tệp đổi (task 06), nếu không `skipped_scope`.

## Kiểm thử

Cây giả có hai vị trí; script giả mã thoát 0/1/timeout/`Cannot find module`; kiểm argv không chứa `--init|--prune`; secret giả trong stdout bị che. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-rule-script-runner.test.ts`.

## Tiêu chí hoàn thành

- [ ] `script_not_found` không bao giờ thành pass; cwd đúng.

## Rủi ro

Hai bản baseline (gốc/desktop): chỉ liên quan ORCA-001 (không chạy ở đây).
