# Bug nghiệm thu e2e, vòng 2 (v2)

Vòng này là bước "chạy e2e → thu bug" sau khi triển khai checklist của [v1](../e2e-acceptance-v1/tasks/README.md). Bộ test: `tests/stages/e2e/orca-flows` (suite mới `notfound`), chạy trên stack Docker local với dev server là máy Mac thật và Claude thật qua phiên đăng nhập CLI (không có khoá API).

Trước khi sửa: **85 PASS / 2 FAIL / 4 BLOCKED / 1 SKIP** (AIS-02 là lỗi của test, AIS-04 là lỗi sản phẩm). Sau khi sửa: **103 PASS / 0 FAIL / 4 BLOCKED / 1 SKIP**.

## Bảng lỗi

| ID | Mức | Phát hiện bởi | Triệu chứng | Nguyên nhân gốc | Trạng thái |
|---|---|---|---|---|---|
| BUG-V2-001 | P0 | AIS-04 | `request.generatePlan` trả `REQUEST_NOT_IMPLEMENTED`; Request không thể đi qua `planning` | `GeneratePlan`, `CommitPlan`, `GetPlanProposal` của request-service chưa có handler; `usecase/generate_and_commit_plan.go` chỉ là hàm rỗng, `domain.PlanShape` rỗng. Phần `task-service.CreatePlanTree` đã xong (CR-REQ-012-01/02) | **Đã sửa** theo [SOL-V2-001](./solutions/SOL-V2-001-plan-stage.md): AIS-04 và PLN-01..08 PASS với Claude thật |
| BUG-V2-002 | P1 | DEV-04/05 ngay sau khởi động lại | Tạo project báo `PROJECT_DEV_SERVER_UNREACHABLE` tối đa 30 giây sau khi agent kết nối lại | Sức khoẻ dev server chỉ cập nhật theo chu kỳ 30 giây; agent vừa attach vẫn bị ghi `unreachable` | **Đã sửa**: `agentwsserver.Server.OnAttached` gọi `PollFleetHealth.PollDevServer` ngay (TASK-V2-002) |
| BUG-V2-003 | P1 | Quan sát agent sau khi triển khai lại | Mỗi lần infra-fleet khởi động lại, mọi agent thoát hẳn (không tự nối lại) | `Client.Close()` đóng session bằng mã 1000, agent coi 1000 là "kết thúc sạch" và thoát | **Đã sửa**: tắt dịch vụ đóng bằng 1001 (going away); thu hồi token vẫn dùng 1000 (TASK-V2-003) |
| BUG-V2-004 | P2 | Log infra-fleet | `outbox: publish failed … nats: no response from stream` lặp mỗi 2 giây vô hạn | Chủ đề `dev_server.health_degraded` không nằm dưới stream nào (`orca.infrafleet.>`), nên JetStream không bao giờ ack | **Đã sửa**: đổi chủ đề, migration `0040` chuyển các dòng đang kẹt, test chặn chủ đề ngoài stream (TASK-V2-004) |
| BUG-V2-005 | P2 | `report_orphan_requests.py` | `request.list` với `status=["awaiting_information"]` bị `INVALID_ARGUMENT` | Danh sách trạng thái hợp lệ ở gateway thiếu `awaiting_information` (CR-REQ-028) | **Đã sửa** + test (TASK-V2-005) |
| BUG-V2-006 | P3 | NFC-03 (đã đổi sang đúng tham số) | Id project sai định dạng cho `Internal: PROJECT_MEMBERSHIP_LOOKUP_FAILED` thay vì NotFound | Postgres từ chối bind id không phải UUID (22P02) và repo bọc thành lỗi chung | **Đã sửa** ở repo Postgres: id không hợp lệ = không tìm thấy (TASK-V2-006) |
| BUG-V2-007 | P3 | Log infra-fleet | Mỗi lần agent đặt tên tay (`dev-local-mac`) lấy token, log `ERROR … invalid input syntax for type uuid` | `linkEphemeralVmRuntime` truyền id không phải UUID vào cột uuid | **Đã sửa**: bỏ qua khi id không phải UUID (TASK-V2-007) |
| BUG-V2-008 | P3 (test) | AIS-02, đối soát dữ liệu | AIS-02 FAIL dù Solution được sinh; thăm dò RQL-20 để lại 1 Request mỗi lượt | Test không chờ run bất đồng bộ; Request của thăm dò không đăng ký dọn | **Đã sửa** trong bộ test (TASK-V2-008) |
| OBS-V2-A | thông tin | `report_orphan_requests.py` | 27 Request mở mà project đã bị xoá (phần lớn từ các lượt thử trước khi có kiểm tra project, và `rate probe` cũ) | Dữ liệu cũ của môi trường local | Cần người quyết định: `python3 report_orphan_requests.py --cancel` (huỷ có lý do `orphan_project`, không xoá). Tôi không tự chạy vì có Request không do phiên này tạo |
| OBS-V2-B | thông tin | AIS-01/03 | BLOCKED khi AI có sẵn | Đường lỗi "không có AI" không dựng được khi dev server có phiên Claude; đã được phủ bằng test đơn vị/Go e2e | Giữ nguyên, ghi trong báo cáo |
| OBS-V2-C | thông tin | MCP-01/02 | BLOCKED | `MCP_ENABLED` tắt trong `deploy/dev/.env` | Cấu hình, không phải lỗi |

## Vì sao không sửa BUG-V2-001 theo cách vá nhanh

Plan là bước giữa Solution và thực thi; thiếu nó thì Request không bao giờ tới `executing`, nên mọi luồng `change_request`, `bug`, `refactor`, `security`, `performance`, `ops_request`, `task`, `docs`, `hotfix` dừng ở `planning`. Task-service đã có `CreatePlanTree` nguyên tử, Approval, state machine và thực thi Phase đã có thật. Còn thiếu đúng phần request-service của CR-REQ-012 (tasks 03 đến 07). Vá bằng hàm trả cây cố định sẽ phá hợp đồng "AI đề xuất, người duyệt, mới ghi", nên làm đủ theo [SOL-V2-001](./solutions/SOL-V2-001-plan-stage.md).
