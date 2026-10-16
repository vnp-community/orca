package wscompat

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ActivityEntryView is one line of a Request's History (CR-REQ-038). Details is the structured JSON behind Summary; screens
// build their own sentence from Kind and Details, Summary is the English fallback.
type ActivityEntryView struct {
	ID          string          `json:"id"`
	Kind        string          `json:"kind"`
	OccurredAt  string          `json:"occurredAt"`
	ActorKind   string          `json:"actorKind"`
	ActorID     string          `json:"actorId,omitempty"`
	SubjectType string          `json:"subjectType,omitempty"`
	SubjectID   string          `json:"subjectId,omitempty"`
	Summary     string          `json:"summary"`
	Attempt     int32           `json:"attempt,omitempty"`
	Severity    string          `json:"severity"`
	Details     json.RawMessage `json:"details"`
}

func activityEntryViews(es []*requestv1.ActivityEntry) []ActivityEntryView {
	out := make([]ActivityEntryView, 0, len(es))
	for _, e := range es {
		details := json.RawMessage(e.GetDetailsJson())
		if len(strings.TrimSpace(string(details))) == 0 || !json.Valid(details) {
			details = json.RawMessage("{}")
		}
		out = append(out, ActivityEntryView{
			ID: e.GetId(), Kind: e.GetKind(), OccurredAt: rfc3339(e.GetOccurredAt()), ActorKind: e.GetActorKind(), ActorID: e.GetActorId(),
			SubjectType: e.GetSubjectType(), SubjectID: e.GetSubjectId(), Summary: e.GetSummary(), Attempt: e.GetAttempt(),
			Severity: e.GetSeverity(), Details: details,
		})
	}
	return out
}

type ExecutionProgressView struct {
	PhasesTotal  int32 `json:"phasesTotal"`
	PhasesDone   int32 `json:"phasesDone"`
	TasksTotal   int32 `json:"tasksTotal"`
	TasksDone    int32 `json:"tasksDone"`
	TasksFailed  int32 `json:"tasksFailed"`
	TasksRunning int32 `json:"tasksRunning"`
}

type ExecutionRunningTaskView struct {
	TaskID      string `json:"taskId"`
	PhaseID     string `json:"phaseId,omitempty"`
	Title       string `json:"title"`
	Attempt     int32  `json:"attempt"`
	MaxAttempts int32  `json:"maxAttempts"`
	StartedAt   string `json:"startedAt,omitempty"`
}

type ExecutionFailedTaskView struct {
	TaskID      string `json:"taskId"`
	PhaseID     string `json:"phaseId,omitempty"`
	Title       string `json:"title"`
	Attempts    int32  `json:"attempts"`
	MaxAttempts int32  `json:"maxAttempts"`
	LastCause   string `json:"lastCause"`
	LastError   string `json:"lastError,omitempty"`
	LastAt      string `json:"lastAt,omitempty"`
}

type ExecutionNeedsYouView struct {
	Kind        string `json:"kind"`
	RefID       string `json:"refId"`
	SubjectType string `json:"subjectType,omitempty"`
	Title       string `json:"title,omitempty"`
	PhaseID     string `json:"phaseId,omitempty"`
	Since       string `json:"since,omitempty"`
	DueAt       string `json:"dueAt,omitempty"`
}

type ExecutionHandoverView struct {
	State           string `json:"state"`
	ArchiveAttempts int32  `json:"archiveAttempts"`
	SummaryRef      string `json:"summaryRef,omitempty"`
}

// ExecutionSummaryView mirrors ExecutionSummary: progress, what runs, what failed and why, what waits for the viewer.
type ExecutionSummaryView struct {
	RequestID      string                     `json:"requestId"`
	Status         string                     `json:"status"`
	Engine         string                     `json:"engine,omitempty"`
	ExecutionMode  string                     `json:"executionMode,omitempty"`
	Progress       ExecutionProgressView      `json:"progress"`
	CurrentPhaseID string                     `json:"currentPhaseId,omitempty"`
	NextPhaseID    string                     `json:"nextPhaseId,omitempty"`
	Running        []ExecutionRunningTaskView `json:"running"`
	Failed         []ExecutionFailedTaskView  `json:"failed"`
	NeedsYou       []ExecutionNeedsYouView    `json:"needsYou"`
	Handover       ExecutionHandoverView      `json:"handover"`
	MaxAttempts    int32                      `json:"maxAttempts"`
	StartedAt      string                     `json:"startedAt,omitempty"`
}

