package usecase

import (
	"time"

	"github.com/stablyai/orca-go/prompts/output_language"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// AnalysisSettings are the tunables of the analysis flow; zero fields fall back to the defaults below.
type AnalysisSettings struct {
	// AICompleteTimeout bounds one ai.complete call.
	AICompleteTimeout time.Duration
	// LeaseTTL is how long a worker may be silent before its run is considered dead.
	LeaseTTL time.Duration
	// Heartbeat is how often a worker renews its lease; keep it well under LeaseTTL.
	Heartbeat time.Duration
	// RecoveryInterval is how often the sweeper looks for expired leases.
	RecoveryInterval time.Duration
	// AgentTimeoutMS bounds one read-only agent run; HotfixTimeoutMS is the shorter hotfix budget.
	AgentTimeoutMS  int
	HotfixTimeoutMS int
	// SpecTimeoutMS bounds one OpenSpec authoring run; it equals the agent's own 30 minute ceiling, since a longer ask is cut there.
	SpecTimeoutMS int
	// SpecTotalTimeoutMS bounds an authoring run with its one retry; each attempt still has its own SpecTimeoutMS.
	SpecTotalTimeoutMS int
	// MaxAgentRunsPerProject caps concurrent read-only runs on one dev server (a double click once dropped its connection).
	MaxAgentRunsPerProject int
	// UseAgentReadonlyFlag lets the runner send accessMode=readonly to agents that advertise it; off forces prompt-only runs.
	UseAgentReadonlyFlag bool
	// RequireEnforcedReadonly fails runs on agents that cannot enforce read-only instead of falling back to prompt-only.
	RequireEnforcedReadonly bool
	// OutputLanguage is the language the AI writes analysis and Plan prose in. Zero adds no instruction, so DefaultAnalysisSettings
	// and withDefaults leave it empty; the service sets it from REQUEST_DEFAULT_OUTPUT_LANGUAGE.
	OutputLanguage output_language.Language
}

// OutputLanguageFor is the only reader of OutputLanguage; it is where a per-project or per-Request override would plug in.
func (s AnalysisSettings) OutputLanguageFor(domain.Request) output_language.Language {
	return s.OutputLanguage
}

func DefaultAnalysisSettings() AnalysisSettings {
	return AnalysisSettings{
		AICompleteTimeout: 120 * time.Second,
		LeaseTTL:          90 * time.Second,
		Heartbeat:         30 * time.Second,
		RecoveryInterval:  30 * time.Second,
		AgentTimeoutMS:    600_000,
		HotfixTimeoutMS:   300_000,
		SpecTimeoutMS:     1_800_000,
		// SpecTotalTimeoutMS stays 0 here: withDefaults derives it from SpecTimeoutMS, so a shorter attempt budget shrinks it too.
		MaxAgentRunsPerProject: 2,
		UseAgentReadonlyFlag:   true,
	}
}

// withDefaults fills zero values only, so a test can set one field and keep the rest sensible.
func (s AnalysisSettings) withDefaults() AnalysisSettings {
	d := DefaultAnalysisSettings()
	if s.AICompleteTimeout <= 0 {
		s.AICompleteTimeout = d.AICompleteTimeout
	}
	if s.LeaseTTL <= 0 {
		s.LeaseTTL = d.LeaseTTL
	}
	if s.Heartbeat <= 0 {
		s.Heartbeat = d.Heartbeat
	}
	if s.RecoveryInterval <= 0 {
		s.RecoveryInterval = d.RecoveryInterval
	}
	if s.AgentTimeoutMS <= 0 {
		s.AgentTimeoutMS = d.AgentTimeoutMS
	}
	if s.HotfixTimeoutMS <= 0 {
		s.HotfixTimeoutMS = d.HotfixTimeoutMS
	}
	if s.SpecTimeoutMS <= 0 {
		s.SpecTimeoutMS = d.SpecTimeoutMS
	}
	if s.SpecTotalTimeoutMS <= 0 {
		// Two attempts and a little slack, derived from the attempt budget unless the operator set one.
		s.SpecTotalTimeoutMS = 2*s.SpecTimeoutMS + 120_000
	}
	if s.MaxAgentRunsPerProject <= 0 {
		s.MaxAgentRunsPerProject = d.MaxAgentRunsPerProject
	}
	return s
}
