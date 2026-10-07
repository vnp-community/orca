# AG-CV-TASK-084-07: Đăng ký profile `repo-rules`, `repo-rules-scripts` và chọn luật theo tệp đổi

**From Solution:** [AG-CV-SOL-084-convention-rule-pack](../solutions/AG-CV-SOL-084-convention-rule-pack.md) mục 5.3
**Priority:** P1
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-rule-profiles.ts` (mới), `.test.ts`; điền catalog qua `registerBuiltinProfiles`; nối planner (081-15)
**Depends on:** AG-CV-TASK-084-04, 084-05, 084-06, AG-CV-TASK-081-12, 081-15
**Status:** [x] DONE

## Context

Hợp đồng §5.1 kind `repo-rules`; luật diff cần `base` (lệch #7).

## Việc cần làm

1. Hai profile `kind:"repo-rules"`: `repo-rules` (diff, không cần `node_modules`, `heavy:false`) và `repo-rules-scripts` (cần `node`, `node_modules` nếu script cần; `requires` kiểm).
2. Step đặc biệt `kind: "rules"` trong planner (không spawn công cụ ngoài): `repo-rules` chạy `runDiffRules` trong tiến trình agent (ghi `PlannedStep` với `inProcess:true`); scope `worktree` → `skipped` (`skipReason:"base_required"`).
3. `repo-rules-scripts`: chọn script theo tệp đổi (ORCA-004 khi `**/i18n/**`, ORCA-005 khi `*.tsx` renderer, ORCA-006 khi `resources/onboarding/feature-wall/**`).
4. Thêm cả hai vào suite `standard`/`full` (không `fast`).

## Kiểm thử

Planner với tệp đổi giả → chọn đúng script; `worktree` scope → skipped; profile qua kiểm token cấm; `repo-rules` không spawn tiến trình (spy). Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-rule-profiles.test.ts src/relay/quality-run-planning.test.ts`.

## Tiêu chí hoàn thành

- [x] Không có đường client truyền pattern/đường dẫn.
- [x] Bước diff không spawn.

## Rủi ro

`inProcess` là khái niệm mới của `PlannedStep` (sửa kiểu ở 081-04); chỉ thêm trường tuỳ chọn.
