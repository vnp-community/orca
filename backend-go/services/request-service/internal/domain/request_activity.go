package domain

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

// ActivitySeverity says how much a line of the activity feed needs a person: info is history, attention is something waiting
// for someone or worth a look, error is something that stopped.
type ActivitySeverity string

const (
	ActivityInfo      ActivitySeverity = "info"
	ActivityAttention ActivitySeverity = "attention"
	ActivityError     ActivitySeverity = "error"
)

func (s ActivitySeverity) rank() int {
	switch s {
	case ActivityError:
		return 2
	case ActivityAttention:
		return 1
	}
	return 0
}

// AtLeast is true when s is as severe as min; an empty min keeps everything.
func (s ActivitySeverity) AtLeast(min ActivitySeverity) bool { return s.rank() >= min.rank() }

func ParseActivitySeverity(s string) (ActivitySeverity, error) {
	switch ActivitySeverity(strings.ToLower(strings.TrimSpace(s))) {
	case "", ActivityInfo:
		return ActivityInfo, nil
	case ActivityAttention:
		return ActivityAttention, nil
	case ActivityError:
		return ActivityError, nil
	}
	return "", ErrActivityFilterInvalid("min_severity must be info, attention or error")
}

// Activity kinds. A kind names what happened, not which table recorded it.
const (
	ActivityRequestCreated    = "request.created"
	ActivityRequestClassified = "request.classified"
	ActivityRequestTypeChange = "request.type_changed"
	ActivityAnalysisGenerated = "analysis.generated"
	ActivityClarifyAsked      = "clarification.asked"
	ActivityClarifyAnswered   = "clarification.answered"
	ActivityApprovalRequested = "approval.requested"
	ActivityApprovalDecided   = "approval.decided"
	ActivityPhaseStarted      = "phase.started"
	ActivityPhaseDone         = "phase.done"
	ActivityPlanDone          = "plan.done"
	ActivityTaskStarted       = "task.started"
	ActivityTaskSucceeded     = "task.succeeded"
	// ActivityTaskAccepted is the finished task being marked done (by Orca's auto-complete or by a person), the second line of a success.
	ActivityTaskAccepted     = "task.accepted"
	ActivityTaskFailed       = "task.failed"
	ActivityTaskCancelled    = "task.cancelled"
	ActivityRequestReturned  = "request.returned"
	ActivityRequestReopened  = "request.reopened"
	ActivityRequestCancelled = "request.cancelled"
)

// ActivityAnalysisRevisionRequested is a person asking for changes to the analysis; the feed records that it happened, never the text.
const ActivityAnalysisRevisionRequested = "analysis.revision_requested"

// ActivityPlanRevisionRequested is a person asking for changes to the Plan; like the analysis line it records that it
// happened, never the text (C9).
const ActivityPlanRevisionRequested = "plan.revision_requested"

// AllActivityKinds is the closed set a filter may name.
var AllActivityKinds = []string{
	ActivityRequestCreated, ActivityRequestClassified, ActivityRequestTypeChange, ActivityAnalysisGenerated, ActivityClarifyAsked,
	ActivityClarifyAnswered, ActivityApprovalRequested, ActivityApprovalDecided, ActivityPhaseStarted, ActivityPhaseDone, ActivityPlanDone,
	ActivityTaskStarted, ActivityTaskSucceeded, ActivityTaskAccepted, ActivityTaskFailed, ActivityTaskCancelled, ActivityRequestReturned, ActivityRequestReopened,
	ActivityRequestCancelled, ActivityAnalysisRevisionRequested, ActivityPlanRevisionRequested,
}

// ActivityRow is one line as a repository read it: the columns are generic (A to D, N) because every source table has its
// own shape, and BuildActivityEntry gives them their meaning by Kind.
type ActivityRow struct {
	Kind        string
	ID          string
	OccurredAt  time.Time
	ActorKind   string
	ActorID     string
	SubjectType string
	SubjectID   string
	A, B, C, D  string
	N           int64
}

