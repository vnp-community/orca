# FE-REQ-TASK-022-01: Quy tắc thuần của hộp duyệt (sắp xếp, duyệt nhanh, nhóm lọc, đích mở)

**From Solution:** [FE-REQ-SOL-022](../solutions/FE-REQ-SOL-022-approval-inbox.md) mục 2.2
**Priority:** P1
**Area:** frontend (renderer, logic thuần)
**File:** `frontend/src/renderer/src/components/request/approval/approval-inbox-rules.ts` (mới), `approval-inbox-rules.test.ts` (mới)
**Depends on:** FE-REQ-SOL-018 (kiểu `Approval`, `ApprovalSubjectType` trong `frontend/src/shared/request-types.ts`)
**Status:** [ ] TODO

## Context

- `components/request/` chưa tồn tại; `request-types.ts` do SOL-018 tạo. Nếu chưa merge, task này dùng kiểu cục bộ khai báo theo CR-REQ-009 mục 2.6 rồi đổi import khi SOL-018 vào.
- 8 giá trị `ApprovalSubjectType` theo README v6 mục 3.5: `request_type, solution, findings, answer, plan, phase, task_list, pre_deploy`, thêm `unknown` do parser chịu enum lạ (SOL-018).
- Không phụ thuộc React, i18n, store: để test nhanh và dùng lại ở CR-REQ-023 nếu cần.

## Việc cần làm

1. Tạo `approval-inbox-rules.ts` xuất:
   - `type ApprovalSubjectGroup = 'all'|'requestType'|'solution'|'plan'|'phase'|'preDeploy'|'other'`.
   - `SUBJECT_GROUP: Record<ApprovalSubjectType, Exclude<ApprovalSubjectGroup,'all'>>`: `request_type→requestType`, `solution→solution`, `plan→plan`, `task_list→plan`, `phase→phase`, `pre_deploy→preDeploy`, `findings→other`, `answer→other`, `unknown→other`.
   - `serverSubjectTypeFor(group): ApprovalSubjectType | undefined`: chỉ trả giá trị khi nhóm ánh xạ đúng một `subject_type` (`requestType`, `solution`, `phase`, `preDeploy`); `plan`, `other`, `all` trả `undefined` (lọc ở client).
   - `matchesGroup(a, group)`.
2. `canQuickApprove(a)`: `true` cho `request_type, findings, answer, task_list, phase, pre_deploy`; `false` cho `solution, plan, unknown`; thêm điều kiện `a.status === 'pending'` và `a.subjectDigest` không rỗng.
3. `isOverdue(a, now)`: `a.dueAt` hợp lệ và `Date.parse(a.dueAt) < now`.
4. `compareApprovals(now)`: quá hạn trước; trong cùng nhóm `dueAt` tăng dần (không có `dueAt` đứng cuối); rồi `createdAt` giảm dần; cuối cùng `id` tăng để ổn định.
5. `groupByRequest(rows)`: giữ thứ tự đã sắp xếp; nhóm đặt ở vị trí hàng đầu tiên của nó.
6. `openTargetFor(a)`: `{section:'requests', requestId: a.requestId, focus}`; `request_type→'type_confirmation'`; `solution|findings|answer→'analysis'`; `plan|task_list|phase|pre_deploy→'plan'`; `unknown→undefined` (mở chi tiết mặc định).
7. `quickApproveConsequenceKey(a)`: trả khoá i18n `auto.components.request.approval.ApprovalRow.confirmApprove.<subject_type>` (dùng ở task 05).

## Kiểm thử

`approval-inbox-rules.test.ts` (vitest, môi trường node):
- bảng test 8 giá trị + `unknown` cho `canQuickApprove`; thiếu `subjectDigest` thì `false`; `status !== 'pending'` thì `false`.
- `compareApprovals`: ba hàng (quá hạn, hạn gần, không hạn) và cặp cùng `dueAt` khác `createdAt`; `now` truyền vào, không dùng `Date.now()` trong test.
- `groupByRequest` giữ thứ tự; `serverSubjectTypeFor` cho 7 nhóm.
- `openTargetFor` đủ 8 giá trị.

Chạy (chưa chạy): `pnpm --filter orca-frontend test frontend/src/renderer/src/components/request/approval/approval-inbox-rules.test.ts`. Nếu vitest không khớp đường dẫn tính từ gốc, dùng đường dẫn tính từ `frontend/`.

## Tiêu chí hoàn thành

- [ ] File logic không import `react`, `@/store`, `@/i18n`.
- [ ] Bảng 8 giá trị khớp CR-REQ-022 mục 2.3 (solution và plan không duyệt nhanh).
- [ ] Test xanh; không có `any`.
- [ ] Không file nào tên `helpers`, `utils`, `common`.

## Rủi ro và lưu ý

- Nhóm `findings`/`answer` vào `other` theo CR-022 mục 2.1 (bộ lọc liệt kê "Khác"); CR-022 Q4 hỏi liệu hai loại này có nên duyệt nhanh, hiện cho phép theo bảng CR.
- `focus` cần `requestPage.focus` ở slice (yêu cầu bổ sung cho SOL-018); nếu chưa có, task 06 bỏ qua `focus`.
