# BUG-QA-06 (phát hiện thêm khi viết test QA-02): duyệt danh sách việc của `task` hiện nhãn "Approve plan"

**Mức độ:** P3 (nhãn sai, không đổi hành vi) · **Khu vực:** frontend (tab Plan) · **Trạng thái:** đã sửa 2026-10-16 (nhận `rawSubjectType = task_list`; test spec nhận nhãn "Task list") · **Nguồn:** test `qa-bug-task-native-flow` ca `task`, 2026-10-16

## Triệu chứng

Request loại `task` (và `docs`): tab Plan đưa nút duyệt **"Approve plan"**, trong khi luồng của loại này là danh sách việc (`startGate = task_list`) và thang tiến độ ghi "Task list". Mã đã định nhãn "Approve task list" nhưng không bao giờ hiện.

## Nguyên nhân gốc

`parseApproval` gộp `task_list` vào `plan` (`APPROVAL_SUBJECT_ALIASES`), giữ giá trị gốc ở `rawSubjectType`. `PlanApprovalBar.tsx:44` và `PlanApprovalCard.tsx:39` so sánh `approval.subjectType === 'task_list'` (đã bị gộp) nên luôn sai.

## Sửa

Dùng `approval.rawSubjectType === 'task_list'` ở cả hai nơi (hoặc cờ dẫn xuất `isTaskListApproval(approval)` ở `plan-approval-model.ts`). Test: `PlanApprovalBar` với approval `rawSubjectType: 'task_list'` có nút "Approve task list". Test e2e: `qa-bug-task-native-flow` ca `task` (`expectApprovalLabel(/approve task list/i)`), đang đỏ.
