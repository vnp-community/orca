package grpcclient

import (
	"context"
	"strings"

	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

const aiNotConfigured = domain.AINotConfigured

// aiCompleteCodes maps the typed errors infra-fleet returns for ai.complete (its MapAgentExecError) to the
// codes request-service shows. infra-fleet's gRPC message is "CODE: text".
var aiCompleteCodes = map[string]string{
	"AI_COMPLETE_NOT_CONFIGURED":    usecase.AICodeNotConfigured,
	"AI_COMPLETE_CLI_NOT_LOGGED_IN": usecase.AICodeNotConfigured,
	"AI_COMPLETE_CLI_UNAVAILABLE":   usecase.AICodeNotConfigured,
	"AI_COMPLETE_SESSION_EXPIRED":   usecase.AICodeSessionExpired,
	"AI_COMPLETE_QUOTA_EXHAUSTED":   usecase.AICodeQuotaExhausted,
	"AI_COMPLETE_BUSY":              usecase.AICodeBusy,
	"AI_COMPLETE_UNSUPPORTED":       usecase.AICodeAgentUnavailable,
	"AI_COMPLETE_UNKNOWN_MODEL":     usecase.AICodeNotConfigured,
}

// mapAIRelayError turns an infra-fleet error from an ai.complete relay into an AIUnavailableError when it names a
// cause the user can fix; anything else is returned unchanged so existing timeout handling still applies.
func mapAIRelayError(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	code, detail, found := strings.Cut(st.Message(), ":")
	if !found {
		return err
	}
	if mapped, known := aiCompleteCodes[strings.TrimSpace(code)]; known {
		return &usecase.AIUnavailableError{Code: mapped, Detail: strings.TrimSpace(detail), Cause: err}
	}
	return err
}

// checkAIReady asks the dev server what it can do before spending a call (and up to a minute of timeout) on it.
// It never blocks on missing information: an unknown or unreadable profile lets the call proceed, because the
// agent's own answer is authoritative. A profile that says "not configured" is re-read once fresh first, since the
// cached profile may predate the user logging in.
func checkAIReady(ctx context.Context, caps usecase.DevServerCapabilityReader, conn AIConnection) error {
	if caps == nil {
		return nil
	}
	ref := usecase.DevServerRef{ConnectionID: conn.ConnectionID, DevServerID: conn.DevServerID}
	profile, err := caps.Get(ctx, ref, false)
	if err != nil {
		return nil
	}
	if profile.AIReadiness() == aiNotConfigured {
		if fresh, err := caps.Get(ctx, ref, true); err == nil {
			profile = fresh
		}
	}
	switch {
	case profile.Source != "" && !profile.Connected:
		return &usecase.AIUnavailableError{Code: usecase.AICodeAgentUnavailable, Detail: "the dev server agent is not connected"}
	case profile.AIReadiness() == aiNotConfigured:
		return &usecase.AIUnavailableError{
			Code:   usecase.AICodeNotConfigured,
			Detail: "the dev server has no Claude Code login and no AI API key; run `claude auth login` on it or set ANTHROPIC_API_KEY for the agent",
		}
	}
	return nil
}
