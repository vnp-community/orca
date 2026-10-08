package usecase

import (
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ErrLeaseLost means another instance owns the run now; the worker must stop without writing.
var ErrLeaseLost = errors.New("analysis run lease lost")

// AnalysisFailure ends a run as failed. Code and Message are stored on the run, Raw is the redacted model output.
type AnalysisFailure struct {
	Code    string
	Message string
	Raw     string
	// Attempt, Enforcement and RepoCheck keep what the worker learned before failing.
	Attempt     int
	Enforcement string
	RepoCheck   string
}

func (f *AnalysisFailure) Error() string { return fmt.Sprintf("%s: %s", f.Code, f.Message) }

func newFailure(run domain.AnalysisRun, code, msg string) *AnalysisFailure {
	return &AnalysisFailure{Code: code, Message: msg, Attempt: run.Attempt, Enforcement: run.Enforcement, RepoCheck: run.RepoCheck}
}

func asFailure(err error) (*AnalysisFailure, bool) {
	var f *AnalysisFailure
	if errors.As(err, &f) {
		return f, true
	}
	return nil, false
}
