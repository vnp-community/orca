# AG-CV-TASK-084-06: `ruleResults[]` trong `QualityStepResult` và quy tắc cổng `unknown`

**From Solution:** [AG-CV-SOL-084-convention-rule-pack](../solutions/AG-CV-SOL-084-convention-rule-pack.md) mục 5.1
**Priority:** P0
**Area:** `agent/` (Dev Server Agent)
**File:** `agent/src/relay/quality-rule-results.ts` (mới), `.test.ts`; sửa `quality-results-store.ts` (081-07)
**Depends on:** AG-CV-TASK-084-04, 084-05, AG-CV-TASK-081-07
**Status:** [ ] TODO

## Context

PQ-26 và hợp đồng thiếu chỗ đặt `ruleResults` (lệch #6): solution đề xuất thêm vào `QualityStepResult` (cần sửa hợp đồng trước khi backend dùng).

## Việc cần làm

1. `type RuleResult = { ruleId: string; status: "ran"|"skipped_scope"|"script_not_found"|"env_not_ready"|"disabled"; durationMs: number }`.
2. `buildRuleResults(pack, outcomes)`: luật `enabled:false` → `disabled`; luật script ngoài phạm vi → `skipped_scope`; `profile-ref` không nằm ở đây (không có mục).
3. `QualityStepResult.ruleResults` (tuỳ chọn) cho bước `repo-rules*`; `quality.results view=steps` trả; không tạo finding `-unavailable`.
4. `status` của bước: có luật `script_not_found|env_not_ready` → bước `env_not_ready` một phần: đặt `envMissing` tương ứng để gate coi `unknown` (backend); có `ran` + phát hiện → `findings`.

## Kiểm thử

Bảng: tổ hợp trạng thái luật → trạng thái bước; JSON `view=steps` có `ruleResults`; test khẳng định không có finding với `ruleId` kết thúc `-unavailable`. Lệnh: `cd /opt/repos/orca/agent && pnpm exec vitest run src/relay/quality-rule-results.test.ts src/relay/quality-results-store.test.ts`.

## Tiêu chí hoàn thành

- [ ] Không finding giả; `unknown` được suy ra được ở backend từ dữ liệu này.

## Rủi ro

Trường mới ngoài hợp đồng: backend phải bỏ qua trường lạ cho tới khi hợp đồng sửa.
