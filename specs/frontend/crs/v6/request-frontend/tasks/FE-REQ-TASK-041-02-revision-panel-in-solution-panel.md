# FE-REQ-TASK-041-02: `AnalysisRevisionPanel` trong `SolutionPanel`

**From Solution:** [FE-REQ-SOL-041](../solutions/FE-REQ-SOL-041-analysis-revision-by-prompt.md)
**Priority:** P1
**Area:** frontend / request / solution
**File:** `renderer/src/components/request/solution/{AnalysisRevisionPanel.tsx,SolutionPanel.tsx}` (sửa), test
**Depends on:** FE-REQ-TASK-041-01
**Status:** [x] DONE (2026-10-19)

---

## Context

- `SolutionPanel` dựng thanh quyết định và danh sách phiên bản; `ordered` là danh sách phiên bản, `viewId` chọn bản xem.
- Panel chỉ có ý nghĩa khi `awaiting_analysis_approval` và xem bản hiện hành.

## Việc cần làm

1. `AnalysisRevisionPanel`: textarea, bộ đếm, chip, nút "Revise analysis", trạng thái "Analyzing version n+1…" khi `busy`, thông báo lỗi dưới ô; `aria-live` cho trạng thái; nhãn truy cập đầy đủ.
2. `SolutionPanel`: chèn panel cạnh `SolutionDecisionBar`; khi `busy` khoá Approve, Choose, Reject, Regenerate, Revise (truyền cờ vào `SolutionDecisionBar`).
3. Ẩn theo loại không có cổng Analysis (`request.flow`) và khi xem bản cũ.
4. Ctrl/Cmd+Enter gửi (theo nền tảng bằng kiểm tra runtime, nhãn `⌘`/`Ctrl+`), không phím tắt nào khác.
5. Sau thành công, xoá ô và chuyển xem sang bản mới khi nó xuất hiện.
6. Dùng token `main.css`, primitive `Textarea`, `Button`; không màu cứng.

## Kiểm thử

- Component: các trạng thái (rỗng, đếm, chip chèn câu, đang chạy, lỗi, ẩn theo loại/quyền/bản cũ); khoá nút khi `busy`; phím tắt theo nền tảng.
- Test `SolutionPanel`/`SolutionDecisionBar` hiện có xanh.

## Tiêu chí hoàn thành

- [x] Chỉnh bằng một thao tác, không qua từ chối.
- [x] Nút quyết định bị khoá trong lúc chạy.
- [x] Đúng truy cập bàn phím và đọc màn hình.

## Rủi ro và lưu ý

- `SolutionPanel.tsx` đã dài: tách nếu cần để dưới `max-lines`.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.

---

## Ghi chú triển khai (2026-10-19)

