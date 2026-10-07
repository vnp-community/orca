package usecase

import "context"

type OpenSpecArchiveConsumer struct {
	// dependencies
}

func NewOpenSpecArchiveConsumer() *OpenSpecArchiveConsumer {
	return &OpenSpecArchiveConsumer{}
}

func (c *OpenSpecArchiveConsumer) ConsumeRequestCompleted(ctx context.Context, requestID string) error {
	// Exec openspec archive
	return nil
}
