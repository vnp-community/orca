package usecase

import "context"

type OrcaDataSourceAdapter struct{}

func (a *OrcaDataSourceAdapter) FetchContext(ctx context.Context, query string) (string, error) {
	return "", nil
}
