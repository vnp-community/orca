# Solution Analysis — Change Requests (v6)

> Bước `analyzing` của Request: sinh Solution nhiều phương án (`change_request`, `refactor`), Chẩn đoán (`bug`, `hotfix`, `security`, `performance`), Findings (`spike`), Answer (`question`); chọn phương án và duyệt. Hợp đồng chung ở [README v6](../README.md).

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-REQ-007](./CR-REQ-007-solution-generation-options-and-selection.md) | Chưa có Solution; `AIDecompose` chỉ chia task, chạy đồng bộ, không bền khi restart; cần so sánh nhiều phương án, chọn và duyệt | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |
| [CR-REQ-008](./CR-REQ-008-diagnosis-findings-answer-analysis.md) | Code không chạy được agent khi không có worktree và vứt `stdout`; `spike`/`question`/chẩn đoán cần agent đọc repo và lưu kết quả có cấu trúc | 🟠 P1 | Large | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-REQ-002, 003, 009 ──▶ CR-REQ-007 ──▶ CR-REQ-008
```

| Bước | Lý do thứ tự |
|------|--------------|
| 007 trước | Tạo bảng `analysis_runs`, lease, vòng quét phục hồi, `GenerateSolution`, `ChooseSolutionOption` và `SubjectHandler` cho `subject_type=solution`; CR-REQ-008 dùng lại toàn bộ |
| 008 sau 007 | Thêm đường chạy `agent_readonly` và ba `kind` còn lại trên cùng bảng run và cùng RPC `GenerateSolution` |
| Cả hai sau 009 | Mở Approval và đăng ký `SubjectHandler` cần `ApprovalService` (CR-REQ-009). Có thể làm trước bằng cổng no-op, giống `ApprovalRecorder` của CR-REQ-005 |

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| A1 | Chỉ một RPC sinh: `GenerateSolution`; `kind` suy ra từ registry loại Request (CR-REQ-003) | README 3.6 không có RPC riêng cho chẩn đoán, findings, answer |
| A2 | Mọi lần sinh là một dòng `analysis_runs` bền (lease 90 giây, heartbeat 30 giây, quét phục hồi 30 giây) | Tránh lỗi Engine 1 mất việc khi restart (CR-TG-008) |
| A3 | Solution chỉ lưu sau khi qua kiểm schema; lỗi xóa dòng `draft`, Request giữ `analyzing` | `solutions.status` không có `failed`; không tự trả backlog |
| A4 | `solution` dùng `ai.complete` (không đọc repo); `diagnosis`, `findings`, `answer` dùng `agent.execPrompt` chỉ đọc trên `repo_path`, không tạo worktree | Solution là thiết kế văn bản; ba loại kia cần đọc code |
| A5 | Cổng duyệt: `subject_type=solution` cho cả `solution` và `diagnosis`; `findings` và `answer` có `subject_type` riêng; `hotfix` không có cổng | Khớp registry CR-REQ-003 và README 3.4 |
| A6 | Nội dung Request là dữ liệu không tin cậy: bọc trong khối có rào, đầu ra bị kiểm schema, che bí mật trước khi lưu | Request đến từ Jira, GitHub, webhook, MCP |
| A7 | `subject_digest` của Approval chống duyệt nội dung đã đổi; chọn phương án làm mới digest | Tiền lệ `ParamsHash` của `mcp-service` |
| A8 | Sinh lại có phản hồi dùng `GenerateSolution` kèm `feedback` (trigger `analysis_revision`), không dùng Reject | Reject đưa Request về backlog (CR-REQ-003) |

## Điểm cần xác nhận khi duyệt feature

- **Chỉ đọc chưa ép được:** `agent.execPrompt` không có tham số chỉ đọc; CR-REQ-008 dùng prompt, `trustPreset` mặc định và kiểm trạng thái repo sau chạy. Cần quyết định chấp nhận cho v1 hay chờ thay đổi ở agent (mã TypeScript, chưa đọc).
- **`ai.complete` và `agent.execPrompt` thật** chưa chạy trong khảo sát này: timeout, giới hạn, hành vi `trustPreset` khác `full` chưa kiểm chứng.
- Cần dev server kết nối cho mọi `kind`; dự án chưa kết nối thì không sinh được (`REQUEST_SOLUTION_NO_CONNECTION`, `REQUEST_ANALYSIS_NO_CONNECTION`).

## Điểm lệch với README v6 và CR khác (đã ghi ở mục "Câu hỏi mở" của từng CR)

- README 3.5 thiếu thực thể chạy cho `solutions.generation_run_id`; feature này thêm `analysis_runs`.
- `content_ref` không có định nghĩa; feature này không dùng.
- CR-REQ-002 đặt `options` mặc định `'[]'`, còn feature này dùng đối tượng bọc; `chosen_option` là chỉ số `INT`.
- `min_options` của `refactor` (README chỉ nêu "≥2" cho `change_request`): đề xuất 1.
