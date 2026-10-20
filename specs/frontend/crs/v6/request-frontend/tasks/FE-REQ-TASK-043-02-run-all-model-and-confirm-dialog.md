# FE-REQ-TASK-043-02: Quy tắc mời lựa chọn và hộp thoại xác nhận "Run all steps"

**From Solution:** [FE-REQ-SOL-043](../solutions/FE-REQ-SOL-043-approve-plan-and-run-all-steps-ui.md)
**Priority:** P1
**Area:** frontend / request
**File:** `frontend/src/renderer/src/components/request/plan/{plan-run-all-model.ts,RunAllStepsConfirmDialog.tsx,PlanRunAllButton.tsx}` và test
**Depends on:** FE-REQ-TASK-043-01
**Status:** [x] DONE (2026-10-20)

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

- [x] Quy tắc mời lựa chọn đúng tập luồng có Phase của backend, không dựa vào bảng dự phòng. (`plan-run-all-model.test.ts`: mỗi điều kiện sai một lần, kể cả `fallback`, `hasPhases=false`, `continuous`, `task_list` hợp lệ).
- [x] Hộp thoại đúng nội dung, truy cập được bằng bàn phím và đọc được bằng trình đọc màn hình. (`PlanRunAllButton.test.tsx`: tên accessible của dialog là tiêu đề, mô tả accessible; Esc đóng, Esc bị chặn khi đang gửi, focus trả về nút mở).
- [x] Không file nào vượt `max-lines`. (file lớn nhất 96 dòng; `npx oxlint` sạch).

## Rủi ro và lưu ý

- Văn bản hộp thoại khác bản gợi ý của người yêu cầu một vế (Clarification); ghi trong PR để người yêu cầu xác nhận.
- Số bước lấy từ `tree.phases.length`; nếu Plan lớn bị cắt (`truncated`) thì số có thể thấp hơn thật: dùng cùng nguồn với thẻ hiện có và không bịa số.

## Ghi chú triển khai (2026-10-20)

**File mới** (`frontend/src/renderer/src/components/request/plan/`): `plan-run-all-model.ts` (`canOfferRunAll`, `runAllDialogModel`), `RunAllStepsConfirmDialog.tsx`, `PlanRunAllButton.tsx`, và test `plan-run-all-model.test.ts` (19), `PlanRunAllButton.test.tsx` (8). `plan-test-fixtures.ts` thêm `backendFlowOf` (luồng `source: 'backend'`).

**Thiết kế**
- `canOfferRunAll` nhận `alreadyPinned` (tên theo task, solution gọi `alreadyPinnedHint`) và xét `rawSubjectType ?? subjectType`.
- `PlanRunAllButton` nhận `locked`, `blockedReason`, `onConfirm` (trả `PlanDecisionOutcome`); chỉ đường xác nhận mới gửi. Đóng dialog sau khi `onConfirm` xong (kể cả lỗi: thông điệp lỗi đã hiện ở thanh duyệt); toast cảnh báo khi `runAllApplied === false`. Khoá gửi bằng `useRef` nên nhấp đúp xác nhận chỉ một lần.
- Văn bản thân hộp thoại tách hai đoạn (`bodyStart`, `bodyStops`) trong `DialogDescription asChild`; vế pre-deploy là khoá riêng `preDeploy` để dịch được. Văn bản khác bản gợi ý của người yêu cầu một vế (Clarification: "when the agent needs information from you"), cần người yêu cầu xác nhận.
- Radix chỉ trả focus về `DialogTrigger`; hộp thoại này mở bằng state nên `PlanRunAllButton` tự đặt `onCloseAutoFocus` trả focus về nút mở (có test).
- Biểu tượng nút: `FastForward` (lucide), cỡ `sm` như nút chính, không màu mới.
- Số bước lấy từ `tree.phases.length` (cùng nguồn với `PlanApprovalCard`); bằng 0 thì thanh duyệt không mời lựa chọn (xem 03) để khỏi bịa "Run all 0 steps".

**Kiểm chứng**: `npx vitest run --config config/vitest.config.ts src/renderer/src/components/request/plan`: xanh (trừ `plan-locale-coverage` cho tới khi có khoá locale ở 06; sau 06 xanh). `oxlint`, `oxfmt --check` sạch.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
