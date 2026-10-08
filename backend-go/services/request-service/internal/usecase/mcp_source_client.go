package usecase

import "context"

type MCPSourceClient struct{}

func (c *MCPSourceClient) QuerySource(ctx context.Context, query string) (string, error) {
	return "", nil
}
