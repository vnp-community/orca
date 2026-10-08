package wscompat

import (
	"encoding/json"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// Views in this file and channels_request_views_flow.go are the only JSON the
// Request channels and HTTP routes emit: camelCase, never raw proto (CONTRACT C6).

// RequestView is CONTRACT RequestView. Body is set only by request.get.
type RequestView struct {
	ID                   string   `json:"id"`
	ProjectID            string   `json:"projectId"`
	Number               int64    `json:"number"`
	Title                string   `json:"title"`
	Body                 *string  `json:"body,omitempty"`
	SourceProvider       string   `json:"sourceProvider"`
	SourceRef            string   `json:"sourceRef,omitempty"`
	SourceURL            string   `json:"sourceUrl,omitempty"`
	SourceSite           string   `json:"sourceSite,omitempty"`
	Type                 *string  `json:"type"`
	TypeSource           *string  `json:"typeSource"`
	Size                 *string  `json:"size"`
	Urgency              *string  `json:"urgency"`
	Confidence           *float64 `json:"confidence"`
	ClassificationReason string   `json:"classificationReason,omitempty"`
	Status               string   `json:"status"`
	ReturnedFromStage    string   `json:"returnedFromStage,omitempty"`
	ReturnCategory       string   `json:"returnCategory,omitempty"`
	ReturnReason         string   `json:"returnReason,omitempty"`
	PlanTaskID           string   `json:"planTaskId,omitempty"`
	ReporterID           string   `json:"reporterId"`
	CreatedAt            string   `json:"createdAt"`
	UpdatedAt            string   `json:"updatedAt"`
	Version              int64    `json:"version"`
}

// RequestViewOf converts a Request; nil yields a zero view so callers never panic on a thin fake.
func RequestViewOf(r *requestv1.Request, withBody bool) RequestView {
	v := RequestView{
		ID: r.GetId(), ProjectID: r.GetProjectId(), Number: r.GetNumber(), Title: r.GetTitle(),
		SourceProvider: r.GetSourceProvider(), SourceRef: r.GetSourceRef(), SourceURL: r.GetSourceUrl(), SourceSite: r.GetSourceSite(),
		Type: nilIfEmpty(r.GetType()), TypeSource: nilIfEmpty(r.GetTypeSource()),
		Size: nilIfEmpty(r.GetSize()), Urgency: nilIfEmpty(r.GetUrgency()),
		ClassificationReason: r.GetClassificationReason(), Status: r.GetStatus(),
		ReturnedFromStage: r.GetReturnedFromStage(), ReturnCategory: r.GetReturnedCategory(), ReturnReason: r.GetReturnReason(),
		PlanTaskID: r.GetPlanTaskId(), ReporterID: r.GetReporterId(),
		CreatedAt: rfc3339(r.GetCreatedAt()), UpdatedAt: rfc3339(r.GetUpdatedAt()), Version: r.GetVersion(),
	}
	if r != nil && r.Confidence != nil {
		c := r.GetConfidence()
		v.Confidence = &c
	}
	if withBody {
		b := r.GetBody()
		v.Body = &b
	}
	return v
}

// RequestViews converts a page of Requests (no body).
func RequestViews(rs []*requestv1.Request) []RequestView {
	out := make([]RequestView, 0, len(rs))
	for _, r := range rs {
		out = append(out, RequestViewOf(r, false))
	}
	return out
}

// TypeHistoryEntryView is CONTRACT TypeHistoryEntryView.
type TypeHistoryEntryView struct {
	FromType  *string `json:"fromType"`
	ToType    string  `json:"toType"`
	ActorID   string  `json:"actorId,omitempty"`
	ActorKind string  `json:"actorKind"`
	Reason    string  `json:"reason,omitempty"`
	At        string  `json:"at"`
}

func typeHistoryViews(cs []*requestv1.RequestTypeChange) []TypeHistoryEntryView {
	out := make([]TypeHistoryEntryView, 0, len(cs))
	for _, c := range cs {
		out = append(out, TypeHistoryEntryView{
			FromType: nilIfEmpty(c.GetFromType()), ToType: c.GetToType(), ActorID: c.GetActorId(),
			ActorKind: c.GetActorKind(), Reason: c.GetReason(), At: rfc3339(c.GetAt()),
		})
	}
	return out
}

// SolutionView is CONTRACT SolutionView. Options is relayed verbatim (C13).
type SolutionView struct {
	ID              string          `json:"id"`
	RequestID       string          `json:"requestId"`
	Kind            string          `json:"kind"`
	Status          string          `json:"status"`
	Options         json.RawMessage `json:"options"`
	ChosenOption    int32           `json:"chosenOption"`
	ChosenOptionID  string          `json:"chosenOptionId,omitempty"`
	GenerationRunID string          `json:"generationRunId,omitempty"`
	CreatedAt       string          `json:"createdAt"`
	Version         int64           `json:"version"`
}

// SolutionViewOf converts a Solution.
func SolutionViewOf(s *requestv1.Solution) SolutionView {
	v := SolutionView{
		ID: s.GetId(), RequestID: s.GetRequestId(), Kind: enumLower(s.GetKind().String(), "SOLUTION_KIND_"),
		Status:  enumLower(s.GetStatus().String(), "SOLUTION_STATUS_"),
		Options: json.RawMessage("null"), ChosenOption: s.GetChosenOption(),
		GenerationRunID: s.GetGenerationRunId(), CreatedAt: rfc3339(s.GetCreatedAt()), Version: s.GetVersion(),
	}
	if raw := strings.TrimSpace(s.GetOptionsJson()); raw != "" && json.Valid([]byte(raw)) {
		v.Options = json.RawMessage(raw)
		v.ChosenOptionID = chosenOptionID(raw, s.GetChosenOption())
	}
	return v
}

// chosenOptionID reads options.options[i].id; any parse problem drops the field, not the channel.
func chosenOptionID(optionsJSON string, chosen int32) string {
	if chosen < 0 {
		return ""
	}
	var doc struct {
		Options []struct {
			ID string `json:"id"`
		} `json:"options"`
	}
	if err := json.Unmarshal([]byte(optionsJSON), &doc); err != nil || int(chosen) >= len(doc.Options) {
		return ""
	}
	return doc.Options[chosen].ID
}

// AnalysisRunView is CONTRACT AnalysisRunView.
type AnalysisRunView struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Status       string `json:"status"`
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	StartedAt    string `json:"startedAt"`
	FinishedAt   string `json:"finishedAt,omitempty"`
}

