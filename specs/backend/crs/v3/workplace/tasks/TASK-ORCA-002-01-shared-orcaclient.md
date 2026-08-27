> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
## Status: 🔲 NOT STARTED

# TASK-ORCA-002-01 — `planner-service`: Orca HTTP Client + Application Ports

**Phase:** 0 — Nền tảng (không phụ thuộc CR khác trong bộ Orca)
**Scope:** ✅ vnp-workplace — Go (`backend/services/planner-service`, `:3013`)
**Source:** [SOL-ORCA-002 §3.5–§3.6](../solutions/SOL-ORCA-002-planner-orca-dispatcher.md#35-internalapplicationport--interfaces), [SOL-ORCA-001 §3](../solutions/SOL-ORCA-001-orca-api-bridge.md#3-api-contract-đầy-đủ)
**Depends On:** — (không phụ thuộc task Go nào khác trong bộ này)
**Blocked by (Orca team):** [TASK-ORCA-001-13](./TASK-ORCA-001-13-orca-planner-task-routes.md) — endpoint thật `/api/planner-tasks*` **chưa tồn tại**. Task này **không bị block** vì test dùng `httptest.Server` giả lập response theo đúng JSON schema đã khoá ở SOL-ORCA-001 §3 — chỉ integration test thật với Orca cần chờ Orca team.
**Estimated Files:** ~9 files (6 code + 3 test)
**Working Dir:** `/opt/repos/vnp-workplace/backend/services/planner-service/internal/`

---

## Bối cảnh quan trọng (đọc trước khi code)

- **Không còn package `shared/pkg/orcaclient` dùng chung liên-service** như thiết kế `vnp-planner`/`temporal-worker` cũ. `planner-service` là service Go duy nhất trong `vnp-workplace` gọi Orca ở thời điểm này — client đặt trực tiếp tại `internal/infrastructure/http/orcaclient/` (TDD-01 `infrastructure/{external}/` convention). Nếu sau này `diagnostics-service` (CR-ORCA-005) cũng cần gọi Orca, cân nhắc trích xuất `pkg/orcaclient` dùng chung **tại thời điểm đó** — không làm trước (SOL-ORCA-002 §9 Discoveries #8).
- Orca thật **chưa có** `/api/planner-tasks*` (xác nhận: `grep -rn "planner-tasks" /opt/repos/orca` → 0 kết quả, xem SOL-ORCA-001 §9). Package này code theo **contract đã khoá** ở SOL-ORCA-001 §3 (JSON schema, mã lỗi) — build và unit-test bằng `httptest.Server` giả lập đúng response shape.
- Base URL đúng theo Orca thật: HTTP `:6769` (KHÔNG phải `:3000` — SOL-ORCA-002 §9 Discoveries #7 lưu ý CR-ORCA-002 gốc còn sót comment `:3000` ở Change 5, đã sửa trong SOL).
- `Authorization: Bearer <ORCA_PLANNER_API_SECRET>` — secret này **chưa tồn tại phía Orca** (biến gần nhất là `ORCA_AGENT_API_SECRET`, dùng cho mục đích khác). Client vẫn implement đúng theo contract; secret thật sẽ do Orca team cấp khi endpoint xong.
- Logger dùng `log/slog` (TDD-00 §8), KHÔNG dùng `zap` như thiết kế `temporal-worker` cũ.
- Task này gồm **2 nhóm deliverable độc lập** nhưng cùng nền tảng, gộp lại vì cả hai đều là "phần không phụ thuộc gì khác" và thường implement song song bởi cùng 1 người: (A) `internal/infrastructure/http/orcaclient` (HTTP client thuần), (B) `internal/application/port` (3 interface: `OrcaClient`, `QueuePublisher`, `EventPublisher`).

---

## Mục tiêu

Viết `internal/infrastructure/http/orcaclient` (HTTP client gọi Orca) + `internal/application/port` (interfaces mà `application/dispatch` — TASK-ORCA-002-04 — và `infrastructure/queue/asynq` — TASK-ORCA-002-05 — phụ thuộc vào, theo TDD-00 §6 Interface Segregation).

---

## Acceptance Criteria

- [ ] `go build ./internal/infrastructure/http/orcaclient/... ./internal/application/port/...` thành công
- [ ] `go test ./internal/infrastructure/http/orcaclient/... -v -race -cover` pass 100%, coverage ≥ 80%
- [ ] `Client.SubmitTask` gọi đúng `POST /api/planner-tasks`, header `Authorization: Bearer <secret>`, `Content-Type: application/json`
- [ ] `Client.GetTaskStatus` gọi đúng `GET /api/planner-tasks/{id}`
- [ ] `Client.CancelTask` gọi đúng `POST /api/planner-tasks/{id}/cancel`
- [ ] Mọi mã lỗi HTTP (401/404/409/422/503) map đúng sang error type riêng trong `errors.go`
- [ ] `IsRetryable(err)` chỉ trả `true` cho `ErrUnavailable`
- [ ] `port.OrcaPollStatusPayload` chỉ định nghĩa **1 lần duy nhất** (`internal/application/port/queue_publisher.go`) — không định nghĩa lại ở bất kỳ file infra nào (SOL-ORCA-002 §2 D3)
- [ ] `port.OrcaClient`/`port.QueuePublisher`/`port.EventPublisher` không import bất kỳ package nào dưới `internal/infrastructure/` ngoại trừ `orcaclient` (chỉ dùng DTO của nó cho chữ ký hàm — TDD-00 §6)

---

## File 1: `internal/infrastructure/http/orcaclient/dto.go`

```go
// Package orcaclient implements a thin HTTP client for the Orca headless
// server API. Contract source of truth: SOL-ORCA-001 §3
// (backend/specs/crs/v3/orca/solutions/).
//
// NOTE: as of 2026-08-10, /api/planner-tasks* does NOT exist in the real Orca
// server — this client targets the contract the Orca team is expected to
// build (see TASK-ORCA-001-13). Build and unit-test against httptest.Server;
// do not point the base URL at a real Orca instance until that endpoint ships.
package orcaclient

// OrcaTaskRequest is the POST /api/planner-tasks request body (SOL-ORCA-001 §3.2).
type OrcaTaskRequest struct {
	PlannerTaskID      string   `json:"planner_task_id"`
	PlannerJobID       string   `json:"planner_job_id,omitempty"`
	PlannerCRID        string   `json:"planner_cr_id,omitempty"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	WorktreeRepo       string   `json:"worktree_repo"`
	WorktreeBranch     string   `json:"worktree_branch,omitempty"`
	AgentType          string   `json:"agent_type"`
	Priority           string   `json:"priority"`
	WHYChain           []string `json:"why_chain,omitempty"`
	AntiPatterns       []string `json:"anti_patterns,omitempty"`
	RequiredPatterns   []string `json:"required_patterns,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	CallbackURL        string   `json:"callback_url,omitempty"`
	TimeoutHours       int      `json:"timeout_hours,omitempty"`
}

// OrcaTaskResponse is the 201 Created response body.
type OrcaTaskResponse struct {
	OrcaTaskID    string `json:"orca_task_id"`
	PlannerTaskID string `json:"planner_task_id"`
	Status        string `json:"status"`
	TraceStream   string `json:"trace_stream"`
}

// OrcaTaskResult is the `result` field of GET /api/planner-tasks/{id} once terminal.
type OrcaTaskResult struct {
	Success       bool     `json:"success"`
	FilesCreated  []string `json:"files_created"`
	FilesModified []string `json:"files_modified"`
	CommitHash    string   `json:"commit_hash"`
	TestOutput    string   `json:"test_output"`
	ErrorMessage  string   `json:"error_message"`
	AgentOutput   string   `json:"agent_output"`
}

// OrcaTaskStatus is the GET /api/planner-tasks/{id} response body (SOL-ORCA-001 §3.3).
type OrcaTaskStatus struct {
	OrcaTaskID     string          `json:"orca_task_id"`
	PlannerTaskID  string          `json:"planner_task_id"`
	PlannerJobID   string          `json:"planner_job_id"`
	Status         string          `json:"status"` // pending|in_progress|review|done|blocked|cancelled
	WorktreePath   string          `json:"worktree_path"`
	AgentSessionID string          `json:"agent_session_id"`
	Progress       int             `json:"progress"`
	StartedAt      *string         `json:"started_at"`
	CompletedAt    *string         `json:"completed_at"`
	Result         *OrcaTaskResult `json:"result"`
}

var terminalStatuses = map[string]bool{"done": true, "blocked": true, "cancelled": true}

// IsTerminal reports whether Status is one of the terminal states.
func (s OrcaTaskStatus) IsTerminal() bool { return terminalStatuses[s.Status] }
```

---

## File 2: `internal/infrastructure/http/orcaclient/errors.go`

```go
package orcaclient

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	ErrUnauthorized  = errors.New("orca: unauthorized (check ORCA_PLANNER_API_SECRET)")
	ErrNotFound      = errors.New("orca: task not found")
	ErrConflict      = errors.New("orca: duplicate planner_task_id")
	ErrUnprocessable = errors.New("orca: agent type unavailable")
	ErrUnavailable   = errors.New("orca: server busy or unreachable") // retryable
)

// mapStatusError converts a non-2xx HTTP response into a typed error per SOL-ORCA-001 §3.2.
func mapStatusError(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusConflict:
		return ErrConflict
	case http.StatusUnprocessableEntity:
		return ErrUnprocessable
	case http.StatusServiceUnavailable, http.StatusTooManyRequests:
		return ErrUnavailable
	default:
		return fmt.Errorf("orca: unexpected status %d", resp.StatusCode)
	}
}

// IsRetryable reports whether the caller should retry. Only transient
// server-side capacity issues are retryable — auth/conflict/validation
// errors are not (SOL-ORCA-001 §3.2 table).
func IsRetryable(err error) bool {
	return errors.Is(err, ErrUnavailable)
}
```

---

## File 3: `internal/infrastructure/http/orcaclient/client.go`

```go
package orcaclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Client calls the Orca headless server REST API (SOL-ORCA-001).
type Client struct {
	baseURL    string
	apiSecret  string
	httpClient *http.Client
	logger     *slog.Logger
}

// New creates a Client. baseURL should point at Orca's real HTTP port, e.g.
// "http://orca:6769" (NOT :3000 — see SOL-ORCA-001 §9 / SOL-ORCA-002 §9 #7).
func New(baseURL, apiSecret string, timeout time.Duration, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{baseURL: baseURL, apiSecret: apiSecret, httpClient: &http.Client{Timeout: timeout}, logger: logger}
}

// SubmitTask calls POST /api/planner-tasks. Non-2xx maps to typed errors
// (errors.go) so callers can decide retry vs. non-retryable without
// inspecting HTTP codes directly.
func (c *Client) SubmitTask(ctx context.Context, req OrcaTaskRequest) (*OrcaTaskResponse, error) {
	var out OrcaTaskResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/planner-tasks", req, http.StatusCreated, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTaskStatus calls GET /api/planner-tasks/{orcaTaskID}.
func (c *Client) GetTaskStatus(ctx context.Context, orcaTaskID string) (*OrcaTaskStatus, error) {
	var out OrcaTaskStatus
	path := fmt.Sprintf("/api/planner-tasks/%s", orcaTaskID)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CancelTask calls POST /api/planner-tasks/{orcaTaskID}/cancel. Idempotent
// per contract (SOL-ORCA-001 §3.4) — calling on an already-terminal task
// returns 200, not an error.
func (c *Client) CancelTask(ctx context.Context, orcaTaskID string) error {
	path := fmt.Sprintf("/api/planner-tasks/%s/cancel", orcaTaskID)
	return c.doJSON(ctx, http.MethodPost, path, nil, http.StatusOK, nil)
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, wantStatus int, out any) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiSecret)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling orca %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		return mapStatusError(resp)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding orca response: %w", err)
	}
	return nil
}
```

---

## File 4: `internal/application/port/orca_client.go`

```go
package port

import (
	"context"

	"github.com/vnptech/kwp/services/planner-service/internal/infrastructure/http/orcaclient"
)

// OrcaClient — app-level interface, implemented by
// infrastructure/http/orcaclient.Client. Application layer depends on this
// abstraction (TDD-00 §6 Interface Segregation) rather than the concrete
// client, so use cases stay unit-testable without a real HTTP server.
type OrcaClient interface {
	SubmitTask(ctx context.Context, req orcaclient.OrcaTaskRequest) (*orcaclient.OrcaTaskResponse, error)
	GetTaskStatus(ctx context.Context, orcaTaskID string) (*orcaclient.OrcaTaskStatus, error)
	CancelTask(ctx context.Context, orcaTaskID string) error
}
```

## File 5: `internal/application/port/queue_publisher.go`

```go
package port

import (
	"context"
	"time"
)

// OrcaPollStatusPayload carries state between poll rounds of the
// self-rescheduling orca:poll_status asynq task (TASK-ORCA-002-05) — replaces
// Temporal activity heartbeat/context from the old design. Defined ONCE here
// (application layer); infrastructure/queue/asynq imports this type rather
// than redefining it (SOL-ORCA-002 §2 D3 — CR-ORCA-002 §3.3 duplicated this
// type, corrected here).
type OrcaPollStatusPayload struct {
	OrcaTaskID    string    `json:"orca_task_id"`
	PlannerTaskID string    `json:"planner_task_id"`
	DeadlineAt    time.Time `json:"deadline_at"`
}

// QueuePublisher enqueues (or re-enqueues) a poll round. Implemented by
// infrastructure/queue/asynq.Publisher (TASK-ORCA-002-05).
type QueuePublisher interface {
	EnqueueOrcaPollStatus(ctx context.Context, payload OrcaPollStatusPayload, delay time.Duration) error
}
```

## File 6: `internal/application/port/event_publisher.go`

```go
package port

import "context"

// EventPublisher publishes integration events (e.g. "orca.task.submitted",
// "orca.task.done"). Implemented by infrastructure/events.RedisPublisher
// (TASK-ORCA-002-04/05) — Redis Pub/Sub per TDD-07 §Event Subscriber, NOT
// NATS (SOL-ORCA-002 §2 D5).
type EventPublisher interface {
	Publish(ctx context.Context, subject string, payload any) error
}
```

---

## Test File 7: `internal/infrastructure/http/orcaclient/client_test.go`

Dùng `net/http/httptest.NewServer` để giả lập Orca — không gọi Orca thật (chưa tồn tại). Test cases bắt buộc:

```go
package orcaclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_SubmitTask_201_ReturnsOrcaTaskID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/planner-tasks" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Fatalf("missing/invalid Authorization header")
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(OrcaTaskResponse{OrcaTaskID: "orca-1", Status: "pending"})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-secret", 5*time.Second, nil)
	resp, err := c.SubmitTask(context.Background(), OrcaTaskRequest{PlannerTaskID: "TASK-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.OrcaTaskID != "orca-1" {
		t.Fatalf("expected orca-1, got %s", resp.OrcaTaskID)
	}
}

func TestClient_SubmitTask_409_ReturnsErrConflict(t *testing.T)      { /* respond 409, assert errors.Is(err, ErrConflict) */ }
func TestClient_SubmitTask_401_ReturnsErrUnauthorized(t *testing.T)  { /* respond 401, assert errors.Is(err, ErrUnauthorized) */ }
func TestClient_SubmitTask_503_ReturnsErrUnavailable(t *testing.T)   { /* respond 503, assert errors.Is(err, ErrUnavailable) */ }
func TestClient_GetTaskStatus_404_ReturnsErrNotFound(t *testing.T)   { /* respond 404, assert errors.Is(err, ErrNotFound) */ }
func TestClient_GetTaskStatus_200_ParsesTerminalResult(t *testing.T) {
	/* respond 200 with status="done", result != nil; assert OrcaTaskStatus.IsTerminal() == true */
}
func TestClient_CancelTask_200_NoError(t *testing.T) { /* respond 200 empty body */ }
func TestIsRetryable_OnlyTrueForErrUnavailable(t *testing.T) {
	if !IsRetryable(ErrUnavailable) {
		t.Fatal("expected ErrUnavailable to be retryable")
	}
	if IsRetryable(ErrUnauthorized) || IsRetryable(ErrConflict) || IsRetryable(ErrNotFound) {
		t.Fatal("expected non-ErrUnavailable errors to be non-retryable")
	}
}
```

Viết đầy đủ thân hàm cho các test còn để trống `/* ... */` ở trên theo đúng pattern của `TestClient_SubmitTask_201_ReturnsOrcaTaskID`.

---

## Verification

```bash
cd /opt/repos/vnp-workplace/backend/services/planner-service

go build ./internal/infrastructure/http/orcaclient/... ./internal/application/port/...
go vet ./internal/infrastructure/http/orcaclient/... ./internal/application/port/...
go test ./internal/infrastructure/http/orcaclient/... -v -race -cover
go test ./internal/infrastructure/http/orcaclient/... -coverprofile=orcaclient_cov.out
go tool cover -func=orcaclient_cov.out | grep total   # kỳ vọng >= 80%
```

**Không** chạy integration test nhắm vào Orca thật ở task này — endpoint chưa tồn tại (xem "Bối cảnh quan trọng"). Khi Orca team hoàn thành TASK-ORCA-001-13, bổ sung 1 test tag `integration` riêng, chạy thủ công/CI có gate, không chặn build mặc định.
