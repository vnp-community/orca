# SOL-E2E-006: Giữ bộ kiểm thử và tài liệu đi cùng mã; đưa e2e xuyên tầng vào CI; chỉnh các điểm vận hành

**Resolves:** OBS-B, OBS-C, OBS-D · **Nguyên nhân gốc:** RC-F (và là điều kiện để RC-A không tái phát)
**Service:** `request-service` (e2e, proto), `tests/stages/e2e/orca-flows`, `docs/`, `.github/workflows`
**Tasks:** TASK-E2E-027 đến 032

## Vấn đề

Bộ e2e Go dựng trước CR-REQ-028 nên 36 kịch bản đỏ; 49 stage còn skip; tài liệu vận hành ghi các giai đoạn là "chưa có". Bộ `orca-flows` mới chạy được nhưng chỉ chạy tay. Một số điểm vận hành (giới hạn ghi, kiểu câu hỏi Clarification) chưa đo/thiết kế hết.

## Thiết kế

1. **Hoàn tất e2e Go (T1):** commit sửa `kit.confirm` và metric (đã làm); viết các stage đang skip bằng stub AI của SOL-E2E-005 (Solution, Plan, `CommitPlan`, `ReportTaskOutcome`), theo TASK-REQ-025-04.
2. **Đưa `orca-flows` vào CI hai tầng:**
   - *Smoke hằng đêm:* `platform`, `workflow`, `request`, `matrix`, `exits` với stub AI trên stack dựng bằng compose.
   - *Nghiệm thu hằng tuần/trước release:* `--strict` (0 FAIL, 0 BLOCKED), cộng Playwright `request-web` (xem phía frontend).
   Báo cáo `last-run.{json,md}` lưu làm artifact.
3. **Giới hạn ghi có thể cấu hình và có thể quan sát:** `REQUEST_RATE_WRITE_PER_SECOND`/`…_BURST` (và các lớp khác) đọc từ env, mặc định giữ 5/10; thông báo lỗi kèm `retryAfterMs`; metric `orca_request_rate_limited_total{class}`. Đo với lưu lượng thật rồi cập nhật mặc định ở CR-REQ-035.
4. **Kiểu giá trị của câu hỏi Clarification (OBS-B):** thêm trường `value_type` (`string|number|boolean|list`) vào `ClarificationQuestion` (proto cộng thêm), suy ra từ luật kiểm tra hiện có của trường đích (ví dụ `time_box_hours` là `number`), để UI hiển thị đúng ô nhập và kiểm tra trước. Giữ tương thích: client cũ bỏ qua trường mới; máy chủ vẫn kiểm tra như cũ.
5. **Tài liệu bám mã:** cập nhật `docs/guides/request/README.md` (trạng thái giai đoạn), F36 (`target: server:|project:|connection:`), specs/backend-go/crs/v6/gateway-and-mcp/CONTRACT-request-ui-api.md (kênh `clarification.*`), README `deploy/dev`.

## Tương thích

Thay đổi cộng thêm. Giới hạn mặc định không đổi.

## Rủi ro

| Rủi ro | Giảm thiểu |
|---|---|
| e2e xuyên tầng chậm và không ổn định trong CI | chỉ dùng stub xác định; smoke dưới 2 phút; chạy `--strict` ít hơn |
| `value_type` suy ra sai làm UI chặn đáp án hợp lệ | UI chỉ chặn khi kiểu chắc chắn; máy chủ vẫn là nguồn quyết định |

## Tiêu chí hoàn thành

- Bộ Go T1: 0 FAIL, 0 stage skip với nhãn "implemented now".
- Job CI hằng đêm xanh 5 ngày liên tiếp trước khi coi là cổng chặn.
- Tài liệu liệt kê đúng trạng thái giai đoạn của luồng Request.
