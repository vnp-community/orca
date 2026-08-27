package vault_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/secrets"
	credentialvault "github.com/stablyai/orca-go/services/credential-broker-service/internal/adapter/vault"
)

// fakeVaultServer stands in for a real Vault server — just enough of the
// wire protocol (GET /v1/sys/health, DELETE <mount>/metadata/<path>) to
// prove SecretStore.Ping/RevokeSecret now call through to the real
// health-check and destroy-metadata endpoints, instead of the prior
// KV-absent-path heuristic and overwrite-with-empty-payload workaround —
// see secret_store.go's doc comments. No real Vault instance is available
// in this sandbox, same honest gap this codebase's other adapter tests
// already flag rather than silently presenting a fake as equivalent to live
// coverage.
type fakeVaultServer struct {
	healthCalls  []string
	deleteCalls  []string
	kvWriteCalls []string
	healthStatus int
}

func newFakeVaultServer(t *testing.T, fv *fakeVaultServer) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sys/health", func(w http.ResponseWriter, r *http.Request) {
		fv.healthCalls = append(fv.healthCalls, r.Method+" "+r.URL.Path)
		status := fv.healthStatus
		if status == 0 {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"initialized": true, "sealed": false, "standby": false, "version": "1.15.0",
		})
	})
	mux.HandleFunc("/v1/credential-secrets/metadata/tenant-1/cred-1", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			fv.deleteCalls = append(fv.deleteCalls, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/v1/credential-secrets/data/tenant-1/cred-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			fv.kvWriteCalls = append(fv.kvWriteCalls, r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	// Catch-all so an unexpected call (e.g. a stray KV read/write during
	// Ping) fails loudly via a 404 the vault SDK surfaces as an error,
	// rather than silently succeeding against a handler that wasn't meant
	// to answer it.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"errors": []string{"not found: " + r.URL.Path}})
	})
	return httptest.NewServer(mux)
}

func newTestStore(t *testing.T, fv *fakeVaultServer) *credentialvault.SecretStore {
	t.Helper()
	server := newFakeVaultServer(t, fv)
	t.Cleanup(server.Close)
	t.Setenv("VAULT_ADDR", server.URL)
	t.Setenv("VAULT_TOKEN", "test-token")

	client, err := secrets.NewClient()
	if err != nil {
		t.Fatalf("secrets.NewClient: %v", err)
	}
	return credentialvault.New(client)
}

// TestPing_CallsRealHealthEndpoint proves Ping now delegates to a genuine
// Sys().Health() call (GET /v1/sys/health) rather than the old
// well-known-absent-KV-path heuristic — regression coverage for the
// documented change in secret_store.go.
func TestPing_CallsRealHealthEndpoint(t *testing.T) {
	fv := &fakeVaultServer{}
	store := newTestStore(t, fv)

	if err := store.Ping(context.Background()); err != nil {
		t.Fatalf("expected Ping to succeed against a healthy fake Vault, got: %v", err)
	}
	if len(fv.healthCalls) != 1 {
		t.Fatalf("expected exactly 1 call to /v1/sys/health, got %d: %v", len(fv.healthCalls), fv.healthCalls)
	}
	// The old heuristic read a KV v2 path under credential-secrets; that
	// must no longer happen for a Ping call.
	if len(fv.kvWriteCalls) != 0 {
		t.Errorf("expected Ping to make no KV calls, got %v", fv.kvWriteCalls)
	}
}

// TestPing_UnreachableVaultErrors proves a genuinely unreachable Vault
// still surfaces as an error through the real health check, not just a
// "not found" heuristic result.
func TestPing_UnreachableVaultErrors(t *testing.T) {
	t.Setenv("VAULT_ADDR", "http://127.0.0.1:1") // nothing listens here
	t.Setenv("VAULT_TOKEN", "test-token")

	client, err := secrets.NewClient()
	if err != nil {
		t.Fatalf("secrets.NewClient: %v", err)
	}
	store := credentialvault.New(client)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := store.Ping(ctx); err == nil {
		t.Fatal("expected Ping to fail against an unreachable Vault")
	}
}

// TestRevokeSecret_CallsDestroyMetadata proves RevokeSecret now calls
// Vault's native metadata-destroy endpoint (DELETE <mount>/metadata/<path>)
// instead of the prior KVWrite-with-empty-payload overwrite.
func TestRevokeSecret_CallsDestroyMetadata(t *testing.T) {
	fv := &fakeVaultServer{}
	store := newTestStore(t, fv)

	if err := store.RevokeSecret(context.Background(), "credential-secrets", "tenant-1/cred-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fv.deleteCalls) != 1 {
		t.Fatalf("expected exactly 1 metadata-destroy call, got %d: %v", len(fv.deleteCalls), fv.deleteCalls)
	}
	if want := "/v1/credential-secrets/metadata/tenant-1/cred-1"; fv.deleteCalls[0] != want {
		t.Errorf("expected destroy call to %s, got %s", want, fv.deleteCalls[0])
	}
	// The old workaround issued a KVWrite (POST/PUT to the data path); that
	// must no longer happen for a RevokeSecret call.
	if len(fv.kvWriteCalls) != 0 {
		t.Errorf("expected RevokeSecret to make no KV write calls, got %v", fv.kvWriteCalls)
	}
}
