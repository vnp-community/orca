# Request Service Foundation — Change Requests (v6)

> Dựng nền móng: service `request-service` và mô hình dữ liệu của Request. Xem bối cảnh, quyết định D1–D5 và hợp đồng chung ở [README v6](../README.md).

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-REQ-001](./CR-REQ-001-scaffold-request-service.md) | Chưa có service nào sở hữu Request, Solution, Approval | 🔴 P0 | Medium | 📝 Đề xuất, chưa triển khai |
| [CR-REQ-002](./CR-REQ-002-request-data-model-and-repositories.md) | Chưa có bảng, entity, repository cho Request và lịch sử loại | 🔴 P0 | Medium | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-REQ-001 (khung service, proto rỗng-có-health, DB trống, CI)
    └──▶ CR-REQ-002 (migration 0001, domain, repository hai dialect)
              └──▶ request-lifecycle (CR-REQ-003 trở đi)
```

CR-REQ-001 phải merge trước CR-REQ-002 vì CR-REQ-002 thêm migration và adapter vào module do CR-REQ-001 tạo. Cả hai là điều kiện tiên quyết của mọi feature còn lại trong v6.

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| F1 | `request-service` hỗ trợ cả Postgres và MySQL ngay từ đầu, chọn bằng `dbcapability.DetectDialectFromDSN` | `task-service` và `notification-service` đã có hai adapter; README v6 mục 6 bắt buộc hai DB. `mcp-service` Postgres-only là ngoại lệ tạm thời, không dùng làm mẫu cho điểm này |
| F2 | Layout theo `notification-service` (có `adapter/postgres` và `adapter/mysql`), không theo `mcp-service` | Cùng lý do F1 |
| F3 | Tên bảng và migration thuộc CR-REQ-002; CR-REQ-001 chỉ tạo bảng `outbox_events` và `processed_events` đủ để relay chạy | Tách khung hạ tầng khỏi mô hình nghiệp vụ, mỗi CR review độc lập |
| F4 | Không FK sang service khác. `project_id`, `reporter_id`, `plan_task_id`, `tenant_id` là id tham chiếu | README v6 mục 6 |
| F5 | Tenant isolation: Postgres dùng RLS, MySQL kiểm tra ở tầng ứng dụng bằng `tenant_id` trong mọi `WHERE` | `dbcapability` ghi `SupportsRLS=false` cho MySQL |
| F6 | Mọi cột thời gian dùng đồng hồ DB cho so sánh hết hạn, đồng hồ ứng dụng chỉ để ghi `created_at` khi cần id sinh trước | Mẫu của CR-TG-008 (lease dùng đồng hồ DB) |

## Phạm vi ngoài feature này

Máy trạng thái, registry luồng (CR-REQ-003); tiếp nhận nguồn (CR-REQ-004); phân loại (CR-REQ-005); Approval (CR-REQ-009); Solution sinh nội dung (CR-REQ-007); kênh gateway (CR-REQ-016). Hai CR ở đây chỉ dựng chỗ chứa và schema.

## Điểm lệch giữa README v6 và code, đã phát hiện khi viết feature này

- README v6 mục 3.5 ghi bảng `outbox`; mọi service hiện có dùng tên `outbox_events` (ví dụ `task.outbox_events`). Hai CR dùng `outbox_events`.
- README v6 không liệt kê bảng dedup consumer (`processed_events`) và bộ đếm số Request (`request_counters`). CR-REQ-001 và CR-REQ-002 bổ sung; cần cập nhật README v6 mục 3.5.
- README v6 mục 3.7 ghi subject `orca.request.*` với sự kiện như `request.created`; quy ước repo là `orca.<service>.<entity>.<event>` (ví dụ `orca.task.task.statuschanged`). Cần chốt: `orca.request.request.created` hay `orca.request.created`.
- README v6 mục 6 trỏ tới `docs/reference/git-compatibility.md` và `docs/STYLEGUIDE.md`; hai file này không có trong repo ở thời điểm khảo sát (2026-10-05). Không ảnh hưởng hai CR này.
