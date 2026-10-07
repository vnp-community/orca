package usecase

import "context"

type RepoBackedSourceAdapter struct {}

func (a *RepoBackedSourceAdapter) FetchContext(ctx context.Context, query string) (string, error) {
	return "", nil
}
