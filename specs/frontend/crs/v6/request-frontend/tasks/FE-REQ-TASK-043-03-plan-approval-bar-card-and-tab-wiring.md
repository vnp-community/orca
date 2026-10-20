# FE-REQ-TASK-043-03: Nối nút phụ vào `PlanApprovalBar`, `PlanApprovalCard` gửi `continuous`, xử lý Request đã ghim

**From Solution:** [FE-REQ-SOL-043](../solutions/FE-REQ-SOL-043-approve-plan-and-run-all-steps-ui.md)
**Priority:** P1
**Area:** frontend / request
**File:** `frontend/src/renderer/src/components/request/plan/{PlanApprovalBar,PlanDecisionButtons,PlanApprovalCard,RequestPlanTab}.tsx` và test (`plan-approval-bars.test.tsx`, `PlanApprovalCard.test.tsx`, `RequestPlanTab.test.tsx`)
**Depends on:** FE-REQ-TASK-043-02
**Status:** [ ] TODO

---

## Context

- `RequestPlanTab` chọn `PlanApprovalCard` khi `continuous && planApproval?.status === 'pending'` (`:181-189`), còn lại `PlanApprovalBar` (`:191-195`). Trường `executionMode` của Request chưa ghim là chế độ sẽ dùng (không phân biệt với ghim).
- `PlanApprovalBar` tính `risk` bằng `useApprovalRiskGate` và có `hideApprove`, `approveBlockedReason`, `approveExtras` (trường rủi ro gửi cùng `approval.approve`); nút phụ phải dùng đúng các giá trị đó, không gửi thiếu.
- `PlanDecisionButtons.tsx:58` gọi `decision.approve(approval, undefined, approveExtras)`.

## Việc cần làm

1. `PlanDecisionButtons`: prop `approveExecutionMode?: 'continuous'` truyền làm `options.executionMode` cho nút chính.
2. `PlanApprovalCard`: truyền `approveExecutionMode="continuous"` cho `PlanApprovalBar` (nút chính "Approve and start execution" gửi chế độ tường minh); **không** thêm nút phụ.
3. `PlanApprovalBar`: prop `runAll?: { stepCount: number; flow: RequestFlowView | null; request: OrcaRequest }`; khi `canOfferRunAll(...)` đúng, dựng `PlanRunAllButton` cạnh nút chính, `onConfirm` gọi `decision.approve(approval, undefined, risk.gate.approveExtras, { executionMode: 'continuous' })`. Bỏ qua nút khi `hideApprove`.
4. `RequestPlanTab`: truyền `runAll` với `stepCount = tree.phases.length`, `flow = flowView`, `request`; lấy `supported = supportsApproveRunAll(state)` từ store (`useAppStore`).
5. Request đã ghim `step_by_step`: khi `decision.failure` có `REQUEST_EXECUTION_MODE_ALREADY_PINNED` (task 01) thì giữ cờ cục bộ trong `PlanApprovalBar` làm `alreadyPinned` để ẩn nút phụ ở các lần render sau và vẫn hiện thông điệp; làm mới Request (`onSettled`) không xoá cờ trong cùng phiên.
6. Giữ nguyên Approval Inbox, `PhaseApprovalBar`, `ExecutionNeedsYou`.

## Kiểm thử

- `plan-approval-bars.test.tsx`: nút phụ hiện ở `change_request` có Phase khi gateway có cờ; ẩn khi không cờ, khi `hasPhases=false`, khi đã `continuous`, khi `fallback`, khi `hideApprove`; bấm → hộp thoại → xác nhận gọi `approve` với `executionMode: 'continuous'` và `approveExtras`; nút chính vẫn gọi `approve` không `options`.
- `PlanApprovalCard.test.tsx`: nút chính gửi `executionMode: 'continuous'`; không có `plan-approve-run-all`.
- `RequestPlanTab.test.tsx`: dự án `step_by_step` có hai nút; sau duyệt-chạy-hết và tải lại thấy `PlanOutline` và dải "Plan approved … The agent runs it on its own."; `ALREADY_PINNED` hiện thông điệp rồi ẩn nút.
- Các test hiện có của ba file vẫn xanh.

## Tiêu chí hoàn thành

- [ ] Hai nút trên thanh duyệt Plan của `change_request` và `bug`/`refactor` cỡ L; một nút ở thẻ liên tục; không nút ở luồng không có Phase.
- [ ] Nút phụ gửi đủ trường rủi ro như nút chính.
- [ ] Nhấp đúp, bấm hai nút liên tiếp: chỉ một lời gọi.
- [ ] Phím tắt, thứ tự Tab: nút chính, nút phụ, Reject.

## Rủi ro và lưu ý

- Chạy `gitnexus_impact` cho `PlanApprovalBar`, `PlanDecisionButtons`, `PlanApprovalCard`, `RequestPlanTab`.
- Bố cục thanh hẹp (ba nút + Regenerate): kiểm ở 3 kích thước, cho phép xuống dòng.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
