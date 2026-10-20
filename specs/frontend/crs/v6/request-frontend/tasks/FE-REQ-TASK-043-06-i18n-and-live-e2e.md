# FE-REQ-TASK-043-06: i18n 5 locale và e2e giao diện (spec `approve-run-all`, cập nhật `continuous-execute-real`)

**From Solution:** [FE-REQ-SOL-043](../solutions/FE-REQ-SOL-043-approve-plan-and-run-all-steps-ui.md)
**Priority:** P1
**Area:** frontend / request
**File:** `frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json`, `tests/stages/e2e/orca-flows/ui-live/approve-run-all.web.e2e.ts` (mới) + `live-run-all-steps.ts` (mới), `tests/stages/e2e/orca-flows/ui-live/continuous-execute-real.web.e2e.ts` (sửa), `tests/stages/e2e/orca-flows/run_ui_flow.py` (docstring)
**Depends on:** FE-REQ-TASK-043-03, 043-04, 043-05, backend TASK-REQ-043-06
**Status:** [ ] PARTIAL (2026-10-20): đã làm và kiểm bằng đơn vị; còn lại: not run on the stack

---

## Context

- Lint chạy `verify:localization-catalog` và `verify:localization-coverage`; cả hai **đỏ sẵn trên HEAD** theo ghi chú của FE-REQ-TASK-040-05 (không do CR này): chạy trên bản sao `git archive` của HEAD và của cây làm việc, so đầu ra, và kiểm ngang hàng khoá mới có đủ 5 locale cùng placeholder.
- Spec UI dùng `runLiveFlow`, `configureProject` (`live-continuous-execution-steps.ts`; hiện đặt chế độ bằng WS vì chưa có màn hình), fixture LedgerLite (`live-ledgerlite-project-flow.ts`), proxy WS để tắt cờ tính năng (`live-base-branch-gateway-features.ts`, kỹ thuật `page.routeWebSocket`).
- `--ai-stub` dừng ở duyệt Plan vì claude giả chưa nhận prompt thực thi; backend TASK-REQ-043-06 mở rộng stub. `qa-bug-task-native-flow` cũng dừng ở đó vì lý do này.
- Thực thi task chỉ khi `ORCA_E2E_EXECUTE_AGENT=1`. Guard quiet-checkout khi `--real` chạy.

## Việc cần làm

1. Thêm mọi khoá mới vào 5 locale (en là nguồn; es/ja/ko/zh dịch có kiểm tra, ghi vào PR rằng cần người bản ngữ rà): nút phụ, hộp thoại (tiêu đề, thân, hai nút, vế pre-deploy), toast cảnh báo, ba thông điệp lỗi, nhãn tab Execution + tooltip, mục Project Settings (tiêu đề, hai lựa chọn, mô tả, ghi chú, chỉ-đọc, xung đột, ghi chú `version === 0`, lỗi, Retry).
2. Spec `approve-run-all.web.e2e.ts` (`ORCA_E2E_LIVE=1`, `--ai-stub`), fixture LedgerLite, dự án engine `native`: (a) dự án `step_by_step`: Plan `change_request` có Phase hiện hai nút "Approve plan" và "Approve and run all steps"; (b) bấm nút phụ: hộp thoại có đúng số bước và nội dung; "Not now" không đổi `request.get`; (c) xác nhận: `request.get.executionMode == continuous`, Plan tab hiện dải "Plan approved … The agent runs it on its own." và `PlanOutline`, không có Approval `phase` chờ, tab Execution hiện nhãn "Runs all steps on its own"; (d) duyệt thường ở một Request khác: hành vi từng bước không đổi, tab Execution hiện "Asks before each step"; (e) Request đã `continuous`: chỉ nút chính, và nút chính vẫn thành công; (f) proxy WS bỏ `approval.runAll` khỏi `features`: nút phụ ẩn; (g) Project Settings: admin đổi sang "Run all steps after one approval", tải lại vẫn giữ, `request.engineSettings` khớp, engine/`solutionAgent` không đổi; trả lại giá trị ban đầu khi xong.
3. Đăng ký tên spec cho `run_ui_flow.py` (tên là bộ lọc Playwright nên `-- approve-run-all` chạy được ngay; cập nhật docstring).
4. Cập nhật `continuous-execute-real.web.e2e.ts` (loại B, Claude thật, native, repo tạm): thay `configureProject(continuous)` bằng dự án `step_by_step` rồi bấm "Approve and run all steps"; kiểm chạy hết mà không có lần bấm nào thêm.
5. Chụp ảnh các trạng thái chính (hai nút, hộp thoại, nhãn Execution, mục Project Settings) vào thư mục ảnh của harness.

