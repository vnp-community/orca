package wscompat

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	commoneventbus "github.com/stablyai/orca-go/common/eventbus"
	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// requestStreamName is the JetStream stream request-service publishes to
// (its classification consumer reads the same name); one place to change.
const requestStreamName = "REQUEST"

// requestReplayGrace lets slightly early events through; older ones are replayed
// history from a fresh ephemeral consumer (JetStream delivers from the start).
const requestReplayGrace = 2 * time.Second

// RequestEventFrame is the push payload of request.event (CONTRACT section 3).
// Only whitelisted ids and states: never title, body or solution content (C9).
type RequestEventFrame struct {
	RequestID   string `json:"requestId"`
	EventType   string `json:"eventType"`
	Status      string `json:"status,omitempty"`
	Type        string `json:"type,omitempty"`
	OccurredAt  string `json:"occurredAt"`
	Trigger     string `json:"trigger,omitempty"`
	ApprovalID  string `json:"approvalId,omitempty"`
	SubjectType string `json:"subjectType,omitempty"`
	SolutionID  string `json:"solutionId,omitempty"`
	PhaseTaskID string `json:"phaseTaskId,omitempty"`
	PlanTaskID  string `json:"planTaskId,omitempty"`
}

// requestEventFields is the superset of payload keys the registry understands;
// ReporterID and ActorID are used for visibility only and never forwarded.
type requestEventFields struct {
	RequestID   string `json:"request_id"`
	To          string `json:"to"`
	Status      string `json:"status"`
	Type        string `json:"type"`
	Trigger     string `json:"trigger"`
	ApprovalID  string `json:"approval_id"`
	SubjectType string `json:"subject_type"`
	SolutionID  string `json:"solution_id"`
	PhaseTaskID string `json:"phase_task_id"`
	PlanTaskID  string `json:"plan_task_id"`
	ReporterID  string `json:"reporter_id"`
	ActorID     string `json:"actor_id"`
}

// requestEventMapping maps one NATS subject to a CONTRACT eventType. Extract is
// optional: nil uses the shared decoder, a row overrides it for a divergent payload.
type requestEventMapping struct {
	Stream, Subject, EventType string
	Extract                    func(json.RawMessage) (requestEventFields, bool)
}

// requestEventRegistry is the single table to extend when a CR adds an event
// (CONTRACT section 7): one new row, no change to the stream logic.
var requestEventRegistry = []requestEventMapping{
	{Stream: requestStreamName, Subject: "orca.request.request.created", EventType: "request.created"},
	{Stream: requestStreamName, Subject: "orca.request.request.classified", EventType: "request.classified"},
	{Stream: requestStreamName, Subject: "orca.request.request.type_confirmed", EventType: "request.type_confirmed"},
	{Stream: requestStreamName, Subject: "orca.request.request.type_changed", EventType: "request.type_changed"},
	{Stream: requestStreamName, Subject: "orca.request.request.status_changed", EventType: "request.status_changed"},
	{Stream: requestStreamName, Subject: "orca.request.request.returned", EventType: "request.returned"},
	{Stream: requestStreamName, Subject: "orca.request.request.completed", EventType: "request.completed"},
	{Stream: requestStreamName, Subject: "orca.request.solution.proposed", EventType: "solution.proposed"},
	{Stream: requestStreamName, Subject: "orca.request.solution.approved", EventType: "solution.approved"},
	{Stream: requestStreamName, Subject: "orca.request.approval.requested", EventType: "approval.requested"},
	{Stream: requestStreamName, Subject: "orca.request.approval.decided", EventType: "approval.decided"},
	{Stream: requestStreamName, Subject: "orca.request.plan.generated", EventType: "plan.generated"},
	{Stream: requestStreamName, Subject: "orca.request.phase.started", EventType: "phase.started"},
	{Stream: requestStreamName, Subject: "orca.request.phase.completed", EventType: "phase.completed"},
}

func extractRequestEventFields(raw json.RawMessage) (requestEventFields, bool) {
	var f requestEventFields
	if err := json.Unmarshal(raw, &f); err != nil || f.RequestID == "" {
		return f, false
	}
	return f, true
}

// registerRequestStreamChannel registers request.subscribe. Only called when NATS
// is connected; the UI otherwise falls back to polling (CONTRACT section 3).
func registerRequestStreamChannel(r *Registry, bus ephemeralSubscriber, req requestv1.RequestServiceClient) {
	registerRequestStream(r, bus, req, requestEventRegistry, time.Now)
}

func registerRequestStream(r *Registry, bus ephemeralSubscriber, req requestv1.RequestServiceClient, events []requestEventMapping, now func() time.Time) {
	r.RegisterStream("request.subscribe", func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
		in, err := decodeRequestArgs[requestIDArgs](args)
		if err != nil {
			return nil, err
		}
		if bus == nil {
			return nil, errRequestUnavailable
		}
		if in.ID != "" {
			if requestClientMissing(req) {
				return nil, errRequestUnavailable
			}
			// Streams bypass Registry.Dispatch, so identity is attached here for the permission probe.
			probe, cancel := context.WithTimeout(gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID, Role: id.Role}), requestRPCTimeout)
			defer cancel()
			if _, err := req.GetRequest(probe, &requestv1.GetRequestRequest{Id: in.ID}); err != nil {
				return nil, requestChannelError(err)
			}
		}
		subscribedAt := now()
		out := make(chan PushEvent)
		go func() {
			defer close(out)
			var wg sync.WaitGroup
			for _, m := range events {
				m := m
				wg.Add(1)
				go func() {
					defer wg.Done()
					_ = bus.SubscribeEphemeral(ctx, m.Stream, m.Subject, func(ctx context.Context, ev commoneventbus.Event) error {
						frame, ok := translateRequestEvent(m, ev, id, in.ID, subscribedAt)
						if !ok {
							return nil
						}
						select {
						case out <- PushEvent{Channel: "request.event", Args: []any{frame}}:
						case <-ctx.Done():
						}
						return nil
					})
				}()
			}
			wg.Wait()
		}()
		return out, nil
	})
}

// translateRequestEvent applies the delivery rules: same tenant, not replayed
// history, matching id when one was given, and visible to this user.
func translateRequestEvent(m requestEventMapping, ev commoneventbus.Event, id Identity, wantID string, subscribedAt time.Time) (RequestEventFrame, bool) {
	if ev.TenantID != id.TenantID || ev.OccurredAt.Before(subscribedAt.Add(-requestReplayGrace)) {
		return RequestEventFrame{}, false
	}
	extract := m.Extract
	if extract == nil {
		extract = extractRequestEventFields
	}
	f, ok := extract(ev.Payload)
	if !ok || (wantID != "" && f.RequestID != wantID) {
		return RequestEventFrame{}, false
	}
	// Tenant-wide feed: until read policy per project is settled (CONTRACT Q3), a
	// member sees only events they reported or caused; admins see all.
	if wantID == "" && id.Role != roleAdmin && (id.UserID == "" || (f.ReporterID != id.UserID && f.ActorID != id.UserID)) {
		return RequestEventFrame{}, false
	}
	status := f.To
	if status == "" {
		status = f.Status
	}
	return RequestEventFrame{
		RequestID: f.RequestID, EventType: m.EventType, Status: status, Type: f.Type,
		OccurredAt: ev.OccurredAt.UTC().Format(time.RFC3339), Trigger: f.Trigger, ApprovalID: f.ApprovalID,
		SubjectType: f.SubjectType, SolutionID: f.SolutionID, PhaseTaskID: f.PhaseTaskID, PlanTaskID: f.PlanTaskID,
	}, true
}
