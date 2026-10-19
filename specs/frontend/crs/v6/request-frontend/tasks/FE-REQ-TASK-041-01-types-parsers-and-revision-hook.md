# FE-REQ-TASK-041-01: Kiểu, parser và hook `useAnalysisRevision`

**From Solution:** [FE-REQ-SOL-041](../solutions/FE-REQ-SOL-041-analysis-revision-by-prompt.md)
**Priority:** P1
**Area:** frontend / request / solution
**File:** `frontend/src/shared/{request-types,request-wire-parsers}.ts` + parser Solution, `renderer/src/components/request/solution/{useAnalysisRevision,revision-suggestions}.ts`, test
**Depends on:** Backend TASK-REQ-041-02
**Status:** [x] DONE (2026-10-19)

---

## Context

- `Solution` chưa có `revisionFeedback`, `seq`.
- `useSolutionDecision` có `actions.generate({feedback, idempotencyKey})`, `clampFeedback`; mã lỗi đã phân loại ở `solution-error-classification.ts`.

## Việc cần làm

1. `Solution.revisionFeedback?`, `Solution.seq?`; parser chịu thiếu/sai kiểu, không ném lỗi.
2. `useAnalysisRevision` theo SOL-041 mục 2.2: kiểm rỗng/độ dài, gọi `generate` trực tiếp, `busy` theo trạng thái Request, ánh xạ lỗi, giữ nội dung khi lỗi.
3. `revision-suggestions.ts`: chip theo `SolutionKind` (solution, diagnosis → solution kind với `diagnosisFocus` không cần ở UI, findings, answer), nhãn qua `translate`.
4. Không gọi `reject`.

## Kiểm thử

- Hook: thành công; rỗng; quá dài; `FORBIDDEN` giữ nội dung; `WRONG_STATE` làm mới; `NO_CONNECTION`; `busy` kéo dài đến khi trạng thái Request đổi (dùng sự kiện giả); bấm đúp chỉ một lời gọi.
- Parser: bảng ca.

## Tiêu chí hoàn thành

- [x] `submit` chỉ gọi `generate`, không `reject`.
- [x] Lỗi không làm mất nội dung đã gõ.
- [x] `busy` theo trạng thái Request.

## Rủi ro và lưu ý

- Tham gia chung `useSolutionDecision`: không đổi hành vi hiện có của nó.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.

## Ghi chú triển khai (2026-10-19)

**File đổi/tạo** (dưới `frontend/src/`):
- `shared/request-types.ts`: `Solution.revisionFeedback?`, `Solution.seq?`.
- `shared/request-solution-wire-parser.ts`: `revisionFeedback` (chuỗi, `''` giữ nguyên), `seq` chỉ nhận số nguyên >= 1 (0 là bản chạy thử của backend; sai kiểu/âm/thập phân/NaN thành `undefined`); không ném lỗi.
- `renderer/src/components/request/solution/useAnalysisRevision.ts` (mới): `useAnalysisRevision({request, solution, actions, onSettled})` trả `{canShow, busy, nextVersion, draft, setDraft, error, clearError, submit}`.
  - `submit(text?)` kiểm rỗng (sau trim) và độ dài (2000 **ký tự Unicode**, không cắt âm thầm như `clampFeedback`; vượt thì báo lỗi và giữ chữ), rồi gọi `actions.generate({feedback, idempotencyKey})` với khoá mới mỗi lần (`createBrowserUuid`, an toàn khi `crypto.randomUUID` không có, vd. trang HTTP thuần); **không có đường nào gọi `reject`** (hook chỉ nhận `Pick<…,'generate'>`); không bao giờ reject Promise (action ném thì bắt, `console.error`, trả lỗi); khoá đồng bộ bằng ref nên bấm đúp chỉ một lời gọi.
  - Giữ nội dung: hook sở hữu `draft` (theo `request.id`), chỉ xoá khi thành công; mọi lỗi giữ nguyên; `setDraft` xoá lỗi cũ.
  - `busy` = đang gọi hoặc "Request chưa đi hết một vòng": `analyzing`, hoặc còn ở cổng với cùng `version`/`updatedAt`/Solution hiện hành như lúc gửi; thấy version/mốc thời gian/Solution đổi, hoặc trạng thái khác thì kết thúc (không treo nếu bỏ lỡ `analyzing`). Khi busy, sự kiện bus của Request (status_changed, solution.*, approval.*, poll) gọi `onSettled()`.
  - `canShow` = flow có cổng Analysis (`REQUEST_FLOW_REGISTRY[type].analysisGate`; hotfix/task/docs/ops_request/unknown thì không) && `awaiting_analysis_approval` && Solution đang xem ở `ready` (bản cũ/superseded/rejected không sửa được).
