package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func testEmulatorDevServer(t *testing.T) domain.DevServer {
	t.Helper()
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelaySSH, "ssht1")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	return ds
}

func TestEmulatorRelay_ListDevices_NoConnectionID_FailsPrecondition(t *testing.T) {
	uc := NewEmulatorRelay(&fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.ListDevices(ctx, "")
	if err == nil {
		t.Fatal("expected an error — emulator control has no local/backend-host fallback")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("expected KindFailedPrecondition, got %v", err)
	}
}

func TestEmulatorRelay_ListDevices_UnresolvedConnection_FailsPrecondition(t *testing.T) {
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{}}
	uc := NewEmulatorRelay(resolver, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.ListDevices(ctx, "unknown-conn")
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("expected KindFailedPrecondition, got %v", err)
	}
}

// This is the direct regression test for TASK-048's "shipped but honestly
// inert" contract: a resolved connection relays for real and a real
// domain.ErrAgentMethodNotFound translates into a typed, permanent
// FailedPrecondition — not a panic, not fabricated data.
func TestEmulatorRelay_ListDevices_AgentMethodNotFound_TranslatesToFailedPrecondition(t *testing.T) {
	ds := testEmulatorDevServer(t)
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{execErr: domain.ErrAgentMethodNotFound}
	uc := NewEmulatorRelay(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.ListDevices(ctx, "conn-1")
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected an *apperrors.AppError, got %T: %v", err, err)
	}
	if ae.Kind != apperrors.KindFailedPrecondition {
		t.Errorf("expected KindFailedPrecondition, got %v", ae.Kind)
	}
	if ae.Code != "INFRA_EMULATOR_UNSUPPORTED" {
		t.Errorf("expected INFRA_EMULATOR_UNSUPPORTED, got %q", ae.Code)
	}
	if len(agent.execCalls) != 1 || agent.execCalls[0] != "device.list" {
		t.Fatalf("expected exactly one device.list relay call, got %v", agent.execCalls)
	}
}

func TestEmulatorRelay_ListDevices_AgentOtherError_IsInternal(t *testing.T) {
	ds := testEmulatorDevServer(t)
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{execErr: errors.New("devserveragent: not connected")}
	uc := NewEmulatorRelay(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err := uc.ListDevices(ctx, "conn-1")
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindInternal {
		t.Fatalf("expected KindInternal for a non-method-not-found failure, got %v", err)
	}
}

func TestEmulatorRelay_ListDevices_Success_DecodesDevices(t *testing.T) {
	ds := testEmulatorDevServer(t)
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{execResult: map[string]any{
		"devices": []any{
			map[string]any{"id": "emulator-5554", "name": "Pixel 6", "platform": "android", "state": "booted"},
		},
	}}
	uc := NewEmulatorRelay(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	devices, err := uc.ListDevices(ctx, "conn-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(devices) != 1 || devices[0].ID != "emulator-5554" || devices[0].Platform != "android" {
		t.Errorf("unexpected devices: %+v", devices)
	}
}

func TestEmulatorRelay_GetAvailability_NoConnectionID_IsHonestFalseNotError(t *testing.T) {
	uc := NewEmulatorRelay(&fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")
	available, reason, err := uc.GetAvailability(ctx, "")
	if err != nil {
		t.Fatalf("expected no error for GetAvailability with no connection — it's the one honest-false method, got %v", err)
	}
	if available {
		t.Error("expected available=false with no connection")
	}
	if reason == "" {
		t.Error("expected a non-empty reason")
	}
}

func TestEmulatorRelay_GetAvailability_AgentMethodNotFound_IsHonestFalseNotError(t *testing.T) {
	ds := testEmulatorDevServer(t)
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{execErr: domain.ErrAgentMethodNotFound}
	uc := NewEmulatorRelay(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	available, _, err := uc.GetAvailability(ctx, "conn-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if available {
		t.Error("expected available=false when the agent doesn't support device.availability")
	}
}

func TestEmulatorRelay_SendCommand_NoConnectionID_FailsPrecondition(t *testing.T) {
	uc := NewEmulatorRelay(&fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	ctx := withTenant(context.Background(), "tenant-1")
	err := uc.SendCommand(ctx, "", "device.tap", map[string]any{"x": 1, "y": 2})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("expected KindFailedPrecondition, got %v", err)
	}
}

func TestEmulatorRelay_SendCommand_AgentMethodNotFound_TranslatesToFailedPrecondition(t *testing.T) {
	ds := testEmulatorDevServer(t)
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{execErr: domain.ErrAgentMethodNotFound}
	uc := NewEmulatorRelay(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	err := uc.SendCommand(ctx, "conn-1", "device.tap", map[string]any{"x": 1, "y": 2})
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition || ae.Code != "INFRA_EMULATOR_UNSUPPORTED" {
		t.Fatalf("expected INFRA_EMULATOR_UNSUPPORTED/KindFailedPrecondition, got %v", err)
	}
	if len(agent.execCalls) != 1 || agent.execCalls[0] != "device.tap" {
		t.Fatalf("expected exactly one device.tap relay call, got %v", agent.execCalls)
	}
}

func TestEmulatorRelay_AttachSession_Success(t *testing.T) {
	ds := testEmulatorDevServer(t)
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{execResult: map[string]any{
		"sessionId": "sess-1", "deviceId": "emulator-5554", "platform": "android",
	}}
	uc := NewEmulatorRelay(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	session, err := uc.AttachSession(ctx, "conn-1", "emulator-5554")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.SessionID != "sess-1" || session.ConnectionID != "conn-1" || session.Platform != "android" {
		t.Errorf("unexpected session: %+v", session)
	}
	if len(agent.execCalls) != 1 || agent.execCalls[0] != "device.attach" {
		t.Fatalf("expected exactly one device.attach relay call, got %v", agent.execCalls)
	}
}
