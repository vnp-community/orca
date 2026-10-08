package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// RequestLocker takes the row lock that orders every approval writer behind the Request (lock order: Request, Approval).
type RequestLocker struct {
	*Repository
}

func NewRequestLocker(r *Repository) *RequestLocker { return &RequestLocker{Repository: r} }

var _ usecase.RequestLocker = (*RequestLocker)(nil)

func (l *RequestLocker) LockRequest(ctx context.Context, requestID string) (domain.Request, error) {
	if !l.inTx(ctx) {
		return domain.Request{}, fmt.Errorf("postgres: LockRequest outside a transaction")
	}
	if _, err := uuid.Parse(requestID); err != nil {
		return domain.Request{}, domain.ErrRequestNotFound(requestID)
	}
	var out domain.Request
	err := l.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		got, err := scanRequest(db.QueryRow(ctx, `SELECT `+requestColumns+` FROM request.requests WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenantID, requestID))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrRequestNotFound(requestID)
		}
		out = got
		return err
	})
	return out, err
}
