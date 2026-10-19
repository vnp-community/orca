package postgres

import "github.com/google/uuid"

// notUUID reports an id that no uuid column can hold. Postgres rejects it at bind time (SQLSTATE 22P02,
// which also aborts an open transaction), so lookups and updates answer "not found" before querying.
func notUUID(id string) bool {
	_, err := uuid.Parse(id)
	return err != nil
}
