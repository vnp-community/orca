package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestGetHostCapabilities_NoConnectionID_ReturnsHonestFalseWithoutRelaying(t *testing.T) {
	agent := &fakeDevServerAgentClient{}
	uc := NewGetHostCapabilities(&fakeConnectionResolver{}, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.WslAvailable || result.PwshAvailable || result.GitBashAvailable || len(result.WslDistros) != 0 {
		t.Errorf("expected an all-false/empty answer, got %+v", result)
	}
	if len(agent.execCalls) != 0 {
		t.Error("expected no relay to the agent when no connectionId is set")
	}
}

func TestGetHostCapabilities_ConnectionIDNotResolved_ReturnsHonestFalseWithoutRelaying(t *testing.T) {
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{}}
	agent := &fakeDevServerAgentClient{}
	uc := NewGetHostCapabilities(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	result, err := uc.Execute(ctx, "unknown-conn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.WslAvailable {
		t.Error("expected wslAvailable=false when the connectionId doesn't resolve")
	}
	if len(agent.execCalls) != 0 {
		t.Error("expected no relay to the agent when the connectionId doesn't resolve to a live dev server")
	}
}

// Direct regression test for TASK-070's "shipped but honestly inert"
// contract: a resolved connection relays for real and a real
// domain.ErrAgentMethodNotFound translates into a typed, permanent
// FailedPrecondition — not a silent honest-false fallback, since (unlike
// the no-connection case) a connection IS bound and driving it is a real,
// failed attempt worth surfacing, not something to paper over.
func TestGetHostCapabilities_AgentMethodNotFound_TranslatesToFailedPrecondition(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelaySSH, "ssht1")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{execErr: domain.ErrAgentMethodNotFound}
	uc := NewGetHostCapabilities(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	_, execErr := uc.Execute(ctx, "conn-1")
	if execErr == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(execErr, &ae) {
		t.Fatalf("expected an *apperrors.AppError, got %T: %v", execErr, execErr)
	}
	if ae.Kind != apperrors.KindFailedPrecondition {
		t.Errorf("expected KindFailedPrecondition, got %v", ae.Kind)
	}
	if ae.Code != "INFRA_HOST_CAPABILITIES_UNSUPPORTED" {
		t.Errorf("expected INFRA_HOST_CAPABILITIES_UNSUPPORTED, got %q", ae.Code)
	}
	if len(agent.execCalls) != 1 || agent.execCalls[0] != "host.capabilities" {
		t.Fatalf("expected exactly one host.capabilities relay call, got %v", agent.execCalls)
	}
}

func TestGetHostCapabilities_AgentOtherError_IsInternal(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelaySSH, "ssht1")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{execErr: errors.New("devserveragent: not connected")}
	uc := NewGetHostCapabilities(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	_, execErr := uc.Execute(ctx, "conn-1")
	var ae *apperrors.AppError
	if !errors.As(execErr, &ae) || ae.Kind != apperrors.KindInternal {
		t.Fatalf("expected KindInternal for a non-method-not-found failure, got %v", execErr)
	}
}

func TestGetHostCapabilities_Success_DecodesFullResult(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelaySSH, "ssht1")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{execResult: map[string]any{
		"wslAvailable": true, "wslDistros": []any{"Ubuntu", "Debian"},
		"pwshAvailable": true, "gitBashAvailable": false,
	}}
	uc := NewGetHostCapabilities(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	result, execErr := uc.Execute(ctx, "conn-1")
	if execErr != nil {
		t.Fatalf("unexpected error: %v", execErr)
	}
	if !result.WslAvailable || !result.PwshAvailable || result.GitBashAvailable {
		t.Errorf("unexpected bool fields: %+v", result)
	}
	if len(result.WslDistros) != 2 || result.WslDistros[0] != "Ubuntu" {
		t.Errorf("unexpected distros: %v", result.WslDistros)
	}
}
