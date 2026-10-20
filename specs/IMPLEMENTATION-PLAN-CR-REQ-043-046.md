# Kế hoạch triển khai CR-REQ-043, 044, 045, 046

**Ngày soạn:** 2026-10-20 · **Nhánh nền:** `feat/e2e-acceptance-fixes` · **Trạng thái:** 📝 chưa bắt đầu (0/49 task)
**Nguồn:** phản hồi của người dùng ngày 2026-10-20 (Bug 2 đến Bug 5; Bug 1 do người dùng tự xác nhận là nhầm lẫn, không phải lỗi). Kế hoạch này theo cùng khung của [IMPLEMENTATION-PLAN-CR-REQ-039-042.md](./IMPLEMENTATION-PLAN-CR-REQ-039-042.md).

## 1. Bốn CR và quyết định đã chốt

| CR | Việc | Quyết định của người dùng | BE | FE |
|---|---|---|---|---|
| [043](../docs/crs/v6/plan-phase-task/CR-REQ-043-approve-plan-and-run-all-steps.md) | Duyệt Plan rồi chạy hết các bước một lần | Phương án (b): chọn `continuous` ngay lúc duyệt Plan; thêm công tắc chế độ thực thi theo dự án | 6 | 6 |
| [044](../docs/crs/v6/plan-phase-task/CR-REQ-044-automatic-plan-generation-after-analysis-approval.md) | Tự sinh Plan khi duyệt Analysis, tự commit, tự thử lại có giới hạn, thẻ Retry | **Bật mặc định cho tất cả** (env `REQUEST_PLAN_AUTO_GENERATE=true`, công tắc dự án mặc định bật, tắt được); chi phí Plan OpenSpec 7–13 USD được chấp nhận | 9 | 4 |
| [045](../docs/crs/v6/plan-phase-task/CR-REQ-045-plan-documents-viewer-and-revision-by-prompt.md) | Xem tài liệu OpenSpec của Plan, chỉnh bằng prompt tại cổng Plan | Chỉ xem và chỉnh bằng prompt; sửa tay ngoài phạm vi | 9 | 8 |
| [046](../docs/crs/v6/ai-governance/CR-REQ-046-ai-output-language-system-setting-default-vietnamese.md) | Ngôn ngữ đầu ra của AI | **Cấu hình hệ thống**, `REQUEST_DEFAULT_OUTPUT_LANGUAGE`, **mặc định `vi`**; không có ghi đè theo dự án hay Request | 7 | 0 |

Tổng 49 task. Solution: `specs/backend-go/crs/v6/{plan-phase-task,ai-governance}/solutions/BE-REQ-SOL-043..046-*.md` và `specs/frontend/crs/v6/request-frontend/solutions/FE-REQ-SOL-043..045-*.md`; task cùng cấp trong `tasks/`.

## 2. Phụ thuộc và thứ tự

```
CR-046 (prompt + cấu hình)      ─────────────┐ (độc lập, đụng template plan: làm trước 045-06)
CR-043 (duyệt + ghim continuous) ────────────┤ (độc lập)
CR-044 (job + worker + commit server) ─▶ CR-045 (dùng lại CommitPlan.ExecuteForSystem, worker, usePlanRunState, PlanRunFailedCard)
```

| Đợt | Nội dung | Có thể song song |
|---|---|---|
| A | 046-01..04, 06 (gói ngôn ngữ, prompt, golden, cắt UTF-8, cấu hình) · 043-01..04 (miền, ghim, precheck, gateway) | Hai nhánh không chung file (prompt/golden so với approval) |
| B | 044-01..09 (miền, migration `0101`, đường hệ thống của GeneratePlan/CommitPlan, worker, retry, gateway, e2e) | Sau A vì chung stub fake claude và `RequestPlanTab` |
| C | 045-01..09 (AuthorizeRevise + OPA, redact bundle, bảng lịch sử `0102..0104`, prompt sửa-tại-chỗ, activity, e2e) | Sau 044-03 và 044-06 |
| D | FE 043-01..06, 044-01..04, 045-01..08 (chung `RequestPlanTab.tsx`, `PlanApprovalBar`, `PlanApprovalCard`: làm tuần tự theo 043, 044, 045) | Mỗi CR một người/phiên |
| E | 046-05, 046-07, 043-05/06 BE, hồi quy cuối, chạy `--real` | Cuối |

## 3. Điểm chồng lấn cần một chủ file (từ báo cáo bốn agent)

| Tài nguyên | CR đụng | Quy tắc |
|---|---|---|
| `tests/stages/e2e/orca-flows/fake_claude_cli/claude` | 043-06, 044-09, 045-09, 046-07 | Mỗi lần một task; mỗi task thêm mã/nhánh riêng, không sửa nhánh cũ |
| Template plan (`prompts/plan_native`, `plan_openspec`) + golden + `prompt_size_budget_test.go` | 045-06, 046-01/03 | Cả hai là khối điều kiện, golden hiện tại phải giữ nguyên byte; thứ tự: 046 trước, 045 sau; golden chỉ đổi bằng regenerate có xem xét |
| `RequestPlanTab.tsx` (249 dòng, trần 300), `PlanApprovalBar`, `PlanApprovalCard` | FE 043, 044, 045 | Tách file theo từng CR, không thêm `max-lines` disable; làm 043, rồi 044, rồi 045 |
| Số migration `request-service` (hiện `0100`) | 044 (`0101`), 045 (`0102..0104`) | Kiểm lại số khi triển khai, cả Postgres và MySQL |
| Số trường proto (`GetPlanProposalResponse` 13, `ProjectEngineSettings` 7, `ApproveRequest`) | 043, 044, 045 | Cấp số tại thời điểm làm task proto, ghi vào file task |
| `request.flowStatus.features` và CONTRACT | 043 (`approval.runAll`), 044, 045 | Cập nhật CONTRACT trong task gateway của từng CR (043-04, 044-08, 045-08) |
| Cổng OPA `request.rego` | 045-01 (`member_plan_steering_rpcs`) | Mẫu từ `member_steering_rpcs` đã có của CR-041; kiểm bằng bộ test rego |
| `reporterIdentity` (`analysis_runner.go`, loại trừ plan run) | 044, 045-02 | Xác định lý do loại trừ trước khi bỏ |

