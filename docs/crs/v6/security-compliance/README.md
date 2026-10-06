# Security and Compliance, Change Requests (v6)

> Nền tảng bảo mật cho luồng Request: mô hình đe doạ AI, thi hành quyền ở `request-service`, quyền mức Request, cách ly tenant, kiểm toán, lưu giữ và xoá, bí mật, giới hạn tốc độ. Hợp đồng chung ở [README v6](../README.md).

| CR | Vấn đề | Priority | Effort | Trạng thái |
|----|--------|----------|--------|------------|
| [CR-REQ-035](./CR-REQ-035-security-and-compliance-baseline.md) | Gateway không có OPA; `TenantExtractionInterceptor` tin metadata; Request chưa có quyền mức Request; RLS của `task-service` không được đặt biến; audit thiếu trường; không có lưu giữ, xoá, xuất; bí mật chưa có bộ che dùng chung | 🔴 P0 | Large | 📝 Đề xuất, chưa triển khai |

## Thứ tự thực thi

```
CR-REQ-001, 002 ──▶ CR-REQ-035 (interceptor, RLS, request.rego)  ──▶  CR-REQ-009, 010, 016, 017 (đã dùng nhóm hành động)
                          │
                          └──▶ secretscan, AppendDetailed ──▶ CR-REQ-005, 007, 008, 024, 034
                          └──▶ lưu giữ, xoá, xuất ──▶ CR-REQ-025 (cổng GA)
```

| Bước | Lý do thứ tự |
|------|--------------|
| 1. Interceptor, `internalcaller`, `request.rego`, RLS có `set_config` | Phải có trước mọi RPC ghi; thêm sau là sửa khắp nơi |
| 2. `secretscan`, `AppendDetailed` | Dùng bởi CR-REQ-005, 007, 008, 024, 034; tách thành gói chung |
| 3. Giới hạn tốc độ | Cần phân nhóm RPC từ bước 1 |
| 4. Lưu giữ, xoá, xuất | Cần dữ liệu thật mới kiểm được; làm trước cổng GA |
| 5. Review bảo mật độc lập | Điều kiện GA của CR-REQ-025 |

## Quyết định chung của feature

| # | Quyết định | Lý do |
|---|-----------|-------|
| S1 | Mọi RPC tự kiểm quyền ở `request-service` bằng interceptor; không dựa gateway | README `api-gateway` ghi chưa có OPA trước định tuyến |
| S2 | `internalcaller.Guard` cho mọi RPC | Metadata tenant/user/role được tin vô điều kiện |
| S3 | Ma trận hành động-vai trò viết bằng Rego (`request.rego`); tập người duyệt giữ Go (CR-REQ-010) | Mỗi loại luật ở đúng chỗ |
| S4 | `actor_type=agent` không bao giờ duyệt, `StartPhase` hay quản trị | Cổng tồn tại để người kiểm soát agent |
| S5 | Postgres `FORCE RLS` và `set_config` mỗi giao dịch; MySQL kiểm bằng test quét SQL | RLS của `task-service` không được đặt biến; MySQL không có RLS |
| S6 | Che bí mật ở cổng vào, trước prompt, ở đầu ra, bằng một gói `common/secretscan` | Hiện chỉ có bản trong `mcp-service/internal` |
| S7 | Audit không chứa nội dung; hành động quan trọng đi qua outbox | Audit best-effort có thể mất |
| S8 | Ẩn danh hoá thay cho xoá hàng | Giữ truy vết và thống kê |

## Điểm cần xác nhận khi duyệt feature

- **Chốt quyền ghi mức Request:** ma trận ở CR-REQ-035 mục 2.3 đóng điểm "chưa ai chốt" của README v6 mục 8; cần chủ sản phẩm xác nhận (đặc biệt `member` không phân loại, `StartPhase` chỉ `owner|admin`).
- **mTLS giữa gateway và service chưa thấy trong repo;** `internalcaller` là lớp bổ sung, không thay thế.
- **Thay đổi ngoài series:** `common/grpcmw` (thêm `MetadataActorType`), `common/auditclient` (`AppendDetailed`), `api-gateway` (điền `actor_type`), `common/secretscan` (mới). Blast radius cần phân tích bằng `gitnexus_impact` trước khi sửa.
- **Chưa kiểm chứng:** độ phủ `secretscan` với dữ liệu khách, hành vi `FORCE RLS` với pool thật, hạn lưu giữ pháp lý của khách.

## Điểm lệch với README v6 và CR khác

- README v6 mục 8 dòng 14 nói `AppendDetailed` là đề xuất của CR-REQ-024; feature này thống nhất định nghĩa ở CR-REQ-035 mục 2.5 để tránh hai nơi.
- CR-REQ-010 chọn Go thuần cho chính sách; CR-REQ-035 dùng Rego cho ma trận hành động. Hai nơi bổ sung nhau, không mâu thuẫn, nhưng cần ghi rõ ranh giới.
- Tài liệu `07-security-architecture.md` yêu cầu mTLS và OPA ở gateway; cả hai chưa có, CR này không thay thế mà bù ở tầng service.
