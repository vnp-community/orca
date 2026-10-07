package usecase

import "context"

type ProposeRequestClassification struct {}

func (uc *ProposeRequestClassification) Execute(ctx context.Context, reqID string) error {
	return nil
}
