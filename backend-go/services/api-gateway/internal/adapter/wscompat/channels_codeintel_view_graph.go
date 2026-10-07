package wscompat

import (
	"context"
	"encoding/json"

	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
)

type structureArgs struct {
	codeIntelSelector
	Path        string `json:"path,omitempty"`
	Depth       *int   `json:"depth,omitempty"`
	Limit       *int   `json:"limit,omitempty"`
	PageToken   string `json:"pageToken,omitempty"`
	IfNoneMatch string `json:"ifNoneMatch,omitempty"`
}

func (a structureArgs) validate() error {
	if err := a.codeIntelSelector.validate(); err != nil {
		return err
	}
	if err := checkRelPath("path", a.Path); err != nil {
		return err
	}
	if err := checkBoundedInt("depth", a.Depth, 1, 3); err != nil {
		return err
	}
	if a.Limit != nil && *a.Limit < 1 {
		return invalidParam("limit", "out_of_range")
	}
	if err := checkPageToken("pageToken", a.PageToken); err != nil {
		return err
	}
	if err := checkStringMax("ifNoneMatch", a.IfNoneMatch, 80); err != nil {
		return err
	}
	return nil
}

type routesArgs struct {
	codeIntelSelector
	Limit       *int   `json:"limit,omitempty"`
	PageToken   string `json:"pageToken,omitempty"`
	IfNoneMatch string `json:"ifNoneMatch,omitempty"`
}

func (a routesArgs) validate() error {
	if err := a.codeIntelSelector.validate(); err != nil {
		return err
	}
	if err := checkBoundedInt("limit", a.Limit, 1, 500); err != nil {
		return err
	}
	if err := checkPageToken("pageToken", a.PageToken); err != nil {
		return err
	}
	if err := checkStringMax("ifNoneMatch", a.IfNoneMatch, 80); err != nil {
		return err
	}
	return nil
}

func int32OrZero(p *int) int32 {
	if p == nil {
		return 0
	}
	return int32(*p)
}

// registerCodeIntelStructure wires codeIntel.structure returning Env<ModuleGraph>.
func registerCodeIntelStructure(r *Registry, d codeIntelDeps) {
	spec := mustCatalogSpec("codeIntel.structure")
	registerCodeIntelUnary(r, d, spec.Name, func(ctx context.Context, c codeIntelCaller, _ Identity, in structureArgs) (any, error) {
		resp, err := c.Core.GetStructure(ctx, &codeintelv1.GetStructureRequest{
			Selector:    toProtoSelector(in.codeIntelSelector),
			Path:        in.Path,
			Depth:       int32OrZero(in.Depth),
			Limit:       int32OrZero(in.Limit),
			PageToken:   in.PageToken,
			IfNoneMatch: in.IfNoneMatch,
		})
		if err != nil {
			return nil, err
		}
		return finishCodeIntelResponse(spec, d.limits, resp, func() (json.RawMessage, error) {
			return encodeEnvelope(resp.GetMeta(), resp.GetData(), resp.GetNextPageToken())
		})
	})
}

// registerCodeIntelRoutes wires codeIntel.routes returning Env<RouteMap>.
func registerCodeIntelRoutes(r *Registry, d codeIntelDeps) {
	spec := mustCatalogSpec("codeIntel.routes")
	registerCodeIntelUnary(r, d, spec.Name, func(ctx context.Context, c codeIntelCaller, _ Identity, in routesArgs) (any, error) {
		resp, err := c.Core.GetRouteMap(ctx, &codeintelv1.GetRouteMapRequest{
			Selector:    toProtoSelector(in.codeIntelSelector),
			Limit:       int32OrZero(in.Limit),
			PageToken:   in.PageToken,
			IfNoneMatch: in.IfNoneMatch,
		})
		if err != nil {
			return nil, err
		}
		return finishCodeIntelResponse(spec, d.limits, resp, func() (json.RawMessage, error) {
			return encodeEnvelope(resp.GetMeta(), resp.GetData(), resp.GetNextPageToken())
		})
	})
}
