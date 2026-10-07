package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func EvaluateRiskGate(ctx context.Context, score domain.RiskScore) bool {
	return score.Score < 80
}
