# FE-REQ-TASK-043-02: Quy tắc mời lựa chọn và hộp thoại xác nhận "Run all steps"

**From Solution:** [FE-REQ-SOL-043](../solutions/FE-REQ-SOL-043-approve-plan-and-run-all-steps-ui.md)
**Priority:** P1
**Area:** frontend / request
**File:** `frontend/src/renderer/src/components/request/plan/{plan-run-all-model.ts,RunAllStepsConfirmDialog.tsx,PlanRunAllButton.tsx}` và test
**Depends on:** FE-REQ-TASK-043-01
**Status:** [ ] TODO

---

## Context

- `RequestFlowView` có `hasPhases` cho cỡ được hỏi và `source: 'backend' | 'fallback'` (`shared/request-execution-types.ts:16-31`); bảng dự phòng đã lệch với backend (CR-REQ-038 mục 1).
- `PlanApprovalCard` có sẵn cách nhận biết cổng `pre_deploy` (`flow.startGate === 'pre_deploy' || flow.executionGates.includes('pre_deploy')`, `PlanApprovalCard.tsx:37-38`) và đếm bước (`stepCount`).
- Không có `alert-dialog`; dùng `components/ui/dialog.tsx`. Hộp thoại phải đóng bằng Esc, giữ focus, có `aria-labelledby`/`aria-describedby`.
- Hệ quả bảo mật cần nói trong hộp thoại: người duyệt là người thực thi cho cả run.

## Việc cần làm

1. `plan-run-all-model.ts`: `canOfferRunAll({request, approval, flow, supported, approverHidden, alreadyPinned}): boolean` thuần, đúng các điều kiện: Approval `pending` và `plan` hoặc `task_list` (xét `rawSubjectType`); `flow?.source === 'backend' && flow.hasPhases`; `request.executionMode !== 'continuous'`; `supported`; `!approverHidden`; `!alreadyPinned`. Và `runAllDialogModel({ stepCount, flow })` trả `{ stepCount, mentionsPreDeploy }`.
2. `RunAllStepsConfirmDialog.tsx`: tiêu đề "Run all {{n}} steps without asking again?", thân như FE-REQ-SOL-043 mục 2.3 (có vế "when the agent needs information from you" và vế pre-deploy có điều kiện); nút primary "Run all steps" (bận thì `Loader2`, khoá cả hai nút), nút "Not now"; Esc và nền đóng; không gửi gì khi đóng.
3. `PlanRunAllButton.tsx`: nút `variant="outline"` "Approve and run all steps" với `data-testid="plan-approve-run-all"`; giữ trạng thái mở hộp thoại; khoá giống nút chính (`locked`, `approveBlockedReason` làm `title`, `coolingDown`); gọi `onConfirm()` do cha truyền (task 03 nối tới `decision.approve`); sau `runAllApplied === false` hiện toast cảnh báo "This server didn't run the plan on its own. Approve each step as usual.".
4. Chuỗi UI dùng `translate('auto.components.request.plan.<Component>.<key>', 'English default')`; khoá thêm đủ 5 locale ở task 06.
5. Dùng token `assets/main.css` và primitive shadcn; không màu mới; kích thước nút `sm` như nút chính.

## Kiểm thử

- `canOfferRunAll`: bảng đủ từng điều kiện (mỗi điều kiện sai một lần), kể cả `task_list`, `fallback`, `hasPhases=false`, đã `continuous`.
- Dialog: hiển thị số bước và vế pre-deploy đúng điều kiện; Esc/Not now không gọi `onConfirm`; xác nhận gọi một lần; nhấp đúp xác nhận chỉ một lần (khoá bận); focus quay lại nút mở.
- `PlanRunAllButton`: khoá theo `locked` và `approveBlockedReason`; toast cảnh báo khi `runAllApplied === false`.

## Tiêu chí hoàn thành

- [ ] Quy tắc mời lựa chọn đúng tập luồng có Phase của backend, không dựa vào bảng dự phòng.
- [ ] Hộp thoại đúng nội dung, truy cập được bằng bàn phím và đọc được bằng trình đọc màn hình.
- [ ] Không file nào vượt `max-lines`.

## Rủi ro và lưu ý

- Văn bản hộp thoại khác bản gợi ý của người yêu cầu một vế (Clarification); ghi trong PR để người yêu cầu xác nhận.
- Số bước lấy từ `tree.phases.length`; nếu Plan lớn bị cắt (`truncated`) thì số có thể thấp hơn thật: dùng cùng nguồn với thẻ hiện có và không bịa số.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
