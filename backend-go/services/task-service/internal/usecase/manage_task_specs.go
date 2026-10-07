package usecase

import "context"

type ManageTaskSpecs struct {}

func (uc *ManageTaskSpecs) SetTaskSpec(ctx context.Context, taskID string, spec []byte) error {
	return nil
}

func (uc *ManageTaskSpecs) GetTaskSpecs(ctx context.Context, taskID string) ([]byte, error) {
	return nil, nil
}

func (uc *ManageTaskSpecs) LockTaskSpecs(ctx context.Context, taskID string) error {
	return nil
}
