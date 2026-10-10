# TASK-FE-E2E-008: Playwright smoke chạy trên stack thật cho luồng Request (có điều kiện)

**From Solution:** SOL-FE-E2E-003 · **Resolves:** BUG-E2E-001
**Priority:** P2 · **Area:** frontend
**Files:**
- tests/stages/e2e/orca-flows/ui-live/ (spec `request-live-flow.web.e2e.ts`, `live-flow-harness.ts`, `live-flow-steps.ts` và các module hỗ trợ)
- tests/stages/e2e/orca-flows/run_ui_flow.py (bọc `--ai-stub`, khôi phục agent)
- tests/stages/e2e/orca-flows/fake_claude_cli/claude (mã kích hoạt `[stub:blocking-question]`) và `test_fake_claude_cli.py`
- tests/configs/playwright.web.config.ts (project `request-live`)
- tests/stages/e2e/orca-flows/README.md (mục "Chạy qua giao diện thật")
**Depends on:** TASK-E2E-001, 026
**Status:** `[x]` DONE (2026-10-10): 3 test qua UI thật (`change_request`, `ask-back`, `question`) xanh với AI stub và với Claude thật; chưa phủ duyệt Plan để thực thi agent

---

## Context

Các spec `request-web` dùng `mock-request-ws.ts` nên không bắt được lỗi hợp đồng thật. Task mở rộng từ "smoke Clarification" thành luồng đầy đủ Request → xác nhận loại → Clarification → Solution (kể cả Hỏi lại) → Plan → Approval, đi qua trình duyệt trên frontend thật. Không có đường lùi qua gateway: bước nào UI không làm được thì test đỏ.

## Việc cần làm

- [x] Đăng nhập bằng form, tạo Request qua hộp thoại, đợi phân loại, xác nhận loại, trả lời Clarification trên UI (đối chiếu trạng thái với gateway ở mỗi bước).
- [x] Solution tự sinh sau Clarification (CR-REQ-028 2.4); sau 20 giây chưa có run thì ghi gap `SOLUTION-NOT-AUTO-STARTED` và bấm "Generate solution". Chọn option và duyệt trên UI.
- [x] Nhánh câu hỏi chặn như người dùng thật: thấy `solution-blocking-question` (hoặc duyệt bị từ chối, `solution-blocking-notice[data-refused]`) → `solution-ask-back` → dialog (`solution-ask-back-reason`, `solution-ask-back-submit`) → trả lời Clarification trên panel → đợi Solution mới (id mới, cũ `superseded`) → chọn lại và duyệt; tối đa 2 vòng; log `ask-back rounds: N`.
- [x] `claude` giả xác định: Request có `[stub:blocking-question]` thì Solution đầu có 1 câu chặn, Solution sinh lại (prompt có `<clarifications>` sau `</request>`) không còn; test thứ ba `ask-back` (chỉ stub). `test_fake_claude_cli.py` 16/16 và `verify-against-request-service.sh` xanh.
- [x] "Generate plan" trên UI → `awaiting_plan_approval`; tab Plan hiển thị Plan; Approval `Plan` trong hộp thư; Reject qua UI; "Cancel request" qua UI. Không duyệt Plan.
- [~] Nhánh "Generate solution" cho Request không có Clarification: không khả thi (mọi loại có phân tích đều có trường bắt buộc). Nút được bấm trong nhánh gap khi Solution không tự bắt đầu; chưa có ca xanh nào chạm nút này.
- [x] Chỉ chạy khi có `ORCA_E2E_LIVE=1` (project riêng `request-live`, không bật Vite); thiếu biến thì `skipped`.
- [x] AI stub (`run_ui_flow.py --ai-stub`), khôi phục agent Claude thật sau khi chạy: kiểm cuối `dev-local-mac` connected, không còn HOME của stub.
- [x] Dữ liệu tiền tố `e2e-ui-<mmddhhmmss>`, tự dọn; sau mọi lượt: 0 project `e2e-ui-*`, 0 Request mồ côi.
- [x] Kiểm gateway: `RestartCount` trước/sau (cần `docker`), gap `GATEWAY-RESTARTED`; ghi các RPC UI bị từ chối; `decision.*` chưa có ở gateway là nhiễu đã biết (allowlist FE-E2E-001), chỉ đếm, không làm đỏ.
- [-] Duyệt Plan để thực thi agent (`ORCA_E2E_EXECUTE_AGENT=1`): chưa tự động hoá; duyệt sẽ chạy agent AI trên máy dev.

## Tiêu chí hoàn thành

- [x] Xanh trên stack local: AI stub 3 test x3 lượt: 8/9 lượt-test xanh (lượt đỏ do lỗi UI không liên tục bên dưới); Claude thật `change_request` x3: 3/3 xanh.
- [x] Bị bỏ qua khi thiếu `ORCA_E2E_LIVE=1`.

## Kiểm thử

```bash
python3 tests/stages/e2e/orca-flows/run_ui_flow.py --ai-stub                      # 3 test, AI giả; khôi phục agent thật
python3 tests/stages/e2e/orca-flows/run_ui_flow.py -- --grep change_request       # Claude thật của dev server
ORCA_E2E_LIVE=1 npx playwright test -c tests/configs/playwright.web.config.ts --project=request-live
```

## Kết quả thực thi

