package wscompat

import (
	"encoding/json"
	"strings"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// RequestLinkView is CONTRACT RequestLinkView.
type RequestLinkView struct {
	ParentRequestID string `json:"parentRequestId"`
	ChildRequestID  string `json:"childRequestId"`
	Reason          string `json:"reason"`
}

func requestLinkViews(ls []*requestv1.RequestLink) []RequestLinkView {
	out := make([]RequestLinkView, 0, len(ls))
	for _, l := range ls {
		out = append(out, RequestLinkView{ParentRequestID: l.GetParentRequestId(), ChildRequestID: l.GetChildRequestId(), Reason: l.GetReason()})
	}
	return out
}

// RequestFlowView mirrors GetRequestFlowResponse so the UI need not copy the flow registry.
type RequestFlowView struct {
	Type                   string   `json:"type"`
	HumanConfirmRequired   bool     `json:"humanConfirmRequired"`
	AnalysisKind           string   `json:"analysisKind"`
	AnalysisGate           string   `json:"analysisGate"`
	PlanKind               string   `json:"planKind"`
	HasPhases              bool     `json:"hasPhases"`
	StartGate              string   `json:"startGate"`
	ExecutionGates         []string `json:"executionGates"`
	CompletesAfterAnalysis bool     `json:"completesAfterAnalysis"`
	StatusPath             []string `json:"statusPath"`
}

func requestFlowViewOf(f *requestv1.GetRequestFlowResponse) RequestFlowView {
	return RequestFlowView{
		Type: f.GetType(), HumanConfirmRequired: f.GetHumanConfirmRequired(), AnalysisKind: f.GetAnalysisKind(),
		AnalysisGate: f.GetAnalysisGate(), PlanKind: f.GetPlanKind(), HasPhases: f.GetHasPhases(),
		StartGate: f.GetStartGate(), ExecutionGates: nonNilStrings(f.GetExecutionGates()),
		CompletesAfterAnalysis: f.GetCompletesAfterAnalysis(), StatusPath: nonNilStrings(f.GetStatusPath()),
	}
}

// RequestCheckView is one per-type check; Metrics is the stored JSON document relayed as is.
type RequestCheckView struct {
	ID         string          `json:"id"`
	RequestID  string          `json:"requestId"`
	Kind       string          `json:"kind"`
	Status     string          `json:"status"`
	Metrics    json.RawMessage `json:"metrics"`
	Summary    string          `json:"summary,omitempty"`
	Source     string          `json:"source,omitempty"`
	TaskID     string          `json:"taskId,omitempty"`
	RecordedBy string          `json:"recordedBy,omitempty"`
	CreatedAt  string          `json:"createdAt"`
}

func requestCheckViews(cs []*requestv1.RequestCheck) []RequestCheckView {
	out := make([]RequestCheckView, 0, len(cs))
	for _, c := range cs {
		m := json.RawMessage("null")
		if raw := strings.TrimSpace(c.GetMetricsJson()); raw != "" && json.Valid([]byte(raw)) {
			m = json.RawMessage(raw)
		}
		out = append(out, RequestCheckView{
			ID: c.GetId(), RequestID: c.GetRequestId(), Kind: c.GetKind(), Status: c.GetStatus(), Metrics: m,
			Summary: c.GetSummary(), Source: c.GetSource(), TaskID: c.GetTaskId(), RecordedBy: c.GetRecordedBy(),
			CreatedAt: rfc3339(c.GetCreatedAt()),
		})
	}
	return out
}

// BacklogRequestRowView is CONTRACT BacklogRequestRowView.
type BacklogRequestRowView struct {
	RequestID         string   `json:"requestId"`
	Number            int64    `json:"number"`
	Title             string   `json:"title"`
	Type              *string  `json:"type"`
	SourceProvider    string   `json:"sourceProvider"`
	SourceRef         string   `json:"sourceRef,omitempty"`
	SourceURL         string   `json:"sourceUrl,omitempty"`
	ReturnedFromStage string   `json:"returnedFromStage"`
	ReturnedCategory  string   `json:"returnedCategory,omitempty"`
	ReturnReason      string   `json:"returnReason,omitempty"`
	ReturnedBy        string   `json:"returnedBy,omitempty"`
	ReturnedAt        string   `json:"returnedAt"`
	ParentRequestIDs  []string `json:"parentRequestIds"`
}

// BacklogTaskRowView is CONTRACT BacklogTaskRowView.
type BacklogTaskRowView struct {
	TaskID          string   `json:"taskId"`
	Title           string   `json:"title"`
	Status          string   `json:"status"`
	EstimatedHours  *float64 `json:"estimatedHours,omitempty"`
	AssigneeID      string   `json:"assigneeId,omitempty"`
	BlockedByTaskID []string `json:"blockedByTaskIds"`
	LastEngine      string   `json:"lastEngine,omitempty"`
	LastLinkStatus  string   `json:"lastLinkStatus,omitempty"`
	FailedAttempts  int32    `json:"failedAttempts"`
	LastError       string   `json:"lastError,omitempty"`
}

// BacklogGroupView is CONTRACT BacklogGroupView.
type BacklogGroupView struct {
	RequestID   string               `json:"requestId"`
	PlanTaskID  string               `json:"planTaskId,omitempty"`
	PlanTitle   string               `json:"planTitle,omitempty"`
	PhaseTaskID string               `json:"phaseTaskId,omitempty"`
	PhaseTitle  string               `json:"phaseTitle,omitempty"`
	GateStatus  string               `json:"gateStatus,omitempty"`
	Tasks       []BacklogTaskRowView `json:"tasks"`
}

func backlogRequestRowViews(rows []*requestv1.BacklogRequestRow) []BacklogRequestRowView {
	out := make([]BacklogRequestRowView, 0, len(rows))
	for _, r := range rows {
		out = append(out, BacklogRequestRowView{
			RequestID: r.GetId(), Number: r.GetNumber(), Title: r.GetTitle(), Type: nilIfEmpty(r.GetType()),
			SourceProvider: r.GetSourceProvider(), SourceRef: r.GetSourceRef(), SourceURL: r.GetSourceUrl(),
			ReturnedFromStage: r.GetStage(), ReturnedCategory: r.GetReturnedCategory(), ReturnReason: r.GetReturnReason(),
			ReturnedBy: r.GetReturnedBy(), ReturnedAt: rfc3339(r.GetReturnedAt()), ParentRequestIDs: nonNilStrings(r.GetParentRequestIds()),
		})
	}
	return out
}

func backlogGroupViews(gs []*requestv1.BacklogGroup) []BacklogGroupView {
	out := make([]BacklogGroupView, 0, len(gs))
	for _, g := range gs {
		v := BacklogGroupView{
			RequestID: g.GetRequestId(), PlanTaskID: g.GetPlanId(), PlanTitle: g.GetPlanTitle(),
			PhaseTaskID: g.GetPhaseId(), PhaseTitle: g.GetPhaseTitle(), GateStatus: g.GetGateStatus(),
			Tasks: make([]BacklogTaskRowView, 0, len(g.GetTasks())),
		}
		for _, t := range g.GetTasks() {
			row := BacklogTaskRowView{
				TaskID: t.GetId(), Title: t.GetTitle(), Status: t.GetStatus(), AssigneeID: t.GetAssigneeId(),
				BlockedByTaskID: nonNilStrings(t.GetBlockedByTaskIds()), LastEngine: t.GetLastEngine(),
				LastLinkStatus: t.GetLastLinkStatus(), FailedAttempts: t.GetFailedAttempts(), LastError: t.GetLastError(),
			}
			if t.GetEstimatedHours() != nil {
				h := t.GetEstimatedHours().GetValue()
				row.EstimatedHours = &h
			}
			v.Tasks = append(v.Tasks, row)
		}
		out = append(out, v)
	}
	return out
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
