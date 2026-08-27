> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
# SOL-ORCA-005 — Planner Session Monitor (`planner-service` module `orcasession` + `diagnostics-service` tái dùng)

| Field | Value |
|-------|-------|
| **CR ref** | [CR-ORCA-005](../../../../../../docs/crs/v3/orca/CR-ORCA-005-planner-orca-session-monitor.md) |
| **Title** | Planner Session Monitor — theo dõi real-time phiên thực thi Orca |
| **Service** | `planner-service` (`:3013`, module `internal/*/orcasession`, **MỚI**) + `diagnostics-service` (`:3010`, **tái dùng nguyên trạng, không sửa code**) |
| **Priority** | P1 |
| **Risk** | medium |
| **Status** | 📐 PROPOSED |
| **TDD refs** | [`backend/specs/tdd/v3/25-agent-diagnostics-service.md`](../../../../tdd/v3/25-agent-diagnostics-service.md) §4, §8 (DIAG-03), [`backend/specs/tdd/v1/00-go-conventions.md`](../../../../tdd/v1/00-go-conventions.md), [`01-project-structure.md`](../../../../tdd/v1/01-project-structure.md), [`02-database.md`](../../../../tdd/v1/02-database.md) |
| **Depends on** | SOL-ORCA-002 (`planner-service` dispatch module), SOL-ORCA-004 (`planner-service` callback module) — **cả hai CHƯA được viết lại theo kiến trúc `vnp-workplace` tại thời điểm SOL này được cập nhật** (2026-08-10); mọi tham chiếu tới chúng dưới đây là **giả định phối hợp**, không phải hợp đồng đã khoá — xem §8. |
| **Re-scope** | Bản gốc của SOL này (`signal-svc`, `vnp-planner`, Postgres read-model event-driven qua NATS, Temporal Signal cho cancel) **bị thay thế toàn bộ** theo quyết định kiến trúc 2026-08-10 (xem [`docs/crs/v3/orca/README.md`](../../../../../../docs/crs/v3/orca/README.md) §Ghi chú Re-scope). Không còn `signal-svc`, không còn NATS cho luồng này, không còn Temporal. |

---

## 1. Tóm tắt vấn đề & mục tiêu

CR-ORCA-005 (đã viết lại) yêu cầu: (1) live visibility tiến độ Orca session, (2) timeout detection, (3) progress streaming real-time tới WKP admin UI, (4) Orca health check — **không** tạo service mới, **không** dùng Temporal.

Quyết định kiến trúc đã chốt trong CR (không còn là câu hỏi mở của SOL này nữa):

- **Business logic theo dõi session** (poll Orca, so sánh progress, phát hiện timeout, health check, forward retry/escalation) → module **`orcasession`** bên trong **`planner-service`** (`:3013`) — cùng service với dispatch (CR-ORCA-002) và callback (CR-ORCA-004), vì đây là domain logic thuộc vòng đời task của planner.
- **Fan-out real-time tới client** → **tái dùng nguyên trạng** `diagnostics-service` (`:3010`, đã build thật trong phiên trước — xem TDD-25) qua cơ chế **Reload Events SSE Stream (DIAG-03)** đã có sẵn: Redis pub/sub `reload-events:{workspace_id}` → Ring Buffer + Broadcaster → `GET /api/v1/diagnostics/events`. `diagnostics-service` chỉ đóng vai trò subscriber/fan-out có sẵn; `planner-service` là publisher mới vào đúng channel pattern đã tồn tại.

Mục tiêu của SOL này: (1) thiết kế module `orcasession` (domain/application/infrastructure/interface) theo Clean Architecture chuẩn của monorepo (`backend/specs/tdd/v1/01-project-structure.md`), (2) khoá chính xác **shape JSON** publish lên `reload-events:{workspace_id}` bằng cách đối chiếu **code thật** của `diagnostics-service` (không phải giả định từ CR pseudocode — xem §2.4, phát hiện quan trọng), (3) mô tả rõ ranh giới với 2 module song song trong cùng `planner-service` (dispatch — CR-ORCA-002, callback — CR-ORCA-004) mà **chưa** có SOL viết lại để tham chiếu chi tiết.

> **Phạm vi KHÔNG thuộc SOL này:** thiết kế nội bộ của module dispatch (`OrcaDispatch`/tương đương CR-ORCA-002) và module callback (CR-ORCA-004) trong `planner-service` — SOL này chỉ định nghĩa **contract phối hợp** (port interface) mà module `orcasession` cần từ chúng, đánh dấu rõ là Open Task ở mỗi điểm chạm.

---

## 2. Quyết định kiến trúc

### 2.1 Vị trí: module `orcasession`, không còn tranh chấp bounded-context

SOL gốc (`signal-svc`) phải cân nhắc "nhét `OrcaSession` vào `signal-svc` sẵn có (đã có `MarketSignal`) hay tách service riêng" vì `signal-svc` là service **đã tồn tại với domain khác** (Market Signal). Vấn đề đó **không còn tồn tại**: `planner-service` là service **hoàn toàn mới**, chưa có aggregate nào để xung đột. `orcasession` chỉ cần tách package sạch với 2 module anh em (`dispatch`, `callback` — do CR-ORCA-002/004 định nghĩa) theo đúng khuôn `internal/domain/{entity}/`, `internal/application/{feature}/` của TDD-01 — không cần bảng so sánh Option A/B như bản gốc.

### 2.2 Trạng thái session: in-memory, KHÔNG Postgres — khác biệt lớn nhất so với bản gốc

