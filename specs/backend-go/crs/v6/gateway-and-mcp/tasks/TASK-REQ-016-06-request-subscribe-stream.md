# TASK-REQ-016-06: Kênh stream `request.subscribe` và bảng `requestEventRegistry`

**From Solution:** BE-REQ-SOL-016
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/wscompat/channels_request_stream.go` (mới), `.../channels_request_stream_test.go` (mới), `.../excluded_channels.yaml`
**Depends on:** TASK-REQ-016-01, TASK-REQ-016-02; CR-REQ-003 (sự kiện `status_changed`) và outbox của `request-service`
**Status:** [x] DONE (đã kiểm chứng 2026-10-08: cd backend-go/services/api-gateway && go build ./... && go vet ./... && go test ./... -count=1)

---

## Context

- Mẫu: `channels_task_activity.go:60-135`: `taskActivitySubjects`, `ephemeralSubscriber`, `RegisterStream`, mỗi subject một goroutine `SubscribeEphemeral`, `PushEvent{Channel, Args}`, đóng `out` khi `ctx` xong.
- `common/eventbus/eventbus.go:24-30`: `Event{ID, TenantID, OccurredAt, Version, Payload}`; `SubscribeEphemeral` (dòng 196) tạo consumer chỉ có `FilterSubject`, `AckPolicy`, `InactiveThreshold` (chưa đặt `DeliverPolicy`, nguy cơ phát lại; SOL-016 mục 1 điểm 4).
- Subject: `orca.request.<entity>.<event>` (README v6 mục 8 dòng 4); stream `REQUEST` (CR-REQ-024 mục 2.1, chưa kiểm chứng). Bảng sự kiện: CONTRACT mục 3.
- Payload mẫu: `status_changed {request_id, project_id, from, to, trigger, type, actor_id, actor_kind, stage, reason, at}` (CR-REQ-003); `solution.proposed {tenant_id, request_id, solution_id, kind, ...}` (CR-REQ-007); `phase.started {request_id, phase_task_id, started_by, dispatched}` (CR-REQ-013). Payload không đồng nhất, nên mỗi dòng registry có hàm trích riêng.

## Việc cần làm

1. `type requestEventMapping struct{ Stream, Subject, EventType string; Extract func(json.RawMessage) requestEventFields }` và `var requestEventRegistry = []requestEventMapping{...}` cho 14 sự kiện CONTRACT mục 3. Hằng `requestStreamName = "REQUEST"`.
2. `RegisterRequestStreamChannel(r *Registry, bus ephemeralSubscriber, client requestv1.RequestServiceClient)` đăng ký `request.subscribe` bằng `RegisterStream`:
   - `args[0]` `{id?}`; nếu `id` có: `GetRequest` một lần dưới `Identity` (lỗi thì trả `requestChannelError`, không mở stream).
   - `subscribedAt := time.Now()`; mỗi sự kiện: bỏ nếu `ev.TenantID != id.TenantID`; bỏ nếu `ev.OccurredAt.Before(subscribedAt.Add(-2*time.Second))`; bỏ nếu có `id` mà `request_id` khác; không `id` thì áp quy tắc xem (mặc định an toàn: admin, hoặc `reporter_id`, `actor_id` bằng mình; ghi `TODO(CR-REQ-010)`).
   - Khung `RequestEventFrame` chỉ gồm trường whitelist CONTRACT mục 1, từ `Extract`; tuyệt đối không sao chép `Payload` nguyên.
3. Đóng gói: `select { case out <- PushEvent{Channel: "request.event", Args: []any{frame}}: case <-ctx.Done(): }`.
4. Đăng ký kênh chỉ khi `streamEnabled` (NATS kết nối). Cập nhật `registerRequestChannels` để gọi.
5. `excluded_channels.yaml`: `request.subscribe` loại trừ vĩnh viễn (`category: stream`, lý do "MCP có resources/ subscription riêng").

## Kiểm thử

- `fakeBus` (mẫu test của `task_activity`): phát sự kiện của tenant A và tenant B, assert chỉ A đến khung; sự kiện cũ (`OccurredAt` trước thời điểm đăng ký) bị bỏ; lọc theo `id`.
- Khung không chứa `body`, `title`, tên khoá snake_case; test marshal khung và quét.
- `GetRequest` lỗi thì không gọi `SubscribeEphemeral`.
- Thêm sự kiện giả mới vào registry không cần sửa logic (test bảng).
- `go test ./internal/adapter/wscompat/... -run RequestStream`.

## Tiêu chí hoàn thành

- [x] 14 sự kiện ánh xạ đúng `eventType`; whitelist trường.
- [x] Không rò tenant khác; bỏ sự kiện cũ.
- [x] Kênh không đăng ký khi NATS không có.
- [x] Parity xanh (loại trừ vĩnh viễn).

## Rủi ro và lưu ý

- N consumer mỗi socket (N = 14); đo trước khi bật rộng. Nếu tải lớn, thay bằng một consumer dùng chung có fan-out trong tiến trình (việc riêng).
- Phát lại lịch sử chưa chạy thử; bước 2 là lưới an toàn, không phải cách sửa gốc.
- Tên stream `REQUEST` có thể khác khi CR-REQ-001 chốt; đọc từ hằng, một chỗ sửa.

## Ghi chú triển khai (2026-10-08)

Chưa kiểm chứng trên NATS thật: hành vi phát lại lịch sử của ephemeral consumer và tên stream `REQUEST`; test dùng bus giả. Quy tắc xem khi không có `id`: admin hoặc `reporter_id`/`actor_id` = mình (Q3 vẫn mở).
