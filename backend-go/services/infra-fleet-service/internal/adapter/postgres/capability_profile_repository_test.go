//go:build integration

package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestCapabilityProfileRepository_UpsertGet(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	store := NewCapabilityProfileStore(repo.pool)

	tenantID := uuid.NewString()
	devServerID := uuid.NewString()
	_, err := repo.pool.Exec(ctx, "INSERT INTO infra.dev_servers (id, tenant_id, host, connection_mode) VALUES ($1, $2, 'localhost', 'direct-websocket')", devServerID, tenantID)
	if err != nil {
		t.Fatalf("insert dev_server: %v", err)
	}

	p := domain.CapabilityProfile{
		DevServerID:       devServerID,
		TenantID:          tenantID,
		Source:            domain.ProfileSourceProbe,
		AgentBuildVersion: "1.0.0",
		ProtocolVersion:   2,
		Features:          []string{"feature1", "feature2"},
		ProfileJSON:       []byte(`{"a":1}`),
		Fingerprint:       "fingerprint1",
		ProbedAt:          time.Now().Truncate(time.Microsecond),
	}

	prev, existed, err := store.Upsert(ctx, p)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if existed {
		t.Errorf("expected existed=false")
	}
	if prev != "" {
		t.Errorf("expected empty prev")
	}

	got, found, err := store.Get(ctx, tenantID, devServerID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatalf("expected found=true")
	}
	if got.AgentBuildVersion != "1.0.0" || got.ProtocolVersion != 2 || got.Fingerprint != "fingerprint1" {
		t.Errorf("unexpected profile: %+v", got)
	}
}

func TestCapabilityProfileRepository_UpsertReturnsPreviousFingerprint(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	store := NewCapabilityProfileStore(repo.pool)

	tenantID := uuid.NewString()
	devServerID := uuid.NewString()
	repo.pool.Exec(ctx, "INSERT INTO infra.dev_servers (id, tenant_id, host, connection_mode) VALUES ($1, $2, 'localhost', 'direct-websocket')", devServerID, tenantID)

	p := domain.CapabilityProfile{
		DevServerID:       devServerID,
		TenantID:          tenantID,
		Source:            domain.ProfileSourceHandshakeOnly,
		AgentBuildVersion: "1.0.0",
		ProtocolVersion:   1,
		Features:          []string{},
		ProfileJSON:       []byte(`{}`),
		Fingerprint:       "fingerprint-1",
		ProbedAt:          time.Now().Truncate(time.Microsecond),
	}

	_, _, err := store.Upsert(ctx, p)
	if err != nil {
		t.Fatalf("Upsert 1: %v", err)
	}

	p.Fingerprint = "fingerprint-2"
	prev, existed, err := store.Upsert(ctx, p)
	if err != nil {
		t.Fatalf("Upsert 2: %v", err)
	}
	if !existed {
		t.Errorf("expected existed=true")
	}
	if prev != "fingerprint-1" {
		t.Errorf("expected prev=fingerprint-1, got %q", prev)
	}
}

func TestCapabilityProfileRepository_TenantIsolation(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	store := NewCapabilityProfileStore(repo.pool)

	tenantA := uuid.NewString()
	tenantB := uuid.NewString()
	devServerID := uuid.NewString()
	repo.pool.Exec(ctx, "INSERT INTO infra.dev_servers (id, tenant_id, host, connection_mode) VALUES ($1, $2, 'localhost', 'direct-websocket')", devServerID, tenantA)

	p := domain.CapabilityProfile{
		DevServerID:       devServerID,
		TenantID:          tenantA,
		Source:            domain.ProfileSourceProbe,
		AgentBuildVersion: "1.0.0",
		ProtocolVersion:   1,
		Features:          []string{},
		ProfileJSON:       []byte(`{}`),
		Fingerprint:       "fingerprint",
		ProbedAt:          time.Now().Truncate(time.Microsecond),
	}

	_, _, err := store.Upsert(ctx, p)
	if err != nil {
		t.Fatalf("Upsert A: %v", err)
	}

	// tenantB trying to upsert to A's dev_server
	pB := p
	pB.TenantID = tenantB
	_, _, err = store.Upsert(ctx, pB)
	if err == nil || !strings.Contains(err.Error(), domain.ErrNotFound.Error()) {
		t.Errorf("expected ErrNotFound for tenant mismatch on Upsert, got: %v", err)
	}

	_, found, err := store.Get(ctx, tenantB, devServerID)
	if err != nil {
		t.Fatalf("Get B: %v", err)
	}
	if found {
		t.Errorf("expected found=false for wrong tenant")
	}
}

func TestCapabilityProfileRepository_CascadeOnDevServerDelete(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	store := NewCapabilityProfileStore(repo.pool)

	tenantID := uuid.NewString()
	devServerID := uuid.NewString()
	repo.pool.Exec(ctx, "INSERT INTO infra.dev_servers (id, tenant_id, host, connection_mode) VALUES ($1, $2, 'localhost', 'direct-websocket')", devServerID, tenantID)

	p := domain.CapabilityProfile{
		DevServerID:       devServerID,
		TenantID:          tenantID,
		Source:            domain.ProfileSourceProbe,
		AgentBuildVersion: "1.0.0",
		ProtocolVersion:   1,
		Features:          []string{},
		ProfileJSON:       []byte(`{}`),
		Fingerprint:       "fingerprint",
		ProbedAt:          time.Now().Truncate(time.Microsecond),
	}
	store.Upsert(ctx, p)

	repo.pool.Exec(ctx, "DELETE FROM infra.dev_servers WHERE id = $1", devServerID)

	_, found, _ := store.Get(ctx, tenantID, devServerID)
	if found {
		t.Errorf("expected capability profile to cascade delete")
	}
}

func TestCapabilityProfileRepository_UnicodeInProfile(t *testing.T) {
	repo := setupRepository(t)
	ctx := context.Background()
	store := NewCapabilityProfileStore(repo.pool)

	tenantID := uuid.NewString()
	devServerID := uuid.NewString()
	repo.pool.Exec(ctx, "INSERT INTO infra.dev_servers (id, tenant_id, host, connection_mode) VALUES ($1, $2, 'localhost', 'direct-websocket')", devServerID, tenantID)

	p := domain.CapabilityProfile{
		DevServerID:       devServerID,
		TenantID:          tenantID,
		Source:            domain.ProfileSourceProbe,
		AgentBuildVersion: "1.0.0",
		ProtocolVersion:   1,
		Features:          []string{},
		ProfileJSON:       []byte(`{"msg":"chào thế giới"}`),
		Fingerprint:       "fingerprint",
		ProbedAt:          time.Now().Truncate(time.Microsecond),
	}

	_, _, err := store.Upsert(ctx, p)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, _, _ := store.Get(ctx, tenantID, devServerID)
	if string(got.ProfileJSON) != `{"msg":"chào thế giới"}` && string(got.ProfileJSON) != `{"msg": "chào thế giới"}` {
		// Postgres might add space or we check Contains
		if !strings.Contains(string(got.ProfileJSON), "chào thế giới") {
			t.Errorf("expected unicode string preserved, got %s", got.ProfileJSON)
		}
	}
}
