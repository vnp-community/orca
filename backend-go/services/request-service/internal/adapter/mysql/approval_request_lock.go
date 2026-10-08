package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
		return domain.Request{}, fmt.Errorf("mysql: LockRequest outside a transaction")
	}
	var out domain.Request
	err := l.scoped(ctx, func(ctx context.Context, tenantID string, db dbExecer) error {
		got, err := scanRequest(db.QueryRowContext(ctx, `SELECT `+requestColumns+` FROM requests WHERE tenant_id = ? AND id = ? FOR UPDATE`, tenantID, requestID))
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrRequestNotFound(requestID)
		}
		out = got
		return err
	})
	return out, err
}
