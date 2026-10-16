# FE-REQ-TASK-037-09: i18n năm locale, truy cập, e2e và cập nhật tài liệu

**From Solution:** [FE-REQ-SOL-037](../solutions/FE-REQ-SOL-037-request-detail-redesign-execution-tab-and-activity.md) mục 2.5 và 5
**Priority:** P1
**Area:** frontend / i18n / e2e
**File:** `frontend/src/renderer/src/i18n/locales/{en,es,ja,ko,zh}.json`, `i18n/request-locale-coverage.test.ts` (sửa), `tests/e2e/request-web/request-detail-redesign.web.e2e.ts` (mới; thư mục `tests/e2e/request-web` hiện chưa có, hiện chỉ có `workflow-web`, kiểm lại và theo mẫu `tests/playwright.workflow-web.config.ts`), `docs/crs/v6/request-frontend/CR-REQ-037-*.md`, `specs/frontend/crs/v6/README.md` (cập nhật trạng thái)
**Depends on:** FE-REQ-TASK-037-01 đến 037-08
**Status:** [~] PARTIAL

---

## Context

- Khoá i18n: `auto.components.request.*`; bộ locale `en, es, ja, ko, zh`; test phủ khoá `request-locale-coverage.test.ts`.
- Bản dịch es, ja, ko, zh cần người bản ngữ rà (README frontend mục 3, điểm 15).
- E2E web chạy trên stack thật (`WORKFLOW_E2E_BASE_URL`-kiểu); runtime thật cần đăng nhập thủ công, không đưa mật khẩu vào mã.

## Việc cần làm

1. Rà mọi khoá mới của task 01 đến 08, đảm bảo đủ 5 locale và không có chuỗi cứng; cập nhật `request-locale-coverage.test.ts` nếu danh sách khoá theo tiền tố.
2. Kiểm truy cập: thang tiến độ có `sr-only` trạng thái và `aria-current`; dải việc tiếp theo `aria-live="polite"`; chấm trên tab có `aria-label`; mọi trạng thái có icon và chữ (không chỉ màu).
3. Kiểm trực quan sáng và tối: "đang chạy" và "chờ người" cùng tông `primary` có phân biệt được bằng icon và chữ; ghi ảnh chụp vào PR.
4. E2E web (cần BE-SOL-038 chạy được trên stack): ca 1 `change_request` duyệt Plan một lần rồi theo dõi Execution tới hết; ca 2 task lỗi hai lần về backlog, thẻ lỗi và "Reopen and rerun"; ca 3 `task` native không có Handover; ca 4 `question` không có Plan/Execution; ca 5 History có dòng AI phân loại, về backlog, task lỗi. Ca nào cần agent thật tốn tiền thì gắn cờ môi trường để không chạy mặc định.
5. Cập nhật trạng thái trong `CR-REQ-037`, `FE-REQ-SOL-037`, `tasks/README.md`, `solutions/README.md` và README frontend v6.

## Kiểm thử

- `pnpm --filter orca-frontend test -- src/renderer/src/i18n/request-locale-coverage` (chưa chạy).
- E2E: hiện chỉ có `tests/playwright.workflow-web.config.ts` (cho `tests/e2e/workflow-web`); chưa có cấu hình Playwright cho Request. Tạo cấu hình tương tự (đổi `testDir` thành `./e2e/request-web`) hoặc mở rộng cấu hình hiện có, rồi chạy `npx playwright test -c <cấu hình> tests/e2e/request-web/request-detail-redesign.web.e2e.ts` (chưa kiểm chứng script và Chromium có sẵn).

## Tiêu chí hoàn thành

- [x] Test phủ khoá i18n xanh cho cả 5 locale.
- [ ] Danh sách kiểm truy cập đạt; ảnh chụp sáng và tối trong PR.
- [x] E2E web 5 ca chạy hoặc ghi rõ lý do chưa chạy (ca 1 chạy thật PASS; ca 2 đến 5 xem trên dữ liệu thật; lý do chưa chạy sống ghi ở dưới).
- [x] Trạng thái các CR và README được cập nhật.

## Rủi ro và lưu ý

- Không có Chromium hoặc script Playwright trong môi trường (README frontend mục 3, điểm 10) thì e2e chỉ là kế hoạch; ghi rõ.
- Bản dịch máy chưa được người bản ngữ rà; ghi vào PR.

