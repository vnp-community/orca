# Phase 7 — Phủ flow, so sánh theo flow, kết quả agent bền hơn, trạng thái suy giảm (CR-OR-041, CR-OR-042)

**Solution:** BE-OR-SOL-005 §3–§6 · **Phụ thuộc:** Phase 6 (`FlowRef`, cơ sở thiết kế) · **Service:** `request-service`, `api-gateway` · **Trạng thái:** 📝 chưa bắt đầu (2026-10-21)

Đã làm trước pha này (không làm lại): log lý do từ chối `3d8e998a8`; prompt `expected` boolean và nút code-only `55dc58d5c`; chặn Approve khi run context lỗi `c2e5052ad`; thông báo tab sau code trống `64c258d29`.

## Spike

- [ ] **BE-OR-T701a** KG có truy vấn nối FR/AC → flow/container không? Thử `kg_neighbors`, `kg_resolve_scope` với `FR-NTF-03`, `FR-NTF-05` trên `snap-000005` của `vnp-qa-platform`.
  Files: kết luận trong SOL-005 §8. Verify: có/không; nếu không, R1(b) của CR-OR-041 bị hoãn và tài liệu ghi rõ.

## Flow ứng viên và hợp đồng (CR-OR-041 R1–R3)

- [ ] **BE-OR-T701** `FlowCandidates(design, touched, requirements)` với lý do `code_touches` và `requirement_link`.
  Files: `domain/outcome_review_flow_candidates.go` (mới). Verify: SAD mẫu + Request chạm `ms-integration` ⇒ ứng viên gồm flow có tin nhắn tới `ms-integration`; flow không chạm ⇒ không có; bảng ca rỗng/một/nhiều.
- [ ] **BE-OR-T702** Hợp đồng `outcome-review-context/2`: `flows[]` (`flowRef`, `status`, `reason`, `evidence`) và `diagrams[].flowRef`; đọc được `/1`.
  Files: `domain/outcome_review_context.go`, `schemas/outcome-review/outcome-review-context-2.schema.json`, `prompts/outcome_review_context/_contract.tmpl`, `context.tmpl` (đưa `<candidate_flows>` vào lời nhắc), nâng `ReviewContextPromptVersion`. Verify: schema hợp lệ với mẫu; golden prompt cập nhật; khối `/1` cũ vẫn parse.
- [ ] **BE-OR-T703** Kiểm: mỗi flow ứng viên có đúng một câu trả lời; `drawn` cần hình cùng `flowRef`; `unchanged|not_applicable|undrawable` cần `reason` và ≥ 1 bằng chứng; flow ngoài ứng viên hợp lệ khi hình `source=code`.
  Files: `domain/outcome_review_context_merge.go`, `domain/outcome_review_diagram_model.go`. Verify: thiếu câu trả lời ⇒ vi phạm fatal; flow mới `source=code` ⇒ nhận, không `deviation`.
- [ ] **BE-OR-T704** `DiagramModel.FlowRef` (vào `InputDigest`) và ghép Design | After code theo `flowRef`.
  Files: `domain/outcome_review_diagram_model.go`, `domain/outcome_review_model.go`. Verify: digest đổi khi `flowRef` đổi; ghép đúng khi tên flow khác nhau ("Flow 8.1" ↔ "Bug resolved…").

## So sánh theo flow (CR-OR-041 R5–R7)

- [ ] **BE-OR-T705** `DesignFlow.HasCall(from,to)` chỉ trong flow cùng `flowRef`; `buildDiagram` đặt `InDesign=nil` khi không có flow thiết kế tương ứng.
  Files: `domain/outcome_review_design.go`, `domain/outcome_review_diagram_model.go`. Verify: cặp `from→to` có ở flow khác nhưng không ở flow này ⇒ `deviation`/`missing` đúng; không flow tương ứng ⇒ không nhãn thiết kế.
- [ ] **BE-OR-T706** `WeakBasis(design)` ⇒ hạ `deviation`/`missing` thành `advisory` kèm `advisoryReason=weak_basis`.
  Files: `domain/outcome_review_edge_state.go`. Verify: fixture dữ liệu #2318 với thiết kế 5 container + cạnh "ms-integration → Google Chat" ⇒ `advisory`, `unplannedCalls` không đếm cạnh này.
- [ ] **BE-OR-T707** `model.flowCoverage{total, drawn, items[]}` và verdict Flow tính trên flow đã ghép; proto + gateway.
  Files: `domain/outcome_review_model.go`, `domain/outcome_review_diagram_model.go` (`ComputeDesignVerdicts`), proto, `wscompat/channels_request_review.go`. Verify: 2 ứng viên, agent vẽ 1 ⇒ `total=2, drawn=1` kèm hàng chưa vẽ và lý do; model cũ không có trường vẫn đọc được.
- [ ] **BE-OR-T708** Trần minh bạch: thay `len(raw) > MaxContextDiagrams` (từ chối) bằng cắt theo ưu tiên (flow ứng viên → kiến trúc → data → behavior) và `diagrams_capped:N`; `REQUEST_REVIEW_MAX_FLOW_DIAGRAMS` (mặc định 8, 1–16); hình bị cắt thành `capped` trong `flowCoverage`.
  Files: `domain/outcome_review_diagram_model.go`, `domain/outcome_review_settings.go`. Verify: 17 hình hợp lệ ⇒ nhận 16, `diagrams_capped:1`; biến môi trường ngoài khoảng ⇒ về mặc định.

