package grpcclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

func TestMapAIRelayError(t *testing.T) {
	cases := []struct {
		message string
		want    string
	}{
		{"AI_COMPLETE_CLI_NOT_LOGGED_IN: Not logged in · Please run /login", usecase.AICodeNotConfigured},
		{"AI_COMPLETE_NOT_CONFIGURED: No AI credential", usecase.AICodeNotConfigured},
		{"AI_COMPLETE_SESSION_EXPIRED: token expired", usecase.AICodeSessionExpired},
		{"AI_COMPLETE_QUOTA_EXHAUSTED: usage limit", usecase.AICodeQuotaExhausted},
		{"AI_COMPLETE_BUSY: busy", usecase.AICodeBusy},
		{"AI_COMPLETE_UNSUPPORTED: old agent", usecase.AICodeAgentUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.message, func(t *testing.T) {
			got := mapAIRelayError(status.Error(codes.FailedPrecondition, tc.message))
			code, ok := usecase.AIUnavailableCode(got)
			if !ok || code != tc.want {
				t.Fatalf("got %v (%v), want %s", got, ok, tc.want)
			}
		})
	}
}

func TestMapAIRelayError_LeavesOtherErrorsAlone(t *testing.T) {
	for _, err := range []error{
		status.Error(codes.DeadlineExceeded, "AI_COMPLETE_TIMEOUT: ai.complete timed out"),
		status.Error(codes.Internal, "INFRA_AGENT_EXEC_FAILED: failed to relay"),
		errors.New("plain"),
		status.Error(codes.Internal, "no colon here"),
	} {
		if got := mapAIRelayError(err); got != err {
			t.Errorf("%v was rewritten to %v", err, got)
		}
	}
}

type scriptedCaps struct {
	first, fresh domain.DevServerCapability
	firstErr     error
	freshCalls   int
}

func (s *scriptedCaps) Get(_ context.Context, _ usecase.DevServerRef, refresh bool) (domain.DevServerCapability, error) {
	if refresh {
		s.freshCalls++
		return s.fresh, nil
	}
	return s.first, s.firstErr
}

func profile(connected bool, aiSources ...string) domain.DevServerCapability {
	return domain.DevServerCapability{Source: "agent", Connected: connected, AISourcesKnown: true, AISources: aiSources}
}

func TestCheckAIReady(t *testing.T) {
	conn := AIConnection{DevServerID: "ds-1"}
	t.Run("no reader configured lets the call proceed", func(t *testing.T) {
		if err := checkAIReady(context.Background(), nil, conn); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("an unreadable profile never blocks", func(t *testing.T) {
		if err := checkAIReady(context.Background(), &scriptedCaps{firstErr: usecase.ErrCapabilityNotAvailable}, conn); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("a ready dev server passes without a fresh read", func(t *testing.T) {
		caps := &scriptedCaps{first: profile(true, "cli_session")}
		if err := checkAIReady(context.Background(), caps, conn); err != nil || caps.freshCalls != 0 {
			t.Fatalf("err=%v freshCalls=%d", err, caps.freshCalls)
		}
	})
	t.Run("not configured is re-read once, so a fresh login is noticed", func(t *testing.T) {
		caps := &scriptedCaps{first: profile(true), fresh: profile(true, "cli_session")}
		if err := checkAIReady(context.Background(), caps, conn); err != nil || caps.freshCalls != 1 {
			t.Fatalf("err=%v freshCalls=%d", err, caps.freshCalls)
		}
	})
	t.Run("still not configured after the fresh read is a typed error with the remedy", func(t *testing.T) {
		caps := &scriptedCaps{first: profile(true), fresh: profile(true)}
		err := checkAIReady(context.Background(), caps, conn)
		if code, ok := usecase.AIUnavailableCode(err); !ok || code != usecase.AICodeNotConfigured {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("a disconnected agent is reported as such", func(t *testing.T) {
		err := checkAIReady(context.Background(), &scriptedCaps{first: profile(false, "cli_session")}, conn)
		if code, ok := usecase.AIUnavailableCode(err); !ok || code != usecase.AICodeAgentUnavailable {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("an unknown profile proceeds", func(t *testing.T) {
		unknown := domain.DevServerCapability{Source: "agent", Connected: true}
		if err := checkAIReady(context.Background(), &scriptedCaps{first: unknown, fresh: unknown}, conn); err != nil {
			t.Fatal(err)
		}
	})
}
