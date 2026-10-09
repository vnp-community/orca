# TASK-E2E-026: Nối stub AI (CLI giả và HTTP) vào CI và mở khoá test BLOCKED

**From Solution:** SOL-E2E-005 · **Resolves:** BUG-E2E-010
**Priority:** P1 · **Service/Area:** tests, ci
**Files:**
- tests/stages/e2e/orca-flows/suite_request_ai_stages.py
- tests/stages/e2e/orca-flows/suite_request_lifecycle.py
- tests/stages/e2e/orca-flows/suite_request_plan_stage.py, ai_artifact_contracts.py (mới), suite_ai_stub_login.py (mới)
- backend-go/services/request-service/e2e/stubs (dùng lại)
**Depends on:** TASK-E2E-033, 035
**Status:** `[x]` XONG (cập nhật 2026-10-09); lỗi sản phẩm AST-05 phát hiện và đã sửa, chạy lại xanh

---

## Context

`RQL-09`, `AIS-02`, `AIS-04` luôn BLOCKED nếu không có AI thật; không thể là cổng chặn.

## Việc cần làm

- [x] Dùng stub của TASK-E2E-035 để chạy các test đó. — _`--ai-stub`: `RQL-09`, `AIS-02`, `AIS-04`, `PLN-01..08`, `MTX-*`, `EXT-*` chạy với `claude` giả; thêm `AIS-01/03` (pha 2) và `AST-01..05`. Chạy Claude thật vẫn được, không đổi_
- [x] Kiểm tra nội dung phản hồi theo hợp đồng (enum loại, size, urgency; Solution có kind đúng). — _`RQL-09`: loại thuộc 11 loại, size S/M/L, urgency normal/urgent (cả stub lẫn AI thật), và với stub lý do phải chứa `fake-claude`. `ai_artifact_contracts.py`: Solution `kind=solution`, trạng thái `draft/proposed`, 2..4 option `opt-N`, đúng một option khuyến nghị và được `recommendation.option_id` nêu tên, `effort.size` S/M/L; Plan: container đúng shape (task list không Phase, `change_request` có Phase và không task rời), loại task hợp lệ, chỉ số phụ thuộc hợp lệ. Dùng ở `AIS-02`, `AIS-04`, `PLN-01`, `PLN-08`_
- [x] Báo cáo ghi rõ test chạy bằng stub, không đánh giá chất lượng AI. — _"AI mode: STUB" trong `last-run.md`, `ai.mode=stub` trong `last-run.json`, dòng cuối lần chạy_

## Tiêu chí hoàn thành

- [x] `python3 run_acceptance.py --strict --ai-stub` cho 0 BLOCKED trừ MCP và tuỳ chọn. — _đạt: BLOCKED chỉ còn `MCP-01/02` (`MCP_ENABLED` tắt, truyền `--allow-blocked MCP-01,MCP-02`); `TSK-08` là SKIP tuỳ chọn. Mã thoát vẫn 1 vì `AST-05` FAIL (lỗi sản phẩm, xem TASK-035)_

## Kiểm thử

Chạy thật.

## Kết quả thực thi

**Làm xong** (2026-10-09, chưa commit).

| Lệnh | Kết quả |
|---|---|
| `python3 run_acceptance.py --strict --ai-stub --allow-blocked MCP-01,MCP-02` (stack local) | **109 PASS, 1 FAIL (AST-05), 2 BLOCKED (MCP-01/02), 1 SKIP (TSK-08)**, 88 giây. `AIS-01/03` hết BLOCKED nhờ pha 2 |
| `python3 run_acceptance.py request ai plan` **không** `--ai-stub` (Claude thật qua phiên đăng nhập, sau các thay đổi test) | 37 PASS, 0 FAIL, 3 BLOCKED (`AIS-01/03` vì có AI; `PLN-08`, xem dưới), 223 giây |

Thay đổi test ngoài việc nối stub (đều do chạy thật mà ra):

- `AIS-03` sai từ trước: giả định `request.generatePlan` báo lỗi đồng bộ khi không có AI, nhưng từ khi Plan chạy nền nó trả `runId` và lỗi nằm ở run. Test chưa bao giờ chạy được vì luôn BLOCKED khi có AI. Đã sửa theo hợp đồng thật: run `failed` + `errorCode`, không có proposal, Request vẫn `planning`.
- `PLN-08` thành BLOCKED (có lý do) khi Solution do AI sinh có câu hỏi mở `blocking: true`: phải trả lời trước khi duyệt, ngoài phạm vi test. Claude thật gặp tình huống này ở hai lần chạy cuối, nên `PLN-08` hiện **không còn được phủ** với AI thật; stub không bao giờ sinh câu hỏi chặn. Nên viết bước trả lời câu hỏi chặn qua `clarification.*` để phủ lại.
- Tên suite mới `aistub` chỉ chạy khi có `--ai-stub`; không đổi hành vi chạy mặc định.

Chưa làm: kiểm nội dung cho các loại Solution khác `solution` (chẩn đoán, findings, answer); stub chưa phủ chúng.

### Chạy lại sau khi sửa AST-05 (người điều phối, stack local đã dựng lại toàn bộ service)

| Lệnh | Kết quả |
|---|---|
| `python3 run_acceptance.py --strict --ai-stub --allow-blocked MCP-01,MCP-02` | **110 PASS, 0 FAIL, 2 BLOCKED (MCP-01/02), 1 SKIP (TSK-08)**, 63 giây, báo cáo ghi `AI: STUB, wiring only` |
| `python3 run_acceptance.py` (Claude thật qua phiên CLI) | **103 PASS, 0 FAIL, 4 BLOCKED (MCP-01/02, AIS-01/03), 1 SKIP**, 409 giây; `PLN-08` PASS |

Sửa `AST-05`: `ProposeRequestClassification` không còn huỷ giao dịch khi `RequestTypeApproval` trả `REQUEST_APPROVAL_NO_ELIGIBLE_APPROVER`; Request vẫn sang `awaiting_type_confirmation` (test `TestPropose_NoEligibleApprover_DoesNotStrandTheRequest`, lỗi khác vẫn huỷ giao dịch).
