# Phase 0 — Đọc code, spike, sửa CR (AM)

**Solutions:** BE-OR-SOL-001..004 · **Phụ thuộc:** không · **Mở khoá:** mọi phase sau · **Trạng thái:** 📝 chưa bắt đầu

Mục đích: giảm rủi ro trước khi viết mã. Không tạo mã sản phẩm ở phase này (trừ tài liệu CR).

## Đọc trước (ghi kết luận vào `specs/backend-go/crs/v9/outcome-review/READING-NOTES.md`)

- [ ] **BE-OR-P0-01** Đọc đường chạy agent có kết quả có cấu trúc: usecase Solution agent, task run (khối nonce, hồ sơ quyền chỉ-đọc, tham số `agent.execPrompt`), `dev_server_capability.go`.
  Verify: ghi tên hàm, tham số, nơi kiểm khả năng (`knowledgeHub`, `permissionProfile`) và cách thêm `reviewContext`, `reviewDraw`.
- [ ] **BE-OR-P0-02** Đọc mốc "Execution xong": `StartExecution`, `EvaluateExecution`, `completePhases`; cách task-service lưu nhánh/worktree task ("head" nhiều nhánh).
  Verify: kết luận có tách được "mọi task xong" khỏi `completed` không; công thức `heads[]`.
- [ ] **BE-OR-P0-03** Đọc `engine_settings.go`, `engine_settings_defaults.go`, `approval_authorization.go`, `outbox_subjects.go`, `ai_budget.go`, `erasable_columns.go`, `request_child_rules.go`, `request_link.go`.
  Verify: ghi (a) chỗ thêm cờ `outcome_review`, (b) tên hành động OPA, (c) tên subject outbox, (d) cách tính hạn mức AI, (e) có phải đăng ký cột xoá không, (f) có tạo con từ Request `completed` được không.
- [ ] **BE-OR-P0-04** Đọc cách đăng ký kênh `request.activity`/`request.executionSummary` ở `api-gateway` (`wscompat`), MCP và OPA `agent_rpcs`.
  Verify: danh sách tệp phải chạm cho một kênh mới.

## Spike (mỗi spike ghi kết quả + chi phí vào `evidence/` cạnh tài liệu này)

- [ ] **BE-OR-S1** `diagram-design` chạy không tương tác với hồ sơ cố định (không hỏi cổng style guide).
  Verify: một lệnh chạy thật sinh 1 HTML hợp lệ; nếu skill hỏi lại thì ghi phương án lùi.
- [ ] **BE-OR-S2** Artifact dùng vai trò màu (không hex) và điền token lúc hiển thị: thử đổi sáng/tối bằng cách thay khối `:root`.
  Verify: ảnh chụp hai giao diện; nếu không được, quyết định vẽ theo giao diện hoặc cố định màu.
- [ ] **BE-OR-S3** Đo token và thời gian cho 1, 4, 8 sơ đồ ≤ 12 nút.
  Verify: bảng số liệu (hoặc "agent không báo token" kèm thời gian và số lượt).
- [ ] **BE-OR-S4** Đọc `references/` của skill: danh mục loại, `self_check.py`, ghi đè phông, ký hiệu gần C4 container cho *architecture delta* và *sequence*.
  Verify: ghi tên loại, cờ cần thiết, giới hạn.
- [ ] **BE-OR-S5** KG của `vnp-qa-platform`: (a) công cụ MCP/`source_get` có trả `reviewState` từng phần tử không; (b) trích cạnh/luồng từ SAD 5.1, 5.3, 5.5, 8.1, 8.4 bằng agent so với parser mermaid; (c) tỉ lệ id xác minh được; (d) chi phí.
  Verify: bảng so kết quả với sơ đồ viết tay (đã có: kiến trúc container và luồng bug → Jira, 2026-10-20).

## Sửa CR (AM-1 đến AM-5) trong `docs/crs/v9`

- [x] **BE-OR-P0-05** (xong 2026-10-20: áp vào README 3.2, 3.3, 3.9, D15; CR-OR-002, 010, 020, 021, 030; link tương đối đã kiểm, không gãy) Áp AM-1 đến AM-5 vào README v9 (mục 3.2, 3.3, 3.9), CR-OR-002 (2.3, 2.4, 2.5, rủi ro), CR-OR-010 (kiểm "chữ phải thuộc mô hình" thêm `sad:`), CR-OR-020/021 (dải "Design not reviewed", nhãn nguồn nút), CR-OR-030 (test).
  Files: `docs/crs/v9/**` (xem `outcome-review/README.md` §2 của spec này).
  Verify: `grep -rn "unmapped" docs/crs/v9` chỉ còn nghĩa "không có ở KG lẫn SAD"; kiểm link tương đối không gãy.
- [ ] **BE-OR-P0-06** Nhờ người dùng xác nhận các điểm chưa chốt: mốc Execution xong; có mở pha `design` sớm hơn (`awaiting_analysis_approval`) không; tên hành động quyền; mặc định cờ trong `deploy/dev`.
  Verify: câu trả lời ghi vào đầu `outcome-review/README.md`.
