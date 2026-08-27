package policy_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/policy"
)

// bundlePath points at the real orca-authz bundle (../../policy/orca-authz
// relative to this package) — this test exercises the actual policies
// every consuming service loads, not a synthetic inline bundle, so a
// policy-file typo or Rego syntax error fails here rather than only inside
// `opa test` (which this Go test suite doesn't otherwise run).
const bundlePath = "../../policy/orca-authz"

func TestEvaluator_AdminDecision(t *testing.T) {
	e := policy.NewEvaluator(bundlePath)
	ctx := context.Background()

	allowed, err := e.Decision(ctx, "data.orca.authz.admin.allow", map[string]any{
		"actor": map[string]any{"role": "admin", "id": "u1"},
	})
	if err != nil {
		t.Fatalf("Decision: %v", err)
	}
	if !allowed {
		t.Fatal("expected admin actor to be allowed")
	}

	allowed, err = e.Decision(ctx, "data.orca.authz.admin.allow", map[string]any{
		"actor": map[string]any{"role": "user", "id": "u1"},
	})
	if err != nil {
		t.Fatalf("Decision: %v", err)
	}
	if allowed {
		t.Fatal("expected non-admin actor to be denied")
	}
}

func TestEvaluator_TaskGrantDecision(t *testing.T) {
	e := policy.NewEvaluator(bundlePath)
	ctx := context.Background()

	allowed, err := e.Decision(ctx, "data.orca.authz.task.allow", map[string]any{
		"level": "company", "action": "write",
	})
	if err != nil {
		t.Fatalf("Decision: %v", err)
	}
	if allowed {
		t.Fatal("expected company-level grant to be denied for write")
	}

	allowed, err = e.Decision(ctx, "data.orca.authz.task.allow", map[string]any{
		"level": "owner", "action": "write",
	})
	if err != nil {
		t.Fatalf("Decision: %v", err)
	}
	if !allowed {
		t.Fatal("expected owner-level grant to be allowed for write")
	}
}

func TestEvaluator_AnnotationDecision(t *testing.T) {
	e := policy.NewEvaluator(bundlePath)
	ctx := context.Background()

	allowed, err := e.Decision(ctx, "data.orca.authz.annotation.allow", map[string]any{
		"actor_id": "u1", "author_id": "u1", "actor_role": "user",
	})
	if err != nil {
		t.Fatalf("Decision: %v", err)
	}
	if !allowed {
		t.Fatal("expected author to be allowed")
	}

	allowed, err = e.Decision(ctx, "data.orca.authz.annotation.allow", map[string]any{
		"actor_id": "u2", "author_id": "u1", "actor_role": "user",
	})
	if err != nil {
		t.Fatalf("Decision: %v", err)
	}
	if allowed {
		t.Fatal("expected non-author non-admin to be denied")
	}
}

func TestEvaluator_UndefinedQueryIsDeny(t *testing.T) {
	e := policy.NewEvaluator(bundlePath)
	allowed, err := e.Decision(context.Background(), "data.orca.authz.nonexistent.allow", map[string]any{})
	if err != nil {
		t.Fatalf("Decision: %v", err)
	}
	if allowed {
		t.Fatal("expected an undefined rule path to default-deny, not error-allow")
	}
}
