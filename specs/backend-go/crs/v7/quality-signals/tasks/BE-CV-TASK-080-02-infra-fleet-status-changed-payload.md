# BE-CV-TASK-080-02: Thêm `worktree_id`, `dev_server_id` vào payload `orca.infra.agent.statusChanged`

**From Solution:** BE-CV-SOL-080
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/eventbus/agent_status_publisher.go`, `internal/usecase/ports.go` (~:712), `internal/usecase/agent_output_classifier.go`, `internal/usecase/agent_output_classifier_test.go` (sửa)
**Depends on:** không
**Status:** [x] DONE

## Context
Đã đọc: `PublishStatusChanged(ctx, tenantID, sessionID, status)` chỉ truyền `session.ID`; `domain.AgentSession` đã có `WorktreeID`, `DevServerID`; classifier gọi ở 3 chỗ (`Run` khi exit, khi đổi status, `onStartupTimeout`) và bỏ lỗi (`_ =`). `api-gateway/.../channels_agent.go:291` đẩy nguyên payload tới renderer. C-DM §2.2: thêm hai trường, tương thích ngược.

## Việc cần làm
1. Đổi cổng thành `PublishStatusChanged(ctx, tenantID string, session domain.AgentSession, status domain.AgentStatus) error`.
2. `statusChangedPayload` thêm `WorktreeID json:"worktree_id,omitempty"`, `DevServerID json:"dev_server_id,omitempty"`.
3. Cập nhật 3 điểm gọi và fake trong test; giữ `PublishRateLimited` nguyên.
4. Không thêm `previous_status` (vượt C-DM §2.2).

## Kiểm thử
- Unit: payload JSON có hai khoá khi session có giá trị, vắng khi rỗng; JSON cũ (chỉ `session_id`,`status`) vẫn giải mã được.
- `go build ./... && go test ./...` trong `infra-fleet-service`.

## Tiêu chí hoàn thành
- [x] Renderer/gateway không đổi hành vi.
- [x] Test classifier xanh.

## Rủi ro
`worktree_id` của infra-fleet có khớp `repo_bindings.worktree_id` hay không chưa kiểm chứng (SOL-080 mục 6).
