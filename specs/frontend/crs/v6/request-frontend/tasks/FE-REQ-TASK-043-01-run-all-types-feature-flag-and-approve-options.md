# FE-REQ-TASK-043-01: Hằng, cờ tính năng `approval.runAll`, `approve` kèm `executionMode`, thông điệp lỗi

**From Solution:** [FE-REQ-SOL-043](../solutions/FE-REQ-SOL-043-approve-plan-and-run-all-steps-ui.md)
**Priority:** P1
**Area:** frontend / request
**File:** `frontend/src/shared/{request-run-all,request-rpc-methods}.ts`, `frontend/src/renderer/src/store/slices/request.ts`, `frontend/src/renderer/src/hooks/usePlanDecision.ts`, `frontend/src/renderer/src/components/request/plan/plan-decision-error-message.ts` và test
**Depends on:** backend TASK-REQ-043-02, 043-04
**Status:** [ ] TODO

---

## Context

- `usePlanDecision.approve(approval, comment, risk)` (`hooks/usePlanDecision.ts:85-111`) dựng args của `approval.approve`; khoá bận `approve:<id>` bằng `useRef` chặn nhấp đúp đồng bộ.
- Cờ tính năng theo mẫu `request.baseRef`: `shared/request-base-ref.ts:16`, `store/slices/request.ts:78` `supportsBaseRef`.
- `plan-decision-error-message.ts:23` đã dịch `REQUEST_EXECUTION_ACTOR_NOT_ALLOWED`; mã lạ rơi xuống `requestErrorMessageForError`.
- Gateway cũ bỏ qua tham số lạ; cần cờ và đối chiếu phản hồi.

## Việc cần làm

1. `shared/request-run-all.ts`: `APPROVAL_RUN_ALL_FEATURE = 'approval.runAll'`, `RUN_ALL_EXECUTION_MODE = 'continuous'`, `parseApproveResult(raw): {requestStatus?: string; executionMode?: 'continuous' | 'step_by_step'}` (thiếu hoặc sai kiểu → `undefined`, không bao giờ ném lỗi).
2. `store/slices/request.ts`: `supportsApproveRunAll(state)` đọc `requestFlowFeatures` (trống ở gateway cũ nên mặc định `false`).
3. `usePlanDecision.approve(approval, comment?, risk?, options?: { executionMode?: 'continuous' })`: thêm `executionMode` vào args chỉ khi có; `PlanDecisionOutcome` thêm `runAllApplied?: boolean` (`true` khi đã yêu cầu và phản hồi `executionMode === 'continuous'`; `false` khi đã yêu cầu mà phản hồi khác; vắng khi không yêu cầu). Giữ nguyên khoá bận và hành vi `onSettled`.
4. `plan-decision-error-message.ts`: `REQUEST_EXECUTION_MODE_ALREADY_PINNED` → "This request already runs step by step because it was started that way before. Approve it normally."; `REQUEST_EXECUTION_MODE_NOT_APPLICABLE` và `REQUEST_EXECUTION_MODE_INVALID` → "Couldn't start the run this way. Approve it normally." kèm `console.warn` mã. Mã `ALREADY_PINNED` còn phải để hook/UI đọc được (hàm `isRunAllAlreadyPinned(failure)` xuất ra từ file này hoặc `request-run-all.ts`) để task 03 ẩn nút.
5. `shared/request-rpc-methods.ts`: thêm `ENGINE_SETTINGS: 'request.engineSettings'` và `ENGINE_SETTINGS_SET: 'request.engineSettingsSet'` (dùng ở task 05); `runtime/request-rate-limit.ts`: `ENGINE_SETTINGS_SET` vào `WRITE_METHODS`.

## Kiểm thử

- Parser: thiếu/sai kiểu/giá trị lạ.
- `usePlanDecision`: không `options` thì args y như cũ (so snapshot args); có `options` thì có `executionMode`; `runAllApplied` đúng ba trường hợp; nhấp đúp vẫn một lời gọi; thông điệp ba mã mới; mã `ACTOR_NOT_ALLOWED` giữ nguyên.
- Store: `supportsApproveRunAll` với tập trống và tập có cờ.
- `pnpm` vitest của `hooks`, `shared`, `components/request/plan`.

## Tiêu chí hoàn thành

- [ ] Duyệt thường không đổi byte args; duyệt kèm `options` gửi `executionMode`.
- [ ] Gateway cũ (không cờ) cho `supportsApproveRunAll === false`.
- [ ] Ba mã lỗi mới có thông điệp.

## Rủi ro và lưu ý

- Chạy `gitnexus_impact` cho `usePlanDecision` (dùng ở `PlanApprovalBar`, `PhaseApprovalBar`, `ExecutionNeedsYou`, `RequestPlanTab`) và báo blast radius.
- Không đổi `useApprovals.ts` và `useApprovalInbox.ts` (Inbox không có lựa chọn, D9 của CR).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
