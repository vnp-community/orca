package mcpserver_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
)

func TestFailClosedGateAllowsOnlyRead(t *testing.T) {
	var g mcpserver.PolicyGate = mcpserver.FailClosedGate{}
	for risk, want := range map[string]string{
		mcpserver.RiskRead: "allow", mcpserver.RiskWriteReversible: "deny", mcpserver.RiskExec: "deny",
		mcpserver.RiskDestructive: "deny", mcpserver.RiskAdmin: "deny", "": "deny", "bogus": "deny",
	} {
		d, err := g.Decide(context.Background(), mcpserver.Principal{}, mcpserver.ToolMeta{Risk: risk}, nil)
		if err != nil || d.Outcome != want {
			t.Errorf("risk %q: %+v %v, want %s", risk, d, err, want)
		}
		ev, _ := g.(mcpserver.ToolPolicyView).EffectiveDecision(context.Background(), "t", mcpserver.ToolMeta{Risk: risk})
		if ev.Decision != want {
			t.Errorf("view risk %q: %+v", risk, ev)
		}
	}
	if ok, _ := g.AwaitApproval(context.Background(), mcpserver.Principal{}, "x"); ok {
		t.Error("approvals must never resolve without a gate")
	}
}
