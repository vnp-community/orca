# AI Governance, Change Requests (v6)

> Lớp quản trị cho mọi lời gọi AI của `request-service`: ngân sách, chọn model và dự phòng, phiên bản prompt và provenance, bộ đánh giá trong CI, người trong vòng lặp, chống ảo giác, lưu vết tái lập, chế độ không gửi dữ liệu ra ngoài. Hợp đồng chung ở [README v6](../README.md).

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-REQ-034](./CR-REQ-034-ai-governance-budgets-evals-prompt-versioning.md) | Không ai đo chi phí AI (`usage-service` theo CLI/người dùng, `RecordTokenUsage` không ai gọi); model do agent chọn cứng; prompt không có phiên bản; không có bộ đo chất lượng; không có công tắc cấm dữ liệu ra ngoài | 🟠 P1 (ngân sách và `egress` là P0 trước khi bật cờ) | Large | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-REQ-033 (usage, maxTokens, lỗi có cấu trúc, hồ sơ năng lực)
        │
        ▼
CR-REQ-001, 002 ──▶ CR-REQ-034 ──▶ CR-REQ-005, 007, 008, 012 gọi qua AIGateway ──▶ CR-REQ-025 (GA)
```

| Bước | Lý do thứ tự |
|------|--------------|
| 1. Sổ cái và `PromptRegistry` | Không ảnh hưởng hành vi; có số liệu sớm (ngân sách mặc định không giới hạn) |
| 2. `EgressGuard`, `BudgetGuard` | Chặn trước khi tenant thật dùng; chỉ cần bảng của CR-REQ-002 |
| 3. `ModelRouter` | Cần hồ sơ năng lực (khoá nhà cung cấp có mặt) và `error.data` từ CR-REQ-033 |
| 4. Eval và `GroundingChecker` | Cần ít nhất CR-REQ-007 và 012 có đầu ra thật; mẫu vàng cần dữ liệu |
| 5. `HumanGatePolicy` chạy bóng | Cần điểm rủi ro (chưa có CR) mới tự duyệt thật; bước này chỉ ghi |

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| H1 | Một cổng `AIGateway` duy nhất cho mọi bước AI (`classify`, `solution`, `diagnosis`, `findings`, `answer`, `plan`, `taskspec`, `execute`) | Một chỗ đặt ngân sách, định tuyến model, ghi sổ, provenance |
| H2 | Sổ cái và ngân sách ở `request-service`, không mở rộng `usage-service` | `usage-service` khoá theo user/provider CLI, không có chiều Request và bước |
| H3 | Ghi sổ luôn bật, hạn mức mặc định không giới hạn | Có số liệu trước khi đặt hạn |
| H4 | Prompt bất biến theo phiên bản; sửa nội dung phải thêm phiên bản, CI kiểm | Tái lập và so sánh trước/sau |
| H5 | Eval xác định chạy mọi PR; eval với model thật chạy riêng, có trần chi phí | PR không phụ thuộc mạng và khoá |
| H6 | Tự duyệt tắt mặc định, có chạy bóng | Điểm rủi ro chưa hiệu chỉnh |
| H7 | Lưu vết mặc định `digest_only` | Mã nguồn khách không nằm trong DB theo mặc định |
| H8 | `internal_only` không có phần tử nội bộ thì như `disabled` | Agent chưa hỗ trợ model nội bộ cho `ai.complete` và `execPrompt` |

## Điểm cần xác nhận khi duyệt feature

- **Phụ thuộc CR-REQ-033 mục 2.8:** `ai.complete` phải trả `usage`, nhận `maxTokens`, trả lỗi có cấu trúc. Nếu không đổi agent, ngân sách chỉ dùng số ước lượng và dự phòng model dựa vào đọc chuỗi lỗi (mong manh).
- **Mâu thuẫn với CR-REQ-007 mục 6:** CR đó coi chi phí token "ngoài phạm vi, cần `usage-service`". Feature này chọn `request-service`, cần xác nhận.
- **RPC quản trị không có trong README 3.6:** `AiBudgetAdminService`; cùng câu hỏi như CR-REQ-010.
- **Mức mặc định** (1 triệu token mỗi Request, grounding 0.8, ngưỡng eval 3 điểm, 30 mẫu mỗi bước) là đề xuất, chưa có dữ liệu.
- **Chưa chạy:** không có lời gọi AI thật nào trong quá trình soạn; ước lượng token, 429 thật, chi phí kiểm chứng đường dẫn đều chưa đo.

## Điểm lệch với README v6 và CR khác

- README v6 mục 8 dòng 1 mô tả đường AI qua relay; feature này thêm lớp `AIGateway` trên cùng đường đó, không đổi đường.
- CR-REQ-007 dùng `solutions.options` có đối tượng bọc; feature này thêm khối `provenance` vào đó (cần CR-REQ-002 và 007 đồng ý).
- Số hiệu CR-REQ-026 đến 032 (đề xuất trong nghiên cứu) chưa có file trong v6; các tham chiếu tới CR-REQ-029 là tạm.
