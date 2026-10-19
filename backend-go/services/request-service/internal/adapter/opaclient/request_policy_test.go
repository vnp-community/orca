package opaclient

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestRequestPolicy_ReadyFollowsTheBundle(t *testing.T) {
	dir := t.TempDir()
	src, err := os.ReadFile("../../../../../policy/orca-authz/request.rego")
	if err != nil {
		entries, _ := filepath.Glob("../../../../../policy/orca-authz/*.rego")
		t.Fatalf("read the request rules: %v (found %v)", err, entries)
	}
	if err := os.WriteFile(filepath.Join(dir, "request.rego"), src, 0o600); err != nil {
		t.Fatal(err)
	}
	p := NewRequestPolicy(dir)
	p.evaluator.SetCheckPeriod(time.Millisecond)
	ctx := context.Background()
	if err := p.Ready(ctx); err != nil {
		t.Fatalf("a loadable bundle is ready: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "request.rego"), []byte("package orca.authz.request\nthis is not rego"), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := p.Ready(ctx); err == nil {
		t.Fatal("a bundle broken after startup must turn readiness red")
	}
}
