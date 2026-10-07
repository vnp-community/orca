package usecase

import "context"

func IngestCoverage(ctx context.Context, report domain.CoverageReport) error {
	return nil
}

func EstimateCoverage(ctx context.Context, repoID string) (float64, error) {
	return 0.0, nil
}
