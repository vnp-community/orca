package wscompat

import (
	"context"
	"encoding/json"

	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
)

type statusArgs struct {
	codeIntelSelector
	Refresh bool `json:"refresh,omitempty"`
}

func (a statusArgs) validate() error {
	return a.codeIntelSelector.validate()
}

// registerCodeIntelStatus wires codeIntel.status returning flat IndexStatus without data/meta envelope (UI-API 3.1, 4.1).
func registerCodeIntelStatus(r *Registry, d codeIntelDeps) {
	spec := mustCatalogSpec("codeIntel.status")
	registerCodeIntelUnary(r, d, spec.Name, func(ctx context.Context, c codeIntelCaller, _ Identity, in statusArgs) (any, error) {
		resp, err := c.Core.GetIndexStatus(ctx, &codeintelv1.GetIndexStatusRequest{
			Selector: toProtoSelector(in.codeIntelSelector),
			Refresh:  in.Refresh,
		})
		if err != nil {
			return nil, err
		}
		// Flat IndexStatus response without result envelope per UI-API contract §4.1.
		return finishCodeIntelResponse(spec, d.limits, resp, func() (json.RawMessage, error) {
			if resp.GetStatus() == nil {
				return json.RawMessage("null"), nil
			}
			return encodeCodeIntelWire(resp.GetStatus())
		})
	})
}
