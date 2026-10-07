package usecase

import "context"

type ManageClarifications struct {}

func (uc *ManageClarifications) CreateClarification(ctx context.Context, reqID, question string) error {
	return nil
}

func (uc *ManageClarifications) AnswerClarification(ctx context.Context, id, answer string) error {
	return nil
}

func (uc *ManageClarifications) CancelClarification(ctx context.Context, id string) error {
	return nil
}
