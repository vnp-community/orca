package wscompat

import (
	"context"
	"encoding/json"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// roleAdmin is the global admin role Identity.Role carries (common/tenant.Role).
const roleAdmin = "admin"

func (c requestChannels) registerFlow(r *Registry) {
	opts := requestChannelOpts{timeout: requestRPCTimeout}
	c.handle(r, "request.flowStatus", opts, func(ctx context.Context, _ Identity, _ []json.RawMessage) (any, error) {
		resp, err := c.req.GetRequestFlowSettings(ctx, &requestv1.GetRequestFlowSettingsRequest{})
		if err != nil {
			return nil, err
		}
		return struct {
			Enabled bool `json:"enabled"`
		}{resp.GetEnabled()}, nil
	})
	c.handle(r, "request.flowSet", opts, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		// Fail early with a clear code; request-service checks the role again.
		if id.Role != roleAdmin {
			return nil, requestInputError{msg: "REQUEST_FORBIDDEN: admin role required"}
		}
		in, err := decodeRequestArgs[struct {
			Enabled *bool `json:"enabled"`
		}](args)
		if err != nil {
			return nil, err
		}
		if in.Enabled == nil {
			return nil, invalidRequestArg("enabled is required")
		}
		resp, err := c.req.SetRequestFlowSettings(ctx, &requestv1.SetRequestFlowSettingsRequest{Enabled: *in.Enabled})
		if err != nil {
			return nil, err
		}
		return struct {
			Enabled bool `json:"enabled"`
		}{resp.GetEnabled()}, nil
	})
}

// registerExtras adds the three read channels of CONTRACT 2.5 (task 016-08).
func (c requestChannels) registerExtras(r *Registry) {
	opts := requestChannelOpts{timeout: requestRPCTimeout}
	c.handle(r, "request.links", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[requestIDArgs](args)
		if err != nil {
			return nil, err
		}
		resp, err := c.req.ListRequestLinks(ctx, &requestv1.ListRequestLinksRequest{RequestId: in.ID})
		if err != nil {
			return nil, err
		}
		return struct {
			Parents  []RequestLinkView `json:"parents"`
			Children []RequestLinkView `json:"children"`
		}{requestLinkViews(resp.GetParents()), requestLinkViews(resp.GetChildren())}, nil
	})
	c.handle(r, "request.flow", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			Type string `json:"type"`
			Size string `json:"size"`
		}](args)
		if err != nil {
			return nil, err
		}
		resp, err := c.req.GetRequestFlow(ctx, &requestv1.GetRequestFlowRequest{Type: in.Type, Size: in.Size})
		if err != nil {
			return nil, err
		}
		return requestFlowViewOf(resp), nil
	})
	c.handle(r, "request.checks", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		}](args)
		if err != nil {
			return nil, err
		}
		resp, err := c.req.ListRequestChecks(ctx, &requestv1.ListRequestChecksRequest{RequestId: in.ID, Kind: in.Kind})
		if err != nil {
			return nil, err
		}
		return struct {
			Checks []RequestCheckView `json:"checks"`
		}{requestCheckViews(resp.GetChecks())}, nil
	})
}
