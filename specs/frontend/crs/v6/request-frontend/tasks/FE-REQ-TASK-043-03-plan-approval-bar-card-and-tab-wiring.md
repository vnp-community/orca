# FE-REQ-TASK-043-03: Nối nút phụ vào `PlanApprovalBar`, `PlanApprovalCard` gửi `continuous`, xử lý Request đã ghim

**From Solution:** [FE-REQ-SOL-043](../solutions/FE-REQ-SOL-043-approve-plan-and-run-all-steps-ui.md)
**Priority:** P1
**Area:** frontend / request
**File:** `frontend/src/renderer/src/components/request/plan/{PlanApprovalBar,PlanDecisionButtons,PlanApprovalCard,RequestPlanTab}.tsx` và test (`plan-approval-bars.test.tsx`, `PlanApprovalCard.test.tsx`, `RequestPlanTab.test.tsx`)
**Depends on:** FE-REQ-TASK-043-02
**Status:** [x] DONE (2026-10-20)

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

- [x] Hai nút trên thanh duyệt Plan của `change_request` và `bug`/`refactor` cỡ L; một nút ở thẻ liên tục; không nút ở luồng không có Phase. (`plan-approval-bars.run-all.test.tsx`, `RequestPlanTab.run-all.test.tsx`, `PlanApprovalCard.test.tsx`; luồng được mời là luồng backend có `hasPhases`, không xét theo tên loại; xem "chưa kiểm" về bố cục).
- [x] Nút phụ gửi đủ trường rủi ro như nút chính. ("sends the risk-gate fields together with the execution mode" trong `plan-approval-bars.run-all.test.tsx`).
- [x] Nhấp đúp, bấm hai nút liên tiếp: chỉ một lời gọi. (test "locks the choice while the main Approve is in flight" và "opening and cancelling... confirming sends executionMode once"; `usePlanDecision.run-all.test.ts` "blocks a second call...").
- [x] Phím tắt, thứ tự Tab: nút chính, nút phụ, Reject. (thứ tự DOM `plan-approve`, `plan-approve-run-all`, `plan-reject` được kiểm; nút phụ nằm giữa vì `PlanDecisionButtons` dựng nó ngay sau Approve).

## Rủi ro và lưu ý

- Chạy `gitnexus_impact` cho `PlanApprovalBar`, `PlanDecisionButtons`, `PlanApprovalCard`, `RequestPlanTab`.
- Bố cục thanh hẹp (ba nút + Regenerate): kiểm ở 3 kích thước, cho phép xuống dòng.

## Ghi chú triển khai (2026-10-20)

**File sửa** (`frontend/src/renderer/src/components/request/plan/`): `PlanDecisionButtons.tsx` (props `approveExecutionMode`, `runAll`; vùng nút `flex-wrap`; nút phụ giữa Approve và Reject), `PlanApprovalBar.tsx` (props `approveExecutionMode`, `runAll`; tự đọc `supportsApproveRunAll` từ store; tính `canOfferRunAll` với `risk.approverNotAllowed` và `decision.runAllPinned`), `PlanApprovalCard.tsx` (`approveExecutionMode="continuous"`, không nút phụ), `RequestPlanTab.tsx` (250 dòng, +1: truyền `runAll={{ request, flow: flowView, stepCount: tree.phases.length }}`), `hooks/usePlanDecision.ts` (cờ `runAllPinned` theo Request, giữ qua các lần làm mới). Test mới: `plan-approval-bars.run-all.test.tsx` (16), `RequestPlanTab.run-all.test.tsx` (6); thêm 1 ca vào `PlanApprovalCard.test.tsx`.

**Blast radius**: `PlanApprovalBar` dùng ở `RequestPlanTab` và `PlanApprovalCard`; `PlanDecisionButtons` dùng ở `PlanApprovalBar`, `PhaseApprovalBar`, `PlanGateChips` (các props mới đều tuỳ chọn nên hành vi cũ không đổi; cổng Phase/pre_deploy không có nút phụ); `PlanApprovalCard` dùng ở `RequestPlanTab`. Approval Inbox, `PhaseApprovalBar`, `ExecutionNeedsYou` không đổi.

**Khác với task text**
- Việc đọc cờ `supported` đặt trong `PlanApprovalBar` (không ở `RequestPlanTab`) để giữ `RequestPlanTab` dưới 300 dòng và để mọi nơi dùng thanh duyệt đúng quy tắc.
- Cờ "đã ghim" lưu trong `usePlanDecision` (sống cùng `RequestPlanTab`) chứ không `useState` trong thanh duyệt, vì thanh bị gỡ rồi dựng lại khi tab tải lại sau `onSettled`.
- Thanh duyệt không mời lựa chọn khi `stepCount === 0` (không có số thật để hỏi).

**Chưa kiểm**: bố cục thanh hẹp (ba nút + Regenerate) ở 3 kích thước chưa nhìn bằng mắt (chưa dựng giao diện); dùng `flex-wrap` nên cho phép xuống dòng. Chưa chạy trên stack.

**Kiểm chứng (từ `frontend/`)**: `npx vitest run --config config/vitest.config.ts src/renderer/src/components/request`: mọi test đã đổi xanh; test hiện có của ba file (`plan-approval-bars.test.tsx`, `PlanApprovalCard.test.tsx`, `RequestPlanTab.test.tsx`) xanh.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
