package usecase

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestAIUnavailableCode_FindsTheCodeThroughWrapping(t *testing.T) {
	err := fmt.Errorf("classify: %w", &AIUnavailableError{Code: AICodeNotConfigured, Detail: "log in"})
	if code, ok := AIUnavailableCode(err); !ok || code != AICodeNotConfigured {
		t.Fatalf("got %q, %v", code, ok)
	}
	if _, ok := AIUnavailableCode(errors.New("plain")); ok {
		t.Fatal("a plain error has no AI code")
	}
}

func TestClassificationFailureUsesTheActionableAICode(t *testing.T) {
	err := &AIUnavailableError{Code: AICodeSessionExpired}
	if got := failureReason(err); got != AICodeSessionExpired {
		t.Errorf("reason = %q", got)
	}
	if got := failureCode(err); got != AICodeSessionExpired {
		t.Errorf("code = %q", got)
	}
	if got := failureReason(errors.New("boom")); got != "classifier error" {
		t.Errorf("an unrelated error must keep the generic reason, got %q", got)
	}
}

func TestCompletionFailureKeepsTheAICode(t *testing.T) {
	run := domain.AnalysisRun{ID: "r1"}
	f := completionFailure(run, &AIUnavailableError{Code: AICodeQuotaExhausted, Detail: "usage limit"})
	if f.Code != AICodeQuotaExhausted {
		t.Fatalf("run code = %q", f.Code)
	}
}
