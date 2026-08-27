package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
)

func TestScanNested_CallsCreateConnectionThenRelay(t *testing.T) {
	relay := &fakeDevServerRelay{relayResult: []byte(`{"candidates":[]}`)}
	uc := NewScanNested(relay)

	ctx := withTenant(context.Background(), "tenant-1")
	if _, err := uc.Execute(ctx, ScanNestedInput{DevServerID: "dev-1", RootPath: "/home/dev"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(relay.createConnectionCalls) != 1 {
		t.Fatalf("expected 1 CreateConnection call, got %d", len(relay.createConnectionCalls))
	}
	got := relay.createConnectionCalls[0]
	if got.DevServerID != "dev-1" || got.RepoPath != "/home/dev" || got.WorktreeID != "" {
		t.Errorf("unexpected CreateConnection args: %+v", got)
	}

	if len(relay.relayCalls) != 1 {
		t.Fatalf("expected 1 Relay call, got %d", len(relay.relayCalls))
	}
	relayCall := relay.relayCalls[0]
	if relayCall.Method != "fs.scanNestedRepos" {
		t.Errorf("expected method fs.scanNestedRepos, got %q", relayCall.Method)
	}
	var params map[string]string
	if err := json.Unmarshal(relayCall.ParamsJSON, &params); err != nil {
		t.Fatalf("failed to decode params: %v", err)
	}
	if params["path"] != "/home/dev" {
		t.Errorf("expected params path=/home/dev, got %+v", params)
	}
}

func TestScanNested_RelayErrorFailsClosed(t *testing.T) {
	relay := &fakeDevServerRelay{relayErr: errors.New("agent unreachable")}
	uc := NewScanNested(relay)

	ctx := withTenant(context.Background(), "tenant-1")
	candidates, err := uc.Execute(ctx, ScanNestedInput{DevServerID: "dev-1", RootPath: "/home/dev"})
	assertAppError(t, err, apperrors.KindInternal, "PROJECT_SCAN_NESTED_FAILED")
	if candidates != nil {
		t.Errorf("expected no candidates on error, got %+v", candidates)
	}
}

func TestScanNested_CreateConnectionErrorMapsToFailedPrecondition(t *testing.T) {
	relay := &fakeDevServerRelay{createConnectionErr: errors.New("unknown dev server")}
	uc := NewScanNested(relay)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, ScanNestedInput{DevServerID: "unknown", RootPath: "/home/dev"})
	assertAppError(t, err, apperrors.KindFailedPrecondition, "PROJECT_DEV_SERVER_CONNECTION_FAILED")
	if len(relay.relayCalls) != 0 {
		t.Errorf("expected Relay to never be called when CreateConnection fails, got %d calls", len(relay.relayCalls))
	}
}

func TestScanNested_RequiresTenantContext(t *testing.T) {
	relay := &fakeDevServerRelay{}
	uc := NewScanNested(relay)

	_, err := uc.Execute(context.Background(), ScanNestedInput{DevServerID: "dev-1", RootPath: "/home/dev"})
	assertAppError(t, err, apperrors.KindUnauthenticated, "PROJECT_NO_TENANT")
}
