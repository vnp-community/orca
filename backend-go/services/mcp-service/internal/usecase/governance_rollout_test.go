package usecase_test

import (
	"testing"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
	tt "github.com/stablyai/orca-go/services/mcp-service/internal/usecase/usecasetest"
)

// Rollback drill (BE-MCP-SOL-015 section E): an administrator's kill switch
// denies every call of the tenant at once, and clearing it restores exactly the
// same decisions - nothing else about the tenant changed, so rollback needs no
// data repair. Runs with the D6 default-on tenant setting.
func TestKillSwitchRollbackRestoresDecisions(t *testing.T) {
	h := tt.NewHarness(true)
	adm := tt.Ctx(tenantA, userAdm, "admin")
	decisions := func() []string {
		var out []string
		for _, tool := range []domain.ToolRef{tt.ToolRead, tt.ToolWrite, tt.ToolExec, tt.ToolDestructive, tt.ToolAdmin, tt.ToolHardDenied} {
			d, err := h.Evaluate.Execute(adm, tool, tt.CC())
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, d.Decision)
		}
		return out
	}
	before := decisions()
	if err := h.KillAdmin.Set(adm, usecase.SetKillSwitchInput{Scope: "tenant", Reason: "incident drill", Active: true}); err != nil {
		t.Fatal(err)
	}
	for i, d := range decisions() {
		if d != "deny" {
			t.Errorf("kill switch active: tool %d decided %q, want deny", i, d)
		}
	}
	if err := h.KillAdmin.Set(adm, usecase.SetKillSwitchInput{Scope: "tenant", Reason: "drill over", Active: false}); err != nil {
		t.Fatal(err)
	}
	after := decisions()
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("tool %d: %q before the drill, %q after rollback", i, before[i], after[i])
		}
	}
}
