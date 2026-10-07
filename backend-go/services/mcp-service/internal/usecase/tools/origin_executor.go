package tools

import "context"

type OriginExecutor struct {}

func (e *OriginExecutor) ExecuteWithOrigin(ctx context.Context, origin string) error {
	return nil
}
