package grpcclient

import (
	"errors"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

func completeVia(infra *fakeAgentInfra, conn AIConnection) (string, error) {
	return relayAIComplete(ctxWithIdentity(), newTenantRelayer(infra), conn, "", "p", time.Second)
}

func TestRelayAIComplete_TypedRefusalBecomesTheServiceCode(t *testing.T) {
	cause := status.Error(codes.FailedPrecondition, "AI_COMPLETE_SESSION_EXPIRED:  token expired ")
	_, err := completeVia(&fakeAgentInfra{err: cause}, AIConnection{ConnectionID: "c"})
	var unavailable *usecase.AIUnavailableError
	if !errors.As(err, &unavailable) || unavailable.Code != usecase.AICodeSessionExpired || unavailable.Detail != "token expired" || !errors.Is(err, cause) {
		t.Fatalf("err = %#v", err)
	}
}

func TestRelayAIComplete_UnknownCodeAndPlainErrorsStayWrappedWithTheirCause(t *testing.T) {
	for name, cause := range map[string]error{
		"unknown AI_COMPLETE code": status.Error(codes.Internal, "AI_COMPLETE_SOMETHING_NEW: x"),
		"not an AI_COMPLETE code":  status.Error(codes.Internal, "INFRA_AGENT_EXEC_FAILED: failed to relay"),
		"plain":                    errors.New("boom"),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := completeVia(&fakeAgentInfra{err: cause}, AIConnection{DevServerID: "ds"})
			if err == nil || !errors.Is(err, cause) || errors.Is(err, usecase.ErrClassifierTimeout) {
				t.Fatalf("err = %v", err)
			}
			if _, isAI := usecase.AIUnavailableCode(err); isAI {
				t.Fatalf("%v must not read as a user-fixable AI problem", err)
			}
			if !strings.Contains(err.Error(), "ai.complete") {
				t.Fatalf("the failing call is not named: %v", err)
			}
		})
	}
}

func TestRelayAIComplete_RemoteDeadlineIsTheClassifierTimeout(t *testing.T) {
	_, err := completeVia(&fakeAgentInfra{err: status.Error(codes.DeadlineExceeded, "AI_COMPLETE_TIMEOUT: ai.complete timed out")}, AIConnection{ConnectionID: "c"})
	if !errors.Is(err, usecase.ErrClassifierTimeout) {
		t.Fatalf("err = %v", err)
	}
}

func TestRelayAIComplete_MalformedAnswerIsAnError(t *testing.T) {
	if _, err := completeVia(&fakeAgentInfra{result: "not json"}, AIConnection{ConnectionID: "c"}); err == nil {
		t.Fatal("garbage result must error")
	}
}

func TestRelayAIComplete_ForwardsTenantAndTargetsByConnectionOrDevServer(t *testing.T) {
	byConn := &fakeAgentInfra{result: `{"content":"a"}`}
	if got, err := completeVia(byConn, AIConnection{ConnectionID: "c"}); err != nil || got != "a" || len(byConn.relay) != 1 || len(byConn.dev) != 0 || len(byConn.md.Get("x-orca-tenant-id")) != 1 {
		t.Fatalf("by connection: %q %v %+v", got, err, byConn)
	}
	byDev := &fakeAgentInfra{result: `{"content":"b"}`}
	if got, err := completeVia(byDev, AIConnection{ConnectionID: "c", DevServerID: "ds"}); err != nil || got != "b" || len(byDev.dev) != 1 || len(byDev.relay) != 0 {
		t.Fatalf("by dev server: %q %v %+v", got, err, byDev)
	}
}

func TestAgentPromptRelay_ReadOnlyWithWriteProfileIsRefusedWithTheSharedSentinel(t *testing.T) {
	_, err := NewAgentPromptRelay(&fakeAgentInfra{}).ExecPrompt(ctxWithIdentity(), usecase.AnalysisConnection{ConnectionID: "c"},
		usecase.AgentPromptInput{Prompt: "p", ReadOnlyEnforced: true, WriteProfile: "spec-author"})
	if err == nil || !strings.Contains(err.Error(), "read-only and a write profile") {
		t.Fatalf("err = %v", err)
	}
}
