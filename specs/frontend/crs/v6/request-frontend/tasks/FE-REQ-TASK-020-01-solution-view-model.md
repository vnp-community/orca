# FE-REQ-TASK-020-01: Mô hình hiển thị Solution (hàm thuần)

**From Solution:** [FE-REQ-SOL-020](../solutions/FE-REQ-SOL-020-solution-review-ui.md) mục 2.2, 2.3
**Priority:** P0
**Area:** frontend / request / solution
**File:** `frontend/src/renderer/src/components/request/solution/solution-view-model.ts` (mới), test `solution-view-model.test.ts`
**Depends on:** FE-REQ-TASK-018-01
**Status:** [x] DONE

## Context

- Kiểu `Solution`, `SolutionOption`, `Approval` ở `shared/request-types.ts` (018-01). Cổng theo README v6 3.4: `solution` (cho `solution` và `diagnosis`), `findings` (spike), `answer` (question), `hotfix` không cổng.
- Thuần TypeScript: không React, dễ test bảng.

## Việc cần làm

1. `getSolutionPresentation(solution, {requestStatus, requestType, hasPendingApproval}) → {bannerKey, tone, actions: Array<'choose'|'approve'|'reject'|'regenerate'|'generatePlan'>, readOnly}` theo bảng SOL-020 2.2; `readOnly=true` khi `requestStatus` đã qua `awaiting_analysis_approval` hoặc `cancelled`, hoặc `hotfix`.
2. `solutionApprovalSubject(kind, requestType): ApprovalSubjectType|null` (`solution|diagnosis` → `solution`; `findings` → `findings`; `answer` → `answer`; `hotfix` → `null`).
3. `buildComparisonRows(options): Array<{criterion, cells: string[], differs: boolean}>` với tiêu chí `summary`, `pros`, `cons`, `effort`, `risk`; thiếu trường thành `''` (UI hiện "Không có dữ liệu"); `differs=true` khi ô không đồng nhất.
4. `canApproveSolution({kind, requestType, options, chosenOptionId}): {ok:boolean, reasonKey?:'needTwoOptions'|'chooseOne'}`: `change_request` + kind `solution` cần `options.length >= 2`; kind `solution` cần đã chọn.
5. `validateRejectReason(text): {ok:boolean, length:number}` (trim, tối thiểu `REJECT_REASON_MIN_LENGTH = 10`); `clampFeedback(text)` cắt 2000 ký tự (CR-016: `feedback <= 2000`).
6. `pickPendingApproval(approvals, solution)`: Approval `pending` có `subjectId===solution.id` (nếu `subjectId` rỗng thì Approval `pending` mới nhất cùng `subjectType`; ghi giả định).

## Kiểm thử

- Bảng test status × requestStatus × quyền cho `getSolutionPresentation` (5 status).
- `buildComparisonRows` với 2/3 phương án, thiếu `pros`, trùng giá trị (không `differs`).
- `canApproveSolution`: 1 phương án `change_request` → `needTwoOptions`; chưa chọn → `chooseOne`; `diagnosis` luôn ok.
- `validateRejectReason`: `'         '`, `'ngắn'`, 10 ký tự sau trim, ký tự Unicode (đếm theo `[...text.trim()].length`).
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/request/solution/solution-view-model`.

## Tiêu chí hoàn thành

- [ ] Không phụ thuộc React hay store.
- [ ] Mọi nhánh bảng SOL-020 2.2 có test.
- [ ] `tsc` sạch lỗi mới.

## Rủi ro và lưu ý

- Giả định `pickPendingApproval` có thể sai khi backend chốt quan hệ Approval ↔ Solution (câu hỏi mở 3 của solution).
- Hằng 10 ký tự phải dùng chung với `useApprovals.reject` (018-03): export từ `request-flow-registry.ts` hoặc `request-types.ts` thay vì lặp.
