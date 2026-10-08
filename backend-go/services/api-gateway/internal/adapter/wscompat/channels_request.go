package wscompat

import (
	"context"
	"encoding/json"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// registerRequestChannels registers the Request, Solution, Approval and Backlog
// channels of CONTRACT-request-ui-api.md. request-service enforces permissions
// and the request_flow_enabled flag; the gateway only maps shapes (CONTRACT C10).
// Identity comes from the session only: no argument can carry tenant or user (C2).
func registerRequestChannels(r *Registry, req requestv1.RequestServiceClient, appr requestv1.ApprovalServiceClient, bus ephemeralSubscriber, streamEnabled bool) {
	c := requestChannels{req: req, appr: appr}
	c.registerLifecycle(r)
	c.registerPlan(r)
	c.registerFlow(r)
	c.registerSolution(r)
	c.registerApproval(r)
	c.registerBacklog(r)
	c.registerExtras(r)
	if streamEnabled {
		registerRequestStreamChannel(r, bus, req)
	}
}

type requestIDArgs struct {
	ID string `json:"id"`
}

type requestEnvelope struct {
	Request RequestView `json:"request"`
}

func (c requestChannels) registerLifecycle(r *Registry) {
	read := requestChannelOpts{timeout: requestRPCTimeout}
	c.handle(r, "request.create", read, c.create)
	c.handle(r, "request.get", read, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[requestIDArgs](args)
		if err != nil {
			return nil, err
		}
		resp, err := c.req.GetRequest(ctx, &requestv1.GetRequestRequest{Id: in.ID})
		if err != nil {
			return nil, err
		}
		return requestEnvelope{Request: RequestViewOf(resp.GetRequest(), true)}, nil
	})
	c.handle(r, "request.list", read, c.list)
	c.handle(r, "request.typeHistory", read, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[requestIDArgs](args)
		if err != nil {
			return nil, err
		}
		resp, err := c.req.ListRequestTypeHistory(ctx, &requestv1.ListRequestTypeHistoryRequest{RequestId: in.ID})
		if err != nil {
			return nil, err
		}
		return struct {
			Changes []TypeHistoryEntryView `json:"changes"`
		}{typeHistoryViews(resp.GetChanges())}, nil
	})
	c.handle(r, "request.classify", requestChannelOpts{timeout: requestAIChannelTimeout, ai: true}, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[requestIDArgs](args)
		if err != nil {
			return nil, err
		}
		resp, err := c.req.ClassifyRequest(ctx, &requestv1.ClassifyRequestRequest{RequestId: in.ID})
		if err != nil {
			return nil, err
		}
		return struct {
			Request RequestView `json:"request"`
			RunID   string      `json:"runId,omitempty"`
		}{RequestViewOf(resp.GetRequest(), false), resp.GetRunId()}, nil
	})
	c.handle(r, "request.confirmType", read, c.confirmType)
	c.handle(r, "request.changeType", read, c.changeType)
	c.handle(r, "request.returnToBacklog", read, c.returnToBacklog)
	c.handle(r, "request.reopen", read, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID              string `json:"id"`
			Note            string `json:"note"`
			ExpectedVersion int64  `json:"expectedVersion"`
		}](args)
		if err != nil {
			return nil, err
		}
		resp, err := c.req.ReopenRequest(ctx, &requestv1.ReopenRequestRequest{RequestId: in.ID, Note: in.Note, ExpectedVersion: in.ExpectedVersion})
		if err != nil {
			return nil, err
		}
		return requestEnvelope{RequestViewOf(resp.GetRequest(), false)}, nil
	})
	c.handle(r, "request.cancel", read, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID              string `json:"id"`
			Reason          string `json:"reason"`
			ExpectedVersion int64  `json:"expectedVersion"`
		}](args)
		if err != nil {
			return nil, err
		}
		resp, err := c.req.CancelRequest(ctx, &requestv1.CancelRequestRequest{RequestId: in.ID, Reason: in.Reason, ExpectedVersion: in.ExpectedVersion})
		if err != nil {
			return nil, err
		}
		return requestEnvelope{RequestViewOf(resp.GetRequest(), false)}, nil
	})
	c.handle(r, "request.spawnChild", read, c.spawnChild)
}

type requestHintsArgs struct {
	IssueType string   `json:"issueType"`
	Labels    []string `json:"labels"`
	Priority  string   `json:"priority"`
}

func (c requestChannels) create(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
	in, err := decodeRequestArgs[struct {
		ProjectID       string              `json:"projectId"`
		Title           string              `json:"title"`
		Body            string              `json:"body"`
		Source          *RequestSourceInput `json:"source"`
		Hints           *requestHintsArgs   `json:"hints"`
		ClientRequestID string              `json:"clientRequestId"`
	}](args)
	if err != nil {
		return nil, err
	}
	var src RequestSourceInput
	if in.Source != nil {
		src = *in.Source
	}
	// Source is decided from the verified origin, never from a client claim (CONTRACT 6.1).
	source, err := resolveRequestSource(ctx, src)
	if err != nil {
		return nil, err
	}
	rpc := &requestv1.CreateRequestRequest{
		ProjectId: in.ProjectID, Title: in.Title, Body: in.Body, Source: source, ClientRequestId: in.ClientRequestID,
	}
	if in.Hints != nil {
		rpc.Hints = &requestv1.SourceHints{IssueType: in.Hints.IssueType, Labels: in.Hints.Labels, Priority: in.Hints.Priority}
	}
	resp, err := c.req.CreateRequest(ctx, rpc)
	if err != nil {
		return nil, err
	}
	return struct {
		Request RequestView `json:"request"`
		Created bool        `json:"created"`
	}{RequestViewOf(resp.GetRequest(), false), resp.GetCreated()}, nil
}

