package wscompat

import (
	"context"
	"encoding/json"
	"strings"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// The Outcome Review channels (CR-OR-001, 002, 010 to 013, 022). request-service decides who may do what and whether the review is
// on; the gateway maps shapes. The model of a review travels as a JSON object, not as a string inside JSON.

// OutcomeReviewView is a review as a client reads it. Model is the review itself (schemaVersion 1); the other fields are what a list shows.
type OutcomeReviewView struct {
	ReviewID     string          `json:"reviewId"`
	RequestID    string          `json:"requestId"`
	Phase        string          `json:"phase"`
	State        string          `json:"state"`
	HeadKey      string          `json:"headKey"`
	Model        json.RawMessage `json:"model"`
	KGSnapshot   string          `json:"kgSnapshot,omitempty"`
	Version      int64           `json:"version"`
	CreatedAt    string          `json:"createdAt"`
	UpdatedAt    string          `json:"updatedAt"`
	ApprovedBy   string          `json:"approvedBy,omitempty"`
	ApprovedAt   string          `json:"approvedAt,omitempty"`
	ApprovalNote string          `json:"approvalNote,omitempty"`
}

func outcomeReviewView(r *requestv1.OutcomeReview) OutcomeReviewView {
	model := json.RawMessage(r.GetModelJson())
	if len(strings.TrimSpace(string(model))) == 0 || !json.Valid(model) {
		model = json.RawMessage("{}")
	}
	return OutcomeReviewView{
		ReviewID: r.GetReviewId(), RequestID: r.GetRequestId(), Phase: r.GetPhase(), State: r.GetState(), HeadKey: r.GetHeadKey(), Model: model,
		KGSnapshot: r.GetKgSnapshot(), Version: r.GetVersion(), CreatedAt: rfc3339(r.GetCreatedAt()), UpdatedAt: rfc3339(r.GetUpdatedAt()),
		ApprovedBy: r.GetApprovedBy(), ApprovedAt: rfc3339(r.GetApprovedAt()), ApprovalNote: r.GetApprovalNote(),
	}
}

type OutcomeReviewRunView struct {
	RunID        string `json:"runId"`
	ReviewID     string `json:"reviewId"`
	Kind         string `json:"kind"`
	Status       string `json:"status"`
	Scope        string `json:"scope"`
	ErrorCode    string `json:"errorCode,omitempty"`
	CostEstimate string `json:"costEstimate,omitempty"`
	CostActual   string `json:"costActual,omitempty"`
	StartedAt    string `json:"startedAt"`
	FinishedAt   string `json:"finishedAt,omitempty"`
}

func outcomeReviewRunView(r *requestv1.OutcomeReviewRun) *OutcomeReviewRunView {
	if r == nil {
		return nil
	}
	return &OutcomeReviewRunView{
		RunID: r.GetRunId(), ReviewID: r.GetReviewId(), Kind: r.GetKind(), Status: r.GetStatus(), Scope: r.GetScope(), ErrorCode: r.GetErrorCode(),
		CostEstimate: r.GetCostEstimate(), CostActual: r.GetCostActual(), StartedAt: rfc3339(r.GetStartedAt()), FinishedAt: rfc3339(r.GetFinishedAt()),
	}
}

type RegenerationEstimateView struct {
	AgentRuns              int32   `json:"agentRuns"`
	DiagramsToDraw         int32   `json:"diagramsToDraw"`
	CostKnown              bool    `json:"costKnown"`
	CostUSD                float64 `json:"costUsd,omitempty"`
	LastContextCost        string  `json:"lastContextCost,omitempty"`
	LastDrawCost           string  `json:"lastDrawCost,omitempty"`
	ConfirmRequired        bool    `json:"confirmRequired"`
	DrawingsDependOnResult bool    `json:"drawingsDependOnResult"`
}

func regenerationEstimateView(e *requestv1.RegenerationEstimate) *RegenerationEstimateView {
	if e == nil {
		return nil
	}
	return &RegenerationEstimateView{
		AgentRuns: e.GetAgentRuns(), DiagramsToDraw: e.GetDiagramsToDraw(), CostKnown: e.GetCostKnown(), CostUSD: e.GetCostUsd(),
		LastContextCost: e.GetLastContextCost(), LastDrawCost: e.GetLastDrawCost(), ConfirmRequired: e.GetConfirmRequired(), DrawingsDependOnResult: e.GetDrawingsDependOnResult(),
	}
}

type ReviewArtifactMetaView struct {
	ArtifactID   string `json:"artifactId"`
	Section      string `json:"section"`
	Variant      string `json:"variant"`
	Name         string `json:"name,omitempty"`
	DiagramType  string `json:"diagramType"`
	InputDigest  string `json:"inputDigest"`
	PresetDigest string `json:"presetDigest"`
	SkillVersion string `json:"skillVersion"`
	Revision     int32  `json:"revision"`
	State        string `json:"state"`
	Origin       string `json:"origin"`
	Reason       string `json:"reason,omitempty"`
	ErrorCode    string `json:"errorCode,omitempty"`
	BlobDigest   string `json:"blobDigest,omitempty"`
	SizeBytes    int32  `json:"sizeBytes"`
	ViewBox      string `json:"viewBox,omitempty"`
	Title        string `json:"title,omitempty"`
	Desc         string `json:"desc,omitempty"`
	CreatedAt    string `json:"createdAt"`
}

func reviewArtifactMetaView(a *requestv1.ReviewArtifactMeta) *ReviewArtifactMetaView {
	if a == nil {
		return nil
	}
	return &ReviewArtifactMetaView{
		ArtifactID: a.GetArtifactId(), Section: a.GetSection(), Variant: a.GetVariant(), Name: a.GetName(), DiagramType: a.GetDiagramType(),
		InputDigest: a.GetInputDigest(), PresetDigest: a.GetPresetDigest(), SkillVersion: a.GetSkillVersion(), Revision: a.GetRevision(), State: a.GetState(),
		Origin: a.GetOrigin(), Reason: a.GetReason(), ErrorCode: a.GetErrorCode(), BlobDigest: a.GetBlobDigest(), SizeBytes: a.GetSizeBytes(),
		ViewBox: a.GetViewBox(), Title: a.GetTitle(), Desc: a.GetDesc(), CreatedAt: rfc3339(a.GetCreatedAt()),
	}
}

type ModelDiffView struct {
	AddedNodes   []string `json:"addedNodes"`
	RemovedNodes []string `json:"removedNodes"`
	ChangedNodes []string `json:"changedNodes"`
	AddedEdges   []string `json:"addedEdges"`
	RemovedEdges []string `json:"removedEdges"`
	ChangedEdges []string `json:"changedEdges"`
}

func modelDiffView(d *requestv1.ModelDiff) *ModelDiffView {
	if d == nil {
		return nil
	}
	return &ModelDiffView{
		AddedNodes: nonNilStrings(d.GetAddedNodes()), RemovedNodes: nonNilStrings(d.GetRemovedNodes()), ChangedNodes: nonNilStrings(d.GetChangedNodes()),
		AddedEdges: nonNilStrings(d.GetAddedEdges()), RemovedEdges: nonNilStrings(d.GetRemovedEdges()), ChangedEdges: nonNilStrings(d.GetChangedEdges()),
	}
}

type ReviewDecisionView struct {
	DecisionID     string `json:"decisionId"`
	ReviewID       string `json:"reviewId"`
	ItemKey        string `json:"itemKey"`
	Decision       string `json:"decision"`
	Note           string `json:"note,omitempty"`
	ChildRequestID string `json:"childRequestId,omitempty"`
	DecidedBy      string `json:"decidedBy"`
	DecidedAt      string `json:"decidedAt"`
	Active         bool   `json:"active"`
	Orphaned       bool   `json:"orphaned"`
}

func reviewDecisionView(d *requestv1.OutcomeReviewDecision) *ReviewDecisionView {
	if d == nil {
		return nil
	}
	return &ReviewDecisionView{
		DecisionID: d.GetDecisionId(), ReviewID: d.GetReviewId(), ItemKey: d.GetItemKey(), Decision: d.GetDecision(), Note: d.GetNote(),
		ChildRequestID: d.GetChildRequestId(), DecidedBy: d.GetDecidedBy(), DecidedAt: rfc3339(d.GetDecidedAt()), Active: d.GetActive(), Orphaned: d.GetOrphaned(),
	}
}

type ReviewProfileLayerView struct {
	ProjectID  string          `json:"projectId"`
	Spec       json.RawMessage `json:"spec"`
	SpecDigest string          `json:"specDigest"`
	Version    int64           `json:"version"`
	UpdatedBy  string          `json:"updatedBy"`
	UpdatedAt  string          `json:"updatedAt"`
}

func reviewProfileLayerView(l *requestv1.ReviewDrawingProfileLayer) *ReviewProfileLayerView {
	if l == nil {
		return nil
	}
	spec := json.RawMessage(l.GetSpecJson())
	if !json.Valid(spec) {
		spec = json.RawMessage("{}")
	}
	return &ReviewProfileLayerView{
		ProjectID: l.GetProjectId(), Spec: spec, SpecDigest: l.GetSpecDigest(), Version: l.GetVersion(), UpdatedBy: l.GetUpdatedBy(), UpdatedAt: rfc3339(l.GetUpdatedAt()),
	}
}

func requireRequestID(id string) error {
	if strings.TrimSpace(id) == "" {
		return invalidRequestArg("id is required")
	}
	return nil
}

// registerReview adds the Outcome Review channels. A review is always read through its Request: the id every channel takes is the
// Request's, which is also how request-service authorizes the call.
func (c requestChannels) registerReview(r *Registry) {
	opts := requestChannelOpts{timeout: requestRPCTimeout}

	c.handle(r, "request.review.estimate", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID       string   `json:"id"`
			Scope    string   `json:"scope"`
			Sections []string `json:"sections"`
		}](args)
		if err != nil {
			return nil, err
		}
		if err := requireRequestID(in.ID); err != nil {
			return nil, err
		}
		resp, err := c.req.EstimateOutcomeReview(ctx, &requestv1.EstimateOutcomeReviewRequest{RequestId: in.ID, Scope: in.Scope, Sections: in.Sections})
		if err != nil {
			return nil, err
		}
		return struct {
			Phase               string                    `json:"phase"`
			ExecutionFinished   bool                      `json:"executionFinished"`
			ReviewExists        bool                      `json:"reviewExists"`
			ReviewID            string                    `json:"reviewId,omitempty"`
			Stale               bool                      `json:"stale"`
			CurrentInputsDigest string                    `json:"currentInputsDigest"`
			Warnings            []string                  `json:"warnings"`
			ContextCached       bool                      `json:"contextCached"`
			DesignLocked        bool                      `json:"designLocked"`
			ContextRunnable     bool                      `json:"contextRunnable"`
			ContextGaps         []string                  `json:"contextGaps"`
			Regeneration        *RegenerationEstimateView `json:"regeneration,omitempty"`
		}{
			Phase: resp.GetPhase(), ExecutionFinished: resp.GetExecutionFinished(), ReviewExists: resp.GetReviewExists(), ReviewID: resp.GetReviewId(), Stale: resp.GetStale(),
			CurrentInputsDigest: resp.GetCurrentInputsDigest(), Warnings: nonNilStrings(resp.GetWarnings()), ContextCached: resp.GetContextCached(), DesignLocked: resp.GetDesignLocked(),
			ContextRunnable: resp.GetContextRunnable(), ContextGaps: nonNilStrings(resp.GetContextGaps()), Regeneration: regenerationEstimateView(resp.GetRegeneration()),
		}, nil
	})

	c.handle(r, "request.review.generate", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID            string `json:"id"`
			Phase         string `json:"phase"`
			RunContext    *bool  `json:"runContext"`
			AllowCodeOnly bool   `json:"allowCodeOnly"`
			Force         bool   `json:"force"`
		}](args)
		if err != nil {
			return nil, err
		}
		if err := requireRequestID(in.ID); err != nil {
			return nil, err
		}
		// The context run is the point of the review, so it starts unless the client says it only wants the facts.
		runContext := in.RunContext == nil || *in.RunContext
		resp, err := c.req.GenerateOutcomeReview(ctx, &requestv1.GenerateOutcomeReviewRequest{
			RequestId: in.ID, Phase: in.Phase, RunContext: runContext, AllowCodeOnly: in.AllowCodeOnly, Force: in.Force,
		})
		if err != nil {
			return nil, err
		}
		return struct {
			Review          OutcomeReviewView     `json:"review"`
			Reused          bool                  `json:"reused"`
			Created         bool                  `json:"created"`
			Run             *OutcomeReviewRunView `json:"run,omitempty"`
			ContextCached   bool                  `json:"contextCached"`
			ContextSkipped  string                `json:"contextSkipped,omitempty"`
			DrawRun         *OutcomeReviewRunView `json:"drawRun,omitempty"`
			DrawingsCached  int32                 `json:"drawingsCached"`
			DrawingsPending int32                 `json:"drawingsPending"`
			DrawError       string                `json:"drawError,omitempty"`
		}{
			Review: outcomeReviewView(resp.GetReview()), Reused: resp.GetReused(), Created: resp.GetCreated(), Run: outcomeReviewRunView(resp.GetRun()),
			ContextCached: resp.GetContextCached(), ContextSkipped: resp.GetContextSkipped(), DrawRun: outcomeReviewRunView(resp.GetDrawRun()),
			DrawingsCached: resp.GetDrawingsCached(), DrawingsPending: resp.GetDrawingsPending(), DrawError: resp.GetDrawError(),
		}, nil
	})

	c.handle(r, "request.review.get", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID       string `json:"id"`
			ReviewID string `json:"reviewId"`
		}](args)
		if err != nil {
			return nil, err
		}
		if err := requireRequestID(in.ID); err != nil {
			return nil, err
		}
		resp, err := c.req.GetOutcomeReview(ctx, &requestv1.GetOutcomeReviewRequest{RequestId: in.ID, ReviewId: in.ReviewID})
		if err != nil {
			return nil, err
		}
		out := struct {
			Found  bool               `json:"found"`
			Review *OutcomeReviewView `json:"review,omitempty"`
		}{Found: resp.GetFound()}
		if resp.GetFound() {
			v := outcomeReviewView(resp.GetReview())
			out.Review = &v
		}
		return out, nil
	})

	c.handle(r, "request.review.list", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID        string `json:"id"`
			PageSize  int32  `json:"pageSize"`
			PageToken string `json:"pageToken"`
		}](args)
		if err != nil {
			return nil, err
		}
		if err := requireRequestID(in.ID); err != nil {
			return nil, err
		}
		resp, err := c.req.ListOutcomeReviews(ctx, &requestv1.ListOutcomeReviewsRequest{RequestId: in.ID, PageSize: in.PageSize, PageToken: in.PageToken})
		if err != nil {
			return nil, err
		}
		items := make([]OutcomeReviewView, 0, len(resp.GetReviews()))
		for _, rv := range resp.GetReviews() {
			items = append(items, outcomeReviewView(rv))
		}
		return struct {
			Reviews       []OutcomeReviewView `json:"reviews"`
			NextPageToken string              `json:"nextPageToken,omitempty"`
		}{Reviews: items, NextPageToken: resp.GetNextPageToken()}, nil
	})

	c.handle(r, "request.review.runs", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID       string `json:"id"`
			ReviewID string `json:"reviewId"`
		}](args)
		if err != nil {
			return nil, err
		}
		if err := requireRequestID(in.ID); err != nil {
			return nil, err
		}
		resp, err := c.req.ListOutcomeReviewRuns(ctx, &requestv1.ListOutcomeReviewRunsRequest{RequestId: in.ID, ReviewId: in.ReviewID})
		if err != nil {
			return nil, err
		}
		runs := make([]OutcomeReviewRunView, 0, len(resp.GetRuns()))
		for _, run := range resp.GetRuns() {
			runs = append(runs, *outcomeReviewRunView(run))
		}
		return struct {
			Runs []OutcomeReviewRunView `json:"runs"`
		}{Runs: runs}, nil
	})

	c.registerReviewArtifacts(r, opts)
	c.registerReviewDecisions(r, opts)
	c.registerReviewProfile(r, opts)
}

