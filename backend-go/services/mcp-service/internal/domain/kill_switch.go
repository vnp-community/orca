package domain

import "time"

const (
	KillScopeTenant  = "tenant"
	KillScopeClient  = "client"
	KillScopeGrant   = "grant"
	KillScopeSession = "session"

	MinKillReason = 3
	MaxKillReason = 500
)

// KillSwitchEntry is one row of mcp.kill_switches.
type KillSwitchEntry struct {
	ID, TenantID, Scope, TargetID, Reason, SetBy string
	Active                                       bool
	SetAt                                        time.Time
}

func ValidKillScope(s string) bool {
	return s == KillScopeTenant || s == KillScopeClient || s == KillScopeGrant || s == KillScopeSession
}

// Validate normalizes target for the tenant scope and enforces the reason bounds.
func (e *KillSwitchEntry) Validate() error {
	if !ValidKillScope(e.Scope) {
		return ErrInvalidArgument("scope must be tenant, client, grant or session")
	}
	n := len([]rune(e.Reason))
	if n < MinKillReason || n > MaxKillReason {
		return ErrInvalidArgument("reason must be 3 to 500 characters")
	}
	if e.Scope == KillScopeTenant {
		e.TargetID = ""
	} else if e.TargetID == "" {
		return ErrInvalidArgument("targetId is required for this scope")
	}
	return nil
}

// KillState is the set of active switches of one tenant.
type KillState struct{ Entries []KillSwitchEntry }

// Blocked reports whether a call with these identifiers is stopped, and by which switch.
func (k KillState) Blocked(clientID, grantID, sessionID string) (KillSwitchEntry, bool) {
	for _, e := range k.Entries {
		if !e.Active {
			continue
		}
		switch {
		case e.Scope == KillScopeTenant,
			e.Scope == KillScopeClient && clientID != "" && e.TargetID == clientID,
			e.Scope == KillScopeGrant && grantID != "" && e.TargetID == grantID,
			e.Scope == KillScopeSession && sessionID != "" && e.TargetID == sessionID:
			return e, true
		}
	}
	return KillSwitchEntry{}, false
}
