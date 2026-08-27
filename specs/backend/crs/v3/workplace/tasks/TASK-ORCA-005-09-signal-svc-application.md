> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
## Status: 🔲 NOT STARTED

# TASK-ORCA-005-09 — `planner-service`: Application Layer (`OrcaSessionMonitor`, `OrcaHealthMonitor`, `OrcaTimeoutHandler`, query/command)

**Phase:** 3 — Read-model / observability
**Scope:** ✅ `vnp-workplace` — Go (`planner-service`, module `orcasession`)
**Source:** [SOL-ORCA-005 §3.4–§3.5, §3.7, §3.9–§3.10](../solutions/SOL-ORCA-005-planner-orca-session-monitor.md#34-application-orcasessionmonitor-change-1-cr-port-trực-tiếp--sửa-kind)
**Depends On:** [TASK-ORCA-005-08](./TASK-ORCA-005-08-signal-svc-domain.md)
**Estimated Files:** ~9 files
**Working Dir:** `backend/services/planner-service/internal/application/orcasession/`

> **Đổi tên khỏi bản gốc:** file này giữ nguyên số hiệu/tên (`TASK-ORCA-005-09`, nhắc `signal-svc` trong tên cũ) nhưng nội dung nói về module `orcasession` trong `planner-service`. KHÔNG có NATS, KHÔNG có Postgres, KHÔNG có Temporal trong task này.

---

## Bối cảnh quan trọng

- **⚠️ Field JSON là `"kind"`, KHÔNG PHẢI `"type"`** — xác thực trực tiếp với code thật `diagnostics-service` (`internal/infrastructure/events/redis/subscriber.go`, xem [SOL-ORCA-005 §2.4](../solutions/SOL-ORCA-005-planner-orca-session-monitor.md#24-️-sửa-lỗi-quan-trọng-field-json-là-kind-không-phải-type--xác-thực-với-code-thật-diagnostics-service)). Mọi payload publish trong task này PHẢI dùng field `"kind"` ở top-level. Đây là điểm dễ sai nhất của task — thêm test regression tường minh (`TestOrcaSessionMonitor_Publish_UsesKindFieldNotTypeField`).
- **KHÔNG persist gì cả** — `OrcaSessionMonitor`/`OrcaHealthMonitor` hoạt động hoàn toàn trên `orcasession.SessionStore` in-memory (port từ TASK-ORCA-005-08), implement thật ở TASK-ORCA-005-10. Task này chỉ cần một **fake/in-memory implementation tối giản trong test** (không cần chờ TASK-ORCA-005-10) để unit test.
- `OrcaHealthMonitor` chỉ publish khi **chuyển sang offline** (KHÔNG publish khi hồi phục — khác bản gốc `signal-svc` từng publish cả `orca.instance.online`). Đây là điểm cố ý đơn giản hoá theo đúng CR-ORCA-005 Change 3 ("Publishes `orca.instance.offline`... on state transition only").
- `OrcaTimeoutHandler` chạy **in-process**, được `OrcaSessionMonitor.tick` gọi trực tiếp ngay sau khi publish `orca_session_timeout` — không qua Redis/event bus (khác bản gốc cần Temporal vì `signal-svc`/`temporal-worker` là 2 service tách biệt).
- Route endpoint health check Orca dùng trong task này là `GET /health` theo đúng CR-ORCA-005 Change 3 pseudocode ("Orca GET /health endpoint (already exists in Orca)") — việc xác nhận `/health` hay `/health/ready` là chính xác/khuyến nghị thuộc phạm vi CR-ORCA-001/006, KHÔNG sửa trong task này nếu chưa có SOL-ORCA-001/006 xác nhận khác.

---

## Mục tiêu

Implement port interfaces + `OrcaSessionMonitor` (Change 1 CR) + `OrcaHealthMonitor` (Change 3 CR) + `OrcaTimeoutHandler` (Change 5 CR) + use case `Track`/`Get`/`List`/`Cancel` cho REST API (TASK-ORCA-005-11).

---

## Acceptance Criteria

- [ ] `OrcaSessionMonitor.Track` thêm session mới vào `SessionStore` — gọi bởi module dispatch (CR-ORCA-002, in-process, ngoài phạm vi task này, chỉ cần method public đúng chữ ký)
- [ ] `OrcaSessionMonitor.tick` publish `orca_session_progress` khi progress đổi HOẶC đã quá `CheckInterval` kể từ lần publish trước (không publish khi cả hai đều sai)
- [ ] `OrcaSessionMonitor.tick` publish `orca_session_unreachable` khi `OrcaClientPort.GetTaskStatus` lỗi — KHÔNG update `LastProgress`/`Status` trong trường hợp này
- [ ] `OrcaSessionMonitor.tick` publish `orca_session_timeout` VÀ gọi `OrcaTimeoutHandler.Handle` khi `session.IsTimedOut(now)` — theo đúng thứ tự (publish trước, handler sau, như CR Change 1/5)
- [ ] Mọi payload publish có top-level field `"kind"` (string), KHÔNG có field `"type"` — test tường minh
- [ ] `OrcaHealthMonitor` chỉ publish khi state chuyển `healthy → unhealthy`; KHÔNG publish khi vẫn offline (đã publish trước đó) hoặc khi hồi phục
- [ ] `OrcaTimeoutHandler.Handle` vẫn gọi `TaskRetryPort.HandleFailure` ngay cả khi `OrcaClientPort.CancelTask` lỗi (best-effort cancel, không chặn escalation)
- [ ] `go build ./internal/application/...` thành công
- [ ] `go test ./internal/application/... -v -race -cover` coverage ≥ 80%

---

## File 1: `internal/application/orcasession/port.go`

```go
package orcasession

import (
	"context"
	"time"
)

// OrcaClientPort wraps the shared Orca HTTP client. Concrete implementation
// (TASK-ORCA-005-10) adapts `backend/pkg/orcaclient` — package NOT yet
// confirmed to exist (TASK-ORCA-002-01 not yet rewritten, see SOL-ORCA-005
// §8) — this port lets application-layer code and tests proceed without
// waiting on that.
type OrcaClientPort interface {
	GetTaskStatus(ctx context.Context, orcaTaskID string) (OrcaTaskStatus, error)
	CancelTask(ctx context.Context, orcaTaskID string) error
	HealthCheck(ctx context.Context) error
}

type OrcaTaskStatus struct {
	Status   string
	Progress int
}

// EventPublisherPort publishes a raw JSON payload onto
// reload-events:{workspaceID}. The payload MUST already contain the
// top-level "kind" field (SOL-ORCA-005 §2.4) — this port does not add it,
// callers (monitor.go, health_monitor.go) are responsible.
type EventPublisherPort interface {
	Publish(ctx context.Context, workspaceID string, payload []byte) error
}

// TaskRetryPort forwards a timed-out session to whatever task
// retry/escalation flow exists ("CR-TASK-006" in the original draft CR,
// outside the Orca CR bucket) — reserved integration point, may remain
// unwired (nil) without breaking OrcaTimeoutHandler (see Handle below).
type TaskRetryPort interface {
	HandleFailure(ctx context.Context, plannerTaskID, reason string) error
}

// ActiveDispatchQueryPort is OrcaSessionMonitor.Bootstrap's dependency —
// PLACEHOLDER shape, owned by the dispatch module (CR-ORCA-002, not yet
// rewritten). Do not treat field names as final; confirm against
// SOL-ORCA-002 once it exists.
type ActiveDispatchQueryPort interface {
	ListActiveOrcaDispatches(ctx context.Context) ([]ActiveDispatch, error)
}

type ActiveDispatch struct {
	OrcaTaskID, PlannerTaskID, WorkspaceID string
	StartedAt, TimeoutAt                   time.Time
}

// DispatchCancelPort is CancelSession's dependency (§3.10 SOL) — the
// dispatch module owns the actual OrcaClient.CancelTask call, this
// application layer only signals intent in-process (no Temporal, no
// network signal — same process now).
type DispatchCancelPort interface {
	CancelDispatch(ctx context.Context, plannerTaskID string) error
}
```

---

## File 2: `internal/application/orcasession/monitor.go`

```go
package orcasession

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	domain "github.com/vnptech/kwp/services/planner-service/internal/domain/orcasession"
)

// OrcaSessionMonitor polls all actively-tracked Orca sessions and publishes
// progress/timeout events onto reload-events:{workspace_id} so
// diagnostics-service can fan them out over SSE. Holds no persisted state —
// SessionStore is purely in-memory (SOL-ORCA-005 §2.2).
type OrcaSessionMonitor struct {
	Store          domain.SessionStore
	OrcaClient     OrcaClientPort
	Publisher      EventPublisherPort
	TimeoutHandler *OrcaTimeoutHandler // may be nil in tests exercising only publish behavior
	CheckInterval  time.Duration       // 60s, per CR-ORCA-005 Change 1
	Logger         *slog.Logger
}

// Track registers a new session — called in-process by the sibling
// dispatch module (CR-ORCA-002) right after it submits a task to Orca.
// Replaces the old signal-svc "orca.task.submitted" NATS event (no longer
// needed: same process now).
func (m *OrcaSessionMonitor) Track(orcaTaskID, plannerTaskID, workspaceID string, startedAt, timeoutAt time.Time) {
	m.Store.Upsert(domain.NewOrcaSession(orcaTaskID, plannerTaskID, workspaceID, startedAt, timeoutAt))
}

// Bootstrap reloads active sessions after a process restart from the
// dispatch module's own persisted state (SOL-ORCA-005 §2.2.4) — exact port
// signature is an Open Task pending SOL-ORCA-002's rewrite.
func (m *OrcaSessionMonitor) Bootstrap(ctx context.Context, dispatchQuery ActiveDispatchQueryPort) error {
	active, err := dispatchQuery.ListActiveOrcaDispatches(ctx)
	if err != nil {
		return fmt.Errorf("bootstrapping orca session monitor: %w", err)
	}
	for _, d := range active {
		m.Track(d.OrcaTaskID, d.PlannerTaskID, d.WorkspaceID, d.StartedAt, d.TimeoutAt)
	}
	return nil
}

func (m *OrcaSessionMonitor) Run(ctx context.Context) {
	ticker := time.NewTicker(m.CheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.tick(ctx)
		}
	}
}

func (m *OrcaSessionMonitor) tick(ctx context.Context) {
	now := time.Now().UTC()
	for _, session := range m.Store.List() {
		if session.Status.IsTerminal() {
			m.Store.Delete(session.OrcaTaskID)
			continue
		}

		status, err := m.OrcaClient.GetTaskStatus(ctx, session.OrcaTaskID)
		if err != nil {
			m.publish(ctx, session.WorkspaceID, domain.EventOrcaSessionUnreachable, orcaUnreachablePayload{
				OrcaTaskID: session.OrcaTaskID, PlannerTaskID: session.PlannerTaskID,
			})
			continue
		}

		changed := session.RecordProgress(domain.SessionStatus(status.Status), status.Progress, now)
		if changed || session.DueForHeartbeat(now, m.CheckInterval) {
			session.MarkPublished(now)
			m.publish(ctx, session.WorkspaceID, domain.EventOrcaSessionProgress, orcaProgressPayload{
				PlannerTaskID: session.PlannerTaskID, Status: status.Status, Progress: status.Progress,
			})
		}

		if session.IsTimedOut(now) {
			session.MarkTimedOut(now)
			m.publish(ctx, session.WorkspaceID, domain.EventOrcaSessionTimeout, orcaTimeoutPayload{
				OrcaTaskID: session.OrcaTaskID, PlannerTaskID: session.PlannerTaskID,
				Duration: now.Sub(session.StartedAt).String(),
			})
			if m.TimeoutHandler != nil {
				if err := m.TimeoutHandler.Handle(ctx, session.OrcaTaskID, session.PlannerTaskID, now.Sub(session.StartedAt)); err != nil {
					m.Logger.Error("orca timeout handler failed", "orca_task_id", session.OrcaTaskID, "err", err)
				}
			}
		}
		m.Store.Upsert(session)
	}
}

// publish marshals {"kind": ..., "data": ...} and sends it on
// reload-events:{workspaceID}. The field name is "kind", NOT "type" — see
// SOL-ORCA-005 §2.4 for why this matters (diagnostics-service's real
// Watcher only lifts "kind" into sse.SSEEvent.Type).
func (m *OrcaSessionMonitor) publish(ctx context.Context, workspaceID string, kind domain.EventKind, payload any) {
	data, err := json.Marshal(struct {
		Kind string `json:"kind"`
		Data any    `json:"data"`
	}{Kind: string(kind), Data: payload})
	if err != nil {
		m.Logger.Error("orca session monitor: marshal failed", "kind", kind, "err", err)
		return
	}
	if err := m.Publisher.Publish(ctx, workspaceID, data); err != nil {
		m.Logger.Error("orca session monitor: publish failed", "workspace_id", workspaceID, "kind", kind, "err", err)
	}
}

type orcaProgressPayload struct {
	PlannerTaskID string `json:"planner_task_id"`
	Status        string `json:"status"`
	Progress      int    `json:"progress"`
}
type orcaUnreachablePayload struct {
	OrcaTaskID    string `json:"orca_task_id"`
	PlannerTaskID string `json:"planner_task_id"`
}
type orcaTimeoutPayload struct {
	OrcaTaskID    string `json:"orca_task_id"`
	PlannerTaskID string `json:"planner_task_id"`
	Duration      string `json:"duration"`
}
```

---

## File 3: `internal/application/orcasession/health_monitor.go`

```go
package orcasession

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	domain "github.com/vnptech/kwp/services/planner-service/internal/domain/orcasession"
)

// OrcaHealthMonitor checks Orca liveness independently of any session and
// publishes ONLY when transitioning healthy -> unhealthy (avoid alert
// spam) — CR-ORCA-005 Change 3 does not publish on recovery, unlike the
// pre-rewrite signal-svc design which published both directions.
type OrcaHealthMonitor struct {
	Client           OrcaClientPort
	Publisher        EventPublisherPort
	ActiveWorkspaces func() []string // workspaces with at least 1 tracked session — derived from SessionStore
	Interval         time.Duration   // 30s
	Logger           *slog.Logger

	mu          sync.Mutex
	lastHealthy *bool
}

func (m *OrcaHealthMonitor) Run(ctx context.Context) {
	ticker := time.NewTicker(m.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.tick(ctx)
		}
	}
}

func (m *OrcaHealthMonitor) tick(ctx context.Context) {
	err := m.Client.HealthCheck(ctx)
	healthy := err == nil

	m.mu.Lock()
	changed := m.lastHealthy == nil || *m.lastHealthy != healthy
	m.lastHealthy = &healthy
	m.mu.Unlock()

	if !changed || healthy {
		return // only alert when transitioning TO offline
	}

	payload, marshalErr := json.Marshal(struct {
		Kind string `json:"kind"`
		Data any    `json:"data"`
	}{Kind: string(domain.EventOrcaInstanceOffline), Data: map[string]string{"error": errString(err)}})
	if marshalErr != nil {
		m.Logger.Error("health monitor: marshal failed", "err", marshalErr)
		return
	}
	// No unsuffixed "global" channel in diagnostics-service's Watcher (it
	// parses workspace_id from the channel-name suffix) — publish once per
	// active workspace instead.
	for _, ws := range m.ActiveWorkspaces() {
		if pubErr := m.Publisher.Publish(ctx, ws, payload); pubErr != nil {
			m.Logger.Error("health monitor: publish failed", "workspace_id", ws, "err", pubErr)
		}
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
```

---

## File 4: `internal/application/orcasession/timeout_handler.go`

```go
package orcasession

import (
	"context"
	"fmt"
	"time"
)

// OrcaTimeoutHandler runs in-process, called directly from
// OrcaSessionMonitor.tick right after it publishes orca_session_timeout —
// no Redis/event bus round-trip needed since both live in the same
// planner-service process (unlike the old signal-svc/temporal-worker
// split, which needed a cross-service Temporal Signal for this).
type OrcaTimeoutHandler struct {
	OrcaClient       OrcaClientPort
	TaskRetryUseCase TaskRetryPort // may be nil — see Handle
}

// Handle cancels the Orca task (best-effort — a session that timed out is
// often ALREADY unreachable, so a CancelTask error here does not stop
// escalation) and forwards to task retry/escalation. If TaskRetryUseCase is
// nil (not wired yet — CR-TASK-006 is outside the Orca CR bucket), Handle
// is a no-op after the best-effort cancel.
func (h *OrcaTimeoutHandler) Handle(ctx context.Context, orcaTaskID, plannerTaskID string, duration time.Duration) error {
	if err := h.OrcaClient.CancelTask(ctx, orcaTaskID); err != nil {
		_ = err // best-effort, do not fail the handler on cancel error
	}
	if h.TaskRetryUseCase == nil {
		return nil
	}
	return h.TaskRetryUseCase.HandleFailure(ctx, plannerTaskID,
		fmt.Sprintf("Orca session timeout after %s", duration))
}
```

---

## File 5: `internal/application/orcasession/get_session.go` + `list_sessions.go`

```go
// internal/application/orcasession/get_session.go
package orcasession

import (
	"context"

	domain "github.com/vnptech/kwp/services/planner-service/internal/domain/orcasession"
)

type GetSessionQuery struct {
	Store domain.SessionStore
}

func (q *GetSessionQuery) Handle(ctx context.Context, plannerTaskID string) (*domain.OrcaSession, error) {
	s, ok := q.Store.GetByPlannerTaskID(plannerTaskID)
	if !ok {
		return nil, domain.ErrNotFound
	}
	return s, nil
}
```

```go
// internal/application/orcasession/list_sessions.go
package orcasession

import (
	"context"

	domain "github.com/vnptech/kwp/services/planner-service/internal/domain/orcasession"
)

type ListSessionsQuery struct {
	Store domain.SessionStore
}

func (q *ListSessionsQuery) Handle(ctx context.Context, workspaceID string) ([]*domain.OrcaSession, error) {
	if workspaceID == "" {
		return q.Store.List(), nil
	}
	return q.Store.ListByWorkspace(workspaceID), nil
}
```

---

## File 6: `internal/application/orcasession/cancel_session.go`

```go
package orcasession

import "context"

// CancelSessionCommand does NOT call OrcaClientPort.CancelTask directly
// (SOL-ORCA-005 §3.10) — it forwards to DispatchCancelPort so the dispatch
// module (CR-ORCA-002, owner of the task's lifecycle) decides how/when to
// actually cancel in Orca, avoiding a race between two independent cancel
// paths. This replaces the pre-rewrite Temporal Signal mechanism with a
// plain in-process call — same process now, no network hop needed.
type CancelSessionCommand struct {
	Dispatch DispatchCancelPort
}

func (c *CancelSessionCommand) Handle(ctx context.Context, plannerTaskID string) error {
	return c.Dispatch.CancelDispatch(ctx, plannerTaskID)
}
```

> **Ghi chú tích hợp:** `DispatchCancelPort.CancelDispatch` chưa có implementation thật (module dispatch, CR-ORCA-002, chưa viết lại) — chữ ký ở đây là placeholder. Unit test của `CancelSessionCommand` dùng fake implementation, không chặn bởi CR-ORCA-002.

---

## Test File 7: `internal/application/orcasession/*_test.go`

```go
func TestOrcaSessionMonitor_Track_AddsToStore(t *testing.T)
func TestOrcaSessionMonitor_Tick_PublishesOnProgressChange(t *testing.T)
func TestOrcaSessionMonitor_Tick_PublishesOnHeartbeatEvenWithoutChange(t *testing.T)
func TestOrcaSessionMonitor_Tick_DoesNotPublish_WhenNoChangeAndNotDue(t *testing.T)
func TestOrcaSessionMonitor_Tick_PublishesUnreachable_OnClientError_DoesNotUpdateProgress(t *testing.T)
func TestOrcaSessionMonitor_Tick_PublishesTimeout_AndCallsTimeoutHandler(t *testing.T)
func TestOrcaSessionMonitor_Tick_RemovesTerminalSessionFromStore(t *testing.T)
func TestOrcaSessionMonitor_Publish_UsesKindFieldNotTypeField(t *testing.T) // regression, SOL §2.4
func TestOrcaSessionMonitor_Bootstrap_LoadsActiveDispatches(t *testing.T)
func TestOrcaSessionMonitor_Bootstrap_PropagatesQueryError(t *testing.T)
func TestOrcaHealthMonitor_PublishesOffline_OnlyOnTransitionToUnhealthy(t *testing.T)
func TestOrcaHealthMonitor_DoesNotPublish_WhenStillOffline(t *testing.T)
func TestOrcaHealthMonitor_DoesNotPublish_OnRecovery(t *testing.T) // CR Change 3: offline-only alert
func TestOrcaHealthMonitor_PublishesOncePerActiveWorkspace(t *testing.T)
func TestOrcaTimeoutHandler_CancelsOrcaTask_AndCallsTaskRetry(t *testing.T)
func TestOrcaTimeoutHandler_StillCallsRetry_WhenCancelFails(t *testing.T)
func TestOrcaTimeoutHandler_NoopWhenTaskRetryUseCaseNil(t *testing.T)
func TestGetSessionQuery_ReturnsErrNotFound_WhenMissing(t *testing.T)
func TestListSessionsQuery_FiltersByWorkspaceWhenProvided(t *testing.T)
func TestCancelSessionCommand_ForwardsToDispatchCancelPort_NotOrcaClientDirectly(t *testing.T)
```

---

## Verification

```bash
cd backend/services/planner-service

go build ./internal/application/...
go vet ./internal/application/...
go test ./internal/application/... -v -race -cover
go test ./internal/application/... -coverprofile=app_cov.out
go tool cover -func=app_cov.out | grep total   # kỳ vọng >= 80%
```