## Kiểm thử

- Hai lệnh verify i18n: so với mốc HEAD, không thêm dòng đỏ nào thuộc file đã đổi; khoá mới có đủ 5 locale.
- `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- approve-run-all` xanh trên stack local.
- `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real continuous` một lần (loại B); chạy lại `qa-bug-task-native-flow` (stub) và `qa-regressions` để chắc không hồi quy. **Không** chạy OpenSpec thật (loại C, 7 đến 13 USD mỗi Plan) khi chưa được đồng ý.

## Tiêu chí hoàn thành

- [x] Đủ khoá ở 5 locale; hai lệnh verify không tệ hơn mốc. (28 khoá, ghi nguyên tử vào 5 locale bằng một script, placeholder `{{n}}`/`{{preDeploy}}` đồng nhất; `plan-locale-coverage.test.ts` xanh. **Hai lệnh `verify:localization-catalog` / `verify:localization-coverage` không chạy được ở cây này**: `config/scripts/verify-localization-catalog.mjs` và `audit-localization-coverage.mjs` không tồn tại; chỉ có `check-max-lines-ratchet.mjs`. Thay bằng kiểm tự viết: mọi khoá `translate(...)` của file đã đổi tồn tại ở `en.json` và mặc định trong mã bằng chuỗi `en`).
- [ ] Spec `approve-run-all` xanh với claude giả; `continuous-execute-real` xanh ở loại B hoặc ghi rõ lý do chưa chạy.
- [ ] Không hồi quy các spec QA hiện có.

## Rủi ro và lưu ý

- Dưới stub không có task chạy thật nên không kiểm được "chạy hết tới xong" ở spec này; việc đó thuộc loại B.
- Dev stack mặc định engine `openspec`: spec phải đặt `native` và tắt `solutionAgent` cho dự án thử.
- Sau khi triển khai lại stack local phải khởi động lại gateway rồi mới chạy; build lại container frontend nếu spec thấy bản cũ.
- Bản dịch es/ja/ko/zh cần người bản ngữ rà.

## Ghi chú triển khai (2026-10-20)

**Trạng thái: PARTIAL.** Phần i18n xong và kiểm bằng đơn vị; hai spec UI đã viết và kiểm kiểu nhưng **not run on the stack** (theo yêu cầu: không chạy, không dựng lại container frontend). Hai tiêu chí còn lại để nguyên chưa tick: "Spec `approve-run-all` xanh với claude giả; `continuous-execute-real` xanh ở loại B" và "Không hồi quy các spec QA hiện có".

**i18n**: 28 khoá mới ở `frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json`, nhóm `auto.components.request.plan.{PlanApprovalBar (runAllAlreadyPinned, runAllNotPossible), PlanRunAllButton, RunAllStepsConfirmDialog}`, `auto.components.request.execution.ExecutionRunModeLabel`, `auto.components.project.ProjectRequestExecutionSection`. Script ghi một lần cho cả 5 file (đọc, thêm, kiểm đồng nhất placeholder, rồi mới ghi; từ chối ghi đè khoá có sẵn khác giá trị; round-trip JSON của 5 file giống hệt byte). **Bản dịch es/ja/ko/zh do tôi dịch, cần người bản ngữ rà.**

