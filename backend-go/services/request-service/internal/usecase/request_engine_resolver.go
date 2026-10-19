package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RequestEngineResolver fills Request.Engine on what a read returns: the engine the Request runs on, which the person looking at
// it cannot work out from the type alone (a bug or a security request can run on OpenSpec when its project says so, and a
// Request keeps the engine it was pinned with). It reads the project settings once per project per call.
type RequestEngineResolver struct {
	Settings EngineSettingsRepository
	// Defaults are the deployment's execution-mode default for a project that sets none.
	Defaults domain.EngineDefaults
	Log      *slog.Logger
}

// Annotate sets Engine on every request, and ExecutionMode on those not pinned yet (what they will run with, so a screen can
// describe the approval before it happens). When a project's settings cannot be read the engine stays empty (unknown) instead
// of failing the read, and the screen then shows no engine-specific part.
func (r *RequestEngineResolver) Annotate(ctx context.Context, reqs []domain.Request) {
	if r == nil {
		return
	}
	type lookup struct {
		settings *domain.ProjectEngineSettings
		failed   bool
	}
	byProject := map[string]lookup{}
	for i := range reqs {
		l, seen := byProject[reqs[i].ProjectID]
		if !seen {
			if reqs[i].ProjectID != "" && r.Settings != nil {
				s, found, err := r.Settings.Get(ctx, reqs[i].ProjectID)
				switch {
				case err != nil:
					l.failed = true
					r.log().WarnContext(ctx, "engine settings unreadable; the request engine is left unknown", slog.String("project_id", reqs[i].ProjectID), slog.Any("error", err))
				case found:
					l.settings = &s
				}
			}
			byProject[reqs[i].ProjectID] = l
		}
		if reqs[i].ExecutionMode == "" && !l.failed {
			reqs[i].ExecutionMode = domain.EffectiveExecutionModeOf("", l.settings, r.Defaults)
		}
		profile := domain.OpenSpecProfileFor(reqs[i].Type)
		if profile == domain.OpenSpecProfileNone {
			reqs[i].Engine = domain.EngineNative
			continue
		}
		if l.failed && reqs[i].SolutionEngine == nil {
			continue
		}
		reqs[i].Engine = domain.EffectiveEngine(reqs[i].SolutionEngine, l.settings, profile)
	}
}

func (r *RequestEngineResolver) log() *slog.Logger {
	if r.Log != nil {
		return r.Log
	}
	return slog.Default()
}
