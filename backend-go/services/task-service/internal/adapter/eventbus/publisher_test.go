package eventbus

import (
	"context"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type recordingOutbox struct{ events []domain.OutboxEvent }

func (r *recordingOutbox) WriteOutboxEvent(_ context.Context, _ string, e domain.OutboxEvent) error {
	r.events = append(r.events, e)
	return nil
}

// The TASK stream covers `orca.task.>` only; a grant event queued as `task.grant_received` was refused on every poll and held back the
// whole outbox, so a member's first approve-and-run-all stalled every run behind it.
func TestPublisher_PutsEveryEventUnderTheTaskStream(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"task.grant_received", "orca.task.grant_received"},
		{"task.grant_revoked", "orca.task.grant_revoked"},
		{"orca.task.task.statuschanged", "orca.task.task.statuschanged"},
	} {
		w := &recordingOutbox{}
		NewPublisher(w, nil).Publish(context.Background(), "tenant-1", c.in, map[string]any{"task_id": "t1"})
		if len(w.events) != 1 || w.events[0].Subject != c.want {
			t.Fatalf("%s: queued %+v, want subject %s", c.in, w.events, c.want)
		}
		if !strings.HasPrefix(w.events[0].Subject, "orca.task.") {
			t.Fatalf("%s is outside the TASK stream", w.events[0].Subject)
		}
	}
}