func (c requestChannels) registerReviewArtifacts(r *Registry, opts requestChannelOpts) {
	c.handle(r, "request.review.artifact", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID         string `json:"id"`
			ArtifactID string `json:"artifactId"`
			IfDigest   string `json:"ifDigest"`
		}](args)
		if err != nil {
			return nil, err
		}
		if err := requireRequestID(in.ID); err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.ArtifactID) == "" {
			return nil, invalidRequestArg("artifactId is required")
		}
		resp, err := c.req.GetReviewArtifact(ctx, &requestv1.GetReviewArtifactRequest{RequestId: in.ID, ArtifactId: in.ArtifactID, IfDigest: in.IfDigest})
		if err != nil {
			return nil, err
		}
		return struct {
			Artifact    *ReviewArtifactMetaView `json:"artifact"`
			NotModified bool                    `json:"notModified"`
			MediaType   string                  `json:"mediaType,omitempty"`
			Content     string                  `json:"content,omitempty"`
		}{Artifact: reviewArtifactMetaView(resp.GetArtifact()), NotModified: resp.GetNotModified(), MediaType: resp.GetMediaType(), Content: resp.GetContent()}, nil
	})

	c.handle(r, "request.review.revisions", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID      string `json:"id"`
			Section string `json:"section"`
			Variant string `json:"variant"`
			Name    string `json:"name"`
		}](args)
		if err != nil {
			return nil, err
		}
		if err := requireRequestID(in.ID); err != nil {
			return nil, err
		}
		resp, err := c.req.ListReviewArtifactRevisions(ctx, &requestv1.ListReviewArtifactRevisionsRequest{RequestId: in.ID, Section: in.Section, Variant: in.Variant, Name: in.Name})
		if err != nil {
			return nil, err
		}
		out := make([]ReviewArtifactMetaView, 0, len(resp.GetRevisions()))
		for _, a := range resp.GetRevisions() {
			out = append(out, *reviewArtifactMetaView(a))
		}
		return struct {
			Revisions []ReviewArtifactMetaView `json:"revisions"`
		}{Revisions: out}, nil
	})

	c.handle(r, "request.review.regenerate", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID              string   `json:"id"`
			Scope           string   `json:"scope"`
			Sections        []string `json:"sections"`
			Reason          string   `json:"reason"`
			Note            string   `json:"note"`
			ConfirmCost     bool     `json:"confirmCost"`
			ClientRequestID string   `json:"clientRequestId"`
			ExpectedVersion int64    `json:"expectedVersion"`
		}](args)
		if err != nil {
			return nil, err
		}
		if err := requireRequestID(in.ID); err != nil {
			return nil, err
		}
		resp, err := c.req.RegenerateReviewArtifacts(ctx, &requestv1.RegenerateReviewArtifactsRequest{
			RequestId: in.ID, Scope: in.Scope, Sections: in.Sections, Reason: in.Reason, Note: in.Note, ConfirmCost: in.ConfirmCost,
			ClientRequestId: in.ClientRequestID, ExpectedVersion: in.ExpectedVersion,
		})
		if err != nil {
			return nil, err
		}
		return struct {
			ContextRun *OutcomeReviewRunView     `json:"contextRun,omitempty"`
			DrawRun    *OutcomeReviewRunView     `json:"drawRun,omitempty"`
			Replayed   bool                      `json:"replayed"`
			Estimate   *RegenerationEstimateView `json:"estimate,omitempty"`
			Skipped    string                    `json:"skipped,omitempty"`
		}{ContextRun: outcomeReviewRunView(resp.GetContextRun()), DrawRun: outcomeReviewRunView(resp.GetDrawRun()), Replayed: resp.GetReplayed(),
			Estimate: regenerationEstimateView(resp.GetEstimate()), Skipped: resp.GetSkipped()}, nil
	})

	c.handle(r, "request.review.restore", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID         string `json:"id"`
			ArtifactID string `json:"artifactId"`
			Note       string `json:"note"`
		}](args)
		if err != nil {
			return nil, err
		}
		if err := requireRequestID(in.ID); err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.ArtifactID) == "" {
			return nil, invalidRequestArg("artifactId is required")
		}
		resp, err := c.req.RestoreReviewArtifactRevision(ctx, &requestv1.RestoreReviewArtifactRevisionRequest{RequestId: in.ID, ArtifactId: in.ArtifactID, Note: in.Note})
		if err != nil {
			return nil, err
		}
		out := struct {
			Artifact *ReviewArtifactMetaView `json:"artifact"`
			Diff     *ModelDiffView          `json:"diff,omitempty"`
		}{Artifact: reviewArtifactMetaView(resp.GetArtifact())}
		if resp.GetHasDiff() {
			out.Diff = modelDiffView(resp.GetDiff())
		}
		return out, nil
	})

	c.handle(r, "request.review.compare", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID   string `json:"id"`
			From string `json:"fromArtifactId"`
			To   string `json:"toArtifactId"`
		}](args)
		if err != nil {
			return nil, err
		}
		if err := requireRequestID(in.ID); err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.From) == "" || strings.TrimSpace(in.To) == "" {
			return nil, invalidRequestArg("fromArtifactId and toArtifactId are required")
		}
		resp, err := c.req.CompareReviewArtifactRevisions(ctx, &requestv1.CompareReviewArtifactRevisionsRequest{RequestId: in.ID, FromArtifactId: in.From, ToArtifactId: in.To})
		if err != nil {
			return nil, err
		}
		return struct {
			Diff *ModelDiffView `json:"diff"`
		}{Diff: modelDiffView(resp.GetDiff())}, nil
	})
}

