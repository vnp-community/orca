# TASK-E2E-016: Báo cáo và xử lý Request mồ côi

**From Solution:** SOL-E2E-003 · **Resolves:** BUG-E2E-009
**Priority:** P2 · **Service/Area:** request-service, ops
**Files:**
- tests/stages/e2e/orca-flows/report_orphan_requests.py (thay cho script shell dự kiến)
- backend-go/services/request-service/internal/usecase/ (metric)
- docs/guides/request/runbook-request-flow.md
**Depends on:** TASK-E2E-015
**Status:** `[x]` XONG về mã, tài liệu và dọn dữ liệu; metric chưa kiểm trên stack thật

---

## Context

Request đã tạo trước bản sửa có thể trỏ project không tồn tại; chúng tính vào hạn mức tenant.

## Việc cần làm

- [~] Truy vấn chỉ đọc: script đi qua gateway (`request.list` + `project.get`), không dùng RPC theo lô.
- [x] Huỷ có lý do `orphan_project` qua `--cancel` sau khi người có thẩm quyền duyệt danh sách. — _đã chạy trên dữ liệu thật: 27 Request, xem "Hoàn tất"_
- [x] Metric `orca_request_orphan_total{action}` (không quét xuyên tenant) và mục runbook.

## Tiêu chí hoàn thành

- [x] Có danh sách và cách xử lý; không xoá dữ liệu.
- [x] Có metric đếm lần chạm Request mồ côi.

## Kiểm thử

Chạy trên dữ liệu thử có Request mồ côi tạo trước đó.

## Kết quả thực thi

Metric đã xong; runbook còn lại.

**Metric** `orca_request_orphan_total{action}` (counter; `action` là nhóm RPC `read|write|decide|...`, hằng số thấp cardinality, không có tenant/id):
- Ý nghĩa: số lần một RPC nhắm vào một Request đang có mà project của nó không còn tồn tại. Đếm lượt chạm, không phải số Request khác nhau (UI poll một Request mồ côi sẽ tăng liên tục: tín hiệu nên báo động khi tốc độ khác 0).
- Điểm tăng: `AuthorizeRequestAction.locate` (`usecase/authorize_request_action.go`), sau khi nạp Request theo id (kể cả qua Approval/entity con) và trước quyết định phân quyền; kiểm bằng `ProjectExistence.Exists` (`ResolveProjectAccess`, đệm 30 giây, thời hạn 2 giây). Không đổi kết quả phân quyền, không đếm khi project-service lỗi hoặc cũ (không khẳng định mồ côi khi không thấy), không đếm RPC nhắm project (vd `CreateRequest`) hay Request không tồn tại. Chỉ nhìn từng Request trong tenant của người gọi nên không quét xuyên tenant. Cần `PROJECT_INTERNAL_CALLER_TOKEN` (không có thì không đếm).
- Nối dây: `usecase.OrphanObserver` (cổng), `metrics.Set.ObserveOrphanRequest`, `securityHooks.Orphans`.
- Test: `internal/adapter/grpc/request_access_orphan_test.go` (admin và người ngoài đều đếm, không đổi kết quả, không đếm khi project tồn tại/lỗi, bỏ qua RPC không nhắm Request, không thăm dò khi chưa cấu hình), `metrics/request_metrics_test.go`.

### Hoàn tất (người điều phối, 2026-10-09)

- Dọn dữ liệu local theo quyết định của người dùng: `report_orphan_requests.py` thấy 30 Request mở, 27 mồ côi (toàn bộ là dữ liệu thăm dò của các lượt e2e cũ: `rate probe`, `x`, `concurrent confirm`, `lifecycle main`); `--cancel` huỷ cả 27 với lý do `orphan_project`, không xoá dữ liệu. Chạy lại: `0 orphaned`.
- Runbook: `docs/guides/request/runbook-request-flow.md` có metric `orca_request_orphan_total{action}`, cách đọc (cảnh báo theo tốc độ tăng) và dòng "Request mồ côi" trong bảng triệu chứng.
- Đã redeploy `request-service` với metric; `/metrics` chạy và `orca_request_rate_limited_total` hiện, nhưng series `orca_request_orphan_total` chưa xuất hiện vì sau khi dọn không còn Request mồ côi nào bị chạm. **Chưa kiểm metric này trên stack thật**, mới kiểm bằng test đơn vị.
