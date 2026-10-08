package mysql

import (
	"context"
	"database/sql"
	"errors"

	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// SourceLookup serves LookupRequestBySource over requests.
type SourceLookup struct{ *Repository }

func NewSourceLookup(r *Repository) *SourceLookup { return &SourceLookup{Repository: r} }

var _ usecase.ActiveSourceFinder = (*SourceLookup)(nil)

func (r *SourceLookup) FindActiveBySource(ctx context.Context, provider, site, ref string) (string, bool, error) {
	var id string
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		err := db.QueryRowContext(ctx, `SELECT id FROM requests
			WHERE tenant_id = ? AND source_provider = ? AND source_ref = ? AND (? = '' OR source_site = ?)
			  AND status NOT IN ('completed', 'cancelled')
			ORDER BY created_at DESC, id DESC LIMIT 1`, tenantID, provider, ref, site, site).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		found = err == nil
		return err
	})
	return id, found, err
}