## Ghi chú triển khai

Một phần, 2026-10-16.

**Đã làm**
- Khoá i18n: toàn bộ khoá mới của 01–08 (192 khoá, rút bằng quét `translate()` rồi thêm vào cả năm locale; es/ja/ko/zh do tôi dịch, **cần người bản ngữ rà**). `request-locale-coverage`, `plan-locale-coverage` và các test phủ khoá khác xanh.
- Truy cập: thang tiến độ có chữ `sr-only` theo trạng thái và `aria-current="step"`; dải việc tiếp theo `aria-live="polite"` (vùng live luôn gắn sẵn khi rỗng); chấm trên tab có `role="img"` + `aria-label`; mọi trạng thái có icon và chữ (đã xem cả sáng lẫn tối trên #1242: "đang chạy" và "chờ bạn" cùng tông primary phân biệt được bằng icon và chữ).
- E2E thật qua giao diện, tuân theo cấu hình có sẵn `playwright.web.config.ts --project=request-live` (không tạo `tests/e2e/request-web`, vì bộ ui-live đã có harness an toàn): spec mới `ui-live/continuous-execute-real.web.e2e.ts` + `live-continuous-execution-steps.ts`, chạy bằng `python3 tests/stages/e2e/orca-flows/run_ui_flow.py --real continuous`. **Kết quả: PASS (177 giây)**: dự án đặt `continuous`, tạo `change_request` qua giao diện, phân loại, Clarification, Solution, Plan 2 Phase; tab Plan có đúng một thẻ duyệt, không có "Approve phase/Start phase/thanh duyệt Phase"; duyệt một lần; `executing` ghim `continuous`; không có Approval `phase` nào mở; tab Execution hiện thanh tóm tắt "0/2 → 1/2 → 2/2 tasks · step 2/2" khi chạy; Request `completed`; Execution đọc Done, History có dòng Phase tự bắt đầu, tab Plan có dải "approved … Follow in Execution"; repo fixture có thay đổi và test của nó qua; guard an toàn 0 vi phạm.
- Các ca còn lại của spec xem trực tiếp trên dữ liệu thật của stack: ca 2 (task lỗi hai lần, về backlog, thẻ lỗi, nút "Reopen and rerun") trên #1242; ca 3 (native không Handover) trên #1252; ca 4 (question không có Plan/Execution) trên #1223 (tab Overview, Analysis, History, Related); ca 5 (History) trên #1242 ("Returned to backlog", "… failed — Attempt 2 …", "Phase started").
- Sửa ba lỗi tìm thấy khi nghiệm thu: `request-service` không khởi động được (`DecideApproval.StartPrecheck is nil` do kiểm tra dây nối chạy trước `wireExecution`); History ghi "Attempt 3" trong khi tab Execution ghi "Failed 2 of 2" (số lần thử bây giờ chỉ đếm lần chạy dùng ngân sách thử lại, lỗi dispatch không tính); Request đã huỷ không rõ chặng bị đánh dấu "Classification stopped".

**Chưa làm / không kiểm chứng được**
- E2E **OpenSpec** (ưu tiên số một của yêu cầu): bản triển khai của dự án có `REQUEST_DEFAULT_ENGINE=openspec` và Solution do agent viết cần knowledge hub tdd-agent (`.tdd-agent/config.toml` trong repo). Repo fixture không có, nên bước tác giả OpenSpec thất bại (`KNOWLEDGE_CONFIG_MISSING`) và Request ở `planning`; e2e đã chạy với engine `native`. Run OpenSpec sống cần repo pilot (`vnp-qa-platform`), tốn $7–13 và tạo worktree thật; chưa chạy khi chưa có xác nhận.
- Ca "task lỗi hai lần" chưa tạo ra được bằng một run sống mới (agent thật không lỗi theo yêu cầu); "Reopen and rerun" chưa bấm trên #1242.
- Bộ `--ai-stub` hiện không chạy được trên stack này vì cấu hình `.env` bật Solution do agent (`KNOWLEDGE_HUB_UNSUPPORTED` với `claude` giả), không liên quan đến thay đổi này.
- Ảnh chụp sáng/tối đã xem nhưng chưa đưa vào PR (chưa có PR).
