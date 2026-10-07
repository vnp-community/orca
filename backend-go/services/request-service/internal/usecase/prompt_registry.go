package usecase

import "context"

type PromptRegistry struct {}

func (r *PromptRegistry) GetPrompt(ctx context.Context, key string) (string, error) {
	return "", nil
}