// ActivityEntry is a line of the feed ready for the screen.
type ActivityEntry struct {
	ID          string
	Kind        string
	OccurredAt  time.Time
	ActorKind   string
	ActorID     string
	SubjectType string
	SubjectID   string
	// Summary is one English line from structured fields. Screens translate from Kind and Details instead.
	Summary  string
	Attempt  int
	Severity ActivitySeverity
	Details  map[string]any
}

// ActivityTask is what the feed knows about a task of the Request: its title and the number people see.
type ActivityTask struct {
	Title  string
	Number int64
}

// ActivityCursor is the keyset of the feed: newest first, ties broken by id.
type ActivityCursor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}

func (c ActivityCursor) Encode() string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func ErrActivityFilterInvalid(msg string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_ACTIVITY_FILTER_INVALID", msg, nil)
}

func ErrActivityCursorInvalid() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_ACTIVITY_CURSOR_INVALID", "page_token is not a valid activity cursor", nil)
}

// DecodeActivityCursor reads a token; an empty one means the first page.
func DecodeActivityCursor(tok string) (*ActivityCursor, error) {
	if tok == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(tok)
	if err != nil {
		return nil, ErrActivityCursorInvalid()
	}
	var c ActivityCursor
	if err := json.Unmarshal(b, &c); err != nil || c.ID == "" || c.At.IsZero() {
		return nil, ErrActivityCursorInvalid()
	}
	return &c, nil
}

// ActivityQuery is what a repository filters in SQL. Severity is not here: it depends on the lines around a row, so the use
// case applies it after reading.
type ActivityQuery struct {
	Kinds     []string
	ActorKind string
	SubjectID string
	Since     *time.Time
	Before    *ActivityCursor
	Limit     int
}

// MaxActivityDetailsBytes bounds the structured details of one line.
const MaxActivityDetailsBytes = 4096

