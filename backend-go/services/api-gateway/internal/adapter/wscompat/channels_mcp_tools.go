package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
)

// McpToolAnnotations / McpToolView mirror CONTRACT section 1.
type McpToolAnnotations struct {
	ReadOnly    bool `json:"readOnly"`
	Destructive bool `json:"destructive"`
	Idempotent  bool `json:"idempotent"`
	OpenWorld   bool `json:"openWorld"`
}

type McpToolView struct {
	Name            string             `json:"name"`
	Channel         string             `json:"channel"`
	Title           string             `json:"title"`
	Description     string             `json:"description"`
	Namespace       string             `json:"namespace"`
	Risk            string             `json:"risk"`
	RequiredScope   string             `json:"requiredScope"`
	Pack            int                `json:"pack"`
	HardDenied      bool               `json:"hardDenied"`
	Effective       string             `json:"effective"`
	EffectiveSource string             `json:"effectiveSource"`
	Annotations     McpToolAnnotations `json:"annotations"`
}

// McpToolLister is implemented by mcpserver/tools.Catalog; it lives here
// because that package imports wscompat, not the other way round.
type McpToolLister interface {
	AdminToolViews(ctx context.Context, tenantID string) ([]McpToolView, error)
}

// registerMcpToolCatalogChannel registers mcp.admin.tool.list. The effective
// decision is the tenant default, not per user.
func registerMcpToolCatalogChannel(r *Registry, d McpChannelDeps) {
	r.Register("mcp.admin.tool.list", mcpHandler(d, true, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		if d.ToolCatalog == nil {
			return nil, errors.New("MCP_UNAVAILABLE: tool catalog is not configured")
		}
		var in struct {
			Namespace string `json:"namespace"`
			Risk      string `json:"risk"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args[0], &in); err != nil {
				return nil, errors.New("MCP_INVALID_ARGUMENT: expected {namespace?, risk?}")
			}
		}
		views, err := d.ToolCatalog.AdminToolViews(ctx, id.TenantID)
		if err != nil {
			return nil, err
		}
		out := make([]McpToolView, 0, len(views))
		for _, v := range views {
			if (in.Namespace == "" || v.Namespace == in.Namespace) && (in.Risk == "" || v.Risk == in.Risk) {
				out = append(out, v)
			}
		}
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Namespace != out[j].Namespace {
				return out[i].Namespace < out[j].Namespace
			}
			return out[i].Name < out[j].Name
		})
		return out, nil
	}))
}
