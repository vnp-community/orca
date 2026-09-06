package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

type fakePollerRepository struct {
	devServers []domain.DevServer
	listErr    error
}

func (f *fakePollerRepository) ListAllDevServers(ctx context.Context) ([]domain.DevServer, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.devServers, nil
}

type fakeFleetHealthWriter struct {
	written   []domain.DevServerHealth
	upsertErr error
	// previous seeds GetDevServerHealth's return per dev server — set
	// before Execute to simulate "this dev server was already reachable
	// last poll."
	previous map[string]domain.DevServerHealth
	getErr   error
}

func (f *fakeFleetHealthWriter) UpsertFleetHealth(ctx context.Context, health domain.DevServerHealth) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.written = append(f.written, health)
	return nil
}

func (f *fakeFleetHealthWriter) GetDevServerHealth(ctx context.Context, devServerID string) (domain.DevServerHealth, bool, error) {
	if f.getErr != nil {
		return domain.DevServerHealth{}, false, f.getErr
	}
	h, ok := f.previous[devServerID]
	return h, ok, nil
}

type fakeOutboxWriter struct {
	inserted  []domain.OutboxEvent
	insertErr error
}

func (f *fakeOutboxWriter) InsertOutboxEvent(ctx context.Context, event domain.OutboxEvent) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserted = append(f.inserted, event)
	return nil
}

func devServerForPollTest(t *testing.T, id string) domain.DevServer {
	t.Helper()
	ds, err := domain.NewDevServer(id, "tenant-1", "10.0.0.1", domain.ConnectionModeDirectWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	return ds
}

// TestPollFleetHealth_WritesReachableSampleForEachDevServer is the live-bug
// regression: infra.fleet_health had zero rows for every dev server,
// forever, because nothing ever wrote to it — DevServerReachability.IsReachable
// (GetFleetHealth's real caller) always fell through to "no sample yet,
// treat as not reachable" even for a genuinely connected agent.
func TestPollFleetHealth_WritesReachableSampleForEachDevServer(t *testing.T) {
	ds1 := devServerForPollTest(t, "ds-1")
	ds2 := devServerForPollTest(t, "ds-2")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds1, ds2}}
	writer := &fakeFleetHealthWriter{}
	agent := &fakeDevServerAgentClient{healthy: true}

	uc := NewPollFleetHealth(repo, writer, nil, agent, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(writer.written) != 2 {
		t.Fatalf("want 2 samples written, got %d", len(writer.written))
	}
	for _, sample := range writer.written {
		if !sample.Reachable {
			t.Errorf("want Reachable=true for %s, got false", sample.DevServerID)
		}
	}
}

func TestPollFleetHealth_RecordsUnreachableWhenHealthCheckFails(t *testing.T) {
	ds := devServerForPollTest(t, "ds-1")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	writer := &fakeFleetHealthWriter{}
	agent := &fakeDevServerAgentClient{healthErr: errors.New("dial failed")}

	uc := NewPollFleetHealth(repo, writer, nil, agent, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(writer.written) != 1 {
		t.Fatalf("want 1 sample written, got %d", len(writer.written))
	}
	if writer.written[0].Reachable {
		t.Error("want Reachable=false when the health check errors")
	}
}

func TestPollFleetHealth_OneDevServerWriteFailureDoesNotStopTheRest(t *testing.T) {
	ds1 := devServerForPollTest(t, "ds-1")
	ds2 := devServerForPollTest(t, "ds-2")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds1, ds2}}
	writer := &fakeFleetHealthWriter{upsertErr: errors.New("db down")}
	agent := &fakeDevServerAgentClient{healthy: true}

	uc := NewPollFleetHealth(repo, writer, nil, agent, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("a per-dev-server write failure must not fail the whole poll: %v", err)
	}
}

func TestPollFleetHealth_ListFailurePropagates(t *testing.T) {
	repo := &fakePollerRepository{listErr: errors.New("db down")}
	writer := &fakeFleetHealthWriter{}
	agent := &fakeDevServerAgentClient{healthy: true}

	uc := NewPollFleetHealth(repo, writer, nil, agent, slog.Default())
	if err := uc.Execute(context.Background()); err == nil {
		t.Fatal("expected the list failure to propagate")
	}
}

