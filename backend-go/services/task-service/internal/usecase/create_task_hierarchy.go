package usecase

import "context"

type CreateTaskHierarchy struct {
	// ...
}

func (uc *CreateTaskHierarchy) Execute(ctx context.Context, in interface{}) error {
	return nil
}