**Spec UI** (`tests/stages/e2e/orca-flows/ui-live/`): mới `approve-run-all.web.e2e.ts` (4 test), `live-run-all-steps.ts`, `live-run-all-gateway-features.ts` (+ `.test.ts`, 5 ca, xanh bằng `npx vitest run` từ gốc repo); sửa `continuous-execute-real.web.e2e.ts` (dự án `step_by_step`, `expectBothApproveButtons` rồi `approveAndRunAllThroughTheUi`, không còn `configureProject(continuous)`); `run_ui_flow.py` chỉ sửa docstring (tên spec là bộ lọc Playwright).
- Test 1: hai nút, thứ tự Tab; hộp thoại có đúng số Phase (từ `task.list`) và nội dung; "Not now" và Esc không đổi `request.get` (version, status, Approval vẫn pending) và trả focus về nút; xác nhận → `executing`, `executionMode=continuous`, không Approval `phase`, Plan tab có dải "The agent runs it on its own." và `plan-outline`, Execution tab có "Runs all steps on its own".
- Test 2: cửa sổ thứ hai qua proxy WS bỏ `approval.runAll` chỉ còn "Approve plan" (kiểm proxy đã viết lại ≥ 1 câu trả lời); sau đó duyệt thường: Approval `phase` mở, nút "Approve phase" hiện, Execution tab "Asks before each step".
- Test 3: dự án `continuous`: thẻ duyệt một nút "Approve and start execution", không `plan-approve-run-all`, bấm thì chạy tự động.
- Test 4: Project Settings (sidebar "Project Workspace (Beta)" → project switcher → bánh răng): admin đổi sang "Run all steps after one approval", lưu; `request.engineSettings` đổi đúng `executionMode`, `version+1`, các trường khác nguyên; tải lại vẫn giữ; trả về "Ask before each step".
- Ảnh chụp: `run-all-two-buttons`, `run-all-dialog`, `run-all-execution-label`, `step-by-step-execution-label`, `run-all-continuous-card`, `project-settings-execution-before`, `project-settings-execution-saved`, `plan-before-run-all`, `run-all-flag-off` (thư mục output Playwright).

**Đã chạy ở máy này (không phải stack)**: kiểm kiểu bằng tsconfig tạm (`moduleResolution: Bundler`, `target ES2023`, `strict`) trên `approve-run-all.web.e2e.ts`, `continuous-execute-real.web.e2e.ts`, `live-run-all-gateway-features.test.ts` và mọi file import: sạch. `ORCA_E2E_LIVE= npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live --list approve-run-all`: liệt kê đủ 4 test. `npx oxlint` và `npx oxfmt --check` sạch. `ORCA_E2E_LIVE=1` **không** được đặt, không chạy test nào.

**Rủi ro chưa kiểm vì chưa chạy trên stack**
- Lối vào Project Settings: chỉ có ở trang Workspace (Beta) (`ProjectSwitcher` chỉ dựng ở `App.tsx` khi `activeView === 'workspace'`); nếu nút sidebar "Project Workspace (Beta)" bị ẩn trong bản web thì test 4 hỏng ở bước đó, không phải ở tính năng.
- Dưới claude giả, `change_request` phải ra Plan có Phase (`phased` trong `fake_claude_cli/claude`); `phaseCount > 0` được khẳng định để lỗi rõ ràng. Backend TASK-REQ-043-06 (mở rộng stub cho prompt thực thi) chưa có nên các bước sau `executing` chỉ kiểm trạng thái, không kiểm chạy hết.
- Hộp thoại: focus trả về nút mở được kiểm bằng đơn vị (happy-dom), chưa bằng trình duyệt thật.
- `continuous-execute-real` (loại B) chưa chạy; không chạy OpenSpec thật (loại C).
- Cần build lại container frontend và khởi động lại gateway sau khi triển khai lại stack trước khi chạy hai spec.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
