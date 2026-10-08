package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"go.opentelemetry.io/otel/propagation"
)

// StatusChangedPayload is the wire shape of orca.request.request.status_changed.
// version and number let consumers order and dedupe; category is set only for backlog returns.
// source_*, reporter_id and traceparent let issue-status-sync pick the Jira issue and credential
// without calling back (CR-REQ-024 section 2.1); the issue is the Request's own source, no title or body.
type StatusChangedPayload struct {
	RequestID string `json:"request_id"`
	ProjectID string `json:"project_id"`
	Number    int64  `json:"number"`
	From      string `json:"from"`
	To        string `json:"to"`
	Trigger   string `json:"trigger"`
	Type      string `json:"type"`
	ActorID   string `json:"actor_id"`
	ActorKind string `json:"actor_kind"`
	Stage     string `json:"stage"`
	Category  string `json:"category"`
	Reason    string `json:"reason"`
	// ResumeStatus is set only on information_provided (additive field).
	ResumeStatus   string `json:"resume_status,omitempty"`
	Version        int64  `json:"version"`
	At             string `json:"at"`
	SourceProvider string `json:"source_provider"`
	SourceSite     string `json:"source_site"`
	SourceRef      string `json:"source_ref"`
	ReporterID     string `json:"reporter_id"`
	TraceParent    string `json:"traceparent,omitempty"`
}

// CompletedPayload is the wire shape of orca.request.request.completed; it carries the same
// issue-identifying fields as status_changed so a consumer can handle either subject alike.
type CompletedPayload struct {
	RequestID      string `json:"request_id"`
	ProjectID      string `json:"project_id"`
	Number         int64  `json:"number"`
	Type           string `json:"type"`
	At             string `json:"at"`
	Version        int64  `json:"version"`
	ActorID        string `json:"actor_id"`
	ActorKind      string `json:"actor_kind"`
	SourceProvider string `json:"source_provider"`
	SourceSite     string `json:"source_site"`
	SourceRef      string `json:"source_ref"`
	ReporterID     string `json:"reporter_id"`
	TraceParent    string `json:"traceparent,omitempty"`
}

// traceparentOf is the W3C traceparent of the span in ctx, empty when there is none. It rides in the
// payload because common/eventbus has no header slot for it.
func traceparentOf(ctx context.Context) string {
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	return carrier.Get("traceparent")
}

func statusChangedPayload(ctx context.Context, r domain.Request, from domain.RequestStatus, in TransitionInput, stage domain.ReturnStage, at time.Time) StatusChangedPayload {
	p := StatusChangedPayload{
		RequestID: r.ID, ProjectID: r.ProjectID, Number: r.Number, From: string(from), To: string(r.Status),
		Trigger: string(in.Trigger), Type: string(r.Type), ActorID: in.ActorID, ActorKind: string(in.ActorKind),
		Stage: string(stage), Reason: in.Reason, Version: r.Version, At: at.UTC().Format(time.RFC3339Nano),
		SourceProvider: string(r.SourceProvider), SourceSite: r.SourceSite, SourceRef: r.SourceRef, ReporterID: r.ReporterID,
		TraceParent: traceparentOf(ctx),
	}
	if in.Trigger == domain.TriggerInformationProvided {
		p.ResumeStatus = string(in.ResumeStatus)
	}
	if r.Status == domain.RequestStatusRequestBacklog {
		p.Category = string(r.ReturnedCategory)
	} else {
		p.Reason = ""
		if in.Trigger == domain.TriggerCancel {
			p.Reason = in.Reason
		}
	}
	return p
}

func completedPayload(ctx context.Context, r domain.Request, in TransitionInput, at time.Time) CompletedPayload {
	return CompletedPayload{
		RequestID: r.ID, ProjectID: r.ProjectID, Number: r.Number, Type: string(r.Type), At: at.UTC().Format(time.RFC3339Nano),
		Version: r.Version, ActorID: in.ActorID, ActorKind: string(in.ActorKind),
		SourceProvider: string(r.SourceProvider), SourceSite: r.SourceSite, SourceRef: r.SourceRef, ReporterID: r.ReporterID,
		TraceParent: traceparentOf(ctx),
	}
}
