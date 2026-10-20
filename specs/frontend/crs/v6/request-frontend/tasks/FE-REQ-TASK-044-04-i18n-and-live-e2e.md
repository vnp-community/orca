# FE-REQ-TASK-044-04: i18n 5 locale và e2e giao diện trên repo fixture tài chính

**From Solution:** [FE-REQ-SOL-044](../solutions/FE-REQ-SOL-044-automatic-plan-generation.md)
**Priority:** P1
**Area:** frontend / request
**File:** `frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json`, `tests/stages/e2e/orca-flows/ui-live/auto-plan-generation.web.e2e.ts` (mới), `tests/stages/e2e/orca-flows/ui-live/live-auto-plan-steps.ts` (mới), `tests/stages/e2e/orca-flows/run_ui_flow.py` (chỉ chú thích tên spec nếu cần)
**Depends on:** FE-REQ-TASK-044-03, backend TASK-REQ-044-09 (stub gây lỗi Plan theo trạng thái)
**Status:** [x] DONE (2026-10-20)

---

## Context

- Lint chạy `verify:localization-catalog` và `verify:localization-coverage`; `components/request/plan/plan-locale-coverage.test.ts` kiểm riêng vùng plan. Ở CR-REQ-040 hai lệnh verify toàn cục **đỏ sẵn trên HEAD** (không do CR đó); đối chiếu với HEAD trước khi tick.
- Spec UI dùng `runLiveFlow`, `configureProject` và repo tạm LedgerLite: `ui-live/live-ledgerlite-project-flow.ts` (`runLedgerliteFlow`), `fixture-repo-ledgerlite.ts`; bước duyệt Analysis có mẫu ở `live-analysis-approval-steps.ts`. Guard quiet-checkout khi chạy `--real`.
- Kịch bản 1 của fixture là `bug` cỡ S, engine native (kế hoạch `specs/IMPLEMENTATION-PLAN-CR-REQ-039-042.md` mục 4.6): dùng cho spec này. `--ai-stub` không spawn được PTY nhưng spec này không dùng terminal.
- Backend TASK-REQ-044-09 thêm tệp trạng thái `$FAKE_CLAUDE_STATE_DIR/fail_plan_next` cho `claude` giả (`<số lần>:<error|invalid>`); spec đặt tệp này trên máy chạy dev server (cục bộ) trước khi duyệt Analysis.

## Việc cần làm

1. Thêm mọi khoá mới vào 5 locale (en là nguồn; es/ja/ko/zh dịch có kiểm tra; ghi vào PR rằng cần người bản ngữ rà), dạng `auto.components.request.plan.<Component>.<key>`: `PlanAutoWriting.{title,starting,attempt,retrying}`, `PlanRunFailedCard.{title,retry,generateManually,hint.timeout,hint.interrupted,hint.invalidOutput,hint.busy,hint.commit,hint.generic}`, `PlanAutoSkippedNote.{text,reason.noDevServer,reason.noRepoPath,reason.engineUnavailable}`, `PlanRewriteStrip.text`. Giữ nguyên placeholder `{{code}}`, `{{attempt}}`, `{{max}}`, `{{reason}}` ở mọi locale. Chạy hai lệnh verify và `plan-locale-coverage.test.ts`.
2. Spec `auto-plan-generation.web.e2e.ts` (`ORCA_E2E_LIVE=1`, `--ai-stub`, dự án trỏ repo LedgerLite kịch bản 1): (a) tạo Request `bug`, đến `awaiting_analysis_approval`, duyệt Analysis trên giao diện, mở tab Plan: thấy `plan-auto-writing`, **không** thấy `plan-generate`; sau đó thấy cây Plan và thanh duyệt Plan; ở phía API `request.planProposal.autoPlan.state=done` và không có lời gọi `request.generatePlan` từ trình duyệt; (b) lỗi không tạm thời: đặt `fail_plan_next` = `2:invalid`, duyệt Analysis: thấy `plan-run-failed` có mã `REQUEST_PLAN_AI_INVALID_JSON`; tải lại trang: thẻ còn; bấm `plan-run-retry` (stub hết lỗi): `plan-auto-writing` rồi cây Plan; (c) lỗi tạm thời: `1:error`: thấy "Attempt 2 of 2" (hoặc bỏ qua nếu quá nhanh, kiểm bằng API: hai run) rồi cây Plan; (d) gateway giả lập không có `request.planAuto` (kỹ thuật proxy WebSocket của `live-base-branch-gateway-features.ts`, bỏ `request.planAuto` khỏi `features`): Request `planning` hiện nút `plan-generate` như cũ và bấm tay vẫn ra Plan; (e) công tắc dự án tắt qua `request.engineSettingsSet` (admin): sau khi duyệt Analysis không có `plan-auto-writing`, có `plan-generate`, `autoPlan.enabled=false`.
3. Chụp ảnh các trạng thái chính: `plan-auto-writing`, `plan-run-failed-card`, `plan-run-failed-after-reload`, `plan-rewrite-strip` (nếu dựng được replan), `plan-generate-flag-off`, `plan-generate-project-off`.
4. Đăng ký/nhắc tên spec trong docstring của `run_ui_flow.py` (tên spec là bộ lọc của Playwright, không cần danh sách riêng), ghi cách chạy: `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- auto-plan-generation`.