// BuildActivityEntry turns a row into a line. maxAttempts is the retry limit of a task, which decides whether a failure is the
// last one; tasks maps task ids to what people call them.
func BuildActivityEntry(row ActivityRow, tasks map[string]ActivityTask, maxAttempts int) ActivityEntry {
	e := ActivityEntry{
		ID: row.ID, Kind: row.Kind, OccurredAt: row.OccurredAt.UTC(), ActorKind: normalizeActivityActor(row.Kind, row.ActorKind),
		ActorID: row.ActorID, SubjectType: row.SubjectType, SubjectID: row.SubjectID, Severity: ActivityInfo, Details: map[string]any{},
	}
	taskName := func() string {
		if t, ok := tasks[row.SubjectID]; ok && t.Title != "" {
			if t.Number > 0 {
				return fmt.Sprintf("#TG-%d %s", t.Number, t.Title)
			}
			return t.Title
		}
		return "task " + shortID(row.SubjectID)
	}
	setTask := func() {
		e.Details["taskId"] = row.SubjectID
		if t, ok := tasks[row.SubjectID]; ok {
			e.Details["taskTitle"] = t.Title
			if t.Number > 0 {
				e.Details["taskNumber"] = t.Number
			}
		}
	}
	switch row.Kind {
	case ActivityRequestCreated:
		e.Summary = "Request created"
	case ActivityRequestClassified:
		e.Summary = "Classified as " + row.B
		e.Details["to"], e.Details["reason"] = row.B, row.C
	case ActivityRequestTypeChange:
		e.Summary = fmt.Sprintf("Type changed from %s to %s", row.A, row.B)
		e.Details["from"], e.Details["to"], e.Details["reason"] = row.A, row.B, row.C
	case ActivityAnalysisGenerated:
		e.Summary = fmt.Sprintf("%s version %d is %s", row.A, row.N, row.B)
		e.Details["analysisKind"], e.Details["status"], e.Details["version"] = row.A, row.B, row.N
	case ActivityAnalysisRevisionRequested:
		// The reviewer's feedback is user content (C9): the row has no column for it, so the line cannot show it.
		e.Summary = "Requested changes to the analysis"
		e.Details["solutionId"] = row.SubjectID
	case ActivityPlanRevisionRequested:
		e.Summary = "Requested changes to the plan"
	case ActivityClarifyAsked:
		e.Summary = "Clarification asked"
		e.Details["source"], e.Details["status"], e.Details["resumeStatus"], e.Details["round"] = row.A, row.B, row.C, row.N
		if row.B == "open" {
			e.Severity = ActivityAttention
		}
	case ActivityClarifyAnswered:
		e.Summary = "Clarification answered"
		e.Details["source"], e.Details["round"] = row.A, row.N
	case ActivityApprovalRequested:
		e.Summary = fmt.Sprintf("Approval of the %s requested", row.SubjectType)
		e.Details["stage"], e.Details["status"] = row.A, row.B
		if row.B == "pending" {
			e.Severity = ActivityAttention
		}
	case ActivityApprovalDecided:
		e.Summary = fmt.Sprintf("The %s was %s", row.SubjectType, row.A)
		e.Details["result"], e.Details["stage"], e.Details["comment"] = row.A, row.B, row.C
		if row.A == "rejected" {
			e.Severity = ActivityAttention
		}
	case ActivityPhaseStarted:
		e.Summary = "Phase started"
		if row.A == "auto" {
			e.Summary = "Phase started automatically"
		}
		e.Details["trigger"], e.Details["phaseId"] = row.A, row.SubjectID
	case ActivityPhaseDone:
		e.Summary = "Phase finished"
		e.Details["phaseId"] = row.SubjectID
	case ActivityPlanDone:
		e.Summary = "All tasks finished"
	case ActivityTaskStarted:
		e.Attempt = int(row.N)
		e.Summary = fmt.Sprintf("%s started (attempt %d)", taskName(), e.Attempt)
		setTask()
		e.Details["attempt"] = e.Attempt
	case ActivityTaskSucceeded:
		e.Attempt = int(row.N)
		e.Summary = taskName() + " finished"
		setTask()
		e.Details["attempt"] = e.Attempt
	case ActivityTaskAccepted:
		e.Summary = taskName() + " marked done"
		setTask()
	case ActivityTaskCancelled:
		e.Summary = taskName() + " cancelled"
		setTask()
	case ActivityTaskFailed:
		e.Attempt = int(row.N)
		cause := ClassifyFailureCause(row.B, row.C)
		e.Summary = fmt.Sprintf("%s failed (%s)", taskName(), cause)
		setTask()
		e.Details["attempt"], e.Details["cause"], e.Details["error"] = e.Attempt, string(cause), truncateActivityText(row.C, 400)
		e.Details["maxAttempts"] = maxAttempts
		e.Severity = ActivityAttention
		// A dispatch error never reached the agent, so it does not use up an attempt and is never the last one.
		if row.B != CauseDispatchError && maxAttempts > 0 && e.Attempt >= maxAttempts {
			e.Severity = ActivityError
		}
	case ActivityRequestReturned:
		e.Summary = "Returned to backlog"
		e.Details["stage"], e.Details["category"], e.Details["reason"] = row.B, row.C, truncateActivityText(row.D, 400)
		e.Severity = ActivityError
	case ActivityRequestReopened:
		e.Summary = "Reopened"
		e.Details["stage"], e.Details["reason"] = row.B, truncateActivityText(row.D, 400)
	case ActivityRequestCancelled:
		e.Summary = "Cancelled"
		e.Details["reason"] = truncateActivityText(row.D, 400)
	default:
		e.Summary = row.Kind
	}
	return e
}

func normalizeActivityActor(kind, actor string) string {
	switch kind {
	case ActivityRequestClassified, ActivityRequestTypeChange:
		if actor == "agent" {
			return "ai" // the classifier is stored as agent; people call it AI
		}
	}
	switch actor {
	case "user", "ai", "agent", "system":
		return actor
	}
	return "system"
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func truncateActivityText(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return strings.ToValidUTF8(s[:max], "") + "…"
}
