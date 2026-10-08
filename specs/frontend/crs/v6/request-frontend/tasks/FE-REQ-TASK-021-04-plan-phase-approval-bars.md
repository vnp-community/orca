# FE-REQ-TASK-021-04: Duyệt Plan, Phase, `pre_deploy` và bắt đầu Phase

**From Solution:** [FE-REQ-SOL-021](../solutions/FE-REQ-SOL-021-plan-phase-tree-and-approval-ui.md) mục 2.4
**Priority:** P0
**Area:** frontend / request / plan
**File:** `frontend/src/renderer/src/components/request/plan/{PlanApprovalBar,PhaseApprovalBar,PlanGateChips}.tsx` (mới); `hooks/usePlanDecision.ts` (mới); test cùng tên
**Depends on:** FE-REQ-TASK-021-02, 021-03, 020-04 (`RejectReasonDialog`), 018-03
**Status:** [x] DONE (verified 2026-10-07: vitest plan-approval-bars.test.tsx (15 tests) pass; oxlint+tsc clean)

**Ghi chú:** dùng `RejectReasonDialog` của 020 (Mod+Enter, test cả Mac/Linux). `generatePlan` chạy `mode: 'propose'` rồi `mode: 'commit'` kèm `proposal`/`rawAiResponse` (`hooks/request-plan-generation.ts`, test trong `useRequestActions.test.ts`); chưa có UI sửa đề xuất trước khi commit. `startPhase` gửi `{id, phaseTaskId}` ở cả `usePlanDecision` và `useRequestActions`. 2026-10-08: approve/reject gửi thêm `id` theo CONTRACT 2.3 (trước chỉ có `approvalId`).

## Context

- Kênh: `approval.approve {id, expectedVersion, expectedDigest, comment?}`, `approval.reject {...comment bắt buộc}`, `request.generatePlan {id}` (25 s), `request.startPhase {id, phaseTaskId}` (exec) (CR-REQ-016 2.3, 2.4).
- Mã lỗi: `APPROVAL_*` như SOL-020, `REQUEST_TRANSITION_NOT_ALLOWED`, `REQUEST_STATE_STALE`, `REQUEST_RATE_LIMITED`.
- Cổng: `plan`/`task_list` (Plan), `phase` (mỗi Phase), `pre_deploy` (hotfix, security, ops_request; README mục 8 số 6: cổng trước task có nhãn `gate:pre_deploy` hoặc task fix).
- Không có `viewerCan`; xử lý `APPROVAL_NOT_APPROVER`.

## Việc cần làm

1. `usePlanDecision(request)`: `approve(approval, comment?)`, `reject(approval, comment, {regenerate})`, `startPhase(phaseTaskId)`, `regeneratePlan()`; cùng cách xử lý `conflict`/`invalid_state`/`expired`/`forbidden` như `useSolutionDecision` (020-04); khoá chống bấm đôi theo `approval.id`/`phaseTaskId`.
2. `PlanApprovalBar({plan, approval, request})`: nút "Duyệt Plan" (nhãn "Duyệt danh sách task" khi `task_list`), "Từ chối" (mở `RejectReasonDialog`, lý do >= 10 ký tự), "Sinh lại Plan"; sau khi Plan bị từ chối hiện lý do và "Sinh lại Plan". Plan đã duyệt: chỉ đọc, không sửa tại chỗ.
3. `PhaseApprovalBar({phase, approval})`: "Duyệt Phase", "Từ chối", và "Bắt đầu Phase" khi Approval `approved` và Phase chưa chạy (`status` chưa `in_progress`/`done`); tooltip "Phase chưa được duyệt" khi khoá. Từ chối Phase: hiện thông báo Request có thể về `planning` (hành vi do backend, CR-REQ-003).
4. `PlanGateChips({approvals})`: chip cho `plan`, `phase`, `pre_deploy` (icon + trạng thái); khối hành động riêng cho `pre_deploy` `pending` (Duyệt/Từ chối).
5. Phím tắt: không có cho Duyệt; `Mod+Enter` gửi hộp từ chối (`isScreenSubmitShortcut`; nhãn `ShortcutKeyCombo`).
6. Lỗi: `REQUEST_TRANSITION_NOT_ALLOWED` ở `startPhase` hiện thông điệp của backend (tuần tự Phase chưa chốt); `rate_limited` ở `generatePlan`.

## Kiểm thử

- `PlanApprovalBar`: duyệt gọi `approval.approve` với đúng `id`, `expectedVersion`, `expectedDigest`; từ chối rỗng/ngắn không gửi; "Sinh lại Plan" gọi `request.generatePlan {id}`; `forbidden` đổi thành "Bạn không có quyền duyệt"; Plan `approved` chỉ đọc.
- `PhaseApprovalBar`: "Bắt đầu Phase" chỉ khi `approved`; gọi `request.startPhase {id, phaseTaskId}`; lỗi `REQUEST_TRANSITION_NOT_ALLOWED`.
- `PlanGateChips`: `pre_deploy` pending có khối hành động.
- Phím `Mod+Enter`: `metaKey` Mac, `ctrlKey` Linux/Windows (mock `getShortcutPlatform`).
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/request/plan/PlanApprovalBar src/renderer/src/components/request/plan/PhaseApprovalBar src/renderer/src/components/request/plan/PlanGateChips`.

## Tiêu chí hoàn thành

- [ ] Duyệt Plan, Phase, `pre_deploy` gọi `approval.approve` đúng `subjectId` của Approval.
- [ ] Từ chối lý do rỗng: nút khoá, không gửi.
- [ ] "Bắt đầu Phase" chỉ sau khi Phase được duyệt.
- [ ] i18n `PlanApprovalBar.*`, `PhaseApprovalBar.*`, `PlanGateChips.*` đủ 5 locale.

## Rủi ro và lưu ý

- Phase tuần tự hay song song chưa rõ: UI không tự chặn, để backend trả lỗi.
- `generatePlan` không có tham số phản hồi trong CR-016: lý do từ chối chỉ lưu ở Approval.
