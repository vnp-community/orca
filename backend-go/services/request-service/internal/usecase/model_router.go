package usecase

import "context"

func RouteToModel(ctx context.Context, req interface{}) (string, error) {
	return "default-model", nil
}
