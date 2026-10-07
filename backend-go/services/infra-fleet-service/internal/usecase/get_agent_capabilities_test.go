package usecase

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestGetAgentCapabilities_RequiresTenantContext(t *testing.T) {
	uc := NewGetAgentCapabilities(&fakeDevServerRepository{}, &fakeDevServerAgentClient{})
	_, err := uc.Execute(context.Background(), "ds-1")
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) && appErr.Kind != apperrors.KindUnauthenticated {
		t.Errorf("expected KindUnauthenticated, got %v", appErr.Kind)
	}
}

func TestGetAgentCapabilities_RequiresDevServerID(t *testing.T) {
	uc := NewGetAgentCapabilities(&fakeDevServerRepository{}, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.Execute(ctx, "")
	if err == nil {
		t.Fatal("expected an error when devServerId is omitted")
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) && appErr.Kind != apperrors.KindInvalidArgument {
		t.Errorf("expected KindInvalidArgument, got %v", appErr.Kind)
	}
}

func TestGetAgentCapabilities_DevServerNotFoundOrOtherTenant(t *testing.T) {
	repo := &fakeDevServerRepository{getErr: errors.New("dev server not found")}
	uc := NewGetAgentCapabilities(repo, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")

	_, err := uc.Execute(ctx, "ds-other-tenant")
	if err == nil {
		t.Fatal("expected an error for unknown or other tenant dev server")
	}
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) && appErr.Kind != apperrors.KindNotFound {
		t.Errorf("expected KindNotFound, got %v", appErr.Kind)
	}
}

func TestGetAgentCapabilities_OnlineFullCapabilities(t *testing.T) {
	ds, err := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.5", domain.ConnectionModeDirectWebSocket, "", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	repo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds-1": ds}}
	ctx := withTenant(context.Background(), "tenant-1")

	agent := &fakeDevServerAgentClient{
		isConnected:     true,
		lastHandshakeOK: true,
		lastHandshakeInfo: HandshakeInfo{
			Platform:        "linux",
			Arch:            "arm64",
			NodeVersion:     "v22.0.0",
			AgentVersion:    "1.5.0",
			SessionID:       "sess-online-1",
			Capabilities:    []string{"pty", "codeintel"},
			Tools:           []string{"gitnexus", "codegraph"},
			Features:        []string{"feat-1"},
			ProtocolVersion: 2,
		},
	}

	got, err := NewGetAgentCapabilities(repo, agent).Execute(ctx, "ds-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := domain.AgentCapabilities{
		Connected:    true,
		Platform:     "linux",
		Arch:         "arm64",
		NodeVersion:  "v22.0.0",
		AgentVersion: "1.5.0",
		Capabilities: []string{"pty", "codeintel"},
		Tools:        []string{"gitnexus", "codegraph"},
		SessionID:    "sess-online-1",
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

func TestGetAgentCapabilities_OfflineReturnsConnectedFalse(t *testing.T) {
	ds, err := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.5", domain.ConnectionModeDirectWebSocket, "", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	repo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds-1": ds}}
	ctx := withTenant(context.Background(), "tenant-1")

	agent := &fakeDevServerAgentClient{
		isConnected: false,
	}

	got, err := NewGetAgentCapabilities(repo, agent).Execute(ctx, "ds-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := domain.AgentCapabilities{
		Connected: false,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

func TestGetAgentCapabilities_RaceConnectedTrueLastHandshakeMissing(t *testing.T) {
	ds, err := domain.NewDevServer("ds-1", "tenant-1", "10.0.0.5", domain.ConnectionModeDirectWebSocket, "", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	repo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds-1": ds}}
	ctx := withTenant(context.Background(), "tenant-1")

	// Connected is true, but LastHandshakeInfo ok is false
	agent := &fakeDevServerAgentClient{
		isConnected:     true,
		lastHandshakeOK: false,
	}

	got, err := NewGetAgentCapabilities(repo, agent).Execute(ctx, "ds-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := domain.AgentCapabilities{
		Connected: true,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}
