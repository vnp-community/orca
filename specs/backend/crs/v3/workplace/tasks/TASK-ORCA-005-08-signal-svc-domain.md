> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
## Status: 🔲 NOT STARTED

# TASK-ORCA-005-08 — `planner-service`: Domain Layer (module `orcasession`)

**Phase:** 3 — Read-model / observability
**Scope:** ✅ `vnp-workplace` — Go (`planner-service`, module `orcasession`)
**Source:** [SOL-ORCA-005 §2–§3.3](../solutions/SOL-ORCA-005-planner-orca-session-monitor.md#2-quyết-định-kiến-trúc)
**Depends On:** — (domain layer, không phụ thuộc task Go nào khác — có thể làm song song với bộ TASK-ORCA-002)
**Estimated Files:** ~7 files (4 code + 3 test)
**Working Dir:** `backend/services/planner-service/internal/domain/orcasession/`

> **Đổi tên khỏi bản gốc:** file này giữ nguyên số hiệu `TASK-ORCA-005-08` và tên file cũ (nhắc `signal-svc`) theo yêu cầu KHÔNG đổi tên file — nhưng toàn bộ nội dung bên dưới nói về module **`orcasession` trong `planner-service`**, KHÔNG còn `signal-svc`/`vnp-planner`.

---

## Bối cảnh quan trọng — ĐỌC KỸ TRƯỚC KHI CODE

1. **`planner-service` là service HOÀN TOÀN MỚI** (chưa tồn tại trong repo tại thời điểm viết task này) — không có `MarketSignal` hay aggregate nào khác để xung đột như bản gốc `signal-svc` từng phải xử lý. `orcasession` chỉ cần tách package sạch khỏi 2 module anh em cũng nằm trong `planner-service`: `dispatch` (CR-ORCA-002) và `callback` (CR-ORCA-004) — cả hai **ngoài phạm vi task này**, chưa có SOL viết lại để tham chiếu chi tiết.

2. **KHÔNG còn Postgres, KHÔNG còn NATS, KHÔNG còn domain event queue (`record()`/`PopEvents()`).** Đây là khác biệt lớn nhất so với bản gốc (`signal-svc` — Postgres read-model event-sourcing qua NATS). `OrcaSession` giờ là bản ghi **in-memory thuần túy**, chỉ dùng để `OrcaSessionMonitor` (TASK-ORCA-005-09) polling/so sánh progress/throttle publish — xem [SOL-ORCA-005 §2.2](../solutions/SOL-ORCA-005-planner-orca-session-monitor.md#22-trạng-thái-session-in-memory-không-postgres--khác-biệt-lớn-nhất-so-với-bản-gốc) cho lý do đầy đủ. Nguồn sự thật nghiệp vụ (task/dispatch) vẫn nằm ở module `dispatch` (CR-ORCA-002), persist ở đó — KHÔNG nhân đôi ở đây.

3. **`OrcaSession` KHÔNG phải nguồn điều khiển workflow** — nó là bản ghi quan sát/theo dõi runtime, có thể mất khi `planner-service` restart (chấp nhận được, xem `Bootstrap()` ở TASK-ORCA-005-09). Ghi rõ điều này trong doc-comment.

4. **`SessionStatus` (§3.2 SOL) chưa xác thực đầy đủ** với response schema thật của Orca `GetTaskStatus` — 2 giá trị `review`/`backlog` là suy luận từ README tổng quan bộ CR-ORCA, không phải hợp đồng đã khoá (CR-ORCA-001/002 sở hữu việc xác thực này). Giữ nguyên comment cảnh báo này trong code thật, không tự ý xoá.

---

## Mục tiêu

Implement domain layer module `orcasession`: entity `OrcaSession` (in-memory tracking record), `SessionStatus`, port `SessionStore`, và hằng `EventKind` publish lên `reload-events:{workspace_id}`.

---

## Acceptance Criteria

- [ ] `go build ./internal/domain/...` thành công
- [ ] `go test ./internal/domain/... -v -race -cover` pass 100%, coverage ≥ 90%
- [ ] `OrcaSession.RecordProgress` trả về `changed=true` khi và chỉ khi `progress` khác giá trị trước đó
- [ ] `OrcaSession.IsTimedOut` chỉ `true` khi `Status` chưa terminal VÀ `now` sau `TimeoutAt`
- [ ] `SessionStatus.IsTerminal()` đúng cho `done`/`cancelled`/`timed_out`, sai cho các giá trị còn lại (kể cả `unreachable`)
- [ ] Package `orcasession` KHÔNG import bất kỳ package nào của module `dispatch`/`callback` (tách bạch, xem Bối cảnh #1)
- [ ] Package `orcasession` KHÔNG import `internal/domain/sse` của `diagnostics-service` (2 service khác nhau, khác go.mod — xem `event_kind.go`)
- [ ] File `internal/domain/orcasession/entity.go` KHÔNG có field/method nào thuộc về domain event-sourcing (`record()`, `PopEvents()`, `[]event.DomainEvent`) — đã bỏ so với bản gốc `signal-svc`

---

## File 1: `internal/domain/orcasession/entity.go`

```go
package orcasession

import "time"

// SessionStatus mirrors Orca's own OrcaTask status string verbatim
// (docs/crs/v3/orca/README.md §Orca Key APIs: "OrcaTask — task với status:
// backlog → in_progress → review → done") PLUS 2 synthetic statuses set
// only by planner-service itself when it — not Orca — decides the session
// is over.
//
// CHƯA XÁC THỰC: giá trị "review"/"backlog" là suy luận từ README tổng quan
// CR-ORCA, KHÔNG phải từ response schema thật của GetTaskStatus (thuộc phạm
// vi CR-ORCA-001/002). Nếu xác thực sau này khoá schema khác, cập nhật type
// này theo — không coi đây là hợp đồng đã khoá.
type SessionStatus string

const (
	StatusInProgress SessionStatus = "in_progress"
	StatusReview      SessionStatus = "review"
	StatusDone        SessionStatus = "done"
	StatusBacklog     SessionStatus = "backlog"
	StatusCancelled   SessionStatus = "cancelled"   // đặt bởi CancelSession, không phải Orca trả về
	StatusTimedOut    SessionStatus = "timed_out"   // đặt bởi OrcaSessionMonitor khi TimeoutAt qua
	StatusUnreachable SessionStatus = "unreachable" // cờ TẠM THỜI trên bản ghi giám sát — KHÔNG đổi Status thật, xem doc-comment RecordProgress
)

// IsTerminal reports whether no further polling/timeout tracking is needed
// for a session in this status.
func (s SessionStatus) IsTerminal() bool {
	switch s {
	case StatusDone, StatusCancelled, StatusTimedOut:
		return true
	}
	return false
}

// OrcaSession is a lightweight, IN-MEMORY tracking record for one Orca
// AI-agent execution session — used ONLY by OrcaSessionMonitor
// (TASK-ORCA-005-09) for polling, progress-change detection and publish
// throttling (SOL-ORCA-005 §2.2). It is NOT the business source of truth
// for the task/dispatch (that lives in the sibling `dispatch` module owned
// by CR-ORCA-002) and is NOT persisted to any database — safe to lose on
// process restart (OrcaSessionMonitor.Bootstrap reloads active sessions
// from the dispatch module, TASK-ORCA-005-09).
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

// NewOrcaSession constructs a session in StatusInProgress. Called by
// OrcaSessionMonitor.Track (TASK-ORCA-005-09) — the entry point the sibling
// dispatch module (CR-ORCA-002) calls in-process right after submitting a
// task to Orca (replaces the old "orca.task.submitted" NATS event: no
// service boundary to cross anymore).
func NewOrcaSession(orcaTaskID, plannerTaskID, workspaceID string, startedAt, timeoutAt time.Time) *OrcaSession {
	return &OrcaSession{
		OrcaTaskID: orcaTaskID, PlannerTaskID: plannerTaskID, WorkspaceID: workspaceID,
		Status: StatusInProgress, StartedAt: startedAt, LastSeenAt: startedAt, TimeoutAt: timeoutAt,
	}
}

// RecordProgress updates LastSeenAt/Status/LastProgress from a fresh poll
// result and reports whether the progress VALUE actually changed since the
// last call — the caller (OrcaSessionMonitor.tick) combines this with
// DueForHeartbeat to decide whether to publish onto reload-events (throttle
// per SOL-ORCA-005 §2.3, avoids flooding the shared 50-event ring buffer).
//
// This does NOT set StatusUnreachable — that is a poll-failure signal
// (Orca did not respond at all), handled by the caller directly, not by
// this method (there is no successful status/progress to record in that
// case).
func (s *OrcaSession) RecordProgress(status SessionStatus, progress int, now time.Time) (changed bool) {
	s.LastSeenAt = now
	s.Status = status
	changed = progress != s.LastProgress
	s.LastProgress = progress
	return changed
}

// MarkPublished records that a reload-events publish just happened, for
// DueForHeartbeat's throttle window.
func (s *OrcaSession) MarkPublished(now time.Time) { s.LastPublishAt = now }

// DueForHeartbeat reports whether at least `interval` has passed since the
// last publish — used to force a publish even without a progress change,
// per CR-ORCA-005 Change 1 ("tối đa 1 lần/60s/session").
func (s *OrcaSession) DueForHeartbeat(now time.Time, interval time.Duration) bool {
	return now.Sub(s.LastPublishAt) >= interval
}

// IsTimedOut reports whether TimeoutAt has passed AND the session has not
// already reached a terminal status (a terminal session is not "timed
// out", it already finished/was cancelled/timed out once).
func (s *OrcaSession) IsTimedOut(now time.Time) bool {
	return !s.Status.IsTerminal() && now.After(s.TimeoutAt)
}

// MarkTimedOut and MarkCancelled are planner-service-local status
// transitions — Orca itself never reports these two status strings.
func (s *OrcaSession) MarkTimedOut(now time.Time)  { s.Status = StatusTimedOut; s.LastSeenAt = now }
func (s *OrcaSession) MarkCancelled(now time.Time) { s.Status = StatusCancelled; s.LastSeenAt = now }
```

---

## File 2: `internal/domain/orcasession/errors.go`

```go
package orcasession

import "errors"

var (
	ErrNotFound       = errors.New("orcasession: not found")
	ErrAlreadyTracked = errors.New("orcasession: already tracked")
)
```

---

## File 3: `internal/domain/orcasession/store.go`

```go
package orcasession

// SessionStore is an in-memory TRACKING store (SOL-ORCA-005 §2.2/§2.3) —
// deliberately NOT a persistence/repository port in the usual TDD-00 §6
// sense. There is no Postgres implementation of this interface anywhere in
// this task set; the sole implementation (TASK-ORCA-005-10) is a
// map + sync.RWMutex living in internal/infrastructure/memory.
type SessionStore interface {
	// Upsert inserts or replaces the tracking record for OrcaTaskID.
	Upsert(s *OrcaSession)
	Get(orcaTaskID string) (*OrcaSession, bool)
	GetByPlannerTaskID(plannerTaskID string) (*OrcaSession, bool)
	// List returns every currently-tracked session (any status).
	List() []*OrcaSession
	ListByWorkspace(workspaceID string) []*OrcaSession
	Delete(orcaTaskID string)
}
```

---

## File 4: `internal/domain/orcasession/event_kind.go`

```go
// EventKind values are published as the top-level "kind" field on
// reload-events:{workspace_id} (SOL-ORCA-005 §2.4 — field name is "kind",
// NOT "type", confirmed against diagnostics-service's real
// internal/infrastructure/events/redis/subscriber.go).
//
// These are LOCAL string constants — this package does NOT import
// diagnostics-service's internal/domain/sse package: different service,
// different go.mod, cross-service internal-package import is
// architecturally invalid in this monorepo (each service/go.work module is
// independently buildable). String VALUES below must stay byte-for-byte in
// sync with the 5 consts requested as a coordination Open Task in
// diagnostics-service's internal/domain/sse/entity.go (CR-ORCA-005
// §Design Decision "Yêu cầu phối hợp") — that file is NOT modified by this
// task (confirmed via TASK-ORCA-005-08 Acceptance Criteria: no such
// consts exist there yet as of this survey).
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

---

## Test File 5: `internal/domain/orcasession/entity_test.go`

```go
func TestOrcaSession_NewOrcaSession_StartsInProgress(t *testing.T)
func TestOrcaSession_RecordProgress_ReportsChangedTrue_WhenProgressDiffers(t *testing.T)
func TestOrcaSession_RecordProgress_ReportsChangedFalse_WhenProgressSame(t *testing.T)
func TestOrcaSession_RecordProgress_UpdatesStatusAndLastSeenAt(t *testing.T)
func TestOrcaSession_DueForHeartbeat_FalseImmediatelyAfterPublish(t *testing.T)
func TestOrcaSession_DueForHeartbeat_TrueAfterIntervalElapsed(t *testing.T)
func TestOrcaSession_IsTimedOut_PastTimeoutAt_NotTerminal_ReturnsTrue(t *testing.T)
func TestOrcaSession_IsTimedOut_PastTimeoutAt_AlreadyTerminal_ReturnsFalse(t *testing.T)
func TestOrcaSession_IsTimedOut_BeforeTimeoutAt_ReturnsFalse(t *testing.T)
func TestOrcaSession_MarkTimedOut_SetsStatusTimedOut(t *testing.T)
func TestOrcaSession_MarkCancelled_SetsStatusCancelled(t *testing.T)
```

## Test File 6: `internal/domain/orcasession/status_test.go`

```go
func TestSessionStatus_IsTerminal_TrueForDoneCancelledTimedOut(t *testing.T)
func TestSessionStatus_IsTerminal_FalseForInProgressReviewBacklog(t *testing.T)
func TestSessionStatus_IsTerminal_FalseForUnreachable(t *testing.T) // unreachable is NOT terminal — signal-quality flag only
```

---

## Verification

```bash
cd backend/services/planner-service

go build ./internal/domain/...
go vet ./internal/domain/...
go test ./internal/domain/... -v -race -cover

go test ./internal/domain/... -coverprofile=domain_cov.out
go tool cover -func=domain_cov.out | grep total   # kỳ vọng >= 90%
```