func (c requestChannels) registerReviewDecisions(r *Registry, opts requestChannelOpts) {
	c.handle(r, "request.review.decide", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID              string `json:"id"`
			ReviewID        string `json:"reviewId"`
			ItemKey         string `json:"itemKey"`
			Decision        string `json:"decision"`
			Note            string `json:"note"`
			ExpectedVersion int64  `json:"expectedVersion"`
		}](args)
		if err != nil {
			return nil, err
		}
		if err := requireRequestID(in.ID); err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.ReviewID) == "" || strings.TrimSpace(in.ItemKey) == "" {
			return nil, invalidRequestArg("reviewId and itemKey are required")
		}
		resp, err := c.req.DecideOutcomeReviewItem(ctx, &requestv1.DecideOutcomeReviewItemRequest{
			RequestId: in.ID, ReviewId: in.ReviewID, ItemKey: in.ItemKey, Decision: in.Decision, Note: in.Note, ExpectedVersion: in.ExpectedVersion,
		})
		if err != nil {
			return nil, err
		}
		return struct {
			Decision      *ReviewDecisionView `json:"decision"`
			ReviewVersion int64               `json:"reviewVersion"`
			ReviewState   string              `json:"reviewState"`
			OpenDecisions int32               `json:"openDecisions"`
		}{Decision: reviewDecisionView(resp.GetDecision()), ReviewVersion: resp.GetReviewVersion(), ReviewState: resp.GetReviewState(), OpenDecisions: resp.GetOpenDecisions()}, nil
	})

	c.handle(r, "request.review.approve", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID              string `json:"id"`
			ReviewID        string `json:"reviewId"`
			Note            string `json:"note"`
			ExpectedVersion int64  `json:"expectedVersion"`
		}](args)
		if err != nil {
			return nil, err
		}
		if err := requireRequestID(in.ID); err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.ReviewID) == "" {
			return nil, invalidRequestArg("reviewId is required")
		}
		resp, err := c.req.ApproveOutcomeReview(ctx, &requestv1.ApproveOutcomeReviewRequest{RequestId: in.ID, ReviewId: in.ReviewID, Note: in.Note, ExpectedVersion: in.ExpectedVersion})
		if err != nil {
			return nil, err
		}
		return struct {
			Review OutcomeReviewView `json:"review"`
		}{Review: outcomeReviewView(resp.GetReview())}, nil
	})

	c.handle(r, "request.review.decisions", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID       string `json:"id"`
			ReviewID string `json:"reviewId"`
			ItemKey  string `json:"itemKey"`
		}](args)
		if err != nil {
			return nil, err
		}
		if err := requireRequestID(in.ID); err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.ReviewID) == "" {
			return nil, invalidRequestArg("reviewId is required")
		}
		resp, err := c.req.ListOutcomeReviewDecisions(ctx, &requestv1.ListOutcomeReviewDecisionsRequest{RequestId: in.ID, ReviewId: in.ReviewID, ItemKey: in.ItemKey})
		if err != nil {
			return nil, err
		}
		list := make([]ReviewDecisionView, 0, len(resp.GetDecisions()))
		for _, d := range resp.GetDecisions() {
			list = append(list, *reviewDecisionView(d))
		}
		return struct {
			Decisions     []ReviewDecisionView `json:"decisions"`
			OpenDecisions int32                `json:"openDecisions"`
		}{Decisions: list, OpenDecisions: resp.GetOpenDecisions()}, nil
	})

	c.handle(r, "request.review.linkChild", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID             string   `json:"id"`
			ReviewID       string   `json:"reviewId"`
			ItemKeys       []string `json:"itemKeys"`
			ChildRequestID string   `json:"childRequestId"`
		}](args)
		if err != nil {
			return nil, err
		}
		if err := requireRequestID(in.ID); err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.ReviewID) == "" || len(in.ItemKeys) == 0 || strings.TrimSpace(in.ChildRequestID) == "" {
			return nil, invalidRequestArg("reviewId, itemKeys and childRequestId are required")
		}
		resp, err := c.req.LinkOutcomeReviewChild(ctx, &requestv1.LinkOutcomeReviewChildRequest{RequestId: in.ID, ReviewId: in.ReviewID, ItemKeys: in.ItemKeys, ChildRequestId: in.ChildRequestID})
		if err != nil {
			return nil, err
		}
		return struct {
			Linked int32 `json:"linked"`
		}{Linked: resp.GetLinked()}, nil
	})
}

