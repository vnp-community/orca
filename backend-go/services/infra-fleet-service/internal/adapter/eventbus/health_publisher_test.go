package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type enqueueCall struct {
	id, tenantID, subject string
	occurredAt            time.Time
	version               int
	payload               []byte
}

type fakeOutboxEnqueuer struct {
	calls []enqueueCall
	err   error
}

func (f *fakeOutboxEnqueuer) EnqueueOutboxEvent(ctx context.Context, id, tenantID, subject string, occurredAt time.Time, version int, payload []byte) error {
	f.calls = append(f.calls, enqueueCall{id: id, tenantID: tenantID, subject: subject, occurredAt: occurredAt, version: version, payload: payload})
	return f.err
}

func TestHealthPublisher_PublishStatusChange_EnqueuesExactlyOneRow(t *testing.T) {
	store := &fakeOutboxEnqueuer{}
	p := NewHealthPublisher(store, nil)

	ds := domain.DevServer{ID: "ds1", TenantID: "t1", Host: "10.0.0.1"}
	p.PublishStatusChange(context.Background(), ds, domain.HealthStatusHealthy, domain.HealthStatusDegraded)

	if len(store.calls) != 1 {
		t.Fatalf("expected exactly 1 enqueue call, got %d", len(store.calls))
	}
	call := store.calls[0]
	if call.subject != HealthDegradedSubject {
		t.Errorf("expected subject %q, got %q", HealthDegradedSubject, call.subject)
	}
	if call.tenantID != "t1" {
		t.Errorf("expected tenantID t1, got %q", call.tenantID)
	}
	if call.id == "" {
		t.Error("expected a non-empty generated event id")
	}
	if call.version != 1 {
		t.Errorf("expected version=1, got %d", call.version)
	}

	var payload map[string]any
	if err := json.Unmarshal(call.payload, &payload); err != nil {
		t.Fatalf("unmarshaling payload: %v", err)
	}
	if payload["devServerId"] != "ds1" || payload["host"] != "10.0.0.1" || payload["tenantId"] != "t1" {
		t.Errorf("unexpected payload identity fields: %+v", payload)
	}
	if payload["from"] != string(domain.HealthStatusHealthy) || payload["to"] != string(domain.HealthStatusDegraded) {
		t.Errorf("unexpected payload from/to: %+v", payload)
	}
	if _, ok := payload["timestamp"].(string); !ok {
		t.Errorf("expected a string timestamp field, got %+v", payload["timestamp"])
	}
}

func TestHealthPublisher_PublishStatusChange_EnqueueFailureIsLoggedNotPanicked(t *testing.T) {
	store := &fakeOutboxEnqueuer{err: errors.New("db unavailable")}
	p := NewHealthPublisher(store, nil)

	ds := domain.DevServer{ID: "ds1", TenantID: "t1", Host: "10.0.0.1"}
	// Must not panic even though the underlying enqueue fails.
	p.PublishStatusChange(context.Background(), ds, domain.HealthStatusHealthy, domain.HealthStatusUnhealthy)

	if len(store.calls) != 1 {
		t.Fatalf("expected the enqueue to still be attempted once, got %d calls", len(store.calls))
	}
}
