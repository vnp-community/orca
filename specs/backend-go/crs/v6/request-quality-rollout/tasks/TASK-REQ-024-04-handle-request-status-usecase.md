# TASK-REQ-024-04: `HandleRequestStatus` và hai subscription trên stream `REQUEST`

**From Solution:** BE-REQ-SOL-024
**Priority:** P1
**Service:** `issue-status-sync`
**File:** `backend-go/services/issue-status-sync/internal/domain/request_events.go` (mới), `.../internal/domain/events.go`, `.../internal/usecase/sync_request_status.go` (mới), `.../internal/usecase/sync_issue_status.go`, `.../internal/usecase/ports.go`, `.../internal/adapter/eventbus/subscriber.go`, `.../internal/config/config.go`, `.../cmd/server/main.go`
**Depends on:** TASK-REQ-024-03; CR-REQ-003 (hai sự kiện, trường bổ sung)
**Status:** `[ ] TODO`

---

## Context

- `sync_issue_status.go:104-124` `updateIssueStatus`: `OnlyFromCategory` một chuỗi; `case "linear","jira"` so `category != state.OnlyFromCategory`. `doWithRetry` 3 lần (dòng 24 `retryAttempts`), `syncableProviders = {"jira": true}`, `canSync` kiểm provider và actor.
- `subscriber.go`: `Run` có mảng `subs` 4 phần tử, mỗi cái một goroutine `consumer.Subscribe(ctx, stream, durableName(subject), subject, handle)`; `errCh` dung lượng 4 (cần tăng); `durableName` dạng `issue-status-sync-<subject với '-'>`.
- Payload `status_changed` và các trường thêm: SOL-024 mục 2.1. Chưa có code `request-service`.
- **Trước khi sửa `updateIssueStatus` và `SyncIssueStatus`, chạy `gitnexus_impact`** và ghi blast radius vào mô tả PR.

## Việc cần làm

1. `domain/request_events.go`: `RequestStatusEvent{EventID, TenantID, RequestID, ProjectID, From, To, Trigger, Type, ActorID, ActorKind, SourceProvider, SourceSite, SourceRef, ReporterID, Number int64, Version int64, Completed bool, TraceParent string}`.
2. `domain/events.go`: thêm `OnlyFromCategories []string` vào `TargetState`.
3. `sync_issue_status.go`: `updateIssueStatus` dùng helper `categoryAllowed(state, category)`: nếu `OnlyFromCategories` không rỗng thì thuộc tập; ngược lại so `OnlyFromCategory`. Hành vi cũ giữ nguyên (test hồi quy).
4. `sync_request_status.go`: `HandleRequestStatus(ctx, ev)` theo thứ tự ở SOL-024 mục 2.5; ánh xạ `mapRequestEventToStatus(ev, cfg)` trả `TargetState` hoặc rỗng theo bảng 2.2 (tên đích từ `cfg`); `HandleRequestStatus` dùng `RequestSyncStateStore.Advance` **trước** khi gọi Jira.
5. Cổng nhận tên trạng thái đích: `StatusNames{InProgress, Done string}` truyền vào `NewSyncIssueStatus` (hoặc hàm dựng thứ hai) từ cấu hình.
6. `subscriber.go`: thêm hai phần tử `{requestStream, "orca.request.request.status_changed", s.handleRequestEvent(false)}` và `{requestStream, "orca.request.request.completed", s.handleRequestEvent(true)}`; hằng `requestStream = "REQUEST"` (chưa kiểm chứng với CR-REQ-001); tăng `errCh` thành 6; `requestStatusWirePayload` đọc JSON.
7. `config.go`: `ISSUE_SYNC_JIRA_STATUS_IN_PROGRESS`, `ISSUE_SYNC_JIRA_STATUS_DONE`, `ISSUE_SYNC_REQUEST_COMMENTS_ENABLED` (bình luận làm ở task 08, ở đây chỉ đọc cấu hình), `REQUEST_SERVICE_ADDR`.
8. `main.go`: dựng store và truyền vào use case.

## Kiểm thử

- `sync_request_status_test.go` (mới): bảng đủ 11 loại; `executing` và `completed` ra đúng đích và đúng `OnlyFromCategories`; `request_backlog`, `cancelled`, `awaiting_*` không gọi tracker; dedupe `EventID`; `version` cũ bị `skipped_stale`; nguồn không phải Jira; thiếu actor; `actor_kind=ai` dùng `reporter_id`.
- `sync_issue_status_test.go` (mở rộng): hồi quy `OnlyFromCategory` cũ.
- `subscriber_contract_test.go`, `durable_name_test.go` (mở rộng): hai subject mới, tên durable `issue-status-sync-orca-request-request-status_changed` (kiểm ký tự hợp lệ: `_` được phép trong tên consumer).
- `go test ./... ` trong `services/issue-status-sync`.

## Tiêu chí hoàn thành

- [ ] Bảng 2.2 được kiểm đủ; sự kiện cũ, lặp không đổi Jira lần hai.
- [ ] Hành vi worktree, PR cũ không đổi (test hồi quy xanh).
- [ ] Hai subscription chạy cùng bốn cái cũ.

## Rủi ro và lưu ý

- Chưa kiểm chứng trên Jira thật; mọi tên trạng thái là cấu hình.
- `errCh` nhỏ hơn số goroutine có thể chặn goroutine thoát; kiểm khi tăng.
