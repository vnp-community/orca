package usecase

import "context"

type GenerateCommitMessageInput struct {
	WorktreeID string
}

// GenerateCommitMessage relays diff/status context (itself fetched via the
// same resolve-and-dispatch path as GetDiff) to the Dev Server Agent's
// ai.complete, per git-gateway-service.md §3.1 — this service never calls
// an LLM API directly. That relay is not implemented in this scaffold; this
// usecase always returns ErrGenerateCommitMessageNotImplemented so the
// gRPC adapter can map it to a clear Unimplemented status rather than
// silently returning an empty message.
type GenerateCommitMessage struct{}

func NewGenerateCommitMessage() *GenerateCommitMessage {
	return &GenerateCommitMessage{}
}

func (uc *GenerateCommitMessage) Execute(_ context.Context, _ GenerateCommitMessageInput) (string, error) {
	return "", ErrGenerateCommitMessageNotImplemented
}
