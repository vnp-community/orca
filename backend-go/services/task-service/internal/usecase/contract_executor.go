package usecase

import "context"

type ContractExecutor struct {}

func (e *ContractExecutor) ExecuteContract(ctx context.Context, taskID string) error {
	return nil
}
