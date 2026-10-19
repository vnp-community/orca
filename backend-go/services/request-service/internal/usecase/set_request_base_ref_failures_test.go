package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func TestSetRequestBaseRef_UnknownAndMalformedRequest(t *testing.T) {
	f := newSetBaseRefFixture(t)
	for _, id := range []string{uuid.NewString(), "abc", ""} {
		_, err := f.uc.Execute(tctx(), SetRequestBaseRefInput{RequestID: id, BaseRef: "main", ActorID: "u", ActorKind: domain.ActorKindUser})
		mustCode(t, err, "REQUEST_NOT_FOUND")
	}
}

func TestSetRequestBaseRef_TenantRequired(t *testing.T) {
	f := newSetBaseRefFixture(t)
	r := f.seed(nil)
	_, err := f.uc.Execute(context.Background(), setIn(r, "main"))
	mustCode(t, err, "REQUEST_TENANT_REQUIRED")
}

func TestSetRequestBaseRef_PresenceFailureFailsClosed(t *testing.T) {
	f := newSetBaseRefFixture(t)
	r := f.seed(nil)
	f.presence.err = domain.ErrExecutionTaskServiceUnavailable(errors.New("down"))
	_, err := f.uc.Execute(tctx(), setIn(r, "main"))
	mustCode(t, err, "REQUEST_EXECUTION_TASK_SERVICE_UNAVAILABLE")
	requireNoWrite(t, f, r)
}

func TestSetRequestBaseRef_OutboxFailureRollsBackTheWrite(t *testing.T) {
	f := newSetBaseRefFixture(t)
	r := f.seed(nil)
	f.store.failEv = errors.New("outbox down")
	if _, err := f.uc.Execute(tctx(), setIn(r, "main")); err == nil {
		t.Fatal("expected the outbox failure")
	}
	if got, _ := f.store.read(r.ID); got.BaseRef != "" || got.Version != r.Version {
		t.Fatalf("the write must roll back with its event: %+v", got)
	}
}

func TestSetRequestBaseRef_CancelledWhileChecking(t *testing.T) {
	f := newSetBaseRefFixture(t)
	r := f.seed(nil)
	ctx, cancel := context.WithTimeout(tctx(), time.Millisecond)
	defer cancel()
	f.uc.d.Checker = checkerFunc(func(ctx context.Context) (BaseRefCheck, error) {
		<-ctx.Done()
		return BaseRefCheck{}, ctx.Err()
	})
	_, err := f.uc.Execute(ctx, setIn(r, "main"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a caller that gave up must get its own deadline back, not a stored value: %v", err)
	}
	requireNoWrite(t, f, r)
}

type checkerFunc func(ctx context.Context) (BaseRefCheck, error)

func TestNewSetRequestBaseRef_RefusesMissingPorts(t *testing.T) {
	s, p, c := newBaseRefStore(), newFakePresence(), &fakeRefChecker{}
	full := SetRequestBaseRefDeps{Requests: s, Locker: s, Writer: s, Presence: p, Checker: c, Tx: s, Outbox: s}
	if _, err := NewSetRequestBaseRef(full); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(d *SetRequestBaseRefDeps){
		"requests": func(d *SetRequestBaseRefDeps) { d.Requests = nil }, "locker": func(d *SetRequestBaseRefDeps) { d.Locker = nil },
		"writer": func(d *SetRequestBaseRefDeps) { d.Writer = nil }, "presence": func(d *SetRequestBaseRefDeps) { d.Presence = nil },
		"checker": func(d *SetRequestBaseRefDeps) { d.Checker = nil }, "tx": func(d *SetRequestBaseRefDeps) { d.Tx = nil },
		"outbox": func(d *SetRequestBaseRefDeps) { d.Outbox = nil },
	} {
		d := full
		mutate(&d)
		if uc, err := NewSetRequestBaseRef(d); err == nil || uc != nil {
			t.Errorf("a missing %s must fail construction, got %v", name, err)
		}
	}
}
