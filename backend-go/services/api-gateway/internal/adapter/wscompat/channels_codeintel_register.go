package wscompat

import (
	"context"
	"encoding/json"
	"errors"
)

// CodeIntelLimits holds resource bounds for code intelligence operations.
type CodeIntelLimits struct {
	MaxResponseBytes int
	MaxStreams       int
}

// registerCodeIntelChannels registers all 46 code intelligence channels.
func registerCodeIntelChannels(r *Registry, d ChannelDeps) {
	deps := codeIntelDeps{
		core:    d.CodeIntel,
		quality: d.QualityGate,
		limits:  d.CodeIntelLimits,
	}
	if deps.limits.MaxResponseBytes <= 0 {
		deps.limits.MaxResponseBytes = 2 << 20
	}
	if deps.limits.MaxStreams <= 0 {
		deps.limits.MaxStreams = 500
	}

	// Group registration stubs (to be populated by respective solutions)
	registerCodeIntelViewChannels(r, deps)
	registerCodeIntelStateChannels(r, deps)
	registerCodeIntelSubscribe(r, deps)
	registerCodeIntelQualityChannels(r, deps)

	// Placeholder fallback: any catalog channel not yet registered receives
	// a placeholder returning CODEINTEL_UNAVAILABLE: channel not wired (G3 gateway contract).
	registerCodeIntelPlaceholders(r, deps)
}

func registerCodeIntelViewChannels(r *Registry, d codeIntelDeps) {
	registerCodeIntelStatus(r, d)
	registerCodeIntelStructure(r, d)
	registerCodeIntelRoutes(r, d)
	registerCodeIntelSourcesPlaceholders(r, d)
}
func registerCodeIntelStateChannels(_ *Registry, _ codeIntelDeps)   {}
func registerCodeIntelSubscribe(_ *Registry, _ codeIntelDeps)       {}
func registerCodeIntelQualityChannels(_ *Registry, _ codeIntelDeps) {}


func registerCodeIntelPlaceholders(r *Registry, _ codeIntelDeps) {
	registered := make(map[string]bool)
	for _, ch := range r.Channels() {
		registered[ch.Name] = true
	}

	for _, spec := range codeIntelChannelCatalog {
		if registered[spec.Name] {
			continue
		}
		if spec.Stream {
			r.RegisterStream(spec.Name, func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
				return nil, errors.New("CODEINTEL_UNAVAILABLE: channel not wired")
			})
		} else {
			r.Register(spec.Name, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
				return nil, errors.New("CODEINTEL_UNAVAILABLE: channel not wired")
			})
		}
	}
}
