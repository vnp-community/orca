package wscompat

import (
	"context"
	"encoding/json"
	"math"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// requestFeaturePlanRevisions is published by request.flowStatus so a screen offers the version switcher and the request-
// for-changes box only when this gateway serves request.planRevisions and the `revision` argument (CR-REQ-045). An older
// gateway ignores `revision` and would show the current version as if it were the one asked for.
const requestFeaturePlanRevisions = "request.planRevisions"

// registerPlanRevisions adds request.planRevisions. It has no MCP tool on purpose: the documents it points at are written
// by an agent from Request content (see excluded_channels.yaml).
func (c requestChannels) registerPlanRevisions(r *Registry) {
	c.handle(r, "request.planRevisions", requestChannelOpts{timeout: requestRPCTimeout}, c.planRevisions)
}

// planRevisionArg checks the `revision` argument; 0 means "the current version".
func planRevisionArg(n int64) (int32, error) {
	if n < 0 || n > math.MaxInt32 {
		return 0, invalidRequestArg("revision must be 0 (current) or a version number")
	}
	return int32(n), nil
}

func (c requestChannels) planRevisions(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
	in, err := decodeRequestArgs[struct {
		ID        string `json:"id"`
		PageSize  int32  `json:"pageSize"`
		PageToken string `json:"pageToken"`
	}](args)
	if err != nil {
		return nil, err
	}
	resp, err := c.req.ListPlanRevisions(ctx, &requestv1.ListPlanRevisionsRequest{RequestId: in.ID, PageSize: clampRequestPageSize(in.PageSize), PageToken: in.PageToken})
	if err != nil {
		return nil, err
	}
	revisions := make([]json.RawMessage, 0, len(resp.GetRevisions()))
	for _, rev := range resp.GetRevisions() {
		revisions = append(revisions, protoJSON(rev))
	}
	return struct {
		Revisions     []json.RawMessage `json:"revisions"`
		NextPageToken string            `json:"nextPageToken"`
	}{revisions, resp.GetNextPageToken()}, nil
}