func (c requestChannels) registerReviewProfile(r *Registry, opts requestChannelOpts) {
	c.handle(r, "request.review.profile.get", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ProjectID string `json:"projectId"`
		}](args)
		if err != nil {
			return nil, err
		}
		resp, err := c.req.GetReviewDrawingProfile(ctx, &requestv1.GetReviewDrawingProfileRequest{ProjectId: in.ProjectID})
		if err != nil {
			return nil, err
		}
		effective := json.RawMessage(resp.GetEffectiveJson())
		if !json.Valid(effective) {
			effective = json.RawMessage("{}")
		}
		sources := resp.GetSources()
		if sources == nil {
			sources = map[string]string{}
		}
		out := struct {
			TenantLayer  *ReviewProfileLayerView `json:"tenantLayer,omitempty"`
			ProjectLayer *ReviewProfileLayerView `json:"projectLayer,omitempty"`
			Effective    json.RawMessage         `json:"effective"`
			Sources      map[string]string       `json:"sources"`
		}{Effective: effective, Sources: sources}
		if resp.GetHasTenantLayer() {
			out.TenantLayer = reviewProfileLayerView(resp.GetTenantLayer())
		}
		if resp.GetHasProjectLayer() {
			out.ProjectLayer = reviewProfileLayerView(resp.GetProjectLayer())
		}
		return out, nil
	})

	c.handle(r, "request.review.profile.save", opts, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		// Fail early with a clear code; request-service checks the role again.
		if id.Role != roleAdmin {
			return nil, requestInputError{msg: "REQUEST_FORBIDDEN: admin role required"}
		}
		in, err := decodeRequestArgs[struct {
			ProjectID       string          `json:"projectId"`
			Spec            json.RawMessage `json:"spec"`
			ExpectedVersion int64           `json:"expectedVersion"`
		}](args)
		if err != nil {
			return nil, err
		}
		if len(strings.TrimSpace(string(in.Spec))) == 0 || string(in.Spec) == "null" {
			return nil, invalidRequestArg("spec is required")
		}
		resp, err := c.req.SaveReviewDrawingProfile(ctx, &requestv1.SaveReviewDrawingProfileRequest{ProjectId: in.ProjectID, SpecJson: string(in.Spec), ExpectedVersion: in.ExpectedVersion})
		if err != nil {
			return nil, err
		}
		return struct {
			Layer *ReviewProfileLayerView `json:"layer"`
		}{Layer: reviewProfileLayerView(resp.GetLayer())}, nil
	})

	c.handle(r, "request.review.profile.reset", opts, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		if id.Role != roleAdmin {
			return nil, requestInputError{msg: "REQUEST_FORBIDDEN: admin role required"}
		}
		in, err := decodeRequestArgs[struct {
			ProjectID       string `json:"projectId"`
			ExpectedVersion int64  `json:"expectedVersion"`
		}](args)
		if err != nil {
			return nil, err
		}
		if _, err := c.req.ResetReviewDrawingProfile(ctx, &requestv1.ResetReviewDrawingProfileRequest{ProjectId: in.ProjectID, ExpectedVersion: in.ExpectedVersion}); err != nil {
			return nil, err
		}
		return struct {
			Reset bool `json:"reset"`
		}{Reset: true}, nil
	})
}
