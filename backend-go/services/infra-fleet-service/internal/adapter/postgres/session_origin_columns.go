package postgres

import "github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"

// originColumn returns the value for one nullable origin_* column: nil (SQL
// NULL) for UI-created sessions or an empty field, so an unset origin never
// stores empty strings (and origin_user_id stays a valid uuid or NULL).
func originColumn(o *domain.SessionOrigin, field func(*domain.SessionOrigin) string) any {
	if o.IsEmpty() {
		return nil
	}
	if v := field(o); v != "" {
		return v
	}
	return nil
}

// buildSessionOrigin maps the scanned nullable origin_* columns back to the
// domain value; nil when the row was not created by an MCP client.
func buildSessionOrigin(typ, client, session, user *string) *domain.SessionOrigin {
	o := &domain.SessionOrigin{}
	if typ != nil {
		o.Type = *typ
	}
	if client != nil {
		o.ClientName = *client
	}
	if session != nil {
		o.MCPSessionID = *session
	}
	if user != nil {
		o.UserID = *user
	}
	if o.IsEmpty() {
		return nil
	}
	return o
}