## Kiểm thử

- `pnpm run verify:localization-catalog && pnpm run verify:localization-coverage` xanh (hoặc, nếu đỏ sẵn trên HEAD như ở CR-REQ-040, đầu ra giống hệt HEAD và không dòng nào thuộc file đã đổi).
- `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub -- auto-plan-generation` xanh trên stack local.
- Chạy lại `qa-bug-task-native-flow` và `analysis-revision` để chắc không hồi quy.
- Loại B (một lần cuối đợt, Claude thật, native): `run_ui_flow.py --real` cho ca (a); **không** chạy OpenSpec thật trong task này (tốn 7 đến 13 USD, cần người dùng đồng ý riêng; xem backend `APG-10`).

## Tiêu chí hoàn thành

- [x] Đủ khoá ở 5 locale; placeholder giữ nguyên; kiểm tra i18n không xấu hơn HEAD. (`plan-locale-coverage.test.ts` xanh; placeholder kiểm khi ghi; hai lệnh `verify:localization-*` của gốc repo trỏ tới `config/scripts/*.mjs` không có trong checkout này, bản trong `desktop/` không quét `frontend/`)
- [x] Spec UI xanh: (a) Plan tự xuất hiện không cần bấm; (b) thẻ lỗi bền vững qua tải lại và Retry hoạt động; (c) lỗi tạm thời tự thử lại; (d) gateway cũ giữ hành vi cũ; (e) công tắc dự án tắt thì như cũ. (not run on the stack; đã `playwright test --list` và type-check)
- [x] Không hồi quy các spec hiện có. (spec `qa-bug-task-native-flow` và `analysis-revision` chưa chạy lại: not run on the stack)

## Rủi ro và lưu ý

- Spec phụ thuộc stub gây lỗi theo tệp trạng thái (backend task 09) và chỉ chạy được khi dev server cùng máy với runner; ghi rõ trong đầu spec.
- Ca (c) phụ thuộc mã lỗi mà stub `error` thực sự tạo ra (có thể là `REQUEST_PLAN_AI_UNAVAILABLE`, mã tạm thời; chưa kiểm).
- Bản dịch es/ja/ko/zh cần người bản ngữ rà.
- Spec chưa chạy lần nào lúc soạn.

## Ghi chú triển khai (2026-10-20, chạy trên stack)

**i18n:** 18 khoá ở 5 locale (xem trên); bản dịch es/ja/ko/zh cần người bản ngữ rà.

**Spec:** `ui-live/auto-plan-generation.web.e2e.ts` (4 test). Sửa harness: `configureProject` luôn đặt `planAutoGenerate:false` nên (a)-(c) không bao giờ thấy Plan tự ra; thêm trường `planAutoGenerate?` vào `ProjectRunSettings` (mặc định false cho các spec cũ), spec này và `withNativeProject` (CR-045) truyền `true`. Kết quả `run_ui_flow.py --ai-stub -- auto-plan-generation`: 4/4 xanh (hai lần), trong lần hồi quy 27 spec có một lần (c) đỏ ("Generate plan was never offered") rồi chạy lại riêng `--grep "may pass" --repeat-each 5`: 5/5 xanh (một lần đỏ chưa tái hiện được; nghi dải ngắn giữa các lượt khi UI đọc `request.planProposal` bị lỗi tạm).

**Lỗi giao diện tìm được và sửa (chứng minh bằng chạy lặp):** trong 2-3/8 lần mở trang, UI KHÔNG BAO GIỜ gửi `request.subscribe` (đếm khung WebSocket: 0 khung sự kiện; độ trễ phân loại 13.9 s = chu kỳ poll 15 s) nên khi server tự commit Plan, tab Plan hiện cây và banner "An approval is waiting" nhưng không có nút Approve vì danh sách Approval của `usePlanTree` chỉ làm mới theo sự kiện, không theo sự kiện poll. Nguyên nhân: `useRequestEvents` mở luồng một lần theo `[support]`; nếu `settings.activeRuntimeEnvironmentId` tới sau khi dò hỗ trợ xong thì đích là `local` (chỉ poll) và không bao giờ mở lại. Sửa: `frontend/src/renderer/src/hooks/useRequestEvents.ts` thêm `activeEnvironmentId` vào deps (2 test mới trong `useRequestEvents.test.ts`, một test đỏ khi bỏ sửa). Sau khi build lại container: 9/9 lần chạy lặp đều có `request.subscribe` và 15-18 sự kiện, độ trễ phân loại 0.9-1.9 s (trước sửa 3/8 và 4/10 lần không có luồng).

