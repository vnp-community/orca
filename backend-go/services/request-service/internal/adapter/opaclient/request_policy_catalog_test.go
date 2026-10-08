package opaclient

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type accessMatrix struct {
	Roles         []string            `json:"roles"`
	PersonAllowed map[string][]string `json:"person_allowed"`
}

func loadMatrix(t *testing.T) accessMatrix {
	t.Helper()
	b, err := os.ReadFile("testdata/access_matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	var m accessMatrix
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func roleInput(role string) (global, project string, reporter bool) {
	switch role {
	case "admin":
		return "admin", "", false
	case "owner", "member":
		return "", role, false
	case "reporter":
		return "", "", true
	}
	return "", "", false
}

// Every RPC of the catalog, for every caller role, matches the shared matrix through the real Rego bundle.
func TestCatalogPersonMatrixAgainstRego(t *testing.T) {
	p := setupPolicy(t)
	m := loadMatrix(t)
	for method, e := range domain.Catalog {
		if e.Group == domain.GroupInternal {
			continue // never evaluated: internalcaller.Guard owns it.
		}
		allowed := map[string]bool{}
		for _, r := range m.PersonAllowed[string(e.Group)] {
			allowed[r] = true
		}
		for _, role := range m.Roles {
			g, pr, rep := roleInput(role)
			got, err := p.Allow(context.Background(), domain.AccessInput{Action: e.Group, RPC: domain.RPCName(method), CallerGlobalRole: g, CallerProjectRole: pr, IsReporter: rep, ActorType: "user"})
			if err != nil {
				t.Fatal(err)
			}
			if got != allowed[role] {
				t.Errorf("%s as %s: allow=%v want %v", method, role, got, allowed[role])
			}
		}
	}
}

// AgentAllowed in the catalog and agent_rpcs in the Rego must be the same set, even for an admin behind the agent.
func TestCatalogAgentFlagMatchesRego(t *testing.T) {
	p := setupPolicy(t)
	for method, e := range domain.Catalog {
		if e.Group == domain.GroupInternal {
			continue
		}
		for _, role := range []string{"admin", "owner"} {
			g, pr, rep := roleInput(role)
			got, err := p.Allow(context.Background(), domain.AccessInput{Action: e.Group, RPC: domain.RPCName(method), CallerGlobalRole: g, CallerProjectRole: pr, IsReporter: rep, ActorType: "agent"})
			if err != nil {
				t.Fatal(err)
			}
			// owner has no role for the admin group; only AgentAllowed RPCs can ever pass for an agent.
			if got && !e.AgentAllowed {
				t.Errorf("%s: agent (%s behind) allowed by Rego but catalog says no", method, role)
			}
			if !got && e.AgentAllowed && role == "admin" {
				t.Errorf("%s: catalog allows agents but Rego refuses an admin-backed agent", method)
			}
		}
	}
}

func TestRequestPolicy_AllowFailsClosedOnMissingBundle(t *testing.T) {
	p := NewRequestPolicy("./nonexistent")
	ok, err := p.Allow(context.Background(), domain.AccessInput{Action: domain.GroupRead, ActorType: "user", CallerGlobalRole: "admin"})
	if err == nil || ok {
		t.Fatalf("missing bundle must deny with an error, got ok=%v err=%v", ok, err)
	}
}

func TestRequestPolicy_AgentNeverReachesForbiddenRPCs(t *testing.T) {
	p := setupPolicy(t)
	for _, c := range []struct {
		group domain.Group
		rpc   string
	}{
		{domain.GroupDecide, "Approve"}, {domain.GroupExecute, "StartPhase"}, {domain.GroupAdmin, "SetRequestFlowSettings"},
		{domain.GroupPlan, "GeneratePlan"}, {domain.GroupLifecycle, "CancelRequest"},
	} {
		ok, err := p.Allow(context.Background(), domain.AccessInput{Action: c.group, RPC: c.rpc, CallerGlobalRole: "admin", CallerProjectRole: "owner", ActorType: "agent"})
		if err != nil || ok {
			t.Errorf("agent %s: ok=%v err=%v", c.rpc, ok, err)
		}
	}
}
