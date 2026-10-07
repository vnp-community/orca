package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

var ErrCapabilityNotAvailable = errors.New("capability not available")

type DevServerRef struct {
	ConnectionID string
	DevServerID  string
}

type DevServerCapabilityReader interface {
	Get(ctx context.Context, ref DevServerRef, refresh bool) (domain.DevServerCapability, error)
}
