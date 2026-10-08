package resources

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

type actorRecordingDispatcher struct {
	inner  *fakeDispatcher
	mu     sync.Mutex
	actors []string
}

func (a *actorRecordingDispatcher) Dispatch(ctx context.Context, id wscompat.Identity, channel string, args []json.RawMessage) (any, error) {
	a.mu.Lock()
	a.actors = append(a.actors, tenant.ActorType(ctx))
	a.mu.Unlock()
	return a.inner.Dispatch(ctx, id, channel, args)
}

func TestRead_MarksActorAgent(t *testing.T) {
	d := &actorRecordingDispatcher{inner: scripted()}
	p := NewProvider(d, nil, nil, Config{}, quietLog())
	if _, _, err := read(t, p, "orca://project/"+idA); err != nil {
		t.Fatal(err)
	}
	if len(d.actors) == 0 {
		t.Fatal("resource read made no dispatch")
	}
	for _, a := range d.actors {
		if a != tenant.ActorAgent {
			t.Fatalf("resource dispatch actor = %q, want agent", a)
		}
	}
}
