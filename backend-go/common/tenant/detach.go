package tenant

import "context"

// Detach returns a context that keeps every value of ctx (tenant, user, role, actor type, trace) but is no longer
// cancelled when ctx is. Use it for work that must outlive the RPC that started it: the RPC's context is cancelled
// the moment the handler returns, and rebuilding identity by hand on context.Background() drops whatever the
// rebuilder forgets (the workflow engine lost the acting user that way).
func Detach(ctx context.Context) context.Context {
	return context.WithoutCancel(ctx)
}