// TestPollFleetHealth_ReachableToUnreachableTransition_EnqueuesOneAlert is
// the admin-alerting regression: a dev server that WAS reachable and just
// went unreachable must enqueue exactly one outbox event (see
// alertDevServerDisconnected's doc comment on why not-repeated).
func TestPollFleetHealth_ReachableToUnreachableTransition_EnqueuesOneAlert(t *testing.T) {
	ds := devServerForPollTest(t, "ds-1")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	writer := &fakeFleetHealthWriter{
		previous: map[string]domain.DevServerHealth{"ds-1": {DevServerID: "ds-1", Reachable: true}},
	}
	outboxW := &fakeOutboxWriter{}
	agent := &fakeDevServerAgentClient{healthErr: errors.New("dial failed")}

	uc := NewPollFleetHealth(repo, writer, outboxW, agent, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(outboxW.inserted) != 1 {
		t.Fatalf("want 1 outbox event enqueued, got %d", len(outboxW.inserted))
	}
	event := outboxW.inserted[0]
	if event.Subject != domain.DevServerDisconnectedSubject {
		t.Errorf("want subject %q, got %q", domain.DevServerDisconnectedSubject, event.Subject)
	}
	if event.TenantID != ds.TenantID {
		t.Errorf("want tenantId %q, got %q", ds.TenantID, event.TenantID)
	}
	var payload domain.DevServerDisconnectedPayload
	if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
		t.Fatalf("unmarshaling payload: %v", err)
	}
	if payload.DevServerID != "ds-1" || payload.Host != ds.Host {
		t.Errorf("unexpected payload: %+v", payload)
	}
}

// TestPollFleetHealth_RepeatedUnreachableSamples_DoNotReAlert proves the
// edge-triggered rule: a dev server already recorded unreachable last poll
// must not enqueue another alert just for staying unreachable.
func TestPollFleetHealth_RepeatedUnreachableSamples_DoNotReAlert(t *testing.T) {
	ds := devServerForPollTest(t, "ds-1")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	writer := &fakeFleetHealthWriter{
		previous: map[string]domain.DevServerHealth{"ds-1": {DevServerID: "ds-1", Reachable: false}},
	}
	outboxW := &fakeOutboxWriter{}
	agent := &fakeDevServerAgentClient{healthErr: errors.New("still down")}

	uc := NewPollFleetHealth(repo, writer, outboxW, agent, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(outboxW.inserted) != 0 {
		t.Fatalf("want no re-alert on a repeated unreachable sample, got %d", len(outboxW.inserted))
	}
}

// TestPollFleetHealth_UnreachableToReachable_DoesNotAlert proves a recovery
// edge (false -> true) never fires the disconnect alert.
func TestPollFleetHealth_UnreachableToReachable_DoesNotAlert(t *testing.T) {
	ds := devServerForPollTest(t, "ds-1")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	writer := &fakeFleetHealthWriter{
		previous: map[string]domain.DevServerHealth{"ds-1": {DevServerID: "ds-1", Reachable: false}},
	}
	outboxW := &fakeOutboxWriter{}
	agent := &fakeDevServerAgentClient{healthy: true}

	uc := NewPollFleetHealth(repo, writer, outboxW, agent, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(outboxW.inserted) != 0 {
		t.Fatalf("want no alert on a recovery edge, got %d", len(outboxW.inserted))
	}
}

// TestPollFleetHealth_FirstEverPoll_NoPreviousSample_DoesNotAlert proves a
// dev server's very first poll (no row exists yet, found=false) never
// alerts even if it comes back unreachable — there is no "was reachable"
// edge to have transitioned from.
func TestPollFleetHealth_FirstEverPoll_NoPreviousSample_DoesNotAlert(t *testing.T) {
	ds := devServerForPollTest(t, "ds-1")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	writer := &fakeFleetHealthWriter{} // previous is nil — GetDevServerHealth returns found=false
	outboxW := &fakeOutboxWriter{}
	agent := &fakeDevServerAgentClient{healthErr: errors.New("dial failed")}

	uc := NewPollFleetHealth(repo, writer, outboxW, agent, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(outboxW.inserted) != 0 {
		t.Fatalf("want no alert on a dev server's first-ever poll, got %d", len(outboxW.inserted))
	}
}

// TestPollFleetHealth_NilOutboxWriter_StillWritesSampleWithoutPanicking
// proves outbox is genuinely optional (main.go leaves it nil when NATS is
// unreachable at startup) — the WARN log still fires, just no enqueue.
func TestPollFleetHealth_NilOutboxWriter_StillWritesSampleWithoutPanicking(t *testing.T) {
	ds := devServerForPollTest(t, "ds-1")
	repo := &fakePollerRepository{devServers: []domain.DevServer{ds}}
	writer := &fakeFleetHealthWriter{
		previous: map[string]domain.DevServerHealth{"ds-1": {DevServerID: "ds-1", Reachable: true}},
	}
	agent := &fakeDevServerAgentClient{healthErr: errors.New("dial failed")}

	uc := NewPollFleetHealth(repo, writer, nil, agent, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(writer.written) != 1 {
		t.Fatalf("want the sample still written despite outbox being nil, got %d", len(writer.written))
	}
}
