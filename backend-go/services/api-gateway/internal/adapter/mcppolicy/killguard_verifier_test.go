package mcppolicy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

type stubVerifier struct {
	p   mcpserver.Principal
	err error
}

func (s stubVerifier) Verify(context.Context, *http.Request) (mcpserver.Principal, error) {
	return s.p, s.err
}

func TestKillGuardVerifier(t *testing.T) {
	state := &mcpv1.GetKillStateResponse{}
	var stateErr error
	f := &fakeClient{killState: func(*mcpv1.GetKillStateRequest) (*mcpv1.GetKillStateResponse, error) { return state, stateErr }}
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)

	v := KillGuardVerifier{Inner: stubVerifier{p: principal}, Guard: NewKillGuard(f, 0)}
	if p, err := v.Verify(context.Background(), req); err != nil || p.UserID != "u1" {
		t.Fatalf("clear: %v", err)
	}

	state = &mcpv1.GetKillStateResponse{Active: true}
	v = KillGuardVerifier{Inner: stubVerifier{p: principal}, Guard: NewKillGuard(f, 0)}
	if _, err := v.Verify(context.Background(), req); !errors.Is(err, ErrKillSwitchActive) {
		t.Fatalf("blocked: %v", err)
	}

	stateErr = errors.New("down")
	v = KillGuardVerifier{Inner: stubVerifier{p: principal}, Guard: NewKillGuard(f, 0)}
	if _, err := v.Verify(context.Background(), req); !errors.Is(err, mcpserver.ErrVerifierUnavailable) {
		t.Fatalf("unreachable kill state must fail closed with 503 semantics: %v", err)
	}

	inner := errors.New("bad token")
	v = KillGuardVerifier{Inner: stubVerifier{err: inner}, Guard: NewKillGuard(f, 0)}
	if _, err := v.Verify(context.Background(), req); !errors.Is(err, inner) {
		t.Fatalf("inner errors pass through untouched: %v", err)
	}
}
