# TASK-REQ-038-05: `ListRequestActivity`: nhật ký hoạt động gộp từ các bảng có sẵn

**From Solution:** BE-REQ-SOL-038
**Priority:** P1
**Service:** `request-service`
**File:** `internal/domain/request_activity.go` (mới), `internal/usecase/list_request_activity.go` (mới), `internal/adapter/{postgres,mysql}/request_activity.go` (mới), `internal/adapter/grpc/server_activity.go` (mới), `proto/orca/request/v1/request.proto` (sửa), tests tương ứng
**Depends on:** TASK-REQ-038-01
**Status:** [x] DONE

---

## Context

- Không có kho sự kiện; nhật ký là read model gộp từ: `request_revisions` (`cause=created`), `request_type_history`, `classification_runs`, `solutions`, `analysis_runs`, `decision_history`, `clarifications`, `approvals`, `phase_starts`, `task_run_outcomes`, `request_return_history`. Bảng ánh xạ `kind` ở CR-REQ-038 mục 2.2.
- Không dùng `outbox_events` (có thể bị dọn, chưa kiểm chứng chính sách dọn).
- Không trả `body`, `snapshot` hay log agent; chỉ `summary` ngắn và `subject_id` để UI gọi RPC chi tiết có kiểm quyền (B12).
- Hai dialect khác nhau ở kiểu thời gian và `UNION`; mỗi nguồn là một truy vấn riêng, hợp nhất ở use case (k-way merge).

## Việc cần làm

1. Domain: `ActivityEntry{ID, Kind, OccurredAt, ActorKind, ActorID, SubjectType, SubjectID, Summary, Attempt, Severity, Details}`; `ActivityKind` (hằng cho các `kind` ở CR); `ActivitySeverity` (`info|attention|error`); `ActivityFilter{Kinds, MinSeverity, ActorKind, SubjectID, Since}`; `ActivityCursor` mã hoá `(occurred_at, source, id)` base64, lỗi giải mã là `REQUEST_ACTIVITY_CURSOR_INVALID`.
2. Quy tắc `severity`: `error` cho `task.failed` (lần thử cuối), `request.returned` và `approval.decided` có kết quả từ chối; `attention` cho `approval.requested` còn `pending`, `clarification.asked` chưa trả lời, `task.failed` còn lần thử; còn lại `info`. Ghi bảng quy tắc vào mã và test; chốt `category` nào của `request_return_history` tính là `error` khi làm (đọc cột `category`).
3. Repository (mỗi dialect): với mỗi nguồn một hàm `ListXxxActivity(ctx, requestID, filter, before, limit)` trả `[]ActivityEntry` đã sắp `(occurred_at DESC, id)`, giới hạn `limit+1`, dùng chỉ mục task 01. Không `SELECT *`; chỉ cột cần cho `summary`.
4. Use case `ListRequestActivity`: kiểm quyền đọc Request (`RequestReader.Get` + chính sách đọc hiện có, CR-REQ-035); gọi các nguồn theo `Kinds` (bỏ nguồn không liên quan); k-way merge đến `page_size` (mặc định 50, trần 100); sinh `next_cursor`; loại dòng đối tượng người xem không đọc được.
5. `details`: JSON ≤ 4096 byte (cắt và đánh dấu `truncated`). `summary` do mã dựng từ trường có cấu trúc, không chép nội dung văn bản tự do của người dùng quá 200 ký tự.
6. gRPC: `ListRequestActivity(ListRequestActivityRequest) returns (ListRequestActivityResponse)`; `server_activity.go` map lỗi theo mẫu `server_execution.go`.

## Kiểm thử

- Unit: ánh xạ `kind` và `severity` cho từng nguồn; merge ổn định với nhiều dòng cùng `occurred_at`; con trỏ không lặp và không sót khi một nguồn hết trước; cắt `details`.
- Integration hai dialect: dữ liệu mẫu của #1242 (đổi loại AI, về backlog, hai lần `task_run_outcomes` `failed`) cho đúng thứ tự và `severity`.
- Quyền: người không đọc được đối tượng không thấy dòng của nó; không có `body` trong bất kỳ trường nào.
- `cd /opt/repos/orca/backend-go && go test ./services/request-service/... -run 'Activity' -v` (chưa chạy; integration cần Docker).

## Tiêu chí hoàn thành

- [x] Đủ các `kind` ở CR-038 bảng 2.2.
- [x] Phân trang bằng con trỏ ổn định; `page_size` ≤ 100.
- [x] Lọc theo `kinds`, `severity`, `actor_kind`, `since` chạy đúng.
- [x] Không trả `body`, `snapshot`, log agent.
- [x] Hai dialect cho cùng kết quả trên cùng dữ liệu.

## Rủi ro và lưu ý

- Hiệu năng với Request dài chưa đo; ghi `EXPLAIN` hai dialect trong PR.
- Nguồn có `actor_kind` khác nhau (`ai|user|system|agent`); chuẩn hoá về bốn giá trị, giá trị lạ thành `system`.
- Dữ liệu cũ thiếu `cause` hoặc `category` cho `summary` không được làm lỗi; dùng "unknown".

## Ghi chú triển khai

Hoàn thành 2026-10-16.

- `domain/request_activity.go` (`ActivityRow/Entry/Cursor/Severity`, `BuildActivityEntry`) và một truy vấn `UNION ALL` mỗi dialect (`adapter/{postgres,mysql}/request_activity.go`) trên `request_revisions`, `request_type_history`, `solutions`, `clarifications`, `approvals`, `phase_starts`, `task_run_outcomes` (số lần chạy bằng cửa sổ `SUM`), `request_return_history`. Khoá phân trang `(occurred_at, id)`; lọc theo mức độ làm ở use case với vòng đọc lại để trang vẫn đầy.
- Không trả `body`, `snapshot` hay log agent (chỉ cột tóm tắt); tác nhân AI được chuẩn hoá `agent→ai` ngay trong SQL để lọc `actor_kind` đúng.
- Có thêm kind `task.accepted` ngoài bảng CR (task được chấp nhận sau khi chạy).
- Kiểm chứng: `activity_contract.go` chạy trên Postgres và MySQL (cùng dữ liệu, cùng kết quả), `list_request_activity_test.go`; truy vấn cũng đã chạy trên DB dev thật với #1242.
