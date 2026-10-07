package devserveragent

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestClientExec_CodeIntelStructuredErrorPreserved(t *testing.T) {
	agent := &fakeAgent{
		t:            t,
		requireToken: fakeAgentToken,
		rpcErrors: map[string]*JSONRPCError{
			"codeintel.overview": {
				Code:    -32000,
				Message: "index missing",
				Data:    json.RawMessage(`{"code":"CODEINTEL_INDEX_MISSING"}`),
			},
		},
	}
	host, port := startFakeAgent(t, agent)

	client := newTestClientWithToken(port, fakeAgentToken)
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-ci-1", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	_, err = client.Exec(context.Background(), devServer, "codeintel.overview", map[string]any{"repo": "orca"})
	if err == nil {
		t.Fatal("expected error from codeintel.overview, got nil")
	}

	var rpcErr *domain.AgentRPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected error to be *domain.AgentRPCError, got %T (%v)", err, err)
	}
	if rpcErr.Code != -32000 {
		t.Errorf("expected code -32000, got %d", rpcErr.Code)
	}
	if rpcErr.Message != "index missing" {
		t.Errorf("expected message 'index missing', got %q", rpcErr.Message)
	}
	if string(rpcErr.Data) != `{"code":"CODEINTEL_INDEX_MISSING"}` {
		t.Errorf("expected Data preserved, got %s", string(rpcErr.Data))
	}
}

func TestClientExec_CodeIntelMethodNotFound_UnwrapsErrAgentMethodNotFound(t *testing.T) {
	agent := &fakeAgent{
		t:            t,
		requireToken: fakeAgentToken,
		rpcErrors: map[string]*JSONRPCError{
			"codeintel.status": {
				Code:    -32601,
				Message: "Method not found: codeintel.status",
			},
		},
	}
	host, port := startFakeAgent(t, agent)

	client := newTestClientWithToken(port, fakeAgentToken)
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-ci-2", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	_, err = client.Exec(context.Background(), devServer, "codeintel.status", nil)
	if err == nil {
		t.Fatal("expected error from codeintel.status, got nil")
	}

	if !errors.Is(err, domain.ErrAgentMethodNotFound) {
		t.Errorf("expected errors.Is(err, domain.ErrAgentMethodNotFound) to be true, got %v", err)
	}

	var rpcErr *domain.AgentRPCError
	if !errors.As(err, &rpcErr) {
		t.Errorf("expected errors.As(err, &rpcErr) to be true for codeintel method, got %T", err)
	}
}

func TestClientExec_NonCodeIntelMethod_ReturnsLegacyErrorType(t *testing.T) {
	agent := &fakeAgent{
		t:            t,
		requireToken: fakeAgentToken,
		rpcErrors: map[string]*JSONRPCError{
			"ports.scan": {
				Code:    -32000,
				Message: "internal error in ports scan",
			},
		},
	}
	host, port := startFakeAgent(t, agent)

	client := newTestClientWithToken(port, fakeAgentToken)
	t.Cleanup(client.Close)

	devServer, err := domain.NewDevServer("ds-ci-3", "tenant-1", host, domain.ConnectionModeRelayWebSocket, "", nil)
	if err != nil {
		t.Fatalf("NewDevServer: %v", err)
	}

	_, err = client.Exec(context.Background(), devServer, "ports.scan", nil)
	if err == nil {
		t.Fatal("expected error from ports.scan, got nil")
	}

	var rpcErr *domain.AgentRPCError
	if errors.As(err, &rpcErr) {
		t.Errorf("expected non-codeintel method NOT to return *domain.AgentRPCError, got %v", rpcErr)
	}
}
