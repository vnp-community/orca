package usecase

import "context"

type TransitionRequest struct {}

func (uc *TransitionRequest) Execute(ctx context.Context, reqID, event string) error {
	return nil
}