- `…/solution/analysis-revision-errors.ts` (mới): ánh xạ lỗi: `REQUEST_SOLUTION_FEEDBACK_TOO_LONG`→tooLong; `REQUEST_SOLUTION_WRONG_STATE` hoặc conflict→wrongState (gọi `onSettled` để làm mới); `REQUEST_ANALYSIS_NO_CONNECTION`/`REQUEST_SOLUTION_NO_CONNECTION`→noConnection ("bản hiện tại không đổi"); forbidden; rate limited; network; còn lại là thông điệp chung (không phát lại thông điệp máy chủ). Nhận theo mã backend vì `shared/request-errors.ts` chưa có mã độ dài.
- `…/solution/revision-suggestions.ts` (mới): chip theo `SolutionKind` (`solution`, `diagnosis`, `findings`, `answer`; 4 chip mỗi loại, kind lạ thì rỗng); chip chỉ điền chữ, không gửi.
- i18n: nhóm `auto.components.request.AnalysisRevision.*` (5 lỗi + 16 gợi ý) × 5 locale, bản dịch tay.
- Test: `useAnalysisRevision.test.ts` (45), `analysis-revision-errors.test.ts`, `revision-suggestions.test.ts`, `analysis-revision-locale-coverage.test.ts`, và các ca parser trong `request-solution-wire-parser.test.ts`.

**Không đổi `useSolutionDecision`** (chỉ import kiểu `SolutionDecisionActions`). Hook chưa được nối vào `SolutionPanel` (task sau, theo phân công).

**Blast radius** (grep): `parseSolution` được dùng ở `request-wire-parsers` (re-export), `useSolutions` và vài test; thêm hai trường tuỳ chọn nên không ảnh hưởng nơi đọc.

**Kiểm chứng đã chạy**
- `cd frontend && npx vitest run --config config/vitest.config.ts src/shared/request-solution-wire-parser.test.ts src/renderer/src/components/request/solution` xanh (kể cả toàn bộ test `SolutionPanel*`, `useSolutionDecision`, `solution-locale-coverage` có sẵn).
- Chạy rộng `src/shared src/renderer/src/hooks src/renderer/src/store src/renderer/src/components/request src/renderer/src/components/settings src/renderer/src/runtime src/renderer/src/lib`: 9 file/36 test đỏ, đều không liên quan (mcp-approval-slice, McpExternalServerDialog, useLogout, useIpcEvents, runtime-cli-client, selectors, feature-interactions, remote-runtime-shared-control-boundary: thiếu `window.api`, `localStorage` của Node 25, `src/main` không tồn tại ở `frontend/`, `useClarifications` do `localStorage.length`); không có file nào thuộc vùng đã sửa. Không chạy được bản "trước thay đổi" (không được `git stash`), nên xác nhận bằng thông điệp lỗi.
- Kiểm kiểu: `cd frontend && npx tsc --noEmit -p tsconfig.json --composite false --incremental false` có 114 lỗi có sẵn (admin/fleet, store/ui, test mock `AppState`...); sau thay đổi còn 113, không lỗi mới (bớt 1 lỗi `beforeEach` không dùng ở `request.test.ts`). Dùng `--incremental false` để không ghi đè `frontend/tsconfig.tsbuildinfo`.
- Lint: `npx oxlint <các file đổi>` sạch. `pnpm run lint` nguyên bản không chạy được ở gốc repo vì repo đã tách (`config/scripts/verify-localization-catalog.mjs`, `config/oxlint-switch-exhaustiveness.json` nằm ở `desktop/`), và `check:max-lines-ratchet` báo lỗi có sẵn ở các file `desktop/`/`tests/` không thuộc task này.
- i18n: `verify-localization-catalog` và `audit-localization-coverage --check` hiện **thất bại sẵn trên HEAD** (284 tham chiếu khoá thiếu trong `en.json`, 1227 chuỗi cứng; chủ yếu `web/`, `DepartmentGate`...). Đã chạy hai script `desktop/config/scripts/*` trên bản sao `src/renderer/src` ở scratchpad (script đòi `<root>/src/{main,renderer/src}`) và so sánh đầu ra trước/sau: **giống hệt**, tức không thêm khoá thiếu hay chuỗi cứng. Thêm kiểm ngang hàng: không locale nào thiếu/thừa khoá hay lệch placeholder so với `en` do thay đổi này (631 khoá thiếu ở es/ja/ko/zh là có sẵn trên HEAD).

**Chưa kiểm**
- Chưa chạy với backend thật (BE-REQ-041-02): mã lỗi và tên trường `revisionFeedback`/`seq` theo brief và proto (`Solution.seq = 12`, `revision_feedback = 18` trong `solution.proto`), chưa đối chiếu cách gateway đặt tên JSON.
- "Analyzing version n+1…" chỉ có `nextVersion` khi Solution hiện hành có `seq`; panel (task sau) cần dự phòng.
