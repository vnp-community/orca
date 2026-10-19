package usecase

import "github.com/stablyai/orca-go/services/request-service/internal/domain"

// EngineObserver receives the facts that become metrics about the engines. Implementations must keep their labels to small
// fixed sets and must never take what a tool read or said: tool names, run kinds and violation codes only.
type EngineObserver interface {
	// RunFinished is one agent or engine run that ended: engine native|openspec, kind the run kind, result succeeded|failed.
	RunFinished(engine, kind, result string)
	// PreflightFailed is one blocker the readiness check found.
	PreflightFailed(code string)
	// KnowledgeToolCalls is how many times each knowledge-graph tool was called in one agent call.
	KnowledgeToolCalls(stats domain.RunStats)
	// SpecViolation is one structure problem found in an authored change.
	SpecViolation(code string)
}

// NoopEngineObserver is what the use cases use when none is wired.
type NoopEngineObserver struct{}

func (NoopEngineObserver) RunFinished(string, string, string) {}
func (NoopEngineObserver) PreflightFailed(string)             {}
func (NoopEngineObserver) KnowledgeToolCalls(domain.RunStats) {}
func (NoopEngineObserver) SpecViolation(string)               {}

var _ EngineObserver = NoopEngineObserver{}

func observerOrNoop(o EngineObserver) EngineObserver {
	if o == nil {
		return NoopEngineObserver{}
	}
	return o
}

// statsPtr is nil for stats with nothing in them, so a run row keeps NULL instead of an empty object.
func statsPtr(s domain.RunStats) *domain.RunStats {
	if s.IsZero() {
		return nil
	}
	c := s
	return &c
}

// engineLabel names the engine that ran: the openspec engine, or the native path (ai.complete or the read-only agent).
func engineLabel(run domain.AnalysisRun) string {
	if run.Mode == domain.AnalysisModeAgentProposal {
		return string(domain.EngineOpenSpec)
	}
	return string(domain.EngineNative)
}

func valueOrZero(s *domain.RunStats) domain.RunStats {
	if s == nil {
		return domain.RunStats{}
	}
	return *s
}
