package usecase

import "context"

type CachedViewReader struct {}

func (r *CachedViewReader) ReadView(ctx context.Context, key string) ([]byte, error) {
	return nil, nil
}