CR-ORCA-005 Change 1 định nghĩa `OrcaSessionMonitor` với `sessions map[string]*OrcaSession` **in-memory**, không có `Repository`/`Postgres` nào trong toàn bộ 5 Change của CR. Đây là chủ đích, không phải thiếu sót — SOL này **giữ nguyên quyết định đó** thay vì phục hồi Postgres read-model (`orca_sessions` table + NATS event-sourcing) như bản gốc `signal-svc`, vì lý do đã đổi:

1. **Không còn ranh giới service** giữa dispatch và monitor — cả hai giờ chạy trong cùng process `planner-service`. Bản gốc cần Postgres + NATS vì `signal-svc` và `temporal-worker` là 2 service độc lập, cần đồng bộ qua sự kiện bền vững. Nay `orcasession` có thể được module dispatch **gọi hàm trực tiếp** (in-process) khi submit task — không cần event bus để tránh mất đồng bộ.
2. **Nguồn sự thật nghiệp vụ vẫn là bản ghi dispatch** (do module dispatch — CR-ORCA-002 — sở hữu và persist, tên bảng/entity chính xác chưa chốt vì SOL-ORCA-002 chưa viết lại). Một bảng `orca_sessions` riêng, song song, sẽ tái tạo đúng rủi ro "2 nguồn sự thật lệch nhau" mà SOL gốc từng cảnh báo (§2 bản gốc) — nay tránh được hoàn toàn bằng cách **không** tạo bảng thứ hai.
3. **Chấp nhận mất trạng thái tracking khi restart**: nếu `planner-service` restart, module `orcasession` mất `LastProgress`/`LastPublishAt` (throttle state) — hệ quả tối đa là 1 lần publish dư ở tick kế tiếp, không phải mất dữ liệu nghiệp vụ (dữ liệu nghiệp vụ thật vẫn nằm ở module dispatch, persisted). Trade-off này được ghi nhận rõ ràng, không phải lỗi.
4. **Bootstrap sau restart**: `OrcaSessionMonitor` cần một bước `Bootstrap(ctx)` gọi vào module dispatch (qua port `ActiveDispatchQueryPort`, xem §3.5) để nạp lại danh sách session đang `IN_PROGRESS` — đây là **Open Task phối hợp** với CR-ORCA-002 (chưa thể khoá chữ ký chính xác vì SOL-ORCA-002 chưa viết lại).

**Hệ quả cho TASK-ORCA-005-10:** KHÔNG có migration `orca_sessions`, KHÔNG có `PostgresOrcaSessionRepository` — thay bằng `SessionStore` in-memory (map + mutex), implement trong `internal/infrastructure/memory/`.

### 2.3 Kênh fan-out: tái dùng `reload-events:{workspace_id}`

