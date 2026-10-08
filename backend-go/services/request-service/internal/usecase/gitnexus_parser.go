package usecase

import "context"

type GitNexusParser struct{}

func (p *GitNexusParser) ParseGraph(ctx context.Context, payload []byte) error {
	return nil
}
