package usecase

import "context"

type RepoFS struct {}

func (fs *RepoFS) ListDir(ctx context.Context, path string) ([]string, error) {
	return nil, nil
}

func (fs *RepoFS) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return nil, nil
}