**File** (dưới `frontend/src/renderer/src/components/request/solution/`):
- `AnalysisRevisionPanel.tsx` (mới): `<section aria-labelledby>` với `Textarea` (có `<label>`, `aria-describedby` tới bộ đếm và lỗi, `aria-invalid`, `aria-keyshortcuts`), bộ đếm `n / 2000` theo ký tự Unicode (không `maxLength` để không cắt âm thầm; quá giới hạn thì chữ đỏ và hook báo `tooLong`), chip theo loại tài liệu (chỉ điền chữ, nối bằng một dấu cách, trả focus về ô), nút "Revise analysis" (spinner khi busy), vùng `role=status aria-live=polite` luôn có mặt (rỗng khi rảnh; "Analyzing version n…" khi busy, "Analyzing the next version…" khi Solution hiện hành không có `seq`), lỗi `role=alert` ngay dưới ô. Ctrl/Cmd+Enter: `getShortcutPlatform()` (userAgent), macOS chỉ `metaKey`, nơi khác chỉ `ctrlKey`; bỏ qua khi có Shift/Alt, khi đang gõ IME, hoặc modifier sai nền tảng; không phím tắt nào khác. Nhãn gợi ý "⌘Enter" / "Ctrl+Enter".
- `SolutionPanel.tsx` (sửa): gọi `useAnalysisRevision` (viewed solution, `actions: {generate}`, `onSettled: refetchAll`); panel nằm cuối vùng cuộn, cùng cột với `SolutionDecisionBar` (không sticky để không ăn chiều cao cố định). Điều kiện hiện: `isCurrent && (busy || (canShow && decision bar hiện))`; `busy` giữ panel hiện dù Request đã sang `analyzing` (nếu không, "Analyzing version n+1…" biến mất đúng lúc cần). `canShow` đã gồm loại có cổng Analysis (`REQUEST_FLOW_REGISTRY[type].analysisGate`) và `awaiting_analysis_approval`. Khoá khi busy: truyền `locked` vào `SolutionDecisionBar`, và thêm `|| revision.busy` cho `regenerateDisabled` của `SolutionBody` và `SolutionRunFailure`.
- `SolutionDecisionBar.tsx` (sửa nhỏ): prop tuỳ chọn `locked` (mặc định false) vào `writeBusy` (Approve, Choose qua Approve, Reject), Regenerate và Generate plan. Blast radius (grep): chỉ `SolutionPanel` dùng, cộng test `SolutionDecisionBar.decision.test.tsx` (không đổi, vẫn xanh).
- Sau thành công: hook xoá ô; `viewId = null` nghĩa là "theo bản mới nhất" nên khi bản mới xuất hiện màn hình tự chuyển (xem 041-03).
- Để `SolutionPanel.tsx` xuống 289 dòng mã (từ 399, giới hạn tsx là 400, brief yêu cầu dưới 300) đã tách mã có sẵn, không đổi hành vi: `useSolutionPanelSync.ts` (các effect đồng bộ/poll: sync key, chờ approval row, poll run, chờ bản mới; `readCooldown` chuyển vào đây), `SolutionEmptyState.tsx` (nhánh chưa có Solution), `SolutionPanelLoadStates.tsx` (skeleton và lỗi tải). Các test `SolutionPanel*` có sẵn giữ nguyên kỳ vọng và xanh.
- i18n: nhóm `request.AnalysisRevision.{title,hint,fieldLabel,placeholder,suggestions,counter,submit,shortcutHint,busy,busyNext}` × 5 locale (dịch tay).
- Test mới: `AnalysisRevisionPanel.test.tsx` (13: rỗng, đếm ký tự/emoji, giới hạn, chip theo loại, chip lạ, gửi, busy + aria-live, lỗi dưới ô, phím tắt macOS/Windows/Linux, không phím tắt khác), `SolutionPanel.revision.test.tsx` (hiện/ẩn theo trạng thái, loại hotfix, bản đã quyết định, gửi qua `generate` không qua `reject`, xoá ô khi thành công, giữ chữ khi lỗi, khoá Approve/Reject/Regenerate khi busy rồi mở khoá khi bản mới đến).

**Blast radius `SolutionPanel`**: chỉ `RequestAnalysisTab` import nó. Thêm tính năng, không đổi chữ ký.

**Chưa kiểm**
- Chưa xem trực quan trong ứng dụng (happy-dom không có bố cục): chiều cao panel ở cửa sổ nhỏ, màu token ở theme tối.
- Chưa thử với trình đọc màn hình thật; chỉ kiểm thuộc tính ARIA (label, describedby, live region, invalid) bằng test.
- Chưa chạy với backend thật hay e2e (task 041-04).
- Quyền: UI không biết trước ai là approver (đúng SOL 2.3); người không phải approver sẽ thấy lỗi `forbidden` dưới ô và chữ được giữ.

**Kiểm chứng đã chạy**
- `cd frontend && npx vitest run --config config/vitest.config.ts src/renderer/src/components/request src/shared/request-solution-wire-parser.test.ts`: 84 file, 933 test xanh (gồm toàn bộ `components/request/solution`: 23 file, 327 test, và test của agent khác trong `components/request`).
- Kiểm kiểu: `npx tsc --noEmit -p tsconfig.json --composite false --incremental false` còn 113 lỗi, đều có sẵn (không lỗi nào ở `components/request/solution`); trước thay đổi của task này cũng 113 (sau 041-01).
- Lint: `npx oxlint frontend/src/renderer/src/components/request/solution` sạch (lỗi `max-lines`/`curly` còn lại nằm ở `src/shared/execution-host-registry.ts`, `task-status-normalization.ts`, không thuộc task).
- i18n: `verify-localization-catalog.mjs` và `audit-localization-coverage.mjs --check` chạy trên bản sao `src/renderer/src` (script `verify-i18n.sh` ở scratchpad, thư mục riêng `lroot041`): không dòng nào thuộc file của task (các lỗi còn lại có sẵn trên HEAD: `DepartmentGate`, `web/`, `useAskBack.ts`). `analysis-revision-ui-locale-coverage.test.ts` kiểm mọi `translate(...)` trong 4 file UI mới có đủ 5 locale, en khớp fallback, cùng placeholder. Các khoá của agent khác (`gitRefPicker.*`, `BaseRef*`) không bị đụng: chỉ thêm khoá của task và bỏ 2 khoá không còn dùng (`SolutionVersionSwitcher.old`, `.version`).
