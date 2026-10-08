package usecase

import (
	"context"
	"sync"
)

type commitQueueKey struct{}

type commitQueue struct {
	mu  sync.Mutex
	fns []func()
}

func (q *commitQueue) add(fn func()) {
	q.mu.Lock()
	q.fns = append(q.fns, fn)
	q.mu.Unlock()
}

func (q *commitQueue) run() {
	q.mu.Lock()
	fns := q.fns
	q.fns = nil
	q.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// AfterCommit runs fn once the outermost CommitHooks transaction around ctx has committed, and drops it if that
// transaction rolls back. Outside such a transaction fn runs at once. Audit and metrics use it so they never
// describe a decision that was rolled back.
func AfterCommit(ctx context.Context, fn func()) {
	if q, ok := ctx.Value(commitQueueKey{}).(*commitQueue); ok {
		q.add(fn)
		return
	}
	fn()
}

// CommitHooks decorates a TxScope so AfterCommit callbacks queued inside it fire after the outermost commit.
// A nested InTx joins the outer queue.
type CommitHooks struct{ Inner TxScope }

var _ TxScope = CommitHooks{}

func (h CommitHooks) InTx(ctx context.Context, fn func(context.Context) error) error {
	if _, nested := ctx.Value(commitQueueKey{}).(*commitQueue); nested {
		return h.Inner.InTx(ctx, fn)
	}
	q := &commitQueue{}
	if err := h.Inner.InTx(context.WithValue(ctx, commitQueueKey{}, q), fn); err != nil {
		return err
	}
	q.run()
	return nil
}

func (h CommitHooks) InTransaction(ctx context.Context) bool { return h.Inner.InTransaction(ctx) }
