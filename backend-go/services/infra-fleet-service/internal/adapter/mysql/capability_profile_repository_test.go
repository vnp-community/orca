//go:build integration

package mysql

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func hexFingerprint(c string) string { return strings.Repeat(c, 64) }

func seedDevServer(t *testing.T, db *sql.DB, tenantID string) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := db.Exec("INSERT INTO dev_servers (id, tenant_id, host, connection_mode) VALUES (?, ?, 'localhost', 'direct-websocket')", id, tenantID); err != nil {
		t.Fatalf("insert dev_server: %v", err)
	}
	return id
}

func sampleProfile(tenantID, devServerID, fp string) domain.CapabilityProfile {
	return domain.CapabilityProfile{
		DevServerID:       devServerID,
		TenantID:          tenantID,
		Source:            domain.ProfileSourceProbe,
		AgentBuildVersion: "2.2.0",
		ProtocolVersion:   2,
		Features:          []string{"agent.execPrompt", "agent.capabilities"},
		ProfileJSON:       []byte(`{"schemaVersion":1}`),
		Fingerprint:       fp,
		ProbedAt:          time.Now().UTC().Truncate(time.Microsecond),
	}
}

func TestCapabilityProfileRepository_UpsertGet(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	store := NewCapabilityProfileStore(db)
	tenantID := uuid.NewString()
	dsID := seedDevServer(t, db, tenantID)

	p := sampleProfile(tenantID, dsID, hexFingerprint("a"))
	prev, existed, err := store.Upsert(ctx, p)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if existed || prev != "" {
		t.Fatalf("first upsert: existed=%v prev=%q", existed, prev)
	}
	got, found, err := store.Get(ctx, tenantID, dsID)
	if err != nil || !found {
		t.Fatalf("Get: found=%v err=%v", found, err)
	}
	if got.AgentBuildVersion != "2.2.0" || got.ProtocolVersion != 2 || got.Fingerprint != p.Fingerprint ||
		got.Source != domain.ProfileSourceProbe || !got.ProbedAt.Equal(p.ProbedAt) {
		t.Errorf("unexpected profile: %+v", got)
	}
	if len(got.Features) != 2 || got.Features[0] != "agent.capabilities" {
		t.Errorf("features not normalized: %v", got.Features)
	}
}

func TestCapabilityProfileRepository_UpsertReturnsPreviousFingerprint(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	store := NewCapabilityProfileStore(db)
	tenantID := uuid.NewString()
	dsID := seedDevServer(t, db, tenantID)

	if _, _, err := store.Upsert(ctx, sampleProfile(tenantID, dsID, hexFingerprint("1"))); err != nil {
		t.Fatalf("Upsert 1: %v", err)
	}
	prev, existed, err := store.Upsert(ctx, sampleProfile(tenantID, dsID, hexFingerprint("2")))
	if err != nil {
		t.Fatalf("Upsert 2: %v", err)
	}
	if !existed || prev != hexFingerprint("1") {
		t.Errorf("existed=%v prev=%q", existed, prev)
	}
}

func TestCapabilityProfileRepository_ConcurrentUpsertsSerialize(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	store := NewCapabilityProfileStore(db)
	tenantID := uuid.NewString()
	dsID := seedDevServer(t, db, tenantID)

	var wg sync.WaitGroup
	var mu sync.Mutex
	existedCount := 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, existed, err := store.Upsert(ctx, sampleProfile(tenantID, dsID, hexFingerprint(string(rune('a'+i)))))
			if err != nil {
				t.Errorf("Upsert: %v", err)
				return
			}
			if existed {
				mu.Lock()
				existedCount++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if existedCount != 1 {
		t.Errorf("exactly one of two concurrent upserts must see existed=true, got %d", existedCount)
	}
}

func TestCapabilityProfileRepository_TenantIsolation(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	store := NewCapabilityProfileStore(db)
	tenantA, tenantB := uuid.NewString(), uuid.NewString()
	dsID := seedDevServer(t, db, tenantA)

	if _, _, err := store.Upsert(ctx, sampleProfile(tenantA, dsID, hexFingerprint("a"))); err != nil {
		t.Fatalf("Upsert A: %v", err)
	}
	if _, _, err := store.Upsert(ctx, sampleProfile(tenantB, dsID, hexFingerprint("b"))); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("tenant B upserting onto A's dev server: want ErrNotFound, got %v", err)
	}
	if _, found, err := store.Get(ctx, tenantB, dsID); err != nil || found {
		t.Errorf("tenant B must not see A's profile: found=%v err=%v", found, err)
	}
}

func TestCapabilityProfileRepository_CascadeOnDevServerDelete(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	store := NewCapabilityProfileStore(db)
	tenantID := uuid.NewString()
	dsID := seedDevServer(t, db, tenantID)
	if _, _, err := store.Upsert(ctx, sampleProfile(tenantID, dsID, hexFingerprint("a"))); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if _, err := db.Exec("DELETE FROM dev_servers WHERE id = ?", dsID); err != nil {
		t.Fatalf("delete dev server: %v", err)
	}
	if _, found, _ := store.Get(ctx, tenantID, dsID); found {
		t.Errorf("profile must be deleted with its dev server")
	}
}

func TestCapabilityProfileRepository_UnicodeInProfile(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	store := NewCapabilityProfileStore(db)
	tenantID := uuid.NewString()
	dsID := seedDevServer(t, db, tenantID)

	p := sampleProfile(tenantID, dsID, hexFingerprint("a"))
	p.ProfileJSON = []byte(`{"ghiChu":"máy chủ phát triển Việt Nam"}`)
	if _, _, err := store.Upsert(ctx, p); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, _, _ := store.Get(ctx, tenantID, dsID)
	if !strings.Contains(string(got.ProfileJSON), "máy chủ phát triển Việt Nam") {
		t.Errorf("unicode lost: %s", got.ProfileJSON)
	}
}

func TestCapabilityProfileRepository_SourceCheckRejectsUnknown(t *testing.T) {
	db := setupDB(t)
	store := NewCapabilityProfileStore(db)
	tenantID := uuid.NewString()
	dsID := seedDevServer(t, db, tenantID)
	p := sampleProfile(tenantID, dsID, hexFingerprint("a"))
	p.Source = "bogus"
	if _, _, err := store.Upsert(context.Background(), p); err == nil {
		t.Error("CHECK constraint must reject an unknown source")
	}
}
