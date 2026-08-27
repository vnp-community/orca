> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
## Status: 🔲 NOT STARTED

# TASK-ORCA-005-10 — `planner-service`: Infrastructure Layer (in-memory `SessionStore`, Redis `ReloadEventPublisher`, `OrcaClient` adapter)

**Phase:** 3 — Read-model / observability
**Scope:** ✅ `vnp-workplace` — Go (`planner-service`, module `orcasession`)
**Source:** [SOL-ORCA-005 §2.2, §3.3, §9](../solutions/SOL-ORCA-005-planner-orca-session-monitor.md#33-sessionstore--port-in-memory-thay-repository-postgres)
**Depends On:** [TASK-ORCA-005-08](./TASK-ORCA-005-08-signal-svc-domain.md), [TASK-ORCA-005-09](./TASK-ORCA-005-09-signal-svc-application.md)
**Coordination cần thiết (không phải blocking cứng):** `OrcaClientAdapter` wrap `backend/pkg/orcaclient` — package do CR-ORCA-002 cung cấp (TASK-ORCA-002-01), **chưa tồn tại trong repo tại thời điểm task này được viết** (`find` không ra kết quả). Có thể code + unit test song song bằng `httptest.Server` giả lập Orca, nhưng wiring thật (`cmd/main.go`, TASK-ORCA-005-11) cần package đó tồn tại.
**Estimated Files:** ~6 files
**Working Dir:** `backend/services/planner-service/internal/infrastructure/`

> **Đổi tên khỏi bản gốc:** giữ nguyên số hiệu/tên file (`TASK-ORCA-005-10`, nhắc `signal-svc`) nhưng nội dung mô tả 3 adapter cho module `orcasession` trong `planner-service`. **KHÔNG có Postgres, KHÔNG có migration, KHÔNG có NATS, KHÔNG có background trace-republisher** — tất cả đã bị loại khỏi kiến trúc (xem SOL-ORCA-005 §2.2, §3.6).

---

## Bối cảnh quan trọng — ĐỌC KỸ TRƯỚC KHI CODE

1. **KHÔNG có `migrations/000002_orca_sessions.up.sql`, KHÔNG có `PostgresOrcaSessionRepository`.** Đây là khác biệt lớn nhất so với bản gốc `signal-svc`. Thay vào đó: `SessionStore` in-memory (`map[string]*orcasession.OrcaSession` + `sync.RWMutex`) trong `internal/infrastructure/memory/`. Lý do đầy đủ: [SOL-ORCA-005 §2.2](../solutions/SOL-ORCA-005-planner-orca-session-monitor.md#22-trạng-thái-session-in-memory-không-postgres--khác-biệt-lớn-nhất-so-với-bản-gốc).

2. **`ReloadEventPublisher` PHẢI tái dùng ĐÚNG pattern đã chạy thật** ở `skills-service`/`plugin-registry` — không phải suy đoán, có code thật + integration test pass. Copy cấu trúc từ:
   - `backend/services/skills-service/internal/infrastructure/messaging/reload_event_publisher.go`
   - `backend/services/skills-service/internal/application/port/reload_publisher.go`

   Điểm khác biệt bắt buộc so với bản gốc (`skills-service` dùng field `Kind string \`json:"kind"\`` phẳng cùng cấp `WorkspaceID`) — port `EventPublisherPort` của `orcasession` (TASK-ORCA-005-09 File 1) nhận `payload []byte` đã marshal sẵn (bao gồm `"kind"` bên trong) từ application layer, KHÔNG nhận `(workspaceID, kind, data)` rời như `skills-service`. Lý do: `orcasession` publish nhiều shape payload khác nhau tuỳ `EventKind` (progress/timeout/unreachable/offline có field riêng) — gói sẵn ở application layer đơn giản hơn định nghĩa 1 struct `ReloadEvent` chung phải nhồi đủ mọi field optional. Task này chỉ cần publish `[]byte` đã có sẵn lên đúng channel, KHÔNG tự marshal lại.

3. **KHÔNG có `OrcaTraceProxy` (background goroutine republish NATS)** như bản gốc — trace stream giờ là **on-demand HTTP proxy handler**, thuộc TASK-ORCA-005-11 (interface layer), không phải infrastructure. Task này chỉ cần cung cấp `OrcaClientPort`/HTTP client dùng chung đủ để TASK-ORCA-005-11 dùng lại cho việc proxy request (không cần thêm code riêng ở đây ngoài client cơ bản).

4. **Route health check**: dùng `GET /health` theo đúng CR-ORCA-005 Change 3 (xem TASK-ORCA-005-09 Bối cảnh). Không tự đổi sang `/health/ready` trong task này.

---

## Mục tiêu

`SessionStore` in-memory + `ReloadEventPublisher` (Redis) + `OrcaClientAdapter` (wrap `pkg/orcaclient`) — 3 adapter implement các port đã khai báo ở TASK-ORCA-005-09.

---

## Acceptance Criteria

- [ ] `SessionStore` implement đủ 6 method của `orcasession.SessionStore` (TASK-ORCA-005-08), an toàn concurrent (`-race` sạch)
- [ ] `ReloadEventPublisher.Publish(ctx, workspaceID, payload)` publish lên đúng channel `reload-events:{workspaceID}` — dùng CHÍNH XÁC `fmt.Sprintf("reload-events:%s", workspaceID)`, khớp `diagnostics-service.Watcher`'s `PSubscribe("reload-events:*")` + tách suffix
- [ ] `ReloadEventPublisher` KHÔNG tự ý bọc lại payload — publish nguyên `[]byte` nhận được từ application layer (không double-marshal, không đổi field `"kind"`)
- [ ] `OrcaClientAdapter` implement đủ `port.OrcaClientPort` (`GetTaskStatus`, `CancelTask`, `HealthCheck`) — gọi `GET /health` cho `HealthCheck`
- [ ] `go build ./internal/infrastructure/...` thành công
- [ ] Coverage ≥ 70% (mức infra/integration theo TDD-00 §12.3)

---

## File 1: `internal/infrastructure/memory/session_store.go`

```go
// Package memory implements orcasession.SessionStore purely in-memory
// (SOL-ORCA-005 §2.2) — NOT a persistence layer. See TASK-ORCA-005-08 for
// why this is the correct design here (module `orcasession` shares a
// process with the dispatch module, CR-ORCA-002, which owns durable
// state).
package memory

import (
	"sync"

	"github.com/vnptech/kwp/services/planner-service/internal/domain/orcasession"
)

type SessionStore struct {
	mu       sync.RWMutex
	byOrca   map[string]*orcasession.OrcaSession // orca_task_id -> session
	byPlanner map[string]string                  // planner_task_id -> orca_task_id (secondary index)
}

func NewSessionStore() *SessionStore {
	return &SessionStore{
		byOrca:    make(map[string]*orcasession.OrcaSession),
		byPlanner: make(map[string]string),
	}
}

func (s *SessionStore) Upsert(sess *orcasession.OrcaSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byOrca[sess.OrcaTaskID] = sess
	s.byPlanner[sess.PlannerTaskID] = sess.OrcaTaskID
}

func (s *SessionStore) Get(orcaTaskID string) (*orcasession.OrcaSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.byOrca[orcaTaskID]
	return sess, ok
}

func (s *SessionStore) GetByPlannerTaskID(plannerTaskID string) (*orcasession.OrcaSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	orcaTaskID, ok := s.byPlanner[plannerTaskID]
	if !ok {
		return nil, false
	}
	sess, ok := s.byOrca[orcaTaskID]
	return sess, ok
}

func (s *SessionStore) List() []*orcasession.OrcaSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*orcasession.OrcaSession, 0, len(s.byOrca))
	for _, sess := range s.byOrca {
		out = append(out, sess)
	}
	return out
}

func (s *SessionStore) ListByWorkspace(workspaceID string) []*orcasession.OrcaSession {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*orcasession.OrcaSession, 0)
	for _, sess := range s.byOrca {
		if sess.WorkspaceID == workspaceID {
			out = append(out, sess)
		}
	}
	return out
}

func (s *SessionStore) Delete(orcaTaskID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.byOrca[orcaTaskID]; ok {
		delete(s.byPlanner, sess.PlannerTaskID)
	}
	delete(s.byOrca, orcaTaskID)
}
```

**Kiểm tra tường minh var `_ orcasession.SessionStore = (*SessionStore)(nil)`** ở cuối file (compile-time interface check, convention chuẩn của monorepo — xem cách `postgres.NewSkillRepo` làm ở `skills-service`).

---

## File 2: `internal/infrastructure/messaging/reload_event_publisher.go`

Tái dùng NGUYÊN pattern thật đã chạy ở `skills-service`/`plugin-registry` (channel construction, `redis.ParseURL` fallback), chỉ khác ở chỗ publish `[]byte` thẳng (đã marshal ở application layer, xem Bối cảnh #2):

```go
// Package messaging implements orcasession.EventPublisherPort against the
// reload-events:{workspace_id} Redis Pub/Sub channel — SAME pattern as
// skills-service/plugin-registry's real, tested
// internal/infrastructure/messaging/reload_event_publisher.go (channel
// construction verified against diagnostics-service's real subscriber,
// SOL-ORCA-005 §9).
package messaging

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type ReloadEventPublisher struct {
	client *redis.Client
}

func NewReloadEventPublisher(redisURL string) *ReloadEventPublisher {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		opts = &redis.Options{Addr: redisURL}
	}
	return &ReloadEventPublisher{client: redis.NewClient(opts)}
}

// NewReloadEventPublisherFromClient wires an already-constructed
// *redis.Client — used when main.go shares one Redis client across the
// publisher and other components (matches skills-service's
// NewReloadEventPublisherFromClient).
func NewReloadEventPublisherFromClient(client *redis.Client) *ReloadEventPublisher {
	return &ReloadEventPublisher{client: client}
}

// Publish sends payload (already-marshaled JSON, including the top-level
// "kind" field set by the application layer — see orcasession.monitor.go
// / health_monitor.go) verbatim onto reload-events:{workspaceID}. Does NOT
// re-marshal or mutate payload.
func (p *ReloadEventPublisher) Publish(ctx context.Context, workspaceID string, payload []byte) error {
	channel := fmt.Sprintf("reload-events:%s", workspaceID)
	if err := p.client.Publish(ctx, channel, payload).Err(); err != nil {
		return fmt.Errorf("publish orca session event on %s: %w", channel, err)
	}
	return nil
}
```

---

## File 3: `internal/infrastructure/orcaclient/adapter.go`

```go
// Package orcaclient adapts backend/pkg/orcaclient (CR-ORCA-002,
// TASK-ORCA-002-01 — package path/API NOT yet confirmed to exist as of
// this task's writing) to orcasession.OrcaClientPort. Until that package
// lands, this file's import path is a placeholder — update the import once
// TASK-ORCA-002-01 is implemented.
package orcaclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/vnptech/kwp/services/planner-service/internal/application/orcasession"
)

// Adapter wraps a plain HTTP client against Orca — implemented directly
// here (not via pkg/orcaclient) if that shared package is not yet
// available, so this task is not hard-blocked. Swap the internals for
// pkg/orcaclient once TASK-ORCA-002-01 lands, keeping the same
// orcasession.OrcaClientPort surface.
type Adapter struct {
	baseURL    string
	httpClient *http.Client
}

func NewAdapter(baseURL string, timeout time.Duration) *Adapter {
	return &Adapter{baseURL: baseURL, httpClient: &http.Client{Timeout: timeout}}
}

type getTaskStatusResponse struct {
	Status   string `json:"status"`
	Progress int    `json:"progress"`
}

// GetTaskStatus calls Orca's task-status endpoint. Exact route (proposed
// as GET /api/planner-tasks/{id} in CR-ORCA-001, NOT confirmed to exist in
// real Orca as of the CR-ORCA bucket's own survey) is out of scope for
// this task to re-verify — follows whatever TASK-ORCA-002-01/CR-ORCA-001
// finalize; placeholder path below.
func (a *Adapter) GetTaskStatus(ctx context.Context, orcaTaskID string) (orcasession.OrcaTaskStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/api/planner-tasks/"+orcaTaskID, nil)
	if err != nil {
		return orcasession.OrcaTaskStatus{}, err
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return orcasession.OrcaTaskStatus{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return orcasession.OrcaTaskStatus{}, fmt.Errorf("orca GetTaskStatus: status %d", resp.StatusCode)
	}
	var body getTaskStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return orcasession.OrcaTaskStatus{}, fmt.Errorf("decode orca task status: %w", err)
	}
	return orcasession.OrcaTaskStatus{Status: body.Status, Progress: body.Progress}, nil
}

func (a *Adapter) CancelTask(ctx context.Context, orcaTaskID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/api/planner-tasks/"+orcaTaskID+"/cancel", nil)
	if err != nil {
		return err
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("orca CancelTask: status %d", resp.StatusCode)
	}
	return nil
}

// HealthCheck calls GET /health (CR-ORCA-005 Change 3 pseudocode — see
// TASK-ORCA-005-09 Bối cảnh for why this route, not /health/ready, is used
// here).
func (a *Adapter) HealthCheck(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("orca health check: status %d", resp.StatusCode)
	}
	return nil
}
```

---

## Test File 4: `internal/infrastructure/{memory,messaging,orcaclient}/*_test.go`

```go
func TestSessionStore_UpsertAndGet(t *testing.T)
func TestSessionStore_GetByPlannerTaskID_ReturnsSameSession(t *testing.T)
func TestSessionStore_ListByWorkspace_FiltersCorrectly(t *testing.T)
func TestSessionStore_Delete_RemovesFromBothIndexes(t *testing.T)
func TestSessionStore_ConcurrentUpsertAndList_NoRace(t *testing.T) // go test -race
func TestReloadEventPublisher_PublishesToCorrectChannel(t *testing.T)  // "reload-events:" + workspaceID
func TestReloadEventPublisher_DoesNotMutatePayload(t *testing.T)
func TestOrcaClientAdapter_GetTaskStatus_MapsResponse(t *testing.T)     // httptest.Server
func TestOrcaClientAdapter_GetTaskStatus_ReturnsErrorOnNon200(t *testing.T)
func TestOrcaClientAdapter_HealthCheck_ReturnsNilOn200(t *testing.T)
func TestOrcaClientAdapter_HealthCheck_ReturnsErrorOnNon200(t *testing.T)
```

`TestReloadEventPublisher_PublishesToCorrectChannel` dùng `redis.NewClient` thật trỏ vào Redis test instance (Docker/miniredis), subscribe `reload-events:{workspaceID}` và xác nhận nhận đúng payload — theo đúng cách `skills-service/test/integration_test.go` đã làm (`sub := redisClient.Subscribe(ctx, "reload-events:"+workspaceID)`).

---

## Verification

```bash
cd backend/services/planner-service

go build ./internal/infrastructure/...
go vet ./internal/infrastructure/...
go test ./internal/infrastructure/... -v -race -cover
go test ./internal/infrastructure/... -coverprofile=infra_cov.out
go tool cover -func=infra_cov.out | grep total   # kỳ vọng >= 70%
```
