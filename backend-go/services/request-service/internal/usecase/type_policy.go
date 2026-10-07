package usecase

import "context"

type TypePolicy interface {
	PreDeployCheck(ctx context.Context) error
}

type TypePolicyRegistry struct {}