func solutionViews(ss []*requestv1.Solution) []SolutionView {
	out := make([]SolutionView, 0, len(ss))
	for _, s := range ss {
		out = append(out, SolutionViewOf(s))
	}
	return out
}

func analysisRunViews(rs []*requestv1.AnalysisRun) []AnalysisRunView {
	out := make([]AnalysisRunView, 0, len(rs))
	for _, r := range rs {
		out = append(out, AnalysisRunView{
			ID: r.GetId(), Kind: enumLower(r.GetKind().String(), "SOLUTION_KIND_"), Status: r.GetStatus(),
			ErrorCode: r.GetErrorCode(), ErrorMessage: r.GetErrorMessage(),
			StartedAt: rfc3339(r.GetStartedAt()), FinishedAt: rfc3339(r.GetFinishedAt()),
		})
	}
	return out
}

// ApprovalView is CONTRACT ApprovalView. The request* members are the additive
// inbox fields ListPendingForUser fills (CONTRACT Q3).
type ApprovalView struct {
	ID            string `json:"id"`
	RequestID     string `json:"requestId"`
	SubjectType   string `json:"subjectType"`
	SubjectID     string `json:"subjectId"`
	Stage         string `json:"stage"`
	Status        string `json:"status"`
	RequestedBy   string `json:"requestedBy"`
	DecidedBy     string `json:"decidedBy,omitempty"`
	DecidedAt     string `json:"decidedAt,omitempty"`
	Comment       string `json:"comment,omitempty"`
	DueAt         string `json:"dueAt,omitempty"`
	Version       int64  `json:"version"`
	SubjectDigest string `json:"subjectDigest"`
	CreatedAt     string `json:"createdAt"`
	RequestTitle  string `json:"requestTitle,omitempty"`
	RequestType   string `json:"requestType,omitempty"`
	RequestNumber int64  `json:"requestNumber,omitempty"`
}

// ApprovalViewOf converts an Approval.
func ApprovalViewOf(a *requestv1.Approval) ApprovalView {
	return ApprovalView{
		ID: a.GetId(), RequestID: a.GetRequestId(),
		SubjectType: enumLower(a.GetSubjectType().String(), "APPROVAL_SUBJECT_TYPE_"), SubjectID: a.GetSubjectId(),
		Stage: a.GetStage(), Status: enumLower(a.GetStatus().String(), "APPROVAL_STATUS_"),
		RequestedBy: a.GetRequestedBy(), DecidedBy: a.GetDecidedBy(), DecidedAt: rfc3339(a.GetDecidedAt()),
		Comment: a.GetComment(), DueAt: rfc3339(a.GetDueAt()), Version: a.GetVersion(),
		SubjectDigest: a.GetSubjectDigest(), CreatedAt: rfc3339(a.GetCreatedAt()),
		RequestTitle: a.GetRequestTitle(), RequestType: a.GetRequestType(), RequestNumber: a.GetRequestNumber(),
	}
}

// ApprovalViews converts a page of Approvals.
func ApprovalViews(as []*requestv1.Approval) []ApprovalView {
	out := make([]ApprovalView, 0, len(as))
	for _, a := range as {
		out = append(out, ApprovalViewOf(a))
	}
	return out
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// enumLower turns SOLUTION_KIND_SOLUTION into "solution"; UNSPECIFIED and unknown values become "".
func enumLower(name, prefix string) string {
	if !strings.HasPrefix(name, prefix) {
		return ""
	}
	v := strings.TrimPrefix(name, prefix)
	if v == "UNSPECIFIED" {
		return ""
	}
	return strings.ToLower(v)
}

func rfc3339(ts *timestamppb.Timestamp) string {
	if ts == nil {
		return ""
	}
	return ts.AsTime().UTC().Format(time.RFC3339)
}
