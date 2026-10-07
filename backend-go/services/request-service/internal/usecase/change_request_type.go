package usecase

import "context"

type ChangeRequestType struct {}

func (uc *ChangeRequestType) Execute(ctx context.Context, reqID, newType string) error {
	return nil
}
