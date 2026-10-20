# FE-REQ-TASK-043-04: Nhãn chế độ chạy trên dải tóm tắt tab Execution

**From Solution:** [FE-REQ-SOL-043](../solutions/FE-REQ-SOL-043-approve-plan-and-run-all-steps-ui.md)
**Priority:** P2
**Area:** frontend / request
**File:** `frontend/src/renderer/src/components/request/execution/{execution-run-mode-model.ts,ExecutionRunModeLabel.tsx,ExecutionSummaryStrip.tsx,ExecutionTab.tsx}` và test (`execution-components.test.tsx`, `execution-run-mode-model.test.ts`)
**Depends on:** FE-REQ-TASK-043-01
**Status:** [x] DONE (2026-10-20)

---

## Context

- `ExecutionSummary.executionMode` có sẵn (`shared/request-execution-types.ts:107`, parser `request-execution-parsers.ts:140`), rỗng trước khi Request vào `executing`.
- `ExecutionSummaryStrip` hiện chỉ có nhãn trạng thái, tiến độ, thời gian và "Stop run"; chưa dùng `executionMode`.
- `ExecutionTab` đã có `request` và (nên có) luồng; kiểm cách lấy `hasPhases` (`flow`) trong tab, nếu chưa có thì nhận qua props từ `RequestDetailPane` như `RequestPlanTab`.
- Không thêm màu mới (F9 của CR-REQ-037); không thêm nút nào.

## Việc cần làm

1. `execution-run-mode-model.ts`: `runModeLabelOf(mode, hasPhases): 'continuous' | 'step_by_step' | null`: `continuous` → hiện; `step_by_step` chỉ khi `hasPhases`; còn lại `null`.
2. `ExecutionRunModeLabel.tsx`: chip nhỏ theo thành phần có sẵn (`Badge` hoặc văn bản `text-xs text-muted-foreground` kèm biểu tượng); "Runs all steps on its own" / "Asks before each step"; tooltip (`components/ui/tooltip`) "Steps run with the access of the person who approved the plan."; `data-testid="execution-run-mode"`.
3. `ExecutionSummaryStrip`: prop `runMode?: 'continuous' | 'step_by_step' | null` và hiển thị nhãn cạnh nhãn trạng thái; `ExecutionTab` truyền `runModeLabelOf(summary.executionMode, flow?.hasPhases)`.
4. Không hiện khi `null`; không hiện khi gateway cũ không trả `executionMode` (parser trả `undefined`).

## Kiểm thử

- Model: bảng (`continuous`, `step_by_step` có/không Phase, rỗng, `undefined`).
- Component: ba trạng thái; tooltip; không đổi bố cục khi `null`; "Stop run" vẫn đúng chỗ.
- `execution-components.test.tsx` hiện có xanh.

## Tiêu chí hoàn thành

- [x] Nhãn đúng cho Request `continuous` và `step_by_step` có Phase; vắng ở các trường hợp còn lại. (`execution-run-mode-model.test.ts` 8 ca; `ExecutionTab.run-mode.test.tsx` 3 ca; `ExecutionRunModeLabel.test.tsx`).
- [x] Không có nút Pause hay điều khiển mới. (chip chỉ là văn bản có tooltip; test khẳng định không có nút Pause và "Stop run" vẫn có).

## Rủi ro và lưu ý

- Không có tên người duyệt trong `summary`; tooltip chỉ nói "the person who approved the plan" (không thêm trường backend).
- Chạy `gitnexus_impact` cho `ExecutionSummaryStrip`.

## Ghi chú triển khai (2026-10-20)

**File** (`frontend/src/renderer/src/components/request/`): mới `execution/execution-run-mode-model.ts`, `execution/ExecutionRunModeLabel.tsx` (`Badge variant="outline"` có `tabIndex=0` để tooltip tới được bằng bàn phím, tự bọc `TooltipProvider`), test `execution-run-mode-model.test.ts`, `ExecutionRunModeLabel.test.tsx`, `ExecutionTab.run-mode.test.tsx`; sửa `execution/ExecutionSummaryStrip.tsx` (prop `runMode`), `execution/ExecutionTab.tsx` (prop `flow?`, truyền `runModeLabelOf(summary.executionMode, flow?.hasPhases)`).
- **Ngoài danh sách file được giao**: `RequestDetailPane.tsx` thêm đúng một dòng `flow={flow}` cho `ExecutionTab` (task cho phép "nhận qua props từ RequestDetailPane như RequestPlanTab"); biến `flow` có sẵn ở đó.

**Blast radius**: `ExecutionSummaryStrip` chỉ dùng ở `ExecutionTab`; `ExecutionTab` chỉ dùng ở `RequestDetailPane`. Prop mới đều tuỳ chọn; `execution-run-components.test.tsx`, `execution-components.test.tsx`, `ExecutionTab.failure-card.test.tsx` xanh.

**Kiểm chứng**: `npx vitest run --config config/vitest.config.ts src/renderer/src/components/request/execution`: 10 file, 71 test xanh. Tooltip chỉ nói "the person who approved the plan" (không thêm trường backend).

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