Ngày 2026-10-10, sau khi người điều phối redeploy api-gateway, request-service, task-service và frontend (nút Hỏi lại, câu hỏi chặn, `RequestClarification` chuyển khỏi nhóm nội bộ).

**AI stub, `run_ui_flow.py --ai-stub` (3 test), 3 lượt cuối trên bản spec cuối:** lượt 1 và 3: 3/3 xanh (1.5 đến 1.6 phút); lượt 2: `change_request` và `question` xanh, `ask-back` đỏ vì gap `UI-STALE-SOLUTION` (xem lỗi 1). `change_request` 32 đến 33 giây, `ask-back` 32 giây (1 vòng Hỏi lại), `question` 25 đến 28 giây. Tổng 8/9 lượt-test xanh. Trước khi sửa mã phát hiện khối `<clarifications>` của stub (đặt sau `</request>`, không phải trước), `ask-back` đỏ 3/3 vì Solution sinh lại vẫn có câu chặn: đó là lỗi của stub, đã sửa.

**Claude thật, `change_request` x3:** 3/3 xanh (3.5, 3.3 và 5.0 phút); lượt 3 Claude hỏi câu chặn nên đi qua **1 vòng Hỏi lại thật** (log `ask-back rounds: 1`) rồi duyệt Solution mới và hoàn tất. Hai lượt kia 0 vòng. 0 gap.

**Gateway:** `RestartCount` 0 trước/sau mọi lượt, 0 dòng `panic` kể từ lúc redeploy. Độ trễ UI so với service: phân loại 0.9 đến 2.9 giây, sau Submit 5 mili giây đến 12 giây, Solution sẵn sàng 5 mili giây đến 1.9 giây.

**Lỗi tìm thấy và đã sửa trong đợt này (bằng chứng chạy lại):** consumer tự sinh Solution; tab Plan; panic gateway; `REQUEST_ANALYSIS_NO_REPO_PATH` (test `question` xanh); `REQUEST_SOLUTION_BLOCKING_QUESTIONS` hiển thị đúng và có nút Hỏi lại; `clarification.request` bị `INTERNAL_CALLER_REQUIRED` (RPC nằm nhầm nhóm `GroupInternal`, phát hiện nhờ test `ask-back`, đã sửa).

**Lỗi sản phẩm còn lại:**
1. **FE, không liên tục (gặp ở ít nhất 4 trong khoảng 12 lượt ask-back của đợt này):** sau khi trả lời Clarification của Hỏi lại, service đã có Solution mới (Solution cũ `superseded`, request `awaiting_analysis_approval`) nhưng màn hình tiếp tục hiện Solution cũ cùng câu hỏi chặn cũ trong hơn 45 giây (gap `UI-STALE-SOLUTION`); bấm duyệt trên màn hình cũ nhận `solution.choose -> REQUEST_SOLUTION_NOT_PROPOSED: solution <id> is not proposed`. Chỉ khỏi sau khi tải lại trang. Nghi: tab Analysis chỉ đọc lại Solution mỗi 5 giây khi Request `analyzing`; với stub Solution mới đến trước lần đọc kế tiếp nên màn hình không bao giờ thấy trạng thái `analyzing`. Cần đọc lại danh sách Solution khi `updatedAt`/trạng thái Request đổi hoặc khi Solution hiện tại thành `superseded`.
2. Giới hạn đã biết: `claude` giả chỉ biết prompt phân loại, Solution, Plan và phân tích chỉ đọc đã được mở rộng; không mô phỏng chất lượng AI.

**Chưa kiểm:** duyệt Plan và thực thi agent (`ORCA_E2E_EXECUTE_AGENT=1` chưa tự động); ca "Generate solution" xanh (cần Request analyzing không có run mà không qua Clarification); chạy trên CI; vai không phải admin; vòng Hỏi lại thứ hai với AI thật (chưa gặp).

### Kết quả cuối (người điều phối, 2026-10-10, sau khi sửa `clarification.request`, câu hỏi chặn và Solution cũ)

| Chạy | Kết quả |
|---|---|
| `run_ui_flow.py --ai-stub` (3 test) | **3/3 xanh** (1.5 phút), 0 gap |
| `run_ui_flow.py --ai-stub -- --grep ask-back` x6 liên tiếp | **6/6 xanh**, 0 gap (trước khi sửa `UI-STALE-SOLUTION`: khoảng 1/3 lượt đỏ) |
| `run_ui_flow.py -- --grep change_request` (Claude thật) | **xanh** (5.1 phút), đi qua 1 vòng Hỏi lại thật do Claude trả câu hỏi chặn; trước đó 3/3 xanh ở vòng 3 |
| Gateway | `RestartCount=0` suốt các lượt, 0 panic từ khi redeploy |
| Dữ liệu | 0 project `e2e-ui-*`, 0 Request mồ côi; agent dev-server là Claude thật sau khi chạy |

Tab Plan hiển thị Plan thật ở mọi lượt (khẳng định mềm "Plan not found" không còn kích hoạt), nên `PLAN-TAB-EMPTY` đã đóng trên stack thật. Còn lại: `console errors` của trình duyệt (7 với stub, 16 với Claude thật, 0 lỗi trang, 0 lệnh gọi thất bại) chưa được phân loại; và nhiễu `decision.list` (kênh chưa có ở gateway, đã nằm trong allowlist của FE-E2E-001).
