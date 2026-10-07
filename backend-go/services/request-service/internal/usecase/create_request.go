package usecase

import "context"

type CreateRequestInput struct {
	Title       string
	Description string
}

type CreateRequest struct {}

func (uc *CreateRequest) Execute(ctx context.Context, in CreateRequestInput) (string, error) {
	return "", nil
}
