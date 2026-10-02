package mysql

import (
	"database/sql"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// originColumn returns the value for one nullable origin_* column: nil (SQL
// NULL) for UI-created sessions or an empty field.
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
func buildSessionOrigin(typ, client, session, user sql.NullString) *domain.SessionOrigin {
	o := &domain.SessionOrigin{Type: typ.String, ClientName: client.String, MCPSessionID: session.String, UserID: user.String}
	if o.IsEmpty() {
		return nil
	}
	return o
}
