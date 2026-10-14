package domain

import "github.com/stablyai/orca-go/common/apperrors"

var (
	ErrEnginePreflightFailed        = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ENGINE_PREFLIGHT_FAILED", "engine preflight failed", nil)
	ErrEngineNoConnection           = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ENGINE_NO_CONNECTION", "no connection", nil)
	ErrEngineOpenSpecMissing        = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ENGINE_OPENSPEC_MISSING", "openspec missing", nil)
	ErrEngineOpenSpecVersion        = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ENGINE_OPENSPEC_VERSION", "openspec version mismatch", nil)
	ErrEngineOpenSpecNotInitialized = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ENGINE_OPENSPEC_NOT_INITIALIZED", "openspec not initialized", nil)
	ErrEngineClaudeMissing          = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ENGINE_CLAUDE_MISSING", "claude missing", nil)
	ErrEngineClaudeNotAuthenticated = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ENGINE_CLAUDE_NOT_AUTHENTICATED", "claude not authenticated", nil)

	ErrEngineUnavailable = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ENGINE_UNAVAILABLE", "this server has no worker for the project's engine; ask an administrator to finish its configuration", nil)

	ErrEngineOverrideNotAllowed = apperrors.New(apperrors.KindPermissionDenied, "REQUEST_ENGINE_OVERRIDE_NOT_ALLOWED", "override not allowed", nil)

	ErrEngineRepoNotResolved         = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ENGINE_REPO_NOT_RESOLVED", "repo not resolved", nil)
	ErrEngineSettingsVersionConflict = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_ENGINE_SETTINGS_VERSION_CONFLICT", "settings version conflict", nil)

	ErrOpenSpecChangeNotFound        = apperrors.New(apperrors.KindNotFound, "REQUEST_OPENSPEC_CHANGE_NOT_FOUND", "openspec change not found", nil)
	ErrOpenSpecChangeVersionConflict = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_OPENSPEC_CHANGE_VERSION_CONFLICT", "openspec change version conflict", nil)

	ErrOpenSpecInvalidOutput    = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_OPENSPEC_INVALID_OUTPUT", "invalid output", nil)
	ErrOpenSpecAgentFailed      = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_OPENSPEC_AGENT_FAILED", "agent failed", nil)
	ErrOpenSpecTimeout          = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_OPENSPEC_TIMEOUT", "timeout", nil)
	ErrOpenSpecOutOfScopeChange = apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_OPENSPEC_OUT_OF_SCOPE_CHANGE", "out of scope change", nil)
)

// ErrEngineSettingsInvalid says which setting was refused; the message is safe to show to the administrator.
func ErrEngineSettingsInvalid(msg string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_ENGINE_SETTINGS_INVALID", msg, nil)
}
