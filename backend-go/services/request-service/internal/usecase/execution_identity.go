package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
)

// asExecutionActor puts the execution actor on ctx. Consumers and the reconcile loop run with a tenant only, but
// task-service filters GetSubtree by user (owner or grant), so any call that reads or writes the Plan tree on
// their behalf must carry the same actor the dispatch uses, or it sees an empty tree.
func asExecutionActor(ctx context.Context, actor string) context.Context {
	if actor == "" {
		return ctx
	}
	return tenant.WithUserID(ctx, actor)
}
