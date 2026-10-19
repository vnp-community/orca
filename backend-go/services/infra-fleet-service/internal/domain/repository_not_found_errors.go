package domain

import "errors"

// Sentinels repositories return (wrapped with %w) when a by-id lookup or
// by-id mutation matches no row for the tenant; usecases map them to
// KindNotFound and every other repo error to KindInternal.
var (
	ErrDevServerNotFound              = errors.New("domain: dev server not found")
	ErrSshTargetNotFound              = errors.New("domain: ssh target not found")
	ErrConnectionNotFound             = errors.New("domain: connection not found")
	ErrTerminalSessionNotFound        = errors.New("domain: terminal session not found")
	ErrBrowserProfileNotFound         = errors.New("domain: browser profile not found")
	ErrDevServerGroupGrantNotFound    = errors.New("domain: dev server group grant not found")
	ErrDevServerAccessRequestNotFound = errors.New("domain: dev server access request not found")
	ErrAgentTokenNotFound             = errors.New("domain: agent token not found")
)
