package usecase

import "context"

type CreatePlanTree struct {}

func (uc *CreatePlanTree) Execute(ctx context.Context, in interface{}) error {
	return nil
}
