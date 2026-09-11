package mysql

import "github.com/google/uuid"

// newUUID generates a fresh row id for the tables this adapter INSERTs into
// without an application-supplied id and without a usable server-side
// default (task_grants.id, task_comments.id, task_share_links.id) — MySQL
// has no RETURNING to read back a DEFAULT (UUID()) value the way
// internal/adapter/postgres's INSERT ... RETURNING id does, so this adapter
// generates the id itself and inserts it explicitly instead.
func newUUID() string {
	return uuid.NewString()
}