func (c requestChannels) list(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
	in, err := decodeRequestArgs[struct {
		ProjectID      string   `json:"projectId"`
		Status         []string `json:"status"`
		Type           []string `json:"type"`
		SourceProvider string   `json:"sourceProvider"`
		SourceSite     string   `json:"sourceSite"`
		SourceRef      string   `json:"sourceRef"`
		PageSize       int32    `json:"pageSize"`
		PageToken      string   `json:"pageToken"`
	}](args)
	if err != nil {
		return nil, err
	}
	if err := checkRequestEnum("status", requestStatuses, in.Status...); err != nil {
		return nil, err
	}
	if err := checkRequestEnum("type", requestTypeSet, in.Type...); err != nil {
		return nil, err
	}
	resp, err := c.req.ListRequests(ctx, &requestv1.ListRequestsRequest{
		ProjectId: in.ProjectID, Status: in.Status, Type: in.Type, PageSize: clampRequestPageSize(in.PageSize),
		PageToken: in.PageToken, SourceProvider: in.SourceProvider, SourceSite: in.SourceSite, SourceRef: in.SourceRef,
	})
	if err != nil {
		return nil, err
	}
	return struct {
		Requests      []RequestView `json:"requests"`
		NextPageToken string        `json:"nextPageToken"`
	}{RequestViews(resp.GetRequests()), resp.GetNextPageToken()}, nil
}

func (c requestChannels) confirmType(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
	in, err := decodeRequestArgs[struct {
		ID              string `json:"id"`
		Type            string `json:"type"`
		Size            string `json:"size"`
		Urgency         string `json:"urgency"`
		Reason          string `json:"reason"`
		ExpectedVersion int64  `json:"expectedVersion"`
	}](args)
	if err != nil {
		return nil, err
	}
	if err := checkRequestEnum("type", requestTypeSet, in.Type); err != nil {
		return nil, err
	}
	resp, err := c.req.ConfirmRequestType(ctx, &requestv1.ConfirmRequestTypeRequest{
		RequestId: in.ID, Type: in.Type, Size: in.Size, Urgency: in.Urgency, Reason: in.Reason, ExpectedVersion: in.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	return requestEnvelope{RequestViewOf(resp.GetRequest(), false)}, nil
}

func (c requestChannels) changeType(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
	in, err := decodeRequestArgs[struct {
		ID              string `json:"id"`
		ToType          string `json:"toType"`
		Size            string `json:"size"`
		Urgency         string `json:"urgency"`
		Reason          string `json:"reason"`
		ExpectedVersion int64  `json:"expectedVersion"`
	}](args)
	if err != nil {
		return nil, err
	}
	if err := checkRequestEnum("toType", requestTypeSet, in.ToType); err != nil {
		return nil, err
	}
	resp, err := c.req.ChangeRequestType(ctx, &requestv1.ChangeRequestTypeRequest{
		RequestId: in.ID, NewType: in.ToType, Size: in.Size, Urgency: in.Urgency, Reason: in.Reason, ExpectedVersion: in.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	return requestEnvelope{RequestViewOf(resp.GetRequest(), false)}, nil
}

func (c requestChannels) returnToBacklog(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
	in, err := decodeRequestArgs[struct {
		ID              string `json:"id"`
		Stage           string `json:"stage"`
		Category        string `json:"category"`
		Reason          string `json:"reason"`
		ExpectedVersion int64  `json:"expectedVersion"`
	}](args)
	if err != nil {
		return nil, err
	}
	if err := checkRequestEnum("stage", requestStageSet, in.Stage); err != nil {
		return nil, err
	}
	resp, err := c.req.ReturnToBacklog(ctx, &requestv1.ReturnToBacklogRequest{
		RequestId: in.ID, Stage: in.Stage, Category: in.Category, Reason: in.Reason, ExpectedVersion: in.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	return requestEnvelope{RequestViewOf(resp.GetRequest(), false)}, nil
}

func (c requestChannels) spawnChild(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
	in, err := decodeRequestArgs[struct {
		ID              string `json:"id"`
		LinkReason      string `json:"linkReason"`
		Title           string `json:"title"`
		Body            string `json:"body"`
		TypeHint        string `json:"typeHint"`
		ClientRequestID string `json:"clientRequestId"`
	}](args)
	if err != nil {
		return nil, err
	}
	if err := checkRequestEnum("linkReason", linkReasonSet, in.LinkReason); err != nil {
		return nil, err
	}
	if err := checkRequestEnum("typeHint", requestTypeSet, in.TypeHint); err != nil {
		return nil, err
	}
	resp, err := c.req.SpawnChildRequest(ctx, &requestv1.SpawnChildRequestRequest{
		ParentRequestId: in.ID, LinkReason: in.LinkReason, Title: in.Title, Body: in.Body,
		TypeHint: in.TypeHint, ClientRequestId: in.ClientRequestID,
	})
	if err != nil {
		return nil, err
	}
	return struct {
		Child   RequestView `json:"child"`
		Created bool        `json:"created"`
	}{RequestViewOf(resp.GetChild(), false), resp.GetCreated()}, nil
}

// RegisterRequestUnaryChannels registers only the request/response Request
// channels. The HTTP routes dispatch through such a registry so the REST and WS
// edges share one set of validation, source rules, views and error shaping.
func RegisterRequestUnaryChannels(r *Registry, req requestv1.RequestServiceClient, appr requestv1.ApprovalServiceClient) {
	registerRequestChannels(r, req, appr, nil, false)
}