## Kết quả agent bền hơn (CR-OR-042 R1–R4)

- [ ] **BE-OR-T709** Phân loại vi phạm `local|fatal`: `ContextViolations` → `[]Violation{Code, Path, Message, Severity}`, vẫn in như cũ cho lời nhắc thử lại.
  Files: `domain/outcome_review_context.go`, `domain/outcome_review_design.go`, `domain/outcome_review_diagram_model.go`. Verify: hồi quy toàn bộ test `ContextViolations` hiện có; bảng mã/mức theo SOL-005 §4.
- [ ] **BE-OR-T710** Ép kiểu an toàn: `expected` `"true"/"false"` ⇒ bool; cảnh báo `coerced:<path>`; container `source=code` trong `design.containers` bị bỏ (cùng cạnh/tin nhắn nối tới) kèm `design_container_code_only_dropped:<label>`.
  Files: `domain/outcome_review_context.go` (parse), `domain/outcome_review_design.go` (`NormalizeDesign`). Verify: **dùng đúng hai khối đã làm hỏng #2318** (`expected` chuỗi; 4 container `ms-integration`, `ms-bug-management`, `MySQL của integration`, `Kafka`) ⇒ kết quả được nhận với cảnh báo.
- [ ] **BE-OR-T711** Ngân sách thử lại `REQUEST_REVIEW_RETRY_BUDGET_USD`: sau lần một, nếu chi phí đã dùng + dự đoán vượt ngân sách thì dừng, trả `REQUEST_REVIEW_RESULT_INVALID`.
  Files: `usecase/outcome_review_context_run.go`, `domain/outcome_review_settings.go`. Verify: agent giả trả sai hai lần với chi phí lớn ⇒ chỉ một lần chạy; ngân sách đủ ⇒ hai lần.
- [ ] **BE-OR-T712** Lưu `error_detail`: migration thêm cột `error_detail TEXT NOT NULL DEFAULT ''` cho `outcome_review_runs` (postgres, mysql, có `down`); ghi bằng `RedactRaw`, ≤ 30 dòng × 200 ký tự; `request.review.runs` trả `errorDetail`.
  Files: `migrations/postgres/*`, `migrations/mysql/*`, `adapter/*/outcome_review_run_repository.go`, `usecase/outcome_review_context_run.go`, proto + gateway. Verify: lỗi giả chứa canary giống khoá AWS ⇒ không lưu canary; migration lên/xuống chạy được ở cả hai dialect.

## Suy giảm và chi phí vẽ (CR-OR-042 R5–R8)

- [ ] **BE-OR-T713** Spike chọn engine: chạy cùng mô hình của #2318 qua `agent` và `builtin`; ghi số hình, chi phí, thời gian, kiểm artifact.
  Files: kết luận trong SOL-005 §8 (và README O3). Verify: bảng so sánh; chỉ đổi mặc định `auto` sau khi có bảng này (chưa quyết).
- [ ] **BE-OR-T714** Suy giảm: context sau code lỗi + có thiết kế đang dùng ⇒ hợp nhất thiết kế vào review (hình `design`, `designChecked=true`, `codeCompared=false`, cảnh báo `code_compare_failed`); Approve vẫn chặn.
  Files: `usecase/outcome_review_context_apply.go`, `usecase/outcome_review_context_run.go` (`execute`). Verify: run lỗi + lock ⇒ review có hình design; không lock ⇒ như hiện nay; Approve trả `REQUEST_REVIEW_CONTEXT_FAILED`.
- [ ] **BE-OR-T715** Vẽ theo yêu cầu: sinh lại hình nhận `sections` + `flowRef`; `selectModels` lọc theo `flowRef`; `estimate` trả chi phí cho đúng số hình; cột tuỳ chọn `diagram_count` để tính chi phí mỗi hình.
  Files: `usecase/outcome_review_draw_run.go` (`selectModels`), `usecase/outcome_review_regenerate.go`, migration, proto + gateway. Verify: chỉ vẽ hình khớp `flowRef`; ước tính = trung bình gần nhất × số hình.
- [ ] **BE-OR-T716** Chỉ số: tỷ lệ từ chối theo loại vi phạm, lần dùng ngân sách thử lại, chi phí mỗi hình, tỷ lệ flow `undrawable`; ghi vào `solutions` khi có số liệu pilot.
  Files: observer của review. Verify: nhãn thuộc tập đóng; không có chữ người dùng trong nhãn.

## Cổng của pha

- [ ] `go test ./services/request-service/... ./services/api-gateway/...` xanh (gồm parity kênh); golden prompt cập nhật.
- [ ] Chạy thật hai lần liên tiếp trên BL-E05: không bị từ chối vì hai lỗi của #2318 (AC6 của CR-OR-042); `flowCoverage` đúng với số flow bị ảnh hưởng.
- [ ] Ghi "Kết quả (ngày): …" cho từng task đã xong, nêu số liệu và chi phí.
