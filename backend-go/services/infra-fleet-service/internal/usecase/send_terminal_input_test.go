package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestSendTerminalInput_WritesExactBytesToAgent(t *testing.T) {
	resolver := &fakeConnectionResolver{
		byConnectionID: map[string]domain.DevServer{"conn-1": {ID: "ds-1"}},
	}
	sessions := &fakeTerminalSessionRepository{byPtyID: map[string]domain.TerminalSession{
		"pty-1": {PtyID: "pty-1", TenantID: "tenant-1", ConnectionID: "conn-1"},
	}}
	agent := &fakeDevServerAgentClient{}
	uc := NewSendTerminalInput(sessions, resolver, agent)

	data := []byte("echo hi\n")
	if err := uc.Execute(withTenant(context.Background(), "tenant-1"), "pty-1", data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agent.writePtyCalls) != 1 {
		t.Fatalf("expected exactly one WritePty call, got %d", len(agent.writePtyCalls))
	}
	if string(agent.writePtyCalls[0]) != string(data) {
		t.Errorf("expected exact bytes %q, got %q", data, agent.writePtyCalls[0])
	}
}

func TestSendTerminalInput_UnknownPtyID_ReturnsNotFoundWithoutWriting(t *testing.T) {
	resolver := &fakeConnectionResolver{}
	sessions := &fakeTerminalSessionRepository{}
	agent := &fakeDevServerAgentClient{}
	uc := NewSendTerminalInput(sessions, resolver, agent)

	err := uc.Execute(withTenant(context.Background(), "tenant-1"), "pty-unknown", []byte("data"))
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "INFRA_TERMINAL_NOT_FOUND" {
		t.Fatalf("expected INFRA_TERMINAL_NOT_FOUND, got %v", err)
	}
	if len(agent.writePtyCalls) != 0 {
		t.Error("expected WritePty NOT to be called for an unknown pty_id")
	}
}
