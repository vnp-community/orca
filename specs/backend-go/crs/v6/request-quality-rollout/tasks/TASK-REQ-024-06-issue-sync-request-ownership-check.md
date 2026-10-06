# TASK-REQ-024-06: `issue-status-sync` bỏ qua đồng bộ worktree/PR khi Request sở hữu issue

**From Solution:** BE-REQ-SOL-024
**Priority:** P1
**Service:** `issue-status-sync`
**File:** `backend-go/services/issue-status-sync/internal/adapter/grpcclient/request_client.go` (mới), `.../internal/usecase/ports.go`, `.../internal/usecase/sync_issue_status.go`, `.../internal/usecase/request_lookup_retry.go` (mới), `.../internal/usecase/sync_issue_status_test.go`, `.../cmd/server/main.go`
**Depends on:** TASK-REQ-024-04, TASK-REQ-024-05
**Status:** `[ ] TODO`

---

## Context

- `adapter/grpcclient/tenant_forwarding.go` (32 dòng) và các client khác (`issuetracking_client.go`, `scm_client.go`, `project_client.go`) cho mẫu chuyển `tenant_id` vào metadata cho dịch vụ khác (vì consumer không có danh tính người gọi).
- `HandleWorktreeLifecycle`: sau `canSync` gọi `updateIssueStatus` trong `doWithRetry`. `HandlePullRequestLifecycle` tương tự; PR event hiện không có actor nên `canSync` đã bỏ qua (SOL-024 mục 1 điểm 1), nhưng vẫn thêm kiểm tra để đúng khi scm-integration-service sau này phát actor.
- Hợp đồng lỗi: handler hiện nuốt lỗi và `MarkSeen`; ở đây lỗi tạm thời của tra cứu phải trả lỗi để Nak (SOL-024 mục 1 điểm 2, 2.3).
- Consumer Nak không giới hạn `MaxDeliver` (chưa đọc `Subscribe`), nên cần bộ đếm.

## Việc cần làm

1. `ports.go`: `RequestLookupClient interface { Lookup(ctx, tenantID, provider, site, ref string) (found bool, err error) }`.
2. `request_client.go`: gọi `LookupRequestBySource` với `internalcaller.ClientInterceptor(token)` và chuyển tenant (như `tenant_forwarding.go`); token từ cấu hình (`INTERNAL_CALLER_TOKEN` hoặc tên đang dùng ở `request-service`; đọc `common/internalcaller` và main của `task-service` để theo đúng tên biến).
3. `request_lookup_retry.go`: `type deliveryCounter` map `eventID -> lần` với giới hạn kích cỡ (ví dụ 10000 phần tử, xoá ngẫu nhiên phần cũ) và `maxLookupDeliveries = 3`.
4. `sync_issue_status.go`: trong hai handler, sau `canSync`: `found, err := uc.requests.Lookup(...)`; `err != nil` và lần giao < 3 thì `return err` (Nak); lần thứ 3 thì ghi `failed`, `MarkSeen`, không chuyển Jira; `found` thì metric `skipped_request_owned{source}`, log, `MarkSeen`, thoát. Constructor nhận `RequestLookupClient` (có thể `nil` khi `REQUEST_SERVICE_ADDR` rỗng: bỏ kiểm tra, hành vi cũ).
5. `main.go`: dial `REQUEST_SERVICE_ADDR` nếu có.

## Kiểm thử

- `sync_issue_status_test.go`: có Request thì không gọi `TransitionIssue` (`skipped_request_owned`); không có Request thì hành vi cũ nguyên vẹn (cả hai handler); lỗi tra cứu trả lỗi hai lần rồi `MarkSeen` ở lần ba; client `nil` giữ hành vi cũ.
- `go test ./...` trong `services/issue-status-sync`.

## Tiêu chí hoàn thành

- [ ] Issue có Request chưa kết thúc: worktree created và PR merge không đổi Jira.
- [ ] Lỗi tra cứu không gây giao lại vô hạn.
- [ ] Không có Request: test hồi quy xanh.

## Rủi ro và lưu ý

- Đóng an toàn khi không chắc (lỗi kéo dài): không đổi Jira; chấp nhận mất một chuyển trạng thái, vá bởi sự kiện sau.
- Bộ đếm theo bộ nhớ không chia sẻ giữa replica; giới hạn thực tế là 3 lần mỗi replica nhận.
