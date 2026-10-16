package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ExecutionModePinner stores the execution mode of a Request the first time it is asked to, and returns the stored mode
// on every later call. Both adapters' RequestRepository implement it.
type ExecutionModePinner interface {
	PinExecutionModeIfUnset(ctx context.Context, requestID string, mode domain.ExecutionMode) (domain.ExecutionMode, error)
}

// ExecutionModeResolver answers which mode a Request runs with: the pinned one, else the project's setting, else the
// deployment default. It never writes; the pin happens when the Request enters executing (PinExecutionMode).
type ExecutionModeResolver struct {
	Settings EngineSettingsRepository
	Defaults domain.EngineDefaults
}

// Mode reads the project setting only for a Request that is not pinned yet.
func (r *ExecutionModeResolver) Mode(ctx context.Context, req domain.Request) (domain.ExecutionMode, error) {
	if req.ExecutionMode != "" {
		return req.ExecutionMode, nil
	}
	var settings *domain.ProjectEngineSettings
	if r != nil && r.Settings != nil && req.ProjectID != "" {
		s, found, err := r.Settings.Get(ctx, req.ProjectID)
		if err != nil {
			return "", err
		}
		if found {
			settings = &s
		}
	}
	var defaults domain.EngineDefaults
	if r != nil {
		defaults = r.Defaults
	}
	return domain.EffectiveExecutionModeOf("", settings, defaults), nil
}

// PinExecutionMode fixes the mode of a Request that is entering executing. It is safe to call again: the first value wins.
func PinExecutionMode(ctx context.Context, pinner ExecutionModePinner, resolver *ExecutionModeResolver, req domain.Request) (domain.ExecutionMode, error) {
	if req.ExecutionMode != "" {
		return req.ExecutionMode, nil
	}
	mode, err := resolver.Mode(ctx, req)
	if err != nil {
		return "", err
	}
	if pinner == nil {
		return mode, nil
	}
	return pinner.PinExecutionModeIfUnset(ctx, req.ID, mode)
}