Giữ nguyên rationale đã chốt trong CR (không lặp lại toàn văn, xem [CR-ORCA-005 §Design Decision](../../../../../../docs/crs/v3/orca/CR-ORCA-005-planner-orca-session-monitor.md#design-decision--redis-channel-tái-dùng-reload-eventsworkspace_id)):

- `diagnostics-service.Watcher` chỉ `PSubscribe("reload-events:*")`, tách `workspace_id` từ **suffix của tên channel**, không có khái niệm `task_id` hay channel riêng — publish theo `reload-events:{workspace_id}`, đặt `planner_task_id`/`orca_task_id` **bên trong payload**, không làm key channel.
- `orca.instance.offline` (không gắn 1 workspace cụ thể) publish lặp lại cho từng workspace đang có session active (không có channel "global" không-suffix trong `Watcher` hiện tại).
- Đánh đổi: Ring Buffer chỉ giữ 50 event/workspace (dùng chung với `skill_updated`/`plugin_installed`/... của các publisher khác) → publish progress có throttle (đổi thật hoặc tối đa 1 lần/60s/session), theo đúng Change 1 gốc.

### 2.4 ⚠️ Sửa lỗi quan trọng: field JSON là `"kind"`, KHÔNG PHẢI `"type"` — xác thực với code thật `diagnostics-service`

CR-ORCA-005 Change 1/3 (pseudocode) publish payload dạng `map[string]any{"type": eventType, "data": payload}`. Đối chiếu với code **thật** đã build:

```go
// backend/services/diagnostics-service/internal/infrastructure/events/redis/subscriber.go (dòng 30-36, 84-87)
type reloadEventEnvelope struct {
    Kind string `json:"kind"`
}
// ...
var envelope reloadEventEnvelope
if json.Unmarshal([]byte(msg.Payload), &envelope) == nil && envelope.Kind != "" {
    event.Type = sse.EventType(envelope.Kind)
}
```

`Watcher.handleMessage` chỉ trích xuất field **`"kind"`** (không phải `"type"`) từ payload JSON vào `sse.SSEEvent.Type` — field này sau đó được `sse_handler.go`'s `writeSSEEvent` ghi thành dòng `event: <kind>\n` trong wire format SSE (cho phép client dùng `EventSource.addEventListener("orca_session_progress", ...)` thay vì phải tự parse `data:` JSON). Đây chính xác là pattern **thật, đã chạy, có test pass** ở `skills-service`/`plugin-registry`:

```go
// backend/services/skills-service/internal/infrastructure/messaging/reload_event_publisher.go (dòng 17-24, 46-53)
type ReloadEvent struct {
    WorkspaceID   string    `json:"workspace_id"`
    Kind          string    `json:"kind"` // "skill_updated"
    ...
}
```

Nếu `planner-service` publish với key `"type"` như pseudocode CR, `Watcher` sẽ **không** lift được vào `event.Type` — payload JSON vẫn tới client (vì `Data` forward nguyên payload, không phụ thuộc parse thành công), nhưng client mất khả năng lọc theo SSE `event:` field và phải tự parse JSON body 100% thủ công (khác pattern đồng nhất với các publisher khác trên cùng channel). **Sửa: dùng `"kind"`** trong mọi payload publish của `orcasession` module — áp dụng xuyên suốt §3.5, §3.7, và TASK-ORCA-005-09/10.

---

## 3. Kiến trúc giải pháp

### 3.1 Cấu trúc thư mục (module mới trong `planner-service`, service chưa tồn tại — tạo cùng lúc với CR-ORCA-002)

```
services/planner-service/
├── go.mod                                                 module github.com/vnptech/kwp/services/planner-service
├── cmd/server/main.go                                     [MODIFY, phối hợp CR-ORCA-002/004] wiring orcasession
├── config/config.go                                       [MODIFY] + Orca session/health config (§3.9)
├── internal/
│   ├── domain/
│   │   ├── dispatch/                                      (CR-ORCA-002 — ngoài phạm vi SOL này)
│   │   ├── callback/                                      (CR-ORCA-004 — ngoài phạm vi SOL này)
│   │   └── orcasession/
│   │       ├── entity.go            [NEW] — OrcaSession, SessionStatus
│   │       ├── errors.go            [NEW]
│   │       ├── store.go             [NEW] — SessionStore port (in-memory, KHÔNG Postgres, xem §2.2)
│   │       └── event_kind.go        [NEW] — hằng "kind" publish lên reload-events (§2.4)
│   ├── application/
│   │   └── orcasession/
│   │       ├── port.go              [NEW] — OrcaClientPort, EventPublisherPort, TaskRetryPort, ActiveDispatchQueryPort
│   │       ├── monitor.go           [NEW] — OrcaSessionMonitor (Change 1 CR)
│   │       ├── health_monitor.go    [NEW] — OrcaHealthMonitor (Change 3 CR)
│   │       ├── timeout_handler.go   [NEW] — OrcaTimeoutHandler (Change 5 CR)
│   │       ├── track_session.go     [NEW] — entrypoint gọi bởi module dispatch khi submit task
│   │       ├── get_session.go       [NEW]
│   │       ├── list_sessions.go     [NEW]
│   │       └── cancel_session.go    [NEW]
│   ├── infrastructure/
│   │   ├── memory/
│   │   │   └── session_store.go               [NEW] — SessionStore impl (map + sync.RWMutex)
│   │   ├── messaging/
│   │   │   └── reload_event_publisher.go       [NEW] — tái dùng NGUYÊN pattern skills-service/plugin-registry
│   │   └── orcaclient/
│   │       └── adapter.go                      [NEW] — wrap `pkg/orcaclient` (CR-ORCA-002, coordination — §8)
│   └── presentation/http/
│       ├── router.go                           [MODIFY] + 5 route (§3.8)
│       └── handler/
│           ├── orca_session_handler.go         [NEW]
│           ├── orca_health_handler.go          [NEW]
│           └── orca_trace_handler.go           [NEW] — on-demand SSE pass-through (§3.6)
```

### 3.2 Domain: `OrcaSession` — đơn giản hoá so với bản gốc (không event-sourcing)

Bản gốc (`signal-svc`) dùng pattern `record()`/`PopEvents()` (domain event nội bộ) vì cần publish `DomainEvent` ra NATS cho service khác consume. Không còn NATS, không còn ranh giới service → domain entity **không cần thu event nội bộ nữa**; application layer (`monitor.go`) gọi thẳng `EventPublisherPort.Publish` sau khi state mutate xong (giống pattern thật `skills-service`: `uc.reload.Publish(ctx, s.WorkspaceID, port.ReloadEvent{...})` gọi trực tiếp trong use case, không qua domain event queue — xem `internal/application/skill/update_skill.go`).

```go
// internal/domain/orcasession/entity.go
package orcasession

import "time"

// SessionStatus mirrors Orca's own OrcaTask status string verbatim
// (README §Orca Key APIs: "OrcaTask — task với status: backlog → in_progress
// → review → done") PLUS 2 synthetic statuses set only by planner-service
// itself when it — not Orca — decides the session is over.
//
// CHƯA XÁC THỰC: giá trị "review"/"backlog" là suy luận từ README tổng quan
// CR-ORCA, KHÔNG phải từ response schema thật của GetTaskStatus (thuộc phạm
// vi SOL-ORCA-001/002, chưa viết lại). Nếu SOL-ORCA-002 khoá schema khác,
// cập nhật type này theo — KHÔNG coi đây là hợp đồng đã khoá.
type SessionStatus string

const (
	StatusInProgress SessionStatus = "in_progress"
	StatusReview      SessionStatus = "review"
	StatusDone        SessionStatus = "done"
	StatusBacklog     SessionStatus = "backlog"
	StatusCancelled   SessionStatus = "cancelled"   // đặt bởi CancelSession, không phải Orca trả về
	StatusTimedOut    SessionStatus = "timed_out"   // đặt bởi OrcaSessionMonitor khi TimeoutAt qua
	StatusUnreachable SessionStatus = "unreachable" // KHÔNG đổi Status thật — chỉ cờ tạm thời trên bản ghi giám sát, xem MarkUnreachable
)

// IsTerminal reports whether no further polling/timeout tracking is needed.
func (s SessionStatus) IsTerminal() bool {
	switch s {
	case StatusDone, StatusCancelled, StatusTimedOut:
		return true
	}
	return false
}

// OrcaSession is a lightweight, IN-MEMORY tracking record for one Orca
// AI-agent execution session — used ONLY by OrcaSessionMonitor for polling,
// progress-change detection and publish throttling (SOL-ORCA-005 §2.2). It
// is NOT the business source of truth for the task/dispatch (that lives in
// the sibling `dispatch` module owned by CR-ORCA-002) and is NOT persisted —
// safe to lose on restart (Bootstrap reloads active sessions, §3.5).
type OrcaSession struct {
	OrcaTaskID    string
	PlannerTaskID string
	WorkspaceID   string // needed to pick the right reload-events:{workspace_id} channel
	Status        SessionStatus
	LastProgress  int
	StartedAt     time.Time
	LastSeenAt    time.Time
	LastPublishAt time.Time
	TimeoutAt     time.Time
}

func NewOrcaSession(orcaTaskID, plannerTaskID, workspaceID string, startedAt, timeoutAt time.Time) *OrcaSession {
	return &OrcaSession{
		OrcaTaskID: orcaTaskID, PlannerTaskID: plannerTaskID, WorkspaceID: workspaceID,
		Status: StatusInProgress, StartedAt: startedAt, LastSeenAt: startedAt, TimeoutAt: timeoutAt,
	}
}

// RecordProgress updates LastSeenAt/LastProgress and reports whether the
// progress value actually changed — the caller (monitor.go) uses this,
// combined with a heartbeat interval check, to decide whether to publish.
func (s *OrcaSession) RecordProgress(status SessionStatus, progress int, now time.Time) (changed bool) {
	s.LastSeenAt = now
	s.Status = status
	changed = progress != s.LastProgress
	s.LastProgress = progress
	return changed
}

func (s *OrcaSession) MarkPublished(now time.Time) { s.LastPublishAt = now }

// DueForHeartbeat reports whether at least `interval` has passed since the
// last publish — used to force a publish even without a progress change
// (Change 1 CR: "tối đa 1 lần/60s/session").
func (s *OrcaSession) DueForHeartbeat(now time.Time, interval time.Duration) bool {
	return now.Sub(s.LastPublishAt) >= interval
}

// IsTimedOut reports whether TimeoutAt has passed and the session has not
// already reached a terminal status.
func (s *OrcaSession) IsTimedOut(now time.Time) bool {
	return !s.Status.IsTerminal() && now.After(s.TimeoutAt)
}

func (s *OrcaSession) MarkTimedOut(now time.Time) { s.Status = StatusTimedOut; s.LastSeenAt = now }
func (s *OrcaSession) MarkCancelled(now time.Time) { s.Status = StatusCancelled; s.LastSeenAt = now }
```

```go
// internal/domain/orcasession/errors.go
package orcasession

import "errors"

var (
	ErrNotFound      = errors.New("orcasession: not found")
	ErrAlreadyTracked = errors.New("orcasession: already tracked")
)
```

```go
// internal/domain/orcasession/event_kind.go
//
// EventKind values published as the top-level "kind" field on
// reload-events:{workspace_id} (SOL-ORCA-005 §2.4). These are LOCAL string
// constants — orcasession does NOT import diagnostics-service's
// internal/domain/sse package (different service, different go.mod;
// cross-service internal-package import is architecturally invalid).
// String VALUES must stay byte-for-byte in sync with the 5 consts requested
// as an Open Task in diagnostics-service's internal/domain/sse/entity.go
// (CR-ORCA-005 §Design Decision "Yêu cầu phối hợp") — that file is NOT
// modified by this SOL/TASK set.
package orcasession

type EventKind string

const (
	EventOrcaSessionProgress    EventKind = "orca_session_progress"
	EventOrcaSessionTimeout     EventKind = "orca_session_timeout"
	EventOrcaSessionUnreachable EventKind = "orca_session_unreachable"
	EventOrcaSessionCompleted   EventKind = "orca_session_completed"
	EventOrcaInstanceOffline    EventKind = "orca_instance_offline"
)
```

### 3.3 `SessionStore` — port in-memory (thay repository Postgres)

```go
// internal/domain/orcasession/store.go
package orcasession

// SessionStore is an in-memory tracking store (SOL-ORCA-005 §2.2) — NOT a
// persistence port. Implemented by internal/infrastructure/memory
// (map + sync.RWMutex); no Postgres implementation exists for this
// interface, unlike other domain repository ports in this monorepo.
type SessionStore interface {
	Upsert(s *OrcaSession)
	Get(orcaTaskID string) (*OrcaSession, bool)
	GetByPlannerTaskID(plannerTaskID string) (*OrcaSession, bool)
	List() []*OrcaSession
	ListByWorkspace(workspaceID string) []*OrcaSession
	Delete(orcaTaskID string)
}
```

### 3.4 Application: `OrcaSessionMonitor` (Change 1 CR, port trực tiếp + sửa `"kind"`)

```go
// internal/application/orcasession/monitor.go
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
// diagnostics-service can fan them out over SSE (SOL-ORCA-005 §1). It does
// NOT persist anything (§2.2) — sessions live only in the injected
// domain.SessionStore for the lifetime of this process.
type OrcaSessionMonitor struct {
	Store         domain.SessionStore
	OrcaClient    OrcaClientPort
	Publisher     EventPublisherPort
	TimeoutHandler *OrcaTimeoutHandler
	CheckInterval time.Duration // 60s, per CR Change 1
	Logger        *slog.Logger
}

// Track registers a new session — called in-process by the sibling dispatch
// module (CR-ORCA-002) right after it submits a task to Orca. Replaces the
// old signal-svc "orca.task.submitted" NATS event (no longer needed: same
// process now).
func (m *OrcaSessionMonitor) Track(orcaTaskID, plannerTaskID, workspaceID string, startedAt, timeoutAt time.Time) {
	m.Store.Upsert(domain.NewOrcaSession(orcaTaskID, plannerTaskID, workspaceID, startedAt, timeoutAt))
}

// Bootstrap reloads active sessions after a restart from the dispatch
// module's own persisted state (SOL-ORCA-005 §2.2.4) — exact port signature
// is an Open Task pending SOL-ORCA-002's rewrite; ActiveDispatchQueryPort
// below is a placeholder shape.
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
		// Publish on real change OR at most once per CheckInterval — avoids
		// flooding the 50-event/workspace ring buffer shared with other
		// publishers (SOL-ORCA-005 §2.3).
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
// reload-events:{workspaceID} — the field name is "kind", NOT "type" (see
// SOL-ORCA-005 §2.4 for why this matters).
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

### 3.5 Ports (`port.go`)

```go
// internal/application/orcasession/port.go
package orcasession

import (
	"context"
	"time"
)

// OrcaClientPort wraps the shared Orca HTTP client (CR-ORCA-002 —
// backend/pkg/orcaclient, path/package NOT yet confirmed since
// TASK-ORCA-002-01 has not been rewritten — see SOL-ORCA-005 §8).
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
// reload-events:{workspaceID} — the payload MUST already contain the
// top-level "kind" field (SOL-ORCA-005 §2.4); this port does not add it.
type EventPublisherPort interface {
	Publish(ctx context.Context, workspaceID string, payload []byte) error
}

// TaskRetryPort forwards a timed-out session to whatever task
// retry/escalation flow exists (CR references "CR-TASK-006" in the original
// draft — outside the Orca CR bucket; kept as a reserved integration point,
// same as the pre-rewrite SOL-ORCA-005).
type TaskRetryPort interface {
	HandleFailure(ctx context.Context, plannerTaskID, reason string) error
}

// ActiveDispatchQueryPort is the Bootstrap() dependency described in §3.4 —
// PLACEHOLDER shape, owned by the dispatch module (CR-ORCA-002). Do not
// treat field names below as final.
type ActiveDispatchQueryPort interface {
	ListActiveOrcaDispatches(ctx context.Context) ([]ActiveDispatch, error)
}

type ActiveDispatch struct {
	OrcaTaskID, PlannerTaskID, WorkspaceID string
	StartedAt, TimeoutAt                   time.Time
}
```

### 3.6 Trace Stream: KHÔNG proxy qua `reload-events` — descope giữ nguyên (Change 2 CR)

Forward toàn bộ Orca SSE `/api/trace-stream` (từng dòng trace, có thể hàng chục event/giây) vào `reload-events:{workspace_id}` không phù hợp với Ring Buffer 50-event/workspace và mục đích thiết kế `diagnostics-service` (TDD-25 §16: phục vụ góc nhìn agent, không phải log streaming tần suất cao). Thay vào đó: `GET /api/v1/planner/orca-sessions/{id}/trace` — **proxy on-demand**, per-client-request, KHÔNG publish Redis, KHÔNG background reconnect loop (khác `OrcaTraceProxy` gốc của `signal-svc` vốn là 1 goroutine nền republish liên tục qua NATS). Vì proxy chỉ mở khi có client gọi, không cần logic reconnect-with-backoff của bản gốc — client's `EventSource` tự retry theo SSE spec nếu kết nối đứt, gọi lại route này, route lại mở kết nối mới tới Orca. Implementation (HTTP handler, không phải background job) thuộc **TASK-ORCA-005-11** (interface layer), không phải TASK-ORCA-005-10 (infra chỉ cần cung cấp `OrcaClientPort`/HTTP client dùng chung để proxy request).

### 3.7 `OrcaHealthMonitor` (Change 3 CR, port + sửa `"kind"`)

```go
// internal/application/orcasession/health_monitor.go
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
// publishes ONLY on state transition (avoid alert spam), mirroring
// diagnostics-service's own "MCP health check" caching philosophy (TDD-25
// §6) even though this is a separate mechanism.
//
// Endpoint used: GET /health (per CR-ORCA-005 Change 3 pseudocode — "Orca
// GET /health endpoint (already exists in Orca)"). Confirming whether
// /health or /health/ready is the operationally-correct choice is IN SCOPE
// of CR-ORCA-001/006 (Orca API Bridge / Headless Deployment), NOT this CR —
// do not silently override without checking those SOLs once rewritten.
type OrcaHealthMonitor struct {
	Client            OrcaClientPort
	Publisher         EventPublisherPort
	ActiveWorkspaces  func() []string // workspace nào đang có session active — Store.List() derived
	Interval          time.Duration   // 30s
	Logger            *slog.Logger

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
		return // only alert on going OFFLINE — matches CR Change 3 ("orca.instance.offline" only)
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
	// active workspace instead (CR Change 3, same rationale).
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

### 3.8 Session Dashboard REST API (`planner-service`, không phải `diagnostics-service`)

Giữ nguyên path/response từ CR Change 4:

```
GET  /api/v1/planner/orca-sessions                     ← List all active Orca sessions
GET  /api/v1/planner/orca-sessions/{planner_task_id}   ← Session detail + progress
GET  /api/v1/planner/orca-sessions/{id}/trace          ← On-demand proxy Orca trace (SSE, §3.6)
POST /api/v1/planner/orca-sessions/{id}/cancel         ← Cancel running session
GET  /api/v1/planner/orca/health                       ← Orca instance health (cached, từ §3.7)
```

`GET /api/v1/planner/orca-sessions` / `{planner_task_id}` đọc từ `SessionStore` (in-memory, §3.3) — **không** query Postgres. **Real-time progress/timeout/offline**: client subscribe `GET /api/v1/diagnostics/events?workspace_id={id}` sẵn có của `diagnostics-service` (route/query-param/auth xác nhận thật ở §9), lọc theo SSE `event:` field hoặc `data.kind` ∈ `{orca_session_progress, orca_session_timeout, orca_session_unreachable, orca_session_completed, orca_instance_offline}`.

Response Session Detail giữ schema gốc CR (không đổi):

```json
{
  "planner_task_id": "TASK-SAGENT-004",
  "orca_task_id": "orca-task-uuid-001",
  "status": "in_progress",
  "progress": 65,
  "started_at": "2026-08-09T16:00:00Z",
  "timeout_at": "2026-08-10T00:00:00Z",
  "last_seen_at": "2026-08-09T17:00:00Z",
  "trace_stream_url": "/api/v1/planner/orca-sessions/TASK-SAGENT-004/trace",
  "realtime_events": {
    "transport": "sse",
    "url": "/api/v1/diagnostics/events?workspace_id={workspace_id}",
    "event_types": ["orca_session_progress", "orca_session_timeout", "orca_session_unreachable", "orca_session_completed"]
  }
}
```

### 3.9 Timeout Handler → Retry/Escalation (Change 5 CR)

```go
// internal/application/orcasession/timeout_handler.go
package orcasession

import (
	"context"
	"fmt"
	"time"
)

// OrcaTimeoutHandler runs in-process, called directly from
// OrcaSessionMonitor.tick right after it publishes orca_session_timeout
// (§3.4) — no Redis/event bus round-trip needed since both live in the same
// planner-service process now (unlike the old signal-svc/temporal-worker
// split, which needed a cross-service Temporal Signal for this).
type OrcaTimeoutHandler struct {
	OrcaClient      OrcaClientPort
	TaskRetryUseCase TaskRetryPort
}

func (h *OrcaTimeoutHandler) Handle(ctx context.Context, orcaTaskID, plannerTaskID string, duration time.Duration) error {
	if err := h.OrcaClient.CancelTask(ctx, orcaTaskID); err != nil {
		// Best-effort — Orca may already be unreachable (that's often WHY it
		// timed out); still proceed to retry/escalation below.
		_ = err
	}
	if h.TaskRetryUseCase == nil {
		return nil // TaskRetryPort not wired yet — reserved integration point (§3.5)
	}
	return h.TaskRetryUseCase.HandleFailure(ctx, plannerTaskID,
		fmt.Sprintf("Orca session timeout after %s", duration))
}
```

### 3.10 Cancel — trực tiếp trong-process, KHÔNG Temporal Signal

Bản gốc (`signal-svc`) phải gửi Temporal Signal vì `signal-svc` và `temporal-worker` là 2 service tách biệt — SOL-ORCA-005 gốc §3.10 giải thích rõ lý do (tránh race giữa 2 process gọi cancel độc lập). CR-ORCA-005 mới **loại bỏ hoàn toàn Temporal** (xem CR §Lưu ý phạm vi) — quyền hủy task giờ vẫn nên thuộc về module sở hữu vòng đời dispatch (tránh race y hệt lý do cũ), nhưng vì cả `orcasession` và module dispatch giờ **cùng process**, cơ chế tránh race đơn giản hơn: **gọi hàm trực tiếp** (in-process call) tới 1 port do module dispatch cung cấp, thay vì network signal.

```go
// internal/application/orcasession/cancel_session.go — chữ ký cuối phụ
// thuộc port thật của module dispatch (Open Task, xem §8); ví dụ minh hoạ:
type DispatchCancelPort interface {
	CancelDispatch(ctx context.Context, plannerTaskID string) error
}
```

`CancelSession` use case KHÔNG gọi `OrcaClientPort.CancelTask` trực tiếp từ handler HTTP — gọi `DispatchCancelPort.CancelDispatch`, để module dispatch tự quyết định khi nào/làm sao gọi Orca trong đúng ngữ cảnh vòng đời nó quản lý (giữ nguyên tinh thần "tránh 2 nơi độc lập cùng gọi cancel" của bản gốc, chỉ đổi cơ chế truyền tin).

---

## 4. Tích hợp giữa các module & service

```
Module dispatch (CR-ORCA-002, planner-service, in-process)
   │ gọi trực tiếp (không NATS): OrcaSessionMonitor.Track(...) sau khi submit task
   ▼
Module orcasession (planner-service)
   ├─ OrcaSessionMonitor.Run() ──(HTTP poll GetTaskStatus, 60s)──▶ Orca
   │        └─ publish {"kind": "orca_session_progress"|"orca_session_timeout"|"orca_session_unreachable", "data": ...}
   │                    ──▶ Redis reload-events:{workspace_id} ──▶ diagnostics-service Watcher ──▶ Broadcaster ──▶ SSE client
   ├─ OrcaHealthMonitor.Run() ──(HTTP GET /health, 30s)──▶ Orca
   │        └─ publish {"kind": "orca_instance_offline"} (chỉ khi chuyển trạng thái) ──▶ Redis (mỗi workspace active)
   ├─ OrcaTimeoutHandler ──(in-process)──▶ Module dispatch.DispatchCancelPort + TaskRetryPort (Open Task)
   ├─ Trace proxy handler ──(SSE pass-through, on-demand)──▶ Orca GET /api/trace-stream
   └─ REST API /api/v1/planner/orca-sessions/** ──▶ WKP admin UI (qua api-gateway)
```

- **CR-ORCA-002**: nguồn `Track()` call (khi submit) và `ActiveDispatchQueryPort`/`DispatchCancelPort` (Bootstrap + Cancel) — **cả hai là Open Task phối hợp**, chữ ký chính xác chưa khoá vì SOL-ORCA-002 chưa viết lại.
- **CR-ORCA-004**: callback tới `planner-service` cập nhật kết quả cuối task — module `orcasession` **không** tham gia đường callback, chỉ tự phát hiện `IsTerminal()` ở tick kế tiếp (session bị `Delete` khỏi `SessionStore` khi status terminal, §3.4) hoặc bị Untrack tường minh nếu module callback gọi thêm 1 hàm — Open Task tương tự.
- **`diagnostics-service`**: KHÔNG sửa code — chỉ cần bổ sung 5 `EventType` const vào `internal/domain/sse/entity.go` để đồng bộ tài liệu (không bắt buộc về mặt runtime — `Watcher` forward `Data` bất kể `Type` có match const đã biết hay không, xem code thật §2.4).

---

## 5. Kế hoạch test

Domain (`internal/domain/orcasession`) — coverage ≥ 90%:

```go
TestOrcaSession_NewOrcaSession_StartsInProgress
TestOrcaSession_RecordProgress_ReportsChanged
TestOrcaSession_RecordProgress_NoChangeWhenSameProgress
TestOrcaSession_DueForHeartbeat_TrueAfterInterval
TestOrcaSession_IsTimedOut_PastTimeoutAt_NotTerminal_ReturnsTrue
TestOrcaSession_IsTimedOut_AlreadyTerminal_ReturnsFalse
TestSessionStatus_IsTerminal_TrueForDoneCancelledTimedOut
TestSessionStatus_IsTerminal_FalseForInProgressReviewBacklog
```

Application (`internal/application/orcasession`) — coverage ≥ 80%:

```go
TestOrcaSessionMonitor_Track_AddsToStore
TestOrcaSessionMonitor_Tick_PublishesOnProgressChange
TestOrcaSessionMonitor_Tick_PublishesOnHeartbeatEvenWithoutChange
TestOrcaSessionMonitor_Tick_DoesNotPublish_WhenNoChangeAndNotDue
TestOrcaSessionMonitor_Tick_PublishesUnreachable_OnClientError
TestOrcaSessionMonitor_Tick_PublishesTimeout_AndCallsTimeoutHandler
TestOrcaSessionMonitor_Publish_UsesKindFieldNotTypeField   // regression test cho §2.4
TestOrcaSessionMonitor_Bootstrap_LoadsActiveDispatches
TestOrcaHealthMonitor_PublishesOffline_OnlyOnTransitionToUnhealthy
TestOrcaHealthMonitor_DoesNotPublish_WhenStillOffline
TestOrcaHealthMonitor_DoesNotPublish_OnRecovery              // CR Change 3 chỉ alert offline
TestOrcaTimeoutHandler_CancelsOrcaTask_AndCallsTaskRetry
TestOrcaTimeoutHandler_StillCallsRetry_WhenCancelFails
```

Infrastructure (`internal/infrastructure/{memory,messaging,orcaclient}`) — coverage ≥ 70%:

```go
TestSessionStore_UpsertAndGet
TestSessionStore_ListByWorkspace_FiltersCorrectly
TestSessionStore_Delete_RemovesSession
TestSessionStore_ConcurrentAccess_NoRace                     // -race
TestReloadEventPublisher_PublishesToCorrectChannel            // reload-events:{workspaceID}
TestReloadEventPublisher_PayloadContainsKindField
TestOrcaClientAdapter_GetTaskStatus_MapsResponse
TestOrcaClientAdapter_HealthCheck_ReturnsErrorOnNon200
```

Interface (HTTP, TASK-ORCA-005-11) — coverage ≥ 60%:

```go
TestOrcaSessionHandler_List_ReturnsActiveSessions
TestOrcaSessionHandler_Get_UnknownID_Returns404
TestOrcaSessionHandler_Cancel_CallsDispatchCancelPort_NotOrcaDirectly
TestOrcaHealthHandler_ReturnsCachedState_NoSyncOrcaCall
TestOrcaTraceHandler_ProxiesOrcaSSE_OnDemand
```

---

## 6. Rủi ro & giảm thiểu

| Rủi ro | Mức độ | Giảm thiểu |
|---|---|---|
| Mất trạng thái tracking (`LastProgress`/`LastPublishAt`) khi `planner-service` restart | Thấp | Chấp nhận theo thiết kế (§2.2) — tối đa 1 publish dư ở tick kế tiếp; `Bootstrap()` nạp lại danh sách active từ module dispatch |
| Field JSON sai (`"type"` thay vì `"kind"`) khiến `diagnostics-service` không set được `event.Type`, giảm khả năng client filter theo SSE `event:` | Trung bình nếu không sửa | Đã sửa tại chỗ trong toàn bộ code mẫu SOL này (§2.4) — cần test regression `TestOrcaSessionMonitor_Publish_UsesKindFieldNotTypeField` |
| Ring buffer 50-event/workspace dùng chung với publisher khác (`skill_updated`, `plugin_installed`, ...) bị tràn nhanh nếu nhiều Orca session đồng thời | Trung bình (đã ghi nhận trong CR) | Throttle publish (≥60s hoặc chỉ khi đổi), không sửa `SSE_RING_BUFFER_SIZE` trong CR này (thuộc vận hành `diagnostics-service`) |
| `ActiveDispatchQueryPort`/`DispatchCancelPort`/`Track()` call-site phụ thuộc module dispatch (CR-ORCA-002) **chưa viết lại** | Cao (blocking cho integration thật, không blocking cho unit test) | Port đã khai báo với chữ ký placeholder, ghi rõ Open Task; unit test dùng fake implementation, không chờ SOL-ORCA-002 |
| `TaskRetryPort` (CR-TASK-006, ngoài phạm vi bộ Orca) chưa tồn tại | Thấp | `OrcaTimeoutHandler.TaskRetryUseCase` cho phép `nil` (no-op) — không panic, không chặn phần còn lại của monitor |
| Health monitor spam nếu Orca flap on/off liên tục | Thấp | Chỉ publish khi chuyển sang offline (không publish khi hồi phục — theo đúng CR Change 3, khác bản gốc `signal-svc` từng publish cả 2 chiều) |

---

## 7. Ước tính công việc theo layer

| Layer | Hạng mục | Giờ |
|---|---|---|
| Domain | `OrcaSession`, `SessionStatus`, `SessionStore` port, `EventKind` consts | 3h |
| Application | `OrcaSessionMonitor`, `OrcaHealthMonitor`, `OrcaTimeoutHandler`, `TrackSession`/`GetSession`/`ListSessions`/`CancelSession` | 8h |
| Infrastructure | `SessionStore` in-memory, `ReloadEventPublisher` (tái dùng pattern skills-service/plugin-registry), `OrcaClient` adapter | 4h |
| Interface | REST handlers (List/Get/Cancel/Health/Trace), router wiring, `config.go`/`cmd/main.go` | 5h |
| **Tổng (phía Go)** | | **20h** — khớp Effort Estimate của CR-ORCA-005 |
| Phối hợp — bổ sung 5 `EventType` const vào `diagnostics-service` | Ngoài phạm vi ước tính này (team `diagnostics-service`) | 1h (theo CR) |

---

## 8. Dependencies

- **CR-ORCA-002** (module dispatch, `planner-service`): nguồn `Track()` call-site + `ActiveDispatchQueryPort`/`DispatchCancelPort`. **SOL-ORCA-002 chưa được viết lại theo kiến trúc `vnp-workplace`** tại thời điểm SOL này cập nhật (vẫn mô tả `temporal-worker`/`vnp-planner`) — mọi chữ ký port trong §3.5/§3.10 là placeholder, cần đối chiếu lại khi SOL-ORCA-002 được viết lại.
- **CR-ORCA-004** (module callback, `planner-service`): tương tự, chưa viết lại; `orcasession` chỉ phụ thuộc gián tiếp (session tự hết hạn theo `IsTerminal()`/poll, không nhận callback trực tiếp).
- **`diagnostics-service`** (đã build thật): phụ thuộc runtime duy nhất đã xác nhận chắc chắn — kênh Redis `reload-events:{workspace_id}` và route `GET /api/v1/diagnostics/events` (xem §9).
- **`backend/pkg/orcaclient`** (dự kiến, CR-ORCA-002 §Change D4 bản gốc — "trích `OrcaClient` HTTP thành package dùng chung") — chưa tồn tại trong repo tại thời điểm viết SOL này (`find` không ra kết quả), path/API chưa khoá.

---

## 9. Xác thực với code thật `diagnostics-service` (khảo sát 2026-08-10)

Đối chiếu trực tiếp với `backend/services/diagnostics-service/` (đã build, có test):

1. **Channel + field `"kind"`** — `internal/infrastructure/events/redis/subscriber.go` xác nhận `PSubscribe("reload-events:*")`, tách `workspace_id` từ suffix, và parse field JSON **`"kind"`** (không phải `"type"`) để set `sse.SSEEvent.Type` — xem §2.4. Đây là phát hiện quan trọng nhất, đã sửa xuyên suốt SOL này.
2. **Route SSE thật**: `internal/presentation/http/router.go` xác nhận `api.GET("/events", h.SSE.Stream)` dưới group `/api/v1/diagnostics` có `middleware.RequireBearer()` + `middleware.ExtractUserContext()` — khớp đúng CR (`Authorization: Bearer`, không phải `X-Internal-Service`).
3. **Query param + auth cho SSE**: `internal/presentation/http/handler/sse_handler.go` xác nhận `workspace_id` là query param (`c.QueryParam("workspace_id")`), `Last-Event-ID` header cho replay, và giới hạn `maxConnPerUser` qua `TryAcquireConnSlot` (trả `429 too_many_sse_connections` khi vượt) — khớp CR ("max 5 concurrent conn/user").
4. **`EventType` consts hiện có**: `internal/domain/sse/entity.go` xác nhận **chưa có** const nào cho Orca (`EventOrcaSessionProgress`, ...) — đúng như CR đã đánh dấu Open Task, KHÔNG tự thêm trong phạm vi SOL/TASK này.
5. **Ring buffer / config**: `config/config.go` xác nhận `SSERingBufferSize` default 50 (`SSE_RING_BUFFER_SIZE`), `SSEMaxConnPerUser` default 5, `SSEKeepaliveSec` default 30 — khớp số liệu CR dùng trong Design Decision.
6. **Publisher pattern tham chiếu thật**: `skills-service`/`plugin-registry` đều có `internal/infrastructure/messaging/reload_event_publisher.go` với cùng shape `channel := fmt.Sprintf("reload-events:%s", workspaceID)`, cùng field `Kind string \`json:"kind"\`` — xác nhận đây là pattern **đã chạy, có integration test pass** (`test/integration_test.go` mỗi service subscribe trực tiếp Redis thật), không phải suy đoán.
