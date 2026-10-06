# Impact Risk: Đánh giá tác động và chấm điểm rủi ro (v6)

> Biến "tác động và rủi ro" của Solution, Plan và thực thi thành tài liệu có cấu trúc: công cụ thu thập dữ liệu xác định, điểm tính bằng quy tắc có phiên bản, AI chỉ diễn giải, mức rủi ro điều khiển cổng duyệt. Hợp đồng chung ở [README v6](../README.md).

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-REQ-030](./CR-REQ-030-impact-assessment-and-risk-scoring.md) | Người duyệt không có số liệu đo về tác động; `make proto-lint` nuốt mọi lỗi (`A && B \|\| true`); không có `buf breaking` trong CI; không có cổng theo mức rủi ro, không phát hiện lệch khi chạy | 🟠 P1 | Large | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-REQ-007, 009, 012 ─▶ CR-REQ-029 ─▶ CR-REQ-030 ─▶ CR-REQ-032, 036 (đồ thị và UI chấp nhận rủi ro)
CR-REQ-031 (danh mục service) ─ nền cho lens Kiến trúc, không chặn
```

CR-REQ-030 giao theo ba giai đoạn để không chặn sai bằng điểm chưa hiệu chỉnh:
0. **Thử ngoại tuyến:** chạy bộ thu thập bằng CLI trên 3 đến 5 thay đổi lịch sử (ví dụ `docs/crs/v3/project-workspace/IMPACT-ASSESSMENT-2026-09-15-worktree-session-jira.md`), so mức tính ra với mức thật; chỉnh bảng ngưỡng.
1. **`shadow`:** đánh giá, hiển thị "tham khảo", ghi `risk_outcomes`, không chặn cổng. Mặc định.
2. **`enforce`:** admin tenant bật khi đủ dữ liệu hiệu chỉnh; có thể chặn từ mức Cao trở lên.

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| R1 | Điểm do quy tắc có phiên bản (`rp/1`), hàm thuần; AI chỉ diễn giải và không được đổi điểm; `narrative` nằm ngoài `digest` | Tái lập, kiểm toán, không bị nội dung Request lèo lái |
| R2 | Ba thời điểm: Option của Solution (ước lượng), Plan/Phase/Task (chi tiết), thực tế sau chạy (`git diff`) | Bản thực tế đáng tin nhất và dùng để phát hiện lệch |
| R3 | Chín chiều, mức tổng = 60% chiều cao nhất + 40% trung bình có trọng số, luật cứng nâng mức tối thiểu | Một chiều nguy hiểm không bị che |
| R4 | Chiều không đo được mang điểm 50 và tăng bất định; `risk: unknown` ở đồ thị, không bao giờ vẽ như `low` | "Không đo được" khác "thấp" |
| R5 | `buf breaking` chạy trực tiếp, không qua `make proto-lint` | `A && B \|\| true` nuốt mọi lỗi |
| R6 | Collector chạy trên dev server qua `agent.exec`, không tự `analyze` lại index; chỉ báo lỗi thời | Chưa đo chi phí; SSH và remote chạy như `ai.complete` |
| R7 | `RiskAcceptance` gắn `assessment_digest`; `assessment.digest` nằm trong `subject_digest` của Approval | Đánh giá đổi thì phải duyệt lại |
| R8 | Lệch kế hoạch (drift) dùng Approval `subject_type=phase`, `stage=drift_review`; không thêm `subject_type` | Giữ CHECK của CR-REQ-009 |
| R9 | Đồ thị trả `GraphPayload` của CR-REQ-032 (`risk`, `change`, `truncated`, `stale`) | Một hợp đồng cho mọi lens |

## Phạm vi ngoài feature này

Giao diện (CR-REQ-032 đồ thị, CR-REQ-036 chấp nhận rủi ro, drift), quyền hạn mức tổ chức (CR-REQ-010, 035), danh mục service và dữ liệu nền (CR-REQ-031), bước deploy (Orca không có; `pre_deploy` là cổng duyệt).

## Điểm lệch và điểm cần xác nhận khi viết feature này

- `Makefile` dòng 81 của backend-go khiến `proto-lint` không bao giờ đỏ; CI không có `buf breaking`. Việc sửa Makefile và thêm job CI là đề nghị riêng, ngoài series.
- GitNexus CLI không có cờ `--json` trong `--help` đã đọc; định dạng đầu ra để phân tích chương trình chưa kiểm chứng. `status` báo index lỗi thời tại thời điểm viết (index `d819812`, HEAD `1b0c760`).
- `buf breaking` đã chạy được trên máy soạn thảo (7,6 giây); chưa thử trong worktree `task/<id>` hay qua SSH.
- Hai người duyệt cho mức Nghiêm trọng cần mở rộng CR-REQ-009 (`required_approvals`): chưa chốt.
- Nhãn mới `gate:feature_flag`, `check:rollback_rehearsal` cần vào bảng nhãn của CR-REQ-012.
