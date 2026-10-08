package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// SpawnFollowUps creates the child Requests a type policy asked for through SpawnChildRequest. Each one is
// keyed by its ClientRequestID, so running this twice leaves exactly one child per follow-up.
type SpawnFollowUps struct {
	Spawn *SpawnChildRequest
	Log   *slog.Logger
}

var _ FollowUpExecutor = (*SpawnFollowUps)(nil)

// ExecuteFollowUps returns an error only for transient failures (worth retrying). A follow-up the parent can
// never have (wrong state, limits) is logged and dropped: it must not undo or block the parent's completion.
func (uc *SpawnFollowUps) ExecuteFollowUps(ctx context.Context, req domain.Request, fus []domain.FollowUp) error {
	log := uc.Log
	if log == nil {
		log = slog.Default()
	}
	var transient []error
	for _, fu := range fus {
		// The reporter column is a user id, and a system actor has none, so the follow-up is filed as the parent's reporter.
		_, err := uc.Spawn.Execute(ctx, SpawnInput{
			ParentRequestID: req.ID, LinkReason: fu.LinkReason, Title: fu.Title, Body: fu.Body, TypeHint: fu.TypeHint,
			ClientRequestID: fu.ClientRequestID, ActorID: req.ReporterID, Provider: domain.SourceProviderManual,
		})
		switch {
		case err == nil:
		case isPermanentFollowUpError(err):
			log.Warn("follow-up request dropped", slog.String("request_id", req.ID), slog.String("client_request_id", fu.ClientRequestID), slog.String("error", truncateForLog(err.Error())))
		default:
			transient = append(transient, fmt.Errorf("follow-up %s: %w", fu.ClientRequestID, err))
		}
	}
	return errors.Join(transient...)
}

func isPermanentFollowUpError(err error) bool {
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		return false
	}
	switch ae.Kind {
	case apperrors.KindInvalidArgument, apperrors.KindFailedPrecondition, apperrors.KindNotFound, apperrors.KindPermissionDenied:
		return true
	}
	return false
}
