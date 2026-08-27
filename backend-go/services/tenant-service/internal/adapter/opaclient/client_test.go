package opaclient_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/policy"
	"github.com/stablyai/orca-go/services/tenant-service/internal/adapter/opaclient"
)

// bundlePath points at the real orca-authz bundle relative to this package
// — exercises the actual tenant.rego policy, not a synthetic inline bundle.
const bundlePath = "../../../../../policy/orca-authz"

func TestClient_Decision_InputShape(t *testing.T) {
	c := opaclient.New(policy.NewEvaluator(bundlePath))
	ctx := context.Background()

	allowed, err := c.Decision(ctx, "admin", "company_edit", false)
	if err != nil {
		t.Fatalf("Decision: %v", err)
	}
	if !allowed {
		t.Fatal("expected admin to be allowed for company_edit")
	}

	allowed, err = c.Decision(ctx, "lead", "company_edit", false)
	if err != nil {
		t.Fatalf("Decision: %v", err)
	}
	if allowed {
		t.Fatal("expected lead to be denied for company_edit")
	}

	allowed, err = c.Decision(ctx, "lead", "department_edit", true)
	if err != nil {
		t.Fatalf("Decision: %v", err)
	}
	if !allowed {
		t.Fatal("expected lead of the same department to be allowed for department_edit")
	}

	allowed, err = c.Decision(ctx, "lead", "department_edit", false)
	if err != nil {
		t.Fatalf("Decision: %v", err)
	}
	if allowed {
		t.Fatal("expected lead of a different department to be denied for department_edit")
	}

	allowed, err = c.Decision(ctx, "", "department_edit", true)
	if err != nil {
		t.Fatalf("Decision: %v", err)
	}
	if allowed {
		t.Fatal("expected empty role to be denied")
	}
}
