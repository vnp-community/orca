package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// SourceLookup serves LookupRequestBySource over request.requests.
type SourceLookup struct{ *Repository }

func NewSourceLookup(r *Repository) *SourceLookup { return &SourceLookup{Repository: r} }

var _ usecase.ActiveSourceFinder = (*SourceLookup)(nil)

func (r *SourceLookup) FindActiveBySource(ctx context.Context, provider, site, ref string) (string, bool, error) {
	var id string
	found := false
	err := r.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		err := db.QueryRow(ctx, `SELECT id::text FROM request.requests
			WHERE tenant_id = $1 AND source_provider = $2 AND source_ref = $3 AND ($4 = '' OR source_site = $4)
			  AND status NOT IN ('completed', 'cancelled')
			ORDER BY created_at DESC, id DESC LIMIT 1`, tenantID, provider, ref, site).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		found = err == nil
		return err
	})
	return id, found, err
}
