package tools

import (
	"context"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

// AdminToolViews implements wscompat.McpToolLister: every catalogued spec of
// an enabled pack (also ones the tenant default denies) plus hard-denied
// channels. A gate without ToolPolicyView reports "allow/default" — the
// call-time Decide stays authoritative.
func (c *Catalog) AdminToolViews(ctx context.Context, tenantID string) ([]wscompat.McpToolView, error) {
	out := make([]wscompat.McpToolView, 0, len(c.ordered)+len(c.hardDeny))
	for _, s := range c.ordered {
		eff := mcpserver.EffectiveDecision{Decision: mcpserver.OutcomeAllow, Source: "default"}
		if d, ok := c.effective(ctx, tenantID, s); ok {
			eff = d
		}
		out = append(out, wscompat.McpToolView{
			Name: s.Name, Channel: s.Channel, Title: s.Title, Description: s.Description, Namespace: s.Namespace,
			Risk: s.Risk, RequiredScope: s.RequiredScope(), Pack: s.Pack,
			Effective: eff.Decision, EffectiveSource: eff.Source,
			Annotations: wscompat.McpToolAnnotations{ReadOnly: s.Annotations.ReadOnly, Destructive: s.Annotations.Destructive,
				Idempotent: s.Annotations.Idempotent, OpenWorld: s.Annotations.OpenWorld},
		})
	}
	for _, h := range c.hardDeny {
		out = append(out, wscompat.McpToolView{
			Name: h.Name, Channel: h.Channel, Title: titleOf(h.Channel), Description: h.Reason, Namespace: h.Namespace,
			Risk: mcpserver.RiskAdmin, RequiredScope: RiskToScope(mcpserver.RiskAdmin), Pack: 4, HardDenied: true,
			Effective: mcpserver.OutcomeDeny, EffectiveSource: "hard_deny",
		})
	}
	return out, nil
}
