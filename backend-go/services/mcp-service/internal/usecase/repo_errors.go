package usecase

import (
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// wrapRepoErr keeps typed domain errors (MCP_* codes) and hides everything
// else behind MCP_INTERNAL so SQL details never reach a caller.
func wrapRepoErr(err error, msg string) error {
	var ae *apperrors.AppError
	if errors.As(err, &ae) {
		return err
	}
	return domain.ErrInternal(msg, err)
}
