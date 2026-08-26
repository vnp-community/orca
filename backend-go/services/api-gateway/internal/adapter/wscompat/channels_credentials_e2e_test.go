//go:build e2e

// TestCredentials_E2E_SetIsReadableBackViaStatusAndList is TASK-043's
// "Cross-service consistency test" — flagged as still-open in that task's
// own status note. It proves, against LIVE scm-integration-service,
// issue-tracking-service, and (transitively, through both)
// credential-broker-service containers — not the in-package fakes every
// other channels_credentials_test.go case uses — that a credential written
// through one wscompat channel (credentials.set) is actually readable back
// through the other two (credentials.status, credentials.list), and that
// credentials.revoke actually undoes it. A test built entirely on fakes
// cannot catch a real wire-format mismatch (e.g. a field name credential-
// broker-service and its two callers disagree on) or a real persistence
// bug; this test can.
//
// Requires SCM_INTEGRATION_SERVICE_ADDR and ISSUE_TRACKING_SERVICE_ADDR
// (the same env var names docker-compose.yml's x-go-common-env already
// defines for every backend-go service) to point at a live docker-compose
// stack — see deploy/dev/README.md for bringing one up. Skips (not fails)
// if either is unset, so a normal `go test ./...` run (no `e2e` build tag
// even reaches this file) and a CI job without a live stack both degrade
// gracefully rather than block on infrastructure this package's other
// tests don't need.
package wscompat

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	issuetrackingv1 "github.com/stablyai/orca-go/proto/gen/go/orca/issuetracking/v1"
	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
)

func realScmClient(t *testing.T) scmintegrationv1.ScmIntegrationServiceClient {
	t.Helper()
	addr := os.Getenv("SCM_INTEGRATION_SERVICE_ADDR")
	if addr == "" {
		t.Skip("SCM_INTEGRATION_SERVICE_ADDR not set — skipping, see this file's package doc comment")
	}
	conn, err := gatewaygrpc.Dial(addr)
	if err != nil {
		t.Fatalf("dialing scm-integration-service at %s: %v", addr, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return scmintegrationv1.NewScmIntegrationServiceClient(conn)
}

func realIssueTrackingClient(t *testing.T) issuetrackingv1.IssueTrackingServiceClient {
	t.Helper()
	addr := os.Getenv("ISSUE_TRACKING_SERVICE_ADDR")
	if addr == "" {
		t.Skip("ISSUE_TRACKING_SERVICE_ADDR not set — skipping, see this file's package doc comment")
	}
	conn, err := gatewaygrpc.Dial(addr)
	if err != nil {
		t.Fatalf("dialing issue-tracking-service at %s: %v", addr, err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return issuetrackingv1.NewIssueTrackingServiceClient(conn)
}

// TestCredentials_E2E_SetIsReadableBackViaStatusAndList covers the
// scm-integration-service-backed side (bitbucket) — the exact provider
// group TASK-042 added in this same change.
func TestCredentials_E2E_SetIsReadableBackViaStatusAndList(t *testing.T) {
	scmClient := realScmClient(t)
	issueClient := realIssueTrackingClient(t)

	r := NewRegistry()
	registerCredentialsChannels(r, scmClient, issueClient)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// Fresh tenant per run — this hits a real, persistent Postgres
	// (credential-broker-service's DB via docker-compose), not a
	// disposable testcontainer, so reusing a fixed tenant ID across runs
	// would make this test's outcome depend on a previous run's leftover
	// state.
	id := Identity{TenantID: uuid.NewString(), UserID: uuid.NewString()}

	setArgs := argsJSON(t, map[string]any{
		"service": "bitbucket",
		"token":   "e2e-real-token-" + uuid.NewString(),
		"config":  map[string]string{"workspace": "orca-e2e"},
	})
	if _, err := r.Dispatch(ctx, id, "credentials.set", setArgs); err != nil {
		t.Fatalf("credentials.set: %v", err)
	}

	statusArgs := argsJSON(t, map[string]any{"service": "bitbucket"})
	statusResult, err := r.Dispatch(ctx, id, "credentials.status", statusArgs)
	if err != nil {
		t.Fatalf("credentials.status: %v", err)
	}
	status, ok := statusResult.(credentialsStatusView)
	if !ok {
		t.Fatalf("expected credentialsStatusView, got %T", statusResult)
	}
	if !status.Configured {
		t.Fatal("expected bitbucket to be configured after credentials.set against a real scm-integration-service")
	}
	if status.Config["workspace"] != "orca-e2e" {
		t.Errorf("expected config_json to round-trip through the real service, got %+v", status.Config)
	}

	listResult, err := r.Dispatch(ctx, id, "credentials.list", nil)
	if err != nil {
		t.Fatalf("credentials.list: %v", err)
	}
	list, ok := listResult.(credentialsListView)
	if !ok {
		t.Fatalf("expected credentialsListView, got %T", listResult)
	}
	found := false
	for _, s := range list.Services {
		if s == "bitbucket" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected bitbucket in credentials.list's services, got %v", list.Services)
	}

	// credentials.revoke must undo credentials.set — proof the round trip
	// isn't a one-way write with a status handler that always says true.
	revokeArgs := argsJSON(t, map[string]any{"service": "bitbucket"})
	if _, err := r.Dispatch(ctx, id, "credentials.revoke", revokeArgs); err != nil {
		t.Fatalf("credentials.revoke: %v", err)
	}
	statusAfterRevoke, err := r.Dispatch(ctx, id, "credentials.status", statusArgs)
	if err != nil {
		t.Fatalf("credentials.status after revoke: %v", err)
	}
	if statusAfterRevoke.(credentialsStatusView).Configured {
		t.Error("expected bitbucket to be unconfigured after credentials.revoke against a real scm-integration-service")
	}
}

// TestCredentials_E2E_IssueTrackingSide mirrors the above against
// issue-tracking-service (jira) — the two backing services this channel
// group fans out to, both proven for real in one run.
func TestCredentials_E2E_IssueTrackingSide(t *testing.T) {
	scmClient := realScmClient(t)
	issueClient := realIssueTrackingClient(t)

	r := NewRegistry()
	registerCredentialsChannels(r, scmClient, issueClient)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	id := Identity{TenantID: uuid.NewString(), UserID: uuid.NewString()}

	setArgs := argsJSON(t, map[string]any{
		"service": "jira",
		"token":   "e2e-real-token-" + uuid.NewString(),
		"config":  map[string]string{"baseUrl": "https://orca-e2e.atlassian.net"},
	})
	if _, err := r.Dispatch(ctx, id, "credentials.set", setArgs); err != nil {
		t.Fatalf("credentials.set: %v", err)
	}

	statusArgs := argsJSON(t, map[string]any{"service": "jira"})
	statusResult, err := r.Dispatch(ctx, id, "credentials.status", statusArgs)
	if err != nil {
		t.Fatalf("credentials.status: %v", err)
	}
	status := statusResult.(credentialsStatusView)
	if !status.Configured {
		t.Fatal("expected jira to be configured after credentials.set against a real issue-tracking-service")
	}
	if status.Config["baseUrl"] != "https://orca-e2e.atlassian.net" {
		t.Errorf("expected config_json to round-trip through the real service, got %+v", status.Config)
	}

	listResult, err := r.Dispatch(ctx, id, "credentials.list", nil)
	if err != nil {
		t.Fatalf("credentials.list: %v", err)
	}
	list := listResult.(credentialsListView)
	found := false
	for _, s := range list.Services {
		if s == "jira" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected jira in credentials.list's services, got %v", list.Services)
	}
}
