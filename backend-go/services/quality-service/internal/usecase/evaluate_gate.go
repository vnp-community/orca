package usecase

import "context"

func EvaluateQualityGate(ctx context.Context, repoID string) (bool, error) {
	return true, nil
}
