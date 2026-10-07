package opaclient

import (
	"context"
	"testing"
)

func setupPolicy(t *testing.T) *RequestPolicy {
	// The path from internal/adapter/opaclient to policy/orca-authz is:
	// internal/adapter/opaclient -> ../../../../../../policy/orca-authz
	// backend-go/services/request-service/internal/adapter/opaclient/
	// Up to backend-go is 5 dirs: opaclient, adapter, internal, request-service, services.
	p := NewRequestPolicy("../../../../../policy/orca-authz")
	if err := p.Warm(context.Background()); err != nil {
		t.Fatalf("failed to warm policy: %v", err)
	}
	return p
}

func TestRequestPolicy_Matrix(t *testing.T) {
	p := setupPolicy(t)
	ctx := context.Background()

	cases := []struct {
		name string
		in   RequestPolicyInput
		want bool
	}{
		{"owner_triage", RequestPolicyInput{Action: "triage", CallerProjectRole: "owner", ActorType: "user"}, true},
		{"member_triage", RequestPolicyInput{Action: "triage", CallerProjectRole: "member", ActorType: "user"}, false},
		{"agent_classify_owner", RequestPolicyInput{Action: "triage", RPC: "ClassifyRequest", CallerProjectRole: "owner", ActorType: "agent"}, true},
		{"agent_approve", RequestPolicyInput{Action: "decide", RPC: "Approve", CallerProjectRole: "owner", ActorType: "agent"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := p.Decision(ctx, tc.in)
			if err != nil {
				t.Fatalf("Decision failed: %v", err)
			}
			if got != tc.want {
				t.Errorf("Decision(%+v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestRequestPolicy_BundleMissing(t *testing.T) {
	p := NewRequestPolicy("./nonexistent")
	if err := p.Warm(context.Background()); err == nil {
		t.Errorf("expected error for nonexistent bundle during Warm")
	}
}