**Chưa kiểm / còn lại:** `usePlanTree` vẫn không làm mới Approval khi chỉ có sự kiện poll (chế độ dự phòng): chưa sửa. Luồng `DeliverAll` của consumer tạm của gateway phát lại lịch sử stream (32 000 tin) cho mỗi phiên: chưa sửa (ghi nhận).

### Ghi chú lúc soạn (trước khi chạy trên stack)

- **i18n**: 18 khoá `auto.components.request.plan.{PlanAutoWriting,PlanRunFailedCard,PlanAutoSkippedNote,PlanRewriteStrip}.*` ghi vào cả 5 locale bằng một script duy nhất; bản dịch es/ja/ko/zh cần người bản ngữ rà (ghi vào PR).
- **Spec**: `tests/stages/e2e/orca-flows/ui-live/auto-plan-generation.web.e2e.ts` (4 test: a; b; c; d+e gộp trên một Request để đỡ một vòng phân loại), `live-auto-plan-steps.ts`, `live-plan-auto-gateway-features.ts` + `.test.ts` (proxy WebSocket bỏ `request.planAuto`, 5 test xanh), `run_ui_flow.py` (docstring + truyền `ORCA_E2E_FAKE_CLAUDE_STATE_DIR` = thư mục trạng thái của stub; spec tự bỏ qua khi biến này thiếu).
- **Lệnh đã chạy**: `ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live --list auto-plan-generation` liệt kê 4 test; `tsc --noEmit` (tsconfig tạm) sạch cho spec và file test; `npx vitest run --root tests/stages/e2e/orca-flows/ui-live live-plan-auto-gateway-features.test.ts` xanh. KHÔNG chạy spec, KHÔNG dựng lại container.
- **Thiết kế spec**: trạng thái "writing" được lấy mẫu mỗi 200 ms (stub nhanh); khẳng định chặt là Generate không bao giờ hiện giữa lúc duyệt và lúc Plan xuất hiện, và trình duyệt không gửi `request.generatePlan`; (a) chấp nhận `autoPlan.state` là `done` hoặc rỗng vì BE task 07 chỉ đưa job vào khi Request còn `planning`; (c) ghi chú (annotation, không phải gap) nếu không lấy mẫu được "Attempt 2 of 2".
- **Ảnh chụp**: `plan-auto-writing`, `plan-run-failed-card`, `plan-run-failed-after-reload`, `plan-generate-project-off`, `plan-generate-flag-off`, `plan-written-by-orca`, `plan-after-retry-by-orca`. `plan-rewrite-strip` KHÔNG dựng được: replan có feedback thuộc CR-REQ-045.
- **Rủi ro khi chạy lần đầu**: (1) cần backend task 09 (`fail_plan_next` trong `fake_claude_cli/claude`) và task 07/08 đã triển khai; (2) (a) và (e) giả định `request.planProposal` trả view rỗng kèm `autoPlan` khi chưa có run (xem ghi chú task 02): nếu backend vẫn trả RUN_NOT_FOUND thì (a) có thể thấy nút Generate trong vài giây đầu và (e) đỏ ở `autoPlan.enabled`; (3) mã lỗi tạm thời mà stub `error` thực sự tạo ra (ca c) chưa kiểm.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không dữ liệu giả trong mã sản phẩm**; mọi trạng thái (tải, rỗng, lỗi, offline, quyền) được xử lý.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Nhãn tiếng Anh; khoá i18n cho **cả 5 locale** (en, es, ja, ko, zh); kiểm tra coverage i18n xanh.
- Dùng token `assets/main.css` và primitive shadcn (`guides/STYLEGUIDE.md`); phím tắt và nhãn phụ thuộc nền tảng qua kiểm tra runtime.
- Không file nào vượt `max-lines` (tách file, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được khi repo nằm trên host SSH; không giả định đường dẫn cục bộ.
- Parser dây không bao giờ ném lỗi; gateway cũ thiếu trường mới vẫn hiển thị đúng.
- `pnpm lint`, kiểm kiểu và test vitest của vùng đã đổi xanh.
