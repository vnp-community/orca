package policyengine_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stablyai/orca-go/services/mcp-service/internal/adapter/policyengine"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	tt "github.com/stablyai/orca-go/services/mcp-service/internal/usecase/usecasetest"
)

type stubValuer struct {
	v   any
	err error
}

func (s stubValuer) Value(context.Context, string, any) (any, error) { return s.v, s.err }
func (s stubValuer) Warm(context.Context, ...string) error           { return s.err }

func input() domain.PolicyInput {
	return domain.PolicyInput{UserID: "u", UserRole: "user", ClientID: "c", ClientStatus: "allowed", Scopes: []string{"orca:read"}, Enabled: true,
		MaxDepth: 1, Tool: domain.ToolRef{Name: "task_list", Channel: "task.list", Namespace: "task", Risk: "read", RequiredScope: "orca:read"}}
}

func TestEvaluateFailsClosed(t *testing.T) {
	cases := map[string]stubValuer{
		"evaluator error":   {err: errors.New("boom")},
		"undefined result":  {v: nil},
		"non-object result": {v: "allow"},
		"unknown decision":  {v: map[string]any{"decision": "yolo"}},
		"missing decision":  {v: map[string]any{"source": "x"}},
	}
	for name, v := range cases {
		t.Run(name, func(t *testing.T) {
			d := policyengine.NewWithEvaluator(v, nil).Evaluate(context.Background(), input())
			if d.Decision != domain.DecisionDeny {
				t.Fatalf("got %+v", d)
			}
		})
	}
}

func TestBrokenBundleDeniesAndWarmFails(t *testing.T) {
	e := policyengine.New(t.TempDir()+"/does-not-exist", nil)
	if err := e.Warm(context.Background()); err == nil {
		t.Fatal("warm must fail for a missing bundle so the service refuses to start")
	}
	if d := e.Evaluate(context.Background(), input()); d.Decision != domain.DecisionDeny || d.Reasons[0] != "policy_unavailable" {
		t.Fatalf("got %+v", d)
	}
}

func TestRealBundleHardDenySets(t *testing.T) {
	e := policyengine.New(tt.BundlePath(), nil)
	if err := e.Warm(context.Background()); err != nil {
		t.Fatal(err)
	}
	ch, pf, err := e.HardDenySets(context.Background())
	if err != nil || !slices.Contains(ch, "team.create") || !slices.Contains(pf, "mcp.") || !slices.Contains(pf, "credentials.") {
		t.Fatalf("%v %v %v", ch, pf, err)
	}
	for channel, want := range map[string]bool{"mcp.approval.decide": true, "credentials.get": true, "devServer.agentTokens.revoke": true, "task.create": false, "task.mcp.note": false} {
		got, err := e.IsHardDenied(context.Background(), channel)
		if err != nil || got != want {
			t.Errorf("%s: got %v err %v", channel, got, err)
		}
	}
}

// Sensitive channels that must never become reachable by an agent.
func TestGoldenSensitiveChannelsAreHardDenied(t *testing.T) {
	e := policyengine.New(tt.BundlePath(), nil)
	for _, ch := range []string{"mcp.token.create", "mcp.admin.policy.upsert", "auth.login", "credentials.set", "admin.createUser", "admin.updateUserRole",
		"admin.forceRevokeSession", "admin.createPolicy", "aiProvider.writeCredential", "devServer.agentTokens.create", "team.addMember",
		"profile.updateUser", "github.startAuthLogin", "gitlab.startCliAuthLogin"} {
		if ok, err := e.IsHardDenied(context.Background(), ch); err != nil || !ok {
			t.Errorf("%s must be hard-denied (err=%v)", ch, err)
		}
	}
}

func BenchmarkEvaluateToolCall(b *testing.B) {
	e := policyengine.New(tt.BundlePath(), nil)
	if err := e.Warm(context.Background()); err != nil {
		b.Fatal(err)
	}
	in := input()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if e.Evaluate(context.Background(), in).Decision != domain.DecisionAllow {
			b.Fatal("unexpected decision")
		}
	}
}
