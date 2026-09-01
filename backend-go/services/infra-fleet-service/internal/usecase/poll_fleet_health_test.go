package usecase

import (
	"context"
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
}

func (f *fakeFleetHealthWriter) UpsertFleetHealth(ctx context.Context, health domain.DevServerHealth) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.written = append(f.written, health)
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

	uc := NewPollFleetHealth(repo, writer, agent, slog.Default())
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

	uc := NewPollFleetHealth(repo, writer, agent, slog.Default())
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

	uc := NewPollFleetHealth(repo, writer, agent, slog.Default())
	if err := uc.Execute(context.Background()); err != nil {
		t.Fatalf("a per-dev-server write failure must not fail the whole poll: %v", err)
	}
}

func TestPollFleetHealth_ListFailurePropagates(t *testing.T) {
	repo := &fakePollerRepository{listErr: errors.New("db down")}
	writer := &fakeFleetHealthWriter{}
	agent := &fakeDevServerAgentClient{healthy: true}

	uc := NewPollFleetHealth(repo, writer, agent, slog.Default())
	if err := uc.Execute(context.Background()); err == nil {
		t.Fatal("expected the list failure to propagate")
	}
}