## 4. Chiến lược kiểm thử và chi phí

- Hạng A (fake claude, stub): phần lớn ca; chạy `run_acceptance.py --ai-stub …` với các suite mới (`APG-01..10`, `ARA-xx`, `OLG-01..06`, các ca của 045) và các spec UI mới trong `ui-live`.
- Hạng B (Claude thật, repo tạm, đã được cho phép trước): `--real` cho `native-execute`, một ca analysis tiếng Việt (046-07), một ca plan native.
- Hạng C (OpenSpec thật, 7–13 USD mỗi lần): chỉ chạy khi có đồng ý rõ ràng. Fake claude không tự soạn được OpenSpec, nên ca xem tài liệu của 045 dùng dữ liệu gieo qua `docker exec` hoặc một tác giả OpenSpec giả (045-09, giới hạn nửa ngày).
- Mọi thay đổi lưu trữ có test hợp đồng Postgres 16 và MySQL 8 (colima, `-tags=integration -p 1`).
- Golden prompt: `make check-prompts` phải xanh; golden chỉ đổi bằng regenerate có xem xét.
- Sau mỗi đợt: triển khai lại dịch vụ đã đổi (thứ tự `infra-fleet → git-gateway → task → request → api-gateway`, `api-gateway` build bằng overlay vì `StepApproval` của luồng khác), build lại container frontend rồi chạy lại hồi quy `platform devserver request matrix task aistub request_base_ref analysis_revision` và các spec UI.

## 5. Rủi ro lớn

1. **Chi phí**: bật tự sinh Plan mặc định (044) tốn 7–13 USD mỗi Request OpenSpec. Chỉ kích hoạt ở `analysis_approved` (và `type_confirmed` cho task/docs/ops); không bao giờ ở revise/reject/replan. Công tắc env và dự án để tắt.
2. **Commit phía server (044)** là điều kiện của 045; nếu trì hoãn thì 045 không có chỗ dựa.
3. **Thay đổi ngôn ngữ (046)** có thể làm model cắt đầu ra hoặc bỏ qua chỉ thị trước nội dung dài (chưa đo); chỉ có `--real` mới đo được. Khoá JSON, enum, ID, ngữ pháp `tasks.md` và `ORCA_RESULT_*` phải giữ tiếng Anh; `openspec validate --strict` đã thử: bắt buộc `SHALL/MUST` và tiêu đề delta tiếng Anh.
4. **Request đang chạy `step_by_step` (ví dụ #1771)** không đổi được chế độ (ghim vĩnh viễn); CR-043 để việc "chuyển Request đang chạy" sang CR riêng.
5. **Quyền**: approver chỉ chỉnh được nếu là thành viên dự án (đọc được repo); người ngoài dự án bị cổng OPA chặn.

## 6. Câu hỏi còn mở (4 câu đã chốt 2026-10-20 theo đề xuất; các câu chưa hỏi lấy theo mặc định)

| CR | Câu hỏi | Mặc định đề xuất |
|---|---|---|
| 043 | Request đã ghim `step_by_step` có cần CR "đổi chế độ cho Request đang chạy" ngay không | ✅ Chốt: không, CR riêng sau |
| 043 | Lựa chọn "chạy hết" ở Approval Inbox | Không |
| 043 | Chủ dự án (không phải admin) có được đổi công tắc chế độ | ✅ Chốt: không (chỉ admin hệ thống) |
| 044 | Giao diện cài đặt cho công tắc tự sinh Plan theo dự án | ✅ Chốt: không làm trong CR này; đổi qua kênh admin hoặc CLI |
| 044 | Coi `REQUEST_PLAN_AI_UNAVAILABLE` ở cấp run là lỗi tạm | ✅ Chốt: có |
| 045 | "Keep the previous version" sau khi lượt chỉnh thất bại; lượt chỉnh không đổi gì là lỗi hay tạo bản trùng | Không làm; lỗi `NO_CHANGE` |
| 046 | Mở rộng chỉ thị sang kết quả task, commit message, PR; ghi ngôn ngữ lên run; dòng nhắc cuối prompt; ngôn ngữ thứ ba | Sau khi xem kết quả `--real`; không thêm ngôn ngữ |

## 7. Định nghĩa "xong"

Giống kế hoạch 039–042: mọi ô "Tiêu chí hoàn thành" được tick kèm bằng chứng trong "Ghi chú triển khai" (tên test, lệnh, kết quả); không tick ô chưa chạy; trạng thái đổi `[ ] TODO` → `[x] DONE (ngày)`; cập nhật CR, các README chỉ mục và bảng dưới đây khi xong.

## 8. Theo dõi tiến độ

| CR | Task | Trạng thái |
|---|---|---|
| 043 BE | 01 … 06 | [ ] [ ] [ ] [ ] [ ] [ ] |
| 043 FE | 01 … 06 | [ ] [ ] [ ] [ ] [ ] [ ] |
| 044 BE | 01 … 09 | [ ] ×9 |
| 044 FE | 01 … 04 | [ ] [ ] [ ] [ ] |
| 045 BE | 01 … 09 | [ ] ×9 |
| 045 FE | 01 … 08 | [ ] ×8 |
| 046 BE | 01 … 07 | [ ] ×7 |
