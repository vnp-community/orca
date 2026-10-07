package usecase

import "context"

type ReturnToBacklog struct {}

func (uc *ReturnToBacklog) Execute(ctx context.Context, reqID, reason string) error {
	return nil
}