func executionSummaryViewOf(s *requestv1.ExecutionSummary) ExecutionSummaryView {
	p := s.GetProgress()
	v := ExecutionSummaryView{
		RequestID: s.GetRequestId(), Status: s.GetStatus(), Engine: s.GetEngine(), ExecutionMode: s.GetExecutionMode(),
		Progress: ExecutionProgressView{
			PhasesTotal: p.GetPhasesTotal(), PhasesDone: p.GetPhasesDone(), TasksTotal: p.GetTasksTotal(), TasksDone: p.GetTasksDone(),
			TasksFailed: p.GetTasksFailed(), TasksRunning: p.GetTasksRunning(),
		},
		CurrentPhaseID: s.GetCurrentPhaseId(), NextPhaseID: s.GetNextPhaseId(), MaxAttempts: s.GetMaxAttempts(), StartedAt: rfc3339(s.GetStartedAt()),
		Running: []ExecutionRunningTaskView{}, Failed: []ExecutionFailedTaskView{}, NeedsYou: []ExecutionNeedsYouView{},
		Handover: ExecutionHandoverView{State: s.GetHandover().GetState(), ArchiveAttempts: s.GetHandover().GetArchiveAttempts(), SummaryRef: s.GetHandover().GetSummaryRef()},
	}
	if v.Handover.State == "" {
		v.Handover.State = "none"
	}
	for _, r := range s.GetRunning() {
		v.Running = append(v.Running, ExecutionRunningTaskView{
			TaskID: r.GetTaskId(), PhaseID: r.GetPhaseId(), Title: r.GetTitle(), Attempt: r.GetAttempt(), MaxAttempts: r.GetMaxAttempts(), StartedAt: rfc3339(r.GetStartedAt()),
		})
	}
	for _, f := range s.GetFailed() {
		v.Failed = append(v.Failed, ExecutionFailedTaskView{
			TaskID: f.GetTaskId(), PhaseID: f.GetPhaseId(), Title: f.GetTitle(), Attempts: f.GetAttempts(), MaxAttempts: f.GetMaxAttempts(),
			LastCause: f.GetLastCause(), LastError: f.GetLastError(), LastAt: rfc3339(f.GetLastAt()),
		})
	}
	for _, n := range s.GetNeedsYou() {
		v.NeedsYou = append(v.NeedsYou, ExecutionNeedsYouView{
			Kind: n.GetKind(), RefID: n.GetRefId(), SubjectType: n.GetSubjectType(), Title: n.GetTitle(), PhaseID: n.GetPhaseId(),
			Since: rfc3339(n.GetSince()), DueAt: rfc3339(n.GetDueAt()),
		})
	}
	return v
}

// registerActivity adds the two read channels behind the Request detail screen's History and Execution tabs (CR-REQ-038).
func (c requestChannels) registerActivity(r *Registry) {
	opts := requestChannelOpts{timeout: requestRPCTimeout}
	c.handle(r, "request.activity", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID          string   `json:"id"`
			PageSize    int32    `json:"pageSize"`
			PageToken   string   `json:"pageToken"`
			Kinds       []string `json:"kinds"`
			MinSeverity string   `json:"minSeverity"`
			ActorKind   string   `json:"actorKind"`
			SubjectID   string   `json:"subjectId"`
			Since       string   `json:"since"`
		}](args)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.ID) == "" {
			return nil, invalidRequestArg("id is required")
		}
		req := &requestv1.ListRequestActivityRequest{
			RequestId: in.ID, PageSize: in.PageSize, PageToken: in.PageToken, Kinds: in.Kinds, MinSeverity: in.MinSeverity,
			ActorKind: in.ActorKind, SubjectId: in.SubjectID,
		}
		if in.Since != "" {
			t, perr := time.Parse(time.RFC3339, in.Since)
			if perr != nil {
				return nil, invalidRequestArg("since must be an RFC 3339 time")
			}
			req.Since = timestamppb.New(t)
		}
		resp, err := c.req.ListRequestActivity(ctx, req)
		if err != nil {
			return nil, err
		}
		return struct {
			Entries       []ActivityEntryView `json:"entries"`
			NextPageToken string              `json:"nextPageToken"`
		}{activityEntryViews(resp.GetEntries()), resp.GetNextPageToken()}, nil
	})
	c.handle(r, "request.executionSummary", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[requestIDArgs](args)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.ID) == "" {
			return nil, invalidRequestArg("id is required")
		}
		resp, err := c.req.GetExecutionSummary(ctx, &requestv1.GetExecutionSummaryRequest{RequestId: in.ID})
		if err != nil {
			return nil, err
		}
		return struct {
			Summary ExecutionSummaryView `json:"summary"`
		}{executionSummaryViewOf(resp.GetSummary())}, nil
	})
}
